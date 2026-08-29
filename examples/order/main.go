// Place a far-from-market limit order on AAPL, watch its status, then cancel
// it. Requires a paper trading account.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/order
//
// The example refuses to run unless every managed account has IBKR's paper
// account prefix.
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

	// A $1 bid for AAPL never fills, so the order rests until we cancel it.
	order := ibkr.LimitOrder(ibkr.ActionBuy, decimal.NewFromInt(1), decimal.RequireFromString("1.00"))
	order.Account = account

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Stock("AAPL"),
		Order:    order,
	})
	if err != nil {
		return err
	}
	fmt.Println("placed order", handle.OrderID())

	// If we leave early, cancel on a fresh context so nothing stays resting on
	// the paper account. Closing the handle alone never cancels an order.
	terminal := false
	defer func() {
		if !terminal {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = errors.Join(err, client.Orders().Cancel(cleanupCtx, handle.OrderID()))
		}
		handle.Close()
	}()

	cancelled := false
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return errors.Join(handle.Wait(), errors.New("order events ended before a terminal status"))
			}
			switch {
			case evt.Status != nil:
				fmt.Printf("status: %-13s filled=%s remaining=%s\n",
					evt.Status.Status, evt.Status.Filled, evt.Status.Remaining)
				if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
					terminal = true
					return nil
				}
				// First acknowledgement from IBKR: it is resting, cancel it.
				if !cancelled {
					cancelled = true
					fmt.Println("cancelling...")
					if err := handle.Cancel(ctx); err != nil {
						return err
					}
				}
			case evt.Execution != nil:
				fmt.Printf("fill: %s @ %s\n", evt.Execution.Shares, evt.Execution.Price)
			case evt.Warning != nil:
				fmt.Println("warning:", evt.Warning)
			case evt.Lifecycle != nil:
				fmt.Println("lifecycle:", evt.Lifecycle.Kind)
			}
		case <-ctx.Done():
			return fmt.Errorf("order %d may still be resting at IBKR: %w", handle.OrderID(), context.Cause(ctx))
		}
	}
}
