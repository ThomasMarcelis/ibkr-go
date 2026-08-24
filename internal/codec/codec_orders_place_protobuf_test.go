package codec

import (
	"bytes"
	"math"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestEncodePlaceOrderRequestVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  OutboundMessage
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
			got, err := Encode(208, tc.msg)
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

func TestIncludeOvernightOrderProtoSourceLaw(t *testing.T) {
	t.Parallel()

	// API 10.48.01 Order.proto SHA-256
	// 3a963d252987b4a1450d6ded1901f46fdbad16039b6f717dc8beb597e78695c8
	// defines optional bool includeOvernight as field 135. The public pointer
	// distinguishes absent from explicit false, so both supplied values retain
	// their protobuf presence. Every Order also contains the official empty
	// SoftDollarTier message at field 105.
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "absent", want: "ca0600"},
		{name: "explicit false field 135", value: "0", want: "ca0600b80800"},
		{name: "true field 135", value: "1", want: "ca0600b80801"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := encodeOrderProto(PlaceOrderRequest{IncludeOvernight: tc.value})
			if err != nil {
				t.Fatal(err)
			}
			want := decodeHex(t, tc.want)
			if !bytes.Equal(got, want) {
				t.Fatalf("include overnight order = %x, want official source-law vector %x", got, want)
			}
		})
	}

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "absent"},
		{name: "explicit false", body: "b80800", want: "0"},
		{name: "explicit true", body: "b80801", want: "1"},
	} {
		t.Run("decode "+tc.name, func(t *testing.T) {
			t.Parallel()
			var got OrderDetails
			if err := decodeOrderDetailsProto(decodeHex(t, tc.body), &got); err != nil {
				t.Fatal(err)
			}
			if got.IncludeOvernight != tc.want {
				t.Fatalf("decoded include overnight = %q, want %q", got.IncludeOvernight, tc.want)
			}
		})
	}
}

func TestOpenOrderProtoPreservesWhatIfPresence(t *testing.T) {
	t.Parallel()

	// API 10.48.01 Order.proto defines optional bool whatIf as field 65.
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "absent"},
		{name: "explicit false", body: "880400", want: "0"},
		{name: "explicit true", body: "880401", want: "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got OrderDetails
			if err := decodeOrderDetailsProto(decodeHex(t, tc.body), &got); err != nil {
				t.Fatal(err)
			}
			if got.WhatIf != tc.want {
				t.Fatalf("decoded what-if = %q, want %q", got.WhatIf, tc.want)
			}
		})
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

func TestEncodeScaleAndPeggedBenchmarkOrderSourceLaws(t *testing.T) {
	t.Parallel()

	// These are the official API 10.48.01 Testbed sample values. The assertion
	// freezes Order.proto field numbers and presence, not live acceptance.
	got, err := encodeOrderProto(PlaceOrderRequest{
		ScalePriceAdjustValue: "189", ScalePriceAdjustInterval: "3600", ScaleProfitOffset: "2",
		ScaleAutoReset: "1", ScaleInitPosition: "10", ScaleInitFillQty: "40", ScaleRandomPercent: "1",
		ScaleTable: "scale-table", StartingPrice: "33", StockRefPrice: "750", StockRangeLower: "650", StockRangeUpper: "800",
		ReferenceContractID: "208813720", PeggedChangeAmount: "0.1",
		PeggedChangeAmountDecrease: "1", ReferenceChangeAmount: "1", ReferenceExchangeID: "ARCA",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := appendProtoTestDouble(nil, 51, 189)
	want = protowire.AppendTag(want, 52, protowire.VarintType)
	want = protowire.AppendVarint(want, 3600)
	want = appendProtoTestDouble(want, 53, 2)
	for _, field := range []struct {
		number protowire.Number
		value  uint64
	}{{54, 1}, {55, 10}, {56, 40}, {57, 1}} {
		want = protowire.AppendTag(want, field.number, protowire.VarintType)
		want = protowire.AppendVarint(want, field.value)
	}
	want = protowire.AppendTag(want, 58, protowire.BytesType)
	want = protowire.AppendString(want, "scale-table")
	want = appendProtoTestDouble(want, 78, 33)
	want = appendProtoTestDouble(want, 79, 750)
	want = appendProtoTestDouble(want, 81, 650)
	want = appendProtoTestDouble(want, 82, 800)
	want = protowire.AppendTag(want, 88, protowire.VarintType)
	want = protowire.AppendVarint(want, 208813720)
	want = appendProtoTestDouble(want, 89, 0.1)
	want = protowire.AppendTag(want, 90, protowire.VarintType)
	want = protowire.AppendVarint(want, 1)
	want = appendProtoTestDouble(want, 91, 1)
	want = protowire.AppendTag(want, 92, protowire.BytesType)
	want = protowire.AppendString(want, "ARCA")
	want = protowire.AppendTag(want, 105, protowire.BytesType)
	want = protowire.AppendBytes(want, nil)
	if !bytes.Equal(got, want) {
		t.Fatalf("advanced order protobuf = %x, want API 10.48.01 Order.proto field vector %x", got, want)
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

func TestEncodePlaceOrdersFailClosed(t *testing.T) {
	t.Parallel()

	for _, msg := range []OutboundMessage{
		PlaceOrderRequest{OrderID: math.MaxInt32 + 1},
		PlaceOrderRequest{OrderID: 1, LmtPrice: "not-a-number"},
		PlaceOrderRequest{OrderID: 1, Conditions: []OrderCondition{{Type: 2}}},
		CancelOrderRequest{OrderID: math.MaxInt32 + 1},
		GlobalCancelRequest{ManualOrderIndicator: "unknown"},
	} {
		if _, err := Encode(208, msg); err == nil {
			t.Fatalf("Encode(%T) accepted an incomplete protobuf shape", msg)
		}
	}
}
