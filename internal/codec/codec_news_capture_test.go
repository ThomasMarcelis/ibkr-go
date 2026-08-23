package codec

import (
	"testing"
)

func TestCaptureDecode_TickNewsLive(t *testing.T) {
	// captures/20260709T230825Z-api_tick_news_aapl_probe, read-only IB
	// Gateway server_version 201. events sha256:
	// a0784d2eddda74681cc301befb98440a96bb76242efd43aec88a9f177a5411df.
	// normalized frames sha256:
	// e3e1901503f7d1dc52489bccb2bce64467e35bad48bc025e438b916a5c639e60.
	// This is the first exact msg-84 payload; only its outer frame length is
	// omitted.
	payload := append([]byte{0, 0, 0, 84}, []byte("1\x001758294759000\x00BRFG\x00BRFG$1c2d5728\x00Apple's iPhone 17 debuts to long lines and high demand as company eyes upgrade cycle boost\x00A:800015:L:en:K:1.00:C:0.9999533295631409\x00")...)

	msgs, err := DecodeBatch(201, payload)
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("DecodeBatch() len = %d, want 1", len(msgs))
	}
	got, ok := msgs[0].(TickNews)
	if !ok {
		t.Fatalf("message type = %T, want TickNews", msgs[0])
	}
	want := TickNews{
		ReqID:        1,
		Time:         "1758294759000",
		ProviderCode: "BRFG",
		ArticleID:    "BRFG$1c2d5728",
		Headline:     "Apple's iPhone 17 debuts to long lines and high demand as company eyes upgrade cycle boost",
		ExtraData:    "A:800015:L:en:K:1.00:C:0.9999533295631409",
	}
	if got != want {
		t.Fatalf("TickNews = %#v, want %#v", got, want)
	}

}
