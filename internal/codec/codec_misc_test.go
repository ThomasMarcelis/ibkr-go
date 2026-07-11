package codec

import (
	"slices"
	"testing"
)

func TestScannerSubscriptionRequestMatchesLiveServer200Shape(t *testing.T) {
	// captures/20260407T190657Z-scanner_subscription/events.jsonl,
	// 2026-04-07T19:06:58.035817304Z. The accepted client payload has SHA-256
	// f0cf2dab760c74ef107e07322055a6de352e2f621fe9eeefd8132f1ca906c519.
	const maxFloat = "1.7976931348623157E308"
	const maxInt = "2147483647"
	fields, err := (ScannerSubscriptionRequest{
		ReqID:                    1001,
		NumberOfRows:             10,
		Instrument:               "STK",
		LocationCode:             "STK.US.MAJOR",
		ScanCode:                 "HOT_BY_VOLUME",
		AbovePrice:               maxFloat,
		BelowPrice:               maxFloat,
		AboveVolume:              maxInt,
		MarketCapAbove:           maxFloat,
		MarketCapBelow:           maxFloat,
		CouponRateAbove:          maxFloat,
		CouponRateBelow:          maxFloat,
		AverageOptionVolumeAbove: maxInt,
	}).encodeWire(200)
	if err != nil {
		t.Fatalf("encodeWire() error = %v", err)
	}
	want := []string{
		"22", "1001", "10", "STK", "STK.US.MAJOR", "HOT_BY_VOLUME",
		maxFloat, maxFloat, maxInt, maxFloat, maxFloat,
		"", "", "", "", "", "",
		maxFloat, maxFloat, "", maxInt, "", "", "", "",
	}
	if !slices.Equal(fields, want) {
		t.Fatalf("encodeWire() = %#v, want %#v", fields, want)
	}
}

func TestScannerSubscriptionRequestEncodesOfficialGenericFilters(t *testing.T) {
	// These filters are the official Testbed scanner example. EClient.cpp's
	// EncodeTagValueList writes the list into one field as tag=value; entries.
	fields, err := (ScannerSubscriptionRequest{
		ReqID:        7002,
		NumberOfRows: -1,
		Instrument:   "STK",
		LocationCode: "STK.US.MAJOR",
		ScanCode:     "HOT_BY_VOLUME",
		FilterOptions: []TagValue{
			{Tag: "usdMarketCapAbove", Value: "10000"},
			{Tag: "optVolumeAbove", Value: "1000"},
			{Tag: "avgVolumeAbove", Value: "100000000"},
		},
	}).encodeWire(200)
	if err != nil {
		t.Fatalf("encodeWire() error = %v", err)
	}
	if got, want := len(fields), 25; got != want {
		t.Fatalf("field count = %d, want %d", got, want)
	}
	if got, want := fields[23], "usdMarketCapAbove=10000;optVolumeAbove=1000;avgVolumeAbove=100000000;"; got != want {
		t.Fatalf("filter options = %q, want %q", got, want)
	}
	if fields[24] != "" {
		t.Fatalf("subscription options = %q, want empty", fields[24])
	}
}
