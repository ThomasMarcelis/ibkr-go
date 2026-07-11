package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

// scenario owns the catalog metadata and exactly one wire or public-API runner
// for a live capture scenario.
type scenario struct {
	metadata    scenarioMetadata
	description string
	run         func(ctx context.Context, conn net.Conn, sess *sessionInfo) error
	runAPI      func(ctx context.Context, addr string, clientID int) error
}

func (s scenario) driver() (string, error) {
	switch {
	case s.run != nil && s.runAPI == nil:
		return driverWire, nil
	case s.run == nil && s.runAPI != nil:
		return driverAPI, nil
	default:
		return "", fmt.Errorf("must define exactly one runner")
	}
}

// logFrame is the default per-frame logger used by readFrames. It prints a
// compact single line per frame for visibility.
func logFrame(msgID int, fields []string) {
	const maxShown = 8
	var shown string
	for i, f := range fields {
		if i > 0 {
			shown += "|"
		}
		if i >= maxShown {
			shown += fmt.Sprintf("…(+%d more)", len(fields)-maxShown)
			break
		}
		if len(f) > 48 {
			shown += f[:48] + "…"
		} else {
			shown += f
		}
	}
	log.Printf("frame msg_id=%d (%d fields): %s", msgID, len(fields), shown)
}

// stopOnMsgID returns a stop predicate that terminates on the given msg_id.
func stopOnMsgID(id int) func(int, []string) bool {
	return func(msgID int, _ []string) bool { return msgID == id }
}

// stopOnMsgIDWithReq terminates when msg_id matches AND any non-msg_id field
// matches the expected reqID. Scans every field so callers don't need to know
// exact layouts (some end markers have a version field before reqId, others
// don't — the wire format is inconsistent).
func stopOnMsgIDWithReq(id int, expectReqID string, _ int) func(int, []string) bool {
	return func(msgID int, fields []string) bool {
		if msgID != id {
			return false
		}
		for _, f := range fields[1:] {
			if f == expectReqID {
				return true
			}
		}
		return false
	}
}

// nextReqID allocates a deterministic reqID for a scenario starting at 1000.
var scenarioReqIDCounter = 1000

func nextReqID() int {
	scenarioReqIDCounter++
	return scenarioReqIDCounter
}

func rawScenarioManagedAccounts(sess *sessionInfo) []string {
	if sess == nil || sess.ManagedAccounts == "" {
		return nil
	}
	parts := strings.Split(sess.ManagedAccounts, ",")
	accounts := parts[:0]
	for _, part := range parts {
		account := strings.TrimSpace(part)
		if account != "" {
			accounts = append(accounts, account)
		}
	}
	return accounts
}

func verifyRawScenarioForSession(name string, sess *sessionInfo) error {
	sc, ok := scenarios[name]
	if !ok {
		return fmt.Errorf("unknown scenario; cannot verify capture safety")
	}
	driver, err := sc.driver()
	if err != nil {
		return err
	}
	if driver != driverWire {
		return nil
	}
	if sess != nil && sess.ServerVersion > 200 && name != "bootstrap" && name != "bootstrap_client_id_0" {
		return fmt.Errorf("raw scenario %s is not envelope-aware above server_version 200", name)
	}
	if !cancelsAllowedForRiskClass(sc.metadata.RiskClass) {
		return nil
	}
	return requirePaperAccounts(rawScenarioManagedAccounts(sess), "raw wire order-mutating scenario "+name)
}

var scenarios = map[string]*scenario{
	// --- Bootstrap-only scenarios ---

	"bootstrap": {
		metadata:    meta("session", []string{"DialContext"}, []int{71, 15, 9}, "read_only", nil, []string{"ready session", "farm status drain"}, 1, "promoted", batchReadOnly),
		description: "clean handshake + START_API + farm-status drain (no feature request)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			// Already bootstrapped. Read a few more frames to catch any late farm-status info.
			return readFramesAt(conn, sess.ServerVersion, 3*time.Second, logFrame, nil)
		},
	},
	"bootstrap_client_id_0": {
		metadata:    meta("session", []string{"DialContext"}, []int{71, 15, 9}, "read_only", []string{"client_id_0"}, []string{"ready session scoped to client ID 0"}, 0, "promoted", batchReadOnly),
		description: "same as bootstrap but client_id=0 (required for REQ_ALL_OPEN_ORDERS scope)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			return readFramesAt(conn, sess.ServerVersion, 3*time.Second, logFrame, nil)
		},
	},
	"current_time": {
		metadata:    meta("session", []string{"Client.CurrentTime"}, []int{49}, "read_only", nil, []string{"parsed server current time"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request and parse the Gateway's current time through the public API",
		runAPI:      runAPICurrentTime,
	},
	"current_time_millis": {
		metadata:    meta("session", []string{"Client.CurrentTimeMillis"}, []int{105, 109}, "read_only", nil, []string{"server current time in milliseconds"}, 1, "promoted", batchReadOnly),
		description: "request and parse millisecond-precision Gateway time through the public API",
		runAPI:      runAPICurrentTimeMillis,
	},
	"req_ids": {
		metadata:    meta("session", []string{"Orders().RefreshOrderID"}, []int{8, 9, 4}, "read_only", nil, []string{"refreshed order ID or real read-only Gateway rejection"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "refresh the engine-owned order ID seed through the public API",
		runAPI:      runAPIRefreshOrderID,
	},

	// --- Contract details ---

	"contract_details_aapl_stk": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"decoded AAPL stock contract details"}, 1, "promoted", batchReadOnly),
		description: "request and decode AAPL stock contract details through the public API",
		runAPI:      runAPIContractDetailsAAPLStock,
	},
	"contract_details_aapl_opt": {
		metadata:    meta("contracts", []string{"Contracts().SecDefOptParams", "Contracts().Details"}, []int{protocol.OutReqSecDefOptParams, protocol.InSecDefOptParams, protocol.InSecDefOptParamsEnd, protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"complete nearest-expiry option contract ladder"}, 1, "promoted", batchReadOnly),
		description: "resolve and completely decode the nearest AAPL option expiry through the public API",
		runAPI:      runAPIContractDetailsAAPLOptions,
	},
	"contract_details_apple_bonds": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InBondContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"bond contract details by live-derived issuer ID"}, 1, "promoted", batchReadOnly),
		description: "request and decode Apple bonds by live-derived issuer ID through the public API",
		runAPI:      runAPIContractDetailsAppleBonds,
	},
	"contract_details_eurusd_cash": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"cash/FX contract details"}, 1, "promoted", batchReadOnly),
		description: "request and decode EUR.USD cash contract details through the public API",
		runAPI:      runAPIContractDetailsEURUSD,
	},
	"contract_details_es_fut": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"ES futures expiry ladder"}, 1, "promoted", batchReadOnly),
		description: "request and decode the ES futures expiry ladder through the public API",
		runAPI:      runAPIContractDetailsESFutures,
	},
	"contract_details_not_found": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InErrMsg}, "read_only", nil, []string{"typed code 200 contract-not-found error"}, 1, "promoted", batchReadOnly),
		description: "request a nonexistent stock and require the live contract-not-found error through the public API",
		runAPI:      runAPIContractDetailsNotFound,
	},

	// --- Account summary ---

	"account_summary_snapshot": {
		metadata:    meta("accounts", []string{"Accounts().Summary", "Client.CurrentTime"}, []int{protocol.OutReqAccountSummary, protocol.InAccountSummary, protocol.InAccountSummaryEnd, protocol.OutReqCurrentTime}, "read_only", nil, []string{"finite account summary snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "collect and close a finite account summary through the public API",
		runAPI:      runAPIAccountSummarySnapshot,
	},
	"account_summary_stream": {
		metadata:    meta("accounts", []string{"Accounts().SubscribeSummary", "Client.CurrentTime"}, []int{protocol.OutReqAccountSummary, protocol.InAccountSummary, protocol.InAccountSummaryEnd, protocol.OutReqCurrentTime}, "read_only", nil, []string{"nonempty summary snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "observe and close an account-summary subscription through the public API",
		runAPI:      runAPIAccountSummaryStream,
	},
	"account_summary_two_subs": {
		metadata:    meta("accounts", []string{"Accounts().SubscribeSummary", "Client.CurrentTime"}, []int{protocol.OutReqAccountSummary, protocol.InAccountSummary, protocol.InAccountSummaryEnd, protocol.OutReqCurrentTime}, "read_only", nil, []string{"two nonempty concurrent summary snapshots and protocol-fenced cancellations"}, 1, "promoted", batchReadOnly),
		description: "observe two concurrent account-summary subscriptions through the public API",
		runAPI:      runAPIAccountSummaryTwoSubscriptions,
	},

	// --- Positions ---

	"positions_snapshot": {
		metadata:    meta("accounts", []string{"Accounts().Positions", "Client.CurrentTime"}, []int{protocol.OutReqPositions, protocol.InPositionEnd, protocol.OutCancelPositions, protocol.OutReqCurrentTime}, "read_only", nil, []string{"finite positions snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "collect and close the positions snapshot through the public API",
		runAPI:      runAPIPositionsSnapshot,
	},

	// --- Historical bars ---

	"historical_bars_1d_1h": {
		metadata:    meta("history", []string{"History().Bars"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalData, protocol.InHistoricalDataEnd}, "read_only", []string{"historical_data"}, []string{"nonempty hourly trade bars for liquid stock"}, 1, "promoted", batchReadOnly),
		description: "request and decode one day of hourly AAPL trade bars through the public API",
		runAPI:      runAPIHistoricalBars1Day1Hour,
	},
	"historical_bars_30d_1day": {
		metadata:    meta("history", []string{"History().Bars"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalData, protocol.InHistoricalDataEnd}, "read_only", []string{"historical_data"}, []string{"nonempty daily trade bars over a 30-day window"}, 1, "candidate", batchReadOnly),
		description: "request and decode 30 days of daily AAPL trade bars through the public API",
		runAPI:      runAPIHistoricalBars30Days1Day,
	},
	"historical_bars_bidask": {
		metadata:    meta("history", []string{"History().Bars"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalData, protocol.InHistoricalDataEnd, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty BID_ASK bars or exact historical-data permission error"}, 1, "candidate", batchReadOnly),
		description: "request and decode hourly AAPL BID_ASK bars or the typed live permission error through the public API",
		runAPI:      runAPIHistoricalBarsBidAsk,
	},
	"historical_bars_error": {
		metadata:    meta("history", []string{"History().Bars"}, []int{protocol.OutReqHistoricalData, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"typed code 200 historical-bars contract-not-found error"}, 1, "candidate", batchReadOnly),
		description: "request historical bars for a nonexistent stock and require the typed not-found error through the public API",
		runAPI:      runAPIHistoricalBarsError,
	},
	"historical_schedule_aapl": {
		metadata:    meta("history", []string{"History().Schedule"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalSchedule}, "read_only", []string{"historical_data"}, []string{"nonempty historical session schedule with timezone"}, 1, "candidate", batchNewV2, batchReadOnly),
		description: "request and decode one month of AAPL trading sessions through the public API",
		runAPI:      runAPIHistoricalScheduleAAPL,
	},

	// --- Market data type control (MarketData().SetType) ---

	"set_type_live": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqCurrentTime}, "read_only", nil, []string{"protocol-fenced live market-data type request"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "select live market data through the public API",
		runAPI:      runAPISetMarketDataLive,
	},
	"set_type_frozen": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqCurrentTime}, "read_only", nil, []string{"protocol-fenced frozen market-data type request"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "select frozen market data through the public API",
		runAPI:      runAPISetMarketDataFrozen,
	},
	"set_type_delayed": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqCurrentTime}, "read_only", nil, []string{"protocol-fenced delayed market-data type request"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "select delayed market data through the public API",
		runAPI:      runAPISetMarketDataDelayed,
	},
	"set_type_delayed_frozen": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqCurrentTime}, "read_only", nil, []string{"protocol-fenced delayed-frozen market-data type request"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "select delayed-frozen market data through the public API",
		runAPI:      runAPISetMarketDataDelayedFrozen,
	},
	"set_type_switch_while_streaming": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InMarketDataType, protocol.InTickPrice, protocol.InTickSize, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"delayed type and price/size evidence followed by accepted live switch and fenced cancellation"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "switch an active delayed AAPL quote stream to live through the public API",
		runAPI:      runAPISetTypeSwitchWhileStreaming,
	},

	// --- Market data quotes ---

	"quote_snapshot_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().Quote"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.InTickPrice, protocol.InTickSize, protocol.InTickString, protocol.InTickSnapshotEnd, protocol.InMarketDataType, protocol.InTickReqParams}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed delayed snapshot with price or size and snapshot completion"}, 1, "promoted", batchReadOnly),
		description: "request and decode a complete delayed AAPL quote snapshot through the public API",
		runAPI:      runAPIQuoteSnapshotAAPL,
	},
	"quote_stream_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InTickPrice, protocol.InTickSize, protocol.InTickString, protocol.InMarketDataType, protocol.InTickReqParams, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed delayed price or size update followed by fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "observe and cleanly cancel a delayed AAPL quote stream through the public API",
		runAPI:      runAPIQuoteStreamAAPL,
	},
	"quote_stream_genericticks": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InTickGeneric, protocol.InTickString, protocol.InMarketDataType, protocol.InTickReqParams, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed quote parameters and a 233/236 value followed by fenced cancellation"}, 1, "candidate", batchReadOnly),
		description: "observe generic ticks 233 and 236 on a delayed AAPL quote stream through the public API",
		runAPI:      runAPIQuoteStreamGenericTicksAAPL,
	},

	// --- Real-time bars ---

	"realtime_bars_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeRealTimeBars", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqRealTimeBars, protocol.OutCancelRealTimeBars, protocol.OutReqCurrentTime, protocol.InRealTimeBars, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed real-time bar and fenced cancellation or exact live permission refusal"}, 1, "promoted", batchReadOnly),
		description: "observe an AAPL real-time bar or the exact live permission refusal through the public API",
		runAPI:      runAPIRealTimeBarsAAPL,
	},

	// --- Open orders ---

	"open_orders_empty": {
		metadata:    meta("orders", []string{"Orders().Open"}, []int{protocol.OutReqOpenOrders, protocol.InOpenOrder, protocol.InOpenOrderEnd, protocol.InErrMsg}, "read_only", nil, []string{"own open-orders snapshot or exact read-only refusal"}, 1, "promoted", batchReadOnly),
		description: "request the client's open-order snapshot or typed read-only refusal through the public API",
		runAPI:      runAPIOpenOrdersClient,
	},
	"open_orders_all": {
		metadata:    meta("orders", []string{"Orders().Open"}, []int{protocol.OutReqAllOpenOrders, protocol.InOpenOrder, protocol.InOpenOrderEnd}, "read_only", []string{"client_id_0"}, []string{"all open-orders snapshot"}, 0, "promoted", batchReadOnly),
		description: "request the all-client open-order snapshot through the public API",
		runAPI:      runAPIOpenOrdersAll,
	},

	// --- Executions ---

	"executions_snapshot": {
		metadata:    meta("orders", []string{"Orders().Executions"}, []int{protocol.OutReqExecutions, protocol.InExecutionData, protocol.InExecutionDataEnd, protocol.InCommissionReport}, "read_only", nil, []string{"finite execution query and commissions when present"}, 1, "promoted", batchReadOnly),
		description: "request an unfiltered execution snapshot through the public API",
		runAPI:      runAPIExecutionsSnapshot,
	},

	// --- v1 expanded scope: Batch C1 — singleton one-shots (no reqID) ---

	"family_codes": {
		metadata:    meta("accounts", []string{"Accounts().FamilyCodes"}, []int{80, 78}, "read_only", nil, []string{"family codes response"}, 1, "promoted", batchReadOnly),
		description: "request and decode account family codes through the public API",
		runAPI:      runAPIFamilyCodes,
	},
	"news_providers": {
		metadata:    meta("news", []string{"News().Providers"}, []int{85}, "read_only", nil, []string{"subscribed news provider list"}, 1, "promoted", batchReadOnly),
		description: "request and decode subscribed news providers through the public API",
		runAPI:      runAPINewsProviders,
	},
	"mkt_depth_exchanges": {
		metadata:    meta("contracts", []string{"Contracts().DepthExchanges"}, []int{protocol.OutReqMktDepthExchanges, protocol.InMktDepthExchanges}, "read_only", nil, []string{"market-depth exchange metadata"}, 1, "promoted", batchReadOnly),
		description: "request and decode market-depth exchanges through the public API",
		runAPI:      runAPIDepthExchanges,
	},
	"scanner_parameters": {
		metadata:    meta("scanner", []string{"Scanner().Parameters"}, []int{24, 19}, "read_only", nil, []string{"scanner XML parameters"}, 1, "promoted", batchReadOnly),
		description: "request and receive scanner parameter XML through the public API",
		runAPI:      runAPIScannerParameters,
	},

	// --- v1 expanded scope: Batch C2 — keyed one-shots ---

	"user_info": {
		metadata:    meta("tws", []string{"TWS().UserInfo"}, []int{protocol.OutReqUserInfo, protocol.InUserInfo}, "read_only", nil, []string{"user info response"}, 1, "promoted", batchReadOnly),
		description: "request and decode TWS user information through the public API",
		runAPI:      runAPIUserInfo,
	},
	"matching_symbols_aapl": {
		metadata:    meta("contracts", []string{"Contracts().Search"}, []int{protocol.OutReqMatchingSymbols, protocol.InSymbolSamples}, "read_only", nil, []string{"exact-ish symbol samples"}, 1, "promoted", batchReadOnly),
		description: "search for AAPL contracts through the public API",
		runAPI:      runAPIMatchingSymbolsAAPL,
	},
	"matching_symbols_partial": {
		metadata:    meta("contracts", []string{"Contracts().Search"}, []int{protocol.OutReqMatchingSymbols, protocol.InSymbolSamples}, "read_only", nil, []string{"broad symbol samples"}, 1, "promoted", batchReadOnly),
		description: "search a broad AA symbol pattern through the public API",
		runAPI:      runAPIMatchingSymbolsPartial,
	},
	"head_timestamp_aapl": {
		metadata:    meta("history", []string{"History().HeadTimestamp"}, []int{protocol.OutReqHeadTimestamp, protocol.InHeadTimestamp}, "read_only", []string{"historical_data"}, []string{"nonzero earliest AAPL trade timestamp"}, 1, "promoted", batchReadOnly),
		description: "request and decode AAPL's earliest trade timestamp through the public API",
		runAPI:      runAPIHeadTimestampAAPL,
	},
	"sec_def_opt_params_aapl": {
		metadata:    meta("contracts", []string{"Contracts().SecDefOptParams"}, []int{protocol.OutReqSecDefOptParams, protocol.InSecDefOptParams, protocol.InSecDefOptParamsEnd}, "read_only", nil, []string{"option parameter surface"}, 1, "promoted", batchReadOnly),
		description: "request and decode AAPL option-chain parameters through the public API",
		runAPI:      runAPISecDefOptParamsAAPL,
	},
	"histogram_data_aapl": {
		metadata:    meta("history", []string{"History().Histogram"}, []int{protocol.OutReqHistogramData, protocol.InHistogramData}, "read_only", []string{"historical_data"}, []string{"nonempty one-week AAPL histogram"}, 1, "promoted", batchReadOnly),
		description: "request and decode a one-week AAPL price histogram through the public API",
		runAPI:      runAPIHistogramAAPL,
	},
	"historical_ticks_aapl_trades": {
		metadata:    meta("history", []string{"History().Ticks"}, []int{protocol.OutReqHistoricalTicks, protocol.InHistoricalTicksLast, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty historical trades or exact permission error"}, 1, "promoted", batchReadOnly),
		description: "request recent AAPL trade ticks or the typed live permission error through the public API",
		runAPI:      runAPIHistoricalTicksTrades,
	},
	"historical_ticks_aapl_bidask": {
		metadata:    meta("history", []string{"History().Ticks"}, []int{protocol.OutReqHistoricalTicks, protocol.InHistoricalTicksBidAsk, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty historical bid/ask ticks or exact permission error"}, 1, "promoted", batchReadOnly),
		description: "request recent AAPL bid/ask ticks or the typed live permission error through the public API",
		runAPI:      runAPIHistoricalTicksBidAsk,
	},
	"historical_ticks_aapl_midpoint": {
		metadata:    meta("history", []string{"History().Ticks"}, []int{protocol.OutReqHistoricalTicks, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty historical midpoint ticks or exact permission error"}, 1, "promoted", batchReadOnly),
		description: "request recent AAPL midpoint ticks or the typed live permission error through the public API",
		runAPI:      runAPIHistoricalTicksMidpoint,
	},
	"historical_news_aapl": {
		metadata:    meta("news", []string{"News().Historical"}, []int{protocol.OutReqHistoricalNews, protocol.InHistoricalNewsEnd}, "read_only", []string{"news_or_historical_news"}, []string{"nonempty historical news snapshot"}, 1, "promoted", batchReadOnly),
		description: "request recent AAPL historical news through the public API",
		runAPI:      runAPIHistoricalNewsAAPL,
	},
	"historical_ticks_aapl_timezone_start": {
		metadata:    meta("history", []string{"History().Ticks"}, []int{protocol.OutReqHistoricalTicks, protocol.InHistoricalTicksBidAsk, protocol.InHistoricalTicksLast, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"explicit UTC start-bound ticks for all kinds or exact permission errors"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request AAPL trade, bid/ask, and midpoint ticks from an explicit UTC start bound through the public API",
		runAPI:      runAPIHistoricalTicksStartBound,
	},
	"historical_news_aapl_timezone_window": {
		metadata:    meta("news", []string{"News().Historical"}, []int{protocol.OutReqHistoricalNews, protocol.InHistoricalNewsEnd}, "read_only", []string{"news_or_historical_news"}, []string{"nonempty historical news at or after an explicit UTC lower bound"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request AAPL historical news with an explicit UTC lower end bound through the public API",
		runAPI:      runAPIHistoricalNewsAAPLTimezoneWindow,
	},

	// --- v1 expanded scope: Batch C3 — completed orders and tick types ---

	"completed_orders": {
		metadata:    meta("orders", []string{"Orders().Completed", "Client.CurrentTime"}, []int{protocol.OutReqCompletedOrders, protocol.InCompletedOrder, protocol.InCompletedOrderEnd, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", nil, []string{"finite apiOnly completed-order snapshot and protocol fence"}, 1, "promoted", batchReadOnly),
		description: "collect the apiOnly completed-order snapshot through the public API",
		runAPI:      runAPICompletedOrders,
	},
	"tick_efp_probe": {
		metadata:    meta("market_data", []string{"official reqMktData EFP BAG"}, []int{1, 2, 4, 58, 59, 81}, "entitlement_probe", []string{"live_market_data", "active_single_stock_future", "matching_stock"}, []string{"TickEFP callback or real contract, entitlement, or no-data result"}, 1, "candidate", batchReadOnly),
		description: "live EFP market-data probe using DTE/EUREX and Tencent/HKFE single-stock-future BAGs",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 1); err != nil {
				return err
			}
			dteReqID := nextReqID()
			if err := sendReqEFPMarketData(conn, dteReqID,
				contractSpec{Symbol: "DTE", SecType: "BAG", Exchange: "SMART", Currency: "EUR"},
				[]comboLegSpec{
					{ConID: 667336572, Ratio: 1, Action: "BUY", Exchange: "EUREX"},
					{ConID: 2254332, Ratio: 100, Action: "SELL", Exchange: "SMART"},
				}); err != nil {
				return err
			}
			tencentReqID := nextReqID()
			if err := sendReqEFPMarketData(conn, tencentReqID,
				contractSpec{Symbol: "700", SecType: "BAG", Exchange: "SMART", Currency: "HKD"},
				[]comboLegSpec{
					{ConID: 842557048, Ratio: 1, Action: "BUY", Exchange: "HKFE"},
					{ConID: 152791428, Ratio: 100, Action: "SELL", Exchange: "SEHK"},
				}); err != nil {
				return err
			}

			if err := readFrames(conn, 20*time.Second, logFrame, stopOnMsgID(47)); err != nil {
				return err
			}
			if err := sendCancelMktData(conn, dteReqID); err != nil {
				return err
			}
			if err := sendCancelMktData(conn, tencentReqID); err != nil {
				return err
			}
			return readFrames(conn, time.Second, logFrame, nil)
		},
	},
	"quote_stream_multi_asset": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InTickPrice, protocol.InTickSize, protocol.InTickGeneric, protocol.InTickString}, "read_only", []string{"market_data_or_delayed_data"}, []string{"concurrent stock and FX quote streams with real price or size evidence and fenced cancellation"}, 1, "candidate", batchNewV2, batchReadOnly),
		description: "observe concurrent delayed AAPL and EUR.USD quote streams through the public API",
		runAPI:      runAPIQuoteStreamMultiAsset,
	},

	// --- v1 expanded scope: Batch C4 — streaming subscriptions ---

	"account_updates": {
		metadata:    meta("accounts", []string{"Accounts().Updates", "Client.CurrentTime"}, []int{protocol.OutReqAccountUpdates, protocol.InUpdateAccountValue, protocol.InUpdatePortfolio, protocol.InUpdateAccountTime, protocol.InAccountDownloadEnd, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", nil, []string{"nonempty typed account and portfolio snapshot with protocol-fenced unsubscribe"}, 1, "promoted", batchReadOnly),
		description: "collect and close the account updates snapshot through the public API",
		runAPI:      runAPIAccountUpdates,
	},
	"account_updates_multi": {
		metadata:    meta("accounts", []string{"Accounts().UpdatesMulti", "Client.CurrentTime"}, []int{protocol.OutReqAccountUpdatesMulti, protocol.InAccountUpdateMulti, protocol.InAccountUpdateMultiEnd, protocol.OutCancelAccountUpdatesMulti, protocol.OutReqCurrentTime}, "read_only", nil, []string{"nonempty multi-account update snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "collect and close a multi-account update snapshot through the public API",
		runAPI:      runAPIAccountUpdatesMulti,
	},
	"positions_multi": {
		metadata:    meta("accounts", []string{"Accounts().PositionsMulti", "Client.CurrentTime"}, []int{protocol.OutReqPositionsMulti, protocol.InPositionMulti, protocol.InPositionMultiEnd, protocol.OutCancelPositionsMulti, protocol.OutReqCurrentTime}, "read_only", nil, []string{"completed multi-account positions snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "collect and close a multi-account positions snapshot through the public API",
		runAPI:      runAPIPositionsMulti,
	},
	"pnl": {
		metadata:    meta("accounts", []string{"Accounts().SubscribePnL", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqPnL, protocol.InPnL, protocol.OutCancelPnL, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "read_only", nil, []string{"typed account PnL update, including an all-zero snapshot, followed by fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "observe and close the account PnL stream through the public API",
		runAPI:      runAPIPnL,
	},
	"pnl_single": {
		metadata:    meta("accounts", []string{"Accounts().Updates", "Accounts().SubscribePnLSingle", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqAccountUpdates, protocol.InUpdatePortfolio, protocol.InAccountDownloadEnd, protocol.OutReqPnLSingle, protocol.InPnLSingle, protocol.OutCancelPnLSingle, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", nil, []string{"typed PnL update for a real held contract followed by fenced cancellation"}, 1, "candidate", batchReadOnly),
		description: "derive a held contract and observe its single-position PnL through the public API",
		runAPI:      runAPIPnLSingle,
	},
	"tick_by_tick_last": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeTickByTick", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqTickByTickData, protocol.OutCancelTickByTickData, protocol.OutReqCurrentTime, protocol.InTickByTick, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed Last tick and fenced cancellation or exact live entitlement refusal"}, 1, "candidate", batchReadOnly),
		description: "observe an AAPL Last tick or the exact live entitlement refusal through the public API",
		runAPI:      runAPITickByTickLastAAPL,
	},
	"tick_by_tick_bidask": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeTickByTick", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqTickByTickData, protocol.OutCancelTickByTickData, protocol.OutReqCurrentTime, protocol.InTickByTick, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed BidAsk tick and fenced cancellation or exact live entitlement refusal"}, 1, "candidate", batchReadOnly),
		description: "observe an AAPL BidAsk tick or the exact live entitlement refusal through the public API",
		runAPI:      runAPITickByTickBidAskAAPL,
	},
	"tick_by_tick_midpoint": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeTickByTick", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqTickByTickData, protocol.OutCancelTickByTickData, protocol.OutReqCurrentTime, protocol.InTickByTick, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed MidPoint tick and fenced cancellation or exact live entitlement refusal"}, 1, "candidate", batchReadOnly),
		description: "observe an AAPL MidPoint tick or the exact live entitlement refusal through the public API",
		runAPI:      runAPITickByTickMidPointAAPL,
	},
	"historical_bars_keepup": {
		metadata:    meta("history", []string{"History().SubscribeBars", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalData, protocol.InHistoricalDataEnd, protocol.InHistoricalDataUpdate, protocol.OutCancelHistoricalData, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty initial one-minute bar snapshot and fenced cancellation; a real streaming update remains a market-hours target"}, 1, "candidate", batchReadOnly),
		description: "collect the initial AAPL keep-up bar snapshot through the public API and close the stream",
		runAPI:      runAPIHistoricalBarsKeepUp,
	},
	"news_bulletins": {
		metadata:    meta("news", []string{"News().SubscribeBulletins", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqNewsBulletins, protocol.InNewsBulletins, protocol.OutCancelNewsBulletins, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "read_only", []string{"news_or_bulletins"}, []string{"bounded typed bulletin observation, including a valid empty window, followed by fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "observe the live bulletin stream through the public API and close it cleanly",
		runAPI:      runAPINewsBulletins,
	},

	// --- v1 expanded scope: Batch C5 — option calculations and scanner ---
	"smart_components": {
		metadata:    meta("contracts", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Contracts().SmartComponents"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.InTickReqParams, protocol.OutReqSmartComponents, protocol.InSmartComponents, protocol.OutCancelMktData}, "read_only", []string{"market_hours", "market_data_or_delayed_data"}, []string{"quote-derived smart component mapping"}, 1, "promoted", batchReadOnly, batchExhaustiveMarketHours),
		description: "derive AAPL's BBO mapping from quote parameters and decode its SMART components through the public API",
		runAPI:      runAPISmartComponents,
	},
	"market_rule": {
		metadata:    meta("contracts", []string{"Contracts().MarketRule"}, []int{91, 93}, "read_only", nil, []string{"price increment ladder"}, 1, "promoted", batchReadOnly),
		description: "request and decode US equity market rule 26 through the public API",
		runAPI:      runAPIMarketRule,
	},

	// --- Order management ---

	"place_order_lmt_buy_aapl": {
		metadata:    meta("orders", []string{"Orders().Place", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"far-from-market order accepted then cancelled"}, 1, "promoted", batchNewV2, batchTrading),
		description: "PLACE_ORDER LMT buy 1 AAPL at $50 (far below market), observe status, then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_mkt_buy_aapl": {
		metadata:    meta("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_marketable_order", []string{"paper_trading", "market_hours"}, []string{"market buy fill or real market-state response"}, 1, "promoted", batchNewV2, batchTrading),
		description: "PLACE_ORDER MKT buy 1 AAPL (will fill), observe status",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"place_order_mkt_sell_aapl": {
		metadata:    meta("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_marketable_order", []string{"paper_trading", "market_hours", "position_or_short_permission"}, []string{"market sell fill or real rejection"}, 1, "promoted", batchNewV2, batchTrading),
		description: "PLACE_ORDER MKT sell 1 AAPL, observe status",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "SELL", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"place_order_modify": {
		metadata:    meta("orders", []string{"Orders().Place", "OrderHandle.Modify", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"modify accepted and open-order update observed"}, 1, "promoted", batchNewV2, batchTrading),
		description: "PLACE_ORDER LMT buy 1 AAPL at $50, modify to $51, then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			// Modify: send PlaceOrder again with same orderID but new price.
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "51.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_cancel": {
		metadata:    meta("orders", []string{"Orders().Place", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"cancel terminal status"}, 1, "promoted", batchNewV2, batchTrading),
		description: "PLACE_ORDER LMT buy 1 AAPL at $50, then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 2*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_direct_cancel": {
		metadata:    meta("orders", []string{"Orders().Place", "Orders().Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"direct Orders().Cancel(orderID) terminal status"}, 1, "candidate", batchNewV2, batchTrading),
		description: "PLACE_ORDER LMT buy 1 AAPL at $50, then cancel via Orders().Cancel(orderID)",
		// Wire-identical to place_order_cancel. The scenario exists so the
		// replay transcript and integration test can exercise the direct-by-ID
		// public facade path, which is conceptually different from the
		// OrderHandle.Cancel flow even though both emit OutCancelOrder=4.
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 2*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_bracket_aapl": {
		metadata:    meta("orders", []string{"Orders().Place"}, []int{3, 5}, "paper_marketable_order", []string{"paper_trading", "market_hours"}, []string{"parent/child transmit sequencing"}, 1, "candidate", batchNewV2, batchTrading),
		description: "bracket order: MKT parent + LMT take-profit + STP stop-loss for AAPL",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			parentID := sess.NextValidID
			tpID := sess.NextValidID + 1
			slID := sess.NextValidID + 2
			sess.NextValidID += 3
			acct := sess.ManagedAccounts
			aaplSTK := contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}

			// Parent: MKT buy, transmit=false (held until children submitted).
			if err := sendPlaceOrder(conn, parentID, aaplSTK, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, Transmit: false}); err != nil {
				return err
			}
			// Take-profit: LMT sell at $300 (far above market), transmit=false.
			if err := sendPlaceOrder(conn, tpID, aaplSTK, orderSpec{Action: "SELL", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "300.00", TIF: "GTC", Account: acct, ParentID: parentID, Transmit: false}); err != nil {
				return err
			}
			// Stop-loss: STP sell at $50 (far below market), transmit=true (triggers all 3).
			if err := sendPlaceOrder(conn, slID, aaplSTK, orderSpec{Action: "SELL", TotalQuantity: "1", OrderType: "STP", AuxPrice: "50.00", TIF: "GTC", Account: acct, ParentID: parentID, Transmit: true}); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, nil)
		},
	},
	"global_cancel": {
		metadata:    meta("orders", []string{"Orders().Place", "Orders().CancelAll"}, []int{3, 5, 58}, "paper_order", []string{"paper_trading"}, []string{"multiple open orders cancelled globally"}, 1, "promoted", batchNewV2, batchTrading),
		description: "place 3 LMT buy orders at $50, then GLOBAL_CANCEL",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			acct := sess.ManagedAccounts
			aaplSTK := contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}
			for i := 0; i < 3; i++ {
				orderID := sess.NextValidID
				sess.NextValidID++
				if err := sendPlaceOrder(conn, orderID, aaplSTK, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
					return err
				}
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendGlobalCancel(conn); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"place_order_option_buy": {
		metadata:    meta("options", []string{"Orders().Place"}, []int{3, 5}, "paper_marketable_order", []string{"paper_trading", "option_permissions"}, []string{"option order fill or real contract/permission error"}, 1, "candidate", batchNewV2, batchTrading),
		description: "PLACE_ORDER buy 1 AAPL far-OTM call option, observe status",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			// Hardcoded far-OTM AAPL call. The exact contract may need adjustment
			// for current market conditions; the capture will show any error response.
			optContract := contractSpec{
				Symbol:   "AAPL",
				SecType:  "OPT",
				Exchange: "SMART",
				Currency: "USD",
				Right:    "C",
				Strike:   300.0,
				// Use a distant expiry to maximize chance of existence.
				LastTradeDateOrContractMonth: "20261218",
				Multiplier:                   "100",
			}
			if err := sendPlaceOrder(conn, orderID, optContract, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"place_order_algo_adaptive_aapl": {
		metadata:    meta("orders", []string{"Orders().Place"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"Adaptive algo open-order wire"}, 1, "candidate", batchNewV2, batchTrading),
		description: "PLACE_ORDER LMT buy 1 AAPL with Adaptive algo, observe open-order wire",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{
				Action:        "BUY",
				TotalQuantity: "1",
				OrderType:     "LMT",
				LmtPrice:      "50.00",
				TIF:           "DAY",
				Account:       acct,
				Transmit:      true,
				AlgoStrategy:  "Adaptive",
				AlgoParams:    []tagValueSpec{{Tag: "adaptivePriority", Value: "Normal"}},
			}); err != nil {
				return err
			}
			if err := readFrames(conn, 4*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_price_condition_aapl": {
		metadata:    meta("orders", []string{"Orders().Place"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"price condition open-order wire"}, 1, "candidate", batchNewV2, batchTrading),
		description: "PLACE_ORDER LMT buy 1 AAPL with a high price condition so it stays inactive",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{
				Action:        "BUY",
				TotalQuantity: "1",
				OrderType:     "LMT",
				LmtPrice:      "50.00",
				TIF:           "DAY",
				Account:       acct,
				Transmit:      true,
				Conditions: []orderConditionSpec{{
					Type:          1,
					Conjunction:   "a",
					Operator:      2,
					ConID:         265598,
					Exchange:      "SMART",
					Value:         "9999.00",
					TriggerMethod: 4,
				}},
				ConditionsIgnoreRTH:   false,
				ConditionsCancelOrder: false,
			}); err != nil {
				return err
			}
			if err := readFrames(conn, 4*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_oca_pair_aapl": {
		metadata:    meta("orders", []string{"Orders().Place", "Orders().CancelAll"}, []int{3, 5, 58}, "paper_order", []string{"paper_trading"}, []string{"OCA pair accepted then globally cancelled"}, 1, "candidate", batchNewV2, batchTrading),
		description: "place two far-from-market AAPL LMT orders in one OCA group, then GLOBAL_CANCEL",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			acct := sess.ManagedAccounts
			aaplSTK := contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}
			oca := "ibkr-go-live-oca-" + strconv.FormatInt(time.Now().Unix(), 10)
			for i, side := range []string{"BUY", "SELL"} {
				orderID := sess.NextValidID
				sess.NextValidID++
				price := "50.00"
				if side == "SELL" {
					price = "500.00"
				}
				if err := sendPlaceOrder(conn, orderID, aaplSTK, orderSpec{Action: side, TotalQuantity: "1", OrderType: "LMT", LmtPrice: price, TIF: "DAY", Account: acct, OcaGroup: oca, OrderRef: fmt.Sprintf("ibkr-go-oca-%d", i), Transmit: true}); err != nil {
					return err
				}
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendGlobalCancel(conn); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"trading_split_round_trip_aapl": {
		metadata:    meta("orders", []string{"Accounts().Summary", "Accounts().Positions", "Orders().Place", "Orders().Executions", "Orders().Completed", "Accounts().SubscribePnL"}, []int{3, 5, 7, 11, 55, 59, 61, 62, 63, 64, 92, 93, 99, 101, 102}, "paper_marketable_order", []string{"paper_trading", "market_hours"}, []string{"split buy/sell round trip with account/order reconciliation"}, 1, "candidate", batchNewV2, batchTrading),
		description: "account baseline, split AAPL market buys, executions/completed orders, split sells, final account/position/PnL probes",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			acct := sess.ManagedAccounts
			aaplSTK := contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}
			accountReqID := nextReqID()
			if err := sendReqAccountSummary(conn, accountReqID, "All", "NetLiquidation,TotalCashValue,BuyingPower,ExcessLiquidity"); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelAccountSummary(conn, accountReqID); err != nil {
				return err
			}
			if err := sendReqPositions(conn); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, stopOnMsgID(62)); err != nil {
				return err
			}

			for i := 0; i < 2; i++ {
				orderID := sess.NextValidID
				sess.NextValidID++
				if err := sendPlaceOrder(conn, orderID, aaplSTK, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, OrderRef: fmt.Sprintf("ibkr-go-split-buy-%d", i), Transmit: true}); err != nil {
					return err
				}
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}

			execReqID := nextReqID()
			if err := sendReqExecutions(conn, execReqID); err != nil {
				return err
			}
			if err := readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(55, strconv.Itoa(execReqID), 1)); err != nil {
				return err
			}
			if err := sendReqCompletedOrders(conn, true); err != nil {
				return err
			}
			if err := readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(102)); err != nil {
				return err
			}

			for i := 0; i < 2; i++ {
				orderID := sess.NextValidID
				sess.NextValidID++
				if err := sendPlaceOrder(conn, orderID, aaplSTK, orderSpec{Action: "SELL", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, OrderRef: fmt.Sprintf("ibkr-go-split-sell-%d", i), Transmit: true}); err != nil {
					return err
				}
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendReqPositions(conn); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, stopOnMsgID(62)); err != nil {
				return err
			}
			pnlReqID := nextReqID()
			if err := sendReqPnL(conn, pnlReqID, acct, ""); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelPnL(conn, pnlReqID); err != nil {
				return err
			}
			if err := sendGlobalCancel(conn); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},

	// --- Market depth ---

	"market_depth_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SubscribeDepth", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMktDepth, protocol.OutCancelMktDepth, protocol.OutReqCurrentTime, protocol.InMarketDepth, protocol.InMarketDepthL2, protocol.InCurrentTime, protocol.InErrMsg}, "entitlement_probe", []string{"l2_market_data_or_error"}, []string{"typed regular depth row with fenced cancellation, or an exact live refusal"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "observe an AAPL regular depth row or an exact live refusal through the public API",
		runAPI:      runAPIMarketDepthAAPL,
	},
	"market_depth_aapl_smart": {
		metadata:    meta("market_data", []string{"MarketData().SubscribeDepth", "Client.SessionEvents", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMktDepth, protocol.OutCancelMktDepth, protocol.OutReqCurrentTime, protocol.InMarketDepth, protocol.InMarketDepthL2, protocol.InCurrentTime, protocol.InErrMsg}, "entitlement_probe", []string{"l2_market_data_or_error"}, []string{"typed smart depth row or exact no-available-depth notice, followed by fenced cancellation"}, 1, "candidate", batchNewV2, batchReadOnly),
		description: "observe an AAPL SMART depth row or the exact live no-available-depth notice through the public API",
		runAPI:      runAPIMarketDepthSmartAAPL,
	},

	// --- Reference data ---

	"soft_dollar_tiers": {
		metadata:    meta("advisors", []string{"Advisors().SoftDollarTiers"}, []int{79, 77}, "read_only", nil, []string{"soft-dollar tier list"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request and decode soft-dollar tiers through the public API",
		runAPI:      runAPISoftDollarTiers,
	},
	"display_groups": {
		metadata:    meta("tws", []string{"TWS().DisplayGroups"}, []int{67}, "read_only", nil, []string{"display group list"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request and decode TWS display groups through the public API",
		runAPI:      runAPIDisplayGroups,
	},
	"display_group_subscribe": {
		metadata:    meta("tws", []string{"TWS().DisplayGroups", "TWS().SubscribeDisplayGroup", "DisplayGroupHandle.Update", "Client.CurrentTime"}, []int{protocol.OutQueryDisplayGroups, protocol.InDisplayGroupList, protocol.OutSubscribeToGroupEvents, protocol.InDisplayGroupUpdated, protocol.OutUpdateDisplayGroup, protocol.OutUnsubscribeFromGroupEvents, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "read_only", []string{"tws_display_groups"}, []string{"typed display group query, subscribe, state-preserving update when possible, unsubscribe"}, 1, "candidate", batchNewV2, batchReadOnly),
		description: "query real display group IDs and exercise the public subscription lifecycle",
		runAPI:      runAPIDisplayGroupSubscription,
	},
	"wsh_meta_data": {
		metadata:    meta("wsh", []string{"WSH().MetaData"}, []int{protocol.OutReqWSHMetaData, protocol.InWSHMetaData, protocol.InErrMsg}, "entitlement_probe", []string{"wsh_subscription_or_error"}, []string{"valid WSH metadata JSON or exact entitlement error"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request WSH metadata or its typed entitlement refusal through the public API",
		runAPI:      runAPIWSHMetaData,
	},
	"wsh_event_data_aapl": {
		metadata:    meta("wsh", []string{"WSH().EventData"}, []int{protocol.OutReqWSHEventData, protocol.InWSHEventData, protocol.InErrMsg}, "entitlement_probe", []string{"wsh_subscription_or_error"}, []string{"valid WSH event JSON or exact entitlement error"}, 1, "candidate", batchNewV2, batchReadOnly),
		description: "request AAPL WSH events or their typed entitlement refusal through the public API",
		runAPI:      runAPIWSHEventDataAAPL,
	},
	"request_fa": {
		metadata:    meta("advisors", []string{"Advisors().Config", "Client.CurrentTime"}, []int{protocol.OutRequestFA, protocol.InReceiveFA, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "entitlement_probe", []string{"fa_account_or_error"}, []string{"typed FA groups XML or exact non-FA refusal"}, 1, "candidate", batchNewV2, batchReadOnly),
		description: "request the FA groups document through the public API",
		runAPI:      runAPIFAConfigGroups,
	},
	"qualify_contract_aapl_exact": {
		metadata:    meta("contracts", []string{"Contracts().Qualify"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"single qualified contract"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "qualify an unpinned AAPL stock contract through the public API",
		runAPI:      runAPIQualifyContractAAPL,
	},
	"qualify_contract_ambiguous": {
		metadata:    meta("contracts", []string{"Contracts().Qualify"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"ErrAmbiguousContract from multiple matches"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "require ambiguous qualification for MSFT stock without an exchange through the public API",
		runAPI:      runAPIQualifyContractAmbiguous,
	},
	"api_order_type_matrix_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Modify", "OrderHandle.Cancel", "Orders().Executions"}, []int{1, 2, 3, 4, 5, 11, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"MKT/LMT/STP/STP LMT/TRAIL/TRAIL LIMIT/MIT/LIT/MTL/REL/MOC/LOC/MOO/LOO/PEG families accepted, rejected, filled, modified, or cancelled with real order lifecycle"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchExhaustivePremarket, batchReplayDefault, batchReplayAll),
		description: "public API campaign for AAPL order type breadth: fills, rests, rejects, modifies, and cancels",
		runAPI:      runAPIOrderTypeMatrixAAPL,
	},
	"api_order_fill_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Modify", "Orders().Executions"}, []int{1, 2, 3, 5, 11, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"MKT, MTL, and delayed modify-to-market fill paths with flattening"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for AAPL fill paths: MKT, MTL, and delayed modify-to-market",
		runAPI:      runAPIOrderFillAAPL,
	},
	"api_order_rest_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"far LMT rest/cancel path"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for AAPL resting order types and cancel/reject behavior",
		runAPI:      runAPIOrderRestCancelAAPL,
	},
	"api_order_direct_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Cancel", "Client.CurrentTime"}, []int{protocol.OutPlaceOrder, protocol.InOpenOrder, protocol.InOrderStatus, protocol.OutCancelOrder, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "paper_order", []string{"paper_trading"}, []string{"same-client top-level direct cancel reaches typed terminal cancellation"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "place a resting AAPL limit order and cancel it through Orders().Cancel",
		runAPI:      runAPIOrderDirectCancelAAPL,
	},
	"api_bracket_place_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().PlaceBracket", "Orders().CancelAll", "Orders().Open", "Client.CurrentTime"}, []int{protocol.OutPlaceOrder, protocol.InOpenOrder, protocol.InOrderStatus, protocol.OutReqGlobalCancel, protocol.OutReqOpenOrders, protocol.InOpenOrderEnd, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "paper_order", []string{"paper_trading"}, []string{"direct PlaceBracket allocates consecutive IDs, binds child parent IDs, stages false/false/true transmit frames, and cleans every leg"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "place and clean up a nonmarketable AAPL bracket through Orders().PlaceBracket",
		runAPI:      runAPIBracketPlaceAAPL,
	},
	"api_order_relative_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"REL rest/cancel behavior isolated because Gateway can reconnect during relative order validation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for AAPL relative order behavior",
		runAPI:      runAPIOrderRelativeCancelAAPL,
	},
	"api_order_trailing_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"TRAIL and TRAIL LIMIT behavior isolated because Gateway can reconnect during trailing validation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for AAPL trailing and trailing-limit behavior",
		runAPI:      runAPIOrderTrailingCancelAAPL,
	},
	"api_order_stop_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"STP and STP LMT rest/cancel behavior isolated because Gateway can reconnect during stop validation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for AAPL stop and stop-limit rest/cancel behavior",
		runAPI:      runAPIOrderStopCancelAAPL,
	},
	"api_order_rejects_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "Orders().Cancel"}, []int{1, 2, 3, 4, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"invalid order type, price band, invalid contract, and unknown cancel real Gateway errors"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for AAPL order rejection and unknown cancel behavior",
		runAPI:      runAPIOrderRejectsAAPL,
	},
	"api_delayed_success_modify_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Modify", "Orders().Executions", "Accounts().Positions"}, []int{3, 4, 5, 11, 59, 61, 62}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"resting limit order later becomes marketable through modify and is observed through the original handle"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign where a resting AAPL limit order succeeds later through OrderHandle.Modify",
		runAPI:      runAPIDelayedSuccessModifyAAPL,
	},
	"api_bracket_trigger_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Modify", "Orders().Open", "Orders().Executions", "Orders().CancelAll"}, []int{3, 4, 5, 11, 16, 53, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"bracket parent fills, children echo the same OCA group, forced take-profit modify reaches real price-band cancellation/rejection"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for bracket parent/child activation and take-profit-trigger sibling cancellation",
		runAPI:      runAPIBracketTriggerAAPL,
	},
	"api_oca_trigger_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Open", "Orders().Executions", "Orders().CancelAll"}, []int{3, 4, 5, 11, 16, 53, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"OCA group echoed on both peers; aggressive peer reaches real price-band cancellation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for OCA fill/cancel behavior",
		runAPI:      runAPIOCATriggerAAPL,
	},
	"api_conditions_matrix_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Cancel", "Orders().CancelAll"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"price/time/margin/execution/volume/percent-change condition families accepted or rejected with real Gateway response"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for IBKR order condition families",
		runAPI:      runAPIConditionsMatrixAAPL,
	},
	"api_tif_attribute_matrix_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Cancel", "Orders().Executions"}, []int{3, 4, 5, 7, 11, 55}, "paper_order", []string{"paper_trading"}, []string{"GTC/GTD/GoodAfterTime/AON/MinQty/TrailingPercent/PercentOffset/Scale/Adjusted/ManualOrderTime/AdvancedErrorOverride accepted or rejected with real Gateway response"}, 1, "promoted", []string{"STK"}, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for TIF values and advanced AAPL order attributes",
		runAPI:      runAPITIFAttributeMatrixAAPL,
	},
	"api_security_type_probe_matrix": {
		metadata:    metaWithAssets("contracts", []string{"Contracts().Details"}, []int{9, 10, 52, 4}, "entitlement_probe", []string{"security_type_permissions_or_real_error"}, []string{"contract details or real rejection for STK/OPT/FUT/FOP/CASH/BOND/CFD/WAR/IND/CRYPTO/FUND/BILL/CMDTY/CONTFUT"}, 1, "candidate", []string{"STK", "OPT", "FUT", "FOP", "CASH", "BOND", "CFD", "WAR", "IND", "CRYPTO", "FUND", "BILL", "CMDTY", "CONTFUT"}, batchNewV2, batchReplayAll),
		description: "public API probe matrix for real Gateway contract-details behavior across security types",
		runAPI:      runAPISecurityTypeProbeMatrix,
	},
	"api_market_data_completeness_aapl": {
		metadata:    metaWithAssets("market_data", []string{"MarketData().SetType", "MarketData().Quote", "MarketData().SubscribeRealTimeBars", "MarketData().SubscribeTickByTick"}, []int{1, 2, 45, 46, 50, 51, 57, 58, 59, 97, 98, 99}, "entitlement_probe", []string{"market_data_or_delayed_data"}, []string{"market data type pushes, generic ticks, real-time TRADES/BID_ASK/MIDPOINT, and tick-by-tick variants or entitlement errors"}, 1, "candidate", []string{"STK"}, batchNewV2, batchReplayAll),
		description: "public API campaign for market-data type, generic tick, real-time bar, and tick-by-tick variants",
		runAPI:      runAPIMarketDataCompletenessAAPL,
	},
	"api_generic_tick_matrix_aapl": {
		metadata:    metaWithAssets("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes"}, []int{1, 2, 45, 46, 58, 59, 81}, "entitlement_probe", []string{"market_data_or_delayed_data"}, []string{"delayed AAPL stream preserves observed mark-price tick 37, shortable ticks 46/89, volume-rate tick 56, delayed timestamp tick 88, and omitted minimum-tick parameters"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReadOnly, batchReplayAll),
		description: "public API probe for exact price, size, generic, string, and parameter tick delivery",
		runAPI:      runAPIGenericTickMatrixAAPL,
	},
	"api_tick_news_aapl_probe": {
		metadata:    metaWithAssets("news", []string{"MarketData().SetType", "MarketData().SubscribeQuotes"}, []int{1, 2, 4, 58, 59, 81, 84}, "entitlement_probe", []string{"api_news_subscription"}, []string{"contract-specific BRFG TickNews or a real entitlement/no-new-headline result"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReadOnly, batchReplayAll),
		description: "public API probe for contract-specific BRFG news ticks",
		runAPI:      runAPITickNewsAAPLProbe,
	},
	"api_scanner_subscription": {
		metadata:    metaWithAssets("scanner", []string{"Scanner().SubscribeResults", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqScannerSubscription, protocol.OutCancelScannerSubscription, protocol.OutReqCurrentTime, protocol.InScannerData, protocol.InCurrentTime, protocol.InErrMsg}, "entitlement_probe", []string{"scanner_permissions_or_real_error"}, []string{"complete ranked or empty result with fenced cancellation, or an exact permission refusal"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReadOnly, batchReplayAll),
		description: "request a complete HOT_BY_VOLUME result, including a valid empty result, or the exact live permission refusal through the public API",
		runAPI:      runAPIScannerSubscription,
	},
	"api_historical_matrix_aapl": {
		metadata:    metaWithAssets("history", []string{"History().Bars"}, []int{20, 17, 4}, "read_only", []string{"historical_data"}, []string{"all planned historical bar-size probes and whatToShow variants return data or real Gateway errors"}, 1, "candidate", []string{"STK"}, batchNewV2, batchReplayAll),
		description: "public API campaign for historical bar-size and whatToShow variants",
		runAPI:      runAPIHistoricalMatrixAAPL,
	},
	"api_news_article_aapl": {
		metadata:    metaWithAssets("news", []string{"News().Historical", "News().Article"}, []int{84, 83, 86, 87, 80, 4}, "entitlement_probe", []string{"news_or_historical_news"}, []string{"article ID sourced from historical news is requested through reqNewsArticle or real entitlement/no-result is frozen"}, 1, "candidate", []string{"STK"}, batchNewV2, batchReplayAll),
		description: "public API campaign that requests a real news article ID from historical news, then fetches the article",
		runAPI:      runAPINewsArticleAAPL,
	},
	"api_wsh_variants_aapl": {
		metadata:    metaWithAssets("wsh", []string{"WSH().MetaData", "WSH().EventData"}, []int{100, 102, 4}, "entitlement_probe", []string{"wsh_subscription_or_error"}, []string{"WSH metadata plus conid, portfolio, watchlist, competitor, and date-window event-data variants return real code 10276 entitlement errors"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReplayAll),
		description: "public API probe for WSH metadata plus conid, portfolio, watchlist, competitor, and date-window event-data entitlement variants",
		runAPI:      runAPIWSHVariantsAAPL,
	},
	"api_algo_variants_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Cancel", "Orders().Executions"}, []int{3, 4, 5, 7, 11, 55}, "paper_order", []string{"paper_trading", "algo_permissions_or_real_error"}, []string{"Adaptive, TWAP, VWAP, ArrivalPx, DarkIce, AccumDist, Inline, Close, PctVol, BalanceImpactRisk, MinImpact, and Jefferies variants accepted, rejected, or cancelled with real Gateway response"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for available IBKR algorithmic strategy variants",
		runAPI:      runAPIAlgoVariantsAAPL,
	},
	"api_pairs_trading_aapl_msft": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"paired AAPL/MSFT market entries and per-symbol flatten fills; source execution-query tail timed out"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for paired AAPL/MSFT market orders and cleanup",
		runAPI:      runAPIPairsTradingAAPLMSFT,
	},
	"api_dollar_cost_averaging_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"three staged AAPL market buys plus aggregate flatten fill; source execution-query tail timed out"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for repeated AAPL buys and post-campaign flattening",
		runAPI:      runAPIDollarCostAveragingAAPL,
	},
	"api_stop_loss_management_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Modify", "OrderHandle.Cancel", "Orders().Executions"}, []int{3, 4, 5, 11, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"market entry, protective stop placement, stop modification, cancellation, flatten, and execution reconciliation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for placing, moving, cancelling, and flattening a protective stop",
		runAPI:      runAPIStopLossManagementAAPL,
	},
	"api_bracket_trailing_stop_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "Orders().CancelAll", "Orders().Executions"}, []int{1, 2, 3, 4, 7, 11, 55, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"live scenario probes quote, places market-parent bracket with TRAIL child, and receives code 328; promoted replay freezes request/rejection before execution-query and cleanup tail"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for bracket order sequencing with a trailing stop child",
		runAPI:      runAPIBracketTrailingStopAAPL,
	},
	"api_fa_replace_non_fa": {
		metadata:    metaWithAssets("advisors", []string{"Advisors().ReplaceConfig"}, []int{19}, "paper_order", []string{"paper_trading"}, []string{"real non-FA account response to FA group replacement"}, 1, "promoted", []string{"STK"}, batchTrading),
		description: "public API probe replacing FA groups on a non-FA paper account, freezing the real account-type response",
		runAPI:      runAPIFAReplaceNonFA,
	},
	"api_option_exercise_aapl": {
		metadata:    meta("options", []string{"Orders().Place", "Options().Exercise"}, []int{3, 5, 21}, "paper_marketable_order", []string{"paper_trading", "market_hours", "option_permissions"}, []string{"option fill then real exercise acknowledgement or no-position error"}, 1, "promoted", batchTrading),
		description: "public API campaign buying one AAPL call then exercising it, freezing the real exercise or no-position response",
		runAPI:      runAPIOptionExerciseAAPL,
	},
	"api_hedge_order_aapl": {
		metadata:    meta("orders", []string{"Orders().Place"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"hedge child accept or real rejection per hedge type"}, 1, "promoted", batchTrading),
		description: "public API campaign attaching delta, beta, FX, and pair hedge children to a staged parent",
		runAPI:      runAPIHedgeOrderAAPL,
	},
	"api_option_campaign_aapl": {
		metadata:    metaWithAssets("options", []string{"Contracts().SecDefOptParams", "Contracts().Qualify", "MarketData().Quote", "Options().Price", "Options().Exercise", "Orders().Place", "Orders().Executions", "Orders().Completed"}, []int{1, 2, 3, 5, 11, 21, 55, 59, 75, 76, 99, 101, 102}, "paper_trigger", []string{"paper_trading", "market_hours", "option_permissions"}, []string{"live-qualified AAPL option quote/calculation/order/exercise-or-real-reject campaign"}, 1, "candidate", []string{"OPT"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for live-qualified AAPL option data, order, execution, and exercise/lapse responses",
		runAPI:      runAPIOptionCampaignAAPL,
	},
	"api_option_calculations_aapl": {
		metadata:    metaWithAssets("options", []string{"Contracts().SecDefOptParams", "Contracts().Details", "MarketData().Quote", "Options().Price", "Options().ImpliedVolatility"}, []int{1, 2, 4, 10, 21, 52, 54, 55, 56, 57, 75, 76}, "read_only", []string{"option_permissions"}, []string{"live-qualified option price and implied-volatility results with field-presence sentinels"}, 1, "promoted", []string{"OPT"}, batchNewV2, batchReadOnly, batchReplayAll),
		description: "read-only public API probe for live-qualified AAPL option price and implied-volatility calculations",
		runAPI:      runAPIOptionCalculationsAAPL,
	},
	"api_future_campaign_mes": {
		metadata:    metaWithAssets("orders", []string{"Contracts().Details", "MarketData().Quote", "Orders().Place", "Orders().Executions", "Accounts().Positions", "Orders().CancelAll"}, []int{1, 2, 3, 5, 10, 11, 52, 57, 58, 59, 61, 62}, "paper_trigger", []string{"paper_trading", "market_hours", "future_permissions"}, []string{"live-qualified MES future order/modify/round-trip or real permission rejection"}, 1, "promoted", []string{"FUT"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for live-qualified MES futures order behavior",
		runAPI:      runAPIFutureCampaignMES,
	},
	"api_combo_option_vertical_aapl": {
		metadata:    metaWithAssets("orders", []string{"Contracts().SecDefOptParams", "Contracts().Qualify", "Orders().Place", "Orders().Open", "Orders().CancelAll"}, []int{3, 4, 5, 16, 53, 58, 75, 76}, "paper_order", []string{"paper_trading", "option_permissions"}, []string{"live-qualified AAPL option BAG vertical accepted/cancelled or real combo rejection"}, 1, "candidate", []string{"BAG", "OPT"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for live-qualified AAPL option vertical BAG order behavior",
		runAPI:      runAPIComboOptionVerticalAAPL,
	},
	"api_algorithmic_campaign_aapl": {
		metadata:    metaWithAssets("orders", []string{"Accounts().Summary", "Accounts().SubscribeUpdates", "Accounts().SubscribePnL", "Accounts().Positions", "MarketData().SubscribeQuotes", "Orders().SubscribeOpen", "Orders().Place", "OrderHandle.Modify", "Orders().Executions", "Orders().Completed", "Orders().CancelAll"}, []int{1, 2, 3, 5, 6, 7, 8, 11, 16, 53, 54, 58, 59, 61, 62, 63, 64, 92, 93, 99, 101, 102}, "paper_destructive", []string{"paper_trading", "market_hours"}, []string{"multi-subscription algorithmic campaign with split fills, resting modifies, reconciliation, and cleanup"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign with concurrent market/account/order observers and multi-step trading",
		runAPI:      runAPIAlgorithmicCampaignAAPL,
	},
	"api_completed_orders_variants_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Completed", "Orders().Executions"}, []int{3, 5, 7, 11, 55, 59, 99, 101, 102}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"fresh paper fill followed by completed-orders apiOnly=false and apiOnly=true queries"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchExhaustiveMarketHours, batchReplayAll),
		description: "public API campaign for completed-orders apiOnly true/false variants after a live paper fill",
		runAPI:      runAPICompletedOrdersVariantsAAPL,
	},
	"api_transmit_false_then_transmit_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Modify", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"Transmit=false staged order is modified to transmit, then cancelled or rejected by the real Gateway"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API campaign for staging Transmit=false then modifying to transmit and cancel",
		runAPI:      runAPITransmitFalseThenTransmitAAPL,
	},
	"api_duplicate_quote_subscriptions_aapl": {
		metadata:    metaWithAssets("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes"}, []int{1, 2, 58, 59}, "entitlement_probe", []string{"market_data_or_delayed_data"}, []string{"SetType(Delayed), then two same-contract quote subscriptions start independently and both receive delayed bid/ask ticks"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReadOnly, batchExhaustiveReadOnly, batchReplayAll),
		description: "public API probe for two same-contract quote subscriptions on one client",
		runAPI:      runAPIDuplicateQuoteSubscriptionsAAPL,
	},
	"api_reconnect_active_order_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Open", "Orders().Cancel"}, []int{3, 4, 5, 53}, "paper_order", []string{"paper_trading", "multi_leg_recorder"}, []string{"resting GTC order survives client reconnect and is visible/cancellable after reconnect"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API campaign for reconnecting with a live resting order and cancelling it after reconnect",
		runAPI:      runAPIReconnectActiveOrderAAPL,
	},
	"api_client_id0_order_observation_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Open", "Orders().Cancel"}, []int{3, 4, 5, 16, 53}, "paper_order", []string{"paper_trading", "client_id_0", "multi_leg_recorder"}, []string{"client ID 0 observes and cancels another client's live resting order"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API campaign for client ID 0 observing and cancelling another client's resting order",
		runAPI:      runAPIClientID0OrderObservationAAPL,
	},
	"api_cross_client_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Open", "Orders().Cancel"}, []int{3, 4, 5, 16, 53}, "paper_order", []string{"paper_trading", "multi_client", "multi_leg_recorder"}, []string{"one client places a resting order and a second client observes/cancels it or returns the real Gateway rejection"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API campaign for placing from one client ID and cancelling from another client ID",
		runAPI:      runAPICrossClientCancelAAPL,
	},
	"api_forex_lifecycle_eurusd": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Modify", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading", "forex_hours"}, []string{"EUR.USD far LMT reaches Inactive with real paper-account leverage rejection"}, 1, "promoted", []string{"CASH"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for EUR.USD forex rest/modify/cancel lifecycle",
		runAPI:      runAPIForexLifecycleEURUSD,
	},
	"api_whatif_margin_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Preview"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"WhatIf margin/commission preview or real Gateway parser/permission response without execution"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for WhatIf margin/commission preview on AAPL",
		runAPI:      runAPIWhatIfMarginAAPL,
	},
	"api_stress_rapid_fire_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().CancelAll"}, []int{3, 4, 5, 58}, "paper_order", []string{"paper_trading"}, []string{"10 rapid-fire far LMT orders plus global cancel"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for rapid-fire 10 orders plus global cancel",
		runAPI:      runAPIStressRapidFireAAPL,
	},
	"api_scale_in_campaign_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"scale-in 2x MKT buy plus protective stop-loss PreSubmitted trigger; source capture tail timed out during cancel/flatten/execution query"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for scale-in buy strategy with protective stop-loss and flatten",
		runAPI:      runAPIScaleInCampaignAAPL,
	},
	"api_ioc_fok_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().Quote", "Orders().Place", "Orders().Executions"}, []int{1, 2, 3, 5, 11, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"IOC marketable cancel and FOK invalid/inactive paths"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayAll),
		description: "public API campaign for IOC and FOK fill/reject paths",
		runAPI:      runAPIIOCFOKAAPL,
	},
}
