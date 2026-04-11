package codec

import (
	"strings"
	"testing"
)

func TestDecodeHistoricalScheduleRejectsMalformedSessionCounts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields []string
	}{
		{
			name:   "non integer count",
			fields: historicalScheduleFields("abc"),
		},
		{
			name:   "empty count",
			fields: historicalScheduleFields(""),
		},
		{
			name:   "negative count",
			fields: historicalScheduleFields("-1"),
		},
		{
			name: "count exceeds available sessions",
			fields: append(historicalScheduleFields("2"),
				"20260312-09:30:00", "20260312-16:00:00", "20260312",
			),
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var err error
			mustNotPanic(t, func() {
				_, err = DecodeBatch(encodeTestFields(tt.fields...))
			})
			if err == nil {
				t.Fatal("DecodeBatch() error = nil, want malformed count error")
			}
		})
	}
}

func TestDecodeHistoricalScheduleAllowsZeroSessions(t *testing.T) {
	t.Parallel()

	msgs, err := DecodeBatch(encodeTestFields(historicalScheduleFields("0")...))
	if err != nil {
		t.Fatalf("DecodeBatch() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("DecodeBatch() returned %d messages, want 1", len(msgs))
	}
	schedule, ok := msgs[0].(HistoricalScheduleResponse)
	if !ok {
		t.Fatalf("DecodeBatch() message = %T, want HistoricalScheduleResponse", msgs[0])
	}
	if len(schedule.Sessions) != 0 {
		t.Fatalf("Sessions = %d, want 0", len(schedule.Sessions))
	}
}

func historicalScheduleFields(count string) []string {
	return []string{
		"106",
		"1001",
		"20260312-09:30:00",
		"20260410-16:00:00",
		"US/Eastern",
		count,
	}
}

func encodeTestFields(fields ...string) []byte {
	return []byte(strings.Join(fields, "\x00") + "\x00")
}
