package ibkr_test

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// Contract constants
// ---------------------------------------------------------------------------

var eurusdContract = ibkr.Contract{
	Symbol:   "EUR",
	SecType:  ibkr.SecTypeForex,
	Exchange: "IDEALPRO",
	Currency: "USD",
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type orderResult struct {
	filled        bool
	cancelled     bool
	inactive      bool
	sawExecution  bool
	sawCommission bool
	lastStatus    ibkr.OrderStatus
	lastLmtPrice  decimal.Decimal
	lastQuantity  decimal.Decimal
	events        []ibkr.OrderEvent
}

func liveAnchorPrice(t *testing.T, ctx context.Context, client *ibkr.Client, contract ibkr.Contract, fallback decimal.Decimal) decimal.Decimal {
	t.Helper()
	// Always set delayed data first so paper accounts without real-time subscriptions
	// can still get a price. This avoids 10168 errors.
	_ = client.MarketData().SetType(ctx, ibkr.MarketDataDelayed)
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
	if err != nil {
		t.Logf("quote failed; using fallback %s: %v", fallback, err)
		return fallback
	}
	for _, candidate := range []decimal.Decimal{quote.Last, quote.Ask, quote.Bid, quote.Close} {
		if candidate.IsPositive() {
			return candidate
		}
	}
	return fallback
}

func liveFarBuy(anchor decimal.Decimal) decimal.Decimal {
	return anchor.Mul(decimal.RequireFromString("0.05")).Round(2)
}

func liveFarSell(anchor decimal.Decimal) decimal.Decimal {
	return anchor.Mul(decimal.RequireFromString("10")).Round(2)
}

func liveMarketableBuy(anchor decimal.Decimal) decimal.Decimal {
	return anchor.Mul(decimal.RequireFromString("1.20")).Round(2)
}

func liveMarketableSell(anchor decimal.Decimal) decimal.Decimal {
	return anchor.Mul(decimal.RequireFromString("0.80")).Round(2)
}

func liveAccount(t *testing.T, client *ibkr.Client) string {
	t.Helper()
	snap := client.Session()
	if len(snap.ManagedAccounts) == 0 {
		t.Fatal("no managed accounts in live session")
	}
	return snap.ManagedAccounts[0]
}

func liveDeferCleanup(t *testing.T, client *ibkr.Client) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Orders().CancelAll(ctx)
		time.Sleep(500 * time.Millisecond)
	})
}

func liveBaseOrder(account string, action ibkr.OrderAction, orderType ibkr.OrderType) ibkr.Order {
	return ibkr.Order{
		Action:    action,
		OrderType: orderType,
		Quantity:  decimal.NewFromInt(1),
		TIF:       ibkr.TIFDay,
		Account:   account,
	}
}

func liveObserveOrder(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle, label string, timeout time.Duration) orderResult {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var result orderResult
	recordEvent := func(evt ibkr.OrderEvent) {
		result.events = append(result.events, evt)
		if evt.OpenOrder != nil {
			t.Logf("%s open_order: orderID=%d type=%s status=%s lmt=%s qty=%s parent=%d oca=%q",
				label, evt.OpenOrder.OrderID, evt.OpenOrder.OrderType, evt.OpenOrder.Status,
				evt.OpenOrder.LmtPrice, evt.OpenOrder.Quantity, evt.OpenOrder.ParentID, evt.OpenOrder.OcaGroup)
			result.lastLmtPrice = evt.OpenOrder.LmtPrice
			result.lastQuantity = evt.OpenOrder.Quantity
		}
		if evt.Status != nil {
			t.Logf("%s status: %s filled=%s remaining=%s avg=%s",
				label, evt.Status.Status, evt.Status.Filled, evt.Status.Remaining, evt.Status.AvgFillPrice)
			result.lastStatus = evt.Status.Status
			if evt.Status.Status == ibkr.OrderStatusFilled {
				result.filled = true
			}
			if evt.Status.Status == ibkr.OrderStatusCancelled || evt.Status.Status == ibkr.OrderStatusApiCancelled {
				result.cancelled = true
			}
			if evt.Status.Status == ibkr.OrderStatusInactive {
				result.inactive = true
			}
		}
		if evt.Execution != nil {
			t.Logf("%s execution: execID=%s side=%s shares=%s price=%s time=%s",
				label, evt.Execution.ExecID, evt.Execution.Side, evt.Execution.Shares, evt.Execution.Price, evt.Execution.Time.Format(time.RFC3339))
			result.sawExecution = true
		}
		if evt.Commission != nil {
			t.Logf("%s commission: execID=%s commission=%s currency=%s pnl=%s",
				label, evt.Commission.ExecID, evt.Commission.Commission, evt.Commission.Currency, evt.Commission.RealizedPNL)
			result.sawCommission = true
		}
	}

	// The correct idiom: drain Events() until the channel closes. The library
	// guarantees events (including execution/commission that arrive after a
	// terminal status) are fully delivered before the channel closes. Done()
	// closes after Events(), so no select on Done() is needed.
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return result
			}
			recordEvent(evt)
		case <-timer.C:
			return result
		case <-ctx.Done():
			return result
		}
	}
}

func livePlaceAndCancel(t *testing.T, ctx context.Context, client *ibkr.Client, contract ibkr.Contract, order ibkr.Order) {
	t.Helper()
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: contract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	t.Logf("placed orderID=%d type=%s action=%s", handle.OrderID(), order.OrderType, order.Action)

	// Wait for at least one event before cancelling.
	select {
	case evt := <-handle.Events():
		if evt.Status != nil {
			t.Logf("initial status: %s", evt.Status.Status)
			if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
				t.Logf("order went terminal immediately (%s); skipping cancel", evt.Status.Status)
				return
			}
		}
	case <-handle.Done():
		t.Log("order reached terminal state before initial event")
		return
	case <-time.After(15 * time.Second):
		t.Log("timeout waiting for initial event; cancelling anyway")
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Logf("Cancel: %v (may already be terminal)", err)
	}

	// Drain events until terminal or timeout. The cancel confirmation arrives
	// as a status event, and handle.Done() only closes after the terminal
	// drain window (750ms).
	result := liveObserveOrder(t, ctx, handle, "cancel-drain", 15*time.Second)
	t.Logf("cancel result: lastStatus=%s cancelled=%v inactive=%v", result.lastStatus, result.cancelled, result.inactive)
}

func liveTryPlaceObserveCancel(t *testing.T, ctx context.Context, client *ibkr.Client, contract ibkr.Contract, order ibkr.Order, label string) orderResult {
	t.Helper()
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: contract, Order: order})
	if err != nil {
		t.Logf("%s PlaceOrder returned real Gateway error: %v", label, err)
		return orderResult{lastStatus: ibkr.OrderStatusInactive}
	}
	t.Logf("%s placed orderID=%d type=%s action=%s", label, handle.OrderID(), order.OrderType, order.Action)

	result := liveObserveOrder(t, ctx, handle, label, 15*time.Second)
	if result.filled || result.cancelled || result.inactive {
		return result
	}
	if err := handle.Cancel(ctx); err != nil {
		t.Logf("%s Cancel: %v", label, err)
	}
	cancelResult := liveObserveOrder(t, ctx, handle, label+" cancel", 15*time.Second)
	if cancelResult.cancelled || cancelResult.inactive {
		result.cancelled = cancelResult.cancelled
		result.inactive = cancelResult.inactive
		result.lastStatus = cancelResult.lastStatus
	}
	return result
}

func livePlaceFillAndFlatten(t *testing.T, ctx context.Context, client *ibkr.Client, contract ibkr.Contract, order ibkr.Order, account string) orderResult {
	t.Helper()
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: contract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	t.Logf("placed orderID=%d type=%s action=%s", handle.OrderID(), order.OrderType, order.Action)

	result := liveObserveOrder(t, ctx, handle, "fill", 30*time.Second)
	if !result.filled {
		t.Fatalf("order did not fill; last status: %s", result.lastStatus)
	}

	// Flatten.
	flattenAction := ibkr.Sell
	if order.Action == ibkr.Sell {
		flattenAction = ibkr.Buy
	}
	flatHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: contract,
		Order: ibkr.Order{
			Action:    flattenAction,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  order.Quantity,
			TIF:       ibkr.TIFDay,
			Account:   account,
		},
	})
	if err != nil {
		t.Fatalf("flatten PlaceOrder: %v", err)
	}
	flatResult := liveObserveOrder(t, ctx, flatHandle, "flatten", 30*time.Second)
	if !flatResult.filled {
		t.Logf("flatten did not fill; last status: %s", flatResult.lastStatus)
	}
	return result
}

// liveQualifyOption qualifies the nearest ATM option for the given underlying.
func liveQualifyOption(t *testing.T, ctx context.Context, client *ibkr.Client, underlyingConID int, anchor decimal.Decimal, right ibkr.Right) ibkr.Contract {
	t.Helper()
	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   underlyingConID,
	})
	if err != nil {
		t.Fatalf("SecDefOptParams: %v", err)
	}
	param, ok := liveChooseOptionParams(params)
	if !ok {
		t.Fatal("no AAPL SMART option params found")
	}
	expiry, ok := liveChooseFutureExpiry(param.Expirations)
	if !ok {
		t.Fatal("no future AAPL option expiration")
	}
	strike, ok := liveChooseNearestStrike(param.Strikes, anchor)
	if !ok {
		t.Fatal("no AAPL option strikes")
	}
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:       "AAPL",
		SecType:      ibkr.SecTypeOption,
		Expiry:       expiry,
		Strike:       strike.String(),
		Right:        right,
		Multiplier:   param.Multiplier,
		Exchange:     "SMART",
		Currency:     "USD",
		TradingClass: param.TradingClass,
	})
	if err != nil {
		t.Fatalf("option ContractDetails: %v", err)
	}
	if len(details) == 0 {
		t.Fatal("no qualified option contract details")
	}
	t.Logf("qualified option: conID=%d symbol=%s expiry=%s strike=%s right=%s",
		details[0].ConID, details[0].Symbol, details[0].Expiry, strike, right)
	return details[0].Contract
}

func liveQualifyVerticalLegs(t *testing.T, ctx context.Context, client *ibkr.Client, anchor decimal.Decimal) (ibkr.Contract, ibkr.Contract) {
	t.Helper()
	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   265598,
	})
	if err != nil {
		t.Fatalf("SecDefOptParams: %v", err)
	}
	param, ok := liveChooseOptionParams(params)
	if !ok {
		t.Fatal("no AAPL SMART option params")
	}
	expiry, ok := liveChooseFutureExpiry(param.Expirations)
	if !ok {
		t.Fatal("no future option expiration")
	}
	lower, upper, ok := liveChooseVerticalStrikes(param.Strikes, anchor)
	if !ok {
		t.Fatal("not enough strikes for vertical")
	}
	qualify := func(strike decimal.Decimal) ibkr.Contract {
		details, err := client.Contracts().Details(ctx, ibkr.Contract{
			Symbol:       "AAPL",
			SecType:      ibkr.SecTypeOption,
			Expiry:       expiry,
			Strike:       strike.String(),
			Right:        ibkr.RightCall,
			Multiplier:   param.Multiplier,
			Exchange:     "SMART",
			Currency:     "USD",
			TradingClass: param.TradingClass,
		})
		if err != nil {
			t.Fatalf("vertical leg ContractDetails strike=%s: %v", strike, err)
		}
		if len(details) == 0 {
			t.Fatalf("no contract details for strike %s", strike)
		}
		return details[0].Contract
	}
	lo := qualify(lower)
	hi := qualify(upper)
	t.Logf("vertical legs: lower conID=%d strike=%s, upper conID=%d strike=%s", lo.ConID, lower, hi.ConID, upper)
	return lo, hi
}

func liveQualifyFrontFuture(t *testing.T, ctx context.Context, client *ibkr.Client, symbol string) ibkr.Contract {
	t.Helper()
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   symbol,
		SecType:  ibkr.SecTypeFuture,
		Exchange: "CME",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("future ContractDetails: %v", err)
	}
	sort.Slice(details, func(i, j int) bool { return details[i].Expiry < details[j].Expiry })
	now := time.Now().Format("20060102")
	for _, d := range details {
		if d.Expiry >= now {
			t.Logf("qualified future: conID=%d symbol=%s expiry=%s", d.ConID, d.Symbol, d.Expiry)
			return d.Contract
		}
	}
	if len(details) > 0 {
		return details[0].Contract
	}
	t.Fatalf("no %s future contract found", symbol)
	return ibkr.Contract{}
}

func liveChooseOptionParams(params []ibkr.SecDefOptParams) (ibkr.SecDefOptParams, bool) {
	for _, p := range params {
		if p.Exchange == "SMART" && p.Multiplier != "" && len(p.Expirations) > 0 && len(p.Strikes) > 0 {
			return p, true
		}
	}
	for _, p := range params {
		if p.Multiplier != "" && len(p.Expirations) > 0 && len(p.Strikes) > 0 {
			return p, true
		}
	}
	return ibkr.SecDefOptParams{}, false
}

func liveChooseFutureExpiry(expirations []string) (string, bool) {
	now := time.Now().Format("20060102")
	sorted := append([]string(nil), expirations...)
	sort.Strings(sorted)
	for _, e := range sorted {
		if e >= now {
			return e, true
		}
	}
	return "", false
}

func liveChooseNearestStrike(strikes []decimal.Decimal, anchor decimal.Decimal) (decimal.Decimal, bool) {
	if len(strikes) == 0 {
		return decimal.Zero, false
	}
	best := strikes[0]
	bestDist := strikes[0].Sub(anchor).Abs()
	for _, s := range strikes[1:] {
		d := s.Sub(anchor).Abs()
		if d.LessThan(bestDist) {
			best = s
			bestDist = d
		}
	}
	return best, true
}

func liveChooseVerticalStrikes(strikes []decimal.Decimal, anchor decimal.Decimal) (decimal.Decimal, decimal.Decimal, bool) {
	sorted := append([]decimal.Decimal(nil), strikes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LessThan(sorted[j]) })
	for i := 0; i+1 < len(sorted); i++ {
		if sorted[i].GreaterThanOrEqual(anchor) {
			return sorted[i], sorted[i+1], true
		}
	}
	if len(sorted) >= 2 {
		return sorted[len(sorted)-2], sorted[len(sorted)-1], true
	}
	return decimal.Zero, decimal.Zero, false
}

// ---------------------------------------------------------------------------
// Tier 1: Order Type Rest/Cancel
// ---------------------------------------------------------------------------

func TestLiveOrderLimitRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(anchor)

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderStopRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeStop)
	order.AuxPrice = liveMarketableBuy(anchor)

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderStopLimitRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeStopLimit)
	order.AuxPrice = liveMarketableBuy(anchor)
	order.LmtPrice = liveMarketableBuy(anchor).Add(decimal.NewFromInt(1))

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderTrailingStopRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Sell, ibkr.OrderTypeTrailingStop)
	order.TrailStopPrice = liveFarSell(anchor)
	order.AuxPrice = decimal.RequireFromString("1")

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderTrailingLimitRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Sell, ibkr.OrderTypeTrailingLimit)
	order.TrailStopPrice = liveFarSell(anchor)
	order.AuxPrice = decimal.RequireFromString("1")
	order.LmtPriceOffset = decimal.RequireFromString("0.05")

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderMITRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarketIfTouched)
	order.AuxPrice = liveFarBuy(anchor)

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderLITRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimitIfTouched)
	order.AuxPrice = liveFarBuy(anchor)
	order.LmtPrice = liveFarBuy(anchor).Add(decimal.NewFromInt(1))

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderRelativeRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeRelative)
	order.LmtPrice = liveFarBuy(anchor)

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderOpenCloseTypesAcceptOrReject(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 45*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	cases := []struct {
		label string
		order ibkr.Order
	}{
		{label: "moc", order: liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarketOnClose)},
		{label: "loc", order: func() ibkr.Order {
			order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimitOnClose)
			order.LmtPrice = liveMarketableBuy(anchor)
			return order
		}()},
		{label: "moo", order: liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarketOnOpen)},
		{label: "loo", order: func() ibkr.Order {
			order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimitOnOpen)
			order.LmtPrice = liveMarketableBuy(anchor)
			return order
		}()},
	}
	for _, tc := range cases {
		result := liveTryPlaceObserveCancel(t, ctx, client, aaplContract, tc.order, tc.label)
		if result.filled {
			flatten := liveBaseOrder(account, ibkr.Sell, ibkr.OrderTypeMarket)
			liveTryPlaceObserveCancel(t, ctx, client, aaplContract, flatten, tc.label+" flatten")
		}
	}
}

func TestLiveOrderPegFamiliesAcceptOrReject(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	for _, orderType := range []ibkr.OrderType{
		ibkr.OrderTypePeggedToMarket,
		ibkr.OrderTypePeggedToPrimary,
		ibkr.OrderTypePeggedToMid,
		ibkr.OrderTypePeggedToBest,
		ibkr.OrderTypePeggedBenchmark,
	} {
		order := liveBaseOrder(account, ibkr.Buy, orderType)
		order.LmtPrice = liveFarBuy(anchor)
		liveTryPlaceObserveCancel(t, ctx, client, aaplContract, order, string(orderType))
	}
}

// ---------------------------------------------------------------------------
// Tier 2: Immediate Fill Scenarios
// ---------------------------------------------------------------------------

func TestLiveOrderMarketBuyFill(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)

	result := livePlaceFillAndFlatten(t, ctx, client, aaplContract, order, account)
	if !result.sawExecution {
		t.Fatal("MKT BUY filled but never delivered Execution event")
	}
	if !result.sawCommission {
		t.Fatal("MKT BUY filled but never delivered Commission event")
	}
	t.Logf("MKT BUY sawExecution=%v sawCommission=%v", result.sawExecution, result.sawCommission)
}

func TestLiveOrderMarketableLimitFill(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveMarketableBuy(anchor)

	result := livePlaceFillAndFlatten(t, ctx, client, aaplContract, order, account)
	t.Logf("marketable LMT sawExecution=%v sawCommission=%v", result.sawExecution, result.sawCommission)
}

func TestLiveOrderMarketToLimitFill(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarketToLimit)

	result := livePlaceFillAndFlatten(t, ctx, client, aaplContract, order, account)
	if !result.filled {
		t.Error("MTL order did not fill")
	}
}

func TestLiveOrderIOCFill(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveMarketableBuy(anchor)
	order.TIF = ibkr.TIFIOC

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder IOC: %v", err)
	}
	result := liveObserveOrder(t, ctx, handle, "ioc", 30*time.Second)

	// IOC semantics: fill immediately or cancel remainder.
	if !result.filled && !result.cancelled && !result.inactive {
		t.Fatalf("IOC order did not reach terminal; last status: %s", result.lastStatus)
	}
	t.Logf("IOC result: filled=%v cancelled=%v", result.filled, result.cancelled)

	// Flatten if filled.
	if result.filled {
		flat, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplContract,
			Order: ibkr.Order{
				Action: ibkr.Sell, OrderType: ibkr.OrderTypeMarket,
				Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: account,
			},
		})
		if err != nil {
			t.Fatalf("flatten: %v", err)
		}
		liveObserveOrder(t, ctx, flat, "ioc-flatten", 30*time.Second)
	}
}

func TestLiveOrderFOKFillOrReject(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	// Fillable FOK.
	fillable := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	fillable.LmtPrice = liveMarketableBuy(anchor)
	fillable.TIF = ibkr.TIFFOK

	h1, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: fillable})
	if err != nil {
		t.Fatalf("PlaceOrder fillable FOK: %v", err)
	}
	r1 := liveObserveOrder(t, ctx, h1, "fok-fillable", 30*time.Second)
	t.Logf("fillable FOK: filled=%v cancelled=%v", r1.filled, r1.cancelled)

	// Flatten if filled.
	if r1.filled {
		flat, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplContract,
			Order: ibkr.Order{
				Action: ibkr.Sell, OrderType: ibkr.OrderTypeMarket,
				Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: account,
			},
		})
		if err != nil {
			t.Fatalf("flatten fillable FOK: %v", err)
		}
		liveObserveOrder(t, ctx, flat, "fok-flatten", 30*time.Second)
	}

	// Unfillable FOK: far below market.
	unfillable := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	unfillable.LmtPrice = liveFarBuy(anchor)
	unfillable.TIF = ibkr.TIFFOK

	h2, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: unfillable})
	if err != nil {
		t.Fatalf("PlaceOrder unfillable FOK: %v", err)
	}
	r2 := liveObserveOrder(t, ctx, h2, "fok-unfillable", 30*time.Second)
	if r2.filled {
		t.Error("unfillable FOK should not have filled")
	}
	if !r2.cancelled && !r2.inactive {
		t.Logf("unfillable FOK unexpected terminal: %s", r2.lastStatus)
	}
}

// ---------------------------------------------------------------------------
// Tier 3: Rejection Scenarios
// ---------------------------------------------------------------------------

func TestLiveOrderRejectInvalidContract(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	bogus := ibkr.Contract{Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"}
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: bogus, Order: order})
	if err != nil {
		t.Logf("PlaceOrder returned immediate error (acceptable): %v", err)
		return
	}
	result := liveObserveOrder(t, ctx, handle, "invalid-contract", 15*time.Second)
	if result.sawExecution {
		t.Error("invalid contract order should never produce an Execution")
	}
	if !result.inactive && !result.cancelled {
		t.Logf("invalid contract result: lastStatus=%s (expected Inactive or Cancelled)", result.lastStatus)
	}
}

func TestLiveOrderRejectInvalidType(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderType("FEELINGS"))
	order.LmtPrice = decimal.RequireFromString("100")

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Logf("PlaceOrder returned immediate error (acceptable): %v", err)
		return
	}
	result := liveObserveOrder(t, ctx, handle, "invalid-type", 15*time.Second)
	if result.sawExecution {
		t.Error("invalid order type should never produce an Execution")
	}
}

func TestLiveOrderCancelUnknownID(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()

	// Fire-and-forget: should not panic.
	err := client.Orders().Cancel(ctx, 999999999)
	t.Logf("Cancel unknown ID: err=%v", err)
}

func TestLiveOrderDoubleCancelOrder(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(anchor)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	// Wait for initial event.
	select {
	case <-handle.Events():
	case <-handle.Done():
	case <-time.After(15 * time.Second):
	}

	// First cancel.
	if err := handle.Cancel(ctx); err != nil {
		t.Logf("first Cancel: %v", err)
	}
	select {
	case <-handle.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for cancelled state")
	}

	// Second cancel should not panic.
	err = handle.Cancel(ctx)
	t.Logf("second Cancel after terminal: err=%v (expected error or nil)", err)
}

func TestLiveOrderModifyFilledOrder(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	result := liveObserveOrder(t, ctx, handle, "fill-then-modify", 30*time.Second)
	if !result.filled {
		t.Fatalf("order did not fill; last status: %s", result.lastStatus)
	}

	// Wait for handle closure (750ms drain window).
	select {
	case <-handle.Done():
	case <-time.After(5 * time.Second):
	}

	// Attempt modify on filled order: should error or be ignored.
	err = handle.Modify(ctx, ibkr.Order{
		Action:    ibkr.Buy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.NewFromInt(1),
		LmtPrice:  decimal.RequireFromString("100"),
		TIF:       ibkr.TIFDay,
		Account:   account,
	})
	t.Logf("Modify after fill: err=%v (expected error)", err)

	// Flatten.
	flat, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Sell, OrderType: ibkr.OrderTypeMarket,
			Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: account,
		},
	})
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	liveObserveOrder(t, ctx, flat, "modify-flatten", 30*time.Second)
}

// ---------------------------------------------------------------------------
// Tier 4: Modification Scenarios
// ---------------------------------------------------------------------------

func TestLiveOrderModifyLimitToFill(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(anchor)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	t.Logf("placed far LMT orderID=%d lmt=%s", handle.OrderID(), order.LmtPrice)

	// Wait for initial status.
	select {
	case evt := <-handle.Events():
		if evt.Status != nil {
			t.Logf("initial status: %s", evt.Status.Status)
		}
	case <-handle.Done():
		t.Fatal("order went terminal before modify")
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for initial status")
	}

	// Modify to market order to force fill.
	if err := handle.Modify(ctx, ibkr.Order{
		Action:    ibkr.Buy,
		OrderType: ibkr.OrderTypeMarket,
		Quantity:  decimal.NewFromInt(1),
		TIF:       ibkr.TIFDay,
		Account:   account,
	}); err != nil {
		t.Fatalf("Modify to MKT: %v", err)
	}

	result := liveObserveOrder(t, ctx, handle, "modified-to-mkt", 30*time.Second)
	if !result.filled {
		t.Fatalf("modified order did not fill; last status: %s", result.lastStatus)
	}
	t.Logf("modify-to-fill sawExecution=%v sawCommission=%v", result.sawExecution, result.sawCommission)

	// Flatten.
	flat, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Sell, OrderType: ibkr.OrderTypeMarket,
			Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: account,
		},
	})
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	liveObserveOrder(t, ctx, flat, "modify-flatten", 30*time.Second)
}

func TestLiveOrderModifyQuantity(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.Quantity = decimal.NewFromInt(5)
	order.LmtPrice = liveFarBuy(anchor)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	select {
	case <-handle.Events():
	case <-handle.Done():
		t.Fatal("order went terminal before modify")
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for initial event")
	}

	// Modify quantity from 5 to 3.
	if err := handle.Modify(ctx, ibkr.Order{
		Action:    ibkr.Buy,
		OrderType: ibkr.OrderTypeLimit,
		Quantity:  decimal.NewFromInt(3),
		LmtPrice:  liveFarBuy(anchor),
		TIF:       ibkr.TIFDay,
		Account:   account,
	}); err != nil {
		t.Fatalf("Modify qty: %v", err)
	}

	// Look for OpenOrder with qty=3.
	deadline := time.After(10 * time.Second)
	var sawModified bool
	for !sawModified {
		select {
		case evt := <-handle.Events():
			if evt.OpenOrder != nil && evt.OpenOrder.Quantity.Equal(decimal.NewFromInt(3)) {
				t.Logf("confirmed modified qty: %s", evt.OpenOrder.Quantity)
				sawModified = true
			}
		case <-handle.Done():
			sawModified = true
		case <-deadline:
			t.Log("timeout waiting for modified OpenOrder; order may have gone terminal")
			sawModified = true
		}
		if sawModified {
			break
		}
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Logf("Cancel: %v", err)
	}
	select {
	case <-handle.Done():
	case <-time.After(15 * time.Second):
	}
}

func TestLiveOrderRapidModifications(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(anchor)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	select {
	case <-handle.Events():
	case <-handle.Done():
		t.Fatal("order went terminal before modifications")
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for initial event")
	}

	// Fire 5 modifications rapidly.
	baseFar := liveFarBuy(anchor)
	for i := 1; i <= 5; i++ {
		price := baseFar.Add(decimal.NewFromInt(int64(i)))
		if err := handle.Modify(ctx, ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.NewFromInt(1),
			LmtPrice:  price,
			TIF:       ibkr.TIFDay,
			Account:   account,
		}); err != nil {
			t.Fatalf("Modify #%d: %v", i, err)
		}
	}

	// Drain events for a few seconds looking for the final price.
	expectedFinal := baseFar.Add(decimal.NewFromInt(5))
	deadline := time.After(10 * time.Second)
	var lastSeen decimal.Decimal
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				goto done
			}
			if evt.OpenOrder != nil {
				lastSeen = evt.OpenOrder.LmtPrice
				t.Logf("rapid modify OpenOrder lmt=%s", lastSeen)
			}
		case <-handle.Done():
			goto done
		case <-deadline:
			goto done
		}
	}
done:
	if lastSeen.IsPositive() {
		t.Logf("last observed lmt=%s, expected final=%s", lastSeen, expectedFinal)
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Logf("Cancel: %v", err)
	}
	select {
	case <-handle.Done():
	case <-time.After(10 * time.Second):
	}
}

// ---------------------------------------------------------------------------
// Tier 5: Bracket Orders
// ---------------------------------------------------------------------------

func TestLiveBracketFillChildrenActivate(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	// Parent: MKT BUY, don't transmit.
	parent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Buy, OrderType: ibkr.OrderTypeMarket,
			Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: account,
			Transmit: new(false),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder parent: %v", err)
	}
	t.Logf("parent orderID=%d", parent.OrderID())

	// Take-profit: LMT SELL far above.
	tp, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Sell, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.NewFromInt(1), LmtPrice: liveFarSell(anchor),
			TIF: ibkr.TIFGTC, Account: account,
			ParentID: parent.OrderID(), Transmit: new(false),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder take-profit: %v", err)
	}
	t.Logf("take-profit orderID=%d", tp.OrderID())

	// Stop-loss: STP SELL far below. Transmit=true triggers bracket.
	sl, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Sell, OrderType: ibkr.OrderTypeStop,
			Quantity: decimal.NewFromInt(1), AuxPrice: liveFarBuy(anchor),
			TIF: ibkr.TIFGTC, Account: account,
			ParentID: parent.OrderID(),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder stop-loss: %v", err)
	}
	t.Logf("stop-loss orderID=%d", sl.OrderID())

	// Parent should fill.
	parentResult := liveObserveOrder(t, ctx, parent, "bracket-parent", 30*time.Second)
	if !parentResult.filled {
		t.Fatalf("bracket parent did not fill; last status: %s", parentResult.lastStatus)
	}

	// Children should become active. Wait for at least one event on each.
	for _, pair := range []struct {
		name   string
		handle *ibkr.OrderHandle
	}{{"bracket-tp", tp}, {"bracket-sl", sl}} {
		select {
		case evt := <-pair.handle.Events():
			if evt.Status != nil {
				t.Logf("%s status: %s", pair.name, evt.Status.Status)
			}
			if evt.OpenOrder != nil {
				t.Logf("%s open_order received", pair.name)
			}
		case <-pair.handle.Done():
			t.Logf("%s reached terminal immediately", pair.name)
		case <-time.After(15 * time.Second):
			t.Logf("%s timeout waiting for activation", pair.name)
		}
	}
}

func TestLiveBracketTriggerTakeProfit(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 90*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	parent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Buy, OrderType: ibkr.OrderTypeMarket,
			Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: account,
			Transmit: new(false),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder parent: %v", err)
	}

	tp, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Sell, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.NewFromInt(1), LmtPrice: liveFarSell(anchor),
			TIF: ibkr.TIFGTC, Account: account,
			ParentID: parent.OrderID(), Transmit: new(false),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder TP: %v", err)
	}

	sl, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Sell, OrderType: ibkr.OrderTypeStop,
			Quantity: decimal.NewFromInt(1), AuxPrice: liveFarBuy(anchor),
			TIF: ibkr.TIFGTC, Account: account,
			ParentID: parent.OrderID(),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder SL: %v", err)
	}

	// Wait for parent fill.
	parentResult := liveObserveOrder(t, ctx, parent, "bracket-parent", 30*time.Second)
	if !parentResult.filled {
		t.Fatalf("parent did not fill; last status: %s", parentResult.lastStatus)
	}

	// Wait for TP to become active.
	select {
	case <-tp.Events():
	case <-tp.Done():
	case <-time.After(15 * time.Second):
	}

	// Modify TP to marketable sell to trigger fill.
	if err := tp.Modify(ctx, ibkr.Order{
		Action: ibkr.Sell, OrderType: ibkr.OrderTypeLimit,
		Quantity: decimal.NewFromInt(1), LmtPrice: liveMarketableSell(anchor),
		TIF: ibkr.TIFGTC, Account: account,
		ParentID: parent.OrderID(),
	}); err != nil {
		t.Fatalf("Modify TP to marketable: %v", err)
	}

	tpResult := liveObserveOrder(t, ctx, tp, "bracket-tp-triggered", 30*time.Second)
	if !tpResult.filled {
		t.Logf("TP did not fill after modify; last status: %s", tpResult.lastStatus)
	}

	// SL should auto-cancel (bracket sibling cancellation).
	slResult := liveObserveOrder(t, ctx, sl, "bracket-sl-cancelled", 15*time.Second)
	if tpResult.filled && !slResult.cancelled {
		t.Logf("SL expected Cancelled after TP fill; got: %s", slResult.lastStatus)
	}
}

func TestLiveBracketCancelBeforeTransmit(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	parent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Buy, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.NewFromInt(1), LmtPrice: liveFarBuy(anchor),
			TIF: ibkr.TIFDay, Account: account,
			Transmit: new(false),
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder parent: %v", err)
	}

	// Cancel parent before transmitting the bracket.
	if err := parent.Cancel(ctx); err != nil {
		t.Logf("Cancel parent: %v", err)
	}
	select {
	case <-parent.Done():
		t.Log("parent cancelled before transmission")
	case <-time.After(15 * time.Second):
		t.Log("timeout waiting for parent cancellation")
	}
}

// ---------------------------------------------------------------------------
// Tier 6: OCA Groups
// ---------------------------------------------------------------------------

func TestLiveOCAFillCancelsOthers(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	ocaGroup := "ibkr-go-oca-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	// Marketable: should fill immediately.
	marketableOrder := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	marketableOrder.LmtPrice = liveMarketableBuy(anchor)
	marketableOrder.OcaGroup = ocaGroup
	marketableOrder.OcaType = 1

	h1, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: marketableOrder})
	if err != nil {
		t.Fatalf("PlaceOrder marketable OCA: %v", err)
	}

	// Resting: should be cancelled by OCA.
	resting1 := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	resting1.LmtPrice = liveFarBuy(anchor)
	resting1.OcaGroup = ocaGroup
	resting1.OcaType = 1

	h2, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: resting1})
	if err != nil {
		t.Fatalf("PlaceOrder resting OCA 1: %v", err)
	}

	resting2 := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	resting2.LmtPrice = liveFarBuy(anchor).Sub(decimal.NewFromInt(1))
	resting2.OcaGroup = ocaGroup
	resting2.OcaType = 1

	h3, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: resting2})
	if err != nil {
		t.Fatalf("PlaceOrder resting OCA 2: %v", err)
	}

	r1 := liveObserveOrder(t, ctx, h1, "oca-marketable", 30*time.Second)
	if !r1.filled {
		t.Logf("OCA marketable did not fill (market may be closed); last status: %s", r1.lastStatus)
		return
	}

	// Resting peers should be cancelled.
	r2 := liveObserveOrder(t, ctx, h2, "oca-resting-1", 15*time.Second)
	r3 := liveObserveOrder(t, ctx, h3, "oca-resting-2", 15*time.Second)
	if !r2.cancelled {
		t.Logf("OCA resting-1 expected Cancelled; got: %s", r2.lastStatus)
	}
	if !r3.cancelled {
		t.Logf("OCA resting-2 expected Cancelled; got: %s", r3.lastStatus)
	}

	// Flatten.
	flat, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.Sell, OrderType: ibkr.OrderTypeMarket,
			Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: account,
		},
	})
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	liveObserveOrder(t, ctx, flat, "oca-flatten", 30*time.Second)
}

func TestLiveOCACancelAll(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	ocaGroup := "ibkr-go-oca-cancel-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	var handles []*ibkr.OrderHandle
	for i := 0; i < 2; i++ {
		order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
		order.LmtPrice = liveFarBuy(anchor).Add(decimal.NewFromInt(int64(i)))
		order.OcaGroup = ocaGroup
		order.OcaType = 1

		h, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
		if err != nil {
			t.Fatalf("PlaceOrder OCA[%d]: %v", i, err)
		}
		handles = append(handles, h)
	}

	// Wait for initial events.
	for i, h := range handles {
		select {
		case <-h.Events():
		case <-h.Done():
		case <-time.After(15 * time.Second):
			t.Fatalf("timeout waiting for OCA[%d] initial event", i)
		}
	}

	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}

	for i, h := range handles {
		select {
		case <-h.Done():
			t.Logf("OCA[%d] done", i)
		case <-time.After(15 * time.Second):
			t.Fatalf("timeout waiting for OCA[%d] terminal after CancelAll", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Tier 7: Multi-Asset Scenarios
// ---------------------------------------------------------------------------

func TestLiveOptionLimitRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 45*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	optContract := liveQualifyOption(t, ctx, client, 265598, anchor, ibkr.RightCall)

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = decimal.RequireFromString("0.01") // Far below any option price.

	livePlaceAndCancel(t, ctx, client, optContract, order)
}

func TestLiveOptionBuySellRoundTrip(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 90*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	optContract := liveQualifyOption(t, ctx, client, 265598, anchor, ibkr.RightCall)

	buyOrder := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)
	buyHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: optContract, Order: buyOrder})
	if err != nil {
		t.Fatalf("PlaceOrder option BUY: %v", err)
	}
	buyResult := liveObserveOrder(t, ctx, buyHandle, "opt-buy", 30*time.Second)
	if !buyResult.filled {
		t.Logf("option BUY did not fill (may need permissions or market closed); last status: %s", buyResult.lastStatus)
		return
	}

	sellOrder := liveBaseOrder(account, ibkr.Sell, ibkr.OrderTypeMarket)
	sellHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: optContract, Order: sellOrder})
	if err != nil {
		t.Fatalf("PlaceOrder option SELL: %v", err)
	}
	sellResult := liveObserveOrder(t, ctx, sellHandle, "opt-sell", 30*time.Second)
	if !sellResult.filled {
		t.Logf("option SELL did not fill; last status: %s", sellResult.lastStatus)
	}
}

func TestLiveFutureLimitRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 45*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	futContract := liveQualifyFrontFuture(t, ctx, client, "MES")
	futAnchor := liveAnchorPrice(t, ctx, client, futContract, decimal.RequireFromString("5000"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(futAnchor)

	livePlaceAndCancel(t, ctx, client, futContract, order)
}

func TestLiveFutureBuySellRoundTrip(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 90*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	futContract := liveQualifyFrontFuture(t, ctx, client, "MES")

	buyOrder := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)
	buyHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: futContract, Order: buyOrder})
	if err != nil {
		t.Fatalf("PlaceOrder future BUY: %v", err)
	}
	buyResult := liveObserveOrder(t, ctx, buyHandle, "fut-buy", 30*time.Second)
	if !buyResult.filled {
		t.Logf("future BUY did not fill (may need permissions); last status: %s", buyResult.lastStatus)
		return
	}

	sellOrder := liveBaseOrder(account, ibkr.Sell, ibkr.OrderTypeMarket)
	sellHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: futContract, Order: sellOrder})
	if err != nil {
		t.Fatalf("PlaceOrder future SELL: %v", err)
	}
	sellResult := liveObserveOrder(t, ctx, sellHandle, "fut-sell", 30*time.Second)
	if !sellResult.filled {
		t.Logf("future SELL did not fill; last status: %s", sellResult.lastStatus)
	}
}

func TestLiveForexLimitRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, eurusdContract, decimal.RequireFromString("1.10"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.Quantity = decimal.NewFromInt(20000)
	order.LmtPrice = anchor.Mul(decimal.RequireFromString("0.90")).Round(5)

	livePlaceAndCancel(t, ctx, client, eurusdContract, order)
}

func TestLiveComboVerticalRestCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 45*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))
	lower, upper := liveQualifyVerticalLegs(t, ctx, client, anchor)

	bag := ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeCombo,
		Exchange: "SMART",
		Currency: "USD",
	}

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = decimal.RequireFromString("0.05")
	order.ComboLegs = []ibkr.ComboLeg{
		{ConID: lower.ConID, Ratio: 1, Action: "BUY", Exchange: "SMART"},
		{ConID: upper.ConID, Ratio: 1, Action: "SELL", Exchange: "SMART"},
	}
	order.SmartComboRoutingParams = []ibkr.TagValue{{Tag: "NonGuaranteed", Value: "1"}}

	livePlaceAndCancel(t, ctx, client, bag, order)
}

// ---------------------------------------------------------------------------
// Tier 8: Advanced Features
// ---------------------------------------------------------------------------

func TestLiveOrderWhatIf(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)
	order.Quantity = decimal.NewFromInt(100)
	order.WhatIf = new(true)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder WhatIf: %v", err)
	}

	result := liveObserveOrder(t, ctx, handle, "whatif", 15*time.Second)
	if result.sawExecution {
		t.Error("WhatIf order should never produce an Execution")
	}

	// Check for commission data in OpenOrder events.
	var sawCommissionData bool
	for _, evt := range result.events {
		if evt.OpenOrder != nil && (evt.OpenOrder.Commission.IsPositive() || evt.OpenOrder.MinCommission.IsPositive()) {
			sawCommissionData = true
			t.Logf("WhatIf commission data: commission=%s min=%s max=%s currency=%s",
				evt.OpenOrder.Commission, evt.OpenOrder.MinCommission, evt.OpenOrder.MaxCommission, evt.OpenOrder.CommissionCurrency)
		}
	}
	if !sawCommissionData {
		t.Log("WhatIf did not return commission preview data (may depend on account permissions)")
	}
}

func TestLiveOrderAdaptiveAlgo(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(anchor)
	order.AlgoStrategy = "Adaptive"
	order.AlgoParams = []ibkr.TagValue{{Tag: "adaptivePriority", Value: "Normal"}}

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder Adaptive: %v", err)
	}

	// Look for OpenOrder confirming algo strategy.
	var sawAlgo bool
	deadline := time.After(15 * time.Second)
	for !sawAlgo {
		select {
		case evt := <-handle.Events():
			if evt.OpenOrder != nil {
				t.Logf("Adaptive open_order: algo=%s params=%v", evt.OpenOrder.AlgoStrategy, evt.OpenOrder.AlgoParams)
				if evt.OpenOrder.AlgoStrategy == "Adaptive" {
					sawAlgo = true
				}
			}
			if evt.Status != nil && ibkr.IsTerminalOrderStatus(evt.Status.Status) {
				t.Logf("Adaptive went terminal: %s", evt.Status.Status)
				return
			}
		case <-handle.Done():
			return
		case <-deadline:
			t.Log("timeout waiting for Adaptive OpenOrder")
			sawAlgo = true
		}
	}

	if err := handle.Cancel(ctx); err != nil {
		t.Logf("Cancel Adaptive: %v", err)
	}
	select {
	case <-handle.Done():
	case <-time.After(10 * time.Second):
	}
}

func TestLiveOrderIceberg(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.Quantity = decimal.NewFromInt(10)
	order.LmtPrice = liveFarBuy(anchor)
	order.DisplaySize = 3

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderOutsideRTH(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(anchor)
	order.OutsideRTH = true

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

func TestLiveOrderConditionPrice(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(anchor)
	order.Conditions = []ibkr.OrderCondition{{
		Type:          1, // Price condition
		Conjunction:   "AND",
		ConID:         265598,
		Exchange:      "SMART",
		Operator:      1,   // <=
		Value:         "1", // Absurdly low: never triggers.
		TriggerMethod: 0,
	}}

	livePlaceAndCancel(t, ctx, client, aaplContract, order)
}

// ---------------------------------------------------------------------------
// Tier 9: Stress and Concurrent Scenarios
// ---------------------------------------------------------------------------

func TestLiveStressRapidFireTenOrders(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	const n = 10
	handles := make([]*ibkr.OrderHandle, 0, n)
	ids := make(map[int64]bool)
	for i := 0; i < n; i++ {
		order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
		order.LmtPrice = liveFarBuy(anchor).Add(decimal.NewFromInt(int64(i)))

		h, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
		if err != nil {
			t.Fatalf("PlaceOrder[%d]: %v", i, err)
		}
		if ids[h.OrderID()] {
			t.Fatalf("duplicate orderID: %d", h.OrderID())
		}
		ids[h.OrderID()] = true
		handles = append(handles, h)
	}
	t.Logf("placed %d orders with distinct IDs", len(handles))

	// Wait for initial events.
	for i, h := range handles {
		select {
		case <-h.Events():
		case <-h.Done():
		case <-time.After(15 * time.Second):
			t.Logf("timeout waiting for handle[%d] initial event", i)
		}
	}

	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}

	for i, h := range handles {
		select {
		case <-h.Done():
		case <-time.After(15 * time.Second):
			t.Fatalf("timeout waiting for handle[%d] terminal after CancelAll", i)
		}
	}
	t.Logf("all %d handles reached terminal state", len(handles))
}

func TestLiveStressConcurrentModifyCancel(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	order.LmtPrice = liveFarBuy(anchor)

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	// Wait for initial event.
	select {
	case <-handle.Events():
	case <-handle.Done():
		t.Log("order went terminal before race test")
		return
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for initial event")
	}

	// Race: modify and cancel concurrently.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = handle.Modify(ctx, ibkr.Order{
			Action: ibkr.Buy, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.NewFromInt(1), LmtPrice: liveFarBuy(anchor).Add(decimal.NewFromInt(10)),
			TIF: ibkr.TIFDay, Account: account,
		})
	}()
	go func() {
		defer wg.Done()
		_ = handle.Cancel(ctx)
	}()
	wg.Wait()

	select {
	case <-handle.Done():
		t.Log("concurrent modify/cancel: handle reached terminal (no panic/deadlock)")
	case <-time.After(15 * time.Second):
		t.Fatal("concurrent modify/cancel: timeout waiting for terminal state")
	}
}

func TestLiveOrdersWithSubscriptions(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	// Start subscriptions.
	quoteSub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: aaplContract})
	if err != nil {
		t.Logf("SubscribeQuotes: %v (continuing without)", err)
	} else {
		defer quoteSub.Close()
	}

	pnlSub, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: account})
	if err != nil {
		t.Logf("SubscribePnL: %v (continuing without)", err)
	} else {
		defer pnlSub.Close()
	}

	// Place 3 far-from-market orders while subscriptions are active.
	var handles []*ibkr.OrderHandle
	for i := 0; i < 3; i++ {
		order := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
		order.LmtPrice = liveFarBuy(anchor).Add(decimal.NewFromInt(int64(i)))

		h, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: order})
		if err != nil {
			t.Fatalf("PlaceOrder[%d]: %v", i, err)
		}
		handles = append(handles, h)
	}

	// Wait for initial events.
	for i, h := range handles {
		select {
		case <-h.Events():
		case <-h.Done():
		case <-time.After(15 * time.Second):
			t.Logf("timeout waiting for handle[%d] initial event", i)
		}
	}

	// Cancel all and verify.
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	for i, h := range handles {
		select {
		case <-h.Done():
		case <-time.After(15 * time.Second):
			t.Fatalf("timeout waiting for handle[%d] terminal", i)
		}
	}
	t.Log("orders and subscriptions coexisted successfully")
}

// ---------------------------------------------------------------------------
// Tier 10: Algorithmic Trader Simulations
// ---------------------------------------------------------------------------

func TestLiveAlgoScaleInWithStopLoss(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 90*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	// Buy 1.
	buy1 := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)
	h1, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: buy1})
	if err != nil {
		t.Fatalf("buy1: %v", err)
	}
	r1 := liveObserveOrder(t, ctx, h1, "scale-buy1", 30*time.Second)
	if !r1.filled {
		t.Logf("buy1 did not fill (market may be closed); skipping test")
		return
	}

	// Buy 1 more.
	h2, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: buy1})
	if err != nil {
		t.Fatalf("buy2: %v", err)
	}
	r2 := liveObserveOrder(t, ctx, h2, "scale-buy2", 30*time.Second)
	if !r2.filled {
		t.Fatalf("buy2 did not fill; last status: %s", r2.lastStatus)
	}

	// Verify position.
	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("Positions: %v", err)
	}
	var aaplPos decimal.Decimal
	for _, p := range positions {
		if p.Contract.Symbol == "AAPL" && p.Contract.SecType == ibkr.SecTypeStock {
			aaplPos = aaplPos.Add(p.Position)
		}
	}
	t.Logf("AAPL position after scale-in: %s", aaplPos)

	// Place protective stop-loss for entire position.
	stopOrder := ibkr.Order{
		Action: ibkr.Sell, OrderType: ibkr.OrderTypeStop,
		Quantity: decimal.NewFromInt(2), AuxPrice: liveFarBuy(anchor),
		TIF: ibkr.TIFGTC, Account: account,
	}
	stopHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: stopOrder})
	if err != nil {
		t.Fatalf("stop-loss: %v", err)
	}

	select {
	case evt := <-stopHandle.Events():
		if evt.Status != nil {
			t.Logf("stop-loss status: %s", evt.Status.Status)
		}
	case <-stopHandle.Done():
	case <-time.After(15 * time.Second):
	}

	// Cancel stop-loss and flatten.
	if err := stopHandle.Cancel(ctx); err != nil {
		t.Logf("cancel stop-loss: %v", err)
	}
	select {
	case <-stopHandle.Done():
	case <-time.After(10 * time.Second):
	}

	flatOrder := ibkr.Order{
		Action: ibkr.Sell, OrderType: ibkr.OrderTypeMarket,
		Quantity: decimal.NewFromInt(2), TIF: ibkr.TIFDay, Account: account,
	}
	flatHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: flatOrder})
	if err != nil {
		t.Fatalf("flatten: %v", err)
	}
	liveObserveOrder(t, ctx, flatHandle, "scale-flatten", 30*time.Second)
}

func TestLiveFillAndImmediateFlatten(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 60*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)

	// Buy.
	buyOrder := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)
	buyHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: buyOrder})
	if err != nil {
		t.Fatalf("buy: %v", err)
	}
	buyResult := liveObserveOrder(t, ctx, buyHandle, "round-trip-buy", 30*time.Second)
	if !buyResult.filled {
		t.Logf("buy did not fill (market may be closed); skipping")
		return
	}

	// Immediate sell.
	sellOrder := liveBaseOrder(account, ibkr.Sell, ibkr.OrderTypeMarket)
	sellHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: sellOrder})
	if err != nil {
		t.Fatalf("sell: %v", err)
	}
	sellResult := liveObserveOrder(t, ctx, sellHandle, "round-trip-sell", 30*time.Second)
	if !sellResult.filled {
		t.Fatalf("sell did not fill after buy; last status: %s", sellResult.lastStatus)
	}

	// Verify executions.
	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(executions) < 2 {
		t.Logf("expected >= 2 execution updates; got %d", len(executions))
	} else {
		t.Logf("execution updates: %d", len(executions))
	}
}

func TestLiveAlgorithmicCampaign(t *testing.T) {
	ibkrlive.RequireTrading(t)
	client, _, cancel := ibkrlive.DialContext(t, 120*time.Second)
	defer cancel()
	defer client.Close()
	liveDeferCleanup(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancelReq()

	account := liveAccount(t, client)
	anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("200"))

	// Step 1: Open subscriptions.
	quoteSub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: aaplContract})
	if err != nil {
		t.Logf("SubscribeQuotes: %v", err)
	} else {
		defer quoteSub.Close()
	}

	accountSub, err := client.Accounts().SubscribeUpdates(ctx, account)
	if err != nil {
		t.Logf("SubscribeUpdates: %v", err)
	} else {
		defer accountSub.Close()
	}

	pnlSub, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: account})
	if err != nil {
		t.Logf("SubscribePnL: %v", err)
	} else {
		defer pnlSub.Close()
	}

	// Step 2: Account summary.
	summary, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: account, Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil {
		t.Fatalf("AccountSummary: %v", err)
	}
	t.Logf("pre-trade account summary: %d values", len(summary))

	// Step 3: Two MKT buys.
	var filledBuys int
	for i := 0; i < 2; i++ {
		h, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplContract,
			Order:    liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeMarket),
		})
		if err != nil {
			t.Fatalf("buy[%d]: %v", i, err)
		}
		r := liveObserveOrder(t, ctx, h, fmt.Sprintf("campaign-buy[%d]", i), 30*time.Second)
		if r.filled {
			filledBuys++
		}
	}

	if filledBuys == 0 {
		t.Log("no buys filled (market may be closed); ending campaign early")
		return
	}

	// Step 4: Resting LMT -> modify to fill.
	restOrder := liveBaseOrder(account, ibkr.Buy, ibkr.OrderTypeLimit)
	restOrder.LmtPrice = liveFarBuy(anchor)
	restHandle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplContract, Order: restOrder})
	if err != nil {
		t.Fatalf("resting LMT: %v", err)
	}

	select {
	case <-restHandle.Events():
	case <-restHandle.Done():
	case <-time.After(15 * time.Second):
	}

	if err := restHandle.Modify(ctx, ibkr.Order{
		Action: ibkr.Buy, OrderType: ibkr.OrderTypeMarket,
		Quantity: decimal.NewFromInt(1), TIF: ibkr.TIFDay, Account: account,
	}); err != nil {
		t.Logf("modify to MKT: %v", err)
	}

	restResult := liveObserveOrder(t, ctx, restHandle, "campaign-modify-fill", 30*time.Second)
	if restResult.filled {
		filledBuys++
	}

	// Step 5: Query executions.
	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"})
	if err != nil {
		t.Logf("Executions: %v", err)
	} else {
		t.Logf("campaign executions: %d", len(executions))
	}

	// Step 6: Flatten all.
	for i := 0; i < filledBuys; i++ {
		h, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplContract,
			Order:    liveBaseOrder(account, ibkr.Sell, ibkr.OrderTypeMarket),
		})
		if err != nil {
			t.Fatalf("flatten[%d]: %v", i, err)
		}
		liveObserveOrder(t, ctx, h, fmt.Sprintf("campaign-flatten[%d]", i), 30*time.Second)
	}

	// Step 7: Completed orders.
	completed, err := client.Orders().Completed(ctx, true)
	if err != nil {
		t.Logf("CompletedOrders: %v", err)
	} else {
		t.Logf("completed orders: %d", len(completed))
	}

	// Step 8: Final positions.
	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Logf("final Positions: %v", err)
	} else {
		t.Logf("final positions: %d", len(positions))
	}
}
