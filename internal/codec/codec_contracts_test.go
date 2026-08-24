package codec

import "testing"

func TestSplitLastTradeDateHyphenatedForm(t *testing.T) {
	t.Parallel()

	date, tradeTime := splitLastTradeDate("20261218-08:30:00-US/Central")
	if date != "20261218" || tradeTime != "08:30:00" {
		t.Fatalf("splitLastTradeDate() = %q/%q, want 20261218/08:30:00", date, tradeTime)
	}
}
