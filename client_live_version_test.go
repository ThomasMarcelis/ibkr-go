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
	if len(details) != 1 || details[0].ConID != 265598 || !details[0].Strike.IsZero() {
		t.Fatalf("conId-only ContractDetails() at sv202 = %+v, want AAPL conId 265598 with zero strike", details)
	}

	updates, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
	if err != nil {
		t.Fatalf("Executions() at sv202 error = %v", err)
	}
	for _, update := range updates {
		if update.Execution != nil && update.Execution.Contract.ConID != 0 && update.Execution.Contract.Strike.IsZero() {
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
