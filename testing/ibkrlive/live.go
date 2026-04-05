package ibkrlive

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/ibkr"
)

const (
	envLive     = "IBKR_LIVE"
	envAddr     = "IBKR_LIVE_ADDR"
	envClientID = "IBKR_LIVE_CLIENT_ID"
)

type Config struct {
	Addr     string
	Host     string
	Port     int
	ClientID int
}

func Enabled() bool {
	return os.Getenv(envLive) != ""
}

func Load() (Config, error) {
	addr := os.Getenv(envAddr)
	if addr == "" {
		addr = "127.0.0.1:4001"
	}
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return Config{}, fmt.Errorf("ibkrlive: parse %s: %w", envAddr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return Config{}, fmt.Errorf("ibkrlive: parse %s port: %w", envAddr, err)
	}
	clientID := 1
	if raw := os.Getenv(envClientID); raw != "" {
		clientID, err = strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("ibkrlive: parse %s: %w", envClientID, err)
		}
	}
	return Config{
		Addr:     addr,
		Host:     host,
		Port:     port,
		ClientID: clientID,
	}, nil
}

func Require(t testing.TB) Config {
	t.Helper()
	if !Enabled() {
		t.Skipf("set %s=1 to enable live IBKR tests", envLive)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return cfg
}

func Options(cfg Config, extra ...ibkr.Option) []ibkr.Option {
	opts := []ibkr.Option{
		ibkr.WithHost(cfg.Host),
		ibkr.WithPort(cfg.Port),
		ibkr.WithClientID(cfg.ClientID),
	}
	opts = append(opts, extra...)
	return opts
}

func DialContext(t testing.TB, timeout time.Duration, extra ...ibkr.Option) (*ibkr.Client, context.Context, context.CancelFunc) {
	t.Helper()
	cfg := Require(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	client, err := ibkr.DialContext(ctx, Options(cfg, extra...)...)
	if err != nil {
		cancel()
		t.Fatalf("DialContext() error = %v", err)
	}
	return client, ctx, cancel
}
