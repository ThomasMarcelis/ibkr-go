package ibkr_test

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/ibkr"
	"github.com/ThomasMarcelis/ibkr-go/testing/testhost"
)

func ExampleDialContext() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
sleep 2s
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	snapshot := client.Session()
	fmt.Println(snapshot.State, snapshot.ManagedAccounts)
	// Output:
	// Ready [DU12345]
}

func ExampleClient_ContractDetails() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
client req_contract_details {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","primary_exchange":"","local_symbol":""}}
server contract_details {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","con_id":265598,"primary_exchange":"NASDAQ","local_symbol":"AAPL","trading_class":"NMS"},"market_name":"NMS","min_tick":"0.01","long_name":"APPLE INC","time_zone_id":"US/Eastern"}
server contract_details_end {"req_id":"$req1"}
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	details, err := client.ContractDetails(ctx, ibkr.Contract{
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

func ExampleClient_SubscribeQuotes() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
client req_quote {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","primary_exchange":"","local_symbol":""},"snapshot":false,"generic_ticks":[]}
server tick_price {"req_id":"$req1","field":1,"price":"189.10"}
server tick_price {"req_id":"$req1","field":2,"price":"189.15"}
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	sub, err := client.SubscribeQuotes(ctx, ibkr.QuoteSubscriptionRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		panic(err)
	}
	defer sub.Close()

	first := <-sub.Events()
	second := <-sub.Events()
	fmt.Println(first.Snapshot.Bid, second.Snapshot.Ask)
	// Output:
	// 189.1 189.15
}

func ExampleClient_HistoricalBars() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
client req_historical_bars {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","primary_exchange":"","local_symbol":""},"end_time":"20260405-12:00:00","duration":"1 D","bar_size":"1 hour","what_to_show":"TRADES","use_rth":true}
server historical_bar {"req_id":"$req1","time":"2026-04-05T10:00:00Z","open":"100.0","high":"101.0","low":"99.5","close":"100.5","volume":"1000"}
server historical_bar {"req_id":"$req1","time":"2026-04-05T11:00:00Z","open":"100.5","high":"102.0","low":"100.0","close":"101.5","volume":"1500"}
server historical_bars_end {"req_id":"$req1","start":"2026-04-05T10:00:00Z","end":"2026-04-05T11:00:00Z"}
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	bars, err := client.HistoricalBars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		EndTime:    time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		Duration:   24 * time.Hour,
		BarSize:    time.Hour,
		WhatToShow: "TRADES",
		UseRTH:     true,
	})
	if err != nil {
		panic(err)
	}

	for _, bar := range bars {
		fmt.Println(bar.Close, bar.Volume)
	}
	// Output:
	// 100.5 1000
	// 101.5 1500
}

func ExampleClient_AccountSummary() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
client req_account_summary {"req_id":"$req1","account":"All","tags":["NetLiquidation"]}
server account_summary {"req_id":"$req1","account":"DU12345","tag":"NetLiquidation","value":"100000.00","currency":"USD"}
server account_summary_end {"req_id":"$req1"}
client cancel_account_summary {"req_id":"$req1"}
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	values, err := client.AccountSummary(ctx, ibkr.AccountSummaryRequest{
		Account: "All",
		Tags:    []string{"NetLiquidation"},
	})
	if err != nil {
		panic(err)
	}

	for _, v := range values {
		fmt.Println(v.Tag, v.Value, v.Currency)
	}
	// Output:
	// NetLiquidation 100000.00 USD
}

func ExampleClient_PositionsSnapshot() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
client req_positions {}
server position {"account":"DU12345","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","primary_exchange":"","local_symbol":""},"position":"10","avg_cost":"189.12"}
server position_end {}
client cancel_positions {}
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	positions, err := client.PositionsSnapshot(ctx)
	if err != nil {
		panic(err)
	}

	for _, p := range positions {
		fmt.Println(p.Contract.Symbol, p.Position, p.AvgCost)
	}
	// Output:
	// AAPL 10 189.12
}

func ExampleClient_PlaceOrder() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU9000001"]}
server next_valid_id {"order_id":1}
server api_error {"req_id":-1,"code":2104,"message":"Market data farm connection is OK:usfarm","advanced_order_reject_json":"","error_time_ms":""}
client place_order {"order_id":"1","contract":{"con_id":"265598","symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD"},"action":"BUY","total_quantity":"1","order_type":"LMT","lmt_price":"150","tif":"DAY","account":"DU9000001"}
sleep 100ms
server open_order {"order_id":1,"account":"DU9000001","contract":{"con_id":265598,"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","local_symbol":"AAPL","trading_class":"NMS"},"action":"BUY","order_type":"LMT","status":"PreSubmitted","quantity":"1","filled":"0","remaining":"1","lmt_price":"150.00","aux_price":"0","tif":"DAY","perm_id":"12345","client_id":"1","origin":"0"}
server order_status {"order_id":1,"status":"PreSubmitted","filled":"0","remaining":"1","avg_fill_price":"0","perm_id":"12345","parent_id":"0","last_fill_price":"0","client_id":"1","why_held":"","mkt_cap_price":"0"}
sleep 200ms
server order_status {"order_id":1,"status":"Filled","filled":"1","remaining":"0","avg_fill_price":"149.50","perm_id":"12345","parent_id":"0","last_fill_price":"149.50","client_id":"1","why_held":"","mkt_cap_price":"0"}
sleep 100ms
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	handle, err := client.PlaceOrder(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: "LMT",
			Quantity:  ibkr.MustParseDecimal("1"),
			LmtPrice:  ibkr.MustParseDecimal("150"),
			TIF:       ibkr.TIFDay,
			Account:   "DU9000001",
		},
	})
	if err != nil {
		panic(err)
	}

	// Wait for the order to reach a terminal state.
	err = handle.Wait()
	fmt.Println("order", handle.OrderID(), "done, err:", err)
	// Output:
	// order 1 done, err: <nil>
}

func ExampleClient_QualifyContract() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
client req_contract_details {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","primary_exchange":"","local_symbol":""}}
server contract_details {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","con_id":265598,"primary_exchange":"NASDAQ","local_symbol":"AAPL","trading_class":"NMS"},"market_name":"NMS","min_tick":"0.01","long_name":"APPLE INC","time_zone_id":"US/Eastern"}
server contract_details_end {"req_id":"$req1"}
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	details, err := client.QualifyContract(ctx, ibkr.Contract{
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

func ExampleClient_SubscribeRealTimeBars() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
client req_realtime_bars {"req_id":"$req1","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","primary_exchange":"","local_symbol":""},"what_to_show":"TRADES","use_rth":true}
server realtime_bar {"req_id":"$req1","time":"2026-04-05T12:00:00Z","open":"189.00","high":"189.20","low":"188.95","close":"189.10","volume":"100"}
server realtime_bar {"req_id":"$req1","time":"2026-04-05T12:00:05Z","open":"189.10","high":"189.25","low":"189.05","close":"189.20","volume":"125"}
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	sub, err := client.SubscribeRealTimeBars(ctx, ibkr.RealTimeBarsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		WhatToShow: "TRADES",
		UseRTH:     true,
	})
	if err != nil {
		panic(err)
	}
	defer sub.Close()

	first := <-sub.Events()
	second := <-sub.Events()
	fmt.Println(first.Close, first.Volume)
	fmt.Println(second.Close, second.Volume)
	// Output:
	// 189.1 100
	// 189.2 125
}

func ExampleClient_SubscribeOpenOrders() {
	client, host, cancel := exampleClient(`
handshake {"server_version":200,"connection_time":"2026-04-06T12:00:00Z"}
server managed_accounts {"accounts":["DU12345"]}
server next_valid_id {"order_id":1001}
client req_open_orders {"scope":"all"}
server open_order {"order_id":2001,"account":"DU12345","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD","primary_exchange":"","local_symbol":""},"action":"BUY","order_type":"LMT","status":"Submitted","quantity":"10","filled":"2","remaining":"8"}
server open_order_end {}
`)
	defer cancel()
	defer client.Close()
	defer host.Close()

	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	orders, err := client.OpenOrdersSnapshot(ctx, ibkr.OpenOrdersScopeAll)
	if err != nil {
		panic(err)
	}

	for _, o := range orders {
		fmt.Println(o.Action, o.OrderType, o.Status, o.Filled, "/", o.Remaining)
	}
	// Output:
	// BUY LMT Submitted 2 / 8
}

func exampleClient(script string) (*ibkr.Client, *testhost.Host, context.CancelFunc) {
	host, err := testhost.New(script)
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
		panic(err)
	}
	return client, host, cancel
}
