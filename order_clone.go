package ibkr

import "github.com/shopspring/decimal"

func clonePlaceOrderRequest(req PlaceOrderRequest) PlaceOrderRequest {
	req.Contract = cloneContract(req.Contract)
	req.Order = cloneOrder(req.Order)
	return req
}

func clonePlaceBracketRequest(req PlaceBracketRequest) PlaceBracketRequest {
	req.Contract = cloneContract(req.Contract)
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
	if order.UsePriceMgmtAlgo != nil {
		order.UsePriceMgmtAlgo = new(*order.UsePriceMgmtAlgo)
	}
	order.Combo = cloneOrderCombo(order.Combo)
	order.Algorithm.Params = append([]TagValue(nil), order.Algorithm.Params...)
	order.Conditions.Values = append([]OrderCondition(nil), order.Conditions.Values...)
	return order
}

func cloneOrderCombo(combo OrderCombo) OrderCombo {
	combo.LegPrices = append([]*decimal.Decimal(nil), combo.LegPrices...)
	for i := range combo.LegPrices {
		if combo.LegPrices[i] != nil {
			combo.LegPrices[i] = new(*combo.LegPrices[i])
		}
	}
	combo.SmartRouting = append([]TagValue(nil), combo.SmartRouting...)
	return combo
}

func cloneOpenOrder(order OpenOrder) OpenOrder {
	order.Contract = cloneContract(order.Contract)
	order.Combo = cloneOrderCombo(order.Combo)
	order.AlgoParams = append([]TagValue(nil), order.AlgoParams...)
	order.Conditions = append([]OrderCondition(nil), order.Conditions...)
	return order
}

func cloneContract(contract Contract) Contract {
	if contract.Strike != nil {
		contract.Strike = new(*contract.Strike)
	}
	contract.ComboLegs = append([]ComboLeg(nil), contract.ComboLegs...)
	for i := range contract.ComboLegs {
		if contract.ComboLegs[i].ExemptCode != nil {
			contract.ComboLegs[i].ExemptCode = new(*contract.ComboLegs[i].ExemptCode)
		}
	}
	if contract.DeltaNeutral != nil {
		contract.DeltaNeutral = new(*contract.DeltaNeutral)
	}
	return contract
}
