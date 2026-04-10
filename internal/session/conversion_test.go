package session

import (
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func TestFromCodecOpenOrderRejectsMalformedNonEmptyNumericField(t *testing.T) {
	t.Parallel()

	_, err := fromCodecOpenOrder(codec.OpenOrder{
		OrderID:   1,
		Account:   "DU12345",
		Contract:  codec.Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
		Action:    "BUY",
		OrderType: "LMT",
		Quantity:  "1",
		ClientID:  "not-an-int",
	})
	if err == nil {
		t.Fatal("fromCodecOpenOrder() error = nil, want malformed client id rejection")
	}
}

func TestFromCodecOrderStatusRejectsMalformedNonEmptyDecimalField(t *testing.T) {
	t.Parallel()

	_, err := fromCodecOrderStatus(codec.OrderStatus{
		OrderID:      1,
		Status:       "Submitted",
		Filled:       "abc",
		Remaining:    "1",
		AvgFillPrice: "0",
	})
	if err == nil {
		t.Fatal("fromCodecOrderStatus() error = nil, want malformed filled rejection")
	}
}

func TestParseOptionalDecimalAllowsBlank(t *testing.T) {
	t.Parallel()

	value, err := parseOptionalDecimal("", "test field")
	if err != nil {
		t.Fatalf("parseOptionalDecimal() error = %v", err)
	}
	if !value.IsZero() {
		t.Fatalf("parseOptionalDecimal() = %s, want zero", value.String())
	}
}
