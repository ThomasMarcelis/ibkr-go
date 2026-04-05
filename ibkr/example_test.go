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

	details, err := client.ContractDetails(ctx, ibkr.ContractDetailsRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(details[0].Contract.Symbol, details[0].MinTick)
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
			SecType:  "STK",
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
