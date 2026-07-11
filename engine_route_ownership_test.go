package ibkr

import (
	"sync"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func TestObservedOpenOrderOwnsEachConsumerPayload(t *testing.T) {
	t.Parallel()

	// This ownership-only composite uses independently grounded fields: order
	// ID 443 and NonGuaranteed routing came from the 2026-06-11 paper BAG
	// campaign (events SHA-256 053baca75621b4a22b8c0e64b87371fcd414082a52e9980f327fe405ccb8ed9e);
	// legs 887307502/887307536 came from the exact-200 BAG quote capture (events
	// SHA-256 1f8354ee5d9ea0570472caa35d905127f5a8c5bab694ba1f9a74532178842c69).
	// The 0.05 per-leg decimal is the official ComboLeg.proto tag-9 source-law
	// vector frozen in codec_orders_proto203_test.go, not a live priced-combo
	// attestation. Combining them here exercises ownership, not Gateway meaning.
	openOrders := make(chan OpenOrder, 1)
	handle := newOrderHandle(443, 64)
	e := &engine{
		orders: map[int64]*orderRoute{443: {orderID: 443, handle: handle}},
		singletons: map[string]*route{singletonOpenOrders: {
			handle: func(msg any, _ *engine) {
				openOrders <- msg.(OpenOrder)
			},
		}},
	}
	e.dispatchObservedOpenOrder(codec.OpenOrder{
		OrderID: 443,
		Contract: codec.Contract{
			Symbol: "AAPL", SecType: "BAG", Exchange: "SMART", Currency: "USD",
			ComboLegs: []codec.ComboLeg{
				{ConID: 887307502, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"},
				{ConID: 887307536, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", ExemptCode: "-1"},
			},
		},
		Action: "BUY", OrderType: "LMT", Quantity: "5", LmtPrice: "0.05", OrderComboLegPrices: []string{"0.05", ""},
		SmartComboRouting: []codec.TagValue{{Tag: "NonGuaranteed", Value: "1"}},
	})

	handleOrder := (<-handle.Events()).OpenOrder
	openOrdersOrder := <-openOrders
	if handleOrder == nil {
		t.Fatal("order handle did not receive open order")
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		handleOrder.Contract.ComboLegs[0].ConID = 887307502
		*handleOrder.Combo.LegPrices[0] = decimal.RequireFromString("0.05")
		handleOrder.Combo.SmartRouting[0].Value = "0"
	}()
	go func() {
		defer wg.Done()
		openOrdersOrder.Contract.ComboLegs[0].ConID = 887307536
		*openOrdersOrder.Combo.LegPrices[0] = decimal.RequireFromString("291.09")
		openOrdersOrder.Combo.SmartRouting[0].Value = "1"
	}()
	wg.Wait()

	if handleOrder.Contract.ComboLegs[0].ConID != 887307502 || openOrdersOrder.Contract.ComboLegs[0].ConID != 887307536 ||
		!handleOrder.Combo.LegPrices[0].Equal(decimal.RequireFromString("0.05")) || !openOrdersOrder.Combo.LegPrices[0].Equal(decimal.RequireFromString("291.09")) ||
		handleOrder.Combo.SmartRouting[0].Value != "0" || openOrdersOrder.Combo.SmartRouting[0].Value != "1" {
		t.Fatalf("dual dispatch shares mutable storage: handle=%+v subscription=%+v", *handleOrder, openOrdersOrder)
	}
}

func TestRequestIDSkipsPendingPreview(t *testing.T) {
	t.Parallel()

	e := &engine{
		nextReqID: 41,
		orders:    make(map[int64]*orderRoute),
		previews: map[int64]*previewRoute{
			41: {result: make(chan previewResult, 1)},
		},
	}

	if got := e.allocReqID(); got != 42 {
		t.Fatalf("allocReqID() = %d, want 42 after pending preview 41", got)
	}
}
