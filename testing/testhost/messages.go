package testhost

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func messageName(msg any) string {
	switch msg.(type) {
	case codec.StartAPI:
		return "start_api"
	case codec.ServerInfo:
		return "server_info"
	case codec.ManagedAccounts:
		return "managed_accounts"
	case codec.NextValidID:
		return "next_valid_id"
	case codec.CurrentTime:
		return "current_time"
	case codec.APIError:
		return "api_error"
	case codec.ContractDetailsRequest:
		return "req_contract_details"
	case codec.ContractDetails:
		return "contract_details"
	case codec.ContractDetailsEnd:
		return "contract_details_end"
	case codec.HistoricalBarsRequest:
		return "req_historical_bars"
	case codec.HistoricalBar:
		return "historical_bar"
	case codec.HistoricalBarsEnd:
		return "historical_bars_end"
	case codec.AccountSummaryRequest:
		return "req_account_summary"
	case codec.CancelAccountSummary:
		return "cancel_account_summary"
	case codec.AccountSummaryValue:
		return "account_summary"
	case codec.AccountSummaryEnd:
		return "account_summary_end"
	case codec.PositionsRequest:
		return "req_positions"
	case codec.CancelPositions:
		return "cancel_positions"
	case codec.Position:
		return "position"
	case codec.PositionEnd:
		return "position_end"
	case codec.QuoteRequest:
		return "req_quote"
	case codec.CancelQuote:
		return "cancel_quote"
	case codec.TickPrice:
		return "tick_price"
	case codec.TickSize:
		return "tick_size"
	case codec.MarketDataType:
		return "market_data_type"
	case codec.TickSnapshotEnd:
		return "tick_snapshot_end"
	case codec.RealTimeBarsRequest:
		return "req_realtime_bars"
	case codec.CancelRealTimeBars:
		return "cancel_realtime_bars"
	case codec.RealTimeBar:
		return "realtime_bar"
	case codec.OpenOrdersRequest:
		return "req_open_orders"
	case codec.CancelOpenOrders:
		return "cancel_open_orders"
	case codec.OpenOrder:
		return "open_order"
	case codec.OpenOrderEnd:
		return "open_order_end"
	case codec.ExecutionsRequest:
		return "req_executions"
	case codec.ExecutionDetail:
		return "execution_detail"
	case codec.ExecutionsEnd:
		return "executions_end"
	case codec.CommissionReport:
		return "commission_report"
	default:
		return ""
	}
}

func messageBody(msg any) (map[string]any, error) {
	switch m := msg.(type) {
	case codec.StartAPI:
		return map[string]any{"client_id": float64(m.ClientID), "optional_capabilities": m.OptionalCapabilities}, nil
	case codec.ServerInfo:
		return map[string]any{"server_version": float64(m.ServerVersion), "connection_time": m.ConnectionTime}, nil
	case codec.ManagedAccounts:
		return map[string]any{"accounts": stringsToAny(m.Accounts)}, nil
	case codec.NextValidID:
		return map[string]any{"order_id": float64(m.OrderID)}, nil
	case codec.APIError:
		return map[string]any{"req_id": float64(m.ReqID), "code": float64(m.Code), "message": m.Message, "advanced_order_reject_json": m.AdvancedOrderRejectJSON, "error_time_ms": m.ErrorTimeMs}, nil
	case codec.ContractDetailsRequest:
		return map[string]any{"req_id": float64(m.ReqID), "contract": contractBody(m.Contract)}, nil
	case codec.ContractDetails:
		return map[string]any{"req_id": float64(m.ReqID), "contract": contractBody(m.Contract), "market_name": m.MarketName, "min_tick": m.MinTick, "time_zone_id": m.TimeZoneID}, nil
	case codec.ContractDetailsEnd:
		return map[string]any{"req_id": float64(m.ReqID)}, nil
	case codec.HistoricalBarsRequest:
		return map[string]any{"req_id": float64(m.ReqID), "contract": contractBody(m.Contract), "end_time": m.EndDateTime, "duration": m.Duration, "bar_size": m.BarSize, "what_to_show": m.WhatToShow, "use_rth": m.UseRTH}, nil
	case codec.HistoricalBar:
		return map[string]any{"req_id": float64(m.ReqID), "time": m.Time, "open": m.Open, "high": m.High, "low": m.Low, "close": m.Close, "volume": m.Volume}, nil
	case codec.HistoricalBarsEnd:
		return map[string]any{"req_id": float64(m.ReqID), "start": m.Start, "end": m.End}, nil
	case codec.AccountSummaryRequest:
		return map[string]any{"req_id": float64(m.ReqID), "account": m.Account, "tags": stringsToAny(m.Tags)}, nil
	case codec.CancelAccountSummary:
		return map[string]any{"req_id": float64(m.ReqID)}, nil
	case codec.AccountSummaryValue:
		return map[string]any{"req_id": float64(m.ReqID), "account": m.Account, "tag": m.Tag, "value": m.Value, "currency": m.Currency}, nil
	case codec.AccountSummaryEnd:
		return map[string]any{"req_id": float64(m.ReqID)}, nil
	case codec.PositionsRequest, codec.CancelPositions, codec.OpenOrderEnd, codec.PositionEnd, codec.CancelOpenOrders:
		return map[string]any{}, nil
	case codec.Position:
		return map[string]any{"account": m.Account, "contract": contractBody(m.Contract), "position": m.Position, "avg_cost": m.AvgCost}, nil
	case codec.QuoteRequest:
		return map[string]any{"req_id": float64(m.ReqID), "contract": contractBody(m.Contract), "snapshot": m.Snapshot, "generic_ticks": stringsToAny(m.GenericTicks)}, nil
	case codec.CancelQuote:
		return map[string]any{"req_id": float64(m.ReqID)}, nil
	case codec.TickPrice:
		return map[string]any{"req_id": float64(m.ReqID), "field": m.Field, "price": m.Price}, nil
	case codec.TickSize:
		return map[string]any{"req_id": float64(m.ReqID), "field": m.Field, "size": m.Size}, nil
	case codec.MarketDataType:
		return map[string]any{"req_id": float64(m.ReqID), "data_type": float64(m.DataType)}, nil
	case codec.TickSnapshotEnd:
		return map[string]any{"req_id": float64(m.ReqID)}, nil
	case codec.RealTimeBarsRequest:
		return map[string]any{"req_id": float64(m.ReqID), "contract": contractBody(m.Contract), "what_to_show": m.WhatToShow, "use_rth": m.UseRTH}, nil
	case codec.CancelRealTimeBars:
		return map[string]any{"req_id": float64(m.ReqID)}, nil
	case codec.RealTimeBar:
		return map[string]any{"req_id": float64(m.ReqID), "time": m.Time, "open": m.Open, "high": m.High, "low": m.Low, "close": m.Close, "volume": m.Volume}, nil
	case codec.OpenOrdersRequest:
		return map[string]any{"scope": m.Scope}, nil
	case codec.OpenOrder:
		return map[string]any{"order_id": float64(m.OrderID), "account": m.Account, "contract": contractBody(m.Contract), "status": m.Status, "quantity": m.Quantity, "filled": m.Filled, "remaining": m.Remaining}, nil
	case codec.ExecutionsRequest:
		return map[string]any{"req_id": float64(m.ReqID), "account": m.Account, "symbol": m.Symbol}, nil
	case codec.ExecutionDetail:
		return map[string]any{"req_id": float64(m.ReqID), "exec_id": m.ExecID, "account": m.Account, "symbol": m.Symbol, "side": m.Side, "shares": m.Shares, "price": m.Price, "time": m.Time}, nil
	case codec.ExecutionsEnd:
		return map[string]any{"req_id": float64(m.ReqID)}, nil
	case codec.CommissionReport:
		return map[string]any{"exec_id": m.ExecID, "commission": m.Commission, "currency": m.Currency, "realized_pnl": m.RealizedPNL}, nil
	default:
		return nil, fmt.Errorf("testhost: unsupported message %T", msg)
	}
}

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
			TimeZoneID: asString(resolve(body["time_zone_id"])),
		}, nil
	case "contract_details_end":
		return codec.ContractDetailsEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "historical_bar":
		return codec.HistoricalBar{ReqID: asInt(resolve(body["req_id"])), Time: asString(resolve(body["time"])), Open: asString(resolve(body["open"])), High: asString(resolve(body["high"])), Low: asString(resolve(body["low"])), Close: asString(resolve(body["close"])), Volume: asString(resolve(body["volume"]))}, nil
	case "historical_bars_end":
		return codec.HistoricalBarsEnd{ReqID: asInt(resolve(body["req_id"])), Start: asString(resolve(body["start"])), End: asString(resolve(body["end"]))}, nil
	case "account_summary":
		return codec.AccountSummaryValue{ReqID: asInt(resolve(body["req_id"])), Account: asString(resolve(body["account"])), Tag: asString(resolve(body["tag"])), Value: asString(resolve(body["value"])), Currency: asString(resolve(body["currency"]))}, nil
	case "account_summary_end":
		return codec.AccountSummaryEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "position":
		return codec.Position{Account: asString(resolve(body["account"])), Contract: asContract(resolve(body["contract"])), Position: asString(resolve(body["position"])), AvgCost: asString(resolve(body["avg_cost"]))}, nil
	case "position_end":
		return codec.PositionEnd{}, nil
	case "tick_price":
		return codec.TickPrice{ReqID: asInt(resolve(body["req_id"])), Field: asString(resolve(body["field"])), Price: asString(resolve(body["price"]))}, nil
	case "tick_size":
		return codec.TickSize{ReqID: asInt(resolve(body["req_id"])), Field: asString(resolve(body["field"])), Size: asString(resolve(body["size"]))}, nil
	case "market_data_type":
		return codec.MarketDataType{ReqID: asInt(resolve(body["req_id"])), DataType: asInt(resolve(body["data_type"]))}, nil
	case "tick_snapshot_end":
		return codec.TickSnapshotEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "realtime_bar":
		return codec.RealTimeBar{ReqID: asInt(resolve(body["req_id"])), Time: asString(resolve(body["time"])), Open: asString(resolve(body["open"])), High: asString(resolve(body["high"])), Low: asString(resolve(body["low"])), Close: asString(resolve(body["close"])), Volume: asString(resolve(body["volume"]))}, nil
	case "open_order":
		return codec.OpenOrder{OrderID: int64(asInt(resolve(body["order_id"]))), Account: asString(resolve(body["account"])), Contract: asContract(resolve(body["contract"])), Status: asString(resolve(body["status"])), Quantity: asString(resolve(body["quantity"])), Filled: asString(resolve(body["filled"])), Remaining: asString(resolve(body["remaining"]))}, nil
	case "open_order_end":
		return codec.OpenOrderEnd{}, nil
	case "execution_detail":
		return codec.ExecutionDetail{ReqID: asInt(resolve(body["req_id"])), ExecID: asString(resolve(body["exec_id"])), Account: asString(resolve(body["account"])), Symbol: asString(resolve(body["symbol"])), Side: asString(resolve(body["side"])), Shares: asString(resolve(body["shares"])), Price: asString(resolve(body["price"])), Time: asString(resolve(body["time"]))}, nil
	case "executions_end":
		return codec.ExecutionsEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "commission_report":
		return codec.CommissionReport{ExecID: asString(resolve(body["exec_id"])), Commission: asString(resolve(body["commission"])), Currency: asString(resolve(body["currency"])), RealizedPNL: asString(resolve(body["realized_pnl"]))}, nil
	default:
		return nil, fmt.Errorf("testhost: unsupported build message %q", name)
	}
}

func contractBody(c codec.Contract) map[string]any {
	return map[string]any{
		"symbol":           c.Symbol,
		"sec_type":         c.SecType,
		"exchange":         c.Exchange,
		"currency":         c.Currency,
		"primary_exchange": c.PrimaryExchange,
		"local_symbol":     c.LocalSymbol,
	}
}

func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
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

func asContract(value any) codec.Contract {
	m, _ := value.(map[string]any)
	return codec.Contract{
		Symbol:          asString(m["symbol"]),
		SecType:         asString(m["sec_type"]),
		Exchange:        asString(m["exchange"]),
		Currency:        asString(m["currency"]),
		PrimaryExchange: asString(m["primary_exchange"]),
		LocalSymbol:     asString(m["local_symbol"]),
	}
}
