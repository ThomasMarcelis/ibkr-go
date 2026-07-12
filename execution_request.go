package ibkr

import (
	"fmt"
	"strconv"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

func executionsRequest(req ExecutionsRequest, serverVersion int) (codec.ExecutionsRequest, error) {
	if req.Side != "" && req.Side != ExecutionFilterBuy && req.Side != ExecutionFilterSell {
		return codec.ExecutionsRequest{}, &ValidationError{
			Field: "ExecutionsRequest.Side", Value: string(req.Side), Message: "must be BUY or SELL",
		}
	}
	if req.LastDays < 0 || req.LastDays > 7 {
		return codec.ExecutionsRequest{}, &ValidationError{
			Field: "ExecutionsRequest.LastDays", Value: strconv.Itoa(req.LastDays), Message: "must be between 0 and 7",
		}
	}
	if (req.LastDays != 0 || len(req.SpecificDates) != 0) && serverVersion < protocol.MinServerVersionParametrizedDaysOfExecutions {
		return codec.ExecutionsRequest{}, fmt.Errorf(
			"ibkr: executions day filters require server_version %d, negotiated %d: %w",
			protocol.MinServerVersionParametrizedDaysOfExecutions, serverVersion, ErrUnsupportedServerVersion,
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
		field := fmt.Sprintf("ExecutionsRequest.SpecificDates[%d]", i)
		if date.IsZero() {
			return codec.ExecutionsRequest{}, &ValidationError{Field: field, Message: "must not be zero"}
		}
		formatted := date.Format("20060102")
		if len(formatted) != 8 {
			return codec.ExecutionsRequest{}, &ValidationError{
				Field: field, Value: strconv.Itoa(date.Year()), Message: "year must fit YYYYMMDD format",
			}
		}
		value, err := strconv.Atoi(formatted)
		if err != nil {
			return codec.ExecutionsRequest{}, &ValidationError{Field: field, Value: formatted, Message: "must be a valid YYYYMMDD date"}
		}
		wireReq.SpecificDates[i] = value
	}
	return wireReq, nil
}
