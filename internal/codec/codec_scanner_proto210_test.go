package codec

import (
	"bytes"
	"reflect"
	"testing"
)

const (
	scannerPnLSV210CaptureHash = "7bc0159773624fae5e2babe8f8fa115bcce8d8403ebdbaa7c37e2515b2af1520"
	pnlSingleSV210CaptureHash  = "7736f2ebaefe422ceaf3354c32a8f346fd1459dd88cc0c1cc51235e067095b2d"
)

func TestEncodeScannerPnLProto210LiveVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  Message
		hex  string
		hash string
	}{
		{"scanner parameters", ScannerParametersRequest{}, "000000e0", scannerPnLSV210CaptureHash},
		{"scanner subscription", ScannerSubscriptionRequest{ReqID: 7203, NumberOfRows: 5, Instrument: "STK", LocationCode: "STK.US.MAJOR", ScanCode: "TOP_PERC_GAIN"}, "000000de08a33812240805120353544b1a0c53544b2e55532e4d414a4f52220d544f505f504552435f4741494e", scannerPnLSV210CaptureHash},
		{"cancel scanner", CancelScannerSubscription{ReqID: 7203}, "000000df08a338", scannerPnLSV210CaptureHash},
		{"PnL", PnLRequest{ReqID: 7201, Account: "DU9000001"}, "0000012408a1381209445539303030303031", scannerPnLSV210CaptureHash},
		{"cancel PnL", CancelPnL{ReqID: 7201}, "0000012508a138", scannerPnLSV210CaptureHash},
		{"single PnL", PnLSingleRequest{ReqID: 7202, Account: "DU9000001", ConID: 45602025}, "0000012608a238120944553930303030303120e9a9df15", pnlSingleSV210CaptureHash},
		{"cancel single PnL", CancelPnLSingle{ReqID: 7202}, "0000012708a238", pnlSingleSV210CaptureHash},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(210, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", got, want, tc.hash)
			}
		})
	}
}

func TestDecodeScannerPnLProto210LiveVectors(t *testing.T) {
	t.Parallel()

	scanner, err := Decode(210, decodeHex(t, "000000dc08a33812420800123908d5bf9ca3031204564545451a0353544b2900000000000000004205534d4152544a064e415344415152035553445a0456454545620353434d1a0353434d12420801123908a6b5def0021204534f42521a0353544b2900000000000000004205534d4152544a064e415344415152035553445a04534f4252620353434d1a0353434d12420802123908f5d6fbcb0212044147454e1a0353544b2900000000000000004205534d4152544a064e415344415152035553445a044147454e620353434d1a0353434d12420803123908c5bdbfca021204515454421a0353544b2900000000000000004205534d4152544a064e415344415152035553445a0451545442620353434d1a0353434d12420804123908fa889ff70212044654524b1a0353544b2900000000000000004205534d4152544a064e415344415152035553445a044654524b620353434d1a0353434d"))
	if err != nil {
		t.Fatal(err)
	}
	wantScanner := ScannerDataResponse{ReqID: 7203, Entries: []ScannerDataEntry{
		{Rank: 0, Contract: Contract{ConID: 879173589, Symbol: "VEEE", SecType: "STK", Strike: "0", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD", LocalSymbol: "VEEE", TradingClass: "SCM"}, MarketName: "SCM"},
		{Rank: 1, Contract: Contract{ConID: 773298854, Symbol: "SOBR", SecType: "STK", Strike: "0", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD", LocalSymbol: "SOBR", TradingClass: "SCM"}, MarketName: "SCM"},
		{Rank: 2, Contract: Contract{ConID: 696183669, Symbol: "AGEN", SecType: "STK", Strike: "0", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD", LocalSymbol: "AGEN", TradingClass: "SCM"}, MarketName: "SCM"},
		{Rank: 3, Contract: Contract{ConID: 693100229, Symbol: "QTTB", SecType: "STK", Strike: "0", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD", LocalSymbol: "QTTB", TradingClass: "SCM"}, MarketName: "SCM"},
		{Rank: 4, Contract: Contract{ConID: 786941050, Symbol: "FTRK", SecType: "STK", Strike: "0", Exchange: "SMART", PrimaryExchange: "NASDAQ", Currency: "USD", LocalSymbol: "FTRK", TradingClass: "SCM"}, MarketName: "SCM"},
	}}
	if !reflect.DeepEqual(scanner, wantScanner) {
		t.Fatalf("Decode(scanner) = %#v\nwant            = %#v\ncapture events sha256 %s", scanner, wantScanner, scannerPnLSV210CaptureHash)
	}

	pnl, err := Decode(210, decodeHex(t, "0000012608a138110aa071e3b9d1534019b33cc4d4aec37240210000000000000000"))
	if err != nil {
		t.Fatal(err)
	}
	wantPnL := PnLValue{ReqID: 7201, DailyPnL: "79.27697073074538", UnrealizedPnL: "300.23018337874527", RealizedPnL: "0"}
	if !reflect.DeepEqual(pnl, wantPnL) {
		t.Fatalf("Decode(PnL) = %#v, want %#v; capture events sha256 %s", pnl, wantPnL, scannerPnLSV210CaptureHash)
	}

	pnlSingle, err := Decode(210, decodeHex(t, "0000012708a2381201311940e17a1486ab364021c4f528dccc5c724031000000608f4b9d40"))
	if err != nil {
		t.Fatal(err)
	}
	wantSingle := PnLSingleValue{ReqID: 7202, Position: "1", DailyPnL: "22.670014648437473", UnrealizedPnL: "293.8000146484376", Value: "1874.8900146484375"}
	if !reflect.DeepEqual(pnlSingle, wantSingle) {
		t.Fatalf("Decode(single PnL) = %#v, want %#v; capture events sha256 %s", pnlSingle, wantSingle, pnlSingleSV210CaptureHash)
	}
}

func TestScannerProto210PreservesOptionalFalseAndMaps(t *testing.T) {
	t.Parallel()

	body, err := encodeScannerSubscriptionProto(ScannerSubscriptionRequest{
		NumberOfRows:       -1,
		ExcludeConvertible: "0",
		FilterOptions:      []TagValue{{Tag: "z", Value: "last"}, {Tag: "a", Value: "first"}},
		SubscriptionOptions: []TagValue{
			{Tag: "underConID", Value: "265598"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := decodeHex(t, "900100b2010a0a016112056669727374b201090a017a12046c617374ba01140a0a756e646572436f6e49441206323635353938")
	if !bytes.Equal(body, want) {
		t.Fatalf("encodeScannerSubscriptionProto() = %x, want %x", body, want)
	}
}

func TestScannerPnLProto210OmittedValuesRemainUnavailable(t *testing.T) {
	t.Parallel()

	pnl, err := decodePnLProto(nil, 210)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Message{PnLValue{ReqID: -1}}; !reflect.DeepEqual(pnl, want) {
		t.Fatalf("decodePnLProto(nil) = %#v, want %#v", pnl, want)
	}
	single, err := decodePnLSingleProto(nil, 210)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Message{PnLSingleValue{ReqID: -1}}; !reflect.DeepEqual(single, want) {
		t.Fatalf("decodePnLSingleProto(nil) = %#v, want %#v", single, want)
	}
}
