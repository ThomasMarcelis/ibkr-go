package main

import "testing"

func TestDefaultAddrForRole(t *testing.T) {
	t.Setenv(envLiveAddr, "")
	t.Setenv(envReadOnlyLiveAddr, "")
	t.Setenv(envPaperDevAddr, "")

	if got := defaultAddrForRole(roleReadOnlyLive); got != defaultReadOnlyLiveAddr {
		t.Fatalf("read-only addr = %q, want %q", got, defaultReadOnlyLiveAddr)
	}
	if got := defaultAddrForRole(rolePaperDev); got != defaultPaperDevAddr {
		t.Fatalf("paper addr = %q, want %q", got, defaultPaperDevAddr)
	}
}

func TestRoleSpecificAddrOverridesLegacyAddr(t *testing.T) {
	t.Setenv(envLiveAddr, "127.0.0.1:4999")
	t.Setenv(envReadOnlyLiveAddr, "127.0.0.1:4101")
	t.Setenv(envPaperDevAddr, "127.0.0.1:4102")

	if got := defaultAddrForRole(roleReadOnlyLive); got != "127.0.0.1:4101" {
		t.Fatalf("read-only addr = %q, want role-specific addr", got)
	}
	if got := defaultAddrForRole(rolePaperDev); got != "127.0.0.1:4102" {
		t.Fatalf("paper addr = %q, want role-specific addr", got)
	}
}

func TestPaperRoleIgnoresLegacyAddr(t *testing.T) {
	t.Setenv(envLiveAddr, "127.0.0.1:4999")
	t.Setenv(envPaperDevAddr, "")

	if got := defaultAddrForRole(rolePaperDev); got != defaultPaperDevAddr {
		t.Fatalf("paper addr = %q, want default paper addr", got)
	}
}

func TestSplitAddr(t *testing.T) {
	host, port, err := splitAddr("127.0.0.1:4002")
	if err != nil {
		t.Fatalf("splitAddr: %v", err)
	}
	if host != "127.0.0.1" || port != 4002 {
		t.Fatalf("splitAddr = %q, %d; want 127.0.0.1, 4002", host, port)
	}
}
