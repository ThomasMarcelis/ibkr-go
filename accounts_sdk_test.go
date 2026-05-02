package ibkr

import (
	"context"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

const accountStreamsFixturePath = "internal/sdkadapter/testdata/fixtures/official_sdk_account_streams_snapshot_20260502.json"

func TestSDKAccountUpdatesPublicRouteReplaysAccountValueFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		updates []AccountUpdate
		err     error
	}, 1)
	go func() {
		updates, err := client.Accounts().Updates(ctx, "DU_REDACTED")
		resultCh <- struct {
			updates []AccountUpdate
			err     error
		}{updates: updates, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandAccountUpdates ||
		!command.AccountUpdates.Subscribe ||
		command.AccountUpdates.Account != "DU_REDACTED" {
		t.Fatalf("account updates command = %+v, want subscribe for redacted account", command)
	}

	dispatchSDKFixtureEvent(t, e, fixtureEvent(t, accountStreamsFixturePath, sdkadapter.EventUpdateAccountValue, 0))
	dispatchSDKFixtureEvent(t, e, fixtureEvent(t, accountStreamsFixturePath, sdkadapter.EventAccountDownloadEnd, 0))

	result := receiveAccountUpdatesResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Accounts().Updates() error = %v", result.err)
	}
	if len(result.updates) != 1 {
		t.Fatalf("Accounts().Updates() len = %d, want 1", len(result.updates))
	}
	value := result.updates[0].AccountValue
	if value == nil || value.Account != "DU_REDACTED" || value.Key != "AccountCode" || value.Value != "REDACTED_VALUE" {
		t.Fatalf("Accounts().Updates()[0].AccountValue = %+v, want redacted fixture account value", value)
	}

	runNextEngineCommand(t, e)
	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want subscribe and unsubscribe: %+v", len(commands), commands)
	}
	unsubscribe := commands[1]
	if unsubscribe.Kind != sdkadapter.CommandAccountUpdates ||
		unsubscribe.AccountUpdates.Subscribe ||
		unsubscribe.AccountUpdates.Account != "DU_REDACTED" {
		t.Fatalf("account updates unsubscribe command = %+v, want unsubscribe for redacted account", unsubscribe)
	}
}

func TestSDKAccountUpdatesMultiPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		values []AccountUpdateMultiValue
		err    error
	}, 1)
	go func() {
		values, err := client.Accounts().UpdatesMulti(ctx, AccountUpdatesMultiRequest{Account: "DU_REDACTED"})
		resultCh <- struct {
			values []AccountUpdateMultiValue
			err    error
		}{values: values, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandAccountUpdatesMulti ||
		command.AccountUpdatesMulti.Account != "DU_REDACTED" {
		t.Fatalf("account updates multi command = %+v, want request for redacted account", command)
	}

	update := fixtureEvent(t, accountStreamsFixturePath, sdkadapter.EventAccountUpdateMulti, 811)
	update.ReqID = command.AccountUpdatesMulti.ReqID
	dispatchSDKFixtureEvent(t, e, update)
	end := fixtureEvent(t, accountStreamsFixturePath, sdkadapter.EventAccountUpdateMultiEnd, 811)
	end.ReqID = command.AccountUpdatesMulti.ReqID
	dispatchSDKFixtureEvent(t, e, end)

	result := receiveAccountUpdatesMultiResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Accounts().UpdatesMulti() error = %v", result.err)
	}
	if len(result.values) != 1 {
		t.Fatalf("Accounts().UpdatesMulti() len = %d, want 1", len(result.values))
	}
	value := result.values[0]
	if value.Account != "DU_REDACTED" ||
		value.Key != "Currency" ||
		value.Currency != "EUR" ||
		value.Value != "REDACTED_VALUE" {
		t.Fatalf("Accounts().UpdatesMulti()[0] = %+v, want redacted fixture account update", value)
	}

	runNextEngineCommand(t, e)
	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want request and cancel: %+v", len(commands), commands)
	}
	cancelCommand := commands[1]
	if cancelCommand.Kind != sdkadapter.CommandCancelAccountUpdatesMulti ||
		cancelCommand.CancelAccountUpdatesMulti.ReqID != command.AccountUpdatesMulti.ReqID {
		t.Fatalf("account updates multi cancel command = %+v, want reqID %d", cancelCommand, command.AccountUpdatesMulti.ReqID)
	}
}

func TestSDKPnLPublicSubscriptionsSendSDKCancel(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		sub *Subscription[PnLUpdate]
		err error
	}, 1)
	go func() {
		sub, err := client.Accounts().SubscribePnL(ctx, PnLRequest{Account: "DU_REDACTED"})
		resultCh <- struct {
			sub *Subscription[PnLUpdate]
			err error
		}{sub: sub, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandPnL ||
		command.PnL.Account != "DU_REDACTED" {
		t.Fatalf("pnl command = %+v, want request for redacted account", command)
	}
	result := receivePnLSubscriptionResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Accounts().SubscribePnL() error = %v", result.err)
	}
	if err := result.sub.Close(); err != nil {
		t.Fatalf("Subscription.Close() error = %v", err)
	}

	runNextEngineCommand(t, e)
	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want request and cancel: %+v", len(commands), commands)
	}
	cancelCommand := commands[1]
	if cancelCommand.Kind != sdkadapter.CommandCancelPnL ||
		cancelCommand.CancelPnL.ReqID != command.PnL.ReqID {
		t.Fatalf("pnl cancel command = %+v, want reqID %d", cancelCommand, command.PnL.ReqID)
	}
}

func TestSDKPnLSinglePublicSubscriptionSendsSDKCancel(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		sub *Subscription[PnLSingleUpdate]
		err error
	}, 1)
	go func() {
		sub, err := client.Accounts().SubscribePnLSingle(ctx, PnLSingleRequest{Account: "DU_REDACTED", ConID: 265598})
		resultCh <- struct {
			sub *Subscription[PnLSingleUpdate]
			err error
		}{sub: sub, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandPnLSingle ||
		command.PnLSingle.Account != "DU_REDACTED" ||
		command.PnLSingle.ConID != 265598 {
		t.Fatalf("pnl single command = %+v, want request for redacted account and AAPL conID", command)
	}
	result := receivePnLSingleSubscriptionResult(t, resultCh)
	if result.err != nil {
		t.Fatalf("Accounts().SubscribePnLSingle() error = %v", result.err)
	}
	if err := result.sub.Close(); err != nil {
		t.Fatalf("Subscription.Close() error = %v", err)
	}

	runNextEngineCommand(t, e)
	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want request and cancel: %+v", len(commands), commands)
	}
	cancelCommand := commands[1]
	if cancelCommand.Kind != sdkadapter.CommandCancelPnLSingle ||
		cancelCommand.CancelPnLSingle.ReqID != command.PnLSingle.ReqID {
		t.Fatalf("pnl single cancel command = %+v, want reqID %d", cancelCommand, command.PnLSingle.ReqID)
	}
}

func TestSDKAccountStreamsUseSDKCommands(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.AccountUpdatesRequest{Subscribe: true, Account: "DU123"}); err != nil {
		t.Fatalf("sendSDKContext(AccountUpdatesRequest subscribe) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.AccountUpdatesRequest{Subscribe: false, Account: "DU123"}); err != nil {
		t.Fatalf("sendSDKContext(AccountUpdatesRequest unsubscribe) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.AccountUpdatesMultiRequest{ReqID: 91, Account: "DU123", ModelCode: "MODEL"}); err != nil {
		t.Fatalf("sendSDKContext(AccountUpdatesMultiRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelAccountUpdatesMulti{ReqID: 91}); err != nil {
		t.Fatalf("sendSDKContext(CancelAccountUpdatesMulti) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.PositionsMultiRequest{ReqID: 92, Account: "DU123", ModelCode: "MODEL"}); err != nil {
		t.Fatalf("sendSDKContext(PositionsMultiRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelPositionsMulti{ReqID: 92}); err != nil {
		t.Fatalf("sendSDKContext(CancelPositionsMulti) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 6 {
		t.Fatalf("commands len = %d, want 6", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandAccountUpdates || !commands[0].AccountUpdates.Subscribe || commands[0].AccountUpdates.Account != "DU123" {
		t.Fatalf("account updates subscribe command = %+v", commands[0])
	}
	if commands[1].Kind != sdkadapter.CommandAccountUpdates || commands[1].AccountUpdates.Subscribe || commands[1].AccountUpdates.Account != "DU123" {
		t.Fatalf("account updates unsubscribe command = %+v", commands[1])
	}
	if commands[2].Kind != sdkadapter.CommandAccountUpdatesMulti || commands[2].AccountUpdatesMulti.ReqID != 91 || commands[2].AccountUpdatesMulti.ModelCode != "MODEL" {
		t.Fatalf("account updates multi command = %+v", commands[2])
	}
	if commands[3].Kind != sdkadapter.CommandCancelAccountUpdatesMulti || commands[3].CancelAccountUpdatesMulti.ReqID != 91 {
		t.Fatalf("cancel account updates multi command = %+v", commands[3])
	}
	if commands[4].Kind != sdkadapter.CommandPositionsMulti || commands[4].PositionsMulti.ReqID != 92 || commands[4].PositionsMulti.Account != "DU123" {
		t.Fatalf("positions multi command = %+v", commands[4])
	}
	if commands[5].Kind != sdkadapter.CommandCancelPositionsMulti || commands[5].CancelPositionsMulti.ReqID != 92 {
		t.Fatalf("cancel positions multi command = %+v", commands[5])
	}
}

func TestSDKAccountStreamsUseSDKEvents(t *testing.T) {
	t.Parallel()

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventUpdateAccountValue,
		AccountValue: sdkadapter.AccountValueEvent{
			Key:      "NetLiquidation",
			Value:    "100000",
			Currency: "USD",
			Account:  "DU123",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(update account value) error = %v", err)
	}
	accountValue, ok := msg.(sdkadapter.UpdateAccountValue)
	if !ok {
		t.Fatalf("sdkEventToMessage(update account value) type = %T, want sdkadapter.UpdateAccountValue", msg)
	}
	if accountValue.Key != "NetLiquidation" || accountValue.Value != "100000" || accountValue.Account != "DU123" {
		t.Fatalf("account value = %+v, want copied account value", accountValue)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind: sdkadapter.EventUpdatePortfolio,
		Portfolio: sdkadapter.PortfolioValueEvent{
			Account: "DU123",
			Contract: sdkadapter.Contract{
				Symbol:   "AAPL",
				SecType:  "STK",
				Exchange: "SMART",
				Currency: "USD",
			},
			Position:      "10",
			MarketPrice:   "200",
			MarketValue:   "2000",
			AvgCost:       "150",
			UnrealizedPNL: "500",
			RealizedPNL:   "25",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(update portfolio) error = %v", err)
	}
	portfolio, ok := msg.(sdkadapter.UpdatePortfolio)
	if !ok {
		t.Fatalf("sdkEventToMessage(update portfolio) type = %T, want sdkadapter.UpdatePortfolio", msg)
	}
	if portfolio.Contract.Symbol != "AAPL" || portfolio.Position != "10" || portfolio.UnrealizedPNL != "500" {
		t.Fatalf("portfolio = %+v, want copied portfolio", portfolio)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{Kind: sdkadapter.EventUpdateAccountTime, AccountTime: "12:00"})
	if err != nil {
		t.Fatalf("sdkEventToMessage(update account time) error = %v", err)
	}
	if got := msg.(sdkadapter.UpdateAccountTime); got.Timestamp != "12:00" {
		t.Fatalf("account time = %+v, want timestamp", got)
	}
	msg, err = sdkEventToMessage(sdkadapter.Event{Kind: sdkadapter.EventAccountDownloadEnd, AccountDownloadEnd: "DU123"})
	if err != nil {
		t.Fatalf("sdkEventToMessage(account download end) error = %v", err)
	}
	if got := msg.(sdkadapter.AccountDownloadEnd); got.Account != "DU123" {
		t.Fatalf("account download end = %+v, want DU123", got)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventAccountUpdateMulti,
		ReqID: 91,
		AccountUpdateMulti: sdkadapter.AccountUpdateMultiEvent{
			Account:   "DU123",
			ModelCode: "MODEL",
			Key:       "CashBalance",
			Value:     "1000",
			Currency:  "USD",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(account update multi) error = %v", err)
	}
	multi, ok := msg.(sdkadapter.AccountUpdateMultiValue)
	if !ok {
		t.Fatalf("sdkEventToMessage(account update multi) type = %T, want sdkadapter.AccountUpdateMultiValue", msg)
	}
	if multi.ReqID != 91 || multi.ModelCode != "MODEL" || multi.Key != "CashBalance" {
		t.Fatalf("account update multi = %+v, want copied multi value", multi)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventPositionMulti,
		ReqID: 92,
		PositionMulti: sdkadapter.PositionMultiEvent{
			Account:   "DU123",
			ModelCode: "MODEL",
			Contract:  sdkadapter.Contract{Symbol: "MSFT", SecType: "STK", Exchange: "SMART", Currency: "USD"},
			Position:  "5",
			AvgCost:   "300",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(position multi) error = %v", err)
	}
	position, ok := msg.(sdkadapter.PositionMulti)
	if !ok {
		t.Fatalf("sdkEventToMessage(position multi) type = %T, want sdkadapter.PositionMulti", msg)
	}
	if position.ReqID != 92 || position.Contract.Symbol != "MSFT" || position.AvgCost != "300" {
		t.Fatalf("position multi = %+v, want copied position", position)
	}
}

func TestSDKPnLUsesSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	if err := e.sendSDKContext(context.Background(), sdkadapter.PnLRequest{ReqID: 101, Account: "DU123", ModelCode: "MODEL"}); err != nil {
		t.Fatalf("sendSDKContext(PnLRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelPnL{ReqID: 101}); err != nil {
		t.Fatalf("sendSDKContext(CancelPnL) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.PnLSingleRequest{ReqID: 102, Account: "DU123", ModelCode: "MODEL", ConID: 265598}); err != nil {
		t.Fatalf("sendSDKContext(PnLSingleRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelPnLSingle{ReqID: 102}); err != nil {
		t.Fatalf("sendSDKContext(CancelPnLSingle) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 4 {
		t.Fatalf("commands len = %d, want 4", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandPnL || commands[0].PnL.ReqID != 101 || commands[0].PnL.ModelCode != "MODEL" {
		t.Fatalf("pnl command = %+v", commands[0])
	}
	if commands[1].Kind != sdkadapter.CommandCancelPnL || commands[1].CancelPnL.ReqID != 101 {
		t.Fatalf("cancel pnl command = %+v", commands[1])
	}
	if commands[2].Kind != sdkadapter.CommandPnLSingle || commands[2].PnLSingle.ReqID != 102 || commands[2].PnLSingle.ConID != 265598 {
		t.Fatalf("pnl single command = %+v", commands[2])
	}
	if commands[3].Kind != sdkadapter.CommandCancelPnLSingle || commands[3].CancelPnLSingle.ReqID != 102 {
		t.Fatalf("cancel pnl single command = %+v", commands[3])
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventPnL,
		ReqID: 101,
		PnL: sdkadapter.PnLEvent{
			DailyPnL:      "150.25",
			UnrealizedPnL: "45",
			RealizedPnL:   "105.25",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(pnl) error = %v", err)
	}
	pnl, ok := msg.(sdkadapter.PnLValue)
	if !ok {
		t.Fatalf("sdkEventToMessage(pnl) type = %T, want sdkadapter.PnLValue", msg)
	}
	if pnl.ReqID != 101 || pnl.DailyPnL != "150.25" || pnl.RealizedPnL != "105.25" {
		t.Fatalf("pnl = %+v, want copied pnl", pnl)
	}

	msg, err = sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventPnLSingle,
		ReqID: 102,
		PnLSingle: sdkadapter.PnLSingleEvent{
			Position:      "10",
			DailyPnL:      "50",
			UnrealizedPnL: "12.5",
			RealizedPnL:   "37.5",
			Value:         "2000",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(pnl single) error = %v", err)
	}
	single, ok := msg.(sdkadapter.PnLSingleValue)
	if !ok {
		t.Fatalf("sdkEventToMessage(pnl single) type = %T, want sdkadapter.PnLSingleValue", msg)
	}
	if single.ReqID != 102 || single.Position != "10" || single.Value != "2000" {
		t.Fatalf("pnl single = %+v, want copied pnl single", single)
	}
}

func receiveAccountUpdatesResult(t *testing.T, resultCh <-chan struct {
	updates []AccountUpdate
	err     error
}) struct {
	updates []AccountUpdate
	err     error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("Accounts().Updates() did not return")
		return struct {
			updates []AccountUpdate
			err     error
		}{}
	}
}

func receiveAccountUpdatesMultiResult(t *testing.T, resultCh <-chan struct {
	values []AccountUpdateMultiValue
	err    error
}) struct {
	values []AccountUpdateMultiValue
	err    error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("Accounts().UpdatesMulti() did not return")
		return struct {
			values []AccountUpdateMultiValue
			err    error
		}{}
	}
}

func receivePnLSubscriptionResult(t *testing.T, resultCh <-chan struct {
	sub *Subscription[PnLUpdate]
	err error
}) struct {
	sub *Subscription[PnLUpdate]
	err error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("Accounts().SubscribePnL() did not return")
		return struct {
			sub *Subscription[PnLUpdate]
			err error
		}{}
	}
}

func receivePnLSingleSubscriptionResult(t *testing.T, resultCh <-chan struct {
	sub *Subscription[PnLSingleUpdate]
	err error
}) struct {
	sub *Subscription[PnLSingleUpdate]
	err error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("Accounts().SubscribePnLSingle() did not return")
		return struct {
			sub *Subscription[PnLSingleUpdate]
			err error
		}{}
	}
}
