//go:build !ibkr_sdk || !cgo || !linux

package ibkr

import (
	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter/native"
)

func newSDKAdapter(queueCapacity int) (sdkadapter.Adapter, error) {
	return native.New(queueCapacity)
}
