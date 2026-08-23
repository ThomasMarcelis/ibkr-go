package codec

import (
	"slices"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

// Hot-path codec benchmarks over live IB Gateway wire data (server_version
// 200). Decode inputs are re-embedded verbatim from the capture fixtures in
// codec_capture_test.go (4-byte length prefix stripped); the encode input is
// copied from the encoder suite in codec_test.go. Each benchmark verifies its
// input decodes/encodes correctly once before the timed loop, so a broken
// input fails loudly instead of benchmarking garbage.

// benchTickPricePayload: captures/20260405T215734Z-quote_snapshot_aapl,
// tickType 68 (delayed last) at line 15. Same bytes as
// TestCaptureDecode_TickPrice. Tick price is the highest-frequency message in
// a streaming session, so this is the per-tick decode floor.
var benchTickPricePayload = []byte("1\x006\x001001\x0068\x00255.45\x00200\x000\x00")

// benchOpenOrderPayload: captures/20260405T215248Z-open_orders_all, the
// 940-byte openOrder frame at line 10 (OBDC PUT option, PreSubmitted): 156
// fields (155 after msg_id, the live sv200 layout with the "None"-sentinel
// delta-neutral block and the official 32-field tail). Same bytes as
// TestCaptureDecode_OpenOrder.
var benchOpenOrderPayload = []byte(
	"5\x000\x00853200900\x00OBDC\x00OPT\x0020261120\x0010\x00P\x00100\x00" +
		"SMART\x00USD\x00OBDC  261120P00010000\x00OBDC\x00SELL\x001\x00LMT\x00" +
		"1.2\x000.0\x00GTC\x00\x00DU9000001\x00\x000\x00\x000\x009000\x00" +
		"0\x000\x000\x00\x009000.1/DU9000001/100\x00\x00\x00\x00\x00\x00" +
		"0\x00\x00\x000\x00\x00-1\x000\x00\x00\x00\x00\x00\x002147483647\x00" +
		"0\x000\x000\x00\x003\x000\x000\x00\x000\x000\x00\x000\x00None\x00\x00" +
		"0\x00\x00\x00\x00?\x000\x000\x00\x000\x000\x00\x00\x00\x00\x00\x00" +
		"0\x000\x000\x002147483647\x002147483647\x00\x00\x000\x00\x00IB\x00" +
		"0\x000\x00\x000\x000\x00PreSubmitted\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00\x00\x00\x00\x00" +
		"\x001.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x00-9223372036854775808\x00\x000\x00\x000\x00" +
		"0\x000\x00None\x001.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x00" +
		"1.7976931348623157E308\x001.7976931348623157E308\x000\x00\x00\x00\x00" +
		"0\x001\x000\x000\x000\x00\x00\x000\x00\x00\x00\x00\x00\x00\x000\x00" +
		"\x000\x00\x002147483647\x00\x000\x00")

// benchContractDetailsPayload: captures/20260405T214938Z-contract_details_aapl_stk,
// line 10 (1171 payload bytes, 44 fields). Full AAPL STK contract details from
// the live gateway; same bytes as TestCaptureDecode_ContractDetails.
var benchContractDetailsPayload = []byte(
	"10\x001001\x00AAPL\x00STK\x00\x00\x000\x00\x00SMART\x00USD\x00AAPL\x00NMS\x00NMS\x00" +
		"265598\x000.01\x00\x00" +
		"ACTIVETIM,AD,ADDONT,ADJUST,ALERT,ALGO,ALLOC,AON,AVGCOST,BASKET,BENCHPX," +
		"CASHQTY,COND,CONDORDER,DARKONLY,DARKPOLL,DAY,DEACT,DEACTDIS,DEACTEOD,DIS," +
		"DUR,GAT,GTC,GTD,GTT,HID,IBKRATS,ICE,IMB,IOC,LIT,LMT,LOC,MIDPX,MIT,MKT," +
		"MOC,MTL,NGCOMB,NODARK,NONALGO,OCA,OPG,OPGREROUT,PEGBENCH,PEGMID,POSTATS," +
		"POSTONLY,PREOPGRTH,PRICECHK,REL,REL2MID,RELPCTOFS,RPI,RTH,SCALE,SCALEODD," +
		"SCALERST,SIZECHK,SNAPMID,SNAPMKT,SNAPREL,STP,STPLMT,SWEEP,TRAIL,TRAILLIT," +
		"TRAILLMT,TRAILMIT,WHATIF\x00" +
		"SMART,AMEX,NYSE,CBOE,PHLX,ISE,CHX,ARCA,NASDAQ,DRCTEDGE,BEX,BATS,EDGEA," +
		"BYX,IEX,EDGX,FOXRIVER,PEARL,NYSENAT,LTSE,MEMX,IBEOS,OVERNIGHT,TPLUS0," +
		"PSX,T24X\x00" +
		"1\x000\x00APPLE INC\x00NASDAQ\x00\x00Technology\x00Computers\x00Computers\x00US/Eastern\x00" +
		"20260405:CLOSED;20260406:0400-20260406:2000;20260407:0400-20260407:2000;" +
		"20260408:0400-20260408:2000;20260409:0400-20260409:2000;20260410:0400-20260410:2000\x00" +
		"20260405:CLOSED;20260406:0930-20260406:1600;20260407:0930-20260407:1600;" +
		"20260408:0930-20260408:1600;20260409:0930-20260409:1600;20260410:0930-20260410:1600\x00" +
		"\x00\x001\x00ISIN\x00US0378331005\x001\x00\x00\x00" +
		"26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26\x00" +
		"\x00COMMON\x000.0001\x000.0001\x00100\x000\x00")

func BenchmarkDecodeTickPrice(b *testing.B) {
	msgs, err := DecodeBatch(200, benchTickPricePayload)
	if err != nil {
		b.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		b.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(TickPrice)
	if !ok || m.ReqID != 1001 || m.TickType != 68 || m.Price != "255.45" {
		b.Fatalf("decoded %#v, want live tickType-68 frame", msgs[0])
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchTickPricePayload)))
	for b.Loop() {
		if _, err := DecodeBatch(200, benchTickPricePayload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeOpenOrderLive(b *testing.B) {
	msgs, err := DecodeBatch(200, benchOpenOrderPayload)
	if err != nil {
		b.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		b.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(OpenOrder)
	if !ok || m.Contract.ConID != 853200900 || m.Status != "PreSubmitted" || m.PermID != "9000" {
		b.Fatalf("decoded %#v, want live OBDC openOrder", msgs[0])
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchOpenOrderPayload)))
	for b.Loop() {
		if _, err := DecodeBatch(200, benchOpenOrderPayload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeContractDetails(b *testing.B) {
	msgs, err := DecodeBatch(200, benchContractDetailsPayload)
	if err != nil {
		b.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		b.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ContractDetails)
	if !ok || m.Contract.ConID != 265598 || m.LongName != "APPLE INC" || m.TimeZoneID != "US/Eastern" {
		b.Fatalf("decoded %#v, want live AAPL contract details", msgs[0])
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchContractDetailsPayload)))
	for b.Loop() {
		if _, err := DecodeBatch(200, benchContractDetailsPayload); err != nil {
			b.Fatal(err)
		}
	}
}

// benchHistoricalBarsPayload is the exact seven-bar frame from
// captures/20260405T215056Z-historical_bars_1d_1h, server_version 200.
var benchHistoricalBarsPayload = []byte(
	"17\x001001\x007\x00" +
		"20260402 09:30:00 US/Eastern\x00254.20\x00254.80\x00250.65\x00252.53\x002829736\x00252.266\x0013633\x00" +
		"20260402 10:00:00 US/Eastern\x00252.52\x00255.40\x00251.19\x00255.38\x002797972\x00252.971\x0016541\x00" +
		"20260402 11:00:00 US/Eastern\x00255.40\x00255.73\x00254.36\x00254.57\x001400669\x00255.002\x007744\x00" +
		"20260402 12:00:00 US/Eastern\x00254.57\x00255.00\x00254.00\x00254.42\x00983738\x00254.453\x005662\x00" +
		"20260402 13:00:00 US/Eastern\x00254.42\x00255.49\x00254.17\x00254.61\x001024324\x00254.878\x005832\x00" +
		"20260402 14:00:00 US/Eastern\x00254.58\x00255.46\x00254.58\x00255.28\x001399189\x00255.101\x007342\x00" +
		"20260402 15:00:00 US/Eastern\x00255.29\x00256.13\x00254.80\x00255.89\x002938382\x00255.576\x0017376\x00")

func BenchmarkDecodeHistoricalBars(b *testing.B) {
	msgs, err := DecodeBatch(200, benchHistoricalBarsPayload)
	if err != nil {
		b.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 7 {
		b.Fatalf("got %d messages, want 7 bars", len(msgs))
	}
	first, ok := msgs[0].(HistoricalBar)
	if !ok || first.ReqID != 1001 || first.Open != "254.20" || first.Count != "13633" {
		b.Fatalf("msgs[0] = %#v, want first live bar", msgs[0])
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(benchHistoricalBarsPayload)))
	for b.Loop() {
		if _, err := DecodeBatch(200, benchHistoricalBarsPayload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodePlaceOrder(b *testing.B) {
	// Field values copied from TestEncodePlaceOrderAdvancedSections in
	// codec_test.go — the fullest placeOrder case in the encoder suite,
	// exercising the combo-leg, leg-price, smart-routing, algo-params, and
	// conditions sections in a single frame.
	req := PlaceOrderRequest{
		OrderID: 77,
		Contract: Contract{
			ConID: 9001, Symbol: "BAG-TEST", SecType: "BAG", Exchange: "SMART", Currency: "USD",
			ComboLegs: []ComboLeg{{ConID: 101, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", DesignatedLocation: "", ExemptCode: "-1"}, {ConID: 102, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: "0", ShortSaleSlot: "0", DesignatedLocation: "", ExemptCode: "-1"}},
		},
		Action:                  "BUY",
		TotalQuantity:           "1",
		OrderType:               "LMT",
		LmtPrice:                "3.50",
		TIF:                     "DAY",
		Account:                 "DU9000001",
		Origin:                  "0",
		Transmit:                "1",
		ParentID:                "0",
		OutsideRTH:              "0",
		OrderComboLegPrices:     []string{"1.10", "2.40"},
		SmartComboRoutingParams: []TagValue{{Tag: "NonGuaranteed", Value: "1"}},
		AlgoStrategy:            "Adaptive",
		AlgoParams:              []TagValue{{Tag: "adaptivePriority", Value: "Patient"}},
		Solicited:               "0",
		RandomizeSize:           "0",
		RandomizePrice:          "0",
		Conditions:              []OrderCondition{{Type: 3, Conjunction: "a", Operator: 2, Value: "20260409 10:00:00 US/Eastern"}},
		ConditionsIgnoreRTH:     "1",
		ConditionsCancelOrder:   "0",
	}

	payload, err := Encode(200, req)
	if err != nil {
		b.Fatalf("Encode() error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		b.Fatalf("ParseFields() error = %v", err)
	}
	if fields[0] != strconv.Itoa(protocol.OutPlaceOrder) {
		b.Fatalf("msg_id = %q, want %d", fields[0], protocol.OutPlaceOrder)
	}
	if !slices.Contains(fields, "adaptivePriority") {
		b.Fatalf("encoded order is missing the algo-params section: %q", fields)
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := Encode(200, req); err != nil {
			b.Fatal(err)
		}
	}
}
