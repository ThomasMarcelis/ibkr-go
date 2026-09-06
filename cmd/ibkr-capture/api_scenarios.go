package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"math"
	"net"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

var apiAAPL = ibkr.Contract{
	ConID:    265598,
	Symbol:   "AAPL",
	SecType:  ibkr.SecTypeStock,
	Exchange: "SMART",
	Currency: "USD",
}

const apiQuotePriceOrSizeFields = ibkr.QuoteFieldBid | ibkr.QuoteFieldAsk | ibkr.QuoteFieldLast |
	ibkr.QuoteFieldBidSize | ibkr.QuoteFieldAskSize | ibkr.QuoteFieldLastSize |
	ibkr.QuoteFieldOpen | ibkr.QuoteFieldHigh | ibkr.QuoteFieldLow | ibkr.QuoteFieldClose

func optionalDecimalString(value *decimal.Decimal) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func setRecordedOrderPrices(event *apiDriverEvent, limit, auxiliary *decimal.Decimal) {
	event.LmtPrice = optionalDecimalString(limit)
	event.AuxPrice = optionalDecimalString(auxiliary)
}

type apiDriverRecorder struct {
	mu          sync.Mutex
	file        *os.File
	enc         *json.Encoder
	err         error
	scenario    string
	definition  *scenario
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
	Submitter   string            `json:"submitter,omitempty"`
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
	apiStockOrderQuantity         = decimal.NewFromInt(1)
	apiStockCampaignOrderQuantity = decimal.NewFromInt(1)
	apiSingleContractQuantity     = decimal.NewFromInt(1)
	apiOptionContractQuantity     = decimal.NewFromInt(1)
)

func newAPIDriverRecorder(path string, name string, definition *scenario) (*apiDriverRecorder, error) {
	now := time.Now().UTC()
	rec := &apiDriverRecorder{
		scenario:    name,
		definition:  definition,
		runID:       now.Format("20060102T150405Z"),
		scenarioTag: scenarioHash(name),
	}
	if path == "" {
		return rec, nil
	}
	// #nosec G304,G703 -- path is the operator-selected private driver-event output;
	// it does not incorporate data received from Gateway/TWS.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create driver events: %w", err)
	}
	rec.file = file
	rec.enc = json.NewEncoder(file)
	return rec, nil
}

func scenarioHash(scenario string) string {
	sum := sha256.Sum256([]byte(scenario))
	return hex.EncodeToString(sum[:4])
}

func (r *apiDriverRecorder) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return r.err
	}
	r.err = errors.Join(r.err, r.file.Sync(), r.file.Close())
	r.file = nil
	r.enc = nil
	return r.err
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
		r.err = errors.Join(r.err, fmt.Errorf("encode %s driver event: %w", kind, err))
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
	if clientID < 0 || int64(clientID) > math.MaxInt32 {
		return nil, fmt.Errorf("client id %d is outside signed-int32 range", clientID)
	}
	protocolClientID := ibkr.ClientID(clientID) // #nosec G115 -- range checked above
	return ibkr.DialContext(ctx, ibkr.WithHost(host), ibkr.WithPort(port), ibkr.WithClientID(protocolClientID))
}

func firstManagedAccount(client *ibkr.Client) (string, error) {
	snapshot := client.Session()
	if len(snapshot.ManagedAccounts) == 0 {
		return "", fmt.Errorf("session has no managed accounts")
	}
	return snapshot.ManagedAccounts[0], nil
}

func managedAccounts(client *ibkr.Client) ([]string, error) {
	snapshot := client.Session()
	if len(snapshot.ManagedAccounts) == 0 {
		return nil, fmt.Errorf("session has no managed accounts")
	}
	return snapshot.ManagedAccounts, nil
}

// paperAccountPrefix is IBKR's paper-trading account prefix. Every
// order-mutating capture operation is refused on any account that lacks it, so
// a global cancel can never reach a live real-money account (e.g. "U…").
const paperAccountPrefix = "DU"

// isPaperAccount reports whether account is an IBKR paper-trading account.
func isPaperAccount(account string) bool {
	return strings.HasPrefix(account, paperAccountPrefix)
}

// requirePaperAccount is the belt-and-braces guard that must precede every
// order-mutating call the tool issues. A DU prefix is necessary but not
// sufficient: the operator must name the exact dedicated paper account in
// IBKR_PAPER_ACCOUNT so a different paper account cannot be mutated by
// accident.
func requirePaperAccount(account, operation string) error {
	if !isPaperAccount(account) {
		return fmt.Errorf("refusing %s on non-paper account %q: order-mutating capture operations require an IBKR paper account (%q prefix)", operation, account, paperAccountPrefix)
	}
	allowed := strings.TrimSpace(os.Getenv("IBKR_PAPER_ACCOUNT"))
	if allowed == "" {
		return fmt.Errorf("refusing %s on paper account %q: set IBKR_PAPER_ACCOUNT to the exact dedicated account", operation, account)
	}
	if account != allowed {
		return fmt.Errorf("refusing %s on paper account %q: IBKR_PAPER_ACCOUNT allows %q", operation, account, allowed)
	}
	return nil
}

func requirePaperAccounts(accounts []string, operation string) error {
	if len(accounts) == 0 {
		return fmt.Errorf("refusing %s: session has no managed accounts", operation)
	}
	for _, account := range accounts {
		if err := requirePaperAccount(account, operation); err != nil {
			return err
		}
	}
	return nil
}

func requirePaperTradingSession(client *ibkr.Client, fallbackAccount string, operation string) error {
	if client == nil {
		return requirePaperAccount(fallbackAccount, operation)
	}
	accounts, err := managedAccounts(client)
	if err != nil {
		return err
	}
	if err := requirePaperAccounts(accounts, operation); err != nil {
		return err
	}
	if fallbackAccount == "" {
		return nil
	}
	if !slices.Contains(accounts, fallbackAccount) {
		return fmt.Errorf("refusing %s on account %q: current session manages %v", operation, fallbackAccount, accounts)
	}
	return requirePaperAccount(fallbackAccount, operation)
}

// guardedCancelAll is the only path through which this tool issues a TWS global
// cancel. In addition to the exact paper-account allowlist it requires a
// purpose-specific operator gate; normal scenario cleanup targets only order
// IDs created by that scenario.
func guardedCancelAll(ctx context.Context, client *ibkr.Client, account, operation string) error {
	if err := requirePaperTradingSession(client, account, operation); err != nil {
		return err
	}
	if err := requireGlobalCancelGate(operation); err != nil {
		return err
	}
	return client.Orders().CancelAll(ctx)
}

func requireGlobalCancelGate(operation string) error {
	if !envFlag("IBKR_CAPTURE_GLOBAL_CANCEL") {
		return fmt.Errorf("refusing %s: set IBKR_CAPTURE_GLOBAL_CANCEL=1 to authorize the paper cleanup fallback", operation)
	}
	return nil
}

func envFlag(name string) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return false
	}
	value, err := strconv.ParseBool(raw)
	return err == nil && value
}

func guardedCancelOrder(ctx context.Context, client *ibkr.Client, account string, ownerClientID int, orderID int64, operation string) error {
	if err := requirePaperTradingSession(client, account, operation); err != nil {
		return err
	}
	if ownerClientID < 0 || int64(ownerClientID) > math.MaxInt32 {
		return fmt.Errorf("owner client id %d is outside signed-int32 range", ownerClientID)
	}
	owner := ibkr.ClientID(ownerClientID) // #nosec G115 -- range checked above.
	return client.Orders().Cancel(ctx, ibkr.OrderTarget{ClientID: owner, OrderID: orderID})
}

// apiScenarioWrapper identifies which capture wrapper a run function selected.
type apiScenarioWrapper int

const (
	wrapperReadOnly apiScenarioWrapper = iota
	wrapperTrading
)

// currentScenarioName returns the name of the scenario being captured. main.go
// always installs the driver recorder (named after -scenario) before invoking
// an api run function, so this is the authoritative scenario identity.
func currentScenarioName() string {
	if apiDriver == nil {
		return ""
	}
	return apiDriver.scenario
}

func currentScenarioDefinition() *scenario {
	if apiDriver == nil {
		return nil
	}
	return apiDriver.definition
}

// verifyWrapperForScenario cross-checks the wrapper a run function chose against
// the scenario's catalog RiskClass. The trading wrapper (baseline plus
// reconciliation) is valid only for paper-trading risk classes; the read-only wrapper
// is valid only for the rest. A mismatch is a wiring bug and is refused before
// any connection is made, so a read-only scenario can never reach the cancel
// path even if it is miswired.
func verifyWrapperForScenario(name string, definition *scenario, wrapper apiScenarioWrapper) error {
	if name == "" || definition == nil {
		return fmt.Errorf("scenario identity and definition are required to verify capture wrapper")
	}
	riskClass := definition.metadata.RiskClass
	if err := validateRiskClass(riskClass); err != nil {
		return fmt.Errorf("scenario %q: %w", name, err)
	}
	wantTrading := cancelsAllowedForRiskClass(riskClass)
	gotTrading := wrapper == wrapperTrading
	if wantTrading != gotTrading {
		return fmt.Errorf("scenario %q RiskClass %q wants trading-wrapper=%t but is wired to trading-wrapper=%t", name, riskClass, wantTrading, gotTrading)
	}
	return nil
}

// apiScenario runs a read-only or entitlement-probe capture body. This path
// contains no order-mutating code, so a read-only scenario is structurally
// incapable of cancelling live orders even if it is pointed at a real-money
// account. Paper-trading scenarios must use apiTradingScenario instead.
func apiScenario(ctx context.Context, addr string, clientID int, timeout time.Duration, run func(context.Context, *ibkr.Client, string) error) error {
	if err := verifyWrapperForScenario(currentScenarioName(), currentScenarioDefinition(), wrapperReadOnly); err != nil {
		recordAPIEvent("scenario_wrapper_mismatch", "", func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
		return err
	}
	return apiScenarioBase(ctx, addr, clientID, timeout, run)
}

// apiTradingScenario runs a paper-trading capture body inside one baseline and
// reconciliation boundary. Scenario-specific targeted cleanup remains useful;
// the wrapper independently proves no working order or position delta survives.
// It may use the separately gated global cancel only when targeted cancellation
// leaves the dedicated paper account in an uncertain state.
func apiTradingScenario(ctx context.Context, addr string, clientID int, timeout time.Duration, run func(context.Context, *ibkr.Client, string) error) error {
	if err := verifyWrapperForScenario(currentScenarioName(), currentScenarioDefinition(), wrapperTrading); err != nil {
		recordAPIEvent("scenario_wrapper_mismatch", "", func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
		return err
	}
	return apiScenarioBase(ctx, addr, clientID, timeout, func(ctx context.Context, client *ibkr.Client, account string) (runErr error) {
		if err := requirePaperTradingSession(client, account, "paper trading scenario"); err != nil {
			log.Printf("%v", err)
			recordAPIEvent("trading_scenario_refused_non_paper_account", "", func(event *apiDriverEvent) {
				event.Account = account
				event.Error = err.Error()
			})
			return err
		}
		if err := requireGlobalCancelGate("paper trading scenario"); err != nil {
			recordAPIEvent("trading_scenario_refused_cleanup_gate", "", func(event *apiDriverEvent) {
				event.Error = err.Error()
			})
			return err
		}
		label := currentScenarioName()
		baseline, err := snapshotPaperCampaignBaseline(ctx, client, account)
		if err != nil {
			return err
		}
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
			defer cancel()
			// Cleanup starts on a new transport generation so scenario teardown
			// cannot leave reconciliation dependent on a connection the scenario
			// may already have damaged.
			cleanupClient, err := paperCleanupClient(cleanupCtx, client, addr, clientID, label)
			if err != nil {
				err = fmt.Errorf("%s cleanup session: %w", label, err)
				recordPaperReconciliationFailure(label, err)
				runErr = errors.Join(runErr, err)
				return
			}
			defer func() { cleanupClient.Close() }()

			for attempt := 1; attempt <= 3; attempt++ {
				cleanupAccount, err := firstManagedAccount(cleanupClient)
				if err != nil {
					err = fmt.Errorf("%s cleanup account: %w", label, err)
					recordPaperReconciliationFailure(label, err)
					runErr = errors.Join(runErr, err)
					return
				}
				if cleanupAccount != account {
					err = fmt.Errorf("%s cleanup account changed from %q to %q", label, account, cleanupAccount)
					recordPaperReconciliationFailure(label, err)
					runErr = errors.Join(runErr, err)
					return
				}
				if err := requirePaperTradingSession(cleanupClient, cleanupAccount, label+" reconciliation"); err != nil {
					recordPaperReconciliationFailure(label, err)
					runErr = errors.Join(runErr, err)
					return
				}
				recordAPIEvent("paper_reconciliation_session", label, func(event *apiDriverEvent) {
					event.Account = cleanupAccount
					event.ClientID = clientID
					event.ServerVer = cleanupClient.Session().ServerVersion
					event.Count = attempt
				})
				reconciliation, reconcileErr := reconcilePaperCampaign(cleanupCtx, cleanupClient, cleanupAccount, label, baseline)
				if reconcileErr == nil {
					recordPaperReconciliation(label, reconciliation, nil)
					return
				}
				if attempt == 3 {
					recordPaperReconciliation(label, reconciliation, reconcileErr)
					runErr = errors.Join(runErr, reconcileErr)
					return
				}

				nextClient, reconnectErr := paperCleanupClient(cleanupCtx, cleanupClient, addr, clientID, label)
				if reconnectErr != nil {
					reconnectErr = errors.Join(reconcileErr, fmt.Errorf("%s reconnect cleanup session: %w", label, reconnectErr))
					recordPaperReconciliation(label, reconciliation, reconnectErr)
					runErr = errors.Join(runErr, reconnectErr)
					return
				}
				recordAPIEvent("paper_reconciliation_retry", label, func(event *apiDriverEvent) {
					event.Count = attempt
					event.Error = reconcileErr.Error()
				})
				cleanupClient = nextClient
			}
		}()

		clearedOpenOrders, err := clearPaperOpenOrders(ctx, client, account, label+" baseline")
		if err != nil {
			return err
		}
		positions, err := snapshotPositions(ctx, client)
		if err != nil {
			return fmt.Errorf("%s post-cancel positions: %w", label, err)
		}
		if !samePositionInventory(baseline.positions, positions) {
			return fmt.Errorf("%s stale-order cancellation changed the position inventory: before=%v after=%v", label, positionInventory(baseline.positions), positionInventory(positions))
		}
		recordPaperBaseline(label, baseline, clearedOpenOrders)
		return run(ctx, client, account)
	})
}

func paperCleanupClient(ctx context.Context, current *ibkr.Client, addr string, clientID int, label string) (*ibkr.Client, error) {
	current.Close()

	dialCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	var errs error
	for attempt := 1; ; attempt++ {
		attemptCtx, attemptCancel := context.WithTimeout(dialCtx, 15*time.Second)
		client, err := dialAPI(attemptCtx, addr, clientID)
		attemptCancel()
		if err == nil {
			return client, nil
		}
		errs = errors.Join(errs, fmt.Errorf("attempt %d: %w", attempt, err))
		recordAPIEvent("paper_reconciliation_redial", label, func(event *apiDriverEvent) {
			event.Count = attempt
			event.Error = err.Error()
		})
		select {
		case <-time.After(500 * time.Millisecond):
		case <-dialCtx.Done():
			return nil, errors.Join(errs, context.Cause(dialCtx))
		}
	}
}

func recordPaperReconciliationFailure(label string, err error) {
	recordPaperReconciliation(label, paperReconciliation{
		openOrders:    "unknown",
		positions:     "unknown",
		executions:    "unknown",
		accountValues: "unknown",
	}, err)
}

// apiScenarioBase is the shared dial/session/record spine for both wrappers. It
// performs no order mutation of its own.
func apiScenarioBase(ctx context.Context, addr string, clientID int, timeout time.Duration, run func(context.Context, *ibkr.Client, string) error) error {
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
	log.Printf("api session ready: server_version=%d next_valid_id=%d", snapshot.ServerVersion, snapshot.NextValidID)
	recordAPIEvent("session_ready", "", func(event *apiDriverEvent) {
		event.Account = account
		event.ServerVer = snapshot.ServerVersion
		event.NextOrderID = snapshot.NextValidID
	})

	runErr := run(ctx, client, account)

	recordAPIEvent("scenario_end", "", func(event *apiDriverEvent) {
		if runErr != nil {
			event.Error = runErr.Error()
		}
	})
	return runErr
}

func runAPIBootstrap(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, _ *ibkr.Client, _ string) error {
		select {
		case <-time.After(3 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

func runAPICurrentTime(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		serverTime, err := client.CurrentTime(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("current_time", "", func(event *apiDriverEvent) {
			event.EventTime = serverTime.Format(time.RFC3339Nano)
		})
		return nil
	})
}

func runAPICurrentTimeMillis(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		serverTime, err := client.CurrentTimeMillis(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("current_time_millis", "", func(event *apiDriverEvent) {
			event.EventTime = serverTime.Format(time.RFC3339Nano)
		})
		return nil
	})
}

func runAPIManagedAccounts(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		accounts, err := client.ManagedAccounts(ctx)
		if err != nil {
			return err
		}
		if len(accounts) == 0 {
			return fmt.Errorf("managed accounts refresh returned no accounts")
		}
		if !slices.Equal(accounts, client.Session().ManagedAccounts) {
			return fmt.Errorf("managed accounts refresh and session snapshot disagree")
		}
		recordAPIEvent("managed_accounts", "refresh", func(event *apiDriverEvent) {
			event.Count = len(accounts)
		})
		return nil
	})
}

func runAPIRefreshOrderID(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		orderID, err := client.Orders().RefreshOrderID(ctx)
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if !ok || apiErr.Code != 321 || !strings.Contains(apiErr.Message, "Read-Only mode") {
				return err
			}
			recordAPIEvent("order_id_refresh_refused", "", func(event *apiDriverEvent) { event.Error = err.Error() })
			return nil
		}
		recordAPIEvent("order_id_refreshed", "", func(event *apiDriverEvent) {
			event.NextOrderID = orderID
		})
		return nil
	})
}

func runAPIFamilyCodes(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		codes, err := client.Accounts().FamilyCodes(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("family_codes", "", func(event *apiDriverEvent) { event.Count = len(codes) })
		return nil
	})
}

func runAPINewsProviders(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		providers, err := client.News().Providers(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("news_providers", "", func(event *apiDriverEvent) { event.Count = len(providers) })
		return nil
	})
}

func runAPINewsBulletins(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		sub, err := client.News().SubscribeBulletins(ctx, true, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe news bulletins: %w", err)
		}
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		count := 0
	collect:
		for {
			select {
			case event, ok := <-sub.Events():
				if !ok {
					if err := sub.Wait(); err != nil {
						return fmt.Errorf("news bulletin subscription: %w", err)
					}
					return errors.New("news bulletin subscription closed before observation window ended")
				}
				if event.Kind == ibkr.StreamData {
					count++
				}
			case <-sub.Done():
				if err := sub.Wait(); err != nil {
					return fmt.Errorf("news bulletin subscription: %w", err)
				}
				return errors.New("news bulletin subscription closed before observation window ended")
			case <-timer.C:
				break collect
			case <-ctx.Done():
				sub.Close()
				return ctx.Err()
			}
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "news bulletin cancellation"); err != nil {
			return err
		}
		if count == 0 {
			return errors.New("news bulletin observation window produced no callback")
		}
		recordAPIEvent("news_bulletins", "all_messages", func(event *apiDriverEvent) { event.Count = count })
		return nil
	})
}

func runAPIDepthExchanges(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		exchanges, err := client.Contracts().DepthExchanges(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("depth_exchanges", "", func(event *apiDriverEvent) { event.Count = len(exchanges) })
		return nil
	})
}

func runAPIScannerParameters(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 30*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		parameters, err := client.Scanner().Parameters(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("scanner_parameters", "", func(event *apiDriverEvent) {
			event.Values = map[string]string{"bytes": strconv.Itoa(len(parameters))}
		})
		return nil
	})
}

func runAPIUserInfo(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		userInfo, err := client.TWS().UserInfo(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("user_info", "", func(event *apiDriverEvent) {
			event.Values = map[string]string{"bytes": strconv.Itoa(len(userInfo))}
		})
		return nil
	})
}

func runAPITWSConfig(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		config, err := client.TWS().Config(ctx)
		if err != nil {
			return err
		}
		if config.API == nil || config.API.Settings == nil || config.Orders == nil {
			return fmt.Errorf("configuration response omitted API settings or order settings")
		}
		recordAPIEvent("tws_config", "snapshot", func(event *apiDriverEvent) {
			event.Count = len(config.Messages)
			event.Values = map[string]string{
				"api_settings_present": "true",
				"orders_present":       "true",
				"trusted_ip_count":     strconv.Itoa(len(config.API.Settings.TrustedIPs)),
			}
		})
		return nil
	})
}

func runAPIMarketRule(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		rule, err := client.Contracts().MarketRule(ctx, 26)
		if err != nil {
			return err
		}
		recordAPIEvent("market_rule", "", func(event *apiDriverEvent) { event.Count = len(rule.Increments) })
		return nil
	})
}

func runAPISoftDollarTiers(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		tiers, err := client.Advisors().SoftDollarTiers(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("soft_dollar_tiers", "", func(event *apiDriverEvent) { event.Count = len(tiers) })
		return nil
	})
}

func runAPIDisplayGroups(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		groups, err := client.TWS().DisplayGroups(ctx)
		if err != nil {
			return err
		}
		recordAPIEvent("display_groups", "", func(event *apiDriverEvent) { event.Count = len(groups) })
		return nil
	})
}

func runAPIDisplayGroupSubscription(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		groups, err := client.TWS().DisplayGroups(ctx)
		if err != nil {
			return fmt.Errorf("query display groups: %w", err)
		}
		if len(groups) == 0 {
			if err := fenceAPIWrites(ctx, client, "empty display group list"); err != nil {
				return err
			}
			recordAPIEvent("display_group_unavailable", "", nil)
			return nil
		}

		handle, err := client.TWS().SubscribeDisplayGroup(ctx, groups[0], ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe display group %d: %w", groups[0], err)
		}
		current := ""
		count, err := awaitSubscriptionEvidence(ctx, handle.Subscription, 8*time.Second, func(update ibkr.DisplayGroupUpdate) bool {
			current = update.ContractInfo
			return update.ContractInfo != ""
		})
		if err != nil {
			handle.Close()
			_ = handle.Wait()
			return fmt.Errorf("observe display group %d: %w", groups[0], err)
		}

		// Re-selecting the current contract exercises Update without changing the
		// operator's TWS state. An empty group reports "none", which is not a
		// valid update token and is therefore left untouched.
		updated := current != "" && current != "none"
		if updated {
			if err := handle.Update(ctx, current); err != nil {
				handle.Close()
				_ = handle.Wait()
				return fmt.Errorf("re-select display group %d contract %q: %w", groups[0], current, err)
			}
		}
		if err := closeAndFenceSubscription(ctx, client, handle.Subscription, "display group subscription"); err != nil {
			return err
		}
		recordAPIEvent("display_group_subscription", strconv.Itoa(int(groups[0])), func(event *apiDriverEvent) {
			event.Count = count
			event.Values = map[string]string{"update_sent": strconv.FormatBool(updated)}
		})
		return nil
	})
}

func runAPIFAConfigGroups(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		document, err := client.Advisors().Config(ctx, ibkr.FADataGroups)
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if ok && apiErr.OpKind == ibkr.OpFAConfig && apiErr.Code == ibkr.ErrCodeServerErrorValidatingRequest &&
				strings.Contains(apiErr.Message, "FA data operations ignored for non FA customers") {
				if err := fenceAPIWrites(ctx, client, "FA groups refusal"); err != nil {
					return err
				}
				recordAPIEvent("fa_config_refused", "groups", func(event *apiDriverEvent) { event.Error = err.Error() })
				return nil
			}
			return fmt.Errorf("request FA groups: %w", err)
		}
		if len(document) == 0 {
			return errors.New("FA groups response is empty")
		}
		if err := fenceAPIWrites(ctx, client, "FA groups response"); err != nil {
			return err
		}
		recordAPIEvent("fa_config", "groups", func(event *apiDriverEvent) { event.Count = len(document) })
		return nil
	})
}

func runAPIOpenOrders(ctx context.Context, addr string, clientID int, scope ibkr.OpenOrdersScope, label string, allowReadOnlyRefusal bool) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		orders, err := client.Orders().Open(ctx, scope)
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if allowReadOnlyRefusal && ok && apiErr.OpKind == ibkr.OpOpenOrders && apiErr.Code == 321 && strings.Contains(apiErr.Message, "Read-Only mode") {
				recordAPIEvent("open_orders_refused", label, func(event *apiDriverEvent) { event.Error = err.Error() })
				return nil
			}
			return err
		}
		recordAPIEvent("open_orders", label, func(event *apiDriverEvent) { event.Count = len(orders) })
		return nil
	})
}

func runAPIOpenOrdersClient(ctx context.Context, addr string, clientID int) error {
	return runAPIOpenOrders(ctx, addr, clientID, ibkr.OpenOrdersScopeClient, "client", true)
}

func runAPIOpenOrdersAll(ctx context.Context, addr string, clientID int) error {
	if clientID != 0 {
		return fmt.Errorf("all-client open orders require client ID 0, got %d", clientID)
	}
	return runAPIOpenOrders(ctx, addr, clientID, ibkr.OpenOrdersScopeAll, "all", false)
}

func runAPIExecutionsSnapshot(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{})
		if err != nil {
			return err
		}
		for _, execution := range executions.Executions {
			if execution.ExecID == "" {
				return errors.New("execution snapshot returned an empty execution ID")
			}
		}
		recordAPIEvent("executions", "snapshot", func(event *apiDriverEvent) {
			event.Count = len(executions.Executions)
			event.Values = map[string]string{"commission_and_fees": strconv.Itoa(len(executions.CommissionAndFees))}
		})
		return nil
	})
}

func runAPIExecutionsConcurrentAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, account string) error {
		type result struct {
			label    string
			snapshot ibkr.ExecutionSnapshot
			err      error
		}
		results := make(chan result, 3)
		queries := []struct {
			label string
			req   ibkr.ExecutionsRequest
		}{
			{label: "all", req: ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"}},
			{label: "buy", req: ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL", Side: ibkr.ExecutionFilterBuy}},
			{label: "sell", req: ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL", Side: ibkr.ExecutionFilterSell}},
		}
		for i, query := range queries {
			if i != 0 {
				time.Sleep(25 * time.Millisecond)
			}
			go func() {
				snapshot, err := client.Orders().Executions(ctx, query.req)
				results <- result{label: query.label, snapshot: snapshot, err: err}
			}()
		}
		byLabel := make(map[string]ibkr.ExecutionSnapshot, len(queries))
		for range queries {
			select {
			case result := <-results:
				if result.err != nil {
					return fmt.Errorf("%s AAPL execution query: %w", result.label, result.err)
				}
				byLabel[result.label] = result.snapshot
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		}
		if len(byLabel["buy"].Executions) == 0 || len(byLabel["sell"].Executions) == 0 {
			return fmt.Errorf("concurrent AAPL execution queries returned buy=%d sell=%d; current paper execution evidence is required", len(byLabel["buy"].Executions), len(byLabel["sell"].Executions))
		}
		allIDs := make(map[string]struct{}, len(byLabel["all"].Executions))
		for _, execution := range byLabel["all"].Executions {
			allIDs[execution.ExecID] = struct{}{}
		}
		for _, label := range []string{"buy", "sell"} {
			for _, execution := range byLabel[label].Executions {
				if _, ok := allIDs[execution.ExecID]; !ok {
					return fmt.Errorf("%s execution %q absent from overlapping all-side result", label, execution.ExecID)
				}
			}
		}
		if err := fenceAPIWrites(ctx, client, "concurrent AAPL execution queries"); err != nil {
			return err
		}
		recordAPIEvent("executions_concurrent", "AAPL", func(event *apiDriverEvent) {
			event.Count = len(byLabel["all"].Executions)
			event.Values = map[string]string{
				"buy":  strconv.Itoa(len(byLabel["buy"].Executions)),
				"sell": strconv.Itoa(len(byLabel["sell"].Executions)),
			}
		})
		return nil
	})
}

func runAPICompletedOrders(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		orders, err := client.Orders().Completed(ctx, true)
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if ok && apiErr.OpKind == ibkr.OpCompletedOrders && apiErr.Code == ibkr.ErrCodeServerErrorValidatingRequest &&
				strings.Contains(apiErr.Message, "Error validating request.-'S'") && strings.Contains(apiErr.Message, "Read-Only mode") {
				if err := fenceAPIWrites(ctx, client, "completed-orders refusal"); err != nil {
					return err
				}
				recordAPIEvent("completed_orders_refused", "api_only", func(event *apiDriverEvent) { event.Error = err.Error() })
				return nil
			}
			return fmt.Errorf("completed API orders: %w", err)
		}
		if err := fenceAPIWrites(ctx, client, "completed-orders snapshot"); err != nil {
			return err
		}
		recordAPIEvent("completed_orders", "api_only", func(event *apiDriverEvent) {
			event.Count = len(orders)
			event.Values = map[string]string{"api_only": "true"}
		})
		return nil
	})
}

func runAPIHistoricalNews(ctx context.Context, addr string, clientID int, label string, start, end time.Time, totalResults int) error {
	return apiScenario(ctx, addr, clientID, 25*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		items, err := client.News().Historical(ctx, ibkr.HistoricalNewsRequest{
			ConID:         apiAAPL.ConID,
			ProviderCodes: []ibkr.NewsProviderCode{"BRFG", "BRFUPDN", "DJNL"},
			StartTime:     start,
			EndTime:       end,
			TotalResults:  totalResults,
		})
		if err != nil {
			return err
		}
		if len(items.Items) == 0 {
			return fmt.Errorf("%s returned no historical news", label)
		}
		for _, item := range items.Items {
			if item.Time.IsZero() || item.ProviderCode == "" || item.ArticleID == "" || item.Headline == "" {
				return fmt.Errorf("%s returned an incomplete historical news item", label)
			}
			if !start.IsZero() && !item.Time.Before(start) {
				return fmt.Errorf("%s returned item %s at or after exclusive upper bound %s", label, item.Time, start)
			}
			if !end.IsZero() && item.Time.Before(end) {
				return fmt.Errorf("%s returned item %s before inclusive lower bound %s", label, item.Time, end)
			}
		}
		recordAPIEvent("historical_news", label, func(event *apiDriverEvent) {
			event.Count = len(items.Items)
			event.Values = map[string]string{"has_more": strconv.FormatBool(items.HasMore)}
		})
		return nil
	})
}

func runAPIHistoricalNewsAAPL(ctx context.Context, addr string, clientID int) error {
	return runAPIHistoricalNews(ctx, addr, clientID, "aapl_recent", time.Time{}, time.Time{}, 10)
}

func runAPIHistoricalNewsAAPLTimezoneWindow(ctx context.Context, addr string, clientID int) error {
	end := time.Now().UTC().AddDate(0, -6, 0).Truncate(time.Second)
	return runAPIHistoricalNews(ctx, addr, clientID, "aapl_utc_end", time.Time{}, end, 20)
}

func captureWSHResult(label string, op ibkr.OpKind, data ibkr.JSONDocument, err error) error {
	if err != nil {
		apiErr, ok := errors.AsType[*ibkr.APIError](err)
		if !ok || apiErr.OpKind != op || apiErr.Code != 10276 || !strings.Contains(apiErr.Message, "News feed is not allowed") {
			return err
		}
		recordAPIEvent("wsh_entitlement_refused", label, func(event *apiDriverEvent) { event.Error = err.Error() })
		return nil
	}
	if len(data) == 0 || !json.Valid(data) {
		return fmt.Errorf("%s returned invalid JSON", label)
	}
	recordAPIEvent("wsh_data", label, func(event *apiDriverEvent) {
		event.Values = map[string]string{"bytes": strconv.Itoa(len(data))}
	})
	return nil
}

func runAPIWSHMetaData(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		data, err := client.WSH().MetaData(ctx)
		return captureWSHResult("metadata", ibkr.OpWSHMetaData, data, err)
	})
}

func runAPIWSHEventDataAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		data, err := client.WSH().EventData(ctx, ibkr.WSHEventDataRequest{ConID: apiAAPL.ConID, TotalLimit: 10})
		return captureWSHResult("aapl_events", ibkr.OpWSHEventData, data, err)
	})
}

func runAPIContractDetails(ctx context.Context, addr string, clientID int, timeout time.Duration, contract ibkr.Contract) error {
	return apiScenario(ctx, addr, clientID, timeout, func(ctx context.Context, client *ibkr.Client, _ string) error {
		details, err := client.Contracts().Details(ctx, contract)
		if err != nil {
			return err
		}
		if len(details) == 0 {
			return fmt.Errorf("contract details returned no matches")
		}
		recordAPIEvent("contract_details", "", func(event *apiDriverEvent) {
			event.Count = len(details)
			event.Symbol = details[0].Symbol
			event.SecType = string(details[0].SecType)
			event.Values = map[string]string{"first_con_id": strconv.FormatInt(int64(details[0].ConID), 10)}
		})
		return nil
	})
}

func runAPIContractDetailsAAPLStock(ctx context.Context, addr string, clientID int) error {
	return runAPIContractDetails(ctx, addr, clientID, 10*time.Second, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
	})
}

func runAPIContractDetailsAAPLOptions(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 45*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		parameters, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
			UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
		})
		if err != nil {
			return fmt.Errorf("resolve AAPL option expirations: %w", err)
		}

		expiry := ""
		for _, parameter := range parameters {
			if parameter.Exchange != "SMART" || parameter.TradingClass != "AAPL" {
				continue
			}
			for _, candidate := range parameter.Expirations {
				if candidate != "" && (expiry == "" || candidate < expiry) {
					expiry = candidate
				}
			}
		}
		if expiry == "" {
			return fmt.Errorf("AAPL option parameters returned no current SMART expiry")
		}

		details, err := client.Contracts().Details(ctx, ibkr.Contract{
			Symbol: "AAPL", SecType: ibkr.SecTypeOption, Expiry: expiry, Exchange: "SMART", Currency: "USD",
		})
		if err != nil {
			return err
		}
		if len(details) == 0 {
			return fmt.Errorf("AAPL option details returned no matches for expiry %s", expiry)
		}
		recordAPIEvent("contract_details", "nearest_expiry", func(event *apiDriverEvent) {
			event.Count = len(details)
			event.Symbol = details[0].Symbol
			event.SecType = string(details[0].SecType)
			event.Values = map[string]string{
				"expiry":       expiry,
				"first_con_id": strconv.FormatInt(int64(details[0].ConID), 10),
			}
		})
		return nil
	})
}

func runAPIContractDetailsAppleBonds(ctx context.Context, addr string, clientID int) error {
	return runAPIContractDetails(ctx, addr, clientID, 30*time.Second, ibkr.Contract{IssuerID: "e1432232"})
}

func runAPIContractDetailsEURUSD(ctx context.Context, addr string, clientID int) error {
	return runAPIContractDetails(ctx, addr, clientID, 10*time.Second, ibkr.Contract{
		Symbol: "EUR", SecType: ibkr.SecTypeForex, Exchange: "IDEALPRO", Currency: "USD",
	})
}

func runAPIContractDetailsESFutures(ctx context.Context, addr string, clientID int) error {
	return runAPIContractDetails(ctx, addr, clientID, 10*time.Second, ibkr.Contract{
		Symbol: "ES", SecType: ibkr.SecTypeFuture, Exchange: "CME", Currency: "USD",
	})
}

func runAPIContractDetailsNotFound(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		_, err := client.Contracts().Details(ctx, ibkr.Contract{
			Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
		})
		if err == nil {
			return errors.New("expected contract-details code 200 not-found error, got nil")
		}
		apiErr, ok := errors.AsType[*ibkr.APIError](err)
		if !ok || apiErr.Code != 200 || apiErr.OpKind != ibkr.OpContractDetails || !strings.Contains(apiErr.Message, "No security definition has been found") {
			return fmt.Errorf("expected contract-details code 200 not-found error, got %v", err)
		}
		recordAPIEvent("contract_not_found", "", func(event *apiDriverEvent) { event.Error = err.Error() })
		return nil
	})
}

func runAPIContractDetailsConcurrent(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		type result struct {
			label   string
			details []ibkr.ContractDetails
			err     error
		}
		results := make(chan result, 2)
		queries := []struct {
			label    string
			contract ibkr.Contract
		}{
			{label: "AAPL", contract: apiAAPL},
			{label: "EUR.USD", contract: apiEURUSD},
		}
		for i, query := range queries {
			if i != 0 {
				time.Sleep(25 * time.Millisecond)
			}
			go func() {
				details, err := client.Contracts().Details(ctx, query.contract)
				results <- result{label: query.label, details: details, err: err}
			}()
		}
		for range queries {
			select {
			case result := <-results:
				if result.err != nil {
					return fmt.Errorf("%s concurrent contract details: %w", result.label, result.err)
				}
				if len(result.details) == 0 {
					return fmt.Errorf("%s concurrent contract details returned no contracts", result.label)
				}
				recordAPIEvent("contract_details_concurrent", result.label, func(event *apiDriverEvent) {
					event.Count = len(result.details)
				})
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		}
		return nil
	})
}

func runAPIQualifyContractAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		details, err := client.Contracts().Qualify(ctx, ibkr.Contract{
			Symbol: "AAPL", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD",
		})
		if err != nil {
			return err
		}
		recordAPIEvent("contract_qualified", "", func(event *apiDriverEvent) {
			event.Symbol = details.Symbol
			event.SecType = string(details.SecType)
			event.Values = map[string]string{"con_id": strconv.FormatInt(int64(details.ConID), 10)}
		})
		return nil
	})
}

func runAPIQualifyContractAmbiguous(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		_, err := client.Contracts().Qualify(ctx, ibkr.Contract{
			Symbol: "MSFT", SecType: ibkr.SecTypeStock, Currency: "USD",
		})
		if err == nil {
			return errors.New("expected ErrAmbiguousContract, got nil")
		}
		if !errors.Is(err, ibkr.ErrAmbiguousContract) {
			return fmt.Errorf("expected ErrAmbiguousContract, got %v", err)
		}
		recordAPIEvent("contract_ambiguous", "", func(event *apiDriverEvent) { event.Error = err.Error() })
		return nil
	})
}

func runAPIMatchingSymbolsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		symbols, err := client.Contracts().Search(ctx, "AAPL")
		if err != nil {
			return err
		}
		if len(symbols) == 0 {
			return fmt.Errorf("AAPL contract search returned no matches")
		}
		recordAPIEvent("matching_symbols", "AAPL", func(event *apiDriverEvent) { event.Count = len(symbols) })
		return nil
	})
}

func runAPIMatchingSymbolsPartial(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		symbols, err := client.Contracts().Search(ctx, "AA")
		if err != nil {
			return err
		}
		if len(symbols) == 0 {
			return fmt.Errorf("AA contract search returned no matches")
		}
		recordAPIEvent("matching_symbols", "AA", func(event *apiDriverEvent) { event.Count = len(symbols) })
		return nil
	})
}

func runAPISecDefOptParamsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		parameters, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
			UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
		})
		if err != nil {
			return err
		}
		if len(parameters) == 0 {
			return fmt.Errorf("AAPL option parameters returned no matches")
		}
		recordAPIEvent("sec_def_opt_params", "", func(event *apiDriverEvent) { event.Count = len(parameters) })
		return nil
	})
}

func runAPISmartComponents(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("set delayed market data: %w", err)
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiAAPL}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL quote parameters: %w", err)
		}

		bboExchange := ""
		for bboExchange == "" {
			select {
			case event, ok := <-sub.Events():
				if !ok {
					if err := sub.Err(); err != nil {
						return fmt.Errorf("AAPL quote closed before BBO mapping: %w", err)
					}
					return errors.New("AAPL quote closed before BBO mapping")
				}
				if event.Kind != ibkr.StreamData {
					continue
				}
				update := event.Value
				if update.Kind == ibkr.QuoteUpdateParameters && update.Parameters != nil {
					bboExchange = update.Parameters.BBOExchange
				}
			case <-sub.Done():
				if err := sub.Err(); err != nil {
					return fmt.Errorf("AAPL quote closed before BBO mapping: %w", err)
				}
				return errors.New("AAPL quote closed before BBO mapping")
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		components, err := client.Contracts().SmartComponents(ctx, bboExchange)
		if err != nil {
			sub.Close()
			return err
		}
		sub.Close()
		if err := sub.Wait(); err != nil {
			return fmt.Errorf("wait for AAPL quote close: %w", err)
		}
		if len(components) == 0 {
			return fmt.Errorf("SMART components returned no matches")
		}
		recordAPIEvent("smart_components", "", func(event *apiDriverEvent) {
			event.Count = len(components)
			event.Values = map[string]string{"bbo_exchange": bboExchange}
		})
		return nil
	})
}

func runAPIAccountSummarySnapshot(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		values, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
			Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"},
		})
		if err != nil {
			return err
		}
		if len(values) == 0 {
			return errors.New("account summary returned no values")
		}
		if err := fenceAPIWrites(ctx, client, "account summary cancellation"); err != nil {
			return err
		}
		recordAPIEvent("account_summary", "snapshot", func(event *apiDriverEvent) { event.Count = len(values) })
		return nil
	})
}

func fenceAPIWrites(ctx context.Context, client *ibkr.Client, label string) error {
	// The response proves earlier writes reached the connection before Client.Close.
	if _, err := client.CurrentTime(ctx); err != nil {
		return fmt.Errorf("%s protocol fence: %w", label, err)
	}
	return nil
}

func drainAccountSummaryEvents(sub *ibkr.Subscription[ibkr.AccountValue]) int {
	count := 0
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				return count
			}
			count++
		default:
			return count
		}
	}
}

func runAPIAccountSummaryStream(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		sub, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
			Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return err
		}
		if err := sub.AwaitSnapshot(ctx); err != nil {
			sub.Close()
			return err
		}
		count := drainAccountSummaryEvents(sub)
		sub.Close()
		if err := sub.Wait(); err != nil {
			return err
		}
		if count == 0 {
			return errors.New("account summary subscription returned no values")
		}
		if err := fenceAPIWrites(ctx, client, "account summary cancellation"); err != nil {
			return err
		}
		recordAPIEvent("account_summary", "subscription", func(event *apiDriverEvent) { event.Count = count })
		return nil
	})
}

func runAPIAccountSummaryTwoSubscriptions(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		first, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
			Tags: []string{"NetLiquidation"},
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return err
		}
		second, err := client.Accounts().SubscribeSummary(ctx, ibkr.AccountSummaryRequest{
			Tags: []string{"TotalCashValue"},
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			first.Close()
			return err
		}
		if err := first.AwaitSnapshot(ctx); err != nil {
			first.Close()
			second.Close()
			return fmt.Errorf("first account summary: %w", err)
		}
		if err := second.AwaitSnapshot(ctx); err != nil {
			first.Close()
			second.Close()
			return fmt.Errorf("second account summary: %w", err)
		}
		firstCount := drainAccountSummaryEvents(first)
		secondCount := drainAccountSummaryEvents(second)
		first.Close()
		second.Close()
		if err := errors.Join(first.Wait(), second.Wait()); err != nil {
			return fmt.Errorf("close account summaries: %w", err)
		}
		if firstCount == 0 || secondCount == 0 {
			return fmt.Errorf("account summary subscriptions returned %d and %d values", firstCount, secondCount)
		}
		if err := fenceAPIWrites(ctx, client, "account summary cancellations"); err != nil {
			return err
		}
		recordAPIEvent("account_summary", "two_subscriptions", func(event *apiDriverEvent) {
			event.Count = firstCount + secondCount
			event.Values = map[string]string{
				"first_count":  strconv.Itoa(firstCount),
				"second_count": strconv.Itoa(secondCount),
			}
		})
		return nil
	})
}

func runAPIPositionsSnapshot(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		positions, err := client.Accounts().Positions(ctx)
		if err != nil {
			return err
		}
		if err := fenceAPIWrites(ctx, client, "positions cancellation"); err != nil {
			return err
		}
		recordAPIEvent("positions", "snapshot", func(event *apiDriverEvent) { event.Count = len(positions) })
		return nil
	})
}

func runAPIPositionsSubscription(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		sub, err := client.Accounts().SubscribePositions(ctx, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe positions: %w", err)
		}
		count := 0
		for {
			select {
			case event, ok := <-sub.Events():
				if !ok {
					return errors.New("positions subscription closed before SnapshotComplete")
				}
				if event.Err != nil {
					return fmt.Errorf("positions subscription event: %w", event.Err)
				}
				switch event.Kind {
				case ibkr.StreamData:
					count++
				case ibkr.StreamSnapshotComplete:
					if count == 0 {
						return errors.New("positions subscription snapshot is empty")
					}
					if err := closeAndFenceSubscription(ctx, client, sub, "positions subscription cancellation"); err != nil {
						return err
					}
					recordAPIEvent("positions_subscription", "snapshot_complete", func(event *apiDriverEvent) {
						event.Count = count
					})
					return nil
				}
			case <-ctx.Done():
				sub.Close()
				return ctx.Err()
			}
		}
	})
}

func runAPISetMarketDataType(ctx context.Context, addr string, clientID int, dataType ibkr.MarketDataType) error {
	return apiScenario(ctx, addr, clientID, 10*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, dataType); err != nil {
			return err
		}
		if err := fenceAPIWrites(ctx, client, "market data type"); err != nil {
			return err
		}
		recordAPIEvent("market_data_type", dataType.String(), nil)
		return nil
	})
}

func runAPISetMarketDataLive(ctx context.Context, addr string, clientID int) error {
	return runAPISetMarketDataType(ctx, addr, clientID, ibkr.MarketDataLive)
}

func runAPISetMarketDataFrozen(ctx context.Context, addr string, clientID int) error {
	return runAPISetMarketDataType(ctx, addr, clientID, ibkr.MarketDataFrozen)
}

func runAPISetMarketDataDelayed(ctx context.Context, addr string, clientID int) error {
	return runAPISetMarketDataType(ctx, addr, clientID, ibkr.MarketDataDelayed)
}

func runAPISetMarketDataDelayedFrozen(ctx context.Context, addr string, clientID int) error {
	return runAPISetMarketDataType(ctx, addr, clientID, ibkr.MarketDataDelayedFrozen)
}

func runAPISetTypeSwitchWhileStreaming(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("set delayed market data: %w", err)
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiAAPL}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL quotes: %w", err)
		}
		var sawDelayedType, sawPriceOrSize bool
		count, err := awaitSubscriptionEvidence(ctx, sub, 12*time.Second, func(update ibkr.QuoteUpdate) bool {
			sawDelayedType = sawDelayedType || update.Snapshot.MarketDataType == ibkr.MarketDataDelayed
			sawPriceOrSize = sawPriceOrSize || update.Changed&apiQuotePriceOrSizeFields != 0
			return sawDelayedType && sawPriceOrSize
		})
		if err != nil {
			sub.Close()
			return fmt.Errorf("observe delayed AAPL quote stream: %w", err)
		}
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
			sub.Close()
			return fmt.Errorf("switch AAPL quote stream to live: %w", err)
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "market-data type switch cancellation"); err != nil {
			return err
		}
		recordAPIEvent("market_data_type_switch", "delayed_to_live", func(event *apiDriverEvent) {
			event.Count = count
			event.Values = map[string]string{
				"delayed_type_observed":  strconv.FormatBool(sawDelayedType),
				"price_or_size_observed": strconv.FormatBool(sawPriceOrSize),
			}
		})
		return nil
	})
}

func runAPIQuoteSnapshotAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("set delayed market data: %w", err)
		}
		quote, err := client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: apiAAPL})
		if err != nil {
			return fmt.Errorf("AAPL quote snapshot: %w", err)
		}
		if quote.Available == 0 || quote.Available&apiQuotePriceOrSizeFields == 0 {
			return fmt.Errorf("AAPL quote snapshot availability %d contains no price or size", quote.Available)
		}
		recordAPIEvent("quote_snapshot", "aapl", func(event *apiDriverEvent) {
			event.Symbol = apiAAPL.Symbol
			event.SecType = string(apiAAPL.SecType)
			event.Values = map[string]string{
				"available":        strconv.FormatUint(uint64(quote.Available), 10),
				"market_data_type": quote.MarketDataType.String(),
			}
		})
		return nil
	})
}

func runAPIQuoteStreamAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("set delayed market data: %w", err)
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiAAPL}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL quotes: %w", err)
		}
		count, err := awaitSubscriptionEvidence(ctx, sub, 12*time.Second, func(update ibkr.QuoteUpdate) bool {
			return update.Changed&apiQuotePriceOrSizeFields != 0
		})
		if err != nil {
			return fmt.Errorf("observe AAPL quote stream: %w", err)
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "AAPL quote stream cancellation"); err != nil {
			return err
		}
		recordAPIEvent("quote_stream", "aapl", func(event *apiDriverEvent) {
			event.Symbol = apiAAPL.Symbol
			event.SecType = string(apiAAPL.SecType)
			event.Count = count
		})
		return nil
	})
}

func runAPIQuoteStreamMultiAsset(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 25*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("set delayed market data: %w", err)
		}
		aapl, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiAAPL}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL quotes: %w", err)
		}
		eurusd, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiEURUSD}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			aapl.Close()
			return fmt.Errorf("subscribe EUR.USD quotes: %w", err)
		}
		aaplCount, err := awaitSubscriptionEvidence(ctx, aapl, 15*time.Second, func(update ibkr.QuoteUpdate) bool {
			return update.Changed&apiQuotePriceOrSizeFields != 0
		})
		if err != nil {
			aapl.Close()
			eurusd.Close()
			return fmt.Errorf("observe AAPL quote stream: %w", err)
		}
		eurusdCount, err := awaitSubscriptionEvidence(ctx, eurusd, 15*time.Second, func(update ibkr.QuoteUpdate) bool {
			return update.Changed&apiQuotePriceOrSizeFields != 0
		})
		if err != nil {
			aapl.Close()
			eurusd.Close()
			return fmt.Errorf("observe EUR.USD quote stream: %w", err)
		}
		aapl.Close()
		eurusd.Close()
		if err := aapl.Wait(); err != nil {
			return fmt.Errorf("AAPL quote stream cancellation wait: %w", err)
		}
		if err := eurusd.Wait(); err != nil {
			return fmt.Errorf("EUR.USD quote stream cancellation wait: %w", err)
		}
		if err := fenceAPIWrites(ctx, client, "multi-asset quote cancellations"); err != nil {
			return err
		}
		recordAPIEvent("quote_stream_multi_asset", "aapl_eurusd", func(event *apiDriverEvent) {
			event.Count = aaplCount + eurusdCount
			event.Values = map[string]string{
				"aapl_count":   strconv.Itoa(aaplCount),
				"eurusd_count": strconv.Itoa(eurusdCount),
			}
		})
		return nil
	})
}

func runAPIQuoteStreamGenericTicksAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("set delayed market data: %w", err)
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
			Contract: apiAAPL, GenericTicks: []ibkr.GenericTick{"233", "236"},
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL generic ticks: %w", err)
		}
		var sawParameters, sawGenericValue bool
		count, err := awaitSubscriptionEvidence(ctx, sub, 12*time.Second, func(update ibkr.QuoteUpdate) bool {
			sawParameters = sawParameters || update.Kind == ibkr.QuoteUpdateParameters
			sawGenericValue = sawGenericValue ||
				(update.GenericTick != nil && update.GenericTick.TickType == 46) ||
				(update.StringTick != nil && update.StringTick.TickType == 48)
			return sawParameters && sawGenericValue
		})
		if err != nil {
			return fmt.Errorf("observe AAPL generic ticks: %w", err)
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "AAPL generic-tick cancellation"); err != nil {
			return err
		}
		recordAPIEvent("quote_stream", "aapl_generic_233_236", func(event *apiDriverEvent) {
			event.Symbol = apiAAPL.Symbol
			event.SecType = string(apiAAPL.SecType)
			event.Count = count
		})
		return nil
	})
}

func runAPIOddLotQuotesAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 35*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
			return fmt.Errorf("set live market data: %w", err)
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
			Contract: apiAAPL, GenericTicks: []ibkr.GenericTick{ibkr.GenericTickOddLotBidAsk},
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL odd-lot quotes: %w", err)
		}
		oddLotFields := ibkr.QuoteFieldOddLotBid | ibkr.QuoteFieldOddLotAsk |
			ibkr.QuoteFieldOddLotBidSize | ibkr.QuoteFieldOddLotAskSize |
			ibkr.QuoteFieldOddLotBidExchange | ibkr.QuoteFieldOddLotAskExchange
		count, err := awaitSubscriptionEvidence(ctx, sub, 20*time.Second, func(update ibkr.QuoteUpdate) bool {
			return update.Changed&oddLotFields != 0
		})
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if !ok || !isExactOddLotEntitlementRefusal(apiErr) {
				return fmt.Errorf("observe AAPL odd-lot quotes: %w", err)
			}
			if err := fenceAPIWrites(ctx, client, "AAPL odd-lot quote refusal"); err != nil {
				return err
			}
			recordSubscriptionRefusal("quote_odd_lot", "aapl_generic_787", apiErr)
			return nil
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "AAPL odd-lot quote cancellation"); err != nil {
			return err
		}
		recordAPIEvent("quote_odd_lot", "aapl_generic_787", func(event *apiDriverEvent) {
			event.Symbol = apiAAPL.Symbol
			event.SecType = string(apiAAPL.SecType)
			event.Count = count
		})
		return nil
	})
}

func isExactOddLotEntitlementRefusal(err *ibkr.APIError) bool {
	if err.OpKind != ibkr.OpQuotes {
		return false
	}
	switch err.Code {
	case ibkr.ErrCodeAdditionalSubscriptionRequired:
		return strings.HasPrefix(err.Message, "Requested market data requires additional subscription for API")
	case 2186:
		return strings.HasPrefix(err.Message, "Warning: Requested real-time market data requires additional subscription for API. You elected to receive delayed market data instead.")
	default:
		return false
	}
}

func runAPITickEFPProbe(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 35*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataLive); err != nil {
			return fmt.Errorf("set live market data: %w", err)
		}
		contracts := []struct {
			label    string
			contract ibkr.Contract
		}{
			{
				label: "dte_eurex",
				contract: ibkr.Contract{
					Symbol: "USD", SecType: ibkr.SecTypeCombo, Exchange: "SMART", Currency: "EUR",
					ComboLegs: []ibkr.ComboLeg{
						{ConID: 667336572, Ratio: 1, Action: ibkr.ActionBuy, Exchange: "EUREX"},
						{ConID: 2254332, Ratio: 100, Action: ibkr.ActionSell, Exchange: "SMART"},
					},
				},
			},
			{
				label: "tencent_hkfe",
				contract: ibkr.Contract{
					Symbol: "USD", SecType: ibkr.SecTypeCombo, Exchange: "SMART", Currency: "HKD",
					ComboLegs: []ibkr.ComboLeg{
						{ConID: 842557048, Ratio: 1, Action: ibkr.ActionBuy, Exchange: "HKFE"},
						{ConID: 152791428, Ratio: 100, Action: ibkr.ActionSell, Exchange: "SEHK"},
					},
				},
			},
		}

		type probe struct {
			label       string
			sub         *ibkr.Subscription[ibkr.QuoteUpdate]
			updates     int
			targetTicks int
		}
		probes := make([]probe, 0, len(contracts))
		for _, candidate := range contracts {
			sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: candidate.contract}, ibkr.WithResumePolicy(ibkr.ResumeNever))
			if err != nil {
				for i := range probes {
					probes[i].sub.Close()
					_ = probes[i].sub.Wait()
				}
				return fmt.Errorf("subscribe %s EFP quotes: %w", candidate.label, err)
			}
			probes = append(probes, probe{label: candidate.label, sub: sub})
		}

		timer := time.NewTimer(20 * time.Second)
		defer timer.Stop()
		firstEvents := probes[0].sub.Events()
		secondEvents := probes[1].sub.Events()
		var sawEFP, sawDeltaNeutral bool
		observe := func(index int, event ibkr.StreamEvent[ibkr.QuoteUpdate]) bool {
			if event.Kind != ibkr.StreamData {
				return false
			}
			probes[index].updates++
			if event.Value.Kind != ibkr.QuoteUpdateEFP && event.Value.Kind != ibkr.QuoteUpdateDeltaNeutralValidation {
				return false
			}
			probes[index].targetTicks++
			sawEFP = sawEFP || event.Value.Kind == ibkr.QuoteUpdateEFP
			sawDeltaNeutral = sawDeltaNeutral || event.Value.Kind == ibkr.QuoteUpdateDeltaNeutralValidation
			recordAPIEvent("tick_efp_callback", probes[index].label, func(driverEvent *apiDriverEvent) {
				driverEvent.Values = quoteUpdateValues(event.Value)
			})
			return sawEFP && sawDeltaNeutral
		}
		finished := false
		for !finished && (firstEvents != nil || secondEvents != nil) {
			select {
			case event, ok := <-firstEvents:
				if ok {
					finished = observe(0, event)
				} else {
					firstEvents = nil
				}
			case event, ok := <-secondEvents:
				if ok {
					finished = observe(1, event)
				} else {
					secondEvents = nil
				}
			case <-timer.C:
				finished = true
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		for i := range probes {
			probes[i].sub.Close()
			if err := probes[i].sub.Wait(); err != nil {
				if apiErr, ok := errors.AsType[*ibkr.APIError](err); ok {
					recordSubscriptionRefusal("tick_efp_probe", probes[i].label, apiErr)
					continue
				}
				return fmt.Errorf("wait for %s EFP cancellation: %w", probes[i].label, err)
			}
		}
		if err := fenceAPIWrites(ctx, client, "EFP quote cancellations"); err != nil {
			return err
		}
		targetTicks := probes[0].targetTicks + probes[1].targetTicks
		if !sawEFP || !sawDeltaNeutral {
			return fmt.Errorf(
				"EFP probes did not receive both TickEFP and delta-neutral validation callbacks: tick_efp=%t delta_neutral=%t %s=%d %s=%d",
				sawEFP,
				sawDeltaNeutral,
				probes[0].label,
				probes[0].updates,
				probes[1].label,
				probes[1].updates,
			)
		}
		recordAPIEvent("tick_efp_probe", "typed_callback", func(event *apiDriverEvent) {
			event.Count = targetTicks
			event.Values = map[string]string{
				probes[0].label + "_updates":      strconv.Itoa(probes[0].updates),
				probes[0].label + "_target_ticks": strconv.Itoa(probes[0].targetTicks),
				probes[1].label + "_updates":      strconv.Itoa(probes[1].updates),
				probes[1].label + "_target_ticks": strconv.Itoa(probes[1].targetTicks),
			}
		})
		return nil
	})
}
func runAPIRealTimeBarsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 25*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("set delayed market data: %w", err)
		}
		sub, err := client.MarketData().SubscribeRealTimeBars(ctx, ibkr.RealTimeBarsRequest{
			Contract: apiAAPL, WhatToShow: ibkr.ShowTrades, UseRTH: true,
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL real-time bars: %w", err)
		}
		count, err := awaitSubscriptionEvidence(ctx, sub, 15*time.Second, func(ibkr.Bar) bool { return true })
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if !ok || apiErr.OpKind != ibkr.OpRealTimeBars || apiErr.Code != ibkr.ErrCodeInvalidRealTimeQuery ||
				!strings.HasPrefix(apiErr.Message, "Invalid Real-time Query:No market data permissions for ISLAND STK.") {
				return fmt.Errorf("observe AAPL real-time bars: %w", err)
			}
			if err := fenceAPIWrites(ctx, client, "AAPL real-time-bars refusal"); err != nil {
				return err
			}
			recordSubscriptionRefusal("realtime_bars", "aapl", apiErr)
			return nil
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "AAPL real-time-bars cancellation"); err != nil {
			return err
		}
		recordProbeResult("realtime_bars", "aapl", count, nil)
		return nil
	})
}

func runAPITickByTickLastAAPL(ctx context.Context, addr string, clientID int) error {
	return runAPITickByTickAAPL(ctx, addr, clientID, ibkr.TickByTickLast)
}

func runAPITickByTickBidAskAAPL(ctx context.Context, addr string, clientID int) error {
	return runAPITickByTickAAPL(ctx, addr, clientID, ibkr.TickByTickBidAsk)
}

func runAPITickByTickMidPointAAPL(ctx context.Context, addr string, clientID int) error {
	return runAPITickByTickAAPL(ctx, addr, clientID, ibkr.TickByTickMidPoint)
}

func runAPITickByTickAAPL(ctx context.Context, addr string, clientID int, tickType ibkr.TickByTickType) error {
	return apiScenario(ctx, addr, clientID, 25*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("set delayed market data: %w", err)
		}
		sub, err := client.MarketData().SubscribeTickByTick(ctx, ibkr.TickByTickRequest{
			Contract: apiAAPL, TickType: tickType,
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL %s ticks: %w", tickType, err)
		}
		wantWireType := map[ibkr.TickByTickType]int{
			ibkr.TickByTickLast: 1, ibkr.TickByTickBidAsk: 3, ibkr.TickByTickMidPoint: 4,
		}[tickType]
		count, err := awaitSubscriptionEvidence(ctx, sub, 15*time.Second, func(tick ibkr.TickByTickData) bool {
			return tick.TickType == wantWireType || tickType == ibkr.TickByTickLast && tick.TickType == 2
		})
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if !ok || !isExactTickByTickEntitlementRefusal(apiErr) {
				return fmt.Errorf("observe AAPL %s ticks: %w", tickType, err)
			}
			if err := fenceAPIWrites(ctx, client, "AAPL tick-by-tick refusal"); err != nil {
				return err
			}
			recordSubscriptionRefusal("tick_by_tick", string(tickType), apiErr)
			return nil
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "AAPL tick-by-tick cancellation"); err != nil {
			return err
		}
		recordProbeResult("tick_by_tick", string(tickType), count, nil)
		return nil
	})
}

func isExactTickByTickEntitlementRefusal(err *ibkr.APIError) bool {
	if err.OpKind != ibkr.OpTickByTick {
		return false
	}
	switch err.Code {
	case ibkr.ErrCodeAdditionalSubscriptionRequired:
		return strings.HasPrefix(err.Message, "Requested market data requires additional subscription for API")
	case ibkr.ErrCodeTickByTickDataNotAllowed:
		return strings.HasPrefix(err.Message, "Failed to request tick-by-tick data.No market data permissions for ISLAND STK.")
	default:
		return false
	}
}

func runAPIMarketDepthAAPL(ctx context.Context, addr string, clientID int) error {
	return runAPIMarketDepth(ctx, addr, clientID, false)
}

func runAPIMarketDepthSmartAAPL(ctx context.Context, addr string, clientID int) error {
	return runAPIMarketDepth(ctx, addr, clientID, true)
}

func runAPIMarketDepth(ctx context.Context, addr string, clientID int, smart bool) error {
	return apiScenario(ctx, addr, clientID, 25*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		sub, err := client.MarketData().SubscribeDepth(ctx, ibkr.MarketDepthRequest{
			Contract: apiAAPL, NumRows: 5, IsSmartDepth: smart,
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe AAPL market depth smart=%t: %w", smart, err)
		}
		label := "regular"
		if smart {
			label = "smart"
		}
		count, unavailable, err := awaitMarketDepthEvidence(ctx, client, sub, 15*time.Second)
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if !ok || smart || !isExactMarketDepthRefusal(apiErr) {
				return fmt.Errorf("observe AAPL %s market depth: %w", label, err)
			}
			if err := fenceAPIWrites(ctx, client, "AAPL "+label+" market-depth refusal"); err != nil {
				return err
			}
			recordSubscriptionRefusal("market_depth", label, apiErr)
			return nil
		}
		if unavailable != nil {
			if err := closeAndFenceSubscription(ctx, client, sub, "AAPL SMART market-depth unavailable cancellation"); err != nil {
				return err
			}
			recordAPIEvent("market_depth_unavailable", label, func(event *apiDriverEvent) {
				event.Symbol = apiAAPL.Symbol
				event.SecType = string(apiAAPL.SecType)
				event.Values = map[string]string{
					"code":    strconv.Itoa(unavailable.Code),
					"message": unavailable.Message,
				}
			})
			return nil
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "AAPL "+label+" market-depth cancellation"); err != nil {
			return err
		}
		recordProbeResult("market_depth", label, count, nil)
		return nil
	})
}

func isExactMarketDepthRefusal(err *ibkr.APIError) bool {
	return err.Code == ibkr.ErrCodeDeepMarketDataNotSupported &&
		err.OpKind == ibkr.OpMarketDepth &&
		err.Message == "Deep market data is not supported for this combination of security type/exchange"
}

func awaitMarketDepthEvidence(ctx context.Context, client *ibkr.Client, sub *ibkr.Subscription[ibkr.DepthRow], timeout time.Duration) (int, *ibkr.Event, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	sessionEvents := client.SessionEvents()
	count := 0
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				if err := sub.Wait(); err != nil {
					return count, nil, err
				}
				return count, nil, errors.New("market-depth subscription closed before required evidence")
			}
			if event.Kind == ibkr.StreamNotice && event.Notice != nil &&
				event.Notice.Code == ibkr.ErrCodeSmartDepthExchanges &&
				isNoMarketDepthAvailable(event.Notice.Message) {
				return count, &ibkr.Event{Code: event.Notice.Code, Message: event.Notice.Message}, nil
			}
			if event.Kind != ibkr.StreamData {
				continue
			}
			count++
			row := event.Value
			if row.Position >= 0 {
				return count, nil, nil
			}
		case event, ok := <-sessionEvents:
			if !ok {
				sessionEvents = nil
				continue
			}
			if event.Code == ibkr.ErrCodeSmartDepthExchanges && isNoMarketDepthAvailable(event.Message) {
				return count, &event, nil
			}
		case <-sub.Done():
			if err := sub.Wait(); err != nil {
				return count, nil, err
			}
			return count, nil, errors.New("market-depth subscription closed before required evidence")
		case <-timer.C:
			return count, nil, fmt.Errorf("required market-depth evidence not observed within %s", timeout)
		case <-ctx.Done():
			return count, nil, ctx.Err()
		}
	}
}

func isNoMarketDepthAvailable(message string) bool {
	return strings.Contains(message, "Need additional market data permissions - Depth:") &&
		!strings.Contains(message, "Exchanges - Depth:")
}

func awaitSubscriptionEvidence[T any](ctx context.Context, sub *ibkr.Subscription[T], timeout time.Duration, accept func(T) bool) (int, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	count := 0
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				if err := sub.Wait(); err != nil {
					return count, err
				}
				return count, errors.New("subscription closed before required evidence")
			}
			if event.Kind != ibkr.StreamData {
				continue
			}
			count++
			value := event.Value
			if accept(value) {
				return count, nil
			}
		case <-sub.Done():
			if err := sub.Wait(); err != nil {
				return count, err
			}
			return count, errors.New("subscription closed before required evidence")
		case <-timer.C:
			return count, fmt.Errorf("required subscription evidence not observed within %s", timeout)
		case <-ctx.Done():
			return count, ctx.Err()
		}
	}
}

func closeAndFenceSubscription[T any](ctx context.Context, client *ibkr.Client, sub *ibkr.Subscription[T], label string) error {
	sub.Close()
	if err := sub.Wait(); err != nil {
		return fmt.Errorf("%s wait: %w", label, err)
	}
	return fenceAPIWrites(ctx, client, label)
}

func recordSubscriptionRefusal(kind, label string, err *ibkr.APIError) {
	recordAPIEvent(kind+"_refused", label, func(event *apiDriverEvent) {
		event.Error = err.Error()
		event.Values = map[string]string{
			"code":    strconv.Itoa(err.Code),
			"op_kind": string(err.OpKind),
		}
	})
}

func runAPIAccountUpdates(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, account string) error {
		updates, err := client.Accounts().Updates(ctx, account)
		if err != nil {
			return fmt.Errorf("account updates snapshot: %w", err)
		}
		if len(updates) == 0 {
			return errors.New("account updates snapshot is empty")
		}
		var accountValues, portfolioValues, updateTimes int
		for i, update := range updates {
			payloads := 0
			if update.AccountValue != nil {
				payloads++
				accountValues++
			}
			if update.Portfolio != nil {
				payloads++
				portfolioValues++
			}
			if update.UpdateTime != nil {
				payloads++
				updateTimes++
			}
			if payloads != 1 {
				return fmt.Errorf("account update %d must contain exactly one payload", i)
			}
		}
		if err := fenceAPIWrites(ctx, client, "account updates snapshot cancellation"); err != nil {
			return err
		}
		recordAPIEvent("account_updates", "snapshot", func(event *apiDriverEvent) {
			event.Count = len(updates)
			event.Values = map[string]string{
				"account_values":   strconv.Itoa(accountValues),
				"portfolio_values": strconv.Itoa(portfolioValues),
				"update_times":     strconv.Itoa(updateTimes),
			}
		})
		return nil
	})
}

func runAPIAccountUpdatesMulti(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, account string) error {
		counts := make(map[string]int, 2)
		for _, includeLedger := range []bool{false, true} {
			values, err := client.Accounts().UpdatesMulti(ctx, ibkr.AccountUpdatesMultiRequest{
				Account: account, LedgerAndNLV: includeLedger,
			})
			if err != nil {
				return err
			}
			if len(values) == 0 {
				return fmt.Errorf("multi-account updates ledger_and_nlv=%t returned no values", includeLedger)
			}
			for _, value := range values {
				if value.Account != account || value.Key == "" {
					return fmt.Errorf("invalid multi-account update: account=%q key=%q", value.Account, value.Key)
				}
			}
			counts[strconv.FormatBool(includeLedger)] = len(values)
		}
		if err := fenceAPIWrites(ctx, client, "multi-account updates cancellation"); err != nil {
			return err
		}
		recordAPIEvent("account_updates_multi", "snapshot", func(event *apiDriverEvent) {
			event.Count = counts["false"] + counts["true"]
			event.Values = map[string]string{
				"ledger_false": strconv.Itoa(counts["false"]),
				"ledger_true":  strconv.Itoa(counts["true"]),
			}
		})
		return nil
	})
}

func runAPIPositionsMulti(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, account string) error {
		positions, err := client.Accounts().PositionsMulti(ctx, ibkr.PositionsMultiRequest{Account: account})
		if err != nil {
			return err
		}
		for _, position := range positions {
			if position.Account != account || position.Contract.Symbol == "" {
				return fmt.Errorf("invalid multi-account position: account=%q symbol=%q", position.Account, position.Contract.Symbol)
			}
		}
		if err := fenceAPIWrites(ctx, client, "multi-account positions cancellation"); err != nil {
			return err
		}
		recordAPIEvent("positions_multi", "snapshot", func(event *apiDriverEvent) { event.Count = len(positions) })
		return nil
	})
}

func runAPIHistoricalBars(ctx context.Context, addr string, clientID int, label string, req ibkr.HistoricalBarsRequest, allowUnavailable bool) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		bars, err := client.History().Bars(ctx, req)
		if err != nil {
			if !allowUnavailable || !isHistoricalDataUnavailable(err, ibkr.OpHistoricalBars) {
				return err
			}
			recordAPIEvent("historical_data_unavailable", label, func(event *apiDriverEvent) { event.Error = err.Error() })
			return nil
		}
		if len(bars) == 0 {
			return fmt.Errorf("%s returned no historical bars", label)
		}
		recordAPIEvent("historical_bars", label, func(event *apiDriverEvent) { event.Count = len(bars) })
		return nil
	})
}

func runAPIHistoricalBars1Day1Hour(ctx context.Context, addr string, clientID int) error {
	return runAPIHistoricalBars(ctx, addr, clientID, "1d_1h", ibkr.HistoricalBarsRequest{
		Contract: apiAAPL, Duration: ibkr.Days(1), BarSize: ibkr.Bar1Hour, WhatToShow: ibkr.ShowTrades, UseRTH: true,
	}, false)
}

func runAPIHistoricalBars30Days1Day(ctx context.Context, addr string, clientID int) error {
	return runAPIHistoricalBars(ctx, addr, clientID, "30d_1day", ibkr.HistoricalBarsRequest{
		Contract: apiAAPL, Duration: ibkr.Days(30), BarSize: ibkr.Bar1Day, WhatToShow: ibkr.ShowTrades, UseRTH: true,
	}, false)
}

func runAPIHistoricalBarsBidAsk(ctx context.Context, addr string, clientID int) error {
	return runAPIHistoricalBars(ctx, addr, clientID, "bidask", ibkr.HistoricalBarsRequest{
		Contract: apiAAPL, Duration: ibkr.Days(1), BarSize: ibkr.Bar1Hour, WhatToShow: ibkr.ShowBidAsk, UseRTH: true,
	}, true)
}

func runAPIHistoricalBarsError(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		_, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
			Contract: ibkr.Contract{Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"},
			Duration: ibkr.Days(1), BarSize: ibkr.Bar1Hour, WhatToShow: ibkr.ShowTrades, UseRTH: true,
		})
		apiErr, ok := errors.AsType[*ibkr.APIError](err)
		if !ok || apiErr.OpKind != ibkr.OpHistoricalBars || apiErr.Code != 200 || !strings.Contains(apiErr.Message, "No security definition") {
			return fmt.Errorf("historical bars not-found error = %v", err)
		}
		recordAPIEvent("historical_bars_error", "not_found", func(event *apiDriverEvent) { event.Error = err.Error() })
		return nil
	})
}

func runAPIHistoricalBarsKeepUp(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 7*time.Minute, func(ctx context.Context, client *ibkr.Client, _ string) error {
		for _, what := range []ibkr.WhatToShow{ibkr.ShowMidpoint, ibkr.ShowBid, ibkr.ShowAsk} {
			if err := captureHistoricalBarsUpdate(ctx, client, what); err != nil {
				apiErr, ok := errors.AsType[*ibkr.APIError](err)
				if !ok || !isHistoricalDataUnavailable(apiErr, ibkr.OpHistoricalBarsStream) {
					return err
				}
				if err := fenceAPIWrites(ctx, client, "AAPL keep-up historical-bars refusal"); err != nil {
					return err
				}
				recordSubscriptionRefusal("historical_bars_keepup", strings.ToLower(string(what)), apiErr)
				return nil
			}
		}
		return nil
	})
}

func captureHistoricalBarsUpdate(ctx context.Context, client *ibkr.Client, what ibkr.WhatToShow) error {
	sub, err := client.History().SubscribeBars(ctx, ibkr.HistoricalBarsRequest{
		Contract:   apiAAPL,
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Min,
		WhatToShow: what,
		UseRTH:     true,
	}, ibkr.WithResumePolicy(ibkr.ResumeNever), ibkr.WithQueueSize(4096))
	if err != nil {
		return fmt.Errorf("subscribe AAPL %s keep-up bars: %w", what, err)
	}
	defer sub.Close()
	snapshotCtx, cancelSnapshot := context.WithTimeout(ctx, 20*time.Second)
	err = sub.AwaitSnapshot(snapshotCtx)
	cancelSnapshot()
	if err != nil {
		return fmt.Errorf("AAPL %s keep-up bars initial snapshot: %w", what, err)
	}
	initialCount := 0
	for {
		event, ok := <-sub.Events()
		if !ok {
			return fmt.Errorf("AAPL %s keep-up bars closed while draining its completed snapshot: %w", what, sub.Wait())
		}
		if event.Kind == ibkr.StreamData {
			initialCount++
		}
		if event.Kind == ibkr.StreamSnapshotComplete {
			break
		}
	}
	if initialCount == 0 {
		return fmt.Errorf("AAPL %s keep-up bars produced an empty initial snapshot", what)
	}
	updateCount, err := awaitSubscriptionEvidence(ctx, sub, 90*time.Second, func(ibkr.Bar) bool { return true })
	if err != nil {
		return fmt.Errorf("AAPL %s keep-up bars streaming update: %w", what, err)
	}
	if err := closeAndFenceSubscription(ctx, client, sub, "AAPL "+string(what)+" keep-up bars cancellation"); err != nil {
		return err
	}
	recordAPIEvent("historical_bars_keepup", "aapl_1min_"+strings.ToLower(string(what)), func(event *apiDriverEvent) {
		event.Count = initialCount
		event.Values = map[string]string{"initial_snapshot_complete": "true", "streaming_updates": strconv.Itoa(updateCount)}
	})
	return nil
}

func runAPIHistoricalScheduleAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		schedule, err := client.History().Schedule(ctx, ibkr.HistoricalScheduleRequest{
			Contract: apiAAPL, Duration: ibkr.Months(1), BarSize: ibkr.Bar1Day, UseRTH: true,
		})
		if err != nil {
			return err
		}
		if schedule.TimeZone == "" || len(schedule.Sessions) == 0 {
			return fmt.Errorf("historical schedule returned timezone=%q sessions=%d", schedule.TimeZone, len(schedule.Sessions))
		}
		recordAPIEvent("historical_schedule", "aapl", func(event *apiDriverEvent) {
			event.Count = len(schedule.Sessions)
			event.Values = map[string]string{"timezone": schedule.TimeZone}
		})
		return nil
	})
}

func runAPIHeadTimestampAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		timestamp, err := client.History().HeadTimestamp(ctx, ibkr.HeadTimestampRequest{
			Contract: apiAAPL, WhatToShow: ibkr.ShowTrades, UseRTH: true,
		})
		if err != nil {
			return err
		}
		if timestamp.IsZero() {
			return errors.New("head timestamp is zero")
		}
		recordAPIEvent("head_timestamp", "aapl", func(event *apiDriverEvent) {
			event.Values = map[string]string{"time": timestamp.UTC().Format(time.RFC3339)}
		})
		return nil
	})
}

func runAPIHistogramAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		entries, err := client.History().Histogram(ctx, ibkr.HistogramDataRequest{Contract: apiAAPL, UseRTH: true, Period: "1 week"})
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return errors.New("histogram returned no entries")
		}
		recordAPIEvent("histogram", "aapl", func(event *apiDriverEvent) { event.Count = len(entries) })
		return nil
	})
}

func isHistoricalDataUnavailable(err error, op ibkr.OpKind) bool {
	apiErr, ok := errors.AsType[*ibkr.APIError](err)
	if !ok || apiErr.OpKind != op {
		return false
	}
	switch apiErr.Code {
	case ibkr.ErrCodeHistoricalDataSubscriptionRequired:
		return strings.HasPrefix(apiErr.Message, "Up-to-the-second historical data requires additional subscription for the API")
	case 10187, 162:
		return strings.Contains(apiErr.Message, "No market data permissions") ||
			strings.Contains(apiErr.Message, "Trading TWS session is connected from a different IP address")
	default:
		return false
	}
}

func historicalTickCount(result ibkr.HistoricalTicksResult, what ibkr.WhatToShow) (int, error) {
	populated := 0
	if len(result.Ticks) > 0 {
		populated++
	}
	if len(result.BidAsk) > 0 {
		populated++
	}
	if len(result.Last) > 0 {
		populated++
	}
	if populated != 1 {
		return 0, fmt.Errorf("historical ticks populated %d result slices", populated)
	}
	switch what {
	case ibkr.ShowTrades:
		return len(result.Last), nil
	case ibkr.ShowBidAsk:
		return len(result.BidAsk), nil
	case ibkr.ShowMidpoint:
		return len(result.Ticks), nil
	default:
		return 0, fmt.Errorf("unsupported historical tick kind %q", what)
	}
}

func captureHistoricalTicks(ctx context.Context, client *ibkr.Client, label string, req ibkr.HistoricalTicksRequest) error {
	result, err := client.History().Ticks(ctx, req)
	if err != nil {
		if !isHistoricalDataUnavailable(err, ibkr.OpHistoricalTicks) {
			return err
		}
		recordAPIEvent("historical_data_unavailable", label, func(event *apiDriverEvent) { event.Error = err.Error() })
		return nil
	}
	count, err := historicalTickCount(result, req.WhatToShow)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%s returned no historical ticks", label)
	}
	recordAPIEvent("historical_ticks", label, func(event *apiDriverEvent) { event.Count = count })
	return nil
}

func runAPIHistoricalTicks(ctx context.Context, addr string, clientID int, what ibkr.WhatToShow) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		return captureHistoricalTicks(ctx, client, strings.ToLower(string(what)), ibkr.HistoricalTicksRequest{
			Contract: apiAAPL, EndTime: time.Now().UTC(), NumberOfTicks: 100, WhatToShow: what, UseRTH: true,
		})
	})
}

func runAPIHistoricalTicksTrades(ctx context.Context, addr string, clientID int) error {
	return runAPIHistoricalTicks(ctx, addr, clientID, ibkr.ShowTrades)
}

func runAPIHistoricalTicksBidAsk(ctx context.Context, addr string, clientID int) error {
	return runAPIHistoricalTicks(ctx, addr, clientID, ibkr.ShowBidAsk)
}

func runAPIHistoricalTicksMidpoint(ctx context.Context, addr string, clientID int) error {
	return runAPIHistoricalTicks(ctx, addr, clientID, ibkr.ShowMidpoint)
}

func runAPIHistoricalTicksStartBound(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 45*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		start := time.Now().UTC().AddDate(0, 0, -7)
		for _, what := range []ibkr.WhatToShow{ibkr.ShowTrades, ibkr.ShowBidAsk, ibkr.ShowMidpoint} {
			if err := captureHistoricalTicks(ctx, client, "start_"+strings.ToLower(string(what)), ibkr.HistoricalTicksRequest{
				Contract: apiAAPL, StartTime: start, NumberOfTicks: 50, WhatToShow: what, UseRTH: true,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func runAPIOrderTypeMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 6*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL anchor price: %s", anchor)

		cases := []struct {
			label        string
			order        ibkr.Order
			allowFill    bool
			cancelAfter  bool
			modifyToFill bool
		}{
			{label: "mkt_buy_fill", order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket), allowFill: true},
			{label: "marketable_lmt_buy_fill", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), marketableBuy(anchor)), allowFill: true},
			{label: "far_lmt_buy_cancel", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), cancelAfter: true},
			{label: "stp_buy_rest_cancel", order: withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeStop), marketableBuy(anchor)), cancelAfter: true},
			{label: "stp_lmt_buy_rest_cancel", order: withLimit(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeStopLimit), marketableBuy(anchor)), marketableBuy(anchor).Add(decimal.NewFromInt(1))), cancelAfter: true},
			{label: "trail_sell_reject_or_rest", order: withTrailing(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeTrailingStop), anchor), cancelAfter: true},
			{label: "trail_limit_sell_reject_or_rest", order: withTrailingLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeTrailingLimit), anchor), cancelAfter: true},
			{label: "mit_buy_reject_or_trigger", order: withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarketIfTouched), marketableBuy(anchor)), allowFill: true, cancelAfter: true},
			{label: "lit_buy_reject_or_trigger", order: withLimit(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimitIfTouched), marketableBuy(anchor)), marketableBuy(anchor).Add(decimal.NewFromInt(1))), allowFill: true, cancelAfter: true},
			{label: "mtl_buy_fill_or_reprice", order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarketToLimit), allowFill: true, cancelAfter: true},
			{label: "rel_buy_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeRelative), farBuy(anchor)), cancelAfter: true},
			{label: "delayed_success_modify", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), modifyToFill: true, allowFill: true},
			{label: "invalid_order_type_reject", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderType("FEELINGS")), farBuy(anchor))},
			{label: "moc_buy_fill_or_reject", order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarketOnClose), allowFill: true, cancelAfter: true},
			{label: "loc_buy_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimitOnClose), marketableBuy(anchor)), allowFill: true, cancelAfter: true},
			{label: "moo_buy_reject_or_queued", order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderType("MOO")), cancelAfter: true},
			{label: "loo_buy_reject_or_queued", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderType("LOO")), marketableBuy(anchor)), cancelAfter: true},
			{label: "peg_mkt_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypePeggedToMarket), farBuy(anchor)), cancelAfter: true},
			{label: "peg_pri_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderType("PEG PRI")), farBuy(anchor)), cancelAfter: true},
			{label: "peg_mid_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypePeggedToMid), farBuy(anchor)), cancelAfter: true},
			{label: "peg_best_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypePeggedToBest), farBuy(anchor)), cancelAfter: true},
			{label: "peg_bench_reject_or_rest", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypePeggedBenchmark), farBuy(anchor)), cancelAfter: true},
		}

		for _, tc := range cases {
			log.Printf("order matrix case start: %s", tc.label)
			if !clientReady(client) {
				recordAPIEvent("order_matrix_stop_session_not_ready", tc.label, nil)
				return fmt.Errorf("%s session state=%s, want ready", tc.label, client.Session().State)
			}
			caseCtx, caseCancel := context.WithTimeout(ctx, 45*time.Second)
			handle, err := placeAPIOrder(caseCtx, client, tc.label, apiAAPL, tc.order)
			if err != nil {
				caseCancel()
				validation, ok := errors.AsType[*ibkr.ValidationError](err)
				if tc.label == "peg_bench_reject_or_rest" && ok && validation.Field == "Order.PeggedBenchmark" {
					continue
				}
				return fmt.Errorf("%s place: %w", tc.label, err)
			}
			obs := observeOrder(caseCtx, handle, tc.label, 8*time.Second)
			if tc.modifyToFill && !obs.FullFill() {
				order := tc.order
				order.OrderType = ibkr.OrderTypeMarket
				order.LmtPrice = nil
				if err := modifyAPIOrder(caseCtx, client, handle, tc.label+" modify", order); err != nil {
					log.Printf("%s modify-to-fill error: %v", tc.label, err)
				} else {
					obs.Merge(observeOrder(caseCtx, handle, tc.label+" modify", 20*time.Second))
				}
			}
			if tc.cancelAfter && !handleDone(handle) {
				cancelOrder(caseCtx, client, account, handle, tc.label)
				obs.Merge(observeOrder(caseCtx, handle, tc.label+" cancel", 8*time.Second))
			}
			if obs.lastStatus == "" && !obs.terminal {
				caseCancel()
				return fmt.Errorf("%s produced no broker order lifecycle", tc.label)
			}
			if obs.AnyFill() {
				if err := flattenAAPLFill(caseCtx, client, account, tc.label, tc.order.Action, obs.filledQty); err != nil {
					caseCancel()
					return fmt.Errorf("%s flatten: %w", tc.label, err)
				}
			}
			caseCancel()
		}

		queryAAPLExecutions(client, account)
		return fenceAPIWrites(ctx, client, "order type matrix cleanup")
	})
}

func runAPIOrderFillAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		baseline, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: account})
		if err != nil {
			return fmt.Errorf("AAPL fill execution baseline: %w", err)
		}

		order := baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
		handle, err := placeAPIOrder(ctx, client, "AAPL one-share market fill", apiAAPL, order)
		if err != nil {
			return fmt.Errorf("place AAPL market fill: %w", err)
		}
		observation := observeOrder(ctx, handle, "AAPL one-share market fill", 60*time.Second)
		if !observation.FullFill() || !observation.sawExecution || !observation.filledQty.Equal(apiStockOrderQuantity) {
			return fmt.Errorf("AAPL market order status=%s filled=%s execution=%t, want one-share execution and terminal fill", observation.lastStatus, observation.filledQty, observation.sawExecution)
		}
		return awaitNewExecutionAndFee(ctx, client, account, "AAPL one-share market fill", baseline)
	})
}

func runAPIOrderRestCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL rest/cancel anchor price: %s", anchor)

		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "rest far lmt buy", order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))},
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
			cancelOrder(ctx, client, account, handle, tc.label)
			_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPIOrderDirectCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		order := withLimit(baseAPIOrder(account, apiSingleContractQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))
		handle, err := placeAPIOrder(ctx, client, "direct cancel resting", apiAAPL, order)
		if err != nil {
			return fmt.Errorf("place direct-cancel order: %w", err)
		}
		open, err := awaitOpenOrderEvidence(ctx, handle, "direct cancel resting", 15*time.Second)
		if err != nil {
			return err
		}
		if ibkr.IsTerminalOrderStatus(open.State.Status) {
			return fmt.Errorf("direct-cancel order reached terminal status %s before cancellation", open.State.Status)
		}

		recordAPIEvent("direct_cancel_start", "direct cancel resting", func(event *apiDriverEvent) {
			event.OrderID = handle.OrderID()
		})
		if err := guardedCancelOrder(ctx, client, account, clientID, handle.OrderID(), "direct order cancellation"); err != nil {
			return fmt.Errorf("direct cancel order %d: %w", handle.OrderID(), err)
		}
		recordAPIEvent("direct_cancel_sent", "direct cancel resting", func(event *apiDriverEvent) {
			event.OrderID = handle.OrderID()
		})
		obs := observeOrder(ctx, handle, "direct cancel terminal", 20*time.Second)
		if obs.lastStatus != ibkr.OrderStatusCancelled && obs.lastStatus != ibkr.OrderStatusAPICancelled {
			return fmt.Errorf("direct-cancel order status = %s, want Cancelled or ApiCancelled", obs.lastStatus)
		}
		return fenceAPIWrites(ctx, client, "direct order cancellation")
	})
}

func runAPIBracketPlaceAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		quantity := apiSingleContractQuantity
		request := ibkr.PlaceBracketRequest{
			Contract: apiAAPL,
			Parent: withLimit(
				baseAPIOrder(account, quantity, ibkr.ActionBuy, ibkr.OrderTypeLimit),
				farBuy(anchor),
			),
			TakeProfit: withLimit(
				baseAPIOrder(account, quantity, ibkr.ActionSell, ibkr.OrderTypeLimit),
				farSell(anchor),
			),
			StopLoss: withAux(
				baseAPIOrder(account, quantity, ibkr.ActionSell, ibkr.OrderTypeStop),
				farBuy(anchor).Div(decimal.NewFromInt(2)).Round(2),
			),
		}
		recordAPIEvent("place_bracket_start", "resting bracket", func(event *apiDriverEvent) {
			event.Account = account
			event.Symbol = apiAAPL.Symbol
			event.Quantity = quantity.String()
		})
		if err := requirePaperTradingSession(client, account, "place resting bracket"); err != nil {
			return err
		}
		bracket, err := client.Orders().PlaceBracket(ctx, request)
		if err != nil {
			return fmt.Errorf("place resting bracket: %w", err)
		}
		parentID := bracket.Parent.OrderID()
		if bracket.TakeProfit.OrderID() != parentID+1 || bracket.StopLoss.OrderID() != parentID+2 {
			return fmt.Errorf("bracket order IDs = %d/%d/%d, want consecutive IDs", parentID, bracket.TakeProfit.OrderID(), bracket.StopLoss.OrderID())
		}
		recordAPIEvent("place_bracket_sent", "resting bracket", func(event *apiDriverEvent) {
			event.OrderID = parentID
			event.Values = map[string]string{
				"take_profit_id": strconv.FormatInt(bracket.TakeProfit.OrderID(), 10),
				"stop_loss_id":   strconv.FormatInt(bracket.StopLoss.OrderID(), 10),
			}
		})

		parentOpen, err := awaitOpenOrderEvidence(ctx, bracket.Parent, "bracket parent", 15*time.Second)
		if err != nil {
			return err
		}
		takeProfitOpen, err := awaitOpenOrderEvidence(ctx, bracket.TakeProfit, "bracket take-profit", 15*time.Second)
		if err != nil {
			return err
		}
		stopLossOpen, err := awaitOpenOrderEvidence(ctx, bracket.StopLoss, "bracket stop-loss", 15*time.Second)
		if err != nil {
			return err
		}
		if (*parentOpen.Order.ParentID) != 0 || (*takeProfitOpen.Order.ParentID) != parentID || (*stopLossOpen.Order.ParentID) != parentID {
			return fmt.Errorf("bracket callback parent IDs = %d/%d/%d, want 0/%d/%d", (*parentOpen.Order.ParentID), (*takeProfitOpen.Order.ParentID), (*stopLossOpen.Order.ParentID), parentID, parentID)
		}

		if err := guardedCancelAll(ctx, client, account, "resting bracket cleanup"); err != nil {
			return fmt.Errorf("cancel resting bracket: %w", err)
		}
		for _, leg := range []struct {
			label  string
			handle *ibkr.OrderHandle
		}{
			{"bracket parent cleanup", bracket.Parent},
			{"bracket take-profit cleanup", bracket.TakeProfit},
			{"bracket stop-loss cleanup", bracket.StopLoss},
		} {
			obs := observeOrder(ctx, leg.handle, leg.label, 15*time.Second)
			if obs.lastStatus != ibkr.OrderStatusCancelled && obs.lastStatus != ibkr.OrderStatusAPICancelled {
				return fmt.Errorf("%s status = %s, want Cancelled or ApiCancelled", leg.label, obs.lastStatus)
			}
		}
		openOrders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeClient)
		if err != nil {
			return fmt.Errorf("verify bracket cleanup: %w", err)
		}
		for _, open := range openOrders {
			if (*open.Order.OrderID) == parentID || (*open.Order.OrderID) == parentID+1 || (*open.Order.OrderID) == parentID+2 {
				return fmt.Errorf("bracket order %d survived cleanup", (*open.Order.OrderID))
			}
		}
		return fenceAPIWrites(ctx, client, "resting bracket cleanup")
	})
}

func runAPIOrderRelativeCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL relative/cancel anchor price: %s", anchor)

		order := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeRelative), farBuy(anchor))
		handle, err := placeAPIOrder(ctx, client, "relative buy", apiAAPL, order)
		if err != nil {
			log.Printf("relative buy place: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "relative buy", 8*time.Second)
		cancelOrder(ctx, client, account, handle, "relative buy")
		_ = observeOrder(ctx, handle, "relative buy cancel", 8*time.Second)
		return nil
	})
}

func runAPIOrderTrailingCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL trailing/cancel anchor price: %s", anchor)

		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "trail sell", order: withTrailing(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeTrailingStop), anchor)},
			{label: "trail limit sell", order: withTrailingLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeTrailingLimit), anchor)},
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
			cancelOrder(ctx, client, account, handle, tc.label)
			_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPIOrderStopCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL stop/cancel anchor price: %s", anchor)

		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "stop buy", order: withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeStop), farSell(anchor))},
			{label: "stop limit buy", order: withLimit(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeStopLimit), farSell(anchor)), farSell(anchor).Add(decimal.NewFromInt(1)))},
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
			cancelOrder(ctx, client, account, handle, tc.label)
			_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
		}
		return nil
	})
}

func runAPIOrderRejectsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL reject anchor price: %s", anchor)

		cases := []struct {
			label    string
			contract ibkr.Contract
			order    ibkr.Order
		}{
			{label: "reject invalid order type", contract: apiAAPL, order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderType("FEELINGS")), farBuy(anchor))},
			{label: "reject price band", contract: apiAAPL, order: withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), anchor.Mul(decimal.NewFromInt(10)).Round(2))},
			{label: "reject invalid contract", contract: ibkr.Contract{Symbol: "ZZZZNONE", SecType: ibkr.SecTypeStock, Exchange: "SMART", Currency: "USD"}, order: baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)},
		}
		for _, tc := range cases {
			handle, err := placeAPIOrder(ctx, client, tc.label, tc.contract, tc.order)
			if err != nil {
				log.Printf("%s place returned: %v", tc.label, err)
				continue
			}
			_ = observeOrder(ctx, handle, tc.label, 12*time.Second)
			if !handleDone(handle) {
				cancelOrder(ctx, client, account, handle, tc.label)
				_ = observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second)
			}
		}
		if err := guardedCancelOrder(ctx, client, account, clientID, 999999999, "reject unknown-order cancel"); err != nil {
			log.Printf("reject cancel unknown order returned: %v", err)
		}
		return nil
	})
}

func runAPIDelayedSuccessModifyAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		order := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))
		handle, err := placeAPIOrder(ctx, client, "delayed resting", apiAAPL, order)
		if err != nil {
			return fmt.Errorf("place resting order: %w", err)
		}
		_ = observeOrder(ctx, handle, "delayed resting", 10*time.Second)

		order.OrderType = ibkr.OrderTypeMarket
		order.LmtPrice = nil
		if err := modifyAPIOrder(ctx, client, handle, "delayed modified", order); err != nil {
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
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))

		parent, err := placeAPIOrder(ctx, client, "bracket parent", apiAAPL,
			withTransmit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket), false))
		if err != nil {
			return fmt.Errorf("place bracket parent: %w", err)
		}
		tp, err := placeAPIOrder(ctx, client, "bracket take-profit", apiAAPL,
			withTransmit(withParent(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeLimit), farSell(anchor)), parent.OrderID()), false))
		if err != nil {
			return fmt.Errorf("place bracket take-profit: %w", err)
		}
		sl, err := placeAPIOrder(ctx, client, "bracket stop-loss", apiAAPL,
			withParent(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeStop), farBuy(anchor)), parent.OrderID()))
		if err != nil {
			return fmt.Errorf("place bracket stop-loss: %w", err)
		}

		parentObs := observeOrder(ctx, parent, "bracket parent", 30*time.Second)
		_ = observeOrder(ctx, tp, "bracket take-profit initial", 5*time.Second)
		_ = observeOrder(ctx, sl, "bracket stop-loss initial", 5*time.Second)
		if parentObs.FullFill() {
			tpOrder := withParent(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeLimit), marketableSell(anchor)), parent.OrderID())
			if err := modifyAPIOrder(ctx, client, tp, "bracket take-profit trigger", tpOrder); err != nil {
				log.Printf("bracket force take-profit modify: %v", err)
			}
			_ = observeOrder(ctx, tp, "bracket take-profit trigger", 30*time.Second)
			_ = observeOrder(ctx, sl, "bracket stop-loss sibling", 15*time.Second)
		}
		return nil
	})
}

func runAPIOCATriggerAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		oca := "ibkr-go-api-oca-" + strconv.FormatInt(time.Now().Unix(), 10)

		resting, err := placeAPIOrder(ctx, client, "oca resting", apiAAPL,
			withOCA(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), oca))
		if err != nil {
			return fmt.Errorf("place OCA resting peer: %w", err)
		}
		marketable, err := placeAPIOrder(ctx, client, "oca marketable", apiAAPL,
			withOCA(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), marketableBuy(anchor)), oca))
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
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		base := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))
		conditions := []struct {
			label string
			cond  ibkr.OrderCondition
		}{
			{label: "price_condition", cond: ibkr.OrderCondition{Type: ibkr.ConditionPrice, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: farSell(anchor).String(), TriggerMethod: 4}},
			// Gateway code 10314 rejects zone abbreviations like CEST; it
			// accepts the documented UTC dash form yyyymmdd-hh:mm:ss.
			{label: "time_condition", cond: ibkr.OrderCondition{Type: ibkr.ConditionTime, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, Value: time.Now().Add(2 * time.Minute).UTC().Format("20060102-15:04:05")}},
			{label: "margin_condition", cond: ibkr.OrderCondition{Type: ibkr.ConditionMargin, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, Value: "10"}},
			{label: "execution_condition", cond: ibkr.OrderCondition{Type: ibkr.ConditionExecution, Conjunction: ibkr.ConditionAnd, SecType: ibkr.SecTypeStock, Exchange: "SMART", Symbol: "AAPL"}},
			{label: "volume_condition", cond: ibkr.OrderCondition{Type: ibkr.ConditionVolume, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: "999999999"}},
			{label: "percent_change_condition", cond: ibkr.OrderCondition{Type: ibkr.ConditionPercentChange, Conjunction: ibkr.ConditionAnd, Operator: ibkr.ConditionMore, ConID: 265598, Exchange: "SMART", Value: "50"}},
		}
		for _, tc := range conditions {
			order := base
			order.Conditions = ibkr.OrderConditions{
				Values:    []ibkr.OrderCondition{tc.cond},
				IgnoreRTH: true,
			}
			handle, err := placeAPIOrder(ctx, client, tc.label, apiAAPL, order)
			if err != nil {
				return fmt.Errorf("%s place: %w", tc.label, err)
			}
			observation := observeOrder(ctx, handle, tc.label, 8*time.Second)
			if !handleDone(handle) {
				cancelOrder(ctx, client, account, handle, tc.label)
				observation.Merge(observeOrder(ctx, handle, tc.label+" cancel", 8*time.Second))
			}
			if observation.lastStatus == "" && !observation.terminal {
				return fmt.Errorf("%s produced no broker order lifecycle", tc.label)
			}
		}
		return nil
	})
}

func runAPITIFAttributeMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 6*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL TIF/attribute anchor: %s", anchor)

		now := time.Now().UTC()
		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "gtc_far_lmt", order: withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTC)},
			{label: "gtd_far_lmt", order: withGoodTillDate(withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTD), orderTimestamp(now.Add(2*time.Hour)))},
			{label: "good_after_far_lmt", order: withGoodAfterTime(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), orderTimestamp(now.Add(2*time.Minute)))},
			{label: "all_or_none_far_lmt", order: withAllOrNone(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)))},
			{label: "min_qty_far_lmt", order: withMinQty(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), decimal.NewFromInt(3), 2)},
			{label: "rel_percent_offset", order: withPercentOffset(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeRelative), farBuy(anchor)), decimal.RequireFromString("0.03"))},
			{label: "trailing_percent", order: withTrailingPercent(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeTrailingStop), anchor, decimal.RequireFromString("1.5"))},
			{label: "trigger_method_stop", order: withTriggerMethod(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeStop), farSell(anchor)), 4)},
			{label: "explicit_order_ref", order: withOrderRef(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "ibkrgo-explicit-ref-"+scenarioHash("api_tif_attribute_matrix_aapl"))},
			{label: "scale_far_lmt", order: withScale(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)))},
			{label: "active_window_far_lmt", order: withActiveWindow(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), orderTimestamp(now.Add(2*time.Minute)), orderTimestamp(now.Add(4*time.Minute)))},
			{label: "price_management_far_lmt", order: withPriceManagement(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)))},
			{label: "adjusted_stop_fields", order: withAdjustedStop(withAux(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeStop), farBuy(anchor)), anchor)},
			{label: "manual_order_time_far_lmt", order: withManualOrderTime(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), orderTimestamp(now))},
			{label: "advanced_error_override_far_lmt", order: withAdvancedErrorOverride(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "IBDBUYTX")},
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
				cancelOrder(caseCtx, client, account, handle, tc.label)
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
		return fenceAPIWrites(ctx, client, "TIF attribute matrix cleanup")
	})
}

func runAPISecurityTypeProbeMatrix(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, _ string) error {
		option, err := qualifyAAPLCall(ctx, client, decimal.NewFromInt(300))
		if err != nil {
			return fmt.Errorf("qualify bounded AAPL option probe: %w", err)
		}
		futureOption, err := qualifyFrontFutureOption(ctx, client, "MES")
		if err != nil {
			return fmt.Errorf("qualify bounded MES future-option probe: %w", err)
		}
		probes := []struct {
			label         string
			contract      ibkr.Contract
			allowAPIError bool
		}{
			{label: "stk_aapl", contract: apiAAPL},
			{label: "opt_aapl", contract: option},
			{label: "fut_mes_front", contract: ibkr.Contract{Symbol: "MES", SecType: ibkr.SecTypeFuture, Exchange: "CME", Currency: "USD"}},
			{label: "fop_mes", contract: futureOption},
			{label: "cash_eurusd", contract: apiEURUSD},
			{label: "bond_probe", contract: ibkr.Contract{Symbol: "912797", SecType: ibkr.SecTypeBond, Exchange: "SMART", Currency: "USD"}, allowAPIError: true},
			{label: "cfd_aapl", contract: ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeCFD, Exchange: "SMART", Currency: "USD"}},
			{label: "war_tencent", contract: ibkr.Contract{Symbol: "700", SecType: ibkr.SecTypeWarrant, Exchange: "SEHK", Currency: "HKD", Expiry: "202612", Strike: new(decimal.NewFromInt(700)), Right: ibkr.RightCall}},
			{label: "ind_spx", contract: ibkr.Contract{Symbol: "SPX", SecType: ibkr.SecTypeIndex, Exchange: "CBOE", Currency: "USD"}},
			{label: "crypto_btc", contract: ibkr.Contract{Symbol: "BTC", SecType: ibkr.SecTypeCrypto, Exchange: "PAXOS", Currency: "USD"}},
			{label: "fund_vtsax", contract: ibkr.Contract{Symbol: "VTSAX", SecType: ibkr.SecTypeFund, Exchange: "FUNDSERV", Currency: "USD"}},
			{label: "bill_probe", contract: ibkr.Contract{Symbol: "912797", SecType: ibkr.SecTypeBill, Exchange: "SMART", Currency: "USD"}, allowAPIError: true},
			{label: "cmdty_xauusd", contract: ibkr.Contract{Symbol: "XAUUSD", SecType: ibkr.SecTypeCommodity, Exchange: "SMART", Currency: "USD"}},
			{label: "contfut_es", contract: ibkr.Contract{Symbol: "ES", SecType: ibkr.SecTypeContFuture, Exchange: "CME", Currency: "USD"}},
		}
		for _, probe := range probes {
			caseCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			details, err := client.Contracts().Details(caseCtx, probe.contract)
			cancel()
			if err != nil {
				recordAPIEvent("contract_probe_error", probe.label, func(event *apiDriverEvent) {
					event.Symbol = probe.contract.Symbol
					event.SecType = string(probe.contract.SecType)
					event.Error = err.Error()
				})
				if _, ok := errors.AsType[*ibkr.APIError](err); probe.allowAPIError && ok {
					continue
				}
				return fmt.Errorf("security probe %s: %w", probe.label, err)
			}
			if len(details) == 0 {
				return fmt.Errorf("security probe %s returned no contract details", probe.label)
			}
			recordAPIEvent("contract_probe", probe.label, func(event *apiDriverEvent) {
				event.Symbol = probe.contract.Symbol
				event.SecType = string(probe.contract.SecType)
				event.Count = len(details)
				event.Values = map[string]string{
					"con_id":        strconv.FormatInt(int64(details[0].ConID), 10),
					"expiry":        details[0].Expiry,
					"local_symbol":  details[0].LocalSymbol,
					"trading_class": details[0].TradingClass,
				}
			})
		}
		return nil
	})
}

func runAPIPnL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 15*time.Second, func(ctx context.Context, client *ibkr.Client, account string) error {
		sub, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: account}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe account PnL: %w", err)
		}
		var update ibkr.PnLUpdate
		count, err := awaitSubscriptionEvidence(ctx, sub, 10*time.Second, func(value ibkr.PnLUpdate) bool {
			update = value
			return true
		})
		if err != nil {
			sub.Close()
			return fmt.Errorf("observe account PnL: %w", err)
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "account PnL cancellation"); err != nil {
			return err
		}
		recordAPIEvent("pnl", "account", func(event *apiDriverEvent) {
			event.Count = count
			event.Values = map[string]string{
				"daily":      optionalDecimalString(update.DailyPnL),
				"unrealized": optionalDecimalString(update.UnrealizedPnL),
				"realized":   optionalDecimalString(update.RealizedPnL),
			}
		})
		return nil
	})
}

func runAPIPnLSingle(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 20*time.Second, func(ctx context.Context, client *ibkr.Client, account string) error {
		accountUpdates, err := client.Accounts().Updates(ctx, account)
		if err != nil {
			return fmt.Errorf("derive held position for single-position PnL: %w", err)
		}
		var held ibkr.PortfolioUpdate
		found := false
		for _, accountUpdate := range accountUpdates {
			position := accountUpdate.Portfolio
			if position == nil || position.Contract.ConID <= 0 || position.Position.IsZero() {
				continue
			}
			if !found || position.Contract.SecType == ibkr.SecTypeStock {
				held = *position
				found = true
			}
			if position.Contract.SecType == ibkr.SecTypeStock {
				break
			}
		}
		if !found {
			return errors.New("single-position PnL requires a live held position with a contract ID")
		}
		sub, err := client.Accounts().SubscribePnLSingle(ctx, ibkr.PnLSingleRequest{
			Account: account,
			ConID:   held.Contract.ConID,
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe %s single-position PnL: %w", held.Contract.Symbol, err)
		}
		var update ibkr.PnLSingleUpdate
		count, err := awaitSubscriptionEvidence(ctx, sub, 10*time.Second, func(value ibkr.PnLSingleUpdate) bool {
			update = value
			return true
		})
		if err != nil {
			sub.Close()
			return fmt.Errorf("observe %s single-position PnL: %w", held.Contract.Symbol, err)
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "single-position PnL cancellation"); err != nil {
			return err
		}
		recordAPIEvent("pnl_single", "held_position", func(event *apiDriverEvent) {
			event.Symbol = held.Contract.Symbol
			event.SecType = string(held.Contract.SecType)
			event.Count = count
			event.Values = map[string]string{
				"con_id":     strconv.FormatInt(int64(held.Contract.ConID), 10),
				"position":   update.Position.String(),
				"daily":      optionalDecimalString(update.DailyPnL),
				"unrealized": optionalDecimalString(update.UnrealizedPnL),
				"realized":   optionalDecimalString(update.RealizedPnL),
				"value":      optionalDecimalString(update.Value),
			}
		})
		return nil
	})
}

func runAPIScannerSubscription(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 30*time.Second, func(ctx context.Context, client *ibkr.Client, _ string) error {
		sub, err := client.Scanner().SubscribeResults(ctx, ibkr.ScannerSubscriptionRequest{
			NumberOfRows: 10,
			Instrument:   ibkr.ScannerInstrument("STK"),
			LocationCode: ibkr.ScannerLocationCode("STK.US.MAJOR"),
			ScanCode:     ibkr.ScannerCode("HOT_BY_VOLUME"),
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("subscribe HOT_BY_VOLUME scanner: %w", err)
		}
		rowCount := 0
		_, err = awaitSubscriptionEvidence(ctx, sub, 20*time.Second, func(rows []ibkr.ScannerResult) bool {
			rowCount = len(rows)
			return true
		})
		if err != nil {
			apiErr, ok := errors.AsType[*ibkr.APIError](err)
			if !ok || !isExactScannerRefusal(apiErr) {
				return fmt.Errorf("observe HOT_BY_VOLUME scanner: %w", err)
			}
			if err := fenceAPIWrites(ctx, client, "HOT_BY_VOLUME scanner refusal"); err != nil {
				return err
			}
			recordSubscriptionRefusal("scanner_subscription", "hot_by_volume", apiErr)
			return nil
		}
		if err := closeAndFenceSubscription(ctx, client, sub, "HOT_BY_VOLUME scanner cancellation"); err != nil {
			return err
		}
		recordProbeResult("scanner_subscription", "hot_by_volume", rowCount, nil)
		return nil
	})
}

func isExactScannerRefusal(err *ibkr.APIError) bool {
	if err.OpKind != ibkr.OpScannerSubscription {
		return false
	}
	switch err.Code {
	case 490:
		return strings.HasPrefix(err.Message, "You must subscribe for additional permissions to run the scanner.")
	default:
		return false
	}
}

func runAPIGenericTickMatrixAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 1*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			recordProbeResult("generic_tick_set_delayed", "aapl", 0, err)
			log.Printf("generic tick matrix set delayed: %v", err)
		}

		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
			Contract: apiAAPL,
			GenericTicks: []ibkr.GenericTick{
				"221", // mark price
				"233", // real-time volume
				"236", // shortable
				"293", // trade count
				"294", // trade rate
				"295", // volume rate
			},
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			recordProbeResult("generic_tick_subscribe", "aapl", 0, err)
			return nil
		}
		defer sub.Close()

		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		count := 0
		for {
			select {
			case event, ok := <-sub.Events():
				if !ok {
					recordProbeResult("generic_tick_matrix", "aapl", count, sub.Err())
					return nil
				}
				if event.Kind != ibkr.StreamData {
					continue
				}
				count++
				update := event.Value
				values := quoteUpdateValues(update)
				recordAPIEvent("quote_update", update.Kind.String(), func(event *apiDriverEvent) {
					event.Symbol = apiAAPL.Symbol
					event.SecType = string(apiAAPL.SecType)
					event.Values = values
				})
				log.Printf("generic tick matrix update kind=%s values=%v", update.Kind, values)
			case <-sub.Done():
				recordProbeResult("generic_tick_matrix", "aapl", count, sub.Err())
				return nil
			case <-timer.C:
				recordProbeResult("generic_tick_matrix", "aapl", count, nil)
				return nil
			case <-ctx.Done():
				recordProbeResult("generic_tick_matrix", "aapl", count, ctx.Err())
				return nil
			}
		}
	})
}

func quoteUpdateValues(update ibkr.QuoteUpdate) map[string]string {
	values := map[string]string{
		"changed":   strconv.FormatUint(uint64(update.Changed), 10),
		"available": strconv.FormatUint(uint64(update.Snapshot.Available), 10),
	}
	switch update.Kind {
	case ibkr.QuoteUpdatePriceTick:
		if update.PriceTick != nil {
			values["tick_type"] = strconv.Itoa(update.PriceTick.TickType)
			values["price"] = update.PriceTick.Price.String()
			values["attr_mask"] = strconv.Itoa(int(update.PriceTick.AttrMask))
			if update.PriceTick.Size != nil {
				values["size"] = update.PriceTick.Size.String()
			}
		}
	case ibkr.QuoteUpdateSizeTick:
		if update.SizeTick != nil {
			values["tick_type"] = strconv.Itoa(update.SizeTick.TickType)
			if update.SizeTick.Size != nil {
				values["size"] = update.SizeTick.Size.String()
			}
		}
	case ibkr.QuoteUpdateGenericTick:
		if update.GenericTick != nil {
			values["tick_type"] = strconv.Itoa(update.GenericTick.TickType)
			values["value"] = update.GenericTick.Value.String()
		}
	case ibkr.QuoteUpdateStringTick:
		if update.StringTick != nil {
			values["tick_type"] = strconv.Itoa(update.StringTick.TickType)
			values["value"] = update.StringTick.Value
		}
	case ibkr.QuoteUpdateNewsTick:
		if update.NewsTick != nil {
			values["time"] = strconv.FormatInt(update.NewsTick.Time.UnixMilli(), 10)
			values["provider_code"] = string(update.NewsTick.ProviderCode)
			values["article_id"] = update.NewsTick.ArticleID
			values["headline"] = update.NewsTick.Headline
			values["extra_data"] = update.NewsTick.ExtraData
		}
	case ibkr.QuoteUpdateParameters:
		if update.Parameters != nil {
			if update.Parameters.MinTick != nil {
				values["min_tick"] = update.Parameters.MinTick.String()
			}
			values["bbo_exchange"] = update.Parameters.BBOExchange
			if update.Parameters.SnapshotPermissions != nil {
				values["snapshot_permissions"] = strconv.Itoa(*update.Parameters.SnapshotPermissions)
			}
			if update.Parameters.LastPricePrecision != nil {
				values["last_price_precision"] = update.Parameters.LastPricePrecision.String()
			}
			if update.Parameters.LastSizePrecision != nil {
				values["last_size_precision"] = update.Parameters.LastSizePrecision.String()
			}
		}
	case ibkr.QuoteUpdateOptionComputation:
		if update.OptionComputation != nil {
			values["tick_type"] = strconv.Itoa(update.OptionComputation.TickType)
			values["tick_attrib"] = strconv.Itoa(update.OptionComputation.TickAttrib)
		}
	}
	return values
}

func runAPITickNewsAAPLProbe(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 1*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			recordProbeResult("tick_news_set_delayed", "aapl_brfg", 0, err)
			return fmt.Errorf("set delayed market data for tick-news probe: %w", err)
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
			Contract:     apiAAPL,
			GenericTicks: []ibkr.GenericTick{"mdoff", "292:BRFG"},
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			recordProbeResult("tick_news_subscribe", "aapl_brfg", 0, err)
			return fmt.Errorf("subscribe AAPL tick-news: %w", err)
		}
		defer sub.Close()

		closeSubscription := func() error {
			return closeAndFenceSubscription(ctx, client, sub, "AAPL tick-news cancellation")
		}

		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		count := 0
		for {
			select {
			case event, ok := <-sub.Events():
				if !ok {
					err := sub.Wait()
					recordProbeResult("tick_news", "aapl_brfg", count, err)
					if err == nil {
						err = errors.New("subscription closed before a news tick")
					}
					return fmt.Errorf("observe AAPL tick-news: %w", err)
				}
				if event.Kind != ibkr.StreamData {
					continue
				}
				count++
				update := event.Value
				recordAPIEvent("quote_update", update.Kind.String(), func(event *apiDriverEvent) {
					event.Symbol = apiAAPL.Symbol
					event.SecType = string(apiAAPL.SecType)
					event.Values = quoteUpdateValues(update)
				})
				if update.Kind == ibkr.QuoteUpdateNewsTick {
					recordProbeResult("tick_news", "aapl_brfg", count, nil)
					return closeSubscription()
				}
			case <-sub.Done():
				err := sub.Wait()
				recordProbeResult("tick_news", "aapl_brfg", count, err)
				if err == nil {
					err = errors.New("subscription closed before a news tick")
				}
				return fmt.Errorf("observe AAPL tick-news: %w", err)
			case <-timer.C:
				recordProbeResult("tick_news", "aapl_brfg", count, nil)
				return errors.Join(closeSubscription(), errors.New("AAPL tick-news produced no news tick within 30s"))
			case <-ctx.Done():
				err := context.Cause(ctx)
				recordProbeResult("tick_news", "aapl_brfg", count, err)
				return err
			}
		}
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
				if errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("historical bar size %s timed out: %w", barCase.label, err)
				}
				log.Printf("historical bar size %s: %v", barCase.label, err)
			}
		}

		for _, what := range []ibkr.WhatToShow{
			ibkr.ShowTrades, ibkr.ShowBidAsk, ibkr.ShowMidpoint, ibkr.ShowAdjustedLast,
			ibkr.ShowHistoricalVolatility, ibkr.ShowOptionImpliedVolatility,
		} {
			caseCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			request := ibkr.HistoricalBarsRequest{
				Contract: apiAAPL, Duration: ibkr.Weeks(1),
				BarSize: ibkr.Bar1Day, WhatToShow: what, UseRTH: true,
			}
			if what != ibkr.ShowAdjustedLast {
				request.EndTime = time.Now()
			}
			bars, err := client.History().Bars(caseCtx, request)
			cancel()
			recordProbeResult("historical_what_to_show", string(what), len(bars), err)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("historical whatToShow %s timed out: %w", what, err)
				}
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
			TotalResults: 5,
		})
		cancel()
		if err != nil {
			recordProbeResult("historical_news_for_article", "aapl", 0, err)
			return fmt.Errorf("query historical news for article: %w", err)
		}
		recordProbeResult("historical_news_for_article", "aapl", len(items.Items), nil)
		if len(items.Items) == 0 {
			return errors.New("historical news returned no article candidate")
		}

		articleReq := ibkr.NewsArticleRequest{ProviderCode: items.Items[0].ProviderCode, ArticleID: items.Items[0].ArticleID}
		articleCtx, articleCancel := context.WithTimeout(ctx, 30*time.Second)
		article, err := client.News().Article(articleCtx, articleReq)
		articleCancel()
		if err != nil {
			recordProbeResult("news_article", string(articleReq.ProviderCode), 0, err)
			return fmt.Errorf("query news article %s/%s: %w", articleReq.ProviderCode, articleReq.ArticleID, err)
		}
		if len(article.ArticleText) == 0 {
			return fmt.Errorf("news article %s/%s returned an empty body", articleReq.ProviderCode, articleReq.ArticleID)
		}
		recordAPIEvent("news_article", string(articleReq.ProviderCode), func(event *apiDriverEvent) {
			event.Values = map[string]string{
				"article_id":   articleReq.ArticleID,
				"article_type": strconv.FormatInt(int64(article.ArticleType), 10),
				"text_bytes":   strconv.Itoa(len(article.ArticleText)),
			}
		})
		return nil
	})
}

func runAPIWSHVariantsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		metaCtx, metaCancel := context.WithTimeout(ctx, 20*time.Second)
		meta, err := client.WSH().MetaData(metaCtx)
		metaCancel()
		if err := captureWSHResult("metadata", ibkr.OpWSHMetaData, meta, err); err != nil {
			return fmt.Errorf("WSH metadata variant: %w", err)
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
			if err := captureWSHResult(eventCase.label, ibkr.OpWSHEventData, data, err); err != nil {
				return fmt.Errorf("WSH event-data variant %s: %w", eventCase.label, err)
			}
		}
		return nil
	})
}

func runAPIAlgoVariantsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 7*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		start := orderTimestamp(time.Now().UTC().Add(3 * time.Minute))
		end := orderTimestamp(time.Now().UTC().Add(20 * time.Minute))
		log.Printf("AAPL algo variants anchor: %s", anchor)

		cases := []struct {
			label string
			order ibkr.Order
		}{
			{label: "adaptive_urgent", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "Adaptive", []ibkr.TagValue{{Tag: "adaptivePriority", Value: "Urgent"}})},
			{label: "adaptive_patient", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "Adaptive", []ibkr.TagValue{{Tag: "adaptivePriority", Value: "Patient"}})},
			{label: "twap", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "Twap", []ibkr.TagValue{{Tag: "strategyType", Value: "Marketable"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "allowPastEndTime", Value: "1"}})},
			{label: "vwap", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "Vwap", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "allowPastEndTime", Value: "1"}, {Tag: "noTakeLiq", Value: "1"}})},
			{label: "arrival_px", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "ArrivalPx", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}, {Tag: "riskAversion", Value: "Neutral"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "forceCompletion", Value: "0"}, {Tag: "allowPastEndTime", Value: "1"}})},
			{label: "dark_ice", order: withAlgo(withDisplaySize(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), 1), "DarkIce", []ibkr.TagValue{{Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "allowPastEndTime", Value: "1"}})},
			{label: "accum_dist", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "AD", []ibkr.TagValue{{Tag: "componentSize", Value: "1"}, {Tag: "timeBetweenOrders", Value: "60"}, {Tag: "randomizeTime20", Value: "0"}, {Tag: "randomizeSize55", Value: "0"}, {Tag: "giveUp", Value: "0"}, {Tag: "catchUp", Value: "1"}, {Tag: "waitForFill", Value: "1"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}})},
			{label: "inline", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "Inline", []ibkr.TagValue{{Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}})},
			{label: "close", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "ClosePx", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}, {Tag: "riskAversion", Value: "Neutral"}, {Tag: "startTime", Value: start}, {Tag: "forceCompletion", Value: "0"}})},
			{label: "pct_vol", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "PctVol", []ibkr.TagValue{{Tag: "pctVol", Value: "0.1"}, {Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "noTakeLiq", Value: "1"}})},
			{label: "balance_impact_risk", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "BalanceImpactRisk", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}, {Tag: "riskAversion", Value: "Neutral"}})},
			{label: "min_impact", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "MinImpact", []ibkr.TagValue{{Tag: "maxPctVol", Value: "0.1"}})},
			{label: "jefferies_ad", order: withAlgo(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), "JefAD", []ibkr.TagValue{{Tag: "startTime", Value: start}, {Tag: "endTime", Value: end}, {Tag: "componentSize", Value: "1"}, {Tag: "timeBetweenOrders", Value: "60"}})},
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
				cancelOrder(caseCtx, client, account, handle, tc.label)
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
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_, _ = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: apiAAPL})
		_, _ = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: apiMSFT})

		aaplOrder := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
		aapl, err := placeAPIOrder(ctx, client, "pairs aapl buy", apiAAPL, aaplOrder)
		if err != nil {
			log.Printf("pairs AAPL buy: %v", err)
		}
		msftOrder := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.ActionSell, ibkr.OrderTypeMarket)
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
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		filledQty := decimal.Zero
		for i := 0; i < 3; i++ {
			order := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
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
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		baselineExecutions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"})
		if err != nil {
			return fmt.Errorf("stop-management execution baseline: %w", err)
		}
		buyOrder := baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
		buy, err := placeAPIOrder(ctx, client, "stop-management buy", apiAAPL, buyOrder)
		if err != nil {
			return fmt.Errorf("place stop-management entry: %w", err)
		}
		buyObs := observeOrder(ctx, buy, "stop-management buy", 30*time.Second)
		if !buyObs.FullFill() || !buyObs.sawExecution || !buyObs.filledQty.Equal(apiStockOrderQuantity) {
			if !handleDone(buy) {
				cancelOrder(ctx, client, account, buy, "stop-management unfilled buy")
				_ = observeOrder(ctx, buy, "stop-management unfilled buy cancel", 8*time.Second)
			}
			return fmt.Errorf("stop-management entry status=%s filled=%s execution=%t, want terminal fill", buyObs.lastStatus, buyObs.filledQty, buyObs.sawExecution)
		}

		stopOrder := withAux(baseAPIOrder(account, buyObs.filledQty, ibkr.ActionSell, ibkr.OrderTypeStop), farBuy(anchor))
		stop, err := placeAPIOrder(ctx, client, "stop-management stop", apiAAPL, stopOrder)
		if err != nil {
			flattenErr := flattenAAPL(ctx, client, account, "stop-management emergency", buyObs.filledQty)
			return errors.Join(fmt.Errorf("place stop-management stop: %w", err), flattenErr, fenceAPIWrites(ctx, client, "stop-management emergency cleanup"))
		}
		stopEcho, err := awaitOpenOrderEvidence(ctx, stop, "stop-management stop", 20*time.Second)
		if err != nil {
			return err
		}
		if stopEcho.Order.OrderType != ibkr.OrderTypeStop || !stopEcho.Order.Prices.AuxPrice.Equal(farBuy(anchor)) {
			return fmt.Errorf("stop-management initial echo type=%s aux=%s, want STP %s", stopEcho.Order.OrderType, stopEcho.Order.Prices.AuxPrice, farBuy(anchor))
		}

		stopOrder.AuxPrice = new(farBuy(anchor).Add(decimal.NewFromInt(1)))
		if err := modifyAPIOrder(ctx, client, stop, "stop-management moved stop", stopOrder); err != nil {
			return fmt.Errorf("replace stop-management stop: %w", err)
		}
		movedEcho, err := awaitOpenOrderEvidence(ctx, stop, "stop-management moved stop", 20*time.Second)
		if err != nil {
			return err
		}
		if !movedEcho.Order.Prices.AuxPrice.Equal(*stopOrder.AuxPrice) {
			return fmt.Errorf("stop-management replacement aux=%s, want %s", movedEcho.Order.Prices.AuxPrice, *stopOrder.AuxPrice)
		}
		cancelOrder(ctx, client, account, stop, "stop-management stop")
		stopObservation := observeOrder(ctx, stop, "stop-management stop cancel", 20*time.Second)
		if !stopObservation.terminal || stopObservation.AnyFill() {
			return fmt.Errorf("stop-management cleanup status=%s filled=%s, want terminal zero fill", stopObservation.lastStatus, stopObservation.filledQty)
		}
		if err := flattenAAPL(ctx, client, account, "stop-management flatten", buyObs.filledQty); err != nil {
			return errors.Join(err, fenceAPIWrites(ctx, client, "stop-management failed flatten"))
		}
		executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"})
		if err != nil {
			return fmt.Errorf("stop-management executions: %w", err)
		}
		if countNewExecutions(baselineExecutions, executions) != 2 {
			return fmt.Errorf("stop-management new executions=%d, want 2", countNewExecutions(baselineExecutions, executions))
		}
		if err := verifyNewExecutionFees(baselineExecutions, executions); err != nil {
			return fmt.Errorf("stop-management execution fees: %w", err)
		}
		return fenceAPIWrites(ctx, client, "stop-management cleanup")
	})
}

func runAPIBracketTrailingStopAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		parentOrder := withTransmit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket), false)
		parent, err := placeAPIOrder(ctx, client, "trailing bracket parent", apiAAPL,
			parentOrder)
		if err != nil {
			log.Printf("trailing bracket parent: %v", err)
			return nil
		}
		takeProfitOrder := withTransmit(withParent(withLimit(baseAPIOrder(account, parentOrder.Quantity, ibkr.ActionSell, ibkr.OrderTypeLimit), farSell(anchor)), parent.OrderID()), false)
		takeProfit, err := placeAPIOrder(ctx, client, "trailing bracket take-profit", apiAAPL,
			takeProfitOrder)
		if err != nil {
			log.Printf("trailing bracket take-profit: %v", err)
			return nil
		}
		trailingStopOrder := withParent(withTrailing(baseAPIOrder(account, parentOrder.Quantity, ibkr.ActionSell, ibkr.OrderTypeTrailingStop), anchor), parent.OrderID())
		trailingStopOrder.TrailStopPrice = new(farBuy(anchor))
		trailingStop, err := placeAPIOrder(ctx, client, "trailing bracket stop", apiAAPL, trailingStopOrder)
		if err != nil {
			log.Printf("trailing bracket stop: %v", err)
			return nil
		}

		parentObs := observeOrder(ctx, parent, "trailing bracket parent", 30*time.Second)
		_ = observeOrder(ctx, takeProfit, "trailing bracket take-profit", 8*time.Second)
		_ = observeOrder(ctx, trailingStop, "trailing bracket stop", 8*time.Second)
		if err := guardedCancelAll(ctx, client, account, "trailing bracket global cancel"); err != nil {
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
	return errors.New("option campaign is blocked until option and resulting stock deltas can be terminally reconciled")
}

func runAPIOptionCalculationsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiScenario(ctx, addr, clientID, 2*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		_ = account
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		opt, err := qualifyAAPLCall(ctx, client, anchor)
		if err != nil {
			recordProbeResult("option_qualify", "aapl call", 0, err)
			return fmt.Errorf("qualify AAPL call for option calculations: %w", err)
		}
		if opt.Strike == nil {
			return fmt.Errorf("qualified AAPL option has no strike")
		}
		recordAPIEvent("option_qualified", "aapl call", func(event *apiDriverEvent) {
			event.Symbol = opt.Symbol
			event.SecType = string(opt.SecType)
			event.Values = map[string]string{
				"con_id":      strconv.FormatInt(int64(opt.ConID), 10),
				"expiry":      opt.Expiry,
				"strike":      opt.Strike.String(),
				"right":       string(opt.Right),
				"under_price": anchor.String(),
			}
		})

		record := func(label string, value ibkr.OptionComputation, err error) {
			recordAPIEvent("option_computation", label, func(event *apiDriverEvent) {
				if err != nil {
					event.Error = err.Error()
					return
				}
				event.Values = map[string]string{
					"available":    strconv.FormatUint(uint64(value.Available), 10),
					"implied_vol":  value.ImpliedVol.String(),
					"delta":        value.Delta.String(),
					"option_price": value.OptPrice.String(),
					"pv_dividend":  value.PvDividend.String(),
					"gamma":        value.Gamma.String(),
					"vega":         value.Vega.String(),
					"theta":        value.Theta.String(),
					"under_price":  value.UndPrice.String(),
				}
			})
		}

		price, priceErr := client.Options().Price(ctx, ibkr.CalcOptionPriceRequest{
			Contract:   opt,
			Volatility: decimal.RequireFromString("0.30"),
			UnderPrice: anchor,
		})
		record("price", price, priceErr)
		var resultErr error
		if priceErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("calculate option price: %w", priceErr))
		} else if price.Available == 0 {
			resultErr = errors.Join(resultErr, errors.New("option-price calculation returned no available fields"))
		}

		optionPrice := decimal.RequireFromString("5")
		if priceErr == nil && price.Available&ibkr.OptionComputationPrice != 0 {
			optionPrice = price.OptPrice
		}
		implied, impliedErr := client.Options().ImpliedVolatility(ctx, ibkr.CalcImpliedVolatilityRequest{
			Contract:    opt,
			OptionPrice: optionPrice,
			UnderPrice:  anchor,
		})
		record("implied_volatility", implied, impliedErr)
		if impliedErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("calculate implied volatility: %w", impliedErr))
		} else if implied.Available == 0 {
			resultErr = errors.Join(resultErr, errors.New("implied-volatility calculation returned no available fields"))
		}
		return resultErr
	})
}

func runAPIFutureCampaignMES(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		fut, err := qualifyFrontFuture(ctx, client, "MES")
		if err != nil {
			return fmt.Errorf("qualify MES future: %w", err)
		}
		log.Printf("qualified future: %+v", fut)
		_, _ = client.MarketData().Quote(ctx, ibkr.QuoteRequest{Contract: fut})

		futureBuy := baseAPIOrder(account, apiSingleContractQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
		handle, err := placeAPIOrder(ctx, client, "future buy", fut, futureBuy)
		if err != nil {
			return fmt.Errorf("place future market buy: %w", err)
		}
		obs := observeOrder(ctx, handle, "future buy", 40*time.Second)
		if !obs.FullFill() || !obs.sawExecution || !obs.filledQty.Equal(apiSingleContractQuantity) {
			return fmt.Errorf("future buy status=%s filled=%s execution=%t, want one-contract terminal fill", obs.lastStatus, obs.filledQty, obs.sawExecution)
		}
		futureSell := baseAPIOrder(account, obs.filledQty, ibkr.ActionSell, ibkr.OrderTypeMarket)
		sell, err := placeAPIOrder(ctx, client, "future flatten", fut, futureSell)
		if err != nil {
			return fmt.Errorf("place future flatten: %w", err)
		}
		flattened := observeOrder(ctx, sell, "future flatten", 40*time.Second)
		if !flattened.FullFill() || !flattened.sawExecution || !flattened.filledQty.Equal(obs.filledQty) {
			return fmt.Errorf("future flatten status=%s filled=%s execution=%t, want %s-contract terminal fill", flattened.lastStatus, flattened.filledQty, flattened.sawExecution, obs.filledQty)
		}
		_, _ = client.Accounts().Positions(ctx)
		queryExecutions(client, ibkr.ExecutionsRequest{Account: account, Symbol: "MES"}, "future executions")
		return nil
	})
}

func runAPIComboOptionVerticalAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		lower, upper, err := qualifyAAPLCallVertical(ctx, client, anchor)
		if err != nil {
			recordAPIEvent("option_qualify_error", "aapl call vertical", func(event *apiDriverEvent) {
				event.Symbol = "AAPL"
				event.SecType = string(ibkr.SecTypeOption)
				event.Error = err.Error()
			})
			return fmt.Errorf("qualify AAPL call vertical: %w", err)
		}
		if lower.Strike == nil || upper.Strike == nil {
			return errors.New("qualified AAPL vertical has no strikes")
		}
		width := upper.Strike.Sub(*lower.Strike)
		lowerPrice := decimal.RequireFromString("0.04")
		upperPrice := decimal.RequireFromString("0.01")
		netDebit := lowerPrice.Sub(upperPrice)
		if !width.IsPositive() || !netDebit.IsPositive() || netDebit.GreaterThan(width) {
			return fmt.Errorf("AAPL vertical width=%s cannot bound net per-leg debit %s", width, netDebit)
		}
		bag := ibkr.Contract{Symbol: "AAPL", SecType: ibkr.SecTypeCombo, Exchange: "SMART", Currency: "USD", ComboLegs: []ibkr.ComboLeg{
			{ConID: lower.ConID, Ratio: 1, Action: "BUY", Exchange: "SMART", OpenClose: ibkr.ComboLegSame},
			{ConID: upper.ConID, Ratio: 1, Action: "SELL", Exchange: "SMART", OpenClose: ibkr.ComboLegSame},
		}}
		order := baseAPIOrder(account, apiOptionContractQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit)
		order.Combo = ibkr.OrderCombo{
			LegPrices:    []*decimal.Decimal{new(lowerPrice), new(upperPrice)},
			SmartRouting: []ibkr.TagValue{{Tag: "NonGuaranteed", Value: "1"}},
		}
		handle, err := placeAPIOrder(ctx, client, "option vertical BAG", bag, order)
		if err != nil {
			return fmt.Errorf("place AAPL vertical BAG: %w", err)
		}
		echo, err := awaitOpenOrderEvidence(ctx, handle, "option vertical BAG", 30*time.Second)
		if err != nil {
			return err
		}
		if len(echo.Contract.ComboLegs) != 2 ||
			echo.Contract.ComboLegs[0].ConID != lower.ConID || echo.Contract.ComboLegs[0].Action != ibkr.ActionBuy ||
			echo.Contract.ComboLegs[1].ConID != upper.ConID || echo.Contract.ComboLegs[1].Action != ibkr.ActionSell {
			return fmt.Errorf("AAPL vertical contract echo = %+v, want qualified buy/sell legs", echo.Contract.ComboLegs)
		}
		if echo.Order.Prices.LmtPrice != nil || len(echo.Order.Combo.LegPrices) != 2 ||
			echo.Order.Combo.LegPrices[0] == nil || !echo.Order.Combo.LegPrices[0].Equal(lowerPrice) ||
			echo.Order.Combo.LegPrices[1] == nil || !echo.Order.Combo.LegPrices[1].Equal(upperPrice) ||
			len(echo.Order.Combo.SmartRouting) != 1 || echo.Order.Combo.SmartRouting[0] != (ibkr.TagValue{Tag: "NonGuaranteed", Value: "1"}) {
			return fmt.Errorf("AAPL vertical order echo lmt=%s combo=%+v, want bounded price and per-leg routing", echo.Order.Prices.LmtPrice, echo.Order.Combo)
		}
		recordAPIEvent("combo_echo", "option vertical BAG", func(event *apiDriverEvent) {
			event.OrderID = handle.OrderID()
			event.Values = map[string]string{
				"lower_con_id": strconv.FormatInt(int64(lower.ConID), 10),
				"upper_con_id": strconv.FormatInt(int64(upper.ConID), 10),
				"lower_strike": lower.Strike.String(),
				"upper_strike": upper.Strike.String(),
				"width":        width.String(),
				"net_debit":    netDebit.String(),
				"lower_price":  echo.Order.Combo.LegPrices[0].String(),
				"upper_price":  echo.Order.Combo.LegPrices[1].String(),
			}
		})
		cancelOrder(ctx, client, account, handle, "option vertical BAG")
		observation := observeOrder(ctx, handle, "option vertical BAG cancel", 30*time.Second)
		if !observation.terminal || observation.AnyFill() {
			return fmt.Errorf("AAPL vertical cleanup status=%s filled=%s, want terminal zero fill", observation.lastStatus, observation.filledQty)
		}
		openOrders, err := client.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
		if err != nil {
			return fmt.Errorf("AAPL vertical open-order reconciliation: %w", err)
		}
		if len(openOrders) != 0 {
			return fmt.Errorf("AAPL vertical cleanup left %d open orders", len(openOrders))
		}
		return fenceAPIWrites(ctx, client, "option vertical cleanup")
	})
}

func runAPIAlgorithmicCampaignAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 7*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) (runErr error) {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("algorithmic campaign anchor=%s", anchor)
		if _, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{AccountFilter: account, Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower", "ExcessLiquidity"}}); err != nil {
			return fmt.Errorf("algorithmic account summary: %w", err)
		}
		initialPositions, err := client.Accounts().Positions(ctx)
		if err != nil {
			return fmt.Errorf("algorithmic initial positions: %w", err)
		}
		baselineExecutions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"})
		if err != nil {
			return fmt.Errorf("algorithmic execution baseline: %w", err)
		}
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return fmt.Errorf("algorithmic delayed market data: %w", err)
		}

		var observerClosers []func() error
		closeObservers := func() error {
			var err error
			for i := len(observerClosers) - 1; i >= 0; i-- {
				err = errors.Join(err, observerClosers[i]())
			}
			observerClosers = nil
			return err
		}
		defer func() { runErr = errors.Join(runErr, closeObservers()) }()
		quotes, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: apiAAPL}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("algorithmic quote observer: %w", err)
		}
		quotesDone := drainObserver(quotes)
		observerClosers = append(observerClosers, func() error {
			quotes.Close()
			return <-quotesDone
		})
		updates, err := client.Accounts().SubscribeUpdates(ctx, account,
			ibkr.WithQueueSize(512), ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("algorithmic account observer: %w", err)
		}
		updatesDone := drainObserver(updates)
		observerClosers = append(observerClosers, func() error {
			updates.Close()
			return <-updatesDone
		})
		pnl, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: account}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("algorithmic PnL observer: %w", err)
		}
		pnlDone := drainObserver(pnl)
		observerClosers = append(observerClosers, func() error {
			pnl.Close()
			return <-pnlDone
		})
		openOrders, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return fmt.Errorf("algorithmic open-order observer: %w", err)
		}
		openOrdersDone := drainObserver(openOrders.Subscription)
		observerClosers = append(observerClosers, func() error {
			openOrders.Close()
			return <-openOrdersDone
		})

		filledQty := decimal.Zero
		for i := 0; i < 2; i++ {
			order := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
			handle, err := placeAPIOrder(ctx, client, fmt.Sprintf("algorithmic split buy[%d]", i), apiAAPL, order)
			if err != nil {
				return fmt.Errorf("algorithmic split buy[%d]: %w", i, err)
			}
			obs := observeOrder(ctx, handle, fmt.Sprintf("algorithmic split buy[%d]", i), 30*time.Second)
			if !obs.FullFill() || !obs.sawExecution || !obs.filledQty.Equal(apiStockCampaignOrderQuantity) {
				if !handleDone(handle) {
					cancelOrder(ctx, client, account, handle, fmt.Sprintf("algorithmic split buy[%d] incomplete", i))
					_ = observeOrder(ctx, handle, fmt.Sprintf("algorithmic split buy[%d] incomplete cancel", i), 10*time.Second)
				}
				return fmt.Errorf("algorithmic split buy[%d] status=%s filled=%s execution=%t, want terminal fill", i, obs.lastStatus, obs.filledQty, obs.sawExecution)
			}
			filledQty = filledQty.Add(obs.filledQty)
		}

		restingOrder := withLimit(baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))
		resting, err := placeAPIOrder(ctx, client, "algorithmic resting buy", apiAAPL, restingOrder)
		if err != nil {
			return fmt.Errorf("algorithmic resting buy: %w", err)
		}
		if _, err := awaitOpenOrderEvidence(ctx, resting, "algorithmic resting buy", 20*time.Second); err != nil {
			return err
		}
		modified := baseAPIOrder(account, restingOrder.Quantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
		if err := modifyAPIOrder(ctx, client, resting, "algorithmic resting modified", modified); err != nil {
			return fmt.Errorf("algorithmic resting modify: %w", err)
		}
		modifiedObservation := observeOrder(ctx, resting, "algorithmic resting modified", 30*time.Second)
		if !modifiedObservation.FullFill() || !modifiedObservation.sawExecution || !modifiedObservation.filledQty.Equal(apiStockCampaignOrderQuantity) {
			if !handleDone(resting) {
				cancelOrder(ctx, client, account, resting, "algorithmic resting modified incomplete")
				_ = observeOrder(ctx, resting, "algorithmic resting modified incomplete cancel", 10*time.Second)
			}
			return fmt.Errorf("algorithmic modified order status=%s filled=%s execution=%t, want terminal fill", modifiedObservation.lastStatus, modifiedObservation.filledQty, modifiedObservation.sawExecution)
		}
		filledQty = filledQty.Add(modifiedObservation.filledQty)

		if !filledQty.Equal(decimal.NewFromInt(3)) {
			return fmt.Errorf("algorithmic entry quantity=%s, want 3", filledQty)
		}
		if err := flattenAAPL(ctx, client, account, "algorithmic flatten", filledQty); err != nil {
			return errors.Join(err, fenceAPIWrites(ctx, client, "algorithmic failed flatten"))
		}
		if err := closeObservers(); err != nil {
			return fmt.Errorf("algorithmic observers: %w", err)
		}

		executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"})
		if err != nil {
			return fmt.Errorf("algorithmic executions: %w", err)
		}
		if countNewExecutions(baselineExecutions, executions) != 4 {
			return fmt.Errorf("algorithmic new executions=%d, want 4", countNewExecutions(baselineExecutions, executions))
		}
		if err := verifyNewExecutionFees(baselineExecutions, executions); err != nil {
			return fmt.Errorf("algorithmic execution fees: %w", err)
		}
		positions, err := client.Accounts().Positions(ctx)
		if err != nil {
			return fmt.Errorf("algorithmic final positions: %w", err)
		}
		if !positionQuantity(positions, apiAAPL.ConID).Equal(positionQuantity(initialPositions, apiAAPL.ConID)) {
			return fmt.Errorf("algorithmic AAPL position changed from %s to %s after flatten", positionQuantity(initialPositions, apiAAPL.ConID), positionQuantity(positions, apiAAPL.ConID))
		}
		completed, err := client.Orders().Completed(ctx, true)
		if err != nil {
			return fmt.Errorf("algorithmic completed orders: %w", err)
		}
		if len(completed) < 4 {
			return fmt.Errorf("algorithmic completed orders=%d, want at least four campaign entries", len(completed))
		}
		recordAPIEvent("completed_orders_query", "algorithmic completed orders", func(event *apiDriverEvent) {
			event.Count = len(completed)
			event.Values = map[string]string{"api_only": "true"}
		})
		return fenceAPIWrites(ctx, client, "algorithmic cleanup")
	})
}

func runAPICompletedOrdersVariantsAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		order := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
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
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		order := withTransmit(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), false)
		handle, err := placeAPIOrder(ctx, client, "transmit false resting", apiAAPL, order)
		if err != nil {
			log.Printf("transmit false place: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "transmit false resting", 10*time.Second)

		order.Transmit = new(true)
		if err := modifyAPIOrder(ctx, client, handle, "transmit true modify", order); err != nil {
			log.Printf("transmit true modify: %v", err)
		}
		_ = observeOrder(ctx, handle, "transmit true modify", 10*time.Second)
		if !handleDone(handle) {
			cancelOrder(ctx, client, account, handle, "transmit true")
			_ = observeOrder(ctx, handle, "transmit true cancel", 10*time.Second)
		}
		return nil
	})
}

func runAPIEmptyTIFDefaultAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("300"))
		// Public constructors leave TIF empty. This scenario records what the
		// Gateway does with that request shape instead of assuming a default.
		order := ibkr.LimitOrder(ibkr.ActionBuy, apiStockOrderQuantity, farBuy(anchor))
		order.Account = account
		order.OrderRef = apiOrderRef("capture")
		handle, err := placeAPIOrder(ctx, client, "empty tif placement", apiAAPL, order)
		if err != nil {
			return fmt.Errorf("place empty tif: %w", err)
		}
		echo, warning, err := awaitOpenOrderEvidenceAndWarning(ctx, handle, "empty tif placement", 20*time.Second)
		if err != nil {
			rejection, ok := errors.AsType[*ibkr.APIError](err)
			if !ok {
				rejection = warning
			}
			if rejection == nil || rejection.Code != 10052 {
				return fmt.Errorf("empty tif placement produced neither an open-order echo nor exact code 10052: %w", err)
			}
			recordAPIEvent("empty_tif_rejected", "placement", func(event *apiDriverEvent) {
				event.OrderID = handle.OrderID()
				event.Values = map[string]string{
					"code":    strconv.Itoa(rejection.Code),
					"message": rejection.Message,
				}
			})
			handle.Close()
			return fenceAPIWrites(ctx, client, "empty tif rejection cleanup")
		}
		if echo.Order.TIF != ibkr.TIFDay {
			return fmt.Errorf("empty tif echo = %q, want DAY", echo.Order.TIF)
		}
		recordAPIEvent("empty_tif_echo", "placement", func(event *apiDriverEvent) {
			event.OrderID = handle.OrderID()
			event.Values = map[string]string{"requested": "", "tif": string(echo.Order.TIF)}
		})
		cancelOrder(ctx, client, account, handle, "empty tif placement")
		observation := observeOrder(ctx, handle, "empty tif placement cancel", 20*time.Second)
		if !observation.terminal {
			return fmt.Errorf("empty tif order %d did not reach a terminal state after targeted cancel; last status %q", handle.OrderID(), observation.lastStatus)
		}
		if observation.AnyFill() {
			return fmt.Errorf("nonmarketable empty tif order %d unexpectedly filled %s", handle.OrderID(), observation.filledQty)
		}
		return fenceAPIWrites(ctx, client, "empty tif placement cleanup")
	})
}

func runAPIIncludeOvernightLifecycleAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 5*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("300"))
		order := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))
		order.IncludeOvernight = new(true)
		handle, err := placeAPIOrder(ctx, client, "include overnight true", apiAAPL, order)
		if err != nil {
			return fmt.Errorf("place include overnight true: %w", err)
		}
		trueEcho, err := awaitOpenOrderEvidence(ctx, handle, "include overnight true", 20*time.Second)
		if err != nil {
			return err
		}
		if trueEcho.Order.IncludeOvernight == nil || !*trueEcho.Order.IncludeOvernight {
			return fmt.Errorf("include overnight placement echo = %v, want explicit true", trueEcho.Order.IncludeOvernight)
		}
		recordAPIEvent("include_overnight_echo", "placement", func(event *apiDriverEvent) {
			event.OrderID = handle.OrderID()
			event.Values = map[string]string{"include_overnight": "true"}
		})

		order.IncludeOvernight = new(false)
		if err := modifyAPIOrder(ctx, client, handle, "include overnight false", order); err != nil {
			return fmt.Errorf("replace include overnight false: %w", err)
		}
		falseEcho, warning, err := awaitOpenOrderEvidenceAndWarning(ctx, handle, "include overnight false", 20*time.Second)
		if err != nil {
			return err
		}
		if falseEcho.Order.IncludeOvernight != nil && !*falseEcho.Order.IncludeOvernight {
			recordAPIEvent("include_overnight_echo", "replacement", func(event *apiDriverEvent) {
				event.OrderID = handle.OrderID()
				event.Values = map[string]string{"include_overnight": "false"}
			})
		} else if falseEcho.Order.IncludeOvernight != nil && *falseEcho.Order.IncludeOvernight {
			if warning == nil {
				warning, err = awaitOrderWarning(ctx, handle, "include overnight false", 5*time.Second)
				if err != nil {
					return fmt.Errorf("include overnight replacement retained true without blocker evidence: %w", err)
				}
			}
			if warning.Code != 462 || !strings.Contains(warning.Message, "Cannot change to the new Time in Force.DAY") {
				return fmt.Errorf("include overnight replacement warning = code %d %q, want exact code-462 TIF blocker", warning.Code, warning.Message)
			}
			recordAPIEvent("include_overnight_blocked", "replacement", func(event *apiDriverEvent) {
				event.OrderID = handle.OrderID()
				event.Values = map[string]string{
					"code":               strconv.Itoa(warning.Code),
					"message":            warning.Message,
					"retained_overnight": "true",
				}
			})
		} else {
			return fmt.Errorf("include overnight replacement echo = %v, want explicit false or retained true with exact code-462 blocker", falseEcho.Order.IncludeOvernight)
		}

		cancelOrder(ctx, client, account, handle, "include overnight false")
		observation := observeOrder(ctx, handle, "include overnight false cancel", 20*time.Second)
		if !observation.terminal {
			return fmt.Errorf("include overnight order %d did not reach a terminal state after targeted cancel; last status %q", handle.OrderID(), observation.lastStatus)
		}
		if observation.AnyFill() {
			return fmt.Errorf("nonmarketable include overnight order %d unexpectedly filled %s", handle.OrderID(), observation.filledQty)
		}
		if _, err := client.CurrentTime(ctx); err != nil {
			return fmt.Errorf("include overnight cleanup fence: %w", err)
		}

		falseOrder := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))
		falseOrder.IncludeOvernight = new(false)
		falseHandle, err := placeAPIOrder(ctx, client, "include overnight false fresh placement", apiAAPL, falseOrder)
		if err != nil {
			return fmt.Errorf("place include overnight false: %w", err)
		}
		falsePlacementEcho, err := awaitOpenOrderEvidence(ctx, falseHandle, "include overnight false fresh placement", 20*time.Second)
		if err != nil {
			return err
		}
		if falsePlacementEcho.Order.IncludeOvernight != nil && *falsePlacementEcho.Order.IncludeOvernight {
			return fmt.Errorf("fresh include overnight false echo = true, want false or broker-canonical absence")
		}
		if falsePlacementEcho.Order.TIF != ibkr.TIFDay {
			return fmt.Errorf("fresh include overnight false TIF = %q, want DAY", falsePlacementEcho.Order.TIF)
		}
		falseEchoValue := "absent"
		if falsePlacementEcho.Order.IncludeOvernight != nil {
			falseEchoValue = "false"
		}
		recordAPIEvent("include_overnight_echo", "fresh placement", func(event *apiDriverEvent) {
			event.OrderID = falseHandle.OrderID()
			event.Values = map[string]string{
				"include_overnight": falseEchoValue,
				"requested":         "false",
				"tif":               string(falsePlacementEcho.Order.TIF),
			}
		})
		cancelOrder(ctx, client, account, falseHandle, "include overnight false fresh placement")
		falseObservation := observeOrder(ctx, falseHandle, "include overnight false fresh placement cancel", 20*time.Second)
		if !falseObservation.terminal {
			return fmt.Errorf("fresh include overnight false order %d did not reach a terminal state; last status %q", falseHandle.OrderID(), falseObservation.lastStatus)
		}
		if falseObservation.AnyFill() {
			return fmt.Errorf("nonmarketable fresh include overnight false order %d unexpectedly filled %s", falseHandle.OrderID(), falseObservation.filledQty)
		}
		return fenceAPIWrites(ctx, client, "include overnight false fresh placement cleanup")
	})
}

func positionInventory(positions []ibkr.Position) map[ibkr.ContractID]string {
	inventory := make(map[ibkr.ContractID]string, len(positions))
	for _, position := range positions {
		if position.Position.IsZero() {
			continue
		}
		inventory[position.Contract.ConID] = position.Position.String()
	}
	return inventory
}

func samePositionInventory(left, right []ibkr.Position) bool {
	leftInventory := positionInventory(left)
	rightInventory := positionInventory(right)
	if len(leftInventory) != len(rightInventory) {
		return false
	}
	for contractID, quantity := range leftInventory {
		if rightInventory[contractID] != quantity {
			return false
		}
	}
	return true
}

type accountValueIdentity struct {
	account  string
	tag      string
	currency string
}

func accountValueIdentities(values []ibkr.AccountValue) map[accountValueIdentity]struct{} {
	identities := make(map[accountValueIdentity]struct{}, len(values))
	for _, value := range values {
		identities[accountValueIdentity{account: value.Account, tag: value.Tag, currency: value.Currency}] = struct{}{}
	}
	return identities
}

func sameAccountValueIdentities(left, right []ibkr.AccountValue) bool {
	return maps.Equal(accountValueIdentities(left), accountValueIdentities(right))
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
		first.Close()
		if second != nil {
			second.Close()
		}
		if err := first.Wait(); err != nil {
			return fmt.Errorf("wait for first duplicate quote subscription: %w", err)
		}
		if second != nil {
			if err := second.Wait(); err != nil {
				return fmt.Errorf("wait for second duplicate quote subscription: %w", err)
			}
		}
		return fenceAPIWrites(ctx, client, "duplicate quote cancellations")
	})
}

func runAPIReconnectActiveOrderAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, first *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, first, apiAAPL, decimal.RequireFromString("200"))
		order := withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTC)
		handle, err := placeAPIOrder(ctx, first, "reconnect resting", apiAAPL, order)
		if err != nil {
			return fmt.Errorf("place reconnect resting order: %w", err)
		}
		_ = observeOrder(ctx, handle, "reconnect resting", 10*time.Second)
		orderID := handle.OrderID()
		recordAPIEvent("disconnect_client", "reconnect resting", func(event *apiDriverEvent) {
			event.OrderID = orderID
		})
		first.Close()
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
		cancelErr := guardedCancelOrder(ctx, second, account, clientID, orderID, "reconnect direct cancel")
		if cancelErr != nil {
			recordAPIEvent("direct_cancel_error", "reconnect resting", func(event *apiDriverEvent) {
				event.OrderID = orderID
				event.Error = cancelErr.Error()
			})
			log.Printf("reconnect direct cancel: %v", cancelErr)
		} else {
			recordAPIEvent("direct_cancel_sent", "reconnect resting", func(event *apiDriverEvent) {
				event.OrderID = orderID
			})
		}
		if err := fenceAPIWrites(ctx, second, "reconnect direct cancel"); err != nil {
			return err
		}
		orders, err = second.Orders().Open(ctx, ibkr.OpenOrdersScopeClient)
		recordOpenOrdersResult("reconnect post-cancel open client", orders, err)
		if err != nil {
			return err
		}
		if cancelErr == nil && containsOrderID(orders, orderID) {
			return fmt.Errorf("reconnect order %d remained open after successful direct cancel", orderID)
		}
		return nil
	})
}

func runAPIClientID0OrderObservationAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, placer *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, placer, apiAAPL, decimal.RequireFromString("200"))
		order := withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTC)
		handle, err := placeAPIOrder(ctx, placer, "client0 observed resting", apiAAPL, order)
		if err != nil {
			return err
		}
		_ = observeOrder(ctx, handle, "client0 observed resting", 10*time.Second)
		orderID := handle.OrderID()
		placer.Close()
		time.Sleep(500 * time.Millisecond)

		observer, err := dialAPI(ctx, addr, 0)
		if err != nil {
			return err
		}
		defer observer.Close()
		recordSessionReady(addr, 0, account, observer)
		orders, err := observer.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
		recordOpenOrdersResult("client0 all open", orders, err)
		cancelErr := guardedCancelOrder(ctx, observer, account, clientID, orderID, "client0 direct cancel")
		if cancelErr != nil {
			recordAPIEvent("direct_cancel_error", "client0 observed resting", func(event *apiDriverEvent) {
				event.OrderID = orderID
				event.Error = cancelErr.Error()
			})
			log.Printf("client0 direct cancel: %v", cancelErr)
		} else {
			recordAPIEvent("direct_cancel_sent", "client0 observed resting", func(event *apiDriverEvent) {
				event.OrderID = orderID
			})
		}
		if err := fenceAPIWrites(ctx, observer, "client0 direct cancel"); err != nil {
			return err
		}
		orders, err = observer.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
		recordOpenOrdersResult("client0 post-cancel all open", orders, err)
		if err != nil {
			return err
		}
		if cancelErr == nil && containsOrderID(orders, orderID) {
			return fmt.Errorf("client0 observed order %d remained open after successful direct cancel", orderID)
		}
		return nil
	})
}

func runAPICrossClientCancelAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, placer *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, placer, apiAAPL, decimal.RequireFromString("200"))
		order := withTIF(withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)), ibkr.TIFGTC)
		handle, err := placeAPIOrder(ctx, placer, "cross-client resting", apiAAPL, order)
		if err != nil {
			return err
		}
		_ = observeOrder(ctx, handle, "cross-client resting", 10*time.Second)
		orderID := handle.OrderID()
		placer.Close()
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
		cancelErr := guardedCancelOrder(ctx, canceller, account, clientID, orderID, "cross-client direct cancel")
		if cancelErr != nil {
			recordAPIEvent("direct_cancel_error", "cross-client resting", func(event *apiDriverEvent) {
				event.ClientID = cancellerID
				event.OrderID = orderID
				event.Error = cancelErr.Error()
			})
			log.Printf("cross-client direct cancel: %v", cancelErr)
		} else {
			recordAPIEvent("direct_cancel_sent", "cross-client resting", func(event *apiDriverEvent) {
				event.ClientID = cancellerID
				event.OrderID = orderID
			})
		}
		if err := fenceAPIWrites(ctx, canceller, "cross-client direct cancel"); err != nil {
			return err
		}
		orders, err = canceller.Orders().Open(ctx, ibkr.OpenOrdersScopeAll)
		recordOpenOrdersResult("cross-client post-cancel all open", orders, err)
		if err != nil {
			return err
		}
		if cancelErr == nil && containsOrderID(orders, orderID) {
			return fmt.Errorf("cross-client order %d remained open after successful direct cancel", orderID)
		}
		return nil
	})
}

func containsOrderID(orders []ibkr.OpenOrder, orderID int64) bool {
	for _, order := range orders {
		if order.Order.OrderID != nil && *order.Order.OrderID == orderID {
			return true
		}
	}
	return false
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

func placeAPIOrder(ctx context.Context, client *ibkr.Client, label string, contract ibkr.Contract, order ibkr.Order) (*ibkr.OrderHandle, error) {
	if err := requirePaperTradingSession(client, order.Account, label+" place order"); err != nil {
		return nil, err
	}
	recordAPIEvent("place_order_start", label, func(event *apiDriverEvent) {
		event.Account = order.Account
		event.OrderRef = order.OrderRef
		event.Symbol = contract.Symbol
		event.SecType = string(contract.SecType)
		event.Action = string(order.Action)
		event.OrderType = string(order.OrderType)
		event.TIF = string(order.TIF)
		event.Quantity = order.Quantity.String()
		setRecordedOrderPrices(event, order.LmtPrice, order.AuxPrice)
		event.ParentID = order.ParentID
		event.OCAGroup = order.OCA.Group
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

func modifyAPIOrder(ctx context.Context, client *ibkr.Client, handle *ibkr.OrderHandle, label string, order ibkr.Order) error {
	if err := requirePaperTradingSession(client, order.Account, label+" replace order"); err != nil {
		return err
	}
	recordAPIEvent("modify_order_start", label, func(event *apiDriverEvent) {
		event.OrderID = handle.OrderID()
		event.Account = order.Account
		event.Action = string(order.Action)
		event.OrderType = string(order.OrderType)
		event.TIF = string(order.TIF)
		event.Quantity = order.Quantity.String()
		setRecordedOrderPrices(event, order.LmtPrice, order.AuxPrice)
		event.ParentID = order.ParentID
	})
	if err := handle.Replace(ctx, order); err != nil {
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
	order.LmtPrice = new(price.Round(2))
	return order
}

func withAux(order ibkr.Order, price decimal.Decimal) ibkr.Order {
	order.AuxPrice = new(price.Round(2))
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
	order.OCA = ibkr.OrderOCA{Group: group, Type: ibkr.OCACancelWithBlock}
	return order
}

func withTrailing(order ibkr.Order, anchor decimal.Decimal) ibkr.Order {
	order.TrailStopPrice = new(farSell(anchor))
	order.AuxPrice = new(decimal.RequireFromString("1"))
	return order
}

func withTrailingLimit(order ibkr.Order, anchor decimal.Decimal) ibkr.Order {
	order.TrailStopPrice = new(farSell(anchor))
	order.AuxPrice = new(decimal.RequireFromString("1"))
	order.LmtPriceOffset = new(decimal.RequireFromString("0.05"))
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

func withMinQty(order ibkr.Order, quantity decimal.Decimal, minQty int) ibkr.Order {
	order.Quantity = quantity
	order.MinQty = new(minQty)
	return order
}

func withPercentOffset(order ibkr.Order, percent decimal.Decimal) ibkr.Order {
	order.PercentOffset = new(percent)
	return order
}

func withTrailingPercent(order ibkr.Order, anchor decimal.Decimal, percent decimal.Decimal) ibkr.Order {
	order.TrailStopPrice = new(farSell(anchor))
	order.TrailingPercent = new(percent)
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
	order.Scale.InitialLevelSize = 1
	order.Scale.SubsequentLevelSize = 1
	order.Scale.PriceIncrement = decimal.RequireFromString("0.05")
	return order
}

func withActiveWindow(order ibkr.Order, start string, stop string) ibkr.Order {
	order.Scale.ActiveStartTime = start
	order.Scale.ActiveStopTime = stop
	return order
}

func withPriceManagement(order ibkr.Order) ibkr.Order {
	order.UsePriceMgmtAlgo = new(true)
	return order
}

func withAdjustedStop(order ibkr.Order, anchor decimal.Decimal) ibkr.Order {
	order.Adjustment = ibkr.OrderAdjustment{
		OrderType:      ibkr.OrderTypeStopLimit,
		TriggerPrice:   farBuy(anchor),
		StopPrice:      farBuy(anchor).Sub(decimal.NewFromInt(1)),
		StopLimitPrice: farBuy(anchor).Sub(decimal.RequireFromString("0.50")),
		TrailingAmount: decimal.RequireFromString("1"),
	}
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
	order.Algorithm = ibkr.OrderAlgorithm{
		Strategy: strategy,
		Params:   append([]ibkr.TagValue(nil), params...),
	}
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
		case event, ok := <-sub.Events():
			if !ok {
				return count
			}
			if event.Kind != ibkr.StreamData {
				continue
			}
			count++
			bar := event.Value
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
		case event, ok := <-sub.Events():
			if !ok {
				return count
			}
			if event.Kind != ibkr.StreamData {
				continue
			}
			count++
			tick := event.Value
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
		case event, ok := <-sub.Events():
			if !ok {
				recordProbeResult("quote_subscription", label, count, nil)
				return count
			}
			if event.Kind != ibkr.StreamData {
				continue
			}
			count++
			quote := event.Value
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

func awaitOpenOrderEvidence(ctx context.Context, handle *ibkr.OrderHandle, label string, wait time.Duration) (ibkr.OpenOrder, error) {
	openOrder, _, err := awaitOpenOrderEvidenceAndWarning(ctx, handle, label, wait)
	return openOrder, err
}

func awaitOpenOrderEvidenceAndWarning(ctx context.Context, handle *ibkr.OrderHandle, label string, wait time.Duration) (ibkr.OpenOrder, *ibkr.APIError, error) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	var warning *ibkr.APIError
	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				if err := handle.Wait(); err != nil {
					return ibkr.OpenOrder{}, warning, fmt.Errorf("%s closed before open-order evidence: %w", label, err)
				}
				return ibkr.OpenOrder{}, warning, fmt.Errorf("%s closed before open-order evidence", label)
			}
			logOrderEvent(label, event)
			recordOrderEvent(label, event)
			if event.Warning != nil {
				warning = event.Warning
			}
			if event.OpenOrder != nil {
				return *event.OpenOrder, warning, nil
			}
		case <-handle.Done():
			if err := handle.Wait(); err != nil {
				return ibkr.OpenOrder{}, warning, fmt.Errorf("%s ended before open-order evidence: %w", label, err)
			}
			return ibkr.OpenOrder{}, warning, fmt.Errorf("%s ended before open-order evidence", label)
		case <-timer.C:
			return ibkr.OpenOrder{}, warning, fmt.Errorf("%s produced no open-order evidence within %s", label, wait)
		case <-ctx.Done():
			return ibkr.OpenOrder{}, warning, ctx.Err()
		}
	}
}

func awaitOrderWarning(ctx context.Context, handle *ibkr.OrderHandle, label string, wait time.Duration) (*ibkr.APIError, error) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				return nil, fmt.Errorf("%s closed before warning evidence: %w", label, handle.Wait())
			}
			logOrderEvent(label, event)
			recordOrderEvent(label, event)
			if event.Warning != nil {
				return event.Warning, nil
			}
		case <-handle.Done():
			return nil, fmt.Errorf("%s ended before warning evidence: %w", label, handle.Wait())
		case <-timer.C:
			return nil, fmt.Errorf("%s produced no warning evidence within %s", label, wait)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
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
	finish := func() {
		if err := handle.Wait(); err != nil {
			log.Printf("%s handle done error: %v", label, err)
			if _, ok := errors.AsType[*ibkr.APIError](err); ok {
				obs.terminal = true
			}
		}
	}
	for {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				finish()
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
			finish()
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
		log.Printf("%s open_order order_id=%d type=%s action=%s status=%s lmt=%s aux=%s parent=%d oca=%s",
			label, (*evt.OpenOrder.Order.OrderID), evt.OpenOrder.Order.OrderType, evt.OpenOrder.Order.Action, evt.OpenOrder.State.Status, evt.OpenOrder.Order.Prices.LmtPrice, evt.OpenOrder.Order.Prices.AuxPrice, (*evt.OpenOrder.Order.ParentID), evt.OpenOrder.Order.OCA.Group)
	}
	if evt.Status != nil {
		log.Printf("%s status order_id=%d status=%s filled=%s remaining=%s avg=%s last=%s why_held=%s",
			label, evt.Status.OrderID, evt.Status.Status, evt.Status.Filled, evt.Status.Remaining, evt.Status.AvgFillPrice, evt.Status.LastFillPrice, evt.Status.WhyHeld)
	}
	if evt.Execution != nil {
		log.Printf("%s execution order_id=%d exec_id=%s side=%s shares=%s price=%s time=%s",
			label, evt.Execution.OrderID, evt.Execution.ExecID, evt.Execution.Side, evt.Execution.Shares, evt.Execution.Price, evt.Execution.Time.Format(time.RFC3339))
	}
	if evt.CommissionAndFees != nil {
		log.Printf("%s commission exec_id=%s commission=%s currency=%s pnl=%s",
			label, evt.CommissionAndFees.ExecID, evt.CommissionAndFees.Amount, evt.CommissionAndFees.Currency, evt.CommissionAndFees.RealizedPnL)
	}
	if evt.Warning != nil {
		log.Printf("%s warning code=%d message=%s", label, evt.Warning.Code, evt.Warning.Message)
	}
}

func recordOrderEvent(label string, evt ibkr.OrderEvent) {
	if evt.OpenOrder != nil {
		recordAPIEvent("open_order", label, func(event *apiDriverEvent) {
			order := evt.OpenOrder
			event.OrderID = *order.Order.OrderID
			event.Account = order.Order.Account
			event.Symbol = order.Contract.Symbol
			event.SecType = string(order.Contract.SecType)
			event.Action = string(order.Order.Action)
			event.OrderType = string(order.Order.OrderType)
			event.TIF = string(order.Order.TIF)
			event.Quantity = order.Order.Quantity.String()
			setRecordedOrderPrices(event, order.Order.Prices.LmtPrice, order.Order.Prices.AuxPrice)
			event.Status = string(order.State.Status)
			event.ParentID = *order.Order.ParentID
			event.OCAGroup = order.Order.OCA.Group
			event.OrderRef = order.Order.OrderRef
			event.PermID = *order.Order.PermID
			event.Submitter = order.Order.Compliance.Submitter
			if len(order.Order.Algorithm.Params) > 0 {
				event.Values = tagValuesMap(order.Order.Algorithm.Params)
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
			event.Symbol = exec.Contract.Symbol
			event.Side = string(exec.Side)
			event.Quantity = exec.Shares.String()
			event.Price = exec.Price.String()
			event.EventTime = exec.Time.Format(time.RFC3339)
		})
	}
	if evt.CommissionAndFees != nil {
		recordAPIEvent("commission", label, func(event *apiDriverEvent) {
			commission := evt.CommissionAndFees
			event.ExecID = commission.ExecID
			event.Commission = optionalDecimalString(commission.Amount)
			event.Currency = commission.Currency
			event.RealizedPNL = optionalDecimalString(commission.RealizedPnL)
		})
	}
	if evt.Warning != nil {
		recordAPIEvent("order_warning", label, func(event *apiDriverEvent) {
			event.OrderID = int64(evt.Warning.RequestID)
			event.Values = map[string]string{
				"code":    strconv.Itoa(evt.Warning.Code),
				"message": evt.Warning.Message,
			}
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

func cancelOrder(ctx context.Context, client *ibkr.Client, account string, handle *ibkr.OrderHandle, label string) {
	if err := requirePaperTradingSession(client, account, label+" cancel order"); err != nil {
		log.Printf("%s cancel refused: %v", label, err)
		recordAPIEvent("cancel_order_error", label, func(event *apiDriverEvent) {
			event.Error = err.Error()
		})
		return
	}
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

type scenarioOrder struct {
	label    string
	handle   *ibkr.OrderHandle
	terminal bool
}

type paperCampaignBaseline struct {
	positions     []ibkr.Position
	executions    ibkr.ExecutionSnapshot
	accountValues []ibkr.AccountValue
}

type paperReconciliation struct {
	openOrders    string
	positions     string
	executions    string
	accountValues string
}

func snapshotPaperCampaignBaseline(ctx context.Context, client *ibkr.Client, account string) (paperCampaignBaseline, error) {
	positions, err := snapshotPositions(ctx, client)
	if err != nil {
		return paperCampaignBaseline{}, fmt.Errorf("baseline positions: %w", err)
	}
	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: account})
	if err != nil {
		return paperCampaignBaseline{}, fmt.Errorf("baseline executions: %w", err)
	}
	accountValues, err := client.Accounts().Summary(ctx, ibkr.AccountSummaryRequest{
		Group: "All", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil {
		return paperCampaignBaseline{}, fmt.Errorf("baseline account values: %w", err)
	}
	return paperCampaignBaseline{positions: positions, executions: executions, accountValues: accountValues}, nil
}

func recordPaperBaseline(label string, baseline paperCampaignBaseline, clearedOpenOrders int) {
	recordAPIEvent("paper_baseline", label, func(event *apiDriverEvent) {
		event.Values = map[string]string{
			"cleared_open_orders": strconv.Itoa(clearedOpenOrders),
			"positions":           strconv.Itoa(len(baseline.positions)),
			"executions":          strconv.Itoa(len(baseline.executions.Executions)),
			"account_values":      strconv.Itoa(len(baseline.accountValues)),
		}
	})
}

func reconcilePaperCampaign(ctx context.Context, client *ibkr.Client, account, label string, baseline paperCampaignBaseline) (paperReconciliation, error) {
	workCtx, cancelWork := context.WithTimeout(ctx, 5*time.Minute)
	var cleanupErr error
	flattenSafe := true
	if _, err := clearPaperOpenOrders(workCtx, client, account, label+" pre-flatten cleanup"); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
		flattenSafe = false
	}
	var positions []ibkr.Position
	for pass := 1; flattenSafe && pass <= 3; pass++ {
		var err error
		positions, err = snapshotPositions(workCtx, client)
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s position snapshot pass %d: %w", label, pass, err))
			break
		}
		deltas := campaignPositionDeltas(baseline.positions, positions)
		if len(deltas) == 0 {
			break
		}
		for _, positionDelta := range deltas {
			contract := positionDelta.contract
			if contract.ConID == apiAAPL.ConID {
				contract = apiAAPL
			}
			action := ibkr.ActionSell
			if positionDelta.delta.IsNegative() {
				action = ibkr.ActionBuy
			}
			order := baseAPIOrder(account, positionDelta.delta.Abs(), action, ibkr.OrderTypeMarket)
			handle, placeErr := placeAPIOrder(workCtx, client, label+" delta flatten", contract, order)
			if placeErr != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s flatten contract %d delta %s on pass %d: %w", label, contract.ConID, positionDelta.delta, pass, placeErr))
				continue
			}
			observation := observeOrder(workCtx, handle, label+" delta flatten", 45*time.Second)
			if !observation.terminal {
				cancelOrder(workCtx, client, account, handle, label+" delta flatten")
				terminal := observeOrder(workCtx, handle, label+" delta flatten cleanup", 15*time.Second)
				observation.Merge(terminal)
				if !observation.terminal {
					cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s flatten contract %d delta %s on pass %d produced no terminal evidence", label, contract.ConID, positionDelta.delta, pass))
					if _, err := clearPaperOpenOrders(workCtx, client, account, fmt.Sprintf("%s uncertain flatten pass %d", label, pass)); err != nil {
						cleanupErr = errors.Join(cleanupErr, err)
						flattenSafe = false
						break
					}
				}
			}
		}
		if !flattenSafe {
			break
		}
		if err := fenceAPIWrites(workCtx, client, fmt.Sprintf("%s reconciliation pass %d", label, pass)); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			break
		}
	}
	cancelWork()

	// The work phase cannot consume the final two minutes of the wrapper's
	// seven-minute cleanup window. Always use that reserve to cancel, fence,
	// and report final broker state even when an earlier step failed.
	finalCtx, cancelFinal := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelFinal()
	reconciliation := paperReconciliation{
		openOrders:    "0",
		positions:     "unknown",
		executions:    "unknown",
		accountValues: "unknown",
	}
	if _, err := clearPaperOpenOrders(finalCtx, client, account, label+" final"); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
		reconciliation.openOrders = "unknown"
	}
	var err error
	positions, err = snapshotPositions(finalCtx, client)
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s final positions: %w", label, err))
	} else {
		reconciliation.positions = strconv.Itoa(len(positions))
		if !samePositionInventory(baseline.positions, positions) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s position inventory changed after three cleanup passes: before=%v after=%v", label, positionInventory(baseline.positions), positionInventory(positions)))
		}
	}
	executions, err := executionsWithReconciledFees(finalCtx, client, account, baseline.executions)
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s final execution reconciliation: %w", label, err))
	} else {
		reconciliation.executions = strconv.Itoa(len(executions.Executions))
	}
	accountValues, err := client.Accounts().Summary(finalCtx, ibkr.AccountSummaryRequest{
		Group: "All", Tags: []string{"NetLiquidation", "TotalCashValue", "BuyingPower"},
	})
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s final account values: %w", label, err))
	} else {
		reconciliation.accountValues = strconv.Itoa(len(accountValues))
		if !sameAccountValueIdentities(baseline.accountValues, accountValues) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s account-value identities changed: before=%v after=%v", label, accountValueIdentities(baseline.accountValues), accountValueIdentities(accountValues)))
		}
	}
	return reconciliation, cleanupErr
}

func recordPaperReconciliation(label string, reconciliation paperReconciliation, err error) {
	eventKind := "paper_reconciled"
	if err != nil {
		eventKind = "paper_reconciliation_failed"
	}
	recordAPIEvent(eventKind, label, func(event *apiDriverEvent) {
		event.Values = map[string]string{
			"open_orders":    reconciliation.openOrders,
			"positions":      reconciliation.positions,
			"executions":     reconciliation.executions,
			"account_values": reconciliation.accountValues,
		}
		if err != nil {
			event.Error = err.Error()
		}
	})
}

func clearPaperOpenOrders(ctx context.Context, client *ibkr.Client, account, label string) (int, error) {
	openOrders, err := snapshotOpenOrders(ctx, client)
	if err != nil {
		snapshotErr := fmt.Errorf("%s open-orders snapshot: %w", label, err)
		return 0, errors.Join(snapshotErr, globalCancelAndVerify(ctx, client, account, label+" unknown-state"))
	}
	cleared := len(openOrders)
	var cleanupErr error
	for _, openOrder := range openOrders {
		if openOrder.Order.OrderID == nil || openOrder.Order.ClientID == nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s cannot target an order with missing identity", label))
			continue
		}
		if err := guardedCancelOrder(ctx, client, account, int(*openOrder.Order.ClientID), *openOrder.Order.OrderID, label+" targeted cancel"); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s cancel order %d: %w", label, *openOrder.Order.OrderID, err))
		}
	}
	if len(openOrders) == 0 {
		return 0, nil
	}
	if err := fenceAPIWrites(ctx, client, label+" targeted cancel"); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	}
	openOrders, err = snapshotOpenOrders(ctx, client)
	if err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s verify targeted cancel: %w", label, err))
		return cleared, errors.Join(cleanupErr, globalCancelAndVerify(ctx, client, account, label+" uncertain"))
	}
	if len(openOrders) == 0 {
		return cleared, cleanupErr
	}
	if err := globalCancelAndVerify(ctx, client, account, label+" uncertain"); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s has %d working orders after targeted cancel: %w", label, len(openOrders), err))
	}
	return cleared, cleanupErr
}

func globalCancelAndVerify(ctx context.Context, client *ibkr.Client, account, label string) error {
	if err := guardedCancelAll(ctx, client, account, label+" global cancel"); err != nil {
		return err
	}
	if err := fenceAPIWrites(ctx, client, label+" global cancel"); err != nil {
		return err
	}
	openOrders, err := snapshotOpenOrders(ctx, client)
	if err != nil {
		return fmt.Errorf("%s verify global cancel: %w", label, err)
	}
	if len(openOrders) != 0 {
		return fmt.Errorf("%s has %d working orders after global cancel", label, len(openOrders))
	}
	return nil
}

func verifyNewExecutionFees(baseline, current ibkr.ExecutionSnapshot) error {
	known := make(map[string]struct{}, len(baseline.Executions))
	for _, execution := range baseline.Executions {
		known[execution.ExecID] = struct{}{}
	}
	fees := make(map[string]struct{}, len(current.CommissionAndFees))
	for _, fee := range current.CommissionAndFees {
		fees[fee.ExecID] = struct{}{}
	}
	missing := 0
	for _, execution := range current.Executions {
		if _, existed := known[execution.ExecID]; existed {
			continue
		}
		if _, ok := fees[execution.ExecID]; !ok {
			missing++
		}
	}
	if missing != 0 {
		return fmt.Errorf("%d new executions lack correlated commission-and-fees reports", missing)
	}
	return nil
}

func countNewExecutions(baseline, current ibkr.ExecutionSnapshot) int {
	known := make(map[string]struct{}, len(baseline.Executions))
	for _, execution := range baseline.Executions {
		known[execution.ExecID] = struct{}{}
	}
	var count int
	for _, execution := range current.Executions {
		if _, existed := known[execution.ExecID]; !existed {
			count++
		}
	}
	return count
}

func executionsWithReconciledFees(ctx context.Context, client *ibkr.Client, account string, baseline ibkr.ExecutionSnapshot) (ibkr.ExecutionSnapshot, error) {
	deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var snapshot ibkr.ExecutionSnapshot
	var reconcileErr error
	for {
		var err error
		snapshot, err = client.Orders().Executions(deadline, ibkr.ExecutionsRequest{Account: account})
		if err == nil {
			reconcileErr = verifyNewExecutionFees(baseline, snapshot)
			if reconcileErr == nil {
				return snapshot, nil
			}
		} else {
			reconcileErr = err
		}
		select {
		case <-deadline.Done():
			return snapshot, reconcileErr
		case <-ticker.C:
		}
	}
}

type campaignPositionDelta struct {
	contract ibkr.Contract
	delta    decimal.Decimal
}

func campaignPositionDeltas(baseline, current []ibkr.Position) []campaignPositionDelta {
	baselineByContract := make(map[ibkr.ContractID]ibkr.Position, len(baseline))
	currentByContract := make(map[ibkr.ContractID]ibkr.Position, len(current))
	for _, position := range baseline {
		baselineByContract[position.Contract.ConID] = position
	}
	for _, position := range current {
		currentByContract[position.Contract.ConID] = position
	}
	contractIDs := make(map[ibkr.ContractID]struct{}, len(baselineByContract)+len(currentByContract))
	for contractID := range baselineByContract {
		contractIDs[contractID] = struct{}{}
	}
	for contractID := range currentByContract {
		contractIDs[contractID] = struct{}{}
	}
	deltas := make([]campaignPositionDelta, 0, len(contractIDs))
	for contractID := range contractIDs {
		before := baselineByContract[contractID]
		after := currentByContract[contractID]
		delta := after.Position.Sub(before.Position)
		if delta.IsZero() {
			continue
		}
		contract := after.Contract
		if contract.ConID == 0 {
			contract = before.Contract
		}
		deltas = append(deltas, campaignPositionDelta{contract: contract, delta: delta})
	}
	sort.Slice(deltas, func(i, j int) bool { return deltas[i].contract.ConID < deltas[j].contract.ConID })
	return deltas
}

func snapshotPositions(ctx context.Context, client *ibkr.Client) ([]ibkr.Position, error) {
	positions, err := client.Accounts().Positions(ctx)
	if err != nil {
		return nil, err
	}
	byContract := make(map[ibkr.ContractID]ibkr.Position, len(positions))
	for _, position := range positions {
		if position.Position.IsZero() {
			delete(byContract, position.Contract.ConID)
			continue
		}
		byContract[position.Contract.ConID] = position
	}
	result := make([]ibkr.Position, 0, len(byContract))
	for _, position := range byContract {
		result = append(result, position)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Contract.ConID < result[j].Contract.ConID })
	return result, nil
}

func snapshotOpenOrders(ctx context.Context, client *ibkr.Client) ([]ibkr.OpenOrder, error) {
	sub, err := client.Orders().SubscribeOpen(ctx, ibkr.OpenOrdersScopeAll, ibkr.WithResumePolicy(ibkr.ResumeNever))
	if err != nil {
		return nil, err
	}
	defer func() {
		sub.Close()
		_ = sub.Wait()
	}()
	return readOpenOrdersSnapshot(ctx, sub)
}

func readOpenOrdersSnapshot(ctx context.Context, sub *ibkr.OpenOrdersSubscription) ([]ibkr.OpenOrder, error) {
	openByID := make(map[int64]ibkr.OpenOrder)
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				return nil, sub.Wait()
			}
			if event.Err != nil {
				return nil, event.Err
			}
			switch event.Kind {
			case ibkr.StreamData:
				switch {
				case event.Value.Order != nil:
					orderID := event.Value.Order.Order.OrderID
					if orderID == nil {
						return nil, errors.New("open-order snapshot returned an order without an order ID")
					}
					if ibkr.IsTerminalOrderStatus(event.Value.Order.State.Status) {
						delete(openByID, *orderID)
					} else {
						openByID[*orderID] = *event.Value.Order
					}
				case event.Value.Status != nil && ibkr.IsTerminalOrderStatus(event.Value.Status.Status):
					delete(openByID, event.Value.Status.OrderID)
				}
			case ibkr.StreamSnapshotComplete:
				openOrders := make([]ibkr.OpenOrder, 0, len(openByID))
				for _, openOrder := range openByID {
					openOrders = append(openOrders, openOrder)
				}
				sort.Slice(openOrders, func(i, j int) bool {
					return *openOrders[i].Order.OrderID < *openOrders[j].Order.OrderID
				})
				return openOrders, nil
			}
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		}
	}
}

func awaitNewExecutionAndFee(ctx context.Context, client *ibkr.Client, account, label string, baseline ibkr.ExecutionSnapshot) error {
	knownExecutions := make(map[string]struct{}, len(baseline.Executions))
	for _, execution := range baseline.Executions {
		knownExecutions[execution.ExecID] = struct{}{}
	}
	deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot, err := client.Orders().Executions(deadline, ibkr.ExecutionsRequest{Account: account, Symbol: "AAPL"})
		if err == nil {
			newExecutions := make(map[string]struct{})
			for _, execution := range snapshot.Executions {
				if _, existed := knownExecutions[execution.ExecID]; !existed {
					newExecutions[execution.ExecID] = struct{}{}
				}
			}
			for _, fee := range snapshot.CommissionAndFees {
				if _, ok := newExecutions[fee.ExecID]; ok {
					recordAPIEvent("execution_and_fee_reconciled", label, func(event *apiDriverEvent) {
						event.Count = len(newExecutions)
						event.Values = map[string]string{"commission_and_fees": strconv.Itoa(len(snapshot.CommissionAndFees))}
					})
					return nil
				}
			}
		}
		select {
		case <-deadline.Done():
			if err != nil {
				return fmt.Errorf("%s execution/fee query: %w", label, err)
			}
			return fmt.Errorf("%s produced no correlated new execution and fee before deadline", label)
		case <-ticker.C:
		}
	}
}

func cleanupScenarioOrders(ctx context.Context, client *ibkr.Client, account, label string, orders []scenarioOrder) error {
	var cleanupErr error
	uncertain := false
	for i := len(orders) - 1; i >= 0; i-- {
		order := &orders[i]
		if order.terminal {
			continue
		}
		cancelOrder(ctx, client, account, order.handle, order.label)
		observation := observeOrder(ctx, order.handle, order.label+" cleanup", 15*time.Second)
		order.terminal = observation.terminal
		if !order.terminal {
			uncertain = true
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%s produced no terminal cleanup evidence", order.label))
		}
	}
	if uncertain {
		if err := guardedCancelAll(ctx, client, account, label+" uncertain cleanup global cancel"); err != nil {
			return errors.Join(cleanupErr, err)
		}
		if err := fenceAPIWrites(ctx, client, label+" uncertain cleanup global cancel"); err != nil {
			return errors.Join(cleanupErr, err)
		}
		openOrders, err := snapshotOpenOrders(ctx, client)
		if err != nil {
			return errors.Join(cleanupErr, fmt.Errorf("%s verify uncertain cleanup global cancel: %w", label, err))
		}
		if len(openOrders) != 0 {
			return errors.Join(cleanupErr, fmt.Errorf("%s has %d working orders after uncertain cleanup global cancel", label, len(openOrders)))
		}
	}
	return errors.Join(cleanupErr, fenceAPIWrites(ctx, client, label+" cleanup"))
}

func flattenAAPL(ctx context.Context, client *ibkr.Client, account string, label string, qty decimal.Decimal) error {
	order := ibkr.Order{
		Action:    ibkr.ActionSell,
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
	observation := observeOrder(ctx, handle, label+" flatten", 30*time.Second)
	if !observation.FullFill() || !observation.sawExecution || !observation.filledQty.Equal(qty) {
		return fmt.Errorf("%s flatten status=%s filled=%s execution=%t, want terminal fill of %s", label, observation.lastStatus, observation.filledQty, observation.sawExecution, qty)
	}
	return nil
}

func flattenAAPLFill(ctx context.Context, client *ibkr.Client, account string, label string, filledAction ibkr.OrderAction, qty decimal.Decimal) error {
	return flattenStockFill(ctx, client, account, label, apiAAPL, filledAction, qty)
}

func flattenStockFill(ctx context.Context, client *ibkr.Client, account string, label string, contract ibkr.Contract, filledAction ibkr.OrderAction, qty decimal.Decimal) error {
	action := ibkr.ActionSell
	if filledAction == ibkr.ActionSell {
		action = ibkr.ActionBuy
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
	observation := observeOrder(ctx, handle, label+" flatten", 30*time.Second)
	if !observation.FullFill() || !observation.sawExecution || !observation.filledQty.Equal(qty) {
		return fmt.Errorf("%s flatten status=%s filled=%s execution=%t, want terminal fill of %s", label, observation.lastStatus, observation.filledQty, observation.sawExecution, qty)
	}
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
	log.Printf("%s query executions=%d commission_and_fees=%d", label, len(updates.Executions), len(updates.CommissionAndFees))
	recordAPIEvent("executions_query", label, func(event *apiDriverEvent) {
		event.Account = req.Account
		event.Symbol = req.Symbol
		event.Count = len(updates.Executions)
		event.Values = map[string]string{"commission_and_fees": strconv.Itoa(len(updates.CommissionAndFees))}
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
	log.Printf("api session ready: server_version=%d next_valid_id=%d client_id=%d", snapshot.ServerVersion, snapshot.NextValidID, clientID)
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
			event.OrderID = *orders[0].Order.OrderID
			event.Account = orders[0].Order.Account
			event.Symbol = orders[0].Contract.Symbol
			event.SecType = string(orders[0].Contract.SecType)
			event.Action = string(orders[0].Order.Action)
			event.OrderType = string(orders[0].Order.OrderType)
			event.TIF = string(orders[0].Order.TIF)
			event.Quantity = orders[0].Order.Quantity.String()
			event.Status = string(orders[0].State.Status)
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
		Strike:       new(strike),
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
			Strike:       new(strike),
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
		if expiry > now {
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

func qualifyFrontFutureOption(ctx context.Context, client *ibkr.Client, symbol string) (ibkr.Contract, error) {
	future, err := qualifyFrontFuture(ctx, client, symbol)
	if err != nil {
		return ibkr.Contract{}, err
	}
	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol:  symbol,
		FutFopExchange:    future.Exchange,
		UnderlyingSecType: ibkr.SecTypeFuture,
		UnderlyingConID:   future.ConID,
	})
	if err != nil {
		return ibkr.Contract{}, err
	}
	param, ok := chooseOptionParams(params)
	if !ok {
		return ibkr.Contract{}, fmt.Errorf("no %s future-option parameters", symbol)
	}
	expiry, ok := chooseFutureExpiry(param.Expirations)
	if !ok {
		return ibkr.Contract{}, fmt.Errorf("no current %s future-option expiry", symbol)
	}
	strike, ok := chooseNearestStrike(param.Strikes, decimal.NewFromInt(6500))
	if !ok {
		return ibkr.Contract{}, fmt.Errorf("no %s future-option strike", symbol)
	}
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol: symbol, SecType: ibkr.SecTypeFutureOption, Expiry: expiry, Strike: new(strike),
		Right: ibkr.RightCall, Multiplier: param.Multiplier, Exchange: future.Exchange,
		Currency: future.Currency, TradingClass: param.TradingClass,
	})
	if err != nil {
		return ibkr.Contract{}, err
	}
	if len(details) == 0 {
		return ibkr.Contract{}, fmt.Errorf("no %s future-option details for %s %s", symbol, expiry, strike)
	}
	return details[0].Contract, nil
}

func drainObserver[T any](sub *ibkr.Subscription[T]) <-chan error {
	done := make(chan error, 1)
	go func() {
		for range sub.Events() {
		}
		done <- sub.Wait()
	}()
	return done
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
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiEURUSD, decimal.RequireFromString("1.10"))
		log.Printf("EUR.USD anchor: %s", anchor)

		// Far LMT rest.
		order := baseAPIOrder(account, decimal.NewFromInt(100000), ibkr.ActionBuy, ibkr.OrderTypeLimit)
		order.LmtPrice = new(anchor.Mul(decimal.RequireFromString("0.90")).Round(5))

		handle, err := placeAPIOrder(ctx, client, "forex rest", apiEURUSD, order)
		if err != nil {
			log.Printf("forex place: %v", err)
			return nil
		}
		_ = observeOrder(ctx, handle, "forex rest", 8*time.Second)

		// Modify price.
		order.LmtPrice = new(anchor.Mul(decimal.RequireFromString("0.92")).Round(5))
		if err := modifyAPIOrder(ctx, client, handle, "forex modified", order); err != nil {
			log.Printf("forex modify: %v", err)
		}
		_ = observeOrder(ctx, handle, "forex modified", 8*time.Second)

		// Cancel.
		cancelOrder(ctx, client, account, handle, "forex")
		_ = observeOrder(ctx, handle, "forex cancel", 8*time.Second)
		return nil
	})
}

func runAPIWhatIfMarginAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 1*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		invalidContract := apiAAPL
		invalidContract.ConID = 0
		invalidContract.Symbol = "ZZZZNONE"
		invalidOrder := baseAPIOrder(account, decimal.NewFromInt(1), ibkr.ActionBuy, ibkr.OrderTypeMarket)
		_, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{Contract: invalidContract, Order: invalidOrder})
		recordProbeResult("whatif_preview", "invalid_contract", 0, err)
		if err == nil {
			return errors.New("invalid-contract preview unexpectedly succeeded")
		}

		start := orderTimestamp(time.Now().UTC().Add(3 * time.Minute))
		end := orderTimestamp(time.Now().UTC().Add(20 * time.Minute))
		darkIce := withAlgo(
			withDisplaySize(withLimit(baseAPIOrder(account, decimal.NewFromInt(1), ibkr.ActionBuy, ibkr.OrderTypeLimit), decimal.NewFromInt(150)), 1),
			"DarkIce",
			[]ibkr.TagValue{
				{Tag: "displaySize", Value: "1"},
				{Tag: "startTime", Value: start},
				{Tag: "endTime", Value: end},
				{Tag: "allowPastEndTime", Value: "1"},
			},
		)
		_, err = client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: darkIce})
		recordProbeResult("whatif_preview", "dark_ice_display_size", 0, err)
		if err == nil {
			return errors.New("DarkIce display-size preview unexpectedly succeeded")
		}

		order := baseAPIOrder(account, decimal.NewFromInt(100), ibkr.ActionBuy, ibkr.OrderTypeMarket)
		state, err := client.Orders().Preview(ctx, ibkr.PlaceOrderRequest{Contract: apiAAPL, Order: order})
		if err != nil {
			log.Printf("whatif preview: %v", err)
			return nil
		}
		log.Printf("whatif preview: init_margin_after=%s maint_margin_after=%s commission=%s min=%s max=%s currency=%s",
			state.InitMarginAfter, state.MaintMarginAfter, state.CommissionAndFees, state.MinCommissionAndFees, state.MaxCommissionAndFees, state.CommissionAndFeesCurrency)
		return nil
	})
}

func runAPIStressRapidFireAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL stress anchor: %s", anchor)

		const n = 10
		handles := make([]*ibkr.OrderHandle, 0, n)
		for i := 0; i < n; i++ {
			order := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor).Add(decimal.NewFromInt(int64(i))))
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
		if err := guardedCancelAll(ctx, client, account, "stress global cancel"); err != nil {
			log.Printf("stress global cancel: %v", err)
		}

		for i, h := range handles {
			_ = observeOrder(ctx, h, fmt.Sprintf("stress[%d] cancel", i), 8*time.Second)
		}
		return nil
	})
}

func runAPIScaleInCampaignAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL scale-in anchor: %s", anchor)
		scaleOrder := withScale(withLimit(baseAPIOrder(account, decimal.NewFromInt(3), ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor)))
		scale, err := placeAPIOrder(ctx, client, "scale nondefault resting", apiAAPL, scaleOrder)
		if err != nil {
			return fmt.Errorf("place nondefault scale order: %w", err)
		}
		scaleEcho, err := awaitOpenOrderEvidence(ctx, scale, "scale nondefault resting", 20*time.Second)
		if err != nil {
			return err
		}
		if scaleEcho.Order.Scale.InitialLevelSize == nil || *scaleEcho.Order.Scale.InitialLevelSize != 1 ||
			scaleEcho.Order.Scale.SubsequentLevelSize == nil || *scaleEcho.Order.Scale.SubsequentLevelSize != 1 ||
			!scaleEcho.Order.Scale.PriceIncrement.Equal(decimal.RequireFromString("0.05")) {
			return fmt.Errorf("scale echo = %+v, want initial/subsequent 1 and increment 0.05", scaleEcho.Order.Scale)
		}
		recordAPIEvent("scale_echo", "nondefault resting", func(event *apiDriverEvent) {
			event.OrderID = scale.OrderID()
			event.Values = map[string]string{
				"initial_size":    strconv.Itoa(*scaleEcho.Order.Scale.InitialLevelSize),
				"subsequent_size": strconv.Itoa(*scaleEcho.Order.Scale.SubsequentLevelSize),
				"price_increment": scaleEcho.Order.Scale.PriceIncrement.String(),
			}
		})
		cancelOrder(ctx, client, account, scale, "scale nondefault resting")
		scaleObservation := observeOrder(ctx, scale, "scale nondefault resting cancel", 20*time.Second)
		if !scaleObservation.terminal || scaleObservation.AnyFill() {
			return fmt.Errorf("scale order cleanup status=%s filled=%s, want terminal zero fill", scaleObservation.lastStatus, scaleObservation.filledQty)
		}

		// 2x MKT buys.
		filledQty := decimal.Zero
		for i := 0; i < 2; i++ {
			order := baseAPIOrder(account, apiStockCampaignOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
			handle, err := placeAPIOrder(ctx, client, fmt.Sprintf("scale buy[%d]", i), apiAAPL, order)
			if err != nil {
				return fmt.Errorf("scale buy[%d]: %w", i, err)
			}
			obs := observeOrder(ctx, handle, fmt.Sprintf("scale buy[%d]", i), 20*time.Second)
			if !obs.FullFill() || !obs.sawExecution || !obs.filledQty.Equal(apiStockCampaignOrderQuantity) {
				if !handleDone(handle) {
					cancelOrder(ctx, client, account, handle, fmt.Sprintf("scale buy[%d] unfilled", i))
					_ = observeOrder(ctx, handle, fmt.Sprintf("scale buy[%d] unfilled cancel", i), 8*time.Second)
				}
				return fmt.Errorf("scale buy[%d] status=%s filled=%s execution=%t, want terminal fill", i, obs.lastStatus, obs.filledQty, obs.sawExecution)
			}
			filledQty = filledQty.Add(obs.filledQty)
		}

		// Protective stop-loss.
		stopOrder := baseAPIOrder(account, filledQty, ibkr.ActionSell, ibkr.OrderTypeStop)
		stopOrder.AuxPrice = new(farBuy(anchor))
		stopOrder.TIF = ibkr.TIFGTC
		stopHandle, err := placeAPIOrder(ctx, client, "scale stop-loss", apiAAPL, stopOrder)
		if err != nil {
			return fmt.Errorf("place scale stop-loss: %w", err)
		}
		if _, err := awaitOpenOrderEvidence(ctx, stopHandle, "scale stop-loss", 20*time.Second); err != nil {
			return err
		}
		cancelOrder(ctx, client, account, stopHandle, "scale stop-loss")
		stopObservation := observeOrder(ctx, stopHandle, "scale stop-loss cancel", 20*time.Second)
		if !stopObservation.terminal || stopObservation.AnyFill() {
			return fmt.Errorf("scale stop cleanup status=%s filled=%s, want terminal zero fill", stopObservation.lastStatus, stopObservation.filledQty)
		}

		// Flatten.
		if err := flattenAAPL(ctx, client, account, "scale flatten", filledQty); err != nil {
			log.Printf("scale flatten: %v", err)
			return errors.Join(err, fenceAPIWrites(ctx, client, "scale-in failed flatten"))
		}

		queryAAPLExecutions(client, account)
		return fenceAPIWrites(ctx, client, "scale-in cleanup")
	})
}

func runAPIIOCFOKAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 3*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		log.Printf("AAPL IOC/FOK anchor: %s", anchor)

		// IOC marketable.
		iocOrder := baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit)
		iocOrder.LmtPrice = new(marketableBuy(anchor))
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
		fokOrder := baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit)
		fokOrder.LmtPrice = new(marketableBuy(anchor))
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
		fokFarOrder := baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit)
		fokFarOrder.LmtPrice = new(farBuy(anchor))
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

func runAPIOptionExerciseAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 6*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) error {
		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("300"))
		option, err := qualifyAAPLITMCall(ctx, client, anchor)
		if err != nil {
			return fmt.Errorf("qualify ITM AAPL call: %w", err)
		}
		before, err := snapshotPositions(ctx, client)
		if err != nil {
			return fmt.Errorf("option exercise pre-trade positions: %w", err)
		}
		recordAPIEvent("option_exercise_contract", "aapl call", func(event *apiDriverEvent) {
			event.Symbol = option.Symbol
			event.SecType = string(option.SecType)
			event.Values = map[string]string{
				"con_id":      strconv.FormatInt(int64(option.ConID), 10),
				"expiry":      option.Expiry,
				"strike":      option.Strike.String(),
				"right":       string(option.Right),
				"under_price": anchor.String(),
			}
		})

		order := baseAPIOrder(account, apiOptionContractQuantity, ibkr.ActionBuy, ibkr.OrderTypeMarket)
		purchase, err := placeAPIOrder(ctx, client, "option exercise seed", option, order)
		if err != nil {
			return fmt.Errorf("buy option for exercise: %w", err)
		}
		purchaseObservation := observeOrder(ctx, purchase, "option exercise seed", 45*time.Second)
		if !purchaseObservation.FullFill() || !purchaseObservation.sawExecution || !purchaseObservation.filledQty.Equal(apiOptionContractQuantity) {
			return fmt.Errorf("option exercise seed status=%s filled=%s execution=%t, want one-contract terminal fill", purchaseObservation.lastStatus, purchaseObservation.filledQty, purchaseObservation.sawExecution)
		}
		if err := waitForPositionDelta(ctx, client, before, option.ConID, apiOptionContractQuantity, 30*time.Second); err != nil {
			return fmt.Errorf("option exercise seed position: %w", err)
		}

		handle, err := client.Options().Exercise(ctx, ibkr.ExerciseOptionsRequest{
			Contract: option, ExerciseAction: ibkr.Exercise, ExerciseQuantity: 1, Account: account,
		})
		if err != nil {
			return fmt.Errorf("admit option exercise: %w", err)
		}
		exercise, err := observeExercise(ctx, handle, "AAPL call exercise", 60*time.Second)
		if err != nil {
			return err
		}
		if exercise.acceptedUnsettled {
			after, err := snapshotPositions(ctx, client)
			if err != nil {
				return fmt.Errorf("accepted option exercise positions: %w", err)
			}
			optionDelta := positionQuantity(after, option.ConID).Sub(positionQuantity(before, option.ConID))
			stockDelta := positionQuantity(after, apiAAPL.ConID).Sub(positionQuantity(before, apiAAPL.ConID))
			if !optionDelta.Equal(apiOptionContractQuantity) || !stockDelta.IsZero() {
				return fmt.Errorf("accepted-but-unsettled exercise option delta=%s stock delta=%s, want 1/0", optionDelta, stockDelta)
			}
			recordAPIEvent("option_exercise_accepted_unsettled", "AAPL call exercise", func(event *apiDriverEvent) {
				event.Status = string(exercise.status)
				event.Values = map[string]string{
					"option_con_id": strconv.FormatInt(int64(option.ConID), 10),
					"option_delta":  optionDelta.String(),
					"stock_delta":   stockDelta.String(),
					"warning_code":  strconv.Itoa(ibkr.ErrCodeOrderTIFSetFromPreset),
				}
			})
			return nil
		}
		if !exercise.terminal {
			return fmt.Errorf("option exercise observation ended at status %q without accepted or terminal evidence", exercise.status)
		}
		if err := waitForExercisePositionTransition(ctx, client, before, option.ConID, 90*time.Second); err != nil {
			return err
		}
		recordAPIEvent("option_exercise_completed", "AAPL call exercise", func(event *apiDriverEvent) {
			event.Status = string(exercise.status)
			event.Values = map[string]string{"option_con_id": strconv.FormatInt(int64(option.ConID), 10)}
		})
		return nil
	})
}

func qualifyAAPLITMCall(ctx context.Context, client *ibkr.Client, anchor decimal.Decimal) (ibkr.Contract, error) {
	params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
		UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: apiAAPL.ConID,
	})
	if err != nil {
		return ibkr.Contract{}, err
	}
	param, ok := chooseOptionParams(params)
	if !ok {
		return ibkr.Contract{}, errors.New("no AAPL SMART option params")
	}
	expiry, ok := chooseFutureExpiry(param.Expirations)
	if !ok {
		return ibkr.Contract{}, errors.New("no future AAPL option expiration")
	}
	strike, ok := chooseITMCallStrike(param.Strikes, anchor)
	if !ok {
		return ibkr.Contract{}, fmt.Errorf("no AAPL call strike below underlier %s", anchor)
	}
	details, err := client.Contracts().Details(ctx, ibkr.Contract{
		Symbol: "AAPL", SecType: ibkr.SecTypeOption, Expiry: expiry, Strike: new(strike),
		Right: ibkr.RightCall, Multiplier: param.Multiplier, Exchange: "SMART", Currency: "USD",
		TradingClass: param.TradingClass,
	})
	if err != nil {
		return ibkr.Contract{}, err
	}
	if len(details) == 0 {
		return ibkr.Contract{}, fmt.Errorf("no qualified AAPL call for expiry %s strike %s", expiry, strike)
	}
	return details[0].Contract, nil
}

func chooseITMCallStrike(strikes []decimal.Decimal, anchor decimal.Decimal) (decimal.Decimal, bool) {
	target := anchor.Mul(decimal.RequireFromString("0.98"))
	var best decimal.Decimal
	found := false
	for _, strike := range strikes {
		if strike.GreaterThan(target) || found && strike.LessThanOrEqual(best) {
			continue
		}
		best = strike
		found = true
	}
	if found {
		return best, true
	}
	for _, strike := range strikes {
		if strike.GreaterThanOrEqual(anchor) || found && strike.LessThanOrEqual(best) {
			continue
		}
		best = strike
		found = true
	}
	return best, found
}

type exerciseObservation struct {
	status            ibkr.OrderStatus
	terminal          bool
	acceptedUnsettled bool
}

func observeExercise(ctx context.Context, handle *ibkr.ExerciseHandle, label string, wait time.Duration) (exerciseObservation, error) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	var observation exerciseObservation
	var presetWarning bool
	for {
		select {
		case event, ok := <-handle.Events():
			if !ok {
				return observation, handle.Wait()
			}
			recordAPIEvent("option_exercise_event", label, func(driverEvent *apiDriverEvent) {
				if event.Status != nil {
					driverEvent.Status = string(event.Status.Status)
					driverEvent.OrderID = event.Status.OrderID
				}
				if event.Warning != nil {
					driverEvent.Error = event.Warning.Error()
				}
			})
			if event.Warning != nil && event.Warning.Code == ibkr.ErrCodeOrderTIFSetFromPreset &&
				strings.Contains(event.Warning.Message, "Order TIF was set to DAY based on order preset.") {
				presetWarning = true
			}
			if event.Status != nil {
				observation.status = event.Status.Status
				observation.terminal = ibkr.IsTerminalOrderStatus(observation.status)
			}
			if observation.terminal {
				handle.Close()
				if err := handle.Wait(); err != nil {
					return observation, err
				}
				return observation, nil
			}
			if presetWarning && observation.status == ibkr.OrderStatusPreSubmitted {
				observation.acceptedUnsettled = true
				handle.Close()
				if err := handle.Wait(); err != nil {
					return observation, err
				}
				return observation, nil
			}
		case <-handle.Done():
			return observation, handle.Wait()
		case <-timer.C:
			handle.Close()
			if err := handle.Wait(); err != nil {
				return observation, err
			}
			return observation, fmt.Errorf("%s produced neither terminal evidence nor exact accepted-but-unsettled admission within %s", label, wait)
		case <-ctx.Done():
			return observation, context.Cause(ctx)
		}
	}
}

func waitForPositionDelta(ctx context.Context, client *ibkr.Client, baseline []ibkr.Position, conID ibkr.ContractID, want decimal.Decimal, wait time.Duration) error {
	baselineQty := positionQuantity(baseline, conID)
	deadline, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		positions, err := snapshotPositions(deadline, client)
		if err == nil && positionQuantity(positions, conID).Sub(baselineQty).Equal(want) {
			return nil
		}
		select {
		case <-deadline.Done():
			if err != nil {
				return errors.Join(context.Cause(deadline), err)
			}
			return fmt.Errorf("contract %d position did not change by %s", conID, want)
		case <-ticker.C:
		}
	}
}

func waitForExercisePositionTransition(ctx context.Context, client *ibkr.Client, baseline []ibkr.Position, optionConID ibkr.ContractID, wait time.Duration) error {
	baselineOption := positionQuantity(baseline, optionConID)
	baselineStock := positionQuantity(baseline, apiAAPL.ConID)
	deadline, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		positions, err := snapshotPositions(deadline, client)
		if err == nil {
			optionDelta := positionQuantity(positions, optionConID).Sub(baselineOption)
			stockDelta := positionQuantity(positions, apiAAPL.ConID).Sub(baselineStock)
			if optionDelta.LessThan(apiOptionContractQuantity) || !stockDelta.IsZero() {
				recordAPIEvent("option_exercise_position_transition", "AAPL call exercise", func(event *apiDriverEvent) {
					event.Values = map[string]string{"option_delta": optionDelta.String(), "stock_delta": stockDelta.String()}
				})
				return nil
			}
		}
		select {
		case <-deadline.Done():
			if err != nil {
				return errors.Join(context.Cause(deadline), err)
			}
			return errors.New("terminal option exercise produced no option or AAPL stock position transition")
		case <-ticker.C:
		}
	}
}

func positionQuantity(positions []ibkr.Position, conID ibkr.ContractID) decimal.Decimal {
	for _, position := range positions {
		if position.Contract.ConID == conID {
			return position.Position
		}
	}
	return decimal.Zero
}

func runAPIHedgeOrderAAPL(ctx context.Context, addr string, clientID int) error {
	return apiTradingScenario(ctx, addr, clientID, 4*time.Minute, func(ctx context.Context, client *ibkr.Client, account string) (runErr error) {
		var orders []scenarioOrder
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			runErr = errors.Join(runErr, cleanupScenarioOrders(cleanupCtx, client, account, "hedge campaign", orders))
		}()

		anchor := quoteAnchor(ctx, client, apiAAPL, decimal.RequireFromString("200"))
		// Current sv225 capture 20260824T210913Z-api_hedge_order_aapl freezes
		// the rule that delta hedges hang off OPTION parents (the stock parent
		// drew code 320 "parent order has to be option order") and that hedge children carry zero quantity
		// (size drew 10032 "Specifying size for hedge order is not
		// allowed"). The compliant shape: option parent, zero-size stock
		// delta child; the stock-parent variants stay for their real
		// rejection evidence.
		opt, optErr := qualifyAAPLCall(ctx, client, anchor)
		if optErr == nil {
			parent := withLimit(baseAPIOrder(account, decimal.NewFromInt(1), ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))
			parentHandle, err := placeAPIOrder(ctx, client, "option hedge parent", opt, parent)
			if err != nil {
				log.Printf("option hedge parent place error: %v", err)
			} else {
				orders = append(orders, scenarioOrder{label: "option hedge parent", handle: parentHandle})
				child := withLimit(baseAPIOrder(account, decimal.Zero, ibkr.ActionSell, ibkr.OrderTypeLimit), farSell(anchor))
				child.ParentID = parentHandle.OrderID()
				child.Hedge = ibkr.OrderHedge{Type: ibkr.HedgeDelta, Param: "0.5"}
				child.Transmit = new(true)
				handle, err := placeAPIOrder(ctx, client, "delta_hedge_compliant", apiAAPL, child)
				if err != nil {
					log.Printf("delta_hedge_compliant place error: %v", err)
				} else {
					obs := observeOrder(ctx, handle, "delta_hedge_compliant", 10*time.Second)
					orders = append(orders, scenarioOrder{label: "delta_hedge_compliant", handle: handle, terminal: obs.terminal})
					log.Printf("delta_hedge_compliant observed status=%s", obs.lastStatus)
				}
			}
		} else {
			log.Printf("qualify option for hedge parent: %v", optErr)
		}

		stockParent := withLimit(baseAPIOrder(account, apiStockOrderQuantity, ibkr.ActionBuy, ibkr.OrderTypeLimit), farBuy(anchor))
		parentHandle, err := placeAPIOrder(ctx, client, "hedge parent", apiAAPL, stockParent)
		if err != nil {
			log.Printf("hedge parent place error: %v", err)
			return nil
		}
		orders = append(orders, scenarioOrder{label: "hedge parent", handle: parentHandle})
		hedges := []struct {
			label string
			typ   ibkr.HedgeType
			param string
		}{
			{label: "delta_hedge_stock_parent", typ: ibkr.HedgeDelta, param: "0.5"},
			{label: "beta_hedge_zero_size", typ: ibkr.HedgeBeta, param: "1.0"},
			{label: "fx_hedge_zero_size", typ: ibkr.HedgeFX, param: ""},
			{label: "pair_hedge_zero_size", typ: ibkr.HedgePair, param: "0.8"},
		}
		for _, h := range hedges {
			qty := decimal.Zero
			if h.typ == ibkr.HedgeDelta {
				qty = apiStockOrderQuantity
			}
			child := withLimit(baseAPIOrder(account, qty, ibkr.ActionSell, ibkr.OrderTypeLimit), farSell(anchor))
			child.ParentID = parentHandle.OrderID()
			child.Hedge = ibkr.OrderHedge{Type: h.typ, Param: h.param}
			child.Transmit = new(true)
			handle, err := placeAPIOrder(ctx, client, h.label, apiAAPL, child)
			if err != nil {
				log.Printf("%s place error: %v", h.label, err)
				continue
			}
			obs := observeOrder(ctx, handle, h.label, 8*time.Second)
			orders = append(orders, scenarioOrder{label: h.label, handle: handle, terminal: obs.terminal})
			log.Printf("%s observed status=%s", h.label, obs.lastStatus)
		}
		return nil
	})
}
