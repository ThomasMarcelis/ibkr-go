package session

import (
	"strconv"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func toCodecPlaceOrder(orderID int64, req PlaceOrderRequest) codec.PlaceOrderRequest {
	return codec.PlaceOrderRequest{
		OrderID:  orderID,
		Contract: toCodecContract(req.Contract),

		Action:        string(req.Order.Action),
		TotalQuantity: decimalOrEmpty(req.Order.Quantity),
		OrderType:     req.Order.OrderType,
		LmtPrice:      decimalOrEmpty(req.Order.LmtPrice),
		AuxPrice:      decimalOrEmpty(req.Order.AuxPrice),

		TIF:                         string(req.Order.TIF),
		OcaGroup:                    req.Order.OcaGroup,
		Account:                     req.Order.Account,
		Origin:                      "0",
		OrderRef:                    req.Order.OrderRef,
		Transmit:                    optBoolToString(req.Order.Transmit, "1"),
		ParentID:                    strconv.FormatInt(req.Order.ParentID, 10),
		OutsideRTH:                  boolToString(req.Order.OutsideRTH),
		ComboLegs:                   comboLegsToCodec(req.Order.ComboLegs),
		OrderComboLegPrices:         append([]string(nil), req.Order.OrderComboLegPrices...),
		SmartComboRoutingParams:     tagValuesToCodec(req.Order.SmartComboRoutingParams),
		ExemptCode:                  "-1",
		GoodAfterTime:               req.Order.GoodAfterTime,
		GoodTillDate:                req.Order.GoodTillDate,
		AlgoStrategy:                req.Order.AlgoStrategy,
		AlgoParams:                  tagValuesToCodec(req.Order.AlgoParams),
		Conditions:                  orderConditionsToCodec(req.Order.Conditions),
		ConditionsIgnoreRTH:         boolToString(req.Order.ConditionsIgnoreRTH),
		ConditionsCancelOrder:       boolToString(req.Order.ConditionsCancelOrder),
		DeltaNeutralContractPresent: "0",
	}
}

func decimalOrEmpty(d Decimal) string {
	if d == (Decimal{}) {
		return ""
	}
	return d.String()
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func optBoolToString(b *bool, dflt string) string {
	if b == nil {
		return dflt
	}
	if *b {
		return "1"
	}
	return "0"
}

func comboLegsToCodec(legs []ComboLeg) []codec.ComboLeg {
	if len(legs) == 0 {
		return nil
	}
	out := make([]codec.ComboLeg, len(legs))
	for i, leg := range legs {
		out[i] = codec.ComboLeg{
			ConID:              leg.ConID,
			Ratio:              leg.Ratio,
			Action:             leg.Action,
			Exchange:           leg.Exchange,
			OpenClose:          leg.OpenClose,
			ShortSaleSlot:      strconv.Itoa(leg.ShortSaleSlot),
			DesignatedLocation: leg.DesignatedLocation,
			ExemptCode:         strconv.Itoa(leg.ExemptCode),
		}
	}
	return out
}

func tagValuesToCodec(values []TagValue) []codec.TagValue {
	if len(values) == 0 {
		return nil
	}
	out := make([]codec.TagValue, len(values))
	for i, value := range values {
		out[i] = codec.TagValue{Tag: value.Tag, Value: value.Value}
	}
	return out
}

func orderConditionsToCodec(values []OrderCondition) []codec.OrderCondition {
	if len(values) == 0 {
		return nil
	}
	out := make([]codec.OrderCondition, len(values))
	for i, value := range values {
		out[i] = codec.OrderCondition{
			Type:          value.Type,
			Conjunction:   value.Conjunction,
			ConID:         value.ConID,
			Exchange:      value.Exchange,
			Operator:      value.Operator,
			Value:         value.Value,
			TriggerMethod: value.TriggerMethod,
			SecType:       value.SecType,
			Symbol:        value.Symbol,
		}
	}
	return out
}

func comboLegsFromCodec(legs []codec.ComboLeg) []ComboLeg {
	if len(legs) == 0 {
		return nil
	}
	out := make([]ComboLeg, len(legs))
	for i, leg := range legs {
		shortSaleSlot, _ := strconv.Atoi(leg.ShortSaleSlot)
		exemptCode, _ := strconv.Atoi(leg.ExemptCode)
		out[i] = ComboLeg{
			ConID:              leg.ConID,
			Ratio:              leg.Ratio,
			Action:             leg.Action,
			Exchange:           leg.Exchange,
			OpenClose:          leg.OpenClose,
			ShortSaleSlot:      shortSaleSlot,
			DesignatedLocation: leg.DesignatedLocation,
			ExemptCode:         exemptCode,
		}
	}
	return out
}

func tagValuesFromCodec(values []codec.TagValue) []TagValue {
	if len(values) == 0 {
		return nil
	}
	out := make([]TagValue, len(values))
	for i, value := range values {
		out[i] = TagValue{Tag: value.Tag, Value: value.Value}
	}
	return out
}

func orderConditionsFromCodec(values []codec.OrderCondition) []OrderCondition {
	if len(values) == 0 {
		return nil
	}
	out := make([]OrderCondition, len(values))
	for i, value := range values {
		out[i] = OrderCondition{
			Type:          value.Type,
			Conjunction:   value.Conjunction,
			ConID:         value.ConID,
			Exchange:      value.Exchange,
			Operator:      value.Operator,
			Value:         value.Value,
			TriggerMethod: value.TriggerMethod,
			SecType:       value.SecType,
			Symbol:        value.Symbol,
		}
	}
	return out
}
