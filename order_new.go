package ibkr

import "github.com/shopspring/decimal"

// MarketOrder returns a market [Order] for quantity shares/contracts.
// TIF is sent as DAY unless set; everything else (account and the rest)
// stays at zero value, which the Gateway treats as its own defaults.
func MarketOrder(action OrderAction, quantity decimal.Decimal) Order {
	return Order{
		Action:    action,
		OrderType: OrderTypeMarket,
		Quantity:  quantity,
	}
}

// LimitOrder returns a limit [Order] for quantity shares/contracts at limit.
// TIF is sent as DAY unless set; everything else (account and the rest)
// stays at zero value, which the Gateway treats as its own defaults.
func LimitOrder(action OrderAction, quantity, limit decimal.Decimal) Order {
	return Order{
		Action:    action,
		OrderType: OrderTypeLimit,
		Quantity:  quantity,
		LmtPrice:  new(limit),
	}
}

// StopOrder returns a stop [Order] for quantity shares/contracts that
// triggers a market order at stop. TIF is sent as DAY unless set; everything
// else (account and the rest) stays at zero value, which the Gateway treats
// as its own defaults.
func StopOrder(action OrderAction, quantity, stop decimal.Decimal) Order {
	return Order{
		Action:    action,
		OrderType: OrderTypeStop,
		Quantity:  quantity,
		AuxPrice:  new(stop),
	}
}

// StopLimitOrder returns a stop-limit [Order] for quantity shares/contracts:
// once the market trades at stop, it submits a limit order at limit.
// TIF is sent as DAY unless set; everything else (account and the rest)
// stays at zero value, which the Gateway treats as its own defaults.
func StopLimitOrder(action OrderAction, quantity, stop, limit decimal.Decimal) Order {
	return Order{
		Action:    action,
		OrderType: OrderTypeStopLimit,
		Quantity:  quantity,
		AuxPrice:  new(stop),
		LmtPrice:  new(limit),
	}
}
