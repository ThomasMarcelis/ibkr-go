//go:build ibkr_sdk && cgo && linux

package ibkr

import (
	"os"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter/native"
)

func sdkRuntimeAvailable() bool { return true }

func sdkRuntimeRequested() bool { return os.Getenv("IBKR_USE_OFFICIAL_SDK") == "1" }

func newSDKAdapter(queueCapacity int) (sdkadapter.Adapter, error) {
	return native.New(queueCapacity)
}
