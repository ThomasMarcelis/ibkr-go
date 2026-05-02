package ibkr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
	"github.com/shopspring/decimal"
)

const (
	optionCalculationsFixturePath          = "internal/sdkadapter/testdata/fixtures/official_sdk_option_calculations_short_20260502.json"
	qualifiedOptionCalculationsFixturePath = "internal/sdkadapter/testdata/fixtures/official_sdk_option_calculations_qualified_20260502.json"
)

func TestSDKOptionImpliedVolatilityPublicRouteReplaysExpectedErrorFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		value OptionComputation
		err   error
	}, 1)
	go func() {
		value, err := client.Options().ImpliedVolatility(ctx, CalcImpliedVolatilityRequest{
			Contract:    aaplOptionContractForSDKTest(),
			OptionPrice: decimal.RequireFromString("5.25"),
			UnderPrice:  decimal.NewFromInt(200),
		})
		resultCh <- struct {
			value OptionComputation
			err   error
		}{value: value, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandCalcImpliedVolatility ||
		command.CalcImpliedVolatility.Contract.Symbol != "AAPL" ||
		command.CalcImpliedVolatility.OptionPrice != "5.25" ||
		command.CalcImpliedVolatility.UnderPrice != "200" {
		t.Fatalf("implied volatility command = %+v, want AAPL option calculation", command)
	}

	event := fixtureEvent(t, optionCalculationsFixturePath, sdkadapter.EventAPIError, 1001)
	event.ReqID = command.CalcImpliedVolatility.ReqID
	event.APIError.ReqID = command.CalcImpliedVolatility.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	result := receiveOptionComputationResult(t, resultCh, "Options().ImpliedVolatility")
	assertOptionCalculationExpectedError(t, result.err, OpCalcImpliedVol)
}

func TestSDKOptionPricePublicRouteReplaysExpectedErrorFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		value OptionComputation
		err   error
	}, 1)
	go func() {
		value, err := client.Options().Price(ctx, CalcOptionPriceRequest{
			Contract:   aaplOptionContractForSDKTest(),
			Volatility: decimal.RequireFromString("0.3"),
			UnderPrice: decimal.NewFromInt(200),
		})
		resultCh <- struct {
			value OptionComputation
			err   error
		}{value: value, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandCalcOptionPrice ||
		command.CalcOptionPrice.Contract.Symbol != "AAPL" ||
		command.CalcOptionPrice.Volatility != "0.3" ||
		command.CalcOptionPrice.UnderPrice != "200" {
		t.Fatalf("option price command = %+v, want AAPL option calculation", command)
	}

	event := fixtureEvent(t, optionCalculationsFixturePath, sdkadapter.EventAPIError, 1002)
	event.ReqID = command.CalcOptionPrice.ReqID
	event.APIError.ReqID = command.CalcOptionPrice.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	result := receiveOptionComputationResult(t, resultCh, "Options().Price")
	assertOptionCalculationExpectedError(t, result.err, OpCalcOptionPrice)
}

func TestSDKOptionImpliedVolatilityPublicRouteReplaysQualifiedFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		value OptionComputation
		err   error
	}, 1)
	go func() {
		value, err := client.Options().ImpliedVolatility(ctx, CalcImpliedVolatilityRequest{
			Contract:    aaplQualifiedOptionContractForSDKTest(),
			OptionPrice: decimal.RequireFromString("5.25"),
			UnderPrice:  decimal.NewFromInt(200),
		})
		resultCh <- struct {
			value OptionComputation
			err   error
		}{value: value, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandCalcImpliedVolatility ||
		command.CalcImpliedVolatility.Contract.Expiry != "20260618" ||
		command.CalcImpliedVolatility.Contract.TradingClass != "AAPL" ||
		command.CalcImpliedVolatility.OptionPrice != "5.25" ||
		command.CalcImpliedVolatility.UnderPrice != "200" {
		t.Fatalf("implied volatility command = %+v, want qualified AAPL option calculation", command)
	}

	event := fixtureEvent(t, qualifiedOptionCalculationsFixturePath, sdkadapter.EventTickOptionComputation, 1001)
	event.ReqID = command.CalcImpliedVolatility.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	result := receiveOptionComputationResult(t, resultCh, "Options().ImpliedVolatility")
	if result.err != nil {
		t.Fatalf("Options().ImpliedVolatility() error = %v", result.err)
	}
	if !result.value.ImpliedVol.Equal(decimal.RequireFromString("0.17100140275259834")) ||
		!result.value.OptPrice.Equal(decimal.RequireFromString("5.25")) ||
		!result.value.UndPrice.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("Options().ImpliedVolatility() = %+v, want live-derived computation", result.value)
	}
}

func TestSDKOptionPricePublicRouteReplaysQualifiedFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		value OptionComputation
		err   error
	}, 1)
	go func() {
		value, err := client.Options().Price(ctx, CalcOptionPriceRequest{
			Contract:   aaplQualifiedOptionContractForSDKTest(),
			Volatility: decimal.RequireFromString("0.3"),
			UnderPrice: decimal.NewFromInt(200),
		})
		resultCh <- struct {
			value OptionComputation
			err   error
		}{value: value, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandCalcOptionPrice ||
		command.CalcOptionPrice.Contract.Expiry != "20260618" ||
		command.CalcOptionPrice.Contract.TradingClass != "AAPL" ||
		command.CalcOptionPrice.Volatility != "0.3" ||
		command.CalcOptionPrice.UnderPrice != "200" {
		t.Fatalf("option price command = %+v, want qualified AAPL option calculation", command)
	}

	event := fixtureEvent(t, qualifiedOptionCalculationsFixturePath, sdkadapter.EventTickOptionComputation, 1002)
	event.ReqID = command.CalcOptionPrice.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	result := receiveOptionComputationResult(t, resultCh, "Options().Price")
	if result.err != nil {
		t.Fatalf("Options().Price() error = %v", result.err)
	}
	if !result.value.ImpliedVol.Equal(decimal.RequireFromString("0.29999999999999999")) ||
		!result.value.OptPrice.Equal(decimal.RequireFromString("8.9158022449933707")) ||
		!result.value.UndPrice.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("Options().Price() = %+v, want live-derived computation", result.value)
	}
}

func TestSDKOptionCalculationsUseSDKCommandsAndEvents(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:     "AAPL",
		SecType:    "OPT",
		Expiry:     "20260619",
		Strike:     "200",
		Right:      "C",
		Exchange:   "SMART",
		Currency:   "USD",
		Multiplier: "100",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CalcImpliedVolatilityRequest{
		ReqID:       71,
		Contract:    contract,
		OptionPrice: "5.25",
		UnderPrice:  "200",
	}); err != nil {
		t.Fatalf("sendSDKContext(CalcImpliedVolatilityRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelCalcImpliedVolatility{ReqID: 71}); err != nil {
		t.Fatalf("sendSDKContext(CancelCalcImpliedVolatility) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CalcOptionPriceRequest{
		ReqID:      72,
		Contract:   contract,
		Volatility: "0.3",
		UnderPrice: "200",
	}); err != nil {
		t.Fatalf("sendSDKContext(CalcOptionPriceRequest) error = %v", err)
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.CancelCalcOptionPrice{ReqID: 72}); err != nil {
		t.Fatalf("sendSDKContext(CancelCalcOptionPrice) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 4 {
		t.Fatalf("commands len = %d, want 4", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandCalcImpliedVolatility {
		t.Fatalf("implied vol command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandCalcImpliedVolatility)
	}
	if commands[0].CalcImpliedVolatility.ReqID != 71 || commands[0].CalcImpliedVolatility.OptionPrice != "5.25" || commands[0].CalcImpliedVolatility.UnderPrice != "200" {
		t.Fatalf("implied vol command = %+v, want reqID 71 prices", commands[0].CalcImpliedVolatility)
	}
	if commands[0].CalcImpliedVolatility.Contract.Symbol != "AAPL" || commands[0].CalcImpliedVolatility.Contract.SecType != "OPT" {
		t.Fatalf("implied vol contract = %+v, want AAPL OPT", commands[0].CalcImpliedVolatility.Contract)
	}
	if commands[1].Kind != sdkadapter.CommandCancelCalcImpliedVol || commands[1].CancelCalcImpliedVol.ReqID != 71 {
		t.Fatalf("cancel implied vol command = %+v, want reqID 71", commands[1])
	}
	if commands[2].Kind != sdkadapter.CommandCalcOptionPrice {
		t.Fatalf("option price command kind = %s, want %s", commands[2].Kind, sdkadapter.CommandCalcOptionPrice)
	}
	if commands[2].CalcOptionPrice.ReqID != 72 || commands[2].CalcOptionPrice.Volatility != "0.3" || commands[2].CalcOptionPrice.UnderPrice != "200" {
		t.Fatalf("option price command = %+v, want reqID 72 vol/under", commands[2].CalcOptionPrice)
	}
	if commands[3].Kind != sdkadapter.CommandCancelCalcOptionPrice || commands[3].CancelCalcOptionPrice.ReqID != 72 {
		t.Fatalf("cancel option price command = %+v, want reqID 72", commands[3])
	}

	msg, err := sdkEventToMessage(sdkadapter.Event{
		Kind:  sdkadapter.EventTickOptionComputation,
		ReqID: 71,
		TickOptionComputation: sdkadapter.TickOptionComputationValue{
			TickType:   13,
			TickAttrib: 1,
			ImpliedVol: "0.25",
			Delta:      "0.5",
			OptPrice:   "5.25",
			PvDividend: "0.1",
			Gamma:      "0.02",
			Vega:       "0.15",
			Theta:      "-0.05",
			UndPrice:   "200",
		},
	})
	if err != nil {
		t.Fatalf("sdkEventToMessage(tick option computation) error = %v", err)
	}
	got, ok := msg.(sdkadapter.TickOptionComputation)
	if !ok {
		t.Fatalf("sdkEventToMessage(tick option computation) type = %T, want sdkadapter.TickOptionComputation", msg)
	}
	if got.ReqID != 71 || got.TickType != 13 || got.ImpliedVol != "0.25" || got.OptPrice != "5.25" {
		t.Fatalf("tick option computation = %+v, want copied option values", got)
	}
}

func TestSDKExerciseOptionsPublicRouteUsesSDKCommand(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- client.Options().Exercise(ctx, ExerciseOptionsRequest{
			Contract:         aaplOptionContractForSDKTest(),
			ExerciseAction:   Lapse,
			ExerciseQuantity: 1,
			Account:          "DU_REDACTED",
			Override:         true,
		})
	}()

	runNextEngineCommand(t, e)
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("Options().Exercise() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Options().Exercise() did not return")
	}

	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandExerciseOptions ||
		command.ExerciseOptions.Contract.Symbol != "AAPL" ||
		command.ExerciseOptions.ExerciseAction != int(Lapse) ||
		command.ExerciseOptions.ExerciseQuantity != 1 ||
		command.ExerciseOptions.Account != "DU_REDACTED" ||
		command.ExerciseOptions.Override != 1 {
		t.Fatalf("exercise options command = %+v, want redacted lapse request", command)
	}
}

func TestSDKExerciseOptionsUsesSDKCommand(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := &engine{adapter: adapter}
	contract := sdkadapter.Contract{
		Symbol:     "AAPL",
		SecType:    "OPT",
		Expiry:     "20260619",
		Strike:     "200",
		Right:      "C",
		Exchange:   "SMART",
		Currency:   "USD",
		Multiplier: "100",
	}
	if err := e.sendSDKContext(context.Background(), sdkadapter.ExerciseOptionsRequest{
		ReqID:            73,
		Contract:         contract,
		ExerciseAction:   int(Lapse),
		ExerciseQuantity: 1,
		Account:          "DU12345",
		Override:         1,
	}); err != nil {
		t.Fatalf("sendSDKContext(ExerciseOptionsRequest) error = %v", err)
	}

	commands := adapter.Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(commands))
	}
	if commands[0].Kind != sdkadapter.CommandExerciseOptions {
		t.Fatalf("exercise options command kind = %s, want %s", commands[0].Kind, sdkadapter.CommandExerciseOptions)
	}
	got := commands[0].ExerciseOptions
	if got.ReqID != 73 || got.ExerciseAction != int(Lapse) || got.ExerciseQuantity != 1 || got.Account != "DU12345" || got.Override != 1 {
		t.Fatalf("exercise options command = %+v, want copied request fields", got)
	}
	if got.Contract.Symbol != "AAPL" || got.Contract.SecType != "OPT" || got.Contract.Right != "C" {
		t.Fatalf("exercise options contract = %+v, want AAPL OPT call", got.Contract)
	}
}

func aaplOptionContractForSDKTest() Contract {
	return Contract{
		Symbol:     "AAPL",
		SecType:    SecTypeOption,
		Expiry:     "20260619",
		Strike:     "200",
		Right:      RightCall,
		Exchange:   "SMART",
		Currency:   "USD",
		Multiplier: "100",
	}
}

func aaplQualifiedOptionContractForSDKTest() Contract {
	return Contract{
		Symbol:       "AAPL",
		SecType:      SecTypeOption,
		Expiry:       "20260618",
		Strike:       "200",
		Right:        RightCall,
		Exchange:     "SMART",
		Currency:     "USD",
		Multiplier:   "100",
		TradingClass: "AAPL",
	}
}

func receiveOptionComputationResult(t *testing.T, resultCh <-chan struct {
	value OptionComputation
	err   error
}, name string) struct {
	value OptionComputation
	err   error
} {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatalf("%s did not return", name)
		return struct {
			value OptionComputation
			err   error
		}{}
	}
}

func assertOptionCalculationExpectedError(t *testing.T, err error, wantOp OpKind) {
	t.Helper()

	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.Code != 200 ||
		apiErr.OpKind != wantOp ||
		apiErr.Message != "No security definition has been found for the request" {
		t.Fatalf("APIError = %+v, want code 200 %s no-security-definition", apiErr, wantOp)
	}
}
