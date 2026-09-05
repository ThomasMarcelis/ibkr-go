package ibkr

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func TestHedgeReplacePreservesBoundParentBeforeValidation(t *testing.T) {
	// Exact sv225 api_hedge_order_aapl capture; events.jsonl SHA-256:
	// 68514f4d1f92b17ca2141c2cf7834387881e889aabf6aaef2ca2adab6591f242.
	// Omission/conflict are caller-side validation inputs; no new wire reply.
	var echo codec.OpenOrder
	for _, message := range capturedServerMessages(t, "testdata/transcripts/api_hedge_order_aapl.txt") {
		if m, ok := message.(codec.OpenOrder); ok && m.HedgeType != "" {
			echo = m
			break
		}
	}
	parent, err := strconv.ParseInt(echo.ParentID, 10, 64)
	if err != nil || parent <= 0 {
		t.Fatalf("captured parent = %q, %v", echo.ParentID, err)
	}
	contract, err := fromCodecContract(echo.Contract)
	if err != nil {
		t.Fatal(err)
	}
	quantity, err := decimal.NewFromString(echo.Quantity)
	if err != nil {
		t.Fatal(err)
	}
	limit, err := parseOptionalDecimalPointer(echo.LmtPrice, "limit")
	if err != nil {
		t.Fatal(err)
	}
	order := Order{Action: OrderAction(echo.Action), OrderType: OrderType(echo.OrderType), Quantity: quantity, LmtPrice: limit, TIF: TimeInForce(echo.TIF), Account: echo.Account, ParentID: parent, Hedge: OrderHedge{Type: HedgeType(echo.HedgeType), Param: echo.HedgeParam}}
	if err := validateOrderRequest(PlaceOrderRequest{Contract: contract, Order: order}); err != nil {
		t.Fatal(err)
	}
	e, peer := newObservedMarketDataEngine(t)
	h := e.bindOrderHandle(echo.OrderID, contract, parent)
	replace := func(order Order) error {
		result := make(chan error, 1)
		go func() { result <- h.Replace(context.Background(), order) }()
		select {
		case err := <-result:
			return err
		case command := <-e.cmds:
			command()
			return <-result
		}
	}
	want, err := codec.Encode(225, toCodecPlaceOrder(echo.OrderID, PlaceOrderRequest{Contract: contract, Order: order}))
	if err != nil {
		t.Fatal(err)
	}
	for _, suppliedParent := range []int64{0, parent} {
		order.ParentID = suppliedParent
		if err := replace(order); err != nil {
			t.Fatalf("Replace(parent=%d): %v", suppliedParent, err)
		}
		if got := readObservedFrame(t, peer); !bytes.Equal(got, want) {
			t.Fatalf("replacement changed bound parent: %x", got)
		}
	}
	order.ParentID = parent + 1
	if err := replace(order); err == nil {
		t.Fatal("conflicting parent accepted")
	} else if v, ok := errors.AsType[*ValidationError](err); !ok || v.Field != "Order.ParentID" {
		t.Fatalf("conflict: %v", err)
	}
	order.ParentID = 0
	e.orders[echo.OrderID].recoveryRequired = true
	if err := replace(order); !errors.Is(err, ErrOrderRecoveryRequired) {
		t.Fatalf("recovery Replace: %v", err)
	}
}
