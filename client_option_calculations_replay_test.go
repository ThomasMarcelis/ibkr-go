package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

func TestOptionCalculationsLiveReplay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(204)
	defer restore()

	client, host := newClient(t, "option_calculations_aapl_live.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	contract := ibkr.Contract{
		ConID:        887307502,
		Symbol:       "AAPL",
		SecType:      ibkr.SecTypeOption,
		Expiry:       "20260710",
		Strike:       decimal.NewFromInt(315),
		Right:        ibkr.RightCall,
		Multiplier:   "100",
		Exchange:     "SMART",
		Currency:     "USD",
		LocalSymbol:  "AAPL  260710C00315000",
		TradingClass: "AAPL",
	}
	price, err := client.Options().Price(ctx, ibkr.CalcOptionPriceRequest{
		Contract:   contract,
		Volatility: decimal.RequireFromString("0.3"),
		UnderPrice: decimal.RequireFromString("314.5"),
	})
	if err != nil {
		t.Fatalf("Price() error = %v", err)
	}
	wantPriceFields := ibkr.OptionComputationImpliedVol |
		ibkr.OptionComputationDelta |
		ibkr.OptionComputationPrice |
		ibkr.OptionComputationGamma |
		ibkr.OptionComputationVega |
		ibkr.OptionComputationTheta |
		ibkr.OptionComputationUnderlyingPrice
	if price.Available != wantPriceFields {
		t.Fatalf("Price().Available = %d, want %d", price.Available, wantPriceFields)
	}
	assertDecimal := func(name string, got decimal.Decimal, want string) {
		t.Helper()
		if !got.Equal(decimal.RequireFromString(want)) {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	assertDecimal("Price().ImpliedVol", price.ImpliedVol, "0.3")
	assertDecimal("Price().Delta", price.Delta, "0.4248045691043341")
	assertDecimal("Price().OptPrice", price.OptPrice, "0.7871894567385895")
	assertDecimal("Price().Gamma", price.Gamma, "0.15506144675853706")
	assertDecimal("Price().Vega", price.Vega, "0.033808051635485614")
	assertDecimal("Price().Theta", price.Theta, "-0.7871894567385895")
	assertDecimal("Price().UndPrice", price.UndPrice, "314.5")
	if !price.PvDividend.IsZero() {
		t.Errorf("Price().PvDividend = %s, want unavailable zero", price.PvDividend)
	}

	implied, err := client.Options().ImpliedVolatility(ctx, ibkr.CalcImpliedVolatilityRequest{
		Contract:    contract,
		OptionPrice: price.OptPrice,
		UnderPrice:  decimal.RequireFromString("314.5"),
	})
	if err != nil {
		t.Fatalf("ImpliedVolatility() error = %v", err)
	}
	wantImpliedFields := ibkr.OptionComputationImpliedVol |
		ibkr.OptionComputationPrice |
		ibkr.OptionComputationUnderlyingPrice
	if implied.Available != wantImpliedFields {
		t.Fatalf("ImpliedVolatility().Available = %d, want %d", implied.Available, wantImpliedFields)
	}
	assertDecimal("ImpliedVolatility().ImpliedVol", implied.ImpliedVol, "0.30000000111488545")
	assertDecimal("ImpliedVolatility().OptPrice", implied.OptPrice, "0.7871894567385895")
	assertDecimal("ImpliedVolatility().UndPrice", implied.UndPrice, "314.5")
	if !implied.Delta.IsZero() || !implied.PvDividend.IsZero() || !implied.Gamma.IsZero() ||
		!implied.Vega.IsZero() || !implied.Theta.IsZero() {
		t.Errorf("ImpliedVolatility() unavailable fields = %+v, want zero", implied)
	}
}
