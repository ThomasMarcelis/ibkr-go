package codec

// MinServerVersion constants gate conditional fields in the wire protocol.
// Each constant is the minimum negotiated server_version at which a field or
// feature is present on the wire, named after the official IBKR client's
// server_versions.py constant. The library negotiates 176..200, so only gates
// inside that window can fire; fields introduced at or below 176 are always
// present and need no constant here.
const (
	MinServerVersionFAProfileDesupport             = 177 // MIN_SERVER_VER_FA_PROFILE_DESUPPORT
	MinServerVersionManualOrderTimeExerciseOptions = 180 // MIN_SERVER_VER_MANUAL_ORDER_TIME_EXERCISE_OPTIONS
	MinServerVersionLastTradeDate                  = 182 // MIN_SERVER_VER_LAST_TRADE_DATE
	MinServerVersionCustomerAccount                = 183 // MIN_SERVER_VER_CUSTOMER_ACCOUNT
	MinServerVersionProfessionalCustomer           = 184 // MIN_SERVER_VER_PROFESSIONAL_CUSTOMER
	MinServerVersionBondAccruedInterest            = 185 // MIN_SERVER_VER_BOND_ACCRUED_INTEREST
	MinServerVersionRFQFields                      = 187 // MIN_SERVER_VER_RFQ_FIELDS
	MinServerVersionIncludeOvernight               = 189 // MIN_SERVER_VER_INCLUDE_OVERNIGHT
	MinServerVersionUndoRFQFields                  = 190 // MIN_SERVER_VER_UNDO_RFQ_FIELDS
	MinServerVersionCMETaggingFields               = 192 // MIN_SERVER_VER_CME_TAGGING_FIELDS
	MinServerVersionCMETaggingFieldsInOpenOrder    = 193 // MIN_SERVER_VER_CME_TAGGING_FIELDS_IN_OPEN_ORDER
	MinServerVersionErrorTime                      = 194 // MIN_SERVER_VER_ERROR_TIME
	MinServerVersionFullOrderPreviewFields         = 195 // MIN_SERVER_VER_FULL_ORDER_PREVIEW_FIELDS
	MinServerVersionHistoricalDataEnd              = 196 // MIN_SERVER_VER_HISTORICAL_DATA_END
	MinServerVersionCurrentTimeInMillis            = 197 // MIN_SERVER_VER_CURRENT_TIME_IN_MILLIS
	MinServerVersionSubmitter                      = 198 // MIN_SERVER_VER_SUBMITTER
	MinServerVersionImbalanceOnly                  = 199 // MIN_SERVER_VER_IMBALANCE_ONLY
	MinServerVersionParametrizedDaysOfExecutions   = 200 // MIN_SERVER_VER_PARAMETRIZED_DAYS_OF_EXECUTIONS
)
