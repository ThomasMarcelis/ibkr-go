// Subscribe to live quotes for AAPL and print bid/ask updates for 10 seconds.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/quotes
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, client.Close()) }()

	// Request delayed data so the example works without a live market data
	// subscription. Remove this line if you have real-time entitlements.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		return err
	}

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		return err
	}

	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	events := sub.Events()
	lifecycle := sub.Lifecycle()
	for {
		select {
		case update, ok := <-events:
			if !ok {
				return errors.Join(ctx.Err(), sub.Wait())
			}
			fmt.Printf("bid=%-10s ask=%-10s last=%-10s\n",
				update.Snapshot.Bid, update.Snapshot.Ask, update.Snapshot.Last)
		case state, ok := <-lifecycle:
			if !ok {
				lifecycle = nil
				continue
			}
			fmt.Println("lifecycle:", state.Kind)
		case <-timeout.C:
			sub.Close()
			if err := sub.Wait(); err != nil {
				return err
			}
			fmt.Println("done")
			return nil
		case <-ctx.Done():
			sub.Close()
			return errors.Join(ctx.Err(), sub.Wait())
		}
	}
}
