package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// The two replays below freeze the option-exercise family (matrix row
// OPT-002) captured live on 2026-06-11 against paper Gateway server_version
// 200. Exercise (msg 21) has no dedicated completion callback, so the returned
// ExerciseHandle correlates request errors, warnings, and the pseudo-order
// lifecycle that the Gateway may materialize under the request id.

var exerciseAAPLJun12Call2925 = ibkr.Contract{
	ConID:        886441502,
	Symbol:       "AAPL",
	SecType:      ibkr.SecTypeOption,
	Expiry:       "20260612 16:00:00 US/Eastern",
	Strike:       new(decimal.RequireFromString("292.5")),
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
	Strike:       new(decimal.RequireFromString("282.5")),
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

	handle.Close()
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return
			}
			if evt.Lifecycle != nil {
				continue
			}
			t.Errorf("%s saw unexpected order event: %+v", name, evt)
		case <-ctx.Done():
			t.Fatalf("timeout draining %s order events", name)
		}
	}
}

func nextExerciseEvent(t *testing.T, ctx context.Context, handle *ibkr.ExerciseHandle) ibkr.OrderEvent {
	t.Helper()

	select {
	case event, ok := <-handle.Events():
		if !ok {
			t.Fatalf("exercise events closed early: %v", handle.Wait())
		}
		return event
	case <-ctx.Done():
		t.Fatal("timed out waiting for exercise event")
		return ibkr.OrderEvent{}
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
// request id. ExerciseHandle.Wait returns that request-scoped refusal.
func TestAPIOptionExerciseNotITMReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_option_exercise_not_itm_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
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
	if open.Contract.Strike == nil || !open.Contract.Strike.Equal(decimal.RequireFromString("292.5")) || open.Contract.Right != ibkr.RightCall {
		t.Fatalf("open strike/right = %s/%s, want 292.5/C", open.Contract.Strike, open.Contract.Right)
	}
	if open.Order.OrderType != ibkr.OrderTypeMarket {
		t.Fatalf("open order type = %s, want MKT", open.Order.OrderType)
	}
	if (*open.Order.PermID) != 900406 {
		t.Fatalf("perm id = %d, want 900406", (*open.Order.PermID))
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
	handle.Close()

	exerciseHandle, err := client.Options().Exercise(ctx, ibkr.ExerciseOptionsRequest{
		Contract:         exerciseAAPLJun12Call2925,
		ExerciseAction:   ibkr.Exercise,
		ExerciseQuantity: 1,
		Account:          "DU9000001",
	})
	if err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	refusal, ok := errors.AsType[*ibkr.APIError](exerciseHandle.Wait())
	if !ok || refusal.Code != ibkr.ErrCodeServerErrorProcessingRequest ||
		refusal.Message != "Error processing request.Exercise ignored because option is not in-the-money." {
		t.Fatalf("exercise Wait() = %#v, want request-scoped code-322 refusal", refusal)
	}

	// The handle saw the fill evidence; the caller now ends observation before
	// checking that no exercise reply was misrouted to the order.
	requireNoMoreOrderEvents(t, ctx, "exercise buy", handle)
	if err := handle.Wait(); err != nil {
		t.Fatalf("handle.Wait() = %v, want nil (terminal Filled before disconnect)", err)
	}
}

// TestAPIOptionExerciseServerRejectReplay freezes the deep-ITM exercise
// server-rejection lifecycle captured live on 2026-06-11
// (captures/20260611T133636Z-api_option_exercise_aapl, events.jsonl sha256
// prefix 267e7806669f2d5c): MKT BUY 1 of the AAPL Jun-12 282.5 call fills
// at 8.90, Exercise produces a working DAY instruction via code 10349 on the
// exercise request id, and the Gateway materializes the
// instruction as a pseudo-order under the same handle. When the
// teardown global cancel runs, the paper Gateway kills the instruction: code
// 322 "Exercise/Lapse failed due to server rejection." and its code-202
// notice both target the exercise request id. The code-161 reply for the filled buy order remains a session
// notice without replacing the handle's clean Filled result.
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
	if open.Contract.ConID != 887760542 || open.Contract.Strike == nil || !open.Contract.Strike.Equal(decimal.RequireFromString("282.5")) {
		t.Fatalf("open contract = %+v, want AAPL OPT 282.5 con 887760542", open.Contract)
	}
	if (*open.Order.PermID) != 900407 {
		t.Fatalf("perm id = %d, want 900407", (*open.Order.PermID))
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
	handle.Close()

	exerciseHandle, err := client.Options().Exercise(ctx, ibkr.ExerciseOptionsRequest{
		Contract:         exerciseAAPLJun12Call2825,
		ExerciseAction:   ibkr.Exercise,
		ExerciseQuantity: 1,
		Account:          "DU9000001",
	})
	if err != nil {
		t.Fatalf("Exercise: %v", err)
	}

	notice := nextExerciseEvent(t, ctx, exerciseHandle).Warning
	if notice == nil || notice.Code != ibkr.ErrCodeOrderTIFSetFromPreset ||
		notice.Message != "Order TIF was set to DAY based on order preset." {
		t.Fatalf("exercise notice = %#v, want request-scoped code 10349", notice)
	}

	openExercise := nextExerciseEvent(t, ctx, exerciseHandle).OpenOrder
	if openExercise == nil || *openExercise.Order.OrderID != int64(exerciseHandle.RequestID()) ||
		openExercise.Contract.ConID != 887760542 || openExercise.Order.Action != ibkr.ActionBuy ||
		openExercise.Order.OrderType != ibkr.OrderTypeLimit || !openExercise.Order.Quantity.Equal(decimal.NewFromInt(1)) ||
		openExercise.Order.TIF != ibkr.TIFDay || *openExercise.Order.PermID != 900005 {
		t.Fatalf("exercise pseudo-order = %+v, want request-bound DAY LMT BUY 1 with perm id 900005", openExercise)
	}
	working := nextExerciseEvent(t, ctx, exerciseHandle).Status
	if working == nil || working.OrderID != int64(exerciseHandle.RequestID()) ||
		working.Status != ibkr.OrderStatusPreSubmitted || !working.Remaining.Equal(decimal.NewFromInt(1)) ||
		working.PermID != 900005 {
		t.Fatalf("exercise status = %+v, want request-bound PreSubmitted remaining 1 with perm id 900005", working)
	}

	// The global cancel then kills the instruction live. The code-161 for the
	// already-filled buy order remains observable at session scope.
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	requireCancelNotCancellableNotice(t, ctx, events, "900407")

	// The 322 server rejection targets the exercise route and retires it. The
	// later 202 arrives after the handle is terminal and remains a session event.
	rejection, ok := errors.AsType[*ibkr.APIError](exerciseHandle.Wait())
	if !ok || rejection.Code != ibkr.ErrCodeServerErrorProcessingRequest ||
		rejection.Message != "Error processing request.Exercise/Lapse failed due to server rejection." {
		t.Fatalf("exercise Wait() = %#v, want request-scoped server rejection", rejection)
	}
	if canceled := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled); canceled.Message != "Order Canceled - reason:" {
		t.Fatalf("202 message = %q", canceled.Message)
	}
	requireNoMoreOrderEvents(t, ctx, "exercise buy", handle)
	requireOrderWaitNil(t, "exercise buy", handle)
}
