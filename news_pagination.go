package ibkr

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"
)

// ErrHistoricalNewsPaginationStalled reports that the Gateway says more news
// exists but its second-resolution cursor cannot advance without risking loss.
var ErrHistoricalNewsPaginationStalled = errors.New("ibkr: historical news pagination stalled")

type historicalNewsIdentity struct {
	provider  NewsProviderCode
	articleID string
}

// HistoricalAll lazily traverses historical-news pages without skipping the
// exclusive cursor's boundary second. TotalResults remains the per-page limit;
// use 300 when exhaustive traversal matters.
func (c NewsClient) HistoricalAll(ctx context.Context, req HistoricalNewsRequest) iter.Seq2[HistoricalNewsItem, error] {
	req.ProviderCodes = append([]NewsProviderCode(nil), req.ProviderCodes...)
	return func(yield func(HistoricalNewsItem, error) bool) {
		if err := validateHistoricalNewsRequest(req); err != nil {
			yield(HistoricalNewsItem{}, err)
			return
		}

		pageReq := req
		lowerBound := req.EndTime
		var overlapSecond time.Time
		overlap := make(map[historicalNewsIdentity]struct{})

		for {
			page, err := c.Historical(ctx, pageReq)
			if err != nil {
				yield(HistoricalNewsItem{}, err)
				return
			}
			if len(page.Items) == 0 {
				if page.HasMore {
					yield(HistoricalNewsItem{}, historicalNewsStall("Gateway returned an empty page with HasMore"))
				}
				return
			}

			pageSeen := make(map[historicalNewsIdentity]struct{}, len(page.Items))
			var previous time.Time
			oldestSecond := page.Items[0].Time.Truncate(time.Second)
			newAtOverlap := 0
			boundary := make(map[historicalNewsIdentity]struct{})

			for _, item := range page.Items {
				if item.ArticleID == "" || item.ProviderCode == "" {
					yield(HistoricalNewsItem{}, historicalNewsStall("Gateway returned an item without a stable provider/article identity"))
					return
				}
				if !previous.IsZero() && item.Time.After(previous) {
					yield(HistoricalNewsItem{}, historicalNewsStall("Gateway returned a page outside descending time order"))
					return
				}
				previous = item.Time
				second := item.Time.Truncate(time.Second)
				if second.Before(oldestSecond) {
					oldestSecond = second
					boundary = make(map[historicalNewsIdentity]struct{})
				}

				identity := historicalNewsIdentity{provider: item.ProviderCode, articleID: item.ArticleID}
				if _, duplicate := pageSeen[identity]; duplicate {
					yield(HistoricalNewsItem{}, historicalNewsStall("Gateway repeated an article identity within one page"))
					return
				}
				pageSeen[identity] = struct{}{}
				if second.Equal(oldestSecond) {
					boundary[identity] = struct{}{}
				}

				if !lowerBound.IsZero() && item.Time.Before(lowerBound) {
					return
				}
				if second.Equal(overlapSecond) {
					if _, duplicate := overlap[identity]; duplicate {
						continue
					}
					newAtOverlap++
				}
				if !yield(item, nil) {
					return
				}
			}

			if !page.HasMore {
				return
			}
			if !overlapSecond.IsZero() {
				if oldestSecond.After(overlapSecond) || oldestSecond.Equal(overlapSecond) && newAtOverlap == 0 {
					yield(HistoricalNewsItem{}, historicalNewsStall("exclusive cursor did not make progress below its boundary second"))
					return
				}
			}
			if oldestSecond.Equal(overlapSecond) {
				for identity := range overlap {
					boundary[identity] = struct{}{}
				}
			}

			overlapSecond = oldestSecond
			overlap = boundary
			pageReq.StartTime = oldestSecond.Add(time.Second)
			pageReq.EndTime = time.Time{}
		}
	}
}

func historicalNewsStall(reason string) error {
	return fmt.Errorf("%w: %s", ErrHistoricalNewsPaginationStalled, reason)
}
