// Subscribe to delayed AAPL quotes and print the first complete bid/ask.
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

func run() (err error) {
	host, port, err := exampleutil.GatewayAddress()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		return err
	}
	defer client.Close()

	// Request delayed data so the example works without a live market data
	// subscription. Remove this line if you have real-time entitlements.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		return err
	}

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Stock("AAPL"),
	})
	if err != nil {
		return err
	}

	want := ibkr.QuoteFieldBid | ibkr.QuoteFieldAsk
	for update := range sub.All(ctx) {
		if update.Snapshot.Available&want == want {
			fmt.Printf("AAPL  bid=%s  ask=%s  data=%s\n",
				update.Snapshot.Bid, update.Snapshot.Ask,
				update.Snapshot.MarketDataType)
			sub.Close()
			return sub.Wait()
		}
	}

	sub.Close()
	return errors.Join(context.Cause(ctx), sub.Wait(), errors.New("quote ended before a complete bid/ask"))
}
