package ibkr

import "testing"

func TestCloneOrderOwnsMutableInput(t *testing.T) {
	t.Parallel()

	// captures/20260415T162717Z-api_transmit_false_then_transmit_aapl,
	// server_version 200, events.jsonl sha256 003abb59dfced54248d50644ec171c406aefc587141bdd7780fb44c4d59d0a45.
	// The live request stages Transmit=false, then modifies the same order with
	// Transmit=true, making pointer ownership observable on the wire.
	original := Order{
		Transmit:         new(false),
		AllOrNone:        new(true),
		Hedge:            OrderHedge{DisableAutomaticPrice: new(true)},
		WhatIf:           new(false),
		UsePriceMgmtAlgo: new(true),
		Combo: OrderCombo{
			Legs:         []ComboLeg{{ConID: 1}},
			LegPrices:    []string{"1.25"},
			SmartRouting: []TagValue{{Tag: "NonGuaranteed", Value: "1"}},
		},
		Algorithm:  OrderAlgorithm{Params: []TagValue{{Tag: "adaptivePriority", Value: "Normal"}}},
		Conditions: OrderConditions{Values: []OrderCondition{{Type: ConditionPrice}}},
	}
	cloned := cloneOrder(original)

	original.Combo.Legs[0].ConID = 2
	original.Combo.LegPrices[0] = "2.50"
	original.Combo.SmartRouting[0].Value = "0"
	original.Algorithm.Params[0].Value = "Patient"
	original.Conditions.Values[0].Type = ConditionTime
	*original.Transmit = true
	*original.AllOrNone = false
	*original.Hedge.DisableAutomaticPrice = false
	*original.WhatIf = true
	*original.UsePriceMgmtAlgo = false

	if cloned.Combo.Legs[0].ConID != 1 || cloned.Combo.LegPrices[0] != "1.25" ||
		cloned.Combo.SmartRouting[0].Value != "1" || cloned.Algorithm.Params[0].Value != "Normal" ||
		cloned.Conditions.Values[0].Type != ConditionPrice {
		t.Fatalf("clone shares nested slice storage: %#v", cloned)
	}

	pointers := []struct {
		name string
		got  *bool
		want bool
	}{
		{name: "Transmit", got: cloned.Transmit, want: false},
		{name: "AllOrNone", got: cloned.AllOrNone, want: true},
		{name: "Hedge.DisableAutomaticPrice", got: cloned.Hedge.DisableAutomaticPrice, want: true},
		{name: "WhatIf", got: cloned.WhatIf, want: false},
		{name: "UsePriceMgmtAlgo", got: cloned.UsePriceMgmtAlgo, want: true},
	}
	for _, pointer := range pointers {
		if pointer.got == nil || *pointer.got != pointer.want {
			t.Errorf("cloned %s = %v, want %t", pointer.name, pointer.got, pointer.want)
		}
	}
}
