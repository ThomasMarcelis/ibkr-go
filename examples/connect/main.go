// Connect to IB Gateway or TWS, print session info, and disconnect.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/connect
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/examples/internal/exampleutil"
)

func main() {
	exampleutil.Run(run)
}

func run() (err error) {
	host, port, err := exampleutil.GatewayAddress()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		return err
	}
	defer client.Close()

	snap := client.Session()
	fmt.Println("state:           ", snap.State)
	fmt.Println("server version:  ", snap.ServerVersion)
	fmt.Println("managed accounts:", snap.ManagedAccounts)
	fmt.Println("next valid ID:   ", snap.NextValidID)

	serverTime, err := client.CurrentTime(ctx)
	if err != nil {
		return err
	}
	fmt.Println("server time:     ", serverTime.Format(time.RFC3339))
	return nil
}
