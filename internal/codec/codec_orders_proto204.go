package codec

import "fmt"

//nolint:unparam // protobuf encoders share one fallible contract.
func (m OpenOrdersRequest) encodeProto(sv int) ([]byte, error) {
	if m.Scope == "auto" {
		return appendProtoVarint(nil, 1, 1), nil
	}
	return nil, nil
}

func (m CancelOpenOrders) encodeProto(sv int) ([]byte, error) {
	// optional false is absent in the official protobuf request.
	return []byte{}, nil
}

//nolint:unparam // protobuf encoders share one fallible contract.
func (m CompletedOrdersRequest) encodeProto(sv int) ([]byte, error) {
	if m.APIOnly {
		return appendProtoVarint(nil, 1, 1), nil
	}
	// optional false is absent in the official protobuf request.
	return nil, nil
}

func decodeCompletedOrderProto(body []byte, sv int) ([]Message, error) {
	m := CompletedOrder{}
	var hasContract, hasOrder, hasOrderState bool
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			if !hasContract || !hasOrder || !hasOrderState {
				return nil, fmt.Errorf("completed order missing required emitted messages: contract=%t order=%t order_state=%t", hasContract, hasOrder, hasOrderState)
			}
			return []Message{m}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("completed order", number, err)
			}
			contract, description, prices, err := decodeCompletedOrderContractProto(value)
			if err != nil {
				return nil, protoFieldError("completed order contract", number, err)
			}
			m.Contract = contract
			m.ComboLegsDescription = description
			m.OrderComboLegPrices = prices
			hasContract = true
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("completed order", number, err)
			}
			if err := decodeCompletedOrderOrderProto(value, &m); err != nil {
				return nil, protoFieldError("completed order order", number, err)
			}
			hasOrder = true
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("completed order", number, err)
			}
			if err := decodeCompletedOrderStateProto(value, &m); err != nil {
				return nil, protoFieldError("completed order state", number, err)
			}
			hasOrderState = true
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("completed order", number, err)
			}
		}
	}
}

func decodeCompletedOrdersEndProto(body []byte, sv int) ([]Message, error) {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{CompletedOrderEnd{}}, nil
		}
		if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("completed orders end", number, err)
		}
	}
}

func decodeCompletedOrderContractProto(body []byte) (Contract, string, []string, error) {
	decoded, err := decodeSharedContractProto(body)
	return decoded.Contract, decoded.ComboLegsDescription, decoded.ComboLegPrices, err
}

func decodeCompletedOrderOrderProto(body []byte, m *CompletedOrder) error {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		switch number {
		case 1, 2, 3, 4, 7, 16, 18, 19, 20, 24, 30, 31, 38, 39, 40, 43, 45, 46,
			48, 49, 52, 54, 55, 56, 57, 69, 70, 72, 83, 85, 86, 87, 88, 90, 101,
			102, 111, 112, 116, 117, 119, 120, 121, 127, 128, 133:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			formattedInt := itoa(decodeProtoInt32(value))
			formattedBool := protoBoolString(value)
			switch number {
			case 1:
				m.ClientID = formattedInt
			case 2:
				m.OrderID = formattedInt
			case 3:
				m.PermID = i64toa(decodeProtoInt64(value))
			case 4:
				m.ParentID = formattedInt
			case 7:
				m.DisplaySize = formattedInt
			case 16:
				m.AllOrNone = formattedBool
			case 18:
				m.Hidden = formattedBool
			case 19:
				m.OutsideRTH = formattedBool
			case 20:
				m.SweepToFill = formattedBool
			case 24:
				m.MinQty = formattedInt
			case 30:
				m.OcaType = formattedInt
			case 31:
				m.TriggerMethod = formattedInt
			case 38:
				m.VolatilityType = formattedInt
			case 39:
				m.ContinuousUpdate = formattedBool
			case 40:
				m.ReferencePriceType = formattedInt
			case 43:
				m.DeltaNeutralConID = formattedInt
			case 45:
				m.DeltaNeutralShortSale = formattedBool
			case 46:
				m.DeltaNeutralShortSaleSlot = formattedInt
			case 48:
				m.ScaleInitLevelSize = formattedInt
			case 49:
				m.ScaleSubsLevelSize = formattedInt
			case 52:
				m.ScalePriceAdjustInterval = formattedInt
			case 54:
				m.ScaleAutoReset = formattedBool
			case 55:
				m.ScaleInitPosition = formattedInt
			case 56:
				m.ScaleInitFillQty = formattedInt
			case 57:
				m.ScaleRandomPercent = formattedBool
			case 69:
				m.Origin = formattedInt
			case 70:
				m.ShortSaleSlot = formattedInt
			case 72:
				m.ExemptCode = formattedInt
			case 83:
				m.NotHeld = formattedBool
			case 85:
				m.Solicited = formattedBool
			case 86:
				m.RandomizeSize = formattedBool
			case 87:
				m.RandomizePrice = formattedBool
			case 88:
				m.ReferenceContractID = formattedInt
			case 90:
				m.PeggedChangeAmountDecrease = formattedBool
			case 101:
				m.ConditionsCancelOrder = formattedBool
			case 102:
				m.ConditionsIgnoreRTH = formattedBool
			case 111:
				m.DontUseAutoPriceForHedge = formattedBool
			case 112:
				m.IsOMSContainer = formattedBool
			case 116:
				m.RefFuturesConID = formattedInt
			case 117:
				m.AutoCancelParent = formattedBool
			case 119:
				m.ImbalanceOnly = formattedBool
			case 120:
				m.RouteMarketableToBBO = formattedInt
			case 121:
				m.ParentPermID = i64toa(decodeProtoInt64(value))
			case 127:
				m.MinTradeQty = formattedInt
			case 128:
				m.MinCompeteSize = formattedInt
			case 133:
				m.ProfessionalCustomer = formattedBool
			}
		case 5, 6, 8, 11, 12, 13, 14, 15, 25, 26, 27, 28, 29, 34, 35, 36, 41, 47,
			59, 60, 61, 68, 71, 92, 103, 114, 115, 118, 132, 137:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("order", number, err)
			}
			formatted := string(value)
			switch number {
			case 5:
				m.Action = formatted
			case 6:
				m.Quantity = formatted
			case 8:
				m.OrderType = formatted
			case 11:
				m.TIF = formatted
			case 12:
				m.Account = formatted
			case 13:
				m.SettlingFirm = formatted
			case 14:
				m.ClearingAccount = formatted
			case 15:
				m.ClearingIntent = formatted
			case 25:
				m.GoodAfterTime = formatted
			case 26:
				m.GoodTillDate = formatted
			case 27:
				m.OcaGroup = formatted
			case 28:
				m.OrderRef = formatted
			case 29:
				m.Rule80A = formatted
			case 34:
				m.FAGroup = formatted
			case 35:
				m.FAMethod = formatted
			case 36:
				m.FAPercentage = formatted
			case 41:
				m.DeltaNeutralOrderType = formatted
			case 47:
				m.DeltaNeutralDesignatedLocation = formatted
			case 59:
				m.HedgeType = formatted
			case 60:
				m.HedgeParam = formatted
			case 61:
				m.AlgoStrategy = formatted
			case 68:
				m.OpenClose = formatted
			case 71:
				m.DesignatedLocation = formatted
			case 92:
				m.ReferenceExchangeID = formatted
			case 103:
				m.ModelCode = formatted
			case 114:
				m.AutoCancelDate = formatted
			case 115:
				m.Filled = formatted
			case 118:
				m.Shareholder = formatted
			case 132:
				m.CustomerAccount = formatted
			case 137:
				m.Submitter = formatted
			}
		case 9, 10, 21, 22, 23, 37, 42, 50, 51, 53, 76, 78, 79, 80, 81, 82, 89,
			91, 94, 99, 106, 129, 130, 131:
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
			case 21:
				m.PercentOffset = formatted
			case 22:
				m.TrailingPercent = formatted
			case 23:
				m.TrailStopPrice = formatted
			case 37:
				m.Volatility = formatted
			case 42:
				m.DeltaNeutralAuxPrice = formatted
			case 50:
				m.ScalePriceIncrement = formatted
			case 51:
				m.ScalePriceAdjustValue = formatted
			case 53:
				m.ScaleProfitOffset = formatted
			case 76:
				m.DiscretionAmt = formatted
			case 78:
				m.StartingPrice = formatted
			case 79:
				m.StockRefPrice = formatted
			case 80:
				m.Delta = formatted
			case 81:
				m.StockRangeLower = formatted
			case 82:
				m.StockRangeUpper = formatted
			case 89:
				m.PeggedChangeAmount = formatted
			case 91:
				m.ReferenceChangeAmount = formatted
			case 94:
				m.StopPrice = formatted
			case 99:
				m.LmtPriceOffset = formatted
			case 106:
				m.CashQty = formatted
			case 129:
				m.CompeteAgainstBestOffset = formatted
			case 130:
				m.MidOffsetAtWhole = formatted
			case 131:
				m.MidOffsetAtHalf = formatted
			}
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
			// Exact 204 emits several defaults from the larger Order schema that
			// have no completed-order API meaning. Keep them out of the public
			// result while still accepting future unknown protobuf fields.
			if err := skipProtoField(&body, number, typ); err != nil {
				return protoFieldError("order", number, err)
			}
		}
	}
}

func decodeCompletedOrderStateProto(body []byte, m *CompletedOrder) error {
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		switch number {
		case 1, 14, 29, 30:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return protoFieldError("order state", number, err)
			}
			switch number {
			case 1:
				m.Status = string(value)
			case 14:
				m.CommissionCurrency = string(value)
			case 29:
				m.CompletedTime = string(value)
			case 30:
				m.CompletedStatus = string(value)
			}
		case 11:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return protoFieldError("order state", number, err)
			}
			m.CommissionAndFees = formatProtoDouble(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return protoFieldError("order state", number, err)
			}
		}
	}
}
