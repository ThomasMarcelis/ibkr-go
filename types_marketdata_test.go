package ibkr

import "testing"

func TestHistoricalBidAskAttributes(t *testing.T) {
	t.Parallel()

	attributes := HistoricalBidAskAttributes(1 | 2 | 8)
	if !attributes.BidPastLow() || !attributes.AskPastHigh() {
		t.Fatalf("attributes %d did not expose both known bits", attributes)
	}
	if int(attributes)&8 == 0 {
		t.Fatal("unknown attribute bit was not preserved")
	}
}

func TestHistoricalLastAttributes(t *testing.T) {
	t.Parallel()

	attributes := HistoricalLastAttributes(1 | 2 | 8)
	if !attributes.PastLimit() || !attributes.Unreported() {
		t.Fatalf("attributes %d did not expose both known bits", attributes)
	}
	if int(attributes)&8 == 0 {
		t.Fatal("unknown attribute bit was not preserved")
	}
}
