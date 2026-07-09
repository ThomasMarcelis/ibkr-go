package codec

// ReqIDer is implemented by every inbound message the engine routes by
// request ID. OpenOrder, OrderStatus, and CompletedOrder deliberately do
// not implement it: they carry an order ID, not a request ID, and route
// through the order-handle dispatch instead.
type ReqIDer interface{ RequestID() int }

func (m AccountSummaryEnd) RequestID() int             { return m.ReqID }
func (m AccountSummaryValue) RequestID() int           { return m.ReqID }
func (m AccountUpdateMultiEnd) RequestID() int         { return m.ReqID }
func (m AccountUpdateMultiValue) RequestID() int       { return m.ReqID }
func (m ContractDetails) RequestID() int               { return m.ReqID }
func (m ContractDetailsEnd) RequestID() int            { return m.ReqID }
func (m DisplayGroupList) RequestID() int              { return m.ReqID }
func (m DisplayGroupUpdated) RequestID() int           { return m.ReqID }
func (m ExecutionDetail) RequestID() int               { return m.ReqID }
func (m ExecutionsEnd) RequestID() int                 { return m.ReqID }
func (m HeadTimestamp) RequestID() int                 { return m.ReqID }
func (m HistogramDataResponse) RequestID() int         { return m.ReqID }
func (m HistoricalBar) RequestID() int                 { return m.ReqID }
func (m HistoricalBarsEnd) RequestID() int             { return m.ReqID }
func (m HistoricalDataUpdate) RequestID() int          { return m.ReqID }
func (m HistoricalNewsEnd) RequestID() int             { return m.ReqID }
func (m HistoricalNewsItem) RequestID() int            { return m.ReqID }
func (m HistoricalScheduleResponse) RequestID() int    { return m.ReqID }
func (m HistoricalTicksBidAskResponse) RequestID() int { return m.ReqID }
func (m HistoricalTicksLastResponse) RequestID() int   { return m.ReqID }
func (m HistoricalTicksResponse) RequestID() int       { return m.ReqID }
func (m MarketDataType) RequestID() int                { return m.ReqID }
func (m MarketDepthL2Update) RequestID() int           { return m.ReqID }
func (m MarketDepthUpdate) RequestID() int             { return m.ReqID }
func (m MatchingSymbols) RequestID() int               { return m.ReqID }
func (m NewsArticleResponse) RequestID() int           { return m.ReqID }
func (m PnLSingleValue) RequestID() int                { return m.ReqID }
func (m PnLValue) RequestID() int                      { return m.ReqID }
func (m PositionMulti) RequestID() int                 { return m.ReqID }
func (m PositionMultiEnd) RequestID() int              { return m.ReqID }
func (m ReplaceFAEnd) RequestID() int                  { return m.ReqID }
func (m RealTimeBar) RequestID() int                   { return m.ReqID }
func (m ScannerDataResponse) RequestID() int           { return m.ReqID }
func (m SecDefOptParamsEnd) RequestID() int            { return m.ReqID }
func (m SecDefOptParamsResponse) RequestID() int       { return m.ReqID }
func (m SmartComponentsResponse) RequestID() int       { return m.ReqID }
func (m SoftDollarTiersResponse) RequestID() int       { return m.ReqID }
func (m TickByTickData) RequestID() int                { return m.ReqID }
func (m TickGeneric) RequestID() int                   { return m.ReqID }
func (m TickOptionComputation) RequestID() int         { return m.ReqID }
func (m TickPrice) RequestID() int                     { return m.ReqID }
func (m TickReqParams) RequestID() int                 { return m.ReqID }
func (m TickSize) RequestID() int                      { return m.ReqID }
func (m TickSnapshotEnd) RequestID() int               { return m.ReqID }
func (m TickString) RequestID() int                    { return m.ReqID }
func (m UserInfo) RequestID() int                      { return m.ReqID }
func (m WSHEventDataResponse) RequestID() int          { return m.ReqID }
func (m WSHMetaDataResponse) RequestID() int           { return m.ReqID }
