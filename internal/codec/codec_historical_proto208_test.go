package codec

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

const historicalSV208CaptureHash = "70229f753fb62a4c553b2fb2d1a2f9c2a19cafc5242945daf433b4b93a378ea3"

func TestHistoricalProto208LiveVectors(t *testing.T) {
	t.Parallel()

	request := HistoricalBarsRequest{
		ReqID: 7001,
		Contract: Contract{
			ConID: 265598, Symbol: "AAPL", SecType: "STK",
			Exchange: "SMART", Currency: "USD",
		},
		BarSize: "1 hour", Duration: "1 D", WhatToShow: "TRADES", UseRTH: true,
	}
	gotRequest, err := Encode(208, request)
	if err != nil {
		t.Fatal(err)
	}
	wantRequest := decodeHex(t, "000000dc08d936121b08fe9a1012044141504c1a0353544b4205534d415254520355534422063120686f75722a0331204430013a065452414445534001")
	if !bytes.Equal(gotRequest, wantRequest) {
		t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", gotRequest, wantRequest, historicalSV208CaptureHash)
	}

	barPayload := decodeHex(t, "000000d908d936125e0a2232303236303731332031353a33303a3030204575726f70652f416d7374657264616d11713d0ad7a3d07340193333333333377440213333333333c7734029ec51b81e851374403207373439323230303a073332302e38353740c6a803125e0a2232303236303731332031363a30303a3030204575726f70652f416d7374657264616d11ec51b81e8513744019cdcccccccc1c74402152b81e85ebe573402985eb51b81ef173403207343235353034333a073332302e30393840ffb202125d0a2232303236303731332031373a30303a3030204575726f70652f416d7374657264616d11ae47e17a14f273401948e17a14aef37340219a99999999cd73402985eb51b81ed173403207313532303631393a073331382e30323740f16e")
	bars, err := DecodeBatch(208, barPayload)
	if err != nil {
		t.Fatal(err)
	}
	wantBars := []Message{
		HistoricalBar{ReqID: 7001, Time: "20260713 15:30:00 Europe/Amsterdam", Open: "317.04", High: "323.45", Low: "316.45", Close: "321.22", Volume: "7492200", WAP: "320.857", Count: "54342"},
		HistoricalBar{ReqID: 7001, Time: "20260713 16:00:00 Europe/Amsterdam", Open: "321.22", High: "321.8", Low: "318.37", Close: "319.07", Volume: "4255043", WAP: "320.098", Count: "39295"},
		HistoricalBar{ReqID: 7001, Time: "20260713 17:00:00 Europe/Amsterdam", Open: "319.13", High: "319.23", Low: "316.85", Close: "317.07", Volume: "1520619", WAP: "318.027", Count: "14193"},
	}
	if !reflect.DeepEqual(bars, wantBars) {
		t.Fatalf("DecodeBatch() = %#v\nwant          = %#v\ncapture events sha256 %s", bars, wantBars, historicalSV208CaptureHash)
	}

	end, err := Decode(208, decodeHex(t, "0000013408d936122232303236303731322031373a34343a3339204575726f70652f416d7374657264616d1a2232303236303731332031373a34343a3339204575726f70652f416d7374657264616d"))
	if err != nil {
		t.Fatal(err)
	}
	wantEnd := HistoricalBarsEnd{ReqID: 7001, StartDate: "20260712 17:44:39 Europe/Amsterdam", EndDate: "20260713 17:44:39 Europe/Amsterdam"}
	if !reflect.DeepEqual(end, wantEnd) {
		t.Fatalf("Decode() = %#v, want %#v; capture events sha256 %s", end, wantEnd, historicalSV208CaptureHash)
	}
}

func TestHistoricalEncodingBoundary208(t *testing.T) {
	t.Parallel()

	msg := HistoricalBarsRequest{
		ReqID: 17, Contract: Contract{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"},
		BarSize: "1 day", Duration: "1 D", WhatToShow: "TRADES",
	}
	classic, err := Encode(207, msg)
	if err != nil {
		t.Fatal(err)
	}
	protobuf, err := Encode(208, msg)
	if err != nil {
		t.Fatal(err)
	}
	classicEnvelope, err := protocol.DecodeEnvelope(207, classic)
	if err != nil {
		t.Fatal(err)
	}
	protobufEnvelope, err := protocol.DecodeEnvelope(208, protobuf)
	if err != nil {
		t.Fatal(err)
	}
	if classicEnvelope.MsgID != protocol.OutReqHistoricalData || classicEnvelope.Encoding != protocol.ClassicBody {
		t.Fatalf("Encode(207) envelope = %#v", classicEnvelope)
	}
	if protobufEnvelope.MsgID != protocol.OutReqHistoricalData || protobufEnvelope.Encoding != protocol.ProtobufBody {
		t.Fatalf("Encode(208) envelope = %#v", protobufEnvelope)
	}
}

func TestHistoricalProto208RequestFamily(t *testing.T) {
	t.Parallel()

	contract := Contract{ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}
	tests := []struct {
		name string
		id   int
		msg  OutboundMessage
	}{
		{"cancel historical", protocol.OutCancelHistoricalData, CancelHistoricalData{ReqID: 1}},
		{"real-time bars", protocol.OutReqRealTimeBars, RealTimeBarsRequest{ReqID: 1, Contract: contract, WhatToShow: "TRADES"}},
		{"cancel real-time bars", protocol.OutCancelRealTimeBars, CancelRealTimeBars{ReqID: 1}},
		{"head timestamp", protocol.OutReqHeadTimestamp, HeadTimestampRequest{ReqID: 1, Contract: contract, WhatToShow: "TRADES"}},
		{"cancel head timestamp", protocol.OutCancelHeadTimestamp, CancelHeadTimestamp{ReqID: 1}},
		{"histogram", protocol.OutReqHistogramData, HistogramDataRequest{ReqID: 1, Contract: contract, Period: "3 days"}},
		{"cancel histogram", protocol.OutCancelHistogramData, CancelHistogramData{ReqID: 1}},
		{"historical ticks", protocol.OutReqHistoricalTicks, HistoricalTicksRequest{ReqID: 1, Contract: contract, EndDateTime: "20260713 17:00:00 Europe/Amsterdam", NumberOfTicks: 10, WhatToShow: "TRADES"}},
		{"tick-by-tick", protocol.OutReqTickByTickData, TickByTickRequest{ReqID: 1, Contract: contract, TickType: "Last", NumberOfTicks: 10}},
		{"cancel tick-by-tick", protocol.OutCancelTickByTickData, CancelTickByTick{ReqID: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload, err := Encode(208, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			envelope, err := protocol.DecodeEnvelope(208, payload)
			if err != nil {
				t.Fatal(err)
			}
			if envelope.MsgID != tc.id || envelope.Encoding != protocol.ProtobufBody {
				t.Fatalf("envelope = %#v, want protobuf base id %d", envelope, tc.id)
			}
		})
	}
}

func TestHistoricalProto208Callbacks(t *testing.T) {
	t.Parallel()

	bar := appendProtoString(nil, 1, "20260713 17:00:00 Europe/Amsterdam")
	bar = appendProtoDouble(bar, 2, 319.13)
	bar = appendProtoDouble(bar, 3, 319.23)
	bar = appendProtoDouble(bar, 4, 316.85)
	bar = appendProtoDouble(bar, 5, 317.07)
	bar = appendProtoString(bar, 6, "1520619")
	bar = appendProtoString(bar, 7, "318.027")
	bar = appendProtoVarint(bar, 8, 14193)
	lastAttributes := appendProtoVarint(nil, 1, 1)
	last := appendProtoVarint(nil, 1, 1_752_423_600)
	last = appendProtoMessage(last, 2, lastAttributes)
	last = appendProtoDouble(last, 3, 317.07)
	last = appendProtoString(last, 4, "100")
	last = appendProtoString(last, 5, "NASDAQ")
	last = appendProtoString(last, 6, "I")
	bidAskAttributes := appendProtoVarint(nil, 2, 1)
	bidAsk := appendProtoVarint(nil, 1, 1_752_423_600)
	bidAsk = appendProtoMessage(bidAsk, 2, bidAskAttributes)
	bidAsk = appendProtoDouble(bidAsk, 3, 317.06)
	bidAsk = appendProtoDouble(bidAsk, 4, 317.08)
	bidAsk = appendProtoString(bidAsk, 5, "200")
	bidAsk = appendProtoString(bidAsk, 6, "300")
	midpoint := appendProtoVarint(nil, 1, 1_752_423_600)
	midpoint = appendProtoDouble(midpoint, 2, 317.07)
	midpoint = appendProtoString(midpoint, 3, "0")
	session := appendProtoString(nil, 1, "20260713 09:30:00 America/New_York")
	session = appendProtoString(session, 2, "20260713 16:00:00 America/New_York")
	session = appendProtoString(session, 3, "20260713")

	tests := []struct {
		name string
		id   int
		body []byte
		want []Message
	}{
		{"historical update", protocol.InHistoricalDataUpdate, appendProtoMessage(appendProtoVarint(nil, 1, 7), 2, bar), []Message{HistoricalDataUpdate{ReqID: 7, BarCount: 14193, Time: "20260713 17:00:00 Europe/Amsterdam", Open: "319.13", High: "319.23", Low: "316.85", Close: "317.07", Volume: "1520619", WAP: "318.027"}}},
		{"real-time bar", protocol.InRealTimeBars, appendProtoVarint(appendProtoString(appendProtoString(appendProtoDouble(appendProtoDouble(appendProtoDouble(appendProtoDouble(appendProtoVarint(appendProtoVarint(nil, 1, 7), 2, 1_752_423_600), 3, 317), 4, 318), 5, 316), 6, 317.5), 7, "1000"), 8, "317.2"), 9, 42), []Message{RealTimeBar{ReqID: 7, Time: "1752423600", Open: "317", High: "318", Low: "316", Close: "317.5", Volume: "1000", WAP: "317.2", Count: "42"}}},
		{"head timestamp", protocol.InHeadTimestamp, appendProtoString(appendProtoVarint(nil, 1, 7), 2, "19801212-14:30:00"), []Message{HeadTimestamp{ReqID: 7, Timestamp: "19801212-14:30:00"}}},
		{"histogram", protocol.InHistogramData, appendProtoMessage(appendProtoVarint(nil, 1, 7), 2, appendProtoString(appendProtoDouble(nil, 1, 317.5), 2, "100")), []Message{HistogramDataResponse{ReqID: 7, Entries: []HistogramDataEntry{{Price: "317.5", Size: "100"}}}}},
		{"historical midpoint", protocol.InHistoricalTicks, appendProtoVarint(appendProtoMessage(appendProtoVarint(nil, 1, 7), 2, midpoint), 3, 1), []Message{HistoricalTicksResponse{ReqID: 7, Ticks: []HistoricalTickEntry{{Time: "1752423600", Price: "317.07", Size: "0"}}, Done: true}}},
		{"historical bid ask", protocol.InHistoricalTicksBidAsk, appendProtoVarint(appendProtoMessage(appendProtoVarint(nil, 1, 7), 2, bidAsk), 3, 1), []Message{HistoricalTicksBidAskResponse{ReqID: 7, Ticks: []HistoricalTickBidAskEntry{{TickAttrib: 2, Time: "1752423600", BidPrice: "317.06", AskPrice: "317.08", BidSize: "200", AskSize: "300"}}, Done: true}}},
		{"historical trade", protocol.InHistoricalTicksLast, appendProtoVarint(appendProtoMessage(appendProtoVarint(nil, 1, 7), 2, last), 3, 1), []Message{HistoricalTicksLastResponse{ReqID: 7, Ticks: []HistoricalTickLastEntry{{TickAttrib: 1, Time: "1752423600", Price: "317.07", Size: "100", Exchange: "NASDAQ", SpecialConditions: "I"}}, Done: true}}},
		{"tick-by-tick last", protocol.InTickByTick, appendProtoMessage(appendProtoVarint(appendProtoVarint(nil, 1, 7), 2, 1), 3, last), []Message{TickByTickData{ReqID: 7, TickType: 1, Time: "1752423600", Price: "317.07", Size: "100", Exchange: "NASDAQ", SpecialConditions: "I", TickAttribLast: 1}}},
		{"historical schedule", protocol.InHistoricalSchedule, appendProtoMessage(appendProtoString(appendProtoString(appendProtoString(appendProtoVarint(nil, 1, 7), 2, "20260713"), 3, "20260714"), 4, "America/New_York"), 5, session), []Message{HistoricalScheduleResponse{ReqID: 7, StartDateTime: "20260713", EndDateTime: "20260714", TimeZone: "America/New_York", Sessions: []HistoricalScheduleSession{{StartDateTime: "20260713 09:30:00 America/New_York", EndDateTime: "20260713 16:00:00 America/New_York", RefDate: "20260713"}}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload, err := protocol.EncodeProtobufEnvelope(208, tc.id, tc.body)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeBatch(208, payload)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("DecodeBatch() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestHistoricalProto208MissingNestedCallbackIsDropped(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		id   int
		body []byte
	}{
		{protocol.InHistoricalDataUpdate, appendProtoVarint(nil, 1, 7)},
		{protocol.InTickByTick, appendProtoVarint(appendProtoVarint(nil, 1, 7), 2, 1)},
	} {
		payload, err := protocol.EncodeProtobufEnvelope(208, tc.id, tc.body)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeBatch(208, payload)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("DecodeBatch(base id %d) = %#v, want no callback", tc.id, got)
		}
	}
}
