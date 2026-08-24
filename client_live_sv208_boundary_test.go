package ibkr_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/ThomasMarcelis/ibkr-go/v2/internal/ibkrlive"
)

// TestLiveSV208ClassicBoundaryFamilies records the reachable callback families
// that still use classic bodies at the supported floor. It is opt-in because
// several requests are paced or market-hours dependent. Run it through
// ibkr-recorder; the test-only handshake cap must never become a public option.
func TestLiveSV208ClassicBoundaryFamilies(t *testing.T) {
	if os.Getenv("IBKR_LIVE_SV208_BOUNDARY") != "1" {
		t.Skip("set IBKR_LIVE_SV208_BOUNDARY=1 and run through ibkr-recorder")
	}
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(208)
	defer restore()

	client, rootCtx, cancel := ibkrlive.DialContext(t, 4*time.Minute)
	defer cancel()
	defer client.Close()
	if got := client.Session().ServerVersion; got != 208 {
		t.Fatalf("negotiated ServerVersion = %d, want 208", got)
	}
	accounts := client.Session().ManagedAccounts
	if len(accounts) == 0 {
		t.Fatal("session reported no managed account")
	}
	account := accounts[0]

	step := func(name string, timeout time.Duration, run func(context.Context) error) {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(rootCtx, timeout)
			defer cancel()
			if err := run(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}

	step("current_time_millis", 10*time.Second, func(ctx context.Context) error {
		_, err := client.CurrentTimeMillis(ctx)
		return err
	})
	step("family_codes", 10*time.Second, func(ctx context.Context) error {
		codes, err := client.Accounts().FamilyCodes(ctx)
		if err == nil && len(codes) == 0 {
			return errors.New("FamilyCodes returned no rows")
		}
		return err
	})
	step("news_providers", 10*time.Second, func(ctx context.Context) error {
		providers, err := client.News().Providers(ctx)
		if err == nil && len(providers) == 0 {
			return errors.New("NewsProviders returned no rows")
		}
		return err
	})

	var articleRequest ibkr.NewsArticleRequest
	step("historical_news", 20*time.Second, func(ctx context.Context) error {
		result, err := client.News().Historical(ctx, ibkr.HistoricalNewsRequest{
			ConID: 265598, ProviderCodes: []ibkr.NewsProviderCode{"BRFG", "BRFUPDN", "DJNL"}, TotalResults: 5,
		})
		if err != nil {
			return err
		}
		if len(result.Items) == 0 {
			return errors.New("HistoricalNews returned no rows")
		}
		articleRequest = ibkr.NewsArticleRequest{ProviderCode: result.Items[0].ProviderCode, ArticleID: result.Items[0].ArticleID}
		return nil
	})
	if articleRequest.ArticleID != "" {
		step("news_article", 20*time.Second, func(ctx context.Context) error {
			article, err := client.News().Article(ctx, articleRequest)
			if err == nil && article.ArticleText == "" {
				return errors.New("NewsArticle returned an empty body")
			}
			return err
		})
	}

	step("scanner_parameters", 30*time.Second, func(ctx context.Context) error {
		parameters, err := client.Scanner().Parameters(ctx)
		if err == nil && len(parameters) == 0 {
			return errors.New("ScannerParameters returned an empty document")
		}
		return err
	})
	step("scanner_results", 30*time.Second, func(ctx context.Context) error {
		sub, err := client.Scanner().SubscribeResults(ctx, ibkr.ScannerSubscriptionRequest{
			NumberOfRows: 10, Instrument: "STK", LocationCode: "STK.US.MAJOR", ScanCode: "HOT_BY_VOLUME",
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return err
		}
		defer sub.Close()
		rows, err := firstStreamData(ctx, sub)
		if err == nil && len(rows) == 0 {
			return errors.New("scanner returned an empty first result")
		}
		return err
	})
	step("sec_def_opt_params", 20*time.Second, func(ctx context.Context) error {
		params, err := client.Contracts().SecDefOptParams(ctx, ibkr.SecDefOptParamsRequest{
			UnderlyingSymbol: "AAPL", UnderlyingSecType: ibkr.SecTypeStock, UnderlyingConID: 265598,
		})
		if err == nil && len(params) == 0 {
			return errors.New("SecDefOptParams returned no rows")
		}
		return err
	})
	step("matching_symbols", 10*time.Second, func(ctx context.Context) error {
		matches, err := client.Contracts().Search(ctx, "AAPL")
		if err == nil && len(matches) == 0 {
			return errors.New("MatchingSymbols returned no rows")
		}
		return err
	})
	step("depth_exchanges", 10*time.Second, func(ctx context.Context) error {
		exchanges, err := client.Contracts().DepthExchanges(ctx)
		if err == nil && len(exchanges) == 0 {
			return errors.New("MktDepthExchanges returned no rows")
		}
		return err
	})
	step("market_rule", 10*time.Second, func(ctx context.Context) error {
		rule, err := client.Contracts().MarketRule(ctx, 26)
		if err == nil && len(rule.Increments) == 0 {
			return errors.New("MarketRule returned no increments")
		}
		return err
	})
	step("soft_dollar_tiers", 10*time.Second, func(ctx context.Context) error {
		_, err := client.Advisors().SoftDollarTiers(ctx)
		return err
	})
	step("user_info", 10*time.Second, func(ctx context.Context) error {
		_, err := client.TWS().UserInfo(ctx)
		return err
	})
	step("display_groups", 10*time.Second, func(ctx context.Context) error {
		_, err := client.TWS().DisplayGroups(ctx)
		return err
	})
	step("display_group_subscribe", 10*time.Second, func(ctx context.Context) error {
		sub, err := client.TWS().SubscribeDisplayGroup(ctx, 1, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return err
		}
		defer sub.Close()
		_, err = firstStreamData(ctx, sub.Subscription)
		return err
	})
	step("account_pnl", 20*time.Second, func(ctx context.Context) error {
		sub, err := client.Accounts().SubscribePnL(ctx, ibkr.PnLRequest{Account: account}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return err
		}
		defer sub.Close()
		_, err = firstStreamData(ctx, sub)
		return err
	})
	step("single_position_pnl", 20*time.Second, func(ctx context.Context) error {
		positions, err := client.Accounts().Positions(ctx)
		if err != nil {
			return err
		}
		for _, position := range positions {
			if position.Account != account || position.Contract.ConID == 0 || position.Position.IsZero() {
				continue
			}
			sub, err := client.Accounts().SubscribePnLSingle(ctx, ibkr.PnLSingleRequest{
				Account: account, ConID: position.Contract.ConID,
			}, ibkr.WithResumePolicy(ibkr.ResumeNever))
			if err != nil {
				return err
			}
			defer sub.Close()
			_, err = firstStreamData(ctx, sub)
			return err
		}
		return errors.New("PnLSingle requires a held position")
	})
	step("smart_components", 30*time.Second, func(ctx context.Context) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return err
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{Contract: aaplContract}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return err
		}
		defer sub.Close()
		for {
			update, err := firstStreamData(ctx, sub)
			if err != nil {
				return err
			}
			if update.Kind != ibkr.QuoteUpdateParameters || update.Parameters == nil || update.Parameters.BBOExchange == "" {
				continue
			}
			_, err = client.Contracts().SmartComponents(ctx, update.Parameters.BBOExchange)
			return err
		}
	})
	step("tick_news", 45*time.Second, func(ctx context.Context) error {
		if err := client.MarketData().SetType(ctx, ibkr.MarketDataDelayed); err != nil {
			return err
		}
		sub, err := client.MarketData().SubscribeQuotes(ctx, ibkr.QuoteRequest{
			Contract: aaplContract, GenericTicks: []ibkr.GenericTick{"mdoff", "292:BRFG"},
		}, ibkr.WithResumePolicy(ibkr.ResumeNever))
		if err != nil {
			return err
		}
		defer sub.Close()
		for {
			update, err := firstStreamData(ctx, sub)
			if err != nil {
				return err
			}
			if update.Kind == ibkr.QuoteUpdateNewsTick {
				return nil
			}
		}
	})
}

// TestLiveSV210ClassicOptionCalculations captures the last classic request
// layout before option calculations move to protobuf at server version 211.
func TestLiveSV210ClassicOptionCalculations(t *testing.T) {
	if os.Getenv("IBKR_LIVE_SV210_OPTION_CALCULATIONS") != "1" {
		t.Skip("set IBKR_LIVE_SV210_OPTION_CALCULATIONS=1 and run through ibkr-recorder")
	}
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(210)
	defer restore()

	client, rootCtx, cancel := ibkrlive.DialContext(t, 2*time.Minute)
	defer cancel()
	defer client.Close()
	if got := client.Session().ServerVersion; got != 210 {
		t.Fatalf("negotiated ServerVersion = %d, want 210", got)
	}
	runLiveOptionCalculations(t, rootCtx, client)
}

func firstStreamData[T any](ctx context.Context, sub *ibkr.Subscription[T]) (T, error) {
	var zero T
	for {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				return zero, sub.Wait()
			}
			if event.Err != nil {
				return zero, event.Err
			}
			if event.Kind == ibkr.StreamData {
				return event.Value, nil
			}
		case <-ctx.Done():
			return zero, context.Cause(ctx)
		}
	}
}
