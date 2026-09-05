package ibkr

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

// These tests decode exact raw server frames from the paper-Gateway
// api_future_campaign_mes capture at server_version 225 instead of
// reconstructing protocol messages. Its events.jsonl SHA-256 is
// dd5eeefb0d5bb095dc3da778767570b5286f733491489cde66abcf70488bd005.
// The capture records each MES fill live, its fee, the same fill replayed by
// an execution query, and the exact fee replay. Tests state any ordering fault
// injection explicitly; captured message fields are never changed.

const (
	executionsCapturePath = "testdata/transcripts/api_future_campaign_mes.txt"
	capturedReplayExecID  = "sanitized-exec-00000001"
)

func TestExecutionSubscriptionAcceptsCorrelationBoundaryAndRejectsNextID(t *testing.T) {
	executions := capturedExecutions(t)
	if len(executions) < 2 {
		t.Fatalf("captured executions = %d, want at least two", len(executions))
	}

	e, peer := newObservedExecutionEngine(t)
	reqID, sub := installObservedExecutionRoute(t, e, WithQueueSize(16), WithExecutionCorrelationLimit(1))
	_ = readObservedFrame(t, peer)
	route := e.keyed[reqID]

	cleanupCalls := 0
	cleanup := route.cleanup
	route.cleanup = func() {
		cleanupCalls++
		cleanup()
	}

	route.handle(executions[0], e)
	route.handle(executions[1], e)

	err := sub.Wait()
	if !errors.Is(err, ErrExecutionCorrelationOverflow) {
		t.Fatalf("Wait() error = %v, want ErrExecutionCorrelationOverflow", err)
	}
	if IsRetryable(err) {
		t.Fatalf("IsRetryable(%v) = true, want false", err)
	}
	if _, ok := e.keyed[reqID]; ok {
		t.Fatalf("overflowed execution route %d retained", reqID)
	}
	if cleanupCalls != 1 || route.cleanup != nil {
		t.Fatalf("route cleanup calls/state = %d/%v, want one call and nil callback", cleanupCalls, route.cleanup != nil)
	}

	updates := closedExecutionUpdates(sub)
	if len(updates) != 1 {
		t.Fatalf("data updates = %d, want one boundary execution", len(updates))
	}
	if updates[0].Execution == nil || updates[0].Execution.ExecID != executions[0].ExecID {
		t.Fatalf("execution updates = %+v, want only the admitted ID", updates)
	}
}

func TestExecutionSubscriptionOrdersCapturedFeeBeforeExecutionAndDedupesReplay(t *testing.T) {
	sequence := capturedQueryReplaySequence(t)
	firstFee := sequence[0].(codec.CommissionReport)
	execution := sequence[1].(codec.ExecutionDetail)
	replayedFee := sequence[2].(codec.CommissionReport)
	if firstFee != replayedFee {
		t.Fatalf("captured fee replay changed: first = %+v, replay = %+v", firstFee, replayedFee)
	}

	e, peer := newObservedExecutionEngine(t)
	reqID, sub := installObservedExecutionRoute(t, e, WithQueueSize(16), WithExecutionCorrelationLimit(1))
	_ = readObservedFrame(t, peer)
	route := e.keyed[reqID]

	// This is the capture's projected query-leg order: the first fee precedes
	// the queried execution detail and the identical replay follows it.
	route.handleCommission(firstFee, e)
	route.handle(execution, e)
	route.handleCommission(replayedFee, e)

	sub.Close()
	(<-e.cmds)()
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait(): %v", err)
	}

	updates := closedExecutionUpdates(sub)
	if len(updates) != 2 {
		t.Fatalf("data updates = %d, want execution + one exact fee", len(updates))
	}
	if updates[0].Execution == nil || updates[0].Execution.ExecID != capturedReplayExecID {
		t.Fatalf("first update = %+v, want execution before queued fee", updates[0])
	}
	if updates[1].CommissionAndFees == nil || updates[1].CommissionAndFees.ExecID != capturedReplayExecID ||
		updates[1].CommissionAndFees.Amount == nil || updates[1].CommissionAndFees.Amount.String() != "0.61" {
		t.Fatalf("second update = %+v, want the captured fee once", updates[1].CommissionAndFees)
	}
}

func TestExecutionSubscriptionQueuedExactReplayDoesNotConsumeCapacity(t *testing.T) {
	sequence := capturedQueryReplaySequence(t)
	firstFee := sequence[0].(codec.CommissionReport)
	execution := sequence[1].(codec.ExecutionDetail)
	replayedFee := sequence[2].(codec.CommissionReport)

	e, peer := newObservedExecutionEngine(t)
	reqID, sub := installObservedExecutionRoute(t, e, WithQueueSize(16), WithExecutionCorrelationLimit(1))
	_ = readObservedFrame(t, peer)
	route := e.keyed[reqID]

	// Fault injection moves the capture's exact replay ahead of its execution
	// detail. No message fields change. A consecutive identical pending report
	// must not consume a second version slot at the configured boundary.
	route.handleCommission(firstFee, e)
	route.handleCommission(replayedFee, e)
	route.handle(execution, e)

	select {
	case <-sub.Done():
		t.Fatalf("subscription closed at the exact pending boundary: %v", sub.Err())
	default:
	}

	sub.Close()
	(<-e.cmds)()
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait(): %v", err)
	}
	if updates := closedExecutionUpdates(sub); len(updates) != 2 ||
		updates[0].Execution == nil || updates[1].CommissionAndFees == nil {
		t.Fatalf("updates = %+v, want execution then one queued fee", updates)
	}
}

func TestExecutionSubscriptionIgnoresUnknownFeesAfterSnapshotAndKeepsKnownLateFee(t *testing.T) {
	sequence := capturedQueryReplaySequence(t)
	execution := sequence[1].(codec.ExecutionDetail)
	lateKnownFee := sequence[2].(codec.CommissionReport)
	var unrelated []codec.CommissionReport
	for _, report := range capturedCommissionsFrom(capturedServerMessages(t, executionsCapturePath)) {
		if report.ExecID != execution.ExecID {
			unrelated = append(unrelated, report)
		}
	}
	if len(unrelated) == 0 {
		t.Fatal("capture contains no fee for the other MES execution")
	}

	e, peer := newObservedExecutionEngine(t)
	reqID, sub := installObservedExecutionRoute(t, e, WithQueueSize(16), WithExecutionCorrelationLimit(1))
	_ = readObservedFrame(t, peer)
	route := e.keyed[reqID]

	route.handle(execution, e)
	route.handle(capturedExecutionsEnd(t, executionsCapturePath), e)
	if err := sub.AwaitSnapshot(context.Background()); err != nil {
		t.Fatalf("AwaitSnapshot(): %v", err)
	}

	// Structural routing fault injection moves exact live-derived commission
	// frames for other executions after this route's end marker. Message fields
	// are unchanged. A completed route cannot receive another keyed execution,
	// so these global broadcasts must not claim correlation capacity.
	for _, report := range unrelated {
		route.handleCommission(report, e)
	}
	select {
	case <-sub.Done():
		t.Fatalf("unrelated post-end fees closed subscription: %v", sub.Err())
	default:
	}
	if e.keyed[reqID] != route {
		t.Fatalf("execution route %d lost after unrelated post-end fees", reqID)
	}

	// The exact captured replay fee is moved after the end marker without
	// changing its fields. Its execution was observed, so the late fee remains
	// deliverable even though unknown ExecIDs are now ignored.
	route.handleCommission(lateKnownFee, e)
	updates := availableExecutionUpdates(sub)
	if len(updates) != 2 || updates[0].Execution == nil ||
		updates[0].Execution.ExecID != execution.ExecID ||
		updates[1].CommissionAndFees == nil ||
		updates[1].CommissionAndFees.ExecID != execution.ExecID {
		t.Fatalf("updates = %+v, want known execution then its late fee", updates)
	}

	sub.Close()
	(<-e.cmds)()
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait(): %v", err)
	}
}

func TestExecutionCorrelationSnapshotCompletionDropsUnmatchedPendingReports(t *testing.T) {
	reports := capturedCommissionsFrom(capturedServerMessages(t, executionsCapturePath))
	executions := executionByID(capturedExecutions(t))
	if len(reports) < 2 {
		t.Fatalf("captured commissions = %d, want at least two", len(reports))
	}

	correlation := newExecutionCorrelation(2)
	unmatched, err := correlation.entry(reports[0].ExecID)
	if err != nil {
		t.Fatalf("entry(unmatched): %v", err)
	}
	if queued, err := correlation.queuePending(unmatched, reports[0]); err != nil || !queued {
		t.Fatalf("queuePending(unmatched) = %v, %v; want true, nil", queued, err)
	}
	knownExecution, ok := executions[reports[1].ExecID]
	if !ok {
		t.Fatalf("capture has fee %q without execution detail", reports[1].ExecID)
	}
	known, err := correlation.entry(knownExecution.ExecID)
	if err != nil {
		t.Fatalf("entry(known): %v", err)
	}
	known.executionSeen = true

	correlation.completeSnapshot()

	if !correlation.snapshotComplete {
		t.Fatal("snapshot completion was not recorded")
	}
	if correlation.pendingReports != 0 {
		t.Fatalf("pending reports = %d, want zero", correlation.pendingReports)
	}
	if len(correlation.byExecID) != 1 || correlation.byExecID[knownExecution.ExecID] != known {
		t.Fatalf("retained entries = %+v, want only known execution %q", correlation.byExecID, knownExecution.ExecID)
	}
}

func TestExecutionCorrelationBoundsPendingReportsIndependently(t *testing.T) {
	reports := capturedCommissionsFrom(capturedServerMessages(t, executionsCapturePath))
	if len(reports) < 2 {
		t.Fatalf("captured commissions = %d, want at least two", len(reports))
	}

	correlation := newExecutionCorrelation(2)
	// Production gives both resources the public limit. A lower internal
	// pending ceiling isolates that invariant with three unchanged live-derived
	// reports; it does not claim evidence of a changed same-ID fee revision.
	correlation.pendingLimit = 1
	for i, report := range reports[:2] {
		entry, err := correlation.entry(report.ExecID)
		if err != nil {
			t.Fatalf("entry(%d): %v", i, err)
		}
		queued, err := correlation.queuePending(entry, report)
		if i < 1 {
			if err != nil || !queued {
				t.Fatalf("queuePending(%d) = %v, %v; want true, nil", i, queued, err)
			}
			continue
		}
		if queued || !errors.Is(err, ErrExecutionCorrelationOverflow) {
			t.Fatalf("queuePending(%d) = %v, %v; want false, ErrExecutionCorrelationOverflow", i, queued, err)
		}
	}
	if correlation.pendingReports != 1 {
		t.Fatalf("pending reports = %d, want boundary 1", correlation.pendingReports)
	}
}

func TestUnrelatedCommissionBroadcastOverflowClosesOnlyBoundedRoute(t *testing.T) {
	messages := capturedServerMessages(t, executionsCapturePath)
	executions := executionByID(capturedExecutionsFrom(messages))
	reports := capturedCommissionsFrom(messages)
	if len(reports) < 2 {
		t.Fatalf("captured commissions = %d, want at least two", len(reports))
	}
	reports = reports[:2]

	e, peer := newObservedExecutionEngine(t)
	targetReqID, target := installObservedExecutionRoute(t, e, WithQueueSize(16), WithExecutionCorrelationLimit(1))
	_ = readObservedFrame(t, peer)
	siblingReqID, sibling := installObservedExecutionRoute(t, e, WithQueueSize(16), WithExecutionCorrelationLimit(2))
	_ = readObservedFrame(t, peer)

	for _, report := range reports {
		_, ok := executions[report.ExecID]
		if !ok {
			t.Fatalf("capture has fee %q without execution detail", report.ExecID)
		}
		// Broadcast exact captured fees before their execution details reach
		// either query. There are no local order handles to correlate.
		e.handleIncoming(report)
	}

	if err := target.Wait(); !errors.Is(err, ErrExecutionCorrelationOverflow) {
		t.Fatalf("target Wait() error = %v, want ErrExecutionCorrelationOverflow", err)
	}
	if _, ok := e.keyed[targetReqID]; ok {
		t.Fatalf("target route %d retained after unrelated broadcast overflow", targetReqID)
	}
	if updates := closedExecutionUpdates(target); len(updates) != 0 {
		t.Fatalf("target emitted unmatched fee callbacks: %+v", updates)
	}
	select {
	case <-sibling.Done():
		t.Fatalf("sibling route closed with target: %v", sibling.Err())
	default:
	}

	siblingRoute := e.keyed[siblingReqID]
	siblingRoute.handle(executions[reports[1].ExecID], e)
	updates := availableExecutionUpdates(sibling)
	if len(updates) != 2 || updates[0].Execution == nil || updates[1].CommissionAndFees == nil {
		t.Fatalf("sibling updates = %+v, want execution then its queued fee", updates)
	}

	sibling.Close()
	(<-e.cmds)()
	if err := sibling.Wait(); err != nil {
		t.Fatalf("sibling Wait(): %v", err)
	}
}

func TestExecutionSnapshotCollectorUsesCorrelationLimit(t *testing.T) {
	var replayed []codec.ExecutionDetail
	for _, execution := range capturedExecutions(t) {
		if execution.ExecID == capturedReplayExecID {
			replayed = append(replayed, execution)
		}
	}
	if len(replayed) != 2 {
		t.Fatalf("captured fill replay executions = %d, want live + query copies", len(replayed))
	}

	e, peer := newObservedExecutionEngine(t)
	reqID, sub := installObservedExecutionRoute(
		t,
		e,
		withSnapshotCollector(),
		WithExecutionCorrelationLimit(1),
	)
	_ = readObservedFrame(t, peer)
	route := e.keyed[reqID]

	// The capture contains the same ExecID first as an unsolicited live fill
	// and then as an execution-query replay. Feed both exact callbacks to the
	// collector as a defensive duplicate-delivery fault injection.
	route.handle(replayed[0], e)
	route.handle(replayed[1], e)

	err := sub.Wait()
	if !errors.Is(err, ErrExecutionCorrelationOverflow) {
		t.Fatalf("snapshot collector Wait() error = %v, want ErrExecutionCorrelationOverflow", err)
	}
	if got := len(sub.takeSnapshotEvents()); got != 1 {
		t.Fatalf("snapshot collector retained %d events, want configured boundary 1", got)
	}
	if _, ok := e.keyed[reqID]; ok {
		t.Fatalf("overflowed snapshot route %d retained", reqID)
	}
}

func newObservedExecutionEngine(t *testing.T) (*engine, net.Conn) {
	t.Helper()
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 225
	e.nextReqID = 701
	return e, peer
}

func installObservedExecutionRoute(
	t *testing.T,
	e *engine,
	opts ...SubscriptionOption,
) (int, *Subscription[ExecutionUpdate]) {
	t.Helper()
	type result struct {
		sub *Subscription[ExecutionUpdate]
		err error
	}
	reqID := e.nextReqID
	resultCh := make(chan result, 1)
	go func() {
		sub, err := e.subscribeExecutions(context.Background(), ExecutionsRequest{}, opts...)
		resultCh <- result{sub: sub, err: err}
	}()
	(<-e.cmds)()
	out := <-resultCh
	if out.err != nil {
		t.Fatalf("subscribeExecutions: %v", out.err)
	}
	return reqID, out.sub
}

func capturedServerMessages(t *testing.T, path string) []codec.Message {
	t.Helper()
	var (
		data []byte
		err  error
	)
	switch path {
	case executionsCapturePath:
		data, err = os.ReadFile(executionsCapturePath)
	case "testdata/transcripts/api_hedge_order_aapl.txt":
		data, err = os.ReadFile("testdata/transcripts/api_hedge_order_aapl.txt")
	default:
		t.Fatalf("unsupported execution capture %q", path)
	}
	if err != nil {
		t.Fatalf("read capture %s: %v", path, err)
	}

	var messages []codec.Message
	for lineNumber, line := range strings.Split(string(data), "\n") {
		encoded, ok := strings.CutPrefix(line, "raw server ")
		if !ok {
			continue
		}
		frame, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("decode capture %s line %d: %v", path, lineNumber+1, err)
		}
		reader := bytes.NewReader(frame)
		payload, err := wire.ReadFrame(reader)
		if err != nil {
			t.Fatalf("read capture frame %s line %d: %v", path, lineNumber+1, err)
		}
		if reader.Len() != 0 {
			t.Fatalf("capture %s line %d has %d trailing frame bytes", path, lineNumber+1, reader.Len())
		}
		decoded, err := codec.DecodeBatch(225, payload)
		if err != nil {
			t.Fatalf("decode capture frame %s line %d: %v", path, lineNumber+1, err)
		}
		messages = append(messages, decoded...)
	}
	return messages
}

func capturedExecutions(t *testing.T) []codec.ExecutionDetail {
	t.Helper()
	return capturedExecutionsFrom(capturedServerMessages(t, executionsCapturePath))
}

func capturedExecutionsFrom(messages []codec.Message) []codec.ExecutionDetail {
	var executions []codec.ExecutionDetail
	for _, message := range messages {
		if execution, ok := message.(codec.ExecutionDetail); ok {
			executions = append(executions, execution)
		}
	}
	return executions
}

func capturedCommissionsFrom(messages []codec.Message) []codec.CommissionReport {
	var reports []codec.CommissionReport
	for _, message := range messages {
		if report, ok := message.(codec.CommissionReport); ok {
			reports = append(reports, report)
		}
	}
	return reports
}

func capturedExecutionsEnd(t *testing.T, path string) codec.ExecutionsEnd {
	t.Helper()
	for _, message := range capturedServerMessages(t, path) {
		if end, ok := message.(codec.ExecutionsEnd); ok {
			return end
		}
	}
	t.Fatalf("capture %s has no executions end", path)
	return codec.ExecutionsEnd{}
}

func executionByID(executions []codec.ExecutionDetail) map[string]codec.ExecutionDetail {
	byID := make(map[string]codec.ExecutionDetail, len(executions))
	for _, execution := range executions {
		byID[execution.ExecID] = execution
	}
	return byID
}

func capturedQueryReplaySequence(t *testing.T) []any {
	t.Helper()
	var sequence []any
	for _, message := range capturedServerMessages(t, executionsCapturePath) {
		switch message := message.(type) {
		case codec.ExecutionDetail:
			if message.ExecID == capturedReplayExecID && message.ReqID >= 0 {
				sequence = append(sequence, message)
			}
		case codec.CommissionReport:
			if message.ExecID == capturedReplayExecID {
				sequence = append(sequence, message)
			}
		}
	}
	if len(sequence) != 3 {
		t.Fatalf("captured query replay sequence = %d messages, want fee + execution + fee", len(sequence))
	}
	if _, ok := sequence[0].(codec.CommissionReport); !ok {
		t.Fatalf("captured query replay message 0 = %T, want codec.CommissionReport", sequence[0])
	}
	if _, ok := sequence[1].(codec.ExecutionDetail); !ok {
		t.Fatalf("captured query replay message 1 = %T, want codec.ExecutionDetail", sequence[1])
	}
	if _, ok := sequence[2].(codec.CommissionReport); !ok {
		t.Fatalf("captured query replay message 2 = %T, want codec.CommissionReport", sequence[2])
	}
	return sequence
}

func closedExecutionUpdates(sub *Subscription[ExecutionUpdate]) []ExecutionUpdate {
	var updates []ExecutionUpdate
	for event := range sub.Events() {
		if event.Kind == StreamData {
			updates = append(updates, event.Value)
		}
	}
	return updates
}

func availableExecutionUpdates(sub *Subscription[ExecutionUpdate]) []ExecutionUpdate {
	var updates []ExecutionUpdate
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				return updates
			}
			if event.Kind == StreamData {
				updates = append(updates, event.Value)
			}
		default:
			return updates
		}
	}
}
