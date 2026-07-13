package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
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

	if err := runNormalize(os.Stdout, *dir, *rawOut, *replayDir, *transcriptOut, *verify); err != nil {
		log.Fatal(err)
	}
}

func runNormalize(out io.Writer, dir, rawOut, replayDir, transcriptOut string, verify bool) error {
	if dir == "" {
		return fmt.Errorf("-dir is required")
	}

	events, err := capturelog.LoadEvents(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		return fmt.Errorf("load events: %w", err)
	}
	meta, err := capturelog.LoadMeta(filepath.Join(dir, "meta.json"))
	if err != nil {
		return fmt.Errorf("load meta: %w", err)
	}
	replayEvents, err := capturelog.NormalizeEvents(events)
	if err != nil {
		return fmt.Errorf("normalize events: %w", err)
	}
	if verify {
		if transcriptOut != "" || rawOut != "" || replayDir != "" {
			return fmt.Errorf("-verify cannot be combined with output flags")
		}
		return writeVerification(out, dir, meta, events, replayEvents)
	}
	if rawOut == "" {
		rawOut = filepath.Join(dir, "raw.txt")
	}
	if replayDir == "" {
		replayDir = filepath.Join(dir, "replay")
	}

	file, err := os.OpenFile(rawOut, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- the caller explicitly selects this CLI output path.
	if err != nil {
		return fmt.Errorf("create raw output: %w", err)
	}

	for _, event := range events {
		kind := event.Kind
		if kind == "" {
			kind = capturelog.EventChunk
		}
		line := fmt.Sprintf("%s leg=%d kind=%s", event.At.Format("2006-01-02T15:04:05.000000000Z07:00"), event.Leg, kind)
		if kind == capturelog.EventChunk {
			data, err := capturelog.DecodeData(event)
			if err != nil {
				_ = file.Close()
				return fmt.Errorf("decode event: %w", err)
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
			_ = file.Close()
			return fmt.Errorf("write raw output: %w", err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close raw output: %w", err)
	}

	if err := capturelog.WriteReplay(replayDir, dir, meta, replayEvents); err != nil {
		return fmt.Errorf("write replay: %w", err)
	}
	if transcriptOut != "" {
		if err := writeTranscriptSkeleton(transcriptOut, meta, replayEvents); err != nil {
			return fmt.Errorf("write transcript skeleton: %w", err)
		}
	}
	return nil
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
