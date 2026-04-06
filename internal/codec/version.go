package codec

// MinServerVersion constants gate conditional fields in the wire protocol.
// Each constant represents the minimum server version at which a particular
// field or feature was introduced.
const (
	MinServerVersionTradingClass        = 68
	MinServerVersionPastLimit           = 87
	MinServerVersionFractionalPositions = 101
	MinServerVersionModelCode           = 103
	MinServerVersionPreOpenBidAsk       = 105
	MinServerVersionPriceMagnifier      = 106
	MinServerVersionUnderConID          = 106
	MinServerVersionSizeRules           = 106
	MinServerVersionAggGroup            = 112
	MinServerVersionMarketRules         = 112
	MinServerVersionLastLiquidity       = 134
	MinServerVersionRealExpirationDate  = 134
	MinServerVersionCashQty             = 139
	MinServerVersionOrderContainer      = 141
	MinServerVersionExtOperator         = 148
	MinServerVersionProfitAndLoss       = 151
	MinServerVersionSoftDollarTier      = 160
	MinServerVersionBondIssuerId        = 176
	MinServerVersionDecimalSizeField    = 176
)
