package ibkr_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
)

type fragmentingDialer struct {
	maxRead   int
	readCalls atomic.Int64
	readBytes atomic.Int64
}

func (d *fragmentingDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := new(net.Dialer).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return &fragmentingConn{Conn: conn, dialer: d}, nil
}

type fragmentingConn struct {
	net.Conn
	dialer *fragmentingDialer
}

func (c *fragmentingConn) Read(p []byte) (int, error) {
	if len(p) > c.dialer.maxRead {
		p = p[:c.dialer.maxRead]
	}
	n, err := c.Conn.Read(p)
	c.dialer.readCalls.Add(1)
	c.dialer.readBytes.Add(int64(n))
	return n, err
}

// TestFrameReassemblyOneByteReads freezes the public receive path against the
// exact sv225 historical entitlement capture while
// forcing every handshake, frame prefix, and payload read through one-byte
// fragments. The read counters make the fragmentation assertion non-vacuous.
func TestFrameReassemblyOneByteReads(t *testing.T) {
	t.Parallel()

	dialer := &fragmentingDialer{maxRead: 1}
	client, host := newClient(t, "historical_bars_subscription_required.txt", ibkr.WithDialer(dialer))
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			ConID: 265598, Symbol: "AAPL", SecType: ibkr.SecTypeStock,
			Exchange: "SMART", Currency: "USD",
		},
		Duration: ibkr.Days(1), BarSize: ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades, UseRTH: true,
	})
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.Code != ibkr.ErrCodeHistoricalDataSubscriptionRequired {
		t.Fatalf("History().Bars() error = %v, want code 2188", err)
	}
	if calls, bytes := dialer.readCalls.Load(), dialer.readBytes.Load(); bytes < 100 || calls < bytes {
		t.Fatalf("fragmentation was not exercised: read calls=%d bytes=%d", calls, bytes)
	}
}
