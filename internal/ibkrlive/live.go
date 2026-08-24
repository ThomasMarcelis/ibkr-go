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

	"github.com/ThomasMarcelis/ibkr-go/v2"
)

const (
	envLive             = "IBKR_LIVE"
	envLiveTrading      = "IBKR_LIVE_TRADING"
	envAddr             = "IBKR_LIVE_ADDR"
	envReadOnlyLiveAddr = "IBKR_LIVE_READONLY_ADDR"
	envPaperDevAddr     = "IBKR_LIVE_PAPER_ADDR"
	envClientID         = "IBKR_LIVE_CLIENT_ID"
	envPaperAccount     = "IBKR_PAPER_ACCOUNT"

	defaultReadOnlyLiveAddr = "127.0.0.1:4001"
	defaultPaperDevAddr     = "127.0.0.1:4002"
)

var generatedClientID atomic.Int32
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
	ClientID ibkr.ClientID
	Role     Role
}

func Enabled() bool {
	return envFlag(envLive)
}

func TradingEnabled() bool {
	return envFlag(envLiveTrading)
}

// envFlag accepts only values recognized by strconv.ParseBool. A typo must not
// authorize a live or trading campaign.
func envFlag(name string) bool {
	enabled, err := strconv.ParseBool(os.Getenv(name))
	return err == nil && enabled
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
	clientID := ibkr.ClientID(1)
	if raw := os.Getenv(envClientID); raw != "" {
		value, parseErr := strconv.ParseInt(raw, 10, 32)
		err = parseErr
		if err != nil {
			return Config{}, fmt.Errorf("ibkrlive: parse %s: %w", envClientID, err)
		}
		clientID = ibkr.ClientID(value)
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
	expectedAccount := strings.TrimSpace(os.Getenv(envPaperAccount))
	if !strings.HasPrefix(expectedAccount, "DU") {
		client.Close()
		cancel()
		t.Fatalf("set %s to the exact DU account authorized for paper trading", envPaperAccount)
	}
	if len(accounts) != 1 || accounts[0] != expectedAccount {
		client.Close()
		cancel()
		t.Fatalf("paper-trading session accounts do not exactly match %s", envPaperAccount)
	}
	return client, ctx, cancel
}

func dialContext(t testing.TB, cfg Config, timeout time.Duration, extra ...ibkr.Option) (*ibkr.Client, context.Context, context.CancelFunc) {
	t.Helper()
	liveSessionMu.Lock()
	locked := true
	unlock := func() {
		if locked {
			locked = false
			liveSessionMu.Unlock()
		}
	}

	dial := func() (*ibkr.Client, context.Context, context.CancelFunc, error) {
		attemptCfg := cfg
		if os.Getenv(envClientID) == "" {
			attemptCfg.ClientID = ibkr.ClientID(generatedClientID.Add(1))
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		client, err := ibkr.DialContext(ctx, Options(attemptCfg, extra...)...)
		return client, ctx, cancel, err
	}

	client, ctx, cancel, err := dial()
	if err != nil {
		cancel()
		t.Logf("first live Gateway dial failed; retrying once with a fresh generated client ID: %v", err)
		client, ctx, cancel, err = dial()
	}
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
