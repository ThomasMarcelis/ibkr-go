package ibkr

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
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

		allocatedReqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID = allocatedReqID
		values := make([]Bar, 0, 16)
		request := buildHistoricalBarsRequest(reqID, req)
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
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelHistoricalData{ReqID: reqID})
			}
		})
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

		allocatedReqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID = allocatedReqID
		request := buildHistoricalScheduleRequest(reqID, req)
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
		e.enqueue(func() {
			if _, ok := e.keyed[reqID]; ok {
				e.deleteKeyedRoute(reqID)
				_ = e.send(codec.CancelHistoricalData{ReqID: reqID})
			}
		})
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
		allocatedReqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID = allocatedReqID

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

		cfg, err := applySubscriptionOptionsFor(e.cfg, OpHistoricalBarsStream, opts)
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		codecReq := buildHistoricalBarsRequest(reqID, req)
		codecReq.KeepUpToDate = true

		sub, ownedRoute := newKeyedSubscriptionRoute[Bar](
			e, cfg, reqID, OpHistoricalBarsStream, codec.CancelHistoricalData{ReqID: reqID},
		)
		sub.expectSnapshot()

		ownedRoute.request = codecReq
		ownedRoute.handle = func(msg any, e *engine) {
			switch m := msg.(type) {
			case codec.HistoricalBar:
				bar, err := fromCodecBar(m)
				if err != nil {
					sub.cancelFromActor(err)
					return
				}
				sub.emit(bar)
			case codec.HistoricalBarsEnd:
				sub.emitState(StreamSnapshotComplete, e.connectionSeq(), nil)
			case codec.HistoricalDataUpdate:
				bar, err := fromCodecHistoricalDataUpdate(m)
				if err != nil {
					sub.cancelFromActor(err)
					return
				}
				sub.emit(bar)
			}
		}
		ownedRoute.handleAPIErr = func(m codec.APIError, e *engine) {
			if e.keyed[reqID] != ownedRoute {
				return
			}
			if m.Code == ErrCodeDelayedMarketDataDisplayed {
				sub.emitNotice(e.apiNotice(OpHistoricalBarsStream, m), e.connectionSeq())
				return
			}
			e.deleteKeyedRoute(reqID)
			sub.closeWithErr(e.apiErr(OpHistoricalBarsStream, m))
		}
		e.keyed[reqID] = ownedRoute
		sub.emitState(StreamStarted, e.connectionSeq(), nil)
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
		allocatedReqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID = allocatedReqID
		e.keyed[reqID] = newKeyedOneShotRoute(reqID, OpHistogramData,
			func(msg any, e *engine) {
				switch m := msg.(type) {
				case codec.HistogramDataResponse:
					e.deleteKeyedRoute(reqID)
					entries := make([]HistogramEntry, len(m.Entries))
					for i, entry := range m.Entries {
						price, err := parseRequiredDecimal(entry.Price, "histogram price")
						if err != nil {
							resp <- result{err: err}
							return
						}
						size, err := parseRequiredDecimal(entry.Size, "histogram size")
						if err != nil {
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
	if err := validateHistoricalTicksRequest(req); err != nil {
		return HistoricalTicksResult{}, err
	}
	req.Contract = cloneContract(req.Contract)
	type result struct {
		value HistoricalTicksResult
		err   error
	}
	resp := make(chan result, 1)
	var reqID int
	var ownedRoute *route
	var midpointTicks []HistoricalTick
	var bidAskTicks []HistoricalTickBidAsk
	var lastTicks []HistoricalTickLast
	enqueueOneShotSetup(ctx, e, func() {
		if err := validateContractFieldSupport(req.Contract, "historical ticks", e.serverVersion, contractFieldPrimaryExchange|contractFieldIncludeExpired); err != nil {
			resp <- result{err: err}
			return
		}
		allocatedReqID, err := e.allocReqID()
		if err != nil {
			resp <- result{err: err}
			return
		}
		reqID = allocatedReqID
		ownedRoute = newKeyedOneShotRoute(reqID, OpHistoricalTicks,
			func(msg any, e *engine) {
				if got, ok := historicalTicksResponseKind(msg); ok && got != req.WhatToShow {
					e.deleteKeyedRoute(reqID)
					resp <- result{err: &ProtocolError{
						Direction: "inbound",
						Message:   "historical ticks",
						Err:       fmt.Errorf("received %s response for %s request", got, req.WhatToShow),
					}}
					return
				}
				switch m := msg.(type) {
				case codec.HistoricalTicksResponse:
					ticks := make([]HistoricalTick, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].Time = parsedTime
						ticks[i].Price, err = parseRequiredDecimal(t.Price, "historical midpoint tick price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].Size, err = parseRequiredDecimal(t.Size, "historical midpoint tick size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
					}
					midpointTicks = append(midpointTicks, ticks...)
					if m.Done {
						e.deleteKeyedRoute(reqID)
						resp <- result{value: HistoricalTicksResult{WhatToShow: ShowMidpoint, Ticks: midpointTicks}}
					}
				case codec.HistoricalTicksBidAskResponse:
					ticks := make([]HistoricalTickBidAsk, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].Attributes = TickBidAskAttributes(t.TickAttrib)
						ticks[i].Time = parsedTime
						ticks[i].BidPrice, err = parseRequiredDecimal(t.BidPrice, "historical bid price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].AskPrice, err = parseRequiredDecimal(t.AskPrice, "historical ask price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].BidSize, err = parseRequiredDecimal(t.BidSize, "historical bid size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].AskSize, err = parseRequiredDecimal(t.AskSize, "historical ask size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
					}
					bidAskTicks = append(bidAskTicks, ticks...)
					if m.Done {
						e.deleteKeyedRoute(reqID)
						resp <- result{value: HistoricalTicksResult{WhatToShow: ShowBidAsk, BidAsk: bidAskTicks}}
					}
				case codec.HistoricalTicksLastResponse:
					ticks := make([]HistoricalTickLast, len(m.Ticks))
					for i, t := range m.Ticks {
						parsedTime, err := parseEpochSeconds(t.Time)
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].Attributes = TickLastAttributes(t.TickAttrib)
						ticks[i].Time = parsedTime
						ticks[i].Price, err = parseRequiredDecimal(t.Price, "historical trade tick price")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].Size, err = parseRequiredDecimal(t.Size, "historical trade tick size")
						if err != nil {
							e.deleteKeyedRoute(reqID)
							resp <- result{err: err}
							return
						}
						ticks[i].Exchange = t.Exchange
						ticks[i].SpecialConditions = t.SpecialConditions
					}
					lastTicks = append(lastTicks, ticks...)
					if m.Done {
						e.deleteKeyedRoute(reqID)
						resp <- result{value: HistoricalTicksResult{WhatToShow: ShowTrades, Last: lastTicks}}
					}
				}
			}, func(err error) {
				resp <- result{err: err}
			})
		e.keyed[reqID] = ownedRoute
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
		e.enqueue(func() {
			if reqID == 0 || ownedRoute == nil || e.keyed[reqID] != ownedRoute {
				return
			}
			e.deleteKeyedRoute(reqID)
			if e.serverVersion >= protocol.MinServerVersionBrokerSideOneShotCancel {
				_ = e.send(codec.CancelHistoricalTicks{ReqID: reqID})
			}
		})
	})
	if err != nil {
		return HistoricalTicksResult{}, err
	}
	return out.value, out.err
}

func historicalTicksResponseKind(msg any) (WhatToShow, bool) {
	switch msg.(type) {
	case codec.HistoricalTicksResponse:
		return ShowMidpoint, true
	case codec.HistoricalTicksBidAskResponse:
		return ShowBidAsk, true
	case codec.HistoricalTicksLastResponse:
		return ShowTrades, true
	default:
		return "", false
	}
}

func fromCodecBar(m codec.HistoricalBar) (Bar, error) {
	ts, err := parseBarTime(m.Time)
	if err != nil {
		return Bar{}, err
	}
	return fromCodecBarAt(m, ts)
}

func fromCodecHistoricalDataUpdate(m codec.HistoricalDataUpdate) (Bar, error) {
	return fromCodecBar(codec.HistoricalBar{
		ReqID: m.ReqID, Time: m.Time, Open: m.Open, High: m.High,
		Low: m.Low, Close: m.Close, Volume: m.Volume, WAP: m.WAP,
		Count: strconv.Itoa(m.BarCount),
	})
}

func fromCodecBarAt(m codec.HistoricalBar, ts time.Time) (Bar, error) {
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

func parseBarTime(raw string) (time.Time, error) {
	if ts, err := time.Parse("20060102 15:04:05Z07:00", raw); err == nil {
		return ts.UTC(), nil
	}
	parts := strings.Fields(raw)
	switch len(parts) {
	case 1:
		if ts, err := time.Parse("20060102", parts[0]); err == nil {
			return ts.UTC(), nil
		}
	case 2:
		if ts, err := time.Parse("20060102 15:04:05", parts[0]+" "+parts[1]); err == nil {
			return ts.UTC(), nil
		}
	case 3:
		if parts[2] == "Z" || parts[2] == "UTC" {
			if ts, err := time.ParseInLocation("20060102 15:04:05", parts[0]+" "+parts[1], time.UTC); err == nil {
				return ts.UTC(), nil
			}
		}
		location, err := time.LoadLocation(parts[2])
		if err != nil {
			return time.Time{}, inboundProtocolError("bar time", fmt.Errorf("parse %q: load location: %w", raw, err))
		}
		if ts, err := time.ParseInLocation("20060102 15:04:05", parts[0]+" "+parts[1], location); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, inboundProtocolError("bar time", fmt.Errorf("parse %q", raw))
}

func parseHeadTimestamp(raw string) (time.Time, error) {
	for _, layout := range []string{"20060102-15:04:05", "20060102-15:04:05Z07:00"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, inboundProtocolError("head timestamp", fmt.Errorf("parse %q", raw))
}
