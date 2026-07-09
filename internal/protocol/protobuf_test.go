package protocol

import "testing"

func TestOutboundProtobufMigrationGates(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		msgID int
		want  int
	}{
		{OutReqExecutions, 201},
		{OutPlaceOrder, 203},
		{OutReqAccountSummary, 207},
		{OutStartAPI, 213},
		{OutReqConfig, 219},
	} {
		got, ok := OutboundProtobufVersion(tc.msgID)
		if !ok || got != tc.want {
			t.Fatalf("OutboundProtobufVersion(%d) = (%d, %t), want (%d, true)", tc.msgID, got, ok, tc.want)
		}
	}
	if _, ok := OutboundProtobufVersion(10_000); ok {
		t.Fatal("unknown outbound ID has a protobuf migration gate")
	}
}
