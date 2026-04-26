//go:build ibkr_swig && cgo && linux

package ibkrsdkprobe

import (
	"os"
	"testing"
)

func TestCompileProbeVersion(t *testing.T) {
	if got := CompileProbeVersion(); got != "ibkr-swig-probe" {
		t.Fatalf("CompileProbeVersion() = %q, want %q", got, "ibkr-swig-probe")
	}
}

func TestSDKProbeLive(t *testing.T) {
	if os.Getenv("IBKR_LIVE") != "1" {
		t.Skip("set IBKR_LIVE=1 after the compile/link probe succeeds")
	}
	t.Skip("live official SDK adapter is the next spike step after compile/link is proven")
}
