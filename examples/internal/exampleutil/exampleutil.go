// Package exampleutil contains the small amount of process and connection
// plumbing shared by the executable examples.
package exampleutil

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:4002"

// Run reports a returned error only after run's deferred cleanup has executed.
func Run(run func() error) {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

// GatewayAddress returns IBKR_ADDR or the paper-Gateway default.
func GatewayAddress() (string, int, error) {
	address := os.Getenv("IBKR_ADDR")
	if address == "" {
		address = defaultAddress
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, fmt.Errorf("parse IBKR_ADDR %q: %w", address, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("parse IBKR_ADDR %q: invalid port %q", address, portText)
	}
	return host, port, nil
}

// RequirePaperTrading requires an explicit opt-in before an example can send
// an order-shaped request to a paper account.
func RequirePaperTrading() error {
	if os.Getenv("IBKR_TRADING") != "paper" {
		return fmt.Errorf("set IBKR_TRADING=paper to confirm paper-only order activity")
	}
	return nil
}

// FirstAccount returns the first managed account after proving one exists.
func FirstAccount(accounts []string) (string, error) {
	if len(accounts) == 0 {
		return "", fmt.Errorf("gateway reported no managed accounts")
	}
	return accounts[0], nil
}

// PaperAccount returns the first managed account only when every account in
// the session has IBKR's paper-account prefix.
func PaperAccount(accounts []string) (string, error) {
	account, err := FirstAccount(accounts)
	if err != nil {
		return "", err
	}
	for _, candidate := range accounts {
		if !strings.HasPrefix(candidate, "DU") {
			return "", fmt.Errorf("refusing to trade: managed account %q is not an IBKR paper account", candidate)
		}
	}
	return account, nil
}
