package ibkr

import (
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func TestToCodecPlaceOrderMapsIncludeOvernight(t *testing.T) {
	t.Parallel()

	request := PlaceOrderRequest{
		Contract: Contract{Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order: Order{
			Action: ActionBuy, OrderType: OrderTypeMarket, Quantity: decimal.NewFromInt(1), TIF: TIFDay,
		},
	}
	if got := toCodecPlaceOrder(78, request).IncludeOvernight; got != "" {
		t.Fatalf("default include overnight = %q, want empty", got)
	}
	request.Order.IncludeOvernight = true
	if got := toCodecPlaceOrder(78, request).IncludeOvernight; got != "1" {
		t.Fatalf("enabled include overnight = %q, want 1", got)
	}
}

func TestFromCodecCompletedOrderProjectsIncludeOvernightPresence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		wire  string
		want  bool
		isNil bool
	}{
		{name: "absent", isNil: true},
		{name: "disabled", wire: "0", want: false},
		{name: "enabled", wire: "1", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			order, err := fromCodecCompletedOrder(codec.CompletedOrder{OrderDetails: codec.OrderDetails{
				Contract: codec.Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK", Strike: "0", Exchange: "SMART", Currency: "USD",
				},
				Quantity: "1", Filled: "0", IncludeOvernight: tc.wire,
			}})
			if err != nil {
				t.Fatal(err)
			}
			got := order.Order.IncludeOvernight
			if tc.isNil {
				if got != nil {
					t.Fatalf("include overnight = %v, want nil", got)
				}
				return
			}
			if got == nil || *got != tc.want {
				t.Fatalf("include overnight = %v, want %t", got, tc.want)
			}
		})
	}
}

func TestCloneOrderDetailsOwnsIncludeOvernight(t *testing.T) {
	t.Parallel()

	original := OrderDetails{IncludeOvernight: new(true)}
	cloned := cloneOrderDetails(original)
	*original.IncludeOvernight = false
	if cloned.IncludeOvernight == nil || !*cloned.IncludeOvernight {
		t.Fatalf("cloned include overnight = %v, want true", cloned.IncludeOvernight)
	}
}
