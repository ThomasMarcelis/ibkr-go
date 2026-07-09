package main

import "testing"

func TestTargetTLSConfigVerifiesByDefault(t *testing.T) {
	t.Parallel()

	config, err := targetTLSConfig("gateway.example:443", false)
	if err != nil {
		t.Fatalf("targetTLSConfig() error = %v", err)
	}
	if config.ServerName != "gateway.example" {
		t.Fatalf("ServerName = %q, want gateway.example", config.ServerName)
	}
	if config.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = true by default")
	}
}

func TestTargetTLSConfigInsecureModeIsExplicit(t *testing.T) {
	t.Parallel()

	config, err := targetTLSConfig("gateway.example:443", true)
	if err != nil {
		t.Fatalf("targetTLSConfig() error = %v", err)
	}
	if !config.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = false, want explicit insecure mode")
	}
}
