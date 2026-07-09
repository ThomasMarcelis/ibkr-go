package ibkr

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func (e *engine) CurrentTime(ctx context.Context) (time.Time, error) {
	type result struct {
		ts  time.Time
		err error
	}
	resp := make(chan result, 1)

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonCurrentTime]; exists {
			resp <- result{err: fmt.Errorf("ibkr: current time request already in progress")}
			return
		}

		// No handleAPIErr: req_current_time carries no reqID, so the engine
		// cannot route an APIError to this singleton. ctx cancellation and
		// onDisconnect are the only failure paths.
		e.singletons[singletonCurrentTime] = &route{
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
				delete(eng.singletons, singletonCurrentTime)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.CurrentTimeRequest{}); err != nil {
			delete(e.singletons, singletonCurrentTime)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonCurrentTime) })
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

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if e.serverVersion < codec.MinServerVersionCurrentTimeInMillis {
			resp <- result{err: fmt.Errorf("ibkr: current time millis: %w", ErrUnsupportedServerVersion)}
			return
		}
		if _, exists := e.singletons[singletonCurrentTimeMillis]; exists {
			resp <- result{err: fmt.Errorf("ibkr: current time millis request already in progress")}
			return
		}

		// Like reqCurrentTime, the request carries no reqID, so APIErrors
		// cannot route here; ctx cancellation and onDisconnect are the only
		// failure paths.
		e.singletons[singletonCurrentTimeMillis] = &route{
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
				delete(eng.singletons, singletonCurrentTimeMillis)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.CurrentTimeMillisRequest{}); err != nil {
			delete(e.singletons, singletonCurrentTimeMillis)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonCurrentTimeMillis) })
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

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if _, exists := e.singletons[singletonScannerParameters]; exists {
			resp <- result{err: fmt.Errorf("ibkr: scanner parameters request already in progress")}
			return
		}

		e.singletons[singletonScannerParameters] = &route{
			opKind: OpScannerParameters,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.ScannerParameters:
					delete(eng.singletons, singletonScannerParameters)
					resp <- result{xml: m.XML}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonScannerParameters)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.ScannerParametersRequest{}); err != nil {
			delete(e.singletons, singletonScannerParameters)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonScannerParameters) })
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
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()

		e.keyed[reqID] = &route{
			opKind: OpUserInfo,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.UserInfo:
					eng.deleteKeyedRoute(reqID)
					resp <- result{whiteBrandingID: m.WhiteBrandingID}
				}
			},
			handleAPIErr: func(m codec.APIError, eng *engine) {
				eng.deleteKeyedRoute(reqID)
				resp <- result{err: eng.apiErr(OpUserInfo, m)}
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
	type result struct {
		sub *Subscription[[]ScannerResult]
		err error
	}
	resp := make(chan result, 1)

	enqueueSubscriptionSetup(ctx, e, resp, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpScannerSubscription, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		var sub *Subscription[[]ScannerResult]
		sub = newSubscription[[]ScannerResult](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelScannerSubscription{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpScannerSubscription,
			subscription: true,
			resume:       cfg.resume,
			request: codec.ScannerSubscriptionRequest{
				ReqID:        reqID,
				NumberOfRows: req.NumberOfRows,
				Instrument:   string(req.Instrument),
				LocationCode: string(req.LocationCode),
				ScanCode:     string(req.ScanCode),
			},
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.ScannerDataResponse:
					results := make([]ScannerResult, len(m.Entries))
					for i, entry := range m.Entries {
						results[i] = ScannerResult{
							Rank:       entry.Rank,
							Contract:   fromCodecContract(entry.Contract),
							Distance:   entry.Distance,
							Benchmark:  entry.Benchmark,
							Projection: entry.Projection,
							LegsStr:    entry.LegsStr,
						}
					}
					emitSubscription(sub, results)
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpScannerSubscription, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) {
		if out.sub != nil {
			_ = out.sub.Close()
		}
	})
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

	enqueueOneShotSetup(ctx, e, func() {
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		if err := validateFADataType(faDataType, e.serverVersion); err != nil {
			resp <- result{err: err}
			return
		}
		if _, exists := e.singletons[singletonFA]; exists {
			resp <- result{err: fmt.Errorf("ibkr: FA request already in progress")}
			return
		}

		e.singletons[singletonFA] = &route{
			opKind: OpFAConfig,
			handle: func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.ReceiveFA:
					delete(eng.singletons, singletonFA)
					resp <- result{xml: m.XML}
				}
			},
			onDisconnect: func(eng *engine, err error) bool {
				delete(eng.singletons, singletonFA)
				resp <- result{err: ErrInterrupted}
				return false
			},
			close: func(err error) {
				resp <- result{err: err}
			},
		}
		if err := e.sendContext(ctx, codec.RequestFA{FADataType: int(faDataType)}); err != nil {
			delete(e.singletons, singletonFA)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { delete(e.singletons, singletonFA) })
	})
	if err != nil {
		return "", err
	}
	return out.xml, out.err
}

func (e *engine) ReplaceFA(ctx context.Context, faDataType FADataType, xml string) error {
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
		}
		if err := validateFADataType(faDataType, e.serverVersion); err != nil {
			return err
		}
		return e.sendContext(ctx, codec.ReplaceFA{ReqID: e.allocReqID(), FADataType: int(faDataType), XML: xml})
	})
}

// validateFADataType rejects the FA profiles data type once the negotiated
// server desupports it (FA_PROFILE_DESUPPORT, 177); the official client raises
// FA_PROFILE_NOT_SUPPORTED for the same case (client.py:4740-4747, 4800-4802).
func validateFADataType(faDataType FADataType, serverVersion int) error {
	if faDataType == FADataProfiles && serverVersion >= codec.MinServerVersionFAProfileDesupport {
		return &ValidationError{
			Field:   "FA data type",
			Value:   faDataType.String(),
			Message: "FA profiles are desupported at server_version 177 and above",
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
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = &route{
			opKind: OpSoftDollarTiers,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.SoftDollarTiersResponse:
					delete(e.keyed, reqID)
					tiers := make([]SoftDollarTier, len(m.Tiers))
					for i, t := range m.Tiers {
						tiers[i] = SoftDollarTier{Name: t.Name, Value: t.Value, DisplayName: t.DisplayName}
					}
					resp <- result{tiers: tiers}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpSoftDollarTiers, m)}
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
		if err := e.sendContext(ctx, codec.SoftDollarTiersRequest{ReqID: reqID}); err != nil {
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
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = &route{
			opKind: OpWSHMetaData,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.WSHMetaDataResponse:
					delete(e.keyed, reqID)
					resp <- result{dataJSON: m.DataJSON}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpWSHMetaData, m)}
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
		if err := e.sendContext(ctx, codec.WSHMetaDataRequest{ReqID: reqID}); err != nil {
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return "", err
	}
	return out.dataJSON, out.err
}

func (e *engine) WSHEventData(ctx context.Context, req WSHEventDataRequest) (string, error) {
	type result struct {
		dataJSON string
		err      error
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
			opKind: OpWSHEventData,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.WSHEventDataResponse:
					delete(e.keyed, reqID)
					resp <- result{dataJSON: m.DataJSON}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpWSHEventData, m)}
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
			delete(e.keyed, reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
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
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = &route{
			opKind: OpDisplayGroups,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.DisplayGroupList:
					delete(e.keyed, reqID)
					resp <- result{groups: m.Groups}
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				delete(e.keyed, reqID)
				resp <- result{err: e.apiErr(OpDisplayGroups, m)}
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
		if err := e.sendContext(ctx, codec.QueryDisplayGroupsRequest{ReqID: reqID}); err != nil {
			delete(e.keyed, reqID)
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
		if !e.isReady() {
			resp <- result{err: ErrNotReady}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpDisplayGroupEvents, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		var sub *Subscription[DisplayGroupUpdate]
		sub = newSubscription[DisplayGroupUpdate](cfg, func() {
			e.enqueue(func() {
				if _, ok := e.keyed[reqID]; !ok {
					return
				}
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.UnsubscribeFromGroupEventsRequest{ReqID: reqID})
				sub.closeWithErr(nil)
			})
		})

		e.keyed[reqID] = &route{
			opKind:       OpDisplayGroupEvents,
			subscription: true,
			resume:       cfg.resume,
			request:      codec.SubscribeToGroupEventsRequest{ReqID: reqID, GroupID: int(groupID)},
			handle: func(msg any, e *engine) {
				if m, ok := msg.(codec.DisplayGroupUpdated); ok {
					emitSubscription(sub, DisplayGroupUpdate{ContractInfo: m.ContractInfo})
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpDisplayGroupEvents, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, e.keyed[reqID].request); err != nil {
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(err)
			resp <- result{err: err}
			return
		}
		resp <- result{sub: sub}
	})

	out, err := awaitSubscriptionResponse(ctx, e, resp, func(out result) {
		if out.sub != nil {
			_ = out.sub.Close()
		}
	})
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
	return awaitFireAndForget(ctx, e, func(ctx context.Context) error {
		if !e.isReady() {
			return ErrNotReady
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
