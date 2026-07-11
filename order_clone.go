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
	if order.LmtPrice != nil {
		order.LmtPrice = new(*order.LmtPrice)
	}
	if order.AuxPrice != nil {
		order.AuxPrice = new(*order.AuxPrice)
	}
	if order.MinQty != nil {
		order.MinQty = new(*order.MinQty)
	}
	if order.PercentOffset != nil {
		order.PercentOffset = new(*order.PercentOffset)
	}
	if order.TrailStopPrice != nil {
		order.TrailStopPrice = new(*order.TrailStopPrice)
	}
	if order.TrailingPercent != nil {
		order.TrailingPercent = new(*order.TrailingPercent)
	}
	if order.LmtPriceOffset != nil {
		order.LmtPriceOffset = new(*order.LmtPriceOffset)
	}
	if order.CashQty != nil {
		order.CashQty = new(*order.CashQty)
	}
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
	if order.LmtPrice != nil {
		order.LmtPrice = new(*order.LmtPrice)
	}
	if order.AuxPrice != nil {
		order.AuxPrice = new(*order.AuxPrice)
	}
	order.Combo = cloneOrderCombo(order.Combo)
	order.AlgoParams = append([]TagValue(nil), order.AlgoParams...)
	order.Conditions = append([]OrderCondition(nil), order.Conditions...)
	order.State = cloneOrderState(order.State)
	return order
}

func cloneOrderState(state OrderState) OrderState {
	for _, field := range []**decimal.Decimal{
		&state.InitMarginBefore, &state.MaintMarginBefore, &state.EquityWithLoanBefore,
		&state.InitMarginChange, &state.MaintMarginChange, &state.EquityWithLoanChange,
		&state.InitMarginAfter, &state.MaintMarginAfter, &state.EquityWithLoanAfter,
		&state.CommissionAndFees, &state.MinCommissionAndFees, &state.MaxCommissionAndFees,
		&state.InitMarginBeforeOutsideRTH, &state.MaintMarginBeforeOutsideRTH, &state.EquityWithLoanBeforeOutsideRTH,
		&state.InitMarginChangeOutsideRTH, &state.MaintMarginChangeOutsideRTH, &state.EquityWithLoanChangeOutsideRTH,
		&state.InitMarginAfterOutsideRTH, &state.MaintMarginAfterOutsideRTH, &state.EquityWithLoanAfterOutsideRTH,
		&state.SuggestedSize,
	} {
		if *field != nil {
			*field = new(**field)
		}
	}
	state.Allocations = append([]OrderAllocation(nil), state.Allocations...)
	for i := range state.Allocations {
		for _, field := range []**decimal.Decimal{
			&state.Allocations[i].Position,
			&state.Allocations[i].PositionDesired,
			&state.Allocations[i].PositionAfter,
			&state.Allocations[i].DesiredAllocQty,
			&state.Allocations[i].AllowedAllocQty,
		} {
			if *field != nil {
				*field = new(**field)
			}
		}
		if state.Allocations[i].IsMonetary != nil {
			state.Allocations[i].IsMonetary = new(*state.Allocations[i].IsMonetary)
		}
	}
	return state
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
