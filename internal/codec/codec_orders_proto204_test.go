package codec

import (
	"bytes"
	"testing"
)

func TestEncodeServer204OrderQueryVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{name: "client open orders", msg: OpenOrdersRequest{Scope: "client"}, want: "000000cd"},
		{name: "all open orders", msg: OpenOrdersRequest{Scope: "all"}, want: "000000d8"},
		{name: "bind auto open orders", msg: OpenOrdersRequest{Scope: "auto"}, want: "000000d70801"},
		{name: "unbind auto open orders", msg: CancelOpenOrders{}, want: "000000d7"},
		{name: "all completed orders", msg: CompletedOrdersRequest{}, want: "0000012b"},
		{name: "API completed orders", msg: CompletedOrdersRequest{APIOnly: true}, want: "0000012b0801"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(204, tc.msg)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			want := decodeHex(t, tc.want)
			if !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x, want exact API 10.48.01 vector %x", got, want)
			}
		})
	}
}

func TestDecodeServer204CompletedOrders(t *testing.T) {
	t.Parallel()

	// Exact callbacks from the sanitized exact-sv204 paper capture frozen in
	// completed_orders_sv204_live.txt. Only account, client, permanent, and
	// submitter identities were replaced; protobuf presence and values remain
	// byte-for-byte live-derived.
	cancelledMessage, err := Decode(204, recordedPayload(t, "AAABQgAAAS0KLwj+mhASBEFBUEwaA1NUSykAAAAAAAAAAEIFU01BUlRSA1VTRFoEQUFQTGIDTk1TEsgBCIUHEAEYgdKTrQMgACoDQlVZMgExQgNMTVRJAAAAAAAASUBRAAAAAAAAAABaA0RBWWIJRFU5MDAwMDAxegJJQrkBAAAAAACASUDwAQP4AQCwAgDAAgDKAgROb25l8AIAqAQAsAQAwAT///////////8B6gUETm9uZZAGAPgGAZoHATCyBylOb3QgYW4gaW5zaWRlciBvciBzdWJzdGFudGlhbCBzaGFyZWhvbGRlcsAHANAHAMoIDXBhcGVydHJhZGVyMDHwCAAaQAoJQ2FuY2VsbGVk6gEcMjAyNjA3MDkgMTk6MzU6MjkgVVMvRWFzdGVybvIBE0NhbmNlbGxlZCBieSBUcmFkZXI="))
	if err != nil {
		t.Fatalf("Decode(cancelled completed order) error = %v", err)
	}
	cancelled, ok := cancelledMessage.(CompletedOrder)
	if !ok {
		t.Fatalf("Decode(cancelled completed order) = %T", cancelledMessage)
	}
	if cancelled.Contract.ConID != 265598 || cancelled.Contract.Strike != "0" ||
		cancelled.ClientID != "901" || cancelled.OrderID != "1" || cancelled.PermID != "900000001" || cancelled.ParentID != "0" ||
		cancelled.Account != "DU9000001" || cancelled.Action != "BUY" || cancelled.Quantity != "1" ||
		cancelled.LmtPrice != "50" || cancelled.TrailStopPrice != "51" || cancelled.Filled != "0" ||
		cancelled.Status != "Cancelled" || cancelled.CompletedStatus != "Cancelled by Trader" || cancelled.Submitter != "papertrader01" {
		t.Fatalf("cancelled completed order = %+v", cancelled)
	}

	filledMessage, err := Decode(204, recordedPayload(t, "AAABTAAAAS0KLwj+mhASBEFBUEwaA1NUSykAAAAAAAAAAEIFU01BUlRSA1VTRFoEQUFQTGIDTk1TEswBCMkBEAIYgtKTrQMgACoEU0VMTDIBMEIDTE1USexRuB6FH3NAUQAAAAAAAAAAWgNEQVliCURVOTAwMDAwMXoCSUKYAQG5AexRuB6FL3NA8AED+AEAsAIAwAIAygIETm9uZfACAKgEALAEAMAE////////////AeoFBE5vbmWQBgD4BgGaBwExsgcpTm90IGFuIGluc2lkZXIgb3Igc3Vic3RhbnRpYWwgc2hhcmVob2xkZXLABwDQBwDKCA1wYXBlcnRyYWRlcjAx8AgAGkYKBkZpbGxlZFldv2A3bBvwP3IDVVNE6gEcMjAyNjA3MDkgMTg6NTU6MDYgVVMvRWFzdGVybvIBDkZpbGxlZCBTaXplOiAx"))
	if err != nil {
		t.Fatalf("Decode(filled completed order) error = %v", err)
	}
	filled, ok := filledMessage.(CompletedOrder)
	if !ok {
		t.Fatalf("Decode(filled completed order) = %T", filledMessage)
	}
	if filled.ClientID != "201" || filled.OrderID != "2" || filled.PermID != "900000002" || filled.ParentID != "0" ||
		filled.Action != "SELL" || filled.Quantity != "0" || filled.LmtPrice != "305.97" ||
		filled.OutsideRTH != "1" || filled.Filled != "1" || filled.Status != "Filled" ||
		filled.CommissionAndFees != "1.006695" || filled.CommissionCurrency != "USD" || filled.CompletedStatus != "Filled Size: 1" {
		t.Fatalf("filled completed order = %+v", filled)
	}

	end, err := Decode(204, decodeHex(t, "0000012e"))
	if err != nil {
		t.Fatalf("Decode(completed orders end) = %v", err)
	}
	if _, ok := end.(CompletedOrderEnd); !ok {
		t.Fatalf("completed orders end = %T", end)
	}
}

func TestDecodeServer204CompletedOrderRequiresEmittedMessages(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		"0000012d",
		"0000012d0a00",
		"0000012d0a001200",
	} {
		if _, err := Decode(204, decodeHex(t, body)); err == nil {
			t.Fatalf("Decode(%s) accepted a missing required nested message", body)
		}
	}
}
