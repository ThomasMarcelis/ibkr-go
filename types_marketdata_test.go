package ibkr

import "testing"

func TestTickBidAskAttributes(t *testing.T) {
	t.Parallel()

	attributes := TickBidAskAttributes(1 | 2 | 8)
	if !attributes.BidPastLow() || !attributes.AskPastHigh() {
		t.Fatalf("attributes %d did not expose both known bits", attributes)
	}
	if int(attributes)&8 == 0 {
		t.Fatal("unknown attribute bit was not preserved")
	}
}

func TestTickLastAttributes(t *testing.T) {
	t.Parallel()

	attributes := TickLastAttributes(1 | 2 | 8)
	if !attributes.PastLimit() || !attributes.Unreported() {
		t.Fatalf("attributes %d did not expose both known bits", attributes)
	}
	if int(attributes)&8 == 0 {
		t.Fatal("unknown attribute bit was not preserved")
	}
}
