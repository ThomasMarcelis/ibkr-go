package ibkr

import "github.com/shopspring/decimal"

// MarketOrder returns a market [Order] for quantity shares/contracts.
// Everything else (TIF, account, and the rest) stays at zero value, which
// the Gateway treats as its own defaults.
func MarketOrder(action OrderAction, quantity decimal.Decimal) Order {
	return Order{
		Action:    action,
		OrderType: OrderTypeMarket,
		Quantity:  quantity,
	}
}

// LimitOrder returns a limit [Order] for quantity shares/contracts at limit.
// Everything else (TIF, account, and the rest) stays at zero value, which
// the Gateway treats as its own defaults.
func LimitOrder(action OrderAction, quantity, limit decimal.Decimal) Order {
	return Order{
		Action:    action,
		OrderType: OrderTypeLimit,
		Quantity:  quantity,
		LmtPrice:  new(limit),
	}
}

// StopOrder returns a stop [Order] for quantity shares/contracts that
// triggers a market order at stop. Everything else (TIF, account, and the
// rest) stays at zero value, which the Gateway treats as its own defaults.
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
// Everything else (TIF, account, and the rest) stays at zero value, which
// the Gateway treats as its own defaults.
func StopLimitOrder(action OrderAction, quantity, stop, limit decimal.Decimal) Order {
	return Order{
		Action:    action,
		OrderType: OrderTypeStopLimit,
		Quantity:  quantity,
		AuxPrice:  new(stop),
		LmtPrice:  new(limit),
	}
}
