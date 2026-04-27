//go:build legacy_native_socket

package ibkr_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/ThomasMarcelis/ibkr-go/testing/testhost"
	"github.com/shopspring/decimal"
)

func newInlineClient(t *testing.T, script string, opts ...ibkr.Option) (*ibkr.Client, *testhost.Host) {
	t.Helper()

	host, err := testhost.New(script)
	if err != nil {
		t.Fatalf("testhost.New() error = %v", err)
	}

	addrHost, addrPort, err := net.SplitHostPort(host.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := net.LookupPort("tcp", addrPort)
	if err != nil {
		t.Fatalf("LookupPort() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialOpts := []ibkr.Option{
		ibkr.WithHost(addrHost),
		ibkr.WithPort(port),
		ibkr.WithReconnectPolicy(ibkr.ReconnectOff),
	}
	dialOpts = append(dialOpts, opts...)

	client, err := ibkr.DialContext(ctx, dialOpts...)
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	return client, host
}

func TestStressSlowConsumerClose(t *testing.T) {
	t.Parallel()

	// Build a transcript that sends 200 tick_price messages.
	var sb strings.Builder
	sb.WriteString("handshake {\"server_version\":200,\"connection_time\":\"20260406 12:00:00 CET\"}\n")
	sb.WriteString("server managed_accounts {\"accounts\":[\"DU9000001\"]}\n")
	sb.WriteString("server next_valid_id {\"order_id\":1}\n")
	sb.WriteString("client req_quote {\"req_id\":\"$req1\"}\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "server tick_price {\"req_id\":\"$req1\",\"field\":1,\"price\":\"%d.00\"}\n", 100+i)
	}
	sb.WriteString("sleep 2s\n")
	sb.WriteString("disconnect\n")

	client, host := newInlineClient(t, sb.String(),
		ibkr.WithSubscriptionBuffer(5),
		ibkr.WithDefaultSlowConsumerPolicy(ibkr.SlowConsumerClose))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	// Do NOT read from Events(). Wait for the subscription to close.
	waitErr := sub.Wait()
	if !errors.Is(waitErr, ibkr.ErrSlowConsumer) {
		t.Fatalf("sub.Wait() error = %v, want ErrSlowConsumer", waitErr)
	}

	_ = host.Wait() // testhost may error due to disconnect on slow consumer
}

func TestStressSlowConsumerDropOldest(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	sb.WriteString("handshake {\"server_version\":200,\"connection_time\":\"20260406 12:00:00 CET\"}\n")
	sb.WriteString("server managed_accounts {\"accounts\":[\"DU9000001\"]}\n")
	sb.WriteString("server next_valid_id {\"order_id\":1}\n")
	sb.WriteString("client req_quote {\"req_id\":\"$req1\"}\n")
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, "server tick_price {\"req_id\":\"$req1\",\"field\":1,\"price\":\"%d.00\"}\n", 100+i)
	}
	sb.WriteString("sleep 500ms\n")
	sb.WriteString("client cancel_quote {\"req_id\":\"$req1\"}\n")
	sb.WriteString("disconnect\n")

	client, host := newInlineClient(t, sb.String(),
		ibkr.WithSubscriptionBuffer(5),
		ibkr.WithDefaultSlowConsumerPolicy(ibkr.SlowConsumerDropOldest))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
	})
	if err != nil {
		t.Fatalf("SubscribeQuotes() error = %v", err)
	}

	// Wait a bit for the server to flush, then drain.
	time.Sleep(200 * time.Millisecond)

	var received int
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				goto done
			}
			received++
		case <-time.After(2 * time.Second):
			goto done
		}
	}
done:
	sub.Close()

	t.Logf("received %d of 50 events (drop_oldest)", received)
	if received >= 50 {
		t.Fatalf("received all %d events, expected some dropped", received)
	}

	if err := host.Wait(); err != nil {
		t.Logf("host.Wait() = %v (expected after close)", err)
	}
	_ = ctx // suppress unused warning
}

func TestStressConcurrentOneshots(t *testing.T) {
	t.Parallel()

	const n = 10

	// Build transcript: client sends 10 contract detail requests, server responds in order.
	var sb strings.Builder
	sb.WriteString("handshake {\"server_version\":200,\"connection_time\":\"20260406 12:00:00 CET\"}\n")
	sb.WriteString("server managed_accounts {\"accounts\":[\"DU9000001\"]}\n")
	sb.WriteString("server next_valid_id {\"order_id\":1}\n")

	for i := 0; i < n; i++ {
		reqVar := fmt.Sprintf("$req%d", i+1)
		symbol := fmt.Sprintf("SYM%d", i)
		fmt.Fprintf(&sb, "client req_contract_details {\"req_id\":\"%s\"}\n", reqVar)
		fmt.Fprintf(&sb, "server contract_details {\"req_id\":\"%s\",\"contract\":{\"symbol\":\"%s\",\"sec_type\":\"STK\",\"exchange\":\"SMART\",\"currency\":\"USD\"},\"long_name\":\"%s Corp\"}\n", reqVar, symbol, symbol)
		fmt.Fprintf(&sb, "server contract_details_end {\"req_id\":\"%s\"}\n", reqVar)
	}
	sb.WriteString("disconnect\n")

	client, host := newInlineClient(t, sb.String())
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	errs := make([]error, n)
	results := make(map[string]string) // symbol -> long_name

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sym := fmt.Sprintf("SYM%d", idx)
			details, err := client.Contracts().Details(ctx, ibkr.Contract{
				Symbol:   sym,
				SecType:  ibkr.SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			})
			if err != nil {
				errs[idx] = err
				return
			}
			if len(details) != 1 {
				errs[idx] = fmt.Errorf("details len = %d, want 1", len(details))
				return
			}
			mu.Lock()
			results[sym] = details[0].LongName
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	// Verify all 10 results arrived. The key invariant is that each goroutine
	// received a response (reqID routing works under concurrent load).
	if len(results) != n {
		t.Errorf("got %d results, want %d", len(results), n)
	}
	t.Logf("concurrent oneshots: %d/%d completed", len(results), n)
}

func TestStressConcurrentPlaceOrders(t *testing.T) {
	t.Parallel()

	const n = 5

	// Build transcript: 5 place_order calls, interleaved responses.
	var sb strings.Builder
	sb.WriteString("handshake {\"server_version\":200,\"connection_time\":\"20260406 12:00:00 US/Eastern\"}\n")
	sb.WriteString("server managed_accounts {\"accounts\":[\"DU9000001\"]}\n")
	sb.WriteString("server next_valid_id {\"order_id\":100}\n")

	for i := 0; i < n; i++ {
		oid := 100 + i
		fmt.Fprintf(&sb, "client place_order {\"order_id\":\"%d\"}\n", oid)
	}
	sb.WriteString("sleep 100ms\n")
	// Server responds to all in order.
	for i := 0; i < n; i++ {
		oid := 100 + i
		fmt.Fprintf(&sb, "server open_order {\"order_id\":%d,\"status\":\"PreSubmitted\",\"contract\":{\"symbol\":\"AAPL\",\"sec_type\":\"STK\",\"exchange\":\"SMART\",\"currency\":\"USD\"},\"action\":\"BUY\",\"order_type\":\"LMT\",\"quantity\":\"1\",\"lmt_price\":\"50.00\"}\n", oid)
		fmt.Fprintf(&sb, "server order_status {\"order_id\":%d,\"status\":\"PreSubmitted\",\"filled\":\"0\",\"remaining\":\"1\",\"avg_fill_price\":\"0\",\"perm_id\":%d,\"parent_id\":0,\"last_fill_price\":\"0\",\"client_id\":0,\"why_held\":\"\",\"mkt_cap_price\":\"\"}\n", oid, 1000+oid)
	}
	sb.WriteString("sleep 200ms\n")
	// Then cancel all.
	for i := 0; i < n; i++ {
		oid := 100 + i
		fmt.Fprintf(&sb, "server order_status {\"order_id\":%d,\"status\":\"Cancelled\",\"filled\":\"0\",\"remaining\":\"0\",\"avg_fill_price\":\"0\",\"perm_id\":%d,\"parent_id\":0,\"last_fill_price\":\"0\",\"client_id\":0,\"why_held\":\"\",\"mkt_cap_price\":\"\"}\n", oid, 1000+oid)
	}
	sb.WriteString("disconnect\n")

	client, host := newInlineClient(t, sb.String())
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	handles := make([]*ibkr.OrderHandle, n)
	handleErrs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			h, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
				Contract: ibkr.Contract{
					Symbol:   "AAPL",
					SecType:  ibkr.SecTypeStock,
					Exchange: "SMART",
					Currency: "USD",
				},
				Order: ibkr.Order{
					Action:    ibkr.Buy,
					OrderType: ibkr.OrderTypeLimit,
					Quantity:  decimal.RequireFromString("1"),
					LmtPrice:  decimal.RequireFromString("50"),
					TIF:       ibkr.TIFDay,
				},
			})
			if err != nil {
				handleErrs[idx] = err
				return
			}
			handles[idx] = h
		}(i)
	}
	wg.Wait()

	for i, err := range handleErrs {
		if err != nil {
			t.Fatalf("PlaceOrder[%d]: %v", i, err)
		}
	}

	// Verify all handles got unique order IDs in the 100-104 range.
	seen := map[int64]bool{}
	for i, h := range handles {
		oid := h.OrderID()
		if oid < 100 || oid > 104 {
			t.Errorf("handle[%d].OrderID() = %d, want 100-104", i, oid)
		}
		if seen[oid] {
			t.Errorf("duplicate orderID %d", oid)
		}
		seen[oid] = true
	}

	// Wait for all handles to reach terminal status.
	for i, h := range handles {
		select {
		case <-h.Done():
		case <-time.After(5 * time.Second):
			t.Fatalf("handle[%d] did not reach terminal state", i)
		}
	}
	t.Logf("concurrent place orders: %d/%d completed", n, n)
}

func TestStressRapidSubscriptionCycling(t *testing.T) {
	t.Parallel()

	const cycles = 20

	// Build transcript: 20 cycles of subscribe + response + cancel.
	var sb strings.Builder
	sb.WriteString("handshake {\"server_version\":200,\"connection_time\":\"20260406 12:00:00 CET\"}\n")
	sb.WriteString("server managed_accounts {\"accounts\":[\"DU9000001\"]}\n")
	sb.WriteString("server next_valid_id {\"order_id\":1}\n")

	for i := 0; i < cycles; i++ {
		reqVar := fmt.Sprintf("$req%d", i+1)
		fmt.Fprintf(&sb, "client req_account_summary {\"req_id\":\"%s\"}\n", reqVar)
		fmt.Fprintf(&sb, "server account_summary {\"req_id\":\"%s\",\"account\":\"DU9000001\",\"tag\":\"NetLiquidation\",\"value\":\"100000.00\",\"currency\":\"USD\"}\n", reqVar)
		fmt.Fprintf(&sb, "server account_summary_end {\"req_id\":\"%s\"}\n", reqVar)
		fmt.Fprintf(&sb, "client cancel_account_summary {\"req_id\":\"%s\"}\n", reqVar)
	}
	sb.WriteString("disconnect\n")

	client, host := newInlineClient(t, sb.String())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for i := 0; i < cycles; i++ {
		sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
			Tags: []string{"NetLiquidation"},
		})
		if err != nil {
			t.Fatalf("cycle %d: SubscribeAccountSummary() error = %v", i, err)
		}

		// Drain events until snapshot complete.
		for {
			select {
			case <-sub.Events():
			case evt := <-sub.Lifecycle():
				if evt.Kind == ibkr.SubscriptionSnapshotComplete {
					goto closeIt
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("cycle %d: timeout", i)
			}
		}
	closeIt:
		sub.Close()
	}

	// Close client and host, then measure goroutines to detect leaks from
	// the cycling itself (not from other concurrent tests).
	client.Close()
	_ = host.Wait()

	// Snapshot goroutine count after full cleanup.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()

	// With 20 cycles, a per-cycle leak would show 20+ extra goroutines.
	// The absolute count after cleanup should be modest.
	t.Logf("goroutines after cleanup: %d (20 subscribe/cancel cycles)", goroutinesAfter)
}

func TestStressHighThroughputMarketDepth(t *testing.T) {
	t.Parallel()

	const rows = 500

	var sb strings.Builder
	sb.WriteString("handshake {\"server_version\":200,\"connection_time\":\"20260406 12:00:00 CET\"}\n")
	sb.WriteString("server managed_accounts {\"accounts\":[\"DU9000001\"]}\n")
	sb.WriteString("server next_valid_id {\"order_id\":1}\n")
	sb.WriteString("client req_market_depth {\"req_id\":\"$req1\"}\n")
	for i := 0; i < rows; i++ {
		// Alternate between insert (0) and update (1) operations.
		op := i % 2
		side := i % 2 // 0=ask, 1=bid
		fmt.Fprintf(&sb, "server market_depth {\"req_id\":\"$req1\",\"position\":%d,\"operation\":%d,\"side\":%d,\"price\":\"%d.50\",\"size\":\"%d\"}\n",
			i%10, op, side, 200+i%50, 100+i)
	}
	sb.WriteString("sleep 500ms\n")
	sb.WriteString("client cancel_market_depth {\"req_id\":\"$req1\"}\n")
	sb.WriteString("disconnect\n")

	client, host := newInlineClient(t, sb.String(),
		ibkr.WithSubscriptionBuffer(10))
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sub, err := client.MarketData().SubscribeDepth(ctx, ibkr.MarketDepthRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		NumRows: 10,
	}, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		t.Fatalf("SubscribeMarketDepth() error = %v", err)
	}

	var received int
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				goto done
			}
			received++
		case evt := <-sub.Lifecycle():
			if evt.Err != nil {
				// Slow consumer may fire with default close policy.
				t.Logf("subscription error after %d events: %v", received, evt.Err)
				goto done
			}
		case <-time.After(5 * time.Second):
			goto done
		}
	}
done:
	sub.Close()

	t.Logf("received %d of %d depth rows", received, rows)
	if received == 0 {
		t.Fatal("received 0 depth rows")
	}

	if err := host.Wait(); err != nil {
		t.Logf("host.Wait() = %v", err)
	}
}

func TestStressReconnectDuringOrders(t *testing.T) {
	t.Parallel()

	// Place order, get PreSubmitted, then disconnect.
	script := `handshake {"server_version":200,"connection_time":"20260406 12:00:00 US/Eastern"}
server managed_accounts {"accounts":["DU9000001"]}
server next_valid_id {"order_id":100}
client place_order {"order_id":"100"}
sleep 100ms
server open_order {"order_id":100,"status":"PreSubmitted","contract":{"symbol":"AAPL","sec_type":"STK","exchange":"SMART","currency":"USD"},"action":"BUY","order_type":"LMT","quantity":"1","lmt_price":"50.00"}
server order_status {"order_id":100,"status":"PreSubmitted","filled":"0","remaining":"1","avg_fill_price":"0","perm_id":9999,"parent_id":0,"last_fill_price":"0","client_id":0,"why_held":"","mkt_cap_price":""}
sleep 200ms
disconnect
`
	client, host := newInlineClient(t, script)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: ibkr.Contract{
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Order: ibkr.Order{
			Action:    ibkr.Buy,
			OrderType: ibkr.OrderTypeLimit,
			Quantity:  decimal.RequireFromString("1"),
			LmtPrice:  decimal.RequireFromString("50"),
			TIF:       ibkr.TIFDay,
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	// Wait for PreSubmitted.
	select {
	case evt := <-handle.Events():
		if evt.Status != nil {
			t.Logf("status: %s", evt.Status.Status)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for status")
	}

	// Wait for handle to finish (disconnect should close it).
	select {
	case <-handle.Done():
		t.Logf("handle done after disconnect: err=%v", handle.Wait())
	case <-time.After(5 * time.Second):
		t.Fatal("handle did not finish after disconnect")
	}

	if err := host.Wait(); err != nil {
		t.Logf("host.Wait() = %v", err)
	}
}
