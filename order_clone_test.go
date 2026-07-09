package ibkr

import "testing"

func TestCloneOrderOwnsNestedSlices(t *testing.T) {
	t.Parallel()

	original := Order{
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

	if cloned.Combo.Legs[0].ConID != 1 || cloned.Combo.LegPrices[0] != "1.25" ||
		cloned.Combo.SmartRouting[0].Value != "1" || cloned.Algorithm.Params[0].Value != "Normal" ||
		cloned.Conditions.Values[0].Type != ConditionPrice {
		t.Fatalf("clone shares nested slice storage: %#v", cloned)
	}
}
