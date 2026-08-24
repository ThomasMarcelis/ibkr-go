package ibkr

import (
	"errors"
	"testing"
)

func TestContractFieldSupportMatchesRequestLayouts(t *testing.T) {
	t.Parallel()

	fields := []struct {
		name     string
		contract Contract
		bit      contractFields
	}{
		{name: "include expired", contract: Contract{IncludeExpired: true}, bit: contractFieldIncludeExpired},
		{name: "security id", contract: Contract{SecurityID: SecurityID{Type: SecurityIDISIN, Value: "US0378331005"}}, bit: contractFieldSecurityID},
		{name: "combo legs", contract: validComboContract(), bit: contractFieldComboLegs},
		{name: "delta neutral", contract: Contract{SecType: SecTypeCombo, DeltaNeutral: &DeltaNeutralContract{ConID: 265598}}, bit: contractFieldDeltaNeutral},
		{name: "issuer id", contract: Contract{IssuerID: "e1432232"}, bit: contractFieldIssuerID},
		{name: "primary exchange", contract: Contract{PrimaryExchange: "NASDAQ"}, bit: contractFieldPrimaryExchange},
	}
	tests := []struct {
		name      string
		supported contractFields
	}{
		{name: "quote depth contract details and order", supported: contractFieldsAll},
		{name: "historical bars schedule stream", supported: contractFieldPrimaryExchange | contractFieldIncludeExpired | contractFieldComboLegs},
		{name: "head histogram historical ticks", supported: contractFieldPrimaryExchange | contractFieldIncludeExpired},
		{name: "realtime tick calculation", supported: contractFieldPrimaryExchange},
		{name: "exercise", supported: 0},
	}
	for _, tc := range tests {
		for _, field := range fields {
			t.Run(tc.name+"/"+field.name, func(t *testing.T) {
				t.Parallel()
				err := validateContractFieldSupport(field.contract, tc.name, 208, tc.supported)
				if wantErr := tc.supported&field.bit == 0; (err != nil) != wantErr {
					t.Fatalf("validateContractFieldSupport() error = %v, wantErr %t", err, wantErr)
				}
			})
		}
	}
}

func TestReducedComboLegLayoutsRejectUnrepresentedFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*ComboLeg)
		field  string
	}{
		{name: "open close", mutate: func(leg *ComboLeg) { leg.OpenClose = ComboLegOpen }, field: "Contract.ComboLegs[0].OpenClose"},
		{name: "short-sale slot", mutate: func(leg *ComboLeg) { leg.ShortSaleSlot = 1 }, field: "Contract.ComboLegs[0].ShortSaleSlot"},
		{name: "designated location", mutate: func(leg *ComboLeg) { leg.DesignatedLocation = "SMART" }, field: "Contract.ComboLegs[0].DesignatedLocation"},
		{name: "exempt code", mutate: func(leg *ComboLeg) { leg.ExemptCode = new(0) }, field: "Contract.ComboLegs[0].ExemptCode"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			contract := validComboContract()
			tc.mutate(&contract.ComboLegs[0])
			err := validateContractFieldSupport(contract, "reduced combo layout", 208, contractFieldPrimaryExchange|contractFieldComboLegs)
			validation, ok := errors.AsType[*ValidationError](err)
			if !ok || validation.Field != tc.field {
				t.Fatalf("validateContractFieldSupport() = %v, want field %q", err, tc.field)
			}
			if err := validateContractFieldSupport(contract, "shared contract", 208, contractFieldsAll); err != nil {
				t.Fatalf("full shared Contract rejected %s: %v", tc.name, err)
			}
		})
	}
}

func TestValidateContractStructuralBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contract Contract
		field    string
	}{
		{name: "negative conid", contract: Contract{ConID: -1}, field: "Contract.ConID"},
		{name: "one BAG leg", contract: Contract{SecType: SecTypeCombo, ComboLegs: validComboContract().ComboLegs[:1]}, field: "Contract.ComboLegs"},
		{name: "delta neutral on stock", contract: Contract{SecType: SecTypeStock, DeltaNeutral: &DeltaNeutralContract{ConID: 1}}, field: "Contract.DeltaNeutral"},
		{name: "open close outside official enum", contract: comboWithLeg(func(leg *ComboLeg) { leg.OpenClose = 4 }), field: "Contract.ComboLegs[0].OpenClose"},
		{name: "explicit negative exempt", contract: comboWithLeg(func(leg *ComboLeg) { leg.ExemptCode = new(-1) }), field: "Contract.ComboLegs[0].ExemptCode"},
		{name: "security id type whitespace", contract: Contract{SecurityID: SecurityID{Type: " ISIN", Value: "US0378331005"}}, field: "Contract.SecurityID.Type"},
		{name: "security id value whitespace", contract: Contract{SecurityID: SecurityID{Type: SecurityIDISIN, Value: "US0378331005 "}}, field: "Contract.SecurityID.Value"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			validation, ok := errors.AsType[*ValidationError](validateContract(tc.contract))
			if !ok || validation.Field != tc.field {
				t.Fatalf("validateContract() = %v, want field %q", validation, tc.field)
			}
		})
	}
	if err := validateContract(Contract{SecType: SecTypeCombo}); err != nil {
		t.Fatalf("empty BAG lookup rejected: %v", err)
	}
}

func validComboContract() Contract {
	// Two adjacent live AAPL option contracts from the current sv225 details
	// replay, events SHA-256
	// 8a4e11ce19b17af162a660ee615aa3499e183d79164a19a7b325a6802d6afe47.
	return Contract{SecType: SecTypeCombo, ComboLegs: []ComboLeg{
		{ConID: 909446159, Ratio: 1, Action: ActionBuy, Exchange: "SMART", OpenClose: ComboLegSame},
		{ConID: 909446204, Ratio: 1, Action: ActionSell, Exchange: "SMART", OpenClose: ComboLegSame},
	}}
}

func comboWithLeg(mutate func(*ComboLeg)) Contract {
	contract := validComboContract()
	mutate(&contract.ComboLegs[0])
	return contract
}
