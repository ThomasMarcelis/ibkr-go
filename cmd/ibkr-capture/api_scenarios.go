package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

var apiAAPL = ibkr.Contract{
	ConID:    265598,
	Symbol:   "AAPL",
	SecType:  ibkr.SecTypeStock,
	Exchange: "SMART",
	Currency: "USD",
}

func dialAPI(ctx context.Context, addr string, clientID int) (*ibkr.Client, error) {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("split addr %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, fmt.Errorf("parse port %q: %w", portText, err)
	}
	return ibkr.DialContext(ctx, ibkr.WithHost(host), ibkr.WithPort(port), ibkr.WithClientID(clientID))
}

func firstManagedAccount(client *ibkr.Client) (string, error) {
	snapshot := client.Session()
	if len(snapshot.ManagedAccounts) == 0 {
		return "", fmt.Errorf("session has no managed accounts")
	}
	return snapshot.ManagedAccounts[0], nil
}

func apiScenario(ctx context.Context, addr string, clientID int, timeout time.Duration, run func(context.Context, *ibkr.Client, string) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := dialAPI(ctx, addr, clientID)
	if err != nil {
		return err
	}
	defer client.Close()

	account, err := firstManagedAccount(client)
	if err != nil {
		return err
	}
	snapshot := client.Session()
	log.Printf("api session ready: server_version=%d account=%s next_valid_id=%d", snapshot.ServerVersion, account, snapshot.NextValidID)

	runErr := run(ctx, client, account)
	cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cleanCancel()
	if err := client.Orders().CancelAll(cleanCtx); err != nil {
		log.Printf("cleanup global cancel: %v", err)
	}
	time.Sleep(2 * time.Second)
	return runErr
}

func runAPIOrderTypeMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 6*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL anchor price: %s", anchor)

		cases := []struct {
			label         string
			order         ibkr.Order
			allowFill     bool
			cancelAfter   bool
			modifyToFill  bool
			roundTripSell bool
		}{
			{label: "mkt_buy_fill", order: baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarket), allowFill: true, roundTripSell: true},
			{label: "marketable_lmt_buy_fill", order: withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), marketableBuy(anchor)), allowFill: true, roundTripSell: true},
			{label: "far_lmt_buy_cancel", order: withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), cancelAfter: true},
			{label: "stp_buy_rest_cancel", order: withAux(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeStop), marketableBuy(anchor)), cancelAfter: true},
			{label: "stp_lmt_buy_rest_cancel", order: withLimit(withAux(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeStopLimit), marketableBuy(anchor)), marketableBuy(anchor).Add(decimal.NewFromInt(1))), cancelAfter: true},
			{label: "trail_sell_reject_or_rest", order: withTrailing(baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeTrailingStop), anchor), cancelAfter: true},
			{label: "trail_limit_sell_reject_or_rest", order: withTrailingLimit(baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeTrailingLimit), anchor), cancelAfter: true},
			{label: "mit_buy_reject_or_trigger", order: withAux(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarketIfTouched), marketableBuy(anchor)), allowFill: true, cancelAfter: true, roundTripSell: true},
			{label: "lit_buy_reject_or_trigger", order: withLimit(withAux(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimitIfTouched), marketableBuy(anchor)), marketableBuy(anchor).Add(decimal.NewFromInt(1))), allowFill: true, cancelAfter: true, roundTripSell: true},
			{label: "mtl_buy_fill_or_reprice", order: baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarketToLimit), allowFill: true, cancelAfter: true, roundTripSell: true},
			{label: "rel_buy_reject_or_rest", order: withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeRelative), farBuy(anchor)), cancelAfter: true},
			{label: "delayed_success_modify", order: withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), modifyToFill: true, allowFill: true, roundTripSell: true},
			{label: "invalid_order_type_reject", order: withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderType("FEELINGS")), farBuy(anchor))},
		}

		for _, tc := range cases {
			log.Printf("order matrix case start: %s", tc.label)
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: tc.order})
			if err != nil {
				log.Printf("%s place returned error: %v", tc.label, err)
				continue
			}
			filled, _ := observeOrder(ctx, handle, tc.label, 8*time.Second)
			if tc.modifyToFill && !filled {
				order := tc.order
				order.OrderType = ibkr.OrderTypeMarket
				order.LmtPrice = decimal.Zero
				if err := handle.Modify(ctx, order); err != nil {
					log.Printf("%s modify-to-fill error: %v", tc.label, err)
				} else {
					filled, _ = observeOrder(ctx, handle, tc.label+" modify", 20*time.Second)
				}
			}
			if tc.cancelAfter && !handleDone(handle) {
				cancelOrder(ctx, handle, tc.label)
				_, _ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
			}
			if tc.roundTripSell && filled {
				if err := flattenAAPL(ctx, client, account, tc.label, decimal.NewFromInt(1)); err != nil {
					log.Printf("%s flatten: %v", tc.label, err)
				}
			}
		}

		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPIOrderFillAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL fill anchor price: %s", anchor)

		if err := placeObserveFlatten(ctx, client, account, "fill mkt buy", baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarket), 30*time.Second); err != nil {
			log.Printf("fill mkt buy: %v", err)
		}

		mtl := baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarketToLimit)
		if err := placeObserveFlatten(ctx, client, account, "fill mtl buy", mtl, 30*time.Second); err != nil {
			log.Printf("fill mtl buy: %v", err)
		}

		resting := withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: resting})
		if err != nil {
			log.Printf("fill delayed place: %v", err)
			return nil
		}
		_, _ = observeOrder(ctx, handle, "fill delayed resting", 8*time.Second)
		resting.OrderType = ibkr.OrderTypeMarket
		resting.LmtPrice = decimal.Zero
		if err := handle.Modify(ctx, resting); err != nil {
			log.Printf("fill delayed modify-to-market: %v", err)
			cancelOrder(ctx, handle, "fill delayed")
			return nil
		}
		filled, _ := observeOrder(ctx, handle, "fill delayed modified", 30*time.Second)
		if filled {
			if err := flattenAAPL(ctx, client, account, "fill delayed", decimal.NewFromInt(1)); err != nil {
				log.Printf("fill delayed flatten: %v", err)
			}
		}
		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPIOrderRestCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL rest/cancel anchor price: %s", anchor)

		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "rest far lmt buy", order: withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))},
		}
		for _, tc := range cases {
			if !clientReady(client) {
				log.Printf("%s skipped: session state=%s", tc.label, client.Session().State)
				continue
			}
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: tc.order})
			if err != nil {
				log.Printf("%s place: %v", tc.label, err)
				continue
			}
			_, _ = observeOrder(ctx, handle, tc.label, 8*time.Second)
			cancelOrder(ctx, handle, tc.label)
			_, _ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPIOrderRelativeCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL relative/cancel anchor price: %s", anchor)

		order := withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeRelative), farBuy(anchor))
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: order})
		if err != nil {
			log.Printf("relative buy place: %v", err)
			return nil
		}
		_, _ = observeOrder(ctx, handle, "relative buy", 8*time.Second)
		cancelOrder(ctx, handle, "relative buy")
		_, _ = observeOrder(ctx, handle, "relative buy cancel", 8*time.Second)
		return nil
	})
}

func runAPIOrderTrailingCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL trailing/cancel anchor price: %s", anchor)

		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "trail sell", order: withTrailing(baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeTrailingStop), anchor)},
			{label: "trail limit sell", order: withTrailingLimit(baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeTrailingLimit), anchor)},
		}
		for _, tc := range cases {
			if !clientReady(client) {
				log.Printf("%s skipped: session state=%s", tc.label, client.Session().State)
				continue
			}
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: tc.order})
			if err != nil {
				log.Printf("%s place: %v", tc.label, err)
				continue
			}
			_, _ = observeOrder(ctx, handle, tc.label, 8*time.Second)
			cancelOrder(ctx, handle, tc.label)
			_, _ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPIOrderStopCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL stop/cancel anchor price: %s", anchor)

		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "stop buy", order: withAux(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeStop), farSell(anchor))},
			{label: "stop limit buy", order: withLimit(withAux(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeStopLimit), farSell(anchor)), farSell(anchor).Add(decimal.NewFromInt(1)))},
		}
		for _, tc := range cases {
			if !clientReady(client) {
				log.Printf("%s skipped: session state=%s", tc.label, client.Session().State)
				continue
			}
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: tc.order})
			if err != nil {
				log.Printf("%s place: %v", tc.label, err)
				continue
			}
			_, _ = observeOrder(ctx, handle, tc.label, 8*time.Second)
			cancelOrder(ctx, handle, tc.label)
			_, _ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPIOrderRejectsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL reject anchor price: %s", anchor)

		cases := []struct {
			label    string
			contract ibkr.Contract
			order    ibkr.Order
		}{
			{label: "reject invalid order type", contract: apiAAPL, order: withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderType("FEELINGS")), farBuy(anchor))},
			{label: "reject price band", contract: apiAAPL, order: withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), anchor.Mul(decimal.NewFromInt(10)).Round(2))},
			{label: "reject invalid contract", contract: ibkr.Contract{Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"}, order: baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarket)},
		}
		for _, tc := range cases {
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: tc.contract, Order: tc.order})
			if err != nil {
				log.Printf("%s place returned: %v", tc.label, err)
				continue
			}
			_, _ = observeOrder(ctx, handle, tc.label, 12*time.Second)
			if !handleDone(handle) {
				cancelOrder(ctx, handle, tc.label)
				_, _ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
			}
		}
		if err := client.Orders().Cancel(ctx, 999999999); err != nil {
			log.Printf("reject cancel unknown order returned: %v", err)
		}
		return nil
	})
}

func runAPIDelayedSuccessModifyAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		order := withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: order})
		if err != nil {
			return fmt.Errorf("place resting order: %w", err)
		}
		_, _ = observeOrder(ctx, handle, "delayed resting", 10*time.Second)

		order.OrderType = ibkr.OrderTypeMarket
		order.LmtPrice = decimal.Zero
		if err := handle.Modify(ctx, order); err != nil {
			return fmt.Errorf("modify resting order to market: %w", err)
		}
		filled, _ := observeOrder(ctx, handle, "delayed modified", 30*time.Second)
		if filled {
			return flattenAAPL(ctx, client, account, "delayed modified", decimal.NewFromInt(1))
		}
		return nil
	})
}

func runAPIBracketTriggerAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))

		parent, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: apiAAPL,
			Order:    withTransmit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarket), false),
		})
		if err != nil {
			return fmt.Errorf("place bracket parent: %w", err)
		}
		tp, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: apiAAPL,
			Order:    withTransmit(withParent(withLimit(baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeLimit), farSell(anchor)), parent.OrderID()), false),
		})
		if err != nil {
			return fmt.Errorf("place bracket take-profit: %w", err)
		}
		sl, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: apiAAPL,
			Order:    withParent(withAux(baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeStop), farBuy(anchor)), parent.OrderID()),
		})
		if err != nil {
			return fmt.Errorf("place bracket stop-loss: %w", err)
		}

		parentFilled, _ := observeOrder(ctx, parent, "bracket parent", 30*time.Second)
		_, _ = observeOrder(ctx, tp, "bracket take-profit initial", 5*time.Second)
		_, _ = observeOrder(ctx, sl, "bracket stop-loss initial", 5*time.Second)
		if parentFilled {
			tpOrder := withParent(withLimit(baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeLimit), marketableSell(anchor)), parent.OrderID())
			if err := tp.Modify(ctx, tpOrder); err != nil {
				log.Printf("bracket force take-profit modify: %v", err)
			}
			_, _ = observeOrder(ctx, tp, "bracket take-profit trigger", 30*time.Second)
			_, _ = observeOrder(ctx, sl, "bracket stop-loss sibling", 15*time.Second)
		}
		return nil
	})
}

func runAPIOCATriggerAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		oca := "ibkr-go-api-oca-" + strconv.FormatInt(time.Now().Unix(), 10)

		resting, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: apiAAPL,
			Order:    withOCA(withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), oca),
		})
		if err != nil {
			return fmt.Errorf("place OCA resting peer: %w", err)
		}
		marketable, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: apiAAPL,
			Order:    withOCA(withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), marketableBuy(anchor)), oca),
		})
		if err != nil {
			return fmt.Errorf("place OCA marketable peer: %w", err)
		}
		filled, _ := observeOrder(ctx, marketable, "oca marketable", 30*time.Second)
		_, _ = observeOrder(ctx, resting, "oca resting sibling", 20*time.Second)
		if filled {
			return flattenAAPL(ctx, client, account, "oca", decimal.NewFromInt(1))
		}
		return nil
	})
}

func runAPIConditionsMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		base := withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))
		conditions := []struct {
			label string
			cond  ibkr.OrderCondition
		}{
			{label: "price_condition", cond: ibkr.OrderCondition{Type: 1, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: farSell(anchor).String(), TriggerMethod: 4}},
			{label: "time_condition", cond: ibkr.OrderCondition{Type: 3, Conjunction: "a", Operator: 2, Value: time.Now().Add(2 * time.Minute).Format("20060102 15:04:05 MST")}},
			{label: "margin_condition", cond: ibkr.OrderCondition{Type: 4, Conjunction: "a", Operator: 2, Value: "10"}},
			{label: "execution_condition", cond: ibkr.OrderCondition{Type: 5, Conjunction: "a", SecType: ibkr.SecTypeStock, Exchange: "SMART", Symbol: "AAPL"}},
			{label: "volume_condition", cond: ibkr.OrderCondition{Type: 6, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "999999999"}},
			{label: "percent_change_condition", cond: ibkr.OrderCondition{Type: 7, Conjunction: "a", Operator: 2, ConID: 265598, Exchange: "SMART", Value: "50"}},
		}
		for _, tc := range conditions {
			order := base
			order.Conditions = []ibkr.OrderCondition{tc.cond}
			order.ConditionsIgnoreRTH = true
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: order})
			if err != nil {
				log.Printf("%s place error: %v", tc.label, err)
				continue
			}
			_, _ = observeOrder(ctx, handle, tc.label, 8*time.Second)
			cancelOrder(ctx, handle, tc.label)
			_, _ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPIOptionCampaignAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 5*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		opt, err := qualifyAAPLCall(ctx, client, anchor)
		if err != nil {
			log.Printf("qualify AAPL option: %v", err)
			return nil
		}
		log.Printf("qualified AAPL option: con_id=%d expiry=%s strike=%s right=%s trading_class=%s", opt.ConID, opt.Expiry, opt.Strike, opt.Right, opt.TradingClass)

		_, _ = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: opt})
		if _, err := client.Options().Price(ctx, ibkr.CalcOptionPriceRequest{Contract: opt, Volatility: decimal.RequireFromString("0.30"), UnderPrice: anchor}); err != nil {
			log.Printf("option price calculation: %v", err)
		}

		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: opt,
			Order:    baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarket),
		})
		if err != nil {
			log.Printf("option market buy place error: %v", err)
		} else {
			filled, _ := observeOrder(ctx, handle, "option buy", 40*time.Second)
			if filled {
				sell, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
					Contract: opt,
					Order:    baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeMarket),
				})
				if err != nil {
					log.Printf("option flatten place error: %v", err)
				} else {
					_, _ = observeOrder(ctx, sell, "option flatten", 40*time.Second)
				}
			}
		}
		if err := client.Options().Exercise(ctx, ibkr.ExerciseOptionsRequest{Contract: opt, ExerciseAction: ibkr.Lapse, ExerciseQuantity: 1, Account: account, Override: false}); err != nil {
			log.Printf("option lapse response: %v", err)
		}
		queryAAPLExecutions(client, account)
		queryCompleted(client, "option completed orders")
		return nil
	})
}

func runAPIFutureCampaignMES(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		fut, err := qualifyFrontFuture(ctx, client, "MES")
		if err != nil {
			log.Printf("qualify MES future: %v", err)
			return nil
		}
		log.Printf("qualified future: %+v", fut)
		_, _ = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: fut})

		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: fut,
			Order:    baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarket),
		})
		if err != nil {
			log.Printf("future market buy place error: %v", err)
			return nil
		}
		filled, _ := observeOrder(ctx, handle, "future buy", 40*time.Second)
		if filled {
			sell, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
				Contract: fut,
				Order:    baseAPIOrder(account, ibkr.Sell, ibkr.OrderTypeMarket),
			})
			if err != nil {
				log.Printf("future flatten place error: %v", err)
			} else {
				_, _ = observeOrder(ctx, sell, "future flatten", 40*time.Second)
			}
		}
		_, _ = client.Accounts().Positions(ctx)
		queryExecutions(client, ibkr.ExecutionsRequest{Account: account, Symbol: "MES"}, "future executions")
		return nil
	})
}

func runAPIComboOptionVerticalAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		lower, upper, err := qualifyAAPLCallVertical(ctx, client, anchor)
		if err != nil {
			log.Printf("qualify option vertical: %v", err)
			return nil
		}
		bag := ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeCombo, Exchange: "SMART", Currency: "USD"}
		order := withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), decimal.RequireFromString("0.05"))
		order.ComboLegs = []ibkr.ComboLeg{
			{ConID: lower.ConID, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ExemptCode: -1},
			{ConID: upper.ConID, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: "0", ExemptCode: -1},
		}
		order.SmartComboRoutingParams = []ibkr.TagValue{{Tag: "NonGuaranteed", Value: "1"}}
		handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: bag, Order: order})
		if err != nil {
			log.Printf("option vertical BAG place error: %v", err)
			return nil
		}
		_, _ = observeOrder(ctx, handle, "option vertical BAG", 15*time.Second)
		cancelOrder(ctx, handle, "option vertical BAG")
		_, _ = observeOrder(ctx, handle, "option vertical BAG cancel", 10*time.Second)
		_, _ = client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
		return nil
	})
}

func runAPIAlgorithmicCampaignAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 7*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("algorithmic campaign anchor=%s", anchor)

		quotes, _ := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiAAPL}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if quotes != nil {
			defer quotes.Close()
		}
		updates, _ := client.Accounts().SubscribeUpdates(ctx, account, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if updates != nil {
			defer updates.Close()
		}
		pnl, _ := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: account}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if pnl != nil {
			defer pnl.Close()
		}
		openOrders, _ := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if openOrders != nil {
			defer openOrders.Close()
		}
		drainObservers(quotes, updates, pnl, openOrders)

		if _, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{Account: account, Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"}}); err != nil {
			log.Printf("algorithmic summary: %v", err)
		}
		_, _ = client.Accounts().Positions(ctx)

		var filledBuys int
		for i := 0; i < 2; i++ {
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
				Contract: apiAAPL,
				Order:    baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeMarket),
			})
			if err != nil {
				log.Printf("algorithmic split buy[%d]: %v", i, err)
				continue
			}
			filled, _ := observeOrder(ctx, handle, fmt.Sprintf("algorithmic split buy[%d]", i), 30*time.Second)
			if filled {
				filledBuys++
			}
		}

		resting, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
			Contract: apiAAPL,
			Order:    withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)),
		})
		if err != nil {
			log.Printf("algorithmic resting buy: %v", err)
		} else {
			_, _ = observeOrder(ctx, resting, "algorithmic resting buy", 8*time.Second)
			modified := withLimit(baseAPIOrder(account, ibkr.Buy, ibkr.OrderTypeLimit), marketableBuy(anchor))
			if err := resting.Modify(ctx, modified); err != nil {
				log.Printf("algorithmic resting modify: %v", err)
			} else if filled, _ := observeOrder(ctx, resting, "algorithmic resting modified", 30*time.Second); filled {
				filledBuys++
			}
		}

		queryAAPLExecutions(client, account)
		for i := 0; i < filledBuys; i++ {
			if err := flattenAAPL(ctx, client, account, fmt.Sprintf("algorithmic flatten[%d]", i), decimal.NewFromInt(1)); err != nil {
				log.Printf("algorithmic flatten[%d]: %v", i, err)
			}
		}
		_, _ = client.Accounts().Positions(ctx)
		queryCompleted(client, "algorithmic completed orders")
		return nil
	})
}

func baseAPIOrder(account string, action ibkr.OrderAction, orderType ibkr.OrderType) ibkr.Order {
	return ibkr.Order{
		Action:    action,
		OrderType: orderType,
		Quantity:  decimal.NewFromInt(1),
		TIF:       ibkr.TIFDay,
		Account:   account,
		OrderRef:  "ibkr-go-api-capture",
	}
}

func placeObserveFlatten(ctx context.Context, client *ibkr.Client, account string, label string, order ibkr.Order, wait time.Duration) error {
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: order})
	if err != nil {
		return err
	}
	filled, _ := observeOrder(ctx, handle, label, wait)
	if filled && order.Action == ibkr.Buy {
		return flattenAAPL(ctx, client, account, label, order.Quantity)
	}
	return nil
}

func clientReady(client *ibkr.Client) bool {
	return client.Session().State == ibkr.StateReady
}

func withLimit(order ibkr.Order, price decimal.Decimal) ibkr.Order {
	order.LmtPrice = price.Round(2)
	return order
}

func withAux(order ibkr.Order, price decimal.Decimal) ibkr.Order {
	order.AuxPrice = price.Round(2)
	return order
}

func withParent(order ibkr.Order, parentID int64) ibkr.Order {
	order.ParentID = parentID
	return order
}

func withTransmit(order ibkr.Order, transmit bool) ibkr.Order {
	order.Transmit = new(transmit)
	return order
}

func withOCA(order ibkr.Order, group string) ibkr.Order {
	order.OcaGroup = group
	order.OcaType = 1
	return order
}

func withTrailing(order ibkr.Order, anchor decimal.Decimal) ibkr.Order {
	order.TrailStopPrice = farSell(anchor)
	order.AuxPrice = decimal.RequireFromString("1")
	return order
}

func withTrailingLimit(order ibkr.Order, anchor decimal.Decimal) ibkr.Order {
	order.TrailStopPrice = farSell(anchor)
	order.AuxPrice = decimal.RequireFromString("1")
	order.LmtPriceOffset = decimal.RequireFromString("0.05")
	return order
}

func quoteAnchor(ctx context.Context, client *ibkr.Client, contract ibkr.Contract, fallback decimal.Decimal) decimal.Decimal {
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		log.Printf("set live market data type: %v", err)
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
	if err != nil {
		log.Printf("live quote failed: %v", err)
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			log.Printf("set delayed market data type: %v", err)
		}
		quote, err = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
		if err != nil {
			log.Printf("delayed quote failed; fallback anchor %s: %v", fallback, err)
			return fallback
		}
	}
	for _, candidate := range []decimal.Decimal{quote.Last, quote.Ask, quote.Bid, quote.Close} {
		if candidate.IsPositive() {
			return candidate
		}
	}
	return fallback
}

func marketableBuy(anchor decimal.Decimal) decimal.Decimal {
	return anchor.Mul(decimal.RequireFromString("1.20")).Round(2)
}

func marketableSell(anchor decimal.Decimal) decimal.Decimal {
	return anchor.Mul(decimal.RequireFromString("0.80")).Round(2)
}

func farBuy(anchor decimal.Decimal) decimal.Decimal {
	return anchor.Mul(decimal.RequireFromString("0.05")).Round(2)
}

func farSell(anchor decimal.Decimal) decimal.Decimal {
	return anchor.Mul(decimal.RequireFromString("10")).Round(2)
}

func observeOrder(ctx context.Context, handle *ibkr.OrderHandle, label string, wait time.Duration) (bool, bool) {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	var filled bool
	var sawExecution bool
	record := func(evt ibkr.OrderEvent) {
		logOrderEvent(label, evt)
		if evt.Execution != nil {
			sawExecution = true
			filled = true
		}
		if evt.Status != nil {
			if evt.Status.Status == ibkr.OrderStatusFilled {
				filled = true
			}
		}
	}
	drain := func() {
		for {
			select {
			case evt, ok := <-handle.Events():
				if !ok {
					return
				}
				record(evt)
			default:
				return
			}
		}
	}
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				return filled, sawExecution
			}
			record(evt)
			if evt.Status != nil {
				if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
					drain()
					return filled, sawExecution
				}
			}
		case <-handle.Done():
			drain()
			if err := handle.Wait(); err != nil {
				log.Printf("%s handle done error: %v", label, err)
			}
			return filled, sawExecution
		case <-timer.C:
			drain()
			return filled, sawExecution
		case <-ctx.Done():
			drain()
			return filled, sawExecution
		}
	}
}

func logOrderEvent(label string, evt ibkr.OrderEvent) {
	if evt.OpenOrder != nil {
		log.Printf("%s open_order order_id=%d type=%s action=%s status=%s filled=%s remaining=%s lmt=%s aux=%s parent=%d oca=%s",
			label, evt.OpenOrder.OrderID, evt.OpenOrder.OrderType, evt.OpenOrder.Action, evt.OpenOrder.Status, evt.OpenOrder.Filled, evt.OpenOrder.Remaining, evt.OpenOrder.LmtPrice, evt.OpenOrder.AuxPrice, evt.OpenOrder.ParentID, evt.OpenOrder.OcaGroup)
	}
	if evt.Status != nil {
		log.Printf("%s status order_id=%d status=%s filled=%s remaining=%s avg=%s last=%s why_held=%s",
			label, evt.Status.OrderID, evt.Status.Status, evt.Status.Filled, evt.Status.Remaining, evt.Status.AvgFillPrice, evt.Status.LastFillPrice, evt.Status.WhyHeld)
	}
	if evt.Execution != nil {
		log.Printf("%s execution order_id=%d exec_id=%s side=%s shares=%s price=%s time=%s",
			label, evt.Execution.OrderID, evt.Execution.ExecID, evt.Execution.Side, evt.Execution.Shares, evt.Execution.Price, evt.Execution.Time.Format(time.RFC3339))
	}
	if evt.Commission != nil {
		log.Printf("%s commission exec_id=%s commission=%s currency=%s pnl=%s",
			label, evt.Commission.ExecID, evt.Commission.Commission, evt.Commission.Currency, evt.Commission.RealizedPNL)
	}
}

func handleDone(handle *ibkr.OrderHandle) bool {
	select {
	case <-handle.Done():
		return true
	default:
		return false
	}
}

func cancelOrder(ctx context.Context, handle *ibkr.OrderHandle, label string) {
	if err := handle.Cancel(ctx); err != nil {
		log.Printf("%s cancel error: %v", label, err)
	}
}

func flattenAAPL(ctx context.Context, client *ibkr.Client, account string, label string, qty decimal.Decimal) error {
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
		Contract: apiAAPL,
		Order: ibkr.Order{
			Action:    ibkr.Sell,
			OrderType: ibkr.OrderTypeMarket,
			Quantity:  qty,
			TIF:       ibkr.TIFDay,
			Account:   account,
			OrderRef:  "ibkr-go-api-flatten",
		},
	})
	if err != nil {
		return err
	}
	_, _ = observeOrder(ctx, handle, label+" flatten", 30*time.Second)
	return nil
}

func queryAAPLExecutions(client *ibkr.Client, account string) {
	queryExecutions(client, ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"}, "AAPL executions")
}

func queryExecutions(client *ibkr.Client, req ibkr.ExecutionsRequest, label string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	updates, err := client.Orders().Executions(ctx, req)
	if err != nil {
		log.Printf("%s query: %v", label, err)
		return
	}
	log.Printf("%s query updates=%d", label, len(updates))
}

func queryCompleted(client *ibkr.Client, label string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	orders, err := client.Orders().Completed(ctx, true)
	if err != nil {
		log.Printf("%s query: %v", label, err)
		return
	}
	log.Printf("%s query orders=%d", label, len(orders))
}

func qualifyAAPLCall(ctx context.Context, client *ibkr.Client, anchor decimal.Decimal) (ibkr.Contract, error) {
	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   265598,
	})
	if err != nil {
		return ibkr.Contract{}, err
	}
	param, ok := chooseOptionParams(params)
	if !ok {
		return ibkr.Contract{}, fmt.Errorf("no AAPL SMART option params")
	}
	expiry, ok := chooseFutureExpiry(param.Expirations)
	if !ok {
		return ibkr.Contract{}, fmt.Errorf("no future AAPL option expiration")
	}
	strike, ok := chooseNearestStrike(param.Strikes, anchor)
	if !ok {
		return ibkr.Contract{}, fmt.Errorf("no AAPL option strikes")
	}
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:       "AAPL",
		SecType:      ibkr.SecTypeOption,
		Expiry:       expiry,
		Strike:       strike.String(),
		Right:        ibkr.RightCall,
		Multiplier:   param.Multiplier,
		Exchange:     "SMART",
		Currency:     "USD",
		TradingClass: param.TradingClass,
	})
	if err != nil {
		return ibkr.Contract{}, err
	}
	if len(details) == 0 {
		return ibkr.Contract{}, fmt.Errorf("no qualified option details")
	}
	return details[0].Contract, nil
}

func qualifyAAPLCallVertical(ctx context.Context, client *ibkr.Client, anchor decimal.Decimal) (ibkr.Contract, ibkr.Contract, error) {
	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  "AAPL",
		UnderlyingSecType: ibkr.SecTypeStock,
		UnderlyingConID:   265598,
	})
	if err != nil {
		return ibkr.Contract{}, ibkr.Contract{}, err
	}
	param, ok := chooseOptionParams(params)
	if !ok {
		return ibkr.Contract{}, ibkr.Contract{}, fmt.Errorf("no AAPL SMART option params")
	}
	expiry, ok := chooseFutureExpiry(param.Expirations)
	if !ok {
		return ibkr.Contract{}, ibkr.Contract{}, fmt.Errorf("no future AAPL option expiration")
	}
	lower, upper, ok := chooseVerticalStrikes(param.Strikes, anchor)
	if !ok {
		return ibkr.Contract{}, ibkr.Contract{}, fmt.Errorf("not enough strikes for vertical")
	}
	qualify := func(strike decimal.Decimal) (ibkr.Contract, error) {
		details, err := client.Contracts().Details(ctx, ibkr.Contract{
			Symbol:       "AAPL",
			SecType:      ibkr.SecTypeOption,
			Expiry:       expiry,
			Strike:       strike.String(),
			Right:        ibkr.RightCall,
			Multiplier:   param.Multiplier,
			Exchange:     "SMART",
			Currency:     "USD",
			TradingClass: param.TradingClass,
		})
		if err != nil {
			return ibkr.Contract{}, err
		}
		if len(details) == 0 {
			return ibkr.Contract{}, fmt.Errorf("no contract details for strike %s", strike)
		}
		return details[0].Contract, nil
	}
	lowContract, err := qualify(lower)
	if err != nil {
		return ibkr.Contract{}, ibkr.Contract{}, err
	}
	highContract, err := qualify(upper)
	if err != nil {
		return ibkr.Contract{}, ibkr.Contract{}, err
	}
	return lowContract, highContract, nil
}

func chooseOptionParams(params []ibkr.SecDefOptParams) (ibkr.SecDefOptParams, bool) {
	for _, param := range params {
		if param.Exchange == "SMART" && param.Multiplier != "" && len(param.Expirations) > 0 && len(param.Strikes) > 0 {
			return param, true
		}
	}
	for _, param := range params {
		if param.Multiplier != "" && len(param.Expirations) > 0 && len(param.Strikes) > 0 {
			return param, true
		}
	}
	return ibkr.SecDefOptParams{}, false
}

func chooseFutureExpiry(expirations []string) (string, bool) {
	now := time.Now().Format("20060102")
	sorted := append([]string(nil), expirations...)
	sort.Strings(sorted)
	for _, expiry := range sorted {
		if expiry >= now {
			return expiry, true
		}
	}
	return "", false
}

func chooseNearestStrike(strikes []decimal.Decimal, anchor decimal.Decimal) (decimal.Decimal, bool) {
	if len(strikes) == 0 {
		return decimal.Zero, false
	}
	sorted := append([]decimal.Decimal(nil), strikes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LessThan(sorted[j]) })
	best := sorted[0]
	bestDistance := sorted[0].Sub(anchor).Abs()
	for _, strike := range sorted[1:] {
		distance := strike.Sub(anchor).Abs()
		if distance.LessThan(bestDistance) {
			best = strike
			bestDistance = distance
		}
	}
	return best, true
}

func chooseVerticalStrikes(strikes []decimal.Decimal, anchor decimal.Decimal) (decimal.Decimal, decimal.Decimal, bool) {
	sorted := append([]decimal.Decimal(nil), strikes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].LessThan(sorted[j]) })
	for i := 0; i+1 < len(sorted); i++ {
		if sorted[i].GreaterThanOrEqual(anchor) {
			return sorted[i], sorted[i+1], true
		}
	}
	if len(sorted) >= 2 {
		return sorted[len(sorted)-2], sorted[len(sorted)-1], true
	}
	return decimal.Zero, decimal.Zero, false
}

func qualifyFrontFuture(ctx context.Context, client *ibkr.Client, symbol string) (ibkr.Contract, error) {
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol:   symbol,
		SecType:  ibkr.SecTypeFuture,
		Exchange: "CME",
		Currency: "USD",
	})
	if err != nil {
		return ibkr.Contract{}, err
	}
	sort.Slice(details, func(i, j int) bool { return details[i].Expiry < details[j].Expiry })
	now := time.Now().Format("20060102")
	for _, detail := range details {
		if detail.Expiry >= now {
			return detail.Contract, nil
		}
	}
	if len(details) > 0 {
		return details[0].Contract, nil
	}
	return ibkr.Contract{}, fmt.Errorf("no %s future contract details", symbol)
}

func drainObservers(
	quotes *ibkr.Subscription[ibkr.QuoteUpdate],
	updates *ibkr.Subscription[ibkr.AccountUpdate],
	pnl *ibkr.Subscription[ibkr.PnLUpdate],
	openOrders *ibkr.Subscription[ibkr.OpenOrderUpdate],
) {
	go func() {
		deadline := time.After(45 * time.Second)
		for {
			select {
			case evt, ok := <-quoteEvents(quotes):
				if ok {
					log.Printf("observer quote changed=%d", evt.Changed)
				}
			case evt, ok := <-accountEvents(updates):
				if ok {
					if evt.AccountValue != nil {
						log.Printf("observer account key=%s account=%s", evt.AccountValue.Key, evt.AccountValue.Account)
					}
					if evt.Portfolio != nil {
						log.Printf("observer portfolio symbol=%s position=%s market_price=%s", evt.Portfolio.Contract.Symbol, evt.Portfolio.Position, evt.Portfolio.MarketPrice)
					}
				}
			case evt, ok := <-pnlEvents(pnl):
				if ok {
					log.Printf("observer pnl daily=%s unrealized=%s realized=%s", evt.DailyPnL, evt.UnrealizedPnL, evt.RealizedPnL)
				}
			case evt, ok := <-openOrderEvents(openOrders):
				if ok {
					log.Printf("observer open order_id=%d status=%s", evt.Order.OrderID, evt.Order.Status)
				}
			case <-deadline:
				return
			}
		}
	}()
}

func quoteEvents(sub *ibkr.Subscription[ibkr.QuoteUpdate]) <-chan ibkr.QuoteUpdate {
	if sub == nil {
		return nil
	}
	return sub.Events()
}

func accountEvents(sub *ibkr.Subscription[ibkr.AccountUpdate]) <-chan ibkr.AccountUpdate {
	if sub == nil {
		return nil
	}
	return sub.Events()
}

func pnlEvents(sub *ibkr.Subscription[ibkr.PnLUpdate]) <-chan ibkr.PnLUpdate {
	if sub == nil {
		return nil
	}
	return sub.Events()
}

func openOrderEvents(sub *ibkr.Subscription[ibkr.OpenOrderUpdate]) <-chan ibkr.OpenOrderUpdate {
	if sub == nil {
		return nil
	}
	return sub.Events()
}
