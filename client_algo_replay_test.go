package ibkr_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go"
	"github.com/shopspring/decimal"
)

// Live algo window the capture driver sent on every variant: a 17-minute UTC
// window that the Gateway's open_order echoes hand back in US/Eastern.
const (
	algoStartUTC     = "20260415 15:38:26 UTC"
	algoEndUTC       = "20260415 15:55:26 UTC"
	algoStartEastern = "20260415 11:38:26 US/Eastern"
	algoEndEastern   = "20260415 11:55:26 US/Eastern"
)

// algoVariantCase is one strategy of the AORD-003 algo matrix. rejectCode == 0
// means the live Gateway accepted the placement; the lifecycle expectations
// then describe the two open_order echoes and the first order_status.
type algoVariantCase struct {
	name        string
	orderID     int64
	strategy    string
	params      []ibkr.TagValue
	displaySize int

	permID      int64
	firstStatus ibkr.OrderStatus
	firstEcho   []ibkr.TagValue
	secondEcho  []ibkr.TagValue // nil = identical to firstEcho

	rejectCode        int
	rejectMessage     string
	cancelAfterReject bool // the capture's safety cancel for the rejected id drew no response
}

func algoTag(tag, value string) ibkr.TagValue {
	return ibkr.TagValue{Tag: tag, Value: value}
}

// requireAlgoParams asserts an open_order echo carried exactly the captured
// algo params, in the Gateway's order.
func requireAlgoParams(t *testing.T, label string, got, want []ibkr.TagValue) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Fatalf("%s algo params = %v, want %v", label, got, want)
	}
}

// collectAlgoEchoes consumes handle events until both live open_order echoes
// and the Submitted status have arrived, returning the echoes in arrival
// order plus the first order_status the Gateway sent (PreSubmitted for the
// Adaptive/Urgent and AD variants, Submitted for the rest).
func collectAlgoEchoes(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle) ([]ibkr.OpenOrder, ibkr.OrderStatus) {
	t.Helper()

	var echoes []ibkr.OpenOrder
	var firstStatus ibkr.OrderStatus
	sawSubmitted := false
	for len(echoes) < 2 || !sawSubmitted {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				t.Fatal("order events closed before both open_order echoes")
			}
			if evt.OpenOrder != nil {
				echoes = append(echoes, *evt.OpenOrder)
			}
			if evt.Status != nil {
				if firstStatus == "" {
					firstStatus = evt.Status.Status
				}
				if evt.Status.Status == ibkr.OrderStatusSubmitted {
					sawSubmitted = true
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for algo open_order echoes")
		}
	}
	return echoes, firstStatus
}

// TestAPIAlgoVariantsReplay freezes the IB algo variant matrix (AORD-003)
// captured live on 2026-04-15 (captures/20260415T153524Z-
// api_algo_variants_aapl, events.jsonl sha256 prefix 1855e2554d7de3ae):
// thirteen algo strategies each placed as a far LMT BUY 1 AAPL. Seven were
// accepted and rest at Submitted before cancelling cleanly with the code-202
// notice (Adaptive Urgent/Patient, Vwap, ArrivalPx, AD, ClosePx, PctVol);
// six were rejected at place time with no order_status: Twap with code 443
// naming the unknown strategyType attribute, DarkIce with code 10255 for its
// display-size attribute, and Inline, BalanceImpactRisk, MinImpact, and JefAD
// with code 439 "Algorithm definition not found". All six close the handle
// as terminal place errors — 10255 through the attested placement-rejection
// set (isOrderPlacementRejection) since the Gateway never sends anything
// else for the id; the capture's follow-up cancel for the DarkIce id drew no
// Gateway response at all. Rejected placements still consume their
// order ids: the accepted lifecycles run on 262..271 with gaps 264/267/269
// held by rejects, and 272..274 reject back to back. The Gateway's open_order
// echoes reorder algo params, convert the UTC window to US/Eastern, and
// append server-side params the client never sent (Vwap auction opt-outs and
// speedUp, PctVol includeBlockTrades, AD activeTimeStart/activeTimeEnd).
func TestAPIAlgoVariantsReplay(t *testing.T) {
	t.Parallel()

	variants := []algoVariantCase{
		{
			name:        "adaptive_urgent",
			orderID:     262,
			strategy:    "Adaptive",
			params:      []ibkr.TagValue{algoTag("adaptivePriority", "Urgent")},
			permID:      900262,
			firstStatus: ibkr.OrderStatusPreSubmitted,
			firstEcho:   []ibkr.TagValue{algoTag("adaptivePriority", "Urgent")},
		},
		{
			name:        "adaptive_patient",
			orderID:     263,
			strategy:    "Adaptive",
			params:      []ibkr.TagValue{algoTag("adaptivePriority", "Patient")},
			permID:      900263,
			firstStatus: ibkr.OrderStatusSubmitted,
			firstEcho:   []ibkr.TagValue{algoTag("adaptivePriority", "Patient")},
		},
		{
			name:     "twap",
			orderID:  264,
			strategy: "Twap",
			params: []ibkr.TagValue{
				algoTag("strategyType", "Marketable"),
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("allowPastEndTime", "1"),
			},
			rejectCode:    ibkr.ErrCodeUnknownAlgoAttribute,
			rejectMessage: "Order processing failed. Unknown algo attribute:strategyType",
		},
		{
			name:     "vwap",
			orderID:  265,
			strategy: "Vwap",
			params: []ibkr.TagValue{
				algoTag("maxPctVol", "0.1"),
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("allowPastEndTime", "1"),
				algoTag("noTakeLiq", "1"),
			},
			permID:      900265,
			firstStatus: ibkr.OrderStatusSubmitted,
			firstEcho: []ibkr.TagValue{
				algoTag("noTakeLiq", "1"),
				algoTag("allowPastEndTime", "1"),
				algoTag("startTime", algoStartEastern),
				algoTag("maxPctVol", "0.1"),
				algoTag("endTime", algoEndEastern),
			},
			secondEcho: []ibkr.TagValue{
				algoTag("noTakeLiq", "1"),
				algoTag("optoutClosingAuction", ""),
				algoTag("allowPastEndTime", "1"),
				algoTag("speedUp", ""),
				algoTag("optoutOpeningAuction", ""),
				algoTag("startTime", algoStartEastern),
				algoTag("maxPctVol", "0.1"),
				algoTag("endTime", algoEndEastern),
			},
		},
		{
			name:     "arrival_px",
			orderID:  266,
			strategy: "ArrivalPx",
			params: []ibkr.TagValue{
				algoTag("maxPctVol", "0.1"),
				algoTag("riskAversion", "Neutral"),
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("forceCompletion", "0"),
				algoTag("allowPastEndTime", "1"),
			},
			permID:      900266,
			firstStatus: ibkr.OrderStatusSubmitted,
			firstEcho: []ibkr.TagValue{
				algoTag("riskAversion", "Neutral"),
				algoTag("allowPastEndTime", "1"),
				algoTag("forceCompletion", "0"),
				algoTag("startTime", algoStartEastern),
				algoTag("maxPctVol", "0.1"),
				algoTag("endTime", algoEndEastern),
			},
		},
		{
			name:     "dark_ice",
			orderID:  267,
			strategy: "DarkIce",
			params: []ibkr.TagValue{
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("allowPastEndTime", "1"),
			},
			displaySize:       1,
			rejectCode:        ibkr.ErrCodeDisplaySizeNotAllowed,
			rejectMessage:     "The 'Display Size' order attribute may not be specified for this order.",
			cancelAfterReject: true,
		},
		{
			name:     "accum_dist",
			orderID:  268,
			strategy: "AD",
			params: []ibkr.TagValue{
				algoTag("componentSize", "1"),
				algoTag("timeBetweenOrders", "60"),
				algoTag("randomizeTime20", "0"),
				algoTag("randomizeSize55", "0"),
				algoTag("giveUp", "0"),
				algoTag("catchUp", "1"),
				algoTag("waitForFill", "1"),
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
			},
			permID:      900268,
			firstStatus: ibkr.OrderStatusPreSubmitted,
			firstEcho: []ibkr.TagValue{
				algoTag("componentSize", "1"),
				algoTag("timeBetweenOrders", "60"),
				algoTag("randomizeTime20", "0"),
				algoTag("randomizeSize55", "0"),
				algoTag("giveUp", "0"),
				algoTag("catchUp", "1"),
				algoTag("waitForFill", "1"),
				algoTag("activeTimeStart", "07:12:55"),
				algoTag("activeTimeEnd", "07:12:55"),
			},
		},
		{
			name:     "inline",
			orderID:  269,
			strategy: "Inline",
			params: []ibkr.TagValue{
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
			},
			rejectCode:    ibkr.ErrCodeAlgoDefinitionNotFound,
			rejectMessage: "Order processing failed. Algorithm definition not found",
		},
		{
			name:     "close",
			orderID:  270,
			strategy: "ClosePx",
			params: []ibkr.TagValue{
				algoTag("maxPctVol", "0.1"),
				algoTag("riskAversion", "Neutral"),
				algoTag("startTime", algoStartUTC),
				algoTag("forceCompletion", "0"),
			},
			permID:      900270,
			firstStatus: ibkr.OrderStatusSubmitted,
			firstEcho: []ibkr.TagValue{
				algoTag("riskAversion", "Neutral"),
				algoTag("forceCompletion", "0"),
				algoTag("startTime", algoStartEastern),
				algoTag("maxPctVol", "0.1"),
			},
		},
		{
			name:     "pct_vol",
			orderID:  271,
			strategy: "PctVol",
			params: []ibkr.TagValue{
				algoTag("pctVol", "0.1"),
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("noTakeLiq", "1"),
			},
			permID:      900271,
			firstStatus: ibkr.OrderStatusSubmitted,
			firstEcho: []ibkr.TagValue{
				algoTag("noTakeLiq", "1"),
				algoTag("pctVol", "0.1"),
				algoTag("startTime", algoStartEastern),
				algoTag("endTime", algoEndEastern),
			},
			secondEcho: []ibkr.TagValue{
				algoTag("noTakeLiq", "1"),
				algoTag("includeBlockTrades", ""),
				algoTag("pctVol", "0.1"),
				algoTag("startTime", algoStartEastern),
				algoTag("endTime", algoEndEastern),
			},
		},
		{
			name:     "balance_impact_risk",
			orderID:  272,
			strategy: "BalanceImpactRisk",
			params: []ibkr.TagValue{
				algoTag("maxPctVol", "0.1"),
				algoTag("riskAversion", "Neutral"),
			},
			rejectCode:    ibkr.ErrCodeAlgoDefinitionNotFound,
			rejectMessage: "Order processing failed. Algorithm definition not found",
		},
		{
			name:          "min_impact",
			orderID:       273,
			strategy:      "MinImpact",
			params:        []ibkr.TagValue{algoTag("maxPctVol", "0.1")},
			rejectCode:    ibkr.ErrCodeAlgoDefinitionNotFound,
			rejectMessage: "Order processing failed. Algorithm definition not found",
		},
		{
			name:     "jef_ad",
			orderID:  274,
			strategy: "JefAD",
			params: []ibkr.TagValue{
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("componentSize", "1"),
				algoTag("timeBetweenOrders", "60"),
			},
			rejectCode:    ibkr.ErrCodeAlgoDefinitionNotFound,
			rejectMessage: "Order processing failed. Algorithm definition not found",
		},
	}

	client, host := newClient(t, "api_algo_variants_aapl.txt")
	defer client.Close()
	defer waitHost(t, host)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	events := client.SessionEvents()

	for i, v := range variants {
		ref := fmt.Sprintf("ibkrgo-sanitized-20260415T153525Z-%03d", i+1)
		t.Run(v.name, func(t *testing.T) {
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
				Contract: orderReplayAAPL,
				Order: ibkr.Order{
					Action:       ibkr.ActionBuy,
					OrderType:    ibkr.OrderTypeLimit,
					Quantity:     decimal.RequireFromString("1"),
					LmtPrice:     decimal.RequireFromString("13.15"),
					TIF:          ibkr.TIFDay,
					Account:      "DU9000001",
					OrderRef:     ref,
					DisplaySize:  v.displaySize,
					AlgoStrategy: v.strategy,
					AlgoParams:   v.params,
				},
			})
			if err != nil {
				t.Fatalf("Place: %v", err)
			}
			if got := handle.OrderID(); got != v.orderID {
				t.Fatalf("order id = %d, want %d", got, v.orderID)
			}

			if v.rejectCode != 0 {
				requireOrderAPIError(t, v.name, handle, v.rejectCode, v.rejectMessage)
				if v.cancelAfterReject {
					// The live Gateway never answered this cancel; the
					// replay proves the cancel frame still goes out for an
					// id whose placement was rejected and whose handle
					// already closed on the rejection.
					if err := handle.Cancel(ctx); err != nil {
						t.Fatalf("Cancel: %v", err)
					}
				}
				return
			}

			echoes, firstStatus := collectAlgoEchoes(t, ctx, handle)
			if firstStatus != v.firstStatus {
				t.Fatalf("first status = %s, want %s", firstStatus, v.firstStatus)
			}
			for i, echo := range echoes {
				if echo.AlgoStrategy != v.strategy {
					t.Fatalf("echo %d algo strategy = %q, want %q", i, echo.AlgoStrategy, v.strategy)
				}
				if echo.PermID != v.permID {
					t.Fatalf("echo %d perm id = %d, want %d", i, echo.PermID, v.permID)
				}
			}
			if !echoes[0].LmtPrice.Equal(decimal.RequireFromString("13.15")) {
				t.Fatalf("echoed lmt price = %s, want 13.15", echoes[0].LmtPrice)
			}
			if echoes[0].OrderRef != ref {
				t.Fatalf("echoed order ref = %q, want %q", echoes[0].OrderRef, ref)
			}
			requireAlgoParams(t, "first echo", echoes[0].AlgoParams, v.firstEcho)
			secondEcho := v.secondEcho
			if secondEcho == nil {
				secondEcho = v.firstEcho
			}
			requireAlgoParams(t, "second echo", echoes[1].AlgoParams, secondEcho)

			if err := handle.Cancel(ctx); err != nil {
				t.Fatalf("Cancel: %v", err)
			}
			waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusCancelled)
			notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
			if notice.Message != "Order Canceled - reason:" {
				t.Fatalf("202 message = %q", notice.Message)
			}
		})
	}
}
