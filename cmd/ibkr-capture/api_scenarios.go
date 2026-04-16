package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strconv"
	"sync"
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

type apiDriverRecorder struct {
	mu          sync.Mutex
	file        *os.File
	enc         *json.Encoder
	scenario    string
	runID       string
	scenarioTag string
	orderSeq    int
	events      []apiDriverEvent
}

type apiDriverEvent struct {
	At          time.Time         `json:"at"`
	Scenario    string            `json:"scenario"`
	RunID       string            `json:"run_id"`
	Kind        string            `json:"kind"`
	Label       string            `json:"label,omitempty"`
	Account     string            `json:"account,omitempty"`
	ClientID    int               `json:"client_id,omitempty"`
	Server      string            `json:"server,omitempty"`
	ServerVer   int               `json:"server_version,omitempty"`
	NextOrderID int64             `json:"next_order_id,omitempty"`
	OrderID     int64             `json:"order_id,omitempty"`
	OrderRef    string            `json:"order_ref,omitempty"`
	PermID      int64             `json:"perm_id,omitempty"`
	ParentID    int64             `json:"parent_id,omitempty"`
	OCAGroup    string            `json:"oca_group,omitempty"`
	Symbol      string            `json:"symbol,omitempty"`
	SecType     string            `json:"sec_type,omitempty"`
	Action      string            `json:"action,omitempty"`
	OrderType   string            `json:"order_type,omitempty"`
	TIF         string            `json:"tif,omitempty"`
	Quantity    string            `json:"quantity,omitempty"`
	Filled      string            `json:"filled,omitempty"`
	Remaining   string            `json:"remaining,omitempty"`
	LmtPrice    string            `json:"lmt_price,omitempty"`
	AuxPrice    string            `json:"aux_price,omitempty"`
	AvgPrice    string            `json:"avg_price,omitempty"`
	LastPrice   string            `json:"last_price,omitempty"`
	Status      string            `json:"status,omitempty"`
	WhyHeld     string            `json:"why_held,omitempty"`
	ExecID      string            `json:"exec_id,omitempty"`
	Side        string            `json:"side,omitempty"`
	Price       string            `json:"price,omitempty"`
	EventTime   string            `json:"event_time,omitempty"`
	Commission  string            `json:"commission,omitempty"`
	Currency    string            `json:"currency,omitempty"`
	RealizedPNL string            `json:"realized_pnl,omitempty"`
	Count       int               `json:"count,omitempty"`
	Error       string            `json:"error,omitempty"`
	Values      map[string]string `json:"values,omitempty"`
}

var apiDriver *apiDriverRecorder

var (
	apiStockOrderQuantity         = decimal.NewFromInt(100)
	apiStockCampaignOrderQuantity = decimal.NewFromInt(500)
	apiSingleContractQuantity     = decimal.NewFromInt(1)
	apiOptionContractQuantity     = decimal.NewFromInt(5)
)

func newAPIDriverRecorder(path string, scenario string) (*apiDriverRecorder, error) {
	now := time.Now().UTC()
	rec := &apiDriverRecorder{
		scenario:    scenario,
		runID:       now.Format("20060102T150405Z"),
		scenarioTag: scenarioHash(scenario),
	}
	if path == "" {
		return rec, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create driver events: %w", err)
	}
	rec.file = file
	rec.enc = json.NewEncoder(file)
	return rec, nil
}

func scenarioHash(scenario string) string {
	sum := sha1.Sum([]byte(scenario))
	return hex.EncodeToString(sum[:4])
}

func (r *apiDriverRecorder) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	if err := r.file.Sync(); err != nil {
		_ = r.file.Close()
		return err
	}
	return r.file.Close()
}

func (r *apiDriverRecorder) record(kind string, label string, fill func(*apiDriverEvent)) {
	if r == nil {
		return
	}
	event := apiDriverEvent{
		At:       time.Now().UTC(),
		Scenario: r.scenario,
		RunID:    r.runID,
		Kind:     kind,
		Label:    label,
	}
	if fill != nil {
		fill(&event)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	if r.enc == nil {
		return
	}
	if err := r.enc.Encode(event); err != nil {
		log.Printf("driver event encode: %v", err)
	}
}

func (r *apiDriverRecorder) Events() []apiDriverEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]apiDriverEvent(nil), r.events...)
}

func recordAPIEvent(kind string, label string, fill func(*apiDriverEvent)) {
	if apiDriver != nil {
		apiDriver.record(kind, label, fill)
	}
}

func apiOrderRef(label string) string {
	if apiDriver == nil {
		return "ibkr-go-api-capture"
	}
	apiDriver.mu.Lock()
	defer apiDriver.mu.Unlock()
	apiDriver.orderSeq++
	return fmt.Sprintf("ibkrgo-%s-%s-%03d", apiDriver.scenarioTag, apiDriver.runID, apiDriver.orderSeq)
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

	recordAPIEvent("scenario_start", "", func(event *apiDriverEvent) {
		event.Server = addr
		event.ClientID = clientID
	})

	client, err := dialAPI(ctx, addr, clientID)
	if err != nil {
		recordAPIEvent("dial_error", "", func(event *apiDriverEvent) {
			event.Server = addr
			event.ClientID = clientID
			event.Error = err.Error()
		})
		return err
	}
	defer client.Close()

	account, err := firstManagedAccount(client)
	if err != nil {
		recordAPIEvent("session_error", "", func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
		return err
	}
	snapshot := client.Session()
	log.Printf("api session ready: server_version=%d account=%s next_valid_id=%d", snapshot.ServerVersion, account, snapshot.NextValidID)
	recordAPIEvent("session_ready", "", func(event *apiDriverEvent) {
		event.Account = account
		event.ServerVer = snapshot.ServerVersion
		event.NextOrderID = snapshot.NextValidID
	})
	preCleanCtx, preCleanCancel := context.WithTimeout(context.Background(), 15*time.Second)
	recordAPIEvent("pre_cleanup_global_cancel_start", "", nil)
	if err := client.Orders().CancelAll(preCleanCtx); err != nil {
		log.Printf("pre-scenario global cancel: %v", err)
		recordAPIEvent("pre_cleanup_global_cancel_error", "", func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
	} else {
		recordAPIEvent("pre_cleanup_global_cancel_sent", "", nil)
	}
	preCleanCancel()
	time.Sleep(1 * time.Second)

	runErr := run(ctx, client, account)
	cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cleanCancel()
	recordAPIEvent("cleanup_global_cancel_start", "", nil)
	if err := client.Orders().CancelAll(cleanCtx); err != nil {
		log.Printf("cleanup global cancel: %v", err)
		recordAPIEvent("cleanup_global_cancel_error", "", func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
	} else {
		recordAPIEvent("cleanup_global_cancel_sent", "", nil)
	}
	time.Sleep(2 * time.Second)
	recordAPIEvent("scenario_end", "", func(event *apiDriverEvent) {
		if runErr != nil {
			event.Error = runErr.Error()
		}
	})
	return runErr
}

func runAPIOrderTypeMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 6*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL anchor price: %s", anchor)

		cases := []struct {
			label        string
			order        ibkr.Order
			allowFill    bool
			cancelAfter  bool
			modifyToFill bool
		}{
			{label: "mkt_buy_fill", order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket), allowFill: true},
			{label: "marketable_lmt_buy_fill", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), marketableBuy(anchor)), allowFill: true},
			{label: "far_lmt_buy_cancel", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), cancelAfter: true},
			{label: "stp_buy_rest_cancel", order: withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeStop), marketableBuy(anchor)), cancelAfter: true},
			{label: "stp_lmt_buy_rest_cancel", order: withLimit(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeStopLimit), marketableBuy(anchor)), marketableBuy(anchor).Add(decimal.NewFromInt(1))), cancelAfter: true},
			{label: "trail_sell_reject_or_rest", order: withTrailing(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeTrailingStop), anchor), cancelAfter: true},
			{label: "trail_limit_sell_reject_or_rest", order: withTrailingLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeTrailingLimit), anchor), cancelAfter: true},
			{label: "mit_buy_reject_or_trigger", order: withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarketIfTouched), marketableBuy(anchor)), allowFill: true, cancelAfter: true},
			{label: "lit_buy_reject_or_trigger", order: withLimit(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimitIfTouched), marketableBuy(anchor)), marketableBuy(anchor).Add(decimal.NewFromInt(1))), allowFill: true, cancelAfter: true},
			{label: "mtl_buy_fill_or_reprice", order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarketToLimit), allowFill: true, cancelAfter: true},
			{label: "rel_buy_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeRelative), farBuy(anchor)), cancelAfter: true},
			{label: "delayed_success_modify", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), modifyToFill: true, allowFill: true},
			{label: "invalid_order_type_reject", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderType("FEELINGS")), farBuy(anchor))},
			{label: "moc_buy_fill_or_reject", order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarketOnClose), allowFill: true, cancelAfter: true},
			{label: "loc_buy_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimitOnClose), marketableBuy(anchor)), allowFill: true, cancelAfter: true},
			{label: "moo_buy_reject_or_queued", order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarketOnOpen), cancelAfter: true},
			{label: "loo_buy_reject_or_queued", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimitOnOpen), marketableBuy(anchor)), cancelAfter: true},
			{label: "peg_mkt_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypePeggedToMarket), farBuy(anchor)), cancelAfter: true},
			{label: "peg_pri_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypePeggedToPrimary), farBuy(anchor)), cancelAfter: true},
			{label: "peg_mid_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypePeggedToMid), farBuy(anchor)), cancelAfter: true},
			{label: "peg_best_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypePeggedToBest), farBuy(anchor)), cancelAfter: true},
			{label: "peg_bench_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypePeggedBenchmark), farBuy(anchor)), cancelAfter: true},
		}

		for _, tc := range cases {
			log.Printf("order matrix case start: %s", tc.label)
			if !clientReady(client) {
				recordAPIEvent("order_matrix_stop_session_not_ready", tc.label, nil)
				log.Printf("%s skipped: session state=%s", tc.label, client.Session().State)
				break
			}
			caseCtx, caseCancel := context.WithTimeout(ctx, 45*time.Second)
			handle, err := placeAPIOrder(caseCtx, client, tc.label, apiAAPL, tc.order)
			if err != nil {
				log.Printf("%s place returned error: %v", tc.label, err)
				caseCancel()
				continue
			}
			obs := observeOrder(caseCtx, handle, tc.label, 8*time.Second)
			if tc.modifyToFill && !obs.FullFill() {
				order := tc.order
				order.OrderType = ibkr.OrderTypeMarket
				order.LmtPrice = decimal.Zero
				if err := modifyAPIOrder(caseCtx, handle, tc.label+" modify", order); err != nil {
					log.Printf("%s modify-to-fill error: %v", tc.label, err)
				} else {
					obs.Merge(observeOrder(caseCtx, handle, tc.label+" modify", 20*time.Second))
				}
			}
			if tc.cancelAfter && !handleDone(handle) {
				cancelOrder(caseCtx, handle, tc.label)
				_ = observeOrder(caseCtx, handle, tc.label+" cancel", 8*time.Second)
			}
			if obs.AnyFill() {
				if err := flattenAAPLFill(caseCtx, client, account, tc.label, tc.order.Action, obs.filledQty); err != nil {
					log.Printf("%s flatten: %v", tc.label, err)
				}
			}
			caseCancel()
		}

		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPIOrderFillAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL fill anchor price: %s", anchor)

		if err := placeObserveFlatten(ctx, client, account, "fill mkt buy", baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket), 30*time.Second); err != nil {
			log.Printf("fill mkt buy: %v", err)
		}

		mtl := baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarketToLimit)
		if err := placeObserveFlatten(ctx, client, account, "fill mtl buy", mtl, 30*time.Second); err != nil {
			log.Printf("fill mtl buy: %v", err)
		}

		resting := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))
		handle, err := placeAPIOrder(ctx, client, "fill delayed resting", apiAAPL, resting)
		if err != nil {
			log.Printf("fill delayed place: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "fill delayed resting", 8*time.Second)
		resting.OrderType = ibkr.OrderTypeMarket
		resting.LmtPrice = decimal.Zero
		if err := modifyAPIOrder(ctx, handle, "fill delayed modify", resting); err != nil {
			log.Printf("fill delayed modify-to-market: %v", err)
			cancelOrder(ctx, handle, "fill delayed")
			return nil
		}
		obs := observeOrder(ctx, handle, "fill delayed modified", 30*time.Second)
		if obs.AnyFill() {
			if err := flattenAAPL(ctx, client, account, "fill delayed", obs.filledQty); err != nil {
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
			{label: "rest far lmt buy", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))},
		}
		for _, tc := range cases {
			if !clientReady(client) {
				log.Printf("%s skipped: session state=%s", tc.label, client.Session().State)
				continue
			}
			handle, err := placeAPIOrder(ctx, client, tc.label, apiAAPL, tc.order)
			if err != nil {
				log.Printf("%s place: %v", tc.label, err)
				continue
			}
			_ = observeOrder(ctx, handle, tc.label, 8*time.Second)
			cancelOrder(ctx, handle, tc.label)
			_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPIOrderRelativeCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL relative/cancel anchor price: %s", anchor)

		order := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeRelative), farBuy(anchor))
		handle, err := placeAPIOrder(ctx, client, "relative buy", apiAAPL, order)
		if err != nil {
			log.Printf("relative buy place: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "relative buy", 8*time.Second)
		cancelOrder(ctx, handle, "relative buy")
		_ = observeOrder(ctx, handle, "relative buy cancel", 8*time.Second)
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
			{label: "trail sell", order: withTrailing(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeTrailingStop), anchor)},
			{label: "trail limit sell", order: withTrailingLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeTrailingLimit), anchor)},
		}
		for _, tc := range cases {
			if !clientReady(client) {
				log.Printf("%s skipped: session state=%s", tc.label, client.Session().State)
				continue
			}
			handle, err := placeAPIOrder(ctx, client, tc.label, apiAAPL, tc.order)
			if err != nil {
				log.Printf("%s place: %v", tc.label, err)
				continue
			}
			_ = observeOrder(ctx, handle, tc.label, 8*time.Second)
			cancelOrder(ctx, handle, tc.label)
			_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
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
			{label: "stop buy", order: withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeStop), farSell(anchor))},
			{label: "stop limit buy", order: withLimit(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeStopLimit), farSell(anchor)), farSell(anchor).Add(decimal.NewFromInt(1)))},
		}
		for _, tc := range cases {
			if !clientReady(client) {
				log.Printf("%s skipped: session state=%s", tc.label, client.Session().State)
				continue
			}
			handle, err := placeAPIOrder(ctx, client, tc.label, apiAAPL, tc.order)
			if err != nil {
				log.Printf("%s place: %v", tc.label, err)
				continue
			}
			_ = observeOrder(ctx, handle, tc.label, 8*time.Second)
			cancelOrder(ctx, handle, tc.label)
			_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
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
			{label: "reject invalid order type", contract: apiAAPL, order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderType("FEELINGS")), farBuy(anchor))},
			{label: "reject price band", contract: apiAAPL, order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), anchor.Mul(decimal.NewFromInt(10)).Round(2))},
			{label: "reject invalid contract", contract: ibkr.Contract{Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"}, order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket)},
		}
		for _, tc := range cases {
			handle, err := placeAPIOrder(ctx, client, tc.label, tc.contract, tc.order)
			if err != nil {
				log.Printf("%s place returned: %v", tc.label, err)
				continue
			}
			_ = observeOrder(ctx, handle, tc.label, 12*time.Second)
			if !handleDone(handle) {
				cancelOrder(ctx, handle, tc.label)
				_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
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
		order := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))
		handle, err := placeAPIOrder(ctx, client, "delayed resting", apiAAPL, order)
		if err != nil {
			return fmt.Errorf("place resting order: %w", err)
		}
		_ = observeOrder(ctx, handle, "delayed resting", 10*time.Second)

		order.OrderType = ibkr.OrderTypeMarket
		order.LmtPrice = decimal.Zero
		if err := modifyAPIOrder(ctx, handle, "delayed modified", order); err != nil {
			return fmt.Errorf("modify resting order to market: %w", err)
		}
		obs := observeOrder(ctx, handle, "delayed modified", 30*time.Second)
		if obs.AnyFill() {
			return flattenAAPL(ctx, client, account, "delayed modified", obs.filledQty)
		}
		return nil
	})
}

func runAPIBracketTriggerAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))

		parent, err := placeAPIOrder(ctx, client, "bracket parent", apiAAPL,
			withTransmit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket), false))
		if err != nil {
			return fmt.Errorf("place bracket parent: %w", err)
		}
		tp, err := placeAPIOrder(ctx, client, "bracket take-profit", apiAAPL,
			withTransmit(withParent(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeLimit), farSell(anchor)), parent.OrderID()), false))
		if err != nil {
			return fmt.Errorf("place bracket take-profit: %w", err)
		}
		sl, err := placeAPIOrder(ctx, client, "bracket stop-loss", apiAAPL,
			withParent(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeStop), farBuy(anchor)), parent.OrderID()))
		if err != nil {
			return fmt.Errorf("place bracket stop-loss: %w", err)
		}

		parentObs := observeOrder(ctx, parent, "bracket parent", 30*time.Second)
		_ = observeOrder(ctx, tp, "bracket take-profit initial", 5*time.Second)
		_ = observeOrder(ctx, sl, "bracket stop-loss initial", 5*time.Second)
		if parentObs.FullFill() {
			tpOrder := withParent(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeLimit), marketableSell(anchor)), parent.OrderID())
			if err := modifyAPIOrder(ctx, tp, "bracket take-profit trigger", tpOrder); err != nil {
				log.Printf("bracket force take-profit modify: %v", err)
			}
			_ = observeOrder(ctx, tp, "bracket take-profit trigger", 30*time.Second)
			_ = observeOrder(ctx, sl, "bracket stop-loss sibling", 15*time.Second)
		}
		return nil
	})
}

func runAPIOCATriggerAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		oca := "ibkr-go-api-oca-" + strconv.FormatInt(time.Now().Unix(), 10)

		resting, err := placeAPIOrder(ctx, client, "oca resting", apiAAPL,
			withOCA(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), oca))
		if err != nil {
			return fmt.Errorf("place OCA resting peer: %w", err)
		}
		marketable, err := placeAPIOrder(ctx, client, "oca marketable", apiAAPL,
			withOCA(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), marketableBuy(anchor)), oca))
		if err != nil {
			return fmt.Errorf("place OCA marketable peer: %w", err)
		}
		obs := observeOrder(ctx, marketable, "oca marketable", 30*time.Second)
		_ = observeOrder(ctx, resting, "oca resting sibling", 20*time.Second)
		if obs.AnyFill() {
			return flattenAAPL(ctx, client, account, "oca", obs.filledQty)
		}
		return nil
	})
}

func runAPIConditionsMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		base := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))
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
			handle, err := placeAPIOrder(ctx, client, tc.label, apiAAPL, order)
			if err != nil {
				log.Printf("%s place error: %v", tc.label, err)
				continue
			}
			_ = observeOrder(ctx, handle, tc.label, 8*time.Second)
			cancelOrder(ctx, handle, tc.label)
			_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPITIFAttributeMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 6*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL TIF/attribute anchor: %s", anchor)

		now := time.Now().UTC()
		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "gtc_far_lmt", order: withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTC)},
			{label: "gtd_far_lmt", order: withGoodTillDate(withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTD), orderTimestamp(now.Add(2*time.Hour)))},
			{label: "good_after_far_lmt", order: withGoodAfterTime(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), orderTimestamp(now.Add(2*time.Minute)))},
			{label: "all_or_none_far_lmt", order: withAllOrNone(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)))},
			{label: "min_qty_far_lmt", order: withMinQty(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), decimal.NewFromInt(3), decimal.NewFromInt(2))},
			{label: "rel_percent_offset", order: withPercentOffset(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeRelative), farBuy(anchor)), decimal.RequireFromString("0.03"))},
			{label: "trailing_percent", order: withTrailingPercent(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeTrailingStop), anchor, decimal.RequireFromString("1.5"))},
			{label: "trigger_method_stop", order: withTriggerMethod(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeStop), farSell(anchor)), 4)},
			{label: "explicit_order_ref", order: withOrderRef(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "ibkrgo-explicit-ref-"+scenarioHash("api_tif_attribute_matrix_aapl"))},
			{label: "scale_far_lmt", order: withScale(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)))},
			{label: "active_window_far_lmt", order: withActiveWindow(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), orderTimestamp(now.Add(2*time.Minute)), orderTimestamp(now.Add(4*time.Minute)))},
			{label: "price_management_far_lmt", order: withPriceManagement(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)))},
			{label: "adjusted_stop_fields", order: withAdjustedStop(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Sell, ibkr.OrderTypeStop), farBuy(anchor)), anchor)},
			{label: "manual_order_time_far_lmt", order: withManualOrderTime(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), orderTimestamp(now))},
			{label: "advanced_error_override_far_lmt", order: withAdvancedErrorOverride(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "IBDBUYTX")},
		}

		for _, tc := range cases {
			if !clientReady(client) {
				log.Printf("%s skipped: session state=%s", tc.label, client.Session().State)
				break
			}
			caseCtx, caseCancel := context.WithTimeout(ctx, 45*time.Second)
			handle, err := placeAPIOrder(caseCtx, client, tc.label, apiAAPL, tc.order)
			if err != nil {
				log.Printf("%s place returned: %v", tc.label, err)
				caseCancel()
				continue
			}
			obs := observeOrder(caseCtx, handle, tc.label, 10*time.Second)
			if !handleDone(handle) {
				cancelOrder(caseCtx, handle, tc.label)
				_ = observeOrder(caseCtx, handle, tc.label+" cancel", 10*time.Second)
			}
			if obs.AnyFill() {
				if err := flattenAAPLFill(caseCtx, client, account, tc.label, tc.order.Action, obs.filledQty); err != nil {
					log.Printf("%s flatten: %v", tc.label, err)
				}
			}
			caseCancel()
		}

		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPISecurityTypeProbeMatrix(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 6*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		probes := []struct {
			label    string
			contract ibkr.Contract
			timeout  time.Duration
		}{
			{label: "stk_aapl", contract: apiAAPL, timeout: 15 * time.Second},
			{label: "opt_aapl_chain", contract: ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeOption, Exchange: "SMART", Currency: "USD"}, timeout: 30 * time.Second},
			{label: "fut_mes_front", contract: ibkr.Contract{Symbol: "MES", SecType: ibkr.SecTypeFuture, Exchange: "CME", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "fop_mes", contract: ibkr.Contract{Symbol: "MES", SecType: ibkr.SecTypeFutureOption, Exchange: "CME", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "cash_eurusd", contract: apiEURUSD, timeout: 15 * time.Second},
			{label: "bond_probe", contract: ibkr.Contract{Symbol: "912797", SecType: ibkr.SecTypeBond, Exchange: "SMART", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "cfd_aapl", contract: ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeCFD, Exchange: "SMART", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "war_aapl", contract: ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeWarrant, Exchange: "SMART", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "ind_spx", contract: ibkr.Contract{Symbol: "SPX", SecType: ibkr.SecTypeIndex, Exchange: "CBOE", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "crypto_btc", contract: ibkr.Contract{Symbol: "BTC", SecType: ibkr.SecTypeCrypto, Exchange: "PAXOS", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "fund_vtsax", contract: ibkr.Contract{Symbol: "VTSAX", SecType: ibkr.SecTypeFund, Exchange: "FUNDSERV", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "bill_probe", contract: ibkr.Contract{Symbol: "912797", SecType: ibkr.SecTypeBill, Exchange: "SMART", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "cmdty_xauusd", contract: ibkr.Contract{Symbol: "XAUUSD", SecType: ibkr.SecTypeCommodity, Exchange: "SMART", Currency: "USD"}, timeout: 20 * time.Second},
			{label: "contfut_es", contract: ibkr.Contract{Symbol: "ES", SecType: ibkr.SecTypeContFuture, Exchange: "CME", Currency: "USD"}, timeout: 20 * time.Second},
		}
		for _, probe := range probes {
			caseCtx, cancel := context.WithTimeout(ctx, probe.timeout)
			details, err := client.Contracts().Details(caseCtx, probe.contract)
			cancel()
			if err != nil {
				log.Printf("security probe %s error: %v", probe.label, err)
				recordAPIEvent("contract_probe_error", probe.label, func(event *apiDriverEvent) {
					event.Symbol = probe.contract.Symbol
					event.SecType = string(probe.contract.SecType)
					event.Error = err.Error()
				})
				continue
			}
			log.Printf("security probe %s details=%d", probe.label, len(details))
			recordAPIEvent("contract_probe", probe.label, func(event *apiDriverEvent) {
				event.Symbol = probe.contract.Symbol
				event.SecType = string(probe.contract.SecType)
				event.Count = len(details)
			})
		}
		return nil
	})
}

func runAPIMarketDataCompletenessAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 7*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		for _, dataType := range []ibkr.MarketDataType{
			ibkr.MarketDataLive,
			ibkr.MarketDataFrozen,
			ibkr.MarketDataDelayed,
			ibkr.MarketDataDelayedFrozen,
		} {
			caseCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := client.MarketData().SetType(caseCtx, dataType)
			cancel()
			recordProbeResult("market_data_type", dataType.String(), 0, err)
			if err != nil {
				log.Printf("set market data type %s: %v", dataType, err)
			}
		}

		quoteCtx, quoteCancel := context.WithTimeout(ctx, 20*time.Second)
		quote, err := client.MarketData().Quote(quoteCtx, ibkr.QuoteRequest{
			Contract: apiAAPL,
			GenericTicks: []ibkr.GenericTick{
				"100", "101", "104", "106", "165", "221", "225", "233",
				"236", "258", "293", "294", "295", "318", "411", "456", "588",
			},
		})
		quoteCancel()
		if err != nil {
			log.Printf("generic tick quote: %v", err)
			recordProbeResult("quote_generic_ticks", "aapl", 0, err)
		} else {
			recordAPIEvent("quote_generic_ticks", "aapl", func(event *apiDriverEvent) {
				event.Symbol = apiAAPL.Symbol
				event.SecType = string(apiAAPL.SecType)
				event.Values = map[string]string{"available": strconv.FormatUint(uint64(quote.Available), 10), "market_data_type": quote.MarketDataType.String()}
			})
		}

		for _, what := range []ibkr.WhatToShow{ibkr.ShowTrades, ibkr.ShowBidAsk, ibkr.ShowMidpoint} {
			caseCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			sub, err := client.MarketData().SubscribeRealTimeBars(caseCtx, ibkr.RealTimeBarsRequest{Contract: apiAAPL, WhatToShow: what, UseRTH: false})
			if err != nil {
				cancel()
				log.Printf("realtime bars %s: %v", what, err)
				recordProbeResult("realtime_bars", string(what), 0, err)
				continue
			}
			count := observeBars(caseCtx, sub, 12*time.Second)
			_ = sub.Close()
			cancel()
			recordProbeResult("realtime_bars", string(what), count, nil)
		}

		for _, tickCase := range []struct {
			label      string
			tickType   ibkr.TickByTickType
			ignoreSize bool
		}{
			{label: "last", tickType: ibkr.TickByTickLast},
			{label: "all_last", tickType: ibkr.TickByTickAllLast},
			{label: "all_last_ignore_size", tickType: ibkr.TickByTickAllLast, ignoreSize: true},
			{label: "bid_ask", tickType: ibkr.TickByTickBidAsk},
			{label: "midpoint", tickType: ibkr.TickByTickMidPoint},
		} {
			caseCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			sub, err := client.MarketData().SubscribeTickByTick(caseCtx, ibkr.TickByTickRequest{
				Contract: apiAAPL, TickType: tickCase.tickType, NumberOfTicks: 0, IgnoreSize: tickCase.ignoreSize,
			})
			if err != nil {
				cancel()
				log.Printf("tick-by-tick %s: %v", tickCase.label, err)
				recordProbeResult("tick_by_tick", tickCase.label, 0, err)
				continue
			}
			count := observeTicks(caseCtx, sub, 12*time.Second)
			_ = sub.Close()
			cancel()
			recordProbeResult("tick_by_tick", tickCase.label, count, nil)
		}

		return nil
	})
}

func runAPIHistoricalMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 9*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		barCases := []struct {
			label    string
			duration ibkr.HistoricalDuration
			size     ibkr.BarSize
		}{
			{label: "1_sec", duration: ibkr.Minutes(1), size: ibkr.Bar1Sec},
			{label: "5_secs", duration: ibkr.Minutes(5), size: ibkr.Bar5Secs},
			{label: "10_secs", duration: ibkr.Minutes(5), size: ibkr.Bar10Secs},
			{label: "15_secs", duration: ibkr.Minutes(10), size: ibkr.Bar15Secs},
			{label: "30_secs", duration: ibkr.Minutes(30), size: ibkr.Bar30Secs},
			{label: "1_min", duration: ibkr.Hours(1), size: ibkr.Bar1Min},
			{label: "5_mins", duration: ibkr.Days(1), size: ibkr.Bar5Mins},
			{label: "15_mins", duration: ibkr.Days(2), size: ibkr.Bar15Mins},
			{label: "30_mins", duration: ibkr.Days(5), size: ibkr.Bar30Mins},
			{label: "1_hour", duration: ibkr.Weeks(1), size: ibkr.Bar1Hour},
			{label: "1_day", duration: ibkr.Months(1), size: ibkr.Bar1Day},
			{label: "1_month", duration: ibkr.Years(2), size: ibkr.Bar1Month},
		}
		for _, barCase := range barCases {
			caseCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			bars, err := client.History().Bars(caseCtx, ibkr.HistoricalBarsRequest{
				Contract: apiAAPL, EndTime: time.Now(), Duration: barCase.duration,
				BarSize: barCase.size, WhatToShow: ibkr.ShowTrades, UseRTH: true,
			})
			cancel()
			recordProbeResult("historical_bar_size", barCase.label, len(bars), err)
			if err != nil {
				log.Printf("historical bar size %s: %v", barCase.label, err)
			}
		}

		for _, what := range []ibkr.WhatToShow{
			ibkr.ShowTrades, ibkr.ShowBidAsk, ibkr.ShowMidpoint, ibkr.ShowAdjustedLast,
			ibkr.ShowHistoricalVolatility, ibkr.ShowOptionImpliedVolatility,
		} {
			caseCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			bars, err := client.History().Bars(caseCtx, ibkr.HistoricalBarsRequest{
				Contract: apiAAPL, EndTime: time.Now(), Duration: ibkr.Weeks(1),
				BarSize: ibkr.Bar1Day, WhatToShow: what, UseRTH: true,
			})
			cancel()
			recordProbeResult("historical_what_to_show", string(what), len(bars), err)
			if err != nil {
				log.Printf("historical whatToShow %s: %v", what, err)
			}
		}
		return nil
	})
}

func runAPINewsArticleAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		caseCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		items, err := client.News().Historical(caseCtx, ibkr.HistoricalNewsRequest{
			ConID: 265598,
			ProviderCodes: []ibkr.NewsProviderCode{
				"BRFG", "BRFUPDN", "DJNL",
			},
			StartTime:    time.Now().Add(-14 * 24 * time.Hour),
			EndTime:      time.Now(),
			TotalResults: 5,
		})
		cancel()
		if err != nil {
			log.Printf("historical news for article: %v", err)
			recordProbeResult("historical_news_for_article", "aapl", 0, err)
			return nil
		}
		recordProbeResult("historical_news_for_article", "aapl", len(items), nil)
		if len(items) == 0 {
			return nil
		}

		articleReq := ibkr.NewsArticleRequest{ProviderCode: items[0].ProviderCode, ArticleID: items[0].ArticleID}
		articleCtx, articleCancel := context.WithTimeout(ctx, 30*time.Second)
		article, err := client.News().Article(articleCtx, articleReq)
		articleCancel()
		if err != nil {
			log.Printf("news article %s/%s: %v", articleReq.ProviderCode, articleReq.ArticleID, err)
			recordProbeResult("news_article", string(articleReq.ProviderCode), 0, err)
			return nil
		}
		recordAPIEvent("news_article", string(articleReq.ProviderCode), func(event *apiDriverEvent) {
			event.Values = map[string]string{
				"article_id":   articleReq.ArticleID,
				"article_type": strconv.Itoa(article.ArticleType),
				"text_bytes":   strconv.Itoa(len(article.ArticleText)),
			}
		})
		return nil
	})
}

func runAPIFundamentalReportsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		for _, reportType := range []ibkr.FundamentalReportType{
			ibkr.FundamentalReportSnapshot,
			ibkr.FundamentalReportsFinSummary,
			ibkr.FundamentalReportsOwnership,
			ibkr.FundamentalReportRatios,
			ibkr.FundamentalReportsFinStatements,
			ibkr.FundamentalRESC,
		} {
			caseCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
			data, err := client.Contracts().FundamentalData(caseCtx, ibkr.FundamentalDataRequest{Contract: apiAAPL, ReportType: reportType})
			cancel()
			recordProbeResult("fundamental_report", string(reportType), len(data), err)
			if err != nil {
				log.Printf("fundamental %s: %v", reportType, err)
			}
		}
		return nil
	})
}

func runAPIWSHVariantsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		metaCtx, metaCancel := context.WithTimeout(ctx, 20*time.Second)
		meta, err := client.WSH().MetaData(metaCtx)
		metaCancel()
		recordProbeResult("wsh_meta_data", "metadata", len(meta), err)
		if err != nil {
			log.Printf("wsh metadata: %v", err)
		}

		eventCases := []struct {
			label string
			req   ibkr.WSHEventDataRequest
		}{
			{label: "by_conid", req: ibkr.WSHEventDataRequest{ConID: 265598, StartDate: time.Now().AddDate(0, 0, -7), EndDate: time.Now().AddDate(0, 1, 0), TotalLimit: 10}},
			{label: "portfolio", req: ibkr.WSHEventDataRequest{FillPortfolio: true, StartDate: time.Now().AddDate(0, 0, -7), EndDate: time.Now().AddDate(0, 1, 0), TotalLimit: 10}},
			{label: "watchlist_competitors", req: ibkr.WSHEventDataRequest{ConID: 265598, FillWatchlist: true, FillCompetitors: true, TotalLimit: 10}},
		}
		for _, eventCase := range eventCases {
			caseCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			data, err := client.WSH().EventData(caseCtx, eventCase.req)
			cancel()
			recordProbeResult("wsh_event_data", eventCase.label, len(data), err)
			if err != nil {
				log.Printf("wsh event %s: %v", eventCase.label, err)
			}
		}
		return nil
	})
}

func runAPIAlgoVariantsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 7*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		start := orderTimestamp(time.Now().UTC().Add(3 * time.Minute))
		end := orderTimestamp(time.Now().UTC().Add(20 * time.Minute))
		log.Printf("AAPL algo variants anchor: %s", anchor)

		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "adaptive_urgent", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "Adaptive", []ibkr.TagValue{{Tag: "adaptivePriority", Value: "Urgent"}})},
			{label: "adaptive_patient", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "Adaptive", []ibkr.TagValue{{Tag: "adaptivePriority", Value: "Patient"}})},
			{label: "twap", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "Twap", []ibkr.TagValue{{Tag: "strategyType", Value: "Marketable"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "allowPastEndTime", Value: "1"}})},
			{label: "vwap", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "Vwap", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "allowPastEndTime", Value: "1"}, {Tag: "noTakeLiq", Value: "1"}})},
			{label: "arrival_px", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "ArrivalPx", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}, {Tag: "riskAversion", Value: "Neutral"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "forceCompletion", Value: "0"}, {Tag: "allowPastEndTime", Value: "1"}})},
			{label: "dark_ice", order: withAlgo(withDisplaySize(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), 1), "DarkIce", []ibkr.TagValue{{Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "allowPastEndTime", Value: "1"}})},
			{label: "accum_dist", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "AD", []ibkr.TagValue{{Tag: "componentSize", Value: "1"}, {Tag: "timeBetweenOrders", Value: "60"}, {Tag: "randomizeTime20", Value: "0"}, {Tag: "randomizeSize55", Value: "0"}, {Tag: "giveUp", Value: "0"}, {Tag: "catchUp", Value: "1"}, {Tag: "waitForFill", Value: "1"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}})},
			{label: "inline", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "Inline", []ibkr.TagValue{{Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}})},
			{label: "close", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "ClosePx", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}, {Tag: "riskAversion", Value: "Neutral"}, {Tag: "startTime", Value: start}, {Tag: "forceCompletion", Value: "0"}})},
			{label: "pct_vol", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "PctVol", []ibkr.TagValue{{Tag: "pctVol", Value: "0.1"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "noTakeLiq", Value: "1"}})},
			{label: "balance_impact_risk", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "BalanceImpactRisk", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}, {Tag: "riskAversion", Value: "Neutral"}})},
			{label: "min_impact", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "MinImpact", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}})},
			{label: "jefferies_ad", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), "JefAD", []ibkr.TagValue{{Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "componentSize", Value: "1"}, {Tag: "timeBetweenOrders", Value: "60"}})},
		}

		for _, tc := range cases {
			caseCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
			handle, err := placeAPIOrder(caseCtx, client, tc.label, apiAAPL, tc.order)
			if err != nil {
				log.Printf("%s algo place returned: %v", tc.label, err)
				cancel()
				continue
			}
			obs := observeOrder(caseCtx, handle, tc.label, 10*time.Second)
			if !handleDone(handle) {
				cancelOrder(caseCtx, handle, tc.label)
				_ = observeOrder(caseCtx, handle, tc.label+" cancel", 10*time.Second)
			}
			if obs.AnyFill() {
				if err := flattenAAPLFill(caseCtx, client, account, tc.label, tc.order.Action, obs.filledQty); err != nil {
					log.Printf("%s flatten: %v", tc.label, err)
				}
			}
			cancel()
		}
		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPIPairsTradingAAPLMSFT(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_, _ = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: apiAAPL})
		_, _ = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: apiMSFT})

		aaplOrder := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket)
		aapl, err := placeAPIOrder(ctx, client, "pairs aapl buy", apiAAPL, aaplOrder)
		if err != nil {
			log.Printf("pairs AAPL buy: %v", err)
		}
		msftOrder := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.Sell, ibkr.OrderTypeMarket)
		msft, err := placeAPIOrder(ctx, client, "pairs msft sell", apiMSFT, msftOrder)
		if err != nil {
			log.Printf("pairs MSFT sell: %v", err)
		}

		var aaplObs, msftObs orderObservation
		if aapl != nil {
			aaplObs = observeOrder(ctx, aapl, "pairs aapl buy", 30*time.Second)
		}
		if msft != nil {
			msftObs = observeOrder(ctx, msft, "pairs msft sell", 30*time.Second)
		}
		if aaplObs.AnyFill() {
			if err := flattenAAPLFill(ctx, client, account, "pairs aapl", aaplOrder.Action, aaplObs.filledQty); err != nil {
				log.Printf("pairs AAPL flatten: %v", err)
			}
		}
		if msftObs.AnyFill() {
			if err := flattenStockFill(ctx, client, account, "pairs msft", apiMSFT, msftOrder.Action, msftObs.filledQty); err != nil {
				log.Printf("pairs MSFT flatten: %v", err)
			}
		}
		queryAAPLExecutions(client, account)
		queryExecutions(client, ibkr.ExecutionsRequest{Account: account, Symbol: "MSFT"}, "MSFT executions")
		return nil
	})
}

func runAPIDollarCostAveragingAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		filledQty := decimal.Zero
		for i := 0; i < 3; i++ {
			order := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket)
			handle, err := placeAPIOrder(ctx, client, fmt.Sprintf("dca buy[%d]", i), apiAAPL, order)
			if err != nil {
				log.Printf("dca buy[%d]: %v", i, err)
				continue
			}
			obs := observeOrder(ctx, handle, fmt.Sprintf("dca buy[%d]", i), 30*time.Second)
			if obs.AnyFill() {
				filledQty = filledQty.Add(obs.filledQty)
			}
			time.Sleep(2 * time.Second)
		}
		if filledQty.IsPositive() {
			if err := flattenAAPL(ctx, client, account, "dca flatten", filledQty); err != nil {
				log.Printf("dca flatten: %v", err)
			}
		}
		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPIStopLossManagementAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		buyOrder := baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket)
		buy, err := placeAPIOrder(ctx, client, "stop-management buy", apiAAPL, buyOrder)
		if err != nil {
			log.Printf("stop-management buy: %v", err)
			return nil
		}
		buyObs := observeOrder(ctx, buy, "stop-management buy", 30*time.Second)
		if !buyObs.AnyFill() {
			return nil
		}

		stopOrder := withAux(baseAPIOrder(account, buyObs.filledQty, ibkr.Sell, ibkr.OrderTypeStop), farBuy(anchor))
		stop, err := placeAPIOrder(ctx, client, "stop-management stop", apiAAPL, stopOrder)
		if err != nil {
			log.Printf("stop-management stop: %v", err)
			_ = flattenAAPL(ctx, client, account, "stop-management emergency", buyObs.filledQty)
			return nil
		}
		_ = observeOrder(ctx, stop, "stop-management stop", 8*time.Second)

		stopOrder.AuxPrice = farBuy(anchor).Add(decimal.NewFromInt(1))
		if err := modifyAPIOrder(ctx, stop, "stop-management moved stop", stopOrder); err != nil {
			log.Printf("stop-management modify: %v", err)
		}
		_ = observeOrder(ctx, stop, "stop-management moved stop", 8*time.Second)
		cancelOrder(ctx, stop, "stop-management stop")
		_ = observeOrder(ctx, stop, "stop-management stop cancel", 8*time.Second)
		if err := flattenAAPL(ctx, client, account, "stop-management flatten", buyObs.filledQty); err != nil {
			log.Printf("stop-management flatten: %v", err)
		}
		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPIBracketTrailingStopAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		parentOrder := withTransmit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket), false)
		parent, err := placeAPIOrder(ctx, client, "trailing bracket parent", apiAAPL,
			parentOrder)
		if err != nil {
			log.Printf("trailing bracket parent: %v", err)
			return nil
		}
		takeProfitOrder := withTransmit(withParent(withLimit(baseAPIOrder(account, parentOrder.Quantity, ibkr.Sell, ibkr.OrderTypeLimit), farSell(anchor)), parent.OrderID()), false)
		takeProfit, err := placeAPIOrder(ctx, client, "trailing bracket take-profit", apiAAPL,
			takeProfitOrder)
		if err != nil {
			log.Printf("trailing bracket take-profit: %v", err)
			return nil
		}
		trailingStopOrder := withParent(withTrailing(baseAPIOrder(account, parentOrder.Quantity, ibkr.Sell, ibkr.OrderTypeTrailingStop), anchor), parent.OrderID())
		trailingStopOrder.TrailStopPrice = farBuy(anchor)
		trailingStop, err := placeAPIOrder(ctx, client, "trailing bracket stop", apiAAPL, trailingStopOrder)
		if err != nil {
			log.Printf("trailing bracket stop: %v", err)
			return nil
		}

		parentObs := observeOrder(ctx, parent, "trailing bracket parent", 30*time.Second)
		_ = observeOrder(ctx, takeProfit, "trailing bracket take-profit", 8*time.Second)
		_ = observeOrder(ctx, trailingStop, "trailing bracket stop", 8*time.Second)
		if err := client.Orders().CancelAll(ctx); err != nil {
			log.Printf("trailing bracket global cancel: %v", err)
		}
		_ = observeOrder(ctx, takeProfit, "trailing bracket take-profit cancel", 10*time.Second)
		_ = observeOrder(ctx, trailingStop, "trailing bracket stop cancel", 10*time.Second)
		if parentObs.AnyFill() {
			if err := flattenAAPL(ctx, client, account, "trailing bracket flatten", parentObs.filledQty); err != nil {
				log.Printf("trailing bracket flatten: %v", err)
			}
		}
		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPIOptionCampaignAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 5*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		opt, err := qualifyAAPLCall(ctx, client, anchor)
		if err != nil {
			log.Printf("qualify AAPL option: %v", err)
			recordAPIEvent("option_qualify_error", "aapl call", func(event *apiDriverEvent) {
				event.Symbol = "AAPL"
				event.SecType = string(ibkr.SecTypeOption)
				event.Error = err.Error()
			})
			return nil
		}
		log.Printf("qualified AAPL option: con_id=%d expiry=%s strike=%s right=%s trading_class=%s", opt.ConID, opt.Expiry, opt.Strike, opt.Right, opt.TradingClass)

		_, _ = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: opt})
		if _, err := client.Options().Price(ctx, ibkr.CalcOptionPriceRequest{Contract: opt, Volatility: decimal.RequireFromString("0.30"), UnderPrice: anchor}); err != nil {
			log.Printf("option price calculation: %v", err)
		}

		optionBuy := baseAPIOrder(account, apiOptionContractQuantity, ibkr.Buy, ibkr.OrderTypeMarket)
		handle, err := placeAPIOrder(ctx, client, "option buy", opt, optionBuy)
		if err != nil {
			log.Printf("option market buy place error: %v", err)
		} else {
			obs := observeOrder(ctx, handle, "option buy", 40*time.Second)
			if obs.AnyFill() {
				optionSell := baseAPIOrder(account, obs.filledQty, ibkr.Sell, ibkr.OrderTypeMarket)
				sell, err := placeAPIOrder(ctx, client, "option flatten", opt, optionSell)
				if err != nil {
					log.Printf("option flatten place error: %v", err)
				} else {
					_ = observeOrder(ctx, sell, "option flatten", 40*time.Second)
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

		futureBuy := baseAPIOrder(account, apiSingleContractQuantity, ibkr.Buy, ibkr.OrderTypeMarket)
		handle, err := placeAPIOrder(ctx, client, "future buy", fut, futureBuy)
		if err != nil {
			log.Printf("future market buy place error: %v", err)
			return nil
		}
		obs := observeOrder(ctx, handle, "future buy", 40*time.Second)
		if obs.AnyFill() {
			futureSell := baseAPIOrder(account, obs.filledQty, ibkr.Sell, ibkr.OrderTypeMarket)
			sell, err := placeAPIOrder(ctx, client, "future flatten", fut, futureSell)
			if err != nil {
				log.Printf("future flatten place error: %v", err)
			} else {
				_ = observeOrder(ctx, sell, "future flatten", 40*time.Second)
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
			recordAPIEvent("option_qualify_error", "aapl call vertical", func(event *apiDriverEvent) {
				event.Symbol = "AAPL"
				event.SecType = string(ibkr.SecTypeOption)
				event.Error = err.Error()
			})
			return nil
		}
		bag := ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeCombo, Exchange: "SMART", Currency: "USD"}
		order := withLimit(baseAPIOrder(account, apiOptionContractQuantity, ibkr.Buy, ibkr.OrderTypeLimit), decimal.RequireFromString("0.05"))
		order.ComboLegs = []ibkr.ComboLeg{
			{ConID: lower.ConID, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: "0", ExemptCode: -1},
			{ConID: upper.ConID, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: "0", ExemptCode: -1},
		}
		order.SmartComboRoutingParams = []ibkr.TagValue{{Tag: "NonGuaranteed", Value: "1"}}
		handle, err := placeAPIOrder(ctx, client, "option vertical BAG", bag, order)
		if err != nil {
			log.Printf("option vertical BAG place error: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "option vertical BAG", 15*time.Second)
		cancelOrder(ctx, handle, "option vertical BAG")
		_ = observeOrder(ctx, handle, "option vertical BAG cancel", 10*time.Second)
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

		filledQty := decimal.Zero
		for i := 0; i < 2; i++ {
			order := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket)
			handle, err := placeAPIOrder(ctx, client, fmt.Sprintf("algorithmic split buy[%d]", i), apiAAPL, order)
			if err != nil {
				log.Printf("algorithmic split buy[%d]: %v", i, err)
				continue
			}
			obs := observeOrder(ctx, handle, fmt.Sprintf("algorithmic split buy[%d]", i), 30*time.Second)
			if obs.AnyFill() {
				filledQty = filledQty.Add(obs.filledQty)
			}
		}

		restingOrder := withLimit(baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor))
		resting, err := placeAPIOrder(ctx, client, "algorithmic resting buy", apiAAPL, restingOrder)
		if err != nil {
			log.Printf("algorithmic resting buy: %v", err)
		} else {
			_ = observeOrder(ctx, resting, "algorithmic resting buy", 8*time.Second)
			modified := withLimit(baseAPIOrder(account, restingOrder.Quantity, ibkr.Buy, ibkr.OrderTypeLimit), marketableBuy(anchor))
			if err := modifyAPIOrder(ctx, resting, "algorithmic resting modified", modified); err != nil {
				log.Printf("algorithmic resting modify: %v", err)
			} else if obs := observeOrder(ctx, resting, "algorithmic resting modified", 30*time.Second); obs.AnyFill() {
				filledQty = filledQty.Add(obs.filledQty)
			}
		}

		queryAAPLExecutions(client, account)
		if filledQty.IsPositive() {
			if err := flattenAAPL(ctx, client, account, "algorithmic flatten", filledQty); err != nil {
				log.Printf("algorithmic flatten: %v", err)
			}
		}
		_, _ = client.Accounts().Positions(ctx)
		queryCompleted(client, "algorithmic completed orders")
		return nil
	})
}

func runAPICompletedOrdersVariantsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		order := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket)
		handle, err := placeAPIOrder(ctx, client, "completed seed buy", apiAAPL, order)
		if err != nil {
			log.Printf("completed seed buy: %v", err)
		} else if obs := observeOrder(ctx, handle, "completed seed buy", 30*time.Second); obs.AnyFill() {
			if err := flattenAAPL(ctx, client, account, "completed seed flatten", obs.filledQty); err != nil {
				log.Printf("completed seed flatten: %v", err)
			}
		}
		queryCompletedVariant(client, "completed api_only false", false)
		queryCompletedVariant(client, "completed api_only true", true)
		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPITransmitFalseThenTransmitAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		order := withTransmit(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), false)
		handle, err := placeAPIOrder(ctx, client, "transmit false resting", apiAAPL, order)
		if err != nil {
			log.Printf("transmit false place: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "transmit false resting", 10*time.Second)

		order.Transmit = new(true)
		if err := modifyAPIOrder(ctx, handle, "transmit true modify", order); err != nil {
			log.Printf("transmit true modify: %v", err)
		}
		_ = observeOrder(ctx, handle, "transmit true modify", 10*time.Second)
		if !handleDone(handle) {
			cancelOrder(ctx, handle, "transmit true")
			_ = observeOrder(ctx, handle, "transmit true cancel", 10*time.Second)
		}
		return nil
	})
}

func runAPIDuplicateQuoteSubscriptionsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 1*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			log.Printf("duplicate quote set delayed: %v", err)
		}
		first, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiAAPL}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			recordProbeResult("duplicate_quote_subscribe", "first", 0, err)
			return nil
		}
		defer first.Close()
		recordProbeResult("duplicate_quote_subscribe", "first", 1, nil)

		second, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiAAPL}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			recordProbeResult("duplicate_quote_subscribe", "second", 0, err)
		} else {
			defer second.Close()
			recordProbeResult("duplicate_quote_subscribe", "second", 1, nil)
		}

		observeQuotes(ctx, first, "duplicate quote first", 8*time.Second)
		if second != nil {
			observeQuotes(ctx, second, "duplicate quote second", 8*time.Second)
		}
		return nil
	})
}

func runAPIReconnectActiveOrderAAPL(ctx context.Context, addr string, clientID int) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	recordAPIEvent("scenario_start", "", func(event *apiDriverEvent) {
		event.Server = addr
		event.ClientID = clientID
	})
	defer recordAPIEvent("scenario_end", "", nil)
	cleanup := apiOrderCleanup{addr: addr, clientID: clientID, label: "reconnect resting"}
	defer cleanup.Run()

	first, err := dialAPI(ctx, addr, clientID)
	if err != nil {
		recordAPIEvent("dial_error", "first", func(event *apiDriverEvent) {
			event.Server = addr
			event.ClientID = clientID
			event.Error = err.Error()
		})
		return err
	}
	account, err := firstManagedAccount(first)
	if err != nil {
		_ = first.Close()
		return err
	}
	recordSessionReady(addr, clientID, account, first)
	if err := first.Orders().CancelAll(ctx); err != nil {
		log.Printf("reconnect pre-cleanup global cancel: %v", err)
	}

	anchor := quoteAnchor(ctx, first, apiAAPL, decimal.RequireFromString("200"))
	order := withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTC)
	handle, err := placeAPIOrder(ctx, first, "reconnect resting", apiAAPL, order)
	if err != nil {
		_ = first.Close()
		return fmt.Errorf("place reconnect resting order: %w", err)
	}
	_ = observeOrder(ctx, handle, "reconnect resting", 10*time.Second)
	orderID := handle.OrderID()
	cleanup.Track(orderID)
	recordAPIEvent("disconnect_client", "reconnect resting", func(event *apiDriverEvent) {
		event.OrderID = orderID
	})
	_ = first.Close()
	time.Sleep(500 * time.Millisecond)

	second, err := dialAPI(ctx, addr, clientID)
	if err != nil {
		recordAPIEvent("dial_error", "second", func(event *apiDriverEvent) {
			event.Server = addr
			event.ClientID = clientID
			event.Error = err.Error()
		})
		return err
	}
	defer second.Close()
	recordSessionReady(addr, clientID, account, second)

	orders, err := second.Orders().Open(ctx, ibkr.OpenOrdersScopeClient)
	recordOpenOrdersResult("reconnect open client", orders, err)
	if err := second.Orders().Cancel(ctx, orderID); err != nil {
		recordAPIEvent("direct_cancel_error", "reconnect resting", func(event *apiDriverEvent) {
			event.OrderID = orderID
			event.Error = err.Error()
		})
		log.Printf("reconnect direct cancel: %v", err)
	} else {
		recordAPIEvent("direct_cancel_sent", "reconnect resting", func(event *apiDriverEvent) {
			event.OrderID = orderID
		})
	}
	time.Sleep(2 * time.Second)
	if err := second.Orders().CancelAll(ctx); err != nil {
		log.Printf("reconnect cleanup global cancel: %v", err)
	} else {
		cleanup.MarkDone()
	}
	return nil
}

func runAPIClientID0OrderObservationAAPL(ctx context.Context, addr string, clientID int) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	recordAPIEvent("scenario_start", "", func(event *apiDriverEvent) {
		event.Server = addr
		event.ClientID = clientID
	})
	defer recordAPIEvent("scenario_end", "", nil)
	cleanup := apiOrderCleanup{addr: addr, clientID: clientID, label: "client0 observed resting"}
	defer cleanup.Run()

	placer, err := dialAPI(ctx, addr, clientID)
	if err != nil {
		return err
	}
	account, err := firstManagedAccount(placer)
	if err != nil {
		_ = placer.Close()
		return err
	}
	recordSessionReady(addr, clientID, account, placer)
	if err := placer.Orders().CancelAll(ctx); err != nil {
		log.Printf("client0 pre-cleanup global cancel: %v", err)
	}
	anchor := quoteAnchor(ctx, placer, apiAAPL, decimal.RequireFromString("200"))
	order := withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTC)
	handle, err := placeAPIOrder(ctx, placer, "client0 observed resting", apiAAPL, order)
	if err != nil {
		_ = placer.Close()
		return err
	}
	_ = observeOrder(ctx, handle, "client0 observed resting", 10*time.Second)
	orderID := handle.OrderID()
	cleanup.Track(orderID)
	_ = placer.Close()
	time.Sleep(500 * time.Millisecond)

	observer, err := dialAPI(ctx, addr, 0)
	if err != nil {
		return err
	}
	defer observer.Close()
	recordSessionReady(addr, 0, account, observer)
	orders, err := observer.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	recordOpenOrdersResult("client0 all open", orders, err)
	if err := observer.Orders().Cancel(ctx, orderID); err != nil {
		recordAPIEvent("direct_cancel_error", "client0 observed resting", func(event *apiDriverEvent) {
			event.OrderID = orderID
			event.Error = err.Error()
		})
		log.Printf("client0 direct cancel: %v", err)
	} else {
		recordAPIEvent("direct_cancel_sent", "client0 observed resting", func(event *apiDriverEvent) {
			event.OrderID = orderID
		})
	}
	time.Sleep(2 * time.Second)
	if err := observer.Orders().CancelAll(ctx); err != nil {
		log.Printf("client0 cleanup global cancel: %v", err)
	} else {
		cleanup.MarkDone()
	}
	return nil
}

func runAPICrossClientCancelAAPL(ctx context.Context, addr string, clientID int) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	recordAPIEvent("scenario_start", "", func(event *apiDriverEvent) {
		event.Server = addr
		event.ClientID = clientID
	})
	defer recordAPIEvent("scenario_end", "", nil)
	cleanup := apiOrderCleanup{addr: addr, clientID: clientID, label: "cross-client resting"}
	defer cleanup.Run()

	placer, err := dialAPI(ctx, addr, clientID)
	if err != nil {
		return err
	}
	account, err := firstManagedAccount(placer)
	if err != nil {
		_ = placer.Close()
		return err
	}
	recordSessionReady(addr, clientID, account, placer)
	if err := placer.Orders().CancelAll(ctx); err != nil {
		log.Printf("cross-client pre-cleanup global cancel: %v", err)
	}
	anchor := quoteAnchor(ctx, placer, apiAAPL, decimal.RequireFromString("200"))
	order := withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTC)
	handle, err := placeAPIOrder(ctx, placer, "cross-client resting", apiAAPL, order)
	if err != nil {
		_ = placer.Close()
		return err
	}
	_ = observeOrder(ctx, handle, "cross-client resting", 10*time.Second)
	orderID := handle.OrderID()
	cleanup.Track(orderID)
	_ = placer.Close()
	time.Sleep(500 * time.Millisecond)

	cancellerID := clientID + 1
	canceller, err := dialAPI(ctx, addr, cancellerID)
	if err != nil {
		return err
	}
	defer canceller.Close()
	recordSessionReady(addr, cancellerID, account, canceller)
	orders, err := canceller.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
	recordOpenOrdersResult("cross-client all open", orders, err)
	if err := canceller.Orders().Cancel(ctx, orderID); err != nil {
		recordAPIEvent("direct_cancel_error", "cross-client resting", func(event *apiDriverEvent) {
			event.ClientID = cancellerID
			event.OrderID = orderID
			event.Error = err.Error()
		})
		log.Printf("cross-client direct cancel: %v", err)
	} else {
		recordAPIEvent("direct_cancel_sent", "cross-client resting", func(event *apiDriverEvent) {
			event.ClientID = cancellerID
			event.OrderID = orderID
		})
	}
	time.Sleep(2 * time.Second)
	if err := canceller.Orders().CancelAll(ctx); err != nil {
		log.Printf("cross-client cleanup global cancel: %v", err)
	} else {
		cleanup.MarkDone()
	}
	return nil
}

type apiOrderCleanup struct {
	addr     string
	clientID int
	label    string
	orderID  int64
	done     bool
}

func (c *apiOrderCleanup) Track(orderID int64) {
	c.orderID = orderID
}

func (c *apiOrderCleanup) MarkDone() {
	c.done = true
}

func (c *apiOrderCleanup) Run() {
	if c.done || c.orderID == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	recordAPIEvent("tracked_order_cleanup_start", c.label, func(event *apiDriverEvent) {
		event.ClientID = c.clientID
		event.OrderID = c.orderID
	})
	client, err := dialAPI(cleanupCtx, c.addr, c.clientID)
	if err != nil {
		recordAPIEvent("tracked_order_cleanup_dial_error", c.label, func(event *apiDriverEvent) {
			event.ClientID = c.clientID
			event.OrderID = c.orderID
			event.Error = err.Error()
		})
		return
	}
	defer client.Close()
	if err := client.Orders().Cancel(cleanupCtx, c.orderID); err != nil {
		recordAPIEvent("tracked_order_cleanup_cancel_error", c.label, func(event *apiDriverEvent) {
			event.ClientID = c.clientID
			event.OrderID = c.orderID
			event.Error = err.Error()
		})
	} else {
		recordAPIEvent("tracked_order_cleanup_cancel_sent", c.label, func(event *apiDriverEvent) {
			event.ClientID = c.clientID
			event.OrderID = c.orderID
		})
	}
	if err := client.Orders().CancelAll(cleanupCtx); err != nil {
		recordAPIEvent("tracked_order_cleanup_global_cancel_error", c.label, func(event *apiDriverEvent) {
			event.ClientID = c.clientID
			event.OrderID = c.orderID
			event.Error = err.Error()
		})
		return
	}
	recordAPIEvent("tracked_order_cleanup_global_cancel_sent", c.label, func(event *apiDriverEvent) {
		event.ClientID = c.clientID
		event.OrderID = c.orderID
	})
}

func baseAPIOrder(account string, quantity decimal.Decimal, action ibkr.OrderAction, orderType ibkr.OrderType) ibkr.Order {
	return ibkr.Order{
		Action:    action,
		OrderType: orderType,
		Quantity:  quantity,
		TIF:       ibkr.TIFDay,
		Account:   account,
		OrderRef:  apiOrderRef("capture"),
	}
}

func placeObserveFlatten(ctx context.Context, client *ibkr.Client, account string, label string, order ibkr.Order, wait time.Duration) error {
	handle, err := placeAPIOrder(ctx, client, label, apiAAPL, order)
	if err != nil {
		return err
	}
	obs := observeOrder(ctx, handle, label, wait)
	if obs.AnyFill() && order.Action == ibkr.Buy {
		return flattenAAPL(ctx, client, account, label, obs.filledQty)
	}
	return nil
}

func placeAPIOrder(ctx context.Context, client *ibkr.Client, label string, contract ibkr.Contract, order ibkr.Order) (*ibkr.OrderHandle, error) {
	recordAPIEvent("place_order_start", label, func(event *apiDriverEvent) {
		event.Account = order.Account
		event.OrderRef = order.OrderRef
		event.Symbol = contract.Symbol
		event.SecType = string(contract.SecType)
		event.Action = string(order.Action)
		event.OrderType = string(order.OrderType)
		event.TIF = string(order.TIF)
		event.Quantity = order.Quantity.String()
		event.LmtPrice = order.LmtPrice.String()
		event.AuxPrice = order.AuxPrice.String()
		event.ParentID = order.ParentID
		event.OCAGroup = order.OcaGroup
	})
	handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{Contract: contract, Order: order})
	if err != nil {
		recordAPIEvent("place_order_error", label, func(event *apiDriverEvent) {
			event.Account = order.Account
			event.OrderRef = order.OrderRef
			event.Symbol = contract.Symbol
			event.SecType = string(contract.SecType)
			event.Action = string(order.Action)
			event.OrderType = string(order.OrderType)
			event.Error = err.Error()
		})
		return nil, err
	}
	recordAPIEvent("place_order_sent", label, func(event *apiDriverEvent) {
		event.OrderID = handle.OrderID()
		event.Account = order.Account
		event.OrderRef = order.OrderRef
		event.Symbol = contract.Symbol
		event.SecType = string(contract.SecType)
		event.Action = string(order.Action)
		event.OrderType = string(order.OrderType)
	})
	return handle, nil
}

func modifyAPIOrder(ctx context.Context, handle *ibkr.OrderHandle, label string, order ibkr.Order) error {
	recordAPIEvent("modify_order_start", label, func(event *apiDriverEvent) {
		event.OrderID = handle.OrderID()
		event.Account = order.Account
		event.Action = string(order.Action)
		event.OrderType = string(order.OrderType)
		event.TIF = string(order.TIF)
		event.Quantity = order.Quantity.String()
		event.LmtPrice = order.LmtPrice.String()
		event.AuxPrice = order.AuxPrice.String()
		event.ParentID = order.ParentID
	})
	if err := handle.Modify(ctx, order); err != nil {
		recordAPIEvent("modify_order_error", label, func(event *apiDriverEvent) {
			event.OrderID = handle.OrderID()
			event.Error = err.Error()
		})
		return err
	}
	recordAPIEvent("modify_order_sent", label, func(event *apiDriverEvent) {
		event.OrderID = handle.OrderID()
	})
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

func withTIF(order ibkr.Order, tif ibkr.TimeInForce) ibkr.Order {
	order.TIF = tif
	return order
}

func withGoodTillDate(order ibkr.Order, value string) ibkr.Order {
	order.GoodTillDate = value
	return order
}

func withGoodAfterTime(order ibkr.Order, value string) ibkr.Order {
	order.GoodAfterTime = value
	return order
}

func withAllOrNone(order ibkr.Order) ibkr.Order {
	order.AllOrNone = new(true)
	return order
}

func withMinQty(order ibkr.Order, quantity decimal.Decimal, minQty decimal.Decimal) ibkr.Order {
	order.Quantity = quantity
	order.MinQty = minQty
	return order
}

func withPercentOffset(order ibkr.Order, percent decimal.Decimal) ibkr.Order {
	order.PercentOffset = percent
	return order
}

func withTrailingPercent(order ibkr.Order, anchor decimal.Decimal, percent decimal.Decimal) ibkr.Order {
	order.TrailStopPrice = farSell(anchor)
	order.TrailingPercent = percent
	return order
}

func withTriggerMethod(order ibkr.Order, triggerMethod int) ibkr.Order {
	order.TriggerMethod = triggerMethod
	return order
}

func withOrderRef(order ibkr.Order, ref string) ibkr.Order {
	order.OrderRef = ref
	return order
}

func withScale(order ibkr.Order) ibkr.Order {
	order.ScaleInitLevelSize = 1
	order.ScaleSubsLevelSize = 1
	order.ScalePriceIncrement = decimal.RequireFromString("0.05")
	return order
}

func withActiveWindow(order ibkr.Order, start string, stop string) ibkr.Order {
	order.ActiveStartTime = start
	order.ActiveStopTime = stop
	return order
}

func withPriceManagement(order ibkr.Order) ibkr.Order {
	order.UsePriceMgmtAlgo = new(true)
	return order
}

func withAdjustedStop(order ibkr.Order, anchor decimal.Decimal) ibkr.Order {
	order.AdjustedOrderType = ibkr.OrderTypeStopLimit
	order.TriggerPrice = farBuy(anchor)
	order.AdjustedStopPrice = farBuy(anchor).Sub(decimal.NewFromInt(1))
	order.AdjustedStopLimitPrice = farBuy(anchor).Sub(decimal.RequireFromString("0.50"))
	order.AdjustedTrailingAmount = decimal.RequireFromString("1")
	order.AdjustableTrailingUnit = 0
	return order
}

func withManualOrderTime(order ibkr.Order, value string) ibkr.Order {
	order.ManualOrderTime = value
	return order
}

func withAdvancedErrorOverride(order ibkr.Order, value string) ibkr.Order {
	order.AdvancedErrorOverride = value
	return order
}

func withAlgo(order ibkr.Order, strategy string, params []ibkr.TagValue) ibkr.Order {
	order.AlgoStrategy = strategy
	order.AlgoParams = append([]ibkr.TagValue(nil), params...)
	return order
}

func withDisplaySize(order ibkr.Order, displaySize int) ibkr.Order {
	order.DisplaySize = displaySize
	return order
}

func orderTimestamp(t time.Time) string {
	return t.UTC().Format("20060102 15:04:05 UTC")
}

func quoteAnchor(ctx context.Context, client *ibkr.Client, contract ibkr.Contract, fallback decimal.Decimal) decimal.Decimal {
	if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
		log.Printf("set live market data type: %v", err)
		recordAPIEvent("market_data_type_error", "", func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
	}
	quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
	if err != nil {
		log.Printf("live quote failed: %v", err)
		recordAPIEvent("quote_error", "", func(event *apiDriverEvent) {
			event.Symbol = contract.Symbol
			event.SecType = string(contract.SecType)
			event.Error = err.Error()
		})
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			log.Printf("set delayed market data type: %v", err)
			recordAPIEvent("market_data_type_error", "", func(event *apiDriverEvent) {
				event.Error = err.Error()
			})
		}
		quote, err = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: contract})
		if err != nil {
			log.Printf("delayed quote failed; fallback anchor %s: %v", fallback, err)
			recordAPIEvent("quote_fallback", "", func(event *apiDriverEvent) {
				event.Symbol = contract.Symbol
				event.SecType = string(contract.SecType)
				event.Price = fallback.String()
				event.Error = err.Error()
			})
			return fallback
		}
	}
	for _, candidate := range []decimal.Decimal{quote.Last, quote.Ask, quote.Bid, quote.Close} {
		if candidate.IsPositive() {
			recordAPIEvent("quote_anchor", "", func(event *apiDriverEvent) {
				event.Symbol = contract.Symbol
				event.SecType = string(contract.SecType)
				event.Price = candidate.String()
			})
			return candidate
		}
	}
	recordAPIEvent("quote_anchor_fallback", "", func(event *apiDriverEvent) {
		event.Symbol = contract.Symbol
		event.SecType = string(contract.SecType)
		event.Price = fallback.String()
	})
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

func recordProbeResult(kind string, label string, count int, err error) {
	if err != nil {
		recordAPIEvent(kind+"_error", label, func(event *apiDriverEvent) {
			event.Count = count
			event.Error = err.Error()
		})
		return
	}
	recordAPIEvent(kind, label, func(event *apiDriverEvent) {
		event.Count = count
	})
}

func observeBars(ctx context.Context, sub *ibkr.Subscription[ibkr.Bar], wait time.Duration) int {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	var count int
	for {
		select {
		case bar, ok := <-sub.Events():
			if !ok {
				return count
			}
			count++
			log.Printf("bar update time=%s close=%s", bar.Time.Format(time.RFC3339), bar.Close)
		case <-sub.Done():
			return count
		case <-timer.C:
			return count
		case <-ctx.Done():
			return count
		}
	}
}

func observeTicks(ctx context.Context, sub *ibkr.Subscription[ibkr.TickByTickData], wait time.Duration) int {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	var count int
	for {
		select {
		case tick, ok := <-sub.Events():
			if !ok {
				return count
			}
			count++
			log.Printf("tick-by-tick update type=%d price=%s bid=%s ask=%s midpoint=%s", tick.TickType, tick.Price, tick.BidPrice, tick.AskPrice, tick.MidPoint)
		case <-sub.Done():
			return count
		case <-timer.C:
			return count
		case <-ctx.Done():
			return count
		}
	}
}

func observeQuotes(ctx context.Context, sub *ibkr.Subscription[ibkr.QuoteUpdate], label string, wait time.Duration) int {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	var count int
	for {
		select {
		case quote, ok := <-sub.Events():
			if !ok {
				recordProbeResult("quote_subscription", label, count, nil)
				return count
			}
			count++
			log.Printf("%s quote update changed=%d last=%s bid=%s ask=%s", label, quote.Changed, quote.Snapshot.Last, quote.Snapshot.Bid, quote.Snapshot.Ask)
		case <-sub.Done():
			recordProbeResult("quote_subscription", label, count, sub.Err())
			return count
		case <-timer.C:
			recordProbeResult("quote_subscription", label, count, nil)
			return count
		case <-ctx.Done():
			recordProbeResult("quote_subscription", label, count, ctx.Err())
			return count
		}
	}
}

type orderObservation struct {
	sawExecution bool
	executionQty decimal.Decimal
	statusQty    decimal.Decimal
	filledQty    decimal.Decimal
	lastStatus   ibkr.OrderStatus
	terminal     bool
}

func (o orderObservation) AnyFill() bool {
	return o.filledQty.IsPositive()
}

func (o orderObservation) FullFill() bool {
	return o.lastStatus == ibkr.OrderStatusFilled
}

func (o *orderObservation) refreshFilledQty() {
	if o.statusQty.GreaterThan(o.executionQty) {
		o.filledQty = o.statusQty
		return
	}
	o.filledQty = o.executionQty
}

func (o *orderObservation) Merge(other orderObservation) {
	o.sawExecution = o.sawExecution || other.sawExecution
	o.executionQty = o.executionQty.Add(other.executionQty)
	if other.statusQty.GreaterThan(o.statusQty) {
		o.statusQty = other.statusQty
	}
	if other.lastStatus != "" {
		o.lastStatus = other.lastStatus
	}
	o.terminal = o.terminal || other.terminal
	o.refreshFilledQty()
}

func observeOrder(ctx context.Context, handle *ibkr.OrderHandle, label string, wait time.Duration) orderObservation {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	var obs orderObservation
	seenExecIDs := make(map[string]struct{})

	record := func(evt ibkr.OrderEvent) {
		logOrderEvent(label, evt)
		recordOrderEvent(label, evt)
		if evt.Execution != nil {
			if evt.Execution.ExecID != "" {
				if _, ok := seenExecIDs[evt.Execution.ExecID]; ok {
					return
				}
				seenExecIDs[evt.Execution.ExecID] = struct{}{}
			}
			obs.sawExecution = true
			obs.executionQty = obs.executionQty.Add(evt.Execution.Shares)
			obs.refreshFilledQty()
		}
		if evt.Status != nil {
			obs.lastStatus = evt.Status.Status
			if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
				obs.terminal = true
			}
			if evt.Status.Filled.GreaterThan(obs.statusQty) {
				obs.statusQty = evt.Status.Filled
				obs.refreshFilledQty()
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
				return obs
			}
			record(evt)
			if evt.Status != nil {
				if ibkr.IsTerminalOrderStatus(evt.Status.Status) {
					drain()
					return obs
				}
			}
		case <-handle.Done():
			drain()
			if err := handle.Wait(); err != nil {
				log.Printf("%s handle done error: %v", label, err)
			}
			return obs
		case <-timer.C:
			drain()
			return obs
		case <-ctx.Done():
			drain()
			return obs
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

func recordOrderEvent(label string, evt ibkr.OrderEvent) {
	if evt.OpenOrder != nil {
		recordAPIEvent("open_order", label, func(event *apiDriverEvent) {
			order := evt.OpenOrder
			event.OrderID = order.OrderID
			event.Account = order.Account
			event.Symbol = order.Contract.Symbol
			event.SecType = string(order.Contract.SecType)
			event.Action = string(order.Action)
			event.OrderType = string(order.OrderType)
			event.TIF = string(order.TIF)
			event.Quantity = order.Quantity.String()
			event.Filled = order.Filled.String()
			event.Remaining = order.Remaining.String()
			event.LmtPrice = order.LmtPrice.String()
			event.AuxPrice = order.AuxPrice.String()
			event.Status = string(order.Status)
			event.ParentID = order.ParentID
			event.OCAGroup = order.OcaGroup
			event.OrderRef = order.OrderRef
			event.PermID = order.PermID
			if len(order.AlgoParams) > 0 {
				event.Values = tagValuesMap(order.AlgoParams)
			}
		})
	}
	if evt.Status != nil {
		recordAPIEvent("order_status", label, func(event *apiDriverEvent) {
			status := evt.Status
			event.OrderID = status.OrderID
			event.Status = string(status.Status)
			event.Filled = status.Filled.String()
			event.Remaining = status.Remaining.String()
			event.AvgPrice = status.AvgFillPrice.String()
			event.LastPrice = status.LastFillPrice.String()
			event.ParentID = status.ParentID
			event.PermID = status.PermID
			event.WhyHeld = status.WhyHeld
		})
	}
	if evt.Execution != nil {
		recordAPIEvent("execution", label, func(event *apiDriverEvent) {
			exec := evt.Execution
			event.OrderID = exec.OrderID
			event.ExecID = exec.ExecID
			event.Account = exec.Account
			event.Symbol = exec.Symbol
			event.Side = exec.Side
			event.Quantity = exec.Shares.String()
			event.Price = exec.Price.String()
			event.EventTime = exec.Time.Format(time.RFC3339)
		})
	}
	if evt.Commission != nil {
		recordAPIEvent("commission", label, func(event *apiDriverEvent) {
			commission := evt.Commission
			event.ExecID = commission.ExecID
			event.Commission = commission.Commission.String()
			event.Currency = commission.Currency
			event.RealizedPNL = commission.RealizedPNL.String()
		})
	}
}

func tagValuesMap(values []ibkr.TagValue) map[string]string {
	out := make(map[string]string, len(values))
	for _, value := range values {
		out[value.Tag] = value.Value
	}
	return out
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
	recordAPIEvent("cancel_order_start", label, func(event *apiDriverEvent) {
		event.OrderID = handle.OrderID()
	})
	if err := handle.Cancel(ctx); err != nil {
		log.Printf("%s cancel error: %v", label, err)
		recordAPIEvent("cancel_order_error", label, func(event *apiDriverEvent) {
			event.OrderID = handle.OrderID()
			event.Error = err.Error()
		})
	} else {
		recordAPIEvent("cancel_order_sent", label, func(event *apiDriverEvent) {
			event.OrderID = handle.OrderID()
		})
	}
}

func flattenAAPL(ctx context.Context, client *ibkr.Client, account string, label string, qty decimal.Decimal) error {
	order := ibkr.Order{
		Action:    ibkr.Sell,
		OrderType: ibkr.OrderTypeMarket,
		Quantity:  qty,
		TIF:       ibkr.TIFDay,
		Account:   account,
		OrderRef:  apiOrderRef("flatten"),
	}
	handle, err := placeAPIOrder(ctx, client, label+" flatten", apiAAPL, order)
	if err != nil {
		recordAPIEvent("flatten_order_error", label, func(event *apiDriverEvent) {
			event.Symbol = apiAAPL.Symbol
			event.SecType = string(apiAAPL.SecType)
			event.Quantity = qty.String()
			event.Error = err.Error()
		})
		return err
	}
	recordAPIEvent("flatten_order_placed", label, func(event *apiDriverEvent) {
		event.OrderID = handle.OrderID()
		event.Symbol = apiAAPL.Symbol
		event.SecType = string(apiAAPL.SecType)
		event.Quantity = qty.String()
	})
	_ = observeOrder(ctx, handle, label+" flatten", 30*time.Second)
	return nil
}

func flattenAAPLFill(ctx context.Context, client *ibkr.Client, account string, label string, filledAction ibkr.OrderAction, qty decimal.Decimal) error {
	return flattenStockFill(ctx, client, account, label, apiAAPL, filledAction, qty)
}

func flattenStockFill(ctx context.Context, client *ibkr.Client, account string, label string, contract ibkr.Contract, filledAction ibkr.OrderAction, qty decimal.Decimal) error {
	action := ibkr.Sell
	if filledAction == ibkr.Sell {
		action = ibkr.Buy
	}
	order := ibkr.Order{
		Action:    action,
		OrderType: ibkr.OrderTypeMarket,
		Quantity:  qty,
		TIF:       ibkr.TIFDay,
		Account:   account,
		OrderRef:  apiOrderRef("flatten"),
	}
	handle, err := placeAPIOrder(ctx, client, label+" flatten", contract, order)
	if err != nil {
		recordAPIEvent("flatten_order_error", label, func(event *apiDriverEvent) {
			event.Symbol = contract.Symbol
			event.SecType = string(contract.SecType)
			event.Action = string(action)
			event.Quantity = qty.String()
			event.Error = err.Error()
		})
		return err
	}
	recordAPIEvent("flatten_order_placed", label, func(event *apiDriverEvent) {
		event.OrderID = handle.OrderID()
		event.Symbol = contract.Symbol
		event.SecType = string(contract.SecType)
		event.Action = string(action)
		event.Quantity = qty.String()
	})
	_ = observeOrder(ctx, handle, label+" flatten", 30*time.Second)
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
		recordAPIEvent("executions_query_error", label, func(event *apiDriverEvent) {
			event.Account = req.Account
			event.Symbol = req.Symbol
			event.Error = err.Error()
		})
		return
	}
	log.Printf("%s query updates=%d", label, len(updates))
	recordAPIEvent("executions_query", label, func(event *apiDriverEvent) {
		event.Account = req.Account
		event.Symbol = req.Symbol
		event.Count = len(updates)
	})
}

func queryCompleted(client *ibkr.Client, label string) {
	queryCompletedVariant(client, label, true)
}

func queryCompletedVariant(client *ibkr.Client, label string, apiOnly bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	orders, err := client.Orders().Completed(ctx, apiOnly)
	if err != nil {
		log.Printf("%s query: %v", label, err)
		recordAPIEvent("completed_orders_query_error", label, func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
		return
	}
	log.Printf("%s query orders=%d", label, len(orders))
	recordAPIEvent("completed_orders_query", label, func(event *apiDriverEvent) {
		event.Count = len(orders)
		event.Values = map[string]string{"api_only": strconv.FormatBool(apiOnly)}
	})
}

func recordSessionReady(addr string, clientID int, account string, client *ibkr.Client) {
	snapshot := client.Session()
	log.Printf("api session ready: server_version=%d account=%s next_valid_id=%d client_id=%d", snapshot.ServerVersion, account, snapshot.NextValidID, clientID)
	recordAPIEvent("session_ready", "", func(event *apiDriverEvent) {
		event.Account = account
		event.Server = addr
		event.ClientID = clientID
		event.ServerVer = snapshot.ServerVersion
		event.NextOrderID = snapshot.NextValidID
	})
}

func recordOpenOrdersResult(label string, orders []ibkr.OpenOrder, err error) {
	if err != nil {
		log.Printf("%s open orders: %v", label, err)
		recordAPIEvent("open_orders_query_error", label, func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
		return
	}
	log.Printf("%s open orders=%d", label, len(orders))
	recordAPIEvent("open_orders_query", label, func(event *apiDriverEvent) {
		event.Count = len(orders)
		if len(orders) > 0 {
			event.OrderID = orders[0].OrderID
			event.Account = orders[0].Account
			event.Symbol = orders[0].Contract.Symbol
			event.SecType = string(orders[0].Contract.SecType)
			event.Action = string(orders[0].Action)
			event.OrderType = string(orders[0].OrderType)
			event.TIF = string(orders[0].TIF)
			event.Quantity = orders[0].Quantity.String()
			event.Remaining = orders[0].Remaining.String()
			event.Status = string(orders[0].Status)
		}
	})
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

// ---------------------------------------------------------------------------
// New capture scenarios
// ---------------------------------------------------------------------------

var apiEURUSD = ibkr.Contract{
	Symbol:   "EUR",
	SecType:  ibkr.SecTypeForex,
	Exchange: "IDEALPRO",
	Currency: "USD",
}

var apiMSFT = ibkr.Contract{
	ConID:    272093,
	Symbol:   "MSFT",
	SecType:  ibkr.SecTypeStock,
	Exchange: "SMART",
	Currency: "USD",
}

func runAPIForexLifecycleEURUSD(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiEURUSD, decimal.RequireFromString("1.10"))
		log.Printf("EUR.USD anchor: %s", anchor)

		// Far LMT rest.
		order := baseAPIOrder(account, decimal.NewFromInt(100000), ibkr.Buy, ibkr.OrderTypeLimit)
		order.LmtPrice = anchor.Mul(decimal.RequireFromString("0.90")).Round(5)

		handle, err := placeAPIOrder(ctx, client, "forex rest", apiEURUSD, order)
		if err != nil {
			log.Printf("forex place: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "forex rest", 8*time.Second)

		// Modify price.
		order.LmtPrice = anchor.Mul(decimal.RequireFromString("0.92")).Round(5)
		if err := modifyAPIOrder(ctx, handle, "forex modified", order); err != nil {
			log.Printf("forex modify: %v", err)
		}
		_ = observeOrder(ctx, handle, "forex modified", 8*time.Second)

		// Cancel.
		cancelOrder(ctx, handle, "forex")
		_ = observeOrder(ctx, handle, "forex cancel", 8*time.Second)
		return nil
	})
}

func runAPIWhatIfMarginAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 1*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		order := baseAPIOrder(account, decimal.NewFromInt(100), ibkr.Buy, ibkr.OrderTypeMarket)
		order.WhatIf = new(true)

		handle, err := placeAPIOrder(ctx, client, "whatif mkt buy 100", apiAAPL, order)
		if err != nil {
			log.Printf("whatif place: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "whatif mkt buy 100", 15*time.Second)
		return nil
	})
}

func runAPIStressRapidFireAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL stress anchor: %s", anchor)

		const n = 10
		handles := make([]*ibkr.OrderHandle, 0, n)
		for i := 0; i < n; i++ {
			order := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit), farBuy(anchor).Add(decimal.NewFromInt(int64(i))))
			h, err := placeAPIOrder(ctx, client, fmt.Sprintf("stress[%d]", i), apiAAPL, order)
			if err != nil {
				log.Printf("stress place[%d]: %v", i, err)
				continue
			}
			handles = append(handles, h)
			log.Printf("stress placed[%d]: orderID=%d", i, h.OrderID())
		}

		// Brief observation window.
		for i, h := range handles {
			_ = observeOrder(ctx, h, fmt.Sprintf("stress[%d]", i), 3*time.Second)
		}

		// Global cancel.
		if err := client.Orders().CancelAll(ctx); err != nil {
			log.Printf("stress global cancel: %v", err)
		}

		for i, h := range handles {
			_ = observeOrder(ctx, h, fmt.Sprintf("stress[%d] cancel", i), 8*time.Second)
		}
		return nil
	})
}

func runAPIScaleInCampaignAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL scale-in anchor: %s", anchor)

		// 2x MKT buys.
		filledQty := decimal.Zero
		for i := 0; i < 2; i++ {
			order := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.Buy, ibkr.OrderTypeMarket)
			handle, err := placeAPIOrder(ctx, client, fmt.Sprintf("scale buy[%d]", i), apiAAPL, order)
			if err != nil {
				log.Printf("scale buy[%d]: %v", i, err)
				continue
			}
			obs := observeOrder(ctx, handle, fmt.Sprintf("scale buy[%d]", i), 20*time.Second)
			if obs.AnyFill() {
				filledQty = filledQty.Add(obs.filledQty)
			}
		}
		if !filledQty.IsPositive() {
			log.Printf("scale-in: no fills, market may be closed")
			return nil
		}

		// Protective stop-loss.
		stopOrder := baseAPIOrder(account, filledQty, ibkr.Sell, ibkr.OrderTypeStop)
		stopOrder.AuxPrice = farBuy(anchor)
		stopOrder.TIF = ibkr.TIFGTC
		stopHandle, err := placeAPIOrder(ctx, client, "scale stop-loss", apiAAPL, stopOrder)
		if err != nil {
			log.Printf("scale stop-loss: %v", err)
		} else {
			_ = observeOrder(ctx, stopHandle, "scale stop-loss", 8*time.Second)
			cancelOrder(ctx, stopHandle, "scale stop-loss")
			_ = observeOrder(ctx, stopHandle, "scale stop-loss cancel", 8*time.Second)
		}

		// Flatten.
		if err := flattenAAPL(ctx, client, account, "scale flatten", filledQty); err != nil {
			log.Printf("scale flatten: %v", err)
		}

		queryAAPLExecutions(client, account)
		return nil
	})
}

func runAPIIOCFOKAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL IOC/FOK anchor: %s", anchor)

		// IOC marketable.
		iocOrder := baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit)
		iocOrder.LmtPrice = marketableBuy(anchor)
		iocOrder.TIF = ibkr.TIFIOC
		handle, err := placeAPIOrder(ctx, client, "ioc marketable", apiAAPL, iocOrder)
		if err != nil {
			log.Printf("ioc place: %v", err)
		} else {
			obs := observeOrder(ctx, handle, "ioc marketable", 15*time.Second)
			if obs.AnyFill() {
				if err := flattenAAPL(ctx, client, account, "ioc flatten", obs.filledQty); err != nil {
					log.Printf("ioc flatten: %v", err)
				}
			}
		}

		// FOK marketable.
		fokOrder := baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit)
		fokOrder.LmtPrice = marketableBuy(anchor)
		fokOrder.TIF = ibkr.TIFFOK
		handle, err = placeAPIOrder(ctx, client, "fok fillable", apiAAPL, fokOrder)
		if err != nil {
			log.Printf("fok fillable place: %v", err)
		} else {
			obs := observeOrder(ctx, handle, "fok fillable", 15*time.Second)
			if obs.AnyFill() {
				if err := flattenAAPL(ctx, client, account, "fok fillable flatten", obs.filledQty); err != nil {
					log.Printf("fok fillable flatten: %v", err)
				}
			}
		}

		// FOK unfillable.
		fokFarOrder := baseAPIOrder(account, apiStockOrderQuantity, ibkr.Buy, ibkr.OrderTypeLimit)
		fokFarOrder.LmtPrice = farBuy(anchor)
		fokFarOrder.TIF = ibkr.TIFFOK
		handle, err = placeAPIOrder(ctx, client, "fok unfillable", apiAAPL, fokFarOrder)
		if err != nil {
			log.Printf("fok unfillable place: %v", err)
		} else {
			_ = observeOrder(ctx, handle, "fok unfillable", 15*time.Second)
		}

		queryAAPLExecutions(client, account)
		return nil
	})
}
