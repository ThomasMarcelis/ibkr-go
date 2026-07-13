package codec

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"math"
	"testing"
)

func TestEncodeServer203OrderRequestVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{
			name: "place",
			msg: PlaceOrderRequest{
				OrderID:  1,
				Contract: Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
				Action:   "BUY", TotalQuantity: "1", DisplaySize: "0", OrderType: "LMT",
				LmtPrice: "50", TIF: "DAY", Transmit: "1", ParentID: "0", Origin: "0",
				OcaType: "0", TriggerMethod: "0", ExemptCode: "-1", AdjustableTrailingUnit: "0",
			},
			// Official API 10.48.01 PlaceOrderRequest.proto / Contract.proto
			// source-law vector. Contract.conId is proto3 optional, but official
			// EClientUtils sets it because Utils::isValidValue accepts zero. This
			// freezes official implementation behavior, not a schema requirement
			// or live order attestation. Raw ID 203 is base ID 3 plus the protobuf
			// discriminator 200.
			want: "000000cb08011219080012044141504c1a0353544b4205534d41525452035553441a3d20002a03425559320131380042034c4d544900000000000049405a03444159f00100f80100900401a80400c004ffffffffffffffffff01900600ca06002200",
		},
		{name: "cancel", msg: CancelOrderRequest{OrderID: 1}, want: "000000cc08011200"},
		{name: "global cancel", msg: GlobalCancelRequest{}, want: "000001020a00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(203, tc.msg)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			want := decodeHex(t, tc.want)
			if !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x, want API 10.48.01 vector %x", got, want)
			}
		})
	}
}

func TestEncodePlaceOrderContractSourceLaws(t *testing.T) {
	t.Parallel()

	contract, err := encodeOrderContractProto(Contract{}, nil)
	if err != nil {
		t.Fatalf("encodeOrderContractProto() error = %v", err)
	}
	// API 10.48.01 Contract.proto: optional int32 conId = 1. Official
	// EClientUtils emits zero because Utils::isValidValue(0) is true.
	if want := decodeHex(t, "0800"); !bytes.Equal(contract, want) {
		t.Fatalf("zero-conId contract = %x, want official explicit tag 1 %x", contract, want)
	}

	leg, err := encodeComboLegProto(ComboLeg{
		ConID: 265598, Ratio: 1, Action: "BUY", Exchange: "SMART",
		OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1",
	}, "0.05")
	if err != nil {
		t.Fatalf("encodeComboLegProto() error = %v", err)
	}
	// API 10.48.01 ComboLeg.proto: optional double price = 9. This exact
	// source-law vector proves a supplied per-leg price is tag 9/fixed64;
	// it is not presented as a live placed-order capture.
	wantLeg := decodeHex(t, "08fe9a1010011a034255592205534d4152542800300040ffffffffffffffffff01499a9999999999a93f")
	if !bytes.Equal(leg, wantLeg) {
		t.Fatalf("priced combo leg = %x, want source-law vector %x", leg, wantLeg)
	}
}

func TestEncodeAdditionalOrderParametersFromLiveSDKCaptures(t *testing.T) {
	t.Parallel()

	order216, err := encodeOrderProto(PlaceOrderRequest{
		Deactivate: "1", PostOnly: "1", AllowPreOpen: "1", IgnoreOpenAuction: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := decodeHex(t, "ca0600d00801d80801e00801e80801"); !bytes.Equal(order216, want) {
		t.Fatalf("sv216 fields = %x, want %x; capture events sha256 %s", order216, want, "dd8e34848c2de885947f2eec9c77b4901d428c5719dc85bcaea4e9417212b7cc")
	}

	order217, err := encodeOrderProto(PlaceOrderRequest{
		RouteMarketableToBBO: "1", SeekPriceImprovement: "1", WhatIfType: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := decodeHex(t, "ca0600c00701f00801f80801"); !bytes.Equal(order217, want) {
		t.Fatalf("sv217 fields = %x, want %x; capture events sha256 %s", order217, want, "302d403d32b43f1107b8e15fa62ac6fd318658d61bc10496d8331907f6e10dc2")
	}

	attached, err := encodeAttachedOrdersProto(PlaceOrderRequest{
		AttachedStopLossOrderID: 4, AttachedStopLossOrderType: "PRESET",
		AttachedTakeProfitOrderID: 5, AttachedTakeProfitOrderType: "PRESET",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := decodeHex(t, "0804120650524553455418052206505245534554"); !bytes.Equal(attached, want) {
		t.Fatalf("sv218 attached orders = %x, want %x; capture events sha256 %s", attached, want, "34ebb09db5b427aed962859ba2f5c137fcd328987eb1c8aa72f18688960fcf62")
	}

	hedge223, err := encodeOrderProto(PlaceOrderRequest{HedgeMaxSize: "7"})
	if err != nil {
		t.Fatal(err)
	}
	if want := decodeHex(t, "ca0600800907"); !bytes.Equal(hedge223, want) {
		t.Fatalf("sv223 hedge maximum size = %x, want %x; capture events sha256 %s", hedge223, want, "205f25d37f53daf6dcc0a7b2f93a58215dcf7e1091e5a3fbafb459f018764061")
	}
}

func TestEncodeAdditionalOrderParameterBoundaries(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		sv  int
		msg PlaceOrderRequest
	}{
		{215, PlaceOrderRequest{OrderID: 1, PostOnly: "1"}},
		{216, PlaceOrderRequest{OrderID: 1, SeekPriceImprovement: "1"}},
		{217, PlaceOrderRequest{OrderID: 1, AttachedStopLossOrderID: 2, AttachedStopLossOrderType: "PRESET"}},
		{222, PlaceOrderRequest{OrderID: 1, HedgeMaxSize: "7"}},
	} {
		if _, err := Encode(tc.sv, tc.msg); err == nil {
			t.Fatalf("Encode(%d, %+v) accepted fields before their server-version boundary", tc.sv, tc.msg)
		}
	}
}

func TestDecodeServer203OrderCallbacks(t *testing.T) {
	t.Parallel()

	// Exact callbacks from the sanitized 20260710 exact-sv203 paper capture
	// frozen in order_lifecycle_sv203_live.txt. These retain every live
	// protobuf field; only account/permanent/submitter identifiers were
	// substituted.
	openMessage, err := Decode(203, recordedPayload(t, "AAABEwAAAM0IwAMSLwj+mhASBEFBUEwaA1NUSykAAAAAAAAAAEIFU01BUlRSA1VTRFoEQUFQTGIDTk1TGsgBCAEQwAMYgdKTrQMgACoDQlVZMgExQgNMTVRJAAAAAAAASUBRAAAAAAAAAABaA0RBWWIJRFU5MDAwMDAxegJJQrkBAAAAAACASUDwAQP4AQCwAgDAAgDKAgROb25l8AIAqAQAsAQAwAT///////////8B6gUETm9uZZAGAPgGAZoHATCyBylOb3QgYW4gaW5zaWRlciBvciBzdWJzdGFudGlhbCBzaGFyZWhvbGRlcsAHANAHAMoIDXBhcGVydHJhZGVyMDHwCAAiDgoMUHJlU3VibWl0dGVk"))
	if err != nil {
		t.Fatalf("Decode(open order) error = %v", err)
	}
	openOrder, ok := openMessage.(OpenOrder)
	if !ok {
		t.Fatalf("Decode(open order) = %T", openMessage)
	}
	if openOrder.OrderID != 448 || openOrder.Contract.ConID != 265598 || openOrder.Contract.Symbol != "AAPL" ||
		openOrder.Contract.Strike != "0" || openOrder.Action != "BUY" || openOrder.Quantity != "1" ||
		openOrder.OrderType != "LMT" || openOrder.LmtPrice != "50" || openOrder.Account != "DU9000001" ||
		openOrder.ClientID != "1" || openOrder.PermID != "900000001" || openOrder.Status != "PreSubmitted" {
		t.Fatalf("open order = %+v", openOrder)
	}

	statusMessage, err := Decode(203, recordedPayload(t, "AAAAQAAAAMsIwAMSDFByZVN1Ym1pdHRlZBoBMCIBMSkAAAAAAAAAADCB0pOtAzgAQQAAAAAAAAAASAFZAAAAAAAAAAA="))
	if err != nil {
		t.Fatalf("Decode(order status) error = %v", err)
	}
	status, ok := statusMessage.(OrderStatus)
	if !ok || status.OrderID != 448 || status.Status != "PreSubmitted" || status.Filled != "0" ||
		status.Remaining != "1" || status.PermID != "900000001" || status.ClientID != "1" {
		t.Fatalf("order status = %#v", statusMessage)
	}

	for _, tc := range []struct {
		name string
		data string
		code int
		when string
	}{
		{"targeted cancel", "AAAAKwAAAMwIwAMQqJCp5vQzGMoBIhhPcmRlciBDYW5jZWxlZCAtIHJlYXNvbjo=", 202, "1783699753000"},
		{"global cancel after terminal", "AAAAZgAAAMwIwAMQrZep5vQzGKEBIlNDYW5jZWwgYXR0ZW1wdGVkIHdoZW4gb3JkZXIgaXMgbm90IGluIGEgY2FuY2VsbGFibGUgc3RhdGUuICBPcmRlciBwZXJtSWQgPTkwMDAwMDAwMQ==", 161, "1783699753901"},
	} {
		message, err := Decode(203, recordedPayload(t, tc.data))
		if err != nil {
			t.Fatalf("Decode(%s error) = %v", tc.name, err)
		}
		apiError, ok := message.(APIError)
		if !ok || apiError.ReqID != 448 || apiError.Code != tc.code || apiError.ErrorTimeMs != tc.when {
			t.Fatalf("%s error = %#v", tc.name, message)
		}
	}

	end, err := Decode(203, decodeHex(t, "000000fd"))
	if err != nil {
		t.Fatalf("Decode(open orders end) = %v", err)
	}
	if _, ok := end.(OpenOrderEnd); !ok {
		t.Fatalf("open orders end = %T", end)
	}
}

func TestEncodeServer203OrdersFailClosed(t *testing.T) {
	t.Parallel()

	for _, msg := range []Message{
		PlaceOrderRequest{OrderID: math.MaxInt32 + 1},
		PlaceOrderRequest{OrderID: 1, LmtPrice: "not-a-number"},
		PlaceOrderRequest{OrderID: 1, Conditions: []OrderCondition{{Type: 2}}},
		CancelOrderRequest{OrderID: math.MaxInt32 + 1},
		GlobalCancelRequest{ManualOrderIndicator: "unknown"},
	} {
		if _, err := Encode(203, msg); err == nil {
			t.Fatalf("Encode(%T) accepted an incomplete protobuf shape", msg)
		}
	}
}

func recordedPayload(t *testing.T, value string) []byte {
	t.Helper()
	frame, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) < 4 || int(binary.BigEndian.Uint32(frame[:4])) != len(frame)-4 {
		t.Fatalf("recorded frame has invalid length prefix")
	}
	return frame[4:]
}
