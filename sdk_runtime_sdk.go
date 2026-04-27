//go:build ibkr_sdk && cgo && linux

package ibkr

import (
	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter/native"
)

func sdkRuntimeAvailable() bool { return true }

func sdkRuntimeRequested() bool { return true }

func newSDKAdapter(queueCapacity int) (sdkadapter.Adapter, error) {
	return native.New(queueCapacity)
}
