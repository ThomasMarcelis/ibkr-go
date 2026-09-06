package ibkr

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/shopspring/decimal"
)

func (e *engine) ContractDetails(ctx context.Context, contract Contract) ([]ContractDetails, error) {
	sub, err := e.StreamContractDetails(ctx, contract, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	return collectSnapshotAndClose(ctx, sub, func(detail ContractDetails) (ContractDetails, bool) { return detail, true })
}

func (e *engine) StreamContractDetails(ctx context.Context, contract Contract, opts ...SubscriptionOption) (*Subscription[ContractDetails], error) {
	if err := validateContract(contract); err != nil {
		return nil, err
	}
	contract = cloneContract(contract)
	type result struct {
		sub *Subscription[ContractDetails]
		err error
	}

	resp := make(chan result, 1)
	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if err := validateContractFieldSupport(contract, "contract details", e.serverVersion, contractFieldsAll); err != nil {
			resp <- result{err: err}
			return
		}
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpContractDetails, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}

		reqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		var cancel codec.OutboundMessage
		if e.serverVersion >= protocol.MinServerVersionBrokerSideOneShotCancel {
			cancel = codec.CancelContractData{ReqID: reqID}
		}
		sub, ownedRoute := newKeyedSubscriptionRoute[ContractDetails](e, cfg, reqID, OpContractDetails, cancel)
		sub.expectSnapshot()
		ownedRoute.onDisconnect = func(_ *engine, err error) bool {
			sub.closeWithErr(interrupted(err))
			return false
		}
		ownedRoute.handle = func(msg any, e *engine) {
			var detail ContractDetails
			var err error
			switch m := msg.(type) {
			case codec.ContractDetails:
				detail, err = fromCodecContractDetails(m)
			case codec.BondContractDetails:
				detail, err = fromCodecBondContractDetails(m)
			case codec.ContractDetailsEnd:
				e.deleteKeyedRoute(reqID)
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
				sub.closeWithErr(nil)
				return
			default:
				return
			}
			if err != nil {
				sub.cancelFromActor(err)
				return
			}
			sub.emit(detail)
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, codec.ContractDetailsRequest{
			ReqID:    reqID,
			Contract: toCodecContract(contract),
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) bool { return out.sub != nil })
	if err != nil {
		return nil, err
	}
	if out.err == nil && out.sub != nil {
		bindContext(ctx, out.sub)
	}
	return out.sub, out.err
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
		allocatedReqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID = allocatedReqID

		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpMatchingSymbols,
			func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.MatchingSymbols:
					eng.deleteKeyedRoute(reqID)
					symbols := make([]MatchingSymbol, len(m.Symbols))
					for i, s := range m.Symbols {
						derivTypes := make([]string, len(s.DerivativeSecTypes))
						copy(derivTypes, s.DerivativeSecTypes)
						symbols[i] = MatchingSymbol{
							ConID: protocolIDFromInt[ContractID](s.ConID), Symbol: s.Symbol, SecType: SecType(s.SecType),
							PrimaryExchange: s.PrimaryExchange, Currency: s.Currency,
							DerivativeSecTypes: derivTypes,
							Description:        s.Description,
							IssuerID:           s.IssuerID,
						}
					}
					resp <- result{symbols: symbols}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
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

func (e *engine) MarketRule(ctx context.Context, marketRuleID MarketRuleID) (MarketRuleResult, error) {
	if marketRuleID <= 0 {
		return MarketRuleResult{}, invalidOrderField("MarketRuleID", marketRuleID, "must be positive; zero means no market rule")
	}
	type result struct {
		rule MarketRuleResult
		err  error
	}
	resp := make(chan result, 1)
	var ownedRoute *route

	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonMarketRule]; exists {
			resp <- result{err: operationActive("market rule")}
			return
		}

		ownedRoute = &route{
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.MarketRule:
					if protocolIDFromInt[MarketRuleID](m.MarketRuleID) != marketRuleID {
						return
					}
					delete(eng.singletons, singletonMarketRule)
					increments := make([]PriceIncrement, len(m.Increments))
					for i, inc := range m.Increments {
						lowEdge, err := parseRequiredDecimal(inc.LowEdge, "market rule low edge")
						if err != nil {
							resp <- result{err: err}
							return
						}
						increment, err := parseRequiredDecimal(inc.Increment, "market rule increment")
						if err != nil {
							resp <- result{err: err}
							return
						}
						increments[i] = PriceIncrement{
							LowEdge:   lowEdge,
							Increment: increment,
						}
					}
					resp <- result{rule: MarketRuleResult{
						MarketRuleID: protocolIDFromInt[MarketRuleID](m.MarketRuleID),
						Increments:   increments,
					}}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: interrupted(err)}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		e.singletons[singletonMarketRule] = ownedRoute
		if err := e.sendContext(ctx, codec.MarketRuleRequest{MarketRuleID: int(marketRuleID)}); err != nil {
			delete(e.singletons, singletonMarketRule)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.abortUnresolvedSingletonOneShot(singletonMarketRule, ownedRoute) })
	})
	if err != nil {
		return MarketRuleResult{}, err
	}
	return out.rule, out.err
}

func (e *engine) SecDefOptParams(ctx context.Context, req SecDefOptParamsRequest) ([]SecDefOptParams, error) {
	sub, err := e.StreamSecDefOptParams(ctx, req, withSnapshotCollector())
	if err != nil {
		return nil, err
	}
	return collectSnapshotAndClose(ctx, sub, func(params SecDefOptParams) (SecDefOptParams, bool) { return params, true })
}

func (e *engine) StreamSecDefOptParams(ctx context.Context, req SecDefOptParamsRequest, opts ...SubscriptionOption) (*Subscription[SecDefOptParams], error) {
	type result struct {
		sub *Subscription[SecDefOptParams]
		err error
	}
	resp := make(chan result, 1)
	enqueueSubscriptionSetup(ctx, e, resp, func() {
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpSecDefOptParams, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		sub, ownedRoute := newKeyedSubscriptionRoute[SecDefOptParams](e, cfg, reqID, OpSecDefOptParams, nil)
		sub.expectSnapshot()
		ownedRoute.onDisconnect = func(_ *engine, err error) bool {
			sub.closeWithErr(interrupted(err))
			return false
		}
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.SecDefOptParamsResponse:
				strikes := make([]decimal.Decimal, len(m.Strikes))
				for i, s := range m.Strikes {
					strike, err := parseRequiredDecimal(s, "sec def opt params strike")
					if err != nil {
						sub.cancelFromActor(err)
						return
					}
					strikes[i] = strike
				}
				sub.emit(SecDefOptParams{
					Exchange:        m.Exchange,
					UnderlyingConID: protocolIDFromInt[ContractID](m.UnderlyingConID),
					TradingClass:    m.TradingClass,
					Multiplier:      m.Multiplier,
					Expirations:     append([]string(nil), m.Expirations...),
					Strikes:         strikes,
				})
			case codec.SecDefOptParamsEnd:
				e.deleteKeyedRoute(reqID)
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
				sub.closeWithErr(nil)
			}
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, codec.SecDefOptParamsRequest{
			ReqID:             reqID,
			UnderlyingSymbol:  req.UnderlyingSymbol,
			FutFopExchange:    req.FutFopExchange,
			UnderlyingSecType: string(req.UnderlyingSecType),
			UnderlyingConID:   int(req.UnderlyingConID),
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) bool { return out.sub != nil })
	if err != nil {
		return nil, err
	}
	if out.err == nil && out.sub != nil {
		bindContext(ctx, out.sub)
	}
	return out.sub, out.err
}

func (e *engine) SmartComponents(ctx context.Context, bboExchange string) ([]SmartComponent, error) {
	type result struct {
		components []SmartComponent
		err        error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		allocatedReqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID = allocatedReqID
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpSmartComponents,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.SmartComponentsResponse:
					e.deleteKeyedRoute(reqID)
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
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.SmartComponentsRequest{ReqID: reqID, BBOExchange: bboExchange}); err != nil {
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
	return out.components, out.err
}

func fromCodecContractDetails(m codec.ContractDetails) (ContractDetails, error) {
	contract, err := fromCodecContract(m.Contract)
	if err != nil {
		return ContractDetails{}, err
	}
	minTick, err := parseOptionalDecimal(m.MinTick, "contract details min tick")
	if err != nil {
		return ContractDetails{}, err
	}
	economicValueMultiplier, err := parseOptionalDecimalPointer(m.EconomicValueMultiplier, "contract details economic value multiplier")
	if err != nil {
		return ContractDetails{}, err
	}
	minSize, err := parseOptionalDecimalPointer(m.MinSize, "contract details minimum size")
	if err != nil {
		return ContractDetails{}, err
	}
	sizeIncrement, err := parseOptionalDecimalPointer(m.SizeIncrement, "contract details size increment")
	if err != nil {
		return ContractDetails{}, err
	}
	suggestedSizeIncrement, err := parseOptionalDecimalPointer(m.SuggestedSizeIncrement, "contract details suggested size increment")
	if err != nil {
		return ContractDetails{}, err
	}
	minAlgoSize, err := parseOptionalDecimalPointer(m.MinAlgoSize, "contract details minimum algorithmic size")
	if err != nil {
		return ContractDetails{}, err
	}
	lastPricePrecision, err := parseOptionalDecimalPointer(m.LastPricePrecision, "contract details last price precision")
	if err != nil {
		return ContractDetails{}, err
	}
	lastSizePrecision, err := parseOptionalDecimalPointer(m.LastSizePrecision, "contract details last size precision")
	if err != nil {
		return ContractDetails{}, err
	}
	var aggGroup *AggregateGroupID
	if m.AggGroup != math.MaxInt32 {
		aggGroup = new(protocolIDFromInt[AggregateGroupID](m.AggGroup))
	}

	exchanges, err := contractExchanges(m.ValidExchanges, m.MarketRuleIDs)
	if err != nil {
		return ContractDetails{}, err
	}
	var securityIDs []TagValue
	if len(m.SecurityIDs) > 0 {
		securityIDs = make([]TagValue, len(m.SecurityIDs))
	}
	for i, id := range m.SecurityIDs {
		securityIDs[i] = TagValue{Tag: id.Tag, Value: id.Value}
	}
	var ineligibilityReasons []IneligibilityReason
	if len(m.IneligibilityReasons) > 0 {
		ineligibilityReasons = make([]IneligibilityReason, len(m.IneligibilityReasons))
	}
	for i, reason := range m.IneligibilityReasons {
		ineligibilityReasons[i] = IneligibilityReason{ID: reason.ID, Description: reason.Description}
	}
	var fund *FundDetails
	if m.Fund != nil {
		fund = &FundDetails{
			Name:                      m.Fund.Name,
			Family:                    m.Fund.Family,
			Type:                      m.Fund.Type,
			FrontLoad:                 m.Fund.FrontLoad,
			BackLoad:                  m.Fund.BackLoad,
			BackLoadTimeInterval:      m.Fund.BackLoadTimeInterval,
			ManagementFee:             m.Fund.ManagementFee,
			Closed:                    m.Fund.Closed,
			ClosedForNewInvestors:     m.Fund.ClosedForNewInvestors,
			ClosedForNewMoney:         m.Fund.ClosedForNewMoney,
			NotifyAmount:              m.Fund.NotifyAmount,
			MinimumInitialPurchase:    m.Fund.MinimumInitialPurchase,
			MinimumSubsequentPurchase: m.Fund.MinimumSubsequentPurchase,
			BlueSkyStates:             m.Fund.BlueSkyStates,
			BlueSkyTerritories:        m.Fund.BlueSkyTerritories,
			DistributionPolicy:        m.Fund.DistributionPolicy,
			AssetType:                 m.Fund.AssetType,
		}
	}

	return ContractDetails{
		Contract:                  contract,
		MarketName:                m.MarketName,
		LongName:                  m.LongName,
		MinTick:                   minTick,
		PriceMagnifier:            m.PriceMagnifier,
		OrderTypes:                splitContractList(m.OrderTypes),
		ValidExchanges:            exchanges,
		UnderConID:                protocolIDFromInt[ContractID](m.UnderConID),
		ContractMonth:             m.ContractMonth,
		Industry:                  m.Industry,
		Category:                  m.Category,
		Subcategory:               m.Subcategory,
		TimeZoneID:                m.TimeZoneID,
		TradingHours:              m.TradingHours,
		LiquidHours:               m.LiquidHours,
		EconomicValueRule:         m.EconomicValueRule,
		EconomicValueMultiplier:   economicValueMultiplier,
		SecurityIDs:               securityIDs,
		AggGroup:                  aggGroup,
		UnderSymbol:               m.UnderSymbol,
		UnderSecType:              SecType(m.UnderSecType),
		RealExpirationDate:        m.RealExpirationDate,
		LastTradeDate:             m.LastTradeDate,
		LastTradeTime:             m.LastTradeTime,
		StockType:                 m.StockType,
		SettlementMethod:          m.SettlementMethod,
		MinSize:                   minSize,
		SizeIncrement:             sizeIncrement,
		SuggestedSizeIncrement:    suggestedSizeIncrement,
		EventContract1:            m.EventContract1,
		EventContractDescription1: m.EventContractDescription1,
		EventContractDescription2: m.EventContractDescription2,
		MinAlgoSize:               minAlgoSize,
		LastPricePrecision:        lastPricePrecision,
		LastSizePrecision:         lastSizePrecision,
		Fund:                      fund,
		IneligibilityReasons:      ineligibilityReasons,
	}, nil
}

func fromCodecBondContractDetails(m codec.BondContractDetails) (ContractDetails, error) {
	detail, err := fromCodecContractDetails(m.ContractDetails)
	if err != nil {
		return ContractDetails{}, err
	}
	coupon, err := parseOptionalDecimalPointer(m.Coupon, "bond contract coupon")
	if err != nil {
		return ContractDetails{}, err
	}
	detail.Bond = &BondDetails{
		CUSIP: m.CUSIP, Coupon: coupon, Maturity: m.Maturity,
		IssueDate: m.IssueDate, Ratings: m.Ratings, Type: m.BondType,
		CouponType: m.CouponType, Convertible: m.Convertible,
		Callable: m.Callable, Putable: m.Putable,
		DescriptionAppend: m.DescriptionAppend,
		NextOptionDate:    m.NextOptionDate, NextOptionType: m.NextOptionType,
		NextOptionPartial: m.NextOptionPartial, Notes: m.Notes,
	}
	return detail, nil
}

func contractExchanges(validExchanges, marketRuleIDs string) ([]ContractExchange, error) {
	exchanges := splitContractList(validExchanges)
	rules := splitContractList(marketRuleIDs)
	if len(exchanges) != len(rules) {
		return nil, inboundProtocolError("contract details exchanges", fmt.Errorf("%d valid exchanges but %d market rule ids", len(exchanges), len(rules)))
	}
	result := make([]ContractExchange, len(exchanges))
	for i := range exchanges {
		marketRuleID, err := parseOptionalInt32(rules[i], "contract details market rule ID")
		if err != nil {
			return nil, err
		}
		result[i] = ContractExchange{Exchange: exchanges[i], MarketRuleID: MarketRuleID(marketRuleID)}
	}
	return result, nil
}

func splitContractList(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}
