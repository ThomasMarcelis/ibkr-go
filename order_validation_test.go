package ibkr

import (
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
				LmtPrice:  new(decimal.RequireFromString("150")),
			},
		}
	}
	cases := []struct {
		name   string
		mutate func(*PlaceOrderRequest)
		field  string
	}{
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
				req.Order.LmtPrice = nil
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
				req.Contract.ComboLegs = []ComboLeg{{ConID: 1}, {ConID: 2}}
			},
			field: "Contract.ComboLegs",
		},
		{
			name: "security id type without value",
			mutate: func(req *PlaceOrderRequest) {
				req.Contract.SecurityID.Type = SecurityIDISIN
			},
			field: "Contract.SecurityID.Value",
		},
		{
			name: "security id value without type",
			mutate: func(req *PlaceOrderRequest) {
				req.Contract.SecurityID.Value = "US0378331005"
			},
			field: "Contract.SecurityID.Type",
		},
		{
			name: "invalid combo leg exempt code",
			mutate: func(req *PlaceOrderRequest) {
				req.Contract = Contract{SecType: SecTypeCombo, ComboLegs: []ComboLeg{
					{ConID: 1, Ratio: 1, Action: ActionBuy, Exchange: "SMART", ExemptCode: new(-2)},
					{ConID: 2, Ratio: 1, Action: ActionSell, Exchange: "SMART"},
				}}
			},
			field: "Contract.ComboLegs[0].ExemptCode",
		},
		{
			name: "parent ID exceeds wire range",
			mutate: func(req *PlaceOrderRequest) {
				req.Order.ParentID = maxWireOrderID + 1
			},
			field: "Order.ParentID",
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := valid()
			tc.mutate(&req)
			err := validateOrderRequest(req)
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
				Order:    Order{Action: ActionBuy, OrderType: OrderTypeMarket, CashQty: new(decimal.NewFromInt(1_000))},
			},
		},
		{
			name: "hedge child without quantity",
			req: PlaceOrderRequest{
				Contract: Contract{ConID: 265598},
				Order:    Order{Action: ActionSell, OrderType: OrderTypeLimit, LmtPrice: new(decimal.NewFromInt(150)), ParentID: 41, Hedge: OrderHedge{Type: HedgeDelta, Param: "0.5"}},
			},
		},
		{
			name: "bag",
			req: PlaceOrderRequest{
				Contract: Contract{Symbol: "AAPL", SecType: SecTypeCombo, ComboLegs: []ComboLeg{
					{ConID: 1, Ratio: 1, Action: ActionBuy, Exchange: "SMART"},
					{ConID: 2, Ratio: 1, Action: ActionSell, Exchange: "SMART"},
				}},
				Order: Order{
					Action: ActionBuy, OrderType: OrderTypeLimit, Quantity: decimal.NewFromInt(1), LmtPrice: new(decimal.RequireFromString("0.05")),
				},
			},
		},
		{
			name: "condition",
			req: PlaceOrderRequest{
				Contract: Contract{ConID: 265598},
				Order: Order{
					Action: ActionBuy, OrderType: OrderTypeLimit, Quantity: decimal.NewFromInt(1), LmtPrice: new(decimal.NewFromInt(150)),
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
					Quantity: decimal.NewFromInt(1), LmtPrice: new(decimal.RequireFromString("-1.25")),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateOrderRequest(tc.req); err != nil {
				t.Fatalf("validateOrderRequest() error = %v", err)
			}
		})
	}
}

func TestPrepareBracketRequestOwnsSequencing(t *testing.T) {
	t.Parallel()

	request := PlaceBracketRequest{
		Contract: Contract{ConID: 265598},
		Parent: Order{
			Action: ActionBuy, OrderType: OrderTypeMarket,
			Quantity: decimal.NewFromInt(1), Account: "DU123",
		},
		TakeProfit: Order{
			Action: ActionSell, OrderType: OrderTypeLimit,
			Quantity: decimal.NewFromInt(1), LmtPrice: new(decimal.NewFromInt(200)), Account: "DU123",
		},
		StopLoss: Order{
			Action: ActionSell, OrderType: OrderTypeStop,
			Quantity: decimal.NewFromInt(1), AuxPrice: new(decimal.NewFromInt(100)), Account: "DU123",
		},
	}
	prepared, err := prepareBracketRequest(request)
	if err != nil {
		t.Fatalf("prepareBracketRequest() error = %v", err)
	}
	if prepared.Parent.Transmit == nil || *prepared.Parent.Transmit ||
		prepared.TakeProfit.Transmit == nil || *prepared.TakeProfit.Transmit ||
		prepared.StopLoss.Transmit == nil || !*prepared.StopLoss.Transmit {
		t.Fatalf("transmit sequence = %v/%v/%v, want false/false/true", prepared.Parent.Transmit, prepared.TakeProfit.Transmit, prepared.StopLoss.Transmit)
	}
	if prepared.TakeProfit.ParentID == 0 || prepared.StopLoss.ParentID == 0 {
		t.Fatalf("child parent placeholders = %d/%d, want non-zero", prepared.TakeProfit.ParentID, prepared.StopLoss.ParentID)
	}

	request.StopLoss.Transmit = new(true)
	_, err = prepareBracketRequest(request)
	validation, ok := errors.AsType[*ValidationError](err)
	if !ok || validation.Field != "StopLoss.Transmit" {
		t.Fatalf("controlled Transmit error = %v, want StopLoss.Transmit ValidationError", err)
	}
}
