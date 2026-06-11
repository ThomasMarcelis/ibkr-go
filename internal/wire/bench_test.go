package wire

import (
	"bytes"
	"testing"
)

// Hot-path framing benchmarks. Every input below is a live IB Gateway frame
// (server_version 200) re-embedded verbatim from the capture-decode fixtures
// in internal/codec/codec_capture_test.go, with the 4-byte length prefix
// stripped. Real wire bytes keep ns/op and B/op representative of production
// framing work; keep these literals in sync with the capture suite.

// benchOpenOrderPayload is the live openOrder frame from
// captures/20260405T215248Z-open_orders_all, line 10 (OBDC PUT option,
// PreSubmitted): the genuine 928-byte frame, 156 NUL-delimited fields. Same bytes as
// TestCaptureDecode_OpenOrder in internal/codec/codec_capture_test.go. It is
// the largest single frame in the capture suite's order flows, so it anchors
// the ParseFields/EncodeFields benchmarks.
var benchOpenOrderPayload = []byte(
	"5\x000\x00853200900\x00OBDC\x00OPT\x0020261120\x0010\x00P\x00100\x00" +
		"SMART\x00USD\x00OBDC  261120P00010000\x00OBDC\x00SELL\x001\x00LMT\x00" +
		"1.2\x000.0\x00GTC\x00\x00DU9000001\x00\x000\x00\x000\x009000\x00" +
		"0\x000\x000\x00\x009000.1/DU9000001/100\x00\x00\x00\x00\x00\x00" +
		"0\x00\x00\x000\x00\x00-1\x000\x00\x00\x00\x00\x00\x002147483647\x00" +
		"0\x000\x000\x00\x003\x000\x000\x00\x000\x000\x00\x000\x00None\x00\x00" +
		"0\x00\x00\x00\x00?\x000\x000\x00\x000\x000\x00\x00\x00\x00\x00\x00" +
		"0\x000\x000\x002147483647\x002147483647\x00\x00\x000\x00\x00IB\x00" +
		"0\x000\x00\x000\x000\x00PreSubmitted\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00\x00\x00\x00\x00" +
		"\x001.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x00-9223372036854775808\x00\x000\x00\x000\x00" +
		"0\x000\x00None\x001.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x000\x00\x00\x00\x00" +
		"0\x001\x000\x000\x000\x00\x00\x000\x00\x00\x00\x00\x00\x00\x000\x00" +
		"\x000\x00\x002147483647\x00\x000\x00")

// benchSessionPayloads is a realistic bootstrap -> quote snapshot -> account
// summary -> open order frame mix: many small frames plus one fat-tail order
// frame, the shape a live session actually pushes through ReadFrame. Sources
// (all in internal/codec/codec_capture_test.go):
//   - managed_accounts, next_valid_id, api_error 2104:
//     captures/20260405T214926Z-bootstrap, lines 6-7
//   - market_data_type, tick_price (tickType 68), tick_size (tickType 74),
//     tick_snapshot_end: captures/20260405T215734Z-quote_snapshot_aapl,
//     lines 11-18
//   - account summary value + end:
//     captures/20260405T215025Z-account_summary_snapshot, lines 10-11
//   - the openOrder frame above.
var benchSessionPayloads = [][]byte{
	[]byte("15\x001\x00DU9000001\x00"),
	[]byte("9\x001\x001\x00"),
	[]byte("4\x00-1\x002104\x00Market data farm connection is OK:usfarm\x00\x001775425766350\x00"),
	[]byte("58\x001\x001001\x003\x00"),
	[]byte("1\x006\x001001\x0068\x00255.45\x00200\x000\x00"),
	[]byte("2\x006\x001001\x0074\x00312894\x00"),
	[]byte("57\x001\x001001\x00"),
	[]byte("63\x001\x001001\x00DU9000001\x00BuyingPower\x00300000.00\x00EUR\x00"),
	[]byte("64\x001\x001001\x00"),
	benchOpenOrderPayload,
}

func BenchmarkReadFrame(b *testing.B) {
	// Build the length-prefixed stream once, from EncodeFields of the real
	// field slices; each iteration replays the whole stream from a reset
	// bytes.Reader.
	var streamBuf bytes.Buffer
	for i, payload := range benchSessionPayloads {
		fields, err := ParseFields(payload)
		if err != nil {
			b.Fatalf("ParseFields() frame %d error = %v", i, err)
		}
		if err := WriteFrame(&streamBuf, EncodeFields(fields)); err != nil {
			b.Fatalf("WriteFrame() frame %d error = %v", i, err)
		}
	}
	stream := streamBuf.Bytes()

	// Verify once that the stream replays every source frame intact, so a
	// broken input fails loudly instead of benchmarking garbage.
	r := bytes.NewReader(stream)
	for i, want := range benchSessionPayloads {
		got, err := ReadFrame(r)
		if err != nil {
			b.Fatalf("ReadFrame() frame %d error = %v", i, err)
		}
		if !bytes.Equal(got, want) {
			b.Fatalf("ReadFrame() frame %d = %q, want %q", i, got, want)
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
}

func BenchmarkParseFields(b *testing.B) {
	fields, err := ParseFields(benchOpenOrderPayload)
	if err != nil {
		b.Fatalf("ParseFields() error = %v", err)
	}
	if len(fields) != 156 || fields[0] != "5" {
		b.Fatalf("ParseFields() = %d fields starting %q, want 170 starting %q", len(fields), fields[0], "5")
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchOpenOrderPayload)))
	for b.Loop() {
		if _, err := ParseFields(benchOpenOrderPayload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeFields(b *testing.B) {
	fields, err := ParseFields(benchOpenOrderPayload)
	if err != nil {
		b.Fatalf("ParseFields() error = %v", err)
	}
	if got := EncodeFields(fields); !bytes.Equal(got, benchOpenOrderPayload) {
		b.Fatalf("EncodeFields() does not round-trip the live frame: %d bytes, want %d", len(got), len(benchOpenOrderPayload))
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchOpenOrderPayload)))
	for b.Loop() {
		EncodeFields(fields)
	}
}
