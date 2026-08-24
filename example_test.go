package ibkr_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/testhost"
	"github.com/shopspring/decimal"
)

func ExampleDialContext() {
	client, cleanup := exampleClient("grounded_bootstrap.txt")
	defer cleanup()

	snapshot := client.Session()
	fmt.Println(snapshot.State, snapshot.ManagedAccounts)
	// Output:
	// Ready [DU9000001]
}

func Example_contractDetails() {
	client, cleanup := exampleClient("grounded_contract_details_aapl.txt")
	defer cleanup()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(details[0].Symbol, details[0].MinTick)
	// Output:
	// AAPL 0.01
}

func Example_historicalBars() {
	client, cleanup := exampleClient("historical_bars_subscription_required.txt")
	defer cleanup()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
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
	apiErr, _ := errors.AsType[*ibkr.APIError](err)
	fmt.Println(apiErr.Code, apiErr.IsEntitlement())
	// Output:
	// 2188 true
}

func Example_accountSummary() {
	client, cleanup := exampleClient("grounded_account_summary.txt")
	defer cleanup()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags: []string{
			"NetLiquidation",
			"TotalCashValue",
			"BuyingPower",
			"ExcessLiquidity",
		},
	})
	if err != nil {
		panic(err)
	}

	for _, value := range values {
		fmt.Println(value.Tag, value.Value, value.Currency)
	}
	// Output:
	// BuyingPower 183875.22 EUR
	// ExcessLiquidity 28591.69 EUR
	// NetLiquidation 36992.49 EUR
	// TotalCashValue -1441.67 EUR
}

func Example_positionsSnapshot() {
	client, cleanup := exampleClient("grounded_positions.txt")
	defer cleanup()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		panic(err)
	}

	for _, position := range positions {
		fmt.Println(position.Contract.Symbol, position.Position)
	}
	// Output:
	// ASML 40
	// ADYEN 100
	// MES 0
	// ASML 30
	// NXE 8188
	// CRDO 753
	// DHER 1568
	// HAG 1556
}

func Example_placeOrder() {
	client, cleanup := exampleClient("direct_cancel_order.txt")
	defer cleanup()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.ActionBuy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  new(decimal.RequireFromString("10")),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
			OrderRef:  "sanitized-order-ref-0000000000000001",
		},
	})
	if err != nil {
		panic(err)
	}

	for event := range handle.Events() {
		if event.Status != nil && event.Status.Status == ibkr.OrderStatusPreSubmitted {
			break
		}
	}
	if err := handle.Cancel(ctx); err != nil {
		panic(err)
	}
	for event := range handle.Events() {
		if event.Status != nil && event.Status.Status == ibkr.OrderStatusCancelled {
			handle.Close()
			break
		}
	}
	if err := handle.Wait(); err != nil {
		panic(err)
	}
	fmt.Println("order", handle.OrderID(), "cancelled")
	// Output:
	// order 506 cancelled
}

func Example_qualifyContract() {
	client, cleanup := exampleClient("grounded_contract_details_aapl.txt")
	defer cleanup()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	details, err := client.Contracts().Qualify(ctx, ibkr.Contract{
		Symbol:   "AAPL",
		SecType:  ibkr.SecTypeStock,
		Exchange: "SMART",
		Currency: "USD",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(details.ConID, details.LongName)
	// Output:
	// 265598 APPLE INC
}

func Example_historicalSchedule() {
	client, cleanup := exampleClient("historical_schedule_aapl.txt")
	defer cleanup()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	schedule, err := client.History().Schedule(ctx, ibkr.HistoricalScheduleRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration: ibkr.Months(1),
		BarSize:  ibkr.Bar1Day,
		UseRTH:   true,
	})
	if err != nil {
		panic(err)
	}

	first := schedule.Sessions[0]
	last := schedule.Sessions[len(schedule.Sessions)-1]
	fmt.Println(schedule.TimeZone, len(schedule.Sessions), "sessions")
	fmt.Println(first.RefDate, "through", last.RefDate)
	// Output:
	// US/Eastern 21 sessions
	// 20260727 through 20260824
}

func Example_awaitSnapshot() {
	client, cleanup := exampleClient("grounded_account_summary.txt")
	defer cleanup()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
		Group: "All",
		Tags: []string{
			"NetLiquidation",
			"TotalCashValue",
			"BuyingPower",
			"ExcessLiquidity",
		},
	})
	if err != nil {
		panic(err)
	}
	defer sub.Close()

	// AwaitSnapshot remains reliable even when its lifecycle notification was
	// dropped from a bounded channel.
	if err := sub.AwaitSnapshot(ctx); err != nil {
		panic(err)
	}
	fmt.Println("snapshot complete")
	// Output:
	// snapshot complete
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

func ExampleOptionContract() {
	c := ibkr.OptionContract("AAPL", "20260320", decimal.RequireFromString("150"), ibkr.RightCall)
	fmt.Println(c.Symbol, c.Expiry, c.Strike, c.Right, c.Multiplier)
	// Output:
	// AAPL 20260320 150 C 100
}

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

// exampleClient replays a sanitized transcript captured from a live Gateway.
// Cleanup verifies that the example consumed the complete scenario.
func exampleClient(transcript string) (*ibkr.Client, func()) {
	host, err := testhost.NewFromFile("testdata/transcripts/" + transcript)
	if err != nil {
		panic(err)
	}

	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		panic(err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	)
	if err != nil {
		cancel()
		_ = host.Close()
		panic(err)
	}

	cleanup := func() {
		if err := host.Wait(); err != nil {
			client.Close()
			cancel()
			_ = host.Close()
			panic(err)
		}
		client.Close()
		cancel()
		_ = host.Close()
	}
	return client, cleanup
}
