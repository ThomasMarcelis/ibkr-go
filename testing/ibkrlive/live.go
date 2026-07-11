package ibkrlive

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

const (
	envLive             = "IBKR_LIVE"
	envLiveTrading      = "IBKR_LIVE_TRADING"
	envAddr             = "IBKR_LIVE_ADDR"
	envReadOnlyLiveAddr = "IBKR_LIVE_READONLY_ADDR"
	envPaperDevAddr     = "IBKR_LIVE_PAPER_ADDR"
	envClientID         = "IBKR_LIVE_CLIENT_ID"

	defaultReadOnlyLiveAddr = "127.0.0.1:4001"
	defaultPaperDevAddr     = "127.0.0.1:4002"
)

var generatedClientID atomic.Int64
var liveSessionMu sync.Mutex

type Role string

const (
	RoleReadOnlyLive Role = "readonly-live"
	RolePaperDev     Role = "paper-dev"
)

type Config struct {
	Addr     string
	Host     string
	Port     int
	ClientID int
	Role     Role
}

func Enabled() bool {
	return envFlag(envLive)
}

func TradingEnabled() bool {
	return envFlag(envLiveTrading)
}

// envFlag reads a boolean-ish gate variable. Empty is off; a value
// strconv.ParseBool reads as false ("0", "false", "f", …) is off; any other
// non-empty value (including "1", "true", and legacy "yes") is on. This keeps
// IBKR_LIVE=0 from accidentally turning live mode on.
func envFlag(name string) bool {
	raw := os.Getenv(name)
	if raw == "" {
		return false
	}
	if b, err := strconv.ParseBool(raw); err == nil {
		return b
	}
	return true
}

func Load() (Config, error) {
	return LoadRole(RoleReadOnlyLive)
}

func LoadRole(role Role) (Config, error) {
	addr, err := roleAddr(role)
	if err != nil {
		return Config{}, err
	}
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return Config{}, fmt.Errorf("ibkrlive: parse %s addr %q: %w", role, addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return Config{}, fmt.Errorf("ibkrlive: parse %s port: %w", role, err)
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
		Role:     role,
	}, nil
}

func roleAddr(role Role) (string, error) {
	switch role {
	case RoleReadOnlyLive:
		if addr := os.Getenv(envReadOnlyLiveAddr); addr != "" {
			return addr, nil
		}
		if addr := os.Getenv(envAddr); addr != "" {
			return addr, nil
		}
		return defaultReadOnlyLiveAddr, nil
	case RolePaperDev:
		if addr := os.Getenv(envPaperDevAddr); addr != "" {
			return addr, nil
		}
		return defaultPaperDevAddr, nil
	default:
		return "", fmt.Errorf("ibkrlive: unknown role %q", role)
	}
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

func RequireTrading(t testing.TB) {
	t.Helper()
	RequireTradingConfig(t)
}

func RequireTradingConfig(t testing.TB) Config {
	t.Helper()
	if !Enabled() {
		t.Skipf("set %s=1 to enable live IBKR tests", envLive)
	}
	if !TradingEnabled() {
		t.Skipf("set %s=1 to enable paper-trading live tests", envLiveTrading)
	}
	cfg, err := LoadRole(RolePaperDev)
	if err != nil {
		t.Fatalf("LoadRole(%s) error = %v", RolePaperDev, err)
	}
	if cfg.Addr == readOnlyAddrForSafetyCheck() {
		t.Fatalf("paper-trading tests resolved %s addr %s; use %s for order-capable paper-dev Gateway", RoleReadOnlyLive, cfg.Addr, envPaperDevAddr)
	}
	return cfg
}

func readOnlyAddrForSafetyCheck() string {
	if addr := os.Getenv(envReadOnlyLiveAddr); addr != "" {
		return addr
	}
	return defaultReadOnlyLiveAddr
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
	return dialContext(t, cfg, timeout, extra...)
}

func DialTradingContext(t testing.TB, timeout time.Duration, extra ...ibkr.Option) (*ibkr.Client, context.Context, context.CancelFunc) {
	t.Helper()
	cfg := RequireTradingConfig(t)
	client, ctx, cancel := dialContext(t, cfg, timeout, extra...)
	accounts, err := client.ManagedAccounts(ctx)
	if err != nil {
		client.Close()
		cancel()
		t.Fatalf("ManagedAccounts() paper-account guard error = %v", err)
	}
	for _, account := range accounts {
		if !strings.HasPrefix(account, "DU") {
			client.Close()
			cancel()
			t.Fatalf("paper-trading session reported non-paper account %q", account)
		}
	}
	if len(accounts) == 0 {
		client.Close()
		cancel()
		t.Fatal("paper-trading session reported no managed accounts")
	}
	return client, ctx, cancel
}

func dialContext(t testing.TB, cfg Config, timeout time.Duration, extra ...ibkr.Option) (*ibkr.Client, context.Context, context.CancelFunc) {
	t.Helper()
	if os.Getenv(envClientID) == "" {
		cfg.ClientID = int(generatedClientID.Add(1))
	}

	liveSessionMu.Lock()
	locked := true
	unlock := func() {
		if locked {
			locked = false
			liveSessionMu.Unlock()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	client, err := ibkr.DialContext(ctx, Options(cfg, extra...)...)
	if err != nil {
		cancel()
		unlock()
		t.Fatalf("DialContext() error = %v", err)
	}
	return client, ctx, func() {
		cancel()
		unlock()
	}
}
