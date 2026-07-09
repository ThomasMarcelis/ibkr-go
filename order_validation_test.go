package ibkr

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestValidateOrderRequest(t *testing.T) {
	t.Parallel()

	valid := func() PlaceOrderRequest {
		return PlaceOrderRequest{
			Contract: Contract{ConID: 265598},
			Order: Order{
				Action:    ActionBuy,
				OrderType: OrderTypeLimit,
				Quantity:  decimal.NewFromInt(1),
				LmtPrice:  decimal.RequireFromString("150"),
			},
		}
	}
	cases := []struct {
		name   string
		intent orderIntent
		mutate func(*PlaceOrderRequest)
		field  string
	}{
		{
			name: "new order id",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.OrderID = 42
			},
			field: "Order.OrderID",
		},
		{
			name: "missing contract identity",
			mutate: func(req *PlaceOrderRequest) {
				req.Contract = Contract{}
			},
			field: "Contract.SecType",
		},
		{
			name: "unsupported action",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.Action = "HOLD"
			},
			field: "Order.Action",
		},
		{
			name: "missing quantity",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.Quantity = decimal.Zero
			},
			field: "Order.Quantity",
		},
		{
			name: "missing limit price",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.LmtPrice = decimal.Zero
			},
			field: "Order.LmtPrice",
		},
		{
			name: "gtd without date",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.TIF = TIFGTD
			},
			field: "Order.GoodTillDate",
		},
		{
			name: "incomplete oca",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.OCA.Group = "risk-group"
			},
			field: "Order.OCA.Type",
		},
		{
			name: "combo legs on stock",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.Combo.Legs = []ComboLeg{{ConID: 1}, {ConID: 2}}
			},
			field: "Order.Combo.Legs",
		},
		{
			name: "algo params without strategy",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.Algorithm.Params = []TagValue{{Tag: "adaptivePriority", Value: "Normal"}}
			},
			field: "Order.Algorithm.Strategy",
		},
		{
			name: "condition flags without conditions",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.Conditions.IgnoreRTH = true
			},
			field: "Order.Conditions.Values",
		},
		{
			name: "adjustment without type",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.Adjustment.TriggerPrice = decimal.NewFromInt(100)
			},
			field: "Order.Adjustment.OrderType",
		},
		{
			name: "hedge without parent",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.Hedge.Type = HedgeDelta
			},
			field: "Order.ParentID",
		},
		{
			name: "what if trade",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.WhatIf = new(true)
			},
			field: "Order.WhatIf",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := valid()
			tc.mutate(&req)
			err := validateOrderRequest(req, tc.intent)
			validation, ok := errors.AsType[*ValidationError](err)
			if !ok {
				t.Fatalf("validateOrderRequest() error = %v, want *ValidationError", err)
			}
			if validation.Field != tc.field {
				t.Fatalf("ValidationError.Field = %q, want %q", validation.Field, tc.field)
			}
		})
	}
}

func TestValidateOrderRequestAcceptsAdvancedShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		req  PlaceOrderRequest
	}{
		{
			name: "cash quantity",
			req: PlaceOrderRequest{
				Contract: Contract{ConID: 12087792},
				Order:    Order{Action: ActionBuy, OrderType: OrderTypeMarket, CashQty: decimal.NewFromInt(1_000)},
			},
		},
		{
			name: "hedge child without quantity",
			req: PlaceOrderRequest{
				Contract: Contract{ConID: 265598},
				Order:    Order{Action: ActionSell, OrderType: OrderTypeLimit, LmtPrice: decimal.NewFromInt(150), ParentID: 41, Hedge: OrderHedge{Type: HedgeDelta, Param: "0.5"}},
			},
		},
		{
			name: "bag",
			req: PlaceOrderRequest{
				Contract: Contract{Symbol: "AAPL", SecType: SecTypeCombo},
				Order: Order{
					Action: ActionBuy, OrderType: OrderTypeLimit, Quantity: decimal.NewFromInt(1), LmtPrice: decimal.RequireFromString("0.05"),
					Combo: OrderCombo{Legs: []ComboLeg{
						{ConID: 1, Ratio: 1, Action: ActionBuy, Exchange: "SMART"},
						{ConID: 2, Ratio: 1, Action: ActionSell, Exchange: "SMART"},
					}},
				},
			},
		},
		{
			name: "condition",
			req: PlaceOrderRequest{
				Contract: Contract{ConID: 265598},
				Order: Order{
					Action: ActionBuy, OrderType: OrderTypeLimit, Quantity: decimal.NewFromInt(1), LmtPrice: decimal.NewFromInt(150),
					Conditions: OrderConditions{Values: []OrderCondition{{
						Type: ConditionPrice, Conjunction: ConditionAnd, Operator: ConditionMore,
						ConID: 265598, Exchange: "SMART", Value: "200", TriggerMethod: 4,
					}}},
				},
			},
		},
		{
			name: "negative outright price",
			req: PlaceOrderRequest{
				Contract: Contract{ConID: 12345},
				Order: Order{
					Action: ActionBuy, OrderType: OrderTypeLimit,
					Quantity: decimal.NewFromInt(1), LmtPrice: decimal.RequireFromString("-1.25"),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateOrderRequest(tc.req, orderIntentPlace); err != nil {
				t.Fatalf("validateOrderRequest() error = %v", err)
			}
		})
	}
}

func TestOrdersClientValidatesBeforeUsingEngine(t *testing.T) {
	t.Parallel()

	_, err := (OrdersClient{}).Place(context.Background(), PlaceOrderRequest{
		Contract: Contract{ConID: 265598},
		Order:    Order{Action: "HOLD", OrderType: OrderTypeMarket, Quantity: decimal.NewFromInt(1)},
	})
	validation, ok := errors.AsType[*ValidationError](err)
	if !ok || validation.Field != "Order.Action" {
		t.Fatalf("Place() error = %v, want Order.Action ValidationError", err)
	}
}
