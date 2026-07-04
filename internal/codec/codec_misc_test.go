package codec

import "testing"

// TestReplaceFAEncodesTrailingReqID freezes the replaceFA layout: version,
// faDataType, xml, then the reqId required since REPLACE_FA_END (157),
// which the encoder omitted until 2026-07-04 (client.py:4805-4816). Live
// verification is blocked: paper and read-only accounts are not financial
// advisors, so the request is rejected before the layout matters.
func TestReplaceFAEncodesTrailingReqID(t *testing.T) {
	fields, err := ReplaceFA{ReqID: 9001, FADataType: 1, XML: "<x/>"}.encodeWire(200)
	if err != nil {
		t.Fatalf("encodeWire() error = %v", err)
	}
	want := []string{itoa(OutReplaceFA), "1", "1", "<x/>", "9001"}
	if len(fields) != len(want) {
		t.Fatalf("encodeWire() = %v, want %v", fields, want)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Fatalf("encodeWire()[%d] = %q, want %q", i, fields[i], want[i])
		}
	}
}

// TestDecodeReplaceFAEnd freezes the replaceFAEnd shape [103, reqId, text]
// (decoder.py:2243-2247). msg_id 103 was previously mis-assigned to
// userInfo; an FA gateway acknowledging replaceFA would have decoded as a
// UserInfo response.
func TestDecodeReplaceFAEnd(t *testing.T) {
	msgs, err := DecodeBatch(200, []byte("103\x009001\x00ok\x00"))
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	end, ok := msgs[0].(ReplaceFAEnd)
	if !ok {
		t.Fatalf("DecodeBatch() message type = %T, want ReplaceFAEnd", msgs[0])
	}
	if end.ReqID != 9001 || end.Text != "ok" {
		t.Fatalf("ReplaceFAEnd = %+v, want ReqID=9001 Text=ok", end)
	}
}
