// Fetch one day of hourly historical bars for AAPL and print them.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 go run ./examples/historical
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		return err
	}
	defer client.Close()

	bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%-20s %10s %10s %10s %10s %12s\n",
		"time", "open", "high", "low", "close", "volume")
	for _, bar := range bars {
		fmt.Printf("%-20s %10s %10s %10s %10s %12s\n",
			bar.Time.Format("2006-01-02 15:04"),
			bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
	}
	return nil
}
