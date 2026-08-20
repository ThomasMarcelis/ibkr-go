package codec

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestIncludeOvernightProtobufField135(t *testing.T) {
	t.Parallel()

	encoded, err := encodeOrderProto(PlaceOrderRequest{IncludeOvernight: "1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xca, 0x06, 0x00, 0xb8, 0x08, 0x01}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("include overnight = %x, want protobuf field 135 %x", encoded, want)
	}

	body := protowire.AppendTag(nil, 135, protowire.VarintType)
	body = protowire.AppendVarint(body, 1)
	var decoded OrderDetails
	if err := decodeOrderDetailsProto(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.IncludeOvernight != "1" {
		t.Fatalf("decoded include overnight = %q, want 1", decoded.IncludeOvernight)
	}
}

func TestIncludeOvernightClassicOpenOrderRoundTrip(t *testing.T) {
	t.Parallel()

	want := OpenOrder{
		OrderID: 42,
		OrderDetails: OrderDetails{
			OrderID: "42",
			Contract: Contract{
				ConID: 265598, Symbol: "AAPL", SecType: "STK", Strike: "0",
				Exchange: "SMART", Currency: "USD", LocalSymbol: "AAPL", TradingClass: "AAPL",
			},
			Account: "DU12345", Action: "BUY", Quantity: "1", OrderType: "LMT",
			LmtPrice: "150", AuxPrice: "0", TIF: "DAY", Origin: "0",
			ClientID: "20", PermID: "123456", OutsideRTH: "0", IncludeOvernight: "1",
			Hidden: "0", DiscretionAmt: "0", AuctionStrategy: "0",
			DeltaNeutralOrderType: "None", DeltaNeutralConID: "0",
			DeltaNeutralShortSale: "0", DeltaNeutralShortSaleSlot: "0", Status: "Submitted",
		},
		Status: "Submitted",
	}
	payload, err := Encode(200, want)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("decoded messages = %d, want 1", len(messages))
	}
	got, ok := messages[0].(OpenOrder)
	if !ok {
		t.Fatalf("decoded message = %T, want OpenOrder", messages[0])
	}
	if got.IncludeOvernight != "1" {
		t.Fatalf("classic include overnight = %q, want 1", got.IncludeOvernight)
	}
}
