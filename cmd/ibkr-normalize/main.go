package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ThomasMarcelis/ibkr-go/internal/capturelog"
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

	file, err := os.Create(*rawOut)
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
	file, err := os.Create(path)
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
			msgID := "server_info"
			fields := splitPayloadFields(payload)
			if len(fields) > 0 {
				msgID = fields[0]
			}
			if _, err := fmt.Fprintf(file, "# leg=%d direction=%s msg_id=%s payload_len=%d\n", event.Leg, event.Direction, msgID, len(payload)); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(file, "raw %s %s\n", event.Direction, base64.StdEncoding.EncodeToString(frameBytes(payload))); err != nil {
				return err
			}
		}
	}
	return nil
}

func frameBytes(payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(out[:4], uint32(len(payload)))
	copy(out[4:], payload)
	return out
}

func splitPayloadFields(payload []byte) []string {
	var fields []string
	start := 0
	for i, b := range payload {
		if b != 0 {
			continue
		}
		fields = append(fields, string(payload[start:i]))
		start = i + 1
	}
	if start < len(payload) {
		fields = append(fields, string(payload[start:]))
	}
	return fields
}
