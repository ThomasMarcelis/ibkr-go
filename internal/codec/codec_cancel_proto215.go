package codec

import (
	"fmt"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

// CancelContractData cancels an in-flight contract-details request. The
// Gateway introduced this as a protobuf-only message at server_version 215.
type CancelContractData struct {
	ReqID int
}

func (m CancelContractData) encodeWire(sv int) ([]string, error) {
	if sv < protocol.MinServerVersionBrokerSideOneShotCancel {
		return nil, fmt.Errorf("codec: contract-details cancellation requires server_version %d", protocol.MinServerVersionBrokerSideOneShotCancel)
	}
	return []string{itoa(protocol.OutCancelContractData)}, nil
}

func (m CancelContractData) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "contract-details cancellation request id")
}

// CancelHistoricalTicks cancels an in-flight historical-ticks request. The
// Gateway introduced this as a protobuf-only message at server_version 215.
type CancelHistoricalTicks struct {
	ReqID int
}

func (m CancelHistoricalTicks) encodeWire(sv int) ([]string, error) {
	if sv < protocol.MinServerVersionBrokerSideOneShotCancel {
		return nil, fmt.Errorf("codec: historical-ticks cancellation requires server_version %d", protocol.MinServerVersionBrokerSideOneShotCancel)
	}
	return []string{itoa(protocol.OutCancelHistoricalTicks)}, nil
}

func (m CancelHistoricalTicks) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "historical-ticks cancellation request id")
}
