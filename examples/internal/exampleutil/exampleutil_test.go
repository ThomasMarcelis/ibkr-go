package exampleutil

import "testing"

func TestGatewayAddress(t *testing.T) {
	t.Setenv("IBKR_ADDR", "[::1]:4002")
	host, port, err := GatewayAddress()
	if err != nil {
		t.Fatalf("GatewayAddress() error = %v", err)
	}
	if host != "::1" || port != 4002 {
		t.Fatalf("GatewayAddress() = %q, %d, want ::1, 4002", host, port)
	}
}

func TestGatewayAddressRejectsMissingPort(t *testing.T) {
	t.Setenv("IBKR_ADDR", "127.0.0.1")
	if _, _, err := GatewayAddress(); err == nil {
		t.Fatal("GatewayAddress() error = nil, want invalid-address error")
	}
}

func TestRequirePaperTrading(t *testing.T) {
	t.Setenv("IBKR_TRADING", "paper")
	if err := RequirePaperTrading(); err != nil {
		t.Fatalf("RequirePaperTrading() error = %v", err)
	}
}

func TestRequirePaperTradingRequiresOptIn(t *testing.T) {
	t.Setenv("IBKR_TRADING", "")
	if err := RequirePaperTrading(); err == nil {
		t.Fatal("RequirePaperTrading() error = nil, want opt-in error")
	}
}

func TestPaperAccountRejectsMixedSession(t *testing.T) {
	if _, err := PaperAccount([]string{"DU9000001", "U123456"}); err == nil {
		t.Fatal("PaperAccount() error = nil, want live-account refusal")
	}
}

func TestPaperAccountRequiresManagedAccount(t *testing.T) {
	if _, err := PaperAccount(nil); err == nil {
		t.Fatal("PaperAccount() error = nil, want empty-session error")
	}
}
