package ibkr

import (
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func TestToCodecPlaceOrderMapsIncludeOvernight(t *testing.T) {
	t.Parallel()

	request := PlaceOrderRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order: Order{
			Action: ActionBuy, OrderType: OrderTypeLimit, Quantity: decimal.NewFromInt(1),
			LmtPrice: new(decimal.NewFromInt(50)), TIF: TIFDay,
		},
	}
	if got := toCodecPlaceOrder(78, request).IncludeOvernight; got != "" {
		t.Fatalf("default include overnight = %q, want empty", got)
	}
	request.Order.IncludeOvernight = new(true)
	if got := toCodecPlaceOrder(78, request).IncludeOvernight; got != "1" {
		t.Fatalf("enabled include overnight = %q, want 1", got)
	}
	request.Order.IncludeOvernight = new(false)
	if got := toCodecPlaceOrder(78, request).IncludeOvernight; got != "0" {
		t.Fatalf("disabled include overnight = %q, want 0", got)
	}
}

func TestFromCodecOrderDetailsProjectsIncludeOvernightPresence(t *testing.T) {
	t.Parallel()

	// OpenOrder and CompletedOrder share this projection. Callback-specific
	// presence is frozen separately against the live-derived replay fixtures.
	for _, tc := range []struct {
		name  string
		wire  string
		want  bool
		isNil bool
	}{
		{name: "absent", isNil: true},
		{name: "disabled", wire: "0", want: false},
		{name: "enabled", wire: "1", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			order, err := fromCodecCompletedOrder(codec.CompletedOrder{OrderDetails: codec.OrderDetails{
				Contract: codec.Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK", Strike: "0", Exchange: "SMART", Currency: "USD",
				},
				Quantity: "1", Filled: "0", IncludeOvernight: tc.wire,
			}})
			if err != nil {
				t.Fatal(err)
			}
			got := order.Order.IncludeOvernight
			if tc.isNil {
				if got != nil {
					t.Fatalf("include overnight = %v, want nil", got)
				}
				return
			}
			if got == nil || *got != tc.want {
				t.Fatalf("include overnight = %v, want %t", got, tc.want)
			}
		})
	}
}

func TestFromCodecOrderDetailsRejectsMalformedIncludeOvernight(t *testing.T) {
	t.Parallel()

	_, err := fromCodecCompletedOrder(codec.CompletedOrder{OrderDetails: codec.OrderDetails{
		Contract: codec.Contract{
			ConID: 265598, Symbol: "AAPL", SecType: "STK", Strike: "0", Exchange: "SMART", Currency: "USD",
		},
		Quantity: "1", Filled: "0", IncludeOvernight: "2",
	}})
	if err == nil || !strings.Contains(err.Error(), "include overnight") {
		t.Fatalf("fromCodecCompletedOrder() error = %v, want include overnight error", err)
	}
}

// Live paper sv225 (2026-08-29) rejects an omitted TIF with code 10052
// "Invalid time in force:Empty", so the client sends DAY unless the caller
// chose otherwise. Capture 20260829T142218Z-api_empty_tif_default_aapl (events SHA-256
// a6d1543d90a099e14011adc9a2e4fac984d7ac4f512a49ec391e86a0aab5859b) records
// the rejection; the promoted replay records the DAY echo after this change.
func TestToCodecPlaceOrderSendsDayWhenTIFEmpty(t *testing.T) {
	t.Parallel()

	contract := Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"}
	if got := toCodecPlaceOrder(77, PlaceOrderRequest{Contract: contract, Order: LimitOrder(ActionBuy, decimal.NewFromInt(1), decimal.NewFromInt(150))}).TIF; got != "DAY" {
		t.Fatalf("empty TIF encoded as %q, want DAY", got)
	}
	gtc := LimitOrder(ActionBuy, decimal.NewFromInt(1), decimal.NewFromInt(150))
	gtc.TIF = TIFGTC
	if got := toCodecPlaceOrder(77, PlaceOrderRequest{Contract: contract, Order: gtc}).TIF; got != "GTC" {
		t.Fatalf("explicit TIF encoded as %q, want GTC", got)
	}
}

func TestToCodecPlaceOrderMapsAdvancedOrderFields(t *testing.T) {
	t.Parallel()

	got := toCodecPlaceOrder(77, PlaceOrderRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order: Order{
			Action:          ActionBuy,
			OrderType:       OrderTypeTrailingLimit,
			Quantity:        decimal.RequireFromString("10"),
			LmtPrice:        new(decimal.RequireFromString("200.25")),
			AuxPrice:        new(decimal.RequireFromString("1.25")),
			TIF:             TIFGTC,
			Account:         "DU123",
			OCA:             OrderOCA{Group: "oca-test", Type: OCACancelWithBlock},
			TriggerMethod:   4,
			DisplaySize:     3,
			AllOrNone:       new(true),
			MinQty:          new(2),
			PercentOffset:   new(decimal.RequireFromString("0.05")),
			TrailStopPrice:  new(decimal.RequireFromString("190.50")),
			TrailingPercent: new(decimal.RequireFromString("1.5")),
			Scale: OrderScale{
				InitialLevelSize:    2,
				SubsequentLevelSize: 1,
				PriceIncrement:      decimal.RequireFromString("0.10"),
				Table:               "1:1",
				ActiveStartTime:     "20260413 09:30:00 US/Eastern",
				ActiveStopTime:      "20260413 16:00:00 US/Eastern",
			},
			Hedge: OrderHedge{
				Type:                  HedgeFX,
				Param:                 "BUY EUR.USD",
				DisableAutomaticPrice: new(true),
			},
			LmtPriceOffset: new(decimal.RequireFromString("0.02")),
			Adjustment: OrderAdjustment{
				OrderType:      OrderTypeStop,
				TriggerPrice:   decimal.RequireFromString("198"),
				StopPrice:      decimal.RequireFromString("195"),
				StopLimitPrice: decimal.RequireFromString("194.5"),
				TrailingAmount: decimal.RequireFromString("1"),
				TrailingUnit:   1,
			},
			CashQty:               new(decimal.RequireFromString("1000")),
			UsePriceMgmtAlgo:      new(false),
			AdvancedErrorOverride: "IBDBUYTX",
			ManualOrderTime:       "20260413 15:00:00 US/Eastern",
		},
	})

	if got.OrderType != "TRAIL LIMIT" {
		t.Fatalf("OrderType = %q, want TRAIL LIMIT", got.OrderType)
	}
	checks := map[string]string{
		"OcaType":                  got.OcaType,
		"TriggerMethod":            got.TriggerMethod,
		"DisplaySize":              got.DisplaySize,
		"AllOrNone":                got.AllOrNone,
		"MinQty":                   got.MinQty,
		"PercentOffset":            got.PercentOffset,
		"TrailStopPrice":           got.TrailStopPrice,
		"TrailingPercent":          got.TrailingPercent,
		"ScaleInitLevelSize":       got.ScaleInitLevelSize,
		"ScaleSubsLevelSize":       got.ScaleSubsLevelSize,
		"ScalePriceIncrement":      got.ScalePriceIncrement,
		"ScaleTable":               got.ScaleTable,
		"ActiveStartTime":          got.ActiveStartTime,
		"ActiveStopTime":           got.ActiveStopTime,
		"HedgeType":                got.HedgeType,
		"HedgeParam":               got.HedgeParam,
		"AdjustedOrderType":        got.AdjustedOrderType,
		"TriggerPrice":             got.TriggerPrice,
		"LmtPriceOffset":           got.LmtPriceOffset,
		"AdjustedStopPrice":        got.AdjustedStopPrice,
		"AdjustedStopLimitPrice":   got.AdjustedStopLimitPrice,
		"AdjustedTrailingAmount":   got.AdjustedTrailingAmount,
		"AdjustableTrailingUnit":   got.AdjustableTrailingUnit,
		"CashQty":                  got.CashQty,
		"DontUseAutoPriceForHedge": got.DontUseAutoPriceForHedge,
		"UsePriceMgmtAlgo":         got.UsePriceMgmtAlgo,
		"AdvancedErrorOverride":    got.AdvancedErrorOverride,
		"ManualOrderTime":          got.ManualOrderTime,
	}
	want := map[string]string{
		"OcaType":                  "1",
		"TriggerMethod":            "4",
		"DisplaySize":              "3",
		"AllOrNone":                "1",
		"MinQty":                   "2",
		"PercentOffset":            "0.05",
		"TrailStopPrice":           "190.5",
		"TrailingPercent":          "1.5",
		"ScaleInitLevelSize":       "2",
		"ScaleSubsLevelSize":       "1",
		"ScalePriceIncrement":      "0.1",
		"ScaleTable":               "1:1",
		"ActiveStartTime":          "20260413 09:30:00 US/Eastern",
		"ActiveStopTime":           "20260413 16:00:00 US/Eastern",
		"HedgeType":                "F",
		"HedgeParam":               "BUY EUR.USD",
		"AdjustedOrderType":        "STP",
		"TriggerPrice":             "198",
		"LmtPriceOffset":           "0.02",
		"AdjustedStopPrice":        "195",
		"AdjustedStopLimitPrice":   "194.5",
		"AdjustedTrailingAmount":   "1",
		"AdjustableTrailingUnit":   "1",
		"CashQty":                  "1000",
		"DontUseAutoPriceForHedge": "1",
		"UsePriceMgmtAlgo":         "0",
		"AdvancedErrorOverride":    "IBDBUYTX",
		"ManualOrderTime":          "20260413 15:00:00 US/Eastern",
	}
	for field, wantValue := range want {
		if checks[field] != wantValue {
			t.Fatalf("%s = %q, want %q", field, checks[field], wantValue)
		}
	}
}

func TestToCodecPlaceOrderMapsCompleteAdvancedShapes(t *testing.T) {
	t.Parallel()

	// Scale and PEG BENCH values are the official API 10.48.01 Python Testbed
	// sample vectors. This freezes public-to-wire ownership, not live acceptance.
	request := PlaceOrderRequest{
		Contract: Contract{ConID: 265598},
		Order: Order{
			Action:    ActionSellShort,
			OrderType: OrderTypePeggedBenchmark,
			Quantity:  decimal.NewFromInt(10),
			ShortSale: OrderShortSale{Slot: 2, DesignatedLocation: "LOCATE", ExemptCode: new(0)},
			Scale: OrderScale{
				InitialLevelSize:    2000,
				SubsequentLevelSize: 500,
				PriceIncrement:      decimal.RequireFromString("0.02"),
				PriceAdjustValue:    new(decimal.RequireFromString("189")),
				PriceAdjustInterval: new(3600),
				ProfitOffset:        new(decimal.RequireFromString("2")),
				AutoReset:           new(true),
				InitialPosition:     new(10),
				InitialFillQty:      new(40),
				RandomPercent:       new(true),
			},
			Auction: OrderAuction{
				StartingPrice: new(decimal.RequireFromString("33")), StockRefPrice: new(decimal.RequireFromString("750")),
				StockRangeLower: new(decimal.RequireFromString("650")), StockRangeUpper: new(decimal.RequireFromString("800")),
			},
			PeggedBenchmark: &OrderPeggedBenchmark{
				ReferenceContractID:   208813720,
				ChangeAmountDecrease:  true,
				ChangeAmount:          decimal.RequireFromString("0.1"),
				ReferenceChangeAmount: new(decimal.RequireFromString("1")),
				ReferenceExchangeID:   "ARCA",
			},
		},
	}
	got := toCodecPlaceOrder(77, request)
	checks := map[string]string{
		"ShortSaleSlot":              got.ShortSaleSlot,
		"DesignatedLocation":         got.DesignatedLocation,
		"ExemptCode":                 got.ExemptCode,
		"ScalePriceAdjustValue":      got.ScalePriceAdjustValue,
		"ScalePriceAdjustInterval":   got.ScalePriceAdjustInterval,
		"ScaleProfitOffset":          got.ScaleProfitOffset,
		"ScaleAutoReset":             got.ScaleAutoReset,
		"ScaleInitPosition":          got.ScaleInitPosition,
		"ScaleInitFillQty":           got.ScaleInitFillQty,
		"ScaleRandomPercent":         got.ScaleRandomPercent,
		"StartingPrice":              got.StartingPrice,
		"StockRefPrice":              got.StockRefPrice,
		"StockRangeLower":            got.StockRangeLower,
		"StockRangeUpper":            got.StockRangeUpper,
		"ReferenceContractID":        got.ReferenceContractID,
		"PeggedChangeAmountDecrease": got.PeggedChangeAmountDecrease,
		"PeggedChangeAmount":         got.PeggedChangeAmount,
		"ReferenceChangeAmount":      got.ReferenceChangeAmount,
		"ReferenceExchangeID":        got.ReferenceExchangeID,
	}
	want := map[string]string{
		"ShortSaleSlot":              "2",
		"DesignatedLocation":         "LOCATE",
		"ExemptCode":                 "0",
		"ScalePriceAdjustValue":      "189",
		"ScalePriceAdjustInterval":   "3600",
		"ScaleProfitOffset":          "2",
		"ScaleAutoReset":             "1",
		"ScaleInitPosition":          "10",
		"ScaleInitFillQty":           "40",
		"ScaleRandomPercent":         "1",
		"StartingPrice":              "33",
		"StockRefPrice":              "750",
		"StockRangeLower":            "650",
		"StockRangeUpper":            "800",
		"ReferenceContractID":        "208813720",
		"PeggedChangeAmountDecrease": "1",
		"PeggedChangeAmount":         "0.1",
		"ReferenceChangeAmount":      "1",
		"ReferenceExchangeID":        "ARCA",
	}
	for field, wantValue := range want {
		if checks[field] != wantValue {
			t.Fatalf("%s = %q, want %q", field, checks[field], wantValue)
		}
	}
}

func TestToCodecPreviewOrderSetsWhatIf(t *testing.T) {
	t.Parallel()

	req := PlaceOrderRequest{
		Contract: Contract{ConID: 265598},
		Order: Order{
			Action:    ActionBuy,
			OrderType: OrderTypeMarket,
			Quantity:  decimal.NewFromInt(1),
		},
	}
	if got := toCodecPlaceOrder(77, req).WhatIf; got != "" {
		t.Fatalf("live WhatIf = %q, want empty", got)
	}
	if got := toCodecPreviewOrder(77, req).WhatIf; got != "1" {
		t.Fatalf("preview WhatIf = %q, want 1", got)
	}
}

// Default-int fields (send(int) in IBKR reference clients) must serialize to a
// decimal digit even when the Order value is zero, because 0 is a semantic
// value: TriggerMethod=0=Default, AdjustableTrailingUnit=0=Amount,
// DisplaySize=0=unset iceberg display, OcaType=0=no OCA type. Scale-size
// fields (sendMax(int)) keep the unset sentinel, matching the reference
// clients' explicit-unset encoding.
func TestToCodecPlaceOrderZeroIntFieldSemantics(t *testing.T) {
	t.Parallel()

	got := toCodecPlaceOrder(1, PlaceOrderRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order: Order{
			Action:    ActionBuy,
			OrderType: OrderTypeMarket,
			Quantity:  decimal.RequireFromString("1"),
		},
	})

	checks := map[string]string{
		"OcaType":                got.OcaType,
		"TriggerMethod":          got.TriggerMethod,
		"DisplaySize":            got.DisplaySize,
		"AdjustableTrailingUnit": got.AdjustableTrailingUnit,
		"ScaleInitLevelSize":     got.ScaleInitLevelSize,
		"ScaleSubsLevelSize":     got.ScaleSubsLevelSize,
	}
	want := map[string]string{
		"OcaType":                "0",
		"TriggerMethod":          "0",
		"DisplaySize":            "0",
		"AdjustableTrailingUnit": "0",
		"ScaleInitLevelSize":     "",
		"ScaleSubsLevelSize":     "",
	}
	for field, wantValue := range want {
		if checks[field] != wantValue {
			t.Fatalf("%s = %q, want %q", field, checks[field], wantValue)
		}
	}
}
