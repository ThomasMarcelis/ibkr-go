package ibkr_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

// The two replays below freeze the option-exercise family (matrix row
// OPT-002) captured live on 2026-06-11 against paper Gateway server_version
// 200. Exercise (msg 21) is fire-and-forget on the wire: the engine allocates
// a request id, sends the frame with the sv200 manual-order-time/customer-
// account/professional-customer tail, and registers a keyed route so the
// request-id-targeted replies are not lost. How the Gateway's answers surface:
//
//   - api_error replies on the exercise request id (322 exercise refusals,
//     the 10349 TIF-preset acknowledgement, and the 202 that cancels a
//     working instruction) route to the exercise route and surface as
//     session events;
//   - the open_order/order_status frames for the pseudo-order the Gateway
//     materializes under the exercise request id are keyed by order id, not
//     request id, so they match no order route and are dropped.
//
// Each replay asserts that surface exactly; the host completing (waitHost)
// proves the dropped pseudo-order frames were really delivered and absorbed.

var exerciseAAPLJun12Call2925 = ibkr.Contract{
	ConID:        886441502,
	Symbol:       "AAPL",
	SecType:      ibkr.SecTypeOption,
	Expiry:       "20260612 16:00:00 US/Eastern",
	Strike:       decimal.RequireFromString("292.5"),
	Right:        ibkr.RightCall,
	Multiplier:   "100",
	Exchange:     "SMART",
	Currency:     "USD",
	LocalSymbol:  "AAPL  260612C00292500",
	TradingClass: "AAPL",
}

var exerciseAAPLJun12Call2825 = ibkr.Contract{
	ConID:        887760542,
	Symbol:       "AAPL",
	SecType:      ibkr.SecTypeOption,
	Expiry:       "20260612 16:00:00 US/Eastern",
	Strike:       decimal.RequireFromString("282.5"),
	Right:        ibkr.RightCall,
	Multiplier:   "100",
	Exchange:     "SMART",
	Currency:     "USD",
	LocalSymbol:  "AAPL  260612C00282500",
	TradingClass: "AAPL",
}

// requireNoMoreOrderEvents drains the handle's events channel to its close,
// failing on any further business event. The bound comes from ctx so a
// desynced replay fails fast instead of hanging.
func requireNoMoreOrderEvents(t *testing.T, ctx context.Context, name string, handle *ibkr.OrderHandle) {
	t.Helper()

	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return
			}
			t.Errorf("%s saw unexpected order event: %+v", name, evt)
		case <-ctx.Done():
			t.Fatalf("timeout draining %s order events", name)
		}
	}
}

// waitOptionFill drains handle events until the Filled status, the
// execution, and the commission report have all arrived, returning them for
// field-level assertions.
func waitOptionFill(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) (ibkr.Execution, ibkr.CommissionAndFeesReport, ibkr.OrderStatusUpdate) {
	t.Helper()

	var exec *ibkr.Execution
	var comm *ibkr.CommissionAndFeesReport
	var filled *ibkr.OrderStatusUpdate
	for exec == nil || comm == nil || filled == nil {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				t.Fatal("order events closed before the fill completed")
			}
			if evt.Execution != nil {
				exec = evt.Execution
			}
			if evt.CommissionAndFees != nil {
				comm = evt.CommissionAndFees
			}
			if evt.Status != nil && evt.Status.Status == ibkr.OrderStatusFilled {
				filled = evt.Status
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for the option fill")
		}
	}
	return *exec, *comm, *filled
}

// TestAPIOptionExerciseNotITMReplay freezes the not-in-the-money exercise
// refusal captured live on 2026-06-11 (captures/20260611T133444Z-
// api_option_exercise_aapl, events.jsonl sha256 prefix a5ce9af5fee56269):
// MKT BUY 1 of the barely-ITM AAPL Jun-12 292.5 call fills at 2.11 with one
// execution and its commission report, then Exercise draws code 322
// "Exercise ignored because option is not in-the-money." on the exercise
// request id. The exercise route surfaces that refusal as a session event;
// the Exercise call itself returns nil and nothing touches the order handle.
func TestAPIOptionExerciseNotITMReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_option_exercise_not_itm_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: exerciseAAPLJun12Call2925,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.NewFromInt(1),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260611T133444Z-001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := handle.OrderID(); got != 406 {
		t.Fatalf("order id = %d, want 406", got)
	}

	open := waitForOpenOrder(t, ctx, handle)
	if open.Contract.ConID != 886441502 || open.Contract.SecType != ibkr.SecTypeOption {
		t.Fatalf("open contract = %+v, want AAPL OPT con 886441502", open.Contract)
	}
	if !open.Contract.Strike.Equal(decimal.RequireFromString("292.5")) || open.Contract.Right != ibkr.RightCall {
		t.Fatalf("open strike/right = %s/%s, want 292.5/C", open.Contract.Strike, open.Contract.Right)
	}
	if open.OrderType != ibkr.OrderTypeMarket {
		t.Fatalf("open order type = %s, want MKT", open.OrderType)
	}
	if open.PermID != 900406 {
		t.Fatalf("perm id = %d, want 900406", open.PermID)
	}

	exec, comm, filled := waitOptionFill(t, ctx, handle)
	if exec.ExecID != "sanitized-exercise-fill-001" || exec.Side != "BOT" {
		t.Fatalf("execution = %+v, want BOT sanitized-exercise-fill-001", exec)
	}
	if !exec.Shares.Equal(decimal.NewFromInt(1)) || !exec.Price.Equal(decimal.RequireFromString("2.11")) {
		t.Fatalf("execution shares/price = %s/%s, want 1/2.11", exec.Shares, exec.Price)
	}
	if comm.ExecID != exec.ExecID || !comm.Amount.Equal(decimal.RequireFromString("0.76825")) || comm.Currency != "USD" {
		t.Fatalf("commission = %+v, want 0.76825 USD on %s", comm, exec.ExecID)
	}
	if !filled.Filled.Equal(decimal.NewFromInt(1)) || !filled.AvgFillPrice.Equal(decimal.RequireFromString("2.11")) {
		t.Fatalf("filled status = %+v, want 1 @ 2.11", filled)
	}

	// Exercise the barely-ITM call. Fire-and-forget: the send succeeds even
	// though the Gateway will refuse the exercise.
	if err := client.Options().Exercise(ctx, ibkr.ExerciseOptionsRequest{
		Contract:         exerciseAAPLJun12Call2925,
		ExerciseAction:   ibkr.Exercise,
		ExerciseQuantity: 1,
		Account:          "DU9000001",
	}); err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	// The code-322 refusal targets the exercise request id, which now carries
	// a keyed route: it surfaces as a session event instead of being dropped.
	refusal := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeServerErrorProcessingRequest)
	if refusal.Message != "Error processing request.Exercise ignored because option is not in-the-money." {
		t.Fatalf("322 message = %q", refusal.Message)
	}

	// The handle saw the terminal Filled status; no exercise reply touches it,
	// so it closes clean (nil) on the transcript disconnect with no further
	// business events.
	requireNoMoreOrderEvents(t, ctx, "exercise buy", handle)
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() = %v, want nil (terminal Filled before disconnect)", err)
	}
}

// TestAPIOptionExerciseServerRejectReplay freezes the deep-ITM exercise
// server-rejection lifecycle captured live on 2026-06-11
// (captures/20260611T133636Z-api_option_exercise_aapl, events.jsonl sha256
// prefix 267e7806669f2d5c): MKT BUY 1 of the AAPL Jun-12 282.5 call fills
// at 8.90, Exercise is acknowledged as a working DAY instruction via code
// 10349 on the exercise request id (a session event via the exercise route),
// and the Gateway materializes the instruction as a pseudo-order keyed by
// that same id, which this client has no order route for and drops. When the
// teardown global cancel runs, the paper Gateway kills the instruction: code
// 322 "Exercise/Lapse failed due to server rejection." and its code-202
// notice both target the exercise request id and surface as session events,
// the pseudo-order's Cancelled status between them is keyed by order id and
// drops, and the code-161 reply for the filled buy order routes to its handle
// and becomes the terminal error.
func TestAPIOptionExerciseServerRejectReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_option_exercise_server_reject_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	events := client.SessionEvents()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: exerciseAAPLJun12Call2825,
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  decimal.NewFromInt(1),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "ibkrgo-sanitized-20260611T133636Z-001",
		},
	})
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if got := handle.OrderID(); got != 407 {
		t.Fatalf("order id = %d, want 407", got)
	}

	open := waitForOpenOrder(t, ctx, handle)
	if open.Contract.ConID != 887760542 || !open.Contract.Strike.Equal(decimal.RequireFromString("282.5")) {
		t.Fatalf("open contract = %+v, want AAPL OPT 282.5 con 887760542", open.Contract)
	}
	if open.PermID != 900407 {
		t.Fatalf("perm id = %d, want 900407", open.PermID)
	}

	exec, comm, filled := waitOptionFill(t, ctx, handle)
	if !exec.Shares.Equal(decimal.NewFromInt(1)) || !exec.Price.Equal(decimal.RequireFromString("8.9")) {
		t.Fatalf("execution shares/price = %s/%s, want 1/8.90", exec.Shares, exec.Price)
	}
	if !comm.Amount.Equal(decimal.RequireFromString("0.76825")) {
		t.Fatalf("commission = %s, want 0.76825", comm.Amount)
	}
	if !filled.AvgFillPrice.Equal(decimal.RequireFromString("8.9")) {
		t.Fatalf("filled avg = %s, want 8.90", filled.AvgFillPrice)
	}

	if err := client.Options().Exercise(ctx, ibkr.ExerciseOptionsRequest{
		Contract:         exerciseAAPLJun12Call2825,
		ExerciseAction:   ibkr.Exercise,
		ExerciseQuantity: 1,
		Account:          "DU9000001",
	}); err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	// The acceptance notice is the only public trace of the working
	// exercise instruction: 10349 on the unrouted exercise request id is
	// emitted as a session event.
	notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderTIFSetFromPreset)
	if notice.Message != "Order TIF was set to DAY based on order preset." {
		t.Fatalf("10349 message = %q", notice.Message)
	}

	// The pseudo-order open_order/order_status under the exercise request id
	// are delivered next and dropped (no order route); the global cancel
	// then kills the instruction live. The code-161 for the already-filled
	// buy order routes to its handle as the terminal error.
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	requireOrderAPIError(t, "exercise buy", handle, ibkr.ErrCodeCancelNotCancellableState,
		"Order permId =900407")

	// The 322 server rejection and the 202 that cancels the exercise
	// instruction both target the exercise request id, which carries the
	// exercise route: both now surface as session events. (The pseudo-order's
	// Cancelled status between them is keyed by order id and still drops.)
	rejection := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeServerErrorProcessingRequest)
	if rejection.Message != "Error processing request.Exercise/Lapse failed due to server rejection." {
		t.Fatalf("322 message = %q", rejection.Message)
	}
	if canceled := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled); canceled.Message != "Order Canceled - reason:" {
		t.Fatalf("202 message = %q", canceled.Message)
	}
}
