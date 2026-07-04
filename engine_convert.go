package ibkr

import "github.com/ThomasMarcelis/ibkr-go/internal/codec"

func toCodecContract(c Contract) codec.Contract {
	return codec.Contract{
		ConID:           c.ConID,
		Symbol:          c.Symbol,
		SecType:         string(c.SecType),
		Expiry:          c.Expiry,
		Strike:          decimalOrEmpty(c.Strike),
		Right:           string(c.Right),
		Multiplier:      c.Multiplier,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		LocalSymbol:     c.LocalSymbol,
		TradingClass:    c.TradingClass,
		PrimaryExchange: c.PrimaryExchange,
	}
}

func fromCodecContract(c codec.Contract) Contract {
	// The wire strike is parsed best-effort: a malformed or empty field decodes
	// to the zero decimal rather than propagating an error, mirroring the
	// comboLegsFromCodec precedent. Contract decode has no error return, and a
	// live contract is never torn down over an unparsable strike.
	strike, _ := parseOptionalDecimal(c.Strike, "contract strike")
	return Contract{
		ConID:           c.ConID,
		Symbol:          c.Symbol,
		SecType:         SecType(c.SecType),
		Expiry:          c.Expiry,
		Strike:          strike,
		Right:           Right(c.Right),
		Multiplier:      c.Multiplier,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		LocalSymbol:     c.LocalSymbol,
		TradingClass:    c.TradingClass,
		PrimaryExchange: c.PrimaryExchange,
	}
}
