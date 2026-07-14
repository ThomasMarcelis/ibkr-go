package ibkr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/shopspring/decimal"
)

// protocolIDFromInt narrows integer fields after the protocol boundary has
// applied the signed-int32 invariant. Classic fields are rejected by ReadInt,
// protobuf int32 fields use int32 wire semantics, and locally allocated request
// IDs wrap inside the same range. Keeping the narrowing here makes that proof
// explicit instead of scattering unchecked platform-int conversions through
// the public bridge.
func protocolIDFromInt[T ~int32](value int) T {
	return T(value) // #nosec G115 -- the protocol boundary owns and enforces the signed-int32 invariant
}

func toCodecContract(c Contract) codec.Contract {
	return codec.Contract{
		ConID:           int(c.ConID),
		Symbol:          c.Symbol,
		SecType:         string(c.SecType),
		Expiry:          c.Expiry,
		Strike:          decimalPointerOrEmpty(c.Strike),
		Right:           string(c.Right),
		Multiplier:      c.Multiplier,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		LocalSymbol:     c.LocalSymbol,
		TradingClass:    c.TradingClass,
		PrimaryExchange: c.PrimaryExchange,
		IncludeExpired:  c.IncludeExpired,
		SecurityIDType:  string(c.SecurityID.Type),
		SecurityID:      c.SecurityID.Value,
		IssuerID:        c.IssuerID,
		ComboLegs:       comboLegsToCodec(c.ComboLegs),
		DeltaNeutral:    deltaNeutralToCodec(c.DeltaNeutral),
	}
}

func fromCodecContract(c codec.Contract) (Contract, error) {
	strike, err := parseOptionalDecimalPointer(c.Strike, "contract strike")
	if err != nil {
		return Contract{}, err
	}
	comboLegs, err := comboLegsFromCodec(c.ComboLegs)
	if err != nil {
		return Contract{}, err
	}
	deltaNeutral, err := deltaNeutralFromCodec(c.DeltaNeutral)
	if err != nil {
		return Contract{}, err
	}
	right := Right(c.Right)
	if c.Right == "?" {
		right = ""
	}
	return Contract{
		ConID:           protocolIDFromInt[ContractID](c.ConID),
		Symbol:          c.Symbol,
		SecType:         SecType(c.SecType),
		Expiry:          c.Expiry,
		Strike:          strike,
		Right:           right,
		Multiplier:      c.Multiplier,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		LocalSymbol:     c.LocalSymbol,
		TradingClass:    c.TradingClass,
		PrimaryExchange: c.PrimaryExchange,
		IncludeExpired:  c.IncludeExpired,
		SecurityID: SecurityID{
			Type:  SecurityIDType(c.SecurityIDType),
			Value: c.SecurityID,
		},
		IssuerID:     c.IssuerID,
		ComboLegs:    comboLegs,
		DeltaNeutral: deltaNeutral,
	}, nil
}

func decimalPointerOrEmpty(value *decimal.Decimal) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func deltaNeutralToCodec(value *DeltaNeutralContract) *codec.DeltaNeutralContract {
	if value == nil {
		return nil
	}
	return &codec.DeltaNeutralContract{
		ConID: int(value.ConID),
		Delta: value.Delta.String(),
		Price: value.Price.String(),
	}
}

func deltaNeutralFromCodec(value *codec.DeltaNeutralContract) (*DeltaNeutralContract, error) {
	if value == nil {
		return nil, nil
	}
	delta, err := parseRequiredDecimal(value.Delta, "delta-neutral contract delta")
	if err != nil {
		return nil, err
	}
	price, err := parseRequiredDecimal(value.Price, "delta-neutral contract price")
	if err != nil {
		return nil, err
	}
	return &DeltaNeutralContract{ConID: protocolIDFromInt[ContractID](value.ConID), Delta: delta, Price: price}, nil
}

func validateContract(contract Contract) error {
	if contract.ConID < 0 {
		return &ValidationError{Field: "Contract.ConID", Value: strconv.FormatInt(int64(contract.ConID), 10), Message: "must be >= 0"}
	}
	securityIDType := string(contract.SecurityID.Type)
	if securityIDType != strings.TrimSpace(securityIDType) {
		return &ValidationError{Field: "Contract.SecurityID.Type", Value: securityIDType, Message: "must not contain surrounding whitespace"}
	}
	if contract.SecurityID.Value != strings.TrimSpace(contract.SecurityID.Value) {
		return &ValidationError{Field: "Contract.SecurityID.Value", Value: contract.SecurityID.Value, Message: "must not contain surrounding whitespace"}
	}
	typeSet := securityIDType != ""
	valueSet := contract.SecurityID.Value != ""
	if !typeSet && valueSet {
		return &ValidationError{Field: "Contract.SecurityID.Type", Message: "is required when Contract.SecurityID.Value is set"}
	}
	if typeSet && !valueSet {
		return &ValidationError{Field: "Contract.SecurityID.Value", Message: "is required when Contract.SecurityID.Type is set"}
	}
	if len(contract.ComboLegs) > 0 && contract.SecType != SecTypeCombo {
		return &ValidationError{Field: "Contract.ComboLegs", Value: strconv.Itoa(len(contract.ComboLegs)), Message: "requires a BAG contract"}
	}
	if contract.SecType == SecTypeCombo && len(contract.ComboLegs) == 1 {
		return &ValidationError{Field: "Contract.ComboLegs", Value: "1", Message: "must be empty for BAG lookup or contain at least two legs"}
	}
	for i, leg := range contract.ComboLegs {
		prefix := fmt.Sprintf("Contract.ComboLegs[%d]", i)
		if leg.ConID <= 0 {
			return &ValidationError{Field: prefix + ".ConID", Value: strconv.FormatInt(int64(leg.ConID), 10), Message: "must be > 0"}
		}
		if leg.Ratio <= 0 {
			return &ValidationError{Field: prefix + ".Ratio", Value: strconv.Itoa(leg.Ratio), Message: "must be > 0"}
		}
		switch leg.Action {
		case ActionBuy, ActionSell, ActionSellShort, ActionSellLong:
		default:
			return &ValidationError{Field: prefix + ".Action", Value: string(leg.Action), Message: "must be BUY, SELL, SSHORT, or SLONG"}
		}
		if strings.TrimSpace(leg.Exchange) == "" {
			return &ValidationError{Field: prefix + ".Exchange", Message: "is required"}
		}
		if leg.OpenClose < ComboLegSame || leg.OpenClose > ComboLegUnknown {
			return &ValidationError{Field: prefix + ".OpenClose", Value: strconv.Itoa(int(leg.OpenClose)), Message: "must be Same, Open, Close, or Unknown (0..3)"}
		}
		if leg.ShortSaleSlot < 0 || leg.ShortSaleSlot > 2 {
			return &ValidationError{Field: prefix + ".ShortSaleSlot", Value: strconv.Itoa(leg.ShortSaleSlot), Message: "must be 0, 1, or 2"}
		}
		if leg.ShortSaleSlot == 2 && strings.TrimSpace(leg.DesignatedLocation) == "" {
			return &ValidationError{Field: prefix + ".DesignatedLocation", Message: "is required for short-sale slot 2"}
		}
		if leg.ExemptCode != nil && *leg.ExemptCode < 0 {
			return &ValidationError{Field: prefix + ".ExemptCode", Value: strconv.Itoa(*leg.ExemptCode), Message: "must be >= 0; use nil for IBKR's unset sentinel"}
		}
	}
	if contract.DeltaNeutral != nil {
		if contract.SecType != SecTypeCombo {
			return &ValidationError{Field: "Contract.DeltaNeutral", Message: "requires a BAG contract"}
		}
		if contract.DeltaNeutral.ConID <= 0 {
			return &ValidationError{Field: "Contract.DeltaNeutral.ConID", Value: strconv.FormatInt(int64(contract.DeltaNeutral.ConID), 10), Message: "must be > 0"}
		}
	}
	return nil
}

type contractFields uint8

const (
	contractFieldIncludeExpired contractFields = 1 << iota
	contractFieldSecurityID
	contractFieldComboLegs
	contractFieldDeltaNeutral
	contractFieldIssuerID
	contractFieldPrimaryExchange
	contractFieldComboLegDetails

	contractFieldsAll = contractFieldIncludeExpired | contractFieldSecurityID | contractFieldComboLegs | contractFieldDeltaNeutral |
		contractFieldIssuerID | contractFieldPrimaryExchange | contractFieldComboLegDetails
)

func validateContractFieldSupport(contract Contract, operation string, serverVersion int, supported contractFields) error {
	unsupported := func(field string, value any) error {
		return &ValidationError{
			Field:   field,
			Value:   fmt.Sprint(value),
			Message: fmt.Sprintf("is not represented by %s at negotiated server_version %d", operation, serverVersion),
		}
	}
	if contract.IncludeExpired && supported&contractFieldIncludeExpired == 0 {
		return unsupported("Contract.IncludeExpired", true)
	}
	if (contract.SecurityID.Type != "" || contract.SecurityID.Value != "") && supported&contractFieldSecurityID == 0 {
		return unsupported("Contract.SecurityID", fmt.Sprintf("%s:%s", contract.SecurityID.Type, contract.SecurityID.Value))
	}
	if len(contract.ComboLegs) > 0 && supported&contractFieldComboLegs == 0 {
		return unsupported("Contract.ComboLegs", len(contract.ComboLegs))
	}
	if supported&contractFieldComboLegDetails == 0 {
		for i, leg := range contract.ComboLegs {
			prefix := fmt.Sprintf("Contract.ComboLegs[%d]", i)
			switch {
			case leg.OpenClose != ComboLegSame:
				return unsupported(prefix+".OpenClose", leg.OpenClose)
			case leg.ShortSaleSlot != 0:
				return unsupported(prefix+".ShortSaleSlot", leg.ShortSaleSlot)
			case leg.DesignatedLocation != "":
				return unsupported(prefix+".DesignatedLocation", leg.DesignatedLocation)
			case leg.ExemptCode != nil:
				return unsupported(prefix+".ExemptCode", *leg.ExemptCode)
			}
		}
	}
	if contract.DeltaNeutral != nil && supported&contractFieldDeltaNeutral == 0 {
		return unsupported("Contract.DeltaNeutral", contract.DeltaNeutral.ConID)
	}
	if contract.IssuerID != "" && supported&contractFieldIssuerID == 0 {
		return unsupported("Contract.IssuerID", contract.IssuerID)
	}
	if contract.PrimaryExchange != "" && supported&contractFieldPrimaryExchange == 0 {
		return unsupported("Contract.PrimaryExchange", contract.PrimaryExchange)
	}
	return nil
}

func quoteContractFields(serverVersion int) contractFields {
	if serverVersion >= protocol.MinServerVersionProtobufMarketData {
		return contractFieldsAll
	}
	return contractFieldPrimaryExchange | contractFieldComboLegs | contractFieldDeltaNeutral
}

func depthContractFields(serverVersion int) contractFields {
	if serverVersion >= protocol.MinServerVersionProtobufMarketData {
		return contractFieldsAll
	}
	return contractFieldPrimaryExchange
}

func contractDetailsContractFields(serverVersion int) contractFields {
	if serverVersion >= protocol.MinServerVersionProtobufContractData {
		return contractFieldsAll
	}
	return contractFieldPrimaryExchange | contractFieldIncludeExpired | contractFieldSecurityID | contractFieldIssuerID
}

func placeOrderContractFields(serverVersion int) contractFields {
	if serverVersion >= protocol.MinServerVersionProtobufPlaceOrder {
		return contractFieldsAll
	}
	return contractFieldPrimaryExchange | contractFieldSecurityID | contractFieldComboLegs | contractFieldComboLegDetails | contractFieldDeltaNeutral
}
