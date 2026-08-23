package codec

import (
	"bytes"
	"reflect"
	"testing"
)

const accountProtoSV207CaptureHash = "936f9f4ea1633770071d9bd07a5ec721b7ddd481fcae6ad1aac95a9c1287a153"

func TestEncodeAccountProto207LiveVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  OutboundMessage
		hex  string
	}{
		{"subscribe account updates", AccountUpdatesRequest{Subscribe: true, Account: "DU9000001"}, "000000ce08011209445539303030303031"},
		{"unsubscribe account updates", AccountUpdatesRequest{Account: "DU9000001"}, "000000ce1209445539303030303031"},
		{"managed accounts", ManagedAccountsRequest{}, "000000d9"},
		{"positions", PositionsRequest{}, "00000105"},
		{"cancel positions", CancelPositions{}, "00000108"},
		{
			"account summary",
			AccountSummaryRequest{ReqID: 7001, Account: "All", Tags: []string{"NetLiquidation", "TotalCashValue"}},
			"0000010608d9361203416c6c1a1d4e65744c69717569646174696f6e2c546f74616c4361736856616c7565",
		},
		{"cancel account summary", CancelAccountSummary{ReqID: 7001}, "0000010708d936"},
		{"positions multi", PositionsMultiRequest{ReqID: 7002, Account: "DU9000001"}, "0000011208da361209445539303030303031"},
		{"cancel positions multi", CancelPositionsMulti{ReqID: 7002}, "0000011308da36"},
		{"account updates multi", AccountUpdatesMultiRequest{ReqID: 7003, Account: "DU9000001", LedgerAndNLV: true}, "0000011408db3612094455393030303030312001"},
		{"cancel account updates multi", CancelAccountUpdatesMulti{ReqID: 7003}, "0000011508db36"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(207, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", got, want, accountProtoSV207CaptureHash)
			}
		})
	}
}

func TestAccountEncodingBoundary207(t *testing.T) {
	t.Parallel()

	msg := AccountSummaryRequest{ReqID: 17, Account: "All", Tags: []string{"NetLiquidation"}}
	classic, err := Encode(206, msg)
	if err != nil {
		t.Fatal(err)
	}
	protobuf, err := Encode(207, msg)
	if err != nil {
		t.Fatal(err)
	}
	if want := decodeHex(t, "0000003e3100313700416c6c004e65744c69717569646174696f6e00"); !bytes.Equal(classic, want) {
		t.Fatalf("Encode(206) = %x, want %x", classic, want)
	}
	if want := decodeHex(t, "0000010608111203416c6c1a0e4e65744c69717569646174696f6e"); !bytes.Equal(protobuf, want) {
		t.Fatalf("Encode(207) = %x, want %x", protobuf, want)
	}
}

func TestAccountUpdatesMultiLedgerFlagBoundary207(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sv   int
		flag bool
		hex  string
	}{
		{"classic false", 206, false, "0000004c3100373030330044553930303030303100003000"},
		{"classic true", 206, true, "0000004c3100373030330044553930303030303100003100"},
		{"protobuf false omitted", 207, false, "0000011408db361209445539303030303031"},
		{"protobuf true", 207, true, "0000011408db3612094455393030303030312001"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(tc.sv, AccountUpdatesMultiRequest{
				ReqID: 7003, Account: "DU9000001", LedgerAndNLV: tc.flag,
			})
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode(%d, LedgerAndNLV=%t) = %x, want %x", tc.sv, tc.flag, got, want)
			}
		})
	}
}

func TestDecodeAccountProto207LiveVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hex  string
		want Message
	}{
		{"managed accounts", "000000d70a09445539303030303031", ManagedAccounts{Accounts: []string{"DU9000001"}}},
		{
			"position",
			"000001050a09445539303030303031123108e9a9df1512044d454c491a0353544b29000000000000000042064e415344415152035553445a044d454c4962034e4d531a0131218fc2f5285cb49840",
			Position{
				Account:  "DU9000001",
				Contract: Contract{ConID: 45602025, Symbol: "MELI", SecType: "STK", Strike: "0", Exchange: "NASDAQ", Currency: "USD", LocalSymbol: "MELI", TradingClass: "NMS"},
				Position: "1", AvgCost: "1581.09",
			},
		},
		{"position end", "00000106", PositionEnd{}},
		{"account summary", "0000010708d93612094455393030303030311a0e4e65744c69717569646174696f6e220833333932352e30342a03455552", AccountSummaryValue{ReqID: 7001, Account: "DU9000001", Tag: "NetLiquidation", Value: "33925.04", Currency: "EUR"}},
		{"account summary end", "0000010808d936", AccountSummaryEnd{ReqID: 7001}},
		{
			"position multi",
			"0000010f08da3612094455393030303030311a3108e9a9df1512044d454c491a0353544b29000000000000000042064e415344415152035553445a044d454c4962034e4d53220131298fc2f5285cb49840",
			PositionMulti{ReqID: 7002, Account: "DU9000001", Contract: Contract{ConID: 45602025, Symbol: "MELI", SecType: "STK", Strike: "0", Exchange: "NASDAQ", Currency: "USD", LocalSymbol: "MELI", TradingClass: "NMS"}, Position: "1", AvgCost: "1581.09"},
		},
		{"position multi end", "0000011008da36", PositionMultiEnd{ReqID: 7002}},
		{"account update multi", "0000011108db361209445539303030303031220843757272656e63792a03484b443203484b44", AccountUpdateMultiValue{ReqID: 7003, Account: "DU9000001", Key: "Currency", Value: "HKD", Currency: "HKD"}},
		{"account update multi end", "0000011208db36", AccountUpdateMultiEnd{ReqID: 7003}},
		{"account value", "000000ce0a0b4163636f756e74436f646512094455393030303030312209445539303030303031", UpdateAccountValue{Key: "AccountCode", Value: "DU9000001", Account: "DU9000001"}},
		{
			"portfolio value",
			"000000cf0a3808e6f6a40812063030303636301a0353544b29000000000000000042034b525852034b52575a093030303636302e4b5362063030303636301201341900000000d4ca40412100000000d4ca6041290000000090ff40413100000000005efac03900000000000000004209445539303030303031",
			UpdatePortfolio{Contract: Contract{ConID: 17382246, Symbol: "000660", SecType: "STK", Strike: "0", Exchange: "KRX", Currency: "KRW", LocalSymbol: "000660.KS", TradingClass: "000660"}, Position: "4", MarketPrice: "2.201e+06", MarketValue: "8.804e+06", AvgCost: "2.228e+06", UnrealizedPNL: "-108000", RealizedPNL: "0", Account: "DU9000001"},
		},
		{"account update time", "000000d00a0532323a3531", UpdateAccountTime{Timestamp: "22:51"}},
		{"account download end", "000000fe0a09445539303030303031", AccountDownloadEnd{Account: "DU9000001"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Decode(207, decodeHex(t, tc.hex))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Decode() = %#v\nwant     = %#v\ncapture events sha256 %s", got, tc.want, accountProtoSV207CaptureHash)
			}
		})
	}
}

func TestAccountProto207DropsMessagesWithoutRequiredContract(t *testing.T) {
	t.Parallel()

	for _, hex := range []string{"000001050a09445539303030303031", "000000cf4209445539303030303031", "0000010f08da36"} {
		messages, err := DecodeBatch(207, decodeHex(t, hex))
		if err != nil {
			t.Fatal(err)
		}
		if len(messages) != 0 {
			t.Fatalf("DecodeBatch(%s) = %#v, want no callbacks", hex, messages)
		}
	}
}
