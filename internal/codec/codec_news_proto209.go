package codec

import "google.golang.org/protobuf/encoding/protowire"

func (m NewsBulletinsRequest) encodeProto(sv int) ([]byte, error) { //nolint:unparam // protobufEncoder requires an error result.
	if m.AllMessages {
		return appendProtoVarint(nil, 1, 1), nil
	}
	return []byte{}, nil
}

func (CancelNewsBulletins) encodeProto(sv int) ([]byte, error) { return []byte{}, nil }

func (m NewsArticleRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "news article request id")
	if err != nil {
		return nil, err
	}
	if m.ProviderCode != "" {
		body = appendProtoString(body, 2, m.ProviderCode)
	}
	if m.ArticleID != "" {
		body = appendProtoString(body, 3, m.ArticleID)
	}
	return body, nil
}

func (NewsProvidersRequest) encodeProto(sv int) ([]byte, error) { return []byte{}, nil }

func (m HistoricalNewsRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "historical news request id")
	if err != nil {
		return nil, err
	}
	body, err = appendProtoInt(body, 2, m.ConID, "historical news conid")
	if err != nil {
		return nil, err
	}
	if m.ProviderCodes != "" {
		body = appendProtoString(body, 3, m.ProviderCodes)
	}
	if m.StartDate != "" {
		body = appendProtoString(body, 4, m.StartDate)
	}
	if m.EndDate != "" {
		body = appendProtoString(body, 5, m.EndDate)
	}
	body, err = appendProtoInt(body, 6, m.TotalResults, "historical news result count")
	if err != nil {
		return nil, err
	}
	return canonicalProtoFields(body), nil
}

func (m WSHMetaDataRequest) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "WSH metadata request id")
}

func (m CancelWSHMetaData) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel WSH metadata request id")
}

func (m WSHEventDataRequest) encodeProto(sv int) ([]byte, error) {
	body, err := appendProtoInt(nil, 1, m.ReqID, "WSH event data request id")
	if err != nil {
		return nil, err
	}
	body, err = appendProtoInt(body, 2, m.ConID, "WSH event data conid")
	if err != nil {
		return nil, err
	}
	if m.Filter != "" {
		body = appendProtoString(body, 3, m.Filter)
	}
	for _, field := range []struct {
		number protowire.Number
		value  bool
	}{{4, m.FillWatchlist}, {5, m.FillPortfolio}, {6, m.FillCompetitors}} {
		if field.value {
			body = appendProtoVarint(body, field.number, 1)
		}
	}
	if m.StartDate != "" {
		body = appendProtoString(body, 7, m.StartDate)
	}
	if m.EndDate != "" {
		body = appendProtoString(body, 8, m.EndDate)
	}
	if m.TotalLimit != 0 {
		body, err = appendProtoInt(body, 9, m.TotalLimit, "WSH event data limit")
		if err != nil {
			return nil, err
		}
	}
	return canonicalProtoFields(body), nil
}

func (m CancelWSHEventData) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "cancel WSH event data request id")
}

func decodeNewsBulletinProto(body []byte, sv int) ([]Message, error) {
	m := NewsBulletin{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("news bulletin", number, err)
			}
			if number == 1 {
				m.MsgID = decodeProtoInt32(value)
			} else {
				m.MsgType = decodeProtoInt32(value)
			}
		case 3, 4:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("news bulletin", number, err)
			}
			if number == 3 {
				m.Headline = string(value)
			} else {
				m.Source = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("news bulletin", number, err)
			}
		}
	}
}

func decodeNewsArticleProto(body []byte, sv int) ([]Message, error) {
	m := NewsArticleResponse{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("news article", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.ArticleType = decodeProtoInt32(value)
			}
		case 3:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("news article", number, err)
			}
			m.ArticleText = string(value)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("news article", number, err)
			}
		}
	}
}

func decodeNewsProvidersProto(body []byte, sv int) ([]Message, error) {
	m := NewsProviders{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number != 1 {
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("news providers", number, err)
			}
			continue
		}
		value, err := consumeProtoBytes(&body, typ)
		if err != nil {
			return nil, protoFieldError("news providers", number, err)
		}
		provider, err := decodeNewsProviderProto(value)
		if err != nil {
			return nil, err
		}
		m.Providers = append(m.Providers, provider)
	}
}

func decodeNewsProviderProto(body []byte) (NewsProviderEntry, error) {
	m := NewsProviderEntry{}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return NewsProviderEntry{}, err
		}
		if !ok {
			return m, nil
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return NewsProviderEntry{}, protoFieldError("news provider", number, err)
			}
			if number == 1 {
				m.Code = string(value)
			} else {
				m.Name = string(value)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return NewsProviderEntry{}, protoFieldError("news provider", number, err)
		}
	}
}

func decodeHistoricalNewsProto(body []byte, sv int) ([]Message, error) {
	m := HistoricalNewsItem{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number == 1 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical news", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		} else if number >= 2 && number <= 5 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical news", number, err)
			}
			switch number {
			case 2:
				m.Time = string(value)
			case 3:
				m.ProviderCode = string(value)
			case 4:
				m.ArticleID = string(value)
			case 5:
				m.Headline = string(value)
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("historical news", number, err)
		}
	}
}

func decodeHistoricalNewsEndProto(body []byte, sv int) ([]Message, error) {
	m := HistoricalNewsEnd{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("historical news end", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.HasMore = value != 0
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return nil, protoFieldError("historical news end", number, err)
		}
	}
}

func decodeWSHMetaDataProto(body []byte, sv int) ([]Message, error) {
	m, err := decodeWSHDocumentProto(body, "WSH metadata")
	if err != nil {
		return nil, err
	}
	return []Message{WSHMetaDataResponse{ReqID: m.reqID, DataJSON: m.data}}, nil
}

func decodeWSHEventDataProto(body []byte, sv int) ([]Message, error) {
	m, err := decodeWSHDocumentProto(body, "WSH event data")
	if err != nil {
		return nil, err
	}
	return []Message{WSHEventDataResponse{ReqID: m.reqID, DataJSON: m.data}}, nil
}

type wshDocumentProto struct {
	reqID int
	data  string
}

func decodeWSHDocumentProto(body []byte, label string) (wshDocumentProto, error) {
	m := wshDocumentProto{reqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return wshDocumentProto{}, err
		}
		if !ok {
			return m, nil
		}
		if number == 1 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return wshDocumentProto{}, protoFieldError(label, number, err)
			}
			m.reqID = decodeProtoInt32(value)
		} else if number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return wshDocumentProto{}, protoFieldError(label, number, err)
			}
			m.data = string(value)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return wshDocumentProto{}, protoFieldError(label, number, err)
		}
	}
}

func decodeTickNewsProto(body []byte, sv int) ([]Message, error) {
	m := TickNews{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1, 2:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick news", number, err)
			}
			if number == 1 {
				m.ReqID = decodeProtoInt32(value)
			} else {
				m.Time = i64toa(decodeProtoInt64(value))
			}
		case 3, 4, 5, 6:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("tick news", number, err)
			}
			switch number {
			case 3:
				m.ProviderCode = string(value)
			case 4:
				m.ArticleID = string(value)
			case 5:
				m.Headline = string(value)
			case 6:
				m.ExtraData = string(value)
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("tick news", number, err)
			}
		}
	}
}
