package ibkr

// TWSConfig is the configuration exposed by TWS or IB Gateway. Optional
// scalar pointers distinguish an explicitly configured zero value from an
// omitted future or unavailable setting.
type TWSConfig struct {
	LockAndExit *TWSLockAndExitConfig
	Messages    []TWSMessageConfig
	API         *TWSAPIConfig
	Orders      *TWSOrdersConfig
}

type TWSLockAndExitConfig struct {
	AutoLogoffTime   *string
	AutoLogoffPeriod *string
	AutoLogoffType   *string
}

type TWSMessageConfig struct {
	ID            *int
	Title         *string
	Message       *string
	DefaultAction *string
	Enabled       *bool
}

type TWSAPIConfig struct {
	Precautions *TWSAPIPrecautionsConfig
	Settings    *TWSAPISettingsConfig
}

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
	MasterClientID                          *int
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

type TWSOrdersConfig struct {
	SmartRouting *TWSOrdersSmartRoutingConfig
}

type TWSOrdersSmartRoutingConfig struct {
	SeekPriceImprovement  *bool
	PreOpenReroute        *bool
	DoNotRouteToDarkPools *bool
	DefaultAlgorithm      *string
}
