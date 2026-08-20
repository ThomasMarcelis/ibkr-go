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
	for _, field := range []**decimal.Decimal{
		&order.Scale.PriceAdjustValue,
		&order.Scale.ProfitOffset,
		&order.Auction.StartingPrice,
		&order.Auction.StockRefPrice,
		&order.Auction.Delta,
		&order.Auction.StockRangeLower,
		&order.Auction.StockRangeUpper,
	} {
		if *field != nil {
			*field = new(**field)
		}
	}
	for _, field := range []**int{
		&order.Scale.PriceAdjustInterval,
		&order.Scale.InitialPosition,
		&order.Scale.InitialFillQty,
	} {
		if *field != nil {
			*field = new(**field)
		}
	}
	for _, field := range []**bool{
		&order.Scale.AutoReset,
		&order.Scale.RandomPercent,
	} {
		if *field != nil {
			*field = new(**field)
		}
	}
	if order.ShortSale.ExemptCode != nil {
		order.ShortSale.ExemptCode = new(*order.ShortSale.ExemptCode)
	}
	if order.Hedge.DisableAutomaticPrice != nil {
		order.Hedge.DisableAutomaticPrice = new(*order.Hedge.DisableAutomaticPrice)
	}
	if order.Hedge.MaxSize != nil {
		order.Hedge.MaxSize = new(*order.Hedge.MaxSize)
	}
	if order.UsePriceMgmtAlgo != nil {
		order.UsePriceMgmtAlgo = new(*order.UsePriceMgmtAlgo)
	}
	if order.RouteMarketableToBBO != nil {
		order.RouteMarketableToBBO = new(*order.RouteMarketableToBBO)
	}
	if order.SeekPriceImprovement != nil {
		order.SeekPriceImprovement = new(*order.SeekPriceImprovement)
	}
	if order.WhatIfType != nil {
		order.WhatIfType = new(*order.WhatIfType)
	}
	if order.PeggedBenchmark != nil {
		order.PeggedBenchmark = new(*order.PeggedBenchmark)
		if order.PeggedBenchmark.ReferenceChangeAmount != nil {
			order.PeggedBenchmark.ReferenceChangeAmount = new(*order.PeggedBenchmark.ReferenceChangeAmount)
		}
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
	order.Order = cloneOrderDetails(order.Order)
	order.State = cloneOrderState(order.State)
	return order
}

func cloneOrderDetails(order OrderDetails) OrderDetails {
	for _, field := range []**int64{&order.OrderID, &order.ParentID, &order.PermID} {
		if *field != nil {
			*field = new(**field)
		}
	}
	if order.ClientID != nil {
		order.ClientID = new(*order.ClientID)
	}
	for _, field := range []**bool{
		&order.Transmit,
		&order.IncludeOvernight,
		&order.Routing.RouteMarketableToBBO,
		&order.Scale.AutoReset,
		&order.Scale.RandomPercent,
		&order.Hedge.DisableAutomaticPrice,
		&order.UsePriceMgmtAlgo,
		&order.Deactivate,
		&order.PostOnly,
		&order.AllowPreOpen,
		&order.IgnoreOpenAuction,
		&order.SeekPriceImprovement,
	} {
		if *field != nil {
			*field = new(**field)
		}
	}
	for _, field := range []**decimal.Decimal{
		&order.Prices.LmtPrice, &order.Prices.AuxPrice, &order.Prices.DiscretionaryAmount,
		&order.Prices.PercentOffset, &order.Prices.TrailStopPrice, &order.Prices.TrailingPercent,
		&order.Prices.StopPrice, &order.Prices.LmtPriceOffset, &order.Prices.CashQty,
		&order.Auction.StartingPrice, &order.Auction.StockRefPrice, &order.Auction.Delta,
		&order.Auction.StockRangeLower, &order.Auction.StockRangeUpper,
		&order.Execution.CompeteAgainstBestOffset, &order.Execution.MidOffsetAtWhole,
		&order.Execution.MidOffsetAtHalf, &order.Volatility.Value,
		&order.Scale.PriceIncrement, &order.Scale.PriceAdjustValue, &order.Scale.ProfitOffset,
		&order.Adjustment.TriggerPrice, &order.Adjustment.StopPrice,
		&order.Adjustment.StopLimitPrice, &order.Adjustment.TrailingAmount,
	} {
		if *field != nil {
			*field = new(**field)
		}
	}
	for _, field := range []**int{
		&order.Routing.ExemptCode, &order.Execution.DisplaySize, &order.Execution.MinQty,
		&order.Execution.MinTradeQty, &order.Execution.MinCompeteSize, &order.Volatility.Type,
		&order.Volatility.ReferencePriceType, &order.Scale.InitialLevelSize,
		&order.Scale.SubsequentLevelSize, &order.Scale.PriceAdjustInterval,
		&order.Scale.InitialPosition, &order.Scale.InitialFillQty, &order.Hedge.MaxSize,
		&order.Adjustment.TrailingUnit, &order.WhatIfType,
	} {
		if *field != nil {
			*field = new(**field)
		}
	}
	if order.Execution.RefFuturesConID != nil {
		order.Execution.RefFuturesConID = new(*order.Execution.RefFuturesConID)
	}
	if order.Volatility.DeltaNeutral != nil {
		order.Volatility.DeltaNeutral = new(*order.Volatility.DeltaNeutral)
		if order.Volatility.DeltaNeutral.AuxPrice != nil {
			order.Volatility.DeltaNeutral.AuxPrice = new(*order.Volatility.DeltaNeutral.AuxPrice)
		}
	}
	order.Combo = cloneOrderCombo(order.Combo)
	order.Algorithm.Params = append([]TagValue(nil), order.Algorithm.Params...)
	order.Conditions.Values = append([]OrderCondition(nil), order.Conditions.Values...)
	if order.PeggedBenchmark != nil {
		order.PeggedBenchmark = new(*order.PeggedBenchmark)
		if order.PeggedBenchmark.ReferenceChangeAmount != nil {
			order.PeggedBenchmark.ReferenceChangeAmount = new(*order.PeggedBenchmark.ReferenceChangeAmount)
		}
	}
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
