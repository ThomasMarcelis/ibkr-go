// Fetch account summary and positions, then stream P&L for 30 seconds.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/portfolio
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

func main() {
	host, port := "127.0.0.1", 4002
	if addr := os.Getenv("IBKR_ADDR"); addr != "" {
		parts := strings.SplitN(addr, ":", 2)
		host = parts[0]
		if len(parts) == 2 {
			p, err := strconv.Atoi(parts[1])
			if err != nil {
				log.Fatalf("invalid port in IBKR_ADDR: %v", err)
			}
			port = p
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	account := client.Session().ManagedAccounts[0]

	// Account summary — one-shot.
	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "TotalCashValue", "UnrealizedPnL"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("=== account summary ===")
	for _, v := range values {
		fmt.Printf("  %-20s %s %s\n", v.Tag, v.Value, v.Currency)
	}

	// Positions — one-shot.
	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== positions ===")
	if len(positions) == 0 {
		fmt.Println("  (none)")
	}
	for _, p := range positions {
		fmt.Printf("  %-6s %s qty=%s avg_cost=%s\n",
			p.Contract.Symbol, p.Contract.SecType, p.Position, p.AvgCost)
	}

	// Stream P&L for 30 seconds.
	pnl, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{
		Account: account,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer pnl.Close()

	fmt.Println("\n=== streaming P&L (30s) ===")
	timeout := time.After(30 * time.Second)
	for {
		select {
		case update := <-pnl.Events():
			fmt.Printf("  daily=%s unrealized=%s realized=%s\n",
				update.DailyPnL, update.UnrealizedPnL, update.RealizedPnL)
		case <-pnl.Done():
			if err := pnl.Wait(); err != nil {
				log.Fatal(err)
			}
			return
		case <-timeout:
			fmt.Println("done")
			return
		}
	}
}
