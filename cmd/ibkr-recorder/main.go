package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/capturelog"
)

type recorderConfig struct {
	listenAddr     string
	upstreamAddr   string
	outRoot        string
	scenario       string
	notes          string
	redact         string
	readyFile      string
	clientID       int
	maxLegs        int
	idleTimeout    time.Duration
	captureTimeout time.Duration
	dial           dialFunc
}

type dialFunc func(context.Context, string, string) (net.Conn, error)

type proxyResult struct {
	direction string
	err       error
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		log.Printf("ibkr-recorder: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("ibkr-recorder", flag.ContinueOnError)
	listenAddr := flags.String("listen", "127.0.0.1:4101", "local listen address")
	upstreamAddr := flags.String("upstream", "127.0.0.1:4002", "upstream IB API address")
	outRoot := flags.String("out", "captures", "capture output root")
	scenario := flags.String("scenario", "bootstrap", "scenario name")
	notes := flags.String("notes", "", "freeform notes")
	readyFile := flags.String("ready-file", "", "write the capture directory here after the listener is ready")
	clientID := flags.Int("client-id", 1, "client id used by the probe/client")
	maxLegs := flags.Int("max-legs", 1, "maximum client connection legs to record before exiting")
	idleTimeout := flags.Duration("idle-timeout", 3*time.Second, "time to wait for another leg after a connection closes")
	captureTimeout := flags.Duration("timeout", 30*time.Minute, "maximum recorder lifetime")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	return record(ctx, recorderConfig{
		listenAddr:     *listenAddr,
		upstreamAddr:   *upstreamAddr,
		outRoot:        *outRoot,
		scenario:       *scenario,
		notes:          *notes,
		redact:         os.Getenv("IBKR_CAPTURE_REDACT"),
		readyFile:      *readyFile,
		clientID:       *clientID,
		maxLegs:        *maxLegs,
		idleTimeout:    *idleTimeout,
		captureTimeout: *captureTimeout,
	})
}

func record(ctx context.Context, cfg recorderConfig) (err error) {
	if cfg.maxLegs <= 0 {
		cfg.maxLegs = 1
	}
	if cfg.dial == nil {
		cfg.dial = new(net.Dialer).DialContext
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("capture context: %w", err)
	}

	listener, err := net.Listen("tcp", cfg.listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer func() {
		err = combineErrors(err, expectedClose("close listener", listener.Close()))
	}()

	session, err := capturelog.Create(cfg.outRoot, capturelog.Meta{
		Scenario:   cfg.scenario,
		ListenAddr: cfg.listenAddr,
		Upstream:   cfg.upstreamAddr,
		ClientID:   cfg.clientID,
		Notes:      cfg.notes,
	})
	if err != nil {
		return fmt.Errorf("create capture session: %w", err)
	}
	defer func() {
		err = combineErrors(err, wrapError("close capture session", session.Close()))
	}()

	// The Gateway login rides the OpenOrder wire tail as an unlabeled token
	// the recorder cannot discover on its own; the operator names it so it
	// never reaches disk. Comma-separated to cover multiple logins.
	for secret := range strings.SplitSeq(cfg.redact, ",") {
		if secret = strings.TrimSpace(secret); secret != "" {
			session.Redact(secret, "papertrader")
		}
	}

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return fmt.Errorf("listen: expected TCP listener, got %T", listener)
	}
	captureCtx, cancel := context.WithTimeout(ctx, cfg.captureTimeout)
	defer cancel()
	stopWake := context.AfterFunc(captureCtx, func() {
		_ = tcpListener.SetDeadline(time.Now())
	})
	defer stopWake()
	if err := captureCtx.Err(); err != nil {
		return fmt.Errorf("capture context: %w", err)
	}
	if cfg.readyFile != "" {
		if err := publishReadyFile(cfg.readyFile, session.Dir()); err != nil {
			return fmt.Errorf("write ready file: %w", err)
		}
	}

	log.Printf("recording %s -> %s into %s", cfg.listenAddr, cfg.upstreamAddr, session.Dir())

	captureDeadline, _ := captureCtx.Deadline()
	acceptedLegs := 0
	for acceptedLegs < cfg.maxLegs {
		if err := captureCtx.Err(); err != nil {
			return fmt.Errorf("capture context: %w", err)
		}
		wait := 500 * time.Millisecond
		if acceptedLegs > 0 {
			wait = cfg.idleTimeout
		}
		if deadline := time.Until(captureDeadline); deadline < wait {
			wait = deadline
		}
		if err := tcpListener.SetDeadline(time.Now().Add(wait)); err != nil {
			return fmt.Errorf("set accept deadline: %w", err)
		}
		clientConn, acceptErr := tcpListener.Accept()
		if acceptErr != nil {
			if netErr, ok := errors.AsType[net.Error](acceptErr); ok && netErr.Timeout() {
				if err := captureCtx.Err(); err != nil {
					return fmt.Errorf("capture context: %w", err)
				}
				if acceptedLegs > 0 {
					break
				}
				continue
			}
			return fmt.Errorf("accept: %w", acceptErr)
		}

		acceptedLegs++
		legErr := runLeg(captureCtx, session, acceptedLegs, cfg.upstreamAddr, clientConn, cfg.dial)
		if legErr != nil {
			return fmt.Errorf("leg %d: %w", acceptedLegs, legErr)
		}
	}

	if acceptedLegs == 0 {
		return fmt.Errorf("capture timed out")
	}
	return nil
}

func publishReadyFile(path, captureDir string) error {
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}
	if _, err := io.WriteString(file, captureDir+"\n"); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanup()
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func runLeg(
	ctx context.Context,
	session *capturelog.Session,
	leg int,
	upstreamAddr string,
	clientConn net.Conn,
	dial dialFunc,
) (err error) {
	var upstreamConn net.Conn
	connectionsClosed := false
	var closeErr error
	closeConnections := func() {
		if connectionsClosed {
			return
		}
		connectionsClosed = true
		closeErr = combineErrors(
			expectedClose("close client connection", clientConn.Close()),
			expectedClose("close upstream connection", closeConn(upstreamConn)),
		)
	}
	connected := false
	defer func() {
		closeConnections()
		var disconnectErr error
		if connected {
			disconnectErr = wrapError("record disconnect", session.RecordDisconnect(leg))
		}
		err = combineErrors(err, closeErr, disconnectErr)
	}()

	upstreamConn, err = dial(ctx, "tcp", upstreamAddr)
	if err != nil {
		return fmt.Errorf("dial upstream: %w", err)
	}
	if err := session.RecordConnect(leg); err != nil {
		return fmt.Errorf("record connect: %w", err)
	}
	connected = true

	proxyCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan proxyResult, 2)
	go func() {
		results <- proxyResult{direction: "client", err: proxy(proxyCtx, session, leg, "client", clientConn, upstreamConn)}
	}()
	go func() {
		results <- proxyResult{direction: "server", err: proxy(proxyCtx, session, leg, "server", upstreamConn, clientConn)}
	}()

	var terminalErr error
	remaining := 2
	select {
	case result := <-results:
		terminalErr = meaningfulProxyError(result)
		remaining--
	case <-ctx.Done():
		terminalErr = fmt.Errorf("capture context: %w", ctx.Err())
	}
	cancel()
	closeConnections()

	for range remaining {
		// Both proxy goroutines publish exactly once. Closing both connections
		// interrupts every direction that did not finish before cancellation.
		terminalErr = combineErrors(terminalErr, meaningfulProxyError(<-results))
	}
	// A proxy result can win the select at the same instant as the parent
	// deadline. The parent context owns recorder lifetime, so it must remain
	// observable regardless of which ready select arm won.
	if ctx.Err() != nil && !errors.Is(terminalErr, ctx.Err()) {
		terminalErr = combineErrors(terminalErr, fmt.Errorf("capture context: %w", ctx.Err()))
	}
	return terminalErr
}

func proxy(ctx context.Context, session *capturelog.Session, leg int, direction string, src, dst net.Conn) error {
	buf := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := src.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}
		n, err := src.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			if recordErr := session.RecordChunk(leg, direction, chunk); recordErr != nil {
				return fmt.Errorf("record chunk: %w", recordErr)
			}
			written, writeErr := dst.Write(chunk)
			if writeErr != nil {
				return fmt.Errorf("write: %w", writeErr)
			}
			if written != len(chunk) {
				return fmt.Errorf("write: %w", io.ErrShortWrite)
			}
		}
		if err != nil {
			if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					continue
				}
			}
			return err
		}
	}
}

func meaningfulProxyError(result proxyResult) error {
	if isExpectedConnectionEnd(result.err) {
		return nil
	}
	return fmt.Errorf("proxy %s: %w", result.direction, result.err)
}

func isExpectedConnectionEnd(err error) bool {
	return err == nil ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func expectedClose(op string, err error) error {
	if err == nil || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

func closeConn(conn net.Conn) error {
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func wrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

func combineErrors(errs ...error) error {
	nonNil := errs[:0]
	for _, err := range errs {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}
	switch len(nonNil) {
	case 0:
		return nil
	case 1:
		return nonNil[0]
	default:
		return errors.Join(nonNil...)
	}
}
