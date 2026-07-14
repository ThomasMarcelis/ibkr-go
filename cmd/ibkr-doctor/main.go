// Command ibkr-doctor checks the local IB Gateway/TWS setup used by ibkr-go.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
)

const (
	roleReadOnlyLive = "readonly-live"
	rolePaperDev     = "paper-dev"

	envDoctorRole       = "IBKR_DOCTOR_ROLE"
	envLiveAddr         = "IBKR_LIVE_ADDR"
	envReadOnlyLiveAddr = "IBKR_LIVE_READONLY_ADDR"
	envPaperDevAddr     = "IBKR_LIVE_PAPER_ADDR"
	envClientID         = "IBKR_LIVE_CLIENT_ID"

	defaultReadOnlyLiveAddr = "127.0.0.1:4001"
	defaultPaperDevAddr     = "127.0.0.1:4002"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	roleDefault := getenv(envDoctorRole, roleReadOnlyLive)
	fs := flag.NewFlagSet("ibkr-doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	role := fs.String("role", roleDefault, "gateway role: readonly-live or paper-dev")
	addr := fs.String("addr", "", "gateway address; defaults from role-specific environment")
	clientID := fs.Int64("client-id", int64Env(envClientID, 91), "TWS API client id")
	timeout := fs.Duration("timeout", 15*time.Second, "overall diagnostic timeout")
	quoteSymbol := fs.String("quote-symbol", "AAPL", "stock symbol for quote probe")
	skipQuote := fs.Bool("skip-quote", false, "skip market data quote probe")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *role != roleReadOnlyLive && *role != rolePaperDev {
		fmt.Fprintf(stderr, "role: want %q or %q, got %q\n", roleReadOnlyLive, rolePaperDev, *role)
		return 2
	}
	if *clientID < 0 || *clientID > math.MaxInt32 {
		fmt.Fprintf(stderr, "client-id: must be between 0 and %d\n", math.MaxInt32)
		return 2
	}
	if *addr == "" {
		*addr = defaultAddrForRole(*role)
	}
	host, port, err := splitAddr(*addr)
	if err != nil {
		fmt.Fprintf(stderr, "addr: %v\n", err)
		return 2
	}

	fmt.Fprintf(stdout, "role: %s\n", *role)
	fmt.Fprintf(stdout, "addr: %s\n", *addr)
	fmt.Fprintf(stdout, "client_id: %d\n", *clientID)
	switch *role {
	case roleReadOnlyLive:
		fmt.Fprintln(stdout, "writes: disabled by role; this command will not place orders")
	case rolePaperDev:
		fmt.Fprintln(stdout, "writes: paper-dev role selected; this command still performs read-only checks only")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
		ibkr.WithClientID(ibkr.ClientID(*clientID)),
	)
	if err != nil {
		fmt.Fprintf(stderr, "dial: %v\n", err)
		return 1
	}
	defer client.Close()

	snap := client.Session()
	fmt.Fprintf(stdout, "session: state=%s server_version=%d connection_seq=%d next_valid_id=%d accounts=%d\n",
		snap.State, snap.ServerVersion, snap.ConnectionSeq, snap.NextValidID, len(snap.ManagedAccounts))

	now, err := client.CurrentTime(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "current_time: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "current_time: %s\n", now.Format(time.RFC3339))

	if *skipQuote {
		return 0
	}
	if *role == rolePaperDev {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			fmt.Fprintf(stdout, "market_data_type: delayed request warning: %v\n", err)
		} else {
			fmt.Fprintln(stdout, "market_data_type: delayed requested")
		}
	}

	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: ibkr.Contract{
		Symbol:   strings.ToUpper(*quoteSymbol),
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	}})
	if err != nil {
		fmt.Fprintf(stdout, "quote: warning: %v\n", err)
		return 0
	}
	fmt.Fprintf(stdout, "quote: symbol=%s available=%d bid=%s ask=%s last=%s close=%s type=%s\n",
		strings.ToUpper(*quoteSymbol),
		quote.Available,
		quote.Bid.String(),
		quote.Ask.String(),
		quote.Last.String(),
		quote.Close.String(),
		quote.MarketDataType,
	)
	return 0
}

func defaultAddrForRole(role string) string {
	switch role {
	case rolePaperDev:
		if addr := os.Getenv(envPaperDevAddr); addr != "" {
			return addr
		}
		return defaultPaperDevAddr
	default:
		if addr := os.Getenv(envReadOnlyLiveAddr); addr != "" {
			return addr
		}
		if addr := os.Getenv(envLiveAddr); addr != "" {
			return addr
		}
		return defaultReadOnlyLiveAddr
	}
}

func splitAddr(addr string) (string, int, error) {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, fmt.Errorf("parse port %q: %w", portText, err)
	}
	return host, port, nil
}

func getenv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func int64Env(name string, fallback int64) int64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
