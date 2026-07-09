package ibkr

import (
	"strconv"

	"github.com/ThomasMarcelis/ibkr-go/internal/codec"
	"github.com/shopspring/decimal"
)

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
