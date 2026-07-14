package ibkr

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// NewsProviderCode is a news provider identifier (for example "BRFG"), as
// listed by [NewsClient.Providers].
type NewsProviderCode string

// NewsProvider is a subscribed news provider and its display name.
type NewsProvider struct {
	Code NewsProviderCode
	Name string
}

// NewsBulletin is a system or exchange news bulletin from
// [NewsClient.SubscribeBulletins].
type NewsBulletin struct {
	MsgID    int32
	MsgType  int // bulletin type: 1 regular, 2 exchange unavailable, 3 exchange available
	Headline string
	Source   string // originating exchange or source
}

// NewsArticleRequest identifies a news article to fetch via
// [NewsClient.Article].
type NewsArticleRequest struct {
	ProviderCode NewsProviderCode
	ArticleID    string
}

// NewsArticleType identifies the representation of a news article body.
// Unknown values are preserved so the type remains open to protocol additions.
type NewsArticleType int32

const (
	NewsArticleTypeText   NewsArticleType = 0
	NewsArticleTypeBinary NewsArticleType = 1
)

// NewsArticle is the body of a news article. Binary bodies remain the
// base64-encoded text supplied by IBKR.
type NewsArticle struct {
	ArticleType NewsArticleType
	ArticleText string
}

// HistoricalNewsRequest queries historical news headlines for a contract via
// [NewsClient.Historical].
type HistoricalNewsRequest struct {
	ConID         ContractID
	ProviderCodes []NewsProviderCode
	StartTime     time.Time // exclusive upper bound in the descending result stream; cannot be combined with EndTime
	EndTime       time.Time // inclusive lower bound in the descending result stream; cannot be combined with StartTime
	TotalResults  int       // maximum headlines to return
}

// HistoricalNewsItem is one historical news headline.
type HistoricalNewsItem struct {
	Time         time.Time
	ProviderCode NewsProviderCode
	ArticleID    string
	Headline     string
}

// HistoricalNewsResult is one page of historical headlines. HasMore is the
// Gateway's pagination signal and must be used to decide whether to request a
// subsequent page.
type HistoricalNewsResult struct {
	Items   []HistoricalNewsItem
	HasMore bool
}

// ScannerInstrument is a scanner instrument type (for example "STK").
type ScannerInstrument string

// ScannerLocationCode is a scanner location filter (for example "STK.US.MAJOR").
type ScannerLocationCode string

// ScannerCode is a scan type (for example "TOP_PERC_GAIN").
type ScannerCode string

// ScannerSubscriptionRequest configures a market scanner subscription for
// [ScannerClient.SubscribeResults]. Valid Instrument, LocationCode, and
// ScanCode values come from [ScannerClient.Parameters].
type ScannerSubscriptionRequest struct {
	NumberOfRows int // maximum ranked rows to return; zero lets IBKR choose the default
	Instrument   ScannerInstrument
	LocationCode ScannerLocationCode
	ScanCode     ScannerCode

	AbovePrice               *decimal.Decimal // minimum instrument price; nil leaves the filter unset
	BelowPrice               *decimal.Decimal // maximum instrument price; nil leaves the filter unset
	AboveVolume              *int             // minimum volume; nil leaves the filter unset
	MarketCapAbove           *decimal.Decimal // minimum market capitalization; nil leaves the filter unset
	MarketCapBelow           *decimal.Decimal // maximum market capitalization; nil leaves the filter unset
	MoodyRatingAbove         string
	MoodyRatingBelow         string
	SPRatingAbove            string
	SPRatingBelow            string
	MaturityDateAbove        string // minimum maturity date in IBKR scanner format
	MaturityDateBelow        string // maximum maturity date in IBKR scanner format
	CouponRateAbove          *decimal.Decimal
	CouponRateBelow          *decimal.Decimal
	ExcludeConvertible       *bool
	AverageOptionVolumeAbove *int
	ScannerSettingPairs      string
	StockTypeFilter          string

	FilterOptions       []TagValue // generic scanner filters advertised by Parameters
	SubscriptionOptions []TagValue // IBKR scanner subscription options
}

// ScannerResult is one ranked contract from a scanner subscription.
type ScannerResult struct {
	Rank       int
	Contract   Contract
	MarketName string
	Distance   string
	Benchmark  string
	Projection string
	LegsStr    string
}

// FADataType selects which Financial Advisor configuration document
// [AdvisorsClient.Config] reads.
type FADataType int

const (
	FADataGroups   FADataType = 1 // account groups
	FADataProfiles FADataType = 2 // allocation profiles (deprecated by IBKR in favor of groups)
	FADataAliases  FADataType = 3 // account aliases
)

func (t FADataType) String() string {
	switch t {
	case FADataGroups:
		return "Groups"
	case FADataProfiles:
		return "Profiles"
	case FADataAliases:
		return "Aliases"
	default:
		return fmt.Sprintf("FADataType(%d)", t)
	}
}

// WSHEventDataRequest queries Wall Street Horizon calendar events via
// [WSHClient.EventData]. Filter is a raw JSON filter document; the Fill flags
// scope results to the user's watchlist, portfolio, or competitors.
type WSHEventDataRequest struct {
	ConID           ContractID
	Filter          JSONDocument // raw JSON event filter; empty for no filter
	FillWatchlist   bool
	FillPortfolio   bool
	FillCompetitors bool
	StartDate       time.Time
	EndDate         time.Time
	TotalLimit      int // maximum events to return
}

// DisplayGroupID identifies a TWS display group, as listed by
// [TWSClient.DisplayGroups].
type DisplayGroupID int32

// DisplayGroupUpdate reports the contract currently selected in a subscribed
// display group. ContractInfo is "none" when the group is empty, otherwise an
// encoded "conID@exchange" token.
type DisplayGroupUpdate struct {
	ContractInfo string
}

// DisplayGroupHandle wraps a display group subscription and exposes an
// Update method that targets the same protocol-level request ID.
type DisplayGroupHandle struct {
	*Subscription[DisplayGroupUpdate]
	updateFn func(context.Context, string) error
}

// Update sends an UpdateDisplayGroup request for this subscription's group.
func (h *DisplayGroupHandle) Update(ctx context.Context, contractInfo string) error {
	if h == nil || h.Subscription == nil || h.updateFn == nil {
		return ErrClosed
	}
	select {
	case <-h.Done():
		return ErrClosed
	default:
	}
	return h.updateFn(ctx, contractInfo)
}
