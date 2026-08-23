package testhost

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

func buildLegacyServerMessage(name string, body map[string]any, bindings map[string]any) (codec.LegacyServerMessage, error) {
	resolve := func(v any) any { return resolveBindings(v, bindings) }

	switch name {
	case "managed_accounts":
		return codec.ManagedAccounts{Accounts: asStrings(resolve(body["accounts"]))}, nil
	case "next_valid_id":
		return codec.NextValidID{OrderID: int64(asInt(resolve(body["order_id"])))}, nil
	case "api_error":
		return codec.APIError{
			ReqID:                   asInt(resolve(body["req_id"])),
			Code:                    asInt(resolve(body["code"])),
			Message:                 asString(resolve(body["message"])),
			AdvancedOrderRejectJSON: asString(resolve(body["advanced_order_reject_json"])),
			ErrorTimeMs:             asString(resolve(body["error_time_ms"])),
		}, nil
	case "open_order":
		contract := asContract(resolve(body["contract"]))
		contract.ComboLegs = asCodecComboLegs(resolve(body["combo_legs"]))
		return codec.OpenOrder{
			OrderID: int64(asInt(resolve(body["order_id"]))),
			OrderDetails: codec.OrderDetails{
				Account:               asString(resolve(body["account"])),
				Contract:              contract,
				Action:                asString(resolve(body["action"])),
				OrderType:             asString(resolve(body["order_type"])),
				Status:                asString(resolve(body["status"])),
				Quantity:              asString(resolve(body["quantity"])),
				LmtPrice:              asString(resolve(body["lmt_price"])),
				AuxPrice:              asString(resolve(body["aux_price"])),
				TIF:                   asString(resolve(body["tif"])),
				OcaGroup:              asString(resolve(body["oca_group"])),
				OpenClose:             asString(resolve(body["open_close"])),
				Origin:                asString(resolve(body["origin"])),
				OrderRef:              asString(resolve(body["order_ref"])),
				ClientID:              asString(resolve(body["client_id"])),
				PermID:                asString(resolve(body["perm_id"])),
				OutsideRTH:            asString(resolve(body["outside_rth"])),
				Hidden:                asString(resolve(body["hidden"])),
				DiscretionAmt:         asString(resolve(body["discretion_amt"])),
				GoodAfterTime:         asString(resolve(body["good_after_time"])),
				ParentID:              asString(resolve(body["parent_id"])),
				OrderComboLegPrices:   asStrings(resolve(body["order_combo_leg_prices"])),
				SmartComboRouting:     asCodecTagValues(resolve(body["smart_combo_routing"])),
				AlgoStrategy:          asString(resolve(body["algo_strategy"])),
				AlgoParams:            asCodecTagValues(resolve(body["algo_params"])),
				Conditions:            asCodecOrderConditions(resolve(body["conditions"])),
				ConditionsIgnoreRTH:   asString(resolve(body["conditions_ignore_rth"])),
				ConditionsCancelOrder: asString(resolve(body["conditions_cancel_order"])),
			},
			Status:               asString(resolve(body["status"])),
			InitMarginBefore:     asString(resolve(body["init_margin_before"])),
			MaintMarginBefore:    asString(resolve(body["maint_margin_before"])),
			EquityWithLoanBefore: asString(resolve(body["equity_with_loan_before"])),
			InitMarginChange:     asString(resolve(body["init_margin_change"])),
			MaintMarginChange:    asString(resolve(body["maint_margin_change"])),
			EquityWithLoanChange: asString(resolve(body["equity_with_loan_change"])),
			InitMarginAfter:      asString(resolve(body["init_margin_after"])),
			MaintMarginAfter:     asString(resolve(body["maint_margin_after"])),
			EquityWithLoanAfter:  asString(resolve(body["equity_with_loan_after"])),
			Commission:           asString(resolve(body["commission"])),
			MinCommission:        asString(resolve(body["min_commission"])),
			MaxCommission:        asString(resolve(body["max_commission"])),
			CommissionCurrency:   asString(resolve(body["commission_currency"])),
			WarningText:          asString(resolve(body["warning_text"])),
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
		contract := asContract(resolve(body["contract"]))
		if contract.Symbol == "" {
			contract.Symbol = asString(resolve(body["symbol"]))
		}
		return codec.ExecutionDetail{
			ReqID:                   asInt(resolve(body["req_id"])),
			OrderID:                 int64(asInt(resolve(body["order_id"]))),
			Contract:                contract,
			ExecID:                  asString(resolve(body["exec_id"])),
			Time:                    asString(resolve(body["time"])),
			Account:                 asString(resolve(body["account"])),
			Exchange:                asString(resolve(body["exchange"])),
			Side:                    asString(resolve(body["side"])),
			Shares:                  asString(resolve(body["shares"])),
			Price:                   asString(resolve(body["price"])),
			PermID:                  asString(resolve(body["perm_id"])),
			ClientID:                asString(resolve(body["client_id"])),
			Liquidation:             asString(resolve(body["liquidation"])),
			CumulativeQuantity:      asString(resolve(body["cumulative_quantity"])),
			AveragePrice:            asString(resolve(body["average_price"])),
			OrderRef:                asString(resolve(body["order_ref"])),
			EconomicValueRule:       asString(resolve(body["economic_value_rule"])),
			EconomicValueMultiplier: asString(resolve(body["economic_value_multiplier"])),
			ModelCode:               asString(resolve(body["model_code"])),
			LastLiquidity:           asString(resolve(body["last_liquidity"])),
			PendingPriceRevision:    asString(resolve(body["pending_price_revision"])),
			Submitter:               asString(resolve(body["submitter"])),
		}, nil
	case "executions_end":
		return codec.ExecutionsEnd{ReqID: asInt(resolve(body["req_id"]))}, nil
	case "commission_report":
		return codec.CommissionReport{
			ExecID:              asString(resolve(body["exec_id"])),
			Commission:          asString(resolve(body["commission"])),
			Currency:            asString(resolve(body["currency"])),
			RealizedPNL:         asString(resolve(body["realized_pnl"])),
			Yield:               asString(resolve(body["bond_yield"])),
			YieldRedemptionDate: asString(resolve(body["yield_redemption_date"])),
		}, nil
	case "completed_order":
		return codec.CompletedOrder{
			OrderDetails: codec.OrderDetails{
				Contract:  asContract(resolve(body["contract"])),
				Action:    asString(resolve(body["action"])),
				OrderType: asString(resolve(body["order_type"])),
				Status:    asString(resolve(body["status"])),
				Quantity:  asString(resolve(body["quantity"])),
				Filled:    asString(resolve(body["filled"])),
			},
		}, nil
	case "completed_order_end":
		return codec.CompletedOrderEnd{}, nil
	default:
		return nil, fmt.Errorf("testhost: unsupported build message %q", name)
	}
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
	case nil:
		return ""
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

func asCodecComboLegs(value any) []codec.ComboLeg {
	return asCodecEntries(value, func(m map[string]any) codec.ComboLeg {
		return codec.ComboLeg{
			ConID:              asInt(m["con_id"]),
			Ratio:              asInt(m["ratio"]),
			Action:             asString(m["action"]),
			Exchange:           asString(m["exchange"]),
			OpenClose:          asString(m["open_close"]),
			ShortSaleSlot:      asString(m["short_sale_slot"]),
			DesignatedLocation: asString(m["designated_location"]),
			ExemptCode:         asString(m["exempt_code"]),
		}
	})
}

func asCodecTagValues(value any) []codec.TagValue {
	return asCodecEntries(value, func(m map[string]any) codec.TagValue {
		return codec.TagValue{Tag: asString(m["tag"]), Value: asString(m["value"])}
	})
}

func asCodecOrderConditions(value any) []codec.OrderCondition {
	return asCodecEntries(value, func(m map[string]any) codec.OrderCondition {
		return codec.OrderCondition{
			Type:          asInt(m["type"]),
			Conjunction:   asString(m["conjunction"]),
			ConID:         asInt(m["con_id"]),
			Exchange:      asString(m["exchange"]),
			Operator:      asInt(m["operator"]),
			Value:         asString(m["value"]),
			TriggerMethod: asInt(m["trigger_method"]),
			SecType:       asString(m["sec_type"]),
			Symbol:        asString(m["symbol"]),
		}
	})
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
		IssuerID:        asString(m["issuer_id"]),
	}
}
