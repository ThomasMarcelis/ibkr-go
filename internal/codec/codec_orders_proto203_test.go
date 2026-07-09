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
			// Exact client frame from the sanitized exact-sv203 live capture.
			// Raw ID 203 is base ID 3 plus the protobuf discriminator 200.
			want: "000000cb0801121712044141504c1a0353544b4205534d41525452035553441a3d20002a03425559320131380042034c4d544900000000000049405a03444159f00100f80100900401a80400c004ffffffffffffffffff01900600ca06002200",
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

func TestDecodeServer203OrderCallbacks(t *testing.T) {
	t.Parallel()

	// Exact callbacks from the sanitized exact-sv203 paper capture frozen in
	// order_lifecycle_sv203_live.txt. These retain every live protobuf field;
	// only account/client/permanent/submitter identifiers were substituted.
	openMessage, err := Decode(203, recordedPayload(t, "AAABEQAAAM0IARIvCP6aEBIEQUFQTBoDU1RLKQAAAAAAAAAAQgVTTUFSVFIDVVNEWgRBQVBMYgNOTVMaxwEIARABGIHSk60DIAAqA0JVWTIBMUIDTE1USQAAAAAAAElAUQAAAAAAAAAAWgNEQVliCURVOTAwMDAwMXoCSUK5AQAAAAAAgElA8AED+AEAsAIAwAIAygIETm9uZfACAKgEALAEAMAE////////////AeoFBE5vbmWQBgD4BgGaBwEwsgcpTm90IGFuIGluc2lkZXIgb3Igc3Vic3RhbnRpYWwgc2hhcmVob2xkZXLABwDQBwDKCA1wYXBlcnRyYWRlcjAx8AgAIg4KDFByZVN1Ym1pdHRlZA=="))
	if err != nil {
		t.Fatalf("Decode(open order) error = %v", err)
	}
	openOrder, ok := openMessage.(OpenOrder)
	if !ok {
		t.Fatalf("Decode(open order) = %T", openMessage)
	}
	if openOrder.OrderID != 1 || openOrder.Contract.ConID != 265598 || openOrder.Contract.Symbol != "AAPL" ||
		openOrder.Contract.Strike != "0" || openOrder.Action != "BUY" || openOrder.Quantity != "1" ||
		openOrder.OrderType != "LMT" || openOrder.LmtPrice != "50" || openOrder.Account != "DU9000001" ||
		openOrder.ClientID != "1" || openOrder.PermID != "900000001" || openOrder.Status != "PreSubmitted" {
		t.Fatalf("open order = %+v", openOrder)
	}

	statusMessage, err := Decode(203, recordedPayload(t, "AAAAPwAAAMsIARIMUHJlU3VibWl0dGVkGgEwIgExKQAAAAAAAAAAMIHSk60DOABBAAAAAAAAAABIAVkAAAAAAAAAAA=="))
	if err != nil {
		t.Fatalf("Decode(order status) error = %v", err)
	}
	status, ok := statusMessage.(OrderStatus)
	if !ok || status.OrderID != 1 || status.Status != "PreSubmitted" || status.Filled != "0" ||
		status.Remaining != "1" || status.PermID != "900000001" || status.ClientID != "1" {
		t.Fatalf("order status = %#v", statusMessage)
	}

	for _, tc := range []struct {
		name string
		data string
		code int
	}{
		{"targeted cancel", "AAAAKgAAAMwIARDo+/HJ9DMYygEiGE9yZGVyIENhbmNlbGVkIC0gcmVhc29uOg==", 202},
		{"global cancel after terminal", "AAAAZQAAAMwIARDo+/HJ9DMYoQEiU0NhbmNlbCBhdHRlbXB0ZWQgd2hlbiBvcmRlciBpcyBub3QgaW4gYSBjYW5jZWxsYWJsZSBzdGF0ZS4gIE9yZGVyIHBlcm1JZCA9OTAwMDAwMDAx", 161},
	} {
		message, err := Decode(203, recordedPayload(t, tc.data))
		if err != nil {
			t.Fatalf("Decode(%s error) = %v", tc.name, err)
		}
		apiError, ok := message.(APIError)
		if !ok || apiError.ReqID != 1 || apiError.Code != tc.code || apiError.ErrorTimeMs != "1783640129000" {
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
