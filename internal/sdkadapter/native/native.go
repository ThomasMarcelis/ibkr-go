//go:build ibkr_sdk && cgo && linux

package native

/*
#include <stdlib.h>
#include "ibkr_adapter.h"
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

type Adapter struct {
	mu     sync.Mutex
	handle *C.ibkr_adapter
	closed bool
}

func New(queueCapacity int) (*Adapter, error) {
	var cErr C.ibkr_error
	handle := C.ibkr_adapter_new(C.int(queueCapacity), &cErr)
	if handle == nil {
		defer C.ibkr_error_clear(&cErr)
		return nil, fromCError(cErr)
	}
	return &Adapter{handle: handle}, nil
}

func BuildInfo() (sdkadapter.BuildInfo, error) {
	var out C.ibkr_build_info_result
	var cErr C.ibkr_error
	ok := C.ibkr_build_info(&out, &cErr)
	if ok == 0 {
		defer C.ibkr_error_clear(&cErr)
		return sdkadapter.BuildInfo{}, fromCError(cErr)
	}
	defer C.ibkr_build_info_free(out)
	return sdkadapter.BuildInfo{
		AdapterABIVersion: goString(out.adapter_abi_version),
		SDKAPIVersion:     goString(out.sdk_api_version),
		Compiler:          goString(out.compiler),
		ProtobufMode:      goString(out.protobuf_mode),
	}, nil
}

func (a *Adapter) Connect(ctx context.Context, req sdkadapter.ConnectRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return sdkadapter.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	host := C.CString(req.Host)
	defer C.free(unsafe.Pointer(host))
	timeoutMS := int(req.Timeout / time.Millisecond)
	if timeoutMS <= 0 {
		timeoutMS = 1
	}
	var cErr C.ibkr_error
	ok := C.ibkr_adapter_connect(a.handle, host, C.int(req.Port), C.int(req.ClientID), C.int(timeoutMS), &cErr)
	if ok == 0 {
		defer C.ibkr_error_clear(&cErr)
		return fromCError(cErr)
	}
	return nil
}

func (a *Adapter) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	C.ibkr_adapter_disconnect(a.handle)
	return nil
}

func (a *Adapter) IsConnected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return false
	}
	return C.ibkr_adapter_is_connected(a.handle) != 0
}

func (a *Adapter) ServerVersion() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return 0
	}
	return int(C.ibkr_adapter_server_version(a.handle))
}

func (a *Adapter) ConnectionTime() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ""
	}
	var out C.ibkr_string
	var cErr C.ibkr_error
	ok := C.ibkr_adapter_connection_time(a.handle, &out, &cErr)
	if ok == 0 {
		C.ibkr_error_clear(&cErr)
		return ""
	}
	defer C.ibkr_string_free(out)
	return goString(out.data)
}

func (a *Adapter) Submit(ctx context.Context, command sdkadapter.Command) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return sdkadapter.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var cErr C.ibkr_error
	var ok C.int
	switch command.Kind {
	case sdkadapter.CommandCurrentTime:
		ok = C.ibkr_adapter_req_current_time(a.handle, &cErr)
	case sdkadapter.CommandCurrentTimeMillis:
		ok = C.ibkr_adapter_req_current_time_millis(a.handle, &cErr)
	case sdkadapter.CommandAccountSummary:
		group := C.CString(command.AccountSummary.Group)
		tags := C.CString(strings.Join(command.AccountSummary.Tags, ","))
		ok = C.ibkr_adapter_req_account_summary(a.handle, C.int(command.AccountSummary.ReqID), group, tags, &cErr)
		C.free(unsafe.Pointer(group))
		C.free(unsafe.Pointer(tags))
	case sdkadapter.CommandCancelAccountSummary:
		ok = C.ibkr_adapter_cancel_account_summary(a.handle, C.int(command.CancelAccountSummary.ReqID), &cErr)
	case sdkadapter.CommandAccountUpdates:
		account := C.CString(command.AccountUpdates.Account)
		ok = C.ibkr_adapter_req_account_updates(a.handle, boolInt(command.AccountUpdates.Subscribe), account, &cErr)
		C.free(unsafe.Pointer(account))
	case sdkadapter.CommandAccountUpdatesMulti:
		account := C.CString(command.AccountUpdatesMulti.Account)
		modelCode := C.CString(command.AccountUpdatesMulti.ModelCode)
		ok = C.ibkr_adapter_req_account_updates_multi(a.handle, C.int(command.AccountUpdatesMulti.ReqID), account, modelCode, &cErr)
		C.free(unsafe.Pointer(account))
		C.free(unsafe.Pointer(modelCode))
	case sdkadapter.CommandCancelAccountUpdatesMulti:
		ok = C.ibkr_adapter_cancel_account_updates_multi(a.handle, C.int(command.CancelAccountUpdatesMulti.ReqID), &cErr)
	case sdkadapter.CommandContractDetails:
		contract := toCContract(command.ContractDetails.Contract)
		ok = C.ibkr_adapter_req_contract_details(a.handle, C.int(command.ContractDetails.ReqID), &contract, &cErr)
		freeCContract(contract)
	case sdkadapter.CommandPositions:
		ok = C.ibkr_adapter_req_positions(a.handle, &cErr)
	case sdkadapter.CommandCancelPositions:
		ok = C.ibkr_adapter_cancel_positions(a.handle, &cErr)
	case sdkadapter.CommandPositionsMulti:
		account := C.CString(command.PositionsMulti.Account)
		modelCode := C.CString(command.PositionsMulti.ModelCode)
		ok = C.ibkr_adapter_req_positions_multi(a.handle, C.int(command.PositionsMulti.ReqID), account, modelCode, &cErr)
		C.free(unsafe.Pointer(account))
		C.free(unsafe.Pointer(modelCode))
	case sdkadapter.CommandCancelPositionsMulti:
		ok = C.ibkr_adapter_cancel_positions_multi(a.handle, C.int(command.CancelPositionsMulti.ReqID), &cErr)
	case sdkadapter.CommandPnL:
		account := C.CString(command.PnL.Account)
		modelCode := C.CString(command.PnL.ModelCode)
		ok = C.ibkr_adapter_req_pnl(a.handle, C.int(command.PnL.ReqID), account, modelCode, &cErr)
		C.free(unsafe.Pointer(account))
		C.free(unsafe.Pointer(modelCode))
	case sdkadapter.CommandCancelPnL:
		ok = C.ibkr_adapter_cancel_pnl(a.handle, C.int(command.CancelPnL.ReqID), &cErr)
	case sdkadapter.CommandPnLSingle:
		account := C.CString(command.PnLSingle.Account)
		modelCode := C.CString(command.PnLSingle.ModelCode)
		ok = C.ibkr_adapter_req_pnl_single(a.handle, C.int(command.PnLSingle.ReqID), account, modelCode, C.int(command.PnLSingle.ConID), &cErr)
		C.free(unsafe.Pointer(account))
		C.free(unsafe.Pointer(modelCode))
	case sdkadapter.CommandCancelPnLSingle:
		ok = C.ibkr_adapter_cancel_pnl_single(a.handle, C.int(command.CancelPnLSingle.ReqID), &cErr)
	case sdkadapter.CommandMarketDataType:
		ok = C.ibkr_adapter_req_market_data_type(a.handle, C.int(command.MarketDataType.DataType), &cErr)
	case sdkadapter.CommandQuote:
		contract := toCContract(command.Quote.Contract)
		genericTicks := C.CString(strings.Join(command.Quote.GenericTicks, ","))
		ok = C.ibkr_adapter_req_mkt_data(a.handle, C.int(command.Quote.ReqID), &contract, genericTicks, boolInt(command.Quote.Snapshot), &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(genericTicks))
	case sdkadapter.CommandCancelQuote:
		ok = C.ibkr_adapter_cancel_mkt_data(a.handle, C.int(command.CancelQuote.ReqID), &cErr)
	case sdkadapter.CommandRealTimeBars:
		contract := toCContract(command.RealTimeBars.Contract)
		whatToShow := C.CString(command.RealTimeBars.WhatToShow)
		ok = C.ibkr_adapter_req_real_time_bars(a.handle, C.int(command.RealTimeBars.ReqID), &contract, whatToShow, boolInt(command.RealTimeBars.UseRTH), &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(whatToShow))
	case sdkadapter.CommandCancelRealTimeBars:
		ok = C.ibkr_adapter_cancel_real_time_bars(a.handle, C.int(command.CancelRealTimeBars.ReqID), &cErr)
	case sdkadapter.CommandTickByTick:
		contract := toCContract(command.TickByTick.Contract)
		tickType := C.CString(command.TickByTick.TickType)
		ok = C.ibkr_adapter_req_tick_by_tick_data(a.handle, C.int(command.TickByTick.ReqID), &contract, tickType, C.int(command.TickByTick.NumberOfTicks), boolInt(command.TickByTick.IgnoreSize), &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(tickType))
	case sdkadapter.CommandCancelTickByTick:
		ok = C.ibkr_adapter_cancel_tick_by_tick_data(a.handle, C.int(command.CancelTickByTick.ReqID), &cErr)
	case sdkadapter.CommandMarketDepth:
		contract := toCContract(command.MarketDepth.Contract)
		ok = C.ibkr_adapter_req_mkt_depth(a.handle, C.int(command.MarketDepth.ReqID), &contract, C.int(command.MarketDepth.NumRows), boolInt(command.MarketDepth.IsSmartDepth), &cErr)
		freeCContract(contract)
	case sdkadapter.CommandCancelMarketDepth:
		ok = C.ibkr_adapter_cancel_mkt_depth(a.handle, C.int(command.CancelMarketDepth.ReqID), boolInt(command.CancelMarketDepth.IsSmartDepth), &cErr)
	case sdkadapter.CommandCalcImpliedVolatility:
		contract := toCContract(command.CalcImpliedVolatility.Contract)
		optionPrice := C.CString(command.CalcImpliedVolatility.OptionPrice)
		underPrice := C.CString(command.CalcImpliedVolatility.UnderPrice)
		ok = C.ibkr_adapter_calc_implied_volatility(a.handle, C.int(command.CalcImpliedVolatility.ReqID), &contract, optionPrice, underPrice, &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(optionPrice))
		C.free(unsafe.Pointer(underPrice))
	case sdkadapter.CommandCancelCalcImpliedVol:
		ok = C.ibkr_adapter_cancel_calc_implied_volatility(a.handle, C.int(command.CancelCalcImpliedVol.ReqID), &cErr)
	case sdkadapter.CommandCalcOptionPrice:
		contract := toCContract(command.CalcOptionPrice.Contract)
		volatility := C.CString(command.CalcOptionPrice.Volatility)
		underPrice := C.CString(command.CalcOptionPrice.UnderPrice)
		ok = C.ibkr_adapter_calc_option_price(a.handle, C.int(command.CalcOptionPrice.ReqID), &contract, volatility, underPrice, &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(volatility))
		C.free(unsafe.Pointer(underPrice))
	case sdkadapter.CommandCancelCalcOptionPrice:
		ok = C.ibkr_adapter_cancel_calc_option_price(a.handle, C.int(command.CancelCalcOptionPrice.ReqID), &cErr)
	case sdkadapter.CommandExerciseOptions:
		contract := toCContract(command.ExerciseOptions.Contract)
		account := C.CString(command.ExerciseOptions.Account)
		ok = C.ibkr_adapter_exercise_options(a.handle, C.int(command.ExerciseOptions.ReqID), &contract, C.int(command.ExerciseOptions.ExerciseAction), C.int(command.ExerciseOptions.ExerciseQuantity), account, C.int(command.ExerciseOptions.Override), &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(account))
	case sdkadapter.CommandPlaceOrder:
		request := toCPlaceOrder(command.PlaceOrder)
		ok = C.ibkr_adapter_place_order(a.handle, &request, &cErr)
		freeCPlaceOrder(request)
	case sdkadapter.CommandOpenOrders:
		scope := C.CString(command.OpenOrders.Scope)
		ok = C.ibkr_adapter_req_open_orders(a.handle, scope, &cErr)
		C.free(unsafe.Pointer(scope))
	case sdkadapter.CommandCompletedOrders:
		ok = C.ibkr_adapter_req_completed_orders(a.handle, boolInt(command.CompletedOrders.APIOnly), &cErr)
	case sdkadapter.CommandCancelOrder:
		manualOrderCancelTime := C.CString(command.CancelOrder.ManualOrderCancelTime)
		extOperator := C.CString(command.CancelOrder.ExtOperator)
		manualOrderIndicator := C.CString(command.CancelOrder.ManualOrderIndicator)
		ok = C.ibkr_adapter_cancel_order(a.handle, C.longlong(command.CancelOrder.OrderID), manualOrderCancelTime, extOperator, manualOrderIndicator, &cErr)
		C.free(unsafe.Pointer(manualOrderCancelTime))
		C.free(unsafe.Pointer(extOperator))
		C.free(unsafe.Pointer(manualOrderIndicator))
	case sdkadapter.CommandGlobalCancel:
		extOperator := C.CString(command.GlobalCancel.ExtOperator)
		manualOrderIndicator := C.CString(command.GlobalCancel.ManualOrderIndicator)
		ok = C.ibkr_adapter_req_global_cancel(a.handle, extOperator, manualOrderIndicator, &cErr)
		C.free(unsafe.Pointer(extOperator))
		C.free(unsafe.Pointer(manualOrderIndicator))
	case sdkadapter.CommandExecutions:
		account := C.CString(command.Executions.Account)
		symbol := C.CString(command.Executions.Symbol)
		ok = C.ibkr_adapter_req_executions(a.handle, C.int(command.Executions.ReqID), account, symbol, &cErr)
		C.free(unsafe.Pointer(account))
		C.free(unsafe.Pointer(symbol))
	case sdkadapter.CommandFamilyCodes:
		ok = C.ibkr_adapter_req_family_codes(a.handle, &cErr)
	case sdkadapter.CommandMktDepthExchanges:
		ok = C.ibkr_adapter_req_mkt_depth_exchanges(a.handle, &cErr)
	case sdkadapter.CommandNewsProviders:
		ok = C.ibkr_adapter_req_news_providers(a.handle, &cErr)
	case sdkadapter.CommandNewsBulletins:
		ok = C.ibkr_adapter_req_news_bulletins(a.handle, boolInt(command.NewsBulletins.AllMessages), &cErr)
	case sdkadapter.CommandCancelNewsBulletins:
		ok = C.ibkr_adapter_cancel_news_bulletins(a.handle, &cErr)
	case sdkadapter.CommandNewsArticle:
		providerCode := C.CString(command.NewsArticle.ProviderCode)
		articleID := C.CString(command.NewsArticle.ArticleID)
		ok = C.ibkr_adapter_req_news_article(a.handle, C.int(command.NewsArticle.ReqID), providerCode, articleID, &cErr)
		C.free(unsafe.Pointer(providerCode))
		C.free(unsafe.Pointer(articleID))
	case sdkadapter.CommandHistoricalNews:
		providerCodes := C.CString(command.HistoricalNews.ProviderCodes)
		startDate := C.CString(command.HistoricalNews.StartDate)
		endDate := C.CString(command.HistoricalNews.EndDate)
		ok = C.ibkr_adapter_req_historical_news(a.handle, C.int(command.HistoricalNews.ReqID), C.int(command.HistoricalNews.ConID), providerCodes, startDate, endDate, C.int(command.HistoricalNews.TotalResults), &cErr)
		C.free(unsafe.Pointer(providerCodes))
		C.free(unsafe.Pointer(startDate))
		C.free(unsafe.Pointer(endDate))
	case sdkadapter.CommandScannerParameters:
		ok = C.ibkr_adapter_req_scanner_parameters(a.handle, &cErr)
	case sdkadapter.CommandScannerSubscription:
		instrument := C.CString(command.ScannerSubscription.Instrument)
		locationCode := C.CString(command.ScannerSubscription.LocationCode)
		scanCode := C.CString(command.ScannerSubscription.ScanCode)
		ok = C.ibkr_adapter_req_scanner_subscription(a.handle, C.int(command.ScannerSubscription.ReqID), C.int(command.ScannerSubscription.NumberOfRows), instrument, locationCode, scanCode, &cErr)
		C.free(unsafe.Pointer(instrument))
		C.free(unsafe.Pointer(locationCode))
		C.free(unsafe.Pointer(scanCode))
	case sdkadapter.CommandCancelScannerSubscription:
		ok = C.ibkr_adapter_cancel_scanner_subscription(a.handle, C.int(command.CancelScannerSubscription.ReqID), &cErr)
	case sdkadapter.CommandRequestFA:
		ok = C.ibkr_adapter_request_fa(a.handle, C.int(command.RequestFA.FADataType), &cErr)
	case sdkadapter.CommandReplaceFA:
		xml := C.CString(command.ReplaceFA.XML)
		ok = C.ibkr_adapter_replace_fa(a.handle, C.int(command.ReplaceFA.ReqID), C.int(command.ReplaceFA.FADataType), xml, &cErr)
		C.free(unsafe.Pointer(xml))
	case sdkadapter.CommandHistoricalData:
		contract := toCContract(command.HistoricalData.Contract)
		endDateTime := C.CString(command.HistoricalData.EndDateTime)
		duration := C.CString(command.HistoricalData.Duration)
		barSize := C.CString(command.HistoricalData.BarSize)
		whatToShow := C.CString(command.HistoricalData.WhatToShow)
		ok = C.ibkr_adapter_req_historical_data(a.handle, C.int(command.HistoricalData.ReqID), &contract, endDateTime, duration, barSize, whatToShow, boolInt(command.HistoricalData.UseRTH), boolInt(command.HistoricalData.KeepUpToDate), &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(endDateTime))
		C.free(unsafe.Pointer(duration))
		C.free(unsafe.Pointer(barSize))
		C.free(unsafe.Pointer(whatToShow))
	case sdkadapter.CommandCancelHistoricalData:
		ok = C.ibkr_adapter_cancel_historical_data(a.handle, C.int(command.CancelHistoricalData.ReqID), &cErr)
	case sdkadapter.CommandHistoricalTicks:
		contract := toCContract(command.HistoricalTicks.Contract)
		startDateTime := C.CString(command.HistoricalTicks.StartDateTime)
		endDateTime := C.CString(command.HistoricalTicks.EndDateTime)
		whatToShow := C.CString(command.HistoricalTicks.WhatToShow)
		ok = C.ibkr_adapter_req_historical_ticks(a.handle, C.int(command.HistoricalTicks.ReqID), &contract, startDateTime, endDateTime, C.int(command.HistoricalTicks.NumberOfTicks), whatToShow, boolInt(command.HistoricalTicks.UseRTH), boolInt(command.HistoricalTicks.IgnoreSize), &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(startDateTime))
		C.free(unsafe.Pointer(endDateTime))
		C.free(unsafe.Pointer(whatToShow))
	case sdkadapter.CommandCancelHistoricalTicks:
		ok = C.ibkr_adapter_cancel_historical_ticks(a.handle, C.int(command.CancelHistoricalTicks.ReqID), &cErr)
	case sdkadapter.CommandHeadTimestamp:
		contract := toCContract(command.HeadTimestamp.Contract)
		whatToShow := C.CString(command.HeadTimestamp.WhatToShow)
		ok = C.ibkr_adapter_req_head_timestamp(a.handle, C.int(command.HeadTimestamp.ReqID), &contract, whatToShow, boolInt(command.HeadTimestamp.UseRTH), &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(whatToShow))
	case sdkadapter.CommandCancelHeadTimestamp:
		ok = C.ibkr_adapter_cancel_head_timestamp(a.handle, C.int(command.CancelHeadTimestamp.ReqID), &cErr)
	case sdkadapter.CommandHistogramData:
		contract := toCContract(command.HistogramData.Contract)
		period := C.CString(command.HistogramData.Period)
		ok = C.ibkr_adapter_req_histogram_data(a.handle, C.int(command.HistogramData.ReqID), &contract, boolInt(command.HistogramData.UseRTH), period, &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(period))
	case sdkadapter.CommandCancelHistogramData:
		ok = C.ibkr_adapter_cancel_histogram_data(a.handle, C.int(command.CancelHistogramData.ReqID), &cErr)
	case sdkadapter.CommandWSHMetaData:
		ok = C.ibkr_adapter_req_wsh_meta_data(a.handle, C.int(command.WSHMetaData.ReqID), &cErr)
	case sdkadapter.CommandCancelWSHMetaData:
		ok = C.ibkr_adapter_cancel_wsh_meta_data(a.handle, C.int(command.CancelWSHMetaData.ReqID), &cErr)
	case sdkadapter.CommandWSHEventData:
		filter := C.CString(command.WSHEventData.Filter)
		startDate := C.CString(command.WSHEventData.StartDate)
		endDate := C.CString(command.WSHEventData.EndDate)
		ok = C.ibkr_adapter_req_wsh_event_data(a.handle, C.int(command.WSHEventData.ReqID), C.int(command.WSHEventData.ConID), filter, boolInt(command.WSHEventData.FillWatchlist), boolInt(command.WSHEventData.FillPortfolio), boolInt(command.WSHEventData.FillCompetitors), startDate, endDate, C.int(command.WSHEventData.TotalLimit), &cErr)
		C.free(unsafe.Pointer(filter))
		C.free(unsafe.Pointer(startDate))
		C.free(unsafe.Pointer(endDate))
	case sdkadapter.CommandCancelWSHEventData:
		ok = C.ibkr_adapter_cancel_wsh_event_data(a.handle, C.int(command.CancelWSHEventData.ReqID), &cErr)
	case sdkadapter.CommandUserInfo:
		ok = C.ibkr_adapter_req_user_info(a.handle, C.int(command.UserInfo.ReqID), &cErr)
	case sdkadapter.CommandSoftDollarTiers:
		ok = C.ibkr_adapter_req_soft_dollar_tiers(a.handle, C.int(command.SoftDollarTiers.ReqID), &cErr)
	case sdkadapter.CommandQueryDisplayGroups:
		ok = C.ibkr_adapter_query_display_groups(a.handle, C.int(command.QueryDisplayGroups.ReqID), &cErr)
	case sdkadapter.CommandSubscribeToGroupEvents:
		ok = C.ibkr_adapter_subscribe_to_group_events(a.handle, C.int(command.SubscribeToGroupEvents.ReqID), C.int(command.SubscribeToGroupEvents.GroupID), &cErr)
	case sdkadapter.CommandUpdateDisplayGroup:
		contractInfo := C.CString(command.UpdateDisplayGroup.ContractInfo)
		ok = C.ibkr_adapter_update_display_group(a.handle, C.int(command.UpdateDisplayGroup.ReqID), contractInfo, &cErr)
		C.free(unsafe.Pointer(contractInfo))
	case sdkadapter.CommandUnsubscribeFromGroupEvents:
		ok = C.ibkr_adapter_unsubscribe_from_group_events(a.handle, C.int(command.UnsubscribeFromGroupEvents.ReqID), &cErr)
	case sdkadapter.CommandMatchingSymbols:
		pattern := C.CString(command.MatchingSymbols.Pattern)
		ok = C.ibkr_adapter_req_matching_symbols(a.handle, C.int(command.MatchingSymbols.ReqID), pattern, &cErr)
		C.free(unsafe.Pointer(pattern))
	case sdkadapter.CommandMarketRule:
		ok = C.ibkr_adapter_req_market_rule(a.handle, C.int(command.MarketRule.MarketRuleID), &cErr)
	case sdkadapter.CommandSecDefOptParams:
		underlyingSymbol := C.CString(command.SecDefOptParams.UnderlyingSymbol)
		futFopExchange := C.CString(command.SecDefOptParams.FutFopExchange)
		underlyingSecType := C.CString(command.SecDefOptParams.UnderlyingSecType)
		ok = C.ibkr_adapter_req_sec_def_opt_params(a.handle, C.int(command.SecDefOptParams.ReqID), underlyingSymbol, futFopExchange, underlyingSecType, C.int(command.SecDefOptParams.UnderlyingConID), &cErr)
		C.free(unsafe.Pointer(underlyingSymbol))
		C.free(unsafe.Pointer(futFopExchange))
		C.free(unsafe.Pointer(underlyingSecType))
	case sdkadapter.CommandSmartComponents:
		bboExchange := C.CString(command.SmartComponents.BBOExchange)
		ok = C.ibkr_adapter_req_smart_components(a.handle, C.int(command.SmartComponents.ReqID), bboExchange, &cErr)
		C.free(unsafe.Pointer(bboExchange))
	case sdkadapter.CommandFundamentalData:
		contract := toCContract(command.FundamentalData.Contract)
		reportType := C.CString(command.FundamentalData.ReportType)
		ok = C.ibkr_adapter_req_fundamental_data(a.handle, C.int(command.FundamentalData.ReqID), &contract, reportType, &cErr)
		freeCContract(contract)
		C.free(unsafe.Pointer(reportType))
	case sdkadapter.CommandCancelFundamentalData:
		ok = C.ibkr_adapter_cancel_fundamental_data(a.handle, C.int(command.CancelFundamentalData.ReqID), &cErr)
	default:
		return sdkadapter.ErrUnsupportedCommand
	}
	if ok == 0 {
		defer C.ibkr_error_clear(&cErr)
		return fromCError(cErr)
	}
	return nil
}

func (a *Adapter) DrainEvents(ctx context.Context, maxEvents int) ([]sdkadapter.Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, sdkadapter.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var batch *C.ibkr_event_batch
	var cErr C.ibkr_error
	ok := C.ibkr_adapter_drain_events(a.handle, C.int(maxEvents), &batch, &cErr)
	if ok == 0 {
		defer C.ibkr_error_clear(&cErr)
		return nil, fromCError(cErr)
	}
	if batch == nil {
		return nil, nil
	}
	defer C.ibkr_adapter_event_batch_free(batch)
	rows := unsafe.Slice(batch.events, int(batch.count))
	events := make([]sdkadapter.Event, 0, len(rows))
	var managedAccounts []string
	for _, row := range rows {
		event := fromCEvent(row)
		if event.Kind == sdkadapter.EventManagedAccounts {
			managedAccounts = append(managedAccounts, event.Accounts...)
			continue
		}
		if len(managedAccounts) > 0 {
			events = append(events, sdkadapter.Event{Kind: sdkadapter.EventManagedAccounts, Accounts: managedAccounts})
			managedAccounts = nil
		}
		events = append(events, event)
	}
	if len(managedAccounts) > 0 {
		events = append(events, sdkadapter.Event{Kind: sdkadapter.EventManagedAccounts, Accounts: managedAccounts})
	}
	return events, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	C.ibkr_adapter_free(a.handle)
	a.handle = nil
	a.closed = true
	return nil
}

func fromCEvent(row C.ibkr_event) sdkadapter.Event {
	event := sdkadapter.Event{
		ReqID: int(row.req_id),
	}
	switch row.kind {
	case C.IBKR_EVENT_CONNECTION_METADATA:
		event.Kind = sdkadapter.EventConnectionMetadata
		event.ServerVersion = int(row.server_version)
		event.ConnectionTime = goString(row.text)
	case C.IBKR_EVENT_CONNECTION_CLOSED:
		event.Kind = sdkadapter.EventConnectionClosed
	case C.IBKR_EVENT_NEXT_VALID_ID:
		event.Kind = sdkadapter.EventNextValidID
		event.NextValidID = int64(row.integer_value)
	case C.IBKR_EVENT_MANAGED_ACCOUNTS:
		event.Kind = sdkadapter.EventManagedAccounts
		if account := goString(row.text); account != "" {
			event.Accounts = []string{account}
		}
	case C.IBKR_EVENT_CURRENT_TIME:
		event.Kind = sdkadapter.EventCurrentTime
		event.CurrentTime = int64(row.integer_value)
	case C.IBKR_EVENT_CURRENT_TIME_MILLIS:
		event.Kind = sdkadapter.EventCurrentTimeMillis
		event.CurrentTime = int64(row.integer_value)
	case C.IBKR_EVENT_ACCOUNT_SUMMARY:
		event.Kind = sdkadapter.EventAccountSummary
		event.AccountSummary = sdkadapter.AccountSummaryValue{
			Account:  goString(row.account_summary.account),
			Tag:      goString(row.account_summary.tag),
			Value:    goString(row.account_summary.value),
			Currency: goString(row.account_summary.currency),
		}
	case C.IBKR_EVENT_ACCOUNT_SUMMARY_END:
		event.Kind = sdkadapter.EventAccountSummaryEnd
	case C.IBKR_EVENT_UPDATE_ACCOUNT_VALUE:
		event.Kind = sdkadapter.EventUpdateAccountValue
		event.AccountValue = sdkadapter.AccountValueEvent{
			Key:      goString(row.account_value.key),
			Value:    goString(row.account_value.value),
			Currency: goString(row.account_value.currency),
			Account:  goString(row.account_value.account),
		}
	case C.IBKR_EVENT_UPDATE_PORTFOLIO:
		event.Kind = sdkadapter.EventUpdatePortfolio
		event.Portfolio = sdkadapter.PortfolioValueEvent{
			Account:       goString(row.portfolio.account),
			Contract:      fromCContract(row.portfolio.contract),
			Position:      goString(row.portfolio.position),
			MarketPrice:   goString(row.portfolio.market_price),
			MarketValue:   goString(row.portfolio.market_value),
			AvgCost:       goString(row.portfolio.avg_cost),
			UnrealizedPNL: goString(row.portfolio.unrealized_pnl),
			RealizedPNL:   goString(row.portfolio.realized_pnl),
		}
	case C.IBKR_EVENT_UPDATE_ACCOUNT_TIME:
		event.Kind = sdkadapter.EventUpdateAccountTime
		event.AccountTime = goString(row.text)
	case C.IBKR_EVENT_ACCOUNT_DOWNLOAD_END:
		event.Kind = sdkadapter.EventAccountDownloadEnd
		event.AccountDownloadEnd = goString(row.account_value.account)
	case C.IBKR_EVENT_CONTRACT_DETAILS:
		event.Kind = sdkadapter.EventContractDetails
		event.ContractDetails = sdkadapter.ContractDetailsValue{
			Contract:   fromCContract(row.contract_details.contract),
			MarketName: goString(row.contract_details.market_name),
			MinTick:    goString(row.contract_details.min_tick),
			LongName:   goString(row.contract_details.long_name),
			TimeZoneID: goString(row.contract_details.time_zone_id),
		}
	case C.IBKR_EVENT_BOND_CONTRACT_DETAILS:
		event.Kind = sdkadapter.EventBondContractDetails
		event.ContractDetails = sdkadapter.ContractDetailsValue{
			Contract:   fromCContract(row.contract_details.contract),
			MarketName: goString(row.contract_details.market_name),
			MinTick:    goString(row.contract_details.min_tick),
			LongName:   goString(row.contract_details.long_name),
			TimeZoneID: goString(row.contract_details.time_zone_id),
		}
	case C.IBKR_EVENT_CONTRACT_DETAILS_END:
		event.Kind = sdkadapter.EventContractDetailsEnd
	case C.IBKR_EVENT_POSITION:
		event.Kind = sdkadapter.EventPosition
		event.Position = sdkadapter.PositionValue{
			Account:  goString(row.position.account),
			Contract: fromCContract(row.position.contract),
			Position: goString(row.position.position),
			AvgCost:  goString(row.position.avg_cost),
		}
	case C.IBKR_EVENT_POSITION_END:
		event.Kind = sdkadapter.EventPositionEnd
	case C.IBKR_EVENT_ACCOUNT_UPDATE_MULTI:
		event.Kind = sdkadapter.EventAccountUpdateMulti
		event.AccountUpdateMulti = sdkadapter.AccountUpdateMultiEvent{
			Account:   goString(row.account_update_multi.account),
			ModelCode: goString(row.account_update_multi.model_code),
			Key:       goString(row.account_update_multi.key),
			Value:     goString(row.account_update_multi.value),
			Currency:  goString(row.account_update_multi.currency),
		}
	case C.IBKR_EVENT_ACCOUNT_UPDATE_MULTI_END:
		event.Kind = sdkadapter.EventAccountUpdateMultiEnd
	case C.IBKR_EVENT_POSITION_MULTI:
		event.Kind = sdkadapter.EventPositionMulti
		event.PositionMulti = sdkadapter.PositionMultiEvent{
			Account:   goString(row.position_multi.account),
			ModelCode: goString(row.position_multi.model_code),
			Contract:  fromCContract(row.position_multi.contract),
			Position:  goString(row.position_multi.position),
			AvgCost:   goString(row.position_multi.avg_cost),
		}
	case C.IBKR_EVENT_POSITION_MULTI_END:
		event.Kind = sdkadapter.EventPositionMultiEnd
	case C.IBKR_EVENT_PNL:
		event.Kind = sdkadapter.EventPnL
		event.PnL = sdkadapter.PnLEvent{
			DailyPnL:      goString(row.pnl.daily_pnl),
			UnrealizedPnL: goString(row.pnl.unrealized_pnl),
			RealizedPnL:   goString(row.pnl.realized_pnl),
		}
	case C.IBKR_EVENT_PNL_SINGLE:
		event.Kind = sdkadapter.EventPnLSingle
		event.PnLSingle = sdkadapter.PnLSingleEvent{
			Position:      goString(row.pnl_single.position),
			DailyPnL:      goString(row.pnl_single.daily_pnl),
			UnrealizedPnL: goString(row.pnl_single.unrealized_pnl),
			RealizedPnL:   goString(row.pnl_single.realized_pnl),
			Value:         goString(row.pnl_single.value),
		}
	case C.IBKR_EVENT_OPEN_ORDER:
		event.Kind = sdkadapter.EventOpenOrder
		event.OpenOrder = fromCOpenOrder(row.open_order)
	case C.IBKR_EVENT_OPEN_ORDER_END:
		event.Kind = sdkadapter.EventOpenOrderEnd
	case C.IBKR_EVENT_COMPLETED_ORDER:
		event.Kind = sdkadapter.EventCompletedOrder
		event.CompletedOrder = sdkadapter.CompletedOrder{
			Contract:  fromCContract(row.completed_order.contract),
			Action:    goString(row.completed_order.action),
			OrderType: goString(row.completed_order.order_type),
			Status:    goString(row.completed_order.status),
			Quantity:  goString(row.completed_order.quantity),
			Filled:    goString(row.completed_order.filled),
			Remaining: goString(row.completed_order.remaining),
		}
	case C.IBKR_EVENT_COMPLETED_ORDER_END:
		event.Kind = sdkadapter.EventCompletedOrderEnd
	case C.IBKR_EVENT_ORDER_STATUS:
		event.Kind = sdkadapter.EventOrderStatus
		event.OrderStatus = sdkadapter.OrderStatusValue{
			OrderID:       int64(row.order_status.order_id),
			Status:        goString(row.order_status.status),
			Filled:        goString(row.order_status.filled),
			Remaining:     goString(row.order_status.remaining),
			AvgFillPrice:  goString(row.order_status.avg_fill_price),
			PermID:        goString(row.order_status.perm_id),
			ParentID:      goString(row.order_status.parent_id),
			LastFillPrice: goString(row.order_status.last_fill_price),
			ClientID:      goString(row.order_status.client_id),
			WhyHeld:       goString(row.order_status.why_held),
			MktCapPrice:   goString(row.order_status.mkt_cap_price),
		}
	case C.IBKR_EVENT_EXECUTION_DETAIL:
		event.Kind = sdkadapter.EventExecutionDetail
		event.ExecutionDetail = sdkadapter.ExecutionDetailValue{
			OrderID: int64(row.execution_detail.order_id),
			ExecID:  goString(row.execution_detail.exec_id),
			Account: goString(row.execution_detail.account),
			Symbol:  goString(row.execution_detail.symbol),
			Side:    goString(row.execution_detail.side),
			Shares:  goString(row.execution_detail.shares),
			Price:   goString(row.execution_detail.price),
			Time:    goString(row.execution_detail.time),
		}
	case C.IBKR_EVENT_EXECUTIONS_END:
		event.Kind = sdkadapter.EventExecutionsEnd
	case C.IBKR_EVENT_COMMISSION_REPORT:
		event.Kind = sdkadapter.EventCommissionReport
		event.CommissionReport = sdkadapter.CommissionReportValue{
			ExecID:      goString(row.commission_report.exec_id),
			Commission:  goString(row.commission_report.commission),
			Currency:    goString(row.commission_report.currency),
			RealizedPNL: goString(row.commission_report.realized_pnl),
		}
	case C.IBKR_EVENT_MARKET_DATA_TYPE:
		event.Kind = sdkadapter.EventMarketDataType
		event.MarketDataType = int(row.integer_value)
	case C.IBKR_EVENT_TICK_PRICE:
		event.Kind = sdkadapter.EventTickPrice
		event.TickPrice = sdkadapter.TickPriceValue{
			TickType: int(row.tick_price.tick_type),
			Price:    goString(row.tick_price.price),
			Size:     goString(row.tick_price.size),
			AttrMask: int(row.tick_price.attr_mask),
		}
	case C.IBKR_EVENT_TICK_SIZE:
		event.Kind = sdkadapter.EventTickSize
		event.TickSize = sdkadapter.TickSizeValue{
			TickType: int(row.tick_size.tick_type),
			Size:     goString(row.tick_size.size),
		}
	case C.IBKR_EVENT_TICK_GENERIC:
		event.Kind = sdkadapter.EventTickGeneric
		event.TickGeneric = sdkadapter.TickGenericValue{
			TickType: int(row.tick_generic.tick_type),
			Value:    goString(row.tick_generic.value),
		}
	case C.IBKR_EVENT_TICK_STRING:
		event.Kind = sdkadapter.EventTickString
		event.TickString = sdkadapter.TickStringValue{
			TickType: int(row.tick_string.tick_type),
			Value:    goString(row.tick_string.value),
		}
	case C.IBKR_EVENT_TICK_REQ_PARAMS:
		event.Kind = sdkadapter.EventTickReqParams
		event.TickReqParams = sdkadapter.TickReqParamsValue{
			MinTick:             goString(row.tick_req_params.min_tick),
			BBOExchange:         goString(row.tick_req_params.bbo_exchange),
			SnapshotPermissions: int(row.tick_req_params.snapshot_permissions),
		}
	case C.IBKR_EVENT_TICK_SNAPSHOT_END:
		event.Kind = sdkadapter.EventTickSnapshotEnd
	case C.IBKR_EVENT_REAL_TIME_BAR:
		event.Kind = sdkadapter.EventRealTimeBar
		event.RealTimeBar = fromCHistoricalBar(row.real_time_bar)
	case C.IBKR_EVENT_TICK_BY_TICK:
		event.Kind = sdkadapter.EventTickByTick
		event.TickByTick = sdkadapter.TickByTickValue{
			TickType:          int(row.tick_by_tick.tick_type),
			Time:              goString(row.tick_by_tick.time),
			Price:             goString(row.tick_by_tick.price),
			Size:              goString(row.tick_by_tick.size),
			Exchange:          goString(row.tick_by_tick.exchange),
			SpecialConditions: goString(row.tick_by_tick.special_conditions),
			BidPrice:          goString(row.tick_by_tick.bid_price),
			AskPrice:          goString(row.tick_by_tick.ask_price),
			BidSize:           goString(row.tick_by_tick.bid_size),
			AskSize:           goString(row.tick_by_tick.ask_size),
			MidPoint:          goString(row.tick_by_tick.midpoint),
			TickAttribLast:    int(row.tick_by_tick.tick_attrib_last),
			TickAttribBidAsk:  int(row.tick_by_tick.tick_attrib_bid_ask),
		}
	case C.IBKR_EVENT_MARKET_DEPTH:
		event.Kind = sdkadapter.EventMarketDepth
		event.MarketDepth = sdkadapter.MarketDepthValue{
			Position:  int(row.market_depth.position),
			Operation: int(row.market_depth.operation),
			Side:      int(row.market_depth.side),
			Price:     goString(row.market_depth.price),
			Size:      goString(row.market_depth.size),
		}
	case C.IBKR_EVENT_MARKET_DEPTH_L2:
		event.Kind = sdkadapter.EventMarketDepthL2
		event.MarketDepthL2 = sdkadapter.MarketDepthL2Value{
			Position:     int(row.market_depth_l2.position),
			MarketMaker:  goString(row.market_depth_l2.market_maker),
			Operation:    int(row.market_depth_l2.operation),
			Side:         int(row.market_depth_l2.side),
			Price:        goString(row.market_depth_l2.price),
			Size:         goString(row.market_depth_l2.size),
			IsSmartDepth: row.market_depth_l2.is_smart_depth != 0,
		}
	case C.IBKR_EVENT_TICK_OPTION_COMPUTATION:
		event.Kind = sdkadapter.EventTickOptionComputation
		event.TickOptionComputation = sdkadapter.TickOptionComputationValue{
			TickType:   int(row.tick_option_computation.tick_type),
			TickAttrib: int(row.tick_option_computation.tick_attrib),
			ImpliedVol: goString(row.tick_option_computation.implied_vol),
			Delta:      goString(row.tick_option_computation.delta),
			OptPrice:   goString(row.tick_option_computation.opt_price),
			PvDividend: goString(row.tick_option_computation.pv_dividend),
			Gamma:      goString(row.tick_option_computation.gamma),
			Vega:       goString(row.tick_option_computation.vega),
			Theta:      goString(row.tick_option_computation.theta),
			UndPrice:   goString(row.tick_option_computation.und_price),
		}
	case C.IBKR_EVENT_FAMILY_CODES:
		event.Kind = sdkadapter.EventFamilyCodes
		rows := unsafe.Slice(row.family_codes, int(row.family_codes_count))
		event.FamilyCodes = make([]sdkadapter.FamilyCodeValue, len(rows))
		for i, familyCode := range rows {
			event.FamilyCodes[i] = sdkadapter.FamilyCodeValue{
				AccountID:  goString(familyCode.account_id),
				FamilyCode: goString(familyCode.family_code),
			}
		}
	case C.IBKR_EVENT_MKT_DEPTH_EXCHANGES:
		event.Kind = sdkadapter.EventMktDepthExchanges
		rows := unsafe.Slice(row.depth_exchanges, int(row.depth_exchanges_count))
		event.DepthExchanges = make([]sdkadapter.DepthExchangeValue, len(rows))
		for i, depthExchange := range rows {
			event.DepthExchanges[i] = sdkadapter.DepthExchangeValue{
				Exchange:        goString(depthExchange.exchange),
				SecType:         goString(depthExchange.sec_type),
				ListingExch:     goString(depthExchange.listing_exch),
				ServiceDataType: goString(depthExchange.service_data_type),
				AggGroup:        int(depthExchange.agg_group),
			}
		}
	case C.IBKR_EVENT_NEWS_PROVIDERS:
		event.Kind = sdkadapter.EventNewsProviders
		rows := unsafe.Slice(row.news_providers, int(row.news_providers_count))
		event.NewsProviders = make([]sdkadapter.NewsProviderValue, len(rows))
		for i, provider := range rows {
			event.NewsProviders[i] = sdkadapter.NewsProviderValue{
				Code: goString(provider.code),
				Name: goString(provider.name),
			}
		}
	case C.IBKR_EVENT_NEWS_BULLETIN:
		event.Kind = sdkadapter.EventNewsBulletin
		event.NewsBulletin = sdkadapter.NewsBulletinEvent{
			MsgID:    int(row.news_bulletin.msg_id),
			MsgType:  int(row.news_bulletin.msg_type),
			Headline: goString(row.news_bulletin.headline),
			Source:   goString(row.news_bulletin.source),
		}
	case C.IBKR_EVENT_NEWS_ARTICLE:
		event.Kind = sdkadapter.EventNewsArticle
		event.NewsArticle = sdkadapter.NewsArticleValue{
			ArticleType: int(row.integer_value),
			ArticleText: goString(row.text),
		}
	case C.IBKR_EVENT_HISTORICAL_NEWS:
		event.Kind = sdkadapter.EventHistoricalNews
		event.HistoricalNews = sdkadapter.HistoricalNewsValue{
			Time:         goString(row.historical_news.time),
			ProviderCode: goString(row.historical_news.provider_code),
			ArticleID:    goString(row.historical_news.article_id),
			Headline:     goString(row.historical_news.headline),
		}
	case C.IBKR_EVENT_HISTORICAL_NEWS_END:
		event.Kind = sdkadapter.EventHistoricalNewsEnd
		event.HistoricalHasMore = row.integer_value != 0
	case C.IBKR_EVENT_SCANNER_PARAMETERS:
		event.Kind = sdkadapter.EventScannerParameters
		event.ScannerXML = goString(row.text)
	case C.IBKR_EVENT_SCANNER_DATA:
		event.Kind = sdkadapter.EventScannerData
		rows := unsafe.Slice(row.scanner_data, int(row.scanner_data_count))
		event.ScannerData = make([]sdkadapter.ScannerDataValue, len(rows))
		for i, entry := range rows {
			event.ScannerData[i] = sdkadapter.ScannerDataValue{
				Rank:       int(entry.rank),
				Contract:   fromCContract(entry.contract),
				Distance:   goString(entry.distance),
				Benchmark:  goString(entry.benchmark),
				Projection: goString(entry.projection),
				LegsStr:    goString(entry.legs_str),
			}
		}
	case C.IBKR_EVENT_RECEIVE_FA:
		event.Kind = sdkadapter.EventReceiveFA
		event.ReceiveFA = sdkadapter.ReceiveFAValue{
			FADataType: int(row.integer_value),
			XML:        goString(row.text),
		}
	case C.IBKR_EVENT_REPLACE_FA_END:
		event.Kind = sdkadapter.EventReplaceFAEnd
		event.ReplaceFAEndText = goString(row.text)
	case C.IBKR_EVENT_HISTORICAL_DATA:
		event.Kind = sdkadapter.EventHistoricalData
		event.HistoricalBar = fromCHistoricalBar(row.historical_bar)
	case C.IBKR_EVENT_HISTORICAL_DATA_END:
		event.Kind = sdkadapter.EventHistoricalDataEnd
	case C.IBKR_EVENT_HISTORICAL_DATA_UPDATE:
		event.Kind = sdkadapter.EventHistoricalDataUpdate
		event.HistoricalBar = fromCHistoricalBar(row.historical_bar)
	case C.IBKR_EVENT_HISTORICAL_SCHEDULE:
		event.Kind = sdkadapter.EventHistoricalSchedule
		event.HistoricalSchedule = fromCHistoricalSchedule(row.historical_schedule)
	case C.IBKR_EVENT_HISTORICAL_TICKS:
		event.Kind = sdkadapter.EventHistoricalTicks
		event.HistoricalTicksDone = row.integer_value != 0
		rows := unsafe.Slice(row.historical_ticks, int(row.historical_ticks_count))
		event.HistoricalTicks = make([]sdkadapter.HistoricalTickValue, len(rows))
		for i, tick := range rows {
			event.HistoricalTicks[i] = sdkadapter.HistoricalTickValue{
				Time:  goString(tick.time),
				Price: goString(tick.price),
				Size:  goString(tick.size),
			}
		}
	case C.IBKR_EVENT_HISTORICAL_TICKS_BID_ASK:
		event.Kind = sdkadapter.EventHistoricalTicksBidAsk
		event.HistoricalTicksDone = row.integer_value != 0
		rows := unsafe.Slice(row.historical_ticks_bid_ask, int(row.historical_ticks_bid_ask_count))
		event.HistoricalTicksBidAsk = make([]sdkadapter.HistoricalTickBidAskValue, len(rows))
		for i, tick := range rows {
			event.HistoricalTicksBidAsk[i] = sdkadapter.HistoricalTickBidAskValue{
				TickAttrib: int(tick.tick_attrib),
				Time:       goString(tick.time),
				BidPrice:   goString(tick.bid_price),
				AskPrice:   goString(tick.ask_price),
				BidSize:    goString(tick.bid_size),
				AskSize:    goString(tick.ask_size),
			}
		}
	case C.IBKR_EVENT_HISTORICAL_TICKS_LAST:
		event.Kind = sdkadapter.EventHistoricalTicksLast
		event.HistoricalTicksDone = row.integer_value != 0
		rows := unsafe.Slice(row.historical_ticks_last, int(row.historical_ticks_last_count))
		event.HistoricalTicksLast = make([]sdkadapter.HistoricalTickLastValue, len(rows))
		for i, tick := range rows {
			event.HistoricalTicksLast[i] = sdkadapter.HistoricalTickLastValue{
				TickAttrib:        int(tick.tick_attrib),
				Time:              goString(tick.time),
				Price:             goString(tick.price),
				Size:              goString(tick.size),
				Exchange:          goString(tick.exchange),
				SpecialConditions: goString(tick.special_conditions),
			}
		}
	case C.IBKR_EVENT_HEAD_TIMESTAMP:
		event.Kind = sdkadapter.EventHeadTimestamp
		event.HeadTimestamp = goString(row.text)
	case C.IBKR_EVENT_HISTOGRAM_DATA:
		event.Kind = sdkadapter.EventHistogramData
		rows := unsafe.Slice(row.histogram_data, int(row.histogram_data_count))
		event.HistogramData = make([]sdkadapter.HistogramDataValue, len(rows))
		for i, entry := range rows {
			event.HistogramData[i] = sdkadapter.HistogramDataValue{
				Price: goString(entry.price),
				Size:  goString(entry.size),
			}
		}
	case C.IBKR_EVENT_WSH_META_DATA:
		event.Kind = sdkadapter.EventWSHMetaData
		event.WSHDataJSON = goString(row.text)
	case C.IBKR_EVENT_WSH_EVENT_DATA:
		event.Kind = sdkadapter.EventWSHEventData
		event.WSHDataJSON = goString(row.text)
	case C.IBKR_EVENT_USER_INFO:
		event.Kind = sdkadapter.EventUserInfo
		event.UserInfo = sdkadapter.UserInfoValue{WhiteBrandingID: goString(row.text)}
	case C.IBKR_EVENT_SOFT_DOLLAR_TIERS:
		event.Kind = sdkadapter.EventSoftDollarTiers
		rows := unsafe.Slice(row.soft_dollar_tiers, int(row.soft_dollar_tiers_count))
		event.SoftDollarTiers = make([]sdkadapter.SoftDollarTierValue, len(rows))
		for i, tier := range rows {
			event.SoftDollarTiers[i] = sdkadapter.SoftDollarTierValue{
				Name:        goString(tier.name),
				Value:       goString(tier.value),
				DisplayName: goString(tier.display_name),
			}
		}
	case C.IBKR_EVENT_DISPLAY_GROUP_LIST:
		event.Kind = sdkadapter.EventDisplayGroupList
		event.DisplayGroups = goString(row.text)
	case C.IBKR_EVENT_DISPLAY_GROUP_UPDATED:
		event.Kind = sdkadapter.EventDisplayGroupUpdated
		event.DisplayGroupContractInfo = goString(row.text)
	case C.IBKR_EVENT_MATCHING_SYMBOLS:
		event.Kind = sdkadapter.EventMatchingSymbols
		rows := unsafe.Slice(row.symbol_samples, int(row.symbol_samples_count))
		event.SymbolSamples = make([]sdkadapter.SymbolSampleValue, len(rows))
		for i, sample := range rows {
			derivatives := unsafe.Slice(sample.derivative_sec_types, int(sample.derivative_sec_types_count))
			event.SymbolSamples[i] = sdkadapter.SymbolSampleValue{
				ConID:              int(sample.con_id),
				Symbol:             goString(sample.symbol),
				SecType:            goString(sample.sec_type),
				PrimaryExchange:    goString(sample.primary_exchange),
				Currency:           goString(sample.currency),
				DerivativeSecTypes: make([]string, len(derivatives)),
				Description:        goString(sample.description),
				IssuerID:           goString(sample.issuer_id),
			}
			for j, derivative := range derivatives {
				event.SymbolSamples[i].DerivativeSecTypes[j] = goString(derivative)
			}
		}
	case C.IBKR_EVENT_MARKET_RULE:
		event.Kind = sdkadapter.EventMarketRule
		event.MarketRuleID = int(row.market_rule_id)
		rows := unsafe.Slice(row.price_increments, int(row.price_increments_count))
		event.PriceIncrements = make([]sdkadapter.PriceIncrementValue, len(rows))
		for i, increment := range rows {
			event.PriceIncrements[i] = sdkadapter.PriceIncrementValue{
				LowEdge:   goString(increment.low_edge),
				Increment: goString(increment.increment),
			}
		}
	case C.IBKR_EVENT_SEC_DEF_OPT_PARAMS:
		event.Kind = sdkadapter.EventSecDefOptParams
		rows := unsafe.Slice(row.sec_def_opt_params, int(row.sec_def_opt_params_count))
		event.SecDefOptParams = make([]sdkadapter.SecDefOptParamsValue, len(rows))
		for i, params := range rows {
			expirations := unsafe.Slice(params.expirations, int(params.expirations_count))
			strikes := unsafe.Slice(params.strikes, int(params.strikes_count))
			event.SecDefOptParams[i] = sdkadapter.SecDefOptParamsValue{
				Exchange:        goString(params.exchange),
				UnderlyingConID: int(params.underlying_con_id),
				TradingClass:    goString(params.trading_class),
				Multiplier:      goString(params.multiplier),
				Expirations:     make([]string, len(expirations)),
				Strikes:         make([]string, len(strikes)),
			}
			for j, expiration := range expirations {
				event.SecDefOptParams[i].Expirations[j] = goString(expiration)
			}
			for j, strike := range strikes {
				event.SecDefOptParams[i].Strikes[j] = goString(strike)
			}
		}
	case C.IBKR_EVENT_SEC_DEF_OPT_PARAMS_END:
		event.Kind = sdkadapter.EventSecDefOptParamsEnd
	case C.IBKR_EVENT_SMART_COMPONENTS:
		event.Kind = sdkadapter.EventSmartComponents
		rows := unsafe.Slice(row.smart_components, int(row.smart_components_count))
		event.SmartComponents = make([]sdkadapter.SmartComponentValue, len(rows))
		for i, component := range rows {
			event.SmartComponents[i] = sdkadapter.SmartComponentValue{
				BitNumber:      int(component.bit_number),
				ExchangeName:   goString(component.exchange_name),
				ExchangeLetter: goString(component.exchange_letter),
			}
		}
	case C.IBKR_EVENT_FUNDAMENTAL_DATA:
		event.Kind = sdkadapter.EventFundamentalData
		event.FundamentalData = goString(row.text)
	case C.IBKR_EVENT_API_ERROR:
		event.Kind = sdkadapter.EventAPIError
		event.ReqID = int(row.api_error.req_id)
		event.APIError = sdkadapter.Error{
			Op:                      "api",
			ReqID:                   int(row.api_error.req_id),
			OrderID:                 int64(row.api_error.order_id),
			Code:                    int(row.api_error.code),
			Message:                 goString(row.api_error.message),
			AdvancedOrderRejectJSON: goString(row.api_error.advanced_order_reject_json),
		}
	case C.IBKR_EVENT_ADAPTER_FATAL:
		event.Kind = sdkadapter.EventAdapterFatal
		event.FatalMessage = goString(row.text)
	default:
		event.Kind = sdkadapter.EventAdapterFatal
		event.FatalMessage = fmt.Sprintf("unknown native adapter event kind %d", int(row.kind))
	}
	return event
}

func toCContract(contract sdkadapter.Contract) C.ibkr_contract {
	return C.ibkr_contract{
		con_id:           C.int(contract.ConID),
		symbol:           C.CString(contract.Symbol),
		sec_type:         C.CString(contract.SecType),
		expiry:           C.CString(contract.Expiry),
		strike:           C.CString(contract.Strike),
		right:            C.CString(contract.Right),
		multiplier:       C.CString(contract.Multiplier),
		exchange:         C.CString(contract.Exchange),
		currency:         C.CString(contract.Currency),
		local_symbol:     C.CString(contract.LocalSymbol),
		trading_class:    C.CString(contract.TradingClass),
		primary_exchange: C.CString(contract.PrimaryExchange),
	}
}

func freeCContract(contract C.ibkr_contract) {
	C.free(unsafe.Pointer(contract.symbol))
	C.free(unsafe.Pointer(contract.sec_type))
	C.free(unsafe.Pointer(contract.expiry))
	C.free(unsafe.Pointer(contract.strike))
	C.free(unsafe.Pointer(contract.right))
	C.free(unsafe.Pointer(contract.multiplier))
	C.free(unsafe.Pointer(contract.exchange))
	C.free(unsafe.Pointer(contract.currency))
	C.free(unsafe.Pointer(contract.local_symbol))
	C.free(unsafe.Pointer(contract.trading_class))
	C.free(unsafe.Pointer(contract.primary_exchange))
}

func toCPlaceOrder(request sdkadapter.PlaceOrderRequest) C.ibkr_place_order_request {
	comboLegs, comboLegsCount := toCComboLegs(request.ComboLegs)
	orderComboLegPrices, orderComboLegPricesCount := toCStringArray(request.OrderComboLegPrices)
	smartComboRoutingParams, smartComboRoutingParamsCount := toCTagValues(request.SmartComboRoutingParams)
	algoParams, algoParamsCount := toCTagValues(request.AlgoParams)
	conditions, conditionsCount := toCOrderConditions(request.Conditions)
	return C.ibkr_place_order_request{
		order_id:                         C.longlong(request.OrderID),
		contract:                         toCContract(request.Contract),
		action:                           C.CString(request.Action),
		total_quantity:                   C.CString(request.TotalQuantity),
		order_type:                       C.CString(request.OrderType),
		lmt_price:                        C.CString(request.LmtPrice),
		aux_price:                        C.CString(request.AuxPrice),
		tif:                              C.CString(request.TIF),
		oca_group:                        C.CString(request.OcaGroup),
		account:                          C.CString(request.Account),
		open_close:                       C.CString(request.OpenClose),
		origin:                           C.CString(request.Origin),
		order_ref:                        C.CString(request.OrderRef),
		transmit:                         C.CString(request.Transmit),
		parent_id:                        C.CString(request.ParentID),
		block_order:                      C.CString(request.BlockOrder),
		sweep_to_fill:                    C.CString(request.SweepToFill),
		display_size:                     C.CString(request.DisplaySize),
		trigger_method:                   C.CString(request.TriggerMethod),
		outside_rth:                      C.CString(request.OutsideRTH),
		hidden:                           C.CString(request.Hidden),
		combo_legs_count:                 comboLegsCount,
		combo_legs:                       comboLegs,
		order_combo_leg_prices_count:     orderComboLegPricesCount,
		order_combo_leg_prices:           orderComboLegPrices,
		smart_combo_routing_params_count: smartComboRoutingParamsCount,
		smart_combo_routing_params:       smartComboRoutingParams,
		fa_group:                         C.CString(request.FAGroup),
		fa_method:                        C.CString(request.FAMethod),
		fa_percentage:                    C.CString(request.FAPercentage),
		model_code:                       C.CString(request.ModelCode),
		short_sale_slot:                  C.CString(request.ShortSaleSlot),
		designated_location:              C.CString(request.DesignatedLocation),
		exempt_code:                      C.CString(request.ExemptCode),
		discretionary_amt:                C.CString(request.DiscretionaryAmt),
		good_after_time:                  C.CString(request.GoodAfterTime),
		good_till_date:                   C.CString(request.GoodTillDate),
		oca_type:                         C.CString(request.OcaType),
		rule80a:                          C.CString(request.Rule80A),
		settling_firm:                    C.CString(request.SettlingFirm),
		all_or_none:                      C.CString(request.AllOrNone),
		min_qty:                          C.CString(request.MinQty),
		percent_offset:                   C.CString(request.PercentOffset),
		auction_strategy:                 C.CString(request.AuctionStrategy),
		starting_price:                   C.CString(request.StartingPrice),
		stock_ref_price:                  C.CString(request.StockRefPrice),
		delta:                            C.CString(request.Delta),
		stock_range_lower:                C.CString(request.StockRangeLower),
		stock_range_upper:                C.CString(request.StockRangeUpper),
		override_percentage_constraints:  C.CString(request.OverridePercentageConstraints),
		volatility:                       C.CString(request.Volatility),
		volatility_type:                  C.CString(request.VolatilityType),
		delta_neutral_order_type:         C.CString(request.DeltaNeutralOrderType),
		delta_neutral_aux_price:          C.CString(request.DeltaNeutralAuxPrice),
		continuous_update:                C.CString(request.ContinuousUpdate),
		reference_price_type:             C.CString(request.ReferencePriceType),
		trail_stop_price:                 C.CString(request.TrailStopPrice),
		trailing_percent:                 C.CString(request.TrailingPercent),
		scale_init_level_size:            C.CString(request.ScaleInitLevelSize),
		scale_subs_level_size:            C.CString(request.ScaleSubsLevelSize),
		scale_price_increment:            C.CString(request.ScalePriceIncrement),
		scale_table:                      C.CString(request.ScaleTable),
		active_start_time:                C.CString(request.ActiveStartTime),
		active_stop_time:                 C.CString(request.ActiveStopTime),
		hedge_type:                       C.CString(request.HedgeType),
		hedge_param:                      C.CString(request.HedgeParam),
		opt_out_smart_routing:            C.CString(request.OptOutSmartRouting),
		clearing_account:                 C.CString(request.ClearingAccount),
		clearing_intent:                  C.CString(request.ClearingIntent),
		not_held:                         C.CString(request.NotHeld),
		delta_neutral_contract_present:   C.CString(request.DeltaNeutralContractPresent),
		algo_strategy:                    C.CString(request.AlgoStrategy),
		algo_params_count:                algoParamsCount,
		algo_params:                      algoParams,
		algo_id:                          C.CString(request.AlgoID),
		what_if:                          C.CString(request.WhatIf),
		order_misc_options:               C.CString(request.OrderMiscOptions),
		solicited:                        C.CString(request.Solicited),
		randomize_size:                   C.CString(request.RandomizeSize),
		randomize_price:                  C.CString(request.RandomizePrice),
		conditions_count:                 conditionsCount,
		conditions:                       conditions,
		conditions_ignore_rth:            C.CString(request.ConditionsIgnoreRTH),
		conditions_cancel_order:          C.CString(request.ConditionsCancelOrder),
		adjusted_order_type:              C.CString(request.AdjustedOrderType),
		trigger_price:                    C.CString(request.TriggerPrice),
		lmt_price_offset:                 C.CString(request.LmtPriceOffset),
		adjusted_stop_price:              C.CString(request.AdjustedStopPrice),
		adjusted_stop_limit_price:        C.CString(request.AdjustedStopLimitPrice),
		adjusted_trailing_amount:         C.CString(request.AdjustedTrailingAmount),
		adjustable_trailing_unit:         C.CString(request.AdjustableTrailingUnit),
		ext_operator:                     C.CString(request.ExtOperator),
		soft_dollar_name:                 C.CString(request.SoftDollarName),
		soft_dollar_value:                C.CString(request.SoftDollarValue),
		cash_qty:                         C.CString(request.CashQty),
		mifid2_decision_maker:            C.CString(request.Mifid2DecisionMaker),
		mifid2_decision_algo:             C.CString(request.Mifid2DecisionAlgo),
		mifid2_execution_trader:          C.CString(request.Mifid2ExecutionTrader),
		mifid2_execution_algo:            C.CString(request.Mifid2ExecutionAlgo),
		dont_use_auto_price_for_hedge:    C.CString(request.DontUseAutoPriceForHedge),
		is_oms_container:                 C.CString(request.IsOmsContainer),
		discretionary_up_to_limit_price:  C.CString(request.DiscretionaryUpToLimitPrice),
		use_price_mgmt_algo:              C.CString(request.UsePriceMgmtAlgo),
		duration:                         C.CString(request.Duration),
		post_to_ats:                      C.CString(request.PostToAts),
		auto_cancel_parent:               C.CString(request.AutoCancelParent),
		advanced_error_override:          C.CString(request.AdvancedErrorOverride),
		manual_order_time:                C.CString(request.ManualOrderTime),
		customer_account:                 C.CString(request.CustomerAccount),
		professional_customer:            C.CString(request.ProfessionalCustomer),
		include_overnight:                C.CString(request.IncludeOvernight),
		manual_order_indicator:           C.CString(request.ManualOrderIndicator),
		imbalance_only:                   C.CString(request.ImbalanceOnly),
	}
}

func freeCPlaceOrder(request C.ibkr_place_order_request) {
	freeCContract(request.contract)
	C.free(unsafe.Pointer(request.action))
	C.free(unsafe.Pointer(request.total_quantity))
	C.free(unsafe.Pointer(request.order_type))
	C.free(unsafe.Pointer(request.lmt_price))
	C.free(unsafe.Pointer(request.aux_price))
	C.free(unsafe.Pointer(request.tif))
	C.free(unsafe.Pointer(request.oca_group))
	C.free(unsafe.Pointer(request.account))
	C.free(unsafe.Pointer(request.open_close))
	C.free(unsafe.Pointer(request.origin))
	C.free(unsafe.Pointer(request.order_ref))
	C.free(unsafe.Pointer(request.transmit))
	C.free(unsafe.Pointer(request.parent_id))
	C.free(unsafe.Pointer(request.block_order))
	C.free(unsafe.Pointer(request.sweep_to_fill))
	C.free(unsafe.Pointer(request.display_size))
	C.free(unsafe.Pointer(request.trigger_method))
	C.free(unsafe.Pointer(request.outside_rth))
	C.free(unsafe.Pointer(request.hidden))
	freeCComboLegs(request.combo_legs, request.combo_legs_count)
	freeCStringArray(request.order_combo_leg_prices, request.order_combo_leg_prices_count)
	freeCTagValues(request.smart_combo_routing_params, request.smart_combo_routing_params_count)
	C.free(unsafe.Pointer(request.fa_group))
	C.free(unsafe.Pointer(request.fa_method))
	C.free(unsafe.Pointer(request.fa_percentage))
	C.free(unsafe.Pointer(request.model_code))
	C.free(unsafe.Pointer(request.short_sale_slot))
	C.free(unsafe.Pointer(request.designated_location))
	C.free(unsafe.Pointer(request.exempt_code))
	C.free(unsafe.Pointer(request.discretionary_amt))
	C.free(unsafe.Pointer(request.good_after_time))
	C.free(unsafe.Pointer(request.good_till_date))
	C.free(unsafe.Pointer(request.oca_type))
	C.free(unsafe.Pointer(request.rule80a))
	C.free(unsafe.Pointer(request.settling_firm))
	C.free(unsafe.Pointer(request.all_or_none))
	C.free(unsafe.Pointer(request.min_qty))
	C.free(unsafe.Pointer(request.percent_offset))
	C.free(unsafe.Pointer(request.auction_strategy))
	C.free(unsafe.Pointer(request.starting_price))
	C.free(unsafe.Pointer(request.stock_ref_price))
	C.free(unsafe.Pointer(request.delta))
	C.free(unsafe.Pointer(request.stock_range_lower))
	C.free(unsafe.Pointer(request.stock_range_upper))
	C.free(unsafe.Pointer(request.override_percentage_constraints))
	C.free(unsafe.Pointer(request.volatility))
	C.free(unsafe.Pointer(request.volatility_type))
	C.free(unsafe.Pointer(request.delta_neutral_order_type))
	C.free(unsafe.Pointer(request.delta_neutral_aux_price))
	C.free(unsafe.Pointer(request.continuous_update))
	C.free(unsafe.Pointer(request.reference_price_type))
	C.free(unsafe.Pointer(request.trail_stop_price))
	C.free(unsafe.Pointer(request.trailing_percent))
	C.free(unsafe.Pointer(request.scale_init_level_size))
	C.free(unsafe.Pointer(request.scale_subs_level_size))
	C.free(unsafe.Pointer(request.scale_price_increment))
	C.free(unsafe.Pointer(request.scale_table))
	C.free(unsafe.Pointer(request.active_start_time))
	C.free(unsafe.Pointer(request.active_stop_time))
	C.free(unsafe.Pointer(request.hedge_type))
	C.free(unsafe.Pointer(request.hedge_param))
	C.free(unsafe.Pointer(request.opt_out_smart_routing))
	C.free(unsafe.Pointer(request.clearing_account))
	C.free(unsafe.Pointer(request.clearing_intent))
	C.free(unsafe.Pointer(request.not_held))
	C.free(unsafe.Pointer(request.delta_neutral_contract_present))
	C.free(unsafe.Pointer(request.algo_strategy))
	freeCTagValues(request.algo_params, request.algo_params_count)
	C.free(unsafe.Pointer(request.algo_id))
	C.free(unsafe.Pointer(request.what_if))
	C.free(unsafe.Pointer(request.order_misc_options))
	C.free(unsafe.Pointer(request.solicited))
	C.free(unsafe.Pointer(request.randomize_size))
	C.free(unsafe.Pointer(request.randomize_price))
	freeCOrderConditions(request.conditions, request.conditions_count)
	C.free(unsafe.Pointer(request.conditions_ignore_rth))
	C.free(unsafe.Pointer(request.conditions_cancel_order))
	C.free(unsafe.Pointer(request.adjusted_order_type))
	C.free(unsafe.Pointer(request.trigger_price))
	C.free(unsafe.Pointer(request.lmt_price_offset))
	C.free(unsafe.Pointer(request.adjusted_stop_price))
	C.free(unsafe.Pointer(request.adjusted_stop_limit_price))
	C.free(unsafe.Pointer(request.adjusted_trailing_amount))
	C.free(unsafe.Pointer(request.adjustable_trailing_unit))
	C.free(unsafe.Pointer(request.ext_operator))
	C.free(unsafe.Pointer(request.soft_dollar_name))
	C.free(unsafe.Pointer(request.soft_dollar_value))
	C.free(unsafe.Pointer(request.cash_qty))
	C.free(unsafe.Pointer(request.mifid2_decision_maker))
	C.free(unsafe.Pointer(request.mifid2_decision_algo))
	C.free(unsafe.Pointer(request.mifid2_execution_trader))
	C.free(unsafe.Pointer(request.mifid2_execution_algo))
	C.free(unsafe.Pointer(request.dont_use_auto_price_for_hedge))
	C.free(unsafe.Pointer(request.is_oms_container))
	C.free(unsafe.Pointer(request.discretionary_up_to_limit_price))
	C.free(unsafe.Pointer(request.use_price_mgmt_algo))
	C.free(unsafe.Pointer(request.duration))
	C.free(unsafe.Pointer(request.post_to_ats))
	C.free(unsafe.Pointer(request.auto_cancel_parent))
	C.free(unsafe.Pointer(request.advanced_error_override))
	C.free(unsafe.Pointer(request.manual_order_time))
	C.free(unsafe.Pointer(request.customer_account))
	C.free(unsafe.Pointer(request.professional_customer))
	C.free(unsafe.Pointer(request.include_overnight))
	C.free(unsafe.Pointer(request.manual_order_indicator))
	C.free(unsafe.Pointer(request.imbalance_only))
}

func toCComboLegs(legs []sdkadapter.ComboLeg) (*C.ibkr_combo_leg_event, C.size_t) {
	if len(legs) == 0 {
		return nil, 0
	}
	ptr := (*C.ibkr_combo_leg_event)(C.calloc(C.size_t(len(legs)), C.size_t(unsafe.Sizeof(C.ibkr_combo_leg_event{}))))
	rows := unsafe.Slice(ptr, len(legs))
	for i, leg := range legs {
		rows[i].con_id = C.int(leg.ConID)
		rows[i].ratio = C.int(leg.Ratio)
		rows[i].action = C.CString(leg.Action)
		rows[i].exchange = C.CString(leg.Exchange)
		rows[i].open_close = C.CString(leg.OpenClose)
		rows[i].short_sale_slot = C.CString(leg.ShortSaleSlot)
		rows[i].designated_location = C.CString(leg.DesignatedLocation)
		rows[i].exempt_code = C.CString(leg.ExemptCode)
	}
	return ptr, C.size_t(len(legs))
}

func freeCComboLegs(ptr *C.ibkr_combo_leg_event, count C.size_t) {
	if ptr == nil || count == 0 {
		return
	}
	rows := unsafe.Slice(ptr, int(count))
	for _, row := range rows {
		C.free(unsafe.Pointer(row.action))
		C.free(unsafe.Pointer(row.exchange))
		C.free(unsafe.Pointer(row.open_close))
		C.free(unsafe.Pointer(row.short_sale_slot))
		C.free(unsafe.Pointer(row.designated_location))
		C.free(unsafe.Pointer(row.exempt_code))
	}
	C.free(unsafe.Pointer(ptr))
}

func toCStringArray(values []string) (**C.char, C.size_t) {
	if len(values) == 0 {
		return nil, 0
	}
	ptr := (**C.char)(C.calloc(C.size_t(len(values)), C.size_t(unsafe.Sizeof((*C.char)(nil)))))
	rows := unsafe.Slice(ptr, len(values))
	for i, value := range values {
		rows[i] = C.CString(value)
	}
	return ptr, C.size_t(len(values))
}

func freeCStringArray(ptr **C.char, count C.size_t) {
	if ptr == nil || count == 0 {
		return
	}
	rows := unsafe.Slice(ptr, int(count))
	for _, row := range rows {
		C.free(unsafe.Pointer(row))
	}
	C.free(unsafe.Pointer(ptr))
}

func toCTagValues(values []sdkadapter.TagValue) (*C.ibkr_tag_value_event, C.size_t) {
	if len(values) == 0 {
		return nil, 0
	}
	ptr := (*C.ibkr_tag_value_event)(C.calloc(C.size_t(len(values)), C.size_t(unsafe.Sizeof(C.ibkr_tag_value_event{}))))
	rows := unsafe.Slice(ptr, len(values))
	for i, value := range values {
		rows[i].tag = C.CString(value.Tag)
		rows[i].value = C.CString(value.Value)
	}
	return ptr, C.size_t(len(values))
}

func freeCTagValues(ptr *C.ibkr_tag_value_event, count C.size_t) {
	if ptr == nil || count == 0 {
		return
	}
	rows := unsafe.Slice(ptr, int(count))
	for _, row := range rows {
		C.free(unsafe.Pointer(row.tag))
		C.free(unsafe.Pointer(row.value))
	}
	C.free(unsafe.Pointer(ptr))
}

func toCOrderConditions(values []sdkadapter.OrderCondition) (*C.ibkr_order_condition_event, C.size_t) {
	if len(values) == 0 {
		return nil, 0
	}
	ptr := (*C.ibkr_order_condition_event)(C.calloc(C.size_t(len(values)), C.size_t(unsafe.Sizeof(C.ibkr_order_condition_event{}))))
	rows := unsafe.Slice(ptr, len(values))
	for i, value := range values {
		rows[i].condition_type = C.int(value.Type)
		rows[i].conjunction = C.CString(value.Conjunction)
		rows[i].con_id = C.int(value.ConID)
		rows[i].exchange = C.CString(value.Exchange)
		rows[i].operator_value = C.int(value.Operator)
		rows[i].value = C.CString(value.Value)
		rows[i].trigger_method = C.int(value.TriggerMethod)
		rows[i].sec_type = C.CString(value.SecType)
		rows[i].symbol = C.CString(value.Symbol)
	}
	return ptr, C.size_t(len(values))
}

func freeCOrderConditions(ptr *C.ibkr_order_condition_event, count C.size_t) {
	if ptr == nil || count == 0 {
		return
	}
	rows := unsafe.Slice(ptr, int(count))
	for _, row := range rows {
		C.free(unsafe.Pointer(row.conjunction))
		C.free(unsafe.Pointer(row.exchange))
		C.free(unsafe.Pointer(row.value))
		C.free(unsafe.Pointer(row.sec_type))
		C.free(unsafe.Pointer(row.symbol))
	}
	C.free(unsafe.Pointer(ptr))
}

func fromCContract(contract C.ibkr_contract) sdkadapter.Contract {
	return sdkadapter.Contract{
		ConID:           int(contract.con_id),
		Symbol:          goString(contract.symbol),
		SecType:         goString(contract.sec_type),
		Expiry:          goString(contract.expiry),
		Strike:          goString(contract.strike),
		Right:           goString(contract.right),
		Multiplier:      goString(contract.multiplier),
		Exchange:        goString(contract.exchange),
		Currency:        goString(contract.currency),
		LocalSymbol:     goString(contract.local_symbol),
		TradingClass:    goString(contract.trading_class),
		PrimaryExchange: goString(contract.primary_exchange),
	}
}

func fromCOpenOrder(order C.ibkr_open_order_event) sdkadapter.OpenOrder {
	return sdkadapter.OpenOrder{
		OrderID:               int64(order.order_id),
		Contract:              fromCContract(order.contract),
		Action:                goString(order.action),
		Quantity:              goString(order.quantity),
		OrderType:             goString(order.order_type),
		LmtPrice:              goString(order.lmt_price),
		AuxPrice:              goString(order.aux_price),
		TIF:                   goString(order.tif),
		OcaGroup:              goString(order.oca_group),
		Account:               goString(order.account),
		OpenClose:             goString(order.open_close),
		Origin:                goString(order.origin),
		OrderRef:              goString(order.order_ref),
		ClientID:              goString(order.client_id),
		PermID:                goString(order.perm_id),
		OutsideRTH:            goString(order.outside_rth),
		Hidden:                goString(order.hidden),
		DiscretionAmt:         goString(order.discretion_amt),
		GoodAfterTime:         goString(order.good_after_time),
		ComboLegs:             fromCComboLegs(order.combo_legs, order.combo_legs_count),
		OrderComboLegPrices:   fromCStringArray(order.order_combo_leg_prices, order.order_combo_leg_prices_count),
		SmartComboRouting:     fromCTagValues(order.smart_combo_routing, order.smart_combo_routing_count),
		AlgoStrategy:          goString(order.algo_strategy),
		AlgoParams:            fromCTagValues(order.algo_params, order.algo_params_count),
		Conditions:            fromCOrderConditions(order.conditions, order.conditions_count),
		ConditionsIgnoreRTH:   goString(order.conditions_ignore_rth),
		ConditionsCancelOrder: goString(order.conditions_cancel_order),
		Status:                goString(order.status),
		InitMarginBefore:      goString(order.init_margin_before),
		MaintMarginBefore:     goString(order.maint_margin_before),
		EquityWithLoanBefore:  goString(order.equity_with_loan_before),
		InitMarginChange:      goString(order.init_margin_change),
		MaintMarginChange:     goString(order.maint_margin_change),
		EquityWithLoanChange:  goString(order.equity_with_loan_change),
		InitMarginAfter:       goString(order.init_margin_after),
		MaintMarginAfter:      goString(order.maint_margin_after),
		EquityWithLoanAfter:   goString(order.equity_with_loan_after),
		Commission:            goString(order.commission),
		MinCommission:         goString(order.min_commission),
		MaxCommission:         goString(order.max_commission),
		CommissionCurrency:    goString(order.commission_currency),
		WarningText:           goString(order.warning_text),
		Filled:                goString(order.filled),
		Remaining:             goString(order.remaining),
		ParentID:              goString(order.parent_id),
	}
}

func fromCComboLegs(ptr *C.ibkr_combo_leg_event, count C.size_t) []sdkadapter.ComboLeg {
	if ptr == nil || count == 0 {
		return nil
	}
	rows := unsafe.Slice(ptr, int(count))
	out := make([]sdkadapter.ComboLeg, len(rows))
	for i, row := range rows {
		out[i] = sdkadapter.ComboLeg{
			ConID:              int(row.con_id),
			Ratio:              int(row.ratio),
			Action:             goString(row.action),
			Exchange:           goString(row.exchange),
			OpenClose:          goString(row.open_close),
			ShortSaleSlot:      goString(row.short_sale_slot),
			DesignatedLocation: goString(row.designated_location),
			ExemptCode:         goString(row.exempt_code),
		}
	}
	return out
}

func fromCStringArray(ptr **C.char, count C.size_t) []string {
	if ptr == nil || count == 0 {
		return nil
	}
	rows := unsafe.Slice(ptr, int(count))
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = goString(row)
	}
	return out
}

func fromCTagValues(ptr *C.ibkr_tag_value_event, count C.size_t) []sdkadapter.TagValue {
	if ptr == nil || count == 0 {
		return nil
	}
	rows := unsafe.Slice(ptr, int(count))
	out := make([]sdkadapter.TagValue, len(rows))
	for i, row := range rows {
		out[i] = sdkadapter.TagValue{Tag: goString(row.tag), Value: goString(row.value)}
	}
	return out
}

func fromCOrderConditions(ptr *C.ibkr_order_condition_event, count C.size_t) []sdkadapter.OrderCondition {
	if ptr == nil || count == 0 {
		return nil
	}
	rows := unsafe.Slice(ptr, int(count))
	out := make([]sdkadapter.OrderCondition, len(rows))
	for i, row := range rows {
		out[i] = sdkadapter.OrderCondition{
			Type:          int(row.condition_type),
			Conjunction:   goString(row.conjunction),
			ConID:         int(row.con_id),
			Exchange:      goString(row.exchange),
			Operator:      int(row.operator_value),
			Value:         goString(row.value),
			TriggerMethod: int(row.trigger_method),
			SecType:       goString(row.sec_type),
			Symbol:        goString(row.symbol),
		}
	}
	return out
}

func fromCHistoricalBar(bar C.ibkr_historical_bar_event) sdkadapter.HistoricalBarValue {
	return sdkadapter.HistoricalBarValue{
		Time:   goString(bar.time),
		Open:   goString(bar.open),
		High:   goString(bar.high),
		Low:    goString(bar.low),
		Close:  goString(bar.close),
		Volume: goString(bar.volume),
		WAP:    goString(bar.wap),
		Count:  goString(bar.count),
	}
}

func fromCHistoricalSchedule(schedule C.ibkr_historical_schedule_event) sdkadapter.HistoricalScheduleValue {
	sessions := unsafe.Slice(schedule.sessions, int(schedule.sessions_count))
	out := sdkadapter.HistoricalScheduleValue{
		StartDateTime: goString(schedule.start_date_time),
		EndDateTime:   goString(schedule.end_date_time),
		TimeZone:      goString(schedule.time_zone),
		Sessions:      make([]sdkadapter.HistoricalScheduleSessionValue, len(sessions)),
	}
	for i, session := range sessions {
		out.Sessions[i] = sdkadapter.HistoricalScheduleSessionValue{
			StartDateTime: goString(session.start_date_time),
			EndDateTime:   goString(session.end_date_time),
			RefDate:       goString(session.ref_date),
		}
	}
	return out
}

func fromCError(cErr C.ibkr_error) error {
	err := sdkadapter.Error{
		Op:                      goString(cErr.operation),
		ReqID:                   int(cErr.req_id),
		OrderID:                 int64(cErr.order_id),
		Code:                    int(cErr.code),
		Message:                 goString(cErr.message),
		AdvancedOrderRejectJSON: goString(cErr.advanced_order_reject_json),
		Phase:                   goString(cErr.phase),
	}
	if err.Message == "" {
		err.Message = "native SDK adapter error"
	}
	return err
}

func goString(value *C.char) string {
	if value == nil {
		return ""
	}
	return C.GoString(value)
}

func boolInt(value bool) C.int {
	if value {
		return 1
	}
	return 0
}
