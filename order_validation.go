package ibkr

import (
	"fmt"
	"math"
	"strings"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
	"github.com/shopspring/decimal"
)

func validateOrderRequest(req PlaceOrderRequest) error {
	order := req.Order
	if err := validateOrderContract(req.Contract); err != nil {
		return err
	}

	switch order.Action {
	case ActionBuy, ActionSell, ActionSellShort, ActionSellLong:
	default:
		return invalidOrderField("Order.Action", order.Action, "must be BUY, SELL, SSHORT, or SLONG")
	}
	if strings.TrimSpace(string(order.OrderType)) == "" {
		return invalidOrderField("Order.OrderType", order.OrderType, "is required")
	}
	if err := validateOrderQuantity(order); err != nil {
		return err
	}
	if err := validateOrderPrices(order); err != nil {
		return err
	}
	if err := validateOrderTIF(order); err != nil {
		return err
	}
	if err := validateOrderID("Order.ParentID", order.ParentID, true); err != nil {
		return err
	}
	if order.DisplaySize < 0 {
		return invalidOrderField("Order.DisplaySize", order.DisplaySize, "must be >= 0")
	}
	if !validOrderTriggerMethod(order.TriggerMethod) {
		return invalidOrderField("Order.TriggerMethod", order.TriggerMethod, "must be 0, 1, 2, 3, 4, 7, or 8")
	}
	if err := validateOrderOCA(order.OCA); err != nil {
		return err
	}
	if err := validateOrderCombo(req.Contract, order.Combo); err != nil {
		return err
	}
	if err := validateOrderScale(order.Scale); err != nil {
		return err
	}
	if err := validateOrderHedge(order); err != nil {
		return err
	}
	if err := validateOrderAlgorithm(order.Algorithm); err != nil {
		return err
	}
	if err := validateOrderConditions(order.Conditions); err != nil {
		return err
	}
	return validateOrderAdjustment(order.Adjustment)
}

const maxWireOrderID = int64(math.MaxInt32)

func validateOrderID(field string, orderID int64, allowZero bool) error {
	minimum := int64(1)
	message := "must be between 1 and 2147483647"
	if allowZero {
		minimum = 0
		message = "must be between 0 and 2147483647"
	}
	if orderID < minimum || orderID > maxWireOrderID {
		return invalidOrderField(field, orderID, message)
	}
	return nil
}

func validateOrderServerVersion(order Order, serverVersion int) error {
	for _, requirement := range []struct {
		set     bool
		field   string
		version int
	}{
		{order.Deactivate, "Order.Deactivate", protocol.MinServerVersionAdditionalOrderParams1},
		{order.PostOnly, "Order.PostOnly", protocol.MinServerVersionAdditionalOrderParams1},
		{order.AllowPreOpen, "Order.AllowPreOpen", protocol.MinServerVersionAdditionalOrderParams1},
		{order.IgnoreOpenAuction, "Order.IgnoreOpenAuction", protocol.MinServerVersionAdditionalOrderParams1},
		{order.RouteMarketableToBBO != nil, "Order.RouteMarketableToBBO", protocol.MinServerVersionAdditionalOrderParams2},
		{order.SeekPriceImprovement != nil, "Order.SeekPriceImprovement", protocol.MinServerVersionAdditionalOrderParams2},
		{order.WhatIfType != nil, "Order.WhatIfType", protocol.MinServerVersionAdditionalOrderParams2},
		{order.Hedge.MaxSize != nil, "Order.Hedge.MaxSize", protocol.MinServerVersionHedgeMaxSize},
	} {
		if requirement.set && serverVersion < requirement.version {
			return fmt.Errorf("ibkr: %s requires server_version %d, negotiated %d: %w", requirement.field, requirement.version, serverVersion, ErrUnsupportedServerVersion)
		}
	}
	return nil
}

func validateExerciseOptionsRequest(req ExerciseOptionsRequest) error {
	if err := validateContract(req.Contract); err != nil {
		return err
	}
	switch req.ExerciseAction {
	case Exercise, Lapse:
	default:
		return invalidOrderField("ExerciseAction", int(req.ExerciseAction), "must be Exercise or Lapse (1 or 2)")
	}
	if req.ExerciseQuantity <= 0 {
		return invalidOrderField("ExerciseQuantity", req.ExerciseQuantity, "must be positive")
	}
	return nil
}

func prepareBracketRequest(req PlaceBracketRequest) (PlaceBracketRequest, error) {
	for _, item := range []struct {
		name  string
		order Order
	}{
		{"Parent", req.Parent},
		{"TakeProfit", req.TakeProfit},
		{"StopLoss", req.StopLoss},
	} {
		if item.order.ParentID != 0 {
			return PlaceBracketRequest{}, invalidOrderField(item.name+".ParentID", item.order.ParentID, "must be zero; PlaceBracket assigns it")
		}
		if item.order.Transmit != nil {
			return PlaceBracketRequest{}, invalidOrderField(item.name+".Transmit", *item.order.Transmit, "must be nil; PlaceBracket controls transmit sequencing")
		}
		if item.order.CashQty != nil {
			return PlaceBracketRequest{}, invalidOrderField(item.name+".CashQty", item.order.CashQty, "cash-quantity orders are not supported in a bracket")
		}
	}

	closingAction := ActionBuy
	switch req.Parent.Action {
	case ActionBuy:
		closingAction = ActionSell
	case ActionSell, ActionSellShort, ActionSellLong:
		closingAction = ActionBuy
	default:
		return PlaceBracketRequest{}, invalidOrderField("Parent.Action", req.Parent.Action, "must be a defined order action")
	}
	for _, child := range []struct {
		name  string
		order Order
	}{
		{"TakeProfit", req.TakeProfit},
		{"StopLoss", req.StopLoss},
	} {
		if child.order.Action != closingAction {
			return PlaceBracketRequest{}, invalidOrderField(child.name+".Action", child.order.Action, "must close the parent position")
		}
		if !child.order.Quantity.Equal(req.Parent.Quantity) {
			return PlaceBracketRequest{}, invalidOrderField(child.name+".Quantity", child.order.Quantity, "must equal Parent.Quantity")
		}
		if child.order.Account != req.Parent.Account {
			return PlaceBracketRequest{}, invalidOrderField(child.name+".Account", child.order.Account, "must equal Parent.Account")
		}
	}

	req = clonePlaceBracketRequest(req)
	req.Parent.Transmit = new(false)
	req.TakeProfit.ParentID = 1 // replaced with the allocated parent ID in the actor
	req.TakeProfit.Transmit = new(false)
	req.StopLoss.ParentID = 1 // replaced with the allocated parent ID in the actor
	req.StopLoss.Transmit = new(true)
	for _, order := range []Order{req.Parent, req.TakeProfit, req.StopLoss} {
		if err := validateOrderRequest(PlaceOrderRequest{Contract: req.Contract, Order: order}); err != nil {
			return PlaceBracketRequest{}, err
		}
	}
	return req, nil
}

func validateOrderContract(contract Contract) error {
	if err := validateContract(contract); err != nil {
		return err
	}
	if contract.ConID == 0 {
		if contract.SecType == "" {
			return invalidOrderField("Contract.SecType", contract.SecType, "is required when ConID is not set")
		}
		if strings.TrimSpace(contract.Symbol) == "" && strings.TrimSpace(contract.LocalSymbol) == "" {
			return invalidOrderField("Contract.Symbol", contract.Symbol, "Symbol or LocalSymbol is required when ConID is not set")
		}
	}
	if contract.SecType == SecTypeCombo && len(contract.ComboLegs) < 2 {
		return invalidOrderField("Contract.ComboLegs", len(contract.ComboLegs), "a BAG order requires at least two legs")
	}
	return nil
}

func validateOrderQuantity(order Order) error {
	if order.Quantity.IsNegative() {
		return invalidOrderField("Order.Quantity", order.Quantity, "must be >= 0")
	}
	if order.CashQty != nil && !order.CashQty.IsPositive() {
		return invalidOrderField("Order.CashQty", order.CashQty, "must be positive")
	}
	if !order.Quantity.IsZero() && order.CashQty != nil {
		return invalidOrderField("Order.CashQty", order.CashQty, "cannot be combined with Quantity")
	}
	if order.Quantity.IsZero() && order.CashQty == nil && order.Hedge.Type == "" {
		return invalidOrderField("Order.Quantity", order.Quantity, "Quantity or CashQty must be positive; only hedge children may omit both")
	}
	if order.MinQty != nil && order.MinQty.IsNegative() {
		return invalidOrderField("Order.MinQty", order.MinQty, "must be >= 0")
	}
	if !order.Quantity.IsZero() && order.MinQty != nil && order.MinQty.GreaterThan(order.Quantity) {
		return invalidOrderField("Order.MinQty", order.MinQty, "must not exceed Quantity")
	}
	return nil
}

func validateOrderPrices(order Order) error {
	// Outright prices may legitimately be negative for some futures and
	// commodity markets. Only percentage and amount fields are sign-limited.
	if order.TrailingPercent != nil && order.TrailingPercent.IsNegative() {
		return invalidOrderField("Order.TrailingPercent", order.TrailingPercent, "must be >= 0")
	}
	if order.LmtPriceOffset != nil && order.LmtPriceOffset.IsNegative() {
		return invalidOrderField("Order.LmtPriceOffset", order.LmtPriceOffset, "must be >= 0")
	}

	switch order.OrderType {
	case OrderTypeLimit, OrderTypeLimitOnClose, OrderTypeLimitOnOpen:
		if order.LmtPrice == nil {
			return invalidOrderField("Order.LmtPrice", order.LmtPrice, "is required for this order type")
		}
	case OrderTypeStop:
		if order.AuxPrice == nil {
			return invalidOrderField("Order.AuxPrice", order.AuxPrice, "is required for a stop order")
		}
	case OrderTypeStopLimit, OrderTypeLimitIfTouched:
		if order.LmtPrice == nil {
			return invalidOrderField("Order.LmtPrice", order.LmtPrice, "is required for this order type")
		}
		if order.AuxPrice == nil {
			return invalidOrderField("Order.AuxPrice", order.AuxPrice, "is required for this order type")
		}
	case OrderTypeMarketIfTouched:
		if order.AuxPrice == nil {
			return invalidOrderField("Order.AuxPrice", order.AuxPrice, "is required for a market-if-touched order")
		}
	case OrderTypeTrailingStop, OrderTypeTrailingLimit:
		if order.AuxPrice == nil && order.TrailingPercent == nil {
			return invalidOrderField("Order.AuxPrice", order.AuxPrice, "AuxPrice or TrailingPercent is required for a trailing order")
		}
	}
	return nil
}

func validateOrderTIF(order Order) error {
	if order.TIF == TIFGTD && strings.TrimSpace(order.GoodTillDate) == "" {
		return invalidOrderField("Order.GoodTillDate", order.GoodTillDate, "is required when TIF is GTD")
	}
	if order.TIF != TIFGTD && order.GoodTillDate != "" {
		return invalidOrderField("Order.GoodTillDate", order.GoodTillDate, "requires TIF GTD")
	}
	return nil
}

func validateOrderOCA(oca OrderOCA) error {
	if oca.Group == "" && oca.Type == 0 {
		return nil
	}
	if strings.TrimSpace(oca.Group) == "" {
		return invalidOrderField("Order.OCA.Group", oca.Group, "is required when OCA.Type is set")
	}
	switch oca.Type {
	case OCACancelWithBlock, OCAReduceWithBlock, OCAReduceWithoutBlock:
		return nil
	default:
		return invalidOrderField("Order.OCA.Type", oca.Type, "must be a defined OCAType when OCA.Group is set")
	}
}

func validateOrderCombo(contract Contract, combo OrderCombo) error {
	if len(combo.SmartRouting) > 0 && contract.SecType != SecTypeCombo {
		return invalidOrderField("Order.Combo.SmartRouting", len(combo.SmartRouting), "requires a BAG contract")
	}
	if len(combo.LegPrices) > 0 && len(combo.LegPrices) != len(contract.ComboLegs) {
		return invalidOrderField("Order.Combo.LegPrices", len(combo.LegPrices), "must contain one price per leg")
	}
	return validateTagValues("Order.Combo.SmartRouting", combo.SmartRouting)
}

func validateOrderScale(scale OrderScale) error {
	if scale.InitialLevelSize < 0 {
		return invalidOrderField("Order.Scale.InitialLevelSize", scale.InitialLevelSize, "must be >= 0")
	}
	if scale.SubsequentLevelSize < 0 {
		return invalidOrderField("Order.Scale.SubsequentLevelSize", scale.SubsequentLevelSize, "must be >= 0")
	}
	if scale.PriceIncrement.IsNegative() {
		return invalidOrderField("Order.Scale.PriceIncrement", scale.PriceIncrement, "must be >= 0")
	}
	return nil
}

func validateOrderHedge(order Order) error {
	hedge := order.Hedge
	if hedge.Type == "" {
		if hedge.Param != "" || hedge.DisableAutomaticPrice != nil {
			return invalidOrderField("Order.Hedge.Type", hedge.Type, "is required when other hedge fields are set")
		}
		return nil
	}
	switch hedge.Type {
	case HedgeDelta, HedgeBeta, HedgeFX, HedgePair:
	default:
		return invalidOrderField("Order.Hedge.Type", hedge.Type, "must be D, B, F, or P")
	}
	if order.ParentID <= 0 {
		return invalidOrderField("Order.ParentID", order.ParentID, "must identify the parent of a hedge order")
	}
	return nil
}

func validateOrderAlgorithm(algorithm OrderAlgorithm) error {
	if algorithm.Strategy == "" && len(algorithm.Params) > 0 {
		return invalidOrderField("Order.Algorithm.Strategy", algorithm.Strategy, "is required when algorithm parameters are set")
	}
	return validateTagValues("Order.Algorithm.Params", algorithm.Params)
}

func validateOrderConditions(conditions OrderConditions) error {
	if len(conditions.Values) == 0 {
		if conditions.IgnoreRTH || conditions.CancelOrder {
			return invalidOrderField("Order.Conditions.Values", 0, "is required when condition flags are set")
		}
		return nil
	}
	for i, condition := range conditions.Values {
		prefix := fmt.Sprintf("Order.Conditions.Values[%d]", i)
		switch condition.Type {
		case ConditionPrice, ConditionTime, ConditionMargin, ConditionExecution, ConditionVolume, ConditionPercentChange:
		default:
			return invalidOrderField(prefix+".Type", condition.Type, "is not supported by the classic socket protocol")
		}
		if condition.Conjunction != ConditionAnd && condition.Conjunction != ConditionOr {
			return invalidOrderField(prefix+".Conjunction", condition.Conjunction, "must be ConditionAnd or ConditionOr")
		}
		if condition.Type != ConditionExecution && condition.Operator != ConditionLess && condition.Operator != ConditionMore {
			return invalidOrderField(prefix+".Operator", condition.Operator, "must be ConditionLess or ConditionMore")
		}
		switch condition.Type {
		case ConditionPrice, ConditionVolume, ConditionPercentChange:
			if condition.ConID <= 0 {
				return invalidOrderField(prefix+".ConID", condition.ConID, "must be > 0")
			}
			if strings.TrimSpace(condition.Exchange) == "" {
				return invalidOrderField(prefix+".Exchange", condition.Exchange, "is required")
			}
			if strings.TrimSpace(condition.Value) == "" {
				return invalidOrderField(prefix+".Value", condition.Value, "is required")
			}
			if condition.Type == ConditionPrice && !validOrderTriggerMethod(condition.TriggerMethod) {
				return invalidOrderField(prefix+".TriggerMethod", condition.TriggerMethod, "must be 0, 1, 2, 3, 4, 7, or 8")
			}
		case ConditionTime, ConditionMargin:
			if strings.TrimSpace(condition.Value) == "" {
				return invalidOrderField(prefix+".Value", condition.Value, "is required")
			}
		case ConditionExecution:
			if condition.SecType == "" || strings.TrimSpace(condition.Exchange) == "" || strings.TrimSpace(condition.Symbol) == "" {
				return invalidOrderField(prefix, condition.Type, "execution conditions require SecType, Exchange, and Symbol")
			}
		}
	}
	return nil
}

func validateOrderAdjustment(adjustment OrderAdjustment) error {
	values := []struct {
		field string
		value decimal.Decimal
	}{
		{"Order.Adjustment.TrailingAmount", adjustment.TrailingAmount},
	}
	for _, value := range values {
		if value.value.IsNegative() {
			return invalidOrderField(value.field, value.value, "must be >= 0")
		}
	}
	if adjustment.TrailingUnit != 0 && adjustment.TrailingUnit != 1 {
		return invalidOrderField("Order.Adjustment.TrailingUnit", adjustment.TrailingUnit, "must be 0 (amount) or 1 (percent)")
	}
	adjustmentOnly := !adjustment.TriggerPrice.IsZero() || !adjustment.StopPrice.IsZero() ||
		!adjustment.StopLimitPrice.IsZero() || !adjustment.TrailingAmount.IsZero() || adjustment.TrailingUnit != 0
	if adjustment.OrderType == "" && adjustmentOnly {
		return invalidOrderField("Order.Adjustment.OrderType", adjustment.OrderType, "is required when adjustment fields are set")
	}
	if adjustment.OrderType != "" && adjustment.TriggerPrice.IsZero() {
		return invalidOrderField("Order.Adjustment.TriggerPrice", adjustment.TriggerPrice, "is required for an adjusted order")
	}
	return nil
}

func validOrderTriggerMethod(method int) bool {
	switch method {
	case 0, 1, 2, 3, 4, 7, 8:
		return true
	default:
		return false
	}
}

func validateTagValues(field string, values []TagValue) error {
	for i, value := range values {
		if strings.TrimSpace(value.Tag) == "" {
			return invalidOrderField(fmt.Sprintf("%s[%d].Tag", field, i), value.Tag, "is required")
		}
	}
	return nil
}

func invalidOrderField(field string, value any, message string) error {
	return &ValidationError{Field: field, Value: fmt.Sprint(value), Message: message}
}
