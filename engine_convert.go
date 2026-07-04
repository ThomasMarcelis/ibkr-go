package ibkr

import "github.com/ThomasMarcelis/ibkr-go/internal/codec"

func toCodecContract(c Contract) codec.Contract {
	return codec.Contract{
		ConID:           c.ConID,
		Symbol:          c.Symbol,
		SecType:         string(c.SecType),
		Expiry:          c.Expiry,
		Strike:          c.Strike,
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
	return Contract{
		ConID:           c.ConID,
		Symbol:          c.Symbol,
		SecType:         SecType(c.SecType),
		Expiry:          c.Expiry,
		Strike:          c.Strike,
		Right:           Right(c.Right),
		Multiplier:      c.Multiplier,
		Exchange:        c.Exchange,
		Currency:        c.Currency,
		LocalSymbol:     c.LocalSymbol,
		TradingClass:    c.TradingClass,
		PrimaryExchange: c.PrimaryExchange,
	}
}
