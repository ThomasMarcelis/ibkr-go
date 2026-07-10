package codec

import (
	"bytes"
	"encoding/base64"
	"math"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

func TestContractDetailsMigrationStartsAtServer205(t *testing.T) {
	t.Parallel()

	msg := ContractDetailsRequest{ReqID: 1, Contract: Contract{ConID: 265598, Exchange: "SMART"}}
	for _, tc := range []struct {
		sv       int
		encoding protocol.BodyEncoding
		wireID   int
	}{
		{204, protocol.ClassicBody, OutReqContractData},
		{205, protocol.ProtobufBody, OutReqContractData + protocol.ProtobufMessageID},
	} {
		payload, err := Encode(tc.sv, msg)
		if err != nil {
			t.Fatalf("Encode(%d) error = %v", tc.sv, err)
		}
		envelope, err := protocol.DecodeEnvelope(tc.sv, payload)
		if err != nil {
			t.Fatalf("DecodeEnvelope(%d) error = %v", tc.sv, err)
		}
		if envelope.MsgID != OutReqContractData || envelope.WireID != tc.wireID || envelope.Encoding != tc.encoding {
			t.Fatalf("server %d envelope = %+v", tc.sv, envelope)
		}
	}
}

func TestEncodeServer205ContractDetailsVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  ContractDetailsRequest
		want string
	}{
		{
			name: "stock by conid",
			msg:  ContractDetailsRequest{ReqID: 20501, Contract: Contract{ConID: 265598, Exchange: "SMART"}},
			want: "000000d10895a001120b08fe9a104205534d415254",
		},
		{
			name: "bond by conid",
			msg:  ContractDetailsRequest{ReqID: 20502, Contract: Contract{ConID: 127128131, Exchange: "SMART"}},
			want: "000000d10896a001120c08c3a4cf3c4205534d415254",
		},
		{
			name: "fund by conid",
			msg:  ContractDetailsRequest{ReqID: 20503, Contract: Contract{ConID: 57041934, Exchange: "FUNDSERV"}},
			want: "000000d10897a001120f088ec8991b420846554e4453455256",
		},
		{
			name: "option by conid",
			msg:  ContractDetailsRequest{ReqID: 20504, Contract: Contract{ConID: 728937835, Exchange: "SMART"}},
			want: "000000d10898a001120d08ebeacadb024205534d415254",
		},
		{
			name: "bond issuer",
			msg:  ContractDetailsRequest{ReqID: 20502, Contract: Contract{IssuerID: "e1432232"}},
			want: "000000d10896a001120d08008201086531343332323332",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(205, tc.msg)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			want := decodeHex(t, tc.want)
			if !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x, want exact API 10.48.01 live vector %x", got, want)
			}
		})
	}
}

func TestDecodeServer205ContractDetailsTypeMatrix(t *testing.T) {
	t.Parallel()

	stockMessage, err := Decode(205, liveContractPayload(t, "AAAA0giVoAESNwj+mhASBEFBUEwaA1NUSykAAAAAAAAAAEIFU01BUlRKBk5BU0RBUVIDVVNEWgRBQVBMYgNOTVMapAkKA05NUxIEMC4wMRrIA0FDVElWRVRJTSxBRCxBRERPTlQsQURKVVNULEFMRVJULEFMR08sQUxMT0MsQU9OLEFWR0NPU1QsQkFTS0VULEJFTkNIUFgsQ0FTSFFUWSxDT05ELENPTkRPUkRFUixEQVJLT05MWSxEQVJLUE9MTCxEQVksREVBQ1QsREVBQ1RESVMsREVBQ1RFT0QsRElTLERVUixHQVQsR1RDLEdURCxHVFQsSElELElCS1JBVFMsSUNFLElNQixJT0MsTElULExNVCxMT0MsTUlEUFgsTUlULE1LVCxNT0MsTVRMLE5HQ09NQixOT0RBUkssTk9OQUxHTyxPQ0EsT1BHLE9QR1JFUk9VVCxQRUdCRU5DSCxQRUdNSUQsUE9TVEFUUyxQT1NUT05MWSxQUkVPUEdSVEgsUFJJQ0VDSEssUkVMLFJFTDJNSUQsUkVMUENUT0ZTLFJQSSxSVEgsU0NBTEUsU0NBTEVPREQsU0NBTEVSU1QsU0laRUNISyxTTkFQTUlELFNOQVBNS1QsU05BUFJFTCxTVFAsU1RQTE1ULFNXRUVQLFRSQUlMLFRSQUlMTElULFRSQUlMTE1ULFRSQUlMTUlULFdIQVRJRiKTAVNNQVJULEFNRVgsTllTRSxDQk9FLFBITFgsSVNFLENIWCxBUkNBLE5BU0RBUSxEUkNURURHRSxCRVgsQkFUUyxFREdFQSxCWVgsSUVYLEVER1gsRk9YUklWRVIsUEVBUkwsTllTRU5BVCxMVFNFLE1FTVgsSUJFT1MsT1ZFUk5JR0hULFRQTFVTMCxQU1gsVDI0WCgBOglBUFBMRSBJTkNKClRlY2hub2xvZ3lSCUNvbXB1dGVyc1oJQ29tcHV0ZXJzYgpVUy9FYXN0ZXJuao8BMjAyNjA3MDk6MDQwMC0yMDI2MDcwOToyMDAwOzIwMjYwNzEwOjA0MDAtMjAyNjA3MTA6MjAwMDsyMDI2MDcxMTpDTE9TRUQ7MjAyNjA3MTI6Q0xPU0VEOzIwMjYwNzEzOjA0MDAtMjAyNjA3MTM6MjAwMDsyMDI2MDcxNDowNDAwLTIwMjYwNzE0OjIwMDByjwEyMDI2MDcwOTowOTMwLTIwMjYwNzA5OjE2MDA7MjAyNjA3MTA6MDkzMC0yMDI2MDcxMDoxNjAwOzIwMjYwNzExOkNMT1NFRDsyMDI2MDcxMjpDTE9TRUQ7MjAyNjA3MTM6MDkzMC0yMDI2MDcxMzoxNjAwOzIwMjYwNzE0OjA5MzAtMjAyNjA3MTQ6MTYwMIoBFAoESVNJThIMVVMwMzc4MzMxMDA1kAEBqgGBATQ1NjMsNDU2Myw0NTYzLDQ1NjMsNDU2Myw0NTYzLDQ1NjMsNDU2Myw0NTYzLDQ1NjMsNDU2Myw0NTYzLDQ1NjMsNDU2Myw0NTYzLDQ1NjMsNDU2Myw0NTYzLDQ1NjMsNDU2Myw0NTYzLDQ1NjMsNDU2Myw0NTYzLDQ1NjMsNDU2M7oBBkNPTU1PTsIBBjAuMDAwMcoBBjAuMDAwMdIBAjQw8gMBMPoDCDAuMDAwMDAxggQIMC4wMDAwMDE="))
	if err != nil {
		t.Fatalf("Decode(stock) error = %v", err)
	}
	stock := stockMessage.(ContractDetails)
	if stock.ReqID != 20501 || stock.Contract.ConID != 265598 || stock.Contract.Symbol != "AAPL" || stock.Contract.SecType != "STK" || stock.Contract.Strike != "0" {
		t.Fatalf("stock identity = %+v", stock)
	}
	if stock.MinSize != "0.0001" || stock.SizeIncrement != "0.0001" || stock.SuggestedSizeIncrement != "40" || stock.MinAlgoSize != "0" || stock.LastPricePrecision != "0.000001" || stock.LastSizePrecision != "0.000001" {
		t.Fatalf("stock size and precision fields = %+v", stock)
	}
	if stock.AggGroup != 1 || len(stock.SecurityIDs) != 1 || stock.SecurityIDs[0] != (TagValue{Tag: "ISIN", Value: "US0378331005"}) {
		t.Fatalf("stock identifiers = %+v / %d", stock.SecurityIDs, stock.AggGroup)
	}

	bondMessage, err := Decode(205, liveContractPayload(t, "AAAA2giWoAESMQjDpM88GgRCT05EKQAAAAAAAAAAQgVTTUFSVFoOSUJDSUQxMjcxMjgxMzFiBEFBUEwaxAUSBjAuMDAwMRrSAUFDVElWRVRJTSxBRCxBREpVU1QsQUxFUlQsQUxMT0MsQU9OLEFWR0NPU1QsQkFTS0VULEJFTkNIUFgsQ09ORE9SREVSLERBWSxERUFDVCxERUFDVERJUyxERUFDVEVPRCxFVlJVTEUsR0FULEdUQyxHVEQsR1RULEhJRCxJQktSQVRTLElPQyxMTVQsTUtULE5PTkFMR08sTk9ORklSTVFULE9DQSxPRERQT1NDTFMsUEFPTixSRlEsUlRILFNDQUxFLFNDQUxFUlNULFdIQVRJRiIFU01BUlQoAUIGMjA0MzA1SgpUZWNobm9sb2d5UglDb21wdXRlcnNaCUNvbXB1dGVyc2IKVVMvRWFzdGVybmqbATIwMjYwNzA4OjIwMDAtMjAyNjA3MDk6MTcwMDsyMDI2MDcwOToyMDAwLTIwMjYwNzEwOjE3MDA7MjAyNjA3MTE6Q0xPU0VEOzIwMjYwNzEyOjIwMDAtMjAyNjA3MTM6MTcwMDsyMDI2MDcxMzoyMDAwLTIwMjYwNzE0OjE3MDA7MjAyNjA3MTQ6MjAwMC0yMDI2MDcxNToxNzAwcpsBMjAyNjA3MDg6MjAwMC0yMDI2MDcwOToxNzAwOzIwMjYwNzA5OjIwMDAtMjAyNjA3MTA6MTcwMDsyMDI2MDcxMTpDTE9TRUQ7MjAyNjA3MTI6MjAwMC0yMDI2MDcxMzoxNzAwOzIwMjYwNzEzOjIwMDAtMjAyNjA3MTQ6MTcwMDsyMDI2MDcxNDoyMDAwLTIwMjYwNzE1OjE3MDCKARIKBUNVU0lQEgkwMzc4MzNBTDSKARQKBElTSU4SDFVTMDM3ODMzQUw0MpABB6oBBDEzODbCAQEyygEBMdIBATHiAg5JQkNJRDEyNzEyODEzMaoDEkFBUEwgMy44NSAwNS8wNC80M/IDATA="))
	if err != nil {
		t.Fatalf("Decode(bond) error = %v", err)
	}
	bond := bondMessage.(BondContractDetails)
	if bond.ReqID != 20502 || bond.Contract.ConID != 127128131 || bond.CUSIP != "IBCID127128131" || bond.DescriptionAppend != "AAPL 3.85 05/04/43" || bond.MinAlgoSize != "0" {
		t.Fatalf("bond = %+v", bond)
	}
	if len(bond.SecurityIDs) != 2 || bond.SecurityIDs[0].Tag != "CUSIP" || bond.SecurityIDs[1].Tag != "ISIN" {
		t.Fatalf("bond SecurityIDs = %+v", bond.SecurityIDs)
	}

	fundMessage, err := Decode(205, liveContractPayload(t, "AAAA0giXoAESQAiOyJkbEgVBQUFJWBoERlVORCkAAAAAAAAAAEIIRlVORFNFUlZSA1VTRFoJMDI1MDg1ODUzYgkwMjUwODU4NTMagQUKBUFBQUlYEgQwLjAxGkRBRCxBTEVSVCxBTExPQyxCQVNLRVQsREFZLERFQUNULERFQUNURElTLEZVTkRTV0FQLE1LVCxOT05BTEdPLFdIQVRJRiIIRlVORFNFUlYoATo5QU1DRU4gU1RSQVRFR0lDIEFMTE9DOiBBR0dSRVNTSVZFLUlOU1QgKEFtZXJpY2FuIENlbnR1cnkpYgpVUy9FYXN0ZXJuao8BMjAyNjA3MDk6MTU1OS0yMDI2MDcwOToyMjAwOzIwMjYwNzEwOjE1NTktMjAyNjA3MTA6MjIwMDsyMDI2MDcxMTpDTE9TRUQ7MjAyNjA3MTI6Q0xPU0VEOzIwMjYwNzEzOjE1NTktMjAyNjA3MTM6MjIwMDsyMDI2MDcxNDoxNTU5LTIwMjYwNzE0OjIyMDByjwEyMDI2MDcwOToxNTU5LTIwMjYwNzA5OjIyMDA7MjAyNjA3MTA6MTU1OS0yMDI2MDcxMDoyMjAwOzIwMjYwNzExOkNMT1NFRDsyMDI2MDcxMjpDTE9TRUQ7MjAyNjA3MTM6MTU1OS0yMDI2MDcxMzoyMjAwOzIwMjYwNzE0OjE1NTktMjAyNjA3MTQ6MjIwMIoBFAoESVNJThIMVVMwMjUwODU4NTM5qgEEMjk2M8IBBTAuMDAxygEFMC4wMDHSAQEx2gEmQU1DRU4gU1RSQVRFR0lDIEFMTE9DOiBBR0dSRVNTSVZFLUlOU1TiARBBbWVyaWNhbiBDZW50dXJ58gEBMPoBATCCAgEwigIEMC45NaoCBjYwMDAwMLICBzUwMDAwMDC6AgQwLjAxwgIDQWxsygIPQVJFLEdVTSxQUkksVklS8gMBMA=="))
	if err != nil {
		t.Fatalf("Decode(fund) error = %v", err)
	}
	fund := fundMessage.(ContractDetails)
	if fund.Contract.ConID != 57041934 || fund.Fund == nil || fund.Fund.Family != "American Century" || fund.Fund.ManagementFee != "0.95" || fund.MinAlgoSize != "0" || fund.AggGroup != math.MaxInt32 {
		t.Fatalf("fund = %+v", fund)
	}

	optionMessage, err := Decode(205, liveContractPayload(t, "AAAA0giYoAESYgjr6srbAhIDUVFRGgNPUFQiCDIwMjcwMTE1KQAAAAAAQH9AMgFQOQAAAAAAAFlAQgVTTUFSVFIDVVNEWhVRUVEgICAyNzAxMTVQMDA1MDAwMDBiA1FRUaoBCDIwMjcwMTE1GsoHCgNRUVESBDAuMDEa/AJBQ1RJVkVUSU0sQUQsQURKVVNULEFMRVJULEFMR08sQUxMT0MsQU9OLEFWR0NPU1QsQkFTS0VULENPTkQsQ09ORE9SREVSLERBWSxERUFDVCxERUFDVERJUyxERUFDVEVPRCxESVMsRk9LLEdBVCxHVEMsR1RELEdUVCxISUQsSUNFLElPQyxMSVQsTE1ULE1JVCxNS1QsTVRMLE5HQ09NQixOT05BTEdPLE9DQSxPUEVOQ0xPU0UsUEFPTixQRUdNSURWT0wsUEVHTUtUVk9MLFBFR1BSTVZPTCxQRUdTUkZWT0wsUE9TVE9OTFksUFJJQ0VDSEssUkVMLFJFTFBDVE9GUyxSRUxTVEssU0NBTEUsU0NBTEVSU1QsU0laRUNISyxTTUFSVFNURyxTTkFQTUlELFNOQVBNS1QsU05BUFJFTCxTVFAsU1RQTE1ULFRSQUlMLFRSQUlMTElULFRSQUlMTE1ULFRSQUlMTUlULFZPTEFULFdIQVRJRiJ6U01BUlQsQU1FWCxDQk9FLFBITFgsUFNFLElTRSxCT1gsQkFUUyxOQVNEQVFPTSxDQk9FMixOQVNEQVFCWCxNSUFYLEdFTUlOSSxFREdYLE1FUkNVUlksUEVBUkwsRU1FUkFMRCxNRU1YLElCVVNPUFQsU0FQUEhJUkUoATDzkdmYAToaSU5WRVNDTyBRUVEgVFJVU1QgU0VSSUVTIDFCBjIwMjcwMWIKVVMvRWFzdGVybmqPATIwMjYwNzA5OjA5MzAtMjAyNjA3MDk6MTYxNTsyMDI2MDcxMDowOTMwLTIwMjYwNzEwOjE2MTU7MjAyNjA3MTE6Q0xPU0VEOzIwMjYwNzEyOkNMT1NFRDsyMDI2MDcxMzowOTMwLTIwMjYwNzEzOjE2MTU7MjAyNjA3MTQ6MDkzMC0yMDI2MDcxNDoxNjE1co8BMjAyNjA3MDk6MDkzMC0yMDI2MDcwOToxNjE1OzIwMjYwNzEwOjA5MzAtMjAyNjA3MTA6MTYxNTsyMDI2MDcxMTpDTE9TRUQ7MjAyNjA3MTI6Q0xPU0VEOzIwMjYwNzEzOjA5MzAtMjAyNjA3MTM6MTYxNTsyMDI2MDcxNDowOTMwLTIwMjYwNzE0OjE2MTWQAQKaAQNRUVGiAQNTVEuqATszMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMiwzMrIBCDIwMjcwMTE1wgEBMcoBATHSAQEx8gMBMA=="))
	if err != nil {
		t.Fatalf("Decode(option) error = %v", err)
	}
	option := optionMessage.(ContractDetails)
	if option.Contract.ConID != 728937835 || option.Contract.Expiry != "20270115" || option.LastTradeDate != "20270115" || option.Contract.Strike != "500" || option.Contract.Multiplier != "100" || option.MinAlgoSize != "0" {
		t.Fatalf("option = %+v", option)
	}
}

func TestDecodeServer205LiveIneligibilityReasonsAndEnds(t *testing.T) {
	t.Parallel()

	message, err := Decode(205, liveContractPayload(t, "AAAA2giWoAESMQi7o+JwGgRCT05EKQAAAAAAAAAAQgVTTUFSVFoOSUJDSUQyMzY0OTExOTViBEFBUEwa/gYSBTAuMDAxGsgBQUNUSVZFVElNLEFELEFESlVTVCxBTEVSVCxBTExPQyxBT04sQVZHQ09TVCxCQVNLRVQsQkVOQ0hQWCxDT05ET1JERVIsREFZLERFQUNULERFQUNURElTLERFQUNURU9ELEVWUlVMRSxHQVQsR1RDLEdURCxHVFQsSElELElCS1JBVFMsSU9DLExNVCxNS1QsTk9OQUxHTyxPQ0EsT0REUE9TQ0xTLFBBT04sUkZRLFJUSCxTQ0FMRSxTQ0FMRVJTVCxXSEFUSUYiBVNNQVJUKAFCBjIwNDYwNkoKVGVjaG5vbG9neVIJQ29tcHV0ZXJzWglDb21wdXRlcnNiClVTL0Vhc3Rlcm5qmwEyMDI2MDcwODoyMDAwLTIwMjYwNzA5OjE3MDA7MjAyNjA3MDk6MjAwMC0yMDI2MDcxMDoxNzAwOzIwMjYwNzExOkNMT1NFRDsyMDI2MDcxMjoyMDAwLTIwMjYwNzEzOjE3MDA7MjAyNjA3MTM6MjAwMC0yMDI2MDcxNDoxNzAwOzIwMjYwNzE0OjIwMDAtMjAyNjA3MTU6MTcwMHKbATIwMjYwNzA4OjIwMDAtMjAyNjA3MDk6MTcwMDsyMDI2MDcwOToyMDAwLTIwMjYwNzEwOjE3MDA7MjAyNjA3MTE6Q0xPU0VEOzIwMjYwNzEyOjIwMDAtMjAyNjA3MTM6MTcwMDsyMDI2MDcxMzoyMDAwLTIwMjYwNzE0OjE3MDA7MjAyNjA3MTQ6MjAwMC0yMDI2MDcxNToxNzAwegdmYWN0b3I6igEUCgRJU0lOEgxYUzE0MzEyNjM1ODiQAQeqAQMzNDLCAQMxMDDKAQEx0gEBMeICDklCQ0lEMjM2NDkxMTk1qgMSQUFQTCA0LjE1IDA2LzIyLzQ20gNsCgRpMTU1EmRObyBPcGVuaW5nIFRyYWRlczogVGhlIG91dHN0YW5kaW5nIGFtb3VudCBvZiB0aGUgYm9uZCBpcyBsZXNzIHRoYW4gMjUlIG9mIHRoZSBvcmlnaW5hbCBpc3N1ZSBhbW91bnQu0gNHCgRpMTU2Ej9ObyBPcGVuaW5nIFRyYWRlczogQSBmdWxsIGNhbGwgZm9yIHRoZSBib25kIGhhcyBiZWVuIGFubm91bmNlZC7SAxQKA2kzMBINUmVkZWVtZWQgYm9uZPIDATA="))
	if err != nil {
		t.Fatalf("Decode(ineligibility response) error = %v", err)
	}
	bond := message.(BondContractDetails)
	if len(bond.IneligibilityReasons) != 3 || bond.IneligibilityReasons[0].ID != "i155" || bond.IneligibilityReasons[1].ID != "i156" || bond.IneligibilityReasons[2].ID != "i30" {
		t.Fatalf("IneligibilityReasons = %+v", bond.IneligibilityReasons)
	}

	for _, tc := range []struct {
		hex   string
		reqID int
	}{
		{"000000fc0895a001", 20501},
		{"000000fc0896a001", 20502},
		{"000000fc0897a001", 20503},
		{"000000fc0898a001", 20504},
	} {
		end, err := Decode(205, decodeHex(t, tc.hex))
		if err != nil {
			t.Fatalf("Decode(end %d) error = %v", tc.reqID, err)
		}
		if got := end.(ContractDetailsEnd).ReqID; got != tc.reqID {
			t.Fatalf("end request id = %d, want %d", got, tc.reqID)
		}
	}
}

func TestServer205ContractDetailsFailClosed(t *testing.T) {
	t.Parallel()

	invalid := []ContractDetailsRequest{
		{ReqID: 1, Contract: Contract{Strike: "not-a-number"}},
		{ReqID: 1, Contract: Contract{Multiplier: "NaN"}},
	}
	if strconv.IntSize == 64 {
		overflow := int64(math.MaxInt32) + 1
		invalid = append(invalid,
			ContractDetailsRequest{ReqID: int(overflow)},
			ContractDetailsRequest{ReqID: 1, Contract: Contract{ConID: int(overflow)}},
		)
	}
	for _, msg := range invalid {
		if _, err := Encode(205, msg); err == nil {
			t.Fatalf("Encode(%+v) accepted invalid protobuf input", msg)
		}
	}

	for _, raw := range []string{
		"000000d20895a001",
		"000000d20895a0011001",
		"000000fc",
	} {
		if _, err := Decode(205, decodeHex(t, raw)); err == nil {
			t.Fatalf("Decode(%s) accepted incomplete or malformed contract data", raw)
		}
	}
}

func liveContractPayload(t *testing.T, value string) []byte {
	t.Helper()
	payload, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
