package main

import (
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
}
