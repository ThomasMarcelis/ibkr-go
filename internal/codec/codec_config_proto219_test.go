package codec

import (
	"bytes"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

func TestEncodeConfigRequestProto219LiveVector(t *testing.T) {
	t.Parallel()

	got, err := Encode(219, ConfigRequest{ReqID: 7901})
	if err != nil {
		t.Fatal(err)
	}
	want := decodeHex(t, "0000013408dd3d")
	if !bytes.Equal(got, want) {
		t.Fatalf("Encode() = %x, want %x", got, want)
	}

	envelope, err := protocol.DecodeEnvelope(219, got)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.MsgID != protocol.OutReqConfig || envelope.WireID != protocol.OutReqConfig+protocol.ProtobufMessageID {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func TestDecodeConfigResponseProto219LiveVector(t *testing.T) {
	t.Parallel()

	// Sanitized subset of the exact live API 10.48.01 / server_version 219
	// response captured 2026-07-13. Capture SHA-256:
	// 928ada9da43be6e71f18c31f9d3f69a07e6decadfbef33ef0e6cc7a8eb01253b.
	frame := decodeHex(t, "0000013608dd3d12130a0531313a30301202504d1a066c6f676f66661a3c088d0312305468652041504920697320696e20526561642d4f6e6c79206d6f646520696e666f726d6174696f6e206d65737361676522035965732800221f0a0208001219080140a11f8a01056572726f72a202093132372e302e302e312a040a020800")
	decoded, err := Decode(219, frame)
	if err != nil {
		t.Fatal(err)
	}
	config, ok := decoded.(ConfigResponse)
	if !ok {
		t.Fatalf("Decode() = %T, want ConfigResponse", decoded)
	}
	if config.ReqID != 7901 || config.LockAndExit == nil || config.LockAndExit.AutoLogoffTime == nil || *config.LockAndExit.AutoLogoffTime != "11:00" {
		t.Fatalf("config identity/lock-and-exit = %+v", config)
	}
	if len(config.Messages) != 1 || config.Messages[0].ID == nil || *config.Messages[0].ID != 397 || config.Messages[0].Enabled == nil || *config.Messages[0].Enabled {
		t.Fatalf("message config = %+v", config.Messages)
	}
	if config.API == nil || config.API.Precautions == nil || config.API.Precautions.BypassOrderPrecautions == nil || *config.API.Precautions.BypassOrderPrecautions {
		t.Fatalf("API precautions = %+v", config.API)
	}
	settings := config.API.Settings
	if settings == nil || settings.ReadOnlyAPI == nil || !*settings.ReadOnlyAPI || settings.SocketPort == nil || *settings.SocketPort != 4001 || settings.LoggingLevel == nil || *settings.LoggingLevel != "error" {
		t.Fatalf("API settings = %+v", settings)
	}
	if len(settings.TrustedIPs) != 1 || settings.TrustedIPs[0] != "127.0.0.1" {
		t.Fatalf("trusted IPs = %v", settings.TrustedIPs)
	}
	if config.Orders == nil || config.Orders.SmartRouting == nil || config.Orders.SmartRouting.SeekPriceImprovement == nil || *config.Orders.SmartRouting.SeekPriceImprovement {
		t.Fatalf("order config = %+v", config.Orders)
	}
}

func TestConfigProto219Boundary(t *testing.T) {
	t.Parallel()

	if _, err := Encode(218, ConfigRequest{ReqID: 1}); err == nil {
		t.Fatal("Encode(218) accepted a v219-only config request")
	}
}
