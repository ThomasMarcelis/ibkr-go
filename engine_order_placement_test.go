package ibkr

import (
	"context"
	"errors"
	"io"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

func TestPlaceOrderAdmissionWinsCallerBoundaries(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(47)
	t.Cleanup(func() { _ = handle.Close() })

	for _, tc := range []struct {
		name         string
		ctx          context.Context
		e            *engine
		closedResult bool
	}{
		{
			name: "caller cancellation",
			ctx:  canceledContext(),
			e:    placementWaitEngine(false),
		},
		{
			name:         "engine shutdown",
			ctx:          context.Background(),
			e:            placementWaitEngine(true),
			closedResult: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.closedResult {
				handle.closeWithErr(ErrClosed)
			}
			resp := make(chan placeOrderResult, 1)
			resp <- placeOrderResult{handle: handle}

			got, err := awaitPlaceOrderResponse(tc.ctx, tc.e, resp)
			if err != nil {
				t.Fatalf("awaitPlaceOrderResponse() error = %v, want nil", err)
			}
			if got != handle {
				t.Fatalf("awaitPlaceOrderResponse() handle = %p, want %p", got, handle)
			}
			if tc.closedResult {
				if err := got.Wait(); !errors.Is(err, ErrClosed) {
					t.Fatalf("shutdown-raced handle error = %v, want ErrClosed", err)
				}
			}
		})
	}

	resp := make(chan placeOrderResult, 1)
	resp <- placeOrderResult{err: context.Canceled}
	got, err := awaitPlaceOrderResponse(canceledContext(), placementWaitEngine(false), resp)
	if got != nil {
		t.Fatalf("pre-admission handle = %p, want nil", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-admission error = %v, want context.Canceled", err)
	}
}

func TestBracketAdmissionWinsCallerBoundaries(t *testing.T) {
	t.Parallel()

	bracket := BracketOrder{
		Parent:     newOrderHandle(47),
		TakeProfit: newOrderHandle(48),
		StopLoss:   newOrderHandle(49),
	}
	t.Cleanup(func() {
		_ = bracket.Parent.Close()
		_ = bracket.TakeProfit.Close()
		_ = bracket.StopLoss.Close()
	})

	for _, tc := range []struct {
		name         string
		ctx          context.Context
		e            *engine
		closedResult bool
	}{
		{
			name: "caller cancellation",
			ctx:  canceledContext(),
			e:    placementWaitEngine(false),
		},
		{
			name:         "engine shutdown",
			ctx:          context.Background(),
			e:            placementWaitEngine(true),
			closedResult: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.closedResult {
				bracket.Parent.closeWithErr(ErrClosed)
				bracket.TakeProfit.closeWithErr(ErrClosed)
				bracket.StopLoss.closeWithErr(ErrClosed)
			}
			resp := make(chan bracketOrderResult, 1)
			resp <- bracketOrderResult{bracket: bracket}

			got, err := awaitBracketOrderResponse(tc.ctx, tc.e, resp)
			if err != nil {
				t.Fatalf("awaitBracketOrderResponse() error = %v, want nil", err)
			}
			if got != bracket {
				t.Fatalf("awaitBracketOrderResponse() = %+v, want %+v", got, bracket)
			}
			if tc.closedResult {
				if err := got.Parent.Wait(); !errors.Is(err, ErrClosed) {
					t.Fatalf("shutdown-raced parent error = %v, want ErrClosed", err)
				}
			}
		})
	}

	resp := make(chan bracketOrderResult, 1)
	resp <- bracketOrderResult{err: context.Canceled}
	got, err := awaitBracketOrderResponse(canceledContext(), placementWaitEngine(false), resp)
	if got != (BracketOrder{}) {
		t.Fatalf("pre-admission bracket = %+v, want zero value", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-admission error = %v, want context.Canceled", err)
	}
}

// TestBracketRollbackCancelsOnlyAdmittedOrders uses the IDs from the live
// server_version 200 api_bracket_trigger_aapl capture (sha256 prefix
// 682a1390b2acf04c). The pipe observes outbound cancel_order frames only; no
// Gateway callback is invented.
func TestBracketRollbackCancelsOnlyAdmittedOrders(t *testing.T) {
	t.Parallel()

	e, peer, handles := newRollbackTestEngine(t, []int64{47, 48, 49})
	placementErr := errors.New("stop-loss placement not admitted")

	gotErr := e.cancelAndCloseOrderRoutes([]int64{47, 48}, []int64{47, 48, 49}, placementErr)
	recovery, ok := errors.AsType[*OrderRecoveryError](gotErr)
	if !ok {
		t.Fatalf("rollback error type = %T, want *OrderRecoveryError", gotErr)
	}
	if !slices.Equal(recovery.OrderIDs, []int64{47, 48}) {
		t.Fatalf("recovery order IDs = %v, want [47 48]", recovery.OrderIDs)
	}
	if recovery.CancelErr != nil {
		t.Fatalf("cancel error = %v, want nil after every cancel entered the queue", recovery.CancelErr)
	}
	if !errors.Is(gotErr, placementErr) {
		t.Fatalf("rollback error = %v, want placement cause %v", gotErr, placementErr)
	}
	if IsRetryable(gotErr) {
		t.Fatal("OrderRecoveryError is retryable; retrying could duplicate live orders")
	}
	if text := gotErr.Error(); !strings.Contains(text, "admitted but not acknowledged") {
		t.Fatalf("rollback error text = %q, want unacknowledged cancellation status", text)
	}
	assertBracketPlacementFailure(t, gotErr, gotErr)
	// A known valid cancel after rollback fences the transport queue. Seeing it
	// immediately after 47/48 proves rollback did not enqueue unsent ID 49.
	if err := e.sendContext(context.Background(), codec.CancelOrderRequest{OrderID: 47}); err != nil {
		t.Fatalf("enqueue transport fence: %v", err)
	}

	for _, want := range [][]string{
		{"4", "47", "", "", ""},
		{"4", "48", "", "", ""},
		{"4", "47", "", "", ""},
	} {
		if got := readWireFields(t, peer); !slices.Equal(got, want) {
			t.Fatalf("cancel_order fields = %q, want %q", got, want)
		}
	}

	assertRollbackRoutesClosed(t, e, handles, gotErr)
}

func TestBracketRollbackBeforeAdmissionReturnsPlacementError(t *testing.T) {
	t.Parallel()

	e, peer, handles := newRollbackTestEngine(t, []int64{47, 48, 49})
	placementErr := errors.New("parent placement not admitted")

	gotErr := e.cancelAndCloseOrderRoutes(nil, []int64{47, 48, 49}, placementErr)
	if gotErr != placementErr {
		t.Fatalf("rollback error = %v, want original placement error", gotErr)
	}
	if _, ok := errors.AsType[*OrderRecoveryError](gotErr); ok {
		t.Fatalf("pre-admission error type = %T, want original placement error", gotErr)
	}
	assertBracketPlacementFailure(t, gotErr, placementErr)
	assertRollbackRoutesClosed(t, e, handles, placementErr)
	_ = peer.Close()
}

func TestBracketRollbackReportsExactUncertainOrders(t *testing.T) {
	t.Parallel()

	e, peer, handles := newRollbackTestEngine(t, []int64{47, 48, 49})
	fillTransportQueue(t, e.transport, peer)
	placementErr := errors.New("stop-loss placement not admitted")
	sentIDs := []int64{47, 48}

	gotErr := e.cancelAndCloseOrderRoutes(sentIDs, []int64{47, 48, 49}, placementErr)
	recovery, ok := errors.AsType[*OrderRecoveryError](gotErr)
	if !ok {
		t.Fatalf("rollback error type = %T, want *OrderRecoveryError", gotErr)
	}
	sentIDs[0] = 999
	if !slices.Equal(recovery.OrderIDs, []int64{47, 48}) {
		t.Fatalf("recovery order IDs = %v, want [47 48]", recovery.OrderIDs)
	}
	if slices.Contains(recovery.OrderIDs, int64(49)) {
		t.Fatalf("recovery order IDs include unsent order 49: %v", recovery.OrderIDs)
	}
	if recovery.PlacementErr != placementErr || !errors.Is(gotErr, placementErr) {
		t.Fatalf("placement cause = %v, want %v", recovery.PlacementErr, placementErr)
	}
	if !errors.Is(recovery.CancelErr, ErrInterrupted) || !errors.Is(gotErr, ErrInterrupted) {
		t.Fatalf("cancel cause = %v, want ErrInterrupted", recovery.CancelErr)
	}
	if IsRetryable(gotErr) {
		t.Fatal("OrderRecoveryError is retryable; retrying could duplicate live orders")
	}
	if text := gotErr.Error(); !strings.Contains(text, "[47 48]") || !strings.Contains(text, "recovery required") {
		t.Fatalf("rollback error text = %q, want exact IDs and recovery guidance", text)
	}

	assertBracketPlacementFailure(t, gotErr, gotErr)

	assertRollbackRoutesClosed(t, e, handles, gotErr)
	_ = peer.Close()
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func placementWaitEngine(closed bool) *engine {
	e := &engine{done: make(chan struct{})}
	if closed {
		close(e.done)
	}
	return e
}

func newRollbackTestEngine(t *testing.T, orderIDs []int64) (*engine, net.Conn, map[int64]*OrderHandle) {
	t.Helper()

	peer, clientConn := net.Pipe()
	tr := transport.New(clientConn, nil, 0)
	e := &engine{
		done:           make(chan struct{}),
		transport:      tr,
		serverVersion:  200,
		orders:         make(map[int64]*orderRoute),
		execDeliveries: make(map[string]*execDelivery),
	}
	handles := make(map[int64]*OrderHandle, len(orderIDs))
	for _, orderID := range orderIDs {
		handles[orderID] = e.bindOrderHandle(orderID, Contract{ConID: 265598, Exchange: "SMART"})
	}
	t.Cleanup(func() {
		_ = tr.Close()
		_ = tr.Wait()
		_ = peer.Close()
	})
	return e, peer, handles
}

func fillTransportQueue(t *testing.T, tr *transport.Conn, peer net.Conn) {
	t.Helper()

	payload, err := codec.Encode(200, codec.CancelOrderRequest{OrderID: 47})
	if err != nil {
		t.Fatalf("encode live-derived cancel frame: %v", err)
	}
	// Consume only the first frame header. The transport writer has then
	// dequeued that frame and is blocked writing its unread payload, so the
	// queue capacity below is stable without sleeps or scheduler assumptions.
	if err := tr.Send(context.Background(), payload); err != nil {
		t.Fatalf("prime transport writer: %v", err)
	}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set transport-prime deadline: %v", err)
	}
	var header [4]byte
	if _, err := io.ReadFull(peer, header[:]); err != nil {
		t.Fatalf("wait for transport writer: %v", err)
	}
	if err := peer.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear transport-prime deadline: %v", err)
	}

	for range 512 {
		if err := tr.Send(context.Background(), payload); err != nil {
			if !errors.Is(err, transport.ErrSendQueueFull) {
				t.Fatalf("fill transport queue: %v", err)
			}
			return
		}
	}
	t.Fatal("transport queue did not fill")
}

func assertRollbackRoutesClosed(t *testing.T, e *engine, handles map[int64]*OrderHandle, wantErr error) {
	t.Helper()

	if len(e.orders) != 0 {
		t.Fatalf("order routes after rollback = %d, want 0", len(e.orders))
	}
	for orderID, handle := range handles {
		select {
		case <-handle.Done():
		default:
			t.Fatalf("order %d handle remains open after rollback", orderID)
		}
		if err := handle.Wait(); err != wantErr {
			t.Fatalf("order %d handle error = %v, want %v", orderID, err, wantErr)
		}
	}
}

func assertBracketPlacementFailure(t *testing.T, resultErr, wantErr error) {
	t.Helper()

	resp := make(chan bracketOrderResult, 1)
	resp <- bracketOrderResult{err: resultErr}
	bracket, err := awaitBracketOrderResponse(context.Background(), placementWaitEngine(false), resp)
	if bracket != (BracketOrder{}) {
		t.Fatalf("failed bracket = %+v, want zero value", bracket)
	}
	if err != wantErr {
		t.Fatalf("failed bracket error = %v, want %v", err, wantErr)
	}
}
