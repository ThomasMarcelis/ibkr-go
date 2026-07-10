package ibkr

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCloneOrderOwnsMutableInput(t *testing.T) {
	t.Parallel()

	// This ownership-only composite labels its independent evidence. The
	// transmit pointer comes from the 20260415 live false-then-true request
	// (events SHA-256 003abb59dfced54248d50644ec171c406aefc587141bdd7780fb44c4d59d0a45),
	// BAG identity/routing from the June paper combo order, and adaptive/price
	// condition values from their live campaigns. The 0.05 leg price and
	// explicit zero exempt code exercise official-schema presence laws; neither
	// is claimed as a live nondefault combo echo.
	original := PlaceOrderRequest{Contract: Contract{
		ConID: 28812380, SecType: SecTypeCombo, Strike: new(decimal.NewFromInt(0)),
		ComboLegs: []ComboLeg{{
			ConID: 878923092, Ratio: 1, Action: ActionSell, Exchange: "SMART", ExemptCode: new(0),
		}},
	}, Order: Order{
		Transmit:         new(false),
		AllOrNone:        new(true),
		Hedge:            OrderHedge{DisableAutomaticPrice: new(true)},
		UsePriceMgmtAlgo: new(true),
		Combo: OrderCombo{
			LegPrices:    []*decimal.Decimal{new(decimal.RequireFromString("0.05"))},
			SmartRouting: []TagValue{{Tag: "NonGuaranteed", Value: "1"}},
		},
		Algorithm:  OrderAlgorithm{Params: []TagValue{{Tag: "adaptivePriority", Value: "Normal"}}},
		Conditions: OrderConditions{Values: []OrderCondition{{Type: ConditionPrice}}},
	}}
	cloned := clonePlaceOrderRequest(original)

	original.Contract.ComboLegs[0].ConID = 886441502
	*original.Contract.ComboLegs[0].ExemptCode = 1
	*original.Contract.Strike = decimal.NewFromInt(1)
	*original.Order.Combo.LegPrices[0] = decimal.RequireFromString("291.09")
	original.Order.Combo.SmartRouting[0].Value = "0"
	original.Order.Algorithm.Params[0].Value = "Patient"
	original.Order.Conditions.Values[0].Type = ConditionTime
	*original.Order.Transmit = true
	*original.Order.AllOrNone = false
	*original.Order.Hedge.DisableAutomaticPrice = false
	*original.Order.UsePriceMgmtAlgo = false

	if cloned.Contract.ComboLegs[0].ConID != 878923092 || *cloned.Contract.ComboLegs[0].ExemptCode != 0 ||
		cloned.Contract.Strike == nil || !cloned.Contract.Strike.IsZero() || cloned.Order.Combo.LegPrices[0] == nil ||
		!cloned.Order.Combo.LegPrices[0].Equal(decimal.RequireFromString("0.05")) ||
		cloned.Order.Combo.SmartRouting[0].Value != "1" || cloned.Order.Algorithm.Params[0].Value != "Normal" ||
		cloned.Order.Conditions.Values[0].Type != ConditionPrice {
		t.Fatalf("clone shares nested slice storage: %#v", cloned)
	}

	pointers := []struct {
		name string
		got  *bool
		want bool
	}{
		{name: "Transmit", got: cloned.Order.Transmit, want: false},
		{name: "AllOrNone", got: cloned.Order.AllOrNone, want: true},
		{name: "Hedge.DisableAutomaticPrice", got: cloned.Order.Hedge.DisableAutomaticPrice, want: true},
		{name: "UsePriceMgmtAlgo", got: cloned.Order.UsePriceMgmtAlgo, want: true},
	}
	for _, pointer := range pointers {
		if pointer.got == nil || *pointer.got != pointer.want {
			t.Errorf("cloned %s = %v, want %t", pointer.name, pointer.got, pointer.want)
		}
	}
}
