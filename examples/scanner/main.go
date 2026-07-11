// Scan the most active US stocks and quote the top result.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/scanner
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

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx, ibkr.WithHost(host), ibkr.WithPort(port))
	if err != nil {
		return err
	}
	defer client.Close()

	sub, err := client.Scanner().SubscribeResults(ctx, ibkr.ScannerSubscriptionRequest{
		NumberOfRows: 10,
		Instrument:   "STK",
		LocationCode: "STK.US.MAJOR",
		ScanCode:     "HOT_BY_VOLUME",
	})
	if err != nil {
		return err
	}

	var results []ibkr.ScannerResult
	for event := range sub.Events() {
		if event.Kind != ibkr.StreamData {
			continue
		}
		results = event.Value
		break
	}
	if results == nil {
		return errors.Join(context.Cause(ctx), sub.Wait(), errors.New("scanner closed before its first result"))
	}
	sub.Close()
	if err := sub.Wait(); err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("scanner returned no matching stocks")
		return nil
	}
	for _, result := range results {
		fmt.Printf("%2d  %-8s %s\n", result.Rank+1,
			result.Contract.Symbol, result.Contract.PrimaryExchange)
	}

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		return err
	}
	top := results[0].Contract
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: top})
	if err != nil {
		return err
	}
	fmt.Printf("\n%s  bid=%s  ask=%s  last=%s  data=%s\n",
		top.Symbol, quote.Bid, quote.Ask, quote.Last, quote.MarketDataType)
	return nil
}
