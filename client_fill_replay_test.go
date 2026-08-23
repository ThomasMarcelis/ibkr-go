package ibkr_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// The replays below freeze the regular-session fill family (matrix row
// ORD-001 fill, modify, and order-type variants) captured live on 2026-06-11
// against paper Gateway server_version 200 during US market hours. They are
// the first replays with real executions and commission reports: every fill
// is asserted through order_status plus Execution/Commission events on the
// order handle (never through OpenOrder, which carries no fill echo on the
// live wire). Live execution times arrive in the Gateway's UTC dash form
// ("20260611-13:30:30"); the executions query re-emits them in the
// "US/Eastern" space form. Both parse, so executions and commissions must
// reach the handles.

// fillReplayAAPLBySymbol mirrors the capture drivers that placed orders with
// a symbol-only contract: the encoder sends con_id 0 and the Gateway echo
// resolves con_id 265598.
var fillReplayAAPLBySymbol = ibkr.Contract{
	Symbol:   "AAPL",
	SecType:  ibkr.SecTypeStock,
	Exchange: "SMART",
	Currency: "USD",
}

// orderEventLog consumes a handle's events on demand and records everything
// it saw, so lifecycle waits and end-of-replay tallies share one consumer.
type orderEventLog struct {
	handle *ibkr.OrderHandle
	events []ibkr.OrderEvent
}

func newOrderEventLog(handle *ibkr.OrderHandle) *orderEventLog {
	return &orderEventLog{handle: handle}
}

func (l *orderEventLog) next(t *testing.T, ctx context.Context) (ibkr.OrderEvent, bool) {
	t.Helper()
	select {
	case evt, ok := <-l.handle.Events():
		if !ok {
			return ibkr.OrderEvent{}, false
		}
		l.events = append(l.events, evt)
		return evt, true
	case <-ctx.Done():
		t.Fatalf("timeout waiting for order %d event", l.handle.OrderID())
		return ibkr.OrderEvent{}, false
	}
}

func (l *orderEventLog) nextOpen(t *testing.T, ctx context.Context) ibkr.OpenOrder {
	t.Helper()
	for {
		evt, ok := l.next(t, ctx)
		if !ok {
			t.Fatalf("order %d events closed before OpenOrder", l.handle.OrderID())
		}
		if evt.OpenOrder != nil {
			return *evt.OpenOrder
		}
	}
}

func (l *orderEventLog) nextStatusAny(t *testing.T, ctx context.Context) ibkr.OrderStatusUpdate {
	t.Helper()
	for {
		evt, ok := l.next(t, ctx)
		if !ok {
			t.Fatalf("order %d events closed before a status", l.handle.OrderID())
		}
		if evt.Status != nil {
			return *evt.Status
		}
	}
}

func (l *orderEventLog) nextStatus(t *testing.T, ctx context.Context, want ibkr.OrderStatus) ibkr.OrderStatusUpdate {
	t.Helper()
	for {
		evt, ok := l.next(t, ctx)
		if !ok {
			t.Fatalf("order %d events closed before status %s", l.handle.OrderID(), want)
		}
		if evt.Status != nil && evt.Status.Status == want {
			return *evt.Status
		}
	}
}

// finish waits for the expected late execution and fee evidence, then ends
// the caller-owned observation window explicitly and records buffered events.
func (l *orderEventLog) finish(t *testing.T, ctx context.Context, wantExecutions, wantCommissions int) {
	t.Helper()
	for len(l.executions()) < wantExecutions || len(l.commissions()) < wantCommissions {
		if _, ok := l.next(t, ctx); !ok {
			t.Fatalf("order %d closed before %d executions and %d commissions", l.handle.OrderID(), wantExecutions, wantCommissions)
		}
	}
	l.handle.Close()
	for evt := range l.handle.Events() {
		l.events = append(l.events, evt)
	}
}

func (l *orderEventLog) executions() []ibkr.Execution {
	var out []ibkr.Execution
	for _, evt := range l.events {
		if evt.Execution != nil {
			out = append(out, *evt.Execution)
		}
	}
	return out
}

func (l *orderEventLog) commissions() []ibkr.CommissionAndFeesReport {
	var out []ibkr.CommissionAndFeesReport
	for _, evt := range l.events {
		if evt.CommissionAndFees != nil {
			out = append(out, *evt.CommissionAndFees)
		}
	}
	return out
}

func (l *orderEventLog) statuses() []ibkr.OrderStatus {
	var out []ibkr.OrderStatus
	for _, evt := range l.events {
		if evt.Status != nil {
			out = append(out, evt.Status.Status)
		}
	}
	return out
}

type wantExec struct {
	side   ibkr.ExecutionSide
	shares string
	price  string
}

func requireExecutions(t *testing.T, name string, got []ibkr.Execution, want []wantExec) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s executions = %d, want %d", name, len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Side != w.side ||
			!g.Shares.Equal(decimal.RequireFromString(w.shares)) ||
			!g.Price.Equal(decimal.RequireFromString(w.price)) {
			t.Fatalf("%s execution %d = %s %s @ %s, want %s %s @ %s",
				name, i, g.Side, g.Shares, g.Price, w.side, w.shares, w.price)
		}
	}
}

// requireCommissions matches commission amounts as a multiset: the Gateway
// interleaves commission reports with the next order's lifecycle, so only
// the values are stable, not their positions.
func requireCommissions(t *testing.T, name string, got []ibkr.CommissionAndFeesReport, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s commissions = %d, want %d (%v)", name, len(got), len(want), got)
	}
	remaining := make([]decimal.Decimal, len(want))
	for i, w := range want {
		remaining[i] = decimal.RequireFromString(w)
	}
	for _, g := range got {
		idx := slices.IndexFunc(remaining, g.Amount.Equal)
		if idx < 0 {
			t.Fatalf("%s unexpected commission %s, want one of %v", name, g.Amount, want)
		}
		remaining = slices.Delete(remaining, idx, idx+1)
	}
}

// TestPlaceOrderMktBuyFillReplay freezes the regular-session market buy fill
// captured live on 2026-06-11 (captures/20260611T133005Z-
// place_order_mkt_buy_aapl, events.jsonl sha256 prefix 5d88f759ad8df7ac):
// MKT BUY 1 AAPL echoes PreSubmitted, fills in one execution at 292.76, and
// the commission report carries a real negative realized PnL because the buy
// closed a prior short. The execution time uses the Gateway's UTC dash form
// and must reach the handle as a parsed timestamp.
func TestPlaceOrderMktBuyFillReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_mkt_buy_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: fillReplayAAPLBySymbol,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := handle.OrderID(); got != 368 {
		t.Fatalf("order id = %d, want 368", got)
	}

	log := newOrderEventLog(handle)
	open := log.nextOpen(t, ctx)
	if (*open.Order.PermID) != 900368 {
		t.Fatalf("perm id = %d, want 900368", (*open.Order.PermID))
	}
	// The symbol-only placement comes back resolved to the real contract.
	if open.Contract.ConID != 265598 {
		t.Fatalf("echoed con id = %d, want 265598", open.Contract.ConID)
	}
	if open.Order.OrderType != ibkr.OrderTypeMarket || !open.Order.Quantity.Equal(decimal.RequireFromString("1")) {
		t.Fatalf("open order = %s qty %s, want MKT qty 1", open.Order.OrderType, open.Order.Quantity)
	}
	if open.Order.OrderRef != "" {
		t.Fatalf("order ref = %q, want empty (placed without a ref)", open.Order.OrderRef)
	}

	log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
	filled := log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.Filled.Equal(decimal.RequireFromString("1")) ||
		!filled.Remaining.IsZero() ||
		!filled.AvgFillPrice.Equal(decimal.RequireFromString("292.76")) ||
		!filled.LastFillPrice.Equal(decimal.RequireFromString("292.76")) {
		t.Fatalf("filled status = %+v, want 1 @ 292.76", filled)
	}

	log.finish(t, ctx, 1, 1)
	execs := log.executions()
	requireExecutions(t, "mkt buy", execs, []wantExec{{"BOT", "1", "292.76"}})
	if execs[0].ExecID != "sanitized-mkt-buy-001" {
		t.Fatalf("exec id = %q", execs[0].ExecID)
	}
	if want := time.Date(2026, 6, 11, 13, 30, 10, 0, time.UTC); !execs[0].Time.Equal(want) {
		t.Fatalf("execution time = %s, want %s (parsed from the UTC dash form)", execs[0].Time, want)
	}
	comms := log.commissions()
	requireCommissions(t, "mkt buy", comms, []string{"1.000003"})
	if comms[0].Currency != "USD" || !comms[0].RealizedPnL.Equal(decimal.RequireFromString("-1.636219")) {
		t.Fatalf("commission report = %+v, want USD with realized PnL -1.636219", comms[0])
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() = %v, want nil terminal close", err)
	}
}

// TestPlaceOrderMktSellFillReplay freezes the regular-session market sell
// fill captured live on 2026-06-11 (captures/20260611T133011Z-
// place_order_mkt_sell_aapl, events.jsonl sha256 prefix 2545fcac22377485):
// MKT SELL 1 AAPL fills in one execution at 292.70 and the commission report
// carries the unset realized-PnL sentinel (the sell opened a position),
// which decodes as zero.
func TestPlaceOrderMktSellFillReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_mkt_sell_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: fillReplayAAPLBySymbol,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := handle.OrderID(); got != 369 {
		t.Fatalf("order id = %d, want 369", got)
	}

	log := newOrderEventLog(handle)
	open := log.nextOpen(t, ctx)
	if (*open.Order.PermID) != 900369 || open.Order.Action != ibkr.ActionSell {
		t.Fatalf("open order perm/action = %d/%s, want 900369/SELL", (*open.Order.PermID), open.Order.Action)
	}

	log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
	filled := log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.AvgFillPrice.Equal(decimal.RequireFromString("292.70")) {
		t.Fatalf("avg fill price = %s, want 292.70", filled.AvgFillPrice)
	}

	log.finish(t, ctx, 1, 1)
	execs := log.executions()
	requireExecutions(t, "mkt sell", execs, []wantExec{{"SLD", "1", "292.70"}})
	if execs[0].ExecID != "sanitized-mkt-sell-001" {
		t.Fatalf("exec id = %q", execs[0].ExecID)
	}
	if want := time.Date(2026, 6, 11, 13, 30, 12, 0, time.UTC); !execs[0].Time.Equal(want) {
		t.Fatalf("execution time = %s, want %s", execs[0].Time, want)
	}
	comms := log.commissions()
	requireCommissions(t, "mkt sell", comms, []string{"1.006228"})
	if comms[0].RealizedPnL != nil {
		t.Fatalf("realized PnL = %s, want nil (unset sentinel)", comms[0].RealizedPnL)
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() = %v, want nil terminal close", err)
	}
}

// TestPlaceOrderLmtBuyRestCancelReplay freezes the regular-session resting
// limit captured live on 2026-06-11 (captures/20260611T133017Z-
// place_order_lmt_buy_aapl, events.jsonl sha256 prefix 5d60edfa9536a59a)
// with the current 5-field cancel encoder: a far LMT BUY 1 @ 50 rests
// directly at Submitted (in-session limits skip PreSubmitted), and the API
// cancel yields order_status Cancelled plus the code-202 session notice.
func TestPlaceOrderLmtBuyRestCancelReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "place_order_lmt_buy_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: fillReplayAAPLBySymbol,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("50")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := handle.OrderID(); got != 370 {
		t.Fatalf("order id = %d, want 370", got)
	}

	log := newOrderEventLog(handle)
	open := log.nextOpen(t, ctx)
	if (*open.Order.PermID) != 900370 || !open.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("50")) {
		t.Fatalf("open order perm/lmt = %d/%s, want 900370/50", (*open.Order.PermID), open.Order.Prices.LmtPrice)
	}
	// During the session the limit goes straight to Submitted.
	if first := log.nextStatusAny(t, ctx); first.Status != ibkr.OrderStatusSubmitted {
		t.Fatalf("first status = %s, want Submitted", first.Status)
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
	notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
	if notice.Message != "Order Canceled - reason:" {
		t.Fatalf("202 message = %q", notice.Message)
	}

	log.finish(t, ctx, 0, 0)
	if got := log.statuses(); !slices.Equal(got, []ibkr.OrderStatus{ibkr.OrderStatusSubmitted, ibkr.OrderStatusCancelled}) {
		t.Fatalf("statuses = %v, want [Submitted Cancelled]", got)
	}
	if len(log.executions()) != 0 || len(log.commissions()) != 0 {
		t.Fatal("resting limit surfaced executions or commissions")
	}
	if err := handle.Wait(); err != nil {
		t.Fatalf("Wait() = %v, want nil terminal close", err)
	}
}

// TestAPIDelayedSuccessModifyReplay freezes the rest-modify-fill lifecycle
// captured live on 2026-06-11 (captures/20260611T133046Z-
// api_delayed_success_modify_aapl, events.jsonl sha256 prefix
// 91132d49f19f9213): a far LMT BUY 100 @ 14.61 rests at Submitted, Replace
// re-places the same order id as MKT and it fills 100 @ 292.27; the flatten
// MKT SELL fills 59 + 41 @ 292.20. The final safety global cancel draws
// code-161 session notices without replacing either Filled terminal result.
// Keeping the handles alive through the drain window also delivers the
// flatten's second commission report after the notice burst.
func TestAPIDelayedSuccessModifyReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_delayed_success_modify_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	rest, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  new(decimal.RequireFromString("14.61")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-redacted-20260611T133046Z-001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := rest.OrderID(); got != 377 {
		t.Fatalf("order id = %d, want 377", got)
	}

	restLog := newOrderEventLog(rest)
	open := restLog.nextOpen(t, ctx)
	if open.Order.OrderType != ibkr.OrderTypeLimit || !open.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("14.61")) {
		t.Fatalf("resting open order = %s @ %s, want LMT @ 14.61", open.Order.OrderType, open.Order.Prices.LmtPrice)
	}
	if (*open.Order.PermID) != 9000000377 {
		t.Fatalf("perm id = %d, want 9000000377", (*open.Order.PermID))
	}
	restLog.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)

	// Replace re-places the same order id as a market order.
	if err := rest.Replace(ctx, ibkr.Order{
		Action:    ibkr.ActionBuy,
		OrderType: ibkr.OrderTypeMarket,
		Quantity:  decimal.RequireFromString("100"),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
		OrderRef:  "ibkrgo-redacted-20260611T133046Z-001",
	}); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	open = restLog.nextOpen(t, ctx)
	if open.Order.OrderType != ibkr.OrderTypeMarket || !open.Order.Prices.LmtPrice.IsZero() {
		t.Fatalf("modified open order = %s @ %s, want MKT @ 0", open.Order.OrderType, open.Order.Prices.LmtPrice)
	}
	filled := restLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.Filled.Equal(decimal.RequireFromString("100")) ||
		!filled.AvgFillPrice.Equal(decimal.RequireFromString("292.27")) {
		t.Fatalf("modified fill = %s @ %s, want 100 @ 292.27", filled.Filled, filled.AvgFillPrice)
	}

	flatten, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: orderReplayAAPL,
		Order: ibkr.Order{
			Action:    ibkr.ActionSell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-redacted-20260611T133046Z-002",
		},
	})
	if err != nil {
		t.Fatalf("flatten Place: %v", err)
	}
	if got := flatten.OrderID(); got != 378 {
		t.Fatalf("flatten order id = %d, want 378", got)
	}
	flattenLog := newOrderEventLog(flatten)
	flattenLog.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
	partial := flattenLog.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
	if !partial.Filled.Equal(decimal.RequireFromString("59")) ||
		!partial.Remaining.Equal(decimal.RequireFromString("41")) ||
		!partial.AvgFillPrice.Equal(decimal.RequireFromString("292.20")) {
		t.Fatalf("flatten partial = %s/%s @ %s, want 59/41 @ 292.20", partial.Filled, partial.Remaining, partial.AvgFillPrice)
	}
	flattenLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)

	// Final safety global cancel: code 161 for both terminal orders.
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}

	restLog.finish(t, ctx, 1, 1)
	execs := restLog.executions()
	requireExecutions(t, "modify", execs, []wantExec{{"BOT", "100", "292.27"}})
	if execs[0].ExecID != "redacted-modify-0000001" {
		t.Fatalf("exec id = %q", execs[0].ExecID)
	}
	comms := restLog.commissions()
	requireCommissions(t, "modify", comms, []string{"1.0003"})
	if !comms[0].RealizedPnL.Equal(decimal.RequireFromString("-9.622032")) {
		t.Fatalf("realized PnL = %s, want -9.622032", comms[0].RealizedPnL)
	}
	requireCancelNotCancellableNotice(t, ctx, events, "9000000377")
	requireOrderWaitNil(t, "modify", rest)

	flattenLog.finish(t, ctx, 2, 2)
	requireExecutions(t, "flatten", flattenLog.executions(), []wantExec{
		{"SLD", "59", "292.20"},
		{"SLD", "41", "292.20"},
	})
	// The second commission trails the code-161 notice but remains inside the
	// terminal drain window, so both live reports reach the handle.
	requireCommissions(t, "flatten", flattenLog.commissions(), []string{"1.366822", "0.25491"})
	requireCancelNotCancellableNotice(t, ctx, events, "9000000378")
	requireOrderWaitNil(t, "flatten", flatten)
}

// TestAPIOrderFillCampaignReplay freezes the regular-session fill campaign
// captured live on 2026-06-11 (captures/20260611T133024Z-api_order_fill_aapl,
// events.jsonl sha256 prefix 4fa597ba1c4bc369): six lifecycles with real
// fills on one connection (MKT buy/sell round trips, an MTL fill, and a
// rest-modify-fill), then a one-shot executions query retaining 26 campaign
// updates (13 executions plus 13 commission deliveries), and a final safety
// global cancel whose code-161 replies remain session notices. The query's
// re-emitted
// executions dual-dispatch to the still-open order routes but are deduped by
// ExecID on the order-handle leg, so each handle sees every fill exactly once
// while the query's own snapshot result still carries every row.
func TestAPIOrderFillCampaignReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_fill_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ref := func(n int) string {
		return fmt.Sprintf("ibkrgo-redacted-20260611T133024Z-%03d", n)
	}
	place := func(action ibkr.OrderAction, orderType ibkr.OrderType, lmt string, refNum int, wantID int64) (*ibkr.OrderHandle, *orderEventLog) {
		t.Helper()
		order := ibkr.Order{
			Action:    action,
			OrderType: orderType,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  ref(refNum),
		}
		if lmt != "" {
			order.LmtPrice = new(decimal.RequireFromString(lmt))
		}
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: order})
		if err != nil {
			t.Fatalf("Place %d: %v", wantID, err)
		}
		if got := handle.OrderID(); got != wantID {
			t.Fatalf("order id = %d, want %d", got, wantID)
		}
		return handle, newOrderEventLog(handle)
	}

	// 371 MKT BUY 100: PreSubmitted, then a single 100-share fill at 292.79.
	mktBuy, mktBuyLog := place(ibkr.ActionBuy, ibkr.OrderTypeMarket, "", 1, 371)
	open := mktBuyLog.nextOpen(t, ctx)
	if (*open.Order.PermID) != 9000000371 || open.Order.OrderRef != ref(1) {
		t.Fatalf("371 open perm/ref = %d/%q", (*open.Order.PermID), open.Order.OrderRef)
	}
	if open.Order.IncludeOvernight == nil || *open.Order.IncludeOvernight {
		t.Fatalf("371 open include overnight = %v, want explicit false", open.Order.IncludeOvernight)
	}
	if first := mktBuyLog.nextStatusAny(t, ctx); first.Status != ibkr.OrderStatusPreSubmitted {
		t.Fatalf("371 first status = %s, want PreSubmitted", first.Status)
	}
	filled := mktBuyLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.AvgFillPrice.Equal(decimal.RequireFromString("292.79")) {
		t.Fatalf("371 avg fill = %s, want 292.79", filled.AvgFillPrice)
	}

	// 372 MKT SELL 100: three executions; the partial statuses carry the
	// running average (40/60 @ 292.33, 80/20 @ 292.315, Filled @ 292.312).
	mktSell, mktSellLog := place(ibkr.ActionSell, ibkr.OrderTypeMarket, "", 2, 372)
	mktSellLog.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
	partial := mktSellLog.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
	if !partial.Filled.Equal(decimal.RequireFromString("40")) ||
		!partial.AvgFillPrice.Equal(decimal.RequireFromString("292.33")) {
		t.Fatalf("372 first partial = %s @ %s, want 40 @ 292.33", partial.Filled, partial.AvgFillPrice)
	}
	filled = mktSellLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.AvgFillPrice.Equal(decimal.RequireFromString("292.312")) ||
		!filled.LastFillPrice.Equal(decimal.RequireFromString("292.30")) {
		t.Fatalf("372 filled = avg %s last %s, want 292.312/292.30", filled.AvgFillPrice, filled.LastFillPrice)
	}

	// 373 MTL BUY 100: PreSubmitted -> Submitted -> two fills at 292.44.
	mtl, mtlLog := place(ibkr.ActionBuy, ibkr.OrderTypeMarketToLimit, "", 3, 373)
	mtlLog.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
	mtlLog.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
	filled = mtlLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.AvgFillPrice.Equal(decimal.RequireFromString("292.44")) {
		t.Fatalf("373 avg fill = %s, want 292.44", filled.AvgFillPrice)
	}

	// 374 MKT SELL 100: three executions, avg 292.342.
	flatten374, flatten374Log := place(ibkr.ActionSell, ibkr.OrderTypeMarket, "", 4, 374)
	flatten374Log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
	filled = flatten374Log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.AvgFillPrice.Equal(decimal.RequireFromString("292.342")) {
		t.Fatalf("374 avg fill = %s, want 292.342", filled.AvgFillPrice)
	}

	// 375 LMT BUY 100 @ 14.61 rests, then Replace re-places it as MKT.
	modify, modifyLog := place(ibkr.ActionBuy, ibkr.OrderTypeLimit, "14.61", 5, 375)
	open = modifyLog.nextOpen(t, ctx)
	if open.Order.OrderType != ibkr.OrderTypeLimit || !open.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("14.61")) {
		t.Fatalf("375 resting open = %s @ %s, want LMT @ 14.61", open.Order.OrderType, open.Order.Prices.LmtPrice)
	}
	modifyLog.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
	if err := modify.Replace(ctx, ibkr.Order{
		Action:    ibkr.ActionBuy,
		OrderType: ibkr.OrderTypeMarket,
		Quantity:  decimal.RequireFromString("100"),
		TIF:       ibkr.TIFDay,
		Account:   "DU9000001",
		OrderRef:  ref(5),
	}); err != nil {
		t.Fatalf("375 Replace: %v", err)
	}
	open = modifyLog.nextOpen(t, ctx)
	if open.Order.OrderType != ibkr.OrderTypeMarket {
		t.Fatalf("375 modified open = %s, want MKT", open.Order.OrderType)
	}
	filled = modifyLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.AvgFillPrice.Equal(decimal.RequireFromString("292.31")) {
		t.Fatalf("375 avg fill = %s, want 292.31", filled.AvgFillPrice)
	}

	// 376 MKT SELL 100: three executions at 292.20.
	flatten376, flatten376Log := place(ibkr.ActionSell, ibkr.OrderTypeMarket, "", 6, 376)
	flatten376Log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
	filled = flatten376Log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
	if !filled.AvgFillPrice.Equal(decimal.RequireFromString("292.20")) {
		t.Fatalf("376 avg fill = %s, want 292.20", filled.AvgFillPrice)
	}

	queryExecs, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(queryExecs.Executions) != 13 {
		t.Fatalf("executions query rows = %d, want 13", len(queryExecs.Executions))
	}
	wantRows := []struct {
		orderID int64
		exec    wantExec
		execID  string
	}{
		{371, wantExec{"BOT", "100", "292.79"}, "redacted-fill-000000001"},
		{372, wantExec{"SLD", "40", "292.33"}, "redacted-fill-000000002"},
		{372, wantExec{"SLD", "40", "292.30"}, "redacted-fill-000000003"},
		{372, wantExec{"SLD", "20", "292.30"}, "redacted-fill-000000004"},
		{373, wantExec{"BOT", "40", "292.44"}, "redacted-fill-000000005"},
		{373, wantExec{"BOT", "60", "292.44"}, "redacted-fill-000000006"},
		{374, wantExec{"SLD", "40", "292.36"}, "redacted-fill-000000007"},
		{374, wantExec{"SLD", "40", "292.33"}, "redacted-fill-000000008"},
		{374, wantExec{"SLD", "20", "292.33"}, "redacted-fill-000000009"},
		{375, wantExec{"BOT", "100", "292.31"}, "redacted-fill-000000010"},
		{376, wantExec{"SLD", "40", "292.20"}, "redacted-fill-000000011"},
		{376, wantExec{"SLD", "40", "292.20"}, "redacted-fill-000000012"},
		{376, wantExec{"SLD", "20", "292.20"}, "redacted-fill-000000013"},
	}
	if len(queryExecs.Executions) != len(wantRows) {
		t.Fatalf("executions query rows = %d, want %d", len(queryExecs.Executions), len(wantRows))
	}
	for i, row := range wantRows {
		got := queryExecs.Executions[i]
		if got.OrderID != row.orderID || got.ExecID != row.execID {
			t.Fatalf("query row %d = order %d exec %q, want order %d exec %q", i, got.OrderID, got.ExecID, row.orderID, row.execID)
		}
	}
	requireExecutions(t, "query rows", queryExecs.Executions, func() []wantExec {
		out := make([]wantExec, len(wantRows))
		for i, row := range wantRows {
			out[i] = row.exec
		}
		return out
	}())
	// The query response carries the Gateway's "US/Eastern" time form, which
	// must parse to the same instant as the streaming UTC dash form.
	if want := time.Date(2026, 6, 11, 13, 30, 30, 0, time.UTC); !queryExecs.Executions[0].Time.Equal(want) {
		t.Fatalf("query row 0 time = %s, want %s (parsed from US/Eastern form)", queryExecs.Executions[0].Time, want)
	}

	// Final safety global cancel: real code-161 replies remain session notices;
	// the Filled statuses continue to own the clean handle closes.
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}

	// Per-handle totals. Both executions and commissions are deduped by ExecID
	// on the order-handle leg: the one-shot query re-emits the same fills and
	// commissions the handle already saw live, so each lands on the handle
	// exactly once (the query's own snapshot result, asserted above, still
	// carries its rows). Every count and value below is fixed by the capture.
	finals := []struct {
		name      string
		handle    *ibkr.OrderHandle
		log       *orderEventLog
		wantExecs []wantExec
		wantComms []string
	}{
		{"371", mktBuy, mktBuyLog,
			[]wantExec{{"BOT", "100", "292.79"}},
			[]string{"1.0003"}},
		{"372", mktSell, mktSellLog,
			[]wantExec{{"SLD", "40", "292.33"}, {"SLD", "40", "292.30"}, {"SLD", "20", "292.30"}},
			[]string{"1.2488", "0.248775", "0.124388"}},
		{"373", mtl, mtlLog,
			[]wantExec{{"BOT", "40", "292.44"}, {"BOT", "60", "292.44"}},
			[]string{"1.00012", "1.8E-4"}},
		{"374", flatten374, flatten374Log,
			[]wantExec{{"SLD", "40", "292.36"}, {"SLD", "40", "292.33"}, {"SLD", "20", "292.33"}},
			[]string{"1.248825", "0.2488", "0.1244"}},
		{"375", modify, modifyLog,
			[]wantExec{{"BOT", "100", "292.31"}},
			[]string{"1.0003"}},
		// 376's three fills each stream one commission live; the query re-emits
		// 011/012's commissions (013's arrives after the query end) but the
		// order-handle leg deduplicates them, so the handle tallies 3 execs / 3
		// commissions.
		{"376", flatten376, flatten376Log,
			[]wantExec{{"SLD", "40", "292.20"}, {"SLD", "40", "292.20"}, {"SLD", "20", "292.20"}},
			[]string{"1.248693", "0.248693", "0.124346"}},
	}
	for _, f := range finals {
		f.log.finish(t, ctx, len(f.wantExecs), len(f.wantComms))
		requireExecutions(t, f.name, f.log.executions(), f.wantExecs)
		requireCommissions(t, f.name, f.log.commissions(), f.wantComms)
		requireOrderWaitNil(t, f.name, f.handle)
	}
}

// TestAPIOrderTypeMatrixReplay freezes the order-type breadth matrix captured
// live on 2026-06-11 (captures/20260611T133103Z-api_order_type_matrix_aapl,
// events.jsonl sha256 prefix 4e0eb7d61bf5a743). Each subtest drives one
// type's lifecycle through PEG PRI exactly as the capture's client frames
// show and asserts its real outcome: fill (with executions and commissions),
// rest+cancel, Gateway-side price-band cancel, or outright rejection
// (terminal handle error). The capture's malformed PEG MID/BEST tail remains
// provenance-only. The terminal-outcome sweep drains every executable handle
// and pins its full execution/commission tally and close error.
func TestAPIOrderTypeMatrixReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_type_matrix_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	events := client.SessionEvents()

	ref := func(n int) string {
		return fmt.Sprintf("ibkrgo-redacted-20260611T133104Z-%03d", n)
	}
	place := func(t *testing.T, order ibkr.Order, refNum int, wantID int64) (*ibkr.OrderHandle, *orderEventLog) {
		t.Helper()
		order.Quantity = decimal.RequireFromString("100")
		order.TIF = ibkr.TIFDay
		order.Account = "DU9000001"
		order.OrderRef = ref(refNum)
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: order})
		if err != nil {
			t.Fatalf("Place %d: %v", wantID, err)
		}
		if got := handle.OrderID(); got != wantID {
			t.Fatalf("order id = %d, want %d", got, wantID)
		}
		return handle, newOrderEventLog(handle)
	}
	cancelOrder := func(t *testing.T, handle *ibkr.OrderHandle) {
		t.Helper()
		if err := handle.Cancel(ctx); err != nil {
			t.Fatalf("Cancel %d: %v", handle.OrderID(), err)
		}
	}
	requireCleanCancelNotice := func(t *testing.T) {
		t.Helper()
		notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
		if notice.Message != "Order Canceled - reason:" {
			t.Fatalf("202 message = %q", notice.Message)
		}
	}
	requireFilledRaceNotice := func(t *testing.T, orderID int64) {
		t.Helper()
		raced := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCannotBeCancelled)
		want := fmt.Sprintf("OrderId %d that needs to be cancelled cannot be cancelled, state: Filled.", orderID)
		if raced.Message != want {
			t.Fatalf("10148 message = %q, want %q", raced.Message, want)
		}
	}

	// final terminal expectations, appended as the subtests run.
	type terminalCase struct {
		name      string
		handle    *ibkr.OrderHandle
		log       *orderEventLog
		wantExecs []wantExec
		wantComms []string
		errCode   int
		errFrag   string
	}
	var terminals []terminalCase
	register := func(name string, handle *ibkr.OrderHandle, log *orderEventLog, execs []wantExec, comms []string, errCode int, errFrag string) {
		terminals = append(terminals, terminalCase{name, handle, log, execs, comms, errCode, errFrag})
	}

	t.Run("mkt_buy_fill", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket}, 1, 379)
		// In-session market orders go straight to Submitted.
		if first := log.nextStatusAny(t, ctx); first.Status != ibkr.OrderStatusSubmitted {
			t.Fatalf("first status = %s, want Submitted", first.Status)
		}
		filled := log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		if !filled.AvgFillPrice.Equal(decimal.RequireFromString("292.194")) ||
			!filled.LastFillPrice.Equal(decimal.RequireFromString("292.21")) {
			t.Fatalf("filled = avg %s last %s, want 292.194/292.21", filled.AvgFillPrice, filled.LastFillPrice)
		}
		register("379", handle, log,
			[]wantExec{{"BOT", "40", "292.18"}, {"BOT", "40", "292.20"}, {"BOT", "20", "292.21"}},
			[]string{"1.00012", "1.2E-4", "6.0E-5"}, 0, "")

		flatten, flattenLog := place(t, ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeMarket}, 23, 380)
		flattenLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		register("380", flatten, flattenLog,
			[]wantExec{{"SLD", "40", "292.17"}, {"SLD", "60", "292.17"}},
			[]string{"1.248668", "0.373002"}, 0, "")
	})

	t.Run("marketable_lmt_buy_price_band_cancel", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, LmtPrice: new(decimal.RequireFromString("350.62"))}, 2, 381)
		// The Gateway cancels the order itself; the client never sends a cancel.
		if first := log.nextStatusAny(t, ctx); first.Status != ibkr.OrderStatusPendingCancel {
			t.Fatalf("first status = %s, want PendingCancel", first.Status)
		}
		log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
		notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
		if !strings.Contains(notice.Message, "We cannot accept an order at a limit price at or more aggressive than 301.449496") {
			t.Fatalf("price-band 202 message = %q", notice.Message)
		}
		register("381", handle, log, nil, nil, 0, "")
	})

	t.Run("far_lmt_buy_rest_cancel", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, LmtPrice: new(decimal.RequireFromString("14.61"))}, 3, 382)
		log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
		cancelOrder(t, handle)
		log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
		requireCleanCancelNotice(t)
		register("382", handle, log, nil, nil, 0, "")
	})

	t.Run("stp_buy_rest_cancel", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeStop, AuxPrice: new(decimal.RequireFromString("350.62"))}, 4, 383)
		open := log.nextOpen(t, ctx)
		// The Gateway computes a limit next to the stop on the echo.
		if !open.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("350.65")) ||
			!open.Order.Prices.AuxPrice.Equal(decimal.RequireFromString("350.62")) {
			t.Fatalf("stp echo = lmt %s aux %s, want 350.65/350.62", open.Order.Prices.LmtPrice, open.Order.Prices.AuxPrice)
		}
		held := log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "trigger" {
			t.Fatalf("why held = %q, want trigger", held.WhyHeld)
		}
		cancelOrder(t, handle)
		log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
		requireCleanCancelNotice(t)
		register("383", handle, log, nil, nil, 0, "")
	})

	t.Run("stp_lmt_buy_rest_cancel", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeStopLimit, LmtPrice: new(decimal.RequireFromString("351.62")), AuxPrice: new(decimal.RequireFromString("350.62"))}, 5, 384)
		held := log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "trigger" {
			t.Fatalf("why held = %q, want trigger", held.WhyHeld)
		}
		cancelOrder(t, handle)
		log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
		requireCleanCancelNotice(t)
		register("384", handle, log, nil, nil, 0, "")
	})

	t.Run("trail_sell_fill", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeTrailingStop, AuxPrice: new(decimal.RequireFromString("1")), TrailStopPrice: new(decimal.RequireFromString("2921.8"))}, 6, 385)
		open := log.nextOpen(t, ctx)
		// First echo carries the Gateway-computed trigger limit, float noise
		// included.
		if !open.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("2921.8300000000004")) {
			t.Fatalf("trail echoed lmt = %s, want 2921.8300000000004", open.Order.Prices.LmtPrice)
		}
		held := log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "trigger" {
			t.Fatalf("why held = %q, want trigger", held.WhyHeld)
		}
		filled := log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		if !filled.AvgFillPrice.Equal(decimal.RequireFromString("291.82")) {
			t.Fatalf("trail avg fill = %s, want 291.82", filled.AvgFillPrice)
		}
		// The cancel raced the fill; the 10148 arrives after the flatten is
		// placed (capture order).
		cancelOrder(t, handle)
		register("385", handle, log,
			[]wantExec{{"SLD", "100", "291.82"}},
			[]string{"1.620949"}, 0, "")

		flatten, flattenLog := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket}, 24, 386)
		requireFilledRaceNotice(t, 385)
		flattenLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		register("386", flatten, flattenLog,
			[]wantExec{{"BOT", "40", "291.97"}, {"BOT", "40", "291.97"}, {"BOT", "20", "292.02"}},
			[]string{"1.00012", "1.2E-4", "6.0E-5"}, 0, "")
	})

	t.Run("trail_limit_sell_rest_cancel", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeTrailingLimit, AuxPrice: new(decimal.RequireFromString("1")), TrailStopPrice: new(decimal.RequireFromString("2921.8")), LmtPriceOffset: new(decimal.RequireFromString("0.05"))}, 7, 387)
		open := log.nextOpen(t, ctx)
		if !open.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("2921.75")) {
			t.Fatalf("trail-limit echoed lmt = %s, want 2921.75", open.Order.Prices.LmtPrice)
		}
		held := log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "trigger" {
			t.Fatalf("why held = %q, want trigger", held.WhyHeld)
		}
		log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
		cancelOrder(t, handle)
		log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
		requireCleanCancelNotice(t)
		register("387", handle, log, nil, nil, 0, "")
	})

	t.Run("mit_buy_fill", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarketIfTouched, AuxPrice: new(decimal.RequireFromString("350.62"))}, 8, 388)
		// MIT holds at PreSubmitted without the trigger why_held the stops carry.
		held := log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		if held.WhyHeld != "" {
			t.Fatalf("mit why held = %q, want empty", held.WhyHeld)
		}
		log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
		filled := log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		if !filled.AvgFillPrice.Equal(decimal.RequireFromString("291.89")) {
			t.Fatalf("mit avg fill = %s, want 291.89", filled.AvgFillPrice)
		}
		cancelOrder(t, handle)
		register("388", handle, log,
			[]wantExec{{"BOT", "100", "291.89"}},
			[]string{"1.0003"}, 0, "")

		flatten, flattenLog := place(t, ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeMarket}, 25, 389)
		requireFilledRaceNotice(t, 388)
		flattenLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		register("389", flatten, flattenLog,
			[]wantExec{{"SLD", "100", "291.75"}},
			[]string{"1.620805"}, 0, "")
	})

	t.Run("lit_buy_fill", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimitIfTouched, LmtPrice: new(decimal.RequireFromString("351.62")), AuxPrice: new(decimal.RequireFromString("350.62"))}, 9, 390)
		log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
		filled := log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		if !filled.AvgFillPrice.Equal(decimal.RequireFromString("291.698")) {
			t.Fatalf("lit avg fill = %s, want 291.698", filled.AvgFillPrice)
		}
		cancelOrder(t, handle)
		requireFilledRaceNotice(t, 390)
		register("390", handle, log,
			[]wantExec{{"BOT", "40", "291.67"}, {"BOT", "40", "291.71"}, {"BOT", "20", "291.73"}},
			[]string{"1.00012", "1.2E-4", "6.0E-5"}, 0, "")

		// The flatten's first fill arrives while the order is still
		// PreSubmitted.
		flatten, flattenLog := place(t, ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeMarket}, 26, 391)
		first := flattenLog.nextStatusAny(t, ctx)
		if first.Status != ibkr.OrderStatusPreSubmitted {
			t.Fatalf("flatten first status = %s, want PreSubmitted", first.Status)
		}
		partial := flattenLog.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		if !partial.Filled.Equal(decimal.RequireFromString("40")) {
			t.Fatalf("flatten PreSubmitted partial = %s, want 40", partial.Filled)
		}
		flattenLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		register("391", flatten, flattenLog,
			[]wantExec{{"SLD", "40", "291.67"}, {"SLD", "40", "291.67"}, {"SLD", "20", "291.67"}},
			[]string{"1.248256", "0.248256", "0.124128"}, 0, "")
	})

	t.Run("mtl_buy_fill", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarketToLimit}, 10, 392)
		log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
		filled := log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		if !filled.AvgFillPrice.Equal(decimal.RequireFromString("291.75")) {
			t.Fatalf("mtl avg fill = %s, want 291.75", filled.AvgFillPrice)
		}
		cancelOrder(t, handle)
		register("392", handle, log,
			[]wantExec{{"BOT", "40", "291.75"}, {"BOT", "60", "291.75"}},
			[]string{"1.00012", "1.8E-4"}, 0, "")

		flatten, flattenLog := place(t, ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeMarket}, 27, 393)
		requireFilledRaceNotice(t, 392)
		flattenLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		register("393", flatten, flattenLog,
			[]wantExec{{"SLD", "40", "291.70"}, {"SLD", "60", "291.69"}},
			[]string{"1.248281", "0.372409"}, 0, "")
	})

	t.Run("rel_buy_rest_cancel", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeRelative, LmtPrice: new(decimal.RequireFromString("14.61"))}, 11, 394)
		open := log.nextOpen(t, ctx)
		// The client sent no offset; the Gateway assigned 0.01.
		if !open.Order.Prices.AuxPrice.Equal(decimal.RequireFromString("0.01")) {
			t.Fatalf("rel gateway-assigned offset = %s, want 0.01", open.Order.Prices.AuxPrice)
		}
		log.nextStatus(t, ctx, ibkr.OrderStatusPreSubmitted)
		log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
		cancelOrder(t, handle)
		log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
		requireCleanCancelNotice(t)
		register("394", handle, log, nil, nil, 0, "")
	})

	t.Run("delayed_success_modify", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, LmtPrice: new(decimal.RequireFromString("14.61"))}, 12, 395)
		log.nextStatus(t, ctx, ibkr.OrderStatusSubmitted)
		if err := handle.Replace(ctx, ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.RequireFromString("100"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  ref(12),
		}); err != nil {
			t.Fatalf("Replace: %v", err)
		}
		open := log.nextOpen(t, ctx)
		if open.Order.OrderType != ibkr.OrderTypeMarket {
			t.Fatalf("modified open = %s, want MKT", open.Order.OrderType)
		}
		filled := log.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		if !filled.AvgFillPrice.Equal(decimal.RequireFromString("291.38")) {
			t.Fatalf("modified avg fill = %s, want 291.38", filled.AvgFillPrice)
		}
		register("395", handle, log,
			[]wantExec{{"BOT", "40", "291.32"}, {"BOT", "60", "291.42"}},
			[]string{"1.00012", "1.8E-4"}, 0, "")

		flatten, flattenLog := place(t, ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeMarket}, 28, 396)
		flattenLog.nextStatus(t, ctx, ibkr.OrderStatusFilled)
		register("396", flatten, flattenLog,
			[]wantExec{{"SLD", "40", "291.32"}, {"SLD", "40", "291.28"}, {"SLD", "20", "291.28"}},
			[]string{"1.247968", "0.247935", "0.123967"}, 0, "")
	})

	t.Run("invalid_order_type_reject", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderType("FEELINGS"), LmtPrice: new(decimal.RequireFromString("14.61"))}, 13, 397)
		register("397", handle, log, nil, nil,
			ibkr.ErrCodeServerErrorValidatingRequest, "Invalid order type was entered")
	})

	t.Run("moc_buy_silent_accept_cancel", func(t *testing.T) {
		// The Gateway accepts the MOC with no echo at all; the lifecycle only
		// surfaces once the cancel lands.
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarketOnClose}, 14, 398)
		cancelOrder(t, handle)
		log.nextStatus(t, ctx, ibkr.OrderStatusPendingCancel)
		log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
		requireCleanCancelNotice(t)
		register("398", handle, log, nil, nil, 0, "")
	})

	t.Run("loc_buy_silent_accept_cancel", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimitOnClose, LmtPrice: new(decimal.RequireFromString("350.62"))}, 15, 399)
		cancelOrder(t, handle)
		log.nextStatus(t, ctx, ibkr.OrderStatusPendingCancel)
		log.nextStatus(t, ctx, ibkr.OrderStatusCancelled)
		requireCleanCancelNotice(t)
		register("399", handle, log, nil, nil, 0, "")
	})

	t.Run("moo_buy_reject", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarketOnOpen}, 16, 400)
		register("400", handle, log, nil, nil,
			ibkr.ErrCodeServerErrorValidatingRequest, "Invalid order type was entered")
	})

	t.Run("loo_buy_reject", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimitOnOpen, LmtPrice: new(decimal.RequireFromString("350.62"))}, 17, 401)
		register("401", handle, log, nil, nil,
			ibkr.ErrCodeServerErrorValidatingRequest, "Invalid order type was entered")
	})

	t.Run("peg_mkt_reject", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypePeggedToMarket, LmtPrice: new(decimal.RequireFromString("14.61"))}, 18, 402)
		register("402", handle, log, nil, nil,
			ibkr.ErrCodeUnsupportedOrderType, "Unsupported order type for this exchange and security type.")
	})

	t.Run("peg_bench_missing_reference_rejected_locally", func(t *testing.T) {
		order := ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypePeggedBenchmark,
			Quantity:  decimal.RequireFromString("100"),
			LmtPrice:  new(decimal.RequireFromString("14.61")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  ref(22),
		}
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: order})
		if handle != nil {
			t.Fatalf("Place() handle = %v, want nil", handle)
		}
		validation, ok := errors.AsType[*ibkr.ValidationError](err)
		if !ok || validation.Field != "Order.PeggedBenchmark" {
			t.Fatalf("Place() error = %v, want Order.PeggedBenchmark ValidationError", err)
		}
	})

	t.Run("peg_pri_reject", func(t *testing.T) {
		handle, log := place(t, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypePeggedToPrimary, LmtPrice: new(decimal.RequireFromString("14.61"))}, 19, 403)
		register("403", handle, log, nil, nil,
			ibkr.ErrCodeServerErrorValidatingRequest, "Invalid order type was entered")
	})

	t.Run("terminal_outcomes", func(t *testing.T) {
		for _, tc := range terminals {
			if tc.errCode != 0 {
				requireOrderAPIError(t, tc.name, tc.handle, tc.errCode, tc.errFrag)
			}
			tc.log.finish(t, ctx, len(tc.wantExecs), len(tc.wantComms))
			requireExecutions(t, tc.name, tc.log.executions(), tc.wantExecs)
			requireCommissions(t, tc.name, tc.log.commissions(), tc.wantComms)
			if tc.errCode != 0 {
				continue
			}
			if err := tc.handle.Wait(); err != nil {
				t.Fatalf("%s Wait() = %v, want nil terminal close", tc.name, err)
			}
		}
	})
}
