package ibkr

import (
	"strconv"
	"strings"

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
			Action:             string(leg.Action),
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
			Type:          int(value.Type),
			Conjunction:   string(value.Conjunction),
			ConID:         value.ConID,
			Exchange:      value.Exchange,
			Operator:      int(value.Operator),
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
			Action:             OrderAction(leg.Action),
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
			Type:          OrderConditionType(value.Type),
			Conjunction:   ConditionConjunction(value.Conjunction),
			ConID:         value.ConID,
			Exchange:      value.Exchange,
			Operator:      ConditionOperator(value.Operator),
			Value:         value.Value,
			TriggerMethod: value.TriggerMethod,
			SecType:       SecType(value.SecType),
			Symbol:        value.Symbol,
		}
	}
	return out
}

func fromCodecCompletedOrder(m codec.CompletedOrder) (CompletedOrderResult, error) {
	quantity, err := parseRequiredDecimal(m.Quantity, "completed order quantity")
	if err != nil {
		return CompletedOrderResult{}, err
	}
	filled, err := parseOptionalDecimal(m.Filled, "completed order filled")
	if err != nil {
		return CompletedOrderResult{}, err
	}

	var parseErr error
	decimalPointer := func(raw, field string) *decimal.Decimal {
		if parseErr != nil {
			return nil
		}
		value, err := parseOptionalDecimalPointer(raw, "completed order "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	decimalValue := func(raw, field string) decimal.Decimal {
		if parseErr != nil {
			return decimal.Decimal{}
		}
		value, err := parseOptionalDecimal(raw, "completed order "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	intPointer := func(raw, field string) *int {
		if parseErr != nil {
			return nil
		}
		value, err := parseOptionalMaxIntPointer(raw, "completed order "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	intValue := func(raw, field string) int {
		if parseErr != nil {
			return 0
		}
		value, err := parseOptionalInt(raw, "completed order "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	int64Value := func(raw, field string) int64 {
		if parseErr != nil {
			return 0
		}
		value, err := parseOptionalInt64(raw, "completed order "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	int64Pointer := func(raw, field string) *int64 {
		if parseErr != nil {
			return nil
		}
		value, err := parseOptionalMaxInt64Pointer(raw, "completed order "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	boolValue := func(raw, field string) bool {
		if parseErr != nil {
			return false
		}
		value, err := parseOptionalBoolString(raw, "completed order "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}

	legPrices := make([]*decimal.Decimal, len(m.OrderComboLegPrices))
	for i, raw := range m.OrderComboLegPrices {
		legPrices[i] = decimalPointer(raw, "combo leg price")
	}

	var deltaNeutralContract *DeltaNeutralContract
	if m.DeltaNeutralContractPresent == "1" {
		conID := intValue(m.DeltaNeutralContractConID, "delta-neutral contract id")
		delta := decimalValue(m.DeltaNeutralContractDelta, "delta-neutral contract delta")
		price := decimalValue(m.DeltaNeutralContractPrice, "delta-neutral contract price")
		deltaNeutralContract = &DeltaNeutralContract{ConID: conID, Delta: delta, Price: price}
	}

	deltaNeutralOrderType := m.DeltaNeutralOrderType
	if strings.EqualFold(deltaNeutralOrderType, "None") {
		deltaNeutralOrderType = ""
	}
	var deltaNeutralOrder *CompletedOrderDeltaNeutral
	if deltaNeutralOrderType != "" {
		deltaNeutralOrder = &CompletedOrderDeltaNeutral{
			OrderType:          OrderType(deltaNeutralOrderType),
			AuxPrice:           decimalPointer(m.DeltaNeutralAuxPrice, "delta-neutral aux price"),
			ConID:              intValue(m.DeltaNeutralConID, "delta-neutral contract id"),
			ShortSale:          boolValue(m.DeltaNeutralShortSale, "delta-neutral short sale"),
			ShortSaleSlot:      intValue(m.DeltaNeutralShortSaleSlot, "delta-neutral short-sale slot"),
			DesignatedLocation: m.DeltaNeutralDesignatedLocation,
		}
	}

	var permID *int64
	if strings.TrimSpace(m.PermID) != "" {
		permID = new(int64Value(m.PermID, "permanent id"))
	}
	var exemptCode *int
	if raw := strings.TrimSpace(m.ExemptCode); raw != "" && raw != "-1" {
		exemptCode = new(intValue(raw, "exempt code"))
	}
	var peggedBenchmark *CompletedOrderPeggedBenchmark
	if OrderType(m.OrderType) == OrderTypePeggedBenchmark {
		peggedBenchmark = &CompletedOrderPeggedBenchmark{
			ReferenceContractID:   intValue(m.ReferenceContractID, "pegged reference contract id"),
			ChangeAmountDecrease:  boolValue(m.PeggedChangeAmountDecrease, "pegged change amount decrease"),
			ChangeAmount:          decimalValue(m.PeggedChangeAmount, "pegged change amount"),
			ReferenceChangeAmount: decimalPointer(m.ReferenceChangeAmount, "pegged reference change amount"),
			ReferenceExchangeID:   m.ReferenceExchangeID,
		}
	}

	var disableAutomaticHedgePrice *bool
	if strings.TrimSpace(m.DontUseAutoPriceForHedge) != "" {
		disableAutomaticHedgePrice = new(boolValue(m.DontUseAutoPriceForHedge, "disable automatic hedge price"))
	}

	result := CompletedOrderResult{
		Contract: fromCodecContract(m.Contract),
		Order: CompletedOrderDetails{
			Action:        OrderAction(m.Action),
			Quantity:      quantity,
			OrderType:     OrderType(m.OrderType),
			TIF:           TimeInForce(m.TIF),
			Account:       m.Account,
			OpenClose:     m.OpenClose,
			Origin:        intValue(m.Origin, "origin"),
			OrderRef:      m.OrderRef,
			PermID:        permID,
			OutsideRTH:    boolValue(m.OutsideRTH, "outside RTH"),
			Hidden:        boolValue(m.Hidden, "hidden"),
			GoodAfterTime: m.GoodAfterTime,
			GoodTillDate:  m.GoodTillDate,
			ModelCode:     m.ModelCode,
			Prices: CompletedOrderPrices{
				LmtPrice:            decimalPointer(m.LmtPrice, "limit price"),
				AuxPrice:            decimalPointer(m.AuxPrice, "aux price"),
				DiscretionaryAmount: decimalPointer(m.DiscretionAmt, "discretionary amount"),
				PercentOffset:       decimalPointer(m.PercentOffset, "percent offset"),
				TrailStopPrice:      decimalPointer(m.TrailStopPrice, "trail stop price"),
				TrailingPercent:     decimalPointer(m.TrailingPercent, "trailing percent"),
				StopPrice:           decimalPointer(m.StopPrice, "stop price"),
				LmtPriceOffset:      decimalPointer(m.LmtPriceOffset, "limit price offset"),
				CashQty:             decimalPointer(m.CashQty, "cash quantity"),
			},
			OCA: OrderOCA{Group: m.OcaGroup, Type: OCAType(intValue(m.OcaType, "OCA type"))},
			Allocation: CompletedOrderAllocation{
				Group:      m.FAGroup,
				Method:     m.FAMethod,
				Percentage: m.FAPercentage,
			},
			Routing: CompletedOrderRouting{
				Rule80A:              m.Rule80A,
				SettlingFirm:         m.SettlingFirm,
				ShortSaleSlot:        intValue(m.ShortSaleSlot, "short-sale slot"),
				DesignatedLocation:   m.DesignatedLocation,
				ExemptCode:           exemptCode,
				ClearingAccount:      m.ClearingAccount,
				ClearingIntent:       m.ClearingIntent,
				NotHeld:              boolValue(m.NotHeld, "not held"),
				ImbalanceOnly:        boolValue(m.ImbalanceOnly, "imbalance only"),
				RouteMarketableToBBO: boolValue(m.RouteMarketableToBBO, "route marketable to BBO"),
			},
			Auction: CompletedOrderAuction{
				StartingPrice:   decimalPointer(m.StartingPrice, "starting price"),
				StockRefPrice:   decimalPointer(m.StockRefPrice, "stock reference price"),
				Delta:           decimalPointer(m.Delta, "delta"),
				StockRangeLower: decimalPointer(m.StockRangeLower, "stock range lower"),
				StockRangeUpper: decimalPointer(m.StockRangeUpper, "stock range upper"),
			},
			Execution: CompletedOrderExecution{
				DisplaySize:              intPointer(m.DisplaySize, "display size"),
				SweepToFill:              boolValue(m.SweepToFill, "sweep to fill"),
				AllOrNone:                boolValue(m.AllOrNone, "all or none"),
				MinQty:                   intPointer(m.MinQty, "minimum quantity"),
				TriggerMethod:            intValue(m.TriggerMethod, "trigger method"),
				RandomizeSize:            boolValue(m.RandomizeSize, "randomize size"),
				RandomizePrice:           boolValue(m.RandomizePrice, "randomize price"),
				RefFuturesConID:          intPointer(m.RefFuturesConID, "reference futures contract id"),
				MinTradeQty:              intPointer(m.MinTradeQty, "minimum trade quantity"),
				MinCompeteSize:           intPointer(m.MinCompeteSize, "minimum compete size"),
				CompeteAgainstBestOffset: decimalPointer(m.CompeteAgainstBestOffset, "compete against best offset"),
				MidOffsetAtWhole:         decimalPointer(m.MidOffsetAtWhole, "mid offset at whole"),
				MidOffsetAtHalf:          decimalPointer(m.MidOffsetAtHalf, "mid offset at half"),
			},
			Volatility: CompletedOrderVolatility{
				Value:              decimalPointer(m.Volatility, "volatility"),
				Type:               intPointer(m.VolatilityType, "volatility type"),
				DeltaNeutral:       deltaNeutralOrder,
				ContinuousUpdate:   boolValue(m.ContinuousUpdate, "continuous update"),
				ReferencePriceType: intPointer(m.ReferencePriceType, "reference price type"),
			},
			Combo: CompletedOrderCombo{
				Description:  m.ComboLegsDescription,
				Legs:         comboLegsFromCodec(m.ComboLegs),
				LegPrices:    legPrices,
				SmartRouting: tagValuesFromCodec(m.SmartComboRouting),
			},
			Scale: CompletedOrderScale{
				InitialLevelSize:    intPointer(m.ScaleInitLevelSize, "scale initial level size"),
				SubsequentLevelSize: intPointer(m.ScaleSubsLevelSize, "scale subsequent level size"),
				PriceIncrement:      decimalPointer(m.ScalePriceIncrement, "scale price increment"),
				PriceAdjustValue:    decimalPointer(m.ScalePriceAdjustValue, "scale price adjust value"),
				PriceAdjustInterval: intPointer(m.ScalePriceAdjustInterval, "scale price adjust interval"),
				ProfitOffset:        decimalPointer(m.ScaleProfitOffset, "scale profit offset"),
				AutoReset:           boolValue(m.ScaleAutoReset, "scale auto reset"),
				InitialPosition:     intPointer(m.ScaleInitPosition, "scale initial position"),
				InitialFillQty:      intPointer(m.ScaleInitFillQty, "scale initial fill quantity"),
				RandomPercent:       boolValue(m.ScaleRandomPercent, "scale random percent"),
			},
			Hedge: OrderHedge{
				Type:                  HedgeType(m.HedgeType),
				Param:                 m.HedgeParam,
				DisableAutomaticPrice: disableAutomaticHedgePrice,
			},
			DeltaNeutralContract: deltaNeutralContract,
			Algorithm:            OrderAlgorithm{Strategy: m.AlgoStrategy, Params: tagValuesFromCodec(m.AlgoParams)},
			Conditions: OrderConditions{
				Values:      orderConditionsFromCodec(m.Conditions),
				IgnoreRTH:   boolValue(m.ConditionsIgnoreRTH, "conditions ignore RTH"),
				CancelOrder: boolValue(m.ConditionsCancelOrder, "conditions cancel order"),
			},
			PeggedBenchmark: peggedBenchmark,
			Compliance: CompletedOrderCompliance{
				Solicited:            boolValue(m.Solicited, "solicited"),
				OMSContainer:         boolValue(m.IsOMSContainer, "OMS container"),
				Shareholder:          m.Shareholder,
				CustomerAccount:      m.CustomerAccount,
				ProfessionalCustomer: boolValue(m.ProfessionalCustomer, "professional customer"),
				Submitter:            m.Submitter,
			},
		},
		Completion: CompletedOrderCompletion{
			Status:           OrderStatus(m.Status),
			Filled:           filled,
			AutoCancelDate:   m.AutoCancelDate,
			AutoCancelParent: boolValue(m.AutoCancelParent, "auto cancel parent"),
			ParentPermID:     int64Pointer(m.ParentPermID, "parent permanent id"),
			Time:             m.CompletedTime,
			StatusText:       m.CompletedStatus,
		},
	}
	if parseErr != nil {
		return CompletedOrderResult{}, parseErr
	}
	return result, nil
}
