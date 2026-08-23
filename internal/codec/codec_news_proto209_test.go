package codec

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

const newsSV209CaptureHash = "37b9fcdc9e5dc4ec5c1961e9a40dcec9aad165bd975e963543f1214efa8254e0"

func TestEncodeNewsProto209LiveVectors(t *testing.T) {
	t.Parallel()

	providerCodes := "BRFG+BRFUPDN+DJ-N+DJ-RT+DJ-RTA+DJ-RTE+DJ-RTG+DJNL"
	tests := []struct {
		name string
		msg  OutboundMessage
		hex  string
	}{
		{"providers", NewsProvidersRequest{}, "0000011d"},
		{"bulletins", NewsBulletinsRequest{AllMessages: true}, "000000d40801"},
		{"historical", HistoricalNewsRequest{ReqID: 7101, ConID: 265598, ProviderCodes: providerCodes, TotalResults: 5}, "0000011e08bd3710fe9a101a31425246472b4252465550444e2b444a2d4e2b444a2d52542b444a2d5254412b444a2d5254452b444a2d5254472b444a4e4c3005"},
		{"cancel bulletins", CancelNewsBulletins{}, "000000d5"},
		{"WSH metadata", WSHMetaDataRequest{ReqID: 7102}, "0000012c08be37"},
		{"cancel WSH metadata", CancelWSHMetaData{ReqID: 7102}, "0000012d08be37"},
		{"WSH event data", WSHEventDataRequest{ReqID: 7103, ConID: 265598}, "0000012e08bf3710fe9a10"},
		{"cancel WSH event data", CancelWSHEventData{ReqID: 7103}, "0000012f08bf37"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(209, tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if want := decodeHex(t, tc.hex); !bytes.Equal(got, want) {
				t.Fatalf("Encode() = %x\nwant     = %x\ncapture events sha256 %s", got, want, newsSV209CaptureHash)
			}
		})
	}
}

func TestDecodeNewsProto209LiveVectors(t *testing.T) {
	t.Parallel()

	providers, err := Decode(209, decodeHex(t, "0000011d0a2b0a044252464712234272696566696e672e636f6d2047656e6572616c204d61726b657420436f6c756d6e730a270a074252465550444e121c4272696566696e672e636f6d20416e616c79737420416374696f6e730a260a04444a2d4e121e446f77204a6f6e657320476c6f62616c20457175697479205472616465720a1e0a05444a2d52541215446f77204a6f6e657320547261646572204e6577730a2c0a06444a2d5254411222446f77204a6f6e657320546f702053746f72696573204173696120506163696669630a260a06444a2d525445121c446f77204a6f6e657320546f702053746f72696573204575726f70650a260a06444a2d525447121c446f77204a6f6e657320546f702053746f7269657320476c6f62616c0a1d0a04444a4e4c1215446f77204a6f6e6573204e6577736c657474657273"))
	if err != nil {
		t.Fatal(err)
	}
	wantProviders := NewsProviders{Providers: []NewsProviderEntry{
		{Code: "BRFG", Name: "Briefing.com General Market Columns"},
		{Code: "BRFUPDN", Name: "Briefing.com Analyst Actions"},
		{Code: "DJ-N", Name: "Dow Jones Global Equity Trader"},
		{Code: "DJ-RT", Name: "Dow Jones Trader News"},
		{Code: "DJ-RTA", Name: "Dow Jones Top Stories Asia Pacific"},
		{Code: "DJ-RTE", Name: "Dow Jones Top Stories Europe"},
		{Code: "DJ-RTG", Name: "Dow Jones Top Stories Global"},
		{Code: "DJNL", Name: "Dow Jones Newsletters"},
	}}
	if !reflect.DeepEqual(providers, wantProviders) {
		t.Fatalf("Decode(providers) = %#v\nwant              = %#v\ncapture events sha256 %s", providers, wantProviders, newsSV209CaptureHash)
	}

	item, err := Decode(209, decodeHex(t, "0000011e08bd371215323032362d30372d31332031353a33393a30302e301a04444a2d4e220d444a2d4e2431656531623766352a4f7b413a3830303031353a4c3a656e7d4170706c652043616e204f7574706572666f726d20696e20536c6f77696e6720536d61727470686f6e65204d61726b6574202d2d204d61726b65742054616c6b"))
	if err != nil {
		t.Fatal(err)
	}
	wantItem := HistoricalNewsItem{ReqID: 7101, Time: "2026-07-13 15:39:00.0", ProviderCode: "DJ-N", ArticleID: "DJ-N$1ee1b7f5", Headline: "{A:800015:L:en}Apple Can Outperform in Slowing Smartphone Market -- Market Talk"}
	if !reflect.DeepEqual(item, wantItem) {
		t.Fatalf("Decode(item) = %#v, want %#v; capture events sha256 %s", item, wantItem, newsSV209CaptureHash)
	}

	end, err := Decode(209, decodeHex(t, "0000011f08bd371001"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (HistoricalNewsEnd{ReqID: 7101, HasMore: true}); !reflect.DeepEqual(end, want) {
		t.Fatalf("Decode(end) = %#v, want %#v; capture events sha256 %s", end, want, newsSV209CaptureHash)
	}
}

func TestNewsProto209CallbackFamily(t *testing.T) {
	t.Parallel()

	provider := appendProtoString(appendProtoString(nil, 1, "BRFG"), 2, "Briefing")
	tests := []struct {
		name string
		id   int
		body []byte
		want Message
	}{
		{"bulletin", protocol.InNewsBulletins, appendProtoString(appendProtoString(appendProtoVarint(appendProtoVarint(nil, 1, 12), 2, 1), 3, "headline"), 4, "NYSE"), NewsBulletin{MsgID: 12, MsgType: 1, Headline: "headline", Source: "NYSE"}},
		{"article", protocol.InNewsArticle, appendProtoString(appendProtoVarint(appendProtoVarint(nil, 1, 7), 2, 1), 3, "body"), NewsArticleResponse{ReqID: 7, ArticleType: 1, ArticleText: "body"}},
		{"providers", protocol.InNewsProviders, appendProtoMessage(nil, 1, provider), NewsProviders{Providers: []NewsProviderEntry{{Code: "BRFG", Name: "Briefing"}}}},
		{"WSH metadata", protocol.InWSHMetaData, appendProtoString(appendProtoVarint(nil, 1, 7), 2, "{}"), WSHMetaDataResponse{ReqID: 7, DataJSON: "{}"}},
		{"WSH event data", protocol.InWSHEventData, appendProtoString(appendProtoVarint(nil, 1, 7), 2, "[]"), WSHEventDataResponse{ReqID: 7, DataJSON: "[]"}},
		{"tick news", protocol.InTickNews, appendProtoString(appendProtoString(appendProtoString(appendProtoString(appendProtoVarint(appendProtoVarint(nil, 1, 7), 2, 1_752_423_600_000), 3, "DJ-N"), 4, "A1"), 5, "headline"), 6, "extra"), TickNews{ReqID: 7, Time: "1752423600000", ProviderCode: "DJ-N", ArticleID: "A1", Headline: "headline", ExtraData: "extra"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload, err := protocol.EncodeProtobufEnvelope(209, tc.id, tc.body)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Decode(209, payload)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Decode() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestNewsEncodingBoundary209(t *testing.T) {
	t.Parallel()

	classic, err := Encode(208, NewsProvidersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	protobuf, err := Encode(209, NewsProvidersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	classicEnvelope, err := protocol.DecodeEnvelope(208, classic)
	if err != nil {
		t.Fatal(err)
	}
	protobufEnvelope, err := protocol.DecodeEnvelope(209, protobuf)
	if err != nil {
		t.Fatal(err)
	}
	if classicEnvelope.Encoding != protocol.ClassicBody || protobufEnvelope.Encoding != protocol.ProtobufBody {
		t.Fatalf("boundary encodings = %v, %v", classicEnvelope.Encoding, protobufEnvelope.Encoding)
	}
}
