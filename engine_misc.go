package ibkr

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

const scannerNoItemsMessage = "Historical Market Data Service query message:no items retrieved"

func (e *engine) CurrentTime(ctx context.Context) (time.Time, error) {
	type result struct {
		ts  time.Time
		err error
	}
	resp := make(chan result, 1)
	var ownedRoute *route

	enqueueClockSetup(ctx, e, singletonCurrentTime, nil, func() {
		resp <- result{err: operationActive("current time")}
	}, func() {
		// No handleAPIErr: req_current_time carries no reqID, so the engine
		// cannot route an APIError to this singleton. ctx cancellation and
		// onDisconnect are the only failure paths.
		ownedRoute = &route{
			opKind: OpCurrentTime,
			handle: func(msg any, eng *engine) {
				m, ok := msg.(codec.CurrentTime)
				if !ok {
					return
				}
				delete(eng.singletons, singletonCurrentTime)
				ts, parseErr := parseEpochSeconds(m.Time)
				if parseErr != nil {
					resp <- result{err: fmt.Errorf("ibkr: current time: %w", parseErr)}
					return
				}
				resp <- result{ts: ts}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		e.singletons[singletonCurrentTime] = ownedRoute
		if err := e.sendContext(ctx, codec.CurrentTimeRequest{}); err != nil {
			delete(e.singletons, singletonCurrentTime)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.cancelSingletonOneShot(singletonCurrentTime, ownedRoute) })
	})
	if err != nil {
		return time.Time{}, err
	}
	return out.ts, out.err
}

func (e *engine) CurrentTimeMillis(ctx context.Context) (time.Time, error) {
	type result struct {
		ts  time.Time
		err error
	}
	resp := make(chan result, 1)
	var ownedRoute *route

	enqueueClockSetup(ctx, e, singletonCurrentTimeMillis, nil, func() {
		resp <- result{err: operationActive("current time millis")}
	}, func() {
		// Like reqCurrentTime, the request carries no reqID, so APIErrors
		// cannot route here; ctx cancellation and onDisconnect are the only
		// failure paths.
		ownedRoute = &route{
			opKind: OpCurrentTime,
			handle: func(msg any, eng *engine) {
				m, ok := msg.(codec.CurrentTimeMillis)
				if !ok {
					return
				}
				delete(eng.singletons, singletonCurrentTimeMillis)
				ts, parseErr := parseEpochMilliseconds(m.TimeMs)
				if parseErr != nil {
					resp <- result{err: fmt.Errorf("ibkr: current time millis: %w", parseErr)}
					return
				}
				resp <- result{ts: ts}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		e.singletons[singletonCurrentTimeMillis] = ownedRoute
		if err := e.sendContext(ctx, codec.CurrentTimeMillisRequest{}); err != nil {
			delete(e.singletons, singletonCurrentTimeMillis)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.cancelSingletonOneShot(singletonCurrentTimeMillis, ownedRoute) })
	})
	if err != nil {
		return time.Time{}, err
	}
	return out.ts, out.err
}

func (e *engine) ScannerParameters(ctx context.Context) (string, error) {
	type result struct {
		xml string
		err error
	}
	resp := make(chan result, 1)
	var ownedRoute *route

	enqueueOneShotSetup(ctx, e, func() {
		if _, exists := e.singletons[singletonScannerParameters]; exists {
			resp <- result{err: operationActive("scanner parameters")}
			return
		}

		ownedRoute = &route{
			opKind: OpScannerParameters,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.ScannerParameters:
					delete(eng.singletons, singletonScannerParameters)
					resp <- result{xml: m.XML}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		e.singletons[singletonScannerParameters] = ownedRoute
		if err := e.sendContext(ctx, codec.ScannerParametersRequest{}); err != nil {
			delete(e.singletons, singletonScannerParameters)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.cancelSingletonOneShot(singletonScannerParameters, ownedRoute) })
	})
	if err != nil {
		return "", err
	}
	return out.xml, out.err
}

func (e *engine) UserInfo(ctx context.Context) (string, error) {
	type result struct {
		whiteBrandingID string
		err             error
	}
	resp := make(chan result, 1)
	var reqID int

	enqueueOneShotSetup(ctx, e, func() {
		reqID = e.allocReqID()

		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpUserInfo,
			func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.UserInfo:
					eng.deleteKeyedRoute(reqID)
					resp <- result{whiteBrandingID: m.WhiteBrandingID}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.UserInfoRequest{ReqID: reqID}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return "", err
	}
	return out.whiteBrandingID, out.err
}

func (e *engine) SubscribeScannerResults(ctx context.Context, req ScannerSubscriptionRequest, opts ...SubscriptionOption) (*Subscription[[]ScannerResult], error) {
	if err := validateScannerSubscriptionRequest(req); err != nil {
		return nil, err
	}
	req = cloneScannerSubscriptionRequest(req)

	type result struct {
		sub *Subscription[[]ScannerResult]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpScannerSubscription, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		sub, ownedRoute := newKeyedSubscriptionRoute[[]ScannerResult](
			e, cfg, reqID, OpScannerSubscription, codec.CancelScannerSubscription{ReqID: reqID},
		)

		ownedRoute.request = toCodecScannerSubscriptionRequest(reqID, req)
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.ScannerDataResponse:
				results := make([]ScannerResult, len(m.Entries))
				for i, entry := range m.Entries {
					contract, err := fromCodecContract(entry.Contract)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					results[i] = ScannerResult{
						Rank:       entry.Rank,
						Contract:   contract,
						MarketName: entry.MarketName,
						Distance:   entry.Distance,
						Benchmark:  entry.Benchmark,
						Projection: entry.Projection,
						LegsStr:    entry.LegsStr,
					}
				}
				sub.emit(results)
			}
		}
		ownedRoute.handleAPIErr = func(m codec.APIError, e *engine) {
			if e.keyed[reqID] != ownedRoute {
				return
			}
			if m.Code == ErrCodeHistoricalDataQueryMessage && m.Message == scannerNoItemsMessage {
				e.emitAPIEvent(m)
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.apiErr(OpScannerSubscription, m))
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
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

// FA Configuration

func (e *engine) RequestFA(ctx context.Context, faDataType FADataType) (string, error) {
	type result struct {
		xml string
		err error
	}
	resp := make(chan result, 1)
	var ownedRoute *route

	enqueueOneShotSetup(ctx, e, func() {
		if err := validateFADataType(faDataType); err != nil {
			resp <- result{err: err}
			return
		}
		if _, exists := e.singletons[singletonFA]; exists {
			resp <- result{err: operationActive("FA configuration")}
			return
		}

		ownedRoute = &route{
			opKind: OpFAConfig,
			handleAPIErr: func(msg codec.APIError, eng *engine) {
				delete(eng.singletons, singletonFA)
				resp <- result{err: eng.apiErr(OpFAConfig, msg)}
			},
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.ReceiveFA:
					delete(eng.singletons, singletonFA)
					resp <- result{xml: m.XML}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		e.singletons[singletonFA] = ownedRoute
		if err := e.sendContext(ctx, codec.RequestFA{FADataType: int(faDataType)}); err != nil {
			delete(e.singletons, singletonFA)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.cancelSingletonOneShot(singletonFA, ownedRoute) })
	})
	if err != nil {
		return "", err
	}
	return out.xml, out.err
}

// validateFADataType mirrors the official client's FA_PROFILE_NOT_SUPPORTED
// rejection; every server version supported by this package desupports it.
func validateFADataType(faDataType FADataType) error {
	if faDataType != FADataGroups && faDataType != FADataAliases {
		return &ValidationError{
			Field:   "FA data type",
			Value:   faDataType.String(),
			Message: "must be Groups or Aliases",
		}
	}
	return nil
}

func (e *engine) SoftDollarTiers(ctx context.Context) ([]SoftDollarTier, error) {
	type result struct {
		tiers []SoftDollarTier
		err   error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpSoftDollarTiers,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.SoftDollarTiersResponse:
					e.deleteKeyedRoute(reqID)
					tiers := make([]SoftDollarTier, len(m.Tiers))
					for i, t := range m.Tiers {
						tiers[i] = SoftDollarTier{Name: t.Name, Value: t.Value, DisplayName: t.DisplayName}
					}
					resp <- result{tiers: tiers}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.SoftDollarTiersRequest{ReqID: reqID}); err != nil {
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
	return out.tiers, out.err
}

// WSH Calendar Events

func (e *engine) WSHMetaData(ctx context.Context) (string, error) {
	type result struct {
		dataJSON string
		err      error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpWSHMetaData,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.WSHMetaDataResponse:
					e.deleteKeyedRoute(reqID)
					if !json.Valid([]byte(m.DataJSON)) {
						resp <- result{err: fmt.Errorf("ibkr: invalid WSH metadata JSON")}
						return
					}
					resp <- result{dataJSON: m.DataJSON}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.WSHMetaDataRequest{ReqID: reqID}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				cancelErr := e.cancelSubscription(OpWSHMetaData, codec.CancelWSHMetaData{ReqID: reqID})
				e.deleteKeyedRoute(reqID)
				e.retireSubscriptionTransport(cancelErr)
			}
		})
	})
	if err != nil {
		return "", err
	}
	return out.dataJSON, out.err
}

func (e *engine) WSHEventData(ctx context.Context, req WSHEventDataRequest) (string, error) {
	if len(req.Filter) > 0 && !json.Valid(req.Filter) {
		return "", &ValidationError{Field: "WSH filter", Message: "must be valid JSON"}
	}
	req.Filter = append(JSONDocument(nil), req.Filter...)
	type result struct {
		dataJSON string
		err      error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpWSHEventData,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.WSHEventDataResponse:
					e.deleteKeyedRoute(reqID)
					if !json.Valid([]byte(m.DataJSON)) {
						resp <- result{err: fmt.Errorf("ibkr: invalid WSH event-data JSON")}
						return
					}
					resp <- result{dataJSON: m.DataJSON}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.WSHEventDataRequest{
			ReqID:           reqID,
			ConID:           req.ConID,
			Filter:          string(req.Filter),
			FillWatchlist:   req.FillWatchlist,
			FillPortfolio:   req.FillPortfolio,
			FillCompetitors: req.FillCompetitors,
			StartDate:       formatWSHDate(req.StartDate),
			EndDate:         formatWSHDate(req.EndDate),
			TotalLimit:      req.TotalLimit,
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				cancelErr := e.cancelSubscription(OpWSHEventData, codec.CancelWSHEventData{ReqID: reqID})
				e.deleteKeyedRoute(reqID)
				e.retireSubscriptionTransport(cancelErr)
			}
		})
	})
	if err != nil {
		return "", err
	}
	return out.dataJSON, out.err
}

// Display Groups

func (e *engine) QueryDisplayGroups(ctx context.Context) (string, error) {
	type result struct {
		groups string
		err    error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpDisplayGroups,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.DisplayGroupList:
					e.deleteKeyedRoute(reqID)
					resp <- result{groups: m.Groups}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.QueryDisplayGroupsRequest{ReqID: reqID}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return "", err
	}
	return out.groups, out.err
}

func (e *engine) SubscribeDisplayGroup(ctx context.Context, groupID DisplayGroupID, opts ...SubscriptionOption) (*DisplayGroupHandle, error) {
	type result struct {
		sub *Subscription[DisplayGroupUpdate]
		err error
	}
	resp := make(chan result, 1)
	var reqID int

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		cfg, err := applySubscriptionOptionsFor(e.cfg, OpDisplayGroupEvents, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		sub, ownedRoute := newKeyedSubscriptionRoute[DisplayGroupUpdate](
			e, cfg, reqID, OpDisplayGroupEvents, codec.UnsubscribeFromGroupEventsRequest{ReqID: reqID},
		)

		ownedRoute.request = codec.SubscribeToGroupEventsRequest{ReqID: reqID, GroupID: int(groupID)}
		ownedRoute.handle = func(msg any, e *engine) {
			if m, ok := msg.(codec.DisplayGroupUpdated); ok {
				sub.emit(DisplayGroupUpdate{ContractInfo: m.ContractInfo})
			}
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
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
	if out.err != nil {
		return nil, out.err
	}
	handle := &DisplayGroupHandle{
		Subscription: out.sub,
		updateFn: func(ctx context.Context, contractInfo string) error {
			return e.updateDisplayGroup(ctx, reqID, contractInfo)
		},
	}
	return handle, nil
}

func (e *engine) updateDisplayGroup(ctx context.Context, reqID int, contractInfo string) error {
	lookup := make(chan *route, 1)
	enqueueContextSetup(ctx, e, nil, func() {
		route, ok := e.keyed[reqID]
		if !ok || route.opKind != OpDisplayGroupEvents {
			lookup <- nil
			return
		}
		lookup <- route
	})
	ownedRoute, err := awaitOneShotResponse(ctx, e, lookup, nil)
	if err != nil {
		return err
	}
	if ownedRoute == nil {
		return ErrClosed
	}

	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if e.keyed[reqID] != ownedRoute {
			return ErrClosed
		}
		return e.sendContext(ctx, codec.UpdateDisplayGroupRequest{ReqID: reqID, ContractInfo: contractInfo})
	})
}

func parseEpochSeconds(raw string) (time.Time, error) {
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("ibkr: parse epoch seconds %q", raw)
	}
	return time.Unix(epoch, 0).UTC(), nil
}

func parseEpochMilliseconds(raw string) (time.Time, error) {
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("ibkr: parse epoch milliseconds %q", raw)
	}
	return time.UnixMilli(epoch).UTC(), nil
}
