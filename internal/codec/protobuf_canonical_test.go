package codec

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestCanonicalProtoFieldsReturnsCanonicalInput(t *testing.T) {
	t.Parallel()

	body := appendProtoVarint(nil, 1, 7)
	body = appendProtoVarint(body, 1, 8)
	body = appendProtoString(body, 2, "captured")
	got := canonicalProtoFields(body)
	if !bytes.Equal(got, body) {
		t.Fatalf("canonicalProtoFields() = %x, want %x", got, body)
	}
	if &got[0] != &body[0] {
		t.Fatal("canonicalProtoFields() copied an already canonical body")
	}

	empty := make([]byte, 0, 16)
	if got := canonicalProtoFields(empty); len(got) != 0 || cap(got) != cap(empty) {
		t.Fatalf("canonicalProtoFields(empty) = len %d cap %d, want len 0 cap %d", len(got), cap(got), cap(empty))
	}
}

func TestCanonicalProtoFieldsSortsStably(t *testing.T) {
	t.Parallel()

	body := appendProtoString(nil, 3, "first-three")
	body = appendProtoVarint(body, 1, 7)
	body = appendProtoString(body, 3, "second-three")
	body = appendProtoVarint(body, 2, 8)

	want := appendProtoVarint(nil, 1, 7)
	want = appendProtoVarint(want, 2, 8)
	want = appendProtoString(want, 3, "first-three")
	want = appendProtoString(want, 3, "second-three")
	if got := canonicalProtoFields(body); !bytes.Equal(got, want) {
		t.Fatalf("canonicalProtoFields() = %x, want stable ordering %x", got, want)
	}
}

func TestCanonicalProtoFieldsPanicsOnMalformedBody(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("canonicalProtoFields() accepted a truncated varint")
		}
	}()
	canonicalProtoFields([]byte{0x08, 0x80})
}

var canonicalProtoFieldsBenchmarkResult []byte

func BenchmarkCanonicalProtoFields(b *testing.B) {
	body := appendProtoVarint(nil, 1, 1)
	body = appendProtoString(body, 2, "AAPL")
	body = appendProtoString(body, 3, "STK")
	body = appendProtoString(body, 8, "SMART")
	body = appendProtoString(body, 10, "USD")

	b.ReportAllocs()
	for b.Loop() {
		canonicalProtoFieldsBenchmarkResult = canonicalProtoFields(body)
	}
}

func BenchmarkEncodeCanonicalProtoMessages(b *testing.B) {
	tests := []struct {
		name string
		sv   int
		msg  OutboundMessage
		want string
	}{
		{
			name: "captured_quote",
			sv:   225,
			msg: QuoteRequest{
				ReqID: 1,
				Contract: Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD",
				},
			},
			// Capture 20260824T202345Z-api_duplicate_quote_subscriptions_aapl,
			// events.jsonl SHA-256
			// 1fbb60beec41483729e2f9e7c96b1bfdd89649810ffdc5e7e4a4077c1eb8b290.
			want: "000000c90801121b08fe9a1012044141504c1a0353544b4205534d4152545203555344",
		},
		{
			name: "place_order_source_vector",
			sv:   208,
			msg: PlaceOrderRequest{
				OrderID:  1,
				Contract: Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
				Action:   "BUY", TotalQuantity: "1", DisplaySize: "0", OrderType: "LMT",
				LmtPrice: "50", TIF: "DAY", Transmit: "1", ParentID: "0", Origin: "0",
				OcaType: "0", TriggerMethod: "0", ExemptCode: "-1", AdjustableTrailingUnit: "0",
			},
			// Official API 10.48.01 source-law vector also frozen by
			// TestEncodePlaceOrderRequestVectors.
			want: "000000cb08011219080012044141504c1a0353544b4205534d41525452035553441a3d20002a03425559320131380042034c4d544900000000000049405a03444159f00100f80100900401a80400c004ffffffffffffffffff01900600ca06002200",
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			want, err := hex.DecodeString(test.want)
			if err != nil {
				b.Fatal(err)
			}
			got, err := Encode(test.sv, test.msg)
			if err != nil {
				b.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				b.Fatalf("Encode() = %x, want %x", got, want)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				canonicalProtoFieldsBenchmarkResult, err = Encode(test.sv, test.msg)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
