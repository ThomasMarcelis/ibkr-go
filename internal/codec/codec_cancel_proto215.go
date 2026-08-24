package codec

// CancelContractData cancels an in-flight contract-details request. The
// Gateway introduced this as a protobuf-only message at server_version 215.
type CancelContractData struct {
	ReqID int
}

func (m CancelContractData) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "contract-details cancellation request id")
}

// CancelHistoricalTicks cancels an in-flight historical-ticks request. The
// Gateway introduced this as a protobuf-only message at server_version 215.
type CancelHistoricalTicks struct {
	ReqID int
}

func (m CancelHistoricalTicks) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "historical-ticks cancellation request id")
}
