package codec

import (
	"bytes"
	"encoding/base64"
	"slices"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

func TestCaptureDecode_BondContractDetails(t *testing.T) {
	t.Parallel()

	// captures/20260709T232431Z-contract_details_apple_bonds,
	// server_version=200, events.jsonl sha256 fd71d9f7bf4f470c. These are exact
	// message-18 payloads for the same issuer query, retaining both observed
	// security-ID cardinalities (CUSIP+ISIN and ISIN-only).
	tests := []struct {
		name        string
		payload     string
		conID       int
		cusip       string
		description string
		securityIDs []TagValue
		marketRule  string
		minSize     string
	}{
		{
			name:    "two security ids",
			payload: "MTgAMTAwMQAAQk9ORABJQkNJRDEyNzEyODEzMQAAAAAAAAAwADAAMABBQVBMIDMuODUgMDUvMDQvNDMAU01BUlQAAABBQVBMADEyNzEyODEzMQAwLjAwMDEAQUNUSVZFVElNLEFELEFESlVTVCxBTEVSVCxBTExPQyxBT04sQVZHQ09TVCxCQVNLRVQsQkVOQ0hQWCxDT05ET1JERVIsREFZLERFQUNULERFQUNURElTLERFQUNURU9ELEVWUlVMRSxHQVQsR1RDLEdURCxHVFQsSElELElCS1JBVFMsSU9DLExNVCxNS1QsTk9OQUxHTyxOT05GSVJNUVQsT0NBLE9ERFBPU0NMUyxQQU9OLFJGUSxSVEgsU0NBTEUsU0NBTEVSU1QsV0hBVElGAFNNQVJUAAAAMAAAAFVTL0Vhc3Rlcm4AMjAyNjA3MDg6MjAwMC0yMDI2MDcwOToxNzAwOzIwMjYwNzA5OjIwMDAtMjAyNjA3MTA6MTcwMDsyMDI2MDcxMTpDTE9TRUQ7MjAyNjA3MTI6MjAwMC0yMDI2MDcxMzoxNzAwOzIwMjYwNzEzOjIwMDAtMjAyNjA3MTQ6MTcwMDsyMDI2MDcxNDoyMDAwLTIwMjYwNzE1OjE3MDAAMjAyNjA3MDg6MjAwMC0yMDI2MDcwOToxNzAwOzIwMjYwNzA5OjIwMDAtMjAyNjA3MTA6MTcwMDsyMDI2MDcxMTpDTE9TRUQ7MjAyNjA3MTI6MjAwMC0yMDI2MDcxMzoxNzAwOzIwMjYwNzEzOjIwMDAtMjAyNjA3MTQ6MTcwMDsyMDI2MDcxNDoyMDAwLTIwMjYwNzE1OjE3MDAAAAAyAENVU0lQADAzNzgzM0FMNABJU0lOAFVTMDM3ODMzQUw0MgA3ADEzODYAMgAxADEA",
			conID:   127128131, cusip: "IBCID127128131", description: "AAPL 3.85 05/04/43",
			securityIDs: []TagValue{{Tag: "CUSIP", Value: "037833AL4"}, {Tag: "ISIN", Value: "US037833AL42"}},
			marketRule:  "1386", minSize: "2",
		},
		{
			name:    "one security id",
			payload: "MTgAMTAwMQAAQk9ORABJQkNJRDE5NDE1NDM0MAAAAAAAAAAwADAAMABBQVBMIDEgNS84IDExLzEwLzI2AFNNQVJUAAAAQUFQTAAxOTQxNTQzNDAAMC4wMDAxAEFDVElWRVRJTSxBRCxBREpVU1QsQUxFUlQsQUxMT0MsQU9OLEFWR0NPU1QsQkFTS0VULEJFTkNIUFgsQ09ORE9SREVSLERBWSxERUFDVCxERUFDVERJUyxERUFDVEVPRCxFVlJVTEUsR0FULEdUQyxHVEQsR1RULEhJRCxJQktSQVRTLElPQyxMTVQsTUtULE5PTkFMR08sT0NBLE9ERFBPU0NMUyxQQU9OLFJGUSxSVEgsU0NBTEUsU0NBTEVSU1QsV0hBVElGAFNNQVJUAAAAMAAAAFVTL0Vhc3Rlcm4AMjAyNjA3MDg6MjAwMC0yMDI2MDcwOToxNzAwOzIwMjYwNzA5OjIwMDAtMjAyNjA3MTA6MTcwMDsyMDI2MDcxMTpDTE9TRUQ7MjAyNjA3MTI6MjAwMC0yMDI2MDcxMzoxNzAwOzIwMjYwNzEzOjIwMDAtMjAyNjA3MTQ6MTcwMDsyMDI2MDcxNDoyMDAwLTIwMjYwNzE1OjE3MDAAMjAyNjA3MDg6MjAwMC0yMDI2MDcwOToxNzAwOzIwMjYwNzA5OjIwMDAtMjAyNjA3MTA6MTcwMDsyMDI2MDcxMTpDTE9TRUQ7MjAyNjA3MTI6MjAwMC0yMDI2MDcxMzoxNzAwOzIwMjYwNzEzOjIwMDAtMjAyNjA3MTQ6MTcwMDsyMDI2MDcxNDoyMDAwLTIwMjYwNzE1OjE3MDAAAAAxAElTSU4AWFMxMTM1MzM3NDk4ADcAMTM4OAAxMDAAMQAxAA==",
			conID:   194154340, cusip: "IBCID194154340", description: "AAPL 1 5/8 11/10/26",
			securityIDs: []TagValue{{Tag: "ISIN", Value: "XS1135337498"}},
			marketRule:  "1388", minSize: "100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := base64.StdEncoding.DecodeString(tc.payload)
			if err != nil {
				t.Fatalf("decode captured payload: %v", err)
			}
			msgs, err := DecodeBatch(200, payload)
			if err != nil {
				t.Fatalf("DecodeBatch: %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("got %d messages, want 1", len(msgs))
			}
			m, ok := msgs[0].(BondContractDetails)
			if !ok {
				t.Fatalf("type = %T, want BondContractDetails", msgs[0])
			}
			if m.ReqID != 1001 || m.Contract.ConID != tc.conID || m.Contract.SecType != "BOND" {
				t.Errorf("identity = req %d conID %d secType %q", m.ReqID, m.Contract.ConID, m.Contract.SecType)
			}
			if m.CUSIP != tc.cusip || m.DescriptionAppend != tc.description {
				t.Errorf("bond identity = %q / %q", m.CUSIP, m.DescriptionAppend)
			}
			if m.TimeZoneID != "US/Eastern" || m.MinTick != "0.0001" {
				t.Errorf("market metadata = zone %q minTick %q", m.TimeZoneID, m.MinTick)
			}
			if !slices.Equal(m.SecurityIDs, tc.securityIDs) {
				t.Errorf("SecurityIDs = %+v, want %+v", m.SecurityIDs, tc.securityIDs)
			}
			if m.MarketRuleIDs != tc.marketRule || m.MinSize != tc.minSize || m.SizeIncrement != "1" || m.SuggestedSizeIncrement != "1" {
				t.Errorf("size rules = market %q min %q increment %q suggested %q", m.MarketRuleIDs, m.MinSize, m.SizeIncrement, m.SuggestedSizeIncrement)
			}
		})
	}
}

// Capture-grounded decode tests. Each payload comes from a real IB Gateway
// session. Tests that sanitize captured identifiers or values say so locally;
// payloads described as exact differ only by removal of the frame-length prefix.

func decodeCapturedFrame(t *testing.T, payload string) Message {
	t.Helper()

	messages, err := DecodeBatch(200, []byte(payload))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
	return messages[0]
}

func TestCaptureDecode_ManagedAccounts(t *testing.T) {
	t.Parallel()
	// captures/20260405T214926Z-bootstrap, frame at line 6. Source frame
	// SHA-256 65ef1538878849262af07c2236117ab81b3ea0a9d81ea1a867b1a8cf4e83aedf.
	// The account token is replaced with DU9000001; no other field changes.
	payload := []byte("15\x001\x00DU9000001\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ManagedAccounts)
	if !ok {
		t.Fatalf("type = %T, want ManagedAccounts", msgs[0])
	}
	if len(m.Accounts) != 1 || m.Accounts[0] != "DU9000001" {
		t.Errorf("Accounts = %v, want [DU9000001]", m.Accounts)
	}
}

func TestCaptureDecode_NextValidID(t *testing.T) {
	t.Parallel()
	// captures/20260405T214926Z-bootstrap, first frame in multi-frame chunk at line 7
	payload := []byte("9\x001\x001\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(NextValidID)
	if !ok {
		t.Fatalf("type = %T, want NextValidID", msgs[0])
	}
	if m.OrderID != 1 {
		t.Errorf("OrderID = %d, want 1", m.OrderID)
	}
}

func TestCaptureDecode_CurrentTimeMillis(t *testing.T) {
	t.Parallel()

	// Exact IN 109 frame from the readonly-live server_version 200 capture
	// 20260710T000300Z-current_time_millis. events.jsonl SHA-256:
	// b3ca14922df144e09de0bd58a81bee132c0759d0acf516481cf59b455c2fc54e.
	payload := []byte("109\x001783641780497\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(CurrentTimeMillis)
	if !ok {
		t.Fatalf("type = %T, want CurrentTimeMillis", msgs[0])
	}
	if m.TimeMs != "1783641780497" {
		t.Errorf("TimeMs = %q, want 1783641780497", m.TimeMs)
	}
}

func TestCaptureDecode_CurrentTimeLive(t *testing.T) {
	t.Parallel()

	// Exact IN 49 frame from readonly-live server_version 200 capture
	// captures/20260611T074046Z-current_time. events.jsonl SHA-256 prefix:
	// efe321755946a395.
	msgs, err := DecodeBatch(200, []byte("49\x001\x001781163646\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(CurrentTime)
	if !ok {
		t.Fatalf("type = %T, want CurrentTime", msgs[0])
	}
	if m.Time != "1781163646" {
		t.Errorf("Time = %q, want 1781163646", m.Time)
	}
}

func TestCaptureDecode_FamilyCodesLive(t *testing.T) {
	t.Parallel()

	// Exact IN 78 frame from paper server_version 200 capture
	// captures/v1/family_codes.log, SHA-256
	// af810a715150cca78a18bcb9c37f4546276a7e7d264a7d2ab6a4f0c7da33768c.
	msgs, err := DecodeBatch(200, []byte("78\x001\x00*\x00\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(FamilyCodes)
	if !ok {
		t.Fatalf("type = %T, want FamilyCodes", msgs[0])
	}
	if len(m.Codes) != 1 || m.Codes[0] != (FamilyCodeEntry{AccountID: "*"}) {
		t.Errorf("Codes = %+v, want wildcard account with empty family code", m.Codes)
	}
}

func TestCaptureDecode_NewsProvidersLive(t *testing.T) {
	t.Parallel()

	// Exact IN 85 frame from paper server_version 200 capture
	// captures/v1/news_providers.log, SHA-256
	// 7c54a1a50c60aef5af5de634b7a23cb1b04269dd92d0ea98dda29b451bbc862f.
	payload := []byte("85\x003\x00BRFG\x00Briefing.com General Market Columns\x00" +
		"BRFUPDN\x00Briefing.com Analyst Actions\x00DJNL\x00Dow Jones Newsletters\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(NewsProviders)
	if !ok {
		t.Fatalf("type = %T, want NewsProviders", msgs[0])
	}
	want := []NewsProviderEntry{
		{Code: "BRFG", Name: "Briefing.com General Market Columns"},
		{Code: "BRFUPDN", Name: "Briefing.com Analyst Actions"},
		{Code: "DJNL", Name: "Dow Jones Newsletters"},
	}
	if !slices.Equal(m.Providers, want) {
		t.Errorf("Providers = %+v, want %+v", m.Providers, want)
	}
}

func TestCaptureDecode_PositionMultiEndLive(t *testing.T) {
	t.Parallel()

	// Exact IN 72 frame from paper server_version 200 capture
	// captures/v1/positions_multi.log, SHA-256
	// 083e158d5335c7326d23117e842c479fcecf0e6786172f003d479449dbb304b8.
	msgs, err := DecodeBatch(200, []byte("72\x001\x001001\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(PositionMultiEnd)
	if !ok {
		t.Fatalf("type = %T, want PositionMultiEnd", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
}

func TestCaptureDecode_PositionMultiServerVersion206(t *testing.T) {
	t.Parallel()

	// Exact server_version 206 row from capture
	// 20260710T224440Z-positions_multi, events sha256
	// 3d62b420e3a600a0ad1c420e7a805cc6adab8bab1ecd6ff1e0959ead167d6d29.
	// Only the account was replaced with a short sanitization token.
	payload := append([]byte{0, 0, 0, protocol.InPositionMulti}, []byte("1\x001\x00U1\x0045602025\x00MELI\x00STK\x00\x000.0\x00\x00\x00NASDAQ\x00USD\x00MELI\x00NMS\x001\x001581.09\x00\x00")...)
	msgs, err := DecodeBatch(206, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(PositionMulti)
	if !ok {
		t.Fatalf("type = %T, want PositionMulti", msgs[0])
	}
	if m.ReqID != 1 || m.Account != "U1" || m.ModelCode != "" {
		t.Fatalf("identity = reqID %d account %q model %q", m.ReqID, m.Account, m.ModelCode)
	}
	if m.Contract.ConID != 45602025 || m.Contract.Symbol != "MELI" || m.Contract.TradingClass != "NMS" {
		t.Fatalf("contract = %+v", m.Contract)
	}
	if m.Position != "1" || m.AvgCost != "1581.09" {
		t.Fatalf("position = %q avgCost = %q", m.Position, m.AvgCost)
	}
}

func TestCaptureDecode_PnLLive(t *testing.T) {
	t.Parallel()

	// Exact first IN 94 update from paper server_version 200 capture
	// captures/20260611T134246Z-api_algorithmic_campaign_aapl. events.jsonl
	// SHA-256: 2e3781761e68dbe5bea8ac9fe62c38b6657bcad56bdceb949b24881c27f3d0bb.
	payload := []byte("94\x004\x0011340.427636781911\x0054385.58271885987\x00-103.92738339177643\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(PnLValue)
	if !ok {
		t.Fatalf("type = %T, want PnLValue", msgs[0])
	}
	if m.ReqID != 4 || m.DailyPnL != "11340.427636781911" ||
		m.UnrealizedPnL != "54385.58271885987" || m.RealizedPnL != "-103.92738339177643" {
		t.Errorf("PnLValue = %+v, want exact captured update", m)
	}
}

func TestCaptureDecode_OrderStatusLive(t *testing.T) {
	t.Parallel()

	// Exact IN 3 payload from captures/20260405T215248Z-open_orders_all,
	// events.jsonl line 10 frame 2. Frame SHA-256:
	// 16859f1335e8091096cc7bf459e217281090f4f4276c0883af4d93a055eb608d.
	// The perm ID is replaced with the repository's 9000 convention; no other
	// field changes.
	message := decodeCapturedFrame(t, "3\x000\x00PreSubmitted\x000\x001\x000\x009000\x000\x000\x000\x00\x000\x00")
	status, ok := message.(OrderStatus)
	if !ok {
		t.Fatalf("type = %T, want OrderStatus", message)
	}
	if status.OrderID != 0 || status.Status != "PreSubmitted" || status.Filled != "0" ||
		status.Remaining != "1" || status.PermID != "9000" || status.ClientID != "0" {
		t.Fatalf("OrderStatus = %+v, want captured pre-submitted state", status)
	}
}

func TestCaptureDecode_AccountUpdatesLive(t *testing.T) {
	t.Parallel()

	// IN 6, 8, and 54 are exact field sequences from captures/v1/account_updates.log
	// (SHA-256 a211d7abf123f4812e6b76fe359b82d560f9130a6e96c914bbf1578a79adbd8e,
	// lines 7, 128, and 129). IN 7 is the exact frame from
	// captures/20260611T134246Z-api_algorithmic_campaign_aapl/events.jsonl line 30,
	// frame SHA-256 8cf56859c14d031629e38e972b807bcebf11735a54862a3a7f61b6f4354aff4d.
	// Account tokens are replaced with DU9000001; no other field changes.
	valueMessage := decodeCapturedFrame(t, "6\x002\x00AccountCode\x00DU9000001\x00\x00DU9000001\x00")
	value, ok := valueMessage.(UpdateAccountValue)
	if !ok {
		t.Fatalf("account value type = %T, want UpdateAccountValue", valueMessage)
	}
	if value != (UpdateAccountValue{Key: "AccountCode", Value: "DU9000001", Account: "DU9000001"}) {
		t.Fatalf("UpdateAccountValue = %+v", value)
	}

	portfolioMessage := decodeCapturedFrame(t, "7\x008\x00265598\x00AAPL\x00STK\x00\x000\x000\x00\x00NASDAQ\x00USD\x00AAPL\x00NMS\x00-100\x00291.6234436\x00-29162.34\x00291.2660812\x00-35.74\x00-71.44\x00DU9000001\x00")
	portfolio, ok := portfolioMessage.(UpdatePortfolio)
	if !ok {
		t.Fatalf("portfolio type = %T, want UpdatePortfolio", portfolioMessage)
	}
	if portfolio.Contract.ConID != 265598 || portfolio.Contract.Symbol != "AAPL" ||
		portfolio.Contract.PrimaryExchange != "NASDAQ" || portfolio.Position != "-100" ||
		portfolio.MarketValue != "-29162.34" || portfolio.RealizedPNL != "-71.44" ||
		portfolio.Account != "DU9000001" {
		t.Fatalf("UpdatePortfolio = %+v", portfolio)
	}

	timeMessage := decodeCapturedFrame(t, "8\x001\x0014:34\x00")
	if update, ok := timeMessage.(UpdateAccountTime); !ok || update.Timestamp != "14:34" {
		t.Fatalf("UpdateAccountTime = %#v", timeMessage)
	}

	endMessage := decodeCapturedFrame(t, "54\x001\x00DU9000001\x00")
	if end, ok := endMessage.(AccountDownloadEnd); !ok || end.Account != "DU9000001" {
		t.Fatalf("AccountDownloadEnd = %#v", endMessage)
	}
}

func TestCaptureDecode_DisplayGroupsLive(t *testing.T) {
	t.Parallel()

	// Exact payloads from captures/20260407T183142Z-display_groups/events.jsonl
	// line 9 and 20260407T183153Z-display_group_subscribe/events.jsonl line 13.
	// Frame SHA-256 values: caa4fc39b0281deccb78ae826e46bab8c461090477583970e74fd816792d0428
	// and 907f762a60a8869065a58bfe8ee0c0dd8165f69a99b2f5d5291e3373a6fd135b.
	listMessage := decodeCapturedFrame(t, "67\x001\x001001\x001|2|3|4|5|6|7\x00")
	if list, ok := listMessage.(DisplayGroupList); !ok || list.ReqID != 1001 || list.Groups != "1|2|3|4|5|6|7" {
		t.Fatalf("DisplayGroupList = %#v", listMessage)
	}

	updateMessage := decodeCapturedFrame(t, "68\x001\x001002\x00none\x00")
	if update, ok := updateMessage.(DisplayGroupUpdated); !ok || update.ReqID != 1002 || update.ContractInfo != "none" {
		t.Fatalf("DisplayGroupUpdated = %#v", updateMessage)
	}
}

func TestCaptureDecode_AccountUpdatesMultiLive(t *testing.T) {
	t.Parallel()

	// Exact IN 73 and IN 74 field sequences from
	// captures/v1/account_updates_multi.log (SHA-256
	// dad696b11bbc111469a20474a0f6b3beec033390320cfe89b4519c344c843547),
	// lines 7 and 57. The account token is replaced with DU9000001; no other
	// field changes.
	valueMessage := decodeCapturedFrame(t, "73\x001\x001001\x00DU9000001\x00\x00Currency\x00EUR\x00EUR\x00")
	value, ok := valueMessage.(AccountUpdateMultiValue)
	if !ok {
		t.Fatalf("value type = %T, want AccountUpdateMultiValue", valueMessage)
	}
	if value != (AccountUpdateMultiValue{ReqID: 1001, Account: "DU9000001", Key: "Currency", Value: "EUR", Currency: "EUR"}) {
		t.Fatalf("AccountUpdateMultiValue = %+v", value)
	}

	endMessage := decodeCapturedFrame(t, "74\x001\x001001\x00")
	if end, ok := endMessage.(AccountUpdateMultiEnd); !ok || end.ReqID != 1001 {
		t.Fatalf("AccountUpdateMultiEnd = %#v", endMessage)
	}
}

func TestCaptureDecode_SecDefOptParamsEndLive(t *testing.T) {
	t.Parallel()

	// Exact IN 76 payload from captures/20260611T074859Z-api_option_campaign_aapl,
	// events.jsonl line 30 frame 4. Frame SHA-256:
	// f926ce02dcb8febcf1b02d69ac9310779782836c6c0307aec8079293d9cf48da.
	message := decodeCapturedFrame(t, "76\x003\x00")
	if end, ok := message.(SecDefOptParamsEnd); !ok || end.ReqID != 3 {
		t.Fatalf("SecDefOptParamsEnd = %#v", message)
	}
}

func TestCaptureDecode_SoftDollarTiersLive(t *testing.T) {
	t.Parallel()

	// Exact zero-tier IN 77 payload from
	// captures/20260407T180546Z-soft_dollar_tiers/events.jsonl line 8.
	// Frame SHA-256: 8deacd34fd4d533ac8bc17b83a3928af5b31f06ca4b24a0fae2de4bfb2b2ebe0.
	message := decodeCapturedFrame(t, "77\x001001\x000\x00")
	tiers, ok := message.(SoftDollarTiersResponse)
	if !ok {
		t.Fatalf("type = %T, want SoftDollarTiersResponse", message)
	}
	if tiers.ReqID != 1001 || len(tiers.Tiers) != 0 {
		t.Fatalf("SoftDollarTiersResponse = %+v", tiers)
	}
}

func TestCaptureDecode_HistoricalNewsFlowLive(t *testing.T) {
	t.Parallel()

	// Exact compact IN 83, 86, and 87 payloads from
	// captures/20260415T162244Z-api_news_article_aapl/events.jsonl (SHA-256
	// 3c6ef62da8d60e95ed8f05418ca218268d652fb6fc99bf1e6e90d7dcad20c8e3),
	// lines 13, 10, and 11. Frame SHA-256 values are, respectively,
	// cb8c015f1486ef5916bc66c9bfe0f56b8d5e2e54ca900d814ba9179abeac3a34,
	// a2028f2723d90537cf17d22bba39fc92615fd6b4e42c61938b2270b95b903c86,
	// and 22d4e8d3e04557b7288bfdfdf7bee0239f8e2e33ce530111deb190edbc00aa92.
	articleMessage := decodeCapturedFrame(t, "83\x002\x000\x00BofA Securities reiterated Apple (AAPL) coverage with Buy rating and price target $325&#10;Previous price target: $320&#10;Issuance Date: 2026-04-14&#10;&#10;Copyright 2026 Briefing.com, Inc.\x00")
	article, ok := articleMessage.(NewsArticleResponse)
	if !ok {
		t.Fatalf("article type = %T, want NewsArticleResponse", articleMessage)
	}
	if article.ReqID != 2 || article.ArticleType != 0 ||
		!strings.Contains(article.ArticleText, "price target $325") ||
		!strings.Contains(article.ArticleText, "Copyright 2026 Briefing.com") {
		t.Fatalf("NewsArticleResponse = %+v", article)
	}

	itemMessage := decodeCapturedFrame(t, "86\x001\x002026-04-14 14:58:42.0\x00BRFUPDN\x00BRFUPDN$1e1f54ec\x00{A:800015:L:en}!BofA Securities reiterated Apple (AAPL) coverage with Buy and target $325\x00")
	item, ok := itemMessage.(HistoricalNewsItem)
	if !ok {
		t.Fatalf("item type = %T, want HistoricalNewsItem", itemMessage)
	}
	if item.ReqID != 1 || item.Time != "2026-04-14 14:58:42.0" ||
		item.ProviderCode != "BRFUPDN" || item.ArticleID != "BRFUPDN$1e1f54ec" ||
		item.Headline != "{A:800015:L:en}!BofA Securities reiterated Apple (AAPL) coverage with Buy and target $325" {
		t.Fatalf("HistoricalNewsItem = %+v", item)
	}

	endMessage := decodeCapturedFrame(t, "87\x001\x001\x00")
	if end, ok := endMessage.(HistoricalNewsEnd); !ok || end.ReqID != 1 || !end.HasMore {
		t.Fatalf("HistoricalNewsEnd = %#v", endMessage)
	}
}

func TestCaptureDecode_HeadTimestampLive(t *testing.T) {
	t.Parallel()

	// Exact IN 88 field sequence from captures/v1/head_timestamp_aapl.log
	// (SHA-256 f6a1a3fb3092f0cc7b96fab359b6dc01eefe12270e2fcc9d5d60a24bc0c253b6),
	// line 7. Reconstructed frame SHA-256:
	// 9a25cedcc074916607c1ad583de9a1a6a9f46522d89433a95804691f2b6c9ec0.
	message := decodeCapturedFrame(t, "88\x001001\x0019801212-14:30:00\x00")
	if head, ok := message.(HeadTimestamp); !ok || head.ReqID != 1001 || head.Timestamp != "19801212-14:30:00" {
		t.Fatalf("HeadTimestamp = %#v", message)
	}
}

func TestCaptureDecode_CompletedOrderEndLive(t *testing.T) {
	t.Parallel()

	// Exact bare IN 102 payload from
	// captures/20260415T162637Z-api_completed_orders_variants_aapl/events.jsonl
	// line 27 frame 11. Frame SHA-256:
	// 2ccb531bffd651a1e09825677ff8850d6b1e2377ee7952ead4ff0f44436e4b46.
	message := decodeCapturedFrame(t, "102\x00")
	if _, ok := message.(CompletedOrderEnd); !ok {
		t.Fatalf("type = %T, want CompletedOrderEnd", message)
	}
}

func TestCaptureDecode_NewsBulletinLive(t *testing.T) {
	t.Parallel()

	// Exact second IN 14 frame from readonly-live server_version 200 capture
	// 20260710T133034Z-news_bulletins. raw.txt SHA-256:
	// 5bdf4ea73165335b485a96f5a694e92583e06f785039d6553b2e2b8f7c5b5e4e.
	framed, err := base64.StdEncoding.DecodeString("AAABhzE0ADEAMTc4MzU3MDExOAAxAD09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PQpUbyBIS0ZFLFNHWCB0cmFkZXJzOgpUaHUgMDkgSnVsIDIwMjYgMTA6Mzg6MTkgUE0gRURUClBsZWFzZSBub3RlIHRoYXQgd2hpbGUgdGhlIG9uc2hvcmUgVGFpd2FuZXNlIG1hcmtldHMgYXJlIGNsb3NlZCB0b2RheSBkdWUgdG8gdGhlIHR5cGhvb24sIFNHWCBGVFNFIFRhaXdhbiAoIlRXTiIpIGFuZCBNaWNybyBGVFNFIFRhaXdhbiBGdXR1cmVzICgiTVRXTiIpIGFzIHdlbGwgYXMgSEtGRSBNU0NJIFRhaXdhbiAoIk1UVyIpIHdpbGwgY29udGludWUgdG8gdHJhZGUgYWNjb3JkaW5nIHRvIHRoZWlyIG5vcm1hbCB0cmFkaW5nIHNjaGVkdWxlLgBIS0ZFLFNHWAA=")
	if err != nil {
		t.Fatalf("decode captured frame: %v", err)
	}
	msgs, err := DecodeBatch(200, framed[4:])
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(NewsBulletin)
	if !ok {
		t.Fatalf("type = %T, want NewsBulletin", msgs[0])
	}
	if m.MsgID != 1783570118 || m.MsgType != 1 || m.Source != "HKFE,SGX" ||
		!strings.Contains(m.Headline, "continue to trade according to their normal trading schedule") {
		t.Errorf("NewsBulletin = %+v, want captured HKFE/SGX notice", m)
	}
}

func TestCaptureDecode_APIError_2104(t *testing.T) {
	t.Parallel()
	// captures/20260405T214926Z-bootstrap, second frame in multi-frame chunk at line 7
	// APIError code 2104: "Market data farm connection is OK:usfarm"
	payload := []byte("4\x00-1\x002104\x00Market data farm connection is OK:usfarm\x00\x001775425766350\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(APIError)
	if !ok {
		t.Fatalf("type = %T, want APIError", msgs[0])
	}
	if m.ReqID != -1 {
		t.Errorf("ReqID = %d, want -1", m.ReqID)
	}
	if m.Code != 2104 {
		t.Errorf("Code = %d, want 2104", m.Code)
	}
	if !strings.Contains(m.Message, "usfarm") {
		t.Errorf("Message = %q, want substring 'usfarm'", m.Message)
	}
}

func TestCaptureDecode_ContractDetails(t *testing.T) {
	t.Parallel()
	// captures/20260405T214938Z-contract_details_aapl_stk, line 10 (1171-byte frame)
	// Full AAPL STK contract details from live gateway.
	payload := []byte(
		"10\x001001\x00AAPL\x00STK\x00\x00\x000\x00\x00SMART\x00USD\x00AAPL\x00NMS\x00NMS\x00" +
			"265598\x000.01\x00\x00" +
			"ACTIVETIM,AD,ADDONT,ADJUST,ALERT,ALGO,ALLOC,AON,AVGCOST,BASKET,BENCHPX," +
			"CASHQTY,COND,CONDORDER,DARKONLY,DARKPOLL,DAY,DEACT,DEACTDIS,DEACTEOD,DIS," +
			"DUR,GAT,GTC,GTD,GTT,HID,IBKRATS,ICE,IMB,IOC,LIT,LMT,LOC,MIDPX,MIT,MKT," +
			"MOC,MTL,NGCOMB,NODARK,NONALGO,OCA,OPG,OPGREROUT,PEGBENCH,PEGMID,POSTATS," +
			"POSTONLY,PREOPGRTH,PRICECHK,REL,REL2MID,RELPCTOFS,RPI,RTH,SCALE,SCALEODD," +
			"SCALERST,SIZECHK,SNAPMID,SNAPMKT,SNAPREL,STP,STPLMT,SWEEP,TRAIL,TRAILLIT," +
			"TRAILLMT,TRAILMIT,WHATIF\x00" +
			"SMART,AMEX,NYSE,CBOE,PHLX,ISE,CHX,ARCA,NASDAQ,DRCTEDGE,BEX,BATS,EDGEA," +
			"BYX,IEX,EDGX,FOXRIVER,PEARL,NYSENAT,LTSE,MEMX,IBEOS,OVERNIGHT,TPLUS0," +
			"PSX,T24X\x00" +
			"1\x000\x00APPLE INC\x00NASDAQ\x00\x00Technology\x00Computers\x00Computers\x00US/Eastern\x00" +
			"20260405:CLOSED;20260406:0400-20260406:2000;20260407:0400-20260407:2000;" +
			"20260408:0400-20260408:2000;20260409:0400-20260409:2000;20260410:0400-20260410:2000\x00" +
			"20260405:CLOSED;20260406:0930-20260406:1600;20260407:0930-20260407:1600;" +
			"20260408:0930-20260408:1600;20260409:0930-20260409:1600;20260410:0930-20260410:1600\x00" +
			"\x00\x001\x00ISIN\x00US0378331005\x001\x00\x00\x00" +
			"26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26,26\x00" +
			"\x00COMMON\x000.0001\x000.0001\x00100\x000\x00")

	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ContractDetails)
	if !ok {
		t.Fatalf("type = %T, want ContractDetails", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.Contract.ConID != 265598 {
		t.Errorf("ConID = %d, want 265598", m.Contract.ConID)
	}
	if m.Contract.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", m.Contract.Symbol)
	}
	if m.Contract.SecType != "STK" {
		t.Errorf("SecType = %q, want STK", m.Contract.SecType)
	}
	if m.LongName != "APPLE INC" {
		t.Errorf("LongName = %q, want APPLE INC", m.LongName)
	}
	if m.Contract.PrimaryExchange != "NASDAQ" {
		t.Errorf("PrimaryExchange = %q, want NASDAQ", m.Contract.PrimaryExchange)
	}
	if m.Contract.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", m.Contract.Currency)
	}
	if m.MinTick != "0.01" {
		t.Errorf("MinTick = %q, want 0.01", m.MinTick)
	}
	if m.TimeZoneID != "US/Eastern" {
		t.Errorf("TimeZoneID = %q, want US/Eastern", m.TimeZoneID)
	}
	if m.PriceMagnifier != 1 || m.UnderConID != 0 || m.ContractMonth != "" {
		t.Errorf("numeric/reference metadata = magnifier %d underConID %d month %q", m.PriceMagnifier, m.UnderConID, m.ContractMonth)
	}
	if m.OrderTypes == "" || m.ValidExchanges == "" || m.MarketRuleIDs == "" {
		t.Errorf("capability metadata missing: orderTypes=%t exchanges=%t rules=%t", m.OrderTypes != "", m.ValidExchanges != "", m.MarketRuleIDs != "")
	}
	if m.Industry != "Technology" || m.Category != "Computers" || m.Subcategory != "Computers" {
		t.Errorf("classification = %q/%q/%q", m.Industry, m.Category, m.Subcategory)
	}
	if m.TradingHours == "" || m.LiquidHours == "" {
		t.Errorf("hours missing: trading=%t liquid=%t", m.TradingHours != "", m.LiquidHours != "")
	}
	if len(m.SecurityIDs) != 1 || m.SecurityIDs[0] != (TagValue{Tag: "ISIN", Value: "US0378331005"}) {
		t.Errorf("SecurityIDs = %#v, want live ISIN", m.SecurityIDs)
	}
	if m.AggGroup != 1 || m.StockType != "COMMON" || m.MinSize != "0.0001" || m.SizeIncrement != "0.0001" || m.SuggestedSizeIncrement != "100" {
		t.Errorf("tail = agg %d stock %q sizes %q/%q/%q", m.AggGroup, m.StockType, m.MinSize, m.SizeIncrement, m.SuggestedSizeIncrement)
	}
	if m.Fund != nil || len(m.IneligibilityReasons) != 0 {
		t.Errorf("stock optional facets = fund %#v ineligibility %#v", m.Fund, m.IneligibilityReasons)
	}
}

func TestCaptureDecode_ContractDetailsFund(t *testing.T) {
	t.Parallel()

	// captures/20260415T150322Z-api_security_type_probe_matrix, server_version=200,
	// request 11. events.jsonl SHA-256 9be83e57ed176a17ec0c31a87e2c19220685e91b5d817dfebc88d8317cca024e.
	hours := "20260415:1559-20260415:2200;20260416:1559-20260416:2200;20260417:1559-20260417:2200;20260418:CLOSED;20260419:CLOSED;20260420:1559-20260420:2200"
	fields := []string{
		"10", "11", "VTSAX", "FUND", "", "", "0", "", "FUNDSERV", "USD",
		"922908728", "VTSAX", "922908728", "48013650", "0.01", "",
		"AD,ALERT,ALLOC,BASKET,DAY,DEACT,DEACTDIS,FUNDSWAP,MKT,NONALGO,WHATIF", "FUNDSERV", "1", "0",
		"Vanguard Total Stock Market Index Fund A (Vanguard)", "", "", "", "", "", "US/Eastern",
		hours, hours, "", "", "1", "ISIN", "US9229087286", "2147483647", "", "", "2963", "", "",
		"0.001", "0.001", "1",
		"Vanguard Total Stock Market Index Fund A", "Vanguard", "", "0", "0", "0", "0.04",
		"0", "0", "0", "10000000", "3000", "1", "All", "ARE,ASM,FSM,GUM,MHL,MNP,PLW,PRI,VIR", "", "", "0",
	}
	msgs, err := DecodeBatch(200, []byte(strings.Join(fields, "\x00")+"\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ContractDetails)
	if !ok {
		t.Fatalf("type = %T, want ContractDetails", msgs[0])
	}
	if m.Contract.Symbol != "VTSAX" || m.Contract.SecType != "FUND" || m.Contract.ConID != 48013650 {
		t.Fatalf("contract = %+v, want live VTSAX FUND", m.Contract)
	}
	if m.Fund == nil {
		t.Fatal("Fund = nil, want live mutual-fund metadata")
	}
	if m.Fund.Name != "Vanguard Total Stock Market Index Fund A" || m.Fund.Family != "Vanguard" || m.Fund.ManagementFee != "0.04" {
		t.Errorf("fund identity/fee = %q/%q/%q", m.Fund.Name, m.Fund.Family, m.Fund.ManagementFee)
	}
	if m.Fund.MinimumInitialPurchase != "3000" || m.Fund.MinimumSubsequentPurchase != "1" || m.Fund.BlueSkyStates != "All" {
		t.Errorf("fund purchase metadata = %q/%q/%q", m.Fund.MinimumInitialPurchase, m.Fund.MinimumSubsequentPurchase, m.Fund.BlueSkyStates)
	}
	if m.MinSize != "0.001" || m.SizeIncrement != "0.001" || m.SuggestedSizeIncrement != "1" {
		t.Errorf("size rules = %q/%q/%q", m.MinSize, m.SizeIncrement, m.SuggestedSizeIncrement)
	}
}

func TestCaptureDecode_ExecutionDetailNativeTime(t *testing.T) {
	t.Parallel()
	// captures/20260413T192703Z-place_order_mkt_buy_aapl, server_version=200,
	// events.jsonl sha256 prefix 301b075b217cbd99. The contract block includes
	// currency between exchange and localSymbol; dropping that field misaligns
	// execID/time/side. Account, exec, and perm identifiers are sanitized.
	payload := []byte("11\x00-1\x001\x00265598\x00AAPL\x00STK\x00\x000.0\x00\x00\x00IEX\x00USD\x00AAPL\x00NMS\x00sanitized-native-exec-001\x0020260413 15:27:04 US/Eastern\x00DU9000001\x00IEX\x00BOT\x001\x00257.95\x00900001\x0094\x000\x001\x00257.95\x00\x00\x00\x00\x002\x000\x00\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ExecutionDetail)
	if !ok {
		t.Fatalf("type = %T, want ExecutionDetail", msgs[0])
	}
	if m.OrderID != 1 {
		t.Errorf("OrderID = %d, want 1", m.OrderID)
	}
	if m.Contract.ConID != 265598 || m.Contract.Symbol != "AAPL" ||
		m.Contract.SecType != "STK" || m.Contract.Exchange != "IEX" ||
		m.Contract.Currency != "USD" || m.Contract.LocalSymbol != "AAPL" ||
		m.Contract.TradingClass != "NMS" {
		t.Errorf("Contract = %+v", m.Contract)
	}
	if m.ExecID != "sanitized-native-exec-001" {
		t.Errorf("ExecID = %q", m.ExecID)
	}
	if m.Time != "20260413 15:27:04 US/Eastern" {
		t.Errorf("Time = %q", m.Time)
	}
	if m.Account != "DU9000001" {
		t.Errorf("Account = %q", m.Account)
	}
	if m.Exchange != "IEX" {
		t.Errorf("Exchange = %q", m.Exchange)
	}
	if m.Side != "BOT" {
		t.Errorf("Side = %q", m.Side)
	}
	if m.Shares != "1" {
		t.Errorf("Shares = %q", m.Shares)
	}
	if m.Price != "257.95" {
		t.Errorf("Price = %q", m.Price)
	}
	if m.PermID != "900001" || m.ClientID != "94" || m.Liquidation != "0" {
		t.Errorf("identity/liquidation = %q/%q/%q", m.PermID, m.ClientID, m.Liquidation)
	}
	if m.CumulativeQuantity != "1" || m.AveragePrice != "257.95" {
		t.Errorf("cumulative/average = %q/%q", m.CumulativeQuantity, m.AveragePrice)
	}
	if m.LastLiquidity != "2" || m.PendingPriceRevision != "0" || m.Submitter != "" {
		t.Errorf("liquidity/revision/submitter = %q/%q/%q", m.LastLiquidity, m.PendingPriceRevision, m.Submitter)
	}
}

func TestCaptureDecode_CommissionAndFeesLive(t *testing.T) {
	t.Parallel()
	// captures/20260611T133024Z-api_order_fill_aapl/replay/frames.jsonl:154,
	// server_version=200, events sha256 4fa597ba1c4bc3690a076f44fe5b124f03c6849a580dc6e4edb7e872fc32c0a0,
	// replay sha256 9a7507af48ee7d05a0aafee917372d559ae9a63c5abf6d3c891d553bbb893f3c.
	// ExecID is sanitized; transformed payload sha256 is
	// dbbb32d8e0d45d6e6e7846e81799e64d86f0be06ce9d633a39e6b115faf65f23.
	payload := []byte("59\x001\x00sanitized-commission-exec-001\x001.0003\x00USD\x00-68.051912\x001.7976931348623157E308\x00\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(CommissionReport)
	if !ok {
		t.Fatalf("type = %T, want CommissionReport", msgs[0])
	}
	if m.ExecID != "sanitized-commission-exec-001" || m.Commission != "1.0003" ||
		m.Currency != "USD" || m.RealizedPNL != "-68.051912" ||
		m.Yield != "1.7976931348623157E308" || m.YieldRedemptionDate != "" {
		t.Fatalf("report = %+v", m)
	}
}

func TestCaptureDecode_ExecutionsEndLive(t *testing.T) {
	t.Parallel()
	// Same live capture as TestCaptureDecode_CommissionAndFeesLive,
	// replay/frames.jsonl:166. The request ID is the capture's reqExecutions ID.
	msgs, err := DecodeBatch(200, []byte("55\x001\x003\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 || msgs[0] != (ExecutionsEnd{ReqID: 3}) {
		t.Fatalf("messages = %#v, want ExecutionsEnd{ReqID:3}", msgs)
	}
}

func TestCaptureDecode_CompletedOrderTrailLimitLive(t *testing.T) {
	t.Parallel()

	// captures/20260415T162637Z-api_completed_orders_variants_aapl,
	// server_version=200. Source frame SHA-256
	// 2c0843f3b7863f358ef397707d2f89a9d401d9c40b767a66d1fbd69a8d2d848f.
	// The account is replaced with DU9000001 and the submitter with paper-user;
	// no other field changes.
	// This live TRAIL LIMIT completed-order shape includes a decimal field
	// before the completed-order tail; treating every advanced section as a
	// count field interrupts the completed-orders request.
	fields := []string{
		"101",
		"265598",
		"AAPL",
		"STK",
		"",
		"0",
		"?",
		"",
		"SMART",
		"USD",
		"AAPL",
		"NMS",
		"BUY",
		"1",
		"TRAIL LIMIT",
		"2000.05",
		"1.0",
		"DAY",
		"",
		"DU9000001",
		"",
		"0",
		"",
		"1426085924",
		"0",
		"0",
		"0",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"0",
		"",
		"-1",
		"",
		"",
		"",
		"",
		"",
		"2147483647",
		"0",
		"0",
		"",
		"3",
		"0",
		"",
		"0",
		"None",
		"",
		"0",
		"0",
		"0",
		"",
		"0",
		"0",
		"2000.0",
		"",
		"",
		"0",
		"0",
		"0",
		"2147483647",
		"2147483647",
		"",
		"",
		"",
		"IB",
		"0",
		"0",
		"",
		"0",
		"Cancelled",
		"0",
		"0",
		"0",
		"2000.0",
		"0.05",
		"0",
		"1",
		"0",
		"",
		"0",
		"2147483647",
		"0",
		"Not an insider or substantial shareholder",
		"0",
		"0",
		"9223372036854775807",
		"20260415 11:00:11 US/Eastern",
		"Cancelled by Trader",
		"",
		"",
		"",
		"",
		"",
		"",
		"0",
		"paper-user",
	}
	msgs, err := DecodeBatch(200, []byte(strings.Join(fields, "\x00")+"\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(CompletedOrder)
	if !ok {
		t.Fatalf("type = %T, want CompletedOrder", msgs[0])
	}
	if m.Contract.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", m.Contract.Symbol)
	}
	if m.OrderType != "TRAIL LIMIT" {
		t.Errorf("OrderType = %q, want TRAIL LIMIT", m.OrderType)
	}
	if m.Status != "Cancelled" {
		t.Errorf("Status = %q, want Cancelled", m.Status)
	}
	if m.Quantity != "1" {
		t.Errorf("Quantity = %q, want 1", m.Quantity)
	}
	if m.Filled != "0" {
		t.Errorf("Filled = %q, want 0", m.Filled)
	}
	if m.PermID != "1426085924" {
		t.Errorf("PermID = %q, want 1426085924", m.PermID)
	}
	if m.LmtPrice != "2000.05" || m.AuxPrice != "1.0" {
		t.Errorf("prices = (%q, %q), want (2000.05, 1.0)", m.LmtPrice, m.AuxPrice)
	}
	if m.TrailStopPrice != "2000.0" || m.StopPrice != "2000.0" || m.LmtPriceOffset != "0.05" {
		t.Errorf("trailing fields = (%q, %q, %q), want (2000.0, 2000.0, 0.05)",
			m.TrailStopPrice, m.StopPrice, m.LmtPriceOffset)
	}
	if m.CompletedTime != "20260415 11:00:11 US/Eastern" || m.CompletedStatus != "Cancelled by Trader" {
		t.Errorf("completion = (%q, %q)", m.CompletedTime, m.CompletedStatus)
	}
	if m.Shareholder != "Not an insider or substantial shareholder" || m.Submitter != "paper-user" {
		t.Errorf("compliance = (%q, %q)", m.Shareholder, m.Submitter)
	}
}

func TestCaptureDecode_ContractDetailsEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T214938Z-contract_details_aapl_stk, line 11
	payload := []byte("52\x001\x001001\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(ContractDetailsEnd)
	if !ok {
		t.Fatalf("type = %T, want ContractDetailsEnd", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
}

func TestCaptureDecode_AccountSummaryValue(t *testing.T) {
	t.Parallel()
	// captures/20260405T215025Z-account_summary_snapshot, line 10 (first frame).
	// Source frame SHA-256
	// 1a1fb0cc2a4c8d8d74641ae11391013bebfe47ed3859d7e2abf5aa933681307b.
	// The account is replaced with DU9000001 and the sensitive account value
	// with 300000.00; the message ID, version, request ID, tag, and currency
	// retain the live field sequence.
	payload := []byte("63\x001\x001001\x00DU9000001\x00BuyingPower\x00300000.00\x00EUR\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(AccountSummaryValue)
	if !ok {
		t.Fatalf("type = %T, want AccountSummaryValue", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", m.Account)
	}
	if m.Tag != "BuyingPower" {
		t.Errorf("Tag = %q, want BuyingPower", m.Tag)
	}
	if m.Value != "300000.00" {
		t.Errorf("Value = %q, want 300000.00", m.Value)
	}
	if m.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", m.Currency)
	}
}

func TestCaptureDecode_AccountSummaryEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T215025Z-account_summary_snapshot, last frame in multi-frame chunk at line 11
	payload := []byte("64\x001\x001001\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(AccountSummaryEnd)
	if !ok {
		t.Fatalf("type = %T, want AccountSummaryEnd", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
}

func TestCaptureDecode_Position(t *testing.T) {
	t.Parallel()
	// captures/20260405T215052Z-positions_snapshot, first AMZN position frame
	// at line 10. Source frame SHA-256
	// 2a8c9491176a1d1293a0be0e71e77feda2c8192b1ad844cda94883cf37b3c058.
	// The account is replaced with DU9000001 and the sensitive average cost
	// with 200.25; no structural field changes are made.
	payload := []byte("61\x003\x00DU9000001\x003691937\x00AMZN\x00STK\x00\x000.0\x00\x00\x00NASDAQ\x00USD\x00AMZN\x00NMS\x0015\x00200.25\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(Position)
	if !ok {
		t.Fatalf("type = %T, want Position", msgs[0])
	}
	if m.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", m.Account)
	}
	if m.Contract.ConID != 3691937 {
		t.Errorf("ConID = %d, want 3691937", m.Contract.ConID)
	}
	if m.Contract.Symbol != "AMZN" {
		t.Errorf("Symbol = %q, want AMZN", m.Contract.Symbol)
	}
	if m.Contract.SecType != "STK" {
		t.Errorf("SecType = %q, want STK", m.Contract.SecType)
	}
	if m.Contract.Exchange != "NASDAQ" {
		t.Errorf("Exchange = %q, want NASDAQ", m.Contract.Exchange)
	}
	if m.Contract.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", m.Contract.Currency)
	}
	if m.Position != "15" {
		t.Errorf("Position = %q, want 15", m.Position)
	}
	if m.AvgCost != "200.25" {
		t.Errorf("AvgCost = %q, want 200.25", m.AvgCost)
	}
}

func TestCaptureDecode_PositionEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T215052Z-positions_snapshot, last frame in multi-frame chunk at line 11
	payload := []byte("62\x001\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if _, ok := msgs[0].(PositionEnd); !ok {
		t.Fatalf("type = %T, want PositionEnd", msgs[0])
	}
}

func TestCaptureDecode_HistoricalData(t *testing.T) {
	t.Parallel()
	// captures/20260405T215056Z-historical_bars_1d_1h, single 558-byte frame at line 10
	// 7 hourly bars for AAPL on 2026-04-02
	payload := []byte(
		"17\x001001\x007\x00" +
			"20260402 09:30:00 US/Eastern\x00254.20\x00254.80\x00250.65\x00252.53\x002829736\x00252.266\x0013633\x00" +
			"20260402 10:00:00 US/Eastern\x00252.52\x00255.40\x00251.19\x00255.38\x002797972\x00252.971\x0016541\x00" +
			"20260402 11:00:00 US/Eastern\x00255.40\x00255.73\x00254.36\x00254.57\x001400669\x00255.002\x007744\x00" +
			"20260402 12:00:00 US/Eastern\x00254.57\x00255.00\x00254.00\x00254.42\x00983738\x00254.453\x005662\x00" +
			"20260402 13:00:00 US/Eastern\x00254.42\x00255.49\x00254.17\x00254.61\x001024324\x00254.878\x005832\x00" +
			"20260402 14:00:00 US/Eastern\x00254.58\x00255.46\x00254.58\x00255.28\x001399189\x00255.101\x007342\x00" +
			"20260402 15:00:00 US/Eastern\x00255.29\x00256.13\x00254.80\x00255.89\x002938382\x00255.576\x0017376\x00")

	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	// At server_version 200 the packed IN 17 frame carries bars only; the
	// terminal range arrives separately as IN 108.
	if len(msgs) != 7 {
		t.Fatalf("got %d messages, want 7 bars", len(msgs))
	}

	// First bar
	bar0, ok := msgs[0].(HistoricalBar)
	if !ok {
		t.Fatalf("msgs[0] type = %T, want HistoricalBar", msgs[0])
	}
	if bar0.ReqID != 1001 {
		t.Errorf("bar0.ReqID = %d, want 1001", bar0.ReqID)
	}
	if bar0.Open != "254.20" {
		t.Errorf("bar0.Open = %q, want 254.20", bar0.Open)
	}
	if bar0.High != "254.80" {
		t.Errorf("bar0.High = %q, want 254.80", bar0.High)
	}
	if bar0.Low != "250.65" {
		t.Errorf("bar0.Low = %q, want 250.65", bar0.Low)
	}
	if bar0.Close != "252.53" {
		t.Errorf("bar0.Close = %q, want 252.53", bar0.Close)
	}
	if bar0.Volume != "2829736" {
		t.Errorf("bar0.Volume = %q, want 2829736", bar0.Volume)
	}
	if bar0.Time != "20260402 09:30:00 US/Eastern" {
		t.Errorf("bar0.Time = %q, want '20260402 09:30:00 US/Eastern'", bar0.Time)
	}

	// Last bar
	bar6, ok := msgs[6].(HistoricalBar)
	if !ok {
		t.Fatalf("msgs[6] type = %T, want HistoricalBar", msgs[6])
	}
	if bar6.Open != "255.29" {
		t.Errorf("bar6.Open = %q, want 255.29", bar6.Open)
	}

	for i, msg := range msgs {
		if _, ok := msg.(HistoricalBarsEnd); ok {
			t.Fatalf("msgs[%d] is a synthetic HistoricalBarsEnd from sv200 IN 17", i)
		}
	}
}

func TestCaptureDecode_TickPrice(t *testing.T) {
	t.Parallel()
	// captures/20260405T215734Z-quote_snapshot_aapl, tickType 68 (delayed last) at line 15
	payload := []byte("1\x006\x001001\x0068\x00255.45\x00200\x000\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(TickPrice)
	if !ok {
		t.Fatalf("type = %T, want TickPrice", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.TickType != 68 {
		t.Errorf("TickType = %d, want 68", m.TickType)
	}
	if m.Price != "255.45" {
		t.Errorf("Price = %q, want 255.45", m.Price)
	}
	if m.Size != "200" {
		t.Errorf("Size = %q, want 200", m.Size)
	}
	if m.AttrMask != 0 {
		t.Errorf("AttrMask = %d, want 0", m.AttrMask)
	}
}

func TestCaptureDecode_TickSize(t *testing.T) {
	t.Parallel()
	// captures/20260405T215734Z-quote_snapshot_aapl, tickType 74 (delayed volume) at line 16
	payload := []byte("2\x006\x001001\x0074\x00312894\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(TickSize)
	if !ok {
		t.Fatalf("type = %T, want TickSize", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.TickType != 74 {
		t.Errorf("TickType = %d, want 74", m.TickType)
	}
	if m.Size != "312894" {
		t.Errorf("Size = %q, want 312894", m.Size)
	}
}

func TestCaptureDecode_MarketDataType(t *testing.T) {
	t.Parallel()
	// captures/20260405T215734Z-quote_snapshot_aapl, line 11
	payload := []byte("58\x001\x001001\x003\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(MarketDataType)
	if !ok {
		t.Fatalf("type = %T, want MarketDataType", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.DataType != 3 {
		t.Errorf("DataType = %d, want 3 (delayed)", m.DataType)
	}
}

func TestCaptureDecode_TickSnapshotEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T215734Z-quote_snapshot_aapl, line 18
	payload := []byte("57\x001\x001001\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(TickSnapshotEnd)
	if !ok {
		t.Fatalf("type = %T, want TickSnapshotEnd", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
}

func TestCaptureDecode_OpenOrder(t *testing.T) {
	t.Parallel()
	// captures/20260405T215248Z-open_orders_all (live IB Gateway,
	// server_version 200). Source frame SHA-256
	// a036b17746c76d3a623a887e342652432dd81593fc2fbb2bea1671552727cc6a.
	// The payload preserves the 940-byte frame's field sequence after replacing
	// the account with DU9000001 and the perm ID with 9000, including both
	// values inside the shares-allocation echo. The line-10 chunk's trailing
	// 43 bytes are the separate order_status frame; an earlier revision of this
	// test misparsed the whole 991-byte chunk as one frame and fossilized a
	// fabricated layout.
	// OBDC PUT option, PreSubmitted.
	payload := []byte(
		"5\x000\x00853200900\x00OBDC\x00OPT\x0020261120\x0010\x00P\x00100\x00" +
			"SMART\x00USD\x00OBDC  261120P00010000\x00OBDC\x00SELL\x001\x00LMT\x00" +
			"1.2\x000.0\x00GTC\x00\x00DU9000001\x00\x000\x00\x000\x009000\x00" +
			"0\x000\x000\x00\x009000.1/DU9000001/100\x00\x00\x00\x00\x00\x00" +
			"0\x00\x00\x000\x00\x00-1\x000\x00\x00\x00\x00\x00\x002147483647\x00" +
			"0\x000\x000\x00\x003\x000\x000\x00\x000\x000\x00\x000\x00None\x00\x00" +
			"0\x00\x00\x00\x00?\x000\x000\x00\x000\x000\x00\x00\x00\x00\x00\x00" +
			"0\x000\x000\x002147483647\x002147483647\x00\x00\x000\x00\x00IB\x00" +
			"0\x000\x00\x000\x000\x00PreSubmitted\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00\x00\x00\x00\x00" +
			"\x001.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x00-9223372036854775808\x00\x000\x00\x000\x00" +
			"0\x000\x00None\x001.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x00" +
			"1.7976931348623157E308\x001.7976931348623157E308\x000\x00\x00\x00\x00" +
			"0\x001\x000\x000\x000\x00\x00\x000\x00\x00\x00\x00\x00\x00\x000\x00" +
			"\x000\x00\x002147483647\x00\x000\x00")

	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("type = %T, want OpenOrder", msgs[0])
	}
	if m.OrderID != 0 {
		t.Errorf("OrderID = %d, want 0", m.OrderID)
	}
	if m.Contract.ConID != 853200900 {
		t.Errorf("ConID = %d, want 853200900", m.Contract.ConID)
	}
	if m.Contract.Symbol != "OBDC" {
		t.Errorf("Symbol = %q, want OBDC", m.Contract.Symbol)
	}
	if m.Contract.SecType != "OPT" {
		t.Errorf("SecType = %q, want OPT", m.Contract.SecType)
	}
	if m.Contract.Expiry != "20261120" {
		t.Errorf("Expiry = %q, want 20261120", m.Contract.Expiry)
	}
	if m.Contract.Strike != "10" {
		t.Errorf("Strike = %q, want 10", m.Contract.Strike)
	}
	if m.Contract.Right != "P" {
		t.Errorf("Right = %q, want P", m.Contract.Right)
	}
	if m.Contract.Multiplier != "100" {
		t.Errorf("Multiplier = %q, want 100", m.Contract.Multiplier)
	}
	if m.Contract.Exchange != "SMART" {
		t.Errorf("Exchange = %q, want SMART", m.Contract.Exchange)
	}
	if m.Contract.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", m.Contract.Currency)
	}
	if m.Contract.LocalSymbol != "OBDC  261120P00010000" {
		t.Errorf("LocalSymbol = %q, want %q", m.Contract.LocalSymbol, "OBDC  261120P00010000")
	}
	if m.Contract.TradingClass != "OBDC" {
		t.Errorf("TradingClass = %q, want OBDC", m.Contract.TradingClass)
	}
	if m.Action != "SELL" {
		t.Errorf("Action = %q, want SELL", m.Action)
	}
	if m.Quantity != "1" {
		t.Errorf("Quantity = %q, want 1", m.Quantity)
	}
	if m.OrderType != "LMT" {
		t.Errorf("OrderType = %q, want LMT", m.OrderType)
	}
	if m.LmtPrice != "1.2" {
		t.Errorf("LmtPrice = %q, want 1.2", m.LmtPrice)
	}
	if m.AuxPrice != "0.0" {
		t.Errorf("AuxPrice = %q, want 0.0", m.AuxPrice)
	}
	if m.TIF != "GTC" {
		t.Errorf("TIF = %q, want GTC", m.TIF)
	}
	if m.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", m.Account)
	}
	if m.Origin != "0" {
		t.Errorf("Origin = %q, want 0", m.Origin)
	}
	if m.PermID != "9000" {
		t.Errorf("PermID = %q, want 9000", m.PermID)
	}
	if m.IncludeOvernight != "0" {
		t.Errorf("IncludeOvernight = %q, want 0", m.IncludeOvernight)
	}
	if m.Status != "PreSubmitted" {
		t.Errorf("Status = %q, want PreSubmitted", m.Status)
	}
	// OrderState margin fields (all UNSET in this capture).
	if m.InitMarginBefore != "1.7976931348623157E308" {
		t.Errorf("InitMarginBefore = %q, want UNSET double", m.InitMarginBefore)
	}
}

func TestCaptureDecode_OpenOrderEnd(t *testing.T) {
	t.Parallel()
	// captures/20260405T215248Z-open_orders_all, line 11
	payload := []byte("53\x001\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if _, ok := msgs[0].(OpenOrderEnd); !ok {
		t.Fatalf("type = %T, want OpenOrderEnd", msgs[0])
	}
}

// Encode validation tests: compare our Encode output to the actual request bytes
// sent by the capture client to IB Gateway.

func TestCaptureEncode_StartAPI(t *testing.T) {
	t.Parallel()
	// captures/20260405T214926Z-bootstrap, line 5: client sends StartAPI with clientID=1
	// Actual wire bytes (after 4-byte length prefix): "71\x002\x001\x00\x00"
	want := []byte("71\x002\x001\x00\x00")
	got, err := Encode(200, StartAPI{ClientID: 1})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Encode(200, StartAPI{1}) = %q, want %q", got, want)
	}
}

func TestCaptureEncode_PositionsRequest(t *testing.T) {
	t.Parallel()
	// captures/20260405T215052Z-positions_snapshot, line 9: client sends PositionsRequest
	// Actual wire bytes (after 4-byte length prefix): "61\x001\x00"
	want := []byte("61\x001\x00")
	got, err := Encode(200, PositionsRequest{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Encode(200, PositionsRequest{}) = %q, want %q", got, want)
	}
}

func TestCaptureEncode_OpenOrdersRequest(t *testing.T) {
	t.Parallel()
	// captures/20260405T215248Z-open_orders_all, line 9: client sends reqAllOpenOrders
	// Actual wire bytes (after 4-byte length prefix): "16\x001\x00"
	want := []byte("16\x001\x00")
	got, err := Encode(200, OpenOrdersRequest{Scope: "all"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Encode(200, OpenOrdersRequest{all}) = %q, want %q", got, want)
	}
}

func TestCaptureDecode_HistoricalSchedule(t *testing.T) {
	t.Parallel()
	// captures/20260411T175212Z-historical_schedule_aapl, server_version 200.
	// events.jsonl sha256 prefix: 1b207a57180e6197
	// AAPL STK / 1 M / 1 day / SCHEDULE. Response shape:
	//   [106, reqId, startDateTime, endDateTime, timeZone, sessionCount,
	//    (sessionStart, sessionEnd, refDate)*count]
	// Live emitted 21 sessions covering 2026-03-12 through 2026-04-10.
	payload := []byte(
		"106\x001001\x0020260312-09:30:00\x0020260410-16:00:00\x00US/Eastern\x0021\x00" +
			"20260312-09:30:00\x0020260312-16:00:00\x0020260312\x00" +
			"20260313-09:30:00\x0020260313-16:00:00\x0020260313\x00" +
			"20260316-09:30:00\x0020260316-16:00:00\x0020260316\x00" +
			"20260317-09:30:00\x0020260317-16:00:00\x0020260317\x00" +
			"20260318-09:30:00\x0020260318-16:00:00\x0020260318\x00" +
			"20260319-09:30:00\x0020260319-16:00:00\x0020260319\x00" +
			"20260320-09:30:00\x0020260320-16:00:00\x0020260320\x00" +
			"20260323-09:30:00\x0020260323-16:00:00\x0020260323\x00" +
			"20260324-09:30:00\x0020260324-16:00:00\x0020260324\x00" +
			"20260325-09:30:00\x0020260325-16:00:00\x0020260325\x00" +
			"20260326-09:30:00\x0020260326-16:00:00\x0020260326\x00" +
			"20260327-09:30:00\x0020260327-16:00:00\x0020260327\x00" +
			"20260330-09:30:00\x0020260330-16:00:00\x0020260330\x00" +
			"20260331-09:30:00\x0020260331-16:00:00\x0020260331\x00" +
			"20260401-09:30:00\x0020260401-16:00:00\x0020260401\x00" +
			"20260402-09:30:00\x0020260402-16:00:00\x0020260402\x00" +
			"20260406-09:30:00\x0020260406-16:00:00\x0020260406\x00" +
			"20260407-09:30:00\x0020260407-16:00:00\x0020260407\x00" +
			"20260408-09:30:00\x0020260408-16:00:00\x0020260408\x00" +
			"20260409-09:30:00\x0020260409-16:00:00\x0020260409\x00" +
			"20260410-09:30:00\x0020260410-16:00:00\x0020260410\x00")

	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(HistoricalScheduleResponse)
	if !ok {
		t.Fatalf("type = %T, want HistoricalScheduleResponse", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.StartDateTime != "20260312-09:30:00" {
		t.Errorf("StartDateTime = %q, want 20260312-09:30:00", m.StartDateTime)
	}
	if m.EndDateTime != "20260410-16:00:00" {
		t.Errorf("EndDateTime = %q, want 20260410-16:00:00", m.EndDateTime)
	}
	if m.TimeZone != "US/Eastern" {
		t.Errorf("TimeZone = %q, want US/Eastern", m.TimeZone)
	}
	if len(m.Sessions) != 21 {
		t.Fatalf("Sessions = %d, want 21", len(m.Sessions))
	}
	first := m.Sessions[0]
	if first.StartDateTime != "20260312-09:30:00" || first.EndDateTime != "20260312-16:00:00" || first.RefDate != "20260312" {
		t.Errorf("Sessions[0] = %+v, want 20260312-09:30:00 / 20260312-16:00:00 / 20260312", first)
	}
	last := m.Sessions[20]
	if last.StartDateTime != "20260410-09:30:00" || last.EndDateTime != "20260410-16:00:00" || last.RefDate != "20260410" {
		t.Errorf("Sessions[20] = %+v, want 20260410-09:30:00 / 20260410-16:00:00 / 20260410", last)
	}
}

func TestCaptureDecode_SecDefOptParamsLive(t *testing.T) {
	t.Parallel()
	// captures/20260611T074417Z-api_option_campaign_aapl (paper Gateway,
	// server_version 200, events.jsonl sha256 prefix fa7f3f46793d3277, verified at
	// promotion time): first securityDefinitionOptionParameter row for the
	// AAPL underlying. The live wire carries no marketRuleId between
	// multiplier and the expiration count; decoding used to skip one field
	// here, read the first expiration date as the count, and kill the
	// session, which is why every option qualify since April died with
	// ErrInterrupted.
	payload := []byte("75\x003\x00CBOE\x00265598\x00AAPL\x00100\x0026\x0020260612\x0020260615\x0020260617\x0020260618\x0020260622\x0020260624\x0020260626\x0020260702\x0020260710\x0020260717\x0020260724\x0020260731\x0020260821\x0020260918\x0020261016\x0020261120\x0020261218\x0020270115\x0020270219\x0020270319\x0020270617\x0020270917\x0020271217\x0020280121\x0020280317\x0020281215\x00123\x005.0\x0010.0\x0015.0\x0020.0\x0025.0\x0030.0\x0035.0\x0040.0\x0045.0\x0050.0\x0055.0\x0060.0\x0065.0\x0070.0\x0075.0\x0080.0\x0085.0\x0090.0\x0095.0\x00100.0\x00105.0\x00110.0\x00115.0\x00120.0\x00125.0\x00130.0\x00135.0\x00140.0\x00145.0\x00150.0\x00155.0\x00160.0\x00165.0\x00170.0\x00175.0\x00180.0\x00185.0\x00190.0\x00195.0\x00200.0\x00205.0\x00210.0\x00215.0\x00220.0\x00225.0\x00230.0\x00235.0\x00240.0\x00245.0\x00250.0\x00255.0\x00257.5\x00260.0\x00262.5\x00265.0\x00267.5\x00270.0\x00272.5\x00275.0\x00277.5\x00280.0\x00282.5\x00285.0\x00287.5\x00290.0\x00292.5\x00295.0\x00297.5\x00300.0\x00302.5\x00305.0\x00307.5\x00310.0\x00312.5\x00315.0\x00317.5\x00320.0\x00322.5\x00325.0\x00327.5\x00330.0\x00332.5\x00335.0\x00337.5\x00340.0\x00342.5\x00345.0\x00347.5\x00350.0\x00355.0\x00360.0\x00365.0\x00370.0\x00375.0\x00380.0\x00385.0\x00390.0\x00395.0\x00400.0\x00405.0\x00410.0\x00415.0\x00420.0\x00425.0\x00430.0\x00435.0\x00440.0\x00450.0\x00460.0\x00470.0\x00480.0\x00490.0\x00500.0\x00510.0\x00520.0\x00530.0\x00540.0\x00550.0\x00560.0\x00570.0\x00580.0\x00590.0\x00600.0\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(SecDefOptParamsResponse)
	if !ok {
		t.Fatalf("type = %T, want SecDefOptParamsResponse", msgs[0])
	}
	if m.ReqID != 3 || m.Exchange != "CBOE" || m.UnderlyingConID != 265598 {
		t.Errorf("header = req %d exchange %q conid %d", m.ReqID, m.Exchange, m.UnderlyingConID)
	}
	if m.TradingClass != "AAPL" || m.Multiplier != "100" {
		t.Errorf("class/multiplier = %q/%q", m.TradingClass, m.Multiplier)
	}
	if len(m.Expirations) != 26 || m.Expirations[0] != "20260612" || m.Expirations[25] != "20281215" {
		t.Errorf("expirations = %d first %q last %q", len(m.Expirations), m.Expirations[0], m.Expirations[len(m.Expirations)-1])
	}
	if len(m.Strikes) != 123 || m.Strikes[0] != "5.0" || m.Strikes[122] != "600.0" {
		t.Errorf("strikes = %d first %q last %q", len(m.Strikes), m.Strikes[0], m.Strikes[len(m.Strikes)-1])
	}
}

func TestCaptureDecode_TickOptionComputationLive(t *testing.T) {
	t.Parallel()
	// captures/20260611T075300Z-api_option_campaign_aapl (paper Gateway,
	// server_version 200, events.jsonl sha256 prefix ede083958c7e2748): real
	// custom-option-computation reply to a CalcOptionPrice request. The live
	// frame carries no version field; the legacy skip used to consume the
	// request id and abort the session.
	payload := []byte("21\x006\x0053\x000\x000.3\x000.5579497967180902\x002.6596901785543805\x00-1\x000.0700889483259586\x000.07475592510994344\x00-0.7568548883674484\x00293.23\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(TickOptionComputation)
	if !ok {
		t.Fatalf("type = %T, want TickOptionComputation", msgs[0])
	}
	if m.ReqID != 6 || m.TickType != 53 || m.TickAttrib != 0 {
		t.Errorf("header = req %d tick %d attrib %d", m.ReqID, m.TickType, m.TickAttrib)
	}
	if m.ImpliedVol != "0.3" || m.OptPrice != "2.6596901785543805" || m.UndPrice != "293.23" {
		t.Errorf("values = vol %q px %q und %q", m.ImpliedVol, m.OptPrice, m.UndPrice)
	}
	if m.Delta != "0.5579497967180902" || m.PvDividend != "-1" {
		t.Errorf("delta %q pvDividend %q", m.Delta, m.PvDividend)
	}
}

func TestCaptureDecode_QuoteAncillaryTicksLive(t *testing.T) {
	t.Parallel()
	// captures/20260405T215752Z-quote_stream_genericticks, IB Gateway
	// server_version 200. raw.txt sha256
	// 9c4fec0cd44041ccfec4fee372ed6cea437418183b42591936c64ee4fdf52bee;
	// events.jsonl sha256
	// 3a280d5f1d165d85f16d009ef7bec253937f58483a908e9c2f8b5b1a599ba7f2.
	// Each payload is copied from the live request for generic ticks 233 and
	// 236 with only its four-byte frame length removed.
	tests := []struct {
		name    string
		payload []byte
		assert  func(*testing.T, Message)
	}{
		{
			name:    "tick request parameters",
			payload: []byte("81\x001001\x000.01\x009c0001\x004\x00"),
			assert: func(t *testing.T, message Message) {
				m, ok := message.(TickReqParams)
				if !ok {
					t.Fatalf("type = %T, want TickReqParams", message)
				}
				if m.ReqID != 1001 || m.MinTick != "0.01" || m.BBOExchange != "9c0001" || m.SnapshotPermissions == nil || *m.SnapshotPermissions != 4 {
					t.Fatalf("message = %+v", m)
				}
			},
		},
		{
			name:    "generic tick",
			payload: []byte("45\x006\x001001\x0046\x003.0\x00"),
			assert: func(t *testing.T, message Message) {
				m, ok := message.(TickGeneric)
				if !ok {
					t.Fatalf("type = %T, want TickGeneric", message)
				}
				if m.ReqID != 1001 || m.TickType != 46 || m.Value != "3.0" {
					t.Fatalf("message = %+v", m)
				}
			},
		},
		{
			name:    "string tick",
			payload: []byte("46\x006\x001001\x0088\x001775174157\x00"),
			assert: func(t *testing.T, message Message) {
				m, ok := message.(TickString)
				if !ok {
					t.Fatalf("type = %T, want TickString", message)
				}
				if m.ReqID != 1001 || m.TickType != 88 || m.Value != "1775174157" {
					t.Fatalf("message = %+v", m)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			messages, err := DecodeBatch(200, test.payload)
			if err != nil {
				t.Fatalf("DecodeBatch: %v", err)
			}
			if len(messages) != 1 {
				t.Fatalf("messages = %d, want 1", len(messages))
			}
			test.assert(t, messages[0])
		})
	}
}

func TestCaptureDecode_ContractDetailsMultiplier(t *testing.T) {
	t.Parallel()

	// The v200 contractData layout carries the contract multiplier in the
	// slot directly after minTick (the slot the decoder used to skip as
	// "mdSizeMultiplier"; mdSizeMultiplier left the wire at SIZE_RULES (164)
	// and at v200 this position is the multiplier). Both frames below
	// are full msg_id=10 frames from live IB Gateway (server_version 200),
	// reconstructed from the captured length-prefixed stream.
	tests := []struct {
		name           string
		fields         []string
		wantConID      int
		wantSymbol     string
		wantMinTick    string
		wantMultiplier string
		wantLongName   string
		wantTimeZoneID string
		wantExpiry     string
		wantTradeTime  string
		wantUnder      string
	}{
		{
			// captures/20260405T214941Z-contract_details_aapl_opt,
			// events.jsonl sha256 prefix 3dcaf0b74a7c27a4, first msg_id=10
			// frame: AAPL 20260618 C100 option, multiplier "100".
			name: "aapl option multiplier 100",
			fields: []string{
				"10", "1001", "AAPL", "OPT",
				"20260618", "20260618", "100", "C",
				"SMART", "USD", "AAPL  260618C00100000", "AAPL",
				"AAPL", "675811965", "0.01", "100",
				"ACTIVETIM,AD,ADJUST,ALERT,ALGO,ALLOC,AON,AVGCOST,BASKET,COND,CONDORDER,DAY,DEACT,DEACTDIS,DEACTEOD,DIS,FOK,GAT,GTC,GTD,GTT,HID,ICE,IOC,LIT,LMT,MIT,MKT,MTL,NGCOMB,NONALGO,OCA,OPENCLOSE,PAON,PEGMIDVOL,PEGMKTVOL,PEGPRMVOL,PEGSRFVOL,POSTONLY,PRICECHK,REL,RELPCTOFS,RELSTK,SCALE,SCALERST,SIZECHK,SMARTSTG,SNAPMID,SNAPMKT,SNAPREL,STP,STPLMT,TRAIL,TRAILLIT,TRAILLMT,TRAILMIT,VOLAT,WHATIF",
				"SMART,AMEX,CBOE,PHLX,PSE,ISE,BOX,BATS,NASDAQOM,CBOE2,NASDAQBX,MIAX,GEMINI,EDGX,MERCURY,PEARL,EMERALD,MEMX,IBUSOPT,SAPPHIRE",
				"1", "265598",
				"APPLE INC", "", "202606", "",
				"", "", "US/Eastern",
				"20260405:CLOSED;20260406:0930-20260406:1600;20260407:0930-20260407:1600;20260408:0930-20260408:1600;20260409:0930-20260409:1600;20260410:0930-20260410:1600",
				"20260405:CLOSED;20260406:0930-20260406:1600;20260407:0930-20260407:1600;20260408:0930-20260408:1600;20260409:0930-20260409:1600;20260410:0930-20260410:1600",
				"", "", "0",
				"2", "AAPL", "STK", "32,109,109,109,109,109,109,109,32,109,32,109,109,109,109,109,109,109,32,109",
				"20260618", "", "1", "1",
				"1", "0",
			},
			wantConID:      675811965,
			wantSymbol:     "AAPL",
			wantMinTick:    "0.01",
			wantMultiplier: "100",
			wantLongName:   "APPLE INC",
			wantTimeZoneID: "US/Eastern",
			wantExpiry:     "20260618",
			wantUnder:      "AAPL",
		},
		{
			// captures/20260405T215018Z-contract_details_es_fut,
			// events.jsonl sha256 prefix e863bfbafe48370f, first msg_id=10
			// frame: ESZ6 future, multiplier "50".
			name: "es future multiplier 50",
			fields: []string{
				"10", "1001", "ES", "FUT",
				"20261218 08:30:00 US/Central", "20261218", "0", "",
				"CME", "USD", "ESZ6", "ES",
				"ES", "515416632", "0.25", "50",
				"ACTIVETIM,AD,ADJUST,ALERT,ALGO,ALLOC,AVGCOST,BASKET,BENCHPX,COND,CONDORDER,DAY,DEACT,DEACTDIS,DEACTEOD,GAT,GTC,GTD,GTT,HID,ICE,IOC,LIT,LMT,LTH,MIT,MKT,MKTPROT,MTL,NGCOMB,NONALGO,OCA,PEGBENCH,SCALE,SCALERST,SNAPMID,SNAPMKT,SNAPREL,STP,STPLMT,STPPROT,TRAIL,TRAILLIT,TRAILLMT,TRAILMIT,WHATIF",
				"CME,QBALGO", "1", "11004968",
				"E-mini S&P 500", "", "202612", "",
				"", "", "US/Central",
				";20260405:1700-20260406:1600;20260406:1700-20260407:1600;20260407:1700-20260408:1600;20260408:1700-20260409:1600;20260409:1700-20260410:1600",
				"20260405:CLOSED;20260406:0830-20260406:1600;20260407:0830-20260407:1600;20260408:0830-20260408:1600;20260409:0830-20260409:1600;20260410:0830-20260410:1600",
				"", "", "0",
				"2147483647", "ES", "IND", "67,67",
				"20261218", "", "1", "1",
				"1", "0",
			},
			wantConID:      515416632,
			wantSymbol:     "ES",
			wantMinTick:    "0.25",
			wantMultiplier: "50",
			wantLongName:   "E-mini S&P 500",
			wantTimeZoneID: "US/Central",
			wantExpiry:     "20261218",
			wantTradeTime:  "08:30:00",
			wantUnder:      "ES",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msgs, err := DecodeBatch(200, []byte(strings.Join(tt.fields, "\x00")+"\x00"))
			if err != nil {
				t.Fatalf("DecodeBatch: %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("got %d messages, want 1", len(msgs))
			}
			m, ok := msgs[0].(ContractDetails)
			if !ok {
				t.Fatalf("type = %T, want ContractDetails", msgs[0])
			}
			if m.Contract.ConID != tt.wantConID {
				t.Errorf("ConID = %d, want %d", m.Contract.ConID, tt.wantConID)
			}
			if m.Contract.Symbol != tt.wantSymbol {
				t.Errorf("Symbol = %q, want %q", m.Contract.Symbol, tt.wantSymbol)
			}
			if m.MinTick != tt.wantMinTick {
				t.Errorf("MinTick = %q, want %q", m.MinTick, tt.wantMinTick)
			}
			if m.Contract.Multiplier != tt.wantMultiplier {
				t.Errorf("Multiplier = %q, want %q", m.Contract.Multiplier, tt.wantMultiplier)
			}
			// Fields decoded after the multiplier slot stay aligned.
			if m.LongName != tt.wantLongName {
				t.Errorf("LongName = %q, want %q", m.LongName, tt.wantLongName)
			}
			if m.TimeZoneID != tt.wantTimeZoneID {
				t.Errorf("TimeZoneID = %q, want %q", m.TimeZoneID, tt.wantTimeZoneID)
			}
			if m.Contract.Expiry != tt.wantExpiry || m.LastTradeDate != tt.wantExpiry || m.LastTradeTime != tt.wantTradeTime {
				t.Errorf("expiry fields = %q/%q/%q, want %q/%q/%q", m.Contract.Expiry, m.LastTradeDate, m.LastTradeTime, tt.wantExpiry, tt.wantExpiry, tt.wantTradeTime)
			}
			if m.UnderSymbol != tt.wantUnder {
				t.Errorf("UnderSymbol = %q, want %q", m.UnderSymbol, tt.wantUnder)
			}
		})
	}
}

func TestCaptureDecode_OpenOrderConditionEchoPrice(t *testing.T) {
	t.Parallel()

	fields := liveOpenOrderPriceConditionFields()
	msgs, err := DecodeBatch(200, []byte(strings.Join(fields, "\x00")+"\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("type = %T, want OpenOrder", msgs[0])
	}
	if m.OrderID != 356 {
		t.Errorf("OrderID = %d, want 356", m.OrderID)
	}
	if m.Account != "DU9000001" {
		t.Errorf("Account = %q, want DU9000001", m.Account)
	}
	// Status sits past the "None" DeltaNeutralOrderType sentinel; a partial
	// decode leaves it empty.
	if m.Status != "PreSubmitted" {
		t.Errorf("Status = %q, want PreSubmitted", m.Status)
	}
	// Post-status alignment: the live frame carries the UNSET-double margin
	// sentinels right after the status field.
	if m.InitMarginBefore != "1.7976931348623157E308" {
		t.Errorf("InitMarginBefore = %q, want UNSET double", m.InitMarginBefore)
	}
	if len(m.Conditions) != 1 {
		t.Fatalf("Conditions len = %d, want 1", len(m.Conditions))
	}
	cond := m.Conditions[0]
	if cond.Type != 1 {
		t.Errorf("Condition Type = %d, want 1 (price)", cond.Type)
	}
	if cond.Conjunction != "a" {
		t.Errorf("Condition Conjunction = %q, want a", cond.Conjunction)
	}
	if cond.Operator != 2 {
		t.Errorf("Condition Operator = %d, want 2 (isMore)", cond.Operator)
	}
	if cond.Value != "2918.10" {
		t.Errorf("Condition Value = %q, want 2918.10", cond.Value)
	}
	if cond.ConID != 265598 {
		t.Errorf("Condition ConID = %d, want 265598", cond.ConID)
	}
	if cond.Exchange != "SMART" {
		t.Errorf("Condition Exchange = %q, want SMART", cond.Exchange)
	}
	if cond.TriggerMethod != 4 {
		t.Errorf("Condition TriggerMethod = %d, want 4", cond.TriggerMethod)
	}
	if m.ConditionsIgnoreRTH != "1" {
		t.Errorf("ConditionsIgnoreRTH = %q, want 1", m.ConditionsIgnoreRTH)
	}
	if m.ConditionsCancelOrder != "0" {
		t.Errorf("ConditionsCancelOrder = %q, want 0", m.ConditionsCancelOrder)
	}
	// The live frame ends with the official 32-field
	// adjustedOrderType..imbalanceOnly tail and carries no fill echo (fills
	// arrive on the separate order_status frame). Reaching the full decode
	// (Status above) with the conditions intact proves the post-conditions
	// tail parsed without misalignment.
}

func TestCaptureDecode_OpenOrderConditionEchoExecution(t *testing.T) {
	t.Parallel()

	// captures/20260610T200935Z-api_conditions_matrix_aapl (server_version
	// 200, events.jsonl sha256 prefix 87059663ed139026), first 163-field
	// msg_id=5 frame: order 359, LMT BUY 100 AAPL with an execution condition
	// (secType STK, exchange echoed by the Gateway as ANY, symbol AAPL),
	// conditionsIgnoreRTH true. The execution condition body is 4 fields wide
	// (no operator/value), so this frame freezes the condition-width handling
	// ahead of the shared 32-field live tail. Sanitized: account ->
	// DU9000001, perm id -> 900359, order ref ->
	// ibkrgo-sanitized-20260610T200935Z-001, submitter dropped.
	fields := []string{
		"5", "359", "265598", "AAPL", "STK", "", "0", "?",
		"", "SMART", "USD", "AAPL", "NMS", "BUY", "100", "LMT",
		"14.59", "0.0", "DAY", "", "DU9000001", "", "0", "ibkrgo-sanitized-20260610T200935Z-001",
		"1", "900359", "0", "0", "0", "", "900359.0/DU9000001/100", "",
		"", "", "", "", "", "", "", "0",
		"", "-1", "0", "", "", "", "", "",
		"2147483647", "0", "0", "0", "", "3", "0", "0",
		"", "0", "0", "", "0", "None", "", "0",
		"", "", "", "?", "0", "0", "", "0",
		"0", "", "", "", "", "", "0", "0",
		"0", "2147483647", "2147483647", "", "", "0", "", "IB",
		"0", "0", "", "0", "0", "PreSubmitted", "1.7976931348623157E308", "1.7976931348623157E308",
		"1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "",
		"", "", "", "", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308",
		"1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "-9223372036854775808", "", "0",
		"", "0", "0", "1", "5", "a", "STK", "ANY",
		"AAPL", "1", "0", "None", "1.7976931348623157E308", "15.59", "1.7976931348623157E308", "1.7976931348623157E308",
		"1.7976931348623157E308", "1.7976931348623157E308", "0", "", "", "", "0", "1",
		"0", "0", "0", "", "", "0", "", "",
		"", "", "", "", "0", "", "0", "",
		"2147483647", "", "0",
	}
	msgs, err := DecodeBatch(200, []byte(strings.Join(fields, "\x00")+"\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("type = %T, want OpenOrder", msgs[0])
	}
	if m.Status != "PreSubmitted" {
		t.Errorf("Status = %q, want PreSubmitted", m.Status)
	}
	if len(m.Conditions) != 1 {
		t.Fatalf("Conditions len = %d, want 1", len(m.Conditions))
	}
	cond := m.Conditions[0]
	if cond.Type != 5 {
		t.Errorf("Condition Type = %d, want 5 (execution)", cond.Type)
	}
	if cond.Conjunction != "a" {
		t.Errorf("Condition Conjunction = %q, want a", cond.Conjunction)
	}
	if cond.SecType != "STK" {
		t.Errorf("Condition SecType = %q, want STK", cond.SecType)
	}
	if cond.Exchange != "ANY" {
		t.Errorf("Condition Exchange = %q, want ANY", cond.Exchange)
	}
	if cond.Symbol != "AAPL" {
		t.Errorf("Condition Symbol = %q, want AAPL", cond.Symbol)
	}
	if m.ConditionsIgnoreRTH != "1" {
		t.Errorf("ConditionsIgnoreRTH = %q, want 1", m.ConditionsIgnoreRTH)
	}
	if m.ConditionsCancelOrder != "0" {
		t.Errorf("ConditionsCancelOrder = %q, want 0", m.ConditionsCancelOrder)
	}
}

func TestCaptureDecode_OpenOrderLiveBaseLayout(t *testing.T) {
	t.Parallel()

	// captures/20260405T215248Z-open_orders_all (live IB Gateway,
	// server_version 200, events.jsonl sha256 prefix cb036e1839ecded6), the
	// actual 156-field msg_id=5 frame from the
	// captured stream: OBDC PUT, no conditions (count 0). This is the
	// unconditioned variant of the live layout: DeltaNeutralOrderType "None"
	// followed by the 8-field delta-neutral block and the 32-field
	// adjustedOrderType..imbalanceOnly tail. It used to hit the "None"
	// partial early-out and lose the status block. Sanitized: account ->
	// DU9000001 (also inside the sharesAllocation echo).
	fields := []string{
		"5", "0", "853200900", "OBDC", "OPT", "20261120", "10", "P",
		"100", "SMART", "USD", "OBDC  261120P00010000", "OBDC", "SELL", "1", "LMT",
		"1.2", "0.0", "GTC", "", "DU9000001", "", "0", "",
		"0", "9000", "0", "0", "0", "", "9000.1/DU9000001/100", "",
		"", "", "", "", "0", "", "", "0",
		"", "-1", "0", "", "", "", "", "",
		"2147483647", "0", "0", "0", "", "3", "0", "0",
		"", "0", "0", "", "0", "None", "", "0",
		"", "", "", "?", "0", "0", "", "0",
		"0", "", "", "", "", "", "0", "0",
		"0", "2147483647", "2147483647", "", "", "0", "", "IB",
		"0", "0", "", "0", "0", "PreSubmitted", "1.7976931348623157E308", "1.7976931348623157E308",
		"1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "",
		"", "", "", "", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308",
		"1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "-9223372036854775808", "", "0",
		"", "0", "0", "0", "None", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308",
		"1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "0", "", "", "", "0",
		"1", "0", "0", "0", "", "", "0", "",
		"", "", "", "", "", "0", "", "0",
		"", "2147483647", "", "0",
	}
	msgs, err := DecodeBatch(200, []byte(strings.Join(fields, "\x00")+"\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("type = %T, want OpenOrder", msgs[0])
	}
	if m.Contract.Symbol != "OBDC" || m.Contract.ConID != 853200900 {
		t.Errorf("contract = %s/%d, want OBDC/853200900", m.Contract.Symbol, m.Contract.ConID)
	}
	if m.Action != "SELL" || m.OrderType != "LMT" || m.TIF != "GTC" {
		t.Errorf("order = %s %s %s, want SELL LMT GTC", m.Action, m.OrderType, m.TIF)
	}
	if m.Status != "PreSubmitted" {
		t.Errorf("Status = %q, want PreSubmitted", m.Status)
	}
	if m.InitMarginBefore != "1.7976931348623157E308" {
		t.Errorf("InitMarginBefore = %q, want UNSET double", m.InitMarginBefore)
	}
	if len(m.Conditions) != 0 {
		t.Errorf("Conditions len = %d, want 0", len(m.Conditions))
	}
	if m.ConditionsIgnoreRTH != "" || m.ConditionsCancelOrder != "" {
		t.Errorf("conditions flags = %q/%q, want empty", m.ConditionsIgnoreRTH, m.ConditionsCancelOrder)
	}
}

// liveOpenOrderPriceConditionFields is the full 165-field conditioned
// open_order echo from captures/20260610T200935Z-api_conditions_matrix_aapl
// (live paper IB Gateway, server_version 200, events.jsonl sha256 prefix
// 87059663ed139026): order 356, LMT BUY 100 AAPL with a price condition
// (value normalized by the Gateway to 2918.10, conId 265598, exchange SMART,
// trigger method 4, conditionsIgnoreRTH true). The same 165-field shape with
// DeltaNeutralOrderType "None" and the 32-field adjustedOrderType..imbalanceOnly
// tail appears in captures/20260611T073844Z-place_order_price_condition_aapl
// (sha256 prefix 6c588c638895f152). Sanitized: account -> DU9000001, perm id
// -> 900356 (also inside the sharesAllocation echo), order ref ->
// ibkrgo-sanitized-20260610T200935Z-001, submitter dropped.
func liveOpenOrderPriceConditionFields() []string {
	return []string{
		"5", "356", "265598", "AAPL", "STK", "", "0", "?",
		"", "SMART", "USD", "AAPL", "NMS", "BUY", "100", "LMT",
		"14.59", "0.0", "DAY", "", "DU9000001", "", "0", "ibkrgo-sanitized-20260610T200935Z-001",
		"1", "900356", "0", "0", "0", "", "900356.0/DU9000001/100", "",
		"", "", "", "", "", "", "", "0",
		"", "-1", "0", "", "", "", "", "",
		"2147483647", "0", "0", "0", "", "3", "0", "0",
		"", "0", "0", "", "0", "None", "", "0",
		"", "", "", "?", "0", "0", "", "0",
		"0", "", "", "", "", "", "0", "0",
		"0", "2147483647", "2147483647", "", "", "0", "", "IB",
		"0", "0", "", "0", "0", "PreSubmitted", "1.7976931348623157E308", "1.7976931348623157E308",
		"1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "",
		"", "", "", "", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308",
		"1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "-9223372036854775808", "", "0",
		"", "0", "0", "1", "1", "a", "1", "2918.10",
		"265598", "SMART", "4", "1", "0", "None", "1.7976931348623157E308", "15.59",
		"1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "1.7976931348623157E308", "0", "", "", "",
		"0", "1", "0", "0", "0", "", "", "0",
		"", "", "", "", "", "", "0", "",
		"0", "", "2147483647", "", "0",
	}
}

func TestCaptureDecode_OpenOrderLiveParentID(t *testing.T) {
	t.Parallel()
	// captures/20260414T183644Z-api_bracket_trigger_aapl (paper Gateway,
	// server_version 200, events.jsonl sha256 prefix 4ee433340c2badc9): a bracket
	// take-profit child whose parentId rides the pre-status slot of the
	// live "None"-sentinel layout. The live tail carries no second copy,
	// so dropping the pre-status slot zeroed ParentID on every live frame.
	// Account, perm id, order ref, and the Gateway login token in the wire
	// tail are sanitized (the real login is replaced with "papertrader").
	payload := []byte("5\x00211\x00265598\x00AAPL\x00STK\x00\x000\x00?\x00\x00SMART\x00USD\x00AAPL\x00NMS\x00SELL\x001\x00LMT\x002000.0\x000.0\x00DAY\x001571710075\x00DU9000001\x00\x000\x00ibkrgo-sanitized-20260414T183644Z-001\x001\x00900211\x000\x000\x000\x00\x00900211.0/DU9000001/100\x00\x00\x00\x00\x00\x00\x00\x00\x000\x00\x00-1\x000\x00\x00\x00\x00\x00\x002147483647\x000\x000\x000\x00\x003\x000\x000\x00\x00210\x000\x00\x000\x00None\x00\x000\x00\x00\x00\x00?\x000\x000\x00\x000\x000\x00\x00\x00\x00\x00\x000\x000\x000\x002147483647\x002147483647\x00\x00\x000\x00\x00IB\x000\x000\x00\x000\x000\x00PreSubmitted\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x00\x00\x00\x00\x00\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x00-9223372036854775808\x00\x000\x00\x000\x000\x000\x00None\x001.7976931348623157E308\x002001.0\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x001.7976931348623157E308\x000\x00\x00\x00\x000\x001\x000\x000\x000\x00\x00\x000\x00\x00\x00\x00\x00\x00\x000\x00\x000\x00\x002147483647\x00papertrader\x000\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	m, ok := msgs[0].(OpenOrder)
	if !ok {
		t.Fatalf("type = %T, want OpenOrder", msgs[0])
	}
	if m.OrderID != 211 || m.ParentID != "210" {
		t.Errorf("order/parent = %d/%q, want 211/210", m.OrderID, m.ParentID)
	}
	if m.Status == "" {
		t.Errorf("status empty: live frame fell back to the partial path")
	}
}

// TestDecodeUserInfoLiveFrame freezes the userInfo msg id against the live
// gateway. The response to reqUserInfo arrives as msg_id 107 (official
// USER_INFO), not 103 (REPLACE_FA_END): the mis-numbered constant replayed
// green through the DSL testhost (which encodes with the same constant) and
// killed live sessions with ErrInterrupted on the unknown id. Captured
// 2026-07-04, server_version 200, capture 014d470efb662e72.
func TestDecodeUserInfoLiveFrame(t *testing.T) {
	payload := []byte("107\x001\x00\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("DecodeBatch() returned %d messages, want 1", len(msgs))
	}
	info, ok := msgs[0].(UserInfo)
	if !ok {
		t.Fatalf("DecodeBatch() message type = %T, want UserInfo", msgs[0])
	}
	if info.ReqID != 1 || info.WhiteBrandingID != "" {
		t.Fatalf("UserInfo = %+v, want ReqID=1 empty branding", info)
	}
}

func TestCaptureDecode_MarketRuleLive(t *testing.T) {
	t.Parallel()
	// captures/v1/market_rule.log line 7 (IB Gateway paper account,
	// server_version 200, captured 2026-04-06): live MarketRule replies
	// arrive on msg_id 93, not the 92 the codec shipped with. The wrong
	// constant made every live reply an unknown frame; this freezes the
	// live id and layout [93, marketRuleId, count, pairs(lowEdge, increment)].
	payload := []byte("93\x0026\x001\x000\x000.01\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(MarketRule)
	if !ok {
		t.Fatalf("type = %T, want MarketRule", msgs[0])
	}
	if m.MarketRuleID != 26 {
		t.Errorf("MarketRuleID = %d, want 26", m.MarketRuleID)
	}
	if len(m.Increments) != 1 || m.Increments[0].LowEdge != "0" || m.Increments[0].Increment != "0.01" {
		t.Errorf("Increments = %+v, want [{0 0.01}]", m.Increments)
	}
}

func TestCaptureDecode_MarketDataReroutesLive(t *testing.T) {
	t.Parallel()

	// Capture 20260710T135301Z-sdk_sv206_cfd_reroute_readonly,
	// events.jsonl sha256 7475841869bc53ceaf779b2672bb1606453e84b1dc0ff66a361510f45470d279.
	// Exact server_version 206 keeps both reroute bodies classic and only changes
	// their message-ID prefix to the post-200 four-byte representation.
	tests := []struct {
		name    string
		payload []byte
		want    Message
	}{
		{"market data", []byte("\x00\x00\x00\x5b20621\x008314\x00SMART\x00"), MarketDataReroute{ReqID: 20621, ConID: 8314, Exchange: "SMART"}},
		{"market depth", []byte("\x00\x00\x00\x5c20622\x008314\x00SMART\x00"), MarketDepthReroute{ReqID: 20622, ConID: 8314, Exchange: "SMART"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decode(206, tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Decode() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestCaptureDecode_MarketDataRerouteProto225Live(t *testing.T) {
	t.Parallel()

	// Capture 20260713T205650Z-live_cfd_quote_reroute, events.jsonl sha256
	// bd73e91139899a8c586856ab744269bce686bdab8bda597d00c2032f97d3952c.
	// The Gateway returned raw ID 291 with protobuf fields reqID=1, conID=8314,
	// exchange=SMART after a CFD quote request.
	payload := decodeHex(t, "00000123080110fa401a05534d415254")
	got, err := Decode(225, payload)
	if err != nil {
		t.Fatal(err)
	}
	want := MarketDataReroute{ReqID: 1, ConID: 8314, Exchange: "SMART"}
	if got != want {
		t.Fatalf("Decode() = %#v, want %#v", got, want)
	}
}

func TestCaptureDecode_HistoricalDataEndLive(t *testing.T) {
	t.Parallel()
	// captures/v1/historical_bars_keepup.log line 8 (IB Gateway paper
	// account, server_version 200, captured 2026-04-06): the standalone
	// HISTORICAL_DATA_END frame that follows the packed IN 17 batch. Shape is
	// [108, reqID, startDateTime, endDateTime]; the
	// codec previously misread 108 as the streaming-update id.
	payload := []byte("108\x001001\x0020260406 07:37:52 US/Eastern\x0020260406 08:37:52 US/Eastern\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m, ok := msgs[0].(HistoricalBarsEnd)
	if !ok {
		t.Fatalf("type = %T, want HistoricalBarsEnd", msgs[0])
	}
	if m.ReqID != 1001 {
		t.Errorf("ReqID = %d, want 1001", m.ReqID)
	}
	if m.StartDate != "20260406 07:37:52 US/Eastern" || m.EndDate != "20260406 08:37:52 US/Eastern" {
		t.Errorf("range = %q..%q, want live capture range", m.StartDate, m.EndDate)
	}
}
