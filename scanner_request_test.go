package ibkr

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCloneScannerSubscriptionRequestOwnsMutableInput(t *testing.T) {
	price := decimal.RequireFromString("3")
	volume := 1000
	excludeConvertible := false
	req := ScannerSubscriptionRequest{
		AbovePrice:         &price,
		AboveVolume:        &volume,
		ExcludeConvertible: &excludeConvertible,
		FilterOptions: []TagValue{
			{Tag: "usdMarketCapAbove", Value: "10000"},
		},
		SubscriptionOptions: []TagValue{
			{Tag: "underConID", Value: "265598"},
		},
	}

	cloned := cloneScannerSubscriptionRequest(req)
	*req.AbovePrice = decimal.RequireFromString("4")
	*req.AboveVolume = 2000
	*req.ExcludeConvertible = true
	req.FilterOptions[0].Value = "20000"
	req.SubscriptionOptions[0].Value = "0"

	if got, want := cloned.AbovePrice.String(), "3"; got != want {
		t.Fatalf("AbovePrice = %s, want %s", got, want)
	}
	if got, want := *cloned.AboveVolume, 1000; got != want {
		t.Fatalf("AboveVolume = %d, want %d", got, want)
	}
	if *cloned.ExcludeConvertible {
		t.Fatal("ExcludeConvertible = true, want false")
	}
	if got, want := cloned.FilterOptions[0].Value, "10000"; got != want {
		t.Fatalf("FilterOptions[0].Value = %q, want %q", got, want)
	}
	if got, want := cloned.SubscriptionOptions[0].Value, "265598"; got != want {
		t.Fatalf("SubscriptionOptions[0].Value = %q, want %q", got, want)
	}
}

func TestToCodecScannerSubscriptionRequestPreservesOfficialSampleValues(t *testing.T) {
	// Values come from the official Java ScannerDlg and Testbed scanner samples.
	abovePrice := decimal.RequireFromString("3")
	aboveVolume := 0
	marketCapAbove := decimal.RequireFromString("100000000")
	averageOptionVolumeAbove := 0
	excludeConvertible := false
	got := toCodecScannerSubscriptionRequest(7002, ScannerSubscriptionRequest{
		NumberOfRows:             10,
		Instrument:               "STK",
		LocationCode:             "STK.US.MAJOR",
		ScanCode:                 "HOT_BY_VOLUME",
		AbovePrice:               &abovePrice,
		AboveVolume:              &aboveVolume,
		MarketCapAbove:           &marketCapAbove,
		ExcludeConvertible:       &excludeConvertible,
		AverageOptionVolumeAbove: &averageOptionVolumeAbove,
		ScannerSettingPairs:      "Annual,true",
		StockTypeFilter:          "ALL",
		FilterOptions: []TagValue{
			{Tag: "usdMarketCapAbove", Value: "10000"},
			{Tag: "optVolumeAbove", Value: "1000"},
			{Tag: "avgVolumeAbove", Value: "100000000"},
		},
	})

	if got.ReqID != 7002 || got.NumberOfRows != 10 || got.AbovePrice != "3" || got.AboveVolume != "0" {
		t.Fatalf("basic scanner fields = %+v", got)
	}
	if got.MarketCapAbove != "100000000" || got.ExcludeConvertible != "0" || got.AverageOptionVolumeAbove != "0" {
		t.Fatalf("optional scanner fields = %+v", got)
	}
	if got.BelowPrice != "" || got.MarketCapBelow != "" {
		t.Fatalf("unset scanner fields = %+v", got)
	}
	if got.ScannerSettingPairs != "Annual,true" || got.StockTypeFilter != "ALL" {
		t.Fatalf("scanner settings = %+v", got)
	}
	if len(got.FilterOptions) != 3 || got.FilterOptions[0].Tag != "usdMarketCapAbove" || got.FilterOptions[2].Value != "100000000" {
		t.Fatalf("filter options = %+v", got.FilterOptions)
	}
	if got := toCodecScannerSubscriptionRequest(7003, ScannerSubscriptionRequest{}).NumberOfRows; got != -1 {
		t.Fatalf("default NumberOfRows = %d, want -1", got)
	}
}
