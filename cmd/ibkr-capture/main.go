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
	"net"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4101", "gateway or recorder listen address")
	clientID := flag.Int("client-id", 1, "TWS API client id sent in START_API")
	minVer := flag.Int("min-version", 100, "handshake minimum version")
	maxVer := flag.Int("max-version", 200, "handshake maximum version")
	scenario := flag.String("scenario", "bootstrap", "scenario name (use -list to see all)")
	listScenarios := flag.Bool("list", false, "list available scenarios and exit")
	dialTimeout := flag.Duration("dial-timeout", 5*time.Second, "tcp dial timeout")
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

	sc, ok := scenarios[*scenario]
	if !ok {
		log.Fatalf("unknown scenario %q; use -list to see available", *scenario)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	conn, err := net.DialTimeout("tcp", *addr, *dialTimeout)
	if err != nil {
		log.Fatalf("dial %s: %v", *addr, err)
	}
	defer conn.Close()
	log.Printf("connected to %s", *addr)

	sess, err := bootstrap(conn, *clientID, *minVer, *maxVer)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	log.Printf("bootstrap complete: server_version=%d account=%s next_valid_id=%d",
		sess.ServerVersion, sess.ManagedAccounts, sess.NextValidID)

	if err := sc.run(ctx, conn, sess); err != nil {
		log.Fatalf("scenario %q: %v", *scenario, err)
	}
	log.Printf("scenario %q complete", *scenario)
}
