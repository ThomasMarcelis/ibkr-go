package ibkr_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func TestHistoricalBarsNotFoundReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "historical_bars_error.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Duration: ibkr.Days(1), BarSize: ibkr.Bar1Hour, WhatToShow: ibkr.ShowTrades, UseRTH: true,
	})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.OpKind != ibkr.OpHistoricalBars || apiErr.Code != 200 || !strings.Contains(apiErr.Message, "No security definition") {
		t.Fatalf("History().Bars() error = %v, want typed not-found code 200", err)
	}
}

func TestQuoteStreamGenericTicksReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_genericticks.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(delayed) error = %v", err)
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract:     aaplReplayContract,
		GenericTicks: []ibkr.GenericTick{"233", "236"},
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}
	var parameters *ibkr.QuoteParameters
	var shortable *ibkr.QuoteGenericTick
	for parameters == nil || shortable == nil {
		event := waitForEvent(t, sub.Events())
		if event.Err != nil {
			t.Fatalf("quote event error = %v", event.Err)
		}
		if event.Kind != ibkr.StreamData {
			continue
		}
		if event.Value.Parameters != nil {
			parameters = event.Value.Parameters
		}
		if event.Value.GenericTick != nil && event.Value.GenericTick.TickType == 46 {
			shortable = event.Value.GenericTick
		}
	}
	if parameters.MinTick == nil || parameters.MinTick.String() != "0.01" || parameters.BBOExchange != "9c0001" {
		t.Fatalf("quote parameters = %+v, want captured minimum tick and BBO exchange", parameters)
	}
	if shortable.Value.String() != "3" {
		t.Fatalf("shortable generic tick = %+v, want value 3", shortable)
	}
	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("quote subscription Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cancellation fence error = %v", err)
	}
}

func TestQuoteStreamMultiAssetReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "quote_stream_multi_asset.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(delayed) error = %v", err)
	}
	aapl, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: aaplReplayContract}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeQuotes(AAPL) error = %v", err)
	}
	eurusd, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		Symbol: "EUR", SecType: ibkr.SecTypeForex, Exchange: "IDEALPRO", Currency: "USD",
	}}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeQuotes(EUR.USD) error = %v", err)
	}

	aaplQuote := waitForQuoteFields(t, aapl, ibkr.QuoteFieldBid|ibkr.QuoteFieldAsk|ibkr.QuoteFieldLast)
	if !aaplQuote.Bid.Equal(decimal.RequireFromString("310.45")) || !aaplQuote.Ask.Equal(decimal.RequireFromString("310.47")) || !aaplQuote.Last.Equal(decimal.RequireFromString("310.47")) {
		t.Fatalf("AAPL quote = %+v, want captured delayed bid/ask/last", aaplQuote)
	}
	eurusdQuote := waitForQuoteFields(t, eurusd, ibkr.QuoteFieldLast)
	if !eurusdQuote.Last.Equal(decimal.RequireFromString("1.1663")) {
		t.Fatalf("EUR.USD quote = %+v, want captured delayed last", eurusdQuote)
	}
	aapl.Close()
	eurusd.Close()
	if err := aapl.Wait(); err != nil {
		t.Fatalf("AAPL subscription Wait() error = %v", err)
	}
	if err := eurusd.Wait(); err != nil {
		t.Fatalf("EUR.USD subscription Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cancellation fence error = %v", err)
	}
}

func TestPnLSingleHeldPositionReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "pnl_single.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updates, err := client.Accounts().Updates(ctx, "DU9000001")
	if err != nil {
		t.Fatalf("Accounts().Updates() error = %v", err)
	}
	var held *ibkr.PortfolioUpdate
	for _, update := range updates {
		if update.Portfolio != nil && update.Portfolio.Contract.ConID == 17382246 {
			held = update.Portfolio
			break
		}
	}
	if held == nil || held.Contract.Symbol != "000660" || held.Position.String() != "4" {
		t.Fatalf("held position = %+v, want captured 000660 quantity 4", held)
	}
	sub, err := client.Accounts().SubscribePnLSingle(ctx, ibkr.PnLSingleRequest{
		Account: "DU9000001", ConID: held.Contract.ConID,
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribePnLSingle() error = %v", err)
	}
	update := waitForStreamData(t, sub.Events())
	if update.Position.String() != "4" || update.DailyPnL == nil || update.DailyPnL.String() != "-28000" ||
		update.UnrealizedPnL == nil || update.UnrealizedPnL.String() != "-2256000" ||
		update.RealizedPnL != nil || update.Value == nil || update.Value.String() != "6656000" {
		t.Fatalf("PnLSingle update = %+v, want captured presence-aware values", update)
	}
	sub.Close()
	if err := sub.Wait(); err != nil {
		t.Fatalf("PnLSingle Wait() error = %v", err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cancellation fence error = %v", err)
	}
}

func TestBracketPlacementAndGlobalCleanupReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_bracket_place_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	quote := replayDelayedAAPLQuoteAnchor(t, ctx, client)
	if !quote.Last.Equal(decimal.RequireFromString("310.65")) {
		t.Fatalf("delayed quote last = %s, want 310.65", quote.Last)
	}
	bracket, err := client.Orders().PlaceBracket(ctx, ibkr.PlaceBracketRequest{
		Contract: aaplReplayContract,
		Parent: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, Quantity: decimal.NewFromInt(1),
			LmtPrice: new(decimal.RequireFromString("15.53")), TIF: ibkr.TIFDay, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000001",
		},
		TakeProfit: ibkr.Order{
			Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeLimit, Quantity: decimal.NewFromInt(1),
			LmtPrice: new(decimal.RequireFromString("3106.5")), TIF: ibkr.TIFDay, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000002",
		},
		StopLoss: ibkr.Order{
			Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeStop, Quantity: decimal.NewFromInt(1),
			AuxPrice: new(decimal.RequireFromString("7.77")), TIF: ibkr.TIFDay, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000003",
		},
	})
	if err != nil {
		t.Fatalf("PlaceBracket() error = %v", err)
	}
	if bracket.Parent.OrderID() != 568 || bracket.TakeProfit.OrderID() != 569 || bracket.StopLoss.OrderID() != 570 {
		t.Fatalf("bracket IDs = %d/%d/%d, want 568/569/570", bracket.Parent.OrderID(), bracket.TakeProfit.OrderID(), bracket.StopLoss.OrderID())
	}
	parent := waitForOpenOrder(t, ctx, bracket.Parent)
	takeProfit := waitForOpenOrder(t, ctx, bracket.TakeProfit)
	stopLoss := waitForOpenOrder(t, ctx, bracket.StopLoss)
	if *parent.Order.ParentID != 0 || *takeProfit.Order.ParentID != 568 || *stopLoss.Order.ParentID != 568 {
		t.Fatalf("bracket parent IDs = %d/%d/%d, want 0/568/568", *parent.Order.ParentID, *takeProfit.Order.ParentID, *stopLoss.Order.ParentID)
	}
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("CancelAll() error = %v", err)
	}
	waitForOrderStatus(t, ctx, bracket.Parent, ibkr.OrderStatusCancelled)
	waitForOrderStatus(t, ctx, bracket.TakeProfit, ibkr.OrderStatusCancelled)
	waitForOrderStatus(t, ctx, bracket.StopLoss, ibkr.OrderStatusCancelled)
	open, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeClient)
	if err != nil {
		t.Fatalf("Orders().Open() error = %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("open orders after global cleanup = %d, want 0", len(open))
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cleanup fence error = %v", err)
	}
}

func TestScaleInCampaignReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_scale_in_campaign_aapl.txt", ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sessionEvents := client.SessionEvents()
	positions, err := client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 7 {
		t.Fatalf("baseline Positions() = %d, %v; want 7", len(positions), err)
	}
	baselineExecutions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001"})
	if err != nil || len(baselineExecutions.Executions) != 0 || len(baselineExecutions.CommissionAndFees) != 0 {
		t.Fatalf("baseline executions/fees = %d/%d, %v; want 0/0", len(baselineExecutions.Executions), len(baselineExecutions.CommissionAndFees), err)
	}
	accountValues, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil || len(accountValues) != 3 {
		t.Fatalf("baseline Summary() = %d, %v; want 3", len(accountValues), err)
	}
	openOrders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil || len(openOrders) != 0 {
		t.Fatalf("baseline Open() = %d, %v; want 0", len(openOrders), err)
	}
	waitForSessionReady(t, ctx, sessionEvents, 2)
	positions, err = client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 7 {
		t.Fatalf("post-reconnect Positions() = %d, %v; want 7", len(positions), err)
	}
	quote := replayDelayedAAPLQuoteAnchor(t, ctx, client)
	if !quote.Last.Equal(decimal.RequireFromString("319.45")) {
		t.Fatalf("delayed quote last = %s, want 319.45", quote.Last)
	}

	scale, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplReplayContract,
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, Quantity: decimal.NewFromInt(3),
			LmtPrice: new(decimal.RequireFromString("15.97")), TIF: ibkr.TIFDay, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000001",
			Scale: ibkr.OrderScale{
				InitialLevelSize: 1, SubsequentLevelSize: 1, PriceIncrement: decimal.RequireFromString("0.05"),
			},
		},
	})
	if err != nil {
		t.Fatalf("place scale order: %v", err)
	}
	if scale.OrderID() != 656 {
		t.Fatalf("scale order ID = %d, want 656", scale.OrderID())
	}
	scaleEcho := waitForOpenOrder(t, ctx, scale)
	if scaleEcho.Order.Scale.InitialLevelSize == nil || *scaleEcho.Order.Scale.InitialLevelSize != 1 ||
		scaleEcho.Order.Scale.SubsequentLevelSize == nil || *scaleEcho.Order.Scale.SubsequentLevelSize != 1 ||
		scaleEcho.Order.Scale.PriceIncrement == nil || !scaleEcho.Order.Scale.PriceIncrement.Equal(decimal.RequireFromString("0.05")) {
		t.Fatalf("scale echo = %+v, want initial/subsequent 1 and increment 0.05", scaleEcho.Order.Scale)
	}
	cancelAndAwaitZeroFill(t, ctx, scale)
	scale.Close()
	if err := scale.Wait(); err != nil {
		t.Fatalf("scale order Wait() error = %v", err)
	}

	waitForFill := func(name string, handle *ibkr.OrderHandle, quantity decimal.Decimal, side ibkr.ExecutionSide) {
		t.Helper()
		var status *ibkr.OrderStatusUpdate
		var execution *ibkr.Execution
		for status == nil || execution == nil {
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
			case <-ctx.Done():
				t.Fatalf("waiting for %s fill: %v", name, ctx.Err())
			}
		}
		if !status.Filled.Equal(quantity) || !status.Remaining.IsZero() {
			t.Fatalf("%s status = %+v, want terminal quantity %s", name, status, quantity)
		}
		if execution.OrderID != handle.OrderID() || execution.Side != side || !execution.Shares.Equal(quantity) {
			t.Fatalf("%s execution = %+v, want order %d side %s quantity %s", name, execution, handle.OrderID(), side, quantity)
		}
	}
	placeMarket := func(name string, action ibkr.OrderAction, quantity int64, orderRef string, orderID int64) *ibkr.OrderHandle {
		t.Helper()
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplReplayContract,
			Order: ibkr.Order{
				Action: action, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(quantity),
				TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: orderRef,
			},
		})
		if err != nil {
			t.Fatalf("%s Place() error = %v", name, err)
		}
		if handle.OrderID() != orderID {
			t.Fatalf("%s order ID = %d, want %d", name, handle.OrderID(), orderID)
		}
		return handle
	}

	firstBuy := placeMarket("first scale buy", ibkr.ActionBuy, 1, "sanitized-order-ref-0000000000000006", 657)
	waitForFill("first scale buy", firstBuy, decimal.NewFromInt(1), ibkr.ExecutionSideBought)
	secondBuy := placeMarket("second scale buy", ibkr.ActionBuy, 1, "sanitized-order-ref-0000000000000010", 658)
	waitForFill("second scale buy", secondBuy, decimal.NewFromInt(1), ibkr.ExecutionSideBought)

	stop, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplReplayContract,
		Order: ibkr.Order{
			Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeStop, Quantity: decimal.NewFromInt(2),
			AuxPrice: new(decimal.RequireFromString("15.97")), TIF: ibkr.TIFGTC, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000015",
		},
	})
	if err != nil {
		t.Fatalf("protective stop Place() error = %v", err)
	}
	if stop.OrderID() != 659 {
		t.Fatalf("protective stop ID = %d, want 659", stop.OrderID())
	}
	stopEcho := waitForOpenOrder(t, ctx, stop)
	if stopEcho.Order.OrderType != ibkr.OrderTypeStop || !stopEcho.Order.Prices.AuxPrice.Equal(decimal.RequireFromString("15.97")) {
		t.Fatalf("protective stop echo = %s @ %s", stopEcho.Order.OrderType, stopEcho.Order.Prices.AuxPrice)
	}
	cancelAndAwaitZeroFill(t, ctx, stop)
	stop.Close()
	if err := stop.Wait(); err != nil {
		t.Fatalf("protective stop Wait() error = %v", err)
	}

	flatten := placeMarket("scale flatten", ibkr.ActionSell, 2, "sanitized-order-ref-0000000000000019", 660)
	waitForFill("scale flatten", flatten, decimal.NewFromInt(2), ibkr.ExecutionSideSold)

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions() error = %v", err)
	}
	if len(executions.Executions) != 3 || len(executions.CommissionAndFees) != 3 {
		t.Fatalf("executions/fees = %d/%d, want 3/3", len(executions.Executions), len(executions.CommissionAndFees))
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() reconciliation fence error = %v", err)
	}

	for name, handle := range map[string]*ibkr.OrderHandle{
		"first buy": firstBuy, "second buy": secondBuy, "flatten": flatten,
	} {
		handle.Close()
		if err := handle.Wait(); err != nil {
			t.Fatalf("%s Wait() error = %v", name, err)
		}
	}
}

func TestComboOptionVerticalReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_combo_option_vertical_aapl.txt", ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sessionEvents := client.SessionEvents()
	positions, err := client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("baseline Positions() = %d, %v; want 8", len(positions), err)
	}
	baselineExecutions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001"})
	if err != nil || len(baselineExecutions.Executions) != 7 || len(baselineExecutions.CommissionAndFees) != 7 {
		t.Fatalf("baseline executions/fees = %d/%d, %v; want 7/7", len(baselineExecutions.Executions), len(baselineExecutions.CommissionAndFees), err)
	}
	accountValues, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil || len(accountValues) != 3 {
		t.Fatalf("baseline Summary() = %d, %v; want 3", len(accountValues), err)
	}
	openOrders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil || len(openOrders) != 0 {
		t.Fatalf("baseline Open() = %d, %v; want 0", len(openOrders), err)
	}
	waitForSessionReady(t, ctx, sessionEvents, 2)
	positions, err = client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("post-reconnect Positions() = %d, %v; want 8", len(positions), err)
	}

	quote := replayDelayedAAPLQuoteAnchor(t, ctx, client)
	if !quote.Last.Equal(decimal.RequireFromString("319.63")) {
		t.Fatalf("delayed quote last = %s, want 319.63", quote.Last)
	}
	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
	})
	if err != nil || len(params) == 0 {
		t.Fatalf("SecDefOptParams() = %d, %v; want current AAPL option chain", len(params), err)
	}
	lower := requireOneContractDetail(t, client, ctx, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeOption, Expiry: "20260831", Strike: new(decimal.NewFromInt(320)),
		Right: ibkr.RightCall, Multiplier: "100", Exchange: "SMART", Currency: "USD", TradingClass: "AAPL",
	}, 911395842)
	upper := requireOneContractDetail(t, client, ctx, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeOption, Expiry: "20260831", Strike: new(decimal.RequireFromString("322.5")),
		Right: ibkr.RightCall, Multiplier: "100", Exchange: "SMART", Currency: "USD", TradingClass: "AAPL",
	}, 911681815)

	lowerPrice := decimal.RequireFromString("0.04")
	upperPrice := decimal.RequireFromString("0.01")
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			Symbol: "AAPL", SecType: ibkr.SecTypeCombo, Exchange: "SMART", Currency: "USD",
			ComboLegs: []ibkr.ComboLeg{
				{ConID: lower.ConID, Ratio: 1, Action: ibkr.ActionBuy, Exchange: "SMART", OpenClose: ibkr.ComboLegSame},
				{ConID: upper.ConID, Ratio: 1, Action: ibkr.ActionSell, Exchange: "SMART", OpenClose: ibkr.ComboLegSame},
			},
		},
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, Quantity: decimal.NewFromInt(1),
			TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: "sanitized-order-ref-0000000000000001",
			Combo: ibkr.OrderCombo{
				LegPrices:    []*decimal.Decimal{new(lowerPrice), new(upperPrice)},
				SmartRouting: []ibkr.TagValue{{Tag: "NonGuaranteed", Value: "1"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Place(BAG) error = %v", err)
	}
	if handle.OrderID() != 668 {
		t.Fatalf("BAG order ID = %d, want 668", handle.OrderID())
	}
	echo := waitForOpenOrder(t, ctx, handle)
	if echo.State.Status != ibkr.OrderStatusPreSubmitted || len(echo.Contract.ComboLegs) != 2 ||
		echo.Contract.ComboLegs[0].ConID != lower.ConID || echo.Contract.ComboLegs[0].Action != ibkr.ActionBuy ||
		echo.Contract.ComboLegs[1].ConID != upper.ConID || echo.Contract.ComboLegs[1].Action != ibkr.ActionSell {
		t.Fatalf("BAG contract/state echo = %+v / %+v", echo.Contract.ComboLegs, echo.State)
	}
	if echo.Order.Prices.LmtPrice != nil || len(echo.Order.Combo.LegPrices) != 2 ||
		echo.Order.Combo.LegPrices[0] == nil || !echo.Order.Combo.LegPrices[0].Equal(lowerPrice) ||
		echo.Order.Combo.LegPrices[1] == nil || !echo.Order.Combo.LegPrices[1].Equal(upperPrice) ||
		len(echo.Order.Combo.SmartRouting) != 1 || echo.Order.Combo.SmartRouting[0] != (ibkr.TagValue{Tag: "NonGuaranteed", Value: "1"}) {
		t.Fatalf("BAG price/routing echo = lmt %v combo %+v", echo.Order.Prices.LmtPrice, echo.Order.Combo)
	}
	cancelAndAwaitZeroFill(t, ctx, handle)
	handle.Close()
	if err := handle.Wait(); err != nil {
		t.Fatalf("BAG Wait() error = %v", err)
	}
	openOrders, err = client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil || len(openOrders) != 0 {
		t.Fatalf("post-cancel Open() = %d, %v; want 0", len(openOrders), err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cleanup fence error = %v", err)
	}
}

func TestAlgorithmicCampaignReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_algorithmic_campaign_aapl.txt", ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sessionEvents := client.SessionEvents()
	positions, err := client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("baseline Positions() = %d, %v; want 8", len(positions), err)
	}
	baseline, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001"})
	if err != nil || len(baseline.Executions) != 14 || len(baseline.CommissionAndFees) != 14 {
		t.Fatalf("baseline executions/fees = %d/%d, %v; want 14/14", len(baseline.Executions), len(baseline.CommissionAndFees), err)
	}
	accountValues, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil || len(accountValues) != 6 {
		t.Fatalf("baseline Summary() = %d, %v; want 6", len(accountValues), err)
	}
	open, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil || len(open) != 0 {
		t.Fatalf("baseline Open() = %d, %v; want 0", len(open), err)
	}
	waitForSessionReady(t, ctx, sessionEvents, 2)
	positions, err = client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("post-baseline Positions() = %d, %v; want 8", len(positions), err)
	}
	quote := replayDelayedAAPLQuoteAnchor(t, ctx, client)
	if !quote.Last.Equal(decimal.RequireFromString("319.67")) {
		t.Fatalf("delayed quote last = %s, want 319.67", quote.Last)
	}
	accountValues, err = client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		AccountFilter: "DU9000001", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
	})
	if err != nil || len(accountValues) != 8 {
		t.Fatalf("campaign Summary() = %d, %v; want 8", len(accountValues), err)
	}
	waitForSessionReady(t, ctx, sessionEvents, 3)
	initialPositions, err := client.Accounts().Positions(ctx)
	if err != nil || len(initialPositions) != 8 {
		t.Fatalf("campaign initial Positions() = %d, %v; want 8", len(initialPositions), err)
	}
	baseline, err = client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil || len(baseline.Executions) != 14 || len(baseline.CommissionAndFees) != 14 {
		t.Fatalf("AAPL baseline executions/fees = %d/%d, %v; want 14/14", len(baseline.Executions), len(baseline.CommissionAndFees), err)
	}
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(delayed) error = %v", err)
	}

	quotes, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: aaplReplayContract}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("quote observer error = %v", err)
	}
	quotesDone := drainReplaySubscription(quotes)
	updates, err := client.Accounts().SubscribeUpdates(ctx, "DU9000001", ibkr.WithQueueSize(512), ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("account observer error = %v", err)
	}
	updatesDone := drainReplaySubscription(updates)
	pnl, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: "DU9000001"}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("PnL observer error = %v", err)
	}
	pnlDone := drainReplaySubscription(pnl)
	openOrders, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("open-order observer error = %v", err)
	}
	openOrdersDone := drainReplaySubscription(openOrders.Subscription)

	placeMarket := func(action ibkr.OrderAction, quantity int64, orderRef string, orderID int64) *ibkr.OrderHandle {
		t.Helper()
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: aaplReplayContract,
			Order: ibkr.Order{
				Action: action, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(quantity),
				TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: orderRef,
			},
		})
		if err != nil {
			t.Fatalf("Place(%s %d) error = %v", action, quantity, err)
		}
		if handle.OrderID() != orderID {
			t.Fatalf("Place(%s %d) order ID = %d, want %d", action, quantity, handle.OrderID(), orderID)
		}
		return handle
	}
	for i, orderID := range []int64{677, 678} {
		handle := placeMarket(ibkr.ActionBuy, 1, []string{
			"sanitized-order-ref-0000000000000001", "sanitized-order-ref-0000000000000005",
		}[i], orderID)
		filled, execution := waitOrderFillAndExecution(t, ctx, handle)
		if !filled || !execution {
			t.Fatalf("split buy[%d] filled/execution = %t/%t, want true/true", i, filled, execution)
		}
	}
	resting, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplReplayContract,
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit, Quantity: decimal.NewFromInt(1),
			LmtPrice: new(decimal.RequireFromString("15.98")), TIF: ibkr.TIFDay, Account: "DU9000001",
			OrderRef: "sanitized-order-ref-0000000000000009",
		},
	})
	if err != nil {
		t.Fatalf("resting Place() error = %v", err)
	}
	if resting.OrderID() != 679 {
		t.Fatalf("resting order ID = %d, want 679", resting.OrderID())
	}
	restingEcho := waitForOpenOrder(t, ctx, resting)
	if restingEcho.State.Status != ibkr.OrderStatusPreSubmitted || !restingEcho.Order.Prices.LmtPrice.Equal(decimal.RequireFromString("15.98")) {
		t.Fatalf("resting echo = %+v, want PreSubmitted limit 15.98", restingEcho)
	}
	if err := resting.Replace(ctx, ibkr.Order{
		Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(1),
		TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: "sanitized-order-ref-0000000000000012",
	}); err != nil {
		t.Fatalf("resting Replace(MKT) error = %v", err)
	}
	filled, execution := waitOrderFillAndExecution(t, ctx, resting)
	if !filled || !execution {
		t.Fatalf("modified order filled/execution = %t/%t, want true/true", filled, execution)
	}
	flatten := placeMarket(ibkr.ActionSell, 3, "sanitized-order-ref-0000000000000015", 680)
	filled, execution = waitOrderFillAndExecution(t, ctx, flatten)
	if !filled || !execution {
		t.Fatalf("flatten filled/execution = %t/%t, want true/true", filled, execution)
	}

	openOrders.Close()
	if err := <-openOrdersDone; err != nil {
		t.Fatalf("open-order observer Wait() error = %v", err)
	}
	pnl.Close()
	if err := <-pnlDone; err != nil {
		t.Fatalf("PnL observer Wait() error = %v", err)
	}
	updates.Close()
	if err := <-updatesDone; err != nil {
		t.Fatalf("account observer Wait() error = %v", err)
	}
	quotes.Close()
	if err := <-quotesDone; err != nil {
		t.Fatalf("quote observer Wait() error = %v", err)
	}

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil || len(executions.Executions) != 18 || len(executions.CommissionAndFees) != 18 {
		t.Fatalf("campaign executions/fees = %d/%d, %v; want 18/18", len(executions.Executions), len(executions.CommissionAndFees), err)
	}
	waitForSessionReady(t, ctx, sessionEvents, 4)
	positions, err = client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("final Positions() = %d, %v; want 8", len(positions), err)
	}
	if got, want := replayPositionQuantity(positions, 265598), replayPositionQuantity(initialPositions, 265598); !got.Equal(want) {
		t.Fatalf("final AAPL position = %s, want baseline %s", got, want)
	}
	completed, err := client.Orders().Completed(ctx, true)
	if err != nil || len(completed) != 25 {
		t.Fatalf("Completed(true) = %d, %v; want 25", len(completed), err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cleanup fence error = %v", err)
	}
}

func TestStopLossManagementReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_stop_loss_management_aapl.txt", ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer func() { cleanupClientHost(t, client, host) }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sessionEvents := client.SessionEvents()
	positions, err := client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("baseline Positions() = %d, %v; want 8", len(positions), err)
	}
	baseline, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001"})
	if err != nil || len(baseline.Executions) != 5 || len(baseline.CommissionAndFees) != 5 {
		t.Fatalf("baseline executions/fees = %d/%d, %v; want 5/5", len(baseline.Executions), len(baseline.CommissionAndFees), err)
	}
	accountValues, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil || len(accountValues) != 3 {
		t.Fatalf("baseline Summary() = %d, %v; want 3", len(accountValues), err)
	}
	openOrders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil || len(openOrders) != 0 {
		t.Fatalf("baseline Open() = %d, %v; want 0", len(openOrders), err)
	}

	waitForSessionReady(t, ctx, sessionEvents, 2)
	positions, err = client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("post-baseline Positions() = %d, %v; want 8", len(positions), err)
	}
	quote := replayDelayedAAPLQuoteAnchor(t, ctx, client)
	if !quote.Last.Equal(decimal.RequireFromString("319.86")) {
		t.Fatalf("delayed quote last = %s, want 319.86", quote.Last)
	}
	baseline, err = client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil || len(baseline.Executions) != 5 || len(baseline.CommissionAndFees) != 5 {
		t.Fatalf("AAPL baseline executions/fees = %d/%d, %v; want 5/5", len(baseline.Executions), len(baseline.CommissionAndFees), err)
	}

	entry, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplReplayContract,
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(1),
			TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("entry Place() error = %v", err)
	}
	if entry.OrderID() != 664 {
		t.Fatalf("entry order ID = %d, want 664", entry.OrderID())
	}
	filled, execution := waitOrderFillAndExecution(t, ctx, entry)
	if !filled || !execution {
		t.Fatalf("entry filled/execution = %t/%t, want true/true", filled, execution)
	}

	stopOrder := ibkr.Order{
		Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeStop, Quantity: decimal.NewFromInt(1),
		AuxPrice: new(decimal.RequireFromString("15.99")), TIF: ibkr.TIFDay, Account: "DU9000001",
		OrderRef: "sanitized-order-ref-0000000000000005",
	}
	stop, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: aaplReplayContract, Order: stopOrder})
	if err != nil {
		t.Fatalf("protective stop Place() error = %v", err)
	}
	if stop.OrderID() != 665 {
		t.Fatalf("protective stop order ID = %d, want 665", stop.OrderID())
	}
	stopEcho := waitForOpenOrder(t, ctx, stop)
	if stopEcho.State.Status != ibkr.OrderStatusPreSubmitted || !stopEcho.Order.Prices.AuxPrice.Equal(decimal.RequireFromString("15.99")) {
		t.Fatalf("protective stop echo = %+v, want PreSubmitted at 15.99", stopEcho)
	}

	stopOrder.AuxPrice = new(decimal.RequireFromString("16.99"))
	if err := stop.Replace(ctx, stopOrder); err != nil {
		t.Fatalf("protective stop Replace() error = %v", err)
	}
	movedEcho := waitForOpenOrder(t, ctx, stop)
	if !movedEcho.Order.Prices.AuxPrice.Equal(decimal.RequireFromString("16.99")) {
		t.Fatalf("moved protective stop aux = %s, want 16.99", movedEcho.Order.Prices.AuxPrice)
	}
	cancelAndAwaitZeroFill(t, ctx, stop)
	stop.Close()
	if err := stop.Wait(); err != nil {
		t.Fatalf("protective stop Wait() error = %v", err)
	}

	flatten, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplReplayContract,
		Order: ibkr.Order{
			Action: ibkr.ActionSell, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(1),
			TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: "sanitized-order-ref-0000000000000011",
		},
	})
	if err != nil {
		t.Fatalf("flatten Place() error = %v", err)
	}
	if flatten.OrderID() != 666 {
		t.Fatalf("flatten order ID = %d, want 666", flatten.OrderID())
	}
	filled, execution = waitOrderFillAndExecution(t, ctx, flatten)
	if !filled || !execution {
		t.Fatalf("flatten filled/execution = %t/%t, want true/true", filled, execution)
	}

	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil || len(executions.Executions) != 7 || len(executions.CommissionAndFees) != 7 {
		t.Fatalf("campaign executions/fees = %d/%d, %v; want 7/7", len(executions.Executions), len(executions.CommissionAndFees), err)
	}
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("CurrentTime() cleanup fence error = %v", err)
	}

	client.Close()
	client = dialHostClient(t, host, ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	sessionEvents = client.SessionEvents()
	openOrders, err = client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil || len(openOrders) != 0 {
		t.Fatalf("first reconciliation Open() = %d, %v; want 0", len(openOrders), err)
	}
	positions, err = client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("first reconciliation Positions() = %d, %v; want 8", len(positions), err)
	}

	waitForSessionReady(t, ctx, sessionEvents, 2)
	openOrders, err = client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil || len(openOrders) != 0 {
		t.Fatalf("final reconciliation Open() = %d, %v; want 0", len(openOrders), err)
	}
	positions, err = client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 8 {
		t.Fatalf("final reconciliation Positions() = %d, %v; want 8", len(positions), err)
	}
	executions, err = client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001"})
	if err != nil || len(executions.Executions) != 7 || len(executions.CommissionAndFees) != 7 {
		t.Fatalf("final reconciliation executions/fees = %d/%d, %v; want 7/7", len(executions.Executions), len(executions.CommissionAndFees), err)
	}
	accountValues, err = client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil || len(accountValues) != 3 {
		t.Fatalf("final reconciliation Summary() = %d, %v; want 3", len(accountValues), err)
	}
}

func TestOptionExerciseAcceptedButUnsettledReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_option_exercise_aapl.txt", ibkr.WithReconnectPolicy(ibkr.ReconnectAuto))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sessionEvents := client.SessionEvents()
	openOrders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil || len(openOrders) != 0 {
		t.Fatalf("baseline Open() = %d, %v; want 0", len(openOrders), err)
	}
	positions, err := client.Accounts().Positions(ctx)
	if err != nil || len(positions) != 7 {
		t.Fatalf("baseline Positions() = %d, %v; want 7", len(positions), err)
	}
	baselineExecutions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001"})
	if err != nil || len(baselineExecutions.Executions) != 0 || len(baselineExecutions.CommissionAndFees) != 0 {
		t.Fatalf("baseline executions/fees = %d/%d, %v; want 0/0", len(baselineExecutions.Executions), len(baselineExecutions.CommissionAndFees), err)
	}
	accountValues, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil || len(accountValues) != 6 {
		t.Fatalf("baseline Summary() = %d, %v; want 6", len(accountValues), err)
	}
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		t.Fatalf("SetType(live) error = %v", err)
	}
	_, err = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: aaplReplayContract})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != 2186 || apiErr.OpKind != ibkr.OpQuotes {
		t.Fatalf("live AAPL Quote() error = %v, want captured typed quotes code 2186", err)
	}
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(delayed) error = %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: aaplReplayContract})
	if err != nil {
		t.Fatalf("delayed AAPL Quote() error = %v", err)
	}
	if !quote.Last.Equal(decimal.RequireFromString("314.06")) {
		t.Fatalf("delayed quote last = %s, want 314.06", quote.Last)
	}

	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
	})
	if err != nil || len(params) == 0 {
		t.Fatalf("SecDefOptParams() = %d, %v; want captured AAPL parameters", len(params), err)
	}
	option := requireOneContractDetail(t, client, ctx, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeOption, Expiry: "20260828",
		Strike: new(decimal.RequireFromString("307.5")), Right: ibkr.RightCall, Multiplier: "100",
		Exchange: "SMART", Currency: "USD", TradingClass: "AAPL",
	}, 909449068)

	waitForSessionReady(t, ctx, sessionEvents, 2)
	before, err := client.Accounts().Positions(ctx)
	if err != nil || len(before) != 7 {
		t.Fatalf("pre-trade Positions() = %d, %v; want 7", len(before), err)
	}
	purchase, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: option,
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeMarket, Quantity: decimal.NewFromInt(1),
			TIF: ibkr.TIFDay, Account: "DU9000001", OrderRef: "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		t.Fatalf("option seed Place() error = %v", err)
	}
	if purchase.OrderID() != 7 {
		t.Fatalf("option seed order ID = %d, want 7", purchase.OrderID())
	}
	filled, execution := waitOrderFillAndExecution(t, ctx, purchase)
	if !filled || !execution {
		t.Fatalf("option seed filled/execution = %t/%t, want true/true", filled, execution)
	}

	waitForSessionReady(t, ctx, sessionEvents, 3)
	afterPurchase, err := client.Accounts().Positions(ctx)
	if err != nil || len(afterPurchase) != 8 {
		t.Fatalf("post-purchase Positions() = %d, %v; want 8", len(afterPurchase), err)
	}
	if got := replayPositionQuantity(afterPurchase, option.ConID).Sub(replayPositionQuantity(before, option.ConID)); !got.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("option position delta = %s, want 1", got)
	}
	if got := replayPositionQuantity(afterPurchase, 265598).Sub(replayPositionQuantity(before, 265598)); !got.IsZero() {
		t.Fatalf("AAPL stock position delta = %s before exercise, want 0", got)
	}

	exercise, err := client.Options().Exercise(ctx, ibkr.ExerciseOptionsRequest{
		Contract: option, ExerciseAction: ibkr.Exercise, ExerciseQuantity: 1, Account: "DU9000001",
	})
	if err != nil {
		t.Fatalf("Exercise() error = %v", err)
	}
	var warning *ibkr.APIError
	var status *ibkr.OrderStatusUpdate
	for warning == nil || status == nil {
		select {
		case event, ok := <-exercise.Events():
			if !ok {
				t.Fatalf("exercise events closed before accepted-but-unsettled evidence: %v", exercise.Wait())
			}
			if event.Warning != nil {
				warning = event.Warning
			}
			if event.Status != nil && event.Status.Status == ibkr.OrderStatusPreSubmitted {
				status = event.Status
			}
		case <-ctx.Done():
			t.Fatalf("waiting for accepted-but-unsettled exercise evidence: %v", ctx.Err())
		}
	}
	if warning.Code != ibkr.ErrCodeOrderTIFSetFromPreset || warning.Message != "Order TIF was set to DAY based on order preset." {
		t.Fatalf("exercise warning = %v, want exact code 10349 preset notice", warning)
	}
	if status.OrderID != 8 || ibkr.IsTerminalOrderStatus(status.Status) {
		t.Fatalf("exercise status = %+v, want non-terminal PreSubmitted order 8", status)
	}
	waitErr := exercise.Wait()
	uncertain, ok := errors.AsType[*ibkr.ExerciseUncertainError](waitErr)
	if !ok || uncertain.RequestID != exercise.RequestID() {
		t.Fatalf("exercise Wait() = %v, want request-scoped uncertain outcome after captured disconnect", waitErr)
	}
}

func TestSecurityTypeProbeMatrixReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "api_security_type_probe_matrix.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	optionParams, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
	})
	if err != nil {
		t.Fatalf("AAPL SecDefOptParams() error = %v", err)
	}
	var aaplParams ibkr.SecDefOptParams
	for _, params := range optionParams {
		if params.Exchange == "SMART" {
			aaplParams = params
			break
		}
	}
	option := requireOneContractDetail(t, client, ctx, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeOption, Expiry: "20260831", Strike: new(decimal.NewFromInt(300)),
		Right: ibkr.RightCall, Multiplier: aaplParams.Multiplier, Exchange: "SMART", Currency: "USD",
		TradingClass: aaplParams.TradingClass,
	}, 911395582)

	futures, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol: "MES", SecType: ibkr.SecTypeFuture, Exchange: "CME", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("MES Details() error = %v", err)
	}
	var future ibkr.Contract
	for _, detail := range futures {
		if detail.ConID == 793356217 {
			future = detail.Contract
			break
		}
	}
	if future.ConID == 0 {
		t.Fatalf("MES details contain no captured front future: %+v", futures)
	}
	if _, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol: "MES", FutFopExchange: future.Exchange,
		UnderlyingSecType: ibkr.SecTypeFuture, UnderlyingConID: future.ConID,
	}); err != nil {
		t.Fatalf("MES SecDefOptParams() error = %v", err)
	}
	futureOption := requireOneContractDetail(t, client, ctx, ibkr.Contract{
		Symbol: "MES", SecType: ibkr.SecTypeFutureOption, Expiry: "20260903", Strike: new(decimal.NewFromInt(6500)),
		Right: ibkr.RightCall, Multiplier: "5", Exchange: "CME", Currency: "USD", TradingClass: "MS1D",
	}, 911926174)

	probes := []struct {
		contract ibkr.Contract
		conID    ibkr.ContractID
		blocked  bool
	}{
		{contract: aaplReplayContract, conID: 265598},
		{contract: option, conID: 911395582},
		{contract: ibkr.Contract{Symbol: "MES", SecType: ibkr.SecTypeFuture, Exchange: "CME", Currency: "USD"}, conID: 793356217},
		{contract: futureOption, conID: 911926174},
		{contract: ibkr.Contract{Symbol: "EUR", SecType: ibkr.SecTypeForex, Exchange: "IDEALPRO", Currency: "USD"}, conID: 12087792},
		{contract: ibkr.Contract{Symbol: "912797", SecType: ibkr.SecTypeBond, Exchange: "SMART", Currency: "USD"}, blocked: true},
		{contract: ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeCFD, Exchange: "SMART", Currency: "USD"}, conID: 120549942},
		{contract: ibkr.Contract{Symbol: "700", SecType: ibkr.SecTypeWarrant, Exchange: "SEHK", Currency: "HKD", Expiry: "202612", Strike: new(decimal.NewFromInt(700)), Right: ibkr.RightCall}, conID: 890086191},
		{contract: ibkr.Contract{Symbol: "SPX", SecType: ibkr.SecTypeIndex, Exchange: "CBOE", Currency: "USD"}, conID: 416904},
		{contract: ibkr.Contract{Symbol: "BTC", SecType: ibkr.SecTypeCrypto, Exchange: "PAXOS", Currency: "USD"}, conID: 479624278},
		{contract: ibkr.Contract{Symbol: "VTSAX", SecType: ibkr.SecTypeFund, Exchange: "FUNDSERV", Currency: "USD"}, conID: 48013650},
		{contract: ibkr.Contract{Symbol: "912797", SecType: ibkr.SecTypeBill, Exchange: "SMART", Currency: "USD"}, blocked: true},
		{contract: ibkr.Contract{Symbol: "XAUUSD", SecType: ibkr.SecTypeCommodity, Exchange: "SMART", Currency: "USD"}, conID: 69067924},
		{contract: ibkr.Contract{Symbol: "ES", SecType: ibkr.SecTypeContFuture, Exchange: "CME", Currency: "USD"}, conID: 649180671},
	}
	for _, probe := range probes {
		details, err := client.Contracts().Details(ctx, probe.contract)
		if probe.blocked {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if !ok || apiErr.Code != 200 || apiErr.OpKind != ibkr.OpContractDetails {
				t.Fatalf("Details(%s) error = %v, want typed code 200", probe.contract.SecType, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Details(%s) error = %v", probe.contract.SecType, err)
		}
		found := false
		for _, detail := range details {
			found = found || detail.ConID == probe.conID
		}
		if !found {
			t.Fatalf("Details(%s) conIDs exclude %d: %+v", probe.contract.SecType, probe.conID, details)
		}
	}
}

var aaplReplayContract = ibkr.Contract{
	ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
}

func waitForQuoteFields(t *testing.T, sub *ibkr.Subscription[ibkr.QuoteUpdate], fields ibkr.QuoteFields) ibkr.Quote {
	t.Helper()
	for {
		event := waitForEvent(t, sub.Events())
		if event.Err != nil {
			t.Fatalf("quote event error = %v", event.Err)
		}
		if event.Kind == ibkr.StreamData && event.Value.Snapshot.Available&fields == fields {
			return event.Value.Snapshot
		}
	}
}

func requireOneContractDetail(t *testing.T, client *ibkr.Client, ctx context.Context, contract ibkr.Contract, conID ibkr.ContractID) ibkr.Contract {
	t.Helper()
	details, err := client.Contracts().Details(ctx, contract)
	if err != nil {
		t.Fatalf("Details(%s) error = %v", contract.SecType, err)
	}
	if len(details) != 1 || details[0].ConID != conID {
		t.Fatalf("Details(%s) = %+v, want one conID %d", contract.SecType, details, conID)
	}
	return details[0].Contract
}

func drainReplaySubscription[T any](sub *ibkr.Subscription[T]) <-chan error {
	done := make(chan error, 1)
	go func() {
		for range sub.Events() {
		}
		done <- sub.Wait()
	}()
	return done
}

func replayPositionQuantity(positions []ibkr.Position, conID ibkr.ContractID) decimal.Decimal {
	for _, position := range positions {
		if position.Contract.ConID == conID {
			return position.Position
		}
	}
	return decimal.Zero
}
