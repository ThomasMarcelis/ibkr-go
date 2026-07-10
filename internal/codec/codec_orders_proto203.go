package codec

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
	"google.golang.org/protobuf/encoding/protowire"
)

func (PlaceOrderRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufPlaceOrder
}

func (m PlaceOrderRequest) encodeProto(sv int) ([]byte, error) {
	if sv < m.protobufVersion() {
		return nil, fmt.Errorf("codec: place order protobuf requires server_version %d", m.protobufVersion())
	}
	orderID, err := protoInt32FromInt64(m.OrderID, "place order id")
	if err != nil {
		return nil, err
	}
	contract, err := encodeOrderContractProto(m.Contract, m.ComboLegs, m.OrderComboLegPrices)
	if err != nil {
		return nil, err
	}
	order, err := encodeOrderProto(m)
	if err != nil {
		return nil, err
	}
	body := appendProtoVarint(nil, 1, orderID)
	body = appendProtoMessage(body, 2, contract)
	body = appendProtoMessage(body, 3, order)
	// API 10.48.01 always emits the optional AttachedOrders message, even
	// when it has no fields. ibkr-go does not expose the newer attached-order
	// shortcut; brackets remain three ordinary place-order requests.
	return appendProtoMessage(body, 4, nil), nil
}

func (CancelOrderRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufPlaceOrder
}

func (m CancelOrderRequest) encodeProto(sv int) ([]byte, error) {
	if sv < m.protobufVersion() {
		return nil, fmt.Errorf("codec: cancel order protobuf requires server_version %d", m.protobufVersion())
	}
	orderID, err := protoInt32FromInt64(m.OrderID, "cancel order id")
	if err != nil {
		return nil, err
	}
	cancel, err := encodeOrderCancelProto(m.ManualOrderCancelTime, m.ExtOperator, m.ManualOrderIndicator)
	if err != nil {
		return nil, err
	}
	body := appendProtoVarint(nil, 1, orderID)
	return appendProtoMessage(body, 2, cancel), nil
}

func (GlobalCancelRequest) protobufVersion() int {
	return protocol.MinServerVersionProtobufPlaceOrder
}

func (m GlobalCancelRequest) encodeProto(sv int) ([]byte, error) {
	if sv < m.protobufVersion() {
		return nil, fmt.Errorf("codec: global cancel protobuf requires server_version %d", m.protobufVersion())
	}
	cancel, err := encodeOrderCancelProto("", m.ExtOperator, m.ManualOrderIndicator)
	if err != nil {
		return nil, err
	}
	return appendProtoMessage(nil, 1, cancel), nil
}

func decodeOrderStatusProto(body []byte, sv int) ([]Message, error) {
	m := OrderStatus{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 6, 7, 9:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("order status", number, err)
			}
			switch number {
			case 1:
				m.OrderID = int64(decodeProtoInt32(value))
			case 6:
				m.PermID = i64toa(decodeProtoInt64(value))
			case 7:
				m.ParentID = itoa(decodeProtoInt32(value))
			case 9:
				m.ClientID = itoa(decodeProtoInt32(value))
			}
		case 2, 3, 4, 10:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("order status", number, err)
			}
			switch number {
			case 2:
				m.Status = string(value)
			case 3:
				m.Filled = string(value)
			case 4:
				m.Remaining = string(value)
			case 10:
				m.WhyHeld = string(value)
			}
		case 5, 8, 11:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("order status", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 5:
				m.AvgFillPrice = formatted
			case 8:
				m.LastFillPrice = formatted
			case 11:
				m.MktCapPrice = formatted
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("order status", number, err)
			}
		}
	}
}

func decodeOpenOrderProto(body []byte, sv int) ([]Message, error) {
	m := OpenOrder{}
	var hasContract, hasOrder, hasOrderState bool
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			if !hasContract || !hasOrder || !hasOrderState {
				return nil, fmt.Errorf("open order missing required emitted messages: contract=%t order=%t order_state=%t", hasContract, hasOrder, hasOrderState)
			}
			return []Message{m}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("open order", number, err)
			}
			m.OrderID = int64(decodeProtoInt32(value))
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("open order", number, err)
			}
			contract, legs, prices, err := decodeOpenOrderContractProto(value)
			if err != nil {
				return nil, protoFieldError("open order contract", number, err)
			}
			m.Contract, m.ComboLegs, m.OrderComboLegPrices = contract, legs, prices
			hasContract = true
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("open order", number, err)
			}
			if err := decodeOpenOrderOrderProto(value, &m); err != nil {
				return nil, protoFieldError("open order order", number, err)
			}
			hasOrder = true
		case 4:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("open order", number, err)
			}
			if err := decodeOpenOrderStateProto(value, &m); err != nil {
				return nil, protoFieldError("open order state", number, err)
			}
			hasOrderState = true
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("open order", number, err)
			}
		}
	}
}

func decodeOpenOrderContractProto(body []byte) (Contract, []ComboLeg, []string, error) {
	decoded, err := decodeSharedContractProto(body)
	return decoded.Contract, decoded.ComboLegs, decoded.ComboLegPrices, err
}

func decodeComboLegProto(body []byte) (ComboLeg, string, error) {
	m := ComboLeg{}
	price := ""
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return ComboLeg{}, "", err
		}
		if !ok {
			return m, price, nil
		}
		switch number {
		case 1, 2, 5, 6, 8:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return ComboLeg{}, "", protoFieldError("combo leg", number, err)
			}
			formatted := itoa(decodeProtoInt32(value))
			switch number {
			case 1:
				m.ConID = decodeProtoInt32(value)
			case 2:
				m.Ratio = decodeProtoInt32(value)
			case 5:
				m.OpenClose = formatted
			case 6:
				m.ShortSaleSlot = formatted
			case 8:
				m.ExemptCode = formatted
			}
		case 3, 4, 7:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return ComboLeg{}, "", protoFieldError("combo leg", number, err)
			}
			switch number {
			case 3:
				m.Action = string(value)
			case 4:
				m.Exchange = string(value)
			case 7:
				m.DesignatedLocation = string(value)
			}
		case 9:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return ComboLeg{}, "", protoFieldError("combo leg", number, err)
			}
			price = formatProtoDouble(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return ComboLeg{}, "", protoFieldError("combo leg", number, err)
			}
		}
	}
}

func decodeOpenOrderOrderProto(body []byte, m *OpenOrder) error {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		switch number {
		case 1, 3, 4, 69:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			switch number {
			case 1:
				m.ClientID = itoa(decodeProtoInt32(value))
			case 3:
				m.PermID = i64toa(decodeProtoInt64(value))
			case 4:
				m.ParentID = itoa(decodeProtoInt32(value))
			case 69:
				m.Origin = itoa(decodeProtoInt32(value))
			}
		case 5, 6, 8, 11, 12, 27, 28, 61, 68:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			switch number {
			case 5:
				m.Action = string(value)
			case 6:
				m.Quantity = string(value)
			case 8:
				m.OrderType = string(value)
			case 11:
				m.TIF = string(value)
			case 12:
				m.Account = string(value)
			case 27:
				m.OcaGroup = string(value)
			case 28:
				m.OrderRef = string(value)
			case 61:
				m.AlgoStrategy = string(value)
			case 68:
				m.OpenClose = string(value)
			}
		case 9, 10, 76:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 9:
				m.LmtPrice = formatted
			case 10:
				m.AuxPrice = formatted
			case 76:
				m.DiscretionAmt = formatted
			}
		case 18, 19, 101, 102:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			formatted := protoBoolString(value)
			switch number {
			case 18:
				m.Hidden = formatted
			case 19:
				m.OutsideRTH = formatted
			case 101:
				m.ConditionsCancelOrder = formatted
			case 102:
				m.ConditionsIgnoreRTH = formatted
			}
		case 25:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			m.GoodAfterTime = string(value)
		case 62, 64:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			pair, err := decodeProtoMapEntry(value)
			if err != nil {
				return protoFieldError("order map", number, err)
			}
			if number == 62 {
				m.AlgoParams = append(m.AlgoParams, pair)
			} else {
				m.SmartComboRouting = append(m.SmartComboRouting, pair)
			}
		case 100:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			condition, err := decodeOrderConditionProto(value)
			if err != nil {
				return protoFieldError("order condition", number, err)
			}
			m.Conditions = append(m.Conditions, condition)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return protoFieldError("order", number, err)
			}
		}
	}
}

func decodeProtoMapEntry(body []byte) (TagValue, error) {
	m := TagValue{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return TagValue{}, err
		}
		if !ok {
			return m, nil
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return TagValue{}, err
			}
			if number == 1 {
				m.Tag = string(value)
			} else {
				m.Value = string(value)
			}
			continue
		}
		if err := skipProtoField(&body, number, typ); err != nil {
			return TagValue{}, err
		}
	}
}

func decodeOrderConditionProto(body []byte) (OrderCondition, error) {
	m := OrderCondition{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return OrderCondition{}, err
		}
		if !ok {
			if m.Type < 1 || m.Type > 7 || m.Type == 2 {
				return OrderCondition{}, fmt.Errorf("unsupported type %d", m.Type)
			}
			return m, nil
		}
		switch number {
		case 1, 2, 3, 4, 8, 11, 13:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return OrderCondition{}, protoFieldError("condition", number, err)
			}
			switch number {
			case 1:
				m.Type = decodeProtoInt32(value)
			case 2:
				if value == 0 {
					m.Conjunction = "o"
				} else {
					m.Conjunction = "a"
				}
			case 3:
				if value == 0 {
					m.Operator = 1
				} else {
					m.Operator = 2
				}
			case 4:
				m.ConID = decodeProtoInt32(value)
			case 8, 13:
				m.Value = itoa(decodeProtoInt32(value))
			case 11:
				m.TriggerMethod = decodeProtoInt32(value)
			}
		case 5, 6, 7, 12:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return OrderCondition{}, protoFieldError("condition", number, err)
			}
			switch number {
			case 5:
				m.Exchange = string(value)
			case 6:
				m.Symbol = string(value)
			case 7:
				m.SecType = string(value)
			case 12:
				m.Value = string(value)
			}
		case 9, 10:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return OrderCondition{}, protoFieldError("condition", number, err)
			}
			m.Value = formatProtoDouble(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return OrderCondition{}, protoFieldError("condition", number, err)
			}
		}
	}
}

func decodeOpenOrderStateProto(body []byte, m *OpenOrder) error {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		switch number {
		case 1, 14, 28:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("order state", number, err)
			}
			switch number {
			case 1:
				m.Status = string(value)
			case 14:
				m.CommissionCurrency = string(value)
			case 28:
				m.WarningText = string(value)
			}
		case 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return protoFieldError("order state", number, err)
			}
			formatted := formatProtoDouble(value)
			switch number {
			case 2:
				m.InitMarginBefore = formatted
			case 3:
				m.MaintMarginBefore = formatted
			case 4:
				m.EquityWithLoanBefore = formatted
			case 5:
				m.InitMarginChange = formatted
			case 6:
				m.MaintMarginChange = formatted
			case 7:
				m.EquityWithLoanChange = formatted
			case 8:
				m.InitMarginAfter = formatted
			case 9:
				m.MaintMarginAfter = formatted
			case 10:
				m.EquityWithLoanAfter = formatted
			case 11:
				m.Commission = formatted
			case 12:
				m.MinCommission = formatted
			case 13:
				m.MaxCommission = formatted
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return protoFieldError("order state", number, err)
			}
		}
	}
}

func encodeOrderCancelProto(manualTime, extOperator, manualIndicator string) ([]byte, error) {
	body := make([]byte, 0, 32)
	if manualTime != "" {
		body = appendProtoString(body, 1, manualTime)
	}
	if extOperator != "" {
		body = appendProtoString(body, 2, extOperator)
	}
	return appendOptionalProtoInt32(body, 3, manualIndicator, "manual order indicator")
}

func encodeOrderContractProto(contract Contract, legs []ComboLeg, legPrices []string) ([]byte, error) {
	return encodeSharedContractProto(contract, legs, legPrices, false)
}

func encodeComboLegProto(leg ComboLeg, price string) ([]byte, error) {
	body := make([]byte, 0, 48)
	var err error
	if leg.ConID != 0 {
		body, err = appendProtoInt(body, 1, leg.ConID, "conid")
		if err != nil {
			return nil, err
		}
	}
	if leg.Ratio != 0 {
		body, err = appendProtoInt(body, 2, leg.Ratio, "ratio")
		if err != nil {
			return nil, err
		}
	}
	if leg.Action != "" {
		body = appendProtoString(body, 3, leg.Action)
	}
	if leg.Exchange != "" {
		body = appendProtoString(body, 4, leg.Exchange)
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
		label  string
	}{{5, leg.OpenClose, "open close"}, {6, leg.ShortSaleSlot, "short sale slot"}, {8, leg.ExemptCode, "exempt code"}} {
		body, err = appendOptionalProtoInt32(body, field.number, field.value, field.label)
		if err != nil {
			return nil, err
		}
	}
	if leg.DesignatedLocation != "" {
		body = appendProtoString(body, 7, leg.DesignatedLocation)
	}
	body, err = appendOptionalProtoDouble(body, 9, price, "per-leg price")
	if err != nil {
		return nil, err
	}
	return canonicalProtoFields(body), nil
}

func encodeOrderProto(m PlaceOrderRequest) ([]byte, error) {
	body := make([]byte, 0, 320)
	var err error
	for _, field := range []struct {
		number protowire.Number
		value  string
	}{
		{5, m.Action}, {6, m.TotalQuantity}, {8, m.OrderType}, {11, m.TIF},
		{12, m.Account}, {13, m.SettlingFirm}, {14, m.ClearingAccount},
		{15, m.ClearingIntent}, {25, m.GoodAfterTime}, {26, m.GoodTillDate},
		{27, m.OcaGroup}, {28, m.OrderRef}, {29, m.Rule80A},
		{32, m.ActiveStartTime}, {33, m.ActiveStopTime}, {34, m.FAGroup},
		{35, m.FAMethod}, {36, m.FAPercentage}, {41, m.DeltaNeutralOrderType},
		{58, m.ScaleTable}, {59, m.HedgeType}, {60, m.HedgeParam},
		{63, m.AlgoID}, {68, m.OpenClose}, {71, m.DesignatedLocation},
		{93, m.AdjustedOrderType}, {103, m.ModelCode}, {104, m.ExtOperator},
		{107, m.Mifid2DecisionMaker}, {108, m.Mifid2DecisionAlgo},
		{109, m.Mifid2ExecutionTrader}, {110, m.Mifid2ExecutionAlgo},
		{125, m.AdvancedErrorOverride}, {126, m.ManualOrderTime},
		{132, m.CustomerAccount},
	} {
		if field.value != "" {
			body = appendProtoString(body, field.number, field.value)
		}
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
		label  string
	}{
		{4, m.ParentID, "parent id"},
		{7, m.DisplaySize, "display size"}, {24, m.MinQty, "minimum quantity"},
		{30, m.OcaType, "oca type"}, {31, m.TriggerMethod, "trigger method"},
		{38, m.VolatilityType, "volatility type"}, {40, m.ReferencePriceType, "reference price type"},
		{48, m.ScaleInitLevelSize, "scale initial level size"},
		{49, m.ScaleSubsLevelSize, "scale subsequent level size"},
		{69, m.Origin, "origin"}, {70, m.ShortSaleSlot, "short sale slot"},
		{72, m.ExemptCode, "exempt code"}, {98, m.AdjustableTrailingUnit, "adjustable trailing unit"},
		{122, m.UsePriceMgmtAlgo, "use price management algo"},
		{123, m.Duration, "duration"}, {124, m.PostToAts, "post to ATS"},
		{136, m.ManualOrderIndicator, "manual order indicator"},
	} {
		body, err = appendOptionalProtoInt32(body, field.number, field.value, field.label)
		if err != nil {
			return nil, err
		}
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
		label  string
	}{
		{9, m.LmtPrice, "limit price"}, {10, m.AuxPrice, "aux price"},
		{21, m.PercentOffset, "percent offset"}, {22, m.TrailingPercent, "trailing percent"},
		{23, m.TrailStopPrice, "trail stop price"}, {37, m.Volatility, "volatility"},
		{42, m.DeltaNeutralAuxPrice, "delta-neutral aux price"},
		{50, m.ScalePriceIncrement, "scale price increment"},
		{76, m.DiscretionaryAmt, "discretionary amount"}, {78, m.StartingPrice, "starting price"},
		{79, m.StockRefPrice, "stock reference price"}, {80, m.Delta, "delta"},
		{81, m.StockRangeLower, "stock range lower"}, {82, m.StockRangeUpper, "stock range upper"},
		{94, m.TriggerPrice, "adjustment trigger price"},
		{95, m.AdjustedStopPrice, "adjusted stop price"},
		{96, m.AdjustedStopLimitPrice, "adjusted stop limit price"},
		{97, m.AdjustedTrailingAmount, "adjusted trailing amount"},
		{99, m.LmtPriceOffset, "limit price offset"}, {106, m.CashQty, "cash quantity"},
	} {
		body, err = appendOptionalProtoDouble(body, field.number, field.value, field.label)
		if err != nil {
			return nil, err
		}
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
		label  string
	}{
		{16, m.AllOrNone, "all or none"}, {17, m.BlockOrder, "block order"},
		{18, m.Hidden, "hidden"}, {19, m.OutsideRTH, "outside RTH"},
		{20, m.SweepToFill, "sweep to fill"}, {39, m.ContinuousUpdate, "continuous update"},
		{65, m.WhatIf, "what if"}, {66, m.Transmit, "transmit"},
		{67, m.OverridePercentageConstraints, "override percentage constraints"},
		{77, m.OptOutSmartRouting, "opt out smart routing"}, {83, m.NotHeld, "not held"},
		{85, m.Solicited, "solicited"}, {86, m.RandomizeSize, "randomize size"},
		{87, m.RandomizePrice, "randomize price"},
		{101, m.ConditionsCancelOrder, "conditions cancel order"},
		{102, m.ConditionsIgnoreRTH, "conditions ignore RTH"},
		{111, m.DontUseAutoPriceForHedge, "disable automatic hedge price"},
		{112, m.IsOmsContainer, "OMS container"},
		{113, m.DiscretionaryUpToLimitPrice, "discretionary up to limit price"},
		{117, m.AutoCancelParent, "auto cancel parent"},
		{119, m.ImbalanceOnly, "imbalance only"},
		{133, m.ProfessionalCustomer, "professional customer"},
		{135, m.IncludeOvernight, "include overnight"},
	} {
		body, err = appendOptionalProtoTrue(body, field.number, field.value, field.label)
		if err != nil {
			return nil, err
		}
	}
	if m.DeltaNeutralContractPresent != "" && m.DeltaNeutralContractPresent != "0" {
		return nil, fmt.Errorf("codec: delta-neutral contract protobuf encoding is not exposed")
	}
	if m.OrderMiscOptions != "" {
		return nil, fmt.Errorf("codec: order misc options protobuf map cannot be encoded from %q", m.OrderMiscOptions)
	}
	if m.AlgoStrategy != "" {
		body = appendProtoString(body, 61, m.AlgoStrategy)
		body = appendProtoMap(body, 62, m.AlgoParams)
	} else if len(m.AlgoParams) != 0 {
		return nil, fmt.Errorf("codec: algorithm parameters require an algorithm strategy")
	}
	body = appendProtoMap(body, 64, m.SmartComboRoutingParams)
	for i, condition := range m.Conditions {
		encoded, err := encodeOrderConditionProto(condition)
		if err != nil {
			return nil, fmt.Errorf("codec: order condition %d: %w", i, err)
		}
		body = appendProtoMessage(body, 100, encoded)
	}
	softDollar := make([]byte, 0, 16)
	if m.SoftDollarName != "" {
		softDollar = appendProtoString(softDollar, 1, m.SoftDollarName)
	}
	if m.SoftDollarValue != "" {
		softDollar = appendProtoString(softDollar, 2, m.SoftDollarValue)
	}
	// API 10.48.01 always emits the optional SoftDollarTier message.
	body = appendProtoMessage(body, 105, softDollar)
	return canonicalProtoFields(body), nil
}

func encodeOrderConditionProto(condition OrderCondition) ([]byte, error) {
	if condition.Type < 1 || condition.Type > 7 || condition.Type == 2 {
		return nil, fmt.Errorf("unsupported type %d", condition.Type)
	}
	body, err := appendProtoInt(nil, 1, condition.Type, "type")
	if err != nil {
		return nil, err
	}
	body = appendProtoVarint(body, 2, boolVarint(condition.Conjunction != "o"))
	isMore := condition.Operator == 2
	switch condition.Type {
	case 1:
		body = appendProtoVarint(body, 3, boolVarint(isMore))
		body, err = appendProtoInt(body, 4, condition.ConID, "conid")
		if err == nil && condition.Exchange != "" {
			body = appendProtoString(body, 5, condition.Exchange)
		}
		if err == nil {
			body, err = appendRequiredProtoDouble(body, 10, condition.Value, "price")
		}
		if err == nil {
			body, err = appendProtoInt(body, 11, condition.TriggerMethod, "trigger method")
		}
	case 3:
		body = appendProtoVarint(body, 3, boolVarint(isMore))
		if condition.Value == "" {
			err = fmt.Errorf("time is empty")
		} else {
			body = appendProtoString(body, 12, condition.Value)
		}
	case 4:
		body = appendProtoVarint(body, 3, boolVarint(isMore))
		body, err = appendRequiredProtoInt32(body, 8, condition.Value, "margin percent")
	case 5:
		if condition.Exchange != "" {
			body = appendProtoString(body, 5, condition.Exchange)
		}
		if condition.Symbol != "" {
			body = appendProtoString(body, 6, condition.Symbol)
		}
		if condition.SecType != "" {
			body = appendProtoString(body, 7, condition.SecType)
		}
	case 6:
		body = appendProtoVarint(body, 3, boolVarint(isMore))
		body, err = appendProtoInt(body, 4, condition.ConID, "conid")
		if err == nil && condition.Exchange != "" {
			body = appendProtoString(body, 5, condition.Exchange)
		}
		if err == nil {
			body, err = appendRequiredProtoInt32(body, 13, condition.Value, "volume")
		}
	case 7:
		body = appendProtoVarint(body, 3, boolVarint(isMore))
		body, err = appendProtoInt(body, 4, condition.ConID, "conid")
		if err == nil && condition.Exchange != "" {
			body = appendProtoString(body, 5, condition.Exchange)
		}
		if err == nil {
			body, err = appendRequiredProtoDouble(body, 9, condition.Value, "change percent")
		}
	}
	if err != nil {
		return nil, err
	}
	return canonicalProtoFields(body), nil
}

func appendProtoMap(body []byte, number protowire.Number, values []TagValue) []byte {
	if len(values) == 0 {
		return body
	}
	byKey := make(map[string]string, len(values))
	for _, value := range values {
		byKey[value.Tag] = value.Value
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry := appendProtoString(nil, 1, key)
		entry = appendProtoString(entry, 2, byKey[key])
		body = appendProtoMessage(body, number, entry)
	}
	return body
}

func appendOptionalProtoInt32(body []byte, number protowire.Number, value, label string) ([]byte, error) {
	if value == "" {
		return body, nil
	}
	return appendRequiredProtoInt32(body, number, value, label)
}

func appendRequiredProtoInt32(body []byte, number protowire.Number, value, label string) ([]byte, error) {
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("codec: %s %q is not protobuf int32: %w", label, value, err)
	}
	// ParseInt's 32-bit bound proves the conversion. Negative protobuf int32
	// values use a sign-extended ten-byte varint.
	return appendProtoVarint(body, number, uint64(parsed)), nil // #nosec G115 -- protobuf int32 bounds checked above
}

func appendProtoInt(body []byte, number protowire.Number, value int, label string) ([]byte, error) {
	encoded, err := encodeProtoInt32(value, label)
	if err != nil {
		return nil, err
	}
	return appendProtoVarint(body, number, encoded), nil
}

func appendOptionalProtoDouble(body []byte, number protowire.Number, value, label string) ([]byte, error) {
	if value == "" {
		return body, nil
	}
	return appendRequiredProtoDouble(body, number, value, label)
}

func appendRequiredProtoDouble(body []byte, number protowire.Number, value, label string) ([]byte, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil, fmt.Errorf("codec: %s %q is not a finite protobuf double", label, value)
	}
	return appendProtoDouble(body, number, parsed), nil
}

func appendOptionalProtoTrue(body []byte, number protowire.Number, value, label string) ([]byte, error) {
	switch value {
	case "", "0":
		return body, nil
	case "1":
		return appendProtoVarint(body, number, 1), nil
	default:
		return nil, fmt.Errorf("codec: %s %q is not a protobuf bool", label, value)
	}
}

func protoInt32FromInt64(value int64, label string) (uint64, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("codec: %s %d exceeds protobuf int32", label, value)
	}
	return uint64(value), nil // #nosec G115 -- protobuf int32 bounds checked above
}

func boolVarint(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

func legPriceAt(prices []string, i int) string {
	if i < len(prices) {
		return prices[i]
	}
	return ""
}

func canonicalProtoFields(body []byte) []byte {
	type field struct {
		number protowire.Number
		bytes  []byte
	}
	fields := make([]field, 0, 16)
	for len(body) != 0 {
		number, _, n := protowire.ConsumeField(body)
		if n < 0 {
			panic(protowire.ParseError(n))
		}
		fields = append(fields, field{number: number, bytes: body[:n]})
		body = body[n:]
	}
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].number < fields[j].number })
	canonical := make([]byte, 0)
	for _, field := range fields {
		canonical = append(canonical, field.bytes...)
	}
	return canonical
}
