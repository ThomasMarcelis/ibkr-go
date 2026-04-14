package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type scenarioMetadata struct {
	Domain           string   `json:"domain"`
	Driver           string   `json:"driver"`
	PublicAPI        []string `json:"public_api"`
	MessageIDs       []int    `json:"message_ids"`
	RiskClass        string   `json:"risk_class"`
	Assets           []string `json:"assets,omitempty"`
	Requirements     []string `json:"requirements,omitempty"`
	ExpectedOutcomes []string `json:"expected_outcomes"`
	DefaultClientID  int      `json:"default_client_id"`
	Batches          []string `json:"batches"`
	PromotionStatus  string   `json:"promotion_status"`
	DefaultReplay    bool     `json:"default_replay"`
}

type scenarioCatalogEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	scenarioMetadata
}

const (
	batchAll      = "all"
	batchReadOnly = "read-only"
	batchTrading  = "trading"
	batchNewV2    = "new-v2"

	batchTradingBasic     = "trading-basic"
	batchTradingAdvanced  = "trading-advanced"
	batchTradingCampaigns = "trading-campaigns"
	batchTradingAll       = "trading-all"
	batchReplayDefault    = "replay-default"
	batchReplayAll        = "replay-all"

	driverWire = "wire"
	driverAPI  = "api"
)

var scenarioMetadataByName = map[string]scenarioMetadata{
	"bootstrap":                             meta("session", []string{"DialContext"}, []int{71, 15, 9}, "read_only", nil, []string{"ready session", "farm status drain"}, 1, "promoted", batchReadOnly),
	"bootstrap_client_id_0":                 meta("session", []string{"DialContext"}, []int{71, 15, 9}, "read_only", []string{"client_id_0"}, []string{"ready session scoped to client ID 0"}, 0, "promoted", batchReadOnly),
	"current_time":                          meta("session", []string{"Client.CurrentTime"}, []int{49}, "read_only", nil, []string{"parsed server current time"}, 1, "candidate", batchNewV2, batchReadOnly),
	"req_ids":                               meta("session", []string{"DialContext"}, []int{8, 9}, "read_only", nil, []string{"next valid id from explicit reqIds"}, 1, "candidate", batchNewV2, batchReadOnly),
	"contract_details_aapl_stk":             meta("contracts", []string{"Contracts().Details"}, []int{9, 10, 52}, "read_only", nil, []string{"stock contract details end marker"}, 1, "promoted", batchReadOnly),
	"contract_details_aapl_opt":             meta("contracts", []string{"Contracts().Details"}, []int{9, 10, 52}, "read_only", nil, []string{"option chain contract details"}, 1, "candidate", batchReadOnly),
	"contract_details_eurusd_cash":          meta("contracts", []string{"Contracts().Details"}, []int{9, 10, 52}, "read_only", nil, []string{"cash/FX contract details"}, 1, "candidate", batchReadOnly),
	"contract_details_es_fut":               meta("contracts", []string{"Contracts().Details"}, []int{9, 10, 52}, "read_only", nil, []string{"futures contract details"}, 1, "candidate", batchReadOnly),
	"contract_details_not_found":            meta("contracts", []string{"Contracts().Details"}, []int{9, 4, 52}, "read_only", nil, []string{"real IBKR not-found API error"}, 1, "candidate", batchReadOnly),
	"account_summary_snapshot":              meta("accounts", []string{"Accounts().Summary"}, []int{62, 63, 64}, "read_only", nil, []string{"finite account summary snapshot"}, 1, "promoted", batchReadOnly),
	"account_summary_stream":                meta("accounts", []string{"Accounts().SubscribeSummary"}, []int{62, 63, 64}, "read_only", nil, []string{"summary snapshot plus streaming window"}, 1, "promoted", batchReadOnly),
	"account_summary_two_subs":              meta("accounts", []string{"Accounts().SubscribeSummary"}, []int{62, 63, 64}, "read_only", nil, []string{"concurrent summary subscriptions"}, 1, "promoted", batchReadOnly),
	"positions_snapshot":                    meta("accounts", []string{"Accounts().Positions"}, []int{61, 62, 64}, "read_only", nil, []string{"positions snapshot end marker"}, 1, "promoted", batchReadOnly),
	"historical_bars_1d_1h":                 meta("history", []string{"History().Bars"}, []int{20, 17}, "read_only", []string{"historical_data"}, []string{"hour bars for liquid stock"}, 1, "promoted", batchReadOnly),
	"historical_bars_30d_1day":              meta("history", []string{"History().Bars"}, []int{20, 17}, "read_only", []string{"historical_data"}, []string{"daily bars over long window"}, 1, "candidate", batchReadOnly),
	"historical_bars_bidask":                meta("history", []string{"History().Bars"}, []int{20, 17}, "read_only", []string{"historical_data"}, []string{"BID_ASK historical bars"}, 1, "candidate", batchReadOnly),
	"historical_bars_error":                 meta("history", []string{"History().Bars"}, []int{20, 4}, "read_only", []string{"historical_data"}, []string{"real historical API error"}, 1, "candidate", batchReadOnly),
	"historical_schedule_aapl":              meta("history", []string{"History().Schedule"}, []int{20, 106}, "read_only", []string{"historical_data"}, []string{"historical session schedule entries"}, 1, "candidate", batchNewV2, batchReadOnly),
	"set_type_live":                         meta("market_data", []string{"MarketData().SetType"}, []int{59, 58}, "read_only", nil, []string{"marketDataType=1 push or real entitlement error"}, 1, "candidate", batchNewV2, batchReadOnly),
	"set_type_frozen":                       meta("market_data", []string{"MarketData().SetType"}, []int{59, 58}, "read_only", nil, []string{"marketDataType=2 push"}, 1, "candidate", batchNewV2, batchReadOnly),
	"set_type_delayed":                      meta("market_data", []string{"MarketData().SetType"}, []int{59, 58}, "read_only", nil, []string{"marketDataType=3 push"}, 1, "candidate", batchNewV2, batchReadOnly),
	"set_type_delayed_frozen":               meta("market_data", []string{"MarketData().SetType"}, []int{59, 58}, "read_only", nil, []string{"marketDataType=4 push"}, 1, "candidate", batchNewV2, batchReadOnly),
	"set_type_invalid":                      meta("market_data", []string{"MarketData().SetType"}, []int{59, 4}, "read_only", nil, []string{"real IBKR invalid data type error"}, 1, "candidate", batchNewV2, batchReadOnly),
	"set_type_switch_while_streaming":       meta("market_data", []string{"MarketData().SetType", "MarketData().SubscribeQuotes"}, []int{59, 1, 2, 58}, "read_only", []string{"market_data_or_delayed_data"}, []string{"setType push observed mid-stream"}, 1, "candidate", batchNewV2, batchReadOnly),
	"quote_snapshot_aapl":                   meta("market_data", []string{"MarketData().Quote"}, []int{1, 2, 57, 58, 59}, "read_only", []string{"market_data_or_delayed_data"}, []string{"snapshot ticks and snapshot end"}, 1, "promoted", batchReadOnly),
	"quote_stream_aapl":                     meta("market_data", []string{"MarketData().SubscribeQuotes"}, []int{1, 2, 45, 46, 58}, "read_only", []string{"market_data_or_delayed_data"}, []string{"stream ticks then cancel"}, 1, "promoted", batchReadOnly),
	"quote_stream_genericticks":             meta("market_data", []string{"MarketData().SubscribeQuotes"}, []int{1, 2, 45, 46, 58}, "read_only", []string{"market_data_or_delayed_data"}, []string{"generic tick stream fields"}, 1, "candidate", batchReadOnly),
	"realtime_bars_aapl":                    meta("market_data", []string{"MarketData().SubscribeRealTimeBars"}, []int{50, 51}, "read_only", []string{"market_data_or_delayed_data"}, []string{"real-time bars then cancel"}, 1, "promoted", batchReadOnly),
	"open_orders_empty":                     meta("orders", []string{"Orders().Open"}, []int{5, 53}, "read_only", nil, []string{"own open-orders snapshot"}, 1, "promoted", batchReadOnly),
	"open_orders_all":                       meta("orders", []string{"Orders().Open"}, []int{16, 53}, "read_only", []string{"client_id_0"}, []string{"all open-orders snapshot"}, 0, "promoted", batchReadOnly),
	"executions_snapshot":                   meta("orders", []string{"Orders().Executions"}, []int{7, 11, 55, 59}, "read_only", nil, []string{"finite execution query and commissions when present"}, 1, "promoted", batchReadOnly),
	"family_codes":                          meta("accounts", []string{"Accounts().FamilyCodes"}, []int{80, 78}, "read_only", nil, []string{"family codes response"}, 1, "promoted", batchReadOnly),
	"news_providers":                        meta("news", []string{"News().Providers"}, []int{85}, "read_only", nil, []string{"free news provider list"}, 1, "promoted", batchReadOnly),
	"mkt_depth_exchanges":                   meta("contracts", []string{"Contracts().DepthExchanges"}, []int{82}, "read_only", nil, []string{"market-depth exchange metadata"}, 1, "promoted", batchReadOnly),
	"scanner_parameters":                    meta("scanner", []string{"Scanner().Parameters"}, []int{24, 19}, "read_only", nil, []string{"scanner XML parameters"}, 1, "promoted", batchReadOnly),
	"user_info":                             meta("tws", []string{"TWS().UserInfo"}, []int{104, 103}, "read_only", nil, []string{"user info response"}, 1, "promoted", batchReadOnly),
	"matching_symbols_aapl":                 meta("contracts", []string{"Contracts().Search"}, []int{81, 79}, "read_only", nil, []string{"exact-ish symbol samples"}, 1, "promoted", batchReadOnly),
	"matching_symbols_partial":              meta("contracts", []string{"Contracts().Search"}, []int{81, 79}, "read_only", nil, []string{"broad symbol samples"}, 1, "candidate", batchReadOnly),
	"head_timestamp_aapl":                   meta("history", []string{"History().HeadTimestamp"}, []int{87, 88, 90}, "read_only", []string{"historical_data"}, []string{"head timestamp response"}, 1, "promoted", batchReadOnly),
	"sec_def_opt_params_aapl":               meta("contracts", []string{"Contracts().SecDefOptParams"}, []int{78, 75, 76}, "read_only", nil, []string{"option parameter surface"}, 1, "promoted", batchReadOnly),
	"histogram_data_aapl":                   meta("history", []string{"History().Histogram"}, []int{88, 89}, "read_only", []string{"historical_data"}, []string{"histogram bins"}, 1, "promoted", batchReadOnly),
	"historical_ticks_aapl_trades":          meta("history", []string{"History().Ticks"}, []int{96, 98}, "read_only", []string{"historical_data"}, []string{"historical last ticks and attributes"}, 1, "promoted", batchReadOnly),
	"historical_ticks_aapl_bidask":          meta("history", []string{"History().Ticks"}, []int{96, 97}, "read_only", []string{"historical_data"}, []string{"historical bid/ask ticks and attributes"}, 1, "promoted", batchReadOnly),
	"historical_ticks_aapl_midpoint":        meta("history", []string{"History().Ticks"}, []int{96}, "read_only", []string{"historical_data"}, []string{"historical midpoint ticks"}, 1, "promoted", batchReadOnly),
	"historical_news_aapl":                  meta("news", []string{"News().Historical"}, []int{86, 87}, "read_only", []string{"news_or_historical_news"}, []string{"historical news items and end marker"}, 1, "promoted", batchReadOnly),
	"historical_ticks_aapl_timezone_window": meta("history", []string{"History().Ticks"}, []int{96, 97, 98}, "read_only", []string{"historical_data"}, []string{"explicit timezone tick windows for all tick kinds"}, 1, "promoted", batchNewV2, batchReadOnly),
	"historical_news_aapl_timezone_window":  meta("news", []string{"News().Historical"}, []int{86, 87}, "read_only", []string{"news_or_historical_news"}, []string{"explicit timezone historical-news window"}, 1, "promoted", batchNewV2, batchReadOnly),
	"completed_orders":                      meta("orders", []string{"Orders().Completed"}, []int{99, 101, 102}, "read_only", nil, []string{"completed order snapshot"}, 1, "promoted", batchReadOnly),
	"quote_with_generic_ticks":              meta("market_data", []string{"MarketData().SubscribeQuotes"}, []int{1, 2, 45, 46, 58}, "read_only", []string{"market_data_or_delayed_data"}, []string{"generic tick list and delayed data"}, 1, "promoted", batchReadOnly),
	"quote_stream_multi_asset":              meta("market_data", []string{"MarketData().SubscribeQuotes"}, []int{1, 2, 45, 46, 58}, "read_only", []string{"market_data_or_delayed_data"}, []string{"concurrent quote streams for multiple asset classes"}, 1, "candidate", batchNewV2, batchReadOnly),
	"account_updates":                       meta("accounts", []string{"Accounts().Updates", "Accounts().SubscribeUpdates"}, []int{6, 7, 8, 54}, "read_only", nil, []string{"account and portfolio update stream"}, 1, "promoted", batchReadOnly),
	"account_updates_multi":                 meta("accounts", []string{"Accounts().UpdatesMulti", "Accounts().SubscribeUpdatesMulti"}, []int{76, 77, 73, 74}, "read_only", nil, []string{"account updates multi stream"}, 1, "promoted", batchReadOnly),
	"positions_multi":                       meta("accounts", []string{"Accounts().PositionsMulti", "Accounts().SubscribePositionsMulti"}, []int{74, 75, 71, 72}, "read_only", nil, []string{"positions multi stream"}, 1, "promoted", batchReadOnly),
	"pnl":                                   meta("accounts", []string{"Accounts().SubscribePnL"}, []int{92, 93, 94}, "read_only", nil, []string{"account PnL stream"}, 1, "promoted", batchReadOnly),
	"pnl_single":                            meta("accounts", []string{"Accounts().SubscribePnLSingle"}, []int{94, 95}, "read_only", nil, []string{"single-position PnL stream or real error"}, 1, "promoted", batchReadOnly),
	"tick_by_tick_last":                     meta("market_data", []string{"MarketData().SubscribeTickByTick"}, []int{97, 98, 99}, "read_only", []string{"market_data_or_delayed_data"}, []string{"tick-by-tick Last stream"}, 1, "candidate", batchReadOnly),
	"tick_by_tick_bidask":                   meta("market_data", []string{"MarketData().SubscribeTickByTick"}, []int{97, 98, 99}, "read_only", []string{"market_data_or_delayed_data"}, []string{"tick-by-tick BidAsk stream"}, 1, "candidate", batchReadOnly),
	"tick_by_tick_midpoint":                 meta("market_data", []string{"MarketData().SubscribeTickByTick"}, []int{97, 98, 99}, "read_only", []string{"market_data_or_delayed_data"}, []string{"tick-by-tick MidPoint stream"}, 1, "candidate", batchReadOnly),
	"historical_bars_keepup":                meta("history", []string{"History().SubscribeBars"}, []int{20, 25, 108}, "read_only", []string{"historical_data"}, []string{"keep-up-to-date historical bars"}, 1, "promoted", batchReadOnly),
	"news_bulletins":                        meta("news", []string{"News().SubscribeBulletins"}, []int{12, 13, 14}, "read_only", []string{"news_or_bulletins"}, []string{"news bulletin subscribe/cancel"}, 1, "promoted", batchReadOnly),
	"scanner_subscription":                  meta("scanner", []string{"Scanner().SubscribeResults"}, []int{22, 23, 20}, "read_only", nil, []string{"scanner rows or real subscription error"}, 1, "promoted", batchReadOnly),
	"smart_components":                      meta("contracts", []string{"Contracts().SmartComponents"}, []int{83, 82}, "read_only", nil, []string{"smart component mapping"}, 1, "promoted", batchReadOnly),
	"market_rule":                           meta("contracts", []string{"Contracts().MarketRule"}, []int{91, 92}, "read_only", nil, []string{"price increment ladder"}, 1, "promoted", batchReadOnly),
	"place_order_lmt_buy_aapl":              meta("orders", []string{"Orders().Place", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"far-from-market order accepted then cancelled"}, 1, "candidate", batchNewV2, batchTrading),
	"place_order_mkt_buy_aapl":              meta("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_marketable_order", []string{"paper_trading", "market_hours"}, []string{"market buy fill or real market-state response"}, 1, "candidate", batchNewV2, batchTrading),
	"place_order_mkt_sell_aapl":             meta("orders", []string{"Orders().Place"}, []int{3, 5, 11, 59}, "paper_marketable_order", []string{"paper_trading", "market_hours", "position_or_short_permission"}, []string{"market sell fill or real rejection"}, 1, "candidate", batchNewV2, batchTrading),
	"place_order_modify":                    meta("orders", []string{"Orders().Place", "OrderHandle.Modify", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"modify accepted and open-order update observed"}, 1, "promoted", batchNewV2, batchTrading),
	"place_order_cancel":                    meta("orders", []string{"Orders().Place", "OrderHandle.Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"cancel terminal status"}, 1, "promoted", batchNewV2, batchTrading),
	"place_order_direct_cancel":             meta("orders", []string{"Orders().Place", "Orders().Cancel"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"direct Orders().Cancel(orderID) terminal status"}, 1, "candidate", batchNewV2, batchTrading),
	"place_order_bracket_aapl":              meta("orders", []string{"Orders().Place"}, []int{3, 5}, "paper_marketable_order", []string{"paper_trading", "market_hours"}, []string{"parent/child transmit sequencing"}, 1, "candidate", batchNewV2, batchTrading),
	"global_cancel":                         meta("orders", []string{"Orders().Place", "Orders().CancelAll"}, []int{3, 5, 58}, "paper_order", []string{"paper_trading"}, []string{"multiple open orders cancelled globally"}, 1, "promoted", batchNewV2, batchTrading),
	"place_order_option_buy":                meta("options", []string{"Orders().Place"}, []int{3, 5}, "paper_marketable_order", []string{"paper_trading", "option_permissions"}, []string{"option order fill or real contract/permission error"}, 1, "candidate", batchNewV2, batchTrading),
	"place_order_algo_adaptive_aapl":        meta("orders", []string{"Orders().Place"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"Adaptive algo open-order wire"}, 1, "candidate", batchNewV2, batchTrading),
	"place_order_price_condition_aapl":      meta("orders", []string{"Orders().Place"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"price condition open-order wire"}, 1, "candidate", batchNewV2, batchTrading),
	"place_order_oca_pair_aapl":             meta("orders", []string{"Orders().Place", "Orders().CancelAll"}, []int{3, 5, 58}, "paper_order", []string{"paper_trading"}, []string{"OCA pair accepted then globally cancelled"}, 1, "candidate", batchNewV2, batchTrading),
	"trading_split_round_trip_aapl":         meta("orders", []string{"Accounts().Summary", "Accounts().Positions", "Orders().Place", "Orders().Executions", "Orders().Completed", "Accounts().SubscribePnL"}, []int{3, 5, 7, 11, 55, 59, 61, 62, 63, 64, 92, 93, 99, 101, 102}, "paper_marketable_order", []string{"paper_trading", "market_hours"}, []string{"split buy/sell round trip with account/order reconciliation"}, 1, "candidate", batchNewV2, batchTrading),
	"market_depth_aapl":                     meta("market_data", []string{"MarketData().SubscribeDepth"}, []int{10, 11, 12, 13}, "entitlement_probe", []string{"l2_market_data_or_error"}, []string{"regular depth rows or entitlement error"}, 1, "promoted", batchNewV2, batchReadOnly),
	"market_depth_aapl_smart":               meta("market_data", []string{"MarketData().SubscribeDepth"}, []int{10, 11, 12, 13}, "entitlement_probe", []string{"l2_market_data_or_error"}, []string{"smart depth rows or entitlement error"}, 1, "candidate", batchNewV2, batchReadOnly),
	"fundamental_data_aapl":                 meta("contracts", []string{"Contracts().FundamentalData"}, []int{52, 53, 51}, "entitlement_probe", []string{"fundamental_data_subscription_or_error"}, []string{"fundamental XML or entitlement error"}, 1, "promoted", batchNewV2, batchReadOnly),
	"soft_dollar_tiers":                     meta("advisors", []string{"Advisors().SoftDollarTiers"}, []int{79, 77}, "read_only", nil, []string{"soft-dollar tier list"}, 1, "promoted", batchNewV2, batchReadOnly),
	"display_groups":                        meta("tws", []string{"TWS().DisplayGroups"}, []int{67}, "read_only", nil, []string{"display group list"}, 1, "promoted", batchNewV2, batchReadOnly),
	"display_group_subscribe":               meta("tws", []string{"TWS().SubscribeDisplayGroup", "DisplayGroupHandle.Update"}, []int{67, 68, 69, 70}, "read_only", []string{"tws_display_groups"}, []string{"display group subscribe/update/unsubscribe"}, 1, "promoted", batchNewV2, batchReadOnly),
	"wsh_meta_data":                         meta("wsh", []string{"WSH().MetaData"}, []int{100, 101, 104}, "entitlement_probe", []string{"wsh_subscription_or_error"}, []string{"WSH metadata JSON or entitlement error"}, 1, "promoted", batchNewV2, batchReadOnly),
	"wsh_event_data_aapl":                   meta("wsh", []string{"WSH().EventData"}, []int{102, 103, 105}, "entitlement_probe", []string{"wsh_subscription_or_error"}, []string{"WSH event JSON or entitlement error"}, 1, "candidate", batchNewV2, batchReadOnly),
	"request_fa":                            meta("advisors", []string{"Advisors().Config"}, []int{18, 16}, "entitlement_probe", []string{"fa_account_or_error"}, []string{"FA XML or non-FA error"}, 1, "promoted", batchNewV2, batchReadOnly),
	"qualify_contract_aapl_exact":           meta("contracts", []string{"Contracts().Qualify"}, []int{9, 10, 52}, "read_only", nil, []string{"single qualified contract"}, 1, "promoted", batchNewV2, batchReadOnly),
	"qualify_contract_ambiguous":            meta("contracts", []string{"Contracts().Qualify", "Contracts().Details"}, []int{9, 10, 52, 4}, "read_only", nil, []string{"ambiguous contract results or real ambiguity error"}, 1, "candidate", batchNewV2, batchReadOnly),

	"api_order_fill_aapl":             apiMeta("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Modify", "Orders().Executions"}, []int{1, 2, 3, 5, 11, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"MKT, MTL, and delayed modify-to-market fill paths with flattening"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
	"api_order_rest_cancel_aapl":      apiMeta("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"far LMT rest/cancel path"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
	"api_order_relative_cancel_aapl":  apiMeta("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"REL rest/cancel behavior isolated because Gateway can reconnect during relative order validation"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
	"api_order_trailing_cancel_aapl":  apiMeta("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"TRAIL and TRAIL LIMIT behavior isolated because Gateway can reconnect during trailing validation"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
	"api_order_stop_cancel_aapl":      apiMeta("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"STP and STP LMT rest/cancel behavior isolated because Gateway can reconnect during stop validation"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
	"api_order_rejects_aapl":          apiMeta("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "Orders().Cancel"}, []int{1, 2, 3, 4, 57, 58}, "paper_order", []string{"paper_trading"}, []string{"invalid order type, price band, invalid contract, and unknown cancel real Gateway errors"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
	"api_order_type_matrix_aapl":      apiMeta("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Modify", "OrderHandle.Cancel", "Orders().Executions"}, []int{1, 2, 3, 4, 5, 11, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"MKT/LMT/STP/STP LMT/TRAIL/TRAIL LIMIT/MIT/LIT/MTL/REL/MOC/LOC/MOO/LOO/PEG families accepted, rejected, filled, modified, or cancelled with real order lifecycle"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
	"api_delayed_success_modify_aapl": apiMeta("orders", []string{"Orders().Place", "OrderHandle.Modify", "Orders().Executions", "Accounts().Positions"}, []int{3, 4, 5, 11, 59, 61, 62}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"resting limit order later becomes marketable through modify and is observed through the original handle"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayDefault, batchReplayAll),
	"api_bracket_trigger_aapl":        apiMeta("orders", []string{"Orders().Place", "OrderHandle.Modify", "Orders().Open", "Orders().Executions", "Orders().CancelAll"}, []int{3, 4, 5, 11, 16, 53, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"bracket parent fills, take-profit is forced marketable, sibling stop-loss cancellation is observed"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayDefault, batchReplayAll),
	"api_oca_trigger_aapl":            apiMeta("orders", []string{"Orders().Place", "Orders().Open", "Orders().Executions", "Orders().CancelAll"}, []int{3, 4, 5, 11, 16, 53, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"marketable OCA peer fills and cancels resting sibling"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayDefault, batchReplayAll),
	"api_conditions_matrix_aapl":      apiMeta("orders", []string{"Orders().Place", "OrderHandle.Cancel", "Orders().CancelAll"}, []int{3, 4, 5}, "paper_order", []string{"paper_trading"}, []string{"price/time/margin/execution/volume/percent-change condition families accepted or rejected with real Gateway response"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
	"api_option_campaign_aapl":        apiMeta("options", []string{"Contracts().SecDefOptParams", "Contracts().Qualify", "MarketData().Quote", "Options().Price", "Options().Exercise", "Orders().Place", "Orders().Executions", "Orders().Completed"}, []int{1, 2, 3, 5, 11, 21, 55, 59, 75, 76, 99, 101, 102}, "paper_trigger", []string{"paper_trading", "market_hours", "option_permissions"}, []string{"live-qualified AAPL option quote/calculation/order/exercise-or-real-reject campaign"}, 1, "candidate", []string{"OPT"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
	"api_future_campaign_mes":         apiMeta("orders", []string{"Contracts().Details", "MarketData().Quote", "Orders().Place", "Orders().Executions", "Accounts().Positions", "Orders().CancelAll"}, []int{1, 2, 3, 5, 10, 11, 52, 57, 58, 59, 61, 62}, "paper_trigger", []string{"paper_trading", "market_hours", "future_permissions"}, []string{"live-qualified MES future order/modify/round-trip or real permission rejection"}, 1, "candidate", []string{"FUT"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
	"api_combo_option_vertical_aapl":  apiMeta("orders", []string{"Contracts().SecDefOptParams", "Contracts().Qualify", "Orders().Place", "Orders().Open", "Orders().CancelAll"}, []int{3, 4, 5, 16, 53, 58, 75, 76}, "paper_order", []string{"paper_trading", "option_permissions"}, []string{"live-qualified AAPL option BAG vertical accepted/cancelled or real combo rejection"}, 1, "candidate", []string{"BAG", "OPT"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
	"api_algorithmic_campaign_aapl":   apiMeta("orders", []string{"Accounts().Summary", "Accounts().SubscribeUpdates", "Accounts().SubscribePnL", "Accounts().Positions", "MarketData().SubscribeQuotes", "Orders().SubscribeOpen", "Orders().Place", "OrderHandle.Modify", "Orders().Executions", "Orders().Completed", "Orders().CancelAll"}, []int{1, 2, 3, 5, 6, 7, 8, 11, 16, 53, 54, 58, 59, 61, 62, 63, 64, 92, 93, 99, 101, 102}, "paper_destructive", []string{"paper_trading", "market_hours"}, []string{"multi-subscription algorithmic campaign with split fills, resting modifies, reconciliation, and cleanup"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
	"api_forex_lifecycle_eurusd":      apiMeta("orders", []string{"MarketData().SetType", "MarketData().Quote", "Orders().Place", "OrderHandle.Modify", "OrderHandle.Cancel"}, []int{1, 2, 3, 4, 5, 57, 58}, "paper_order", []string{"paper_trading", "forex_hours"}, []string{"EUR.USD rest/modify/cancel lifecycle"}, 1, "candidate", []string{"CASH"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
	"api_whatif_margin_aapl":          apiMeta("orders", []string{"Orders().Place"}, []int{3, 5}, "paper_order", []string{"paper_trading"}, []string{"WhatIf margin/commission preview without execution"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
	"api_stress_rapid_fire_aapl":      apiMeta("orders", []string{"Orders().Place", "Orders().CancelAll"}, []int{3, 4, 5, 58}, "paper_order", []string{"paper_trading"}, []string{"10 rapid-fire far LMT orders plus global cancel"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingAdvanced, batchTradingAll, batchReplayAll),
	"api_scale_in_campaign_aapl":      apiMeta("orders", []string{"MarketData().Quote", "Orders().Place", "OrderHandle.Cancel", "Orders().Executions", "Accounts().Positions"}, []int{1, 2, 3, 4, 5, 11, 57, 58, 59, 61, 62}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"scale-in 2x MKT buy, protective stop-loss, flatten, execution query"}, 1, "candidate", []string{"STK"}, batchNewV2, batchTrading, batchTradingCampaigns, batchTradingAll, batchReplayAll),
	"api_ioc_fok_aapl":                apiMeta("orders", []string{"MarketData().Quote", "Orders().Place", "Orders().Executions"}, []int{1, 2, 3, 5, 11, 57, 58, 59}, "paper_trigger", []string{"paper_trading", "market_hours"}, []string{"IOC marketable cancel and FOK invalid/inactive paths"}, 1, "promoted", []string{"STK"}, batchNewV2, batchTrading, batchTradingBasic, batchTradingAll, batchReplayAll),
}

func meta(domain string, publicAPI []string, messageIDs []int, riskClass string, requirements []string, expected []string, defaultClientID int, promotionStatus string, batches ...string) scenarioMetadata {
	return scenarioMetadata{
		Domain:           domain,
		Driver:           driverWire,
		PublicAPI:        publicAPI,
		MessageIDs:       messageIDs,
		RiskClass:        riskClass,
		Requirements:     requirements,
		ExpectedOutcomes: expected,
		DefaultClientID:  defaultClientID,
		Batches:          append([]string{batchAll}, batches...),
		PromotionStatus:  promotionStatus,
		DefaultReplay:    promotionStatus == "promoted",
	}
}

func apiMeta(domain string, publicAPI []string, messageIDs []int, riskClass string, requirements []string, expected []string, defaultClientID int, promotionStatus string, assets []string, batches ...string) scenarioMetadata {
	md := meta(domain, publicAPI, messageIDs, riskClass, requirements, expected, defaultClientID, promotionStatus, batches...)
	md.Driver = driverAPI
	md.Assets = append([]string(nil), assets...)
	return md
}

func catalogEntries() ([]scenarioCatalogEntry, error) {
	names := make([]string, 0, len(scenarios))
	for name := range scenarios {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]scenarioCatalogEntry, 0, len(names))
	for _, name := range names {
		sc := scenarios[name]
		md, ok := scenarioMetadataByName[name]
		if !ok {
			return nil, fmt.Errorf("missing scenario metadata for %s", name)
		}
		entries = append(entries, scenarioCatalogEntry{
			Name:             name,
			Description:      sc.description,
			scenarioMetadata: md,
		})
	}
	return entries, nil
}

func writeCatalogJSON(w io.Writer) error {
	entries, err := catalogEntries()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func writeBatchList(w io.Writer, batch string) error {
	if batch == "" {
		batch = batchNewV2
	}
	entries, err := catalogEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.inBatch(batch) && !entry.inReplayDefaultBatch(batch) {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s|%d\n", entry.Name, entry.DefaultClientID); err != nil {
			return err
		}
	}
	return nil
}

func (e scenarioCatalogEntry) inBatch(batch string) bool {
	if batch == batchReplayAll {
		return true
	}
	for _, candidate := range e.Batches {
		if candidate == batch {
			return true
		}
	}
	return false
}

func (e scenarioCatalogEntry) inReplayDefaultBatch(batch string) bool {
	return batch == batchReplayDefault && e.DefaultReplay
}
