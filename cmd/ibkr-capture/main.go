// Command ibkr-capture drives live capture scenarios against a running IB
// Gateway or TWS. It performs the real TWS handshake and START_API, then
// runs one of several named scenarios that send a feature request and read
// replies for a bounded time. Intended to be pointed at the ibkr-recorder
// listen address so that the full bidirectional traffic is captured to disk.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"syscall"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4101", "gateway or recorder listen address")
	clientID := flag.Int("client-id", 1, "TWS API client id sent in START_API")
	scenario := flag.String("scenario", "bootstrap", "scenario name (use -list to see all)")
	listScenarios := flag.Bool("list", false, "list available scenarios and exit")
	listJSON := flag.Bool("list-json", false, "list available scenarios as JSON and exit")
	listBatch := flag.String("list-batch", "", "list scenario|client_id entries for a batch and exit")
	roleFor := flag.Bool("role-for", false, "print capture role for positional scenario names and exit")
	driverEvents := flag.String("driver-events", os.Getenv("IBKR_DRIVER_EVENTS"), "optional JSONL path for public API driver events")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)

	if *listScenarios {
		names := make([]string, 0, len(scenarios))
		for n := range scenarios {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Printf("  %-40s  %s\n", n, scenarios[n].description)
		}
		return
	}
	if *listJSON {
		if err := writeCatalogJSON(os.Stdout); err != nil {
			log.Fatalf("list-json: %v", err)
		}
		return
	}
	if *listBatch != "" {
		if err := writeBatchList(os.Stdout, *listBatch); err != nil {
			log.Fatalf("list-batch: %v", err)
		}
		return
	}
	if *roleFor {
		if err := writeScenarioRole(os.Stdout, flag.Args()); err != nil {
			log.Fatalf("role-for: %v", err)
		}
		return
	}

	sc, ok := scenarios[*scenario]
	if !ok {
		log.Fatalf("unknown scenario %q; use -list to see available", *scenario)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	recorder, err := newAPIDriverRecorder(*driverEvents, *scenario, sc)
	if err != nil {
		log.Fatalf("driver-events: %v", err)
	}
	apiDriver = recorder
	defer func() {
		if err := recorder.Close(); err != nil {
			log.Printf("close driver events: %v", err)
		}
	}()
	if err := sc.run(ctx, *addr, *clientID); err != nil {
		log.Fatalf("scenario %q: %v", *scenario, err)
	}
	log.Printf("scenario %q complete", *scenario)
}
