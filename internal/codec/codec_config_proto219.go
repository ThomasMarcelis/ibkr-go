package codec

import (
	"fmt"

	"github.com/ThomasMarcelis/ibkr-go/v2/internal/protocol"
)

type ConfigRequest struct {
	ReqID int
}

func (m ConfigRequest) encodeWire(sv int) ([]string, error) {
	if sv < protocol.MinServerVersionConfig {
		return nil, fmt.Errorf("codec: config requires server_version %d", protocol.MinServerVersionConfig)
	}
	return []string{itoa(protocol.OutReqConfig)}, nil
}

func (m ConfigRequest) encodeProto(sv int) ([]byte, error) {
	return appendProtoInt(nil, 1, m.ReqID, "config request id")
}

type ConfigResponse struct {
	ReqID       int
	LockAndExit *LockAndExitConfig
	Messages    []MessageConfig
	API         *APIConfig
	Orders      *OrdersConfig
}

func (m ConfigResponse) encodeWire(sv int) ([]string, error) {
	return []string{itoa(protocol.InConfig)}, nil
}

type LockAndExitConfig struct {
	AutoLogoffTime   *string
	AutoLogoffPeriod *string
	AutoLogoffType   *string
}

type MessageConfig struct {
	ID            *int
	Title         *string
	Message       *string
	DefaultAction *string
	Enabled       *bool
}

type APIConfig struct {
	Precautions *APIPrecautionsConfig
	Settings    *APISettingsConfig
}

type APIPrecautionsConfig struct {
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

type APISettingsConfig struct {
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

type OrdersConfig struct {
	SmartRouting *OrdersSmartRoutingConfig
}

type OrdersSmartRoutingConfig struct {
	SeekPriceImprovement  *bool
	PreOpenReroute        *bool
	DoNotRouteToDarkPools *bool
	DefaultAlgorithm      *string
}

func decodeConfigResponseProto(body []byte, sv int) ([]Message, error) {
	m := ConfigResponse{ReqID: -1}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return []Message{m}, nil
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return nil, protoFieldError("config response", number, err)
			}
			m.ReqID = decodeProtoInt32(value)
		case 2, 3, 4, 5:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return nil, protoFieldError("config response", number, err)
			}
			switch number {
			case 2:
				decoded, err := decodeLockAndExitConfigProto(value)
				if err != nil {
					return nil, err
				}
				m.LockAndExit = &decoded
			case 3:
				decoded, err := decodeMessageConfigProto(value)
				if err != nil {
					return nil, err
				}
				m.Messages = append(m.Messages, decoded)
			case 4:
				decoded, err := decodeAPIConfigProto(value)
				if err != nil {
					return nil, err
				}
				m.API = &decoded
			case 5:
				decoded, err := decodeOrdersConfigProto(value)
				if err != nil {
					return nil, err
				}
				m.Orders = &decoded
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return nil, protoFieldError("config response", number, err)
			}
		}
	}
}

func decodeLockAndExitConfigProto(body []byte) (LockAndExitConfig, error) {
	var m LockAndExitConfig
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil || !ok {
			return m, err
		}
		if number >= 1 && number <= 3 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return m, protoFieldError("lock-and-exit config", number, err)
			}
			decoded := string(value)
			switch number {
			case 1:
				m.AutoLogoffTime = &decoded
			case 2:
				m.AutoLogoffPeriod = &decoded
			case 3:
				m.AutoLogoffType = &decoded
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return m, protoFieldError("lock-and-exit config", number, err)
		}
	}
}

func decodeMessageConfigProto(body []byte) (MessageConfig, error) {
	var m MessageConfig
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil || !ok {
			return m, err
		}
		switch number {
		case 1:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return m, protoFieldError("message config", number, err)
			}
			m.ID = new(decodeProtoInt32(value))
		case 2, 3, 4:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return m, protoFieldError("message config", number, err)
			}
			decoded := string(value)
			switch number {
			case 2:
				m.Title = &decoded
			case 3:
				m.Message = &decoded
			case 4:
				m.DefaultAction = &decoded
			}
		case 5:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return m, protoFieldError("message config", number, err)
			}
			m.Enabled = new(value != 0)
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return m, protoFieldError("message config", number, err)
			}
		}
	}
}

func decodeAPIConfigProto(body []byte) (APIConfig, error) {
	var m APIConfig
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil || !ok {
			return m, err
		}
		if number == 1 || number == 2 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return m, protoFieldError("API config", number, err)
			}
			if number == 1 {
				decoded, err := decodeAPIPrecautionsConfigProto(value)
				if err != nil {
					return m, err
				}
				m.Precautions = &decoded
			} else {
				decoded, err := decodeAPISettingsConfigProto(value)
				if err != nil {
					return m, err
				}
				m.Settings = &decoded
			}
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return m, protoFieldError("API config", number, err)
		}
	}
}

func decodeAPIPrecautionsConfigProto(body []byte) (APIPrecautionsConfig, error) {
	var m APIPrecautionsConfig
	fields := []**bool{
		&m.BypassOrderPrecautions, &m.BypassBondWarning, &m.BypassNegativeYieldConfirmation,
		&m.BypassCalledBondWarning, &m.BypassSameActionPairTradeWarning, &m.BypassFlaggedAccountsWarning,
		&m.BypassPriceBasedVolatilityWarning, &m.BypassRedirectOrderWarning,
		&m.BypassNoOverfillProtection, &m.BypassRouteMarketableToBBO,
	}
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil || !ok {
			return m, err
		}
		if number >= 1 && number <= 10 {
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return m, protoFieldError("API precautions config", number, err)
			}
			*fields[number-1] = new(value != 0)
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return m, protoFieldError("API precautions config", number, err)
		}
	}
}

func decodeAPISettingsConfigProto(body []byte) (APISettingsConfig, error) {
	var m APISettingsConfig
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil || !ok {
			return m, err
		}
		switch number {
		case 1, 2, 3, 4, 5, 6, 9, 10, 11, 12, 13, 14, 15, 16, 21, 22, 23, 24, 25, 26, 27, 28, 30, 31, 33, 35:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return m, protoFieldError("API settings config", number, err)
			}
			decoded := new(value != 0)
			switch number {
			case 1:
				m.ReadOnlyAPI = decoded
			case 2:
				m.TotalQuantityForMutualFunds = decoded
			case 3:
				m.DownloadOpenOrdersOnConnection = decoded
			case 4:
				m.IncludeVirtualFXPositions = decoded
			case 5:
				m.PrepareDailyPnL = decoded
			case 6:
				m.SendStatusUpdatesForVolatilityOrders = decoded
			case 9:
				m.UseNegativeAutoRange = decoded
			case 10:
				m.CreateAPIMessageLogFile = decoded
			case 11:
				m.IncludeMarketDataInLogFile = decoded
			case 12:
				m.ExposeTradingScheduleToAPI = decoded
			case 13:
				m.SplitInsuredDepositFromCashBalance = decoded
			case 14:
				m.SendZeroPositionsForTodayOnly = decoded
			case 15:
				m.LetAPIAccountRequestsSwitchSubscription = decoded
			case 16:
				m.UseAccountGroupsWithAllocationMethods = decoded
			case 21:
				m.ShowForexDataInOneTenthPips = decoded
			case 22:
				m.AllowForexTradingInOneTenthPips = decoded
			case 23:
				m.RoundAccountValuesToNearestWholeNumber = decoded
			case 24:
				m.SendMarketDataInLotsForUSStocks = decoded
			case 25:
				m.ShowAdvancedOrderRejectInUI = decoded
			case 26:
				m.RejectMessagesAboveMaxRate = decoded
			case 27:
				m.MaintainConnectionOnIncorrectFields = decoded
			case 28:
				m.CompatibilityModeNASDAQStocks = decoded
			case 30:
				m.SendForexDataInCompatibilityMode = decoded
			case 31:
				m.MaintainAndResubmitOrdersOnReconnect = decoded
			case 33:
				m.AutoReportNettingEventContractTrades = decoded
			case 35:
				m.AllowLocalhostOnly = decoded
			}
		case 7, 17, 20, 29, 34, 36:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return m, protoFieldError("API settings config", number, err)
			}
			decoded := string(value)
			switch number {
			case 7:
				m.EncodeAPIMessages = &decoded
			case 17:
				m.LoggingLevel = &decoded
			case 20:
				m.ComponentExchangeSeparator = &decoded
			case 29:
				m.SendInstrumentTimezone = &decoded
			case 34:
				m.OptionExerciseRequestType = &decoded
			case 36:
				m.TrustedIPs = append(m.TrustedIPs, decoded)
			}
		case 8, 18, 19, 32:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return m, protoFieldError("API settings config", number, err)
			}
			decoded := new(decodeProtoInt32(value))
			switch number {
			case 8:
				m.SocketPort = decoded
			case 18:
				m.MasterClientID = decoded
			case 19:
				m.BulkDataTimeout = decoded
			case 32:
				m.HistoricalDataMaxSize = decoded
			}
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return m, protoFieldError("API settings config", number, err)
			}
		}
	}
}

func decodeOrdersConfigProto(body []byte) (OrdersConfig, error) {
	var m OrdersConfig
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil || !ok {
			return m, err
		}
		if number == 1 {
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return m, protoFieldError("orders config", number, err)
			}
			decoded, err := decodeOrdersSmartRoutingConfigProto(value)
			if err != nil {
				return m, err
			}
			m.SmartRouting = &decoded
		} else if err := skipProtoField(&body, number, typ); err != nil {
			return m, protoFieldError("orders config", number, err)
		}
	}
}

func decodeOrdersSmartRoutingConfigProto(body []byte) (OrdersSmartRoutingConfig, error) {
	var m OrdersSmartRoutingConfig
	for {
		number, typ, ok, err := consumeProtoTag(&body)
		if err != nil || !ok {
			return m, err
		}
		switch number {
		case 1, 2, 3:
			value, err := consumeProtoVarint(&body, typ)
			if err != nil {
				return m, protoFieldError("orders smart-routing config", number, err)
			}
			decoded := new(value != 0)
			switch number {
			case 1:
				m.SeekPriceImprovement = decoded
			case 2:
				m.PreOpenReroute = decoded
			case 3:
				m.DoNotRouteToDarkPools = decoded
			}
		case 4:
			value, err := consumeProtoBytes(&body, typ)
			if err != nil {
				return m, protoFieldError("orders smart-routing config", number, err)
			}
			m.DefaultAlgorithm = new(string(value))
		default:
			if err := skipProtoField(&body, number, typ); err != nil {
				return m, protoFieldError("orders smart-routing config", number, err)
			}
		}
	}
}
