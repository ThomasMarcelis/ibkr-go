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
)
