package ibkr

import (
	"context"
	"fmt"
	"time"
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
	MsgID    int
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

// NewsArticle is the body of a news article. ArticleType distinguishes plain
// text from binary/HTML payloads.
type NewsArticle struct {
	ArticleType int // 0 = plain text/HTML, 1 = binary (e.g. PDF, base64-encoded)
	ArticleText string
}

// HistoricalNewsRequest queries historical news headlines for a contract via
// [NewsClient.Historical].
type HistoricalNewsRequest struct {
	ConID         int
	ProviderCodes []NewsProviderCode
	StartTime     time.Time
	EndTime       time.Time
	TotalResults  int // maximum headlines to return
}

// HistoricalNewsItem is one historical news headline.
type HistoricalNewsItem struct {
	Time         time.Time
	ProviderCode NewsProviderCode
	ArticleID    string
	Headline     string
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
	NumberOfRows int // maximum ranked rows to return
	Instrument   ScannerInstrument
	LocationCode ScannerLocationCode
	ScanCode     ScannerCode
}

// ScannerResult is one ranked contract from a scanner subscription.
type ScannerResult struct {
	Rank       int
	Contract   Contract
	Distance   string
	Benchmark  string
	Projection string
	LegsStr    string
}

// FundamentalReportType selects which fundamental data report
// [ContractsClient.FundamentalData] returns.
type FundamentalReportType string

const (
	FundamentalReportSnapshot       FundamentalReportType = "ReportSnapshot"       // company overview snapshot
	FundamentalReportsFinSummary    FundamentalReportType = "ReportsFinSummary"    // financial summary
	FundamentalReportsOwnership     FundamentalReportType = "ReportsOwnership"     // ownership report
	FundamentalReportRatios         FundamentalReportType = "ReportRatios"         // financial ratios
	FundamentalReportsFinStatements FundamentalReportType = "ReportsFinStatements" // financial statements
	FundamentalRESC                 FundamentalReportType = "RESC"                 // analyst estimates
)

// FundamentalDataRequest asks for a fundamental data report on a contract.
type FundamentalDataRequest struct {
	Contract   Contract
	ReportType FundamentalReportType
}

// FADataType selects which Financial Advisor configuration document
// [AdvisorsClient.Config] and [AdvisorsClient.ReplaceConfig] operate on.
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
	ConID           int
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
type DisplayGroupID int

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
	return h.updateFn(ctx, contractInfo)
}
