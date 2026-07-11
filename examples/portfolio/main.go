// Fetch account summary and positions, then read the first P&L update.
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

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/examples/internal/exampleutil"
	"github.com/shopspring/decimal"
)

func main() {
	exampleutil.Run(run)
}

func run() (err error) {
	host, port, err := exampleutil.GatewayAddress()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		return err
	}
	defer client.Close()

	account, err := exampleutil.FirstAccount(client.Session().ManagedAccounts)
	if err != nil {
		return err
	}

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "TotalCashValue", "UnrealizedPnL"},
	})
	if err != nil {
		return err
	}
	fmt.Println("=== account summary ===")
	for _, v := range values {
		fmt.Printf("  %-20s %s %s\n", v.Tag, v.Value, v.Currency)
	}

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

	pnl, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{
		Account: account,
	})
	if err != nil {
		return err
	}

	streamCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	fmt.Println("\n=== P&L ===")
	for update := range pnl.All(streamCtx) {
		fmt.Printf("  daily=%s unrealized=%s realized=%s\n",
			optionalDecimal(update.DailyPnL),
			optionalDecimal(update.UnrealizedPnL),
			optionalDecimal(update.RealizedPnL))
		pnl.Close()
		return pnl.Wait()
	}

	pnl.Close()
	return errors.Join(context.Cause(streamCtx), pnl.Wait(), errors.New("P&L stream ended before its first update"))
}

func optionalDecimal(value *decimal.Decimal) string {
	if value == nil {
		return "n/a"
	}
	return value.String()
}
