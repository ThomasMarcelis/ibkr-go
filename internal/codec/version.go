package codec

import "github.com/ThomasMarcelis/ibkr-go/internal/protocol"

// Codec aliases keep field-layout decisions readable while the protocol
// package remains the single owner of negotiated-version gates.
const (
	MinServerVersionSmartDepth                     = protocol.MinServerVersionSmartDepth
	MinServerVersionFAProfileDesupport             = protocol.MinServerVersionFAProfileDesupport
	MinServerVersionManualOrderTimeExerciseOptions = protocol.MinServerVersionManualOrderTimeExerciseOptions
	MinServerVersionLastTradeDate                  = protocol.MinServerVersionLastTradeDate
	MinServerVersionCustomerAccount                = protocol.MinServerVersionCustomerAccount
	MinServerVersionProfessionalCustomer           = protocol.MinServerVersionProfessionalCustomer
	MinServerVersionBondAccruedInterest            = protocol.MinServerVersionBondAccruedInterest
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
