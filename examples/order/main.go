// Place a far-from-market limit order on AAPL, observe status updates, then
// cancel it. Requires a paper trading account.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/order
//
// The example also verifies that every managed account has IBKR's paper
// account prefix before sending an order.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/examples/internal/exampleutil"
	"github.com/shopspring/decimal"
)

func main() {
	exampleutil.Run(run)
}

func run() (err error) {
	if os.Getenv("IBKR_TRADING") != "paper" {
		return fmt.Errorf("set IBKR_TRADING=paper to confirm paper-only order placement")
	}
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
	defer func() { err = errors.Join(err, client.Close()) }()

	account, err := exampleutil.PaperAccount(client.Session().ManagedAccounts)
	if err != nil {
		return err
	}

	// Place a far-from-market limit buy so it won't fill.
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("1.00"), // far from market
			TIF:       ibkr.TIFDay,
			Account:   account,
		},
	})
	if err != nil {
		return err
	}
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = errors.Join(err, handle.Cancel(cleanupCtx))
		}
		err = errors.Join(err, handle.Close())
	}()

	fmt.Println("placed order", handle.OrderID())

	// Read events until the handle closes, then inspect Wait for the final error.
	cancelled := false
	for evt := range handle.Events() {
		switch {
		case evt.Status != nil:
			fmt.Printf("status: %s  filled=%s remaining=%s\n",
				evt.Status.Status, evt.Status.Filled, evt.Status.Remaining)

			// Cancel once the order is live on the server.
			if !cancelled && !ibkr.IsTerminalOrderStatus(evt.Status.Status) {
				fmt.Println("cancelling order...")
				if err := handle.Cancel(ctx); err != nil {
					return err
				}
				cancelled = true
			}
		case evt.OpenOrder != nil:
			fmt.Printf("open order: %s %s %s @ %s\n",
				evt.OpenOrder.Action, evt.OpenOrder.Quantity,
				evt.OpenOrder.OrderType, evt.OpenOrder.LmtPrice)
		case evt.Execution != nil:
			fmt.Printf("execution: %s shares @ %s\n",
				evt.Execution.Shares, evt.Execution.Price)
		case evt.CommissionAndFees != nil:
			fmt.Printf("commission: %s %s\n",
				evt.CommissionAndFees.Amount, evt.CommissionAndFees.Currency)
		}
	}
	cleanupNeeded = false

	if err := handle.Wait(); err != nil {
		return err
	}
	fmt.Println("order done")
	return nil
}
