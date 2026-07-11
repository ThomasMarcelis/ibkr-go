package ibkr

import (
	"strconv"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
	"github.com/shopspring/decimal"
)

func validateScannerSubscriptionRequest(req ScannerSubscriptionRequest) error {
	if strings.TrimSpace(string(req.Instrument)) == "" {
		return &ValidationError{Field: "ScannerSubscriptionRequest.Instrument", Message: "is required"}
	}
	if strings.TrimSpace(string(req.LocationCode)) == "" {
		return &ValidationError{Field: "ScannerSubscriptionRequest.LocationCode", Message: "is required"}
	}
	if strings.TrimSpace(string(req.ScanCode)) == "" {
		return &ValidationError{Field: "ScannerSubscriptionRequest.ScanCode", Message: "is required"}
	}
	if req.NumberOfRows < 0 {
		return &ValidationError{Field: "ScannerSubscriptionRequest.NumberOfRows", Message: "must be >= 0"}
	}
	if req.AboveVolume != nil && *req.AboveVolume < 0 {
		return &ValidationError{Field: "ScannerSubscriptionRequest.AboveVolume", Message: "must be >= 0"}
	}
	if req.AverageOptionVolumeAbove != nil && *req.AverageOptionVolumeAbove < 0 {
		return &ValidationError{Field: "ScannerSubscriptionRequest.AverageOptionVolumeAbove", Message: "must be >= 0"}
	}
	if req.AbovePrice != nil && req.BelowPrice != nil && req.AbovePrice.GreaterThan(*req.BelowPrice) {
		return &ValidationError{Field: "ScannerSubscriptionRequest.AbovePrice", Message: "must not exceed BelowPrice"}
	}
	if req.MarketCapAbove != nil && req.MarketCapBelow != nil && req.MarketCapAbove.GreaterThan(*req.MarketCapBelow) {
		return &ValidationError{Field: "ScannerSubscriptionRequest.MarketCapAbove", Message: "must not exceed MarketCapBelow"}
	}
	if req.CouponRateAbove != nil && req.CouponRateBelow != nil && req.CouponRateAbove.GreaterThan(*req.CouponRateBelow) {
		return &ValidationError{Field: "ScannerSubscriptionRequest.CouponRateAbove", Message: "must not exceed CouponRateBelow"}
	}
	if err := validateTagValues("ScannerSubscriptionRequest.FilterOptions", req.FilterOptions); err != nil {
		return err
	}
	return validateTagValues("ScannerSubscriptionRequest.SubscriptionOptions", req.SubscriptionOptions)
}

func cloneScannerSubscriptionRequest(req ScannerSubscriptionRequest) ScannerSubscriptionRequest {
	if req.AbovePrice != nil {
		req.AbovePrice = new(*req.AbovePrice)
	}
	if req.BelowPrice != nil {
		req.BelowPrice = new(*req.BelowPrice)
	}
	if req.AboveVolume != nil {
		req.AboveVolume = new(*req.AboveVolume)
	}
	if req.MarketCapAbove != nil {
		req.MarketCapAbove = new(*req.MarketCapAbove)
	}
	if req.MarketCapBelow != nil {
		req.MarketCapBelow = new(*req.MarketCapBelow)
	}
	if req.CouponRateAbove != nil {
		req.CouponRateAbove = new(*req.CouponRateAbove)
	}
	if req.CouponRateBelow != nil {
		req.CouponRateBelow = new(*req.CouponRateBelow)
	}
	if req.ExcludeConvertible != nil {
		req.ExcludeConvertible = new(*req.ExcludeConvertible)
	}
	if req.AverageOptionVolumeAbove != nil {
		req.AverageOptionVolumeAbove = new(*req.AverageOptionVolumeAbove)
	}
	req.FilterOptions = append([]TagValue(nil), req.FilterOptions...)
	req.SubscriptionOptions = append([]TagValue(nil), req.SubscriptionOptions...)
	return req
}

func toCodecScannerSubscriptionRequest(reqID int, req ScannerSubscriptionRequest) codec.ScannerSubscriptionRequest {
	numberOfRows := req.NumberOfRows
	if numberOfRows == 0 {
		numberOfRows = -1
	}
	return codec.ScannerSubscriptionRequest{
		ReqID:                    reqID,
		NumberOfRows:             numberOfRows,
		Instrument:               string(req.Instrument),
		LocationCode:             string(req.LocationCode),
		ScanCode:                 string(req.ScanCode),
		AbovePrice:               formatScannerDecimal(req.AbovePrice),
		BelowPrice:               formatScannerDecimal(req.BelowPrice),
		AboveVolume:              formatScannerInt(req.AboveVolume),
		MarketCapAbove:           formatScannerDecimal(req.MarketCapAbove),
		MarketCapBelow:           formatScannerDecimal(req.MarketCapBelow),
		MoodyRatingAbove:         req.MoodyRatingAbove,
		MoodyRatingBelow:         req.MoodyRatingBelow,
		SPRatingAbove:            req.SPRatingAbove,
		SPRatingBelow:            req.SPRatingBelow,
		MaturityDateAbove:        req.MaturityDateAbove,
		MaturityDateBelow:        req.MaturityDateBelow,
		CouponRateAbove:          formatScannerDecimal(req.CouponRateAbove),
		CouponRateBelow:          formatScannerDecimal(req.CouponRateBelow),
		ExcludeConvertible:       optBoolToString(req.ExcludeConvertible, ""),
		AverageOptionVolumeAbove: formatScannerInt(req.AverageOptionVolumeAbove),
		ScannerSettingPairs:      req.ScannerSettingPairs,
		StockTypeFilter:          req.StockTypeFilter,
		FilterOptions:            tagValuesToCodec(req.FilterOptions),
		SubscriptionOptions:      tagValuesToCodec(req.SubscriptionOptions),
	}
}

func formatScannerDecimal(value *decimal.Decimal) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func formatScannerInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}
