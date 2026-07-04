package ibkr

import (
	"context"
	"fmt"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/shopspring/decimal"
)

func (e *engine) ContractDetails(ctx context.Context, contract Contract) ([]ContractDetails, error) {
	type result struct {
		values []ContractDetails
		err    error
	}

	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		reqID = e.allocReqID()
		values := make([]ContractDetails, 0, 4)
		e.keyed[reqID] = &route{
			opKind:       OpContractDetails,
			subscription: false,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.ContractDetails:
					detail, err := fromCodecContractDetails(m)
					if err != nil {
						delete(e.keyed, reqID)
						resp <- result{err: err}
						return
					}
					values = append(values, detail)
				case codec.ContractDetailsEnd:
					delete(e.keyed, reqID)
					resp <- result{values: values}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpContractDetails, m)}
			},
			onDisconnect: func(e *engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.ContractDetailsRequest{
			ReqID:    reqID,
			Contract: toCodecContract(contract),
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.values, out.err
}

func (e *engine) QualifyContract(ctx context.Context, contract Contract) (ContractDetails, error) {
	details, err := e.ContractDetails(ctx, contract)
	if err != nil {
		return ContractDetails{}, err
	}
	switch len(details) {
	case 0:
		return ContractDetails{}, ErrNoMatch
	case 1:
		return details[0], nil
	default:
		return ContractDetails{}, ErrAmbiguousContract
	}
}

func (e *engine) MatchingSymbols(ctx context.Context, pattern string) ([]MatchingSymbol, error) {
	type result struct {
		symbols []MatchingSymbol
		err     error
	}
	resp := make(chan result, 1)
	var reqID int

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()

		e.keyed[reqID] = &route{
			opKind: OpMatchingSymbols,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.MatchingSymbols:
					eng.deleteKeyedRoute(reqID)
					symbols := make([]MatchingSymbol, len(m.Symbols))
					for i, s := range m.Symbols {
						derivTypes := make([]string, len(s.DerivativeSecTypes))
						copy(derivTypes, s.DerivativeSecTypes)
						symbols[i] = MatchingSymbol{
							ConID: s.ConID, Symbol: s.Symbol, SecType: SecType(s.SecType),
							PrimaryExchange: s.PrimaryExchange, Currency: s.Currency,
							DerivativeSecTypes: derivTypes,
							Description:        s.Description,
							IssuerID:           s.IssuerID,
						}
					}
					resp <- result{symbols: symbols}
				}
			},
			handleAPIErr: func(m codec.APIError, eng *engine) {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: eng.apiErr(OpMatchingSymbols, m)}
			},
			onDisconnect: func(eng *engine, err error) bool {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.MatchingSymbolsRequest{ReqID: reqID, Pattern: pattern}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.symbols, out.err
}

func (e *engine) MarketRule(ctx context.Context, marketRuleID int) (MarketRuleResult, error) {
	type result struct {
		rule MarketRuleResult
		err  error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonMarketRule]; exists {
			resp <- result{err: fmt.Errorf("ibkr: market rule request already in progress")}
			return
		}

		e.singletons[singletonMarketRule] = &route{
			opKind: OpMarketRule,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.MarketRule:
					delete(eng.singletons, singletonMarketRule)
					increments := make([]PriceIncrement, len(m.Increments))
					for i, inc := range m.Increments {
						lowEdge, err := parseRequiredDecimal(inc.LowEdge, "market rule low edge")
						if err != nil {
							delete(eng.singletons, singletonMarketRule)
							resp <- result{err: err}
							return
						}
						increment, err := parseRequiredDecimal(inc.Increment, "market rule increment")
						if err != nil {
							delete(eng.singletons, singletonMarketRule)
							resp <- result{err: err}
							return
						}
						increments[i] = PriceIncrement{
							LowEdge:   lowEdge,
							Increment: increment,
						}
					}
					resp <- result{rule: MarketRuleResult{
						MarketRuleID: m.MarketRuleID,
						Increments:   increments,
					}}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonMarketRule)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.MarketRuleRequest{MarketRuleID: marketRuleID}); err != nil {
			delete(e.singletons, singletonMarketRule)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonMarketRule) })
	})
	if err != nil {
		return MarketRuleResult{}, err
	}
	return out.rule, out.err
}

func (e *engine) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
	type result struct {
		values []SecDefOptParams
		err    error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()
		values := make([]SecDefOptParams, 0, 4)
		e.keyed[reqID] = &route{
			opKind: OpSecDefOptParams,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.SecDefOptParamsResponse:
					strikes := make([]decimal.Decimal, len(m.Strikes))
					for i, s := range m.Strikes {
						strike, err := parseRequiredDecimal(s, "sec def opt params strike")
						if err != nil {
							delete(e.keyed, reqID)
							resp <- result{err: err}
							return
						}
						strikes[i] = strike
					}
					values = append(values, SecDefOptParams{
						Exchange:        m.Exchange,
						UnderlyingConID: m.UnderlyingConID,
						TradingClass:    m.TradingClass,
						Multiplier:      m.Multiplier,
						Expirations:     append([]string(nil), m.Expirations...),
						Strikes:         strikes,
					})
				case codec.SecDefOptParamsEnd:
					delete(e.keyed, reqID)
					resp <- result{values: values}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSecDefOptParams, m)}
			},
			onDisconnect: func(e *engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.SecDefOptParamsRequest{
			ReqID:             reqID,
			UnderlyingSymbol:  req.UnderlyingSymbol,
			FutFopExchange:    req.FutFopExchange,
			UnderlyingSecType: string(req.UnderlyingSecType),
			UnderlyingConID:   req.UnderlyingConID,
		}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.values, out.err
}

func (e *engine) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
	type result struct {
		components []SmartComponent
		err        error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = &route{
			opKind: OpSmartComponents,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.SmartComponentsResponse:
					delete(e.keyed, reqID)
					components := make([]SmartComponent, len(m.Components))
					for i, c := range m.Components {
						components[i] = SmartComponent{
							BitNumber:      c.BitNumber,
							ExchangeName:   c.ExchangeName,
							ExchangeLetter: c.ExchangeLetter,
						}
					}
					resp <- result{components: components}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSmartComponents, m)}
			},
			onDisconnect: func(e *engine, err error) bool {
				delete(e.keyed, reqID)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.SmartComponentsRequest{ReqID: reqID, BBOExchange: bboExchange}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return nil, err
	}
	return out.components, out.err
}

func fromCodecContractDetails(m codec.ContractDetails) (ContractDetails, error) {
	minTick, err := parseOptionalDecimal(m.MinTick, "contract details min tick")
	if err != nil {
		return ContractDetails{}, err
	}
	return ContractDetails{
		Contract:   fromCodecContract(m.Contract),
		MarketName: m.MarketName,
		LongName:   m.LongName,
		MinTick:    minTick,
		TimeZoneID: m.TimeZoneID,
	}, nil
}
