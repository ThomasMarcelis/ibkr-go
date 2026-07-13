package codec

import "testing"

func TestDecodeTickEFPFromOfficialLayout(t *testing.T) {
	t.Parallel()

	// API 10.48.01 EDecoder::processTickEfpMsg source-law vector. Positive
	// market-hours capture remains entitlement-blocked and is tracked by the
	// decoder evidence ledger.
	message, err := Decode(200, []byte("47\x001\x007801\x0038\x0012.5\x0012.5%\x00316.25\x0042\x0020260918\x000.75\x001.5\x00"))
	if err != nil {
		t.Fatal(err)
	}
	efp, ok := message.(TickEFP)
	if !ok || efp.ReqID != 7801 || efp.TickType != 38 || efp.BasisPoints != "12.5" ||
		efp.FormattedBasisPoints != "12.5%" || efp.ImpliedFuturesPrice != "316.25" ||
		efp.HoldDays != 42 || efp.FutureLastTradeDate != "20260918" ||
		efp.DividendImpact != "0.75" || efp.DividendsToLastTradeDate != "1.5" {
		t.Fatalf("TickEFP = %+v", message)
	}
}

func TestDecodeDeltaNeutralValidationFromOfficialLayout(t *testing.T) {
	t.Parallel()

	// API 10.48.01 EDecoder::processDeltaNeutralValidationMsg source-law
	// vector. A positive BAG validation callback remains a live-evidence gap.
	message, err := Decode(200, []byte("56\x001\x007802\x00265598\x000.52\x00316.25\x00"))
	if err != nil {
		t.Fatal(err)
	}
	validation, ok := message.(DeltaNeutralValidation)
	if !ok || validation.ReqID != 7802 || validation.Contract.ConID != 265598 ||
		validation.Contract.Delta != "0.52" || validation.Contract.Price != "316.25" {
		t.Fatalf("DeltaNeutralValidation = %+v", message)
	}
}
