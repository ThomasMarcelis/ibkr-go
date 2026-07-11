// Resolve the nearest complete AAPL option chain and print a small sample.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/option-chain
package main

import (
	"context"
	"fmt"
	"slices"
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

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx, ibkr.WithHost(host), ibkr.WithPort(port))
	if err != nil {
		return err
	}
	defer client.Close()

	underlying, err := client.Contracts().Qualify(ctx, ibkr.Stock("AAPL"))
	if err != nil {
		return err
	}
	parameters, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  underlying.Symbol,
		UnderlyingSecType: underlying.SecType,
		UnderlyingConID:   underlying.ConID,
	})
	if err != nil {
		return err
	}

	chain, ok := standardAAPLChain(parameters)
	if !ok {
		return fmt.Errorf("IBKR returned no SMART AAPL option chain with multiplier 100")
	}
	expiry, ok := nearestExpiry(chain.Expirations)
	if !ok {
		return fmt.Errorf("SMART AAPL option chain returned no YYYYMMDD expiry")
	}

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:       underlying.Symbol,
		SecType:      ibkr.SecTypeOption,
		Expiry:       expiry,
		Multiplier:   chain.Multiplier,
		Exchange:     chain.Exchange,
		Currency:     underlying.Currency,
		TradingClass: chain.TradingClass,
	})
	if err != nil {
		return err
	}
	if len(details) == 0 {
		return fmt.Errorf("IBKR returned no AAPL option contracts for %s", expiry)
	}

	slices.SortFunc(details, compareOptions)
	fmt.Printf("AAPL %s: %d contracts\n", expiry, len(details))
	for _, detail := range details[:min(12, len(details))] {
		strike := "n/a"
		if detail.Strike != nil {
			strike = detail.Strike.String()
		}
		fmt.Printf("  %s %8s  %s\n", detail.Right, strike, detail.LocalSymbol)
	}
	if len(details) > 12 {
		fmt.Printf("  ... %d more\n", len(details)-12)
	}
	return nil
}

func standardAAPLChain(parameters []ibkr.SecDefOptParams) (ibkr.SecDefOptParams, bool) {
	for _, parameter := range parameters {
		if parameter.Exchange == "SMART" &&
			parameter.TradingClass == "AAPL" &&
			parameter.Multiplier == "100" {
			return parameter, true
		}
	}
	return ibkr.SecDefOptParams{}, false
}

func nearestExpiry(expirations []string) (string, bool) {
	nearest := ""
	for _, expiry := range expirations {
		if _, err := time.Parse("20060102", expiry); err == nil &&
			(nearest == "" || expiry < nearest) {
			nearest = expiry
		}
	}
	return nearest, nearest != ""
}

func compareOptions(a, b ibkr.ContractDetails) int {
	if a.Right < b.Right {
		return -1
	}
	if a.Right > b.Right {
		return 1
	}
	if a.Strike == nil && b.Strike == nil {
		return 0
	}
	if a.Strike == nil {
		return -1
	}
	if b.Strike == nil {
		return 1
	}
	return a.Strike.Cmp(*b.Strike)
}
