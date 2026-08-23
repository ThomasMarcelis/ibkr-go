package codec

import "github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"

type NewsProvidersRequest struct{}

func (m NewsProvidersRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqNewsProviders)}, nil
}

type NewsProviders struct {
	Providers []NewsProviderEntry
}

type NewsProviderEntry struct {
	Code string
	Name string
}

// News bulletins (OUT 12, cancel OUT 13 / IN 14)

type NewsBulletinsRequest struct {
	AllMessages bool
}

func (m NewsBulletinsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqNewsBulletins), "1", btoa(m.AllMessages)}, nil
}

type CancelNewsBulletins struct{}

func (m CancelNewsBulletins) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutCancelNewsBulletins), "1"}, nil
}

type NewsBulletin struct {
	MsgID    int
	MsgType  int
	Headline string
	Source   string
}

// NewsArticle (OUT 84 / IN 83)

type NewsArticleRequest struct {
	ReqID        int
	ProviderCode string
	ArticleID    string
}

func (m NewsArticleRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqNewsArticle), itoa(m.ReqID), m.ProviderCode, m.ArticleID, ""}, nil
}

type NewsArticleResponse struct {
	ReqID       int
	ArticleType int
	ArticleText string
}

// TickNews is one contract-specific news headline delivered through a market
// data subscription. Time preserves IBKR's epoch-millisecond wire value.
type TickNews struct {
	ReqID        int
	Time         string
	ProviderCode string
	ArticleID    string
	Headline     string
	ExtraData    string
}

// HistoricalNews (OUT 86 / IN 87+80)

type HistoricalNewsRequest struct {
	ReqID         int
	ConID         int
	ProviderCodes string
	StartDate     string
	EndDate       string
	TotalResults  int
}

func (m HistoricalNewsRequest) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.OutReqHistoricalNews), itoa(m.ReqID), itoa(m.ConID), m.ProviderCodes, m.StartDate, m.EndDate, itoa(m.TotalResults), ""}, nil
}

type HistoricalNewsItem struct {
	ReqID        int
	Time         string
	ProviderCode string
	ArticleID    string
	Headline     string
}

type HistoricalNewsEnd struct {
	ReqID   int
	HasMore bool
}

// [83, reqID, articleType, articleText] — no version
func decodeNewsArticle(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	articleType, _ := r.ReadInt()
	articleText := r.ReadString()
	return []Message{NewsArticleResponse{ReqID: reqID, ArticleType: articleType, ArticleText: articleText}}, nil
}

// [84, reqID, time, providerCode, articleId, headline, extraData] — no version
func decodeTickNews(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	return []Message{TickNews{
		ReqID:        reqID,
		Time:         r.ReadString(),
		ProviderCode: r.ReadString(),
		ArticleID:    r.ReadString(),
		Headline:     r.ReadString(),
		ExtraData:    r.ReadString(),
	}}, nil
}

// [85, count, repeated(code, name)] — no version
func decodeNewsProviders(r *fieldReader, sv int) ([]Message, error) {
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
func decodeHistoricalNews(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	timeStr := r.ReadString()
	providerCode := r.ReadString()
	articleID := r.ReadString()
	headline := r.ReadString()
	return []Message{HistoricalNewsItem{ReqID: reqID, Time: timeStr, ProviderCode: providerCode, ArticleID: articleID, Headline: headline}}, nil
}

// [87, reqID, hasMore]
func decodeHistoricalNewsEnd(r *fieldReader, sv int) ([]Message, error) {
	reqID, _ := r.ReadInt()
	hasMore, _ := r.ReadBool()
	return []Message{HistoricalNewsEnd{ReqID: reqID, HasMore: hasMore}}, nil
}

// [14, version=1, msgId, msgType, headline, source]
func decodeNewsBulletins(r *fieldReader, sv int) ([]Message, error) {
	r.Skip(1) // version
	msgId, _ := r.ReadInt()
	msgType, _ := r.ReadInt()
	headline := r.ReadString()
	source := r.ReadString()
	return []Message{NewsBulletin{MsgID: msgId, MsgType: msgType, Headline: headline, Source: source}}, nil
}
