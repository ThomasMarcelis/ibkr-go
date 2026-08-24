package ibkr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func toCodecPlaceOrder(orderID int64, req PlaceOrderRequest) codec.PlaceOrderRequest {
	return codec.PlaceOrderRequest{
		OrderID:  orderID,
		Contract: toCodecContract(req.Contract),

		Action:        string(req.Order.Action),
		TotalQuantity: decimalOrEmpty(req.Order.Quantity),
		OrderType:     string(req.Order.OrderType),
		LmtPrice:      decimalPointerOrEmpty(req.Order.LmtPrice),
		AuxPrice:      decimalPointerOrEmpty(req.Order.AuxPrice),

		TIF:                        string(req.Order.TIF),
		OcaGroup:                   req.Order.OCA.Group,
		OcaType:                    strconv.Itoa(int(req.Order.OCA.Type)),
		Account:                    req.Order.Account,
		Origin:                     "0",
		OrderRef:                   req.Order.OrderRef,
		Transmit:                   optBoolToString(req.Order.Transmit, "1"),
		ParentID:                   strconv.FormatInt(req.Order.ParentID, 10),
		TriggerMethod:              strconv.Itoa(req.Order.TriggerMethod),
		OutsideRTH:                 boolToString(req.Order.OutsideRTH),
		IncludeOvernight:           optBoolToString(req.Order.IncludeOvernight, ""),
		DisplaySize:                strconv.Itoa(req.Order.DisplaySize),
		OrderComboLegPrices:        comboLegPricesToCodec(req.Order.Combo.LegPrices),
		SmartComboRoutingParams:    tagValuesToCodec(req.Order.Combo.SmartRouting),
		ShortSaleSlot:              scaleSizeOrEmpty(req.Order.ShortSale.Slot),
		DesignatedLocation:         req.Order.ShortSale.DesignatedLocation,
		ExemptCode:                 intPointerOrDefault(req.Order.ShortSale.ExemptCode, "-1"),
		GoodAfterTime:              req.Order.GoodAfterTime,
		GoodTillDate:               req.Order.GoodTillDate,
		AllOrNone:                  optBoolToString(req.Order.AllOrNone, ""),
		MinQty:                     intPointerOrEmpty(req.Order.MinQty),
		AuctionStrategy:            scaleSizeOrEmpty(req.Order.Auction.Strategy),
		StartingPrice:              decimalPointerOrEmpty(req.Order.Auction.StartingPrice),
		StockRefPrice:              decimalPointerOrEmpty(req.Order.Auction.StockRefPrice),
		Delta:                      decimalPointerOrEmpty(req.Order.Auction.Delta),
		StockRangeLower:            decimalPointerOrEmpty(req.Order.Auction.StockRangeLower),
		StockRangeUpper:            decimalPointerOrEmpty(req.Order.Auction.StockRangeUpper),
		PercentOffset:              decimalPointerOrEmpty(req.Order.PercentOffset),
		TrailStopPrice:             decimalPointerOrEmpty(req.Order.TrailStopPrice),
		TrailingPercent:            decimalPointerOrEmpty(req.Order.TrailingPercent),
		ScaleInitLevelSize:         scaleSizeOrEmpty(req.Order.Scale.InitialLevelSize),
		ScaleSubsLevelSize:         scaleSizeOrEmpty(req.Order.Scale.SubsequentLevelSize),
		ScalePriceIncrement:        decimalOrEmpty(req.Order.Scale.PriceIncrement),
		ScalePriceAdjustValue:      decimalPointerOrEmpty(req.Order.Scale.PriceAdjustValue),
		ScalePriceAdjustInterval:   intPointerOrEmpty(req.Order.Scale.PriceAdjustInterval),
		ScaleProfitOffset:          decimalPointerOrEmpty(req.Order.Scale.ProfitOffset),
		ScaleAutoReset:             optBoolToString(req.Order.Scale.AutoReset, ""),
		ScaleInitPosition:          intPointerOrEmpty(req.Order.Scale.InitialPosition),
		ScaleInitFillQty:           intPointerOrEmpty(req.Order.Scale.InitialFillQty),
		ScaleRandomPercent:         optBoolToString(req.Order.Scale.RandomPercent, ""),
		ScaleTable:                 req.Order.Scale.Table,
		ActiveStartTime:            req.Order.Scale.ActiveStartTime,
		ActiveStopTime:             req.Order.Scale.ActiveStopTime,
		HedgeType:                  string(req.Order.Hedge.Type),
		HedgeParam:                 req.Order.Hedge.Param,
		HedgeMaxSize:               intPointerOrEmpty(req.Order.Hedge.MaxSize),
		AlgoStrategy:               req.Order.Algorithm.Strategy,
		AlgoParams:                 tagValuesToCodec(req.Order.Algorithm.Params),
		WhatIf:                     "",
		Conditions:                 orderConditionsToCodec(req.Order.Conditions.Values),
		ConditionsIgnoreRTH:        boolToString(req.Order.Conditions.IgnoreRTH),
		ConditionsCancelOrder:      boolToString(req.Order.Conditions.CancelOrder),
		ReferenceContractID:        peggedReferenceContractID(req.Order.PeggedBenchmark),
		PeggedChangeAmountDecrease: peggedBool(req.Order.PeggedBenchmark),
		PeggedChangeAmount:         peggedChangeAmount(req.Order.PeggedBenchmark),
		ReferenceChangeAmount:      peggedReferenceChangeAmount(req.Order.PeggedBenchmark),
		ReferenceExchangeID:        peggedReferenceExchangeID(req.Order.PeggedBenchmark),
		AdjustedOrderType:          string(req.Order.Adjustment.OrderType),
		TriggerPrice:               decimalOrEmpty(req.Order.Adjustment.TriggerPrice),
		LmtPriceOffset:             decimalPointerOrEmpty(req.Order.LmtPriceOffset),
		AdjustedStopPrice:          decimalOrEmpty(req.Order.Adjustment.StopPrice),
		AdjustedStopLimitPrice:     decimalOrEmpty(req.Order.Adjustment.StopLimitPrice),
		AdjustedTrailingAmount:     decimalOrEmpty(req.Order.Adjustment.TrailingAmount),
		AdjustableTrailingUnit:     strconv.Itoa(req.Order.Adjustment.TrailingUnit),
		CashQty:                    decimalPointerOrEmpty(req.Order.CashQty),
		DontUseAutoPriceForHedge:   optBoolToString(req.Order.Hedge.DisableAutomaticPrice, ""),
		UsePriceMgmtAlgo:           optBoolToString(req.Order.UsePriceMgmtAlgo, ""),
		AdvancedErrorOverride:      req.Order.AdvancedErrorOverride,
		ManualOrderTime:            req.Order.ManualOrderTime,
		Deactivate:                 trueOrEmpty(req.Order.Deactivate),
		PostOnly:                   trueOrEmpty(req.Order.PostOnly),
		AllowPreOpen:               trueOrEmpty(req.Order.AllowPreOpen),
		IgnoreOpenAuction:          trueOrEmpty(req.Order.IgnoreOpenAuction),
		RouteMarketableToBBO:       optBoolToString(req.Order.RouteMarketableToBBO, ""),
		SeekPriceImprovement:       optBoolToString(req.Order.SeekPriceImprovement, ""),
		WhatIfType:                 intPointerOrEmpty(req.Order.WhatIfType),
	}
}

func toCodecPreviewOrder(orderID int64, req PlaceOrderRequest) codec.PlaceOrderRequest {
	order := toCodecPlaceOrder(orderID, req)
	order.WhatIf = "1"
	return order
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

func trueOrEmpty(value bool) string {
	if value {
		return "1"
	}
	return ""
}

func intPointerOrEmpty(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func intPointerOrDefault(value *int, dflt string) string {
	if value == nil {
		return dflt
	}
	return strconv.Itoa(*value)
}

func peggedReferenceContractID(value *OrderPeggedBenchmark) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(int64(value.ReferenceContractID), 10)
}

func peggedBool(value *OrderPeggedBenchmark) string {
	if value == nil {
		return ""
	}
	return boolToString(value.ChangeAmountDecrease)
}

func peggedChangeAmount(value *OrderPeggedBenchmark) string {
	if value == nil {
		return ""
	}
	return value.ChangeAmount.String()
}

func peggedReferenceChangeAmount(value *OrderPeggedBenchmark) string {
	if value == nil {
		return ""
	}
	return decimalPointerOrEmpty(value.ReferenceChangeAmount)
}

func peggedReferenceExchangeID(value *OrderPeggedBenchmark) string {
	if value == nil {
		return ""
	}
	return value.ReferenceExchangeID
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
			ConID:              int(leg.ConID),
			Ratio:              leg.Ratio,
			Action:             string(leg.Action),
			Exchange:           leg.Exchange,
			OpenClose:          strconv.Itoa(int(leg.OpenClose)),
			ShortSaleSlot:      strconv.Itoa(leg.ShortSaleSlot),
			DesignatedLocation: leg.DesignatedLocation,
			ExemptCode:         "-1",
		}
		if leg.ExemptCode != nil {
			out[i].ExemptCode = strconv.Itoa(*leg.ExemptCode)
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
			ConID:         int(value.ConID),
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

func comboLegsFromCodec(legs []codec.ComboLeg) ([]ComboLeg, error) {
	if len(legs) == 0 {
		return nil, nil
	}
	out := make([]ComboLeg, len(legs))
	for i, leg := range legs {
		prefix := fmt.Sprintf("contract combo leg %d", i)
		openClose, err := parseOptionalInt(leg.OpenClose, prefix+" open/close")
		if err != nil {
			return nil, err
		}
		if openClose < int(ComboLegSame) || openClose > int(ComboLegUnknown) {
			return nil, inboundProtocolError(prefix+" open/close", fmt.Errorf("value %d is outside IBKR range 0..3", openClose))
		}
		shortSaleSlot, err := parseOptionalInt(leg.ShortSaleSlot, prefix+" short-sale slot")
		if err != nil {
			return nil, err
		}
		out[i] = ComboLeg{
			ConID:              protocolIDFromInt[ContractID](leg.ConID),
			Ratio:              leg.Ratio,
			Action:             OrderAction(leg.Action),
			Exchange:           leg.Exchange,
			OpenClose:          ComboLegOpenClose(openClose),
			ShortSaleSlot:      shortSaleSlot,
			DesignatedLocation: leg.DesignatedLocation,
		}
		rawExempt := strings.TrimSpace(leg.ExemptCode)
		if rawExempt != "" && rawExempt != "-1" {
			exemptCode, err := parseOptionalInt(rawExempt, prefix+" exempt code")
			if err != nil {
				return nil, err
			}
			if exemptCode < 0 {
				return nil, inboundProtocolError(prefix+" exempt code", fmt.Errorf("value %d must be >= 0 or the -1 unset sentinel", exemptCode))
			}
			out[i].ExemptCode = new(exemptCode)
		}
	}
	return out, nil
}

func comboLegPricesToCodec(prices []*decimal.Decimal) []string {
	if len(prices) == 0 {
		return nil
	}
	out := make([]string, len(prices))
	for i, price := range prices {
		out[i] = decimalPointerOrEmpty(price)
	}
	return out
}

func comboLegPricesFromCodec(prices []string, field string) ([]*decimal.Decimal, error) {
	if len(prices) == 0 {
		return nil, nil
	}
	out := make([]*decimal.Decimal, len(prices))
	for i, raw := range prices {
		price, err := parseOptionalDecimalPointer(raw, fmt.Sprintf("%s %d", field, i))
		if err != nil {
			return nil, err
		}
		out[i] = price
	}
	return out, nil
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
			ConID:         protocolIDFromInt[ContractID](value.ConID),
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
	return fromCodecOrderDetails(m.OrderDetails, "completed order")
}

// OpenOrder and CompletedOrder carry the same OrderDetails schema. The label
// keeps malformed-field diagnostics tied to the callback the user received.
func fromCodecOrderDetails(m codec.OrderDetails, label string) (CompletedOrderResult, error) {
	quantity, err := parseRequiredDecimal(m.Quantity, label+" quantity")
	if err != nil {
		return CompletedOrderResult{}, err
	}
	filled, err := parseOptionalDecimal(m.Filled, label+" filled")
	if err != nil {
		return CompletedOrderResult{}, err
	}

	var parseErr error
	decimalPointer := func(raw, field string) *decimal.Decimal {
		if parseErr != nil {
			return nil
		}
		value, err := parseOptionalDecimalPointer(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	decimalValue := func(raw, field string) decimal.Decimal {
		if parseErr != nil {
			return decimal.Decimal{}
		}
		value, err := parseOptionalDecimal(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	intPointer := func(raw, field string) *int {
		if parseErr != nil {
			return nil
		}
		value, err := parseOptionalMaxIntPointer(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	intValue := func(raw, field string) int {
		if parseErr != nil {
			return 0
		}
		value, err := parseOptionalInt(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	int32Value := func(raw, field string) int32 {
		if parseErr != nil {
			return 0
		}
		value, err := parseOptionalInt32(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	int64Value := func(raw, field string) int64 {
		if parseErr != nil {
			return 0
		}
		value, err := parseOptionalInt64(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	int64Pointer := func(raw, field string) *int64 {
		if parseErr != nil {
			return nil
		}
		value, err := parseOptionalMaxInt64Pointer(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	boolValue := func(raw, field string) bool {
		if parseErr != nil {
			return false
		}
		value, err := parseOptionalBoolString(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}
	boolPointer := func(raw, field string) *bool {
		if parseErr != nil {
			return nil
		}
		value, err := parseOptionalBoolPointer(raw, label+" "+field)
		if err != nil {
			parseErr = err
		}
		return value
	}

	legPrices, err := comboLegPricesFromCodec(m.OrderComboLegPrices, label+" combo leg price")
	if err != nil {
		return CompletedOrderResult{}, err
	}

	deltaNeutralOrderType := m.DeltaNeutralOrderType
	if strings.EqualFold(deltaNeutralOrderType, "None") {
		deltaNeutralOrderType = ""
	}
	var deltaNeutralOrder *OrderDeltaNeutralDetails
	if deltaNeutralOrderType != "" {
		deltaNeutralOrder = &OrderDeltaNeutralDetails{
			OrderType:          OrderType(deltaNeutralOrderType),
			AuxPrice:           decimalPointer(m.DeltaNeutralAuxPrice, "delta-neutral aux price"),
			ConID:              ContractID(int32Value(m.DeltaNeutralConID, "delta-neutral contract id")),
			ShortSale:          boolValue(m.DeltaNeutralShortSale, "delta-neutral short sale"),
			ShortSaleSlot:      intValue(m.DeltaNeutralShortSaleSlot, "delta-neutral short-sale slot"),
			DesignatedLocation: m.DeltaNeutralDesignatedLocation,
		}
	}

	var permID *int64
	if strings.TrimSpace(m.PermID) != "" {
		permID = new(int64Value(m.PermID, "permanent id"))
	}
	var orderID *int64
	if strings.TrimSpace(m.OrderID) != "" {
		orderID = new(int64Value(m.OrderID, "order id"))
	}
	var clientID *ClientID
	if strings.TrimSpace(m.ClientID) != "" {
		clientID = new(ClientID(int32Value(m.ClientID, "client id")))
	}
	var parentID *int64
	if strings.TrimSpace(m.ParentID) != "" {
		parentID = new(int64Value(m.ParentID, "parent id"))
	}
	var exemptCode *int
	if raw := strings.TrimSpace(m.ExemptCode); raw != "" && raw != "-1" {
		exemptCode = new(intValue(raw, "exempt code"))
	}
	var peggedBenchmark *OrderPeggedBenchmark
	if OrderType(m.OrderType) == OrderTypePeggedBenchmark {
		peggedBenchmark = &OrderPeggedBenchmark{
			ReferenceContractID:   ContractID(int32Value(m.ReferenceContractID, "pegged reference contract id")),
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

	contract, err := fromCodecContract(m.Contract)
	if err != nil {
		return CompletedOrderResult{}, err
	}
	result := CompletedOrderResult{
		Contract: contract,
		Order: OrderDetails{
			OrderID:          orderID,
			ClientID:         clientID,
			ParentID:         parentID,
			Action:           OrderAction(m.Action),
			Quantity:         quantity,
			OrderType:        OrderType(m.OrderType),
			TIF:              TimeInForce(m.TIF),
			Account:          m.Account,
			OpenClose:        m.OpenClose,
			Origin:           intValue(m.Origin, "origin"),
			OrderRef:         m.OrderRef,
			PermID:           permID,
			OutsideRTH:       boolValue(m.OutsideRTH, "outside RTH"),
			IncludeOvernight: boolPointer(m.IncludeOvernight, "include overnight"),
			Hidden:           boolValue(m.Hidden, "hidden"),
			GoodAfterTime:    m.GoodAfterTime,
			GoodTillDate:     m.GoodTillDate,
			ModelCode:        m.ModelCode,
			Transmit:         boolPointer(m.Transmit, "transmit"),
			Prices: OrderPrices{
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
			Allocation: OrderAllocationDetails{
				Group:      m.FAGroup,
				Method:     m.FAMethod,
				Percentage: m.FAPercentage,
			},
			Routing: OrderRoutingDetails{
				Rule80A:              m.Rule80A,
				SettlingFirm:         m.SettlingFirm,
				ShortSaleSlot:        intValue(m.ShortSaleSlot, "short-sale slot"),
				DesignatedLocation:   m.DesignatedLocation,
				ExemptCode:           exemptCode,
				ClearingAccount:      m.ClearingAccount,
				ClearingIntent:       m.ClearingIntent,
				NotHeld:              boolValue(m.NotHeld, "not held"),
				ImbalanceOnly:        boolValue(m.ImbalanceOnly, "imbalance only"),
				RouteMarketableToBBO: boolPointer(m.RouteMarketableToBBO, "route marketable to BBO"),
			},
			Auction: OrderAuctionDetails{
				Strategy:        intPointer(m.AuctionStrategy, "auction strategy"),
				StartingPrice:   decimalPointer(m.StartingPrice, "starting price"),
				StockRefPrice:   decimalPointer(m.StockRefPrice, "stock reference price"),
				Delta:           decimalPointer(m.Delta, "delta"),
				StockRangeLower: decimalPointer(m.StockRangeLower, "stock range lower"),
				StockRangeUpper: decimalPointer(m.StockRangeUpper, "stock range upper"),
			},
			Execution: OrderExecutionDetails{
				DisplaySize:              intPointer(m.DisplaySize, "display size"),
				SweepToFill:              boolValue(m.SweepToFill, "sweep to fill"),
				AllOrNone:                boolValue(m.AllOrNone, "all or none"),
				MinQty:                   intPointer(m.MinQty, "minimum quantity"),
				TriggerMethod:            intValue(m.TriggerMethod, "trigger method"),
				RandomizeSize:            boolValue(m.RandomizeSize, "randomize size"),
				RandomizePrice:           boolValue(m.RandomizePrice, "randomize price"),
				RefFuturesConID:          contractIDPointer(intPointer(m.RefFuturesConID, "reference futures contract id")),
				MinTradeQty:              intPointer(m.MinTradeQty, "minimum trade quantity"),
				MinCompeteSize:           intPointer(m.MinCompeteSize, "minimum compete size"),
				CompeteAgainstBestOffset: decimalPointer(m.CompeteAgainstBestOffset, "compete against best offset"),
				MidOffsetAtWhole:         decimalPointer(m.MidOffsetAtWhole, "mid offset at whole"),
				MidOffsetAtHalf:          decimalPointer(m.MidOffsetAtHalf, "mid offset at half"),
			},
			Volatility: OrderVolatilityDetails{
				Value:              decimalPointer(m.Volatility, "volatility"),
				Type:               intPointer(m.VolatilityType, "volatility type"),
				DeltaNeutral:       deltaNeutralOrder,
				ContinuousUpdate:   boolValue(m.ContinuousUpdate, "continuous update"),
				ReferencePriceType: intPointer(m.ReferencePriceType, "reference price type"),
			},
			Combo: OrderCombo{
				LegPrices:    legPrices,
				SmartRouting: tagValuesFromCodec(m.SmartComboRouting),
			},
			ComboDescription: m.ComboLegsDescription,
			Scale: OrderScaleDetails{
				InitialLevelSize:    intPointer(m.ScaleInitLevelSize, "scale initial level size"),
				SubsequentLevelSize: intPointer(m.ScaleSubsLevelSize, "scale subsequent level size"),
				PriceIncrement:      decimalPointer(m.ScalePriceIncrement, "scale price increment"),
				PriceAdjustValue:    decimalPointer(m.ScalePriceAdjustValue, "scale price adjust value"),
				PriceAdjustInterval: intPointer(m.ScalePriceAdjustInterval, "scale price adjust interval"),
				ProfitOffset:        decimalPointer(m.ScaleProfitOffset, "scale profit offset"),
				AutoReset:           boolPointer(m.ScaleAutoReset, "scale auto reset"),
				InitialPosition:     intPointer(m.ScaleInitPosition, "scale initial position"),
				InitialFillQty:      intPointer(m.ScaleInitFillQty, "scale initial fill quantity"),
				RandomPercent:       boolPointer(m.ScaleRandomPercent, "scale random percent"),
				Table:               m.ScaleTable,
				ActiveStartTime:     m.ActiveStartTime,
				ActiveStopTime:      m.ActiveStopTime,
			},
			Hedge: OrderHedge{
				Type:                  HedgeType(m.HedgeType),
				Param:                 m.HedgeParam,
				DisableAutomaticPrice: disableAutomaticHedgePrice,
				MaxSize:               intPointer(m.HedgeMaxSize, "hedge maximum size"),
			},
			Algorithm: OrderAlgorithm{Strategy: m.AlgoStrategy, Params: tagValuesFromCodec(m.AlgoParams)},
			Conditions: OrderConditions{
				Values:      orderConditionsFromCodec(m.Conditions),
				IgnoreRTH:   boolValue(m.ConditionsIgnoreRTH, "conditions ignore RTH"),
				CancelOrder: boolValue(m.ConditionsCancelOrder, "conditions cancel order"),
			},
			PeggedBenchmark: peggedBenchmark,
			Adjustment: OrderAdjustmentDetails{
				OrderType:      OrderType(m.AdjustedOrderType),
				TriggerPrice:   decimalPointer(m.TriggerPrice, "adjustment trigger price"),
				StopPrice:      decimalPointer(m.AdjustedStopPrice, "adjusted stop price"),
				StopLimitPrice: decimalPointer(m.AdjustedStopLimitPrice, "adjusted stop limit price"),
				TrailingAmount: decimalPointer(m.AdjustedTrailingAmount, "adjusted trailing amount"),
				TrailingUnit:   intPointer(m.AdjustableTrailingUnit, "adjustable trailing unit"),
			},
			Compliance: OrderComplianceDetails{
				Solicited:            boolValue(m.Solicited, "solicited"),
				OMSContainer:         boolValue(m.IsOMSContainer, "OMS container"),
				Shareholder:          m.Shareholder,
				CustomerAccount:      m.CustomerAccount,
				ProfessionalCustomer: boolValue(m.ProfessionalCustomer, "professional customer"),
				Submitter:            m.Submitter,
			},
			UsePriceMgmtAlgo:      boolPointer(m.UsePriceMgmtAlgo, "use price management algo"),
			AdvancedErrorOverride: m.AdvancedErrorOverride,
			ManualOrderTime:       m.ManualOrderTime,
			Deactivate:            boolPointer(m.Deactivate, "deactivate"),
			PostOnly:              boolPointer(m.PostOnly, "post only"),
			AllowPreOpen:          boolPointer(m.AllowPreOpen, "allow pre-open"),
			IgnoreOpenAuction:     boolPointer(m.IgnoreOpenAuction, "ignore open auction"),
			SeekPriceImprovement:  boolPointer(m.SeekPriceImprovement, "seek price improvement"),
			WhatIfType:            intPointer(m.WhatIfType, "what-if type"),
		},
		Completion: CompletedOrderCompletion{
			Status:                    OrderStatus(m.Status),
			Filled:                    filled,
			CommissionAndFees:         decimalPointer(m.CommissionAndFees, "commission and fees"),
			CommissionAndFeesCurrency: m.CommissionCurrency,
			AutoCancelDate:            m.AutoCancelDate,
			AutoCancelParent:          boolValue(m.AutoCancelParent, "auto cancel parent"),
			ParentPermID:              int64Pointer(m.ParentPermID, "parent permanent id"),
			Time:                      m.CompletedTime,
			StatusText:                m.CompletedStatus,
		},
	}
	if parseErr != nil {
		return CompletedOrderResult{}, parseErr
	}
	return result, nil
}

func contractIDPointer(value *int) *ContractID {
	if value == nil {
		return nil
	}
	return new(protocolIDFromInt[ContractID](*value))
}
