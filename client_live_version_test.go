package ibkr_test

import (
	"strconv"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/ibkrlive"
)

func versionName(sv int) string { return "sv" + strconv.Itoa(sv) }

// TestLiveSupportedVersionMatrix negotiates each supported version exactly.
// Run the matrix through the recorder once per Gateway role so every leg keeps
// the Gateway's actual handshake and a read-only round-trip in its raw trace.
func TestLiveSupportedVersionMatrix(t *testing.T) {
	for sv := 208; sv <= 225; sv++ {
		t.Run(versionName(sv), func(t *testing.T) {
			restore := ibkr.SetAdvertisedServerVersionMaxForTest(sv)
			defer restore()

			client, ctx, cancel := ibkrlive.DialContext(t, 20*time.Second)
			defer cancel()
			defer client.Close()
			if got := client.Session().ServerVersion; got != sv {
				t.Fatalf("negotiated ServerVersion = %d, want %d", got, sv)
			}
			if _, err := client.CurrentTime(ctx); err != nil {
				t.Fatalf("CurrentTime() at sv%d: %v", sv, err)
			}
		})
	}
}
