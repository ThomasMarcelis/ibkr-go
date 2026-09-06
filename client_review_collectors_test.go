package ibkr_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/binary"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/testhost"
)

func TestPositionsMultiOneShotCaptured(t *testing.T) {
	client, host := newClient(t, "positions_multi.txt")
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	values, err := client.Accounts().PositionsMulti(ctx, ibkr.PositionsMultiRequest{Account: "DU9000001"})
	if err != nil || len(values) != 19 {
		t.Fatalf("PositionsMulti = %d rows, %v", len(values), err)
	}
	if first := values[0]; first.Contract.Symbol != "MELI" || first.Position.String() != "1" || first.AvgCost.String() != "1581.09" {
		t.Fatalf("first position = %+v", first)
	}
	if values[18].Contract.Symbol != "UBER" || values[18].Position.String() != "35" {
		t.Fatalf("last position = %+v", values[18])
	}
}

func TestScannerParametersPublicCaptured(t *testing.T) {
	// Compose the retained sv225 bootstrap with the scanner request/response
	// from 20260824T202844Z-scanner_parameters, events.jsonl SHA-256
	// 02ca289379189356eacedc56576be6d863e4a2b46feb8c28386c36ac07948ba7.
	// The compressed payload is shared with the direct decoder attestation.
	bootstrap, err := os.ReadFile("testdata/transcripts/grounded_bootstrap.txt")
	if err != nil {
		t.Fatal(err)
	}
	script, _, ok := strings.Cut(string(bootstrap), "sleep ")
	if !ok {
		t.Fatal("bootstrap fixture has no completion point")
	}
	compressed, err := os.ReadFile("internal/codec/testdata/scanner_parameters_sv225.gz")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	frame := binary.BigEndian.AppendUint32(nil, uint32(len(body))) // #nosec G115 -- fixed 1.8 MB captured payload
	frame = append(frame, body...)
	script += "raw client AAAABAAAAOA=\nraw server " + base64.StdEncoding.EncodeToString(frame) + "\n"
	host, err := testhost.New(script)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })
	client := dialHostClient(t, host)
	defer cleanupClientHost(t, client, host)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	xml, err := client.Scanner().Parameters(ctx)
	if err != nil || len(xml) != 1_801_494 || !strings.HasPrefix(string(xml), "<?xml version=") {
		t.Fatalf("Parameters = %d bytes, %v", len(xml), err)
	}
}
