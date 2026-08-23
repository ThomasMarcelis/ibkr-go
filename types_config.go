package ibkr

// TWSConfig is the configuration exposed by TWS or IB Gateway. Every optional
// scalar is a pointer: nil means the field was omitted or unavailable, while a
// non-nil pointer preserves an explicit zero, false, or empty value.
type TWSConfig struct {
	LockAndExit *TWSLockAndExitConfig
	Messages    []TWSMessageConfig
	API         *TWSAPIConfig
	Orders      *TWSOrdersConfig
}

// TWSLockAndExitConfig contains automatic logoff settings.
type TWSLockAndExitConfig struct {
	AutoLogoffTime   *string
	AutoLogoffPeriod *string
	AutoLogoffType   *string
}

// TWSMessageConfig describes one configurable TWS confirmation message.
type TWSMessageConfig struct {
	ID            *int
	Title         *string
	Message       *string
	DefaultAction *string
	Enabled       *bool
}

// TWSAPIConfig groups API precautions and general API settings.
type TWSAPIConfig struct {
	Precautions *TWSAPIPrecautionsConfig
	Settings    *TWSAPISettingsConfig
}

// TWSAPIPrecautionsConfig reports which API order warnings TWS bypasses.
type TWSAPIPrecautionsConfig struct {
	BypassOrderPrecautions            *bool
	BypassBondWarning                 *bool
	BypassNegativeYieldConfirmation   *bool
	BypassCalledBondWarning           *bool
	BypassSameActionPairTradeWarning  *bool
	BypassFlaggedAccountsWarning      *bool
	BypassPriceBasedVolatilityWarning *bool
	BypassRedirectOrderWarning        *bool
	BypassNoOverfillProtection        *bool
	BypassRouteMarketableToBBO        *bool
}

// TWSAPISettingsConfig contains the general TWS socket API settings.
type TWSAPISettingsConfig struct {
	ReadOnlyAPI                             *bool
	TotalQuantityForMutualFunds             *bool
	DownloadOpenOrdersOnConnection          *bool
	IncludeVirtualFXPositions               *bool
	PrepareDailyPnL                         *bool
	SendStatusUpdatesForVolatilityOrders    *bool
	EncodeAPIMessages                       *string
	SocketPort                              *int
	UseNegativeAutoRange                    *bool
	CreateAPIMessageLogFile                 *bool
	IncludeMarketDataInLogFile              *bool
	ExposeTradingScheduleToAPI              *bool
	SplitInsuredDepositFromCashBalance      *bool
	SendZeroPositionsForTodayOnly           *bool
	LetAPIAccountRequestsSwitchSubscription *bool
	UseAccountGroupsWithAllocationMethods   *bool
	LoggingLevel                            *string
	MasterClientID                          *ClientID
	BulkDataTimeout                         *int
	ComponentExchangeSeparator              *string
	ShowForexDataInOneTenthPips             *bool
	AllowForexTradingInOneTenthPips         *bool
	RoundAccountValuesToNearestWholeNumber  *bool
	SendMarketDataInLotsForUSStocks         *bool
	ShowAdvancedOrderRejectInUI             *bool
	RejectMessagesAboveMaxRate              *bool
	MaintainConnectionOnIncorrectFields     *bool
	CompatibilityModeNASDAQStocks           *bool
	SendInstrumentTimezone                  *string
	SendForexDataInCompatibilityMode        *bool
	MaintainAndResubmitOrdersOnReconnect    *bool
	HistoricalDataMaxSize                   *int
	AutoReportNettingEventContractTrades    *bool
	OptionExerciseRequestType               *string
	AllowLocalhostOnly                      *bool
	TrustedIPs                              []string
}

// TWSOrdersConfig groups TWS order-handling settings.
type TWSOrdersConfig struct {
	SmartRouting *TWSOrdersSmartRoutingConfig
}

// TWSOrdersSmartRoutingConfig contains SMART-routing preferences.
type TWSOrdersSmartRoutingConfig struct {
	SeekPriceImprovement  *bool
	PreOpenReroute        *bool
	DoNotRouteToDarkPools *bool
	DefaultAlgorithm      *string
}
