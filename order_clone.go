package ibkr

func clonePlaceOrderRequest(req PlaceOrderRequest) PlaceOrderRequest {
	req.Order = cloneOrder(req.Order)
	return req
}

func clonePlaceBracketRequest(req PlaceBracketRequest) PlaceBracketRequest {
	req.Parent = cloneOrder(req.Parent)
	req.TakeProfit = cloneOrder(req.TakeProfit)
	req.StopLoss = cloneOrder(req.StopLoss)
	return req
}

func cloneOrder(order Order) Order {
	if order.Transmit != nil {
		order.Transmit = new(*order.Transmit)
	}
	if order.AllOrNone != nil {
		order.AllOrNone = new(*order.AllOrNone)
	}
	if order.Hedge.DisableAutomaticPrice != nil {
		order.Hedge.DisableAutomaticPrice = new(*order.Hedge.DisableAutomaticPrice)
	}
	if order.WhatIf != nil {
		order.WhatIf = new(*order.WhatIf)
	}
	if order.UsePriceMgmtAlgo != nil {
		order.UsePriceMgmtAlgo = new(*order.UsePriceMgmtAlgo)
	}
	order.Combo.Legs = append([]ComboLeg(nil), order.Combo.Legs...)
	order.Combo.LegPrices = append([]string(nil), order.Combo.LegPrices...)
	order.Combo.SmartRouting = append([]TagValue(nil), order.Combo.SmartRouting...)
	order.Algorithm.Params = append([]TagValue(nil), order.Algorithm.Params...)
	order.Conditions.Values = append([]OrderCondition(nil), order.Conditions.Values...)
	return order
}
