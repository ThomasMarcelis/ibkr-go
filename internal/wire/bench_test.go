package wire

import (
	"bytes"
	"encoding/base64"
	"runtime"
	"testing"
)

// Hot-path framing benchmarks use exact server_version 208 or 225 Gateway
// frames. The sv208 classic CurrentTime body keeps ParseFields and EncodeFields
// representative of the classic bodies still reachable at the supported
// floor. The protobuf mix comes from current sv225 public replay captures.

// Capture 20260824T213929Z-supported_version_matrix_paper, events SHA-256
// 64ee4350f0bde347a9da914a82865e88e0a68d06924cb13335fd2084595a7727.
var benchClassicPayload = []byte("1\x001787607569\x00")

// The mix includes bootstrap, farm status, quote price/size, account summary,
// and an open order. The source captures and hashes are retained in the
// corresponding tracked transcripts.
var benchSessionPayloads = [][]byte{
	mustBenchPayload("AAAADwAAANcKCURVOTAwMDAwMQ=="),
	mustBenchPayload("AAAABgAAANEIAQ=="),
	mustBenchPayload("AAAAQwAAAMwI////////////ARC2+sWrgzQYuBAiKE1hcmtldCBkYXRhIGZhcm0gY29ubmVjdGlvbiBpcyBPSzp1c2Zhcm0="),
	mustBenchPayload("AAAAGAAAAMkIARBEGc3MzMzMaHNAIgMxNDEoAA=="),
	mustBenchPayload("AAAAEAAAAMoIARBKGgY4MTIyNTQ="),
	mustBenchPayload("AAABOgAAAM0I5gMSLwj+mhASBEFBUEwaA1NUSykAAAAAAAAAAEIFU01BUlRSA1VTRFoEQUFQTGIDTk1TGu8BCAEQ5gMY5tWTrQMgACoDQlVZMgExQgNNS1RJAAAAAAAAAABRAAAAAAAAAABaA0RBWWIJRFU5MDAwMDAxegJJQrkBXI/C9Sh8c0DiASRzYW5pdGl6ZWQtb3JkZXItcmVmLTAwMDAwMDAwMDAwMDAwMDbwAQP4AQCwAgDAAgDKAgROb25l8AIAqAQAsAQAwAT///////////8B6gUETm9uZZAGAPgGAZoHATCyBylOb3QgYW4gaW5zaWRlciBvciBzdWJzdGFudGlhbCBzaGFyZWhvbGRlcsAHANAHAMoIDXBhcGVyLXVzZXItMDHwCAAiDgoMUHJlU3VibWl0dGVk"),
}

func mustBenchPayload(encoded string) []byte {
	framed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		panic("wire benchmark: decode captured frame: " + err.Error())
	}
	payload, err := ReadFrame(bytes.NewReader(framed))
	if err != nil {
		panic("wire benchmark: parse captured frame: " + err.Error())
	}
	return payload
}

func BenchmarkReadFrame(b *testing.B) {
	for i, payload := range benchSessionPayloads {
		name := []string{
			"managed_accounts_sv225",
			"next_valid_id_sv225",
			"farm_status_sv225",
			"tick_price_sv225",
			"tick_size_sv225",
			"open_order_sv225",
		}[i]
		framed, err := EncodeFrame(payload)
		if err != nil {
			b.Fatalf("EncodeFrame() %s error = %v", name, err)
		}
		b.Run(name, func(b *testing.B) {
			r := bytes.NewReader(framed)
			got, err := ReadFrame(r)
			if err != nil || !bytes.Equal(got, payload) || r.Len() != 0 {
				b.Fatalf("ReadFrame() preflight: bytes_equal=%t trailing=%d err=%v", bytes.Equal(got, payload), r.Len(), err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(framed)))
			for b.Loop() {
				r.Reset(framed)
				if _, err := ReadFrame(r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	b.Run("session_mix_batch_6", func(b *testing.B) {
		var streamBuf bytes.Buffer
		for i, payload := range benchSessionPayloads {
			if err := WriteFrame(&streamBuf, payload); err != nil {
				b.Fatalf("WriteFrame() frame %d error = %v", i, err)
			}
		}
		stream := streamBuf.Bytes()

		r := bytes.NewReader(stream)
		for i, want := range benchSessionPayloads {
			got, err := ReadFrame(r)
			if err != nil {
				b.Fatalf("ReadFrame() frame %d error = %v", i, err)
			}
			if !bytes.Equal(got, want) {
				b.Fatalf("ReadFrame() frame %d differs from captured payload", i)
			}
		}
		if r.Len() != 0 {
			b.Fatalf("stream has %d trailing bytes after %d frames", r.Len(), len(benchSessionPayloads))
		}

		b.ReportAllocs()
		b.SetBytes(int64(len(stream)))
		for b.Loop() {
			r.Reset(stream)
			for range benchSessionPayloads {
				if _, err := ReadFrame(r); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

// BenchmarkParseFields is a wire-helper diagnostic for a captured classic
// body. Production inbound classic decoding uses codec.fieldReader directly;
// this result must not be cited as production decoder performance.
func BenchmarkParseFields(b *testing.B) {
	fields, err := ParseFields(benchClassicPayload)
	if err != nil {
		b.Fatalf("ParseFields() error = %v", err)
	}
	if len(fields) != 2 || fields[0] != "1" || fields[1] != "1787607569" {
		b.Fatalf("ParseFields() = %q, want captured sv208 CurrentTime body", fields)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchClassicPayload)))
	for b.Loop() {
		if _, err := ParseFields(benchClassicPayload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeFields(b *testing.B) {
	fields, err := ParseFields(benchClassicPayload)
	if err != nil {
		b.Fatalf("ParseFields() error = %v", err)
	}
	if got := EncodeFields(fields); !bytes.Equal(got, benchClassicPayload) {
		b.Fatalf("EncodeFields() does not round-trip the captured classic body")
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchClassicPayload)))
	var checksum byte
	for b.Loop() {
		encoded := EncodeFields(fields)
		checksum ^= encoded[len(encoded)-1]
	}
	runtime.KeepAlive(checksum)
}
