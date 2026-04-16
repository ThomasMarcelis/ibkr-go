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
	runAPI      func(ctx context.Context, addr string, clientID int) error
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

func captureHistoricalTickTime(t time.Time) string {
	return t.UTC().Format("20060102 15:04:05") + " UTC"
}

func captureHistoricalNewsTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05") + " UTC"
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
	"current_time": {
		name:        "current_time",
		description: "REQ_CURRENT_TIME, drain until CURRENT_TIME (msg_id=49)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqCurrentTime(conn); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, stopOnMsgID(49))
		},
	},
	"req_ids": {
		name:        "req_ids",
		description: "REQ_IDS numIds=1, drain until NEXT_VALID_ID (msg_id=9)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqIds(conn, 1); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, stopOnMsgID(9))
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
			// msg 52 wire: [52, version, reqID] — reqID position varies, use field-scanning helper.
			reqIDStr := strconv.Itoa(reqID)
			endPred := stopOnMsgIDWithReq(52, reqIDStr, 0)
			stop := func(msgID int, fields []string) bool {
				return endPred(msgID, fields) ||
					(msgID == 4 && len(fields) >= 2 && fields[1] == reqIDStr)
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
	"historical_schedule_aapl": {
		name:        "historical_schedule_aapl",
		description: "REQ_HISTORICAL_DATA AAPL STK 1 M / 1 day / SCHEDULE",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", "1 M", "1 day", "SCHEDULE", true); err != nil {
				return err
			}
			// Stop on either an api_error or the historical_schedule callback
			// (msg_id 106, InHistoricalSchedule in internal/codec/msgid.go)
			// for our reqID. The 15 s deadline is a safety net only.
			stop := func(msgID int, fields []string) bool {
				if len(fields) < 2 || fields[1] != strconv.Itoa(reqID) {
					return false
				}
				return msgID == 4 || msgID == 106
			}
			return readFrames(conn, 15*time.Second, logFrame, stop)
		},
	},

	// --- Market data type control (MarketData().SetType) ---

	"set_type_live": {
		name:        "set_type_live",
		description: "REQ_MARKET_DATA_TYPE=1 (live), drain for marketDataType push",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 1); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"set_type_frozen": {
		name:        "set_type_frozen",
		description: "REQ_MARKET_DATA_TYPE=2 (frozen), drain for marketDataType push",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 2); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"set_type_delayed": {
		name:        "set_type_delayed",
		description: "REQ_MARKET_DATA_TYPE=3 (delayed), drain for marketDataType push",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"set_type_delayed_frozen": {
		name:        "set_type_delayed_frozen",
		description: "REQ_MARKET_DATA_TYPE=4 (delayed-frozen), drain for marketDataType push",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 4); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"set_type_invalid": {
		name:        "set_type_invalid",
		description: "REQ_MARKET_DATA_TYPE=99 (invalid), drain for real IBKR API error",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 99); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, stopOnMsgID(4))
		},
	},
	"set_type_switch_while_streaming": {
		name:        "set_type_switch_while_streaming",
		description: "Start delayed quote stream, switch SetType to live mid-stream, drain, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqMktData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", false); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendReqMarketDataType(conn, 1); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelMktData(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 2*time.Second, logFrame, nil)
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

	// --- v1 expanded scope: Batch C1 — singleton one-shots (no reqID) ---

	"family_codes": {
		name:        "family_codes",
		description: "REQ_FAMILY_CODES, read FAMILY_CODES response (msg 78)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqFamilyCodes(conn); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(78))
		},
	},
	"news_providers": {
		name:        "news_providers",
		description: "REQ_NEWS_PROVIDERS, read NEWS_PROVIDERS response (msg 86)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqNewsProviders(conn); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(86))
		},
	},
	"mkt_depth_exchanges": {
		name:        "mkt_depth_exchanges",
		description: "REQ_MKT_DEPTH_EXCHANGES, read response (msg 79)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMktDepthExchanges(conn); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(79))
		},
	},
	"scanner_parameters": {
		name:        "scanner_parameters",
		description: "REQ_SCANNER_PARAMETERS, read response (msg 19)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqScannerParameters(conn); err != nil {
				return err
			}
			return readFrames(conn, 30*time.Second, logFrame, stopOnMsgID(19))
		},
	},

	// --- v1 expanded scope: Batch C2 — keyed one-shots ---

	"user_info": {
		name:        "user_info",
		description: "REQ_USER_INFO, read USER_INFO response (msg 103)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqUserInfo(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(103))
		},
	},
	"matching_symbols_aapl": {
		name:        "matching_symbols_aapl",
		description: "REQ_MATCHING_SYMBOLS pattern=AAPL, read SYMBOL_SAMPLES (msg 79)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqMatchingSymbols(conn, reqID, "AAPL"); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(79, strconv.Itoa(reqID), 0))
		},
	},
	"matching_symbols_partial": {
		name:        "matching_symbols_partial",
		description: "REQ_MATCHING_SYMBOLS pattern=AA (partial), read SYMBOL_SAMPLES (msg 79)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqMatchingSymbols(conn, reqID, "AA"); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(79, strconv.Itoa(reqID), 0))
		},
	},
	"head_timestamp_aapl": {
		name:        "head_timestamp_aapl",
		description: "REQ_HEAD_TIMESTAMP AAPL/STK/TRADES, read HEAD_TIMESTAMP (msg 88)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHeadTimestamp(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "TRADES", true); err != nil {
				return err
			}
			return readFrames(conn, 15*time.Second, logFrame, stopOnMsgIDWithReq(88, strconv.Itoa(reqID), 0))
		},
	},
	"sec_def_opt_params_aapl": {
		name:        "sec_def_opt_params_aapl",
		description: "REQ_SEC_DEF_OPT_PARAMS AAPL/STK conId=265598, read responses (msg 75+76)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			// AAPL conId=265598 (from contract details)
			if err := sendReqSecDefOptParams(conn, reqID, "AAPL", "", "STK", 265598); err != nil {
				return err
			}
			// SEC_DEF_OPT_PARAMS_END msg_id=76
			return readFrames(conn, 15*time.Second, logFrame, stopOnMsgIDWithReq(76, strconv.Itoa(reqID), 0))
		},
	},
	"histogram_data_aapl": {
		name:        "histogram_data_aapl",
		description: "REQ_HISTOGRAM_DATA AAPL/1 week, read response (msg 89)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistogramData(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, true, "1 week"); err != nil {
				return err
			}
			stop := func(msgID int, fields []string) bool {
				return (msgID == 89 || msgID == 4) && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 15*time.Second, logFrame, stop)
		},
	},
	"historical_ticks_aapl_trades": {
		name:        "historical_ticks_aapl_trades",
		description: "REQ_HISTORICAL_TICKS AAPL/TRADES, read response (msg 98)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			// Request last 100 trade ticks ending now.
			if err := sendReqHistoricalTicks(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", captureHistoricalTickTime(time.Now()), 100, "TRADES", true, false); err != nil {
				return err
			}
			// historicalTicksLast msg_id=98
			stop := func(msgID int, fields []string) bool {
				return (msgID == 96 || msgID == 97 || msgID == 98 || msgID == 4) && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 20*time.Second, logFrame, stop)
		},
	},
	"historical_ticks_aapl_bidask": {
		name:        "historical_ticks_aapl_bidask",
		description: "REQ_HISTORICAL_TICKS AAPL/BID_ASK, read response (msg 97)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalTicks(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", captureHistoricalTickTime(time.Now()), 100, "BID_ASK", true, false); err != nil {
				return err
			}
			stop := func(msgID int, fields []string) bool {
				return (msgID == 96 || msgID == 97 || msgID == 98 || msgID == 4) && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 20*time.Second, logFrame, stop)
		},
	},
	"historical_ticks_aapl_midpoint": {
		name:        "historical_ticks_aapl_midpoint",
		description: "REQ_HISTORICAL_TICKS AAPL/MIDPOINT, read response (msg 96)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalTicks(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "", captureHistoricalTickTime(time.Now()), 100, "MIDPOINT", true, false); err != nil {
				return err
			}
			stop := func(msgID int, fields []string) bool {
				return (msgID == 96 || msgID == 97 || msgID == 98 || msgID == 4) && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 20*time.Second, logFrame, stop)
		},
	},
	"historical_news_aapl": {
		name:        "historical_news_aapl",
		description: "REQ_HISTORICAL_NEWS AAPL conId=265598, read responses (msg 86+87)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalNews(conn, reqID, 265598, "BRFG:BRFUPDN:DJNL", "", "", 10); err != nil {
				return err
			}
			// HISTORICAL_NEWS_END msg_id=87
			stop := func(msgID int, fields []string) bool {
				return (msgID == 87 || msgID == 4) && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 15*time.Second, logFrame, stop)
		},
	},
	"historical_ticks_aapl_timezone_window": {
		name:        "historical_ticks_aapl_timezone_window",
		description: "REQ_HISTORICAL_TICKS AAPL with explicit UTC start/end timezone windows for TRADES, BID_ASK, MIDPOINT",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			end := time.Now()
			start := end.Add(-30 * time.Minute)
			for _, what := range []string{"TRADES", "BID_ASK", "MIDPOINT"} {
				reqID := nextReqID()
				if err := sendReqHistoricalTicks(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, captureHistoricalTickTime(start), captureHistoricalTickTime(end), 50, what, true, false); err != nil {
					return err
				}
				reqIDStr := strconv.Itoa(reqID)
				stop := func(msgID int, fields []string) bool {
					return (msgID == 96 || msgID == 97 || msgID == 98 || msgID == 4) && len(fields) >= 2 && fields[1] == reqIDStr
				}
				if err := readFrames(conn, 20*time.Second, logFrame, stop); err != nil {
					return err
				}
			}
			return nil
		},
	},
	"historical_news_aapl_timezone_window": {
		name:        "historical_news_aapl_timezone_window",
		description: "REQ_HISTORICAL_NEWS AAPL with explicit UTC start/end timezone window",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			end := time.Now()
			start := end.AddDate(0, 0, -7)
			if err := sendReqHistoricalNews(conn, reqID, 265598, "BRFG+BRFUPDN+DJNL", captureHistoricalNewsTime(start), captureHistoricalNewsTime(end), 20); err != nil {
				return err
			}
			reqIDStr := strconv.Itoa(reqID)
			stop := func(msgID int, fields []string) bool {
				return (msgID == 87 || msgID == 4) && len(fields) >= 2 && fields[1] == reqIDStr
			}
			return readFrames(conn, 20*time.Second, logFrame, stop)
		},
	},

	// --- v1 expanded scope: Batch C3 — completed orders and tick types ---

	"completed_orders": {
		name:        "completed_orders",
		description: "REQ_COMPLETED_ORDERS apiOnly=true, read to COMPLETED_ORDERS_END (msg 102)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqCompletedOrders(conn, true); err != nil {
				return err
			}
			return readFrames(conn, 15*time.Second, logFrame, stopOnMsgID(102))
		},
	},
	"quote_with_generic_ticks": {
		name:        "quote_with_generic_ticks",
		description: "REQ_MKT_DATA with generic ticks 100,101,104,106,233,236,258 to observe TickGeneric/TickString/TickReqParams",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqMktData(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "100,101,104,106,233,236,258", false); err != nil {
				return err
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelMktData(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"quote_stream_multi_asset": {
		name:        "quote_stream_multi_asset",
		description: "concurrent delayed REQ_MKT_DATA streams for AAPL stock and EUR.USD cash, then cancel both",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqAAPL := nextReqID()
			reqEURUSD := nextReqID()
			if err := sendReqMktData(conn, reqAAPL, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "100,101,104,106,233,236,258", false); err != nil {
				return err
			}
			if err := sendReqMktData(conn, reqEURUSD, sess.ServerVersion, contractSpec{Symbol: "EUR", SecType: "CASH", Exchange: "IDEALPRO", Currency: "USD"}, "", false); err != nil {
				return err
			}
			if err := readFrames(conn, 20*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelMktData(conn, reqAAPL); err != nil {
				return err
			}
			if err := sendCancelMktData(conn, reqEURUSD); err != nil {
				return err
			}
			return readFrames(conn, 2*time.Second, logFrame, nil)
		},
	},

	// --- v1 expanded scope: Batch C4 — streaming subscriptions ---

	"account_updates": {
		name:        "account_updates",
		description: "REQ_ACCOUNT_UPDATES subscribe=true, read for 10s, then unsubscribe",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			acct := sess.ManagedAccounts
			if err := sendReqAccountUpdates(conn, true, acct); err != nil {
				return err
			}
			// Read until ACCOUNT_DOWNLOAD_END (msg 54) then a bit more for streaming.
			if err := readFrames(conn, 10*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendReqAccountUpdates(conn, false, acct); err != nil {
				return err
			}
			return readFrames(conn, 2*time.Second, logFrame, nil)
		},
	},
	"account_updates_multi": {
		name:        "account_updates_multi",
		description: "REQ_ACCOUNT_UPDATES_MULTI, read to end marker (msg 74), then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			acct := sess.ManagedAccounts
			if err := sendReqAccountUpdatesMulti(conn, reqID, acct, ""); err != nil {
				return err
			}
			// ACCOUNT_UPDATE_MULTI_END msg_id=74
			if err := readFrames(conn, 15*time.Second, logFrame, stopOnMsgIDWithReq(74, strconv.Itoa(reqID), 0)); err != nil {
				return err
			}
			if err := sendCancelAccountUpdatesMulti(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"positions_multi": {
		name:        "positions_multi",
		description: "REQ_POSITIONS_MULTI, read to POSITION_MULTI_END (msg 72), then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			acct := sess.ManagedAccounts
			if err := sendReqPositionsMulti(conn, reqID, acct, ""); err != nil {
				return err
			}
			// POSITION_MULTI_END msg_id=72
			if err := readFrames(conn, 15*time.Second, logFrame, stopOnMsgIDWithReq(72, strconv.Itoa(reqID), 0)); err != nil {
				return err
			}
			if err := sendCancelPositionsMulti(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"pnl": {
		name:        "pnl",
		description: "REQ_PNL for account, read for 5s, then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			acct := sess.ManagedAccounts
			if err := sendReqPnL(conn, reqID, acct, ""); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelPnL(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"pnl_single": {
		name:        "pnl_single",
		description: "REQ_PNL_SINGLE for AAPL conId=265598, read for 5s, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			acct := sess.ManagedAccounts
			if err := sendReqPnLSingle(conn, reqID, acct, "", 265598); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelPnLSingle(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"tick_by_tick_last": {
		name:        "tick_by_tick_last",
		description: "REQ_TICK_BY_TICK_DATA Last for AAPL (delayed), read for 15s, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqTickByTickData(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "Last", 0, false); err != nil {
				return err
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelTickByTickData(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"tick_by_tick_bidask": {
		name:        "tick_by_tick_bidask",
		description: "REQ_TICK_BY_TICK_DATA BidAsk for AAPL (delayed), read for 15s, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqTickByTickData(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "BidAsk", 0, false); err != nil {
				return err
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelTickByTickData(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"tick_by_tick_midpoint": {
		name:        "tick_by_tick_midpoint",
		description: "REQ_TICK_BY_TICK_DATA MidPoint for AAPL (delayed), read for 15s, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqMarketDataType(conn, 3); err != nil {
				return err
			}
			reqID := nextReqID()
			if err := sendReqTickByTickData(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "MidPoint", 0, false); err != nil {
				return err
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelTickByTickData(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"historical_bars_keepup": {
		name:        "historical_bars_keepup",
		description: "REQ_HISTORICAL_DATA keepUpToDate=true for AAPL 5 secs bars, read 15s, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqHistoricalDataKeepUp(conn, reqID, sess.ServerVersion, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "5 secs", "TRADES", true); err != nil {
				return err
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelHistoricalData(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"news_bulletins": {
		name:        "news_bulletins",
		description: "REQ_NEWS_BULLETINS allMessages=true, read for 5s, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendReqNewsBulletins(conn, true); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelNewsBulletins(conn); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},

	// --- v1 expanded scope: Batch C5 — option calculations and scanner ---

	"scanner_subscription": {
		name:        "scanner_subscription",
		description: "REQ_SCANNER_SUBSCRIPTION top 10 hot US stocks, read response, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqScannerSubscription(conn, reqID, 10, "STK", "STK.US.MAJOR", "HOT_BY_VOLUME"); err != nil {
				return err
			}
			// SCANNER_DATA msg_id=20: wire [20, version=3, reqID, ...] — reqID at fields[2].
			reqIDStr := strconv.Itoa(reqID)
			endPred := stopOnMsgIDWithReq(20, reqIDStr, 0)
			stop := func(msgID int, fields []string) bool {
				return endPred(msgID, fields) ||
					(msgID == 4 && len(fields) >= 2 && fields[1] == reqIDStr)
			}
			if err := readFrames(conn, 15*time.Second, logFrame, stop); err != nil {
				return err
			}
			if err := sendCancelScannerSubscription(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"smart_components": {
		name:        "smart_components",
		description: "REQ_SMART_COMPONENTS for AAPL bboExchange=9c0001, read response",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqSmartComponents(conn, reqID, "9c0001"); err != nil {
				return err
			}
			// Read for 10s, stop on any response with matching reqID.
			return readFrames(conn, 10*time.Second, logFrame, nil)
		},
	},
	"market_rule": {
		name:        "market_rule",
		description: "REQ_MARKET_RULE id=26 (common US equity rule), read response (msg 92)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			// Market rule 26 is common for US equities on SMART/ISLAND.
			if err := sendReqMarketRule(conn, 26); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(92))
		},
	},

	// --- Order management ---

	"place_order_lmt_buy_aapl": {
		name:        "place_order_lmt_buy_aapl",
		description: "PLACE_ORDER LMT buy 1 AAPL at $50 (far below market), observe status, then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_mkt_buy_aapl": {
		name:        "place_order_mkt_buy_aapl",
		description: "PLACE_ORDER MKT buy 1 AAPL (will fill), observe status",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"place_order_mkt_sell_aapl": {
		name:        "place_order_mkt_sell_aapl",
		description: "PLACE_ORDER MKT sell 1 AAPL, observe status",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "SELL", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"place_order_modify": {
		name:        "place_order_modify",
		description: "PLACE_ORDER LMT buy 1 AAPL at $50, modify to $51, then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			// Modify: send PlaceOrder again with same orderID but new price.
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "51.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_cancel": {
		name:        "place_order_cancel",
		description: "PLACE_ORDER LMT buy 1 AAPL at $50, then cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 2*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_direct_cancel": {
		name:        "place_order_direct_cancel",
		description: "PLACE_ORDER LMT buy 1 AAPL at $50, then cancel via Orders().Cancel(orderID)",
		// Wire-identical to place_order_cancel. The scenario exists so the
		// replay transcript and integration test can exercise the direct-by-ID
		// public facade path, which is conceptually different from the
		// OrderHandle.Cancel flow even though both emit OutCancelOrder=4.
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			if err := readFrames(conn, 2*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_bracket_aapl": {
		name:        "place_order_bracket_aapl",
		description: "bracket order: MKT parent + LMT take-profit + STP stop-loss for AAPL",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			parentID := sess.NextValidID
			tpID := sess.NextValidID + 1
			slID := sess.NextValidID + 2
			sess.NextValidID += 3
			acct := sess.ManagedAccounts
			aaplSTK := contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}

			// Parent: MKT buy, transmit=false (held until children submitted).
			if err := sendPlaceOrder(conn, parentID, aaplSTK, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, Transmit: false}); err != nil {
				return err
			}
			// Take-profit: LMT sell at $300 (far above market), transmit=false.
			if err := sendPlaceOrder(conn, tpID, aaplSTK, orderSpec{Action: "SELL", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "300.00", TIF: "GTC", Account: acct, ParentID: parentID, Transmit: false}); err != nil {
				return err
			}
			// Stop-loss: STP sell at $50 (far below market), transmit=true (triggers all 3).
			if err := sendPlaceOrder(conn, slID, aaplSTK, orderSpec{Action: "SELL", TotalQuantity: "1", OrderType: "STP", AuxPrice: "50.00", TIF: "GTC", Account: acct, ParentID: parentID, Transmit: true}); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, nil)
		},
	},
	"global_cancel": {
		name:        "global_cancel",
		description: "place 3 LMT buy orders at $50, then GLOBAL_CANCEL",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			acct := sess.ManagedAccounts
			aaplSTK := contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}
			for i := 0; i < 3; i++ {
				orderID := sess.NextValidID
				sess.NextValidID++
				if err := sendPlaceOrder(conn, orderID, aaplSTK, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "LMT", LmtPrice: "50.00", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
					return err
				}
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendGlobalCancel(conn); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"place_order_option_buy": {
		name:        "place_order_option_buy",
		description: "PLACE_ORDER buy 1 AAPL far-OTM call option, observe status",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			// Hardcoded far-OTM AAPL call. The exact contract may need adjustment
			// for current market conditions; the capture will show any error response.
			optContract := contractSpec{
				Symbol:   "AAPL",
				SecType:  "OPT",
				Exchange: "SMART",
				Currency: "USD",
				Right:    "C",
				Strike:   300.0,
				// Use a distant expiry to maximize chance of existence.
				LastTradeDateOrContractMonth: "20261218",
				Multiplier:                   "100",
			}
			if err := sendPlaceOrder(conn, orderID, optContract, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, Transmit: true}); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"place_order_algo_adaptive_aapl": {
		name:        "place_order_algo_adaptive_aapl",
		description: "PLACE_ORDER LMT buy 1 AAPL with Adaptive algo, observe open-order wire",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{
				Action:        "BUY",
				TotalQuantity: "1",
				OrderType:     "LMT",
				LmtPrice:      "50.00",
				TIF:           "DAY",
				Account:       acct,
				Transmit:      true,
				AlgoStrategy:  "Adaptive",
				AlgoParams:    []tagValueSpec{{Tag: "adaptivePriority", Value: "Normal"}},
			}); err != nil {
				return err
			}
			if err := readFrames(conn, 4*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_price_condition_aapl": {
		name:        "place_order_price_condition_aapl",
		description: "PLACE_ORDER LMT buy 1 AAPL with a high price condition so it stays inactive",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			orderID := sess.NextValidID
			sess.NextValidID++
			acct := sess.ManagedAccounts
			if err := sendPlaceOrder(conn, orderID, contractSpec{ConID: 265598, Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, orderSpec{
				Action:        "BUY",
				TotalQuantity: "1",
				OrderType:     "LMT",
				LmtPrice:      "50.00",
				TIF:           "DAY",
				Account:       acct,
				Transmit:      true,
				Conditions: []orderConditionSpec{{
					Type:          1,
					Conjunction:   "a",
					Operator:      2,
					ConID:         265598,
					Exchange:      "SMART",
					Value:         "9999.00",
					TriggerMethod: 4,
				}},
				ConditionsIgnoreRTH:   false,
				ConditionsCancelOrder: false,
			}); err != nil {
				return err
			}
			if err := readFrames(conn, 4*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelOrder(conn, orderID); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},
	"place_order_oca_pair_aapl": {
		name:        "place_order_oca_pair_aapl",
		description: "place two far-from-market AAPL LMT orders in one OCA group, then GLOBAL_CANCEL",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			acct := sess.ManagedAccounts
			aaplSTK := contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}
			oca := "ibkr-go-live-oca-" + strconv.FormatInt(time.Now().Unix(), 10)
			for i, side := range []string{"BUY", "SELL"} {
				orderID := sess.NextValidID
				sess.NextValidID++
				price := "50.00"
				if side == "SELL" {
					price = "500.00"
				}
				if err := sendPlaceOrder(conn, orderID, aaplSTK, orderSpec{Action: side, TotalQuantity: "1", OrderType: "LMT", LmtPrice: price, TIF: "DAY", Account: acct, OcaGroup: oca, OrderRef: fmt.Sprintf("ibkr-go-oca-%d", i), Transmit: true}); err != nil {
					return err
				}
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendGlobalCancel(conn); err != nil {
				return err
			}
			return readFrames(conn, 5*time.Second, logFrame, nil)
		},
	},
	"trading_split_round_trip_aapl": {
		name:        "trading_split_round_trip_aapl",
		description: "account baseline, split AAPL market buys, executions/completed orders, split sells, final account/position/PnL probes",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			acct := sess.ManagedAccounts
			aaplSTK := contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}
			accountReqID := nextReqID()
			if err := sendReqAccountSummary(conn, accountReqID, "All", "NetLiquidation,TotalCashValue,BuyingPower,ExcessLiquidity"); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelAccountSummary(conn, accountReqID); err != nil {
				return err
			}
			if err := sendReqPositions(conn); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, stopOnMsgID(62)); err != nil {
				return err
			}

			for i := 0; i < 2; i++ {
				orderID := sess.NextValidID
				sess.NextValidID++
				if err := sendPlaceOrder(conn, orderID, aaplSTK, orderSpec{Action: "BUY", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, OrderRef: fmt.Sprintf("ibkr-go-split-buy-%d", i), Transmit: true}); err != nil {
					return err
				}
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}

			execReqID := nextReqID()
			if err := sendReqExecutions(conn, execReqID); err != nil {
				return err
			}
			if err := readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(55, strconv.Itoa(execReqID), 1)); err != nil {
				return err
			}
			if err := sendReqCompletedOrders(conn, true); err != nil {
				return err
			}
			if err := readFrames(conn, 10*time.Second, logFrame, stopOnMsgID(102)); err != nil {
				return err
			}

			for i := 0; i < 2; i++ {
				orderID := sess.NextValidID
				sess.NextValidID++
				if err := sendPlaceOrder(conn, orderID, aaplSTK, orderSpec{Action: "SELL", TotalQuantity: "1", OrderType: "MKT", TIF: "DAY", Account: acct, OrderRef: fmt.Sprintf("ibkr-go-split-sell-%d", i), Transmit: true}); err != nil {
					return err
				}
			}
			if err := readFrames(conn, 15*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendReqPositions(conn); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, stopOnMsgID(62)); err != nil {
				return err
			}
			pnlReqID := nextReqID()
			if err := sendReqPnL(conn, pnlReqID, acct, ""); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelPnL(conn, pnlReqID); err != nil {
				return err
			}
			if err := sendGlobalCancel(conn); err != nil {
				return err
			}
			return readFrames(conn, 3*time.Second, logFrame, nil)
		},
	},

	// --- Market depth ---

	"market_depth_aapl": {
		name:        "market_depth_aapl",
		description: "REQ_MKT_DEPTH AAPL numRows=5 isSmartDepth=false, read 10s, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqMktDepth(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, 5, false); err != nil {
				return err
			}
			if err := readFrames(conn, 10*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelMktDepth(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"market_depth_aapl_smart": {
		name:        "market_depth_aapl_smart",
		description: "REQ_MKT_DEPTH AAPL numRows=5 isSmartDepth=true, read 10s, cancel",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqMktDepth(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, 5, true); err != nil {
				return err
			}
			if err := readFrames(conn, 10*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendCancelMktDepth(conn, reqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},

	// --- Reference data ---

	"fundamental_data_aapl": {
		name:        "fundamental_data_aapl",
		description: "REQ_FUNDAMENTAL_DATA ReportSnapshot for AAPL (may error without subscription)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqFundamentalData(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}, "ReportSnapshot"); err != nil {
				return err
			}
			// FUNDAMENTAL_DATA: [51, version, reqID, data] — reqID at fields[2].
			// ERR_MSG: [4, reqID, code, msg, ...] — reqID at fields[1].
			stop := func(msgID int, fields []string) bool {
				if msgID == 51 && len(fields) >= 3 && fields[2] == strconv.Itoa(reqID) {
					return true
				}
				return msgID == 4 && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 10*time.Second, logFrame, stop)
		},
	},
	"soft_dollar_tiers": {
		name:        "soft_dollar_tiers",
		description: "REQ_SOFT_DOLLAR_TIERS, read response (msg 77)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqSoftDollarTiers(conn, reqID); err != nil {
				return err
			}
			// SOFT_DOLLAR_TIERS: [77, reqID, count, ...] — reqID at fields[1].
			stop := func(msgID int, fields []string) bool {
				return (msgID == 77 || msgID == 4) && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 5*time.Second, logFrame, stop)
		},
	},
	"display_groups": {
		name:        "display_groups",
		description: "QUERY_DISPLAY_GROUPS, read response (msg 67)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendQueryDisplayGroups(conn, reqID); err != nil {
				return err
			}
			// DISPLAY_GROUP_LIST: [67, version, reqID, groups] — reqID at fields[2].
			stop := func(msgID int, fields []string) bool {
				if msgID == 67 && len(fields) >= 3 && fields[2] == strconv.Itoa(reqID) {
					return true
				}
				return msgID == 4 && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 5*time.Second, logFrame, stop)
		},
	},
	"display_group_subscribe": {
		name:        "display_group_subscribe",
		description: "QUERY_DISPLAY_GROUPS then SUBSCRIBE_TO_GROUP_EVENTS for group 1, observe, unsubscribe",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			queryReqID := nextReqID()
			if err := sendQueryDisplayGroups(conn, queryReqID); err != nil {
				return err
			}
			if err := readFrames(conn, 3*time.Second, logFrame, nil); err != nil {
				return err
			}
			subReqID := nextReqID()
			if err := sendSubscribeToGroupEvents(conn, subReqID, 1); err != nil {
				return err
			}
			if err := readFrames(conn, 5*time.Second, logFrame, nil); err != nil {
				return err
			}
			if err := sendUnsubscribeFromGroupEvents(conn, subReqID); err != nil {
				return err
			}
			return readFrames(conn, 1*time.Second, logFrame, nil)
		},
	},
	"wsh_meta_data": {
		name:        "wsh_meta_data",
		description: "REQ_WSH_META_DATA, read response (msg 105 or error)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqWSHMetaData(conn, reqID); err != nil {
				return err
			}
			// WSH_META_DATA: [105, reqID, data] — reqID at fields[1].
			stop := func(msgID int, fields []string) bool {
				return (msgID == 105 || msgID == 4) && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 5*time.Second, logFrame, stop)
		},
	},
	"wsh_event_data_aapl": {
		name:        "wsh_event_data_aapl",
		description: "REQ_WSH_EVENT_DATA for AAPL conId=265598, read response (msg 106 or error)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqWSHEventData(conn, reqID, 265598, "", false, false, false, "", "", 10); err != nil {
				return err
			}
			// WSH_EVENT_DATA: [106, reqID, data] — reqID at fields[1].
			stop := func(msgID int, fields []string) bool {
				return (msgID == 106 || msgID == 4) && len(fields) >= 2 && fields[1] == strconv.Itoa(reqID)
			}
			return readFrames(conn, 5*time.Second, logFrame, stop)
		},
	},
	"request_fa": {
		name:        "request_fa",
		description: "REQUEST_FA groups (faDataType=1), read response (may error on non-FA accounts)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			if err := sendRequestFA(conn, 1); err != nil {
				return err
			}
			// RECEIVE_FA msg_id=16 or ERR_MSG msg_id=4.
			stop := func(msgID int, fields []string) bool {
				if msgID == 16 {
					return true
				}
				if msgID == 4 && len(fields) >= 4 {
					code, err := strconv.Atoi(fields[3])
					if err == nil && code >= 2000 && code < 3000 {
						return false // skip farm-status bootstrap messages
					}
					return true
				}
				return false
			}
			return readFrames(conn, 5*time.Second, logFrame, stop)
		},
	},
	"qualify_contract_aapl_exact": {
		name:        "qualify_contract_aapl_exact",
		description: "REQ_CONTRACT_DATA for fully-qualified AAPL STK SMART USD",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqContractDetails(conn, reqID, contractSpec{Symbol: "AAPL", SecType: "STK", Exchange: "SMART", Currency: "USD"}); err != nil {
				return err
			}
			return readFrames(conn, 10*time.Second, logFrame, stopOnMsgIDWithReq(52, strconv.Itoa(reqID), 1))
		},
	},
	"qualify_contract_ambiguous": {
		name:        "qualify_contract_ambiguous",
		description: "REQ_CONTRACT_DATA with just symbol=MSFT, no exchange (may return multiple results)",
		run: func(ctx context.Context, conn net.Conn, sess *sessionInfo) error {
			reqID := nextReqID()
			if err := sendReqContractDetails(conn, reqID, contractSpec{Symbol: "MSFT", SecType: "STK", Currency: "USD"}); err != nil {
				return err
			}
			// CONTRACT_DATA_END msg_id=52: wire [52, version, reqID] — use field-scanning helper.
			reqIDStr := strconv.Itoa(reqID)
			endPred := stopOnMsgIDWithReq(52, reqIDStr, 0)
			stop := func(msgID int, fields []string) bool {
				return endPred(msgID, fields) ||
					(msgID == 4 && len(fields) >= 2 && fields[1] == reqIDStr)
			}
			return readFrames(conn, 10*time.Second, logFrame, stop)
		},
	},
	"api_order_type_matrix_aapl": {
		name:        "api_order_type_matrix_aapl",
		description: "public API campaign for AAPL order type breadth: fills, rests, rejects, modifies, and cancels",
		runAPI:      runAPIOrderTypeMatrixAAPL,
	},
	"api_order_fill_aapl": {
		name:        "api_order_fill_aapl",
		description: "public API campaign for AAPL fill paths: MKT, MTL, and delayed modify-to-market",
		runAPI:      runAPIOrderFillAAPL,
	},
	"api_order_rest_cancel_aapl": {
		name:        "api_order_rest_cancel_aapl",
		description: "public API campaign for AAPL resting order types and cancel/reject behavior",
		runAPI:      runAPIOrderRestCancelAAPL,
	},
	"api_order_relative_cancel_aapl": {
		name:        "api_order_relative_cancel_aapl",
		description: "public API campaign for AAPL relative order behavior",
		runAPI:      runAPIOrderRelativeCancelAAPL,
	},
	"api_order_trailing_cancel_aapl": {
		name:        "api_order_trailing_cancel_aapl",
		description: "public API campaign for AAPL trailing and trailing-limit behavior",
		runAPI:      runAPIOrderTrailingCancelAAPL,
	},
	"api_order_stop_cancel_aapl": {
		name:        "api_order_stop_cancel_aapl",
		description: "public API campaign for AAPL stop and stop-limit rest/cancel behavior",
		runAPI:      runAPIOrderStopCancelAAPL,
	},
	"api_order_rejects_aapl": {
		name:        "api_order_rejects_aapl",
		description: "public API campaign for AAPL order rejection and unknown cancel behavior",
		runAPI:      runAPIOrderRejectsAAPL,
	},
	"api_delayed_success_modify_aapl": {
		name:        "api_delayed_success_modify_aapl",
		description: "public API campaign where a resting AAPL limit order succeeds later through OrderHandle.Modify",
		runAPI:      runAPIDelayedSuccessModifyAAPL,
	},
	"api_bracket_trigger_aapl": {
		name:        "api_bracket_trigger_aapl",
		description: "public API campaign for bracket parent/child activation and take-profit-trigger sibling cancellation",
		runAPI:      runAPIBracketTriggerAAPL,
	},
	"api_oca_trigger_aapl": {
		name:        "api_oca_trigger_aapl",
		description: "public API campaign for OCA fill/cancel behavior",
		runAPI:      runAPIOCATriggerAAPL,
	},
	"api_conditions_matrix_aapl": {
		name:        "api_conditions_matrix_aapl",
		description: "public API campaign for IBKR order condition families",
		runAPI:      runAPIConditionsMatrixAAPL,
	},
	"api_tif_attribute_matrix_aapl": {
		name:        "api_tif_attribute_matrix_aapl",
		description: "public API campaign for TIF values and advanced AAPL order attributes",
		runAPI:      runAPITIFAttributeMatrixAAPL,
	},
	"api_security_type_probe_matrix": {
		name:        "api_security_type_probe_matrix",
		description: "public API probe matrix for real Gateway contract-details behavior across security types",
		runAPI:      runAPISecurityTypeProbeMatrix,
	},
	"api_market_data_completeness_aapl": {
		name:        "api_market_data_completeness_aapl",
		description: "public API campaign for market-data type, generic tick, real-time bar, and tick-by-tick variants",
		runAPI:      runAPIMarketDataCompletenessAAPL,
	},
	"api_historical_matrix_aapl": {
		name:        "api_historical_matrix_aapl",
		description: "public API campaign for historical bar-size and whatToShow variants",
		runAPI:      runAPIHistoricalMatrixAAPL,
	},
	"api_news_article_aapl": {
		name:        "api_news_article_aapl",
		description: "public API campaign that requests a real news article ID from historical news, then fetches the article",
		runAPI:      runAPINewsArticleAAPL,
	},
	"api_fundamental_reports_aapl": {
		name:        "api_fundamental_reports_aapl",
		description: "public API probe for every fundamental report type on AAPL",
		runAPI:      runAPIFundamentalReportsAAPL,
	},
	"api_wsh_variants_aapl": {
		name:        "api_wsh_variants_aapl",
		description: "public API probe for WSH metadata and event-data filter variants",
		runAPI:      runAPIWSHVariantsAAPL,
	},
	"api_algo_variants_aapl": {
		name:        "api_algo_variants_aapl",
		description: "public API campaign for available IBKR algorithmic strategy variants",
		runAPI:      runAPIAlgoVariantsAAPL,
	},
	"api_pairs_trading_aapl_msft": {
		name:        "api_pairs_trading_aapl_msft",
		description: "public API campaign for paired AAPL/MSFT market orders and cleanup",
		runAPI:      runAPIPairsTradingAAPLMSFT,
	},
	"api_dollar_cost_averaging_aapl": {
		name:        "api_dollar_cost_averaging_aapl",
		description: "public API campaign for repeated AAPL buys and post-campaign flattening",
		runAPI:      runAPIDollarCostAveragingAAPL,
	},
	"api_stop_loss_management_aapl": {
		name:        "api_stop_loss_management_aapl",
		description: "public API campaign for placing, moving, cancelling, and flattening a protective stop",
		runAPI:      runAPIStopLossManagementAAPL,
	},
	"api_bracket_trailing_stop_aapl": {
		name:        "api_bracket_trailing_stop_aapl",
		description: "public API campaign for bracket order sequencing with a trailing stop child",
		runAPI:      runAPIBracketTrailingStopAAPL,
	},
	"api_option_campaign_aapl": {
		name:        "api_option_campaign_aapl",
		description: "public API campaign for live-qualified AAPL option data, order, execution, and exercise/lapse responses",
		runAPI:      runAPIOptionCampaignAAPL,
	},
	"api_future_campaign_mes": {
		name:        "api_future_campaign_mes",
		description: "public API campaign for live-qualified MES futures order behavior",
		runAPI:      runAPIFutureCampaignMES,
	},
	"api_combo_option_vertical_aapl": {
		name:        "api_combo_option_vertical_aapl",
		description: "public API campaign for live-qualified AAPL option vertical BAG order behavior",
		runAPI:      runAPIComboOptionVerticalAAPL,
	},
	"api_algorithmic_campaign_aapl": {
		name:        "api_algorithmic_campaign_aapl",
		description: "public API campaign with concurrent market/account/order observers and multi-step trading",
		runAPI:      runAPIAlgorithmicCampaignAAPL,
	},
	"api_completed_orders_variants_aapl": {
		name:        "api_completed_orders_variants_aapl",
		description: "public API campaign for completed-orders apiOnly true/false variants after a live paper fill",
		runAPI:      runAPICompletedOrdersVariantsAAPL,
	},
	"api_transmit_false_then_transmit_aapl": {
		name:        "api_transmit_false_then_transmit_aapl",
		description: "public API campaign for staging Transmit=false then modifying to transmit and cancel",
		runAPI:      runAPITransmitFalseThenTransmitAAPL,
	},
	"api_duplicate_quote_subscriptions_aapl": {
		name:        "api_duplicate_quote_subscriptions_aapl",
		description: "public API probe for two same-contract quote subscriptions on one client",
		runAPI:      runAPIDuplicateQuoteSubscriptionsAAPL,
	},
	"api_reconnect_active_order_aapl": {
		name:        "api_reconnect_active_order_aapl",
		description: "public API campaign for reconnecting with a live resting order and cancelling it after reconnect",
		runAPI:      runAPIReconnectActiveOrderAAPL,
	},
	"api_client_id0_order_observation_aapl": {
		name:        "api_client_id0_order_observation_aapl",
		description: "public API campaign for client ID 0 observing and cancelling another client's resting order",
		runAPI:      runAPIClientID0OrderObservationAAPL,
	},
	"api_cross_client_cancel_aapl": {
		name:        "api_cross_client_cancel_aapl",
		description: "public API campaign for placing from one client ID and cancelling from another client ID",
		runAPI:      runAPICrossClientCancelAAPL,
	},
	"api_forex_lifecycle_eurusd": {
		name:        "api_forex_lifecycle_eurusd",
		description: "public API campaign for EUR.USD forex rest/modify/cancel lifecycle",
		runAPI:      runAPIForexLifecycleEURUSD,
	},
	"api_whatif_margin_aapl": {
		name:        "api_whatif_margin_aapl",
		description: "public API campaign for WhatIf margin/commission preview on AAPL",
		runAPI:      runAPIWhatIfMarginAAPL,
	},
	"api_stress_rapid_fire_aapl": {
		name:        "api_stress_rapid_fire_aapl",
		description: "public API campaign for rapid-fire 10 orders plus global cancel",
		runAPI:      runAPIStressRapidFireAAPL,
	},
	"api_scale_in_campaign_aapl": {
		name:        "api_scale_in_campaign_aapl",
		description: "public API campaign for scale-in buy strategy with protective stop-loss and flatten",
		runAPI:      runAPIScaleInCampaignAAPL,
	},
	"api_ioc_fok_aapl": {
		name:        "api_ioc_fok_aapl",
		description: "public API campaign for IOC and FOK fill/reject paths",
		runAPI:      runAPIIOCFOKAAPL,
	},
}
