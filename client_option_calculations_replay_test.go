package ibkr_test

import (
	"context"
	"slices"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func TestOptionCalculationsSV211Replay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(211)
	defer restore()

	client, host := newClient(t, "option_calculations_aapl.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	quote := replayDelayedAAPLQuoteAnchor(t, ctx, client)
	anchor := decimal.Zero
	for _, candidate := range []decimal.Decimal{quote.Last, quote.Ask, quote.Bid, quote.Close} {
		if candidate.IsPositive() {
			anchor = candidate
			break
		}
	}
	if !anchor.Equal(decimal.RequireFromString("316.89")) {
		t.Fatalf("quote anchor = %s, want captured 316.89", anchor)
	}

	parameters, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   265598,
	})
	if err != nil {
		t.Fatalf("SecDefOptParams(): %v", err)
	}
	if len(parameters) != 1 {
		t.Fatalf("SecDefOptParams() len = %d, want projected SMART row", len(parameters))
	}
	parameter := parameters[0]
	strike := decimal.RequireFromString("317.5")
	if parameter.Exchange != "SMART" || parameter.TradingClass != "AAPL" || parameter.Multiplier != "100" ||
		!slices.Contains(parameter.Expirations, "20260713") || !slices.ContainsFunc(parameter.Strikes, strike.Equal) {
		t.Fatalf("SMART parameters = %+v, want captured AAPL 20260713 C317.5 inputs", parameter)
	}

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:       "AAPL",
		SecType:      ibkr.SecTypeOption,
		Expiry:       "20260713",
		Strike:       new(strike),
		Right:        ibkr.RightCall,
		Multiplier:   parameter.Multiplier,
		Exchange:     "SMART",
		Currency:     "USD",
		TradingClass: parameter.TradingClass,
	})
	if err != nil {
		t.Fatalf("ContractDetails(): %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("ContractDetails() len = %d, want 1", len(details))
	}
	contract := details[0].Contract
	if contract.ConID != 897862208 || contract.LocalSymbol != "AAPL  260713C00317500" {
		t.Fatalf("qualified option = %+v, want captured AAPL call", contract)
	}

	price, err := client.Options().Price(ctx, ibkr.CalcOptionPriceRequest{
		Contract:   contract,
		Volatility: decimal.RequireFromString("0.3"),
		UnderPrice: anchor,
	})
	if err != nil {
		t.Fatalf("Price(): %v", err)
	}
	if price.Available != 247 {
		t.Fatalf("Price().Available = %d, want 247", price.Available)
	}
	assertOptionDecimal(t, "Price().ImpliedVol", price.ImpliedVol, "0.3")
	assertOptionDecimal(t, "Price().Delta", price.Delta, "0.37990541015940876")
	assertOptionDecimal(t, "Price().OptPrice", price.OptPrice, "0.5172515148628609")
	assertOptionDecimal(t, "Price().Gamma", price.Gamma, "0.19516303062548224")
	assertOptionDecimal(t, "Price().Vega", price.Vega, "0.02496717315373298")
	assertOptionDecimal(t, "Price().Theta", price.Theta, "-0.5172515148628609")
	assertOptionDecimal(t, "Price().UndPrice", price.UndPrice, "316.89")

	implied, err := client.Options().ImpliedVolatility(ctx, ibkr.CalcImpliedVolatilityRequest{
		Contract:    contract,
		OptionPrice: price.OptPrice,
		UnderPrice:  anchor,
	})
	if err != nil {
		t.Fatalf("ImpliedVolatility(): %v", err)
	}
	if implied.Available != 133 {
		t.Fatalf("ImpliedVolatility().Available = %d, want 133", implied.Available)
	}
	assertOptionDecimal(t, "ImpliedVolatility().ImpliedVol", implied.ImpliedVol, "0.3000000156250214")
	assertOptionDecimal(t, "ImpliedVolatility().OptPrice", implied.OptPrice, "0.5172515148628609")
	assertOptionDecimal(t, "ImpliedVolatility().UndPrice", implied.UndPrice, "316.89")
}

func assertOptionDecimal(t *testing.T, name string, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(decimal.RequireFromString(want)) {
		t.Errorf("%s = %s, want %s", name, got, want)
	}
}
