package ibkr

import (
	"context"
	"fmt"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func (e *engine) HistoricalBars(ctx context.Context, req HistoricalBarsRequest) ([]Bar, error) {
	if err := validateHistoricalBarsRequest(req); err != nil {
		return nil, err
	}
	req.Contract = cloneContract(req.Contract)

	type result struct {
		values []Bar
		err    error
	}

	resp := make(chan result, 1)
	var reqID int
	enqueueHistoricalSetup(ctx, e, historicalBarsPacingKey(req), nil, func() {
		if err := validateContractFieldSupport(req.Contract, "historical bars", e.serverVersion, contractFieldPrimaryExchange|contractFieldIncludeExpired|contractFieldComboLegs); err != nil {
			resp <- result{err: err}
			return
		}

		reqID = e.allocReqID()
		values := make([]Bar, 0, 16)
		request, err := buildHistoricalBarsRequest(reqID, req)
		if err != nil {
			resp <- result{err: err}
			return
		}
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpHistoricalBars,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistoricalBar:
					bar, err := fromCodecBar(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						resp <- result{err: err}
						return
					}
					values = append(values, bar)
				case codec.HistoricalBarsEnd:
					e.deleteKeyedRoute(reqID)
					resp <- result{values: values}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, request); err != nil {
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
	return out.values, out.err
}

func (e *engine) HistoricalSchedule(ctx context.Context, req HistoricalScheduleRequest) (HistoricalSchedule, error) {
	if err := validateHistoricalScheduleRequest(req); err != nil {
		return HistoricalSchedule{}, err
	}
	req.Contract = cloneContract(req.Contract)

	type result struct {
		value HistoricalSchedule
		err   error
	}

	resp := make(chan result, 1)
	var reqID int
	enqueueHistoricalSetup(ctx, e, historicalSchedulePacingKey(req), nil, func() {
		if err := validateContractFieldSupport(req.Contract, "historical schedule", e.serverVersion, contractFieldPrimaryExchange|contractFieldIncludeExpired|contractFieldComboLegs); err != nil {
			resp <- result{err: err}
			return
		}

		reqID = e.allocReqID()
		request, err := buildHistoricalScheduleRequest(reqID, req)
		if err != nil {
			resp <- result{err: err}
			return
		}
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpHistoricalSchedule,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistoricalScheduleResponse:
					e.deleteKeyedRoute(reqID)
					sessions := make([]HistoricalScheduleSession, len(m.Sessions))
					for i, s := range m.Sessions {
						sessions[i] = HistoricalScheduleSession{
							StartDateTime: s.StartDateTime,
							EndDateTime:   s.EndDateTime,
							RefDate:       s.RefDate,
						}
					}
					resp <- result{value: HistoricalSchedule{
						StartDateTime: m.StartDateTime,
						EndDateTime:   m.EndDateTime,
						TimeZone:      m.TimeZone,
						Sessions:      sessions,
					}}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, request); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return HistoricalSchedule{}, err
	}
	return out.value, out.err
}

func (e *engine) HeadTimestamp(ctx context.Context, req HeadTimestampRequest) (time.Time, error) {
	if err := validateContract(req.Contract); err != nil {
		return time.Time{}, err
	}
	req.Contract = cloneContract(req.Contract)
	type result struct {
		timestamp time.Time
		err       error
	}
	resp := make(chan result, 1)
	var reqID int

	enqueueOneShotSetup(ctx, e, func() {
		if err := validateContractFieldSupport(req.Contract, "head timestamp", e.serverVersion, contractFieldPrimaryExchange|contractFieldIncludeExpired); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()

		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpHeadTimestamp,
			func(msg any, eng *engine) {
				switch m := msg.(type) {
				case codec.HeadTimestamp:
					timestamp, err := parseHeadTimestamp(m.Timestamp)
					if err != nil {
						eng.deleteKeyedRoute(reqID)
						resp <- result{err: err}
						return
					}
					eng.deleteKeyedRoute(reqID)
					resp <- result{timestamp: timestamp}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.HeadTimestampRequest{
			ReqID:      reqID,
			Contract:   toCodecContract(req.Contract),
			WhatToShow: string(req.WhatToShow),
			UseRTH:     req.UseRTH,
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelHeadTimestamp{ReqID: reqID})
			}
		})
	})
	if err != nil {
		return time.Time{}, err
	}
	return out.timestamp, out.err
}

// SubscribeHistoricalBars sends a historical bars request with keepUpToDate=true,
// returning initial bars as events, SnapshotComplete on the initial batch end,
// then streaming bar updates via IN 108.
func (e *engine) SubscribeHistoricalBars(ctx context.Context, req HistoricalBarsRequest, opts ...SubscriptionOption) (*Subscription[Bar], error) {
	if err := validateHistoricalBarsStreamRequest(req); err != nil {
		return nil, err
	}
	req.Contract = cloneContract(req.Contract)

	type result struct {
		sub *Subscription[Bar]
		err error
	}
	resp := make(chan result, 1)

	enqueueHistoricalSetup(ctx, e, historicalBarsPacingKey(req), func() {
		resp <- result{}
	}, func() {
		if err := validateContractFieldSupport(req.Contract, "historical bars stream", e.serverVersion, contractFieldPrimaryExchange|contractFieldIncludeExpired|contractFieldComboLegs); err != nil {
			resp <- result{err: err}
			return
		}

		cfg, err := applySubscriptionOptions(e.cfg, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		if err := validateResumePolicy(OpHistoricalBarsStream, cfg.resume); err != nil {
			resp <- result{err: err}
			return
		}
		reqID := e.allocReqID()
		codecReq, err := buildHistoricalBarsRequest(reqID, req)
		if err != nil {
			resp <- result{err: err}
			return
		}
		codecReq.KeepUpToDate = true

		var sub *Subscription[Bar]
		actorCancel := func() {
			if _, ok := e.keyed[reqID]; !ok {
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.cancelSubscription(OpHistoricalBarsStream, codec.CancelHistoricalData{ReqID: reqID}))
		}
		sub = newEngineSubscription[Bar](cfg, e, actorCancel)
		sub.expectSnapshot()

		e.keyed[reqID] = &route{
			opKind:       OpHistoricalBarsStream,
			subscription: true,
			resume:       cfg.resume,
			request:      codecReq,
			handle: func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistoricalBar:
					bar, err := fromCodecBar(m)
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, bar)
				case codec.HistoricalBarsEnd:
					sub.emitState(SubscriptionStateEvent{Kind: SubscriptionSnapshotComplete, ConnectionSeq: e.connectionSeq()})
				case codec.HistoricalDataUpdate:
					// Streaming updates carry no per-bar trade count on the wire.
					bar, err := fromCodecBar(codec.HistoricalBar{
						ReqID: m.ReqID, Time: m.Time, Open: m.Open, High: m.High,
						Low: m.Low, Close: m.Close, Volume: m.Volume, WAP: m.WAP,
					})
					if err != nil {
						e.deleteKeyedRoute(reqID)
						sub.closeWithErr(err)
						return
					}
					emitSubscription(sub, bar)
				}
			},
			handleAPIErr: func(m codec.APIError, e *engine) {
				if m.Code == 10167 {
					e.emitEvent(m.Code, m.Message)
					return
				}
				e.deleteKeyedRoute(reqID)
				sub.closeWithErr(e.apiErr(OpHistoricalBarsStream, m))
			},
			onDisconnect: func(e *engine, err error) bool {
				sub.closeWithErr(ErrResumeRequired)
				return false
			},
			close: func(err error) { sub.closeWithErr(err) },
		}
		sub.emitState(SubscriptionStateEvent{Kind: SubscriptionStarted, ConnectionSeq: e.connectionSeq()})
		if err := e.sendContext(ctx, codecReq); err != nil {
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

func (e *engine) HistogramData(ctx context.Context, req HistogramDataRequest) ([]HistogramEntry, error) {
	if err := validateContract(req.Contract); err != nil {
		return nil, err
	}
	req.Contract = cloneContract(req.Contract)
	type result struct {
		entries []HistogramEntry
		err     error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if err := validateContractFieldSupport(req.Contract, "histogram data", e.serverVersion, contractFieldPrimaryExchange|contractFieldIncludeExpired); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpHistogramData,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistogramDataResponse:
					e.deleteKeyedRoute(reqID)
					entries := make([]HistogramEntry, len(m.Entries))
					for i, entry := range m.Entries {
						price, err := parseRequiredDecimal(entry.Price, "histogram price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						size, err := parseRequiredDecimal(entry.Size, "histogram size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						entries[i].Price = price
						entries[i].Size = size
					}
					resp <- result{entries: entries}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.HistogramDataRequest{
			ReqID:    reqID,
			Contract: toCodecContract(req.Contract),
			UseRTH:   req.UseRTH,
			Period:   req.Period,
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelHistogramData{ReqID: reqID})
			}
		})
	})
	if err != nil {
		return nil, err
	}
	return out.entries, out.err
}

func (e *engine) HistoricalTicks(ctx context.Context, req HistoricalTicksRequest) (HistoricalTicksResult, error) {
	if err := validateContract(req.Contract); err != nil {
		return HistoricalTicksResult{}, err
	}
	req.Contract = cloneContract(req.Contract)
	type result struct {
		value HistoricalTicksResult
		err   error
	}
	resp := make(chan result, 1)
	var reqID int
	enqueueOneShotSetup(ctx, e, func() {
		if err := validateContractFieldSupport(req.Contract, "historical ticks", e.serverVersion, contractFieldPrimaryExchange|contractFieldIncludeExpired); err != nil {
			resp <- result{err: err}
			return
		}
		reqID = e.allocReqID()
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpHistoricalTicks,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistoricalTicksResponse:
					e.deleteKeyedRoute(reqID)
					ticks := make([]HistoricalTick, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Time = parsedTime
						ticks[i].Price, err = parseRequiredDecimal(t.Price, "historical midpoint tick price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Size, err = parseRequiredDecimal(t.Size, "historical midpoint tick size")
						if err != nil {
							resp <- result{err: err}
							return
						}
					}
					resp <- result{value: HistoricalTicksResult{Ticks: ticks}}
				case codec.HistoricalTicksBidAskResponse:
					e.deleteKeyedRoute(reqID)
					ticks := make([]HistoricalTickBidAsk, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].TickAttrib = t.TickAttrib
						ticks[i].Time = parsedTime
						ticks[i].BidPrice, err = parseRequiredDecimal(t.BidPrice, "historical bid price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].AskPrice, err = parseRequiredDecimal(t.AskPrice, "historical ask price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].BidSize, err = parseRequiredDecimal(t.BidSize, "historical bid size")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].AskSize, err = parseRequiredDecimal(t.AskSize, "historical ask size")
						if err != nil {
							resp <- result{err: err}
							return
						}
					}
					resp <- result{value: HistoricalTicksResult{BidAsk: ticks}}
				case codec.HistoricalTicksLastResponse:
					e.deleteKeyedRoute(reqID)
					ticks := make([]HistoricalTickLast, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].TickAttrib = t.TickAttrib
						ticks[i].Time = parsedTime
						ticks[i].Price, err = parseRequiredDecimal(t.Price, "historical trade tick price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Size, err = parseRequiredDecimal(t.Size, "historical trade tick size")
						if err != nil {
							resp <- result{err: err}
							return
						}
						ticks[i].Exchange = t.Exchange
						ticks[i].SpecialConditions = t.SpecialConditions
					}
					resp <- result{value: HistoricalTicksResult{Last: ticks}}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		if err := e.sendContext(ctx, codec.HistoricalTicksRequest{
			ReqID:         reqID,
			Contract:      toCodecContract(req.Contract),
			StartDateTime: formatHistoricalTickTime(req.StartTime),
			EndDateTime:   formatHistoricalTickTime(req.EndTime),
			NumberOfTicks: req.NumberOfTicks,
			WhatToShow:    string(req.WhatToShow),
			UseRTH:        req.UseRTH,
			IgnoreSize:    req.IgnoreSize,
		}); err != nil {
			e.deleteKeyedRoute(reqID)
			resp <- result{err: err}
		}
	})

	out, err := awaitOneShotResponse(ctx, e, resp, func() {
		e.enqueue(func() { e.deleteKeyedRoute(reqID) })
	})
	if err != nil {
		return HistoricalTicksResult{}, err
	}
	return out.value, out.err
}

func fromCodecBar(m codec.HistoricalBar) (Bar, error) {
	ts, err := parseBarTime(m.Time)
	if err != nil {
		return Bar{}, err
	}
	open, err := parseRequiredDecimal(m.Open, "bar open")
	if err != nil {
		return Bar{}, err
	}
	high, err := parseRequiredDecimal(m.High, "bar high")
	if err != nil {
		return Bar{}, err
	}
	low, err := parseRequiredDecimal(m.Low, "bar low")
	if err != nil {
		return Bar{}, err
	}
	closeValue, err := parseRequiredDecimal(m.Close, "bar close")
	if err != nil {
		return Bar{}, err
	}
	volume, err := parseRequiredDecimal(m.Volume, "bar volume")
	if err != nil {
		return Bar{}, err
	}
	wap, err := parseOptionalDecimal(m.WAP, "bar wap")
	if err != nil {
		return Bar{}, err
	}
	count, err := parseOptionalInt(m.Count, "bar count")
	if err != nil {
		return Bar{}, err
	}
	return Bar{Time: ts, Open: open, High: high, Low: low, Close: closeValue, Volume: volume, WAP: wap, Count: count}, nil
}

// parseBarTime handles both RFC3339 (from testhost transcripts) and IBKR native
// bar date formats ("20260402  09:30:00" or "20260402" for daily bars).
func parseBarTime(raw string) (time.Time, error) {
	// Try RFC3339 first (for backward compat with test transcripts)
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
	}
	// IBKR intraday format: "20260402  09:30:00" (note: double space, no timezone)
	if ts, err := time.Parse("20060102  15:04:05", raw); err == nil {
		return ts, nil
	}
	// IBKR daily format: "20260402"
	if ts, err := time.Parse("20060102", raw); err == nil {
		return ts, nil
	}
	// IBKR format with timezone: "20260402 09:30:00 US/Eastern"
	// Strip timezone suffix and parse the datetime prefix.
	if len(raw) > 17 {
		if ts, err := time.Parse("20060102 15:04:05", raw[:17]); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("ibkr: parse bar time %q", raw)
}

func parseHeadTimestamp(raw string) (time.Time, error) {
	ts, err := time.Parse("20060102-15:04:05", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("ibkr: parse head timestamp %q", raw)
	}
	return ts.UTC(), nil
}
