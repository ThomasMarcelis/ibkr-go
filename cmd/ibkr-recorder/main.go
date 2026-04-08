package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/capturelog"
)

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:4101", "local listen address")
	upstreamAddr := flag.String("upstream", "127.0.0.1:4001", "upstream IB API address")
	outRoot := flag.String("out", "captures", "capture output root")
	scenario := flag.String("scenario", "bootstrap", "scenario name")
	notes := flag.String("notes", "", "freeform notes")
	clientID := flag.Int("client-id", 1, "client id used by the probe/client")
	flag.Parse()

	session, err := capturelog.Create(*outRoot, capturelog.Meta{
		Scenario:   *scenario,
		ListenAddr: *listenAddr,
		Upstream:   *upstreamAddr,
		ClientID:   *clientID,
		Notes:      *notes,
	})
	if err != nil {
		log.Fatalf("create capture session: %v", err)
	}
	defer session.Close()

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	log.Printf("recording %s -> %s into %s", *listenAddr, *upstreamAddr, session.Dir())

	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		log.Fatal("listen: expected TCP listener")
	}

	captureDeadline := time.Now().Add(30 * time.Minute)
	acceptedLegs := 0
	for time.Now().Before(captureDeadline) {
		if err := tcpListener.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			log.Fatalf("set accept deadline: %v", err)
		}
		clientConn, err := tcpListener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			log.Fatalf("accept: %v", err)
		}

		acceptedLegs++
		proxyErr := runLeg(session, acceptedLegs, *upstreamAddr, clientConn)
		if proxyErr != nil && proxyErr != io.EOF && !isNetClosed(proxyErr) {
			fmt.Fprintln(os.Stderr, proxyErr)
			os.Exit(1)
		}
		break // one recorder instance per scenario; exit after first leg
	}

	if acceptedLegs == 0 {
		fmt.Fprintln(os.Stderr, fmt.Errorf("capture timed out"))
		os.Exit(1)
	}
}

func runLeg(session *capturelog.Session, leg int, upstreamAddr string, clientConn net.Conn) error {
	defer clientConn.Close()

	upstreamConn, err := net.Dial("tcp", upstreamAddr)
	if err != nil {
		return fmt.Errorf("dial upstream: %w", err)
	}
	defer upstreamConn.Close()

	if err := session.RecordConnect(leg); err != nil {
		return err
	}
	defer func() {
		_ = session.RecordDisconnect(leg)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)
	go proxy(ctx, &wg, errCh, session, leg, "client", clientConn, upstreamConn)
	go proxy(ctx, &wg, errCh, session, leg, "server", upstreamConn, clientConn)

	var proxyErr error
	select {
	case proxyErr = <-errCh:
	case <-time.After(30 * time.Minute):
		proxyErr = fmt.Errorf("capture timed out")
	}
	cancel()
	_ = clientConn.Close()
	_ = upstreamConn.Close()
	wg.Wait()
	return proxyErr
}

func proxy(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error, session *capturelog.Session, leg int, direction string, src, dst net.Conn) {
	defer wg.Done()

	buf := make([]byte, 8192)
	for {
		_ = src.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, err := src.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			if recordErr := session.RecordChunk(leg, direction, chunk); recordErr != nil {
				errCh <- recordErr
				return
			}
			if _, writeErr := dst.Write(chunk); writeErr != nil {
				errCh <- writeErr
				return
			}
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			errCh <- err
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func isNetClosed(err error) bool {
	return err != nil && (err.Error() == "use of closed network connection")
}
