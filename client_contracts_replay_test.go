package ibkr_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

// Replay coverage for the contract-details asset-type matrix (REF-001).
// Each transcript is derived from a live IB Gateway server_version=200
// capture; the capture directory and events.jsonl hash are recorded in the
// transcript headers.

func TestContractDetailsAAPLOptionReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_aapl_opt.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeOption,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) != 4 {
		t.Fatalf("details len = %d, want 4", len(details))
	}

	first := details[0]
	if first.ConID != 675811965 {
		t.Errorf("ConID = %d, want 675811965", first.ConID)
	}
	if first.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", first.Symbol)
	}
	if first.SecType != ibkr.SecTypeOption {
		t.Errorf("SecType = %q, want OPT", first.SecType)
	}
	if first.Expiry != "20260618" {
		t.Errorf("Expiry = %q, want 20260618", first.Expiry)
	}
	if !first.Strike.Equal(decimal.RequireFromString("100")) {
		t.Errorf("Strike = %s, want 100", first.Strike)
	}
	if first.Right != ibkr.RightCall {
		t.Errorf("Right = %q, want C", first.Right)
	}
	if first.Exchange != "SMART" {
		t.Errorf("Exchange = %q, want SMART", first.Exchange)
	}
	if first.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", first.Currency)
	}
	if first.LocalSymbol != "AAPL  260618C00100000" {
		t.Errorf("LocalSymbol = %q, want AAPL  260618C00100000", first.LocalSymbol)
	}
	if first.TradingClass != "AAPL" {
		t.Errorf("TradingClass = %q, want AAPL", first.TradingClass)
	}
	if first.MarketName != "AAPL" {
		t.Errorf("MarketName = %q, want AAPL", first.MarketName)
	}
	if first.MinTick.String() != "0.01" {
		t.Errorf("MinTick = %s, want 0.01", first.MinTick.String())
	}
	if first.LongName != "APPLE INC" {
		t.Errorf("LongName = %q, want APPLE INC", first.LongName)
	}
	if first.TimeZoneID != "US/Eastern" {
		t.Errorf("TimeZoneID = %q, want US/Eastern", first.TimeZoneID)
	}

	wantCalls := []struct {
		conID  int
		strike string
	}{
		{675811965, "100"},
		{675812035, "105"},
		{675812080, "110"},
	}
	for i, want := range wantCalls {
		if details[i].ConID != want.conID {
			t.Errorf("details[%d].ConID = %d, want %d", i, details[i].ConID, want.conID)
		}
		if !details[i].Strike.Equal(decimal.RequireFromString(want.strike)) {
			t.Errorf("details[%d].Strike = %s, want %s", i, details[i].Strike, want.strike)
		}
		if details[i].Right != ibkr.RightCall {
			t.Errorf("details[%d].Right = %q, want C", i, details[i].Right)
		}
	}

	put := details[3]
	if put.ConID != 675815175 {
		t.Errorf("put ConID = %d, want 675815175", put.ConID)
	}
	if put.Right != ibkr.RightPut {
		t.Errorf("put Right = %q, want P", put.Right)
	}
	if !put.Strike.Equal(decimal.RequireFromString("100")) {
		t.Errorf("put Strike = %s, want 100", put.Strike)
	}
	if put.LocalSymbol != "AAPL  260618P00100000" {
		t.Errorf("put LocalSymbol = %q, want AAPL  260618P00100000", put.LocalSymbol)
	}
}

func TestContractDetailsAppleBondsReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_apple_bonds.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	details, err := client.Contracts().Details(ctx, ibkr.Contract{IssuerID: "e1432232"})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("details len = %d, want 2", len(details))
	}

	first := details[0]
	if first.ConID != 127128131 || first.SecType != ibkr.SecTypeBond || first.TradingClass != "AAPL" {
		t.Errorf("first identity = conID %d secType %q tradingClass %q", first.ConID, first.SecType, first.TradingClass)
	}
	if first.Bond == nil {
		t.Fatal("first Bond = nil")
	}
	if first.Bond.CUSIP != "IBCID127128131" || first.Bond.Coupon != nil || first.Bond.DescriptionAppend != "AAPL 3.85 05/04/43" {
		t.Errorf("first Bond = %+v", *first.Bond)
	}
	if len(first.SecurityIDs) != 2 || first.SecurityIDs[0] != (ibkr.TagValue{Tag: "CUSIP", Value: "037833AL4"}) || first.SecurityIDs[1] != (ibkr.TagValue{Tag: "ISIN", Value: "US037833AL42"}) {
		t.Errorf("first SecurityIDs = %+v", first.SecurityIDs)
	}
	if len(first.ValidExchanges) != 1 || first.ValidExchanges[0] != (ibkr.ContractExchange{Exchange: "SMART", MarketRuleID: 1386}) {
		t.Errorf("first ValidExchanges = %+v", first.ValidExchanges)
	}
	if first.MinTick.String() != "0.0001" || first.MinSize == nil || first.MinSize.String() != "2" || first.SizeIncrement == nil || first.SizeIncrement.String() != "1" {
		t.Errorf("first tick/size = %s / %v / %v", first.MinTick, first.MinSize, first.SizeIncrement)
	}

	second := details[1]
	if second.ConID != 194154340 || second.Bond == nil || second.Bond.CUSIP != "IBCID194154340" || second.Bond.DescriptionAppend != "AAPL 1 5/8 11/10/26" {
		t.Errorf("second = %+v", second)
	}
	if len(second.SecurityIDs) != 1 || second.SecurityIDs[0] != (ibkr.TagValue{Tag: "ISIN", Value: "XS1135337498"}) {
		t.Errorf("second SecurityIDs = %+v", second.SecurityIDs)
	}
	if second.MinSize == nil || second.MinSize.String() != "100" {
		t.Errorf("second MinSize = %v, want 100", second.MinSize)
	}
}

func TestContractDetailsEURUSDCashReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_eurusd_cash.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "EUR",
		SecType:  ibkr.SecTypeForex,
		Exchange: "IDEALPRO",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("details len = %d, want 1", len(details))
	}

	d := details[0]
	if d.ConID != 12087792 {
		t.Errorf("ConID = %d, want 12087792", d.ConID)
	}
	if d.Symbol != "EUR" {
		t.Errorf("Symbol = %q, want EUR", d.Symbol)
	}
	if d.SecType != ibkr.SecTypeForex {
		t.Errorf("SecType = %q, want CASH", d.SecType)
	}
	if d.Exchange != "IDEALPRO" {
		t.Errorf("Exchange = %q, want IDEALPRO", d.Exchange)
	}
	if d.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", d.Currency)
	}
	if d.LocalSymbol != "EUR.USD" {
		t.Errorf("LocalSymbol = %q, want EUR.USD", d.LocalSymbol)
	}
	if d.TradingClass != "EUR.USD" {
		t.Errorf("TradingClass = %q, want EUR.USD", d.TradingClass)
	}
	if d.MarketName != "EUR.USD" {
		t.Errorf("MarketName = %q, want EUR.USD", d.MarketName)
	}
	if d.MinTick.String() != "0.00005" {
		t.Errorf("MinTick = %s, want 0.00005", d.MinTick.String())
	}
	if d.LongName != "European Monetary Union Euro" {
		t.Errorf("LongName = %q, want European Monetary Union Euro", d.LongName)
	}
	if d.TimeZoneID != "US/Eastern" {
		t.Errorf("TimeZoneID = %q, want US/Eastern", d.TimeZoneID)
	}
	if d.PriceMagnifier != 1 || d.AggGroup == nil || *d.AggGroup != 4 {
		t.Errorf("numeric metadata = magnifier %d aggregate group %v", d.PriceMagnifier, d.AggGroup)
	}
	if len(d.ValidExchanges) != 1 || d.ValidExchanges[0] != (ibkr.ContractExchange{Exchange: "IDEALPRO", MarketRuleID: 3188}) {
		t.Errorf("ValidExchanges = %#v", d.ValidExchanges)
	}
	if d.TradingHours == "" || d.LiquidHours == "" {
		t.Errorf("hours missing: trading=%t liquid=%t", d.TradingHours != "", d.LiquidHours != "")
	}
	if d.MinSize == nil || d.MinSize.String() != "0.01" || d.SizeIncrement == nil || d.SuggestedSizeIncrement == nil {
		t.Errorf("size rules = %v/%v/%v", d.MinSize, d.SizeIncrement, d.SuggestedSizeIncrement)
	}
	hasCashQuantity := false
	for _, orderType := range d.OrderTypes {
		if orderType == "CASHQTY" {
			hasCashQuantity = true
			break
		}
	}
	if !hasCashQuantity {
		t.Errorf("OrderTypes = %#v, want live CASHQTY capability", d.OrderTypes)
	}
	if d.Expiry != "" || d.Right != "" {
		t.Errorf("Expiry/Right = %q/%q, want empty for CASH", d.Expiry, d.Right)
	}
}

func TestContractDetailsESFutureReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_es_fut.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "ES",
		SecType:  ibkr.SecTypeFuture,
		Exchange: "CME",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) != 21 {
		t.Fatalf("details len = %d, want 21 expiries", len(details))
	}

	front := details[0]
	if front.ConID != 515416632 {
		t.Errorf("front ConID = %d, want 515416632", front.ConID)
	}
	if front.Symbol != "ES" {
		t.Errorf("front Symbol = %q, want ES", front.Symbol)
	}
	if front.SecType != ibkr.SecTypeFuture {
		t.Errorf("front SecType = %q, want FUT", front.SecType)
	}
	if front.LocalSymbol != "ESZ6" {
		t.Errorf("front LocalSymbol = %q, want ESZ6", front.LocalSymbol)
	}
	if front.Expiry != "20261218" || front.LastTradeDate != "20261218" || front.LastTradeTime != "08:30:00" {
		t.Errorf("front expiry fields = %q/%q/%q, want 20261218/20261218/08:30:00", front.Expiry, front.LastTradeDate, front.LastTradeTime)
	}
	if front.Exchange != "CME" {
		t.Errorf("front Exchange = %q, want CME", front.Exchange)
	}
	if front.Currency != "USD" {
		t.Errorf("front Currency = %q, want USD", front.Currency)
	}
	if front.MinTick.String() != "0.25" {
		t.Errorf("front MinTick = %s, want 0.25", front.MinTick.String())
	}
	if front.MarketName != "ES" {
		t.Errorf("front MarketName = %q, want ES", front.MarketName)
	}
	if front.LongName != "E-mini S&P 500" {
		t.Errorf("front LongName = %q, want E-mini S&P 500", front.LongName)
	}
	if front.TimeZoneID != "US/Central" {
		t.Errorf("front TimeZoneID = %q, want US/Central", front.TimeZoneID)
	}

	for i, d := range details {
		if d.SecType != ibkr.SecTypeFuture {
			t.Errorf("details[%d].SecType = %q, want FUT", i, d.SecType)
		}
		if d.TradingClass != "ES" {
			t.Errorf("details[%d].TradingClass = %q, want ES", i, d.TradingClass)
		}
	}

	last := details[20]
	if last.ConID != 866514761 {
		t.Errorf("last ConID = %d, want 866514761", last.ConID)
	}
	if last.LocalSymbol != "ESM1" {
		t.Errorf("last LocalSymbol = %q, want ESM1", last.LocalSymbol)
	}
	if last.Expiry != "20310620" || last.LastTradeDate != "20310620" || last.LastTradeTime != "08:30:00" {
		t.Errorf("last expiry fields = %q/%q/%q, want 20310620/20310620/08:30:00", last.Expiry, last.LastTradeDate, last.LastTradeTime)
	}
}

func TestContractDetailsNotFoundReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_not_found.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "ZZZZNONE",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err == nil {
		t.Fatalf("ContractDetails() = %v, want code 200 API error", details)
	}

	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok {
		t.Fatalf("error type = %T, want *ibkr.APIError", err)
	}
	if apiErr.Code != 200 {
		t.Errorf("APIError.Code = %d, want 200", apiErr.Code)
	}
	if apiErr.Message != "No security definition has been found for the request" {
		t.Errorf("APIError.Message = %q, want live not-found text", apiErr.Message)
	}
	if apiErr.OpKind != ibkr.OpContractDetails {
		t.Errorf("APIError.OpKind = %q, want %q", apiErr.OpKind, ibkr.OpContractDetails)
	}
}

func TestQualifyContractAmbiguousReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "qualify_contract_ambiguous.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// MSFT STK with no exchange matches one contractData row per exchange
	// (26 in the live capture), so Qualify must refuse to pick one.
	d, err := client.Contracts().Qualify(ctx, ibkr.Contract{
		Symbol:   "MSFT",
		SecType:  ibkr.SecTypeStock,
		Currency: "USD",
	})
	if !errors.Is(err, ibkr.ErrAmbiguousContract) {
		t.Fatalf("Qualify() error = %v, want ErrAmbiguousContract", err)
	}
	if d.ConID != 0 || d.Symbol != "" {
		t.Errorf("Qualify() details = %+v, want zero value on ambiguity", d)
	}
}
