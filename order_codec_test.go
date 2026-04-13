package ibkr

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestToCodecPlaceOrderMapsAdvancedOrderFields(t *testing.T) {
	t.Parallel()

	got := toCodecPlaceOrder(77, PlaceOrderRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order: Order{
			Action:                   Buy,
			OrderType:                OrderTypeTrailingLimit,
			Quantity:                 decimal.RequireFromString("10"),
			LmtPrice:                 decimal.RequireFromString("200.25"),
			AuxPrice:                 decimal.RequireFromString("1.25"),
			TIF:                      TIFGTC,
			Account:                  "DU123",
			OcaGroup:                 "oca-test",
			OcaType:                  1,
			TriggerMethod:            4,
			DisplaySize:              3,
			AllOrNone:                new(true),
			MinQty:                   decimal.RequireFromString("2"),
			PercentOffset:            decimal.RequireFromString("0.05"),
			TrailStopPrice:           decimal.RequireFromString("190.50"),
			TrailingPercent:          decimal.RequireFromString("1.5"),
			ScaleInitLevelSize:       2,
			ScaleSubsLevelSize:       1,
			ScalePriceIncrement:      decimal.RequireFromString("0.10"),
			ScaleTable:               "1:1",
			ActiveStartTime:          "20260413 09:30:00 US/Eastern",
			ActiveStopTime:           "20260413 16:00:00 US/Eastern",
			HedgeType:                "F",
			HedgeParam:               "BUY EUR.USD",
			WhatIf:                   new(true),
			AdjustedOrderType:        OrderTypeStop,
			TriggerPrice:             decimal.RequireFromString("198"),
			LmtPriceOffset:           decimal.RequireFromString("0.02"),
			AdjustedStopPrice:        decimal.RequireFromString("195"),
			AdjustedStopLimitPrice:   decimal.RequireFromString("194.5"),
			AdjustedTrailingAmount:   decimal.RequireFromString("1"),
			AdjustableTrailingUnit:   1,
			CashQty:                  decimal.RequireFromString("1000"),
			DontUseAutoPriceForHedge: new(true),
			UsePriceMgmtAlgo:         new(false),
			AdvancedErrorOverride:    "IBDBUYTX",
			ManualOrderTime:          "20260413 15:00:00 US/Eastern",
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
		"WhatIf":                   got.WhatIf,
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
		"WhatIf":                   "1",
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
