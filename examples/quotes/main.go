// Subscribe to live quotes for AAPL and print bid/ask updates for 10 seconds.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/quotes
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

func main() {
	host, port := "127.0.0.1", 4002
	if addr := os.Getenv("IBKR_ADDR"); addr != "" {
		parts := strings.SplitN(addr, ":", 2)
		host = parts[0]
		if len(parts) == 2 {
			p, err := strconv.Atoi(parts[1])
			if err != nil {
				log.Fatalf("invalid port in IBKR_ADDR: %v", err)
			}
			port = p
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	// Request delayed data so the example works without a live market data
	// subscription. Remove this line if you have real-time entitlements.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}
	defer func() { _ = sub.Close() }()

	timeout := time.After(10 * time.Second)
	for {
		select {
		case update := <-sub.Events():
			fmt.Printf("bid=%-10s ask=%-10s last=%-10s\n",
				update.Snapshot.Bid, update.Snapshot.Ask, update.Snapshot.Last)
		case state := <-sub.Lifecycle():
			fmt.Println("lifecycle:", state.Kind)
		case <-sub.Done():
			if err := sub.Wait(); err != nil {
				log.Fatal(err)
			}
			return
		case <-timeout:
			fmt.Println("done")
			return
		}
	}
}
