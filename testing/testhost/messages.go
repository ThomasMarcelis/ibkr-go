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
		return codec.OpenOrder{OrderID: int64(asInt(resolve(body["order_id"]))), Account: asString(resolve(body["account"])), Contract: asContract(resolve(body["contract"])), Action: asString(resolve(body["action"])), OrderType: asString(resolve(body["order_type"])), Status: asString(resolve(body["status"])), Quantity: asString(resolve(body["quantity"])), Filled: asString(resolve(body["filled"])), Remaining: asString(resolve(body["remaining"]))}, nil
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
