package codec

import (
	"slices"
	"testing"
)

func TestPlaceOrderClassicPeggedTailSlots(t *testing.T) {
	t.Parallel()

	// API 10.48.01 EClient.java:3061-3079 writes the IBKRATS minimum-trade
	// quantity independently of the two PEG BEST or PEG MID offset slots.
	// The public model does not expose those fields, so their mandatory wire
	// positions are empty. Distinct following values make any shift visible.
	for _, tc := range []struct {
		name      string
		exchange  string
		orderType string
		empty     int
	}{
		{name: "plain SMART", exchange: "SMART", orderType: "LMT"},
		{name: "plain IBKRATS", exchange: "IBKRATS", orderType: "LMT", empty: 1},
		{name: "SMART PEG BEST", exchange: "SMART", orderType: "PEG BEST", empty: 2},
		{name: "SMART PEG MID", exchange: "SMART", orderType: "PEG MID", empty: 2},
		{name: "IBKRATS PEG MID overlap", exchange: "IBKRATS", orderType: "PEG MID", empty: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fields, err := (PlaceOrderRequest{
				Contract:             Contract{Exchange: tc.exchange},
				OrderType:            tc.orderType,
				ManualOrderTime:      "manual-time",
				CustomerAccount:      "customer-account",
				ProfessionalCustomer: "professional-customer",
				IncludeOvernight:     "include-overnight",
				ManualOrderIndicator: "manual-indicator",
				ImbalanceOnly:        "imbalance-only",
			}).encodeWire(200)
			if err != nil {
				t.Fatal(err)
			}
			manual := slices.Index(fields, "manual-time")
			if manual < 0 {
				t.Fatal("manual-order-time field not found")
			}
			want := append(make([]string, tc.empty),
				"customer-account", "professional-customer", "include-overnight", "manual-indicator", "imbalance-only")
			if got := fields[manual+1:]; !slices.Equal(got, want) {
				t.Fatalf("tail after ManualOrderTime = %#v, want %#v", got, want)
			}
		})
	}
}
