package ibkr

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
	"github.com/shopspring/decimal"
)

const (
	paperOrderPlaceCancelFixturePath  = "internal/sdkadapter/testdata/fixtures/official_sdk_paper_order_place_cancel_20260502.json"
	paperOrderModifyCancelFixturePath = "internal/sdkadapter/testdata/fixtures/official_sdk_paper_order_modify_cancel_20260502.json"
	paperOpenOrdersFixturePath        = "internal/sdkadapter/testdata/fixtures/official_sdk_paper_open_orders_place_cancel_20260502.json"
)

func TestSDKOrdersCancelPublicRouteUsesSDKCommand(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- client.Orders().Cancel(ctx, 42)
	}()

	runNextEngineCommand(t, e)
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("Orders().Cancel() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Orders().Cancel() did not return")
	}

	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandCancelOrder {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandCancelOrder)
	}
	if command.CancelOrder.OrderID != 42 {
		t.Fatalf("cancel order ID = %d, want 42", command.CancelOrder.OrderID)
	}
}

func TestSDKOrderCancellationUsesSDKCommandsAndStatusEvent(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelOrderRequest{
		OrderID:               42,
		ManualOrderCancelTime: "20260502-12:34:56",
		ExtOperator:           "operator",
		ManualOrderIndicator:  "1",
	}); err != nil {
		t.Fatalf("sendSDKContext(CancelOrder) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.GlobalCancelRequest{
		ExtOperator:          "global-operator",
		ManualOrderIndicator: "2",
	}); err != nil {
		t.Fatalf("sendSDKContext(GlobalCancel) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want 2", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandCancelOrder {
		t.Fatalf("first command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandCancelOrder)
	}
	if commands[0].CancelOrder.OrderID != 42 ||
		commands[0].CancelOrder.ManualOrderCancelTime != "20260502-12:34:56" ||
		commands[0].CancelOrder.ExtOperator != "operator" ||
		commands[0].CancelOrder.ManualOrderIndicator != "1" {
		t.Fatalf("cancel order command = %+v, want copied cancel fields", commands[0].CancelOrder)
	}
	if commands[1].Kind != sdkadapter.CommandGlobalCancel {
		t.Fatalf("second command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandGlobalCancel)
	}
	if commands[1].GlobalCancel.ExtOperator != "global-operator" ||
		commands[1].GlobalCancel.ManualOrderIndicator != "2" {
		t.Fatalf("global cancel command = %+v, want copied cancel fields", commands[1].GlobalCancel)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventOrderStatus,
		OrderStatus: sdkadapter.OrderStatusValue{
			OrderID:       42,
			Status:        "Cancelled",
			Filled:        "0",
			Remaining:     "100",
			AvgFillPrice:  "0",
			PermID:        "12345",
			ParentID:      "0",
			LastFillPrice: "0",
			ClientID:      "7",
			WhyHeld:       "",
			MktCapPrice:   "",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage() error = %v", err)
	}
	got, ok := msg.(sdkadapter.OrderStatus)
	if !ok {
		t.Fatalf("sdkEventToMessage() type = %T, want sdkadapter.OrderStatus", msg)
	}
	if got.OrderID != 42 || got.Status != "Cancelled" || got.Filled != "0" ||
		got.Remaining != "100" || got.PermID != "12345" || got.ClientID != "7" {
		t.Fatalf("order status = %+v, want copied status fields", got)
	}
}

func TestSDKPlaceOrderPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		handle *OrderHandle
		err    error
	}, 1)
	go func() {
		handle, err := client.Orders().Place(ctx, PlaceOrderRequest{
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			Order: Order{
				Action:    Buy,
				OrderType: OrderTypeLimit,
				Quantity:  decimal.NewFromInt(1),
				LmtPrice:  decimal.RequireFromString("252.06"),
				TIF:       TIFDay,
				Account:   "DU_REDACTED",
			},
		})
		resultCh <- struct {
			handle *OrderHandle
			err    error
		}{handle: handle, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandPlaceOrder {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandPlaceOrder)
	}
	if command.PlaceOrder.Contract.Symbol != "AAPL" ||
		command.PlaceOrder.Action != "BUY" ||
		command.PlaceOrder.TotalQuantity != "1" ||
		command.PlaceOrder.OrderType != "LMT" ||
		command.PlaceOrder.LmtPrice != "252.06" ||
		command.PlaceOrder.Account != "DU_REDACTED" {
		t.Fatalf("place order command = %+v, want redacted AAPL BUY LMT", command.PlaceOrder)
	}

	result := receiveOrderHandleResult(t, resultCh, "Orders().Place")
	if result.err != nil {
		t.Fatalf("Orders().Place() error = %v", result.err)
	}
	if result.handle.OrderID() != command.PlaceOrder.OrderID {
		t.Fatalf("handle orderID = %d, want command orderID %d", result.handle.OrderID(), command.PlaceOrder.OrderID)
	}
	defer func() {
		_ = result.handle.Close()
		runNextEngineCommand(t, e)
	}()

	event := fixtureEvent(t, paperOrderPlaceCancelFixturePath, sdkadapter.EventOpenOrder, 0)
	event.OpenOrder.OrderID = command.PlaceOrder.OrderID
	dispatchSDKFixtureEvent(t, e, event)

	select {
	case update := <-result.handle.Events():
		if update.OpenOrder == nil ||
			update.OpenOrder.OrderID != command.PlaceOrder.OrderID ||
			update.OpenOrder.Account != "DU_REDACTED" ||
			update.OpenOrder.Contract.Symbol != "AAPL" ||
			update.OpenOrder.Action != Buy ||
			update.OpenOrder.OrderType != OrderTypeLimit ||
			update.OpenOrder.Status != OrderStatusPreSubmitted {
			t.Fatalf("order handle event = %+v, want replayed redacted AAPL open order", update)
		}
	case <-time.After(time.Second):
		t.Fatal("order handle did not receive replayed open order")
	}
}

func TestSDKOrderHandleModifyPublicRouteUsesSDKCommand(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		handle *OrderHandle
		err    error
	}, 1)
	go func() {
		handle, err := client.Orders().Place(ctx, PlaceOrderRequest{
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			Order: Order{
				Action:    Buy,
				OrderType: OrderTypeLimit,
				Quantity:  decimal.NewFromInt(1),
				LmtPrice:  decimal.RequireFromString("252.06"),
				TIF:       TIFDay,
				Account:   "DU_REDACTED",
			},
		})
		resultCh <- struct {
			handle *OrderHandle
			err    error
		}{handle: handle, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	result := receiveOrderHandleResult(t, resultCh, "Orders().Place")
	if result.err != nil {
		t.Fatalf("Orders().Place() error = %v", result.err)
	}
	defer func() {
		_ = result.handle.Close()
		runNextEngineCommand(t, e)
	}()

	modifyCh := make(chan error, 1)
	go func() {
		modifyCh <- result.handle.Modify(ctx, Order{
			Action:    Buy,
			OrderType: OrderTypeLimit,
			Quantity:  decimal.NewFromInt(2),
			LmtPrice:  decimal.RequireFromString("252.06"),
			TIF:       TIFDay,
			Account:   "DU_REDACTED",
		})
	}()
	runNextEngineCommand(t, e)
	select {
	case err := <-modifyCh:
		if err != nil {
			t.Fatalf("OrderHandle.Modify() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("OrderHandle.Modify() did not return")
	}

	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want place and modify: %+v", len(commands), commands)
	}
	modify := commands[1]
	if modify.Kind != sdkadapter.CommandPlaceOrder ||
		modify.PlaceOrder.OrderID != command.PlaceOrder.OrderID ||
		modify.PlaceOrder.TotalQuantity != "2" ||
		modify.PlaceOrder.Contract.Symbol != "AAPL" {
		t.Fatalf("modify command = %+v, want same orderID quantity=2 AAPL placeOrder", modify)
	}

	event := fixtureOpenOrderEventWithQuantity(t, paperOrderModifyCancelFixturePath, "+2E+0")
	event.OpenOrder.OrderID = command.PlaceOrder.OrderID
	dispatchSDKFixtureEvent(t, e, event)

	select {
	case update := <-result.handle.Events():
		if update.OpenOrder == nil ||
			update.OpenOrder.OrderID != command.PlaceOrder.OrderID ||
			update.OpenOrder.Quantity.String() != "2" {
			t.Fatalf("modified open-order event = %+v, want replayed quantity=2 update", update)
		}
	case <-time.After(time.Second):
		t.Fatal("order handle did not receive replayed modified open order")
	}
}

func TestSDKOpenOrdersPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		orders []OpenOrder
		err    error
	}, 1)
	go func() {
		orders, err := client.Orders().Open(ctx, OpenOrdersScopeClient)
		resultCh <- struct {
			orders []OpenOrder
			err    error
		}{orders: orders, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandOpenOrders {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandOpenOrders)
	}
	if command.OpenOrders.Scope != "client" {
		t.Fatalf("open orders scope = %q, want client", command.OpenOrders.Scope)
	}

	dispatchSDKFixtureEvent(t, e, fixtureEvent(t, paperOpenOrdersFixturePath, sdkadapter.EventOpenOrder, 0))
	dispatchSDKFixtureEvent(t, e, fixtureEvent(t, paperOpenOrdersFixturePath, sdkadapter.EventOpenOrderEnd, 0))

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Orders().Open() error = %v", result.err)
		}
		if len(result.orders) != 1 {
			t.Fatalf("Orders().Open() len = %d, want 1 replayed order", len(result.orders))
		}
		order := result.orders[0]
		if order.Account != "DU_REDACTED" ||
			order.Action != Buy ||
			order.OrderType != OrderTypeLimit ||
			order.Contract.Symbol != "AAPL" ||
			order.Status != "PreSubmitted" {
			t.Fatalf("Orders().Open() order = %+v, want redacted captured AAPL LMT order", order)
		}
	case <-time.After(time.Second):
		t.Fatal("Orders().Open() did not return")
	}
}

func TestSDKSubscribeOpenOrdersPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		sub *Subscription[OpenOrderUpdate]
		err error
	}, 1)
	go func() {
		sub, err := client.Orders().SubscribeOpen(ctx, OpenOrdersScopeClient)
		resultCh <- struct {
			sub *Subscription[OpenOrderUpdate]
			err error
		}{sub: sub, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandOpenOrders ||
		command.OpenOrders.Scope != "client" {
		t.Fatalf("open-orders command = %+v, want client scope", command)
	}
	result := receiveOpenOrdersSubscriptionResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Orders().SubscribeOpen() error = %v", result.err)
	}

	dispatchSDKFixtureEvent(t, e, fixtureEvent(t, paperOpenOrdersFixturePath, sdkadapter.EventOpenOrder, 0))
	select {
	case update := <-result.sub.Events():
		if update.Order.Account != "DU_REDACTED" ||
			update.Order.Contract.Symbol != "AAPL" ||
			update.Order.Status != OrderStatusPreSubmitted {
			t.Fatalf("SubscribeOpen() event = %+v, want redacted AAPL open order", update)
		}
	case <-time.After(time.Second):
		t.Fatal("Orders().SubscribeOpen() did not emit replayed open order")
	}

	dispatchSDKFixtureEvent(t, e, fixtureEvent(t, paperOpenOrdersFixturePath, sdkadapter.EventOpenOrderEnd, 0))
	if err := result.sub.AwaitSnapshot(ctx); err != nil {
		t.Fatalf("Orders().SubscribeOpen().AwaitSnapshot() error = %v", err)
	}
	if err := result.sub.Close(); err != nil {
		t.Fatalf("Subscription.Close() error = %v", err)
	}
	runNextEngineCommand(t, e)
	if commands := adapter.Commands(); len(commands) != 1 {
		t.Fatalf("commands len after close = %d, want only snapshot request: %+v", len(commands), commands)
	}
}

func TestSDKPlaceOrderUsesSDKCommand(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	req := sdkadapter.PlaceOrderRequest{
		OrderID:       42,
		Contract:      sdkadapter.Contract{ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
		Action:        "BUY",
		TotalQuantity: "100",
		OrderType:     "LMT",
		LmtPrice:      "190.5",
		TIF:           "DAY",
		Account:       "DU123",
		Transmit:      "1",
		ParentID:      "0",
		OutsideRTH:    "1",
		ComboLegs:     []sdkadapter.ComboLeg{{ConID: 1, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"}},
		AlgoStrategy:  "Adaptive",
		AlgoParams:    []sdkadapter.TagValue{{Tag: "adaptivePriority", Value: "Normal"}},
		Conditions:    []sdkadapter.OrderCondition{{Type: 1, Conjunction: "a", ConID: 265598, Exchange: "SMART", Operator: 2, Value: "200", TriggerMethod: 4}},
	}
	if err := e.sendSDKContext(context.Background(), req); err != nil {
		t.Fatalf("sendSDKContext(PlaceOrder) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandPlaceOrder {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandPlaceOrder)
	}
	got := commands[0].PlaceOrder
	if got.OrderID != 42 || got.Contract.Symbol != "AAPL" || got.Action != "BUY" ||
		got.TotalQuantity != "100" || got.LmtPrice != "190.5" || got.AlgoStrategy != "Adaptive" {
		t.Fatalf("place order command = %+v, want copied core fields", got)
	}
	if len(got.ComboLegs) != 1 || got.ComboLegs[0].ConID != 1 {
		t.Fatalf("combo legs = %+v, want copied combo leg", got.ComboLegs)
	}
	if len(got.AlgoParams) != 1 || got.AlgoParams[0].Value != "Normal" {
		t.Fatalf("algo params = %+v, want copied algo params", got.AlgoParams)
	}
	if len(got.Conditions) != 1 || got.Conditions[0].Value != "200" {
		t.Fatalf("conditions = %+v, want copied conditions", got.Conditions)
	}
}

func TestSDKOpenOrdersUseSDKCommandAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.OpenOrdersRequest{Scope: "all"}); err != nil {
		t.Fatalf("sendSDKContext(OpenOrders) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandOpenOrders {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandOpenOrders)
	}
	if commands[0].OpenOrders.Scope != "all" {
		t.Fatalf("open orders scope = %q, want all", commands[0].OpenOrders.Scope)
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventOpenOrder,
		OpenOrder: sdkadapter.OpenOrder{
			OrderID: 42,
			Contract: sdkadapter.Contract{
				ConID:    265598,
				Symbol:   "AAPL",
				SecType:  "STK",
				Exchange: "SMART",
				Currency: "USD",
			},
			Action:                "BUY",
			Quantity:              "100",
			OrderType:             "LMT",
			LmtPrice:              "190.5",
			TIF:                   "DAY",
			Account:               "DU123",
			Status:                "Submitted",
			Filled:                "0",
			Remaining:             "100",
			ParentID:              "0",
			ComboLegs:             []sdkadapter.ComboLeg{{ConID: 1, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"}},
			OrderComboLegPrices:   []string{"1.23"},
			SmartComboRouting:     []sdkadapter.TagValue{{Tag: "NonGuaranteed", Value: "1"}},
			AlgoStrategy:          "Adaptive",
			AlgoParams:            []sdkadapter.TagValue{{Tag: "adaptivePriority", Value: "Normal"}},
			Conditions:            []sdkadapter.OrderCondition{{Type: 1, Conjunction: "a", ConID: 265598, Exchange: "SMART", Operator: 2, Value: "200", TriggerMethod: 4}},
			ConditionsIgnoreRTH:   "1",
			ConditionsCancelOrder: "0",
			Commission:            "1.23",
			CommissionCurrency:    "USD",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(open order) error = %v", err)
	}
	got, ok := msg.(sdkadapter.OpenOrder)
	if !ok {
		t.Fatalf("open order type = %T, want sdkadapter.OpenOrder", msg)
	}
	if got.OrderID != 42 || got.Contract.Symbol != "AAPL" || got.Quantity != "100" ||
		got.OrderType != "LMT" || got.Status != "Submitted" || got.Commission != "1.23" {
		t.Fatalf("open order = %+v, want copied order fields", got)
	}
	if len(got.ComboLegs) != 1 || got.ComboLegs[0].ConID != 1 {
		t.Fatalf("combo legs = %+v, want copied combo leg", got.ComboLegs)
	}
	if len(got.AlgoParams) != 1 || got.AlgoParams[0].Tag != "adaptivePriority" {
		t.Fatalf("algo params = %+v, want copied params", got.AlgoParams)
	}
	if len(got.Conditions) != 1 || got.Conditions[0].TriggerMethod != 4 {
		t.Fatalf("conditions = %+v, want copied condition", got.Conditions)
	}

	end, err := sdkEventToMessage(sdkadapter.Event{Kind: sdkadapter.EventOpenOrderEnd})
	if err != nil {
		t.Fatalf("sdkEventToMessage(open order end) error = %v", err)
	}
	if _, ok := end.(sdkadapter.OpenOrderEnd); !ok {
		t.Fatalf("open order end type = %T, want sdkadapter.OpenOrderEnd", end)
	}
}

func TestSDKCompletedOrdersUseSDKCommandAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CompletedOrdersRequest{APIOnly: true}); err != nil {
		t.Fatalf("sendSDKContext(CompletedOrders) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandCompletedOrders {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandCompletedOrders)
	}
	if !commands[0].CompletedOrders.APIOnly {
		t.Fatalf("completed orders apiOnly = false, want true")
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventCompletedOrder,
		CompletedOrder: sdkadapter.CompletedOrder{
			Contract:  sdkadapter.Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
			Action:    "BUY",
			OrderType: "MKT",
			Status:    "Filled",
			Quantity:  "5",
			Filled:    "5",
			Remaining: "0",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(completed order) error = %v", err)
	}
	got, ok := msg.(sdkadapter.CompletedOrder)
	if !ok {
		t.Fatalf("completed order type = %T, want sdkadapter.CompletedOrder", msg)
	}
	if got.Contract.Symbol != "AAPL" || got.Action != "BUY" || got.Status != "Filled" ||
		got.Quantity != "5" || got.Filled != "5" || got.Remaining != "0" {
		t.Fatalf("completed order = %+v, want copied fields", got)
	}

	end, err := sdkEventToMessage(sdkadapter.Event{Kind: sdkadapter.EventCompletedOrderEnd})
	if err != nil {
		t.Fatalf("sdkEventToMessage(completed order end) error = %v", err)
	}
	if _, ok := end.(sdkadapter.CompletedOrderEnd); !ok {
		t.Fatalf("completed order end type = %T, want sdkadapter.CompletedOrderEnd", end)
	}
}

func TestSDKExecutionsUseSDKCommandAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.ExecutionsRequest{
		ReqID:   55,
		Account: "DU123",
		Symbol:  "AAPL",
	}); err != nil {
		t.Fatalf("sendSDKContext(Executions) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandExecutions {
		t.Fatalf("command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandExecutions)
	}
	if commands[0].Executions.ReqID != 55 || commands[0].Executions.Account != "DU123" || commands[0].Executions.Symbol != "AAPL" {
		t.Fatalf("executions command = %+v, want reqID/account/symbol", commands[0].Executions)
	}

	detail, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventExecutionDetail,
		ReqID: 55,
		ExecutionDetail: sdkadapter.ExecutionDetailValue{
			OrderID: 42,
			ExecID:  "exec-1",
			Account: "DU123",
			Symbol:  "AAPL",
			Side:    "BOT",
			Shares:  "100",
			Price:   "190.5",
			Time:    "20260413 13:35:50 US/Eastern",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(execution detail) error = %v", err)
	}
	gotDetail, ok := detail.(sdkadapter.ExecutionDetail)
	if !ok {
		t.Fatalf("execution detail type = %T, want sdkadapter.ExecutionDetail", detail)
	}
	if gotDetail.ReqID != 55 || gotDetail.OrderID != 42 || gotDetail.ExecID != "exec-1" ||
		gotDetail.Account != "DU123" || gotDetail.Symbol != "AAPL" || gotDetail.Shares != "100" {
		t.Fatalf("execution detail = %+v, want copied fields", gotDetail)
	}

	end, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventExecutionsEnd,
		ReqID: 55,
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(executions end) error = %v", err)
	}
	gotEnd, ok := end.(sdkadapter.ExecutionsEnd)
	if !ok {
		t.Fatalf("executions end type = %T, want sdkadapter.ExecutionsEnd", end)
	}
	if gotEnd.ReqID != 55 {
		t.Fatalf("executions end reqID = %d, want 55", gotEnd.ReqID)
	}

	commission, err := sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventCommissionReport,
		CommissionReport: sdkadapter.CommissionReportValue{
			ExecID:      "exec-1",
			Commission:  "1.23",
			Currency:    "USD",
			RealizedPNL: "4.56",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(commission) error = %v", err)
	}
	gotCommission, ok := commission.(sdkadapter.CommissionReport)
	if !ok {
		t.Fatalf("commission type = %T, want sdkadapter.CommissionReport", commission)
	}
	if gotCommission.ExecID != "exec-1" || gotCommission.Commission != "1.23" ||
		gotCommission.Currency != "USD" || gotCommission.RealizedPNL != "4.56" {
		t.Fatalf("commission = %+v, want copied fields", gotCommission)
	}
}

func TestSDKExecutionsPublicRouteReplaysEmptyOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		executions []ExecutionUpdate
		err        error
	}, 1)
	go func() {
		executions, err := client.Orders().Executions(ctx, ExecutionsRequest{Symbol: "IBKR_GO_NO_SUCH_SYMBOL"})
		resultCh <- struct {
			executions []ExecutionUpdate
			err        error
		}{executions: executions, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandExecutions {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandExecutions)
	}
	if command.Executions.Symbol != "IBKR_GO_NO_SUCH_SYMBOL" {
		t.Fatalf("executions symbol = %q, want impossible filter", command.Executions.Symbol)
	}

	event := fixtureEvent(t, "internal/sdkadapter/testdata/fixtures/official_sdk_executions_empty_filter_20260502.json", sdkadapter.EventExecutionsEnd, 1101)
	event.ReqID = command.Executions.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Orders().Executions() error = %v", result.err)
		}
		if len(result.executions) != 0 {
			t.Fatalf("Orders().Executions() len = %d, want empty replay", len(result.executions))
		}
	case <-time.After(time.Second):
		t.Fatal("Orders().Executions() did not return")
	}
}

func TestSDKOrderWarningAPIErrorsDoNotCloseHandle(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(42)
	e := &engine{
		events: newObserver[Event](8),
		orders: map[int64]*orderRoute{
			42: {orderID: 42, handle: handle},
		},
	}
	e.handleAPIError(sdkadapter.APIError{
		ReqID:   42,
		Code:    399,
		Message: "Order Message:\nWarning: Your order will not be placed at the exchange until 2026-05-04 09:30:00 US/Eastern.",
	})

	select {
	case <-handle.Done():
		t.Fatalf("order warning closed handle with err = %v", handle.Wait())
	default:
	}
}

func TestSDKOrderRejectionAPIErrorsCloseHandle(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(42)
	e := &engine{
		events: newObserver[Event](8),
		orders: map[int64]*orderRoute{
			42: {orderID: 42, handle: handle},
		},
	}
	e.handleAPIError(sdkadapter.APIError{
		ReqID:   42,
		Code:    201,
		Message: "Order rejected",
	})

	select {
	case <-handle.Done():
	default:
		t.Fatal("order rejection did not close handle")
	}
	err := handle.Wait()
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("handle Wait() error = %T %v, want *APIError", err, err)
	}
	if apiErr.Code != 201 || apiErr.OpKind != OpPlaceOrder {
		t.Fatalf("APIError = %+v, want place-order code 201", apiErr)
	}
}

func TestSDKOrderLateAPIErrorsAfterCancelledStatusDoNotCloseHandle(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(42)
	e := &engine{
		events: newObserver[Event](8),
		orders: map[int64]*orderRoute{
			42: {orderID: 42, handle: handle, terminalStatus: OrderStatusCancelled},
		},
	}
	e.handleAPIError(sdkadapter.APIError{
		ReqID:   42,
		Code:    201,
		Message: "Order rejected - reason:Order has been cancelled already, too late to replace",
	})

	select {
	case <-handle.Done():
		t.Fatalf("late post-cancel API error closed handle with err = %v", handle.Wait())
	default:
	}
}

func TestSDKOrderTooLateToReplaceNoticeDoesNotCloseHandle(t *testing.T) {
	t.Parallel()

	handle := newOrderHandle(42)
	e := &engine{
		events: newObserver[Event](8),
		orders: map[int64]*orderRoute{
			42: {orderID: 42, handle: handle},
		},
	}
	e.handleAPIError(sdkadapter.APIError{
		ReqID:   42,
		Code:    201,
		Message: "Order rejected - reason:Order has been cancelled already, too late to replace",
	})

	select {
	case <-handle.Done():
		t.Fatalf("too-late-to-replace notice closed handle with err = %v", handle.Wait())
	default:
	}
}

func receiveOrderHandleResult(t *testing.T, resultCh <-chan struct {
	handle *OrderHandle
	err    error
}, name string) struct {
	handle *OrderHandle
	err    error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatalf("%s did not return", name)
		return struct {
			handle *OrderHandle
			err    error
		}{}
	}
}

func receiveOpenOrdersSubscriptionResult(t *testing.T, resultCh <-chan struct {
	sub *Subscription[OpenOrderUpdate]
	err error
}) struct {
	sub *Subscription[OpenOrderUpdate]
	err error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("Orders().SubscribeOpen() did not return")
		return struct {
			sub *Subscription[OpenOrderUpdate]
			err error
		}{}
	}
}

func fixtureOpenOrderEventWithQuantity(t *testing.T, path string, quantity string) sdkadapter.Event {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	fixture, err := sdkadapter.DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	for _, event := range fixture.Events {
		if event.Kind == sdkadapter.EventOpenOrder && event.OpenOrder.Quantity == quantity {
			return event
		}
	}
	t.Fatalf("fixture open_order quantity=%s not found", quantity)
	return sdkadapter.Event{}
}
