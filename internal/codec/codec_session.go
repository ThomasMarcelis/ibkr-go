package codec

import (
	"strings"
)

type StartAPI struct {
	ClientID             int
	OptionalCapabilities string
}

func (StartAPI) messageName() string { return "start_api" }

type ServerInfo struct {
	ServerVersion  int
	ConnectionTime string
}

func (ServerInfo) messageName() string { return "server_info" }

type ManagedAccounts struct {
	Accounts []string
}

func (ManagedAccounts) messageName() string { return "managed_accounts" }

type NextValidID struct {
	OrderID int64
}

func (NextValidID) messageName() string { return "next_valid_id" }

type CurrentTime struct {
	Time string
}

func (CurrentTime) messageName() string { return "current_time" }

// CurrentTimeRequest is the outbound reqCurrentTime message (OUT 49). The
// server responds asynchronously with a CurrentTime frame using the same
// numeric msg_id.
type CurrentTimeRequest struct{}

func (CurrentTimeRequest) messageName() string { return "req_current_time" }

// CurrentTimeMillis is the inbound currentTimeInMillis frame (IN 109): the
// server epoch time in milliseconds, no version field.
type CurrentTimeMillis struct {
	TimeMs string
}

func (CurrentTimeMillis) messageName() string { return "current_time_millis" }

// CurrentTimeMillisRequest is the outbound reqCurrentTimeInMillis message
// (OUT 105, server_version >= 197): the bare message id with no version or
// fields, answered by a CurrentTimeMillis frame (IN 109).
type CurrentTimeMillisRequest struct{}

func (CurrentTimeMillisRequest) messageName() string { return "req_current_time_millis" }

// ReqIDsRequest is the outbound reqIds message (OUT 8). The server responds
// with a NextValidID frame (msg_id 9) carrying the next available order ID.
// NumIDs is a legacy parameter kept at 1 in the official EClient.
type ReqIDsRequest struct {
	NumIDs int
}

func (ReqIDsRequest) messageName() string { return "req_ids" }

type APIError struct {
	ReqID                   int
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
	ErrorTimeMs             string
}

func (APIError) messageName() string { return "api_error" }

type UserInfoRequest struct {
	ReqID int
}

func (UserInfoRequest) messageName() string { return "req_user_info" }

type UserInfo struct {
	ReqID           int
	WhiteBrandingID string
}

func (UserInfo) messageName() string { return "user_info" }

// [4, reqId, code, message, advancedJson, errorTimeMs]
func decodeErrMsg(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	code, _ := r.ReadInt()
	message := r.ReadString()
	advJSON := r.ReadString()
	errTime := r.ReadString()
	return []Message{APIError{ReqID: reqID, Code: code, Message: message, AdvancedOrderRejectJSON: advJSON, ErrorTimeMs: errTime}}, nil
}

// [109, timeMs] — no version
func decodeCurrentTimeInMillis(r *fieldReader) ([]Message, error) {
	return []Message{CurrentTimeMillis{TimeMs: r.ReadString()}}, nil
}

// [9, version, orderID]
func decodeNextValidID(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	orderID, err := r.ReadInt64()
	if err != nil {
		return nil, err
	}
	return []Message{NextValidID{OrderID: orderID}}, nil
}

// [15, version, accountsList]
func decodeManagedAccounts(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	raw := r.ReadString()
	accounts := []string{}
	if raw != "" {
		accounts = strings.Split(strings.TrimRight(raw, ","), ",")
	}
	return []Message{ManagedAccounts{Accounts: accounts}}, nil
}

// [49, version, time]
func decodeCurrentTime(r *fieldReader) ([]Message, error) {
	r.Skip(1)
	return []Message{CurrentTime{Time: r.ReadString()}}, nil
}

// [103, reqId, whiteBrandingId] — no version
func decodeUserInfo(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	whiteBrandingID := r.ReadString()
	return []Message{UserInfo{ReqID: reqID, WhiteBrandingID: whiteBrandingID}}, nil
}
