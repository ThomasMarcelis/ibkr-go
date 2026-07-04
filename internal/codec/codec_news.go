package codec

type NewsProvidersRequest struct{}

func (m NewsProvidersRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqNewsProviders)}, nil
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

func (m NewsBulletinsRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqNewsBulletins), "1", btoa(m.AllMessages)}, nil
}

type CancelNewsBulletins struct{}

func (m CancelNewsBulletins) encodeWire() ([]string, error) {
	return []string{itoa(OutCancelNewsBulletins), "1"}, nil
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

func (m NewsArticleRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqNewsArticle), itoa(m.ReqID), m.ProviderCode, m.ArticleID, ""}, nil
}

type NewsArticleResponse struct {
	ReqID       int
	ArticleType int
	ArticleText string
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

func (m HistoricalNewsRequest) encodeWire() ([]string, error) {
	return []string{itoa(OutReqHistoricalNews), itoa(m.ReqID), itoa(m.ConID), m.ProviderCodes, m.StartDate, m.EndDate, itoa(m.TotalResults), ""}, nil
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
func decodeNewsArticle(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	articleType, _ := r.ReadInt()
	articleText := r.ReadString()
	return []Message{NewsArticleResponse{ReqID: reqID, ArticleType: articleType, ArticleText: articleText}}, nil
}

func (m NewsArticleResponse) encodeWire() ([]string, error) {
	return []string{itoa(InNewsArticle), itoa(m.ReqID), itoa(m.ArticleType), m.ArticleText}, nil
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

func (m NewsProviders) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InNewsProviders)
	w.WriteInt(len(m.Providers))
	for _, p := range m.Providers {
		w.WriteString(p.Code)
		w.WriteString(p.Name)
	}
	return w.Fields(), nil
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

func (m HistoricalNewsItem) encodeWire() ([]string, error) {
	return []string{itoa(InHistoricalNews), itoa(m.ReqID), m.Time, m.ProviderCode, m.ArticleID, m.Headline}, nil
}

// [87, reqID, hasMore]
func decodeHistoricalNewsEnd(r *fieldReader) ([]Message, error) {
	reqID, _ := r.ReadInt()
	hasMore, _ := r.ReadBool()
	return []Message{HistoricalNewsEnd{ReqID: reqID, HasMore: hasMore}}, nil
}

func (m HistoricalNewsEnd) encodeWire() ([]string, error) {
	w := fieldWriter{}
	w.WriteInt(InHistoricalNewsEnd)
	w.WriteInt(m.ReqID)
	w.WriteBool(m.HasMore)
	return w.Fields(), nil
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

func (m NewsBulletin) encodeWire() ([]string, error) {
	return []string{itoa(InNewsBulletins), "1", itoa(m.MsgID), itoa(m.MsgType), m.Headline, m.Source}, nil
}
