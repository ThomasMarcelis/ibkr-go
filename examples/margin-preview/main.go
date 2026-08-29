// Preview AAPL margin and commission without placing an order.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/margin-preview
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/examples/internal/exampleutil"
	"github.com/shopspring/decimal"
)

func main() {
	exampleutil.Run(run)
}

func run() error {
	if err := exampleutil.RequirePaperTrading(); err != nil {
		return err
	}
	host, port, err := exampleutil.GatewayAddress()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx, ibkr.WithHost(host), ibkr.WithPort(port))
	if err != nil {
		return err
	}
	defer client.Close()

	account, err := exampleutil.PaperAccount(client.Session().ManagedAccounts)
	if err != nil {
		return err
	}
	order := ibkr.MarketOrder(ibkr.ActionBuy, decimal.NewFromInt(100))
	order.Account = account

	// Preview runs IBKR's what-if check: same request as Place, no order.
	state, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Stock("AAPL"),
		Order:    order,
	})
	if err != nil {
		return err
	}

	fmt.Printf("margin currency: %s\n", state.MarginCurrency)
	printMargin("initial", state.InitMarginBefore, state.InitMarginChange, state.InitMarginAfter)
	printMargin("maintenance", state.MaintMarginBefore, state.MaintMarginChange, state.MaintMarginAfter)
	printMargin("equity + loan", state.EquityWithLoanBefore, state.EquityWithLoanChange, state.EquityWithLoanAfter)
	fmt.Printf("commission: estimate=%s range=%s..%s %s\n",
		optionalDecimal(state.CommissionAndFees),
		optionalDecimal(state.MinCommissionAndFees),
		optionalDecimal(state.MaxCommissionAndFees),
		state.CommissionAndFeesCurrency)
	return nil
}

func printMargin(name string, before, change, after *decimal.Decimal) {
	fmt.Printf("%-13s before=%s change=%s after=%s\n", name,
		optionalDecimal(before), optionalDecimal(change), optionalDecimal(after))
}

// IBKR omits values it did not compute; nil means "not reported".
func optionalDecimal(value *decimal.Decimal) string {
	if value == nil {
		return "n/a"
	}
	return value.String()
}
