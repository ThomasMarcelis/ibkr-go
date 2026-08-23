package codec

import (
	"bytes"
	"reflect"
	"testing"
)

// API 10.48.01 source-law vectors. Order.proto SHA-256:
// 3a963d252987b4a1450d6ded1901f46fdbad16039b6f717dc8beb597e78695c8.
// Related schema SHA-256 values: OrderCondition a0a1f6bba0620e59522d8e2d62a90bfd434ea8bbcdd511178025769723c4c76f,
// ComboLeg 9d80a4a8ab9ab8b1b6e1eef8b79ef0cc0f8959f1f0a630d4ddfb7a82431549aa,
// OrderState 63b79d7d3de82591a61a1f0f9699edae93e8be4ef74a430fffff2ac2cf4d247e,
// and OrderAllocation 571f8191500e64cdabd5166cd92c52f98cef4cc3164a4e76f74eec6a44bf7c60.
func TestProtoOrderConditionSourceVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want OrderCondition
		hex  string
	}{
		{"price", OrderCondition{Type: 1, Conjunction: "o", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "123.5", TriggerMethod: 4}, "08011000180120fe9a102a05534d415254510000000000e05e405804"},
		{"time", OrderCondition{Type: 3, Conjunction: "a", Operator: 1, Value: "20260710 16:00:00 US/Eastern"}, "080310011800621c32303236303731302031363a30303a30302055532f4561737465726e"},
		{"margin", OrderCondition{Type: 4, Conjunction: "o", Operator: 2, Value: "35"}, "0804100018014023"},
		{"execution", OrderCondition{Type: 5, Conjunction: "a", Exchange: "NASDAQ", Symbol: "AAPL", SecType: "STK"}, "080510012a064e415344415132044141504c3a0353544b"},
		{"volume", OrderCondition{Type: 6, Conjunction: "o", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "100000"}, "08061000180120fe9a102a05534d41525468a08d06"},
		{"percent change", OrderCondition{Type: 7, Conjunction: "a", Operator: 1, ConID: 265598, Exchange: "SMART", Value: "2.5"}, "08071001180020fe9a102a05534d415254490000000000000440"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wantBytes := decodeHex(t, tc.hex)
			gotBytes, err := encodeOrderConditionProto(tc.want)
			if err != nil {
				t.Fatalf("encodeOrderConditionProto() error = %v", err)
			}
			if !bytes.Equal(gotBytes, wantBytes) {
				t.Fatalf("encoded condition = %x, want schema vector %x", gotBytes, wantBytes)
			}
			got, err := decodeOrderConditionProto(wantBytes)
			if err != nil {
				t.Fatalf("decodeOrderConditionProto() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("decoded condition = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestProtoComboLegSourceVector(t *testing.T) {
	t.Parallel()

	wantLeg := ComboLeg{ConID: 265598, Ratio: 2, Action: "SELL", Exchange: "SMART", OpenClose: "1", ShortSaleSlot: "2", DesignatedLocation: "NYSE", ExemptCode: "-1"}
	wantBytes := decodeHex(t, "08fe9a1010021a0453454c4c2205534d415254280130023a044e59534540ffffffffffffffffff01490000000000002940")
	gotBytes, err := encodeComboLegProto(wantLeg, "12.5")
	if err != nil {
		t.Fatalf("encodeComboLegProto() error = %v", err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatalf("encoded combo leg = %x, want schema vector %x", gotBytes, wantBytes)
	}
	gotLeg, gotPrice, err := decodeComboLegProto(wantBytes)
	if err != nil {
		t.Fatalf("decodeComboLegProto() error = %v", err)
	}
	if !reflect.DeepEqual(gotLeg, wantLeg) || gotPrice != "12.5" {
		t.Fatalf("decoded combo leg = %#v at %q, want %#v at 12.5", gotLeg, gotPrice, wantLeg)
	}
}

func TestProtoOrderAllocationSourceVector(t *testing.T) {
	t.Parallel()

	want := OrderAllocation{Account: "DU9000001", Position: "-10", PositionDesired: "5", PositionAfter: "-5", DesiredAllocQty: "4.5", AllowedAllocQty: "4", IsMonetary: "1"}
	got, err := decodeOrderAllocationProto(decodeHex(t, "0a0944553930303030303112032d31301a013522022d352a03342e353201343801"))
	if err != nil {
		t.Fatalf("decodeOrderAllocationProto() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded allocation = %#v, want %#v", got, want)
	}
}

func TestProtoTransmitFalseOmission(t *testing.T) {
	t.Parallel()

	got, err := encodeOrderProto(PlaceOrderRequest{Transmit: "0"})
	if err != nil {
		t.Fatalf("encodeOrderProto() error = %v", err)
	}
	// Order.proto field 66 is optional bool. EClientUtils.cpp emits it only
	// when true; field 105's empty SoftDollarTier remains mandatory.
	if want := decodeHex(t, "ca0600"); !bytes.Equal(got, want) {
		t.Fatalf("Transmit=false order = %x, want omitted field 66 in %x", got, want)
	}
}
