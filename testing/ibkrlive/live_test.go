package ibkrlive

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv(envAddr, "")
	t.Setenv(envClientID, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != "127.0.0.1:4001" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, "127.0.0.1:4001")
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
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv(envAddr, "127.0.0.1:4101")
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
