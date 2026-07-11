package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/capturelog"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/wire"
)

func main() {
	dir := flag.String("dir", "", "capture directory containing events.jsonl")
	rawOut := flag.String("out", "", "raw output file path (default: <dir>/raw.txt)")
	replayDir := flag.String("replay-dir", "", "directory for normalized replay artifacts (default: <dir>/replay)")
	transcriptOut := flag.String("transcript-out", "", "optional raw transcript skeleton output path")
	verify := flag.Bool("verify", false, "verify capture integrity and print a protocol-aware summary")
	flag.Parse()

	if *dir == "" {
		log.Fatal("-dir is required")
	}
	if *rawOut == "" {
		*rawOut = filepath.Join(*dir, "raw.txt")
	}
	if *replayDir == "" {
		*replayDir = filepath.Join(*dir, "replay")
	}

	events, err := capturelog.LoadEvents(filepath.Join(*dir, "events.jsonl"))
	if err != nil {
		log.Fatalf("load events: %v", err)
	}
	meta, err := capturelog.LoadMeta(filepath.Join(*dir, "meta.json"))
	if err != nil {
		log.Fatalf("load meta: %v", err)
	}
	replayEvents, err := capturelog.NormalizeEvents(events)
	if err != nil {
		log.Fatalf("normalize events: %v", err)
	}

	file, err := os.OpenFile(*rawOut, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatalf("create output: %v", err)
	}
	defer file.Close()

	for _, event := range events {
		kind := event.Kind
		if kind == "" {
			kind = capturelog.EventChunk
		}
		line := fmt.Sprintf("%s leg=%d kind=%s", event.At.Format("2006-01-02T15:04:05.000000000Z07:00"), event.Leg, kind)
		if kind == capturelog.EventChunk {
			data, err := capturelog.DecodeData(event)
			if err != nil {
				log.Fatalf("decode event: %v", err)
			}
			line = fmt.Sprintf("%s direction=%s len=%d hex=%s quoted=%q",
				line,
				event.Direction,
				event.Length,
				hex.EncodeToString(data),
				string(data),
			)
		}
		if _, err := fmt.Fprintln(file, line); err != nil {
			log.Fatalf("write output: %v", err)
		}
	}

	if err := capturelog.WriteReplay(*replayDir, *dir, meta, replayEvents); err != nil {
		log.Fatalf("write replay: %v", err)
	}
	if *verify {
		if err := writeVerification(os.Stdout, *dir, events, replayEvents); err != nil {
			log.Fatalf("verify capture: %v", err)
		}
	}
	if *transcriptOut != "" {
		if err := writeTranscriptSkeleton(*transcriptOut, meta, replayEvents); err != nil {
			log.Fatalf("write transcript skeleton: %v", err)
		}
	}
}

func writeTranscriptSkeleton(path string, meta capturelog.Meta, replayEvents []capturelog.ReplayEvent) error {
	// #nosec G304 -- path is the operator-selected transcript output.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create transcript skeleton: %w", err)
	}
	defer file.Close()

	if _, err := fmt.Fprintf(file, "# Raw transcript skeleton for %s.\n", meta.Scenario); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(file, "# Sanitize sensitive values and retain exact raw frames before promotion."); err != nil {
		return err
	}
	frameState := newCaptureFrameState()
	for _, event := range replayEvents {
		switch event.Kind {
		case capturelog.EventConnect:
			if err := frameState.connect(event.Leg); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(file, "# connect leg=%d\n", event.Leg); err != nil {
				return err
			}
		case capturelog.EventDisconnect:
			if err := frameState.disconnect(event.Leg); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(file, "disconnect"); err != nil {
				return err
			}
		case capturelog.ReplayEventFrame:
			payload, err := base64.StdEncoding.DecodeString(event.Data)
			if err != nil {
				return fmt.Errorf("decode replay frame: %w", err)
			}
			description, err := frameState.describe(event, payload)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(file, "# leg=%d direction=%s msg_id=%s encoding=%s payload_len=%d\n", event.Leg, event.Direction, description.messageID(), description.encoding, len(payload)); err != nil {
				return err
			}
			frame, err := frameBytes(payload)
			if err != nil {
				return fmt.Errorf("frame replay payload: %w", err)
			}
			if _, err := fmt.Fprintf(file, "raw %s %s\n", event.Direction, base64.StdEncoding.EncodeToString(frame)); err != nil {
				return err
			}
		}
	}
	return nil
}

type captureFrameState struct {
	legs map[int]captureLegState
}

type captureLegState struct {
	phase         captureLegPhase
	minVersion    int
	maxVersion    int
	serverVersion int
}

type captureLegPhase uint8

const (
	awaitVersionRange captureLegPhase = iota + 1
	awaitServerInfo
	sessionFrames
)

type frameDescription struct {
	label         string
	msgID         int
	encoding      string
	serverVersion int
	session       bool
}

func (d frameDescription) messageID() string {
	if d.label != "" {
		return d.label
	}
	return strconv.Itoa(d.msgID)
}

func newCaptureFrameState() captureFrameState {
	return captureFrameState{legs: make(map[int]captureLegState)}
}

func (s *captureFrameState) connect(leg int) error {
	if _, ok := s.legs[leg]; ok {
		return fmt.Errorf("describe leg %d: duplicate connect", leg)
	}
	s.legs[leg] = captureLegState{phase: awaitVersionRange}
	return nil
}

func (s *captureFrameState) disconnect(leg int) error {
	if _, ok := s.legs[leg]; !ok {
		return fmt.Errorf("describe leg %d: disconnect before connect", leg)
	}
	delete(s.legs, leg)
	return nil
}

func (s *captureFrameState) describe(event capturelog.ReplayEvent, payload []byte) (frameDescription, error) {
	leg, ok := s.legs[event.Leg]
	if !ok {
		return frameDescription{}, fmt.Errorf("describe leg %d: frame before connect", event.Leg)
	}
	switch leg.phase {
	case awaitVersionRange:
		if event.Direction != "client" {
			return frameDescription{}, fmt.Errorf("describe leg %d: want client version range, got %s frame", event.Leg, event.Direction)
		}
		minVersion, maxVersion, err := decodeVersionRange(payload)
		if err != nil {
			return frameDescription{}, fmt.Errorf("describe leg %d version range: %w", event.Leg, err)
		}
		leg.phase = awaitServerInfo
		leg.minVersion = minVersion
		leg.maxVersion = maxVersion
		s.legs[event.Leg] = leg
		return frameDescription{label: "version_range", encoding: "pre_session"}, nil
	case awaitServerInfo:
		if event.Direction != "server" {
			return frameDescription{}, fmt.Errorf("describe leg %d: want server info, got %s frame", event.Leg, event.Direction)
		}
		info, err := codec.DecodeServerInfo(payload)
		if err != nil {
			return frameDescription{}, fmt.Errorf("describe leg %d server-info frame: %w", event.Leg, err)
		}
		if info.ServerVersion < leg.minVersion || info.ServerVersion > leg.maxVersion {
			return frameDescription{}, fmt.Errorf(
				"describe leg %d: server version %d outside requested range %d..%d",
				event.Leg,
				info.ServerVersion,
				leg.minVersion,
				leg.maxVersion,
			)
		}
		leg.phase = sessionFrames
		leg.serverVersion = info.ServerVersion
		s.legs[event.Leg] = leg
		return frameDescription{label: "server_info", encoding: "pre_session", serverVersion: info.ServerVersion}, nil
	case sessionFrames:
		return describeSessionFrame(event, payload, leg.serverVersion)
	default:
		return frameDescription{}, fmt.Errorf("describe leg %d: invalid capture phase %d", event.Leg, leg.phase)
	}
}

func describeSessionFrame(event capturelog.ReplayEvent, payload []byte, serverVersion int) (frameDescription, error) {
	envelope, err := protocol.DecodeEnvelope(serverVersion, payload)
	if err != nil {
		return frameDescription{}, fmt.Errorf("describe leg %d %s frame: %w", event.Leg, event.Direction, err)
	}
	encoding := "classic"
	if envelope.Encoding == protocol.ProtobufBody {
		encoding = "protobuf"
	}
	return frameDescription{
		msgID:         envelope.MsgID,
		encoding:      encoding,
		serverVersion: serverVersion,
		session:       true,
	}, nil
}

func decodeVersionRange(payload []byte) (int, int, error) {
	value := string(payload)
	if !strings.HasPrefix(value, "v") {
		return 0, 0, fmt.Errorf("invalid payload %q", value)
	}
	minText, maxText, ok := strings.Cut(value[1:], "..")
	if !ok || minText == "" || maxText == "" {
		return 0, 0, fmt.Errorf("invalid payload %q", value)
	}
	minVersion, err := strconv.Atoi(minText)
	if err != nil || minVersion <= 0 {
		return 0, 0, fmt.Errorf("invalid minimum %q", minText)
	}
	maxVersion, err := strconv.Atoi(maxText)
	if err != nil || maxVersion <= 0 {
		return 0, 0, fmt.Errorf("invalid maximum %q", maxText)
	}
	if minVersion > maxVersion {
		return 0, 0, fmt.Errorf("minimum %d exceeds maximum %d", minVersion, maxVersion)
	}
	return minVersion, maxVersion, nil
}

func frameBytes(payload []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := wire.WriteFrame(&out, payload); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
