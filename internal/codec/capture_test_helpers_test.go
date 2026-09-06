package codec

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"os"
	"testing"
)

func decodeGzipBase64(t *testing.T, value string) []byte {
	t.Helper()
	compressed, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func readCapturedGzip(t *testing.T, path string) []byte {
	t.Helper()
	// #nosec G304 -- fixed test-owned capture paths, never input from the wire.
	compressed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
