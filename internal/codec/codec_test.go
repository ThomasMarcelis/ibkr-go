package codec

import "testing"

func TestCodecRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []Message{
		Hello{MinVersion: 1, MaxVersion: 4, ClientID: 7},
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
			got, err := Decode(payload)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if got.messageName() != tt.messageName() {
				t.Fatalf("messageName() = %q, want %q", got.messageName(), tt.messageName())
			}
		})
	}
}
