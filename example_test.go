package ibkr_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func ExampleDialContext() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
		ibkr.WithClientID(1),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	snapshot := client.Session()
	fmt.Println(snapshot.State, snapshot.ServerVersion, snapshot.ManagedAccounts)
}

func ExampleContractsClient_Details() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	details, err := client.Contracts().Details(ctx, ibkr.Stock("AAPL"))
	if err != nil {
		log.Print(err)
		return
	}
	for _, detail := range details {
		fmt.Println(detail.ConID, detail.LongName, detail.MinTick)
	}
}

func ExampleContractsClient_Qualify() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	details, err := client.Contracts().Qualify(ctx, ibkr.Stock("AAPL"))
	if err != nil {
		log.Print(err)
		return
	}
	fmt.Println(details.ConID, details.LongName)
}

func ExampleAccountsClient_Summary() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "TotalCashValue"},
	})
	if err != nil {
		log.Print(err)
		return
	}
	for _, value := range values {
		fmt.Println(value.Tag, value.Value, value.Currency)
	}
}

func ExampleAccountsClient_Positions() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		log.Print(err)
		return
	}
	for _, position := range positions {
		fmt.Println(position.Contract.Symbol, position.Position, position.AvgCost)
	}
}

func ExampleHistoryClient_Bars() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract:   ibkr.Stock("AAPL"),
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		if apiErr, ok := errors.AsType[*ibkr.APIError](err); ok && apiErr.IsEntitlement() {
			log.Printf("historical data needs market-data permissions: %v", apiErr)
		} else {
			log.Print(err)
		}
		return
	}
	for _, bar := range bars {
		fmt.Println(bar.Time, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
	}
}

func ExampleHistoryClient_Schedule() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	schedule, err := client.History().Schedule(ctx, ibkr.HistoricalScheduleRequest{
		Contract: ibkr.Stock("AAPL"),
		Duration: ibkr.Months(1),
		BarSize:  ibkr.Bar1Day,
		UseRTH:   true,
	})
	if err != nil {
		log.Print(err)
		return
	}
	fmt.Println("time zone:", schedule.TimeZone)
	for _, session := range schedule.Sessions {
		fmt.Println(session.RefDate, session.StartDateTime, session.EndDateTime)
	}
}

func ExampleMarketDataClient_SubscribeQuotes() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	// Request delayed data where available without a market-data subscription.
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
		log.Print(err)
		return
	}
	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Stock("AAPL"),
	})
	if err != nil {
		log.Print(err)
		return
	}
	defer sub.Close()

	// Fields arrive separately. All yields data; use Events for notices and gaps.
	want := ibkr.QuoteFieldBid | ibkr.QuoteFieldAsk
	for update := range sub.All(ctx) {
		q := update.Snapshot
		if q.Available&want == want {
			fmt.Println(q.Bid, q.Ask, q.MarketDataType)
			break
		}
	}
	sub.Close()
	if err := errors.Join(sub.Wait(), context.Cause(ctx)); err != nil {
		log.Print(err)
	}
}

func ExampleSubscription_AwaitSnapshot() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost("127.0.0.1"),
		ibkr.WithPort(4002),
	)
	if err != nil {
		log.Print(err)
		return
	}
	defer client.Close()

	sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags:  []string{"NetLiquidation", "TotalCashValue"},
	})
	if err != nil {
		log.Print(err)
		return
	}
	defer sub.Close()

	// AwaitSnapshot does not drain events. Start the single consumer first.
	// Use Accounts().Summary when only the initial snapshot is needed.
	consumed := make(chan struct{})
	go func() {
		defer close(consumed)
		for event := range sub.Events() {
			if event.Kind == ibkr.StreamData {
				fmt.Println(event.Value.Tag, event.Value.Value)
			}
		}
	}()
	if err := sub.AwaitSnapshot(ctx); err != nil {
		log.Print(err)
	} else {
		fmt.Println("snapshot complete")
	}

	// Completion does not guarantee that the stream remains healthy.
	sub.Close()
	<-consumed
	if err := sub.Wait(); err != nil {
		log.Print(err)
	}
}

func ExampleStock() {
	c := ibkr.Stock("AAPL")
	fmt.Println(c.Symbol, c.SecType, c.Exchange, c.Currency)
	// Output:
	// AAPL STK SMART USD
}

func ExampleForex() {
	c, err := ibkr.Forex("EURUSD")
	if err != nil {
		panic(err)
	}
	fmt.Println(c.Symbol, c.Currency, c.SecType, c.Exchange)
	// Output:
	// EUR USD CASH IDEALPRO
}

// The fixed expiry illustrates the fields; select a current expiry with
// Contracts().SecDefOptParams for a live request.
func ExampleOptionContract() {
	c := ibkr.OptionContract("AAPL", "20260320", decimal.RequireFromString("150"), ibkr.RightCall)
	fmt.Println(c.Symbol, c.Expiry, c.Strike, c.Right, c.Multiplier)
	// Output:
	// AAPL 20260320 150 C 100
}

// The fixed contract month illustrates the fields; qualify the contract
// before requesting data for a current expiry.
func ExampleFuture() {
	c := ibkr.Future("ES", "202609", "CME")
	fmt.Println(c.Symbol, c.Expiry, c.Exchange, c.Currency)
	// Output:
	// ES 202609 CME USD
}

func ExampleMarketOrder() {
	o := ibkr.MarketOrder(ibkr.ActionBuy, decimal.NewFromInt(10))
	fmt.Println(o.Action, o.OrderType, o.Quantity)
	// Output:
	// BUY MKT 10
}

func ExampleLimitOrder() {
	o := ibkr.LimitOrder(ibkr.ActionBuy, decimal.NewFromInt(10), decimal.RequireFromString("150.00"))
	fmt.Println(o.Action, o.OrderType, o.Quantity, o.LmtPrice)
	// Output:
	// BUY LMT 10 150
}

func ExampleStopOrder() {
	o := ibkr.StopOrder(ibkr.ActionSell, decimal.NewFromInt(10), decimal.RequireFromString("140.00"))
	fmt.Println(o.Action, o.OrderType, o.Quantity, o.AuxPrice)
	// Output:
	// SELL STP 10 140
}

func ExampleStopLimitOrder() {
	o := ibkr.StopLimitOrder(ibkr.ActionSell, decimal.NewFromInt(10), decimal.RequireFromString("140.00"), decimal.RequireFromString("139.50"))
	fmt.Println(o.Action, o.OrderType, o.Quantity, o.AuxPrice, o.LmtPrice)
	// Output:
	// SELL STP LMT 10 140 139.5
}
