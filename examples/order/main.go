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

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		return err
	}
	defer client.Close()

	account, err := exampleutil.PaperAccount(client.Session().ManagedAccounts)
	if err != nil {
		return err
	}

	order := ibkr.LimitOrder(
		ibkr.ActionBuy,
		decimal.NewFromInt(1),
		decimal.RequireFromString("1.00"), // deliberately far from market
	)
	order.Account = account
	order.TIF = ibkr.TIFDay

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Stock("AAPL"),
		Order:    order,
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
		handle.Close()
		err = errors.Join(err, handle.Wait())
	}()

	fmt.Println("placed order", handle.OrderID())

	cancelSent := false
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return errors.Join(handle.Wait(), errors.New("order observation ended before a terminal status"))
			}
			switch {
			case evt.Status != nil:
				fmt.Printf("status: %s  filled=%s remaining=%s\n",
					evt.Status.Status, evt.Status.Filled, evt.Status.Remaining)
				if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
					cleanupNeeded = false
					fmt.Println("order done")
					return nil
				}
				if !cancelSent {
					fmt.Println("cancelling order...")
					if err := handle.Cancel(ctx); err != nil {
						return err
					}
					cancelSent = true
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
			case evt.Warning != nil:
				fmt.Println("warning:", evt.Warning)
			}
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
}
