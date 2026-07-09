package protocol

// Version gates are the first negotiated server_version at which a field or
// layout is present. Names follow the protocol concepts, not SDK class names.
const (
	MinServerVersionLastLiquidity                  = 136
	MinServerVersionSmartDepth                     = 146
	MinServerVersionPendingPriceRevision           = 178
	MinServerVersionFAProfileDesupport             = 177
	MinServerVersionFundDataFields                 = 179
	MinServerVersionManualOrderTimeExerciseOptions = 180
	MinServerVersionLastTradeDate                  = 182
	MinServerVersionCustomerAccount                = 183
	MinServerVersionProfessionalCustomer           = 184
	MinServerVersionBondAccruedInterest            = 185
	MinServerVersionIneligibilityReasons           = 186
	MinServerVersionRFQFields                      = 187
	MinServerVersionBondTradingHours               = 188
	MinServerVersionIncludeOvernight               = 189
	MinServerVersionUndoRFQFields                  = 190
	MinServerVersionCMETaggingFields               = 192
	MinServerVersionCMETaggingFieldsInOpenOrder    = 193
	MinServerVersionErrorTime                      = 194
	MinServerVersionFullOrderPreviewFields         = 195
	MinServerVersionHistoricalDataEnd              = 196
	MinServerVersionCurrentTimeInMillis            = 197
	MinServerVersionSubmitter                      = 198
	MinServerVersionImbalanceOnly                  = 199
	MinServerVersionParametrizedDaysOfExecutions   = 200
	MinServerVersionProtobuf                       = 201
	MinServerVersionZeroStrike                     = 202
	MinServerVersionProtobufPlaceOrder             = 203
	MinServerVersionProtobufCompletedOrder         = 204
	MinServerVersionProtobufContractData           = 205
	MinServerVersionProtobufMarketData             = 206
	MinServerVersionProtobufAccountsPositions      = 207
	MinServerVersionProtobufHistoricalData         = 208
	MinServerVersionProtobufNewsData               = 209
	MinServerVersionProtobufScanData               = 210
	MinServerVersionProtobufRestMessages1          = 211
	MinServerVersionProtobufRestMessages2          = 212
	MinServerVersionProtobufRestMessages3          = 213
	MinServerVersionAddZSuffixToUTCDateTime        = 214
	MinServerVersionCancelContractData             = 215
	MinServerVersionAdditionalOrderParams1         = 216
	MinServerVersionAdditionalOrderParams2         = 217
	MinServerVersionAttachedOrders                 = 218
	MinServerVersionConfig                         = 219
	MinServerVersionMarketDataVolumesInShares      = 220
	MinServerVersionUpdateConfig                   = 221
	MinServerVersionFractionalLastSize             = 222
	MinServerVersionHedgeMaxSize                   = 223
	MinServerVersionUsePrecisionFromSecDef         = 224
	MinServerVersionOddLotBidAskQuotes             = 225
)
