package ibkr_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func TestAPIOrderFillCampaignReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_fill_aapl.txt", ibkr.WithClientID(1))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	placeAndAssertFill := func(name string, action ibkr.OrderAction, wantID int64, ref string, side ibkr.ExecutionSide) {
		t.Helper()
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: ibkr.Order{
			Action: action, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(1),
			TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: ref,
		}})
		if err != nil {
			t.Fatalf("%s Place: %v", name, err)
		}
		if handle.OrderID() != wantID {
			t.Fatalf("%s OrderID() = %d, want %d", name, handle.OrderID(), wantID)
		}

		var status *ibkr.OrderStatusUpdate
		var execution *ibkr.Execution
		var fee *ibkr.CommissionAndFeesReport
		for status == nil || execution == nil || fee == nil {
			select {
			case event, ok := <-handle.Events():
				if !ok {
					t.Fatalf("%s events closed before fill evidence: %v", name, handle.Wait())
				}
				if event.Status != nil && event.Status.Status == ibkr.OrderStatusFilled {
					status = event.Status
				}
				if event.Execution != nil {
					execution = event.Execution
				}
				if event.CommissionAndFees != nil {
					fee = event.CommissionAndFees
				}
			case <-ctx.Done():
				t.Fatalf("%s waiting for fill evidence: %v", name, ctx.Err())
			}
		}
		if !status.Filled.Equal(decimal.NewFromInt(1)) || !status.Remaining.IsZero() {
			t.Fatalf("%s status = %+v, want one-share terminal fill", name, status)
		}
		if execution.OrderID != wantID || execution.Side != side ||
			!execution.Shares.Equal(decimal.NewFromInt(1)) || execution.OrderRef != ref {
			t.Fatalf("%s execution = %+v, want order %d side %s one share ref %q", name, execution, wantID, side, ref)
		}
		if fee.ExecID != execution.ExecID || fee.Amount == nil || fee.Currency != "USD" {
			t.Fatalf("%s fee = %+v, want USD report correlated with execution %q", name, fee, execution.ExecID)
		}
		handle.Close()
		requireCloseOrCapturedDisconnect(t, name, handle.Wait())
	}

	placeAndAssertFill("buy", ibkr.ActionBuy, 600, "sanitized-order-ref-0000000000000001", ibkr.ExecutionSideBought)
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime fill fence: %v", err)
	}
	placeAndAssertFill("flattening sell", ibkr.ActionSell, 601, "sanitized-order-ref-0000000000000005", ibkr.ExecutionSideSold)
}

func TestAPIOrderTypeMatrixReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_order_type_matrix_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)

	refs := []string{
		"sanitized-order-ref-0000000000000001", "sanitized-order-ref-0000000000000006",
		"sanitized-order-ref-0000000000000012", "sanitized-order-ref-0000000000000018",
		"sanitized-order-ref-0000000000000023", "sanitized-order-ref-0000000000000029",
		"sanitized-order-ref-0000000000000034", "sanitized-order-ref-0000000000000040",
		"sanitized-order-ref-0000000000000045", "sanitized-order-ref-0000000000000051",
		"sanitized-order-ref-0000000000000057", "sanitized-order-ref-0000000000000064",
		"sanitized-order-ref-0000000000000072", "sanitized-order-ref-0000000000000074",
		"sanitized-order-ref-0000000000000080", "sanitized-order-ref-0000000000000086",
		"sanitized-order-ref-0000000000000088", "sanitized-order-ref-0000000000000090",
		"sanitized-order-ref-0000000000000092", "sanitized-order-ref-0000000000000094",
		"sanitized-order-ref-0000000000000100",
	}
	placeIndex := 0
	place := func(name string, order ibkr.Order, wantID int64) *ibkr.OrderHandle {
		t.Helper()
		order.Quantity = decimal.NewFromInt(1)
		order.TIF = ibkr.TIFDay
		order.Account = "DU9000001"
		order.OrderRef = refs[placeIndex]
		placeIndex++
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: order})
		if err != nil {
			t.Fatalf("%s Place: %v", name, err)
		}
		if handle.OrderID() != wantID {
			t.Fatalf("%s OrderID() = %d, want %d", name, handle.OrderID(), wantID)
		}
		return handle
	}
	cancelWorking := func(name string, handle *ibkr.OrderHandle) {
		t.Helper()
		waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusPreSubmitted)
		if err := handle.Cancel(ctx); err != nil {
			t.Fatalf("%s Cancel: %v", name, err)
		}
		waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusCancelled)
		if err := handle.Wait(); err != nil {
			t.Fatalf("%s Wait: %v", name, err)
		}
	}

	active := map[string]*ibkr.OrderHandle{}
	active["market"] = place("market", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket}, 519)
	waitForOrderStatus(t, ctx, active["market"], ibkr.OrderStatusPreSubmitted)
	active["marketable limit"] = place("marketable limit", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, LmtPrice: new(decimal.NewFromInt(240))}, 520)
	waitForOrderStatus(t, ctx, active["marketable limit"], ibkr.OrderStatusPreSubmitted)

	cancelWorking("far limit", place("far limit", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, LmtPrice: new(decimal.NewFromInt(10))}, 521))
	cancelWorking("stop", place("stop", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeStop, AuxPrice: new(decimal.NewFromInt(240))}, 522))
	cancelWorking("stop-limit", place("stop-limit", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeStopLimit, LmtPrice: new(decimal.NewFromInt(241)), AuxPrice: new(decimal.NewFromInt(240))}, 523))
	cancelWorking("trailing", place("trailing", ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeTrailingStop, AuxPrice: new(decimal.NewFromInt(1)), TrailStopPrice: new(decimal.NewFromInt(2000))}, 524))
	cancelWorking("trailing-limit", place("trailing-limit", ibkr.Order{Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeTrailingLimit, AuxPrice: new(decimal.NewFromInt(1)), TrailStopPrice: new(decimal.NewFromInt(2000)), LmtPriceOffset: new(decimal.RequireFromString("0.05"))}, 525))
	cancelWorking("market-if-touched", place("market-if-touched", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarketIfTouched, AuxPrice: new(decimal.NewFromInt(240))}, 526))
	cancelWorking("limit-if-touched", place("limit-if-touched", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimitIfTouched, LmtPrice: new(decimal.NewFromInt(241)), AuxPrice: new(decimal.NewFromInt(240))}, 527))
	cancelWorking("market-to-limit", place("market-to-limit", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarketToLimit}, 528))
	cancelWorking("relative", place("relative", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeRelative, LmtPrice: new(decimal.NewFromInt(10))}, 529))

	modified := place("delayed modify", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, LmtPrice: new(decimal.NewFromInt(10))}, 530)
	waitForOrderStatus(t, ctx, modified, ibkr.OrderStatusPreSubmitted)
	if err := modified.Replace(ctx, ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: refs[11]}); err != nil {
		t.Fatalf("delayed Replace: %v", err)
	}
	waitForOrderStatus(t, ctx, modified, ibkr.OrderStatusPreSubmitted)
	active["modified"] = modified

	invalid := place("invalid type", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderType("FEELINGS"), LmtPrice: new(decimal.NewFromInt(10))}, 531)
	requireOrderAPIError(t, "invalid type", invalid, ibkr.ErrCodeServerErrorValidatingRequest, "Invalid order type")

	moc := place("market-on-close", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarketOnClose}, 532)
	waitForOrderStatus(t, ctx, moc, ibkr.OrderStatusPreSubmitted)
	if err := moc.Cancel(ctx); err != nil {
		t.Fatalf("MOC Cancel: %v", err)
	}
	mocStatuses := waitOrderStatuses(t, ctx, moc)
	if !hasOrderStatus(mocStatuses, ibkr.OrderStatusPendingCancel) || !hasOrderStatus(mocStatuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("MOC statuses = %v, want PendingCancel and Cancelled", mocStatuses)
	}

	loc := place("limit-on-close", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimitOnClose, LmtPrice: new(decimal.NewFromInt(240))}, 533)
	waitForOrderStatus(t, ctx, loc, ibkr.OrderStatusPreSubmitted)
	if err := loc.Cancel(ctx); err != nil {
		t.Fatalf("LOC Cancel: %v", err)
	}
	locStatuses := waitOrderStatuses(t, ctx, loc)
	if !hasOrderStatus(locStatuses, ibkr.OrderStatusPendingCancel) || !hasOrderStatus(locStatuses, ibkr.OrderStatusCancelled) {
		t.Fatalf("LOC statuses = %v, want PendingCancel and Cancelled", locStatuses)
	}

	for _, tc := range []struct {
		name string
		id   int64
		typ  ibkr.OrderType
		lmt  *decimal.Decimal
		code int
	}{
		{"market-on-open", 534, ibkr.OrderType("MOO"), nil, ibkr.ErrCodeServerErrorValidatingRequest},
		{"limit-on-open", 535, ibkr.OrderType("LOO"), new(decimal.NewFromInt(240)), ibkr.ErrCodeServerErrorValidatingRequest},
		{"pegged-market", 536, ibkr.OrderTypePeggedToMarket, new(decimal.NewFromInt(10)), ibkr.ErrCodeUnsupportedOrderType},
		{"pegged-primary", 537, ibkr.OrderType("PEG PRI"), new(decimal.NewFromInt(10)), ibkr.ErrCodeServerErrorValidatingRequest},
	} {
		handle := place(tc.name, ibkr.Order{Action: ibkr.ActionBuy, OrderType: tc.typ, LmtPrice: tc.lmt}, tc.id)
		requireOrderAPIError(t, tc.name, handle, tc.code, "")
	}

	cancelWorking("pegged-mid", place("pegged-mid", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypePeggedToMid, LmtPrice: new(decimal.NewFromInt(10))}, 538))
	pegBest := place("pegged-best", ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypePeggedToBest, LmtPrice: new(decimal.NewFromInt(10))}, 539)
	waitForOrderStatus(t, ctx, pegBest, ibkr.OrderStatusInactive)
	if err := pegBest.Cancel(ctx); err != nil {
		t.Fatalf("pegged-best Cancel: %v", err)
	}

	benchOrder := ibkr.Order{Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypePeggedBenchmark, Quantity: decimal.NewFromInt(1), LmtPrice: new(decimal.NewFromInt(10)), TIF: ibkr.TIFDay, Account: "DU9000001"}
	if handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: orderReplayAAPL, Order: benchOrder}); handle != nil {
		t.Fatalf("pegged-benchmark handle = %v, want nil", handle)
	} else if validation, ok := errors.AsType[*ibkr.ValidationError](err); !ok || validation.Field != "Order.PeggedBenchmark" {
		t.Fatalf("pegged-benchmark error = %v, want Order.PeggedBenchmark ValidationError", err)
	}

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(executions.Executions) != 0 || len(executions.CommissionAndFees) != 0 {
		t.Fatalf("executions/fees = %d/%d, want 0/0", len(executions.Executions), len(executions.CommissionAndFees))
	}
	for name, handle := range active {
		if err := handle.Wait(); !errors.Is(err, ibkr.ErrOrderRecoveryRequired) || !errors.Is(err, io.EOF) {
			t.Fatalf("%s Wait = %v, want ErrOrderRecoveryRequired and EOF", name, err)
		}
	}
}
