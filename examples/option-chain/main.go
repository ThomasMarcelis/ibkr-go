// Resolve the nearest complete AAPL option chain and print a small sample.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/option-chain
package main

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/examples/internal/exampleutil"
	"github.com/shopspring/decimal"
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

	// 1. Qualify the underlying to get its contract ID.
	underlying, err := client.Contracts().Qualify(ctx, ibkr.Stock("AAPL"))
	if err != nil {
		return err
	}

	// 2. Ask which chains exist: one row per exchange and trading class,
	//    each with its expirations and strikes.
	chains, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  underlying.Symbol,
		UnderlyingSecType: underlying.SecType,
		UnderlyingConID:   underlying.ConID,
	})
	if err != nil {
		return err
	}
	chain, ok := smartChain(chains, underlying.Symbol)
	if !ok {
		return fmt.Errorf("IBKR returned no SMART %s option chain with multiplier 100", underlying.Symbol)
	}
	expiry, ok := nearestExpiry(chain.Expirations)
	if !ok {
		return fmt.Errorf("chain returned no YYYYMMDD expiry")
	}

	// 3. Resolve every contract in that expiry.
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
		return fmt.Errorf("IBKR returned no %s option contracts for %s", underlying.Symbol, expiry)
	}

	slices.SortFunc(details, func(a, b ibkr.ContractDetails) int {
		return cmp.Or(cmp.Compare(a.Right, b.Right), strike(a).Cmp(strike(b)))
	})
	fmt.Printf("%s %s: %d contracts\n", underlying.Symbol, expiry, len(details))
	for _, detail := range details[:min(12, len(details))] {
		fmt.Printf("  %s %8s  %s\n", detail.Right, strike(detail), detail.LocalSymbol)
	}
	if len(details) > 12 {
		fmt.Printf("  ... %d more\n", len(details)-12)
	}
	return nil
}

// smartChain picks the standard SMART-routed, 100-multiplier chain whose
// trading class is the symbol itself (weeklies and adjusted classes differ).
func smartChain(chains []ibkr.SecDefOptParams, tradingClass string) (ibkr.SecDefOptParams, bool) {
	for _, chain := range chains {
		if chain.Exchange == "SMART" && chain.TradingClass == tradingClass && chain.Multiplier == "100" {
			return chain, true
		}
	}
	return ibkr.SecDefOptParams{}, false
}

func nearestExpiry(expirations []string) (string, bool) {
	nearest := ""
	for _, expiry := range expirations {
		if _, err := time.Parse("20060102", expiry); err == nil && (nearest == "" || expiry < nearest) {
			nearest = expiry
		}
	}
	return nearest, nearest != ""
}

// IBKR omits the strike on a few placeholder rows; treat those as zero.
func strike(detail ibkr.ContractDetails) decimal.Decimal {
	if detail.Strike == nil {
		return decimal.Zero
	}
	return *detail.Strike
}
