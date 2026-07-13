package ibkr

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

func TestConfigRoutesLiveDerivedResponse(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 219
	e.nextReqID = 7901
	result := make(chan struct {
		config TWSConfig
		err    error
	}, 1)
	go func() {
		config, err := e.Config(context.Background())
		result <- struct {
			config TWSConfig
			err    error
		}{config, err}
	}()

	(<-e.cmds)()
	want, err := codec.Encode(219, codec.ConfigRequest{ReqID: 7901})
	if err != nil {
		t.Fatal(err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, want) {
		t.Fatalf("config request = %x, want %x", got, want)
	}
	trustedIPs := []string{"127.0.0.1"}
	e.handleIncoming(codec.ConfigResponse{
		ReqID:    7901,
		Messages: []codec.MessageConfig{{ID: new(397), Enabled: new(false)}},
		API: &codec.APIConfig{
			Settings: &codec.APISettingsConfig{ReadOnlyAPI: new(true), SocketPort: new(4001), TrustedIPs: trustedIPs},
		},
		Orders: &codec.OrdersConfig{SmartRouting: &codec.OrdersSmartRoutingConfig{SeekPriceImprovement: new(false)}},
	})
	trustedIPs[0] = "mutated"

	out := <-result
	if out.err != nil {
		t.Fatal(out.err)
	}
	if out.config.API == nil || out.config.API.Settings == nil || out.config.API.Settings.ReadOnlyAPI == nil || !*out.config.API.Settings.ReadOnlyAPI {
		t.Fatalf("config = %+v", out.config)
	}
	if got := out.config.API.Settings.TrustedIPs; len(got) != 1 || got[0] != "127.0.0.1" {
		t.Fatalf("trusted IP ownership = %v", got)
	}
	if len(out.config.Messages) != 1 || out.config.Messages[0].Enabled == nil || *out.config.Messages[0].Enabled {
		t.Fatalf("message presence = %+v", out.config.Messages)
	}
	if _, ok := e.keyed[7901]; ok {
		t.Fatal("completed config request retained its route")
	}
}

func TestConfigRejectsServerVersion218BeforeAllocation(t *testing.T) {
	e, _ := newObservedMarketDataEngine(t)
	e.serverVersion = 218
	e.nextReqID = 7901
	result := make(chan error, 1)
	go func() {
		_, err := e.Config(context.Background())
		result <- err
	}()
	(<-e.cmds)()
	if err := <-result; !errors.Is(err, ErrUnsupportedServerVersion) {
		t.Fatalf("Config() error = %v, want ErrUnsupportedServerVersion", err)
	}
	if e.nextReqID != 7901 || len(e.keyed) != 0 {
		t.Fatalf("rejected config mutated route state: next=%d routes=%d", e.nextReqID, len(e.keyed))
	}
}
