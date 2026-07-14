package ibkr

import (
	"slices"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/codec"
)

func fromCodecTWSConfig(config codec.ConfigResponse) TWSConfig {
	result := TWSConfig{Messages: make([]TWSMessageConfig, len(config.Messages))}
	if config.LockAndExit != nil {
		result.LockAndExit = &TWSLockAndExitConfig{
			AutoLogoffTime:   config.LockAndExit.AutoLogoffTime,
			AutoLogoffPeriod: config.LockAndExit.AutoLogoffPeriod,
			AutoLogoffType:   config.LockAndExit.AutoLogoffType,
		}
	}
	for i, message := range config.Messages {
		result.Messages[i] = TWSMessageConfig{
			ID: message.ID, Title: message.Title, Message: message.Message,
			DefaultAction: message.DefaultAction, Enabled: message.Enabled,
		}
	}
	if config.API != nil {
		result.API = &TWSAPIConfig{}
		if config.API.Precautions != nil {
			p := config.API.Precautions
			result.API.Precautions = &TWSAPIPrecautionsConfig{
				BypassOrderPrecautions: p.BypassOrderPrecautions, BypassBondWarning: p.BypassBondWarning,
				BypassNegativeYieldConfirmation: p.BypassNegativeYieldConfirmation, BypassCalledBondWarning: p.BypassCalledBondWarning,
				BypassSameActionPairTradeWarning: p.BypassSameActionPairTradeWarning, BypassFlaggedAccountsWarning: p.BypassFlaggedAccountsWarning,
				BypassPriceBasedVolatilityWarning: p.BypassPriceBasedVolatilityWarning, BypassRedirectOrderWarning: p.BypassRedirectOrderWarning,
				BypassNoOverfillProtection: p.BypassNoOverfillProtection, BypassRouteMarketableToBBO: p.BypassRouteMarketableToBBO,
			}
		}
		if config.API.Settings != nil {
			s := config.API.Settings
			result.API.Settings = &TWSAPISettingsConfig{
				ReadOnlyAPI: s.ReadOnlyAPI, TotalQuantityForMutualFunds: s.TotalQuantityForMutualFunds,
				DownloadOpenOrdersOnConnection: s.DownloadOpenOrdersOnConnection, IncludeVirtualFXPositions: s.IncludeVirtualFXPositions,
				PrepareDailyPnL: s.PrepareDailyPnL, SendStatusUpdatesForVolatilityOrders: s.SendStatusUpdatesForVolatilityOrders,
				EncodeAPIMessages: s.EncodeAPIMessages, SocketPort: s.SocketPort, UseNegativeAutoRange: s.UseNegativeAutoRange,
				CreateAPIMessageLogFile: s.CreateAPIMessageLogFile, IncludeMarketDataInLogFile: s.IncludeMarketDataInLogFile,
				ExposeTradingScheduleToAPI: s.ExposeTradingScheduleToAPI, SplitInsuredDepositFromCashBalance: s.SplitInsuredDepositFromCashBalance,
				SendZeroPositionsForTodayOnly: s.SendZeroPositionsForTodayOnly, LetAPIAccountRequestsSwitchSubscription: s.LetAPIAccountRequestsSwitchSubscription,
				UseAccountGroupsWithAllocationMethods: s.UseAccountGroupsWithAllocationMethods, LoggingLevel: s.LoggingLevel,
				MasterClientID: optionalClientID(s.MasterClientID), BulkDataTimeout: s.BulkDataTimeout, ComponentExchangeSeparator: s.ComponentExchangeSeparator,
				ShowForexDataInOneTenthPips: s.ShowForexDataInOneTenthPips, AllowForexTradingInOneTenthPips: s.AllowForexTradingInOneTenthPips,
				RoundAccountValuesToNearestWholeNumber: s.RoundAccountValuesToNearestWholeNumber,
				SendMarketDataInLotsForUSStocks:        s.SendMarketDataInLotsForUSStocks, ShowAdvancedOrderRejectInUI: s.ShowAdvancedOrderRejectInUI,
				RejectMessagesAboveMaxRate: s.RejectMessagesAboveMaxRate, MaintainConnectionOnIncorrectFields: s.MaintainConnectionOnIncorrectFields,
				CompatibilityModeNASDAQStocks: s.CompatibilityModeNASDAQStocks, SendInstrumentTimezone: s.SendInstrumentTimezone,
				SendForexDataInCompatibilityMode: s.SendForexDataInCompatibilityMode, MaintainAndResubmitOrdersOnReconnect: s.MaintainAndResubmitOrdersOnReconnect,
				HistoricalDataMaxSize: s.HistoricalDataMaxSize, AutoReportNettingEventContractTrades: s.AutoReportNettingEventContractTrades,
				OptionExerciseRequestType: s.OptionExerciseRequestType, AllowLocalhostOnly: s.AllowLocalhostOnly,
				TrustedIPs: slices.Clone(s.TrustedIPs),
			}
		}
	}
	if config.Orders != nil {
		result.Orders = &TWSOrdersConfig{}
		if config.Orders.SmartRouting != nil {
			s := config.Orders.SmartRouting
			result.Orders.SmartRouting = &TWSOrdersSmartRoutingConfig{
				SeekPriceImprovement: s.SeekPriceImprovement, PreOpenReroute: s.PreOpenReroute,
				DoNotRouteToDarkPools: s.DoNotRouteToDarkPools, DefaultAlgorithm: s.DefaultAlgorithm,
			}
		}
	}
	return result
}

func optionalClientID(value *int) *ClientID {
	if value == nil {
		return nil
	}
	return new(protocolIDFromInt[ClientID](*value))
}
