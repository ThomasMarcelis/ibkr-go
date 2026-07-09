package ibkr

import (
	"fmt"
	"strconv"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
)

func executionsRequest(req ExecutionsRequest, serverVersion int) (codec.ExecutionsRequest, error) {
	if req.Side != "" && req.Side != ExecutionFilterBuy && req.Side != ExecutionFilterSell {
		return codec.ExecutionsRequest{}, fmt.Errorf("ibkr: executions side %q: want BUY or SELL", req.Side)
	}
	if req.LastDays < 0 || req.LastDays > 7 {
		return codec.ExecutionsRequest{}, fmt.Errorf("ibkr: executions last days %d: want 0 through 7", req.LastDays)
	}
	if (req.LastDays != 0 || len(req.SpecificDates) != 0) && serverVersion < codec.MinServerVersionParametrizedDaysOfExecutions {
		return codec.ExecutionsRequest{}, fmt.Errorf(
			"ibkr: executions day filters require server_version %d, negotiated %d: %w",
			codec.MinServerVersionParametrizedDaysOfExecutions, serverVersion, ErrUnsupportedServerVersion,
		)
	}

	wireReq := codec.ExecutionsRequest{
		ClientID: req.ClientID,
		Account:  req.Account,
		Symbol:   req.Symbol,
		SecType:  string(req.SecType),
		Exchange: req.Exchange,
		Side:     string(req.Side),
	}
	if !req.Since.IsZero() {
		wireReq.Time = req.Since.UTC().Format("20060102-15:04:05")
	}
	if req.LastDays != 0 {
		wireReq.LastNDays = new(req.LastDays)
	}
	wireReq.SpecificDates = make([]int, len(req.SpecificDates))
	for i, date := range req.SpecificDates {
		if date.IsZero() {
			return codec.ExecutionsRequest{}, fmt.Errorf("ibkr: executions specific date %d is zero", i)
		}
		formatted := date.Format("20060102")
		if len(formatted) != 8 {
			return codec.ExecutionsRequest{}, fmt.Errorf("ibkr: executions specific date %d has out-of-range year %d", i, date.Year())
		}
		value, err := strconv.Atoi(formatted)
		if err != nil {
			return codec.ExecutionsRequest{}, fmt.Errorf("ibkr: executions specific date %d: %w", i, err)
		}
		wireReq.SpecificDates[i] = value
	}
	return wireReq, nil
}
