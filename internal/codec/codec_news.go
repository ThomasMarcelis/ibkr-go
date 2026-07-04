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

// [83, reqID, articleType, articleText] — no version
func decodeNewsArticle(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	articleType, _ := r.ReadInt()
	articleText := r.ReadString()
	return []Message{NewsArticleResponse{ReqID: reqID, ArticleType: articleType, ArticleText: articleText}}, nil
}

// [85, count, repeated(code, name)] — no version
func decodeNewsProviders(r *fieldReader) ([]Message, error) {
	count, err := r.ReadCount("news provider count")
	if err != nil {
		return nil, err
	}
	if err := r.RequireFixedEntryFields("news providers", count, 2, 0); err != nil {
		return nil, err
	}
	entries := make([]NewsProviderEntry, count)
	for i := range entries {
		entries[i] = NewsProviderEntry{Code: r.ReadString(), Name: r.ReadString()}
	}
	return []Message{NewsProviders{Providers: entries}}, nil
}

// [86, reqID, time, providerCode, articleId, headline] — no version
func decodeHistoricalNews(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	timeStr := r.ReadString()
	providerCode := r.ReadString()
	articleID := r.ReadString()
	headline := r.ReadString()
	return []Message{HistoricalNewsItem{ReqID: reqID, Time: timeStr, ProviderCode: providerCode, ArticleID: articleID, Headline: headline}}, nil
}

// [87, reqID, hasMore]
func decodeHistoricalNewsEnd(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	hasMore, _ := r.ReadBool()
	return []Message{HistoricalNewsEnd{ReqID: reqID, HasMore: hasMore}}, nil
}

// [14, version=1, msgId, msgType, headline, source]
func decodeNewsBulletins(r *fieldReader) ([]Message, error) {
	r.Skip(1) // version
	msgId, _ := r.ReadInt()
	msgType, _ := r.ReadInt()
	headline := r.ReadString()
	source := r.ReadString()
	return []Message{NewsBulletin{MsgID: msgId, MsgType: msgType, Headline: headline, Source: source}}, nil
}
