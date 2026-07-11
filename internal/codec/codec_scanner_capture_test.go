package codec

import "testing"

func TestCaptureDecode_ScannerDataLive(t *testing.T) {
	t.Parallel()

	// captures/20260709T221545Z-api_scanner_subscription/events.jsonl,
	// server_version=200, events sha256 prefix c84c81b3ee772bcc. The payload
	// below is the exact ten-row server frame after stripping its length prefix.
	payload := []byte("20\x003\x001\x0010\x000\x00888872117\x00BGIA\x00STK\x00\x000\x00\x00SMART\x00USD\x00BGIA\x00NMS\x00NMS\x00\x00\x00\x00\x001\x00863272583\x00ENHI\x00STK\x00\x000\x00\x00SMART\x00USD\x00ENHI\x00NMS\x00NMS\x00\x00\x00\x00\x002\x00794922960\x00JLHL\x00STK\x00\x000\x00\x00SMART\x00USD\x00JLHL\x00SCM\x00SCM\x00\x00\x00\x00\x003\x00895811731\x00VRAX\x00STK\x00\x000\x00\x00SMART\x00USD\x00VRAX\x00SCM\x00SCM\x00\x00\x00\x00\x004\x00319808443\x00WRAP\x00STK\x00\x000\x00\x00SMART\x00USD\x00WRAP\x00SCM\x00SCM\x00\x00\x00\x00\x005\x00694625709\x00ZBAO\x00STK\x00\x000\x00\x00SMART\x00USD\x00ZBAO\x00SCM\x00SCM\x00\x00\x00\x00\x006\x00791113853\x00CCAQ\x00STK\x00\x000\x00\x00SMART\x00USD\x00CCAQ\x00NMS\x00NMS\x00\x00\x00\x00\x007\x00895507024\x00TPTS\x00STK\x00\x000\x00\x00SMART\x00USD\x00TPTS\x00TPTS\x00TPTS\x00\x00\x00\x00\x008\x00834260493\x00LGHL\x00STK\x00\x000\x00\x00SMART\x00USD\x00LGHL\x00SCM\x00SCM\x00\x00\x00\x00\x009\x0044000292\x00FAB\x00STK\x00\x000\x00\x00SMART\x00USD\x00FAB\x00NMS\x00NMS\x00\x00\x00\x00\x00")
	msgs, err := DecodeBatch(200, payload)
	if err != nil {
		t.Fatalf("DecodeBatch: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages len = %d, want 1", len(msgs))
	}
	response, ok := msgs[0].(ScannerDataResponse)
	if !ok {
		t.Fatalf("message type = %T, want ScannerDataResponse", msgs[0])
	}
	if response.ReqID != 1 || len(response.Entries) != 10 {
		t.Fatalf("response = req_id %d entries %d, want req_id 1 entries 10", response.ReqID, len(response.Entries))
	}
	if got := response.Entries[0]; got.Rank != 0 ||
		got.Contract.ConID != 888872117 || got.Contract.Symbol != "BGIA" ||
		got.Contract.Multiplier != "" || got.Contract.Exchange != "SMART" ||
		got.Contract.Currency != "USD" || got.Contract.LocalSymbol != "BGIA" ||
		got.MarketName != "NMS" || got.Contract.TradingClass != "NMS" {
		t.Fatalf("first entry = %+v, want exact live BGIA scanner contract mapping", got)
	}
	if got := response.Entries[9]; got.Rank != 9 || got.Contract.ConID != 44000292 || got.Contract.Symbol != "FAB" {
		t.Fatalf("last entry = %+v, want live rank 9 FAB contract 44000292", got)
	}
}
