package codec

import "github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"

type StartAPI struct {
	ClientID             int
	OptionalCapabilities string
}

func (m StartAPI) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutStartAPI), "2", itoa(m.ClientID), m.OptionalCapabilities}, nil
}

type ServerInfo struct {
	ServerVersion  int
	ConnectionTime string
}

type ManagedAccounts struct {
	Accounts []string
}

// ManagedAccountsRequest is the outbound reqManagedAccts message (OUT 17).
// The server answers with the same ManagedAccounts callback used at bootstrap.
type ManagedAccountsRequest struct{}

type NextValidID struct {
	OrderID int64
}

type CurrentTime struct {
	Time string
}

// CurrentTimeRequest is the outbound reqCurrentTime message (OUT 49). The
// server responds asynchronously with a CurrentTime frame using the same
// numeric msg_id.
type CurrentTimeRequest struct{}

func (m CurrentTimeRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqCurrentTime), "1"}, nil
}

// CurrentTimeMillis is the inbound currentTimeInMillis frame (IN 109): the
// server epoch time in milliseconds, no version field.
type CurrentTimeMillis struct {
	TimeMs string
}

// CurrentTimeMillisRequest is the outbound reqCurrentTimeInMillis message
// (OUT 105): the bare message id with no version or fields, answered by a
// CurrentTimeMillis frame (IN 109).
type CurrentTimeMillisRequest struct{}

func (m CurrentTimeMillisRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqCurrentTimeInMillis)}, nil
}

// ReqIDsRequest is the outbound reqIds message (OUT 8). The server responds
// with a NextValidID frame (msg_id 9) carrying the next available order ID.
// NumIDs is a legacy parameter kept at 1 in the official EClient.
type ReqIDsRequest struct {
	NumIDs int
}

func (m ReqIDsRequest) encodeWire(sv int) ([]string, error) {
	numIDs := m.NumIDs
	if numIDs <= 0 {
		numIDs = 1
	}
	return []string{itoa(protocol.OutReqIds), "1", itoa(numIDs)}, nil
}

type APIError struct {
	ReqID                   int
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
	ErrorTimeMs             string
}

type UserInfoRequest struct {
	ReqID int
}

func (m UserInfoRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqUserInfo), itoa(m.ReqID)}, nil
}

type UserInfo struct {
	ReqID           int
	WhiteBrandingID string
}

// [109, timeMs] — no version
func decodeCurrentTimeInMillis(r *fieldReader, sv int) ([]Message, error) {
	return []Message{CurrentTimeMillis{TimeMs: r.ReadString()}}, nil
}

// [9, version, orderID]
func decodeNextValidID(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	orderID, err := r.ReadInt64()
	if err != nil {
		return nil, err
	}
	return []Message{NextValidID{OrderID: orderID}}, nil
}

// [49, version, time]
func decodeCurrentTime(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	return []Message{CurrentTime{Time: r.ReadString()}}, nil
}

// Wire fields after the message ID: [reqId, whiteBrandingId] — no version.
func decodeUserInfo(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	whiteBrandingID := r.ReadString()
	return []Message{UserInfo{ReqID: reqID, WhiteBrandingID: whiteBrandingID}}, nil
}
