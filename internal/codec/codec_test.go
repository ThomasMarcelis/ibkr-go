package codec

import (
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func TestCodecSymbolicRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []Message{
		ManagedAccounts{Accounts: []string{"DU12345", "DU67890"}},
		ContractDetailsRequest{
			ReqID: 1,
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  "STK",
				Exchange: "SMART",
				Currency: "USD",
			},
		},
		HistoricalBar{
			ReqID:  2,
			Time:   "2026-04-05T12:00:00Z",
			Open:   "100.0",
			High:   "101.0",
			Low:    "99.5",
			Close:  "100.5",
			Volume: "1000",
		},
		QuoteRequest{
			ReqID: 3,
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  "STK",
				Exchange: "SMART",
				Currency: "USD",
			},
			Snapshot:     true,
			GenericTicks: []string{"233"},
		},
		ExecutionDetail{
			ReqID:   5,
			ExecID:  "exec-1",
			Account: "DU12345",
			Symbol:  "AAPL",
			Side:    "BOT",
			Shares:  "10",
			Price:   "123.45",
			Time:    "2026-04-05T12:00:00Z",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.messageName(), func(t *testing.T) {
			t.Parallel()

			payload, err := Encode(tt)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			got, err := DecodeSymbolic(payload)
			if err != nil {
				t.Fatalf("DecodeSymbolic() error = %v", err)
			}
			if got.messageName() != tt.messageName() {
				t.Fatalf("messageName() = %q, want %q", got.messageName(), tt.messageName())
			}
		})
	}
}

func TestDecodeByMsgID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []string
		want   string
	}{
		{"managed_accounts", []string{"15", "1", "DU12345,DU67890"}, "managed_accounts"},
		{"next_valid_id", []string{"9", "1", "1001"}, "next_valid_id"},
		{"current_time", []string{"49", "1", "1712345678"}, "current_time"},
		{"api_error", []string{"4", "-1", "2104", "Market data farm connected", "", "1712345678000"}, "api_error"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := wire.EncodeFields(tt.fields)
			msgs, err := DecodeBatch(payload)
			if err != nil {
				t.Fatalf("DecodeBatch() error = %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("DecodeBatch() len = %d, want 1", len(msgs))
			}
			if msgs[0].messageName() != tt.want {
				t.Fatalf("messageName() = %q, want %q", msgs[0].messageName(), tt.want)
			}
		})
	}
}

func TestDecodeServerInfo(t *testing.T) {
	t.Parallel()

	payload := wire.EncodeFields([]string{"200", "20260405 23:49:26 CET"})
	info, err := DecodeServerInfo(payload)
	if err != nil {
		t.Fatalf("DecodeServerInfo() error = %v", err)
	}
	if info.ServerVersion != 200 {
		t.Fatalf("ServerVersion = %d, want 200", info.ServerVersion)
	}
	if info.ConnectionTime != "20260405 23:49:26 CET" {
		t.Fatalf("ConnectionTime = %q, want %q", info.ConnectionTime, "20260405 23:49:26 CET")
	}
}

func TestEncodeStartAPI(t *testing.T) {
	t.Parallel()

	payload, err := Encode(StartAPI{ClientID: 1})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if fields[0] != "71" {
		t.Fatalf("msg_id = %q, want 71", fields[0])
	}
	if fields[1] != "2" {
		t.Fatalf("version = %q, want 2", fields[1])
	}
	if fields[2] != "1" {
		t.Fatalf("clientID = %q, want 1", fields[2])
	}
}
