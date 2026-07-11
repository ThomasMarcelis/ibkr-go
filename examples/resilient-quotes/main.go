// Stream delayed AAPL quotes across automatic reconnects until interrupted.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/resilient-quotes
package main

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/examples/internal/exampleutil"
)

func main() {
	exampleutil.Run(run)
}

func run() error {
	host, port, err := exampleutil.GatewayAddress()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dialCtx, cancelDial := context.WithTimeout(ctx, 15*time.Second)
	client, err := ibkr.DialContext(dialCtx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectAuto),
		ibkr.WithTCPKeepAlive(30*time.Second),
	)
	cancelDial()
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		return err
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Stock("AAPL"),
	}, ibkr.WithResumePolicy(ibkr.ResumeAuto))
	if err != nil {
		return err
	}

	want := ibkr.QuoteFieldBid | ibkr.QuoteFieldAsk
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				return sub.Wait()
			}
			if event.Kind != ibkr.StreamData {
				fmt.Printf("lifecycle  %-12s connection=%d\n",
					event.Kind, event.ConnectionSeq)
				continue
			}
			update := event.Value
			if update.Changed&want != 0 && update.Snapshot.Available&want == want {
				fmt.Printf("quote      bid=%s ask=%s\n",
					update.Snapshot.Bid, update.Snapshot.Ask)
			}
		case <-ctx.Done():
			sub.Close()
			return errors.Join(context.Cause(ctx), sub.Wait())
		}
	}
}
