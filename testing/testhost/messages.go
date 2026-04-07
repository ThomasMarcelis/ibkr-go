package testhost

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func buildMessage(name string, body map[string]any, bindings map[string]any) (codec.Message, error) {
	resolve := func(v any) any { return resolveBindings(v, bindings) }

	switch name {
	case "server_info":
		return codec.ServerInfo{ServerVersion: asInt(resolve(body["server_version"])), ConnectionTime: asString(resolve(body["connection_time"]))}, nil
	case "managed_accounts":
		return codec.ManagedAccounts{Accounts: asStrings(resolve(body["accounts"]))}, nil
	case "next_valid_id":
		return codec.NextValidID{OrderID: int64(asInt(resolve(body["order_id"])))}, nil
	case "current_time":
		return codec.CurrentTime{Time: asString(resolve(body["time"]))}, nil
	case "api_error":
		return codec.APIError{ReqID: asInt(resolve(body["req_id"])), Code: asInt(resolve(body["code"])), Message: asString(resolve(body["message"])), AdvancedOrderRejectJSON: asString(resolve(body["advanced_order_reject_json"])), ErrorTimeMs: asString(resolve(body["error_time_ms"]))}, nil
	case "contract_details":
		return codec.ContractDetails{
			ReqID:      asInt(resolve(body["req_id"])),
			Contract:   asContract(resolve(body["contract"])),
			MarketName: asString(resolve(body["market_name"])),
			MinTick:    asString(resolve(body["min_tick"])),
			LongName:   asString(resolve(body["long_name"])),
			TimeZoneID: asString(resolve(body["time_zone_id"])),
		}, nil
	case "contract_details_end":
		return codec.ContractDetailsEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "historical_bar":
		return codec.HistoricalBar{ReqID: asInt(resolve(body["req_id"])), Time: asString(resolve(body["time"])), Open: asString(resolve(body["open"])), High: asString(resolve(body["high"])), Low: asString(resolve(body["low"])), Close: asString(resolve(body["close"])), Volume: asString(resolve(body["volume"]))}, nil
	case "historical_bars_end":
		return codec.HistoricalBarsEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "account_summary":
		return codec.AccountSummaryValue{ReqID: asInt(resolve(body["req_id"])), Account: asString(resolve(body["account"])), Tag: asString(resolve(body["tag"])), Value: asString(resolve(body["value"])), Currency: asString(resolve(body["currency"]))}, nil
	case "account_summary_end":
		return codec.AccountSummaryEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "position":
		return codec.Position{Account: asString(resolve(body["account"])), Contract: asContract(resolve(body["contract"])), Position: asString(resolve(body["position"])), AvgCost: asString(resolve(body["avg_cost"]))}, nil
	case "position_end":
		return codec.PositionEnd{}, nil
	case "tick_price":
		return codec.TickPrice{ReqID: asInt(resolve(body["req_id"])), TickType: asInt(resolve(body["field"])), Price: asString(resolve(body["price"]))}, nil
	case "tick_size":
		return codec.TickSize{ReqID: asInt(resolve(body["req_id"])), TickType: asInt(resolve(body["field"])), Size: asString(resolve(body["size"]))}, nil
	case "market_data_type":
		return codec.MarketDataType{ReqID: asInt(resolve(body["req_id"])), DataType: asInt(resolve(body["data_type"]))}, nil
	case "tick_snapshot_end":
		return codec.TickSnapshotEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "realtime_bar":
		return codec.RealTimeBar{ReqID: asInt(resolve(body["req_id"])), Time: asString(resolve(body["time"])), Open: asString(resolve(body["open"])), High: asString(resolve(body["high"])), Low: asString(resolve(body["low"])), Close: asString(resolve(body["close"])), Volume: asString(resolve(body["volume"]))}, nil
	case "open_order":
		return codec.OpenOrder{
			OrderID:       int64(asInt(resolve(body["order_id"]))),
			Account:       asString(resolve(body["account"])),
			Contract:      asContract(resolve(body["contract"])),
			Action:        asString(resolve(body["action"])),
			OrderType:     asString(resolve(body["order_type"])),
			Status:        asString(resolve(body["status"])),
			Quantity:      asString(resolve(body["quantity"])),
			Filled:        asString(resolve(body["filled"])),
			Remaining:     asString(resolve(body["remaining"])),
			LmtPrice:      asString(resolve(body["lmt_price"])),
			AuxPrice:      asString(resolve(body["aux_price"])),
			TIF:           asString(resolve(body["tif"])),
			OcaGroup:      asString(resolve(body["oca_group"])),
			OpenClose:     asString(resolve(body["open_close"])),
			Origin:        asString(resolve(body["origin"])),
			OrderRef:      asString(resolve(body["order_ref"])),
			ClientID:      asString(resolve(body["client_id"])),
			PermID:        asString(resolve(body["perm_id"])),
			OutsideRTH:    asString(resolve(body["outside_rth"])),
			Hidden:        asString(resolve(body["hidden"])),
			DiscretionAmt: asString(resolve(body["discretion_amt"])),
			GoodAfterTime: asString(resolve(body["good_after_time"])),
		}, nil
	case "open_order_end":
		return codec.OpenOrderEnd{}, nil
	case "order_status":
		return codec.OrderStatus{
			OrderID:       int64(asInt(resolve(body["order_id"]))),
			Status:        asString(resolve(body["status"])),
			Filled:        asString(resolve(body["filled"])),
			Remaining:     asString(resolve(body["remaining"])),
			AvgFillPrice:  asString(resolve(body["avg_fill_price"])),
			PermID:        asString(resolve(body["perm_id"])),
			ParentID:      asString(resolve(body["parent_id"])),
			LastFillPrice: asString(resolve(body["last_fill_price"])),
			ClientID:      asString(resolve(body["client_id"])),
			WhyHeld:       asString(resolve(body["why_held"])),
			MktCapPrice:   asString(resolve(body["mkt_cap_price"])),
		}, nil
	case "execution_detail":
		return codec.ExecutionDetail{ReqID: asInt(resolve(body["req_id"])), OrderID: int64(asInt(resolve(body["order_id"]))), ExecID: asString(resolve(body["exec_id"])), Account: asString(resolve(body["account"])), Symbol: asString(resolve(body["symbol"])), Side: asString(resolve(body["side"])), Shares: asString(resolve(body["shares"])), Price: asString(resolve(body["price"])), Time: asString(resolve(body["time"]))}, nil
	case "executions_end":
		return codec.ExecutionsEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "commission_report":
		return codec.CommissionReport{ExecID: asString(resolve(body["exec_id"])), Commission: asString(resolve(body["commission"])), Currency: asString(resolve(body["currency"])), RealizedPNL: asString(resolve(body["realized_pnl"]))}, nil
	case "family_codes":
		entries := asCodecEntries(resolve(body["codes"]), func(m map[string]any) codec.FamilyCodeEntry {
			return codec.FamilyCodeEntry{AccountID: asString(m["account_id"]), FamilyCode: asString(m["family_code"])}
		})
		return codec.FamilyCodes{Codes: entries}, nil
	case "mkt_depth_exchanges":
		entries := asCodecEntries(resolve(body["exchanges"]), func(m map[string]any) codec.DepthExchangeEntry {
			return codec.DepthExchangeEntry{
				Exchange: asString(m["exchange"]), SecType: asString(m["sec_type"]),
				ListingExch: asString(m["listing_exch"]), ServiceDataType: asString(m["service_data_type"]),
				AggGroup: asInt(m["agg_group"]),
			}
		})
		return codec.MktDepthExchanges{Exchanges: entries}, nil
	case "news_providers":
		entries := asCodecEntries(resolve(body["providers"]), func(m map[string]any) codec.NewsProviderEntry {
			return codec.NewsProviderEntry{Code: asString(m["code"]), Name: asString(m["name"])}
		})
		return codec.NewsProviders{Providers: entries}, nil
	case "scanner_parameters":
		return codec.ScannerParameters{XML: asString(resolve(body["xml"]))}, nil
	case "user_info":
		return codec.UserInfo{ReqID: asInt(resolve(body["req_id"])), WhiteBrandingID: asString(resolve(body["white_branding_id"]))}, nil
	case "matching_symbols":
		entries := asCodecEntries(resolve(body["symbols"]), func(m map[string]any) codec.SymbolSample {
			return codec.SymbolSample{
				ConID: asInt(m["con_id"]), Symbol: asString(m["symbol"]),
				SecType: asString(m["sec_type"]), PrimaryExchange: asString(m["primary_exchange"]),
				Currency: asString(m["currency"]), DerivativeSecTypes: asStrings(m["derivative_sec_types"]),
			}
		})
		return codec.MatchingSymbols{ReqID: asInt(resolve(body["req_id"])), Symbols: entries}, nil
	case "head_timestamp":
		return codec.HeadTimestamp{ReqID: asInt(resolve(body["req_id"])), Timestamp: asString(resolve(body["timestamp"]))}, nil
	case "market_rule":
		entries := asCodecEntries(resolve(body["increments"]), func(m map[string]any) codec.PriceIncrement {
			return codec.PriceIncrement{LowEdge: asString(m["low_edge"]), Increment: asString(m["increment"])}
		})
		return codec.MarketRule{MarketRuleID: asInt(resolve(body["market_rule_id"])), Increments: entries}, nil
	case "completed_order":
		return codec.CompletedOrder{
			Contract: asContract(resolve(body["contract"])), Action: asString(resolve(body["action"])),
			OrderType: asString(resolve(body["order_type"])), Status: asString(resolve(body["status"])),
			Quantity: asString(resolve(body["quantity"])), Filled: asString(resolve(body["filled"])),
			Remaining: asString(resolve(body["remaining"])),
		}, nil
	case "completed_order_end":
		return codec.CompletedOrderEnd{}, nil
	case "tick_generic":
		return codec.TickGeneric{ReqID: asInt(resolve(body["req_id"])), TickType: asInt(resolve(body["field"])), Value: asString(resolve(body["value"]))}, nil
	case "tick_string":
		return codec.TickString{ReqID: asInt(resolve(body["req_id"])), TickType: asInt(resolve(body["field"])), Value: asString(resolve(body["value"]))}, nil
	case "tick_req_params":
		return codec.TickReqParams{ReqID: asInt(resolve(body["req_id"])), MinTick: asString(resolve(body["min_tick"])), BBOExchange: asString(resolve(body["bbo_exchange"])), SnapshotPermissions: asInt(resolve(body["snapshot_permissions"]))}, nil
	case "update_account_value":
		return codec.UpdateAccountValue{Key: asString(resolve(body["key"])), Value: asString(resolve(body["value"])), Currency: asString(resolve(body["currency"])), Account: asString(resolve(body["account"]))}, nil
	case "update_portfolio":
		return codec.UpdatePortfolio{
			Contract: asContract(resolve(body["contract"])),
			Position: asString(resolve(body["position"])), MarketPrice: asString(resolve(body["market_price"])),
			MarketValue: asString(resolve(body["market_value"])), AvgCost: asString(resolve(body["avg_cost"])),
			UnrealizedPNL: asString(resolve(body["unrealized_pnl"])), RealizedPNL: asString(resolve(body["realized_pnl"])),
			Account: asString(resolve(body["account"])),
		}, nil
	case "update_account_time":
		return codec.UpdateAccountTime{Timestamp: asString(resolve(body["timestamp"]))}, nil
	case "account_download_end":
		return codec.AccountDownloadEnd{Account: asString(resolve(body["account"]))}, nil
	case "account_update_multi":
		return codec.AccountUpdateMultiValue{ReqID: asInt(resolve(body["req_id"])), Account: asString(resolve(body["account"])), ModelCode: asString(resolve(body["model_code"])), Key: asString(resolve(body["key"])), Value: asString(resolve(body["value"])), Currency: asString(resolve(body["currency"]))}, nil
	case "account_update_multi_end":
		return codec.AccountUpdateMultiEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "position_multi":
		return codec.PositionMulti{ReqID: asInt(resolve(body["req_id"])), Account: asString(resolve(body["account"])), ModelCode: asString(resolve(body["model_code"])), Contract: asContract(resolve(body["contract"])), Position: asString(resolve(body["position"])), AvgCost: asString(resolve(body["avg_cost"]))}, nil
	case "position_multi_end":
		return codec.PositionMultiEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "pnl":
		return codec.PnLValue{ReqID: asInt(resolve(body["req_id"])), DailyPnL: asString(resolve(body["daily_pnl"])), UnrealizedPnL: asString(resolve(body["unrealized_pnl"])), RealizedPnL: asString(resolve(body["realized_pnl"]))}, nil
	case "pnl_single":
		return codec.PnLSingleValue{ReqID: asInt(resolve(body["req_id"])), Position: asString(resolve(body["position"])), DailyPnL: asString(resolve(body["daily_pnl"])), UnrealizedPnL: asString(resolve(body["unrealized_pnl"])), RealizedPnL: asString(resolve(body["realized_pnl"])), Value: asString(resolve(body["value"]))}, nil
	case "tick_by_tick":
		return codec.TickByTickData{
			ReqID: asInt(resolve(body["req_id"])), TickType: asInt(resolve(body["tick_type"])),
			Time: asString(resolve(body["time"])), Price: asString(resolve(body["price"])),
			Size: asString(resolve(body["size"])), Exchange: asString(resolve(body["exchange"])),
			SpecialConditions: asString(resolve(body["special_conditions"])),
			BidPrice:          asString(resolve(body["bid_price"])), AskPrice: asString(resolve(body["ask_price"])),
			BidSize: asString(resolve(body["bid_size"])), AskSize: asString(resolve(body["ask_size"])),
			MidPoint:         asString(resolve(body["mid_point"])),
			TickAttribLast:   asInt(resolve(body["tick_attrib_last"])),
			TickAttribBidAsk: asInt(resolve(body["tick_attrib_bid_ask"])),
		}, nil
	case "news_bulletin":
		return codec.NewsBulletin{MsgID: asInt(resolve(body["msg_id"])), MsgType: asInt(resolve(body["msg_type"])), Headline: asString(resolve(body["headline"])), Source: asString(resolve(body["source"]))}, nil
	case "historical_data_update":
		return codec.HistoricalDataUpdate{ReqID: asInt(resolve(body["req_id"])), BarCount: asInt(resolve(body["bar_count"])), Time: asString(resolve(body["time"])), Open: asString(resolve(body["open"])), High: asString(resolve(body["high"])), Low: asString(resolve(body["low"])), Close: asString(resolve(body["close"])), Volume: asString(resolve(body["volume"])), WAP: asString(resolve(body["wap"])), Count: asString(resolve(body["count"]))}, nil
	case "sec_def_opt_params":
		return codec.SecDefOptParamsResponse{
			ReqID: asInt(resolve(body["req_id"])), Exchange: asString(resolve(body["exchange"])),
			UnderlyingConID: asInt(resolve(body["underlying_con_id"])), TradingClass: asString(resolve(body["trading_class"])),
			Multiplier:  asString(resolve(body["multiplier"])),
			Expirations: asStrings(resolve(body["expirations"])), Strikes: asStrings(resolve(body["strikes"])),
		}, nil
	case "sec_def_opt_params_end":
		return codec.SecDefOptParamsEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "smart_components":
		entries := asCodecEntries(resolve(body["components"]), func(m map[string]any) codec.SmartComponentEntry {
			return codec.SmartComponentEntry{BitNumber: asInt(m["bit_number"]), ExchangeName: asString(m["exchange_name"]), ExchangeLetter: asString(m["exchange_letter"])}
		})
		return codec.SmartComponentsResponse{ReqID: asInt(resolve(body["req_id"])), Components: entries}, nil
	case "tick_option_computation":
		return codec.TickOptionComputation{
			ReqID: asInt(resolve(body["req_id"])), TickType: asInt(resolve(body["tick_type"])), TickAttrib: asInt(resolve(body["tick_attrib"])),
			ImpliedVol: asString(resolve(body["implied_vol"])), Delta: asString(resolve(body["delta"])),
			OptPrice: asString(resolve(body["opt_price"])), PvDividend: asString(resolve(body["pv_dividend"])),
			Gamma: asString(resolve(body["gamma"])), Vega: asString(resolve(body["vega"])),
			Theta: asString(resolve(body["theta"])), UndPrice: asString(resolve(body["und_price"])),
		}, nil
	case "histogram_data":
		entries := asCodecEntries(resolve(body["entries"]), func(m map[string]any) codec.HistogramDataEntry {
			return codec.HistogramDataEntry{Price: asString(m["price"]), Size: asString(m["size"])}
		})
		return codec.HistogramDataResponse{ReqID: asInt(resolve(body["req_id"])), Entries: entries}, nil
	case "historical_ticks":
		entries := asCodecEntries(resolve(body["ticks"]), func(m map[string]any) codec.HistoricalTickEntry {
			return codec.HistoricalTickEntry{Time: asString(m["time"]), Price: asString(m["price"]), Size: asString(m["size"])}
		})
		return codec.HistoricalTicksResponse{ReqID: asInt(resolve(body["req_id"])), Ticks: entries, Done: asBool(resolve(body["done"]))}, nil
	case "historical_ticks_bid_ask":
		entries := asCodecEntries(resolve(body["ticks"]), func(m map[string]any) codec.HistoricalTickBidAskEntry {
			return codec.HistoricalTickBidAskEntry{
				Time: asString(m["time"]), BidPrice: asString(m["bid_price"]), AskPrice: asString(m["ask_price"]),
				BidSize: asString(m["bid_size"]), AskSize: asString(m["ask_size"]),
			}
		})
		return codec.HistoricalTicksBidAskResponse{ReqID: asInt(resolve(body["req_id"])), Ticks: entries, Done: asBool(resolve(body["done"]))}, nil
	case "historical_ticks_last":
		entries := asCodecEntries(resolve(body["ticks"]), func(m map[string]any) codec.HistoricalTickLastEntry {
			return codec.HistoricalTickLastEntry{
				Time: asString(m["time"]), Price: asString(m["price"]), Size: asString(m["size"]),
				Exchange: asString(m["exchange"]), SpecialConditions: asString(m["special_conditions"]),
			}
		})
		return codec.HistoricalTicksLastResponse{ReqID: asInt(resolve(body["req_id"])), Ticks: entries, Done: asBool(resolve(body["done"]))}, nil
	case "news_article":
		return codec.NewsArticleResponse{ReqID: asInt(resolve(body["req_id"])), ArticleType: asInt(resolve(body["article_type"])), ArticleText: asString(resolve(body["article_text"]))}, nil
	case "historical_news":
		return codec.HistoricalNewsItem{ReqID: asInt(resolve(body["req_id"])), Time: asString(resolve(body["time"])), ProviderCode: asString(resolve(body["provider_code"])), ArticleID: asString(resolve(body["article_id"])), Headline: asString(resolve(body["headline"]))}, nil
	case "historical_news_end":
		return codec.HistoricalNewsEnd{ReqID: asInt(resolve(body["req_id"])), HasMore: asBool(resolve(body["has_more"]))}, nil
	case "market_depth":
		return codec.MarketDepthUpdate{
			ReqID: asInt(resolve(body["req_id"])), Position: asInt(resolve(body["position"])),
			Operation: asInt(resolve(body["operation"])), Side: asInt(resolve(body["side"])),
			Price: asString(resolve(body["price"])), Size: asString(resolve(body["size"])),
		}, nil
	case "market_depth_l2":
		return codec.MarketDepthL2Update{
			ReqID: asInt(resolve(body["req_id"])), Position: asInt(resolve(body["position"])),
			MarketMaker: asString(resolve(body["market_maker"])),
			Operation:   asInt(resolve(body["operation"])), Side: asInt(resolve(body["side"])),
			Price: asString(resolve(body["price"])), Size: asString(resolve(body["size"])),
			IsSmartDepth: asBool(resolve(body["is_smart_depth"])),
		}, nil
	case "fundamental_data":
		return codec.FundamentalDataResponse{ReqID: asInt(resolve(body["req_id"])), Data: asString(resolve(body["data"]))}, nil
	case "scanner_data":
		entries := asCodecEntries(resolve(body["entries"]), func(m map[string]any) codec.ScannerDataEntry {
			return codec.ScannerDataEntry{
				Rank: asInt(m["rank"]), Contract: asContract(m["contract"]),
				Distance: asString(m["distance"]), Benchmark: asString(m["benchmark"]),
				Projection: asString(m["projection"]), LegsStr: asString(m["legs_str"]),
			}
		})
		return codec.ScannerDataResponse{ReqID: asInt(resolve(body["req_id"])), Entries: entries}, nil
	case "receive_fa":
		return codec.ReceiveFA{FADataType: asInt(resolve(body["fa_data_type"])), XML: asString(resolve(body["xml"]))}, nil
	case "soft_dollar_tiers":
		entries := asCodecEntries(resolve(body["tiers"]), func(m map[string]any) codec.SoftDollarTier {
			return codec.SoftDollarTier{Name: asString(m["name"]), Value: asString(m["value"]), DisplayName: asString(m["display_name"])}
		})
		return codec.SoftDollarTiersResponse{ReqID: asInt(resolve(body["req_id"])), Tiers: entries}, nil
	case "wsh_meta_data":
		return codec.WSHMetaDataResponse{ReqID: asInt(resolve(body["req_id"])), DataJSON: asString(resolve(body["data_json"]))}, nil
	case "wsh_event_data":
		return codec.WSHEventDataResponse{ReqID: asInt(resolve(body["req_id"])), DataJSON: asString(resolve(body["data_json"]))}, nil
	case "display_group_list":
		return codec.DisplayGroupList{ReqID: asInt(resolve(body["req_id"])), Groups: asString(resolve(body["groups"]))}, nil
	case "display_group_updated":
		return codec.DisplayGroupUpdated{ReqID: asInt(resolve(body["req_id"])), ContractInfo: asString(resolve(body["contract_info"]))}, nil
	default:
		return nil, fmt.Errorf("testhost: unsupported build message %q", name)
	}
}

// buildPackedHistoricalBars encodes consecutive historical_bar steps into a
// single packed frame matching the real IBKR wire format for msg 17:
// [17, reqID, barCount, bar1_time, bar1_O, bar1_H, bar1_L, bar1_C, bar1_vol, bar1_wap, bar1_count, ...]
func buildPackedHistoricalBars(bars []step, bindings map[string]any) ([]byte, error) {
	resolve := func(v any) any { return resolveBindings(v, bindings) }

	if len(bars) == 0 {
		return nil, fmt.Errorf("testhost: no bars to pack")
	}

	reqID := asString(resolve(bars[0].body["req_id"]))
	fields := []string{
		strconv.Itoa(codec.InHistoricalData),
		reqID,
		strconv.Itoa(len(bars)),
	}
	for _, bar := range bars {
		fields = append(fields,
			asString(resolve(bar.body["time"])),
			asString(resolve(bar.body["open"])),
			asString(resolve(bar.body["high"])),
			asString(resolve(bar.body["low"])),
			asString(resolve(bar.body["close"])),
			asString(resolve(bar.body["volume"])),
			asString(resolve(bar.body["wap"])),
			asString(resolve(bar.body["count"])),
		)
	}
	return wire.EncodeFields(fields), nil
}

func resolveBindings(value any, bindings map[string]any) any {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "$") {
			if got, ok := bindings[v]; ok {
				return got
			}
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = resolveBindings(item, bindings)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = resolveBindings(item, bindings)
		}
		return out
	default:
		return value
	}
}

func asInt(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		out, _ := strconv.Atoi(v)
		return out
	default:
		return 0
	}
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(value)
	}
}

func asStrings(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, asString(item))
	}
	return out
}

func asBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return v == "1" || v == "true"
	default:
		return false
	}
}

func asCodecEntries[T any](value any, mapFn func(map[string]any) T) []T {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]T, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, mapFn(m))
		}
	}
	return out
}

func asContract(value any) codec.Contract {
	m, _ := value.(map[string]any)
	return codec.Contract{
		ConID:           asInt(m["con_id"]),
		Symbol:          asString(m["symbol"]),
		SecType:         asString(m["sec_type"]),
		Expiry:          asString(m["expiry"]),
		Strike:          asString(m["strike"]),
		Right:           asString(m["right"]),
		Multiplier:      asString(m["multiplier"]),
		Exchange:        asString(m["exchange"]),
		Currency:        asString(m["currency"]),
		LocalSymbol:     asString(m["local_symbol"]),
		TradingClass:    asString(m["trading_class"]),
		PrimaryExchange: asString(m["primary_exchange"]),
	}
}
