package ibkr

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func TestPlacePresetBracketSendsOneAttachedOrderRequest(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 218
	req := PlaceOrderRequest{
		Contract: Contract{ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
		Order:    Order{Action: ActionBuy, OrderType: OrderTypeLimit, Quantity: decimal.NewFromInt(1), LmtPrice: new(decimal.NewFromInt(1)), TIF: TIFDay},
	}
	result := make(chan struct {
		bracket BracketOrder
		err     error
	}, 1)

	go func() {
		bracket, err := e.PlacePresetBracket(context.Background(), req)
		result <- struct {
			bracket BracketOrder
			err     error
		}{bracket, err}
	}()
	(<-e.cmds)()

	out := <-result
	if out.err != nil {
		t.Fatal(out.err)
	}
	if out.bracket.Parent.OrderID() != 1 || out.bracket.StopLoss.OrderID() != 2 || out.bracket.TakeProfit.OrderID() != 3 {
		t.Fatalf("preset bracket IDs = parent %d stop %d profit %d", out.bracket.Parent.OrderID(), out.bracket.StopLoss.OrderID(), out.bracket.TakeProfit.OrderID())
	}
	want, err := codec.Encode(218, toCodecPresetBracketOrder(1, 2, 3, req))
	if err != nil {
		t.Fatal(err)
	}
	if got := readObservedFrame(t, peer); !bytes.Equal(got, want) {
		t.Fatalf("preset bracket request = %x, want %x", got, want)
	}
	if len(e.orders) != 3 {
		t.Fatalf("order routes = %d, want 3", len(e.orders))
	}
}

func TestPlacePresetBracketRejectsServerVersion217BeforeAllocation(t *testing.T) {
	e, _ := newObservedMarketDataEngine(t)
	e.serverVersion = 217
	result := make(chan error, 1)
	go func() {
		_, err := e.PlacePresetBracket(context.Background(), PlaceOrderRequest{
			Contract: Contract{ConID: 265598, SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
			Order:    Order{Action: ActionBuy, OrderType: OrderTypeMarket, Quantity: decimal.NewFromInt(1)},
		})
		result <- err
	}()
	(<-e.cmds)()
	if err := <-result; !errors.Is(err, ErrUnsupportedServerVersion) {
		t.Fatalf("PlacePresetBracket() error = %v, want ErrUnsupportedServerVersion", err)
	}
	if len(e.orders) != 0 || e.snapshot.NextValidID != 0 {
		t.Fatalf("rejected preset bracket mutated allocation state: routes=%d next=%d", len(e.orders), e.snapshot.NextValidID)
	}
}

func TestPlacePresetBracketParentRejectionClosesAttachedHandles(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 218
	result := make(chan bracketOrderResult, 1)
	go func() {
		bracket, err := e.PlacePresetBracket(context.Background(), PlaceOrderRequest{
			Contract: Contract{ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
			Order:    Order{Action: ActionBuy, OrderType: OrderTypeMarket, Quantity: decimal.NewFromInt(1)},
		})
		result <- bracketOrderResult{bracket: bracket, err: err}
	}()
	(<-e.cmds)()
	out := <-result
	if out.err != nil {
		t.Fatal(out.err)
	}
	_ = readObservedFrame(t, peer)

	e.handleAPIError(codec.APIError{ReqID: 1, Code: ErrCodeOrderRejected, Message: "preset parent rejected"})

	for name, handle := range map[string]*OrderHandle{
		"parent": out.bracket.Parent,
		"profit": out.bracket.TakeProfit,
		"stop":   out.bracket.StopLoss,
	} {
		err := handle.Wait()
		apiErr, ok := errors.AsType[*APIError](err)
		if !ok || !apiErr.IsOrderRejection() {
			t.Fatalf("%s Wait() error = %v, want order-rejection APIError", name, err)
		}
	}
	if len(e.orders) != 0 {
		t.Fatalf("order routes after parent rejection = %d, want 0", len(e.orders))
	}
}

func TestPresetBracketParentDetachKeepsAttachedHandles(t *testing.T) {
	e, peer := newObservedMarketDataEngine(t)
	e.serverVersion = 218
	result := make(chan bracketOrderResult, 1)
	go func() {
		bracket, err := e.PlacePresetBracket(context.Background(), PlaceOrderRequest{
			Contract: Contract{ConID: 265598, Symbol: "AAPL", SecType: SecTypeStock, Exchange: "SMART", Currency: "USD"},
			Order:    Order{Action: ActionBuy, OrderType: OrderTypeMarket, Quantity: decimal.NewFromInt(1)},
		})
		result <- bracketOrderResult{bracket: bracket, err: err}
	}()
	(<-e.cmds)()
	out := <-result
	if out.err != nil {
		t.Fatal(out.err)
	}
	_ = readObservedFrame(t, peer)

	e.closeOrderRoute(1, e.orders[1], nil)

	if _, ok := e.orders[2]; !ok {
		t.Fatal("stop-loss route closed when only the parent observer detached")
	}
	if _, ok := e.orders[3]; !ok {
		t.Fatal("take-profit route closed when only the parent observer detached")
	}
}
