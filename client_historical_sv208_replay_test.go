package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func TestHistoricalBarsSV208Replay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(208)
	defer restore()

	client, host := newClient(t, "historical_bars_sv208.txt")
	defer cleanupClientHost(t, client, host)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bars, err := client.History().Bars(ctx, ibkr.HistoricalBarsRequest{
		Contract: ibkr.Contract{
			ConID:    265598,
			Symbol:   "AAPL",
			SecType:  ibkr.SecTypeStock,
			Exchange: "SMART",
			Currency: "USD",
		},
		Duration:   ibkr.Days(1),
		BarSize:    ibkr.Bar1Hour,
		WhatToShow: ibkr.ShowTrades,
		UseRTH:     true,
	})
	if err != nil {
		t.Fatalf("History().Bars(): %v", err)
	}
	if len(bars) != 3 {
		t.Fatalf("History().Bars() len = %d, want 3", len(bars))
	}
	want := []struct {
		time                   time.Time
		open, high, low, close string
		volume, wap            string
		count                  int
	}{
		{time.Date(2026, 7, 13, 13, 30, 0, 0, time.UTC), "317.04", "323.45", "316.45", "321.22", "7492200", "320.857", 54342},
		{time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC), "321.22", "321.8", "318.37", "319.07", "4255043", "320.098", 39295},
		{time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC), "319.13", "319.23", "316.09", "316.62", "2132509", "317.623", 19417},
	}
	for i, bar := range bars {
		if !bar.Time.Equal(want[i].time) || !bar.Open.Equal(decimal.RequireFromString(want[i].open)) ||
			!bar.High.Equal(decimal.RequireFromString(want[i].high)) || !bar.Low.Equal(decimal.RequireFromString(want[i].low)) ||
			!bar.Close.Equal(decimal.RequireFromString(want[i].close)) || !bar.Volume.Equal(decimal.RequireFromString(want[i].volume)) ||
			!bar.WAP.Equal(decimal.RequireFromString(want[i].wap)) || bar.Count != want[i].count {
			t.Errorf("bar[%d] = %+v, want %+v", i, bar, want[i])
		}
	}
}
