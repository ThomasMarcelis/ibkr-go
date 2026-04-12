// Connect to IB Gateway or TWS, print session info, and disconnect.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/connect
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
)

func main() {
	host, port := "127.0.0.1", 4002
	if addr := os.Getenv("IBKR_ADDR"); addr != "" {
		parts := strings.SplitN(addr, ":", 2)
		host = parts[0]
		if len(parts) == 2 {
			p, err := strconv.Atoi(parts[1])
			if err != nil {
				log.Fatalf("invalid port in IBKR_ADDR: %v", err)
			}
			port = p
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	snap := client.Session()
	fmt.Println("state:           ", snap.State)
	fmt.Println("server version:  ", snap.ServerVersion)
	fmt.Println("managed accounts:", snap.ManagedAccounts)
	fmt.Println("next valid ID:   ", snap.NextValidID)

	serverTime, err := client.CurrentTime(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("server time:     ", serverTime.Format(time.RFC3339))
}
