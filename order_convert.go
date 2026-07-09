package ibkr

import (
	"strconv"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/shopspring/decimal"
)

func toCodecPlaceOrder(orderID int64, req PlaceOrderRequest) codec.PlaceOrderRequest {
	return codec.PlaceOrderRequest{
		OrderID:  orderID,
		Contract: toCodecContract(req.Contract),

		Action:        string(req.Order.Action),
		TotalQuantity: decimalOrEmpty(req.Order.Quantity),
		OrderType:     string(req.Order.OrderType),
		LmtPrice:      decimalOrEmpty(req.Order.LmtPrice),
		AuxPrice:      decimalOrEmpty(req.Order.AuxPrice),

		TIF:                         string(req.Order.TIF),
		OcaGroup:                    req.Order.OCA.Group,
		OcaType:                     strconv.Itoa(int(req.Order.OCA.Type)),
		Account:                     req.Order.Account,
		Origin:                      "0",
		OrderRef:                    req.Order.OrderRef,
		Transmit:                    optBoolToString(req.Order.Transmit, "1"),
		ParentID:                    strconv.FormatInt(req.Order.ParentID, 10),
		TriggerMethod:               strconv.Itoa(req.Order.TriggerMethod),
		OutsideRTH:                  boolToString(req.Order.OutsideRTH),
		DisplaySize:                 strconv.Itoa(req.Order.DisplaySize),
		ComboLegs:                   comboLegsToCodec(req.Order.Combo.Legs),
		OrderComboLegPrices:         append([]string(nil), req.Order.Combo.LegPrices...),
		SmartComboRoutingParams:     tagValuesToCodec(req.Order.Combo.SmartRouting),
		ExemptCode:                  "-1",
		GoodAfterTime:               req.Order.GoodAfterTime,
		GoodTillDate:                req.Order.GoodTillDate,
		AllOrNone:                   optBoolToString(req.Order.AllOrNone, ""),
		MinQty:                      decimalOrEmpty(req.Order.MinQty),
		PercentOffset:               decimalOrEmpty(req.Order.PercentOffset),
		TrailStopPrice:              decimalOrEmpty(req.Order.TrailStopPrice),
		TrailingPercent:             decimalOrEmpty(req.Order.TrailingPercent),
		ScaleInitLevelSize:          scaleSizeOrEmpty(req.Order.Scale.InitialLevelSize),
		ScaleSubsLevelSize:          scaleSizeOrEmpty(req.Order.Scale.SubsequentLevelSize),
		ScalePriceIncrement:         decimalOrEmpty(req.Order.Scale.PriceIncrement),
		ScaleTable:                  req.Order.Scale.Table,
		ActiveStartTime:             req.Order.Scale.ActiveStartTime,
		ActiveStopTime:              req.Order.Scale.ActiveStopTime,
		HedgeType:                   string(req.Order.Hedge.Type),
		HedgeParam:                  req.Order.Hedge.Param,
		AlgoStrategy:                req.Order.Algorithm.Strategy,
		AlgoParams:                  tagValuesToCodec(req.Order.Algorithm.Params),
		WhatIf:                      optBoolToString(req.Order.WhatIf, ""),
		Conditions:                  orderConditionsToCodec(req.Order.Conditions.Values),
		ConditionsIgnoreRTH:         boolToString(req.Order.Conditions.IgnoreRTH),
		ConditionsCancelOrder:       boolToString(req.Order.Conditions.CancelOrder),
		AdjustedOrderType:           string(req.Order.Adjustment.OrderType),
		TriggerPrice:                decimalOrEmpty(req.Order.Adjustment.TriggerPrice),
		LmtPriceOffset:              decimalOrEmpty(req.Order.Adjustment.LmtPriceOffset),
		AdjustedStopPrice:           decimalOrEmpty(req.Order.Adjustment.StopPrice),
		AdjustedStopLimitPrice:      decimalOrEmpty(req.Order.Adjustment.StopLimitPrice),
		AdjustedTrailingAmount:      decimalOrEmpty(req.Order.Adjustment.TrailingAmount),
		AdjustableTrailingUnit:      strconv.Itoa(req.Order.Adjustment.TrailingUnit),
		CashQty:                     decimalOrEmpty(req.Order.CashQty),
		DontUseAutoPriceForHedge:    optBoolToString(req.Order.Hedge.DisableAutomaticPrice, ""),
		UsePriceMgmtAlgo:            optBoolToString(req.Order.UsePriceMgmtAlgo, ""),
		AdvancedErrorOverride:       req.Order.AdvancedErrorOverride,
		ManualOrderTime:             req.Order.ManualOrderTime,
		DeltaNeutralContractPresent: "0",
	}
}

func decimalOrEmpty(d decimal.Decimal) string {
	if d.IsZero() {
		return ""
	}
	return d.String()
}

// scaleSizeOrEmpty mirrors IBKR reference clients' sendMax(int): zero encodes
// as an explicit-unset empty field for the scale-size sentinels.
func scaleSizeOrEmpty(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
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
			SecType:       string(value.SecType),
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
			SecType:       SecType(value.SecType),
			Symbol:        value.Symbol,
		}
	}
	return out
}
