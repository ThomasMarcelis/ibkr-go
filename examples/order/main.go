// Place a far-from-market limit order on AAPL, observe status updates, then
// cancel it. Requires a paper trading account.
//
// Usage:
//
//	IBKR_ADDR=127.0.0.1:4002 IBKR_TRADING=1 go run ./examples/order
//
// The IBKR_TRADING=1 environment variable is a safety gate — the example
// refuses to run without it to prevent accidental order placement.
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
	"github.com/shopspring/decimal"
)

func main() {
	if os.Getenv("IBKR_TRADING") != "1" {
		log.Fatal("set IBKR_TRADING=1 to confirm you want to place a paper order")
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := ibkr.DialContext(ctx,
		ibkr.WithHost(host),
		ibkr.WithPort(port),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	account := client.Session().ManagedAccounts[0]

	// Place a far-from-market limit buy so it won't fill.
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
			LmtPrice:  decimal.RequireFromString("1.00"), // far from market
			TIF:       ibkr.TIFDay,
			Account:   account,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("placed order", handle.OrderID())

	// Read events until we see the order acknowledged, then cancel.
	cancelled := false
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				fmt.Println("events channel closed")
				goto done
			}
			switch {
			case evt.Status != nil:
				fmt.Printf("status: %s  filled=%s remaining=%s\n",
					evt.Status.Status, evt.Status.Filled, evt.Status.Remaining)

				// Cancel once the order is live on the server.
				if !cancelled && !ibkr.IsTerminalOrderStatus(evt.Status.Status) {
					fmt.Println("cancelling order...")
					if err := handle.Cancel(ctx); err != nil {
						log.Fatal(err)
					}
					cancelled = true
				}
			case evt.OpenOrder != nil:
				fmt.Printf("open order: %s %s %s @ %s\n",
					evt.OpenOrder.Action, evt.OpenOrder.Quantity,
					evt.OpenOrder.OrderType, evt.OpenOrder.LmtPrice)
			case evt.Execution != nil:
				fmt.Printf("execution: %s shares @ %s\n",
					evt.Execution.Shares, evt.Execution.Price)
			case evt.Commission != nil:
				fmt.Printf("commission: %s %s\n",
					evt.Commission.Commission, evt.Commission.Currency)
			}
		case <-handle.Done():
			goto done
		}
	}
done:
	err = handle.Wait()
	if err != nil {
		fmt.Println("order error:", err)
	} else {
		fmt.Println("order done")
	}
}
