//go:build !ibkr_sdk || !cgo || !linux

package ibkr

import (
	"fmt"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func sdkRuntimeAvailable() bool { return false }

func sdkRuntimeRequested() bool { return false }

func newSDKAdapter(int) (sdkadapter.Adapter, error) {
	return nil, fmt.Errorf("ibkr: SDK runtime requires -tags=ibkr_sdk, cgo, and linux")
}
