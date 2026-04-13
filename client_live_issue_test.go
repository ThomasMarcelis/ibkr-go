package ibkr_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/testing/ibkrlive"
)

const (
	envLiveRestart        = "IBKR_LIVE_RESTART"
	envLiveRestartAt      = "IBKR_LIVE_RESTART_AT"
	envLiveRestartLead    = "IBKR_LIVE_RESTART_LEAD"
	envLiveRestartMaxWait = "IBKR_LIVE_RESTART_MAX_WAIT"
	envLiveRecoveryWait   = "IBKR_LIVE_RECOVERY_WAIT"
)

func TestLiveIssue9GatewayDailyRestartAutoReconnects(t *testing.T) {
	requireLiveRestart(t)

	client, cancel := dialLiveIssueClient(t,
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto),
		ibkr.WithTCPKeepAlive(30*time.Second))
	defer cancel()
	defer closeLiveIssueClient(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()
	_ = client.MarketData().SetType(ctx, ibkr.MarketDataDelayed)

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: aaplContract,
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}
	defer sub.Close()

	started, err := waitLiveSubscriptionState(ctx, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if err != nil {
		t.Fatalf("wait Started: %v", err)
	}
	initialSeq := started.ConnectionSeq
	if initialSeq == 0 {
		initialSeq = client.Session().ConnectionSeq
	}

	waitForGatewayRestartWindow(t)

	recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), liveDurationEnv(envLiveRecoveryWait, 5*time.Minute))
	defer cancelRecovery()

	resumed, err := waitLiveSubscriptionState(recoveryCtx, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if err != nil {
		t.Fatalf("wait Resumed after real Gateway restart: %v", err)
	}
	if resumed.ConnectionSeq <= initialSeq {
		t.Fatalf("resumed.ConnectionSeq = %d, want > %d", resumed.ConnectionSeq, initialSeq)
	}
	if err := waitLiveReadySeq(recoveryCtx, client, initialSeq+1); err != nil {
		t.Fatalf("wait ready after restart: %v", err)
	}
	if _, err := client.CurrentTime(recoveryCtx); err != nil {
		t.Fatalf("CurrentTime() after restart error = %v", err)
	}
}

func TestLiveIssue9ReconnectsAfterRealGatewayProxyOutage(t *testing.T) {
	cfg := ibkrlive.Require(t)
	proxy := newLiveGatewayProxy(t, cfg.Addr)
	defer proxy.Close()

	client := dialLiveIssueClientAt(t, 15*time.Second, proxy.Addr(),
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto),
		ibkr.WithTCPKeepAlive(30*time.Second))
	defer closeLiveIssueClient(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()
	_ = client.MarketData().SetType(ctx, ibkr.MarketDataDelayed)

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: aaplContract,
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}
	defer sub.Close()

	started, err := waitLiveSubscriptionState(ctx, sub.Lifecycle(), ibkr.SubscriptionStarted)
	if err != nil {
		t.Fatalf("wait Started: %v", err)
	}
	initialSeq := started.ConnectionSeq
	if initialSeq == 0 {
		initialSeq = client.Session().ConnectionSeq
	}

	proxy.Outage(t, 4*time.Second)

	recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelRecovery()
	resumed, err := waitLiveSubscriptionState(recoveryCtx, sub.Lifecycle(), ibkr.SubscriptionResumed)
	if err != nil {
		t.Fatalf("wait Resumed after real Gateway proxy outage: %v", err)
	}
	if resumed.ConnectionSeq <= initialSeq {
		t.Fatalf("resumed.ConnectionSeq = %d, want > %d", resumed.ConnectionSeq, initialSeq)
	}
	if err := waitLiveReadySeq(recoveryCtx, client, initialSeq+1); err != nil {
		t.Fatalf("wait ready after proxy outage: %v", err)
	}
}

func TestLiveIssue12GatewayIdleSessionDetectsRestart(t *testing.T) {
	requireLiveRestart(t)

	client, cancel := dialLiveIssueClient(t,
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto),
		ibkr.WithTCPKeepAlive(30*time.Second))
	defer cancel()
	defer closeLiveIssueClient(t, client)

	initialSeq := client.Session().ConnectionSeq
	waitForGatewayRestartWindow(t)

	recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), liveDurationEnv(envLiveRecoveryWait, 5*time.Minute))
	defer cancelRecovery()
	if err := waitLiveReadySeq(recoveryCtx, client, initialSeq+1); err != nil {
		t.Fatalf("idle session did not recover after real Gateway restart: %v", err)
	}
	if _, err := client.CurrentTime(recoveryCtx); err != nil {
		t.Fatalf("CurrentTime() after idle restart error = %v", err)
	}
}

func TestLiveIssue12IdleSessionRecoversAfterRealGatewayProxyOutage(t *testing.T) {
	cfg := ibkrlive.Require(t)
	proxy := newLiveGatewayProxy(t, cfg.Addr)
	defer proxy.Close()

	client := dialLiveIssueClientAt(t, 15*time.Second, proxy.Addr(),
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto),
		ibkr.WithTCPKeepAlive(30*time.Second))
	defer closeLiveIssueClient(t, client)

	initialSeq := client.Session().ConnectionSeq
	proxy.Outage(t, 4*time.Second)

	recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelRecovery()
	if err := waitLiveReadySeq(recoveryCtx, client, initialSeq+1); err != nil {
		t.Fatalf("idle session did not recover after real Gateway proxy outage: %v", err)
	}
	if _, err := client.CurrentTime(recoveryCtx); err != nil {
		t.Fatalf("CurrentTime() after proxy outage error = %v", err)
	}
}

func TestLiveIssue13SubscriptionErrAfterClose(t *testing.T) {
	client, cancel := dialLiveIssueClient(t)
	defer cancel()
	defer closeLiveIssueClient(t, client)

	ctx, cancelReq := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelReq()
	_ = client.MarketData().SetType(ctx, ibkr.MarketDataDelayed)

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: aaplContract,
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}
	if _, err := waitLiveSubscriptionState(ctx, sub.Lifecycle(), ibkr.SubscriptionStarted); err != nil {
		t.Fatalf("wait Started: %v", err)
	}
	if err := sub.Close(); err != nil {
		t.Fatalf("sub.Close() error = %v", err)
	}
	select {
	case <-sub.Done():
	case <-ctx.Done():
		t.Fatalf("sub.Done() did not close: %v", ctx.Err())
	}
	if err := sub.Err(); err != nil {
		t.Fatalf("sub.Err() after clean close = %v, want nil", err)
	}
	if err := sub.Wait(); err != nil {
		t.Fatalf("sub.Wait() after clean close = %v, want nil", err)
	}
}

func TestLiveIssue11HistoricalRequestValidationErrorsAreTyped(t *testing.T) {
	client, cancel := dialLiveIssueClient(t)
	defer cancel()
	defer closeLiveIssueClient(t, client)

	base := ibkr.HistoricalBarsRequest{
		Contract:   aaplContract,
		EndTime:    time.Now(),
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	}
	testCases := []struct {
		name  string
		req   ibkr.HistoricalBarsRequest
		field string
	}{
		{name: "schedule", req: liveWithWhatToShow(base, ibkr.ShowSchedule), field: "WhatToShow"},
		{name: "unsupported what to show", req: liveWithWhatToShow(base, ibkr.WhatToShow("FEELINGS")), field: "WhatToShow"},
		{name: "invalid duration", req: liveWithDuration(base, ibkr.HistoricalDuration("1 fortnight")), field: "Duration"},
		{name: "invalid bar size", req: liveWithBarSize(base, ibkr.BarSize("90 mins")), field: "BarSize"},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancelReq := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelReq()
			_, err := client.History().Bars(ctx, tt.req)
			validationErr, ok := errors.AsType[*ibkr.ValidationError](err)
			if !ok {
				t.Fatalf("History().Bars() error = %v, want *ValidationError", err)
			}
			if validationErr.Field != tt.field {
				t.Fatalf("ValidationError.Field = %q, want %q", validationErr.Field, tt.field)
			}
		})
	}
}

func requireLiveRestart(t *testing.T) {
	t.Helper()
	ibkrlive.Require(t)
	if os.Getenv(envLiveRestart) == "" {
		t.Skipf("set %s=1 to run real Gateway restart repro tests", envLiveRestart)
	}
}

func dialLiveIssueClient(t *testing.T, extra ...ibkr.Option) (*ibkr.Client, context.CancelFunc) {
	t.Helper()
	cfg := ibkrlive.Require(t)
	return dialLiveIssueClientConfig(t, cfg, 15*time.Second, extra...)
}

func dialLiveIssueClientAt(t *testing.T, timeout time.Duration, addr string, extra ...ibkr.Option) *ibkr.Client {
	t.Helper()
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q) error = %v", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse proxy port %q: %v", portText, err)
	}
	cfg := ibkrlive.Config{Addr: addr, Host: host, Port: port, ClientID: liveIssueClientID()}
	client, cancel := dialLiveIssueClientConfig(t, cfg, timeout, extra...)
	t.Cleanup(cancel)
	return client
}

func dialLiveIssueClientConfig(t *testing.T, cfg ibkrlive.Config, timeout time.Duration, extra ...ibkr.Option) (*ibkr.Client, context.CancelFunc) {
	t.Helper()
	if os.Getenv("IBKR_LIVE_CLIENT_ID") == "" {
		cfg.ClientID = liveIssueClientID()
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	client, err := ibkr.DialContext(ctx, ibkrlive.Options(cfg, extra...)...)
	if err != nil {
		cancel()
		t.Fatalf("DialContext() error = %v", err)
	}
	return client, cancel
}

func waitForGatewayRestartWindow(t *testing.T) {
	t.Helper()

	now := time.Now()
	restart := nextGatewayRestart(now, liveClockEnv(envLiveRestartAt, "23:45"))
	lead := liveDurationEnv(envLiveRestartLead, 15*time.Second)
	maxWait := liveDurationEnv(envLiveRestartMaxWait, 15*time.Minute)
	wait := time.Until(restart.Add(-lead))
	if wait > maxWait {
		t.Skipf("Gateway restart window begins in %s at %s; rerun near the window or raise %s",
			wait.Round(time.Second), restart.Format(time.RFC3339), envLiveRestartMaxWait)
	}
	if wait > 0 {
		t.Logf("waiting %s for real Gateway restart window at %s", wait.Round(time.Second), restart.Format(time.RFC3339))
		time.Sleep(wait)
	}
}

func nextGatewayRestart(now time.Time, clock string) time.Time {
	parts := strings.Split(clock, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	restart := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if now.After(restart.Add(5 * time.Minute)) {
		restart = restart.Add(24 * time.Hour)
	}
	return restart
}

func liveClockEnv(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return fallback
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return fallback
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return fallback
	}
	return value
}

func liveDurationEnv(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func waitLiveSubscriptionState(ctx context.Context, ch <-chan ibkr.SubscriptionStateEvent, want ibkr.SubscriptionStateKind) (ibkr.SubscriptionStateEvent, error) {
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return ibkr.SubscriptionStateEvent{}, fmt.Errorf("lifecycle closed before %s", want)
			}
			if evt.Kind == want {
				return evt, nil
			}
			if evt.Kind == ibkr.SubscriptionClosed {
				return ibkr.SubscriptionStateEvent{}, fmt.Errorf("subscription closed before %s: %v", want, evt.Err)
			}
		case <-ctx.Done():
			return ibkr.SubscriptionStateEvent{}, ctx.Err()
		}
	}
}

func waitLiveReadySeq(ctx context.Context, client *ibkr.Client, minSeq uint64) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		snap := client.Session()
		if snap.State == ibkr.StateReady && snap.ConnectionSeq >= minSeq {
			return nil
		}
		select {
		case <-ticker.C:
		case <-client.Done():
			return client.Wait()
		case <-ctx.Done():
			return fmt.Errorf("%w; last session=%+v", ctx.Err(), snap)
		}
	}
}

func closeLiveIssueClient(t *testing.T, client *ibkr.Client) {
	t.Helper()
	_ = client.Close()
	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatalf("client.Done() did not close")
	}
	time.Sleep(500 * time.Millisecond)
}

func liveIssueClientID() int {
	return 10_000 + int(time.Now().UnixNano()%1_000_000)
}

type liveGatewayProxy struct {
	upstream string
	addr     string

	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]struct{}
	closed   bool
}

func newLiveGatewayProxy(t *testing.T, upstream string) *liveGatewayProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen live Gateway proxy: %v", err)
	}
	proxy := &liveGatewayProxy{
		upstream: upstream,
		addr:     ln.Addr().String(),
		listener: ln,
		conns:    make(map[net.Conn]struct{}),
	}
	go proxy.accept(ln)
	return proxy
}

func (p *liveGatewayProxy) Addr() string {
	return p.addr
}

func (p *liveGatewayProxy) Outage(t *testing.T, duration time.Duration) {
	t.Helper()
	p.closeListenerAndConns()
	time.Sleep(duration)
	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		t.Fatalf("resume live Gateway proxy listener: %v", err)
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		_ = ln.Close()
		return
	}
	p.listener = ln
	p.mu.Unlock()
	go p.accept(ln)
}

func (p *liveGatewayProxy) Close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.closeListenerAndConns()
}

func (p *liveGatewayProxy) accept(ln net.Listener) {
	for {
		clientConn, err := ln.Accept()
		if err != nil {
			return
		}
		upstreamConn, err := net.Dial("tcp", p.upstream)
		if err != nil {
			_ = clientConn.Close()
			continue
		}
		p.track(clientConn)
		p.track(upstreamConn)
		go p.pipe(clientConn, upstreamConn)
		go p.pipe(upstreamConn, clientConn)
	}
}

func (p *liveGatewayProxy) pipe(dst, src net.Conn) {
	_, _ = io.Copy(dst, src)
	_ = dst.Close()
	_ = src.Close()
	p.untrack(dst)
	p.untrack(src)
}

func (p *liveGatewayProxy) track(conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		_ = conn.Close()
		return
	}
	p.conns[conn] = struct{}{}
}

func (p *liveGatewayProxy) untrack(conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.conns, conn)
}

func (p *liveGatewayProxy) closeListenerAndConns() {
	p.mu.Lock()
	listener := p.listener
	p.listener = nil
	conns := make([]net.Conn, 0, len(p.conns))
	for conn := range p.conns {
		conns = append(conns, conn)
	}
	p.conns = make(map[net.Conn]struct{})
	p.mu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
}

func liveWithWhatToShow(req ibkr.HistoricalBarsRequest, value ibkr.WhatToShow) ibkr.HistoricalBarsRequest {
	req.WhatToShow = value
	return req
}

func liveWithDuration(req ibkr.HistoricalBarsRequest, value ibkr.HistoricalDuration) ibkr.HistoricalBarsRequest {
	req.Duration = value
	return req
}

func liveWithBarSize(req ibkr.HistoricalBarsRequest, value ibkr.BarSize) ibkr.HistoricalBarsRequest {
	req.BarSize = value
	return req
}
