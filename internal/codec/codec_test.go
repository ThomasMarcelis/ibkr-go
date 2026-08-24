package codec

import (
	"errors"
	"slices"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func TestDecodeSupportedFloorClassicControlFrames(t *testing.T) {
	t.Parallel()

	// Exact sv208 frames from capture
	// 20260824T213929Z-supported_version_matrix_paper, events SHA-256
	// 64ee4350f0bde347a9da914a82865e88e0a68d06924cb13335fd2084595a7727.
	tests := []struct {
		name    string
		payload []byte
		want    Message
	}{
		{name: "next valid ID", payload: []byte{0, 0, 0, 9, '1', 0, '5', '8', '1', 0}, want: NextValidID{OrderID: 581}},
		{name: "current time", payload: []byte{0, 0, 0, 49, '1', 0, '1', '7', '8', '7', '6', '0', '7', '5', '6', '9', 0}, want: CurrentTime{Time: "1787607569"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decode(208, tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Decode() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestDecodeUnknownClassicBodyRemainsObservable(t *testing.T) {
	t.Parallel()

	payload := mustEncodeClassicEnvelope(t, 199, []string{"first", "last"})
	msgs, err := DecodeBatch(208, payload[:len(payload)-1])
	if err != nil || len(msgs) != 1 {
		t.Fatalf("DecodeBatch() = %v, messages=%d", err, len(msgs))
	}
	unknown, ok := msgs[0].(UnknownInbound)
	if !ok || unknown.MsgID != 199 || unknown.Encoding != protocol.ClassicBody {
		t.Fatalf("DecodeBatch() message = %#v, want classic UnknownInbound", msgs[0])
	}
	if want := []string{"first", "last"}; !slices.Equal(unknown.Fields, want) {
		t.Fatalf("UnknownInbound.Fields = %q, want %q", unknown.Fields, want)
	}
}

func TestMalformedRegisteredClassicBodyIsObservable(t *testing.T) {
	t.Parallel()

	payload := mustEncodeClassicEnvelope(t, protocol.InCurrentTime, []string{"1", "1712345678"})
	msgs, err := DecodeBatch(208, payload[:len(payload)-1])
	malformed := requireMalformedInbound(t, msgs, err)
	if malformed.MsgID != protocol.InCurrentTime || !errors.Is(malformed.Err, wire.ErrMalformedFrame) {
		t.Fatalf("MalformedInbound = %+v", malformed)
	}
}

func TestDecodeMalformedRawMessageID(t *testing.T) {
	t.Parallel()

	for _, payload := range [][]byte{{0, 0, 0}, {0, 0, 0, 0}, {0x80, 0, 0, 0}} {
		if _, err := DecodeBatch(208, payload); !errors.Is(err, wire.ErrMalformedFrame) {
			t.Fatalf("DecodeBatch(%x) error = %v, want ErrMalformedFrame", payload, err)
		}
	}
}

func TestInboundMessagesAreNotOutbound(t *testing.T) {
	t.Parallel()

	for _, msg := range []Message{
		APIError{}, NextValidID{}, ManagedAccounts{}, OrderStatus{}, OpenOrder{},
		ExecutionDetail{}, OpenOrderEnd{}, ExecutionsEnd{}, CommissionReport{},
		CompletedOrder{}, CompletedOrderEnd{}, UnknownInbound{},
	} {
		if _, ok := msg.(OutboundMessage); ok {
			t.Fatalf("%T satisfies OutboundMessage", msg)
		}
	}
}

func TestDecodeServerInfo(t *testing.T) {
	t.Parallel()

	info, err := DecodeServerInfo([]byte("225\x0020260824 23:39:29 CET\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if info.ServerVersion != 225 || info.ConnectionTime != "20260824 23:39:29 CET" {
		t.Fatalf("DecodeServerInfo() = %+v", info)
	}
}

func mustEncodeClassicEnvelope(t *testing.T, msgID int, fields []string) []byte {
	t.Helper()
	payload, err := protocol.EncodeClassicEnvelope(208, msgID, fields)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func requireMalformedInbound(t *testing.T, msgs []Message, err error) MalformedInbound {
	t.Helper()
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v, want message-scoped malformed result", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("DecodeBatch() returned %d messages, want 1", len(msgs))
	}
	malformed, ok := msgs[0].(MalformedInbound)
	if !ok {
		t.Fatalf("DecodeBatch() message = %T, want MalformedInbound", msgs[0])
	}
	return malformed
}
