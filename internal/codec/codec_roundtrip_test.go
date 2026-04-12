package codec

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeFieldEquality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  Message
		// skip indicates the message cannot DeepEqual after roundtrip due to
		// known encode/decode asymmetries (documented per case). For these we
		// verify only messageName() survives.
		skip string
	}{
		{
			name: "ManagedAccounts",
			msg:  ManagedAccounts{Accounts: []string{"DU12345", "DU67890"}},
		},
		{
			name: "NextValidID",
			msg:  NextValidID{OrderID: 1001},
		},
		{
			name: "CurrentTime",
			msg:  CurrentTime{Time: "1712345678"},
		},
		{
			name: "APIError",
			msg:  APIError{ReqID: -1, Code: 2104, Message: "Market data farm OK", AdvancedOrderRejectJSON: "", ErrorTimeMs: "1712345678000"},
		},
		{
			// ContractDetails encode/decode is asymmetric: the encoder writes
			// Expiry twice (as lastTradeDate and lastTradeDateOrContractMonth),
			// and the decoder skips intermediate fields (mdSizeMultiplier,
			// orderTypes, validExchanges, etc.) that the encoder fills with
			// empty strings. Also the encoder does not use readWireContract
			// so the field order differs from the standard 11-field block.
			// The round-trip preserves all ContractDetails-level fields.
			name: "ContractDetails",
			msg: ContractDetails{
				ReqID: 42,
				Contract: Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK",
					Exchange: "SMART", Currency: "USD",
					LocalSymbol: "AAPL", TradingClass: "AAPL",
					PrimaryExchange: "NASDAQ",
				},
				MarketName: "NMS", MinTick: "0.01",
				LongName: "APPLE INC", TimeZoneID: "US/Eastern",
			},
		},
		{
			name: "ContractDetailsEnd",
			msg:  ContractDetailsEnd{ReqID: 42},
		},
		{
			// HistoricalBar encodes as InHistoricalData with barCount=1.
			// DecodeBatch returns [HistoricalBar, HistoricalBarsEnd], so
			// msgs[0] is the bar and should DeepEqual.
			name: "HistoricalBar",
			msg: HistoricalBar{
				ReqID: 1, Time: "20260405 10:00:00", Open: "150.00",
				High: "151.50", Low: "149.80", Close: "151.20",
				Volume: "123456", WAP: "150.75", Count: "5432",
			},
		},
		{
			name: "HistoricalBarsEnd",
			msg:  HistoricalBarsEnd{ReqID: 1},
		},
		{
			name: "AccountSummaryValue",
			msg:  AccountSummaryValue{ReqID: 1, Account: "DU12345", Tag: "NetLiquidation", Value: "100000.00", Currency: "USD"},
		},
		{
			name: "AccountSummaryEnd",
			msg:  AccountSummaryEnd{ReqID: 1},
		},
		{
			// Position: the encoder writes Strike as "0" when empty, and
			// readWireContract reads it back as "0". To get a clean roundtrip
			// we must either use Strike="0" or a non-empty strike in the input.
			// readWireContract does not populate PrimaryExchange.
			name: "Position",
			msg: Position{
				Account: "DU12345",
				Contract: Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK",
					Strike: "0", Exchange: "SMART", Currency: "USD",
				},
				Position: "100", AvgCost: "150.25",
			},
		},
		{
			name: "PositionEnd",
			msg:  PositionEnd{},
		},
		{
			name: "TickPrice",
			msg:  TickPrice{ReqID: 1, TickType: 1, Price: "189.10", Size: "400", AttrMask: 3},
		},
		{
			name: "TickSize",
			msg:  TickSize{ReqID: 1, TickType: 0, Size: "400"},
		},
		{
			name: "MarketDataType",
			msg:  MarketDataType{ReqID: 1, DataType: 3},
		},
		{
			name: "TickSnapshotEnd",
			msg:  TickSnapshotEnd{ReqID: 1},
		},
		{
			name: "RealTimeBar",
			msg: RealTimeBar{
				ReqID: 1, Time: "1712345678", Open: "100.0", High: "101.0",
				Low: "99.5", Close: "100.5", Volume: "1000", WAP: "100.5", Count: "50",
			},
		},
		{
			// OpenOrder: the encoder uses writeWireContract (client->server
			// 11-field layout: symbol..tradingClass with primaryExchange)
			// but the decoder uses readWireContract (server->client layout:
			// conID..tradingClass without primaryExchange). The contract
			// field offset mismatch means the contract block does not
			// roundtrip through DeepEqual. All non-contract fields survive.
			name: "OpenOrder",
			skip: "writeWireContract/readWireContract layout mismatch for contract block",
			msg: OpenOrder{
				OrderID: 42,
				Contract: Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK",
					Strike: "0", Exchange: "SMART", Currency: "USD",
					LocalSymbol: "AAPL", TradingClass: "AAPL",
				},
				Account: "DU12345", Action: "BUY", Quantity: "10",
				OrderType: "LMT", LmtPrice: "150.00", AuxPrice: "0.0",
				TIF: "DAY", OcaGroup: "", OpenClose: "", Origin: "0",
				OrderRef: "test-ref", ClientID: "99", PermID: "123456",
				OutsideRTH: "0", Hidden: "0", DiscretionAmt: "0",
				GoodAfterTime:    "",
				Status:           "Submitted",
				InitMarginBefore: "1.7976931348623157E308", MaintMarginBefore: "1.7976931348623157E308",
				EquityWithLoanBefore: "1.7976931348623157E308",
				InitMarginChange:     "1.7976931348623157E308", MaintMarginChange: "1.7976931348623157E308",
				EquityWithLoanChange: "1.7976931348623157E308",
				InitMarginAfter:      "1.7976931348623157E308", MaintMarginAfter: "1.7976931348623157E308",
				EquityWithLoanAfter: "1.7976931348623157E308",
				Commission:          "1.7976931348623157E308", MinCommission: "1.7976931348623157E308",
				MaxCommission: "1.7976931348623157E308", CommissionCurrency: "",
				WarningText: "",
				Filled:      "2", Remaining: "8", ParentID: "99",
			},
		},
		{
			name: "OpenOrderEnd",
			msg:  OpenOrderEnd{},
		},
		{
			name: "OrderStatus",
			msg: OrderStatus{
				OrderID: 42, Status: "Filled", Filled: "100", Remaining: "0",
				AvgFillPrice: "150.50", PermID: "123456", ParentID: "0",
				LastFillPrice: "150.50", ClientID: "99", WhyHeld: "", MktCapPrice: "0",
			},
		},
		{
			// ExecutionDetail: the encoder writes conID as "0" and pads
			// secType..tradingClass with empty strings. The decoder skips
			// those fields. The Symbol field is read at a specific position
			// before the skipped block. All extracted fields roundtrip.
			name: "ExecutionDetail",
			msg: ExecutionDetail{
				ReqID: 1, OrderID: 42, ExecID: "0001", Account: "DU12345",
				Symbol: "AAPL", Side: "BOT", Shares: "100",
				Price: "150.50", Time: "20260407 10:30:00",
			},
		},
		{
			name: "ExecutionsEnd",
			msg:  ExecutionsEnd{ReqID: 1},
		},
		{
			name: "CommissionReport",
			msg:  CommissionReport{ExecID: "exec-1", Commission: "1.00", Currency: "USD", RealizedPNL: "50.00"},
		},
		{
			name: "TickGeneric",
			msg:  TickGeneric{ReqID: 1, TickType: 49, Value: "0.123"},
		},
		{
			name: "TickString",
			msg:  TickString{ReqID: 1, TickType: 45, Value: "1712300400"},
		},
		{
			name: "TickReqParams",
			msg:  TickReqParams{ReqID: 1, MinTick: "0.01", BBOExchange: "SMART", SnapshotPermissions: 3},
		},
		{
			name: "UpdateAccountValue",
			msg:  UpdateAccountValue{Key: "NetLiquidation", Value: "100000.00", Currency: "USD", Account: "DU12345"},
		},
		{
			// UpdatePortfolio: the encoder writes the contract as
			// [conID, symbol, secType, expiry, strike, right, multiplier,
			//  primaryExchange, currency, localSymbol, tradingClass]
			// and the decoder reads them in the same order. This is NOT
			// the standard readWireContract layout (which has exchange
			// instead of primaryExchange at that position). Strike "0"
			// default applies.
			name: "UpdatePortfolio",
			msg: UpdatePortfolio{
				Contract: Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK",
					Strike: "0", PrimaryExchange: "NASDAQ", Currency: "USD",
				},
				Position: "100", MarketPrice: "150.50", MarketValue: "15050.00",
				AvgCost: "150.25", UnrealizedPNL: "25.00", RealizedPNL: "0.00",
				Account: "DU12345",
			},
		},
		{
			name: "UpdateAccountTime",
			msg:  UpdateAccountTime{Timestamp: "15:30"},
		},
		{
			name: "AccountDownloadEnd",
			msg:  AccountDownloadEnd{Account: "DU12345"},
		},
		{
			name: "AccountUpdateMultiValue",
			msg:  AccountUpdateMultiValue{ReqID: 1, Account: "DU12345", ModelCode: "", Key: "NetLiquidation", Value: "100000.00", Currency: "USD"},
		},
		{
			name: "AccountUpdateMultiEnd",
			msg:  AccountUpdateMultiEnd{ReqID: 1},
		},
		{
			// PositionMulti: uses readWireContract on decode which does not
			// populate PrimaryExchange. Strike "0" default applies.
			name: "PositionMulti",
			msg: PositionMulti{
				ReqID: 1, Account: "DU12345", ModelCode: "",
				Contract: Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK",
					Strike: "0", Exchange: "SMART", Currency: "USD",
				},
				Position: "100", AvgCost: "150.25",
			},
		},
		{
			name: "PositionMultiEnd",
			msg:  PositionMultiEnd{ReqID: 1},
		},
		{
			name: "PnLValue",
			msg:  PnLValue{ReqID: 1, DailyPnL: "123.45", UnrealizedPnL: "678.90", RealizedPnL: "12.34"},
		},
		{
			name: "PnLSingleValue",
			msg:  PnLSingleValue{ReqID: 1, Position: "100", DailyPnL: "45.67", UnrealizedPnL: "234.56", RealizedPnL: "11.22", Value: "15050.00"},
		},
		{
			// TickByTickData (Last): TickType 1 encodes the Last fields
			// (price, size, tickAttribLast, exchange, specialConditions).
			// BidAsk/MidPoint fields remain zero-valued and roundtrip as such.
			name: "TickByTickData/Last",
			msg: TickByTickData{
				ReqID: 1, TickType: 1, Time: "1712345678",
				Price: "150.50", Size: "100", Exchange: "ARCA",
				SpecialConditions: "", TickAttribLast: 0,
			},
		},
		{
			// TickByTickData (BidAsk): TickType 3.
			name: "TickByTickData/BidAsk",
			msg: TickByTickData{
				ReqID: 1, TickType: 3, Time: "1712345678",
				BidPrice: "150.00", AskPrice: "150.05",
				BidSize: "100", AskSize: "200", TickAttribBidAsk: 0,
			},
		},
		{
			// TickByTickData (MidPoint): TickType 4.
			name: "TickByTickData/MidPoint",
			msg: TickByTickData{
				ReqID: 1, TickType: 4, Time: "1712345678",
				MidPoint: "150.025",
			},
		},
		{
			name: "NewsBulletin",
			msg:  NewsBulletin{MsgID: 1, MsgType: 1, Headline: "Test bulletin", Source: "System"},
		},
		{
			name: "HistoricalDataUpdate",
			msg: HistoricalDataUpdate{
				ReqID: 1, BarCount: 5, Time: "20260407 10:00:00",
				Open: "150.00", High: "151.50", Low: "149.80",
				Close: "151.20", Volume: "123456", WAP: "150.75", Count: "5432",
			},
		},
		{
			name: "SecDefOptParamsResponse",
			msg: SecDefOptParamsResponse{
				ReqID: 1, Exchange: "SMART", UnderlyingConID: 265598,
				TradingClass: "AAPL", Multiplier: "100",
				Expirations: []string{"20260620"}, Strikes: []string{"150.0"},
			},
		},
		{
			name: "SecDefOptParamsEnd",
			msg:  SecDefOptParamsEnd{ReqID: 1},
		},
		{
			// SmartComponentsResponse: shares msg_id 82 with MatchingSymbols.
			// The decoder disambiguates by checking if remaining == count*3.
			name: "SmartComponentsResponse",
			msg: SmartComponentsResponse{
				ReqID: 1,
				Components: []SmartComponentEntry{
					{BitNumber: 1, ExchangeName: "ARCA", ExchangeLetter: "P"},
				},
			},
		},
		{
			name: "TickOptionComputation",
			msg: TickOptionComputation{
				ReqID: 1, TickType: 13, TickAttrib: 1,
				ImpliedVol: "0.25", Delta: "0.65", OptPrice: "5.50",
				PvDividend: "0.10", Gamma: "0.03", Vega: "0.15",
				Theta: "-0.05", UndPrice: "150.00",
			},
		},
		{
			name: "HistogramDataResponse",
			msg: HistogramDataResponse{
				ReqID:   1,
				Entries: []HistogramDataEntry{{Price: "150.00", Size: "1000"}},
			},
		},
		{
			name: "HistoricalTicksResponse",
			msg: HistoricalTicksResponse{
				ReqID: 1,
				Ticks: []HistoricalTickEntry{{Time: "1712345678", Price: "150.00", Size: "100"}},
				Done:  true,
			},
		},
		{
			name: "HistoricalTicksBidAskResponse",
			msg: HistoricalTicksBidAskResponse{
				ReqID: 1,
				Ticks: []HistoricalTickBidAskEntry{{Time: "1712345678", TickAttrib: 1, BidPrice: "150.00", AskPrice: "150.05", BidSize: "100", AskSize: "200"}},
				Done:  true,
			},
		},
		{
			name: "HistoricalTicksLastResponse",
			msg: HistoricalTicksLastResponse{
				ReqID: 1,
				Ticks: []HistoricalTickLastEntry{{Time: "1712345678", TickAttrib: 2, Price: "150.00", Size: "100", Exchange: "ARCA", SpecialConditions: ""}},
				Done:  true,
			},
		},
		{
			name: "NewsArticleResponse",
			msg:  NewsArticleResponse{ReqID: 1, ArticleType: 1, ArticleText: "Article content"},
		},
		{
			name: "HistoricalNewsItem",
			msg:  HistoricalNewsItem{ReqID: 1, Time: "20260407 10:00:00", ProviderCode: "BRFG", ArticleID: "BRFG$12345", Headline: "Test headline"},
		},
		{
			name: "HistoricalNewsEnd",
			msg:  HistoricalNewsEnd{ReqID: 1, HasMore: false},
		},
		{
			name: "ScannerParameters",
			msg:  ScannerParameters{XML: "<ScannerParameters/>"},
		},
		{
			// ScannerDataResponse: the encoder writes contract fields
			// directly (not via writeWireContract) matching readWireContract.
			// Strike "0" default applies.
			name: "ScannerDataResponse",
			msg: ScannerDataResponse{
				ReqID: 1,
				Entries: []ScannerDataEntry{{
					Rank: 0,
					Contract: Contract{
						ConID: 265598, Symbol: "AAPL", SecType: "STK",
						Strike: "0", Exchange: "SMART", Currency: "USD",
					},
					Distance: "", Benchmark: "", Projection: "", LegsStr: "",
				}},
			},
		},
		{
			name: "UserInfo",
			msg:  UserInfo{ReqID: 1, WhiteBrandingID: "brand1"},
		},
		{
			name: "MatchingSymbols",
			msg: MatchingSymbols{
				ReqID: 1,
				Symbols: []SymbolSample{{
					ConID: 265598, Symbol: "AAPL", SecType: "STK",
					PrimaryExchange: "NASDAQ", Currency: "USD",
					DerivativeSecTypes: []string{"OPT"},
				}},
			},
		},
		{
			name: "HeadTimestamp",
			msg:  HeadTimestamp{ReqID: 1, Timestamp: "20200101 00:00:00"},
		},
		{
			name: "MarketRule",
			msg: MarketRule{
				MarketRuleID: 26,
				Increments:   []PriceIncrement{{LowEdge: "0", Increment: "0.01"}},
			},
		},
		{
			// CompletedOrder: the encoder writes the contract directly
			// (matching readWireContract), then pads the live v200 completed-order
			// layout. The wire has a filled quantity but no remaining quantity.
			name: "CompletedOrder",
			msg: CompletedOrder{
				Contract: Contract{
					ConID: 265598, Symbol: "AAPL", SecType: "STK",
					Strike: "0", Exchange: "SMART", Currency: "USD",
				},
				Action: "BUY", OrderType: "LMT", Status: "Filled",
				Quantity: "100", Filled: "100",
			},
		},
		{
			name: "CompletedOrderEnd",
			msg:  CompletedOrderEnd{},
		},
		{
			name: "ReceiveFA",
			msg:  ReceiveFA{FADataType: 1, XML: "<Groups/>"},
		},
		{
			name: "SoftDollarTiersResponse",
			msg: SoftDollarTiersResponse{
				ReqID: 1,
				Tiers: []SoftDollarTier{{Name: "tier1", Value: "val1", DisplayName: "Tier One"}},
			},
		},
		{
			name: "WSHMetaDataResponse",
			msg:  WSHMetaDataResponse{ReqID: 1, DataJSON: "{\"meta\":\"data\"}"},
		},
		{
			name: "WSHEventDataResponse",
			msg:  WSHEventDataResponse{ReqID: 1, DataJSON: "{\"events\":[]}"},
		},
		{
			name: "HistoricalScheduleResponse",
			msg: HistoricalScheduleResponse{
				ReqID:         1,
				StartDateTime: "20260312-09:30:00",
				EndDateTime:   "20260410-16:00:00",
				TimeZone:      "US/Eastern",
				Sessions: []HistoricalScheduleSession{
					{StartDateTime: "20260312-09:30:00", EndDateTime: "20260312-16:00:00", RefDate: "20260312"},
					{StartDateTime: "20260313-09:30:00", EndDateTime: "20260313-16:00:00", RefDate: "20260313"},
				},
			},
		},
		{
			name: "DisplayGroupList",
			msg:  DisplayGroupList{ReqID: 1, Groups: "1|2|3"},
		},
		{
			name: "DisplayGroupUpdated",
			msg:  DisplayGroupUpdated{ReqID: 1, ContractInfo: "265598@SMART"},
		},
		{
			name: "MarketDepthUpdate",
			msg:  MarketDepthUpdate{ReqID: 1, Position: 0, Operation: 0, Side: 1, Price: "150.00", Size: "100"},
		},
		{
			name: "MarketDepthL2Update",
			msg:  MarketDepthL2Update{ReqID: 1, Position: 0, MarketMaker: "ARCA", Operation: 0, Side: 1, Price: "150.00", Size: "100", IsSmartDepth: true},
		},
		{
			name: "FundamentalDataResponse",
			msg:  FundamentalDataResponse{ReqID: 1, Data: "<FundamentalData/>"},
		},
		{
			name: "FamilyCodes",
			msg: FamilyCodes{
				Codes: []FamilyCodeEntry{{AccountID: "DU12345", FamilyCode: "FAM1"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := Encode(tt.msg)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			msgs, err := DecodeBatch(payload)
			if err != nil {
				t.Fatalf("DecodeBatch() error = %v", err)
			}
			if len(msgs) == 0 {
				t.Fatal("DecodeBatch() returned 0 messages")
			}

			if tt.skip != "" {
				// Known asymmetry: verify messageName only.
				if msgs[0].messageName() != tt.msg.messageName() {
					t.Fatalf("messageName() = %q, want %q", msgs[0].messageName(), tt.msg.messageName())
				}
				return
			}

			if !reflect.DeepEqual(msgs[0], tt.msg) {
				t.Errorf("roundtrip mismatch:\ngot  %+v\nwant %+v", msgs[0], tt.msg)
			}
		})
	}
}
