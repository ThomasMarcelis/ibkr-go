package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/capturelog"
)

func TestRecordFailureFlushesRedactionAndClosesResources(t *testing.T) {
	listenAddr := reserveTCPAddress(t)
	root := t.TempDir()
	proxyErr := errors.New("forced upstream proxy failure")
	entered := make(chan struct{})
	release := make(chan struct{})
	upstreamConn, upstreamPeer := net.Pipe()
	defer upstreamPeer.Close()
	upstream := &deadlineFailureConn{
		Conn:    upstreamConn,
		entered: entered,
		release: release,
		err:     proxyErr,
	}

	result := make(chan error, 1)
	go func() {
		result <- record(context.Background(), recorderConfig{
			listenAddr:     listenAddr,
			upstreamAddr:   "upstream",
			outRoot:        root,
			scenario:       "recorder-flush",
			redact:         "supersecret",
			maxLegs:        1,
			captureTimeout: time.Second,
			dial: func(context.Context, string, string) (net.Conn, error) {
				return upstream, nil
			},
		})
	}()

	client := dialRecorder(t, listenAddr)
	forwarded := make(chan []byte, 1)
	go func() {
		buf := make([]byte, len("prefix-supersecret-suffix"))
		_, err := io.ReadFull(upstreamPeer, buf)
		if err != nil {
			forwarded <- nil
			return
		}
		forwarded <- buf
	}()
	for _, part := range []string{"prefix-super", "secret-suffix"} {
		if _, err := client.Write([]byte(part)); err != nil {
			t.Fatalf("client.Write() error = %v", err)
		}
	}
	if got := <-forwarded; !bytes.Equal(got, []byte("prefix-supersecret-suffix")) {
		t.Fatalf("upstream bytes = %q, want original input", got)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("upstream proxy did not start")
	}
	close(release)
	if err := <-result; !errors.Is(err, proxyErr) {
		t.Fatalf("record() error = %v, want proxy cause", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Fatal("client connection remained open after record returned")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close() error = %v", err)
	}

	dir := onlyCaptureDir(t, root)
	events, err := capturelog.LoadEvents(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) < 3 || events[0].Kind != capturelog.EventConnect || events[len(events)-1].Kind != capturelog.EventDisconnect {
		t.Fatalf("capture events = %+v, want balanced connect/chunks/disconnect", events)
	}
	var recorded []byte
	for _, event := range events {
		if event.Kind != capturelog.EventChunk {
			continue
		}
		chunk, err := capturelog.DecodeData(event)
		if err != nil {
			t.Fatalf("DecodeData() error = %v", err)
		}
		recorded = append(recorded, chunk...)
	}
	if want := []byte("prefix-papertrader-suffix"); !bytes.Equal(recorded, want) {
		t.Fatalf("flushed capture = %q, want %q", recorded, want)
	}

	rebound, err := net.Listen("tcp", listenAddr)
	if err != nil {
		t.Fatalf("recorder listener was not released: %v", err)
	}
	if err := rebound.Close(); err != nil {
		t.Fatalf("rebound.Close() error = %v", err)
	}
}

func TestRecordCancellationBeforeFirstLegClosesResources(t *testing.T) {
	listenAddr := reserveTCPAddress(t)
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := record(ctx, recorderConfig{
		listenAddr:     listenAddr,
		upstreamAddr:   "upstream",
		outRoot:        root,
		scenario:       "recorder-canceled",
		maxLegs:        1,
		captureTimeout: time.Second,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("record() error = %v, want context.Canceled", err)
	}
	events, err := capturelog.LoadEvents(filepath.Join(onlyCaptureDir(t, root), "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v, want none before first leg", events)
	}
	rebound, err := net.Listen("tcp", listenAddr)
	if err != nil {
		t.Fatalf("recorder listener was not released: %v", err)
	}
	if err := rebound.Close(); err != nil {
		t.Fatalf("rebound.Close() error = %v", err)
	}
}

func TestRecordReadyFileRequiresBoundListener(t *testing.T) {
	listenAddr := reserveTCPAddress(t)
	occupied, err := net.Listen("tcp", listenAddr)
	if err != nil {
		t.Fatalf("occupy recorder address: %v", err)
	}
	defer occupied.Close()
	readyFile := filepath.Join(t.TempDir(), "ready")

	err = record(context.Background(), recorderConfig{
		listenAddr:     listenAddr,
		upstreamAddr:   "upstream",
		outRoot:        t.TempDir(),
		scenario:       "recorder-bind-failure",
		readyFile:      readyFile,
		maxLegs:        1,
		captureTimeout: time.Second,
	})
	if err == nil {
		t.Fatal("record() error = nil, want bind failure")
	}
	if _, statErr := os.Stat(readyFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("ready file stat error = %v, want os.ErrNotExist", statErr)
	}
}

func TestRecordReadyFileNamesCaptureDirectory(t *testing.T) {
	listenAddr := reserveTCPAddress(t)
	root := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- record(ctx, recorderConfig{
			listenAddr:     listenAddr,
			upstreamAddr:   "upstream",
			outRoot:        root,
			scenario:       "recorder-ready",
			readyFile:      readyFile,
			maxLegs:        1,
			captureTimeout: time.Second,
		})
	}()

	var captureDir string
	deadline := time.Now().Add(time.Second)
	for captureDir == "" && time.Now().Before(deadline) {
		// #nosec G304 -- readyFile is rooted in this test's temporary directory.
		data, err := os.ReadFile(readyFile)
		if err == nil {
			captureDir = strings.TrimSpace(string(data))
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read ready file: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	if captureDir == "" {
		t.Fatal("recorder did not publish readiness")
	}
	if want := onlyCaptureDir(t, root); captureDir != want {
		t.Fatalf("ready capture directory = %q, want %q", captureDir, want)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("record() error = %v, want context.Canceled", err)
	}
}

func TestRunLegRecordsCleanDisconnectAndClosesConnections(t *testing.T) {
	t.Parallel()

	session, err := capturelog.Create(t.TempDir(), capturelog.Meta{Scenario: "recorder-clean-leg"})
	if err != nil {
		t.Fatalf("capturelog.Create() error = %v", err)
	}

	clientConn, clientPeer := net.Pipe()
	upstreamConn, upstreamPeer := net.Pipe()
	client := &observedCloseConn{Conn: clientConn}
	upstream := &observedCloseConn{Conn: upstreamConn}
	defer clientPeer.Close()
	defer upstreamPeer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- runLeg(ctx, session, 1, "upstream", client, func(context.Context, string, string) (net.Conn, error) {
			return upstream, nil
		})
	}()
	if err := clientPeer.Close(); err != nil {
		t.Fatalf("client peer Close() error = %v", err)
	}
	if err := <-result; err != nil {
		t.Fatalf("runLeg() error = %v", err)
	}
	if !client.closed.Load() || !upstream.closed.Load() {
		t.Fatalf("connection close state client=%t upstream=%t, want both closed", client.closed.Load(), upstream.closed.Load())
	}
	if err := session.Close(); err != nil {
		t.Fatalf("session.Close() error = %v", err)
	}

	events, err := capturelog.LoadEvents(filepath.Join(session.Dir(), "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) != 2 || events[0].Kind != capturelog.EventConnect || events[1].Kind != capturelog.EventDisconnect {
		t.Fatalf("leg events = %+v, want connect then disconnect", events)
	}
}

func TestRunLegPreservesParentCancellation(t *testing.T) {
	t.Parallel()

	session, err := capturelog.Create(t.TempDir(), capturelog.Meta{Scenario: "recorder-canceled-leg"})
	if err != nil {
		t.Fatalf("capturelog.Create() error = %v", err)
	}
	defer session.Close()

	clientConn, clientPeer := net.Pipe()
	upstreamConn, upstreamPeer := net.Pipe()
	defer clientPeer.Close()
	defer upstreamPeer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = runLeg(ctx, session, 1, "upstream", clientConn, func(context.Context, string, string) (net.Conn, error) {
		return upstreamConn, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runLeg() error = %v, want context.Canceled", err)
	}
}

func TestRunLegDialHonorsContext(t *testing.T) {
	t.Parallel()

	session, err := capturelog.Create(t.TempDir(), capturelog.Meta{Scenario: "recorder-canceled-dial"})
	if err != nil {
		t.Fatalf("capturelog.Create() error = %v", err)
	}
	clientConn, clientPeer := net.Pipe()
	client := &observedCloseConn{Conn: clientConn}
	defer clientPeer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	dialStarted := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- runLeg(ctx, session, 1, "upstream", client, func(ctx context.Context, _, _ string) (net.Conn, error) {
			close(dialStarted)
			<-ctx.Done()
			return nil, ctx.Err()
		})
	}()
	<-dialStarted
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("runLeg() error = %v, want context.Canceled", err)
	}
	if !client.closed.Load() {
		t.Fatal("accepted client connection remained open after canceled dial")
	}
	if err := session.Close(); err != nil {
		t.Fatalf("session.Close() error = %v", err)
	}
	events, err := capturelog.LoadEvents(filepath.Join(session.Dir(), "events.jsonl"))
	if err != nil {
		t.Fatalf("LoadEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v, want none before upstream connect", events)
	}
}

func TestRunLegPreservesProxyCloseAndDisconnectFailures(t *testing.T) {
	t.Parallel()

	session, err := capturelog.Create(t.TempDir(), capturelog.Meta{Scenario: "recorder-failed-leg"})
	if err != nil {
		t.Fatalf("capturelog.Create() error = %v", err)
	}

	clientProxyErr := errors.New("forced client proxy failure")
	serverProxyErr := errors.New("forced server proxy failure")
	closeErr := errors.New("forced close failure")
	clientEntered := make(chan struct{})
	serverEntered := make(chan struct{})
	release := make(chan struct{})
	clientConn, clientPeer := net.Pipe()
	upstreamConn, upstreamPeer := net.Pipe()
	client := &observedCloseConn{
		Conn: &deadlineFailureConn{
			Conn:    clientConn,
			entered: clientEntered,
			release: release,
			err:     clientProxyErr,
		},
		err: closeErr,
	}
	upstream := &observedCloseConn{Conn: &deadlineFailureConn{
		Conn:    upstreamConn,
		entered: serverEntered,
		release: release,
		err:     serverProxyErr,
	}}
	defer clientPeer.Close()
	defer upstreamPeer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- runLeg(ctx, session, 1, "upstream", client, func(context.Context, string, string) (net.Conn, error) {
			return upstream, nil
		})
	}()
	for direction, entered := range map[string]<-chan struct{}{
		"client": clientEntered,
		"server": serverEntered,
	} {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatalf("%s proxy did not start", direction)
		}
	}
	if err := session.Close(); err != nil {
		t.Fatalf("session.Close() error = %v", err)
	}
	close(release)

	err = <-result
	for name, want := range map[string]error{
		"client proxy": clientProxyErr,
		"server proxy": serverProxyErr,
		"connection":   closeErr,
		"disconnect":   os.ErrClosed,
	} {
		if !errors.Is(err, want) {
			t.Errorf("runLeg() error = %v, want %s cause %v", err, name, want)
		}
	}
	if !client.closed.Load() || !upstream.closed.Load() {
		t.Fatalf("connection close state client=%t upstream=%t, want both closed", client.closed.Load(), upstream.closed.Load())
	}
}

type observedCloseConn struct {
	net.Conn
	closed atomic.Bool
	err    error
}

func (c *observedCloseConn) Close() error {
	c.closed.Store(true)
	return errors.Join(c.Conn.Close(), c.err)
}

type deadlineFailureConn struct {
	net.Conn
	once    sync.Once
	entered chan struct{}
	release chan struct{}
	err     error
}

func (c *deadlineFailureConn) SetReadDeadline(time.Time) error {
	c.once.Do(func() {
		close(c.entered)
		<-c.release
	})
	return c.err
}

func reserveTCPAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve TCP address: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved TCP address: %v", err)
	}
	return addr
}

func dialRecorder(t *testing.T, addr string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			return conn
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial recorder %s: %v", addr, err)
		}
		time.Sleep(time.Millisecond)
	}
}

func onlyCaptureDir(t *testing.T, root string) string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read capture root: %v", err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("capture root entries = %v, want one directory", entries)
	}
	return filepath.Join(root, entries[0].Name())
}
