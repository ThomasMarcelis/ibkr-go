package codec

type NewsProvidersRequest struct{}

func (NewsProvidersRequest) messageName() string { return "req_news_providers" }

type NewsProviders struct {
	Providers []NewsProviderEntry
}

func (NewsProviders) messageName() string { return "news_providers" }

type NewsProviderEntry struct {
	Code string
	Name string
}

// News bulletins (OUT 12, cancel OUT 13 / IN 14)

type NewsBulletinsRequest struct {
	AllMessages bool
}

func (NewsBulletinsRequest) messageName() string { return "req_news_bulletins" }

type CancelNewsBulletins struct{}

func (CancelNewsBulletins) messageName() string { return "cancel_news_bulletins" }

type NewsBulletin struct {
	MsgID    int
	MsgType  int
	Headline string
	Source   string
}

func (NewsBulletin) messageName() string { return "news_bulletin" }

// NewsArticle (OUT 84 / IN 83)

type NewsArticleRequest struct {
	ReqID        int
	ProviderCode string
	ArticleID    string
}

func (NewsArticleRequest) messageName() string { return "req_news_article" }

type NewsArticleResponse struct {
	ReqID       int
	ArticleType int
	ArticleText string
}

func (NewsArticleResponse) messageName() string { return "news_article" }

// HistoricalNews (OUT 86 / IN 87+80)

type HistoricalNewsRequest struct {
	ReqID         int
	ConID         int
	ProviderCodes string
	StartDate     string
	EndDate       string
	TotalResults  int
}

func (HistoricalNewsRequest) messageName() string { return "req_historical_news" }

type HistoricalNewsItem struct {
	ReqID        int
	Time         string
	ProviderCode string
	ArticleID    string
	Headline     string
}

func (HistoricalNewsItem) messageName() string { return "historical_news" }

type HistoricalNewsEnd struct {
	ReqID   int
	HasMore bool
}

func (HistoricalNewsEnd) messageName() string { return "historical_news_end" }
