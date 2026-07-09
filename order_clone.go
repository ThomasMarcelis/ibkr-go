package ibkr

func clonePlaceOrderRequest(req PlaceOrderRequest) PlaceOrderRequest {
	req.Order = cloneOrder(req.Order)
	return req
}

func cloneOrder(order Order) Order {
	order.Combo.Legs = append([]ComboLeg(nil), order.Combo.Legs...)
	order.Combo.LegPrices = append([]string(nil), order.Combo.LegPrices...)
	order.Combo.SmartRouting = append([]TagValue(nil), order.Combo.SmartRouting...)
	order.Algorithm.Params = append([]TagValue(nil), order.Algorithm.Params...)
	order.Conditions.Values = append([]OrderCondition(nil), order.Conditions.Values...)
	return order
}
