//go:build !ibkr_sdk || !cgo || !linux

package native

import (
	"context"
	"errors"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

var ErrUnavailable = errors.New("native: SDK runtime requires -tags=ibkr_sdk, cgo, and linux")

type Adapter struct{}

func New(int) (*Adapter, error) { return nil, ErrUnavailable }

func BuildInfo() (sdkadapter.BuildInfo, error) {
	return sdkadapter.BuildInfo{}, ErrUnavailable
}

func (*Adapter) Connect(context.Context, sdkadapter.ConnectRequest) error { return ErrUnavailable }

func (*Adapter) Disconnect() error { return nil }

func (*Adapter) IsConnected() bool { return false }

func (*Adapter) ServerVersion() int { return 0 }

func (*Adapter) ConnectionTime() string { return "" }

func (*Adapter) Submit(context.Context, sdkadapter.Command) error { return ErrUnavailable }

func (*Adapter) DrainEvents(context.Context, int) ([]sdkadapter.Event, error) {
	return nil, ErrUnavailable
}

func (*Adapter) Close() error { return nil }
