package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/testhost"
)

func TestSupportedVersionMatrixReplay(t *testing.T) {
	host, err := testhost.NewFromFile("testdata/transcripts/supported_version_matrix.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer waitHost(t, host)

	for serverVersion := 208; serverVersion <= 225; serverVersion++ {
		func() {
			restore := ibkr.SetAdvertisedServerVersionMaxForTest(serverVersion)
			defer restore()

			client := dialHostClient(t, host)
			defer client.Close()
			if got := client.Session().ServerVersion; got != serverVersion {
				t.Fatalf("negotiated server version = %d, want %d", got, serverVersion)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := client.CurrentTime(ctx); err != nil {
				t.Fatalf("CurrentTime() at server version %d: %v", serverVersion, err)
			}
		}()
	}
}
