package codec

import (
	"slices"
	"testing"
)

func TestScannerSubscriptionRequestClassicShape(t *testing.T) {
	// The classic scanner body remains reachable at supported versions 208 and
	// 209. Its fields follow the official EClient request order.
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
	}).encodeWire(208)
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
	}).encodeWire(208)
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

func TestUserInfoClassicSV208LiveVector(t *testing.T) {
	t.Parallel()

	// Exact request and response from capture 20260825T194619Z-sv208_user_info,
	// events.jsonl SHA-256
	// 672370162ad17e46cf045647775d1d6bc4480353b2f044392c43431a88717bd5.
	// SDK 10.48.01 EClient::reqUserInfo independently confirms that the request
	// is [msgID, reqID], with no version field.
	request, err := Encode(208, UserInfoRequest{ReqID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if want := decodeHex(t, "000000683100"); !slices.Equal(request, want) {
		t.Fatalf("Encode(UserInfoRequest) = %x, want %x", request, want)
	}

	message, err := Decode(208, decodeHex(t, "0000006b310000"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (UserInfo{ReqID: 1}); message != want {
		t.Fatalf("Decode(UserInfo) = %#v, want %#v", message, want)
	}
}
