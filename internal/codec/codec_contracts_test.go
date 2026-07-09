package codec

import "testing"

func TestSplitLastTradeDateHyphenatedForm(t *testing.T) {
	t.Parallel()

	date, tradeTime := splitLastTradeDate("20261218-08:30:00-US/Central")
	if date != "20261218" || tradeTime != "08:30:00" {
		t.Fatalf("splitLastTradeDate() = %q/%q, want 20261218/08:30:00", date, tradeTime)
	}
}

func TestDecodeContractDetailsASCII7LongName(t *testing.T) {
	t.Parallel()

	// APPLE INC is a live ContractDetails long name; the escaped space applies
	// the official ASCII7 representation without inventing protocol content.
	if got := decodeUnicodeEscapes(`APPLE\u0020INC`); got != "APPLE INC" {
		t.Fatalf("decodeUnicodeEscapes() = %q, want APPLE INC", got)
	}
}
