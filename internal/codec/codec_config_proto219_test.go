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

func TestDecodeConfigResponseProto225ExactLiveFrame(t *testing.T) {
	t.Parallel()

	// Capture 20260824T202907Z-tws_config, events.jsonl SHA-256
	// 2ce43d03432505101804957e7b58702439d27887ad24f42b923b48e88563be1b.
	// The compressed literal expands to the exact length-prefix-free server frame.
	frame := decodeGzipBase64(t, "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")
	decoded, err := Decode(225, frame)
	if err != nil {
		t.Fatal(err)
	}
	config, ok := decoded.(ConfigResponse)
	if !ok {
		t.Fatalf("Decode() = %T, want ConfigResponse", decoded)
	}
	if config.ReqID != 1 || config.LockAndExit == nil || config.LockAndExit.AutoLogoffTime == nil || *config.LockAndExit.AutoLogoffTime != "11:00" {
		t.Fatalf("config identity/lock-and-exit = %+v", config)
	}
	if len(config.Messages) < 10 {
		t.Fatalf("message configurations = %d, want current full response", len(config.Messages))
	}
	if config.API == nil || config.API.Settings == nil {
		t.Fatalf("API configuration = %+v", config.API)
	}
	settings := config.API.Settings
	if settings.ReadOnlyAPI == nil || !*settings.ReadOnlyAPI || settings.SocketPort == nil || *settings.SocketPort != 4001 || settings.LoggingLevel == nil || *settings.LoggingLevel != "error" {
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
