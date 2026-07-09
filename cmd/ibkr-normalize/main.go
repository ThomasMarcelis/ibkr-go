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

	"github.com/ThomasMarcelis/ibkr-go/internal/capturelog"
	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
	"github.com/ThomasMarcelis/ibkr-go/internal/wire"
)

func main() {
	dir := flag.String("dir", "", "capture directory containing events.jsonl")
	rawOut := flag.String("out", "", "raw output file path (default: <dir>/raw.txt)")
	replayDir := flag.String("replay-dir", "", "directory for normalized replay artifacts (default: <dir>/replay)")
	transcriptOut := flag.String("transcript-out", "", "optional raw transcript skeleton output path")
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

	if err := capturelog.WriteReplay(*replayDir, *dir, meta, events); err != nil {
		log.Fatalf("write replay: %v", err)
	}
	if *transcriptOut != "" {
		if err := writeTranscriptSkeleton(*transcriptOut, meta, events); err != nil {
			log.Fatalf("write transcript skeleton: %v", err)
		}
	}
}

func writeTranscriptSkeleton(path string, meta capturelog.Meta, events []capturelog.Event) error {
	replayEvents, err := capturelog.NormalizeEvents(events)
	if err != nil {
		return err
	}
	// #nosec G304 -- path is the operator-selected transcript output.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create transcript skeleton: %w", err)
	}
	defer file.Close()

	if _, err := fmt.Fprintf(file, "# Raw transcript skeleton for %s.\n", meta.Scenario); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(file, "# Curate raw steps into typed client/server lines before promotion."); err != nil {
		return err
	}
	frameState := transcriptFrameState{serverVersions: make(map[int]int)}
	for _, event := range replayEvents {
		switch event.Kind {
		case capturelog.EventConnect:
			if _, err := fmt.Fprintf(file, "# connect leg=%d\n", event.Leg); err != nil {
				return err
			}
		case capturelog.EventDisconnect:
			if _, err := fmt.Fprintln(file, "disconnect"); err != nil {
				return err
			}
		case capturelog.ReplayEventFrame:
			payload, err := base64.StdEncoding.DecodeString(event.Data)
			if err != nil {
				return fmt.Errorf("decode replay frame: %w", err)
			}
			msgID, encoding, err := frameState.describe(event, payload)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(file, "# leg=%d direction=%s msg_id=%s encoding=%s payload_len=%d\n", event.Leg, event.Direction, msgID, encoding, len(payload)); err != nil {
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

type transcriptFrameState struct {
	serverVersions map[int]int
}

func (s *transcriptFrameState) describe(event capturelog.ReplayEvent, payload []byte) (string, string, error) {
	serverVersion := s.serverVersions[event.Leg]
	if serverVersion == 0 {
		if event.Direction == "client" {
			return "version_range", "pre_session", nil
		}
		info, err := codec.DecodeServerInfo(payload)
		if err != nil {
			return "", "", fmt.Errorf("describe leg %d server-info frame: %w", event.Leg, err)
		}
		s.serverVersions[event.Leg] = info.ServerVersion
		return "server_info", "pre_session", nil
	}

	envelope, err := protocol.DecodeEnvelope(serverVersion, payload)
	if err != nil {
		return "", "", fmt.Errorf("describe leg %d %s frame: %w", event.Leg, event.Direction, err)
	}
	encoding := "classic"
	if envelope.Encoding == protocol.ProtobufBody {
		encoding = "protobuf"
	}
	return fmt.Sprintf("%d", envelope.MsgID), encoding, nil
}

func frameBytes(payload []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := wire.WriteFrame(&out, payload); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
