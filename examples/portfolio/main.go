// Fetch account summary and positions, then stream P&L for 30 seconds.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/portfolio
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/examples/internal/exampleutil"
)

func main() {
	exampleutil.Run(run)
}

func run() (err error) {
	host, port, err := exampleutil.GatewayAddress()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, client.Close()) }()

	account, err := exampleutil.FirstAccount(client.Session().ManagedAccounts)
	if err != nil {
		return err
	}

	// Account summary — one-shot.
	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation", "TotalCashValue", "UnrealizedPnL"},
	})
	if err != nil {
		return err
	}
	fmt.Println("=== account summary ===")
	for _, v := range values {
		fmt.Printf("  %-20s %s %s\n", v.Tag, v.Value, v.Currency)
	}

	// Positions — one-shot.
	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		return err
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
		return err
	}
	defer func() { err = errors.Join(err, pnl.Close()) }()

	fmt.Println("\n=== streaming P&L (30s) ===")
	timeout := time.After(30 * time.Second)
	for {
		select {
		case update, ok := <-pnl.Events():
			if !ok {
				if err := pnl.Wait(); err != nil {
					return err
				}
				return nil
			}
			fmt.Printf("  daily=%s unrealized=%s realized=%s\n",
				update.DailyPnL, update.UnrealizedPnL, update.RealizedPnL)
		case <-timeout:
			fmt.Println("done")
			return nil
		}
	}
}
