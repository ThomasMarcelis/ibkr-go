package codec

import (
	"fmt"
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

// decodedContractProto carries the response-only metadata and order-leg prices
// that accompany the canonical Contract in the official shared schema.
type decodedContractProto struct {
	Contract             Contract
	Description          string
	ComboLegsDescription string
	ComboLegPrices       []string
	LastTradeDate        string
}

func decodeSharedContractProto(body []byte) (decodedContractProto, error) {
	m := decodedContractProto{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return decodedContractProto{}, err
		}
		if !ok {
			return m, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return decodedContractProto{}, protoFieldError("contract", number, err)
			}
			m.Contract.ConID = decodeProtoInt32(value)
		case 2, 3, 4, 6, 8, 9, 10, 11, 12, 13, 14, 15, 16, 19, 21:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return decodedContractProto{}, protoFieldError("contract", number, err)
			}
			switch number {
			case 2:
				m.Contract.Symbol = string(value)
			case 3:
				m.Contract.SecType = string(value)
			case 4:
				m.Contract.Expiry = string(value)
			case 6:
				m.Contract.Right = string(value)
			case 8:
				m.Contract.Exchange = string(value)
			case 9:
				m.Contract.PrimaryExchange = string(value)
			case 10:
				m.Contract.Currency = string(value)
			case 11:
				m.Contract.LocalSymbol = string(value)
			case 12:
				m.Contract.TradingClass = string(value)
			case 13:
				m.Contract.SecurityIDType = string(value)
			case 14:
				m.Contract.SecurityID = string(value)
			case 15:
				m.Description = string(value)
			case 16:
				m.Contract.IssuerID = string(value)
			case 19:
				m.ComboLegsDescription = string(value)
			case 21:
				m.LastTradeDate = string(value)
			}
		case 5, 7:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return decodedContractProto{}, protoFieldError("contract", number, err)
			}
			if number == 5 {
				m.Contract.Strike = formatProtoDouble(value)
			} else {
				m.Contract.Multiplier = formatProtoDouble(value)
			}
		case 17:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return decodedContractProto{}, protoFieldError("contract", number, err)
			}
			m.Contract.DeltaNeutral, err = decodeDeltaNeutralContractProto(value)
			if err != nil {
				return decodedContractProto{}, protoFieldError("delta-neutral contract", number, err)
			}
		case 18:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return decodedContractProto{}, protoFieldError("contract", number, err)
			}
			m.Contract.IncludeExpired = value != 0
		case 20:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return decodedContractProto{}, protoFieldError("contract", number, err)
			}
			leg, price, err := decodeComboLegProto(value)
			if err != nil {
				return decodedContractProto{}, protoFieldError("contract combo leg", number, err)
			}
			m.Contract.ComboLegs = append(m.Contract.ComboLegs, leg)
			m.ComboLegPrices = append(m.ComboLegPrices, price)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return decodedContractProto{}, protoFieldError("contract", number, err)
			}
		}
	}
}

func encodeSharedContractProto(contract Contract, legPrices []string, emitZeroConID bool) ([]byte, error) {
	body := make([]byte, 0, 96)
	var err error
	if contract.ConID != 0 || emitZeroConID {
		body, err = appendProtoInt(body, 1, contract.ConID, "contract conid")
		if err != nil {
			return nil, err
		}
	}
	for _, field := range []struct {
		number protowire.Number
		value  string
	}{
		{2, contract.Symbol}, {3, contract.SecType}, {4, contract.Expiry},
		{6, contract.Right}, {8, contract.Exchange}, {9, contract.PrimaryExchange},
		{10, contract.Currency}, {11, contract.LocalSymbol}, {12, contract.TradingClass},
		{13, contract.SecurityIDType}, {14, contract.SecurityID}, {16, contract.IssuerID},
	} {
		if field.value != "" {
			body = appendProtoString(body, field.number, field.value)
		}
	}
	body, err = appendOptionalProtoDouble(body, 5, contract.Strike, "contract strike")
	if err != nil {
		return nil, err
	}
	body, err = appendOptionalProtoDouble(body, 7, contract.Multiplier, "contract multiplier")
	if err != nil {
		return nil, err
	}
	if contract.DeltaNeutral != nil {
		deltaNeutral, err := encodeDeltaNeutralContractProto(*contract.DeltaNeutral)
		if err != nil {
			return nil, err
		}
		body = appendProtoMessage(body, 17, deltaNeutral)
	}
	if contract.IncludeExpired {
		body = appendProtoVarint(body, 18, 1)
	}
	if len(legPrices) > len(contract.ComboLegs) {
		return nil, fmt.Errorf("codec: %d combo leg prices for %d combo legs", len(legPrices), len(contract.ComboLegs))
	}
	for i, leg := range contract.ComboLegs {
		legBody, err := encodeComboLegProto(leg, legPriceAt(legPrices, i))
		if err != nil {
			return nil, fmt.Errorf("codec: combo leg %d: %w", i, err)
		}
		body = appendProtoMessage(body, 20, legBody)
	}
	return canonicalProtoFields(body), nil
}

func decodeDeltaNeutralContractProto(body []byte) (*DeltaNeutralContract, error) {
	// API 10.48.01 declares both double fields optional in proto3, so their
	// decoded value is zero when the sender omits either tag.
	m := &DeltaNeutralContract{Delta: "0", Price: "0"}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return m, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("delta-neutral contract", number, err)
			}
			m.ConID = decodeProtoInt32(value)
		case 2, 3:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return nil, protoFieldError("delta-neutral contract", number, err)
			}
			if number == 2 {
				m.Delta = formatProtoDouble(value)
			} else {
				m.Price = formatProtoDouble(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("delta-neutral contract", number, err)
			}
		}
	}
}

func encodeDeltaNeutralContractProto(m DeltaNeutralContract) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ConID, "delta-neutral contract conid")
	if err != nil {
		return nil, err
	}
	body, err = appendRequiredProtoDouble(body, 2, m.Delta, "delta-neutral contract delta")
	if err != nil {
		return nil, err
	}
	body, err = appendRequiredProtoDouble(body, 3, m.Price, "delta-neutral contract price")
	if err != nil {
		return nil, err
	}
	return canonicalProtoFields(body), nil
}

func (m ContractDetailsRequest) encodeProto(sv int) ([]byte, error) {
	reqID, err := encodeProtoInt32(m.ReqID, "contract details request id")
	if err != nil {
		return nil, err
	}
	contract, err := encodeSharedContractProto(m.Contract, nil, true)
	if err != nil {
		return nil, err
	}
	body := appendProtoVarint(nil, 1, reqID)
	return appendProtoMessage(body, 2, contract), nil
}

func decodeContractDataProto(body []byte, sv int) ([]Message, error) {
	return decodeContractDataMessageProto(body, false)
}

func decodeBondContractDataProto(body []byte, sv int) ([]Message, error) {
	return decodeContractDataMessageProto(body, true)
}

func decodeContractDataMessageProto(body []byte, bond bool) ([]Message, error) {
	var reqID int
	var contract decodedContractProto
	var details BondContractDetails
	var hasReqID, hasContract, hasDetails bool
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			if !hasReqID || !hasContract || !hasDetails {
				return nil, fmt.Errorf("contract data missing required fields: req_id=%t contract=%t contract_details=%t", hasReqID, hasContract, hasDetails)
			}
			details.ReqID = reqID
			details.Contract = contract.Contract
			if bond {
				maturity, tradeTime, timeZone := splitBondLastTradeDate(details.Contract.Expiry)
				details.Maturity = maturity
				details.LastTradeTime = tradeTime
				if details.TimeZoneID == "" {
					details.TimeZoneID = timeZone
				}
				return []Message{details}, nil
			}
			details.Contract.Expiry, details.LastTradeTime = splitLastTradeDate(details.Contract.Expiry)
			details.LastTradeDate = contract.LastTradeDate
			return []Message{details.ContractDetails}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("contract data", number, err)
			}
			reqID = decodeProtoInt32(value)
			hasReqID = true
		case 2:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("contract data", number, err)
			}
			contract, err = decodeSharedContractProto(value)
			if err != nil {
				return nil, protoFieldError("contract data contract", number, err)
			}
			hasContract = true
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("contract data", number, err)
			}
			details, err = decodeContractDetailsProto(value)
			if err != nil {
				return nil, protoFieldError("contract details", number, err)
			}
			hasDetails = true
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("contract data", number, err)
			}
		}
	}
}

func decodeContractDetailsProto(body []byte) (BondContractDetails, error) {
	m := BondContractDetails{ContractDetails: ContractDetails{AggGroup: math.MaxInt32}}
	var fund FundDetails
	var hasFund bool
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return BondContractDetails{}, err
		}
		if !ok {
			if hasFund {
				m.Fund = &fund
			}
			return m, nil
		}
		switch number {
		case 5, 6, 18:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return BondContractDetails{}, protoFieldError("contract details", number, err)
			}
			switch number {
			case 5:
				m.PriceMagnifier = decodeProtoInt32(value)
			case 6:
				m.UnderConID = decodeProtoInt32(value)
			case 18:
				m.AggGroup = decodeProtoInt32(value)
			}
		case 16, 48:
			value, err := consumeProtoDouble(&body, typ)
			if err != nil {
				return BondContractDetails{}, protoFieldError("contract details", number, err)
			}
			if number == 16 {
				m.EconomicValueMultiplier = formatProtoDouble(value)
			} else {
				m.Coupon = formatProtoDouble(value)
			}
		case 17:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return BondContractDetails{}, protoFieldError("contract details", number, err)
			}
			id, err := decodeProtoMapEntry(value)
			if err != nil {
				return BondContractDetails{}, protoFieldError("contract security id", number, err)
			}
			m.SecurityIDs = append(m.SecurityIDs, id)
		case 34, 35, 36, 50, 51, 52, 56:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return BondContractDetails{}, protoFieldError("contract details", number, err)
			}
			set := value != 0
			switch number {
			case 34:
				fund.Closed, hasFund = set, true
			case 35:
				fund.ClosedForNewInvestors, hasFund = set, true
			case 36:
				fund.ClosedForNewMoney, hasFund = set, true
			case 50:
				m.Convertible = set
			case 51:
				m.Callable = set
			case 52:
				m.Putable = set
			case 56:
				m.NextOptionPartial = set
			}
		case 58:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return BondContractDetails{}, protoFieldError("contract details", number, err)
			}
			reason, err := decodeIneligibilityReasonProto(value)
			if err != nil {
				return BondContractDetails{}, protoFieldError("contract ineligibility reason", number, err)
			}
			m.IneligibilityReasons = append(m.IneligibilityReasons, reason)
		case 1, 2, 3, 4, 7, 8, 9, 10, 11, 12, 13, 14, 15,
			19, 20, 21, 22, 23, 24, 25, 26,
			27, 28, 29, 30, 31, 32, 33, 37, 38, 39, 40, 41, 42, 43,
			44, 45, 46, 47, 49, 53, 54, 55, 57, 59, 60, 61, 62, 63, 64, 65:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return BondContractDetails{}, protoFieldError("contract details", number, err)
			}
			text := string(value)
			switch number {
			case 1:
				m.MarketName = text
			case 2:
				m.MinTick = text
			case 3:
				m.OrderTypes = text
			case 4:
				m.ValidExchanges = text
			case 7:
				m.LongName = text
			case 8:
				m.ContractMonth = text
			case 9:
				m.Industry = text
			case 10:
				m.Category = text
			case 11:
				m.Subcategory = text
			case 12:
				m.TimeZoneID = text
			case 13:
				m.TradingHours = text
			case 14:
				m.LiquidHours = text
			case 15:
				m.EconomicValueRule = text
			case 19:
				m.UnderSymbol = text
			case 20:
				m.UnderSecType = text
			case 21:
				m.MarketRuleIDs = text
			case 22:
				m.RealExpirationDate = text
			case 23:
				m.StockType = text
			case 24:
				m.MinSize = text
			case 25:
				m.SizeIncrement = text
			case 26:
				m.SuggestedSizeIncrement = text
			case 27:
				fund.Name, hasFund = text, true
			case 28:
				fund.Family, hasFund = text, true
			case 29:
				fund.Type, hasFund = text, true
			case 30:
				fund.FrontLoad, hasFund = text, true
			case 31:
				fund.BackLoad, hasFund = text, true
			case 32:
				fund.BackLoadTimeInterval, hasFund = text, true
			case 33:
				fund.ManagementFee, hasFund = text, true
			case 37:
				fund.NotifyAmount, hasFund = text, true
			case 38:
				fund.MinimumInitialPurchase, hasFund = text, true
			case 39:
				fund.MinimumSubsequentPurchase, hasFund = text, true
			case 40:
				fund.BlueSkyStates, hasFund = text, true
			case 41:
				fund.BlueSkyTerritories, hasFund = text, true
			case 42:
				fund.DistributionPolicy, hasFund = text, true
			case 43:
				fund.AssetType, hasFund = text, true
			case 44:
				m.CUSIP = text
			case 45:
				m.IssueDate = text
			case 46:
				m.Ratings = text
			case 47:
				m.BondType = text
			case 49:
				m.CouponType = text
			case 53:
				m.DescriptionAppend = text
			case 54:
				m.NextOptionDate = text
			case 55:
				m.NextOptionType = text
			case 57:
				m.Notes = text
			case 59:
				m.EventContract1 = text
			case 60:
				m.EventContractDescription1 = text
			case 61:
				m.EventContractDescription2 = text
			case 62:
				m.MinAlgoSize = text
			case 63:
				m.LastPricePrecision = text
			case 64:
				m.LastSizePrecision = text
			case 65:
				m.SettlementMethod = text
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return BondContractDetails{}, protoFieldError("contract details", number, err)
			}
		}
	}
}

func decodeIneligibilityReasonProto(body []byte) (IneligibilityReason, error) {
	m := IneligibilityReason{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return IneligibilityReason{}, err
		}
		if !ok {
			return m, nil
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return IneligibilityReason{}, protoFieldError("ineligibility reason", number, err)
			}
			if number == 1 {
				m.ID = string(value)
			} else {
				m.Description = string(value)
			}
			continue
		}
		if err := skipProtoField(&body, number, typ); err != nil {
			return IneligibilityReason{}, protoFieldError("ineligibility reason", number, err)
		}
	}
}

func decodeContractDataEndProto(body []byte, sv int) ([]Message, error) {
	var reqID int
	var hasReqID bool
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			if !hasReqID {
				return nil, fmt.Errorf("contract data end missing required request id")
			}
			return []Message{ContractDetailsEnd{ReqID: reqID}}, nil
		}
		if number == 1 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("contract data end", number, err)
			}
			reqID = decodeProtoInt32(value)
			hasReqID = true
			continue
		}
		if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("contract data end", number, err)
		}
	}
}
