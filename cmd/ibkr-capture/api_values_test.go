package main

import (
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func TestChooseFutureExpirySkipsToday(t *testing.T) {
	t.Parallel()

	now := time.Now()
	today := now.Format("20060102")
	tomorrow := now.AddDate(0, 0, 1).Format("20060102")
	got, ok := chooseFutureExpiry([]string{tomorrow, today})
	if !ok || got != tomorrow {
		t.Fatalf("chooseFutureExpiry() = %q, %t, want %q, true", got, ok, tomorrow)
	}
}

func TestChooseITMCallStrike(t *testing.T) {
	t.Parallel()

	strikes := []decimal.Decimal{
		decimal.NewFromInt(300), decimal.NewFromInt(305), decimal.NewFromInt(310), decimal.NewFromInt(315),
	}
	if got, ok := chooseITMCallStrike(strikes, decimal.NewFromInt(312)); !ok || !got.Equal(decimal.NewFromInt(305)) {
		t.Fatalf("chooseITMCallStrike() = %s, %t; want 305, true", got, ok)
	}
	if _, ok := chooseITMCallStrike([]decimal.Decimal{decimal.NewFromInt(315)}, decimal.NewFromInt(312)); ok {
		t.Fatal("chooseITMCallStrike() found a strike above the underlier")
	}
}

func TestOptionalDecimalString(t *testing.T) {
	t.Parallel()

	if got := optionalDecimalString(nil); got != "" {
		t.Fatalf("optionalDecimalString(nil) = %q, want empty", got)
	}
	if got := optionalDecimalString(new(decimal.RequireFromString("1.25"))); got != "1.25" {
		t.Fatalf("optionalDecimalString(1.25) = %q, want 1.25", got)
	}
}

func TestSetRecordedOrderPricesPreservesOmissions(t *testing.T) {
	t.Parallel()

	event := &apiDriverEvent{}
	setRecordedOrderPrices(event, nil, nil)
	if event.LmtPrice != "" || event.AuxPrice != "" {
		t.Fatalf("recorded omitted prices = %q/%q, want empty", event.LmtPrice, event.AuxPrice)
	}

	setRecordedOrderPrices(event,
		new(decimal.RequireFromString("1.25")),
		new(decimal.RequireFromString("0.50")),
	)
	if event.LmtPrice != "1.25" || event.AuxPrice != "0.5" {
		t.Fatalf("recorded prices = %q/%q, want 1.25/0.5", event.LmtPrice, event.AuxPrice)
	}
}

func TestCampaignPositionDeltasPreserveUnrelatedBaseline(t *testing.T) {
	t.Parallel()

	baseline := []ibkr.Position{
		{Contract: ibkr.Contract{ConID: 10, Symbol: "BASE"}, Position: decimal.RequireFromString("-2")},
		{Contract: ibkr.Contract{ConID: 20, Symbol: "SAME"}, Position: decimal.RequireFromString("7")},
	}
	current := []ibkr.Position{
		{Contract: ibkr.Contract{ConID: 20, Symbol: "SAME"}, Position: decimal.RequireFromString("7")},
		{Contract: ibkr.Contract{ConID: 30, Symbol: "NEW"}, Position: decimal.RequireFromString("1.5")},
	}

	deltas := campaignPositionDeltas(baseline, current)
	if len(deltas) != 2 {
		t.Fatalf("campaignPositionDeltas() = %+v, want two changed contracts", deltas)
	}
	if deltas[0].contract.ConID != 10 || !deltas[0].delta.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("removed baseline delta = %+v, want contract 10 delta +2", deltas[0])
	}
	if deltas[1].contract.ConID != 30 || !deltas[1].delta.Equal(decimal.RequireFromString("1.5")) {
		t.Fatalf("new campaign delta = %+v, want contract 30 delta +1.5", deltas[1])
	}
}

func TestSameAccountValueIdentitiesIgnoresValuesAndOrdering(t *testing.T) {
	t.Parallel()

	baseline := []ibkr.AccountValue{
		{Account: "DU123", Tag: "NetLiquidation", Value: "1000", Currency: "USD"},
		{Account: "DU123", Tag: "BuyingPower", Value: "4000", Currency: "USD"},
	}
	current := []ibkr.AccountValue{
		{Account: "DU123", Tag: "BuyingPower", Value: "3999", Currency: "USD"},
		{Account: "DU123", Tag: "NetLiquidation", Value: "1001", Currency: "USD"},
	}
	if !sameAccountValueIdentities(baseline, current) {
		t.Fatal("sameAccountValueIdentities() rejected unchanged identities with new values")
	}

	current[0].Currency = "EUR"
	if sameAccountValueIdentities(baseline, current) {
		t.Fatal("sameAccountValueIdentities() accepted a changed currency identity")
	}
}
