package ibkr_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
)

// Replay coverage for the contract-details asset-type matrix (REF-001).
// Each transcript is derived from a live IB Gateway capture; the server
// version, capture identity, and events hash are recorded in its header.

func TestContractDetailsAAPLOptionReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_aapl_opt.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	parameters, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   265598,
	})
	if err != nil {
		t.Fatalf("SecDefOptParams() error = %v", err)
	}
	var foundExpiry bool
	for _, parameter := range parameters {
		if parameter.Exchange == "SMART" && parameter.TradingClass == "AAPL" && slices.Contains(parameter.Expirations, "20260824") {
			foundExpiry = true
			break
		}
	}
	if !foundExpiry {
		t.Fatal("SMART AAPL option parameters lack captured nearest expiry 20260824")
	}

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeOption,
		Expiry:   "20260824",
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) != 112 {
		t.Fatalf("details len = %d, want 112", len(details))
	}

	first := details[0]
	if first.ConID != 909446159 {
		t.Errorf("ConID = %d, want 909446159", first.ConID)
	}
	if first.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", first.Symbol)
	}
	if first.SecType != ibkr.SecTypeOption {
		t.Errorf("SecType = %q, want OPT", first.SecType)
	}
	if first.Expiry != "20260824" {
		t.Errorf("Expiry = %q, want 20260824", first.Expiry)
	}
	if first.Strike == nil || first.Strike.String() != "310" {
		t.Errorf("Strike = %s, want 310", first.Strike)
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
	if first.LocalSymbol != "AAPL  260824C00310000" {
		t.Errorf("LocalSymbol = %q, want AAPL  260824C00310000", first.LocalSymbol)
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
	if first.TimeZoneID != "" {
		t.Errorf("TimeZoneID = %q, want absent in sv225 protobuf echo", first.TimeZoneID)
	}

	var sawPut bool
	for _, detail := range details {
		if detail.Right == ibkr.RightPut {
			sawPut = true
			break
		}
	}
	if !sawPut {
		t.Fatal("nearest-expiry ladder contains no put")
	}
}

func TestContractDetailsAAPLOptionSettlementReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_settlement_aapl_opt.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The source campaign qualified the current option expiry before asking
	// for its contract ladder, so retain that captured request sequence.
	if _, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   265598,
	}); err != nil {
		t.Fatalf("SecDefOptParams() error = %v", err)
	}
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeOption,
		Expiry:   "20260828",
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		t.Fatalf("ContractDetails() error = %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("details len = %d, want retained captured row", len(details))
	}
	if details[0].SecType != ibkr.SecTypeOption || details[0].SettlementMethod != "Physical Delivery" {
		t.Fatalf("details = secType %q settlement %q, want OPT/Physical Delivery", details[0].SecType, details[0].SettlementMethod)
	}
}

func TestContractDetailsAppleBondsReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_apple_bonds.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Contracts().Details(ctx, ibkr.Contract{IssuerID: "e1432232"})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != 2130 || apiErr.OpKind != ibkr.OpContractDetails || !strings.Contains(apiErr.Message, "2 products are trading on the basis of currency price with factor") {
		t.Fatalf("ContractDetails() error = %v, want typed code-2130 issuer ambiguity blocker", err)
	}
	if ibkr.IsRetryable(apiErr) {
		t.Fatalf("ContractDetails() error = %v, want non-retryable", err)
	}
}

func TestContractDetailsEURUSDCashReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_eurusd_cash.txt")
	defer cleanupClientHost(t, client, host)

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
	defer cleanupClientHost(t, client, host)

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
	if front.ConID != 649180671 {
		t.Errorf("front ConID = %d, want 649180671", front.ConID)
	}
	if front.Symbol != "ES" {
		t.Errorf("front Symbol = %q, want ES", front.Symbol)
	}
	if front.LocalSymbol != "ESU6" {
		t.Errorf("front LocalSymbol = %q, want ESU6", front.LocalSymbol)
	}
	if front.Expiry != "20260918" || front.LastTradeDate != "20260918" || front.LastTradeTime != "08:30:00" {
		t.Errorf("front expiry fields = %q/%q/%q, want 20260918/20260918/08:30:00", front.Expiry, front.LastTradeDate, front.LastTradeTime)
	}

	for i, d := range details {
		if d.SecType != ibkr.SecTypeFuture {
			t.Errorf("details[%d].SecType = %q, want FUT", i, d.SecType)
		}
		if d.TradingClass != "ES" {
			t.Errorf("details[%d].TradingClass = %q, want ES", i, d.TradingClass)
		}
		if d.Exchange != "CME" || d.Currency != "USD" {
			t.Errorf("details[%d] venue = %q/%q, want CME/USD", i, d.Exchange, d.Currency)
		}
		if d.MarketName != "ES" || d.LongName != "E-mini S&P 500" {
			t.Errorf("details[%d] identity = %q/%q, want ES/E-mini S&P 500", i, d.MarketName, d.LongName)
		}
		if d.MinTick.String() != "0.25" || d.TimeZoneID != "US/Central" {
			t.Errorf("details[%d] terms = %s/%q, want 0.25/US/Central", i, d.MinTick.String(), d.TimeZoneID)
		}
	}
}

func TestContractDetailsESFutureSettlementReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_settlement_es_fut.txt")
	defer cleanupClientHost(t, client, host)

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
	if len(details) != 1 {
		t.Fatalf("details len = %d, want retained captured row", len(details))
	}
	if details[0].SecType != ibkr.SecTypeFuture || details[0].SettlementMethod != "Cash" {
		t.Fatalf("details = secType %q settlement %q, want FUT/Cash", details[0].SecType, details[0].SettlementMethod)
	}
}

func TestContractDetailsNotFoundReplay(t *testing.T) {
	t.Parallel()

	client, host := newClient(t, "contract_details_not_found.txt")
	defer cleanupClientHost(t, client, host)

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
	defer cleanupClientHost(t, client, host)

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
