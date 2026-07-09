package codec

import "github.com/ThomasMarcelis/ibkr-go/internal/protocol"

// Codec aliases keep field-layout decisions readable while the protocol
// package remains the single owner of negotiated-version gates.
const (
	MinServerVersionLastLiquidity                  = protocol.MinServerVersionLastLiquidity
	MinServerVersionSmartDepth                     = protocol.MinServerVersionSmartDepth
	MinServerVersionPendingPriceRevision           = protocol.MinServerVersionPendingPriceRevision
	MinServerVersionFAProfileDesupport             = protocol.MinServerVersionFAProfileDesupport
	MinServerVersionFundDataFields                 = protocol.MinServerVersionFundDataFields
	MinServerVersionManualOrderTimeExerciseOptions = protocol.MinServerVersionManualOrderTimeExerciseOptions
	MinServerVersionLastTradeDate                  = protocol.MinServerVersionLastTradeDate
	MinServerVersionCustomerAccount                = protocol.MinServerVersionCustomerAccount
	MinServerVersionProfessionalCustomer           = protocol.MinServerVersionProfessionalCustomer
	MinServerVersionBondAccruedInterest            = protocol.MinServerVersionBondAccruedInterest
	MinServerVersionIneligibilityReasons           = protocol.MinServerVersionIneligibilityReasons
	MinServerVersionRFQFields                      = protocol.MinServerVersionRFQFields
	MinServerVersionIncludeOvernight               = protocol.MinServerVersionIncludeOvernight
	MinServerVersionUndoRFQFields                  = protocol.MinServerVersionUndoRFQFields
	MinServerVersionCMETaggingFields               = protocol.MinServerVersionCMETaggingFields
	MinServerVersionCMETaggingFieldsInOpenOrder    = protocol.MinServerVersionCMETaggingFieldsInOpenOrder
	MinServerVersionErrorTime                      = protocol.MinServerVersionErrorTime
	MinServerVersionFullOrderPreviewFields         = protocol.MinServerVersionFullOrderPreviewFields
	MinServerVersionHistoricalDataEnd              = protocol.MinServerVersionHistoricalDataEnd
	MinServerVersionCurrentTimeInMillis            = protocol.MinServerVersionCurrentTimeInMillis
	MinServerVersionSubmitter                      = protocol.MinServerVersionSubmitter
	MinServerVersionImbalanceOnly                  = protocol.MinServerVersionImbalanceOnly
	MinServerVersionParametrizedDaysOfExecutions   = protocol.MinServerVersionParametrizedDaysOfExecutions
)
