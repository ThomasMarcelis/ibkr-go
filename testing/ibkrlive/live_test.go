package ibkrlive

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv(envAddr, "")
	t.Setenv(envReadOnlyLiveAddr, "")
	t.Setenv(envPaperDevAddr, "")
	t.Setenv(envClientID, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != defaultReadOnlyLiveAddr {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, defaultReadOnlyLiveAddr)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 4001 {
		t.Fatalf("Port = %d, want 4001", cfg.Port)
	}
	if cfg.ClientID != 1 {
		t.Fatalf("ClientID = %d, want 1", cfg.ClientID)
	}
	if cfg.Role != RoleReadOnlyLive {
		t.Fatalf("Role = %q, want %q", cfg.Role, RoleReadOnlyLive)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv(envAddr, "127.0.0.1:4101")
	t.Setenv(envReadOnlyLiveAddr, "")
	t.Setenv(envClientID, "7")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 4101 {
		t.Fatalf("Port = %d, want 4101", cfg.Port)
	}
	if cfg.ClientID != 7 {
		t.Fatalf("ClientID = %d, want 7", cfg.ClientID)
	}
}

func TestLoadRoleAddresses(t *testing.T) {
	t.Setenv(envAddr, "")
	t.Setenv(envReadOnlyLiveAddr, "127.0.0.1:4101")
	t.Setenv(envPaperDevAddr, "127.0.0.1:4102")

	readOnly, err := LoadRole(RoleReadOnlyLive)
	if err != nil {
		t.Fatalf("LoadRole(readonly): %v", err)
	}
	if readOnly.Addr != "127.0.0.1:4101" {
		t.Fatalf("read-only Addr = %q, want 127.0.0.1:4101", readOnly.Addr)
	}
	if readOnly.Role != RoleReadOnlyLive {
		t.Fatalf("read-only Role = %q, want %q", readOnly.Role, RoleReadOnlyLive)
	}

	paper, err := LoadRole(RolePaperDev)
	if err != nil {
		t.Fatalf("LoadRole(paper): %v", err)
	}
	if paper.Addr != "127.0.0.1:4102" {
		t.Fatalf("paper Addr = %q, want 127.0.0.1:4102", paper.Addr)
	}
	if paper.Role != RolePaperDev {
		t.Fatalf("paper Role = %q, want %q", paper.Role, RolePaperDev)
	}
}

func TestPaperRoleIgnoresLegacyAddr(t *testing.T) {
	t.Setenv(envAddr, "127.0.0.1:4999")
	t.Setenv(envPaperDevAddr, "")

	cfg, err := LoadRole(RolePaperDev)
	if err != nil {
		t.Fatalf("LoadRole(paper): %v", err)
	}
	if cfg.Addr != defaultPaperDevAddr {
		t.Fatalf("paper Addr = %q, want default paper addr", cfg.Addr)
	}
}

func TestReadOnlyAddrForSafetyCheck(t *testing.T) {
	t.Setenv(envReadOnlyLiveAddr, "")
	if got := readOnlyAddrForSafetyCheck(); got != defaultReadOnlyLiveAddr {
		t.Fatalf("readOnlyAddrForSafetyCheck() = %q, want default %q", got, defaultReadOnlyLiveAddr)
	}

	t.Setenv(envReadOnlyLiveAddr, "127.0.0.1:4101")
	if got := readOnlyAddrForSafetyCheck(); got != "127.0.0.1:4101" {
		t.Fatalf("readOnlyAddrForSafetyCheck() = %q, want env addr", got)
	}
}

func TestEnabled(t *testing.T) {
	t.Setenv(envLive, "")
	if Enabled() {
		t.Fatal("Enabled() = true, want false")
	}
	t.Setenv(envLive, "1")
	if !Enabled() {
		t.Fatal("Enabled() = false, want true")
	}
}

func TestTradingEnabled(t *testing.T) {
	t.Setenv(envLiveTrading, "")
	if TradingEnabled() {
		t.Fatal("TradingEnabled() = true, want false")
	}
	t.Setenv(envLiveTrading, "1")
	if !TradingEnabled() {
		t.Fatal("TradingEnabled() = false, want true")
	}
}

func TestGateFlagParsing(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"FALSE", false},
		{"f", false},
		{"1", true},
		{"true", true},
		{"t", true},
		{"yes", true}, // legacy truthy value, not ParseBool-recognized
		{"on", true},  // any other non-empty value enables
		{"enable", true},
	}
	for _, tc := range cases {
		t.Run("IBKR_LIVE="+tc.value, func(t *testing.T) {
			t.Setenv(envLive, tc.value)
			if got := Enabled(); got != tc.want {
				t.Fatalf("Enabled() with %s=%q = %v, want %v", envLive, tc.value, got, tc.want)
			}
		})
		t.Run("IBKR_LIVE_TRADING="+tc.value, func(t *testing.T) {
			t.Setenv(envLiveTrading, tc.value)
			if got := TradingEnabled(); got != tc.want {
				t.Fatalf("TradingEnabled() with %s=%q = %v, want %v", envLiveTrading, tc.value, got, tc.want)
			}
		})
	}
}
