package ibkr_test

import (
	"context"
	"testing"
	"time"

	ibkr "github.com/ThomasMarcelis/ibkr-go/v2"
	"github.com/shopspring/decimal"
)

func TestContractDetailsServer205Replay(t *testing.T) {
	restore := ibkr.SetAdvertisedServerVersionMaxForTest(205)
	defer restore()

	client, host := newClient(t, "contract_details_sv205_live.txt")
	defer client.Close()
	defer waitHost(t, host)
	if got := client.Session().ServerVersion; got != 205 {
		t.Fatalf("Session().ServerVersion = %d, want 205", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stock, err := client.Contracts().Qualify(ctx, ibkr.Contract{ConID: 265598, Exchange: "SMART"})
	if err != nil {
		t.Fatalf("Qualify(stock) error = %v", err)
	}
	if stock.ConID != 265598 || stock.Symbol != "AAPL" || stock.SecType != ibkr.SecTypeStock ||
		stock.MinAlgoSize == nil || !stock.MinAlgoSize.IsZero() ||
		stock.LastPricePrecision == nil || !stock.LastPricePrecision.Equal(decimal.RequireFromString("0.000001")) ||
		stock.LastSizePrecision == nil || !stock.LastSizePrecision.Equal(decimal.RequireFromString("0.000001")) {
		t.Fatalf("stock details = %+v", stock)
	}

	bond, err := client.Contracts().Qualify(ctx, ibkr.Contract{ConID: 127128131, Exchange: "SMART"})
	if err != nil {
		t.Fatalf("Qualify(bond) error = %v", err)
	}
	if bond.ConID != 127128131 || bond.SecType != ibkr.SecTypeBond || bond.Bond == nil ||
		bond.Bond.CUSIP != "IBCID127128131" || bond.Bond.DescriptionAppend != "AAPL 3.85 05/04/43" ||
		bond.MinAlgoSize == nil || !bond.MinAlgoSize.IsZero() {
		t.Fatalf("bond details = %+v", bond)
	}
}
