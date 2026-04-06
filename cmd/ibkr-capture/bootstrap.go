package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

// sessionInfo captures the bootstrap state observed from the gateway.
type sessionInfo struct {
	ServerVersion   int
	ConnectionTime  string
	ManagedAccounts string
	NextValidID     int64
}

// bootstrap performs the real TWS handshake: API prelude, server info frame,
// START_API, MANAGED_ACCTS, NEXT_VALID_ID. It also drains any informational
// farm-status errors (codes 2104/2106/2158) that arrive alongside. Returns
// once NEXT_VALID_ID has been observed.
func bootstrap(conn net.Conn, clientID, minVer, maxVer int) (*sessionInfo, error) {
	// Step 1: write "API\0" raw, followed by a length-prefixed version range.
	if _, err := conn.Write([]byte("API\x00")); err != nil {
		return nil, fmt.Errorf("write api marker: %w", err)
	}
	version := fmt.Appendf(nil, "v%d..%d", minVer, maxVer)
	if err := wire.WriteFrame(conn, version); err != nil {
		return nil, fmt.Errorf("write version frame: %w", err)
	}
	log.Printf("sent handshake: API\\0 + framed %q", string(version))

	// Step 2: read the first framed reply (server_version, connection_time).
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	payload, err := wire.ReadFrame(conn)
	if err != nil {
		return nil, fmt.Errorf("read server info frame: %w", err)
	}
	fields, err := wire.ParseFields(payload)
	if err != nil {
		return nil, fmt.Errorf("parse server info: %w", err)
	}
	if len(fields) < 2 {
		return nil, fmt.Errorf("server info has %d fields, want 2: %v", len(fields), fields)
	}
	serverVersion, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, fmt.Errorf("parse server_version %q: %w", fields[0], err)
	}
	info := &sessionInfo{
		ServerVersion:  serverVersion,
		ConnectionTime: fields[1],
	}

	// Step 3: send START_API. Layout: [msg_id=71, version=2, client_id, optional_capabilities=""].
	startAPI := wire.EncodeFields([]string{"71", "2", strconv.Itoa(clientID), ""})
	if err := wire.WriteFrame(conn, startAPI); err != nil {
		return nil, fmt.Errorf("write START_API: %w", err)
	}

	// Step 4: drain bootstrap frames until NEXT_VALID_ID arrives. Informational
	// errors and farm-status codes may be interleaved; that's fine.
	deadline := time.Now().Add(10 * time.Second)
	for info.NextValidID == 0 {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set bootstrap read deadline: %w", err)
		}
		payload, err := wire.ReadFrame(conn)
		if err != nil {
			return nil, fmt.Errorf("read bootstrap frame: %w", err)
		}
		fields, err := wire.ParseFields(payload)
		if err != nil {
			return nil, fmt.Errorf("parse bootstrap frame: %w", err)
		}
		if len(fields) == 0 {
			continue
		}
		msgID, _ := strconv.Atoi(fields[0])
		switch msgID {
		case 15: // MANAGED_ACCTS: [15, version, accounts]
			if len(fields) >= 3 {
				info.ManagedAccounts = fields[2]
			}
		case 9: // NEXT_VALID_ID: [9, version, order_id]
			if len(fields) >= 3 {
				info.NextValidID, _ = strconv.ParseInt(fields[2], 10, 64)
			}
		case 4: // ERR_MSG (informational) — log and continue
			if len(fields) >= 4 {
				log.Printf("bootstrap err_msg: reqId=%s code=%s msg=%s", fields[1], fields[2], fields[3])
			}
		default:
			log.Printf("bootstrap frame: msg_id=%d fields=%v", msgID, fields)
		}
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("clear read deadline: %w", err)
	}
	return info, nil
}

// readFrames reads frames from conn and invokes onFrame for each one until
// either the context is cancelled, the stop predicate returns true, or the
// total duration elapses, or the connection closes.
//
// The predicate is evaluated AFTER onFrame is called so the caller can
// observe the terminating frame.
func readFrames(conn net.Conn, duration time.Duration, onFrame func(msgID int, fields []string), stop func(msgID int, fields []string) bool) error {
	deadline := time.Now().Add(duration)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}
		readWait := remaining
		if readWait > time.Second {
			readWait = time.Second
		}
		if err := conn.SetReadDeadline(time.Now().Add(readWait)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}
		payload, err := wire.ReadFrame(conn)
		if err != nil {
			if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
				continue
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read frame: %w", err)
		}
		fields, parseErr := wire.ParseFields(payload)
		if parseErr != nil {
			log.Printf("unparseable frame (%d bytes): %v", len(payload), parseErr)
			continue
		}
		if len(fields) == 0 {
			continue
		}
		msgID, _ := strconv.Atoi(fields[0])
		if onFrame != nil {
			onFrame(msgID, fields)
		}
		if stop != nil && stop(msgID, fields) {
			return nil
		}
	}
}

// sendMessage encodes and writes a framed message from a list of null-separated fields.
func sendMessage(conn net.Conn, fields []string) error {
	payload := wire.EncodeFields(fields)
	if err := wire.WriteFrame(conn, payload); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}
