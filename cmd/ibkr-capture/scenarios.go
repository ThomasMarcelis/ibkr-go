package main

import (
	"context"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

// scenario owns the catalog metadata and its public-API runner.
type scenario struct {
	metadata    scenarioMetadata
	description string
	run         func(ctx context.Context, addr string, clientID int) error
}

var scenarios = map[string]*scenario{
	// --- Bootstrap-only scenarios ---

	"bootstrap": {
		metadata:    meta("session", []string{"DialContext"}, []int{71, 15, 9}, "read_only", nil, []string{"ready session", "farm status drain"}, 1, "promoted", batchReadOnly),
		description: "clean handshake + START_API + farm-status drain (no feature request)",
		run:         runAPIBootstrap,
	},
	"bootstrap_client_id_0": {
		metadata:    meta("session", []string{"DialContext"}, []int{71, 15, 9}, "read_only", []string{"client_id_0"}, []string{"ready session scoped to client ID 0"}, 0, "promoted", batchReadOnly),
		description: "same as bootstrap but client_id=0 (required for REQ_ALL_OPEN_ORDERS scope)",
		run:         runAPIBootstrap,
	},
	"current_time": {
		metadata:    meta("session", []string{"Client.CurrentTime"}, []int{49}, "read_only", nil, []string{"parsed server current time"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request and parse the Gateway's current time through the public API",
		run:         runAPICurrentTime,
	},
	"current_time_millis": {
		metadata:    meta("session", []string{"Client.CurrentTimeMillis"}, []int{105, 109}, "read_only", nil, []string{"server current time in milliseconds"}, 1, "promoted", batchReadOnly),
		description: "request and parse millisecond-precision Gateway time through the public API",
		run:         runAPICurrentTimeMillis,
	},
	"managed_accounts_refresh": {
		metadata:    meta("session", []string{"Client.ManagedAccounts"}, []int{protocol.OutReqManagedAccounts, protocol.InManagedAccounts}, "read_only", nil, []string{"nonempty managed-account refresh and session snapshot update"}, 1, "promoted", batchReadOnly),
		description: "refresh the login's managed accounts through the public API",
		run:         runAPIManagedAccounts,
	},
	"req_ids": {
		metadata:    meta("session", []string{"Orders().RefreshOrderID"}, []int{8, 9, 4}, "read_only", nil, []string{"refreshed order ID or real read-only Gateway rejection"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "refresh the engine-owned order ID seed through the public API",
		run:         runAPIRefreshOrderID,
	},

	// --- Contract details ---

	"contract_details_aapl_stk": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"decoded AAPL stock contract details"}, 1, "promoted", batchReadOnly),
		description: "request and decode AAPL stock contract details through the public API",
		run:         runAPIContractDetailsAAPLStock,
	},
	"contract_details_aapl_opt": {
		metadata:    meta("contracts", []string{"Contracts().SecDefOptParams", "Contracts().Details"}, []int{protocol.OutReqSecDefOptParams, protocol.InSecDefOptParams, protocol.InSecDefOptParamsEnd, protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"complete nearest-expiry option contract ladder"}, 1, "promoted", batchReadOnly),
		description: "resolve and completely decode the nearest AAPL option expiry through the public API",
		run:         runAPIContractDetailsAAPLOptions,
	},
	"contract_details_apple_bonds": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InErrMsg}, "read_only", nil, []string{"bond contract details or exact issuer-ambiguity blocker"}, 1, "blocked", batchReadOnly),
		description: "request Apple bonds by live-derived issuer ID and record the exact Gateway result",
		run:         runAPIContractDetailsAppleBonds,
	},
	"contract_details_eurusd_cash": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"cash/FX contract details"}, 1, "promoted", batchReadOnly),
		description: "request and decode EUR.USD cash contract details through the public API",
		run:         runAPIContractDetailsEURUSD,
	},
	"contract_details_es_fut": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"ES futures expiry ladder"}, 1, "promoted", batchReadOnly),
		description: "request and decode the ES futures expiry ladder through the public API",
		run:         runAPIContractDetailsESFutures,
	},
	"contract_details_not_found": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InErrMsg}, "read_only", nil, []string{"typed code 200 contract-not-found error"}, 1, "promoted", batchReadOnly),
		description: "request a nonexistent stock and require the live contract-not-found error through the public API",
		run:         runAPIContractDetailsNotFound,
	},
	"contract_details_concurrent": {
		metadata:    meta("contracts", []string{"Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"simultaneous AAPL and EUR.USD contract-detail requests complete on distinct request IDs"}, 1, "promoted", batchReadOnly),
		description: "request AAPL and EUR.USD contract details concurrently through the public API",
		run:         runAPIContractDetailsConcurrent,
	},

	// --- Account summary ---

	"account_summary_snapshot": {
		metadata:    meta("accounts", []string{"Accounts().Summary", "Client.CurrentTime"}, []int{protocol.OutReqAccountSummary, protocol.InAccountSummary, protocol.InAccountSummaryEnd, protocol.OutReqCurrentTime}, "read_only", nil, []string{"finite account summary snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "collect and close a finite account summary through the public API",
		run:         runAPIAccountSummarySnapshot,
	},
	"account_summary_stream": {
		metadata:    meta("accounts", []string{"Accounts().SubscribeSummary", "Client.CurrentTime"}, []int{protocol.OutReqAccountSummary, protocol.InAccountSummary, protocol.InAccountSummaryEnd, protocol.OutReqCurrentTime}, "read_only", nil, []string{"nonempty summary snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "observe and close an account-summary subscription through the public API",
		run:         runAPIAccountSummaryStream,
	},
	"account_summary_two_subs": {
		metadata:    meta("accounts", []string{"Accounts().SubscribeSummary", "Client.CurrentTime"}, []int{protocol.OutReqAccountSummary, protocol.InAccountSummary, protocol.InAccountSummaryEnd, protocol.OutReqCurrentTime}, "read_only", nil, []string{"two nonempty concurrent summary snapshots and protocol-fenced cancellations"}, 1, "promoted", batchReadOnly),
		description: "observe two concurrent account-summary subscriptions through the public API",
		run:         runAPIAccountSummaryTwoSubscriptions,
	},

	// --- Positions ---

	"positions_snapshot": {
		metadata:    meta("accounts", []string{"Accounts().Positions", "Client.CurrentTime"}, []int{protocol.OutReqPositions, protocol.InPositionEnd, protocol.OutCancelPositions, protocol.OutReqCurrentTime}, "read_only", nil, []string{"finite positions snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "collect and close the positions snapshot through the public API",
		run:         runAPIPositionsSnapshot,
	},
	"positions_subscription": {
		metadata:    meta("accounts", []string{"Accounts().SubscribePositions", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqPositions, protocol.InPositionData, protocol.InPositionEnd, protocol.OutCancelPositions, protocol.OutReqCurrentTime}, "read_only", nil, []string{"nonempty positions stream reaches SnapshotComplete before protocol-fenced cancellation"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "observe the positions subscription snapshot boundary through the public API",
		run:         runAPIPositionsSubscription,
	},

	// --- Historical bars ---

	"historical_bars_1d_1h": {
		metadata:    meta("history", []string{"History().Bars"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalData, protocol.InHistoricalDataEnd}, "read_only", []string{"historical_data"}, []string{"nonempty hourly trade bars for liquid stock"}, 1, "promoted", batchReadOnly),
		description: "request and decode one day of hourly AAPL trade bars through the public API",
		run:         runAPIHistoricalBars1Day1Hour,
	},
	"historical_bars_30d_1day": {
		metadata:    meta("history", []string{"History().Bars"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalData, protocol.InHistoricalDataEnd}, "read_only", []string{"historical_data"}, []string{"nonempty daily trade bars over a 30-day window"}, 1, "blocked", batchReadOnly),
		description: "request and decode 30 days of daily AAPL trade bars through the public API",
		run:         runAPIHistoricalBars30Days1Day,
	},
	"historical_bars_bidask": {
		metadata:    meta("history", []string{"History().Bars"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalData, protocol.InHistoricalDataEnd, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty BID_ASK bars or exact historical-data permission error"}, 1, "blocked", batchReadOnly),
		description: "request and decode hourly AAPL BID_ASK bars or the typed live permission error through the public API",
		run:         runAPIHistoricalBarsBidAsk,
	},
	"historical_bars_error": {
		metadata:    meta("history", []string{"History().Bars"}, []int{protocol.OutReqHistoricalData, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"typed code 200 historical-bars contract-not-found error"}, 1, "promoted", batchReadOnly),
		description: "request historical bars for a nonexistent stock and require the typed not-found error through the public API",
		run:         runAPIHistoricalBarsError,
	},
	"historical_schedule_aapl": {
		metadata:    meta("history", []string{"History().Schedule"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalSchedule}, "read_only", []string{"historical_data"}, []string{"nonempty historical session schedule with timezone"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request and decode one month of AAPL trading sessions through the public API",
		run:         runAPIHistoricalScheduleAAPL,
	},

	// --- Market data type control (MarketData().SetType) ---

	"set_type_live": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqCurrentTime}, "read_only", nil, []string{"protocol-fenced live market-data type request"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "select live market data through the public API",
		run:         runAPISetMarketDataLive,
	},
	"set_type_frozen": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqCurrentTime}, "read_only", nil, []string{"protocol-fenced frozen market-data type request"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "select frozen market data through the public API",
		run:         runAPISetMarketDataFrozen,
	},
	"set_type_delayed": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqCurrentTime}, "read_only", nil, []string{"protocol-fenced delayed market-data type request"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "select delayed market data through the public API",
		run:         runAPISetMarketDataDelayed,
	},
	"set_type_delayed_frozen": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqCurrentTime}, "read_only", nil, []string{"protocol-fenced delayed-frozen market-data type request"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "select delayed-frozen market data through the public API",
		run:         runAPISetMarketDataDelayedFrozen,
	},
	"set_type_switch_while_streaming": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InMarketDataType, protocol.InTickPrice, protocol.InTickSize, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"delayed type and price/size evidence followed by accepted live switch and fenced cancellation"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "switch an active delayed AAPL quote stream to live through the public API",
		run:         runAPISetTypeSwitchWhileStreaming,
	},

	// --- Market data quotes ---

	"quote_snapshot_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().Quote"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.InTickPrice, protocol.InTickSize, protocol.InTickString, protocol.InTickSnapshotEnd, protocol.InMarketDataType, protocol.InTickReqParams}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed delayed snapshot with price or size and snapshot completion"}, 1, "promoted", batchReadOnly),
		description: "request and decode a complete delayed AAPL quote snapshot through the public API",
		run:         runAPIQuoteSnapshotAAPL,
	},
	"quote_stream_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InTickPrice, protocol.InTickSize, protocol.InTickString, protocol.InMarketDataType, protocol.InTickReqParams, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed delayed price or size update followed by fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "observe and cleanly cancel a delayed AAPL quote stream through the public API",
		run:         runAPIQuoteStreamAAPL,
	},
	"quote_stream_genericticks": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InTickGeneric, protocol.InTickString, protocol.InMarketDataType, protocol.InTickReqParams, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed quote parameters and a 233/236 value followed by fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "observe generic ticks 233 and 236 on a delayed AAPL quote stream through the public API",
		run:         runAPIQuoteStreamGenericTicksAAPL,
	},
	"quote_odd_lot_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InTickPrice, protocol.InTickSize, protocol.InTickString, protocol.InTickReqParams, protocol.InErrMsg}, "entitlement_probe", []string{"market_hours", "live_market_data_for_odd_lots"}, []string{"generic tick 787 request with typed odd-lot fields or an exact entitlement/no-row boundary and fenced cancellation"}, 1, "blocked", batchNewV2, batchReadOnly, batchExhaustiveMarketHours),
		description: "request and observe the v225 AAPL odd-lot quote family through the public API",
		run:         runAPIOddLotQuotesAAPL,
	},

	// --- Real-time bars ---

	"realtime_bars_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeRealTimeBars", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqRealTimeBars, protocol.OutCancelRealTimeBars, protocol.OutReqCurrentTime, protocol.InRealTimeBars, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed real-time bar and fenced cancellation or exact live permission refusal"}, 1, "promoted", batchReadOnly),
		description: "observe an AAPL real-time bar or the exact live permission refusal through the public API",
		run:         runAPIRealTimeBarsAAPL,
	},

	// --- Open orders ---

	"open_orders_empty": {
		metadata:    meta("orders", []string{"Orders().Open"}, []int{protocol.OutReqOpenOrders, protocol.InOpenOrder, protocol.InOpenOrderEnd, protocol.InErrMsg}, "read_only", nil, []string{"own open-orders snapshot or exact read-only refusal"}, 1, "promoted", batchReadOnly),
		description: "request the client's open-order snapshot or typed read-only refusal through the public API",
		run:         runAPIOpenOrdersClient,
	},
	"open_orders_all": {
		metadata:    meta("orders", []string{"Orders().Open"}, []int{protocol.OutReqAllOpenOrders, protocol.InOpenOrder, protocol.InOpenOrderEnd}, "read_only", []string{"client_id_0"}, []string{"all open-orders snapshot"}, 0, "promoted", batchReadOnly),
		description: "request the all-client open-order snapshot through the public API",
		run:         runAPIOpenOrdersAll,
	},

	// --- Executions ---

	"executions_snapshot": {
		metadata:    meta("orders", []string{"Orders().Executions"}, []int{protocol.OutReqExecutions, protocol.InExecutionData, protocol.InExecutionDataEnd}, "read_only", nil, []string{"finite execution query"}, 1, "promoted", batchReadOnly),
		description: "request an unfiltered execution snapshot through the public API",
		run:         runAPIExecutionsSnapshot,
	},
	"executions_concurrent_aapl": {
		metadata:    meta("orders", []string{"Orders().Executions"}, []int{protocol.OutReqExecutions, protocol.InExecutionData, protocol.InExecutionDataEnd, protocol.InCommissionReport}, "read_only", nil, []string{"concurrent all-side, buy, and sell AAPL execution queries remain request-ID isolated"}, 1, "promoted", batchReadOnly),
		description: "query all-side, buy, and sell AAPL executions concurrently through the public API",
		run:         runAPIExecutionsConcurrentAAPL,
	},

	// --- v1 expanded scope: Batch C1 — singleton one-shots (no reqID) ---

	"family_codes": {
		metadata:    meta("accounts", []string{"Accounts().FamilyCodes"}, []int{80, 78}, "read_only", nil, []string{"family codes response"}, 1, "promoted", batchReadOnly),
		description: "request and decode account family codes through the public API",
		run:         runAPIFamilyCodes,
	},
	"news_providers": {
		metadata:    meta("news", []string{"News().Providers"}, []int{85}, "read_only", nil, []string{"subscribed news provider list"}, 1, "promoted", batchReadOnly),
		description: "request and decode subscribed news providers through the public API",
		run:         runAPINewsProviders,
	},
	"mkt_depth_exchanges": {
		metadata:    meta("contracts", []string{"Contracts().DepthExchanges"}, []int{protocol.OutReqMktDepthExchanges, protocol.InMktDepthExchanges}, "read_only", nil, []string{"market-depth exchange metadata"}, 1, "promoted", batchReadOnly),
		description: "request and decode market-depth exchanges through the public API",
		run:         runAPIDepthExchanges,
	},
	"scanner_parameters": {
		metadata:    meta("scanner", []string{"Scanner().Parameters"}, []int{24, 19}, "read_only", nil, []string{"scanner XML parameters"}, 1, "promoted", batchReadOnly),
		description: "request and receive scanner parameter XML through the public API",
		run:         runAPIScannerParameters,
	},

	// --- v1 expanded scope: Batch C2 — keyed one-shots ---

	"user_info": {
		metadata:    meta("tws", []string{"TWS().UserInfo"}, []int{protocol.OutReqUserInfo, protocol.InUserInfo}, "read_only", nil, []string{"user info response"}, 1, "promoted", batchReadOnly),
		description: "request and decode TWS user information through the public API",
		run:         runAPIUserInfo,
	},
	"tws_config": {
		metadata:    meta("tws", []string{"TWS().Config"}, []int{protocol.OutReqConfig, protocol.InConfig}, "read_only", nil, []string{"presence-preserving TWS or Gateway configuration snapshot"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request the current TWS or Gateway configuration through the public API",
		run:         runAPITWSConfig,
	},
	"matching_symbols_aapl": {
		metadata:    meta("contracts", []string{"Contracts().Search"}, []int{protocol.OutReqMatchingSymbols, protocol.InSymbolSamples}, "read_only", nil, []string{"exact-ish symbol samples"}, 1, "promoted", batchReadOnly),
		description: "search for AAPL contracts through the public API",
		run:         runAPIMatchingSymbolsAAPL,
	},
	"matching_symbols_partial": {
		metadata:    meta("contracts", []string{"Contracts().Search"}, []int{protocol.OutReqMatchingSymbols, protocol.InSymbolSamples}, "read_only", nil, []string{"broad symbol samples"}, 1, "promoted", batchReadOnly),
		description: "search a broad AA symbol pattern through the public API",
		run:         runAPIMatchingSymbolsPartial,
	},
	"head_timestamp_aapl": {
		metadata:    meta("history", []string{"History().HeadTimestamp"}, []int{protocol.OutReqHeadTimestamp, protocol.InHeadTimestamp}, "read_only", []string{"historical_data"}, []string{"nonzero earliest AAPL trade timestamp"}, 1, "promoted", batchReadOnly),
		description: "request and decode AAPL's earliest trade timestamp through the public API",
		run:         runAPIHeadTimestampAAPL,
	},
	"sec_def_opt_params_aapl": {
		metadata:    meta("contracts", []string{"Contracts().SecDefOptParams"}, []int{protocol.OutReqSecDefOptParams, protocol.InSecDefOptParams, protocol.InSecDefOptParamsEnd}, "read_only", nil, []string{"option parameter surface"}, 1, "promoted", batchReadOnly),
		description: "request and decode AAPL option-chain parameters through the public API",
		run:         runAPISecDefOptParamsAAPL,
	},
	"histogram_data_aapl": {
		metadata:    meta("history", []string{"History().Histogram"}, []int{protocol.OutReqHistogramData, protocol.InHistogramData}, "read_only", []string{"historical_data"}, []string{"nonempty one-week AAPL histogram"}, 1, "blocked", batchReadOnly),
		description: "request and decode a one-week AAPL price histogram through the public API",
		run:         runAPIHistogramAAPL,
	},
	"historical_ticks_aapl_trades": {
		metadata:    meta("history", []string{"History().Ticks"}, []int{protocol.OutReqHistoricalTicks, protocol.InHistoricalTicksLast, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty historical trades or exact permission error"}, 1, "blocked", batchReadOnly),
		description: "request recent AAPL trade ticks or the typed live permission error through the public API",
		run:         runAPIHistoricalTicksTrades,
	},
	"historical_ticks_aapl_bidask": {
		metadata:    meta("history", []string{"History().Ticks"}, []int{protocol.OutReqHistoricalTicks, protocol.InHistoricalTicksBidAsk, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty historical bid/ask ticks or exact permission error"}, 1, "blocked", batchReadOnly),
		description: "request recent AAPL bid/ask ticks or the typed live permission error through the public API",
		run:         runAPIHistoricalTicksBidAsk,
	},
	"historical_ticks_aapl_midpoint": {
		metadata:    meta("history", []string{"History().Ticks"}, []int{protocol.OutReqHistoricalTicks, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"nonempty historical midpoint ticks or exact permission error"}, 1, "blocked", batchReadOnly),
		description: "request recent AAPL midpoint ticks or the typed live permission error through the public API",
		run:         runAPIHistoricalTicksMidpoint,
	},
	"historical_news_aapl": {
		metadata:    meta("news", []string{"News().Historical"}, []int{protocol.OutReqHistoricalNews, protocol.InHistoricalNewsEnd}, "read_only", []string{"news_or_historical_news"}, []string{"nonempty historical news snapshot"}, 1, "promoted", batchReadOnly),
		description: "request recent AAPL historical news through the public API",
		run:         runAPIHistoricalNewsAAPL,
	},
	"historical_ticks_aapl_timezone_start": {
		metadata:    meta("history", []string{"History().Ticks"}, []int{protocol.OutReqHistoricalTicks, protocol.InHistoricalTicksBidAsk, protocol.InHistoricalTicksLast, protocol.InErrMsg}, "read_only", []string{"historical_data"}, []string{"explicit UTC start-bound ticks for all kinds or exact permission errors"}, 1, "blocked", batchNewV2, batchReadOnly),
		description: "request AAPL trade, bid/ask, and midpoint ticks from an explicit UTC start bound through the public API",
		run:         runAPIHistoricalTicksStartBound,
	},
	"historical_news_aapl_timezone_window": {
		metadata:    meta("news", []string{"News().Historical"}, []int{protocol.OutReqHistoricalNews, protocol.InHistoricalNewsEnd}, "read_only", []string{"news_or_historical_news"}, []string{"nonempty historical news at or after an explicit UTC lower bound"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request AAPL historical news with an explicit UTC lower end bound through the public API",
		run:         runAPIHistoricalNewsAAPLTimezoneWindow,
	},

	// --- v1 expanded scope: Batch C3 — completed orders and tick types ---

	"completed_orders": {
		metadata:    meta("orders", []string{"Orders().Completed", "Client.CurrentTime"}, []int{protocol.OutReqCompletedOrders, protocol.InCompletedOrder, protocol.InCompletedOrderEnd, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", nil, []string{"finite apiOnly completed-order snapshot and protocol fence"}, 1, "promoted", batchReadOnly),
		description: "collect the apiOnly completed-order snapshot through the public API",
		run:         runAPICompletedOrders,
	},
	"tick_efp_probe": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.InMarketDataType, protocol.InTickReqParams, protocol.InTickPrice, protocol.InTickSize, protocol.InTickEFP, protocol.InDeltaNeutralValidation, protocol.InErrMsg, protocol.OutReqCurrentTime}, "entitlement_probe", []string{"live_market_data", "active_single_stock_future", "matching_stock"}, []string{"typed TickEFP or delta-neutral validation callback, or a real contract, entitlement, or no-data result with fenced cancellation"}, 1, "blocked", batchReadOnly),
		description: "live EFP market-data probe using DTE/EUREX and Tencent/HKFE single-stock-future BAGs",
		run:         runAPITickEFPProbe,
	},
	"quote_stream_multi_asset": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.OutCancelMktData, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InTickPrice, protocol.InTickSize, protocol.InTickGeneric, protocol.InTickString}, "read_only", []string{"market_data_or_delayed_data"}, []string{"concurrent stock and FX quote streams with real price or size evidence and fenced cancellation"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "observe concurrent delayed AAPL and EUR.USD quote streams through the public API",
		run:         runAPIQuoteStreamMultiAsset,
	},

	// --- v1 expanded scope: Batch C4 — streaming subscriptions ---

	"account_updates": {
		metadata:    meta("accounts", []string{"Accounts().Updates", "Client.CurrentTime"}, []int{protocol.OutReqAccountUpdates, protocol.InUpdateAccountValue, protocol.InUpdatePortfolio, protocol.InUpdateAccountTime, protocol.InAccountDownloadEnd, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", nil, []string{"nonempty typed account and portfolio snapshot with protocol-fenced unsubscribe"}, 1, "promoted", batchReadOnly),
		description: "collect and close the account updates snapshot through the public API",
		run:         runAPIAccountUpdates,
	},
	"account_updates_multi": {
		metadata:    meta("accounts", []string{"Accounts().UpdatesMulti", "Client.CurrentTime"}, []int{protocol.OutReqAccountUpdatesMulti, protocol.InAccountUpdateMulti, protocol.InAccountUpdateMultiEnd, protocol.OutCancelAccountUpdatesMulti, protocol.OutReqCurrentTime}, "read_only", nil, []string{"nonempty multi-account update snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "collect and close a multi-account update snapshot through the public API",
		run:         runAPIAccountUpdatesMulti,
	},
	"positions_multi": {
		metadata:    meta("accounts", []string{"Accounts().PositionsMulti", "Client.CurrentTime"}, []int{protocol.OutReqPositionsMulti, protocol.InPositionMulti, protocol.InPositionMultiEnd, protocol.OutCancelPositionsMulti, protocol.OutReqCurrentTime}, "read_only", nil, []string{"completed multi-account positions snapshot and protocol-fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "collect and close a multi-account positions snapshot through the public API",
		run:         runAPIPositionsMulti,
	},
	"pnl": {
		metadata:    meta("accounts", []string{"Accounts().SubscribePnL", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqPnL, protocol.InPnL, protocol.OutCancelPnL, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "read_only", nil, []string{"typed account PnL update, including an all-zero snapshot, followed by fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "observe and close the account PnL stream through the public API",
		run:         runAPIPnL,
	},
	"pnl_single": {
		metadata:    meta("accounts", []string{"Accounts().Updates", "Accounts().SubscribePnLSingle", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqAccountUpdates, protocol.InUpdatePortfolio, protocol.InAccountDownloadEnd, protocol.OutReqPnLSingle, protocol.InPnLSingle, protocol.OutCancelPnLSingle, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", nil, []string{"typed PnL update for a real held contract followed by fenced cancellation"}, 1, "promoted", batchReadOnly),
		description: "derive a held contract and observe its single-position PnL through the public API",
		run:         runAPIPnLSingle,
	},
	"tick_by_tick_last": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeTickByTick", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqTickByTickData, protocol.OutCancelTickByTickData, protocol.OutReqCurrentTime, protocol.InTickByTick, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed Last tick and fenced cancellation or exact live entitlement refusal"}, 1, "blocked", batchReadOnly),
		description: "observe an AAPL Last tick or the exact live entitlement refusal through the public API",
		run:         runAPITickByTickLastAAPL,
	},
	"tick_by_tick_bidask": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeTickByTick", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqTickByTickData, protocol.OutCancelTickByTickData, protocol.OutReqCurrentTime, protocol.InTickByTick, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed BidAsk tick and fenced cancellation or exact live entitlement refusal"}, 1, "blocked", batchReadOnly),
		description: "observe an AAPL BidAsk tick or the exact live entitlement refusal through the public API",
		run:         runAPITickByTickBidAskAAPL,
	},
	"tick_by_tick_midpoint": {
		metadata:    meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeTickByTick", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMarketDataType, protocol.OutReqTickByTickData, protocol.OutCancelTickByTickData, protocol.OutReqCurrentTime, protocol.InTickByTick, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_data_or_delayed_data"}, []string{"typed MidPoint tick and fenced cancellation or exact live entitlement refusal"}, 1, "blocked", batchReadOnly),
		description: "observe an AAPL MidPoint tick or the exact live entitlement refusal through the public API",
		run:         runAPITickByTickMidPointAAPL,
	},
	"historical_bars_keepup": {
		metadata:    meta("history", []string{"History().SubscribeBars", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqHistoricalData, protocol.InHistoricalData, protocol.InHistoricalDataEnd, protocol.InHistoricalDataUpdate, protocol.OutCancelHistoricalData, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "read_only", []string{"market_hours", "historical_data"}, []string{"nonempty MIDPOINT, BID, and ASK one-minute snapshots, one real streaming update for each, and explicit protocol-fenced cancellation"}, 1, "blocked", batchReadOnly),
		description: "capture post-snapshot AAPL MIDPOINT, BID, and ASK historical-bar updates through the public API",
		run:         runAPIHistoricalBarsKeepUp,
	},
	"news_bulletins": {
		metadata:    meta("news", []string{"News().SubscribeBulletins", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqNewsBulletins, protocol.InNewsBulletins, protocol.OutCancelNewsBulletins, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "read_only", []string{"news_or_bulletins"}, []string{"at least one typed bulletin callback followed by fenced cancellation"}, 1, "blocked", batchReadOnly),
		description: "observe the live bulletin stream through the public API and close it cleanly",
		run:         runAPINewsBulletins,
	},

	// --- v1 expanded scope: Batch C5 — option calculations and scanner ---
	"smart_components": {
		metadata:    meta("contracts", []string{"MarketData().SetType", "MarketData().SubscribeQuotes", "Contracts().SmartComponents"}, []int{protocol.OutReqMarketDataType, protocol.OutReqMktData, protocol.InTickReqParams, protocol.OutReqSmartComponents, protocol.InSmartComponents, protocol.OutCancelMktData}, "read_only", []string{"market_hours", "market_data_or_delayed_data"}, []string{"quote-derived smart component mapping"}, 1, "promoted", batchReadOnly, batchExhaustiveMarketHours),
		description: "derive AAPL's BBO mapping from quote parameters and decode its SMART components through the public API",
		run:         runAPISmartComponents,
	},
	"market_rule": {
		metadata:    meta("contracts", []string{"Contracts().MarketRule"}, []int{91, 93}, "read_only", nil, []string{"price increment ladder"}, 1, "promoted", batchReadOnly),
		description: "request and decode US equity market rule 26 through the public API",
		run:         runAPIMarketRule,
	},

	"market_depth_aapl": {
		metadata:    meta("market_data", []string{"MarketData().SubscribeDepth", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMktDepth, protocol.OutCancelMktDepth, protocol.OutReqCurrentTime, protocol.InMarketDepth, protocol.InMarketDepthL2, protocol.InCurrentTime, protocol.InErrMsg}, "entitlement_probe", []string{"l2_market_data_or_error"}, []string{"typed regular depth row with fenced cancellation, or an exact live refusal"}, 1, "blocked", batchNewV2, batchReadOnly),
		description: "observe an AAPL regular depth row or an exact live refusal through the public API",
		run:         runAPIMarketDepthAAPL,
	},
	"market_depth_aapl_smart": {
		metadata:    meta("market_data", []string{"MarketData().SubscribeDepth", "Client.SessionEvents", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqMktDepth, protocol.OutCancelMktDepth, protocol.OutReqCurrentTime, protocol.InMarketDepth, protocol.InMarketDepthL2, protocol.InCurrentTime, protocol.InErrMsg}, "entitlement_probe", []string{"l2_market_data_or_error"}, []string{"typed smart depth row or exact no-available-depth notice, followed by fenced cancellation"}, 1, "blocked", batchNewV2, batchReadOnly),
		description: "observe an AAPL SMART depth row or the exact live no-available-depth notice through the public API",
		run:         runAPIMarketDepthSmartAAPL,
	},

	// --- Reference data ---

	"soft_dollar_tiers": {
		metadata:    meta("advisors", []string{"Advisors().SoftDollarTiers"}, []int{79, 77}, "read_only", nil, []string{"soft-dollar tier list"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request and decode soft-dollar tiers through the public API",
		run:         runAPISoftDollarTiers,
	},
	"display_groups": {
		metadata:    meta("tws", []string{"TWS().DisplayGroups"}, []int{67}, "read_only", nil, []string{"display group list"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request and decode TWS display groups through the public API",
		run:         runAPIDisplayGroups,
	},
	"display_group_subscribe": {
		metadata:    meta("tws", []string{"TWS().DisplayGroups", "TWS().SubscribeDisplayGroup", "DisplayGroupHandle.Update", "Client.CurrentTime"}, []int{protocol.OutQueryDisplayGroups, protocol.InDisplayGroupList, protocol.OutSubscribeToGroupEvents, protocol.InDisplayGroupUpdated, protocol.OutUpdateDisplayGroup, protocol.OutUnsubscribeFromGroupEvents, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "read_only", []string{"tws_display_groups"}, []string{"typed display group query, subscribe, state-preserving update when possible, unsubscribe"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "query real display group IDs and exercise the public subscription lifecycle",
		run:         runAPIDisplayGroupSubscription,
	},
	"wsh_meta_data": {
		metadata:    meta("wsh", []string{"WSH().MetaData"}, []int{protocol.OutReqWSHMetaData, protocol.InWSHMetaData, protocol.InErrMsg}, "entitlement_probe", []string{"wsh_subscription_or_error"}, []string{"valid WSH metadata JSON or exact entitlement error"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "request WSH metadata or its typed entitlement refusal through the public API",
		run:         runAPIWSHMetaData,
	},
	"wsh_event_data_aapl": {
		metadata:    meta("wsh", []string{"WSH().EventData"}, []int{protocol.OutReqWSHEventData, protocol.InWSHEventData, protocol.InErrMsg}, "entitlement_probe", []string{"wsh_subscription_or_error"}, []string{"valid WSH event JSON or exact entitlement error"}, 1, "blocked", batchNewV2, batchReadOnly),
		description: "request AAPL WSH events or their typed entitlement refusal through the public API",
		run:         runAPIWSHEventDataAAPL,
	},
	"request_fa": {
		metadata:    meta("advisors", []string{"Advisors().Config", "Client.CurrentTime"}, []int{protocol.OutRequestFA, protocol.InReceiveFA, protocol.OutReqCurrentTime, protocol.InCurrentTime, protocol.InErrMsg}, "entitlement_probe", []string{"fa_account_or_error"}, []string{"typed FA groups XML or exact non-FA refusal"}, 1, "blocked", batchNewV2, batchReadOnly),
		description: "request the FA groups document through the public API",
		run:         runAPIFAConfigGroups,
	},
	"qualify_contract_aapl_exact": {
		metadata:    meta("contracts", []string{"Contracts().Qualify"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"single qualified contract"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "qualify an unpinned AAPL stock contract through the public API",
		run:         runAPIQualifyContractAAPL,
	},
	"qualify_contract_ambiguous": {
		metadata:    meta("contracts", []string{"Contracts().Qualify"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd}, "read_only", nil, []string{"ErrAmbiguousContract from multiple matches"}, 1, "promoted", batchNewV2, batchReadOnly),
		description: "require ambiguous qualification for MSFT stock without an exchange through the public API",
		run:         runAPIQualifyContractAmbiguous,
	},
	"api_order_type_matrix_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Replace", "OrderHandle.Cancel", "Orders().Executions"}, []int{1, 2, 3, 4, 5, 11, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"MKT/LMT/STP/STP LMT/TRAIL/TRAIL LIMIT/MIT/LIT/MTL/REL/MOC/LOC/MOO/LOO/PEG families reach a real order lifecycle; incomplete PEG BENCH input reaches exact local validation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchExhaustivePremarket, batchReplayDefault, batchReplayAll),
		description: "public API campaign for AAPL order type breadth: fills, rests, rejects, modifies, and cancels",
		run:         runAPIOrderTypeMatrixAAPL,
	},
	"api_order_fill_aapl": {
		metadata: metaWithAssets("orders", []string{"Accounts().Summary", "Accounts().SubscribePositions", "Orders().SubscribeOpen", "Orders().Executions", "Orders().Place", "Client.CurrentTime"}, []int{
			protocol.OutPlaceOrder, protocol.InOrderStatus, protocol.InOpenOrder, protocol.OutReqExecutions,
			protocol.InExecutionData, protocol.OutReqAllOpenOrders, protocol.OutReqCurrentTime,
			protocol.InCurrentTime, protocol.InOpenOrderEnd, protocol.InExecutionDataEnd,
			protocol.InCommissionReport, protocol.OutReqPositions, protocol.OutReqAccountSummary,
			protocol.InPositionData, protocol.InPositionEnd, protocol.InAccountSummary, protocol.InAccountSummaryEnd,
		}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"one-share SMART AAPL market buy fills with execution and fee; cleanup sells exactly the campaign delta and verifies a second execution/fee, zero working orders, and the unchanged position inventory"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "guarded one-share AAPL fill with execution/fee proof and baseline reconciliation",
		run:         runAPIOrderFillAAPL,
	},
	"api_order_rest_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"far LMT rest/cancel path"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for AAPL resting order types and cancel/reject behavior",
		run:         runAPIOrderRestCancelAAPL,
	},
	"api_order_direct_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Cancel", "Client.CurrentTime"}, []int{protocol.OutPlaceOrder, protocol.InOpenOrder, protocol.InOrderStatus, protocol.OutCancelOrder, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "paper_order", []string{"paper_trading"}, []string{"same-client top-level direct cancel reaches typed terminal cancellation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "place a resting AAPL limit order and cancel it through Orders().Cancel",
		run:         runAPIOrderDirectCancelAAPL,
	},
	"api_bracket_place_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().PlaceBracket", "Orders().CancelAll", "Orders().Open", "Client.CurrentTime"}, []int{protocol.OutPlaceOrder, protocol.InOpenOrder, protocol.InOrderStatus, protocol.OutReqGlobalCancel, protocol.OutReqOpenOrders, protocol.InOpenOrderEnd, protocol.OutReqCurrentTime, protocol.InCurrentTime}, "paper_order", []string{"paper_trading"}, []string{"direct PlaceBracket allocates consecutive IDs, binds child parent IDs, stages false/false/true transmit frames, and cleans every leg"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "place and clean up a nonmarketable AAPL bracket through Orders().PlaceBracket",
		run:         runAPIBracketPlaceAAPL,
	},
	"api_order_relative_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"REL rest/cancel behavior isolated because Gateway can reconnect during relative order validation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for AAPL relative order behavior",
		run:         runAPIOrderRelativeCancelAAPL,
	},
	"api_order_trailing_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"TRAIL and TRAIL LIMIT behavior isolated because Gateway can reconnect during trailing validation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for AAPL trailing and trailing-limit behavior",
		run:         runAPIOrderTrailingCancelAAPL,
	},
	"api_order_stop_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"STP and STP LMT rest/cancel behavior isolated because Gateway can reconnect during stop validation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for AAPL stop and stop-limit rest/cancel behavior",
		run:         runAPIOrderStopCancelAAPL,
	},
	"api_order_rejects_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "Orders().Cancel"}, []int{1, 2, 3, 4, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"invalid order type, price band, invalid contract, and unknown cancel real Gateway errors"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for AAPL order rejection and unknown cancel behavior",
		run:         runAPIOrderRejectsAAPL,
	},
	"api_delayed_success_modify_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Replace", "Orders().Executions", "Accounts().Positions"}, []int{3, 4, 5, 11, 59, 61, 62}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"resting limit order later becomes marketable through modify and is observed through the original handle"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign where a resting AAPL limit order succeeds later through OrderHandle.Replace",
		run:         runAPIDelayedSuccessModifyAAPL,
	},
	"api_bracket_trigger_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Replace", "Orders().Open", "Orders().Executions", "Orders().CancelAll"}, []int{3, 4, 5, 11, 16, 53, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"bracket parent fills, children echo the same OCA group, forced take-profit modify reaches real price-band cancellation/rejection"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for bracket parent/child activation and take-profit-trigger sibling cancellation",
		run:         runAPIBracketTriggerAAPL,
	},
	"api_oca_trigger_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Open", "Orders().Executions", "Orders().CancelAll"}, []int{3, 4, 5, 11, 16, 53, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"OCA group echoed on both peers; aggressive peer reaches real price-band cancellation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayDefault, batchReplayAll),
		description: "public API campaign for OCA fill/cancel behavior",
		run:         runAPIOCATriggerAAPL,
	},
	"api_conditions_matrix_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Cancel", "Orders().CancelAll"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"price/time/margin/execution/volume/percent-change condition families accepted or rejected with real Gateway response"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for IBKR order condition families",
		run:         runAPIConditionsMatrixAAPL,
	},
	"api_tif_attribute_matrix_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Cancel", "Orders().Executions"}, []int{3, 4, 5, 7, 11, 55}, "paper_order", []string{"paper_trading"}, []string{"GTC/GTD/GoodAfterTime/AON/MinQty/TrailingPercent/PercentOffset/Scale/Adjusted/ManualOrderTime/AdvancedErrorOverride accepted or rejected with real Gateway response"}, 1, "promoted", []string{"STK"}, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for TIF values and advanced AAPL order attributes",
		run:         runAPITIFAttributeMatrixAAPL,
	},
	"api_security_type_probe_matrix": {
		metadata:    metaWithAssets("contracts", []string{"Contracts().SecDefOptParams", "Contracts().Details"}, []int{protocol.OutReqContractData, protocol.InContractData, protocol.InContractDataEnd, protocol.OutReqSecDefOptParams, protocol.InSecDefOptParams, protocol.InSecDefOptParamsEnd, protocol.InErrMsg}, "entitlement_probe", []string{"security_type_permissions_or_real_error"}, []string{"SecDef-qualified OPT/FOP plus exact contract details or real rejection for STK/OPT/FUT/FOP/CASH/BOND/CFD/WAR/IND/CRYPTO/FUND/BILL/CMDTY/CONTFUT"}, 1, "promoted", []string{"STK", "OPT", "FUT", "FOP", "CASH", "BOND", "CFD", "WAR", "IND", "CRYPTO", "FUND", "BILL", "CMDTY", "CONTFUT"}, batchNewV2, batchReplayAll),
		description: "public API probe matrix for real Gateway contract-details behavior across security types",
		run:         runAPISecurityTypeProbeMatrix,
	},
	"api_generic_tick_matrix_aapl": {
		metadata:    metaWithAssets("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes"}, []int{1, 2, 45, 46, 58, 59, 81}, "entitlement_probe", []string{"market_data_or_delayed_data"}, []string{"delayed AAPL stream preserves observed mark-price tick 37, shortable ticks 46/89, volume-rate tick 56, delayed timestamp tick 88, and omitted minimum-tick parameters"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReadOnly, batchReplayAll),
		description: "public API probe for exact price, size, generic, string, and parameter tick delivery",
		run:         runAPIGenericTickMatrixAAPL,
	},
	"api_tick_news_aapl_probe": {
		metadata:    metaWithAssets("news", []string{"MarketData().SetType", "MarketData().SubscribeQuotes"}, []int{1, 2, 4, 58, 59, 81, 84}, "entitlement_probe", []string{"api_news_subscription"}, []string{"contract-specific BRFG TickNews or a real entitlement/no-new-headline result"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReadOnly, batchReplayAll),
		description: "public API probe for contract-specific BRFG news ticks",
		run:         runAPITickNewsAAPLProbe,
	},
	"api_scanner_subscription": {
		metadata:    metaWithAssets("scanner", []string{"Scanner().SubscribeResults", "Subscription.Close", "Client.CurrentTime"}, []int{protocol.OutReqScannerSubscription, protocol.OutCancelScannerSubscription, protocol.OutReqCurrentTime, protocol.InScannerData, protocol.InCurrentTime, protocol.InErrMsg}, "entitlement_probe", []string{"scanner_permissions_or_real_error"}, []string{"complete ranked or empty result with fenced cancellation, or an exact permission refusal"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReadOnly, batchReplayAll),
		description: "request a complete HOT_BY_VOLUME result, including a valid empty result, or the exact live permission refusal through the public API",
		run:         runAPIScannerSubscription,
	},
	"api_historical_matrix_aapl": {
		metadata:    metaWithAssets("history", []string{"History().Bars"}, []int{20, 17, 4}, "read_only", []string{"historical_data"}, []string{"all planned historical bar-size probes and whatToShow variants return data or real Gateway errors"}, 1, "blocked", []string{"STK"}, batchNewV2, batchReplayAll),
		description: "public API campaign for historical bar-size and whatToShow variants",
		run:         runAPIHistoricalMatrixAAPL,
	},
	"api_news_article_aapl": {
		metadata:    metaWithAssets("news", []string{"News().Historical", "News().Article"}, []int{84, 83, 86, 87, 80, 4}, "entitlement_probe", []string{"news_or_historical_news"}, []string{"article ID sourced from historical news is requested through reqNewsArticle or real entitlement/no-result is frozen"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReplayAll),
		description: "public API campaign that requests a real news article ID from historical news, then fetches the article",
		run:         runAPINewsArticleAAPL,
	},
	"api_wsh_variants_aapl": {
		metadata:    metaWithAssets("wsh", []string{"WSH().MetaData", "WSH().EventData"}, []int{100, 102, 4}, "entitlement_probe", []string{"wsh_subscription_or_error"}, []string{"WSH metadata plus conid, portfolio, watchlist, competitor, and date-window event-data variants return real code 10276 entitlement errors"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReplayAll),
		description: "public API probe for WSH metadata plus conid, portfolio, watchlist, competitor, and date-window event-data entitlement variants",
		run:         runAPIWSHVariantsAAPL,
	},
	"api_algo_variants_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Cancel", "Orders().Executions"}, []int{3, 4, 5, 7, 11, 55}, "paper_order", []string{"paper_trading", "algo_permissions_or_real_error"}, []string{"Adaptive, TWAP, VWAP, ArrivalPx, DarkIce, AccumDist, Inline, Close, PctVol, BalanceImpactRisk, MinImpact, and Jefferies variants accepted, rejected, or cancelled with real Gateway response"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for available IBKR algorithmic strategy variants",
		run:         runAPIAlgoVariantsAAPL,
	},
	"api_pairs_trading_aapl_msft": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"paired AAPL/MSFT market entries and per-symbol flatten fills; source execution-query tail timed out"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for paired AAPL/MSFT market orders and cleanup",
		run:         runAPIPairsTradingAAPLMSFT,
	},
	"api_dollar_cost_averaging_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"three staged AAPL market buys plus aggregate flatten fill; source execution-query tail timed out"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for repeated AAPL buys and post-campaign flattening",
		run:         runAPIDollarCostAveragingAAPL,
	},
	"api_stop_loss_management_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Replace", "OrderHandle.Cancel", "Orders().Executions"}, []int{3, 4, 5, 11, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"filled entry, protective stop placement and repricing, zero-fill cancellation, exact flatten, and execution/fee reconciliation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for placing, moving, cancelling, and flattening a protective stop",
		run:         runAPIStopLossManagementAAPL,
	},
	"api_bracket_trailing_stop_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "Orders().CancelAll", "Orders().Executions"}, []int{1, 2, 3, 4, 7, 11, 55, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"live scenario probes quote, places market-parent bracket with TRAIL child, and receives code 328; promoted replay freezes request/rejection before execution-query and cleanup tail"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for bracket order sequencing with a trailing stop child",
		run:         runAPIBracketTrailingStopAAPL,
	},
	"api_option_exercise_aapl": {
		metadata:    metaWithAssets("options", []string{"Contracts().SecDefOptParams", "Contracts().Details", "MarketData().Quote", "Orders().Place", "Options().Exercise", "Accounts().Positions", "Orders().Executions"}, []int{1, 2, 3, 5, 10, 11, 21, 52, 55, 57, 59, 61, 62, 75, 76}, "paper_marketable_order", []string{"paper_trading", "options_market_hours", "option_permissions", "safe_option_and_stock_reconciliation"}, []string{"one live-qualified ITM AAPL call fills; exact warning 10349 plus PreSubmitted proves accepted-but-unsettled admission and captured disconnect produces an uncertain outcome without a settlement claim"}, 1, "promoted", []string{"OPT", "STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "buy a live-qualified ITM AAPL call and attest accepted-but-unsettled exercise admission",
		run:         runAPIOptionExerciseAAPL,
	},
	"api_hedge_order_aapl": {
		metadata:    meta("orders", []string{"Orders().Place"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"hedge child accept or real rejection per hedge type"}, 1, "promoted", batchTrading),
		description: "public API campaign attaching delta, beta, FX, and pair hedge children to a staged parent",
		run:         runAPIHedgeOrderAAPL,
	},
	"api_option_campaign_aapl": {
		metadata:    metaWithAssets("options", []string{"Contracts().SecDefOptParams", "Contracts().Qualify", "MarketData().Quote", "Options().Price", "Options().Exercise", "Orders().Place", "Orders().Executions", "Orders().Completed"}, []int{1, 2, 3, 5, 11, 21, 55, 59, 75, 76, 99, 101, 102}, "paper_trigger", []string{"paper_trading", "market_hours", "option_permissions", "safe_option_and_stock_reconciliation"}, []string{"blocked until option orders, lapse/exercise requests, and every resulting option or stock delta can be terminally restored"}, 1, "blocked", []string{"OPT"}),
		description: "blocked option campaign pending terminal option and stock reconciliation",
		run:         runAPIOptionCampaignAAPL,
	},
	"api_option_calculations_aapl": {
		metadata:    metaWithAssets("options", []string{"Contracts().SecDefOptParams", "Contracts().Details", "MarketData().Quote", "Options().Price", "Options().ImpliedVolatility"}, []int{1, 2, 4, 10, 21, 52, 54, 55, 56, 57, 75, 76}, "read_only", []string{"option_permissions"}, []string{"live-qualified option price and implied-volatility results with field-presence sentinels"}, 1, "promoted", []string{"OPT"}, batchNewV2, batchReadOnly, batchReplayAll),
		description: "read-only public API probe for live-qualified AAPL option price and implied-volatility calculations",
		run:         runAPIOptionCalculationsAAPL,
	},
	"api_future_campaign_mes": {
		metadata:    metaWithAssets("orders", []string{"Contracts().Details", "MarketData().Quote", "Orders().Place", "Orders().Executions", "Accounts().Positions", "Orders().CancelAll"}, []int{1, 2, 3, 5, 10, 11, 52, 57, 58, 59, 61, 62}, "paper_trigger", []string{"paper_trading", "market_hours", "future_permissions"}, []string{"live-qualified MES future order/modify/round-trip or real permission rejection"}, 1, "promoted", []string{"FUT"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for live-qualified MES futures order behavior",
		run:         runAPIFutureCampaignMES,
	},
	"api_combo_option_vertical_aapl": {
		metadata:    metaWithAssets("orders", []string{"Contracts().SecDefOptParams", "Contracts().Qualify", "Orders().Place", "Orders().Open", "OrderHandle.Cancel"}, []int{3, 4, 5, 16, 53, 75, 76}, "paper_order", []string{"paper_trading", "option_permissions"}, []string{"live-qualified AAPL call vertical carries per-leg prices and NonGuaranteed=1, is accepted PreSubmitted without a combo-level limit, and cancels with zero fill"}, 1, "promoted", []string{"BAG", "OPT"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for live-qualified AAPL option vertical BAG order behavior",
		run:         runAPIComboOptionVerticalAAPL,
	},
	"api_algorithmic_campaign_aapl": {
		metadata:    metaWithAssets("orders", []string{"Accounts().Summary", "Accounts().SubscribeUpdates", "Accounts().SubscribePnL", "Accounts().Positions", "MarketData().SubscribeQuotes", "Orders().SubscribeOpen", "Orders().Place", "OrderHandle.Replace", "Orders().Executions", "Orders().Completed", "Orders().CancelAll"}, []int{1, 2, 3, 5, 6, 7, 8, 11, 16, 53, 54, 58, 59, 61, 62, 63, 64, 92, 93, 99, 101, 102}, "paper_destructive", []string{"paper_trading", "market_hours"}, []string{"four correlated fills and fees across split entries, a resting-limit replacement, exact flatten, four concurrent observers, completed orders, and baseline reconciliation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign with concurrent market/account/order observers and multi-step trading",
		run:         runAPIAlgorithmicCampaignAAPL,
	},
	"api_completed_orders_variants_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Completed", "Orders().Executions"}, []int{3, 5, 7, 11, 55, 59, 99, 101, 102}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"fresh paper fill followed by completed-orders apiOnly=false and apiOnly=true queries"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchExhaustiveMarketHours, batchReplayAll),
		description: "public API campaign for completed-orders apiOnly true/false variants after a live paper fill",
		run:         runAPICompletedOrdersVariantsAAPL,
	},
	"api_transmit_false_then_transmit_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Replace", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"Transmit=false staged order is modified to transmit, then cancelled or rejected by the real Gateway"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API campaign for staging Transmit=false then modifying to transmit and cancel",
		run:         runAPITransmitFalseThenTransmitAAPL,
	},
	"api_empty_tif_default_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"a constructor-built limit order that sets no TIF is sent as DAY, echoed as DAY by the Gateway, and cancelled terminally with no fill"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API probe for the TIF the Gateway applies to a constructor-built order that sets none",
		run:         runAPIEmptyTIFDefaultAAPL,
	},
	"api_include_overnight_lifecycle_aapl": {
		metadata:    metaWithAssets("orders", []string{"Accounts().Summary", "Accounts().SubscribePositions", "Orders().SubscribeOpen", "Orders().Executions", "Orders().Place", "OrderHandle.Replace", "OrderHandle.Cancel", "Client.CurrentTime"}, []int{3, 4, 5, 7, 11, 16, 49, 53, 55, 59, 61, 62, 63, 64}, "paper_order", []string{"paper_trading", "client_id_0", "overnight_session", "multi_leg_recorder"}, []string{"nonmarketable SMART AAPL DAY order echoes IncludeOvernight=true; explicit-false replacement records exact code 462 while the working order retains true; a fresh explicit-false placement is accepted and broker-canonicalized to absent with TIF DAY; both orders cancel terminally and paper state reconciles to baseline"}, 0, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "guarded true-to-false IncludeOvernight placement, replacement, cancellation, and reconciliation",
		run:         runAPIIncludeOvernightLifecycleAAPL,
	},
	"api_duplicate_quote_subscriptions_aapl": {
		metadata:    metaWithAssets("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes"}, []int{1, 2, 58, 59}, "entitlement_probe", []string{"market_data_or_delayed_data"}, []string{"SetType(Delayed), then two same-contract quote subscriptions start independently and both receive delayed bid/ask ticks"}, 1, "promoted", []string{"STK"}, batchNewV2, batchReadOnly, batchExhaustiveReadOnly, batchReplayAll),
		description: "public API probe for two same-contract quote subscriptions on one client",
		run:         runAPIDuplicateQuoteSubscriptionsAAPL,
	},
	"api_reconnect_active_order_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Open", "Orders().Cancel"}, []int{3, 4, 5, 53}, "paper_order", []string{"paper_trading", "multi_leg_recorder"}, []string{"resting GTC order survives client reconnect and is visible/cancellable after reconnect"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API campaign for reconnecting with a live resting order and cancelling it after reconnect",
		run:         runAPIReconnectActiveOrderAAPL,
	},
	"api_client_id0_order_observation_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Open", "Orders().Cancel"}, []int{3, 4, 5, 16, 53}, "paper_order", []string{"paper_trading", "client_id_0", "multi_leg_recorder"}, []string{"client ID 0 observes and cancels another client's live resting order"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API campaign for client ID 0 observing and cancelling another client's resting order",
		run:         runAPIClientID0OrderObservationAAPL,
	},
	"api_cross_client_cancel_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().Open", "Orders().Cancel"}, []int{3, 4, 5, 16, 53}, "paper_order", []string{"paper_trading", "multi_client", "multi_leg_recorder"}, []string{"one client places a resting order and a second client observes/cancels it or returns the real Gateway rejection"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchExhaustiveTrading, batchReplayAll),
		description: "public API campaign for placing from one client ID and cancelling from another client ID",
		run:         runAPICrossClientCancelAAPL,
	},
	"api_forex_lifecycle_eurusd": {
		metadata:    metaWithAssets("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Replace", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading", "forex_hours"}, []string{"EUR.USD far LMT reaches Inactive with real paper-account leverage rejection"}, 1, "promoted", []string{"CASH"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for EUR.USD forex rest/modify/cancel lifecycle",
		run:         runAPIForexLifecycleEURUSD,
	},
	"api_whatif_margin_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Preview"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"WhatIf margin/commission preview or real Gateway parser/permission response without execution"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for WhatIf margin/commission success and rejection on AAPL",
		run:         runAPIWhatIfMarginAAPL,
	},
	"api_stress_rapid_fire_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "Orders().CancelAll"}, []int{3, 4, 5, 58}, "paper_order", []string{"paper_trading"}, []string{"10 rapid-fire far LMT orders plus global cancel"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
		description: "public API campaign for rapid-fire 10 orders plus global cancel",
		run:         runAPIStressRapidFireAAPL,
	},
	"api_scale_in_campaign_aapl": {
		metadata:    metaWithAssets("orders", []string{"Orders().Place", "OrderHandle.Cancel", "Orders().Executions"}, []int{3, 4, 5, 11, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"nondefault scale-field echo, two market fills, protective-stop cancellation, exact flatten, and execution/fee reconciliation"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
		description: "public API campaign for scale-in buy strategy with protective stop-loss and flatten",
		run:         runAPIScaleInCampaignAAPL,
	},
	"api_ioc_fok_aapl": {
		metadata:    metaWithAssets("orders", []string{"MarketData().Quote", "Orders().Place", "Orders().Executions"}, []int{1, 2, 3, 5, 11, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"IOC marketable cancel and FOK invalid/inactive paths"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayAll),
		description: "public API campaign for IOC and FOK fill/reject paths",
		run:         runAPIIOCFOKAAPL,
	},
}
