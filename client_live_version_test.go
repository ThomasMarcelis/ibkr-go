package ibkr_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
	"github.com/shopspring/decimal"
)

// TestLiveDownNegotiatedVersions forces the gateway onto older wire layouts
// by capping the advertised handshake maximum, then exercises the flows
// whose decode paths are version-gated: an API error frame (ErrMsg layout
// changes at 194), contract details (explicit lastTradeDate slot at 182),
// historical bars (inline dataset dates below 196), and the
// CurrentTimeMillis feature gate (197). The boundaries chosen straddle
// every inbound gate. Not parallel: the advertised max is package state.
func TestLiveDownNegotiatedVersions(t *testing.T) {
	for _, sv := range []int{199, 176, 184, 193, 195} {
		t.Run(versionName(sv), func(t *testing.T) {
			restore := ibkr.SetAdvertisedServerVersionMaxForTest(sv)
			defer restore()

			client, _, cancel := ibkrlive.DialContext(t, 15*time.Second)
			defer cancel()
			defer client.Close()

			if got := client.Session().ServerVersion; got != sv {
				t.Fatalf("negotiated ServerVersion = %d, want %d", got, sv)
			}

			ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelReq()

			// One-shot time sanity: exactly one time request per
			// connection. The gateway silently drops time requests fired
			// within a few seconds of the last answered one (live-attested
			// 2026-07-04, any sv), so below 197 CurrentTime is the probe and
			// at 197+ the CurrentTimeMillis call at the end is.
			if sv < 197 {
				if _, err := client.CurrentTime(ctx); err != nil {
					t.Fatalf("CurrentTime() at sv%d error = %v", sv, err)
				}
			}

			// Contract details: prefix decode crosses the lastTradeDate gate.
			details, err := client.Contracts().Details(ctx, aaplContract)
			if err != nil {
				t.Fatalf("ContractDetails() at sv%d error = %v", sv, err)
			}
			if len(details) == 0 || details[0].Symbol != "AAPL" {
				t.Fatalf("ContractDetails() at sv%d = %+v, want AAPL", sv, details)
			}

			// A rejected request produces a real error frame, proving the
			// ErrMsg layout gate: below 194 the frame has a leading version
			// int and no trailing errorTime.
			_, err = client.Contracts().Details(ctx, ibkr.Contract{
				Symbol: "ZZZZZZZZ", SecType: ibkr.SecTypeStock,
				Exchange: "SMART", Currency: "USD",
			})
			var apiErr *ibkr.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("garbage ContractDetails() at sv%d error = %v, want *APIError", sv, err)
			}
			if apiErr.Code != 200 {
				t.Fatalf("garbage ContractDetails() at sv%d code = %d, want 200", sv, apiErr.Code)
			}

			// Historical bars: below 196 the frame inlines dataset dates.
			// The bar size varies per subtest so the engine's identical-
			// request spacing (15s) does not serialize the matrix.
			barCtx, cancelBars := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelBars()
			bars, err := client.History().Bars(barCtx, ibkr.HistoricalBarsRequest{
				Contract:   aaplContract,
				Duration:   ibkr.Days(1),
				BarSize:    barSizes[sv],
				WhatToShow: ibkr.ShowTrades,
				UseRTH:     true,
			})
			if err != nil {
				t.Fatalf("HistoricalBars() at sv%d error = %v", sv, err)
			}
			if len(bars) == 0 {
				t.Fatalf("HistoricalBars() at sv%d returned no bars", sv)
			}

			// Feature gate: CurrentTimeMillis exists only at 197+.
			_, err = client.CurrentTimeMillis(ctx)
			if sv < 197 {
				if !errors.Is(err, ibkr.ErrUnsupportedServerVersion) {
					t.Fatalf("CurrentTimeMillis() at sv%d error = %v, want ErrUnsupportedServerVersion", sv, err)
				}
			} else if err != nil {
				t.Fatalf("CurrentTimeMillis() at sv%d error = %v", sv, err)
			}
		})
	}
}

var barSizes = map[int]ibkr.BarSize{
	176: ibkr.Bar1Hour, 184: ibkr.Bar30Mins, 193: ibkr.Bar15Mins,
	195: ibkr.Bar10Mins, 199: ibkr.Bar5Mins,
}

func versionName(sv int) string {
	return "sv" + strconv.Itoa(sv)
}

// TestLiveServer202ZeroStrikeBoundary freezes the only named API 10.48.01
// boundary at server_version 202. No message family migrates at this version:
// a conId-only contract request stays raw-ID classic, while executions stay on
// the protobuf family introduced at 201. The exact-202 Gateway must resolve the
// classic request and preserve an explicitly present zero strike on protobuf
// execution contracts without losing the conId.
func TestLiveServer202ZeroStrikeBoundary(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(202)
	defer restore()

	client, _, cancel := ibkrlive.DialTradingContext(t, 15*time.Second)
	defer cancel()
	defer client.Close()

	if got := client.Session().ServerVersion; got != 202 {
		t.Fatalf("negotiated ServerVersion = %d, want 202", got)
	}

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()
	details, err := client.Contracts().Details(ctx, ibkr.Contract{ConID: 265598})
	if err != nil {
		t.Fatalf("conId-only ContractDetails() at sv202 error = %v", err)
	}
	if len(details) != 1 || details[0].ConID != 265598 || details[0].Strike == nil || !details[0].Strike.IsZero() {
		t.Fatalf("conId-only ContractDetails() at sv202 = %+v, want AAPL conId 265598 with zero strike", details)
	}

	updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if err != nil {
		t.Fatalf("Executions() at sv202 error = %v", err)
	}
	for _, update := range updates {
		if update.Execution != nil && update.Execution.Contract.ConID != 0 &&
			update.Execution.Contract.Strike != nil && update.Execution.Contract.Strike.IsZero() {
			t.Logf("sv202 execution attested: exec_id=%s con_id=%d strike=%s",
				update.Execution.ExecID, update.Execution.Contract.ConID, update.Execution.Contract.Strike)
			return
		}
	}
	t.Log("sv202 execution query returned no present-zero-strike execution; deterministic live replay covers the non-empty callback")
}

// TestLiveServer203OrderProtobufBoundary exercises every outbound family that
// migrates at 203: placeOrder, cancelOrder, and reqGlobalCancel. The order is a
// one-share AAPL limit far below market and cleanup issues global cancel even
// if the targeted cancel path fails. Exact-203 open-order and order-status
// callbacks prove the paired inbound protobuf decoders.
func TestLiveServer203OrderProtobufBoundary(t *testing.T) {
	ibkrlive.RequireTrading(t)
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(203)
	defer restore()

	client, _, cancel := ibkrlive.DialTradingContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()
	cleaned := false
	defer func() {
		if cleaned {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := client.Orders().CancelAll(cleanupCtx); err != nil {
			t.Logf("sv203 cleanup global cancel: %v", err)
		}
	}()

	if got := client.Session().ServerVersion; got != 203 {
		t.Fatalf("negotiated ServerVersion = %d, want 203", got)
	}
	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: aaplContract,
		Order: ibkr.Order{
			Action: ibkr.ActionBuy, OrderType: ibkr.OrderTypeLimit,
			Quantity: decimal.NewFromInt(1), LmtPrice: decimal.NewFromInt(50), TIF: ibkr.TIFDay,
		},
	})
	if err != nil {
		t.Fatalf("Place at sv203: %v", err)
	}

	var sawOpen, sawStatus bool
	for !sawOpen || !sawStatus {
		select {
		case event := <-handle.Events():
			if event.OpenOrder != nil {
				sawOpen = true
				if event.OpenOrder.Contract.Symbol != "AAPL" || event.OpenOrder.OrderType != ibkr.OrderTypeLimit {
					t.Fatalf("sv203 open order = %+v", event.OpenOrder)
				}
			}
			if event.Status != nil {
				sawStatus = true
			}
		case <-ctx.Done():
			t.Fatalf("waiting for sv203 open/status callbacks: %v", ctx.Err())
		}
	}
	if err := handle.Cancel(ctx); err != nil {
		t.Fatalf("Cancel at sv203: %v", err)
	}
	select {
	case <-handle.Done():
	case <-ctx.Done():
		t.Fatalf("waiting for sv203 targeted cancellation: %v", ctx.Err())
	}
	if err := client.Orders().CancelAll(ctx); err != nil {
		t.Fatalf("global cancel at sv203: %v", err)
	}
	// CancelAll has no completion callback. A following round trip proves the
	// writer flushed the six-byte global-cancel protobuf frame before teardown.
	if _, err := client.CurrentTime(ctx); err != nil {
		t.Fatalf("round trip after global cancel at sv203: %v", err)
	}
	cleaned = true
}

// TestLiveServer204CompletedOrderBoundary verifies the exact 204 migration
// through the public API without placing or cancelling an order. It also
// asserts the local paper account is left with no working order and no AAPL
// position after the earlier guarded capture campaign.
func TestLiveServer204CompletedOrderBoundary(t *testing.T) {
	ibkrlive.RequireTrading(t)
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(204)
	defer restore()

	client, _, cancel := ibkrlive.DialTradingContext(t, 30*time.Second, ibkr.WithClientID(0))
	defer cancel()
	defer client.Close()

	if got := client.Session().ServerVersion; got != 204 {
		t.Fatalf("negotiated ServerVersion = %d, want 204", got)
	}
	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	open, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		t.Fatalf("Open(all) at sv204: %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("Open(all) at sv204 returned %d working orders; account must be clean", len(open))
	}

	completed, err := client.Orders().Completed(ctx, false)
	if err != nil {
		t.Fatalf("Completed(false) at sv204: %v", err)
	}
	for i, order := range completed {
		if order.Order.OrderID == nil || order.Order.ClientID == nil || order.Order.ParentID == nil {
			t.Fatalf("completed[%d] lost protobuf identities: %+v", i, order.Order)
		}
	}

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		t.Fatalf("Positions() after sv204 queries: %v", err)
	}
	for _, position := range positions {
		if position.Contract.Symbol == "AAPL" && !position.Position.IsZero() {
			t.Fatalf("AAPL position after sv204 queries = %s; account must remain flat", position.Position)
		}
	}
}

// TestLiveServer205ContractDataBoundary verifies the exact 205 migration
// through the public API. Every request is read-only, so the same test can run
// against either Gateway role by selecting IBKR_LIVE_ADDR.
func TestLiveServer205ContractDataBoundary(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(205)
	defer restore()

	client, _, cancel := ibkrlive.DialContext(t, 30*time.Second)
	defer cancel()
	defer client.Close()

	if got := client.Session().ServerVersion; got != 205 {
		t.Fatalf("negotiated ServerVersion = %d, want 205", got)
	}
	ctx, cancelReq := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelReq()

	stock, err := client.Contracts().Qualify(ctx, ibkr.Contract{ConID: 265598, Exchange: "SMART"})
	if err != nil {
		t.Fatalf("AAPL stock at sv205: %v", err)
	}
	if stock.SecType != ibkr.SecTypeStock || stock.MinAlgoSize == nil || !stock.MinAlgoSize.IsZero() ||
		stock.LastPricePrecision == nil || !stock.LastPricePrecision.Equal(decimal.RequireFromString("0.000001")) ||
		stock.LastSizePrecision == nil || !stock.LastSizePrecision.Equal(decimal.RequireFromString("0.000001")) {
		t.Fatalf("AAPL stock at sv205 = %+v", stock)
	}

	bond, err := client.Contracts().Qualify(ctx, ibkr.Contract{ConID: 127128131, Exchange: "SMART"})
	if err != nil {
		t.Fatalf("Apple bond at sv205: %v", err)
	}
	if bond.SecType != ibkr.SecTypeBond || bond.Bond == nil || bond.Bond.CUSIP != "IBCID127128131" ||
		bond.MinAlgoSize == nil || !bond.MinAlgoSize.IsZero() {
		t.Fatalf("Apple bond at sv205 = %+v", bond)
	}

	fund, err := client.Contracts().Qualify(ctx, ibkr.Contract{ConID: 57041934, Exchange: "FUNDSERV"})
	if err != nil {
		t.Fatalf("fund at sv205: %v", err)
	}
	if fund.SecType != ibkr.SecTypeFund || fund.Fund == nil || fund.Fund.Family != "American Century" ||
		fund.MinAlgoSize == nil || !fund.MinAlgoSize.IsZero() {
		t.Fatalf("fund at sv205 = %+v", fund)
	}

	option, err := client.Contracts().Qualify(ctx, ibkr.Contract{ConID: 728937835, Exchange: "SMART"})
	if err != nil {
		t.Fatalf("option at sv205: %v", err)
	}
	if option.SecType != ibkr.SecTypeOption || option.Expiry != "20270115" || option.LastTradeDate != "20270115" ||
		option.MinAlgoSize == nil || !option.MinAlgoSize.IsZero() {
		t.Fatalf("option at sv205 = %+v", option)
	}

	ineligible, err := client.Contracts().Qualify(ctx, ibkr.Contract{ConID: 236491195, Exchange: "SMART"})
	if err != nil {
		t.Fatalf("ineligible bond at sv205: %v", err)
	}
	if len(ineligible.IneligibilityReasons) != 3 || ineligible.IneligibilityReasons[0].ID != "i155" ||
		ineligible.IneligibilityReasons[1].ID != "i156" || ineligible.IneligibilityReasons[2].ID != "i30" {
		t.Fatalf("ineligible bond reasons at sv205 = %+v", ineligible.IneligibilityReasons)
	}
}

// TestLiveCanonicalContractRequestMatrix crosses the classic/protobuf request
// boundaries with the public Contract selectors and composition fields. All
// operations are read-only: external-ID and expired-future lookups plus a BAG
// quote snapshot assembled from dynamically qualified AAPL option legs.
func TestLiveCanonicalContractRequestMatrix(t *testing.T) {
	for _, sv := range []int{200, 205, 206} {
		t.Run(versionName(sv), func(t *testing.T) {
			restore := ibkr.SetAdvertisedServerVersionMaxForTest(sv)
			defer restore()

			client, ctx, cancel := ibkrlive.DialContext(t, 60*time.Second)
			defer cancel()
			defer client.Close()
			if got := client.Session().ServerVersion; got != sv {
				t.Fatalf("negotiated ServerVersion = %d, want %d", got, sv)
			}

			byISIN, err := client.Contracts().Details(ctx, ibkr.Contract{
				Exchange:   "SMART",
				SecurityID: ibkr.SecurityID{Type: ibkr.SecurityIDISIN, Value: "US0378331005"},
			})
			if err != nil {
				t.Fatalf("ISIN ContractDetails at sv%d: %v", sv, err)
			}
			foundAAPL := false
			for _, detail := range byISIN {
				if detail.ConID == 265598 && detail.Symbol == "AAPL" && detail.Currency == "USD" {
					foundAAPL = true
					break
				}
			}
			if !foundAAPL {
				t.Fatalf("ISIN ContractDetails at sv%d returned %d contracts without AAPL conId 265598", sv, len(byISIN))
			}

			expired, err := client.Contracts().Details(ctx, ibkr.Contract{
				Symbol: "MES", SecType: ibkr.SecTypeFuture, Expiry: "202606",
				Exchange: "CME", Currency: "USD", IncludeExpired: true,
			})
			if err != nil {
				t.Fatalf("expired MES ContractDetails at sv%d: %v", sv, err)
			}
			foundExpiredMES := false
			for _, detail := range expired {
				if detail.SecType == ibkr.SecTypeFuture && len(detail.Expiry) >= 6 && detail.Expiry[:6] == "202606" {
					foundExpiredMES = true
					break
				}
			}
			if !foundExpiredMES {
				t.Fatalf("expired MES ContractDetails at sv%d returned %d contracts without a 202606 future", sv, len(expired))
			}

			anchor := liveAnchorPrice(t, ctx, client, aaplContract, decimal.RequireFromString("300"))
			lower, upper := liveQualifyVerticalLegs(t, ctx, client, anchor)
			bag := ibkr.Contract{
				Symbol: "AAPL", SecType: ibkr.SecTypeCombo, Exchange: "SMART", Currency: "USD",
				ComboLegs: []ibkr.ComboLeg{
					{ConID: lower.ConID, Ratio: 1, Action: ibkr.ActionBuy, Exchange: "SMART"},
					{ConID: upper.ConID, Ratio: 1, Action: ibkr.ActionSell, Exchange: "SMART"},
				},
			}
			if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
				t.Fatalf("SetType(delayed) at sv%d: %v", sv, err)
			}
			if _, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: bag}); err != nil {
				t.Fatalf("BAG Quote at sv%d: %v", sv, err)
			}
		})
	}
}

// TestLiveServer206MarketDataBoundary validates the exact protobuf migration
// through the public quote API. The request is read-only and can run against
// either Gateway role selected by IBKR_LIVE_ADDR.
func TestLiveServer206MarketDataBoundary(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(206)
	defer restore()

	client, ctx, cancel := ibkrlive.DialContext(t, 20*time.Second)
	defer cancel()
	defer client.Close()
	if got := client.Session().ServerVersion; got != 206 {
		t.Fatalf("negotiated ServerVersion = %d, want 206", got)
	}
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		t.Fatalf("SetType(delayed): %v", err)
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock,
			Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD",
		},
		GenericTicks: []ibkr.GenericTick{"221"},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes(): %v", err)
	}
	defer sub.Close()

	for {
		select {
		case update := <-sub.Events():
			if update.Kind != ibkr.QuoteUpdateParameters {
				continue
			}
			parameters := update.Parameters
			if parameters == nil || parameters.SnapshotPermissions == nil ||
				parameters.LastPricePrecision == nil || parameters.LastSizePrecision == nil {
				t.Fatalf("server_version 206 parameters = %+v", parameters)
			}
			return
		case <-sub.Done():
			t.Fatalf("quote subscription closed before request parameters: %v", sub.Err())
		case <-ctx.Done():
			t.Fatalf("waiting for server_version 206 request parameters: %v", ctx.Err())
		}
	}
}

// TestLiveDownNegotiatedPreview validates the OpenOrder inbound gates with
// real down-negotiated echoes: the what-if open_order reply crosses the
// FULL_ORDER_PREVIEW block gate (195) and the width-gated tail
// (183..199), the two positional decodes the deterministic boundary tests
// can only mirror. A what-if computes margin and leaves nothing resting on
// the server. Requires the trading role; not parallel (advertised max is
// package state).
func TestLiveDownNegotiatedPreview(t *testing.T) {
	ibkrlive.RequireTrading(t)
	for _, sv := range []int{193, 195, 198, 200} {
		t.Run(versionName(sv), func(t *testing.T) {
			restore := ibkr.SetAdvertisedServerVersionMaxForTest(sv)
			defer restore()

			client, _, cancel := ibkrlive.DialTradingContext(t, 15*time.Second)
			defer cancel()
			defer client.Close()

			if got := client.Session().ServerVersion; got != sv {
				t.Fatalf("negotiated ServerVersion = %d, want %d", got, sv)
			}
			ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelReq()

			state, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{
				Contract: aaplContract,
				Order: ibkr.Order{
					Action:    ibkr.ActionBuy,
					OrderType: ibkr.OrderTypeMarket,
					Quantity:  decimal.NewFromInt(100),
				},
			})
			if err != nil {
				t.Fatalf("Preview at sv%d: %v", sv, err)
			}
			// The core margin fields predate every gate in range and must
			// decode at all four versions; a desync in the preview block or
			// tail would corrupt or truncate them.
			if state.InitMarginAfter.IsZero() && state.MaintMarginAfter.IsZero() {
				t.Fatalf("Preview at sv%d returned no margin data: %+v", sv, state)
			}
			t.Logf("sv%d: initAfter=%s maintAfter=%s commission=%s warning=%q",
				sv, state.InitMarginAfter, state.MaintMarginAfter, state.Commission, state.WarningText)
		})
	}
}
