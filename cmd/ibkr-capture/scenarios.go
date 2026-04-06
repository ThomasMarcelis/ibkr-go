package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

// scenario describes a single live capture scenario. It runs against an
// already-bootstrapped connection.
type scenario struct {
	name        string
	description string
	run         func(ctx context.Context, conn net.Conn, sess *sessionInfo) error
}

// logFrame is the default per-frame logger used by readFrames. It prints a
// compact single line per frame for visibility.
func logFrame(msgID int, fields []string) {
	const maxShown = 8
	var shown string
	for i, f := range fields {
		if i > 0 {
			shown += "|"
		}
		if i >= maxShown {
			shown += fmt.Sprintf("…(+%d more)", len(fields)-maxShown)
			break
		}
		if len(f) > 48 {
			shown += f[:48] + "…"
		} else {
			shown += f
		}
	}
	log.Printf("frame msg_id=%d (%d fields): %s", msgID, len(fields), shown)
}

// stopOnMsgID returns a stop predicate that terminates on the given msg_id.
func stopOnMsgID(id int) func(int, []string) bool {
	return func(msgID int, _ []string) bool { return msgID == id }
}

// stopOnMsgIDWithReq terminates when msg_id matches AND any non-msg_id field
// matches the expected reqID. Scans every field so callers don't need to know
// exact layouts (some end markers have a version field before reqId, others
// don't — the wire format is inconsistent).
func stopOnMsgIDWithReq(id int, expectReqID string, _ int) func(int, []string) bool {
	return func(msgID int, fields []string) bool {
		if msgID != id {
			return false
		}
		for _, f := range fields[1:] {
			if f == expectReqID {
				return true
			}
		}
		return false
	}
}

// nextReqID allocates a deterministic reqID for a scenario starting at 1000.
var scenarioReqIDCounter = 1000

func nextReqID() int {
	scenarioReqIDCounter++
	return scenarioReqIDCounter
}

var scenarios = map[string]scenario{
	// --- Bootstrap-only scenarios ---

	"bootstrap": {
		name:        "bootstrap",
		description: "clean handshake + START_API + farm-status drain (no feature request)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			// Already bootstrapped. Read a few more frames to catch any late farm-status info.
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"bootstrap_client_id_0": {
		name:        "bootstrap_client_id_0",
		description: "same as bootstrap but client_id=0 (required for REQ_ALL_OPEN_ORDERS scope)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},

	// --- Contract details ---

	"contract_details_aapl_stk": {
		name:        "contract_details_aapl_stk",
		description: "REQ_CONTRACT_DATA for AAPL STK SMART USD",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqContractDetails(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}); err != nil {
				return err
			}
			// CONTRACT_DATA_END msg_id=52
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(52, strconv.Itoa(reqID), 1))
		},
	},
	"contract_details_aapl_opt": {
		name:        "contract_details_aapl_opt",
		description: "REQ_CONTRACT_DATA for AAPL OPT SMART USD (all strikes/expiries)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqContractDetails(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "OPT", Exchange: "SMART", Currency: "USD"}); err != nil {
				return err
			}
			return readFrames(conn, 30*time.Second, logFrame, stopOnMsgIDWithReq(52, strconv.Itoa(reqID), 1))
		},
	},
	"contract_details_eurusd_cash": {
		name:        "contract_details_eurusd_cash",
		description: "REQ_CONTRACT_DATA for EUR.USD CASH IDEALPRO",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqContractDetails(conn, reqID, contractSpec{Symbol: "EUR", SecType: "CASH", Exchange: "IDEALPRO", Currency: "USD"}); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(52, strconv.Itoa(reqID), 1))
		},
	},
	"contract_details_es_fut": {
		name:        "contract_details_es_fut",
		description: "REQ_CONTRACT_DATA for ES FUT CME USD front month",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqContractDetails(conn, reqID, contractSpec{Symbol: "ES", SecType: "FUT", Exchange: "CME", Currency: "USD"}); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(52, strconv.Itoa(reqID), 1))
		},
	},
	"contract_details_not_found": {
		name:        "contract_details_not_found",
		description: "REQ_CONTRACT_DATA for bogus symbol (expect ERR_MSG code 200)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqContractDetails(conn, reqID, contractSpec{Symbol: "ZZZZNONE", SecType: "STK", Exchange: "SMART", Currency: "USD"}); err != nil {
				return err
			}
			// Stop on either CONTRACT_DATA_END (52) or ERR_MSG (4) with matching reqID.
			stop := func(msgID int, fields []string) bool {
				if msgID == 52 && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID) {
					return true
				}
				if msgID == 4 && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID) {
					return true
				}
				return false
			}
			return readFrames(conn, 10*time.Second, logFrame, stop)
		},
	},

	// --- Account summary ---

	"account_summary_snapshot": {
		name:        "account_summary_snapshot",
		description: "REQ_ACCOUNT_SUMMARY then cancel after end marker",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqAccountSummary(conn, reqID, "All", "NetLiquidation,TotalCashValue,BuyingPower,ExcessLiquidity"); err != nil {
				return err
			}
			// ACCOUNT_SUMMARY_END msg_id=64
			if err := readFrames(conn, 15*time.Second, logFrame, stopOnMsgIDWithReq(64, strconv.Itoa(reqID), 2)); err != nil {
				return err
			}
			if err := sendCancelAccountSummary(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"account_summary_stream": {
		name:        "account_summary_stream",
		description: "REQ_ACCOUNT_SUMMARY held open for 10s to catch streaming updates",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqAccountSummary(conn, reqID, "All", "NetLiquidation,TotalCashValue,BuyingPower"); err != nil {
				return err
			}
			// Read for 10 seconds regardless of end marker — we want streaming updates.
			if err := readFrames(conn, 10*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelAccountSummary(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"account_summary_two_subs": {
		name:        "account_summary_two_subs",
		description: "two concurrent REQ_ACCOUNT_SUMMARY with different tags",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqA := nextReqID()
			reqB := nextReqID()
			if err := sendReqAccountSummary(conn, reqA, "All", "NetLiquidation"); err != nil {
				return err
			}
			if err := sendReqAccountSummary(conn, reqB, "All", "TotalCashValue"); err != nil {
				return err
			}
			// Read for 5 seconds to capture both snapshot ends.
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelAccountSummary(conn, reqA); err != nil {
				return err
			}
			if err := sendCancelAccountSummary(conn, reqB); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},

	// --- Positions ---

	"positions_snapshot": {
		name:        "positions_snapshot",
		description: "REQ_POSITIONS drained to POSITION_END, then CANCEL_POSITIONS",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqPositions(conn); err != nil {
				return err
			}
			// POSITION_END msg_id=62
			if err := readFrames(conn, 15*time.Second, logFrame, stopOnMsgID(62)); err != nil {
				return err
			}
			if err := sendCancelPositions(conn); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},

	// --- Historical bars ---

	"historical_bars_1d_1h": {
		name:        "historical_bars_1d_1h",
		description: "REQ_HISTORICAL_DATA AAPL STK 1 D / 1 hour / TRADES",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", "1 D", "1 hour", "TRADES", true); err != nil {
				return err
			}
			// HISTORICAL_DATA msg_id=17 is a single frame containing all bars. Stop on it.
			return readFrames(conn, 20*time.Second, logFrame, stopOnMsgIDWithReq(17, strconv.Itoa(reqID), 1))
		},
	},
	"historical_bars_30d_1day": {
		name:        "historical_bars_30d_1day",
		description: "REQ_HISTORICAL_DATA AAPL STK 30 D / 1 day / TRADES",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", "30 D", "1 day", "TRADES", true); err != nil {
				return err
			}
			return readFrames(conn, 20*time.Second, logFrame, stopOnMsgIDWithReq(17, strconv.Itoa(reqID), 1))
		},
	},
	"historical_bars_bidask": {
		name:        "historical_bars_bidask",
		description: "REQ_HISTORICAL_DATA AAPL STK 1 D / 1 hour / BID_ASK",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", "1 D", "1 hour", "BID_ASK", true); err != nil {
				return err
			}
			return readFrames(conn, 20*time.Second, logFrame, stopOnMsgIDWithReq(17, strconv.Itoa(reqID), 1))
		},
	},
	"historical_bars_error": {
		name:        "historical_bars_error",
		description: "REQ_HISTORICAL_DATA with bogus symbol (expect ERR_MSG)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "ZZZZNONE", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", "1 D", "1 hour", "TRADES", true); err != nil {
				return err
			}
			stop := func(msgID int, fields []string) bool {
				if msgID == 17 && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID) {
					return true
				}
				if msgID == 4 && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID) {
					return true
				}
				return false
			}
			return readFrames(conn, 10*time.Second, logFrame, stop)
		},
	},

	// --- Market data quotes ---

	"quote_snapshot_aapl": {
		name:        "quote_snapshot_aapl",
		description: "REQ_MKT_DATA snapshot=true for AAPL, drain to TICK_SNAPSHOT_END (delayed data)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqMktData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", true); err != nil {
				return err
			}
			// TICK_SNAPSHOT_END msg_id=57
			return readFrames(conn, 15*time.Second, logFrame, stopOnMsgIDWithReq(57, strconv.Itoa(reqID), 1))
		},
	},
	"quote_stream_aapl": {
		name:        "quote_stream_aapl",
		description: "REQ_MKT_DATA snapshot=false for AAPL, 10s observation (delayed data), then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqMktData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", false); err != nil {
				return err
			}
			if err := readFrames(conn, 10*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelMktData(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"quote_stream_genericticks": {
		name:        "quote_stream_genericticks",
		description: "REQ_MKT_DATA with generic tick list 233,236 (delayed data)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqMktData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "233,236", false); err != nil {
				return err
			}
			if err := readFrames(conn, 10*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelMktData(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},

	// --- Real-time bars ---

	"realtime_bars_aapl": {
		name:        "realtime_bars_aapl",
		description: "REQ_REAL_TIME_BARS AAPL, 15s observation (delayed data), cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqRealTimeBars(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "TRADES", true); err != nil {
				return err
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelRealTimeBars(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},

	// --- Open orders ---

	"open_orders_empty": {
		name:        "open_orders_empty",
		description: "REQ_OPEN_ORDERS drained to OPEN_ORDER_END",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqOpenOrders(conn); err != nil {
				return err
			}
			// OPEN_ORDER_END msg_id=53
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(53))
		},
	},
	"open_orders_all": {
		name:        "open_orders_all",
		description: "REQ_ALL_OPEN_ORDERS drained to OPEN_ORDER_END (requires client_id=0)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqAllOpenOrders(conn); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(53))
		},
	},

	// --- Executions ---

	"executions_snapshot": {
		name:        "executions_snapshot",
		description: "REQ_EXECUTIONS with empty filter drained to EXECUTION_DATA_END",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqExecutions(conn, reqID); err != nil {
				return err
			}
			// EXECUTION_DATA_END msg_id=55
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(55, strconv.Itoa(reqID), 1))
		},
	},
}
