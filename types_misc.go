package ibkr

import (
	"context"
	"fmt"
	"time"
)

type NewsProviderCode string

type NewsProvider struct {
	Code NewsProviderCode
	Name string
}

type NewsBulletin struct {
	MsgID    int
	MsgType  int
	Headline string
	Source   string
}

type NewsArticleRequest struct {
	ProviderCode NewsProviderCode
	ArticleID    string
}

type NewsArticle struct {
	ArticleType int
	ArticleText string
}

type HistoricalNewsRequest struct {
	ConID         int
	ProviderCodes []NewsProviderCode
	StartTime     time.Time
	EndTime       time.Time
	TotalResults  int
}

type HistoricalNewsItem struct {
	Time         time.Time
	ProviderCode NewsProviderCode
	ArticleID    string
	Headline     string
}

type ScannerInstrument string
type ScannerLocationCode string
type ScannerCode string

type ScannerSubscriptionRequest struct {
	NumberOfRows int
	Instrument   ScannerInstrument
	LocationCode ScannerLocationCode
	ScanCode     ScannerCode
}

type ScannerResult struct {
	Rank       int
	Contract   Contract
	Distance   string
	Benchmark  string
	Projection string
	LegsStr    string
}

type FundamentalReportType string

const (
	FundamentalReportSnapshot       FundamentalReportType = "ReportSnapshot"
	FundamentalReportsFinSummary    FundamentalReportType = "ReportsFinSummary"
	FundamentalReportsOwnership     FundamentalReportType = "ReportsOwnership"
	FundamentalReportRatios         FundamentalReportType = "ReportRatios"
	FundamentalReportsFinStatements FundamentalReportType = "ReportsFinStatements"
	FundamentalRESC                 FundamentalReportType = "RESC"
)

type FundamentalDataRequest struct {
	Contract   Contract
	ReportType FundamentalReportType
}

type FADataType int

const (
	FADataGroups   FADataType = 1
	FADataProfiles FADataType = 2
	FADataAliases  FADataType = 3
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

type WSHEventDataRequest struct {
	ConID           int
	Filter          JSONDocument
	FillWatchlist   bool
	FillPortfolio   bool
	FillCompetitors bool
	StartDate       time.Time
	EndDate         time.Time
	TotalLimit      int
}

type DisplayGroupID int

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
