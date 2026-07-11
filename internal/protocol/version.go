package protocol

// Version gates are the first negotiated server_version at which a field or
// layout is present. Names follow the protocol concepts, not SDK class names.
const (
	MinServerVersionParametrizedDaysOfExecutions = 200
	MinServerVersionProtobuf                     = 201
	MinServerVersionZeroStrike                   = 202
	MinServerVersionProtobufPlaceOrder           = 203
	MinServerVersionProtobufCompletedOrder       = 204
	MinServerVersionProtobufContractData         = 205
	MinServerVersionProtobufMarketData           = 206
	MinServerVersionProtobufAccountsPositions    = 207
)
