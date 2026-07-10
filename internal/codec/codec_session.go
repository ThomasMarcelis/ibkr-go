package codec

import (
	"fmt"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

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

func (m ServerInfo) encodeWire(sv int) ([]string, error) {
	return nil, fmt.Errorf("codec: unsupported message type %T", m)
}

type ManagedAccounts struct {
	Accounts []string
}

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
// (OUT 105, server_version >= 197): the bare message id with no version or
// fields, answered by a CurrentTimeMillis frame (IN 109).
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
	return []string{itoa(protocol.OutReqUserInfo), "1", itoa(m.ReqID)}, nil
}

type UserInfo struct {
	ReqID           int
	WhiteBrandingID string
}

// [4, reqId, code, message, advancedJson, errorTimeMs]
func decodeErrMsg(r *fieldReader, sv int) ([]Message, error) {
	if sv < protocol.MinServerVersionErrorTime {
		r.Skip(1) // legacy leading version int (decoder.py:2368-2369)
	}
	reqID, _ := r.ReadInt()
	code, _ := r.ReadInt()
	message := r.ReadString()
	// advancedOrderRejectJson is present from ADVANCED_ORDER_REJECT (166),
	// below the 176 floor, so it is always on the wire.
	advJSON := r.ReadString()
	errTime := ""
	if sv >= protocol.MinServerVersionErrorTime {
		errTime = r.ReadString() // decoder.py:2380-2382
	}
	return []Message{APIError{ReqID: reqID, Code: code, Message: message, AdvancedOrderRejectJSON: advJSON, ErrorTimeMs: errTime}}, nil
}

func (m APIError) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InErrMsg), itoa(m.ReqID), itoa(m.Code), m.Message, m.AdvancedOrderRejectJSON, m.ErrorTimeMs}, nil
}

// [109, timeMs] — no version
func decodeCurrentTimeInMillis(r *fieldReader, sv int) ([]Message, error) {
	return []Message{CurrentTimeMillis{TimeMs: r.ReadString()}}, nil
}

func (m CurrentTimeMillis) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InCurrentTimeInMillis), m.TimeMs}, nil
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

func (m NextValidID) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InNextValidID), "1", i64toa(m.OrderID)}, nil
}

// [15, version, accountsList]
func decodeManagedAccounts(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	raw := r.ReadString()
	accounts := []string{}
	if raw != "" {
		accounts = strings.Split(strings.TrimRight(raw, ","), ",")
	}
	return []Message{ManagedAccounts{Accounts: accounts}}, nil
}

func (m ManagedAccounts) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InManagedAccounts), "1", strings.Join(m.Accounts, ",")}, nil
}

// [49, version, time]
func decodeCurrentTime(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1)
	return []Message{CurrentTime{Time: r.ReadString()}}, nil
}

func (m CurrentTime) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InCurrentTime), "1", m.Time}, nil
}

// Wire fields after the message ID: [reqId, whiteBrandingId] — no version.
func decodeUserInfo(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	whiteBrandingID := r.ReadString()
	return []Message{UserInfo{ReqID: reqID, WhiteBrandingID: whiteBrandingID}}, nil
}

func (m UserInfo) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InUserInfo), itoa(m.ReqID), m.WhiteBrandingID}, nil
}
