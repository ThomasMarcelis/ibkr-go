package ibkr

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

// TestScannerNoItemsMessagePreservesLiveRoute freezes the exact server-side
// sequence from /tmp/ibkr-subscription-audit-exact/
// 20260710T232512Z-scanner_subscription, server_version 200, events.jsonl
// sha256 14cdc2913735bb3c3839beff106ab0524f5b8d509340a49d8d07470e39728e7e.
// The old raw runner sent its cancel between the two inbound frames. Replaying
// the exact inbound order before public Close proves code 165 must leave the
// route alive so the already-sent empty ScannerData result reaches the caller.
func TestScannerNoItemsMessagePreservesLiveRoute(t *testing.T) {
	t.Parallel()

	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 200
	e.nextReqID = 1001
	sub := installObservedScannerRoute(t, e)

	requestFields := bytes.Split(readObservedFrame(t, peer), []byte{0})
	if len(requestFields) < 2 || string(requestFields[0]) != "22" || string(requestFields[1]) != "1001" {
		t.Fatalf("scanner request identity fields = %q, want [22 1001]", requestFields[:min(2, len(requestFields))])
	}

	e.handleIncoming(decodeLiveScannerFrame(t, "AAAAWjQAMTAwMQAxNjUASGlzdG9yaWNhbCBNYXJrZXQgRGF0YSBTZXJ2aWNlIHF1ZXJ5IG1lc3NhZ2U6bm8gaXRlbXMgcmV0cmlldmVkAAAxNzgzNzI1OTEyMDcxAA=="))
	if _, ok := e.keyed[1001]; !ok {
		t.Fatal("live code-165 no-items message deleted the scanner route")
	}

	e.handleIncoming(decodeLiveScannerFrame(t, "AAAADDIwADMAMTAwMQAwAA=="))
	results := <-sub.Events()
	if len(results) != 0 {
		t.Fatalf("scanner results len = %d, want exact live empty result", len(results))
	}

	sub.Close()
	(<-e.cmds)()
	wantCancel := liveCapturedFrame(t, "AAAACjIzADEAMTAwMQA=")
	if got := readObservedFrame(t, peer); !bytes.Equal(got, wantCancel) {
		t.Fatalf("scanner cancel = %x, want exact live cancel %x", got, wantCancel)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

func installObservedScannerRoute(t *testing.T, e *engine) *Subscription[[]ScannerResult] {
	t.Helper()
	result := make(chan *Subscription[[]ScannerResult], 1)
	go func() {
		sub, err := e.SubscribeScannerResults(context.Background(), ScannerSubscriptionRequest{
			NumberOfRows: 10,
			Instrument:   "STK",
			LocationCode: "STK.US.MAJOR",
			ScanCode:     "HOT_BY_VOLUME",
		}, WithResumePolicy(ResumeNever))
		if err != nil {
			t.Errorf("SubscribeScannerResults: %v", err)
		}
		result <- sub
	}()
	(<-e.cmds)()
	return <-result
}

func decodeLiveScannerFrame(t *testing.T, value string) any {
	t.Helper()
	message, err := codec.Decode(200, liveCapturedFrame(t, value))
	if err != nil {
		t.Fatalf("decode exact live scanner frame: %v", err)
	}
	return message
}

func liveCapturedFrame(t *testing.T, value string) []byte {
	t.Helper()
	frame, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decode exact live scanner frame: %v", err)
	}
	if len(frame) < 4 {
		t.Fatalf("exact live scanner frame has %d bytes", len(frame))
	}
	return frame[4:]
}
