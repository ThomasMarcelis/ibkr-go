package ibkr_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

// Live algo window the capture driver sent on every variant: a 17-minute UTC
// window that the Gateway's open_order echoes hand back in US/Eastern.
const (
	algoStartUTC     = "20260824 20:34:58 UTC"
	algoEndUTC       = "20260824 20:51:58 UTC"
	algoStartEastern = "20260825 16:34:58 US/Eastern"
	algoEndEastern   = "20260825 16:51:58 US/Eastern"
)

// algoVariantCase is one strategy of the AORD-003 algo matrix. rejectCode == 0
// means the live Gateway accepted the placement; the lifecycle expectations
// then describe every pre-cancel open_order echo and order_status.
type algoVariantCase struct {
	name        string
	orderID     int64
	strategy    string
	params      []ibkr.TagValue
	displaySize int

	permID           int64
	echoCount        int
	echoParams       []ibkr.TagValue
	initialStatuses  []ibkr.OrderStatus
	autoCancelReason string

	rejectCode    int
	rejectMessage string
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

// collectAlgoLifecycle consumes the complete captured lifecycle before the
// test emits any targeted cancellation. Warnings are deliberately ignored:
// their raw frames remain frozen, while the public behavior under test is the
// echo shape and status sequence.
func collectAlgoLifecycle(t *testing.T, ctx context.Context, handle *ibkr.OrderHandle, wantEchoes int, wantStatuses []ibkr.OrderStatus) []ibkr.OpenOrder {
	t.Helper()

	var echoes []ibkr.OpenOrder
	var statuses []ibkr.OrderStatus
	for len(echoes) < wantEchoes || len(statuses) < len(wantStatuses) {
		select {
		case evt, ok := <-handle.Events():
			if !ok {
				t.Fatal("order events closed before the captured algo lifecycle")
			}
			if evt.OpenOrder != nil {
				echoes = append(echoes, *evt.OpenOrder)
				if len(echoes) > wantEchoes {
					t.Fatalf("open_order echo count exceeded %d", wantEchoes)
				}
			}
			if evt.Status != nil {
				statuses = append(statuses, evt.Status.Status)
				if len(statuses) > len(wantStatuses) {
					t.Fatalf("order statuses = %v, want %v", statuses, wantStatuses)
				}
			}
		case <-ctx.Done():
			t.Fatal("timeout waiting for algo lifecycle")
		}
	}
	if !slices.Equal(statuses, wantStatuses) {
		t.Fatalf("order statuses = %v, want %v", statuses, wantStatuses)
	}
	return echoes
}

// TestAPIAlgoVariantsReplay freezes the IB algo variant matrix (AORD-003)
// captured live on 2026-08-24 against paper Gateway server_version 225
// (events SHA-256 bf33cd7eea5fe75a06666f1af121980c1b5e863e5b5605291162a0b4e1cff291).
// Thirteen strategies were placed as far LMT BUY 100 AAPL orders. Seven
// produced open-order lifecycles and six were rejected during placement.
// Adaptive Urgent/Patient, AD, and ClosePx rested until targeted cancellation;
// Vwap, ArrivalPx, and PctVol self-cancelled after IBKR shifted their closed-
// venue time windows to July 13. Echo counts and status sequences are kept
// per strategy because current live behavior ranges from one to three echoes.
func TestAPIAlgoVariantsReplay(t *testing.T) {
	t.Parallel()

	variants := []algoVariantCase{
		{
			name:       "adaptive_urgent",
			orderID:    455,
			strategy:   "Adaptive",
			params:     []ibkr.TagValue{algoTag("adaptivePriority", "Urgent")},
			permID:     9000000475,
			echoCount:  3,
			echoParams: []ibkr.TagValue{algoTag("adaptivePriority", "Urgent")},
			initialStatuses: []ibkr.OrderStatus{
				ibkr.OrderStatusPreSubmitted,
				ibkr.OrderStatusPreSubmitted,
				ibkr.OrderStatusPreSubmitted,
			},
		},
		{
			name:       "adaptive_patient",
			orderID:    456,
			strategy:   "Adaptive",
			params:     []ibkr.TagValue{algoTag("adaptivePriority", "Patient")},
			permID:     9000000476,
			echoCount:  3,
			echoParams: []ibkr.TagValue{algoTag("adaptivePriority", "Patient")},
			initialStatuses: []ibkr.OrderStatus{
				ibkr.OrderStatusPreSubmitted,
				ibkr.OrderStatusPreSubmitted,
				ibkr.OrderStatusPreSubmitted,
			},
		},
		{
			name:     "twap",
			orderID:  457,
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
			orderID:  458,
			strategy: "Vwap",
			params: []ibkr.TagValue{
				algoTag("maxPctVol", "0.1"),
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("allowPastEndTime", "1"),
				algoTag("noTakeLiq", "1"),
			},
			permID:    9000000478,
			echoCount: 2,
			echoParams: []ibkr.TagValue{
				algoTag("noTakeLiq", "1"),
				algoTag("allowPastEndTime", "1"),
				algoTag("startTime", algoStartEastern),
				algoTag("maxPctVol", "0.1"),
				algoTag("endTime", algoEndEastern),
			},
			initialStatuses: []ibkr.OrderStatus{
				ibkr.OrderStatusPreSubmitted,
				ibkr.OrderStatusSubmitted,
				ibkr.OrderStatusCancelled,
			},
			autoCancelReason: "Vwap: start time is after close time",
		},
		{
			name:     "arrival_px",
			orderID:  459,
			strategy: "ArrivalPx",
			params: []ibkr.TagValue{
				algoTag("maxPctVol", "0.1"),
				algoTag("riskAversion", "Neutral"),
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("forceCompletion", "0"),
				algoTag("allowPastEndTime", "1"),
			},
			permID:    9000000479,
			echoCount: 2,
			echoParams: []ibkr.TagValue{
				algoTag("riskAversion", "Neutral"),
				algoTag("allowPastEndTime", "1"),
				algoTag("forceCompletion", "0"),
				algoTag("startTime", algoStartEastern),
				algoTag("maxPctVol", "0.1"),
				algoTag("endTime", algoEndEastern),
			},
			initialStatuses: []ibkr.OrderStatus{
				ibkr.OrderStatusPreSubmitted,
				ibkr.OrderStatusSubmitted,
				ibkr.OrderStatusCancelled,
			},
			autoCancelReason: "ArrivalPx: start time is after close time",
		},
		{
			name:     "dark_ice",
			orderID:  460,
			strategy: "DarkIce",
			params: []ibkr.TagValue{
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("allowPastEndTime", "1"),
			},
			displaySize:   1,
			rejectCode:    ibkr.ErrCodeDisplaySizeNotAllowed,
			rejectMessage: "The 'Display Size' order attribute may not be specified for this order.",
		},
		{
			name:     "accum_dist",
			orderID:  461,
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
			permID:    9000000481,
			echoCount: 1,
			echoParams: []ibkr.TagValue{
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
			initialStatuses: []ibkr.OrderStatus{ibkr.OrderStatusPreSubmitted},
		},
		{
			name:     "inline",
			orderID:  462,
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
			orderID:  463,
			strategy: "ClosePx",
			params: []ibkr.TagValue{
				algoTag("maxPctVol", "0.1"),
				algoTag("riskAversion", "Neutral"),
				algoTag("startTime", algoStartUTC),
				algoTag("forceCompletion", "0"),
			},
			permID:    9000000483,
			echoCount: 2,
			echoParams: []ibkr.TagValue{
				algoTag("riskAversion", "Neutral"),
				algoTag("forceCompletion", "0"),
				algoTag("startTime", algoStartEastern),
				algoTag("maxPctVol", "0.1"),
			},
			initialStatuses: []ibkr.OrderStatus{
				ibkr.OrderStatusPreSubmitted,
				ibkr.OrderStatusSubmitted,
			},
			autoCancelReason: "ClosePx: start time is after close time",
		},
		{
			name:     "pct_vol",
			orderID:  464,
			strategy: "PctVol",
			params: []ibkr.TagValue{
				algoTag("pctVol", "0.1"),
				algoTag("startTime", algoStartUTC),
				algoTag("endTime", algoEndUTC),
				algoTag("noTakeLiq", "1"),
			},
			permID:    9000000484,
			echoCount: 2,
			echoParams: []ibkr.TagValue{
				algoTag("noTakeLiq", "1"),
				algoTag("pctVol", "0.1"),
				algoTag("startTime", algoStartEastern),
				algoTag("endTime", algoEndEastern),
			},
			initialStatuses: []ibkr.OrderStatus{
				ibkr.OrderStatusPreSubmitted,
				ibkr.OrderStatusSubmitted,
				ibkr.OrderStatusCancelled,
			},
			autoCancelReason: "PctVol: start time is after close time",
		},
		{
			name:     "balance_impact_risk",
			orderID:  465,
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
			orderID:       466,
			strategy:      "MinImpact",
			params:        []ibkr.TagValue{algoTag("maxPctVol", "0.1")},
			rejectCode:    ibkr.ErrCodeAlgoDefinitionNotFound,
			rejectMessage: "Order processing failed. Algorithm definition not found",
		},
		{
			name:     "jef_ad",
			orderID:  467,
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
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	events := client.SessionEvents()
	replayDelayedAAPLQuoteAnchor(t, ctx, client)
	refs := []string{
		"sanitized-order-ref-0000000000000001",
		"sanitized-order-ref-0000000000000006",
		"sanitized-order-ref-0000000000000011",
		"sanitized-order-ref-0000000000000013",
		"sanitized-order-ref-0000000000000017",
		"sanitized-order-ref-0000000000000021",
		"sanitized-order-ref-0000000000000023",
		"sanitized-order-ref-0000000000000026",
		"sanitized-order-ref-0000000000000028",
		"sanitized-order-ref-0000000000000032",
		"sanitized-order-ref-0000000000000036",
		"sanitized-order-ref-0000000000000038",
		"sanitized-order-ref-0000000000000040",
	}

	for i, v := range variants {
		ref := refs[i]
		t.Run(v.name, func(t *testing.T) {
			handle, err := client.Orders().Place(ctx, ibkr.PlaceOrderRequest{
				Contract: orderReplayAAPL,
				Order: ibkr.Order{
					Action:      ibkr.ActionBuy,
					OrderType:   ibkr.OrderTypeLimit,
					Quantity:    decimal.RequireFromString("1"),
					LmtPrice:    new(decimal.RequireFromString("15.53")),
					TIF:         ibkr.TIFDay,
					Account:     "DU9000001",
					OrderRef:    ref,
					DisplaySize: v.displaySize,
					Algorithm:   ibkr.OrderAlgorithm{Strategy: v.strategy, Params: v.params},
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
				return
			}

			echoes := collectAlgoLifecycle(t, ctx, handle, v.echoCount, v.initialStatuses)
			for i, echo := range echoes {
				if echo.Order.Algorithm.Strategy != v.strategy {
					t.Fatalf("echo %d algo strategy = %q, want %q", i, echo.Order.Algorithm.Strategy, v.strategy)
				}
				requireAlgoParams(t, fmt.Sprintf("echo %d", i), echo.Order.Algorithm.Params, v.echoParams)
			}
			if !echoes[0].Order.Prices.LmtPrice.Equal(decimal.RequireFromString("15.53")) {
				t.Fatalf("echoed lmt price = %s, want 15.53", echoes[0].Order.Prices.LmtPrice)
			}
			if echoes[0].Order.OrderRef != ref {
				t.Fatalf("echoed order ref = %q, want %q", echoes[0].Order.OrderRef, ref)
			}
			if err := handle.Cancel(ctx); err != nil {
				t.Fatalf("Cancel: %v", err)
			}
			if v.autoCancelReason == "" {
				waitForOrderStatus(t, ctx, handle, ibkr.OrderStatusCancelled)
				notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
				if notice.Message != "Order Canceled - reason:" {
					t.Fatalf("202 message = %q", notice.Message)
				}
				return
			}

			notice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCanceled)
			wantNotice := "Order Canceled - reason:" + v.autoCancelReason
			if notice.Message != wantNotice {
				t.Fatalf("202 message = %q, want %q", notice.Message, wantNotice)
			}
			cancelNotice := waitForSessionEventCode(t, ctx, events, ibkr.ErrCodeOrderCannotBeCancelled)
			wantCancelNotice := fmt.Sprintf("OrderId %d that needs to be cancelled cannot be cancelled, state: Cancelled.", v.orderID)
			if cancelNotice.Message != wantCancelNotice {
				t.Fatalf("10148 message = %q, want %q", cancelNotice.Message, wantCancelNotice)
			}
		})
	}
	executions, err := client.Orders().Executions(ctx, ibkr.ExecutionsRequest{Account: "DU9000001", Symbol: "AAPL"})
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(executions.Executions) != 0 || len(executions.CommissionAndFees) != 0 {
		t.Fatalf("algo executions/fees = %d/%d, want 0/0", len(executions.Executions), len(executions.CommissionAndFees))
	}
}
