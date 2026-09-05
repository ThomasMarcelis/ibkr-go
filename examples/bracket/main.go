// Place a bracket (entry plus take-profit and stop-loss) far from the market,
// then cancel the entry and confirm all three legs end. Requires a paper
// trading account.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=paper go run ./examples/bracket
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

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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

	// Anchor the prices on a delayed quote so every leg sits far from the
	// market. Placement and cancellation outcomes still require observation.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		return err
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: ibkr.Stock("AAPL")})
	if err != nil {
		return err
	}
	if quote.Available&ibkr.QuoteFieldLast == 0 {
		return errors.New("no last price available to anchor the bracket")
	}
	entry := quote.Last.Mul(decimal.RequireFromString("0.5")).Round(2)
	takeProfit := quote.Last.Mul(decimal.RequireFromString("1.5")).Round(2)
	stopLoss := quote.Last.Mul(decimal.RequireFromString("0.4")).Round(2)
	fmt.Printf("AAPL last=%s  entry=%s  take-profit=%s  stop-loss=%s\n", quote.Last, entry, takeProfit, stopLoss)

	quantity := decimal.NewFromInt(1)
	parent := ibkr.LimitOrder(ibkr.ActionBuy, quantity, entry)
	parent.Account = account
	profit := ibkr.LimitOrder(ibkr.ActionSell, quantity, takeProfit)
	profit.Account = account
	stop := ibkr.StopOrder(ibkr.ActionSell, quantity, stopLoss)
	stop.Account = account

	// PlaceBracket allocates the three IDs, links the children to the parent,
	// and sets the transmit flags so the bracket goes live as one unit.
	bracket, err := client.Orders().PlaceBracket(ctx, ibkr.PlaceBracketRequest{
		Contract:   ibkr.Stock("AAPL"),
		Parent:     parent,
		TakeProfit: profit,
		StopLoss:   stop,
	})
	if err != nil {
		return err
	}
	legs := []struct {
		name   string
		handle *ibkr.OrderHandle
	}{
		{"entry", bracket.Parent},
		{"take-profit", bracket.TakeProfit},
		{"stop-loss", bracket.StopLoss},
	}
	for _, leg := range legs {
		fmt.Printf("locally queued %-11s order %d\n", leg.name, leg.handle.OrderID())
		defer leg.handle.Close()
	}

	// Early cleanup is best effort. Request parent cancellation, then retain
	// every leg ID because queue admission does not confirm any cancellation.
	done := false
	defer func() {
		if !done {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = errors.Join(err, client.Orders().Cancel(cleanupCtx, bracket.Parent.OrderID()),
				fmt.Errorf("reconcile bracket orders %d, %d, %d: cancellation outcomes are unconfirmed",
					bracket.Parent.OrderID(), bracket.TakeProfit.OrderID(), bracket.StopLoss.OrderID()))
		}
	}()

	// Wait for IBKR to acknowledge the entry, then cancel it. Cancelling the
	// parent of a bracket cancels its children too.
	if _, err := awaitStatus(ctx, "entry", bracket.Parent, func(ibkr.OrderStatus) bool { return true }); err != nil {
		return err
	}
	fmt.Println("cancelling entry...")
	if err := bracket.Parent.Cancel(ctx); err != nil {
		return err
	}
	for _, leg := range legs {
		status, err := awaitStatus(ctx, leg.name, leg.handle, ibkr.IsTerminalOrderStatus)
		if err != nil {
			return err
		}
		fmt.Printf("%-11s ended %s\n", leg.name, status)
	}
	done = true
	fmt.Println("status observation complete; reconcile later fills and fees using the leg IDs")
	return nil
}

// awaitStatus prints status updates for one leg until want accepts one.
func awaitStatus(ctx context.Context, name string, handle *ibkr.OrderHandle, want func(ibkr.OrderStatus) bool) (ibkr.OrderStatus, error) {
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return "", errors.Join(handle.Wait(), fmt.Errorf("%s: order events ended early", name))
			}
			if evt.Status == nil {
				continue
			}
			fmt.Printf("%-11s status: %s\n", name, evt.Status.Status)
			if want(evt.Status.Status) {
				return evt.Status.Status, nil
			}
		case <-ctx.Done():
			return "", fmt.Errorf("%s: order %d may still be resting at IBKR: %w", name, handle.OrderID(), context.Cause(ctx))
		}
	}
}
