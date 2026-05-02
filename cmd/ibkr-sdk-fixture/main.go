package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter/native"
	"github.com/shopspring/decimal"
)

const (
	recorderQueueSize = 1024
	redactedAccount   = "DU_REDACTED"
	redactedArticleID = "REDACTED_ARTICLE_ID"
	redactedHeadline  = "REDACTED_HEADLINE"
	redactedArticle   = "REDACTED_ARTICLE_TEXT"
	redactedContract  = "REDACTED_CONTRACT"
	redactedModelCode = "REDACTED_MODEL"
	redactedValue     = "REDACTED_VALUE"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Gateway/TWS host")
	port := flag.Int("port", 4002, "Gateway/TWS port")
	clientID := flag.Int("client-id", 9101, "Gateway/TWS client ID")
	scenario := flag.String("scenario", "read_only_smoke", "fixture scenario: read_only_smoke, current_time_millis, account_summary_snapshot, account_streams_snapshot, family_codes_snapshot, bond_contract_details_snapshot, quote_stream_short, real_time_bars_short, tick_by_tick_midpoint_short, market_depth_smart_short, historical_bars_short, historical_bars_keepup_short, historical_schedule_short, historical_ticks_midpoint_short, historical_ticks_bidask_short, historical_ticks_trades_short, fundamental_data_snapshot, scanner_parameters_snapshot, scanner_subscription_short, display_group_subscription_short, news_invalid_requests, news_article_snapshot, news_bulletins_short, option_calculations_short, option_calculations_qualified, executions_empty_filter, completed_orders_snapshot, paper_order_place_cancel, paper_order_modify_cancel, paper_open_orders_place_cancel, or paper_order_reject_invalid_type")
	outPath := flag.String("out", "", "write fixture JSON to this path instead of stdout")
	timeout := flag.Duration("timeout", 30*time.Second, "overall capture timeout")
	flag.Parse()

	if err := run(*host, *port, *clientID, *scenario, *outPath, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(host string, port int, clientID int, scenario string, outPath string, timeout time.Duration) error {
	info, err := native.BuildInfo()
	if err != nil {
		return fmt.Errorf("build info: %w", err)
	}
	adapter, err := native.New(recorderQueueSize)
	if err != nil {
		return fmt.Errorf("new adapter: %w", err)
	}
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := adapter.Connect(ctx, sdkadapter.ConnectRequest{
		Host:      host,
		Port:      port,
		ClientID:  clientID,
		Timeout:   10 * time.Second,
		QueueSize: recorderQueueSize,
	}); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	rec := &recorder{adapter: adapter}
	if err := rec.drainUntil(ctx, "bootstrap", func(events []sdkadapter.Event) bool {
		return hasKind(events, sdkadapter.EventConnectionMetadata) &&
			hasKind(events, sdkadapter.EventManagedAccounts) &&
			hasKind(events, sdkadapter.EventNextValidID)
	}); err != nil {
		return err
	}

	switch scenario {
	case "read_only_smoke":
		if err := rec.captureReadOnlySmoke(ctx); err != nil {
			return err
		}
	case "current_time_millis":
		if err := rec.submitAndWait(ctx, sdkadapter.Command{Kind: sdkadapter.CommandCurrentTimeMillis}, "current time millis", func(event sdkadapter.Event) bool {
			return event.Kind == sdkadapter.EventCurrentTimeMillis
		}); err != nil {
			return err
		}
	case "account_summary_snapshot":
		if err := rec.captureAccountSummarySnapshot(ctx); err != nil {
			return err
		}
	case "account_streams_snapshot":
		if err := rec.captureAccountStreamsSnapshot(ctx); err != nil {
			return err
		}
	case "family_codes_snapshot":
		if err := rec.captureFamilyCodesSnapshot(ctx); err != nil {
			return err
		}
	case "bond_contract_details_snapshot":
		if err := rec.captureBondContractDetailsSnapshot(ctx); err != nil {
			return err
		}
	case "quote_stream_short":
		if err := rec.captureQuoteStreamShort(ctx); err != nil {
			return err
		}
	case "real_time_bars_short":
		if err := rec.captureRealTimeBarsShort(ctx); err != nil {
			return err
		}
	case "tick_by_tick_midpoint_short":
		if err := rec.captureTickByTickMidpointShort(ctx); err != nil {
			return err
		}
	case "market_depth_smart_short":
		if err := rec.captureMarketDepthSmartShort(ctx); err != nil {
			return err
		}
	case "historical_bars_short":
		if err := rec.captureHistoricalBarsShort(ctx); err != nil {
			return err
		}
	case "historical_bars_keepup_short":
		if err := rec.captureHistoricalBarsKeepUpShort(ctx); err != nil {
			return err
		}
	case "historical_schedule_short":
		if err := rec.captureHistoricalScheduleShort(ctx); err != nil {
			return err
		}
	case "historical_ticks_midpoint_short":
		if err := rec.captureHistoricalTicksMidpointShort(ctx); err != nil {
			return err
		}
	case "historical_ticks_bidask_short":
		if err := rec.captureHistoricalTicksBidAskShort(ctx); err != nil {
			return err
		}
	case "historical_ticks_trades_short":
		if err := rec.captureHistoricalTicksTradesShort(ctx); err != nil {
			return err
		}
	case "fundamental_data_snapshot":
		if err := rec.captureFundamentalDataSnapshot(ctx); err != nil {
			return err
		}
	case "scanner_parameters_snapshot":
		if err := rec.captureScannerParametersSnapshot(ctx); err != nil {
			return err
		}
	case "scanner_subscription_short":
		if err := rec.captureScannerSubscriptionShort(ctx); err != nil {
			return err
		}
	case "display_group_subscription_short":
		if err := rec.captureDisplayGroupSubscriptionShort(ctx); err != nil {
			return err
		}
	case "news_invalid_requests":
		if err := rec.captureNewsInvalidRequests(ctx); err != nil {
			return err
		}
	case "news_article_snapshot":
		if err := rec.captureNewsArticleSnapshot(ctx); err != nil {
			return err
		}
	case "news_bulletins_short":
		if err := rec.captureNewsBulletinsShort(ctx); err != nil {
			return err
		}
	case "option_calculations_short":
		if err := rec.captureOptionCalculationsShort(ctx); err != nil {
			return err
		}
	case "option_calculations_qualified":
		if err := rec.captureOptionCalculationsQualified(ctx); err != nil {
			return err
		}
	case "executions_empty_filter":
		if err := rec.captureExecutionsEmptyFilter(ctx); err != nil {
			return err
		}
	case "completed_orders_snapshot":
		if err := rec.captureCompletedOrdersSnapshot(ctx); err != nil {
			return err
		}
	case "paper_order_place_cancel":
		if err := rec.capturePaperOrderPlaceCancel(ctx); err != nil {
			return err
		}
	case "paper_order_modify_cancel":
		if err := rec.capturePaperOrderModifyCancel(ctx); err != nil {
			return err
		}
	case "paper_open_orders_place_cancel":
		if err := rec.capturePaperOpenOrdersPlaceCancel(ctx); err != nil {
			return err
		}
	case "paper_order_reject_invalid_type":
		if err := rec.capturePaperOrderRejectInvalidType(ctx); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown scenario %q", scenario)
	}

	rawEvents := sdkadapter.CloneEvents(rec.events)
	sourceHash, err := eventHash(rawEvents)
	if err != nil {
		return err
	}

	fixture := sdkadapter.Fixture{
		Metadata: sdkadapter.FixtureMetadata{
			SDKVersion:     info.SDKAPIVersion,
			ServerVersion:  adapter.ServerVersion(),
			CapturedAt:     time.Now().UTC().Format(time.RFC3339),
			Scenario:       scenario,
			RedactionNotes: redactionNotes(scenario),
			SourceSHA256:   sourceHash,
		},
		Events: redactPrivateValues(redactAccountIdentifiers(rawEvents), scenario),
	}

	if outPath == "" {
		return sdkadapter.EncodeFixture(os.Stdout, fixture)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create fixture: %w", err)
	}
	defer f.Close()
	if err := sdkadapter.EncodeFixture(f, fixture); err != nil {
		return fmt.Errorf("encode fixture: %w", err)
	}
	return nil
}

func (r *recorder) capturePaperOrderPlaceCancel(ctx context.Context) error {
	account, err := paperAccountFromEvents(r.events)
	if err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return fmt.Errorf("market data type submit: %w", err)
	}
	const quoteReqID = 201
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandQuote,
		Quote: sdkadapter.QuoteCommand{
			ReqID:    quoteReqID,
			Contract: aaplStockContract(),
			Snapshot: true,
		},
	}, "paper order quote snapshot", reqKind(quoteReqID, sdkadapter.EventTickSnapshotEnd)); err != nil {
		return err
	}

	limitPrice, err := paperOrderLimitPrice(r.events, quoteReqID)
	if err != nil {
		return err
	}
	orderID, err := nextValidOrderID(r.events)
	if err != nil {
		return err
	}

	cancelled := false
	cancel := sdkadapter.Command{
		Kind:        sdkadapter.CommandCancelOrder,
		CancelOrder: sdkadapter.CancelOrderCommand{OrderID: orderID},
	}
	defer func() {
		if !cancelled {
			cancelCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
			_ = r.submit(cancelCtx, cancel)
			cancelCleanup()
		}
	}()

	if err := r.submit(ctx, sdkadapter.Command{
		Kind:       sdkadapter.CommandPlaceOrder,
		PlaceOrder: paperOrderRequest(orderID, account, limitPrice, "1", "ibkr-go-sdk-fixture-"),
	}); err != nil {
		return fmt.Errorf("paper order submit: %w", err)
	}

	accepted := false
	rejected := sdkadapter.Event{}
	if err := r.drainUntil(ctx, "paper order accepted", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if paperOrderAccepted(event, orderID) {
				accepted = true
				return true
			}
			if paperOrderRejected(event, orderID) {
				rejected = event
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("paper order rejected before acceptance: code=%d message=%s", rejected.APIError.Code, rejected.APIError.Message)
	}

	if err := r.submit(ctx, cancel); err != nil {
		return fmt.Errorf("paper order cancel submit: %w", err)
	}
	filled := false
	if err := r.drainUntil(ctx, "paper order cancelled", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.Kind != sdkadapter.EventOrderStatus || event.OrderStatus.OrderID != orderID {
				continue
			}
			switch event.OrderStatus.Status {
			case "Cancelled", "ApiCancelled":
				cancelled = true
				return true
			case "Filled":
				filled = true
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if filled {
		return fmt.Errorf("paper safety order filled before cancellation completed")
	}
	return nil
}

func (r *recorder) captureQuoteStreamShort(ctx context.Context) error {
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return fmt.Errorf("market data type submit: %w", err)
	}
	const reqID = 301
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandQuote,
		Quote: sdkadapter.QuoteCommand{
			ReqID:    reqID,
			Contract: aaplStockContract(),
			Snapshot: false,
		},
	}); err != nil {
		return fmt.Errorf("quote stream submit: %w", err)
	}
	if err := r.drainUntil(ctx, "quote stream first tick", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID == reqID && quoteStreamDataEvent(event.Kind) {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:        sdkadapter.CommandCancelQuote,
		CancelQuote: sdkadapter.CancelQuoteCommand{ReqID: reqID},
	}); err != nil {
		return fmt.Errorf("quote stream cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureAccountSummarySnapshot(ctx context.Context) error {
	const reqID = 121
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandAccountSummary,
		AccountSummary: sdkadapter.AccountSummaryCommand{
			ReqID: reqID,
			Group: "All",
			Tags:  []string{"NetLiquidation", "BuyingPower"},
		},
	}, "account summary", reqKind(reqID, sdkadapter.EventAccountSummaryEnd)); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureAccountStreamsSnapshot(ctx context.Context) error {
	account, err := managedAccountFromEvents(r.events)
	if err != nil {
		return err
	}

	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandAccountUpdates,
		AccountUpdates: sdkadapter.AccountUpdatesCommand{
			Subscribe: true,
			Account:   account,
		},
	}, "account updates", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventAccountDownloadEnd
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandAccountUpdates,
		AccountUpdates: sdkadapter.AccountUpdatesCommand{
			Subscribe: false,
			Account:   account,
		},
	}); err != nil {
		return fmt.Errorf("account updates unsubscribe submit: %w", err)
	}

	const updatesMultiReqID = 811
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandAccountUpdatesMulti,
		AccountUpdatesMulti: sdkadapter.AccountUpdatesMultiCommand{
			ReqID:   updatesMultiReqID,
			Account: account,
		},
	}, "account updates multi", reqKind(updatesMultiReqID, sdkadapter.EventAccountUpdateMultiEnd)); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                      sdkadapter.CommandCancelAccountUpdatesMulti,
		CancelAccountUpdatesMulti: sdkadapter.CancelAccountUpdatesMultiCommand{ReqID: updatesMultiReqID},
	}); err != nil {
		return fmt.Errorf("account updates multi cancel submit: %w", err)
	}

	if err := r.submitAndWait(ctx, sdkadapter.Command{Kind: sdkadapter.CommandPositions}, "positions", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventPositionEnd
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{Kind: sdkadapter.CommandCancelPositions}); err != nil {
		return fmt.Errorf("positions cancel submit: %w", err)
	}

	const positionsMultiReqID = 812
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandPositionsMulti,
		PositionsMulti: sdkadapter.PositionsMultiCommand{
			ReqID:   positionsMultiReqID,
			Account: account,
		},
	}, "positions multi", reqKind(positionsMultiReqID, sdkadapter.EventPositionMultiEnd)); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                 sdkadapter.CommandCancelPositionsMulti,
		CancelPositionsMulti: sdkadapter.CancelPositionsMultiCommand{ReqID: positionsMultiReqID},
	}); err != nil {
		return fmt.Errorf("positions multi cancel submit: %w", err)
	}

	const pnlReqID = 813
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandPnL,
		PnL: sdkadapter.PnLCommand{
			ReqID:   pnlReqID,
			Account: account,
		},
	}, "pnl", reqKind(pnlReqID, sdkadapter.EventPnL)); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:      sdkadapter.CommandCancelPnL,
		CancelPnL: sdkadapter.CancelPnLCommand{ReqID: pnlReqID},
	}); err != nil {
		return fmt.Errorf("pnl cancel submit: %w", err)
	}

	conID, err := firstPositionConID(r.events)
	if err != nil {
		return err
	}
	const pnlSingleReqID = 814
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandPnLSingle,
		PnLSingle: sdkadapter.PnLSingleCommand{
			ReqID:   pnlSingleReqID,
			Account: account,
			ConID:   conID,
		},
	}, "pnl single", reqKind(pnlSingleReqID, sdkadapter.EventPnLSingle)); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:            sdkadapter.CommandCancelPnLSingle,
		CancelPnLSingle: sdkadapter.CancelPnLSingleCommand{ReqID: pnlSingleReqID},
	}); err != nil {
		return fmt.Errorf("pnl single cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureFamilyCodesSnapshot(ctx context.Context) error {
	if err := r.submitAndWait(ctx, sdkadapter.Command{Kind: sdkadapter.CommandFamilyCodes}, "family codes", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventFamilyCodes
	}); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureBondContractDetailsSnapshot(ctx context.Context) error {
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandContractDetails,
		ContractDetails: sdkadapter.ContractDetailsCommand{
			ReqID:    1601,
			Contract: officialSampleBondWithCUSIPContract(),
		},
	}, "bond contract details", reqKind(1601, sdkadapter.EventContractDetailsEnd)); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureRealTimeBarsShort(ctx context.Context) error {
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return fmt.Errorf("market data type submit: %w", err)
	}
	const reqID = 304
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandRealTimeBars,
		RealTimeBars: sdkadapter.RealTimeBarsCommand{
			ReqID:      reqID,
			Contract:   aaplStockContract(),
			WhatToShow: "TRADES",
			UseRTH:     true,
		},
	}); err != nil {
		return fmt.Errorf("real-time bars submit: %w", err)
	}
	if err := r.drainUntil(ctx, "real-time bars", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID == reqID && (event.Kind == sdkadapter.EventRealTimeBar || event.Kind == sdkadapter.EventAPIError) {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:               sdkadapter.CommandCancelRealTimeBars,
		CancelRealTimeBars: sdkadapter.CancelRealTimeBarsCommand{ReqID: reqID},
	}); err != nil {
		return fmt.Errorf("real-time bars cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureHistoricalBarsShort(ctx context.Context) error {
	const reqID = 401
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandHistoricalData,
		HistoricalData: sdkadapter.HistoricalDataCommand{
			ReqID:      reqID,
			Contract:   aaplStockContract(),
			Duration:   "1 D",
			BarSize:    "1 hour",
			WhatToShow: "TRADES",
			UseRTH:     true,
		},
	}, "historical bars", reqKind(reqID, sdkadapter.EventHistoricalDataEnd)); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureHistoricalBarsKeepUpShort(ctx context.Context) error {
	const reqID = 406
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandHistoricalData,
		HistoricalData: sdkadapter.HistoricalDataCommand{
			ReqID:        reqID,
			Contract:     aaplStockContract(),
			Duration:     "1 D",
			BarSize:      "1 hour",
			WhatToShow:   "TRADES",
			UseRTH:       true,
			KeepUpToDate: true,
		},
	}); err != nil {
		return fmt.Errorf("historical keep-up bars submit: %w", err)
	}
	if err := r.drainUntil(ctx, "historical keep-up bars", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID == reqID && (event.Kind == sdkadapter.EventHistoricalDataEnd || event.Kind == sdkadapter.EventAPIError) {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                 sdkadapter.CommandCancelHistoricalData,
		CancelHistoricalData: sdkadapter.CancelHistoricalDataCommand{ReqID: reqID},
	}); err != nil {
		return fmt.Errorf("historical keep-up bars cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureHistoricalScheduleShort(ctx context.Context) error {
	const reqID = 402
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandHistoricalData,
		HistoricalData: sdkadapter.HistoricalDataCommand{
			ReqID:      reqID,
			Contract:   aaplStockContract(),
			Duration:   "1 M",
			BarSize:    "1 day",
			WhatToShow: "SCHEDULE",
			UseRTH:     true,
		},
	}, "historical schedule", reqKind(reqID, sdkadapter.EventHistoricalSchedule)); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureHistoricalTicksMidpointShort(ctx context.Context) error {
	return r.captureHistoricalTicksShort(ctx, 403, "MIDPOINT", false, "historical midpoint ticks", sdkadapter.EventHistoricalTicks)
}

func (r *recorder) captureHistoricalTicksBidAskShort(ctx context.Context) error {
	return r.captureHistoricalTicksShort(ctx, 404, "BID_ASK", true, "historical bid/ask ticks", sdkadapter.EventHistoricalTicksBidAsk)
}

func (r *recorder) captureHistoricalTicksTradesShort(ctx context.Context) error {
	return r.captureHistoricalTicksShort(ctx, 405, "TRADES", true, "historical trade ticks", sdkadapter.EventHistoricalTicksLast)
}

func (r *recorder) captureHistoricalTicksShort(ctx context.Context, reqID int, whatToShow string, ignoreSize bool, label string, eventKind sdkadapter.EventKind) error {
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandHistoricalTicks,
		HistoricalTicks: sdkadapter.HistoricalTicksCommand{
			ReqID:         reqID,
			Contract:      aaplStockContract(),
			EndDateTime:   "20260501 16:00:00 US/Eastern",
			NumberOfTicks: 10,
			WhatToShow:    whatToShow,
			UseRTH:        true,
			IgnoreSize:    ignoreSize,
		},
	}, label, reqKind(reqID, eventKind)); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureTickByTickMidpointShort(ctx context.Context) error {
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return fmt.Errorf("market data type submit: %w", err)
	}
	const reqID = 302
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandTickByTick,
		TickByTick: sdkadapter.TickByTickCommand{
			ReqID:    reqID,
			Contract: aaplStockContract(),
			TickType: "MidPoint",
		},
	}); err != nil {
		return fmt.Errorf("tick-by-tick submit: %w", err)
	}
	if err := r.drainUntil(ctx, "tick-by-tick midpoint", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID != reqID {
				continue
			}
			if event.Kind == sdkadapter.EventTickByTick || event.Kind == sdkadapter.EventAPIError {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:             sdkadapter.CommandCancelTickByTick,
		CancelTickByTick: sdkadapter.CancelTickByTickCommand{ReqID: reqID},
	}); err != nil {
		return fmt.Errorf("tick-by-tick cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureMarketDepthSmartShort(ctx context.Context) error {
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return fmt.Errorf("market data type submit: %w", err)
	}
	const reqID = 303
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandMarketDepth,
		MarketDepth: sdkadapter.MarketDepthCommand{
			ReqID:        reqID,
			Contract:     aaplStockContract(),
			NumRows:      5,
			IsSmartDepth: true,
		},
	}); err != nil {
		return fmt.Errorf("market depth submit: %w", err)
	}
	if err := r.drainUntil(ctx, "market depth smart", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID != reqID {
				continue
			}
			switch event.Kind {
			case sdkadapter.EventMarketDepth, sdkadapter.EventMarketDepthL2, sdkadapter.EventAPIError:
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:              sdkadapter.CommandCancelMarketDepth,
		CancelMarketDepth: sdkadapter.CancelMarketDepthCommand{ReqID: reqID, IsSmartDepth: true},
	}); err != nil {
		return fmt.Errorf("market depth cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureFundamentalDataSnapshot(ctx context.Context) error {
	const reqID = 501
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandFundamentalData,
		FundamentalData: sdkadapter.FundamentalDataCommand{
			ReqID:      reqID,
			Contract:   aaplStockContract(),
			ReportType: "ReportSnapshot",
		},
	}, "fundamental data snapshot", func(event sdkadapter.Event) bool {
		return event.ReqID == reqID && (event.Kind == sdkadapter.EventFundamentalData || event.Kind == sdkadapter.EventAPIError)
	}); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureScannerParametersSnapshot(ctx context.Context) error {
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandScannerParameters,
	}, "scanner parameters", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventScannerParameters
	}); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureScannerSubscriptionShort(ctx context.Context) error {
	const reqID = 701
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandScannerSubscription,
		ScannerSubscription: sdkadapter.ScannerSubscriptionCommand{
			ReqID:        reqID,
			NumberOfRows: 5,
			Instrument:   "STK",
			LocationCode: "STK.US.MAJOR",
			ScanCode:     "TOP_PERC_GAIN",
		},
	}); err != nil {
		return fmt.Errorf("scanner subscription submit: %w", err)
	}
	if err := r.drainUntil(ctx, "scanner subscription", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID == reqID && (event.Kind == sdkadapter.EventScannerData || event.Kind == sdkadapter.EventAPIError) {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                      sdkadapter.CommandCancelScannerSubscription,
		CancelScannerSubscription: sdkadapter.CancelScannerSubscriptionCommand{ReqID: reqID},
	}); err != nil {
		return fmt.Errorf("scanner subscription cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureDisplayGroupSubscriptionShort(ctx context.Context) error {
	const queryReqID = 801
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind:               sdkadapter.CommandQueryDisplayGroups,
		QueryDisplayGroups: sdkadapter.QueryDisplayGroupsCommand{ReqID: queryReqID},
	}, "display groups", reqKind(queryReqID, sdkadapter.EventDisplayGroupList)); err != nil {
		return err
	}
	groupID, err := firstDisplayGroupID(r.events, queryReqID)
	if err != nil {
		return err
	}

	const reqID = 802
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandSubscribeToGroupEvents,
		SubscribeToGroupEvents: sdkadapter.SubscribeToGroupEventsCommand{
			ReqID:   reqID,
			GroupID: groupID,
		},
	}); err != nil {
		return fmt.Errorf("display group subscribe submit: %w", err)
	}
	if err := r.drainUntil(ctx, "display group subscription", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID == reqID && (event.Kind == sdkadapter.EventDisplayGroupUpdated || event.Kind == sdkadapter.EventAPIError) {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                       sdkadapter.CommandUnsubscribeFromGroupEvents,
		UnsubscribeFromGroupEvents: sdkadapter.UnsubscribeFromGroupEventsCommand{ReqID: reqID},
	}); err != nil {
		return fmt.Errorf("display group unsubscribe submit: %w", err)
	}
	return nil
}

func (r *recorder) captureNewsInvalidRequests(ctx context.Context) error {
	const historicalReqID = 901
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandHistoricalNews,
		HistoricalNews: sdkadapter.HistoricalNewsCommand{
			ReqID:         historicalReqID,
			ConID:         265598,
			ProviderCodes: "NO_SUCH_PROVIDER",
			StartDate:     "2026-05-01 00:00:00 UTC",
			EndDate:       "2026-05-02 00:00:00 UTC",
			TotalResults:  1,
		},
	}, "invalid historical news", func(event sdkadapter.Event) bool {
		return event.ReqID == historicalReqID &&
			(event.Kind == sdkadapter.EventHistoricalNewsEnd || event.Kind == sdkadapter.EventAPIError)
	}); err != nil {
		return err
	}

	const articleReqID = 902
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandNewsArticle,
		NewsArticle: sdkadapter.NewsArticleCommand{
			ReqID:        articleReqID,
			ProviderCode: "NO_SUCH_PROVIDER",
			ArticleID:    "NO_SUCH_PROVIDER$missing",
		},
	}, "invalid news article", func(event sdkadapter.Event) bool {
		return event.ReqID == articleReqID &&
			(event.Kind == sdkadapter.EventNewsArticle || event.Kind == sdkadapter.EventAPIError)
	}); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureNewsArticleSnapshot(ctx context.Context) error {
	const historicalReqID = 903
	end := time.Now().UTC()
	start := end.Add(-14 * 24 * time.Hour)

	var providerCode string
	var articleID string
	var historicalErr sdkadapter.Error
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandHistoricalNews,
		HistoricalNews: sdkadapter.HistoricalNewsCommand{
			ReqID:         historicalReqID,
			ConID:         265598,
			ProviderCodes: "BRFG+BRFUPDN+DJNL",
			StartDate:     formatFixtureHistoricalNewsTime(start),
			EndDate:       formatFixtureHistoricalNewsTime(end),
			TotalResults:  5,
		},
	}); err != nil {
		return fmt.Errorf("historical news submit: %w", err)
	}
	if err := r.drainUntil(ctx, "historical news for article", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID != historicalReqID {
				continue
			}
			switch event.Kind {
			case sdkadapter.EventHistoricalNews:
				if articleID == "" {
					providerCode = event.HistoricalNews.ProviderCode
					articleID = event.HistoricalNews.ArticleID
				}
			case sdkadapter.EventHistoricalNewsEnd:
				return true
			case sdkadapter.EventAPIError:
				historicalErr = event.APIError
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if historicalErr.Code != 0 {
		return fmt.Errorf("historical news error: code=%d message=%s", historicalErr.Code, historicalErr.Message)
	}
	if providerCode == "" || articleID == "" {
		return fmt.Errorf("historical news returned no article ID")
	}

	const articleReqID = 904
	var articleErr sdkadapter.Error
	var sawArticle bool
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandNewsArticle,
		NewsArticle: sdkadapter.NewsArticleCommand{
			ReqID:        articleReqID,
			ProviderCode: providerCode,
			ArticleID:    articleID,
		},
	}); err != nil {
		return fmt.Errorf("news article submit: %w", err)
	}
	if err := r.drainUntil(ctx, "news article", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID != articleReqID {
				continue
			}
			switch event.Kind {
			case sdkadapter.EventNewsArticle:
				sawArticle = true
				return true
			case sdkadapter.EventAPIError:
				articleErr = event.APIError
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if articleErr.Code != 0 {
		return fmt.Errorf("news article error: code=%d message=%s", articleErr.Code, articleErr.Message)
	}
	if !sawArticle {
		return fmt.Errorf("news article returned no article callback")
	}
	return nil
}

func (r *recorder) captureNewsBulletinsShort(ctx context.Context) error {
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:          sdkadapter.CommandNewsBulletins,
		NewsBulletins: sdkadapter.NewsBulletinsCommand{AllMessages: true},
	}); err != nil {
		return fmt.Errorf("news bulletins submit: %w", err)
	}
	bulletinCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := r.drainUntil(bulletinCtx, "news bulletins", func(events []sdkadapter.Event) bool {
		return hasKind(events, sdkadapter.EventNewsBulletin)
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{Kind: sdkadapter.CommandCancelNewsBulletins}); err != nil {
		return fmt.Errorf("news bulletins cancel submit: %w", err)
	}
	return nil
}

func formatFixtureHistoricalNewsTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05") + " UTC"
}

func (r *recorder) captureOptionCalculationsShort(ctx context.Context) error {
	contract := sdkadapter.Contract{
		Symbol:     "AAPL",
		SecType:    "OPT",
		Expiry:     "20260619",
		Strike:     "200",
		Right:      "C",
		Exchange:   "SMART",
		Currency:   "USD",
		Multiplier: "100",
	}

	const impliedReqID = 1001
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandCalcImpliedVolatility,
		CalcImpliedVolatility: sdkadapter.CalcImpliedVolatilityCommand{
			ReqID:       impliedReqID,
			Contract:    contract,
			OptionPrice: "5.25",
			UnderPrice:  "200",
		},
	}); err != nil {
		return fmt.Errorf("calc implied volatility submit: %w", err)
	}
	if err := r.drainUntil(ctx, "calc implied volatility", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID == impliedReqID &&
				(event.Kind == sdkadapter.EventTickOptionComputation || event.Kind == sdkadapter.EventAPIError) {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                 sdkadapter.CommandCancelCalcImpliedVol,
		CancelCalcImpliedVol: sdkadapter.CancelCalcImpliedVolCommand{ReqID: impliedReqID},
	}); err != nil {
		return fmt.Errorf("calc implied volatility cancel submit: %w", err)
	}

	const priceReqID = 1002
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandCalcOptionPrice,
		CalcOptionPrice: sdkadapter.CalcOptionPriceCommand{
			ReqID:      priceReqID,
			Contract:   contract,
			Volatility: "0.3",
			UnderPrice: "200",
		},
	}); err != nil {
		return fmt.Errorf("calc option price submit: %w", err)
	}
	if err := r.drainUntil(ctx, "calc option price", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID == priceReqID &&
				(event.Kind == sdkadapter.EventTickOptionComputation || event.Kind == sdkadapter.EventAPIError) {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                  sdkadapter.CommandCancelCalcOptionPrice,
		CancelCalcOptionPrice: sdkadapter.CancelCalcOptionPriceCommand{ReqID: priceReqID},
	}); err != nil {
		return fmt.Errorf("calc option price cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureOptionCalculationsQualified(ctx context.Context) error {
	contract := sdkadapter.Contract{
		Symbol:       "AAPL",
		SecType:      "OPT",
		Expiry:       "20260618",
		Strike:       "200",
		Right:        "C",
		Exchange:     "SMART",
		Currency:     "USD",
		Multiplier:   "100",
		TradingClass: "AAPL",
	}

	const detailsReqID = 1000
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandContractDetails,
		ContractDetails: sdkadapter.ContractDetailsCommand{
			ReqID:    detailsReqID,
			Contract: contract,
		},
	}); err != nil {
		return fmt.Errorf("qualified option contract details submit: %w", err)
	}
	if err := r.drainUntil(ctx, "qualified option contract details", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID != detailsReqID {
				continue
			}
			if event.Kind == sdkadapter.EventContractDetailsEnd || event.Kind == sdkadapter.EventAPIError {
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if apiErr, ok := requestAPIError(r.events, detailsReqID); ok {
		return fmt.Errorf("qualified option contract details error: code=%d message=%s", apiErr.Code, apiErr.Message)
	}
	contract, err := firstContractDetailsContract(r.events, detailsReqID)
	if err != nil {
		return err
	}

	const impliedReqID = 1001
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandCalcImpliedVolatility,
		CalcImpliedVolatility: sdkadapter.CalcImpliedVolatilityCommand{
			ReqID:       impliedReqID,
			Contract:    contract,
			OptionPrice: "5.25",
			UnderPrice:  "200",
		},
	}); err != nil {
		return fmt.Errorf("qualified calc implied volatility submit: %w", err)
	}
	if err := r.drainUntil(ctx, "qualified calc implied volatility", optionComputationOrAPIError(impliedReqID)); err != nil {
		return err
	}
	if apiErr, ok := requestAPIError(r.events, impliedReqID); ok {
		return fmt.Errorf("qualified calc implied volatility error: code=%d message=%s", apiErr.Code, apiErr.Message)
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                 sdkadapter.CommandCancelCalcImpliedVol,
		CancelCalcImpliedVol: sdkadapter.CancelCalcImpliedVolCommand{ReqID: impliedReqID},
	}); err != nil {
		return fmt.Errorf("qualified calc implied volatility cancel submit: %w", err)
	}

	const priceReqID = 1002
	if err := r.submit(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandCalcOptionPrice,
		CalcOptionPrice: sdkadapter.CalcOptionPriceCommand{
			ReqID:      priceReqID,
			Contract:   contract,
			Volatility: "0.3",
			UnderPrice: "200",
		},
	}); err != nil {
		return fmt.Errorf("qualified calc option price submit: %w", err)
	}
	if err := r.drainUntil(ctx, "qualified calc option price", optionComputationOrAPIError(priceReqID)); err != nil {
		return err
	}
	if apiErr, ok := requestAPIError(r.events, priceReqID); ok {
		return fmt.Errorf("qualified calc option price error: code=%d message=%s", apiErr.Code, apiErr.Message)
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:                  sdkadapter.CommandCancelCalcOptionPrice,
		CancelCalcOptionPrice: sdkadapter.CancelCalcOptionPriceCommand{ReqID: priceReqID},
	}); err != nil {
		return fmt.Errorf("qualified calc option price cancel submit: %w", err)
	}
	return nil
}

func (r *recorder) captureExecutionsEmptyFilter(ctx context.Context) error {
	const reqID = 1101
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandExecutions,
		Executions: sdkadapter.ExecutionsCommand{
			ReqID:  reqID,
			Symbol: "ZZZZZZZZZZ",
		},
	}, "empty executions filter", reqKind(reqID, sdkadapter.EventExecutionsEnd)); err != nil {
		return err
	}
	return nil
}

func (r *recorder) captureCompletedOrdersSnapshot(ctx context.Context) error {
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind:            sdkadapter.CommandCompletedOrders,
		CompletedOrders: sdkadapter.CompletedOrdersCommand{APIOnly: true},
	}, "completed orders", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventCompletedOrderEnd
	}); err != nil {
		return err
	}
	return nil
}

func quoteStreamDataEvent(kind sdkadapter.EventKind) bool {
	switch kind {
	case sdkadapter.EventTickPrice,
		sdkadapter.EventTickSize,
		sdkadapter.EventTickGeneric,
		sdkadapter.EventTickString:
		return true
	default:
		return false
	}
}

func (r *recorder) capturePaperOrderModifyCancel(ctx context.Context) error {
	account, err := paperAccountFromEvents(r.events)
	if err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return fmt.Errorf("market data type submit: %w", err)
	}
	const quoteReqID = 211
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandQuote,
		Quote: sdkadapter.QuoteCommand{
			ReqID:    quoteReqID,
			Contract: aaplStockContract(),
			Snapshot: true,
		},
	}, "paper modify quote snapshot", reqKind(quoteReqID, sdkadapter.EventTickSnapshotEnd)); err != nil {
		return err
	}

	limitPrice, err := paperOrderLimitPrice(r.events, quoteReqID)
	if err != nil {
		return err
	}
	orderID, err := nextValidOrderID(r.events)
	if err != nil {
		return err
	}

	cancelled := false
	cancel := sdkadapter.Command{
		Kind:        sdkadapter.CommandCancelOrder,
		CancelOrder: sdkadapter.CancelOrderCommand{OrderID: orderID},
	}
	defer func() {
		if !cancelled {
			cancelCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
			_ = r.submit(cancelCtx, cancel)
			cancelCleanup()
		}
	}()

	if err := r.submit(ctx, sdkadapter.Command{
		Kind:       sdkadapter.CommandPlaceOrder,
		PlaceOrder: paperOrderRequest(orderID, account, limitPrice, "1", "ibkr-go-sdk-fixture-mod-"),
	}); err != nil {
		return fmt.Errorf("paper order submit: %w", err)
	}
	accepted, rejected, err := r.waitPaperOrderAccepted(ctx, orderID)
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("paper order rejected before acceptance: code=%d message=%s", rejected.APIError.Code, rejected.APIError.Message)
	}

	if err := r.submit(ctx, sdkadapter.Command{
		Kind:       sdkadapter.CommandPlaceOrder,
		PlaceOrder: paperOrderRequest(orderID, account, limitPrice, "2", "ibkr-go-sdk-fixture-mod-"),
	}); err != nil {
		return fmt.Errorf("paper order modify submit: %w", err)
	}
	modified := false
	if err := r.drainUntil(ctx, "paper order modified", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if paperOrderOpenQuantity(event, orderID, decimal.NewFromInt(2)) {
				modified = true
				return true
			}
			if paperOrderRejected(event, orderID) {
				rejected = event
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if !modified {
		return fmt.Errorf("paper order modify rejected: code=%d message=%s", rejected.APIError.Code, rejected.APIError.Message)
	}

	if err := r.submit(ctx, cancel); err != nil {
		return fmt.Errorf("paper order cancel submit: %w", err)
	}
	filled := false
	if err := r.drainUntil(ctx, "paper modified order cancelled", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.Kind != sdkadapter.EventOrderStatus || event.OrderStatus.OrderID != orderID {
				continue
			}
			switch event.OrderStatus.Status {
			case "Cancelled", "ApiCancelled":
				cancelled = true
				return true
			case "Filled":
				filled = true
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if filled {
		return fmt.Errorf("paper safety order filled before cancellation completed")
	}
	return nil
}

func (r *recorder) capturePaperOpenOrdersPlaceCancel(ctx context.Context) error {
	account, err := paperAccountFromEvents(r.events)
	if err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return fmt.Errorf("market data type submit: %w", err)
	}
	const quoteReqID = 221
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandQuote,
		Quote: sdkadapter.QuoteCommand{
			ReqID:    quoteReqID,
			Contract: aaplStockContract(),
			Snapshot: true,
		},
	}, "paper open orders quote snapshot", reqKind(quoteReqID, sdkadapter.EventTickSnapshotEnd)); err != nil {
		return err
	}

	limitPrice, err := paperOrderLimitPrice(r.events, quoteReqID)
	if err != nil {
		return err
	}
	orderID, err := nextValidOrderID(r.events)
	if err != nil {
		return err
	}

	cancelled := false
	cancel := sdkadapter.Command{
		Kind:        sdkadapter.CommandCancelOrder,
		CancelOrder: sdkadapter.CancelOrderCommand{OrderID: orderID},
	}
	defer func() {
		if !cancelled {
			cancelCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
			_ = r.submit(cancelCtx, cancel)
			cancelCleanup()
		}
	}()

	const orderRefPrefix = "ibkr-go-sdk-fixture-open-"
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:       sdkadapter.CommandPlaceOrder,
		PlaceOrder: paperOrderRequest(orderID, account, limitPrice, "1", orderRefPrefix),
	}); err != nil {
		return fmt.Errorf("paper order submit: %w", err)
	}
	accepted, rejected, err := r.waitPaperOrderAccepted(ctx, orderID)
	if err != nil {
		return err
	}
	if !accepted {
		return fmt.Errorf("paper order rejected before acceptance: code=%d message=%s", rejected.APIError.Code, rejected.APIError.Message)
	}

	openOrdersStart := len(r.events)
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:       sdkadapter.CommandOpenOrders,
		OpenOrders: sdkadapter.OpenOrdersCommand{Scope: "client"},
	}); err != nil {
		return fmt.Errorf("paper open orders submit: %w", err)
	}
	sawSnapshotOpenOrder := false
	if err := r.drainUntil(ctx, "paper open orders snapshot", func(events []sdkadapter.Event) bool {
		for _, event := range events[openOrdersStart:] {
			switch event.Kind {
			case sdkadapter.EventOpenOrder:
				if event.OpenOrder.OrderID == orderID && strings.HasPrefix(event.OpenOrder.OrderRef, orderRefPrefix) {
					sawSnapshotOpenOrder = true
				}
			case sdkadapter.EventOpenOrderEnd:
				return sawSnapshotOpenOrder
			}
		}
		return false
	}); err != nil {
		return err
	}
	if !sawSnapshotOpenOrder {
		return fmt.Errorf("paper open orders snapshot completed without the scenario order")
	}

	if err := r.submit(ctx, cancel); err != nil {
		return fmt.Errorf("paper order cancel submit: %w", err)
	}
	filled := false
	if err := r.drainUntil(ctx, "paper open orders order cancelled", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.Kind != sdkadapter.EventOrderStatus || event.OrderStatus.OrderID != orderID {
				continue
			}
			switch event.OrderStatus.Status {
			case "Cancelled", "ApiCancelled":
				cancelled = true
				return true
			case "Filled":
				filled = true
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if filled {
		return fmt.Errorf("paper safety order filled before cancellation completed")
	}
	return nil
}

func (r *recorder) capturePaperOrderRejectInvalidType(ctx context.Context) error {
	account, err := paperAccountFromEvents(r.events)
	if err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return fmt.Errorf("market data type submit: %w", err)
	}
	const quoteReqID = 231
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandQuote,
		Quote: sdkadapter.QuoteCommand{
			ReqID:    quoteReqID,
			Contract: aaplStockContract(),
			Snapshot: true,
		},
	}, "paper invalid order quote snapshot", reqKind(quoteReqID, sdkadapter.EventTickSnapshotEnd)); err != nil {
		return err
	}

	limitPrice, err := paperOrderLimitPrice(r.events, quoteReqID)
	if err != nil {
		return err
	}
	orderID, err := nextValidOrderID(r.events)
	if err != nil {
		return err
	}

	cancelled := false
	cancel := sdkadapter.Command{
		Kind:        sdkadapter.CommandCancelOrder,
		CancelOrder: sdkadapter.CancelOrderCommand{OrderID: orderID},
	}
	defer func() {
		if !cancelled {
			cancelCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
			_ = r.submit(cancelCtx, cancel)
			cancelCleanup()
		}
	}()

	request := paperOrderRequest(orderID, account, limitPrice, "1", "ibkr-go-sdk-fixture-reject-")
	request.OrderType = "FEELINGS"
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:       sdkadapter.CommandPlaceOrder,
		PlaceOrder: request,
	}); err != nil {
		return fmt.Errorf("paper invalid order submit: %w", err)
	}

	accepted := false
	rejected := sdkadapter.Event{}
	if err := r.drainUntil(ctx, "paper invalid order rejected", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if paperOrderAccepted(event, orderID) {
				accepted = true
				return true
			}
			if paperOrderRejected(event, orderID) {
				rejected = event
				return true
			}
		}
		return false
	}); err != nil {
		return err
	}
	if accepted {
		return fmt.Errorf("paper invalid order type was unexpectedly accepted")
	}
	if rejected.Kind == "" {
		return fmt.Errorf("paper invalid order type did not produce a rejection event")
	}
	cancelled = true
	return nil
}

type recorder struct {
	adapter sdkadapter.Adapter
	events  []sdkadapter.Event
}

func (r *recorder) captureReadOnlySmoke(ctx context.Context) error {
	if err := r.submitAndWait(ctx, sdkadapter.Command{Kind: sdkadapter.CommandCurrentTime}, "current time", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventCurrentTime
	}); err != nil {
		return err
	}
	if err := r.submit(ctx, sdkadapter.Command{
		Kind:           sdkadapter.CommandMarketDataType,
		MarketDataType: sdkadapter.MarketDataTypeCommand{DataType: 3},
	}); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandContractDetails,
		ContractDetails: sdkadapter.ContractDetailsCommand{
			ReqID:    101,
			Contract: aaplStockContract(),
		},
	}, "contract details", reqKind(101, sdkadapter.EventContractDetailsEnd)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandHeadTimestamp,
		HeadTimestamp: sdkadapter.HeadTimestampCommand{
			ReqID:      102,
			Contract:   aaplStockContract(),
			WhatToShow: "TRADES",
			UseRTH:     true,
		},
	}, "head timestamp", reqKind(102, sdkadapter.EventHeadTimestamp)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandHistogramData,
		HistogramData: sdkadapter.HistogramDataCommand{
			ReqID:    108,
			Contract: aaplStockContract(),
			UseRTH:   true,
			Period:   "1 year",
		},
	}, "histogram data", reqKind(108, sdkadapter.EventHistogramData)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandQuote,
		Quote: sdkadapter.QuoteCommand{
			ReqID:    103,
			Contract: aaplStockContract(),
			Snapshot: true,
		},
	}, "quote snapshot", reqKind(103, sdkadapter.EventTickSnapshotEnd)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandMatchingSymbols,
		MatchingSymbols: sdkadapter.MatchingSymbolsCommand{
			ReqID:   104,
			Pattern: "AAPL",
		},
	}, "matching symbols", reqKind(104, sdkadapter.EventMatchingSymbols)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind:       sdkadapter.CommandMarketRule,
		MarketRule: sdkadapter.MarketRuleCommand{MarketRuleID: 26},
	}, "market rule", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventMarketRule && event.MarketRuleID == 26
	}); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandSecDefOptParams,
		SecDefOptParams: sdkadapter.SecDefOptParamsCommand{
			ReqID:             105,
			UnderlyingSymbol:  "AAPL",
			UnderlyingSecType: "STK",
			UnderlyingConID:   265598,
		},
	}, "sec def opt params", reqKind(105, sdkadapter.EventSecDefOptParamsEnd)); err != nil {
		return err
	}
	bboExchange := quoteBBOExchange(r.events)
	if bboExchange == "" {
		return fmt.Errorf("quote snapshot did not return a BBO exchange for smart components")
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandSmartComponents,
		SmartComponents: sdkadapter.SmartComponentsCommand{
			ReqID:       106,
			BBOExchange: bboExchange,
		},
	}, "smart components", reqKind(106, sdkadapter.EventSmartComponents)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandMktDepthExchanges,
	}, "market depth exchanges", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventMktDepthExchanges
	}); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandNewsProviders,
	}, "news providers", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventNewsProviders
	}); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind:            sdkadapter.CommandSoftDollarTiers,
		SoftDollarTiers: sdkadapter.SoftDollarTiersCommand{ReqID: 109},
	}, "soft dollar tiers", reqKind(109, sdkadapter.EventSoftDollarTiers)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind:     sdkadapter.CommandUserInfo,
		UserInfo: sdkadapter.UserInfoCommand{ReqID: 110},
	}, "user info", reqKind(110, sdkadapter.EventUserInfo)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind:               sdkadapter.CommandQueryDisplayGroups,
		QueryDisplayGroups: sdkadapter.QueryDisplayGroupsCommand{ReqID: 111},
	}, "display groups", reqKind(111, sdkadapter.EventDisplayGroupList)); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind:        sdkadapter.CommandWSHMetaData,
		WSHMetaData: sdkadapter.WSHMetaDataCommand{ReqID: 107},
	}, "wsh metadata entitlement", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventWSHMetaData || (event.Kind == sdkadapter.EventAPIError && event.ReqID == 107)
	}); err != nil {
		return err
	}
	if err := r.submitAndWait(ctx, sdkadapter.Command{
		Kind: sdkadapter.CommandWSHEventData,
		WSHEventData: sdkadapter.WSHEventDataCommand{
			ReqID: 112,
			ConID: 265598,
		},
	}, "wsh event data entitlement", func(event sdkadapter.Event) bool {
		return event.Kind == sdkadapter.EventWSHEventData || (event.Kind == sdkadapter.EventAPIError && event.ReqID == 112)
	}); err != nil {
		return err
	}
	return nil
}

func (r *recorder) submitAndWait(ctx context.Context, command sdkadapter.Command, label string, done func(sdkadapter.Event) bool) error {
	if err := r.submit(ctx, command); err != nil {
		return fmt.Errorf("%s submit: %w", label, err)
	}
	return r.drainUntil(ctx, label, func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if done(event) {
				return true
			}
		}
		return false
	})
}

func (r *recorder) submit(ctx context.Context, command sdkadapter.Command) error {
	return r.adapter.Submit(ctx, command)
}

func (r *recorder) drainUntil(ctx context.Context, label string, done func([]sdkadapter.Event) bool) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		events, err := r.adapter.DrainEvents(ctx, 128)
		if err != nil {
			return fmt.Errorf("%s drain: %w", label, err)
		}
		if len(events) > 0 {
			r.events = append(r.events, events...)
			if done(r.events) {
				return nil
			}
			for _, event := range events {
				if event.Kind == sdkadapter.EventAdapterFatal {
					return fmt.Errorf("%s adapter fatal: %s", label, event.FatalMessage)
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", label, ctx.Err())
		case <-ticker.C:
		}
	}
}

func reqKind(reqID int, kind sdkadapter.EventKind) func(sdkadapter.Event) bool {
	return func(event sdkadapter.Event) bool {
		return event.ReqID == reqID && event.Kind == kind
	}
}

func optionComputationOrAPIError(reqID int) func([]sdkadapter.Event) bool {
	return func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if event.ReqID == reqID &&
				(event.Kind == sdkadapter.EventTickOptionComputation || event.Kind == sdkadapter.EventAPIError) {
				return true
			}
		}
		return false
	}
}

func requestAPIError(events []sdkadapter.Event, reqID int) (sdkadapter.Error, bool) {
	for _, event := range events {
		if event.Kind == sdkadapter.EventAPIError && event.ReqID == reqID {
			return event.APIError, true
		}
	}
	return sdkadapter.Error{}, false
}

func firstContractDetailsContract(events []sdkadapter.Event, reqID int) (sdkadapter.Contract, error) {
	for _, event := range events {
		if event.Kind == sdkadapter.EventContractDetails && event.ReqID == reqID {
			return event.ContractDetails.Contract, nil
		}
	}
	return sdkadapter.Contract{}, fmt.Errorf("qualified option contract details returned no contractDetails callback")
}

func aaplStockContract() sdkadapter.Contract {
	return sdkadapter.Contract{
		Symbol:   "AAPL",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}
}

func officialSampleBondWithCUSIPContract() sdkadapter.Contract {
	return sdkadapter.Contract{
		Symbol:   "449276AA2",
		SecType:  "BOND",
		Exchange: "SMART",
		Currency: "USD",
	}
}

func quoteBBOExchange(events []sdkadapter.Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Kind == sdkadapter.EventTickReqParams && event.TickReqParams.BBOExchange != "" {
			return event.TickReqParams.BBOExchange
		}
	}
	return ""
}

func firstDisplayGroupID(events []sdkadapter.Event, reqID int) (int, error) {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Kind != sdkadapter.EventDisplayGroupList || event.ReqID != reqID {
			continue
		}
		for _, part := range strings.Split(event.DisplayGroups, "|") {
			if part == "" {
				continue
			}
			groupID, err := strconv.Atoi(part)
			if err != nil {
				return 0, fmt.Errorf("parse display group ID %q: %w", part, err)
			}
			return groupID, nil
		}
	}
	return 0, fmt.Errorf("display group subscription fixture refused to subscribe: no display groups returned")
}

func managedAccountFromEvents(events []sdkadapter.Event) (string, error) {
	for _, event := range events {
		if event.Kind == sdkadapter.EventManagedAccounts && len(event.Accounts) > 0 {
			return event.Accounts[0], nil
		}
	}
	return "", fmt.Errorf("account stream fixture refused to subscribe: no managed accounts")
}

func paperAccountFromEvents(events []sdkadapter.Event) (string, error) {
	var accounts []string
	for _, event := range events {
		if event.Kind == sdkadapter.EventManagedAccounts {
			accounts = append(accounts[:0], event.Accounts...)
		}
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("paper order fixture refused to place order: no managed accounts")
	}
	for _, account := range accounts {
		if !strings.HasPrefix(account, "DU") {
			return "", fmt.Errorf("paper order fixture refused to place order: managed account does not look like an IBKR paper account")
		}
	}
	return accounts[0], nil
}

func firstPositionConID(events []sdkadapter.Event) (int, error) {
	for _, event := range events {
		if event.Kind == sdkadapter.EventPosition && event.Position.Contract.ConID > 0 {
			return event.Position.Contract.ConID, nil
		}
	}
	return 0, fmt.Errorf("account stream fixture refused to request pnl single: no position conID")
}

func nextValidOrderID(events []sdkadapter.Event) (int64, error) {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Kind == sdkadapter.EventNextValidID && event.NextValidID > 0 {
			return event.NextValidID, nil
		}
	}
	return 0, fmt.Errorf("paper order fixture refused to place order: no nextValidId callback")
}

func paperOrderLimitPrice(events []sdkadapter.Event, reqID int) (decimal.Decimal, error) {
	reference := decimal.Zero
	for _, tickType := range []int{1, 66, 4, 68, 9, 75, 2, 67} {
		for _, event := range events {
			if event.Kind != sdkadapter.EventTickPrice || event.ReqID != reqID || event.TickPrice.TickType != tickType {
				continue
			}
			value, err := decimal.NewFromString(event.TickPrice.Price)
			if err != nil {
				return decimal.Zero, fmt.Errorf("parse quote reference price %q: %w", event.TickPrice.Price, err)
			}
			if value.GreaterThan(decimal.Zero) {
				reference = value
				break
			}
		}
		if reference.GreaterThan(decimal.Zero) {
			break
		}
	}
	if reference.IsZero() {
		return decimal.Zero, fmt.Errorf("paper order fixture refused to place order: quote snapshot returned no positive bid/last/close/ask")
	}
	limit := reference.Mul(decimal.NewFromInt(9)).Div(decimal.NewFromInt(10)).Round(2)
	minimum := decimal.NewFromInt(1).Div(decimal.NewFromInt(100))
	if limit.LessThan(minimum) {
		return minimum, nil
	}
	return limit, nil
}

func paperOrderRequest(orderID int64, account string, limitPrice decimal.Decimal, quantity string, orderRefPrefix string) sdkadapter.PlaceOrderRequest {
	return sdkadapter.PlaceOrderRequest{
		OrderID:                     orderID,
		Contract:                    aaplStockContract(),
		Action:                      "BUY",
		TotalQuantity:               quantity,
		OrderType:                   "LMT",
		LmtPrice:                    limitPrice.StringFixed(2),
		TIF:                         "DAY",
		Account:                     account,
		Origin:                      "0",
		OrderRef:                    orderRefPrefix + time.Now().UTC().Format("20060102T150405"),
		Transmit:                    "1",
		ParentID:                    "0",
		TriggerMethod:               "0",
		OutsideRTH:                  "0",
		DisplaySize:                 "0",
		ExemptCode:                  "-1",
		ConditionsIgnoreRTH:         "0",
		ConditionsCancelOrder:       "0",
		AdjustableTrailingUnit:      "0",
		DeltaNeutralContractPresent: "0",
	}
}

func (r *recorder) waitPaperOrderAccepted(ctx context.Context, orderID int64) (bool, sdkadapter.Event, error) {
	accepted := false
	rejected := sdkadapter.Event{}
	if err := r.drainUntil(ctx, "paper order accepted", func(events []sdkadapter.Event) bool {
		for _, event := range events {
			if paperOrderAccepted(event, orderID) {
				accepted = true
				return true
			}
			if paperOrderRejected(event, orderID) {
				rejected = event
				return true
			}
		}
		return false
	}); err != nil {
		return false, sdkadapter.Event{}, err
	}
	return accepted, rejected, nil
}

func paperOrderAccepted(event sdkadapter.Event, orderID int64) bool {
	switch event.Kind {
	case sdkadapter.EventOpenOrder:
		return event.OpenOrder.OrderID == orderID
	case sdkadapter.EventOrderStatus:
		if event.OrderStatus.OrderID != orderID {
			return false
		}
		switch event.OrderStatus.Status {
		case "PendingSubmit", "PreSubmitted", "Submitted":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func paperOrderOpenQuantity(event sdkadapter.Event, orderID int64, want decimal.Decimal) bool {
	if event.Kind != sdkadapter.EventOpenOrder || event.OpenOrder.OrderID != orderID {
		return false
	}
	quantity, err := decimal.NewFromString(event.OpenOrder.Quantity)
	if err != nil {
		return false
	}
	return quantity.Equal(want)
}

func paperOrderRejected(event sdkadapter.Event, orderID int64) bool {
	if event.Kind != sdkadapter.EventAPIError || int64(event.ReqID) != orderID {
		return false
	}
	return !isPaperOrderInformationalAPIError(event.APIError)
}

func isPaperOrderInformationalAPIError(err sdkadapter.Error) bool {
	if err.Code == 201 &&
		strings.Contains(err.Message, "too late to replace") &&
		strings.Contains(err.Message, "cancelled already") {
		return true
	}
	switch err.Code {
	case 202, 399:
		return true
	default:
		return false
	}
}

func hasKind(events []sdkadapter.Event, kind sdkadapter.EventKind) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func eventHash(events []sdkadapter.Event) (string, error) {
	raw, err := json.Marshal(events)
	if err != nil {
		return "", fmt.Errorf("hash source events: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func redactAccountIdentifiers(events []sdkadapter.Event) []sdkadapter.Event {
	redacted := sdkadapter.CloneEvents(events)
	accounts := collectAccounts(redacted)
	for i := range redacted {
		event := &redacted[i]
		redactStrings(event.Accounts, accounts)
		event.AccountSummary.Account = redactString(event.AccountSummary.Account, accounts)
		event.AccountValue.Account = redactString(event.AccountValue.Account, accounts)
		event.Portfolio.Account = redactString(event.Portfolio.Account, accounts)
		event.AccountDownloadEnd = redactString(event.AccountDownloadEnd, accounts)
		event.AccountUpdateMulti.Account = redactString(event.AccountUpdateMulti.Account, accounts)
		event.Position.Account = redactString(event.Position.Account, accounts)
		event.PositionMulti.Account = redactString(event.PositionMulti.Account, accounts)
		event.ExecutionDetail.Account = redactString(event.ExecutionDetail.Account, accounts)
		event.OpenOrder.Account = redactString(event.OpenOrder.Account, accounts)
		event.OpenOrder.PermID = redactOrderIdentifier(event.OpenOrder.PermID)
		event.OrderStatus.PermID = redactOrderIdentifier(event.OrderStatus.PermID)
		for j := range event.FamilyCodes {
			event.FamilyCodes[j].AccountID = redactString(event.FamilyCodes[j].AccountID, accounts)
		}
		event.APIError.Message = redactString(event.APIError.Message, accounts)
		event.FatalMessage = redactString(event.FatalMessage, accounts)
	}
	return redacted
}

func redactionNotes(scenario string) string {
	switch scenario {
	case "paper_order_place_cancel":
		return "account identifiers redacted to DU_REDACTED and order permIDs zeroed; fixture contains one paper order placement and cancel callback sequence"
	case "paper_order_modify_cancel":
		return "account identifiers redacted to DU_REDACTED and order permIDs zeroed; fixture contains one paper order placement, quantity modify, and cancel callback sequence"
	case "paper_open_orders_place_cancel":
		return "account identifiers redacted to DU_REDACTED and order permIDs zeroed; fixture contains one paper order placement, client-scope open-orders snapshot, and cancel callback sequence"
	case "paper_order_reject_invalid_type":
		return "account identifiers redacted to DU_REDACTED and order permIDs zeroed; fixture contains one rejected paper order with an invalid order type"
	case "account_summary_snapshot":
		return "account identifiers redacted to DU_REDACTED and account summary values replaced with REDACTED_VALUE; fixture preserves real SDK account summary callback shape"
	case "account_streams_snapshot":
		return "account identifiers redacted to DU_REDACTED; account, portfolio, position, PnL, model, and position-contract values redacted; fixture preserves real SDK account stream callback shape"
	case "completed_orders_snapshot":
		return "account identifiers redacted to DU_REDACTED; completed-order contracts and order fields redacted; fixture preserves real SDK completed-order callback shape"
	case "news_article_snapshot":
		return "account identifiers redacted to DU_REDACTED; provider article ID, headline, and article text replaced with redaction placeholders while preserving callback shape"
	case "news_bulletins_short":
		return "account identifiers redacted to DU_REDACTED; bulletin headlines replaced with redaction placeholders while preserving callback shape"
	default:
		return "account identifiers redacted to DU_REDACTED; fixture contains read-only SDK callbacks only"
	}
}

func redactPrivateValues(events []sdkadapter.Event, scenario string) []sdkadapter.Event {
	for i := range events {
		switch scenario {
		case "account_summary_snapshot":
			if events[i].Kind == sdkadapter.EventAccountSummary {
				events[i].AccountSummary.Value = redactValue(events[i].AccountSummary.Value)
			}
		case "account_streams_snapshot":
			redactAccountStreamEvent(&events[i])
		case "completed_orders_snapshot":
			redactCompletedOrderEvent(&events[i])
		case "news_article_snapshot":
			redactNewsEvent(&events[i])
		case "news_bulletins_short":
			redactNewsBulletinEvent(&events[i])
		}
	}
	return events
}

func redactNewsEvent(event *sdkadapter.Event) {
	switch event.Kind {
	case sdkadapter.EventHistoricalNews:
		event.HistoricalNews.ArticleID = redactedArticleID
		event.HistoricalNews.Headline = redactedHeadline
	case sdkadapter.EventNewsArticle:
		event.NewsArticle.ArticleText = redactedArticle
	}
}

func redactNewsBulletinEvent(event *sdkadapter.Event) {
	if event.Kind == sdkadapter.EventNewsBulletin {
		event.NewsBulletin.Headline = redactedHeadline
	}
}

func redactAccountStreamEvent(event *sdkadapter.Event) {
	switch event.Kind {
	case sdkadapter.EventUpdateAccountValue:
		event.AccountValue.Value = redactValue(event.AccountValue.Value)
	case sdkadapter.EventUpdatePortfolio:
		event.Portfolio.Contract = redactedPrivateContract(event.Portfolio.Contract)
		event.Portfolio.Position = redactValue(event.Portfolio.Position)
		event.Portfolio.MarketPrice = redactValue(event.Portfolio.MarketPrice)
		event.Portfolio.MarketValue = redactValue(event.Portfolio.MarketValue)
		event.Portfolio.AvgCost = redactValue(event.Portfolio.AvgCost)
		event.Portfolio.UnrealizedPNL = redactValue(event.Portfolio.UnrealizedPNL)
		event.Portfolio.RealizedPNL = redactValue(event.Portfolio.RealizedPNL)
	case sdkadapter.EventAccountUpdateMulti:
		event.AccountUpdateMulti.ModelCode = redactModel(event.AccountUpdateMulti.ModelCode)
		event.AccountUpdateMulti.Value = redactValue(event.AccountUpdateMulti.Value)
	case sdkadapter.EventPosition:
		event.Position.Contract = redactedPrivateContract(event.Position.Contract)
		event.Position.Position = redactValue(event.Position.Position)
		event.Position.AvgCost = redactValue(event.Position.AvgCost)
	case sdkadapter.EventPositionMulti:
		event.PositionMulti.ModelCode = redactModel(event.PositionMulti.ModelCode)
		event.PositionMulti.Contract = redactedPrivateContract(event.PositionMulti.Contract)
		event.PositionMulti.Position = redactValue(event.PositionMulti.Position)
		event.PositionMulti.AvgCost = redactValue(event.PositionMulti.AvgCost)
	case sdkadapter.EventPnL:
		event.PnL.DailyPnL = redactValue(event.PnL.DailyPnL)
		event.PnL.UnrealizedPnL = redactValue(event.PnL.UnrealizedPnL)
		event.PnL.RealizedPnL = redactValue(event.PnL.RealizedPnL)
	case sdkadapter.EventPnLSingle:
		event.PnLSingle.Position = redactValue(event.PnLSingle.Position)
		event.PnLSingle.DailyPnL = redactValue(event.PnLSingle.DailyPnL)
		event.PnLSingle.UnrealizedPnL = redactValue(event.PnLSingle.UnrealizedPnL)
		event.PnLSingle.RealizedPnL = redactValue(event.PnLSingle.RealizedPnL)
		event.PnLSingle.Value = redactValue(event.PnLSingle.Value)
	}
}

func redactCompletedOrderEvent(event *sdkadapter.Event) {
	if event.Kind != sdkadapter.EventCompletedOrder {
		return
	}
	event.CompletedOrder.Contract = redactedPrivateContract(event.CompletedOrder.Contract)
	event.CompletedOrder.Action = redactValue(event.CompletedOrder.Action)
	event.CompletedOrder.OrderType = redactValue(event.CompletedOrder.OrderType)
	event.CompletedOrder.Status = redactValue(event.CompletedOrder.Status)
	event.CompletedOrder.Quantity = redactValue(event.CompletedOrder.Quantity)
	event.CompletedOrder.Filled = redactValue(event.CompletedOrder.Filled)
	event.CompletedOrder.Remaining = redactValue(event.CompletedOrder.Remaining)
}

func redactedPrivateContract(contract sdkadapter.Contract) sdkadapter.Contract {
	if contract == (sdkadapter.Contract{}) {
		return contract
	}
	return sdkadapter.Contract{
		Symbol:   redactedContract,
		SecType:  redactedContract,
		Exchange: redactedContract,
		Currency: redactedContract,
	}
}

func redactModel(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return redactedModelCode
}

func redactValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return redactedValue
}

func collectAccounts(events []sdkadapter.Event) []string {
	seen := make(map[string]bool)
	var accounts []string
	add := func(account string) {
		account = strings.TrimSpace(account)
		if account == "" || seen[account] {
			return
		}
		seen[account] = true
		accounts = append(accounts, account)
	}
	for _, event := range events {
		for _, account := range event.Accounts {
			add(account)
		}
		add(event.AccountSummary.Account)
		add(event.AccountValue.Account)
		add(event.Portfolio.Account)
		add(event.AccountDownloadEnd)
		add(event.AccountUpdateMulti.Account)
		add(event.Position.Account)
		add(event.PositionMulti.Account)
		add(event.ExecutionDetail.Account)
		add(event.OpenOrder.Account)
		for _, code := range event.FamilyCodes {
			add(code.AccountID)
		}
	}
	return accounts
}

func redactStrings(values []string, accounts []string) {
	for i := range values {
		values[i] = redactString(values[i], accounts)
	}
}

func redactString(value string, accounts []string) string {
	for _, account := range accounts {
		value = strings.ReplaceAll(value, account, redactedAccount)
	}
	return value
}

func redactOrderIdentifier(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return "0"
}
