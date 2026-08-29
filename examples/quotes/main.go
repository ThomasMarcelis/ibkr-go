// Subscribe to delayed AAPL quotes and print the first complete bid/ask/last.
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

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/examples/internal/exampleutil"
)

func main() {
	exampleutil.Run(run)
}

func run() error {
	host, port, err := exampleutil.GatewayAddress()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx, ibkr.WithHost(host), ibkr.WithPort(port))
	if err != nil {
		return err
	}
	defer client.Close()

	// Delayed data works without a market-data subscription. Remove this line
	// if the login has real-time entitlements.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		return err
	}

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Stock("AAPL"),
	})
	if err != nil {
		return err
	}

	// Fields arrive one tick at a time, so wait until the ones we want are all
	// populated. IBKR reports -1 for a side with no quote, for example outside
	// market hours.
	want := ibkr.QuoteFieldBid | ibkr.QuoteFieldAsk | ibkr.QuoteFieldLast
	for update := range sub.All(ctx) {
		q := update.Snapshot
		if q.Available&want != want {
			continue
		}
		fmt.Printf("AAPL  bid=%s  ask=%s  last=%s  data=%s\n", q.Bid, q.Ask, q.Last, q.MarketDataType)
		sub.Close()
		return sub.Wait()
	}

	sub.Close()
	return errors.Join(context.Cause(ctx), sub.Wait(), errors.New("quote ended before bid, ask, and last were all available"))
}
