//go:build ibkr_sdk && cgo && linux

#include "ibkr_adapter.h"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <climits>
#include <condition_variable>
#include <cstdlib>
#include <cstring>
#include <deque>
#include <exception>
#include <map>
#include <memory>
#include <mutex>
#include <set>
#include <sstream>
#include <string>
#include <thread>
#include <utility>
#include <vector>

#include "DefaultEWrapper.h"
#include "CommissionAndFeesReport.h"
#include "Contract.h"
#include "ContractCondition.h"
#include "Decimal.h"
#include "DepthMktDataDescription.h"
#include "EClientSocket.h"
#include "EReader.h"
#include "EReaderOSSignal.h"
#include "Execution.h"
#include "ExecutionCondition.h"
#include "FamilyCode.h"
#include "HistogramEntry.h"
#include "HistoricalSession.h"
#include "HistoricalTick.h"
#include "HistoricalTickBidAsk.h"
#include "HistoricalTickLast.h"
#include "NewsProvider.h"
#include "MarginCondition.h"
#include "OperatorCondition.h"
#include "Order.h"
#include "OrderCancel.h"
#include "OrderState.h"
#include "PercentChangeCondition.h"
#include "PriceIncrement.h"
#include "PriceCondition.h"
#include "ScannerSubscription.h"
#include "SoftDollarTier.h"
#include "TimeCondition.h"
#include "VolumeCondition.h"
#include "WshEventData.h"
#include "bar.h"
#include "google/protobuf/stubs/common.h"

#ifndef IBKR_SDK_API_VERSION
#define IBKR_SDK_API_VERSION unknown
#endif

#define IBKR_STRINGIFY_VALUE(value) #value
#define IBKR_STRINGIFY(value) IBKR_STRINGIFY_VALUE(value)

namespace {

constexpr const char* kAdapterABIVersion = "1";

char* copy_string(const std::string& value) {
	char* out = static_cast<char*>(std::malloc(value.size() + 1));
	if (!out) {
		return nullptr;
	}
	std::memcpy(out, value.c_str(), value.size() + 1);
	return out;
}

void set_c_string(char** target, const std::string& value) {
	if (!target) {
		return;
	}
	*target = copy_string(value);
}

void set_error(ibkr_error* error, const std::string& operation, const std::string& message) {
	if (!error) {
		return;
	}
	ibkr_error_clear(error);
	set_c_string(&error->operation, operation);
	set_c_string(&error->message, message);
}

void set_error(ibkr_error* error, const std::string& operation, const std::exception& err) {
	set_error(error, operation, err.what());
}

struct SecDefOptParamsEvent {
	std::string exchange;
	int underlyingConID = 0;
	std::string tradingClass;
	std::string multiplier;
	std::vector<std::string> expirations;
	std::vector<double> strikes;
};

struct TickOptionComputationEvent {
	int tickType = 0;
	int tickAttrib = 0;
	std::string impliedVol;
	std::string delta;
	std::string optPrice;
	std::string pvDividend;
	std::string gamma;
	std::string vega;
	std::string theta;
	std::string undPrice;
};

struct ScannerDataEvent {
	int rank = 0;
	Contract contract;
	std::string distance;
	std::string benchmark;
	std::string projection;
	std::string legsStr;
};

struct HistoricalBarEvent {
	std::string time;
	std::string open;
	std::string high;
	std::string low;
	std::string close;
	std::string volume;
	std::string wap;
	std::string count;
};

struct HistoricalScheduleEvent {
	std::string startDateTime;
	std::string endDateTime;
	std::string timeZone;
	std::vector<HistoricalSession> sessions;
};

struct HistoricalTickEvent {
	std::string time;
	std::string price;
	std::string size;
};

struct HistoricalTickBidAskEvent {
	int tickAttrib = 0;
	std::string time;
	std::string bidPrice;
	std::string askPrice;
	std::string bidSize;
	std::string askSize;
};

struct HistoricalTickLastEvent {
	int tickAttrib = 0;
	std::string time;
	std::string price;
	std::string size;
	std::string exchange;
	std::string specialConditions;
};

struct TickByTickEvent {
	int tickType = 0;
	std::string time;
	std::string price;
	std::string size;
	std::string exchange;
	std::string specialConditions;
	std::string bidPrice;
	std::string askPrice;
	std::string bidSize;
	std::string askSize;
	std::string midPoint;
	int tickAttribLast = 0;
	int tickAttribBidAsk = 0;
};

struct MarketDepthEvent {
	int position = 0;
	int operation = 0;
	int side = 0;
	std::string price;
	std::string size;
};

struct MarketDepthL2Event {
	int position = 0;
	std::string marketMaker;
	int operation = 0;
	int side = 0;
	std::string price;
	std::string size;
	bool isSmartDepth = false;
};

struct TickPriceEvent {
	int tickType = 0;
	std::string price;
	std::string size;
	int attrMask = 0;
};

struct TickSizeEvent {
	int tickType = 0;
	std::string size;
};

struct TickValueEvent {
	int tickType = 0;
	std::string value;
};

struct TickReqParamsEvent {
	std::string minTick;
	std::string bboExchange;
	int snapshotPermissions = 0;
};

struct ComboLegEvent {
	int conID = 0;
	int ratio = 0;
	std::string action;
	std::string exchange;
	std::string openClose;
	std::string shortSaleSlot;
	std::string designatedLocation;
	std::string exemptCode;
};

struct TagValueEvent {
	std::string tag;
	std::string value;
};

struct OrderConditionEvent {
	int type = 0;
	std::string conjunction;
	int conID = 0;
	std::string exchange;
	int operatorValue = 0;
	std::string value;
	int triggerMethod = 0;
	std::string secType;
	std::string symbol;
};

struct OpenOrderEvent {
	long long orderID = 0;
	Contract contract;
	std::string action;
	std::string quantity;
	std::string orderType;
	std::string lmtPrice;
	std::string auxPrice;
	std::string tif;
	std::string ocaGroup;
	std::string account;
	std::string openClose;
	std::string origin;
	std::string orderRef;
	std::string clientID;
	std::string permID;
	std::string outsideRTH;
	std::string hidden;
	std::string discretionAmt;
	std::string goodAfterTime;
	std::vector<ComboLegEvent> comboLegs;
	std::vector<std::string> orderComboLegPrices;
	std::vector<TagValueEvent> smartComboRouting;
	std::string algoStrategy;
	std::vector<TagValueEvent> algoParams;
	std::vector<OrderConditionEvent> conditions;
	std::string conditionsIgnoreRTH;
	std::string conditionsCancelOrder;
	std::string status;
	std::string initMarginBefore;
	std::string maintMarginBefore;
	std::string equityWithLoanBefore;
	std::string initMarginChange;
	std::string maintMarginChange;
	std::string equityWithLoanChange;
	std::string initMarginAfter;
	std::string maintMarginAfter;
	std::string equityWithLoanAfter;
	std::string commission;
	std::string minCommission;
	std::string maxCommission;
	std::string commissionCurrency;
	std::string warningText;
	std::string filled;
	std::string remaining;
	std::string parentID;
};

struct CompletedOrderEvent {
	Contract contract;
	std::string action;
	std::string orderType;
	std::string status;
	std::string quantity;
	std::string filled;
	std::string remaining;
};

struct OrderStatusEvent {
	long long orderID = 0;
	std::string status;
	std::string filled;
	std::string remaining;
	std::string avgFillPrice;
	std::string permID;
	std::string parentID;
	std::string lastFillPrice;
	std::string clientID;
	std::string whyHeld;
	std::string mktCapPrice;
};

struct ExecutionDetailEvent {
	long long orderID = 0;
	std::string execID;
	std::string account;
	std::string symbol;
	std::string side;
	std::string shares;
	std::string price;
	std::string time;
};

struct CommissionReportEvent {
	std::string execID;
	std::string commission;
	std::string currency;
	std::string realizedPNL;
};

struct AdapterEvent {
	int kind = 0;
	int reqID = 0;
	int serverVersion = 0;
	long long integerValue = 0;
	std::string text;

	std::string account;
	std::string tag;
	std::string value;
	std::string currency;
	std::string modelCode;

	long long orderID = 0;
	long long errorTime = 0;
	int code = 0;
	int messageType = 0;
	std::string advancedOrderRejectJSON;
	std::string source;

	Contract contract;
	std::string marketName;
	std::string minTick;
	std::string longName;
	std::string timeZoneID;
	std::string position;
	std::string avgCost;
	std::string marketPrice;
	std::string marketValue;
	std::string unrealizedPNL;
	std::string realizedPNL;
	std::string dailyPNL;
	TickOptionComputationEvent tickOptionComputation;
	std::vector<FamilyCode> familyCodes;
	std::vector<DepthMktDataDescription> depthExchanges;
	std::vector<NewsProvider> newsProviders;
	std::vector<SoftDollarTier> softDollarTiers;
	std::vector<ContractDescription> symbolSamples;
	int marketRuleID = 0;
	std::vector<PriceIncrement> priceIncrements;
	std::vector<SecDefOptParamsEvent> secDefOptParams;
	SmartComponentsMap smartComponents;
	HistogramDataVector histogramData;
	std::vector<ScannerDataEvent> scannerData;
	HistoricalBarEvent historicalBar;
	HistoricalBarEvent realTimeBar;
	TickByTickEvent tickByTick;
	MarketDepthEvent marketDepth;
	MarketDepthL2Event marketDepthL2;
	TickPriceEvent tickPrice;
	TickSizeEvent tickSize;
	TickValueEvent tickGeneric;
	TickValueEvent tickString;
	TickReqParamsEvent tickReqParams;
	OpenOrderEvent openOrder;
	CompletedOrderEvent completedOrder;
	OrderStatusEvent orderStatus;
	ExecutionDetailEvent executionDetail;
	CommissionReportEvent commissionReport;
	HistoricalScheduleEvent historicalSchedule;
	std::vector<HistoricalTickEvent> historicalTicks;
	std::vector<HistoricalTickBidAskEvent> historicalTicksBidAsk;
	std::vector<HistoricalTickLastEvent> historicalTicksLast;
};

void set_c_contract(ibkr_contract& out, const Contract& contract) {
	out.con_id = contract.conId;
	out.symbol = copy_string(contract.symbol);
	out.sec_type = copy_string(contract.secType);
	out.expiry = copy_string(contract.lastTradeDateOrContractMonth);
	out.strike = copy_string(contract.strike == UNSET_DOUBLE ? "" : std::to_string(contract.strike));
	out.right = copy_string(contract.right);
	out.multiplier = copy_string(contract.multiplier);
	out.exchange = copy_string(contract.exchange);
	out.currency = copy_string(contract.currency);
	out.local_symbol = copy_string(contract.localSymbol);
	out.trading_class = copy_string(contract.tradingClass);
	out.primary_exchange = copy_string(contract.primaryExchange);
}

void free_c_contract(ibkr_contract& contract) {
	std::free(contract.symbol);
	std::free(contract.sec_type);
	std::free(contract.expiry);
	std::free(contract.strike);
	std::free(contract.right);
	std::free(contract.multiplier);
	std::free(contract.exchange);
	std::free(contract.currency);
	std::free(contract.local_symbol);
	std::free(contract.trading_class);
	std::free(contract.primary_exchange);
}

std::string decimal_to_string(Decimal value) {
	if (value == UNSET_DECIMAL) {
		return "";
	}
	return DecimalFunctions::decimalToString(value);
}

std::string double_to_string(double value) {
	if (value == UNSET_DOUBLE) {
		return "";
	}
	std::ostringstream out;
	out.precision(17);
	out << value;
	return out.str();
}

std::string bool_to_string(bool value) {
	return value ? "1" : "0";
}

std::string decimal_difference_to_string(Decimal left, Decimal right) {
	if (left == UNSET_DECIMAL || right == UNSET_DECIMAL) {
		return "";
	}
	return decimal_to_string(DecimalFunctions::sub(left, right));
}

std::vector<ComboLegEvent> combo_legs_from_sdk(const Contract::ComboLegListSPtr& legs) {
	std::vector<ComboLegEvent> out;
	if (!legs) {
		return out;
	}
	out.reserve(legs->size());
	for (const auto& leg : *legs) {
		if (!leg) {
			continue;
		}
		ComboLegEvent row;
		row.conID = leg->conId;
		row.ratio = leg->ratio;
		row.action = leg->action;
		row.exchange = leg->exchange;
		row.openClose = std::to_string(leg->openClose);
		row.shortSaleSlot = std::to_string(leg->shortSaleSlot);
		row.designatedLocation = leg->designatedLocation;
		row.exemptCode = std::to_string(leg->exemptCode);
		out.push_back(std::move(row));
	}
	return out;
}

std::vector<std::string> order_combo_leg_prices_from_sdk(const Order::OrderComboLegListSPtr& legs) {
	std::vector<std::string> out;
	if (!legs) {
		return out;
	}
	out.reserve(legs->size());
	for (const auto& leg : *legs) {
		if (!leg) {
			continue;
		}
		out.push_back(double_to_string(leg->price));
	}
	return out;
}

std::vector<TagValueEvent> tag_values_from_sdk(const TagValueListSPtr& values) {
	std::vector<TagValueEvent> out;
	if (!values) {
		return out;
	}
	out.reserve(values->size());
	for (const auto& value : *values) {
		if (!value) {
			continue;
		}
		out.push_back(TagValueEvent{value->tag, value->value});
	}
	return out;
}

OrderConditionEvent order_condition_from_sdk(OrderCondition* condition) {
	OrderConditionEvent out;
	if (!condition) {
		return out;
	}
	out.type = static_cast<int>(condition->type());
	out.conjunction = condition->conjunctionConnection() ? "a" : "o";
	if (auto operatorCondition = dynamic_cast<OperatorCondition*>(condition)) {
		out.operatorValue = operatorCondition->isMore() ? 2 : 1;
	}
	if (auto contractCondition = dynamic_cast<ContractCondition*>(condition)) {
		out.conID = contractCondition->conId();
		out.exchange = contractCondition->exchange();
	}
	switch (condition->type()) {
	case OrderCondition::Price:
		if (auto price = dynamic_cast<PriceCondition*>(condition)) {
			out.value = double_to_string(price->price());
			out.triggerMethod = static_cast<int>(price->triggerMethod());
		}
		break;
	case OrderCondition::Time:
		if (auto time = dynamic_cast<TimeCondition*>(condition)) {
			out.value = time->time();
		}
		break;
	case OrderCondition::Margin:
		if (auto margin = dynamic_cast<MarginCondition*>(condition)) {
			out.value = std::to_string(margin->percent());
		}
		break;
	case OrderCondition::Execution:
		if (auto execution = dynamic_cast<ExecutionCondition*>(condition)) {
			out.exchange = execution->exchange();
			out.secType = execution->secType();
			out.symbol = execution->symbol();
		}
		break;
	case OrderCondition::Volume:
		if (auto volume = dynamic_cast<VolumeCondition*>(condition)) {
			out.value = std::to_string(volume->volume());
		}
		break;
	case OrderCondition::PercentChange:
		if (auto percent = dynamic_cast<PercentChangeCondition*>(condition)) {
			out.value = double_to_string(percent->changePercent());
		}
		break;
	}
	return out;
}

std::vector<OrderConditionEvent> order_conditions_from_sdk(const std::vector<std::shared_ptr<OrderCondition>>& conditions) {
	std::vector<OrderConditionEvent> out;
	out.reserve(conditions.size());
	for (const auto& condition : conditions) {
		if (!condition) {
			continue;
		}
		out.push_back(order_condition_from_sdk(condition.get()));
	}
	return out;
}

OpenOrderEvent open_order_from_sdk(int orderID, const Contract& contract, const Order& order, const OrderState& orderState) {
	OpenOrderEvent out;
	out.orderID = orderID;
	out.contract = contract;
	out.action = order.action;
	out.quantity = decimal_to_string(order.totalQuantity);
	out.orderType = order.orderType;
	out.lmtPrice = double_to_string(order.lmtPrice);
	out.auxPrice = double_to_string(order.auxPrice);
	out.tif = order.tif;
	out.ocaGroup = order.ocaGroup;
	out.account = order.account;
	out.openClose = order.openClose;
	out.origin = std::to_string(static_cast<int>(order.origin));
	out.orderRef = order.orderRef;
	out.clientID = std::to_string(order.clientId);
	out.permID = std::to_string(order.permId);
	out.outsideRTH = bool_to_string(order.outsideRth);
	out.hidden = bool_to_string(order.hidden);
	out.discretionAmt = double_to_string(order.discretionaryAmt);
	out.goodAfterTime = order.goodAfterTime;
	out.comboLegs = combo_legs_from_sdk(contract.comboLegs);
	out.orderComboLegPrices = order_combo_leg_prices_from_sdk(order.orderComboLegs);
	out.smartComboRouting = tag_values_from_sdk(order.smartComboRoutingParams);
	out.algoStrategy = order.algoStrategy;
	out.algoParams = tag_values_from_sdk(order.algoParams);
	out.conditions = order_conditions_from_sdk(order.conditions);
	out.conditionsIgnoreRTH = bool_to_string(order.conditionsIgnoreRth);
	out.conditionsCancelOrder = bool_to_string(order.conditionsCancelOrder);
	out.status = orderState.status;
	out.initMarginBefore = orderState.initMarginBefore;
	out.maintMarginBefore = orderState.maintMarginBefore;
	out.equityWithLoanBefore = orderState.equityWithLoanBefore;
	out.initMarginChange = orderState.initMarginChange;
	out.maintMarginChange = orderState.maintMarginChange;
	out.equityWithLoanChange = orderState.equityWithLoanChange;
	out.initMarginAfter = orderState.initMarginAfter;
	out.maintMarginAfter = orderState.maintMarginAfter;
	out.equityWithLoanAfter = orderState.equityWithLoanAfter;
	out.commission = double_to_string(orderState.commissionAndFees);
	out.minCommission = double_to_string(orderState.minCommissionAndFees);
	out.maxCommission = double_to_string(orderState.maxCommissionAndFees);
	out.commissionCurrency = orderState.commissionAndFeesCurrency;
	out.warningText = orderState.warningText;
	out.filled = decimal_to_string(order.filledQuantity);
	out.remaining = decimal_difference_to_string(order.totalQuantity, order.filledQuantity);
	out.parentID = std::to_string(order.parentId);
	return out;
}

CompletedOrderEvent completed_order_from_sdk(const Contract& contract, const Order& order, const OrderState& orderState) {
	CompletedOrderEvent out;
	out.contract = contract;
	out.action = order.action;
	out.orderType = order.orderType;
	out.status = orderState.completedStatus.empty() ? orderState.status : orderState.completedStatus;
	out.quantity = decimal_to_string(order.totalQuantity);
	out.filled = decimal_to_string(order.filledQuantity);
	out.remaining = decimal_difference_to_string(order.totalQuantity, order.filledQuantity);
	return out;
}

HistoricalBarEvent historical_bar_from_sdk(const Bar& bar) {
	HistoricalBarEvent out;
	out.time = bar.time;
	out.open = double_to_string(bar.open);
	out.high = double_to_string(bar.high);
	out.low = double_to_string(bar.low);
	out.close = double_to_string(bar.close);
	out.volume = decimal_to_string(bar.volume);
	out.wap = decimal_to_string(bar.wap);
	out.count = bar.count == INT_MAX ? "" : std::to_string(bar.count);
	return out;
}

int tick_attrib_bid_ask(const TickAttribBidAsk& attrib) {
	int mask = 0;
	if (attrib.askPastHigh) {
		mask |= 1;
	}
	if (attrib.bidPastLow) {
		mask |= 2;
	}
	return mask;
}

int tick_attrib_last(const TickAttribLast& attrib) {
	int mask = 0;
	if (attrib.pastLimit) {
		mask |= 1;
	}
	if (attrib.unreported) {
		mask |= 2;
	}
	return mask;
}

int tick_attrib_price(const TickAttrib& attrib) {
	int mask = 0;
	if (attrib.canAutoExecute) {
		mask |= 1;
	}
	if (attrib.pastLimit) {
		mask |= 2;
	}
	if (attrib.preOpen) {
		mask |= 4;
	}
	return mask;
}

double parse_double(const char* value) {
	if (!value || std::strlen(value) == 0) {
		return 0;
	}
	return std::strtod(value, nullptr);
}

int parse_optional_int(const char* value) {
	if (!value || std::strlen(value) == 0) {
		return INT_MAX;
	}
	return static_cast<int>(std::strtol(value, nullptr, 10));
}

std::string c_string(const char* value) {
	return value ? value : "";
}

bool parse_bool_string(const char* value, bool defaultValue = false) {
	if (!value || std::strlen(value) == 0) {
		return defaultValue;
	}
	return std::strcmp(value, "1") == 0 || std::strcmp(value, "true") == 0 || std::strcmp(value, "TRUE") == 0;
}

int parse_int_string(const char* value, int defaultValue = 0) {
	if (!value || std::strlen(value) == 0) {
		return defaultValue;
	}
	return static_cast<int>(std::strtol(value, nullptr, 10));
}

long long parse_long_long_string(const char* value, long long defaultValue = 0) {
	if (!value || std::strlen(value) == 0) {
		return defaultValue;
	}
	return std::strtoll(value, nullptr, 10);
}

double parse_optional_double(const char* value) {
	if (!value || std::strlen(value) == 0) {
		return UNSET_DOUBLE;
	}
	return std::strtod(value, nullptr);
}

Decimal parse_optional_decimal(const char* value) {
	if (!value || std::strlen(value) == 0) {
		return UNSET_DECIMAL;
	}
	return DecimalFunctions::stringToDecimal(value);
}

UsePriceMmgtAlgo parse_use_price_mgmt_algo(const char* value) {
	if (!value || std::strlen(value) == 0) {
		return UsePriceMmgtAlgo::DEFAULT;
	}
	return parse_bool_string(value) ? UsePriceMmgtAlgo::USE : UsePriceMmgtAlgo::DONT_USE;
}

Contract contract_from_c(const ibkr_contract* in) {
	Contract contract;
	if (!in) {
		return contract;
	}
	contract.conId = in->con_id;
	contract.symbol = in->symbol ? in->symbol : "";
	contract.secType = in->sec_type ? in->sec_type : "";
	contract.lastTradeDateOrContractMonth = in->expiry ? in->expiry : "";
	contract.strike = parse_double(in->strike);
	contract.right = in->right ? in->right : "";
	contract.multiplier = in->multiplier ? in->multiplier : "";
	contract.exchange = in->exchange ? in->exchange : "";
	contract.currency = in->currency ? in->currency : "";
	contract.localSymbol = in->local_symbol ? in->local_symbol : "";
	contract.tradingClass = in->trading_class ? in->trading_class : "";
	contract.primaryExchange = in->primary_exchange ? in->primary_exchange : "";
	return contract;
}

Contract::ComboLegListSPtr combo_legs_to_sdk(const ibkr_combo_leg_event* rows, std::size_t count) {
	if (!rows || count == 0) {
		return Contract::ComboLegListSPtr();
	}
	Contract::ComboLegListSPtr legs(new Contract::ComboLegList());
	legs->reserve(count);
	for (std::size_t i = 0; i < count; i++) {
		ComboLegSPtr leg(new ComboLeg());
		leg->conId = rows[i].con_id;
		leg->ratio = rows[i].ratio;
		leg->action = c_string(rows[i].action);
		leg->exchange = c_string(rows[i].exchange);
		leg->openClose = parse_int_string(rows[i].open_close);
		leg->shortSaleSlot = parse_int_string(rows[i].short_sale_slot);
		leg->designatedLocation = c_string(rows[i].designated_location);
		leg->exemptCode = parse_int_string(rows[i].exempt_code, -1);
		legs->push_back(leg);
	}
	return legs;
}

Order::OrderComboLegListSPtr order_combo_legs_to_sdk(char** prices, std::size_t count) {
	if (!prices || count == 0) {
		return Order::OrderComboLegListSPtr();
	}
	Order::OrderComboLegListSPtr legs(new Order::OrderComboLegList());
	legs->reserve(count);
	for (std::size_t i = 0; i < count; i++) {
		OrderComboLegSPtr leg(new OrderComboLeg());
		leg->price = parse_optional_double(prices[i]);
		legs->push_back(leg);
	}
	return legs;
}

TagValueListSPtr tag_values_to_sdk(const ibkr_tag_value_event* rows, std::size_t count) {
	if (!rows || count == 0) {
		return TagValueListSPtr();
	}
	TagValueListSPtr values(new TagValueList());
	values->reserve(count);
	for (std::size_t i = 0; i < count; i++) {
		values->push_back(TagValueSPtr(new TagValue(c_string(rows[i].tag), c_string(rows[i].value))));
	}
	return values;
}

std::shared_ptr<OrderCondition> order_condition_to_sdk(const ibkr_order_condition_event& row) {
	OrderCondition* raw = OrderCondition::create(static_cast<OrderCondition::OrderConditionType>(row.condition_type));
	if (!raw) {
		return std::shared_ptr<OrderCondition>();
	}
	std::shared_ptr<OrderCondition> condition(raw);
	condition->conjunctionConnection(c_string(row.conjunction) != "o");
	if (auto operatorCondition = dynamic_cast<OperatorCondition*>(condition.get())) {
		operatorCondition->isMore(row.operator_value == 2);
	}
	if (auto contractCondition = dynamic_cast<ContractCondition*>(condition.get())) {
		contractCondition->conId(row.con_id);
		contractCondition->exchange(c_string(row.exchange));
	}
	switch (condition->type()) {
	case OrderCondition::Price:
		if (auto price = dynamic_cast<PriceCondition*>(condition.get())) {
			price->price(parse_double(row.value));
			price->triggerMethod(row.trigger_method);
		}
		break;
	case OrderCondition::Time:
		if (auto time = dynamic_cast<TimeCondition*>(condition.get())) {
			time->time(c_string(row.value));
		}
		break;
	case OrderCondition::Margin:
		if (auto margin = dynamic_cast<MarginCondition*>(condition.get())) {
			margin->percent(parse_int_string(row.value));
		}
		break;
	case OrderCondition::Execution:
		if (auto execution = dynamic_cast<ExecutionCondition*>(condition.get())) {
			execution->exchange(c_string(row.exchange));
			execution->secType(c_string(row.sec_type));
			execution->symbol(c_string(row.symbol));
		}
		break;
	case OrderCondition::Volume:
		if (auto volume = dynamic_cast<VolumeCondition*>(condition.get())) {
			volume->volume(parse_int_string(row.value));
		}
		break;
	case OrderCondition::PercentChange:
		if (auto percent = dynamic_cast<PercentChangeCondition*>(condition.get())) {
			percent->changePercent(parse_double(row.value));
		}
		break;
	}
	return condition;
}

std::vector<std::shared_ptr<OrderCondition>> order_conditions_to_sdk(const ibkr_order_condition_event* rows, std::size_t count) {
	std::vector<std::shared_ptr<OrderCondition>> conditions;
	if (!rows || count == 0) {
		return conditions;
	}
	conditions.reserve(count);
	for (std::size_t i = 0; i < count; i++) {
		std::shared_ptr<OrderCondition> condition = order_condition_to_sdk(rows[i]);
		if (condition) {
			conditions.push_back(condition);
		}
	}
	return conditions;
}

Order order_from_c(const ibkr_place_order_request* in) {
	Order order;
	if (!in) {
		return order;
	}
	order.orderId = static_cast<int>(in->order_id);
	order.action = c_string(in->action);
	order.totalQuantity = parse_optional_decimal(in->total_quantity);
	order.orderType = c_string(in->order_type);
	order.lmtPrice = parse_optional_double(in->lmt_price);
	order.auxPrice = parse_optional_double(in->aux_price);
	order.tif = c_string(in->tif);
	order.activeStartTime = c_string(in->active_start_time);
	order.activeStopTime = c_string(in->active_stop_time);
	order.ocaGroup = c_string(in->oca_group);
	order.ocaType = parse_int_string(in->oca_type);
	order.orderRef = c_string(in->order_ref);
	order.transmit = parse_bool_string(in->transmit, true);
	order.parentId = static_cast<int>(parse_long_long_string(in->parent_id));
	order.blockOrder = parse_bool_string(in->block_order);
	order.sweepToFill = parse_bool_string(in->sweep_to_fill);
	order.displaySize = parse_int_string(in->display_size);
	order.triggerMethod = parse_int_string(in->trigger_method);
	order.outsideRth = parse_bool_string(in->outside_rth);
	order.hidden = parse_bool_string(in->hidden);
	order.goodAfterTime = c_string(in->good_after_time);
	order.goodTillDate = c_string(in->good_till_date);
	order.rule80A = c_string(in->rule80a);
	order.allOrNone = parse_bool_string(in->all_or_none);
	order.minQty = parse_optional_int(in->min_qty);
	order.percentOffset = parse_optional_double(in->percent_offset);
	order.overridePercentageConstraints = parse_bool_string(in->override_percentage_constraints);
	order.trailStopPrice = parse_optional_double(in->trail_stop_price);
	order.trailingPercent = parse_optional_double(in->trailing_percent);
	order.faGroup = c_string(in->fa_group);
	order.faMethod = c_string(in->fa_method);
	order.faPercentage = c_string(in->fa_percentage);
	order.openClose = c_string(in->open_close);
	order.origin = static_cast<Origin>(parse_int_string(in->origin));
	order.shortSaleSlot = parse_int_string(in->short_sale_slot);
	order.designatedLocation = c_string(in->designated_location);
	order.exemptCode = parse_int_string(in->exempt_code, -1);
	order.discretionaryAmt = parse_double(in->discretionary_amt);
	order.optOutSmartRouting = parse_bool_string(in->opt_out_smart_routing);
	order.auctionStrategy = parse_int_string(in->auction_strategy);
	order.startingPrice = parse_optional_double(in->starting_price);
	order.stockRefPrice = parse_optional_double(in->stock_ref_price);
	order.delta = parse_optional_double(in->delta);
	order.stockRangeLower = parse_optional_double(in->stock_range_lower);
	order.stockRangeUpper = parse_optional_double(in->stock_range_upper);
	order.volatility = parse_optional_double(in->volatility);
	order.volatilityType = parse_optional_int(in->volatility_type);
	order.deltaNeutralOrderType = c_string(in->delta_neutral_order_type);
	order.deltaNeutralAuxPrice = parse_optional_double(in->delta_neutral_aux_price);
	order.continuousUpdate = parse_bool_string(in->continuous_update);
	order.referencePriceType = parse_optional_int(in->reference_price_type);
	order.scaleInitLevelSize = parse_optional_int(in->scale_init_level_size);
	order.scaleSubsLevelSize = parse_optional_int(in->scale_subs_level_size);
	order.scalePriceIncrement = parse_optional_double(in->scale_price_increment);
	order.scaleTable = c_string(in->scale_table);
	order.hedgeType = c_string(in->hedge_type);
	order.hedgeParam = c_string(in->hedge_param);
	order.account = c_string(in->account);
	order.settlingFirm = c_string(in->settling_firm);
	order.clearingAccount = c_string(in->clearing_account);
	order.clearingIntent = c_string(in->clearing_intent);
	order.algoStrategy = c_string(in->algo_strategy);
	order.algoParams = tag_values_to_sdk(in->algo_params, in->algo_params_count);
	order.smartComboRoutingParams = tag_values_to_sdk(in->smart_combo_routing_params, in->smart_combo_routing_params_count);
	order.algoId = c_string(in->algo_id);
	order.whatIf = parse_bool_string(in->what_if);
	order.notHeld = parse_bool_string(in->not_held);
	order.solicited = parse_bool_string(in->solicited);
	order.modelCode = c_string(in->model_code);
	order.orderComboLegs = order_combo_legs_to_sdk(in->order_combo_leg_prices, in->order_combo_leg_prices_count);
	order.conditions = order_conditions_to_sdk(in->conditions, in->conditions_count);
	order.conditionsCancelOrder = parse_bool_string(in->conditions_cancel_order);
	order.conditionsIgnoreRth = parse_bool_string(in->conditions_ignore_rth);
	order.extOperator = c_string(in->ext_operator);
	order.softDollarTier = SoftDollarTier(c_string(in->soft_dollar_name), c_string(in->soft_dollar_value), "");
	order.cashQty = parse_optional_double(in->cash_qty);
	order.mifid2DecisionMaker = c_string(in->mifid2_decision_maker);
	order.mifid2DecisionAlgo = c_string(in->mifid2_decision_algo);
	order.mifid2ExecutionTrader = c_string(in->mifid2_execution_trader);
	order.mifid2ExecutionAlgo = c_string(in->mifid2_execution_algo);
	order.dontUseAutoPriceForHedge = parse_bool_string(in->dont_use_auto_price_for_hedge);
	order.isOmsContainer = parse_bool_string(in->is_oms_container);
	order.discretionaryUpToLimitPrice = parse_bool_string(in->discretionary_up_to_limit_price);
	order.usePriceMgmtAlgo = parse_use_price_mgmt_algo(in->use_price_mgmt_algo);
	order.duration = parse_optional_int(in->duration);
	order.postToAts = parse_optional_int(in->post_to_ats);
	order.autoCancelParent = parse_bool_string(in->auto_cancel_parent);
	order.advancedErrorOverride = c_string(in->advanced_error_override);
	order.manualOrderTime = c_string(in->manual_order_time);
	order.customerAccount = c_string(in->customer_account);
	order.professionalCustomer = parse_bool_string(in->professional_customer);
	order.includeOvernight = parse_bool_string(in->include_overnight);
	order.manualOrderIndicator = parse_optional_int(in->manual_order_indicator);
	order.imbalanceOnly = parse_bool_string(in->imbalance_only);
	order.randomizeSize = parse_bool_string(in->randomize_size);
	order.randomizePrice = parse_bool_string(in->randomize_price);
	order.adjustedOrderType = c_string(in->adjusted_order_type);
	order.triggerPrice = parse_optional_double(in->trigger_price);
	order.adjustedStopPrice = parse_optional_double(in->adjusted_stop_price);
	order.adjustedStopLimitPrice = parse_optional_double(in->adjusted_stop_limit_price);
	order.adjustedTrailingAmount = parse_optional_double(in->adjusted_trailing_amount);
	order.adjustableTrailingUnit = parse_optional_int(in->adjustable_trailing_unit);
	order.lmtPriceOffset = parse_optional_double(in->lmt_price_offset);
	return order;
}

void set_c_combo_leg(ibkr_combo_leg_event& out, const ComboLegEvent& leg) {
	out.con_id = leg.conID;
	out.ratio = leg.ratio;
	out.action = copy_string(leg.action);
	out.exchange = copy_string(leg.exchange);
	out.open_close = copy_string(leg.openClose);
	out.short_sale_slot = copy_string(leg.shortSaleSlot);
	out.designated_location = copy_string(leg.designatedLocation);
	out.exempt_code = copy_string(leg.exemptCode);
}

void free_c_combo_leg(ibkr_combo_leg_event& leg) {
	std::free(leg.action);
	std::free(leg.exchange);
	std::free(leg.open_close);
	std::free(leg.short_sale_slot);
	std::free(leg.designated_location);
	std::free(leg.exempt_code);
}

void set_c_tag_value(ibkr_tag_value_event& out, const TagValueEvent& value) {
	out.tag = copy_string(value.tag);
	out.value = copy_string(value.value);
}

void free_c_tag_value(ibkr_tag_value_event& value) {
	std::free(value.tag);
	std::free(value.value);
}

void set_c_order_condition(ibkr_order_condition_event& out, const OrderConditionEvent& condition) {
	out.condition_type = condition.type;
	out.conjunction = copy_string(condition.conjunction);
	out.con_id = condition.conID;
	out.exchange = copy_string(condition.exchange);
	out.operator_value = condition.operatorValue;
	out.value = copy_string(condition.value);
	out.trigger_method = condition.triggerMethod;
	out.sec_type = copy_string(condition.secType);
	out.symbol = copy_string(condition.symbol);
}

void free_c_order_condition(ibkr_order_condition_event& condition) {
	std::free(condition.conjunction);
	std::free(condition.exchange);
	std::free(condition.value);
	std::free(condition.sec_type);
	std::free(condition.symbol);
}

void set_c_open_order(ibkr_open_order_event& out, const OpenOrderEvent& order) {
	out.order_id = order.orderID;
	set_c_contract(out.contract, order.contract);
	out.action = copy_string(order.action);
	out.quantity = copy_string(order.quantity);
	out.order_type = copy_string(order.orderType);
	out.lmt_price = copy_string(order.lmtPrice);
	out.aux_price = copy_string(order.auxPrice);
	out.tif = copy_string(order.tif);
	out.oca_group = copy_string(order.ocaGroup);
	out.account = copy_string(order.account);
	out.open_close = copy_string(order.openClose);
	out.origin = copy_string(order.origin);
	out.order_ref = copy_string(order.orderRef);
	out.client_id = copy_string(order.clientID);
	out.perm_id = copy_string(order.permID);
	out.outside_rth = copy_string(order.outsideRTH);
	out.hidden = copy_string(order.hidden);
	out.discretion_amt = copy_string(order.discretionAmt);
	out.good_after_time = copy_string(order.goodAfterTime);
	if (!order.comboLegs.empty()) {
		out.combo_legs = static_cast<ibkr_combo_leg_event*>(std::calloc(order.comboLegs.size(), sizeof(ibkr_combo_leg_event)));
		if (out.combo_legs) {
			out.combo_legs_count = order.comboLegs.size();
			for (std::size_t i = 0; i < order.comboLegs.size(); i++) {
				set_c_combo_leg(out.combo_legs[i], order.comboLegs[i]);
			}
		}
	}
	if (!order.orderComboLegPrices.empty()) {
		out.order_combo_leg_prices = static_cast<char**>(std::calloc(order.orderComboLegPrices.size(), sizeof(char*)));
		if (out.order_combo_leg_prices) {
			out.order_combo_leg_prices_count = order.orderComboLegPrices.size();
			for (std::size_t i = 0; i < order.orderComboLegPrices.size(); i++) {
				out.order_combo_leg_prices[i] = copy_string(order.orderComboLegPrices[i]);
			}
		}
	}
	if (!order.smartComboRouting.empty()) {
		out.smart_combo_routing = static_cast<ibkr_tag_value_event*>(std::calloc(order.smartComboRouting.size(), sizeof(ibkr_tag_value_event)));
		if (out.smart_combo_routing) {
			out.smart_combo_routing_count = order.smartComboRouting.size();
			for (std::size_t i = 0; i < order.smartComboRouting.size(); i++) {
				set_c_tag_value(out.smart_combo_routing[i], order.smartComboRouting[i]);
			}
		}
	}
	out.algo_strategy = copy_string(order.algoStrategy);
	if (!order.algoParams.empty()) {
		out.algo_params = static_cast<ibkr_tag_value_event*>(std::calloc(order.algoParams.size(), sizeof(ibkr_tag_value_event)));
		if (out.algo_params) {
			out.algo_params_count = order.algoParams.size();
			for (std::size_t i = 0; i < order.algoParams.size(); i++) {
				set_c_tag_value(out.algo_params[i], order.algoParams[i]);
			}
		}
	}
	if (!order.conditions.empty()) {
		out.conditions = static_cast<ibkr_order_condition_event*>(std::calloc(order.conditions.size(), sizeof(ibkr_order_condition_event)));
		if (out.conditions) {
			out.conditions_count = order.conditions.size();
			for (std::size_t i = 0; i < order.conditions.size(); i++) {
				set_c_order_condition(out.conditions[i], order.conditions[i]);
			}
		}
	}
	out.conditions_ignore_rth = copy_string(order.conditionsIgnoreRTH);
	out.conditions_cancel_order = copy_string(order.conditionsCancelOrder);
	out.status = copy_string(order.status);
	out.init_margin_before = copy_string(order.initMarginBefore);
	out.maint_margin_before = copy_string(order.maintMarginBefore);
	out.equity_with_loan_before = copy_string(order.equityWithLoanBefore);
	out.init_margin_change = copy_string(order.initMarginChange);
	out.maint_margin_change = copy_string(order.maintMarginChange);
	out.equity_with_loan_change = copy_string(order.equityWithLoanChange);
	out.init_margin_after = copy_string(order.initMarginAfter);
	out.maint_margin_after = copy_string(order.maintMarginAfter);
	out.equity_with_loan_after = copy_string(order.equityWithLoanAfter);
	out.commission = copy_string(order.commission);
	out.min_commission = copy_string(order.minCommission);
	out.max_commission = copy_string(order.maxCommission);
	out.commission_currency = copy_string(order.commissionCurrency);
	out.warning_text = copy_string(order.warningText);
	out.filled = copy_string(order.filled);
	out.remaining = copy_string(order.remaining);
	out.parent_id = copy_string(order.parentID);
}

void free_c_open_order(ibkr_open_order_event& order) {
	free_c_contract(order.contract);
	std::free(order.action);
	std::free(order.quantity);
	std::free(order.order_type);
	std::free(order.lmt_price);
	std::free(order.aux_price);
	std::free(order.tif);
	std::free(order.oca_group);
	std::free(order.account);
	std::free(order.open_close);
	std::free(order.origin);
	std::free(order.order_ref);
	std::free(order.client_id);
	std::free(order.perm_id);
	std::free(order.outside_rth);
	std::free(order.hidden);
	std::free(order.discretion_amt);
	std::free(order.good_after_time);
	for (std::size_t i = 0; i < order.combo_legs_count; i++) {
		free_c_combo_leg(order.combo_legs[i]);
	}
	std::free(order.combo_legs);
	for (std::size_t i = 0; i < order.order_combo_leg_prices_count; i++) {
		std::free(order.order_combo_leg_prices[i]);
	}
	std::free(order.order_combo_leg_prices);
	for (std::size_t i = 0; i < order.smart_combo_routing_count; i++) {
		free_c_tag_value(order.smart_combo_routing[i]);
	}
	std::free(order.smart_combo_routing);
	std::free(order.algo_strategy);
	for (std::size_t i = 0; i < order.algo_params_count; i++) {
		free_c_tag_value(order.algo_params[i]);
	}
	std::free(order.algo_params);
	for (std::size_t i = 0; i < order.conditions_count; i++) {
		free_c_order_condition(order.conditions[i]);
	}
	std::free(order.conditions);
	std::free(order.conditions_ignore_rth);
	std::free(order.conditions_cancel_order);
	std::free(order.status);
	std::free(order.init_margin_before);
	std::free(order.maint_margin_before);
	std::free(order.equity_with_loan_before);
	std::free(order.init_margin_change);
	std::free(order.maint_margin_change);
	std::free(order.equity_with_loan_change);
	std::free(order.init_margin_after);
	std::free(order.maint_margin_after);
	std::free(order.equity_with_loan_after);
	std::free(order.commission);
	std::free(order.min_commission);
	std::free(order.max_commission);
	std::free(order.commission_currency);
	std::free(order.warning_text);
	std::free(order.filled);
	std::free(order.remaining);
	std::free(order.parent_id);
}

void set_c_completed_order(ibkr_completed_order_event& out, const CompletedOrderEvent& order) {
	set_c_contract(out.contract, order.contract);
	out.action = copy_string(order.action);
	out.order_type = copy_string(order.orderType);
	out.status = copy_string(order.status);
	out.quantity = copy_string(order.quantity);
	out.filled = copy_string(order.filled);
	out.remaining = copy_string(order.remaining);
}

void free_c_completed_order(ibkr_completed_order_event& order) {
	free_c_contract(order.contract);
	std::free(order.action);
	std::free(order.order_type);
	std::free(order.status);
	std::free(order.quantity);
	std::free(order.filled);
	std::free(order.remaining);
}

ibkr_event to_c_event(const AdapterEvent& event) {
	ibkr_event out{};
	out.kind = event.kind;
	out.req_id = event.reqID;
	out.server_version = event.serverVersion;
	out.integer_value = event.integerValue;
	out.text = copy_string(event.text);
	if (event.kind == IBKR_EVENT_HISTORICAL_NEWS) {
		out.historical_news.time = copy_string(event.marketName);
		out.historical_news.provider_code = copy_string(event.account);
		out.historical_news.article_id = copy_string(event.tag);
		out.historical_news.headline = copy_string(event.value);
	}
	out.account_summary.req_id = event.reqID;
	out.account_summary.account = copy_string(event.account);
	out.account_summary.tag = copy_string(event.tag);
	out.account_summary.value = copy_string(event.value);
	out.account_summary.currency = copy_string(event.currency);
	out.api_error.req_id = event.reqID;
	out.api_error.order_id = event.orderID;
	out.api_error.error_time = event.errorTime;
	out.api_error.code = event.code;
	out.api_error.message = copy_string(event.text);
	out.api_error.advanced_order_reject_json = copy_string(event.advancedOrderRejectJSON);
	out.contract_details.req_id = event.reqID;
	set_c_contract(out.contract_details.contract, event.contract);
	out.contract_details.market_name = copy_string(event.marketName);
	out.contract_details.min_tick = copy_string(event.minTick);
	out.contract_details.long_name = copy_string(event.longName);
	out.contract_details.time_zone_id = copy_string(event.timeZoneID);
	out.position.account = copy_string(event.account);
	set_c_contract(out.position.contract, event.contract);
	out.position.position = copy_string(event.position);
	out.position.avg_cost = copy_string(event.avgCost);
	out.account_value.key = copy_string(event.tag);
	out.account_value.value = copy_string(event.value);
	out.account_value.currency = copy_string(event.currency);
	out.account_value.account = copy_string(event.account);
	out.portfolio.account = copy_string(event.account);
	set_c_contract(out.portfolio.contract, event.contract);
	out.portfolio.position = copy_string(event.position);
	out.portfolio.market_price = copy_string(event.marketPrice);
	out.portfolio.market_value = copy_string(event.marketValue);
	out.portfolio.avg_cost = copy_string(event.avgCost);
	out.portfolio.unrealized_pnl = copy_string(event.unrealizedPNL);
	out.portfolio.realized_pnl = copy_string(event.realizedPNL);
	out.account_update_multi.account = copy_string(event.account);
	out.account_update_multi.model_code = copy_string(event.modelCode);
	out.account_update_multi.key = copy_string(event.tag);
	out.account_update_multi.value = copy_string(event.value);
	out.account_update_multi.currency = copy_string(event.currency);
	out.position_multi.account = copy_string(event.account);
	out.position_multi.model_code = copy_string(event.modelCode);
	set_c_contract(out.position_multi.contract, event.contract);
	out.position_multi.position = copy_string(event.position);
	out.position_multi.avg_cost = copy_string(event.avgCost);
	out.pnl.daily_pnl = copy_string(event.dailyPNL);
	out.pnl.unrealized_pnl = copy_string(event.unrealizedPNL);
	out.pnl.realized_pnl = copy_string(event.realizedPNL);
	out.pnl_single.position = copy_string(event.position);
	out.pnl_single.daily_pnl = copy_string(event.dailyPNL);
	out.pnl_single.unrealized_pnl = copy_string(event.unrealizedPNL);
	out.pnl_single.realized_pnl = copy_string(event.realizedPNL);
	out.pnl_single.value = copy_string(event.value);
	set_c_open_order(out.open_order, event.openOrder);
	set_c_completed_order(out.completed_order, event.completedOrder);
	out.news_bulletin.msg_id = static_cast<int>(event.integerValue);
	out.news_bulletin.msg_type = event.messageType;
	out.news_bulletin.headline = copy_string(event.text);
	out.news_bulletin.source = copy_string(event.source);
	out.historical_bar.time = copy_string(event.historicalBar.time);
	out.historical_bar.open = copy_string(event.historicalBar.open);
	out.historical_bar.high = copy_string(event.historicalBar.high);
	out.historical_bar.low = copy_string(event.historicalBar.low);
	out.historical_bar.close = copy_string(event.historicalBar.close);
	out.historical_bar.volume = copy_string(event.historicalBar.volume);
	out.historical_bar.wap = copy_string(event.historicalBar.wap);
	out.historical_bar.count = copy_string(event.historicalBar.count);
	out.real_time_bar.time = copy_string(event.realTimeBar.time);
	out.real_time_bar.open = copy_string(event.realTimeBar.open);
	out.real_time_bar.high = copy_string(event.realTimeBar.high);
	out.real_time_bar.low = copy_string(event.realTimeBar.low);
	out.real_time_bar.close = copy_string(event.realTimeBar.close);
	out.real_time_bar.volume = copy_string(event.realTimeBar.volume);
	out.real_time_bar.wap = copy_string(event.realTimeBar.wap);
	out.real_time_bar.count = copy_string(event.realTimeBar.count);
	out.tick_by_tick.tick_type = event.tickByTick.tickType;
	out.tick_by_tick.time = copy_string(event.tickByTick.time);
	out.tick_by_tick.price = copy_string(event.tickByTick.price);
	out.tick_by_tick.size = copy_string(event.tickByTick.size);
	out.tick_by_tick.exchange = copy_string(event.tickByTick.exchange);
	out.tick_by_tick.special_conditions = copy_string(event.tickByTick.specialConditions);
	out.tick_by_tick.bid_price = copy_string(event.tickByTick.bidPrice);
	out.tick_by_tick.ask_price = copy_string(event.tickByTick.askPrice);
	out.tick_by_tick.bid_size = copy_string(event.tickByTick.bidSize);
	out.tick_by_tick.ask_size = copy_string(event.tickByTick.askSize);
	out.tick_by_tick.midpoint = copy_string(event.tickByTick.midPoint);
	out.tick_by_tick.tick_attrib_last = event.tickByTick.tickAttribLast;
	out.tick_by_tick.tick_attrib_bid_ask = event.tickByTick.tickAttribBidAsk;
	out.market_depth.position = event.marketDepth.position;
	out.market_depth.operation = event.marketDepth.operation;
	out.market_depth.side = event.marketDepth.side;
	out.market_depth.price = copy_string(event.marketDepth.price);
	out.market_depth.size = copy_string(event.marketDepth.size);
	out.market_depth_l2.position = event.marketDepthL2.position;
	out.market_depth_l2.market_maker = copy_string(event.marketDepthL2.marketMaker);
	out.market_depth_l2.operation = event.marketDepthL2.operation;
	out.market_depth_l2.side = event.marketDepthL2.side;
	out.market_depth_l2.price = copy_string(event.marketDepthL2.price);
	out.market_depth_l2.size = copy_string(event.marketDepthL2.size);
	out.market_depth_l2.is_smart_depth = event.marketDepthL2.isSmartDepth ? 1 : 0;
	out.tick_price.tick_type = event.tickPrice.tickType;
	out.tick_price.price = copy_string(event.tickPrice.price);
	out.tick_price.size = copy_string(event.tickPrice.size);
	out.tick_price.attr_mask = event.tickPrice.attrMask;
	out.tick_size.tick_type = event.tickSize.tickType;
	out.tick_size.size = copy_string(event.tickSize.size);
	out.tick_generic.tick_type = event.tickGeneric.tickType;
	out.tick_generic.value = copy_string(event.tickGeneric.value);
	out.tick_string.tick_type = event.tickString.tickType;
	out.tick_string.value = copy_string(event.tickString.value);
	out.tick_req_params.min_tick = copy_string(event.tickReqParams.minTick);
	out.tick_req_params.bbo_exchange = copy_string(event.tickReqParams.bboExchange);
	out.tick_req_params.snapshot_permissions = event.tickReqParams.snapshotPermissions;
	out.order_status.order_id = event.orderStatus.orderID;
	out.order_status.status = copy_string(event.orderStatus.status);
	out.order_status.filled = copy_string(event.orderStatus.filled);
	out.order_status.remaining = copy_string(event.orderStatus.remaining);
	out.order_status.avg_fill_price = copy_string(event.orderStatus.avgFillPrice);
	out.order_status.perm_id = copy_string(event.orderStatus.permID);
	out.order_status.parent_id = copy_string(event.orderStatus.parentID);
	out.order_status.last_fill_price = copy_string(event.orderStatus.lastFillPrice);
	out.order_status.client_id = copy_string(event.orderStatus.clientID);
	out.order_status.why_held = copy_string(event.orderStatus.whyHeld);
	out.order_status.mkt_cap_price = copy_string(event.orderStatus.mktCapPrice);
	out.execution_detail.order_id = event.executionDetail.orderID;
	out.execution_detail.exec_id = copy_string(event.executionDetail.execID);
	out.execution_detail.account = copy_string(event.executionDetail.account);
	out.execution_detail.symbol = copy_string(event.executionDetail.symbol);
	out.execution_detail.side = copy_string(event.executionDetail.side);
	out.execution_detail.shares = copy_string(event.executionDetail.shares);
	out.execution_detail.price = copy_string(event.executionDetail.price);
	out.execution_detail.time = copy_string(event.executionDetail.time);
	out.commission_report.exec_id = copy_string(event.commissionReport.execID);
	out.commission_report.commission = copy_string(event.commissionReport.commission);
	out.commission_report.currency = copy_string(event.commissionReport.currency);
	out.commission_report.realized_pnl = copy_string(event.commissionReport.realizedPNL);
	out.historical_schedule.start_date_time = copy_string(event.historicalSchedule.startDateTime);
	out.historical_schedule.end_date_time = copy_string(event.historicalSchedule.endDateTime);
	out.historical_schedule.time_zone = copy_string(event.historicalSchedule.timeZone);
	if (!event.historicalSchedule.sessions.empty()) {
		out.historical_schedule.sessions = static_cast<ibkr_historical_schedule_session_event*>(std::calloc(event.historicalSchedule.sessions.size(), sizeof(ibkr_historical_schedule_session_event)));
		if (out.historical_schedule.sessions) {
			out.historical_schedule.sessions_count = event.historicalSchedule.sessions.size();
			for (std::size_t i = 0; i < event.historicalSchedule.sessions.size(); i++) {
				const HistoricalSession& session = event.historicalSchedule.sessions[i];
				out.historical_schedule.sessions[i].start_date_time = copy_string(session.startDateTime);
				out.historical_schedule.sessions[i].end_date_time = copy_string(session.endDateTime);
				out.historical_schedule.sessions[i].ref_date = copy_string(session.refDate);
			}
		}
	}
	if (!event.historicalTicks.empty()) {
		out.historical_ticks = static_cast<ibkr_historical_tick_event*>(std::calloc(event.historicalTicks.size(), sizeof(ibkr_historical_tick_event)));
		if (out.historical_ticks) {
			out.historical_ticks_count = event.historicalTicks.size();
			for (std::size_t i = 0; i < event.historicalTicks.size(); i++) {
				const HistoricalTickEvent& tick = event.historicalTicks[i];
				out.historical_ticks[i].time = copy_string(tick.time);
				out.historical_ticks[i].price = copy_string(tick.price);
				out.historical_ticks[i].size = copy_string(tick.size);
			}
		}
	}
	if (!event.historicalTicksBidAsk.empty()) {
		out.historical_ticks_bid_ask = static_cast<ibkr_historical_tick_bid_ask_event*>(std::calloc(event.historicalTicksBidAsk.size(), sizeof(ibkr_historical_tick_bid_ask_event)));
		if (out.historical_ticks_bid_ask) {
			out.historical_ticks_bid_ask_count = event.historicalTicksBidAsk.size();
			for (std::size_t i = 0; i < event.historicalTicksBidAsk.size(); i++) {
				const HistoricalTickBidAskEvent& tick = event.historicalTicksBidAsk[i];
				out.historical_ticks_bid_ask[i].tick_attrib = tick.tickAttrib;
				out.historical_ticks_bid_ask[i].time = copy_string(tick.time);
				out.historical_ticks_bid_ask[i].bid_price = copy_string(tick.bidPrice);
				out.historical_ticks_bid_ask[i].ask_price = copy_string(tick.askPrice);
				out.historical_ticks_bid_ask[i].bid_size = copy_string(tick.bidSize);
				out.historical_ticks_bid_ask[i].ask_size = copy_string(tick.askSize);
			}
		}
	}
	if (!event.historicalTicksLast.empty()) {
		out.historical_ticks_last = static_cast<ibkr_historical_tick_last_event*>(std::calloc(event.historicalTicksLast.size(), sizeof(ibkr_historical_tick_last_event)));
		if (out.historical_ticks_last) {
			out.historical_ticks_last_count = event.historicalTicksLast.size();
			for (std::size_t i = 0; i < event.historicalTicksLast.size(); i++) {
				const HistoricalTickLastEvent& tick = event.historicalTicksLast[i];
				out.historical_ticks_last[i].tick_attrib = tick.tickAttrib;
				out.historical_ticks_last[i].time = copy_string(tick.time);
				out.historical_ticks_last[i].price = copy_string(tick.price);
				out.historical_ticks_last[i].size = copy_string(tick.size);
				out.historical_ticks_last[i].exchange = copy_string(tick.exchange);
				out.historical_ticks_last[i].special_conditions = copy_string(tick.specialConditions);
			}
		}
	}
	out.tick_option_computation.tick_type = event.tickOptionComputation.tickType;
	out.tick_option_computation.tick_attrib = event.tickOptionComputation.tickAttrib;
	out.tick_option_computation.implied_vol = copy_string(event.tickOptionComputation.impliedVol);
	out.tick_option_computation.delta = copy_string(event.tickOptionComputation.delta);
	out.tick_option_computation.opt_price = copy_string(event.tickOptionComputation.optPrice);
	out.tick_option_computation.pv_dividend = copy_string(event.tickOptionComputation.pvDividend);
	out.tick_option_computation.gamma = copy_string(event.tickOptionComputation.gamma);
	out.tick_option_computation.vega = copy_string(event.tickOptionComputation.vega);
	out.tick_option_computation.theta = copy_string(event.tickOptionComputation.theta);
	out.tick_option_computation.und_price = copy_string(event.tickOptionComputation.undPrice);
	if (!event.familyCodes.empty()) {
		out.family_codes = static_cast<ibkr_family_code_event*>(std::calloc(event.familyCodes.size(), sizeof(ibkr_family_code_event)));
		if (out.family_codes) {
			out.family_codes_count = event.familyCodes.size();
			for (std::size_t i = 0; i < event.familyCodes.size(); i++) {
				out.family_codes[i].account_id = copy_string(event.familyCodes[i].accountID);
				out.family_codes[i].family_code = copy_string(event.familyCodes[i].familyCodeStr);
			}
		}
	}
	if (!event.depthExchanges.empty()) {
		out.depth_exchanges = static_cast<ibkr_depth_exchange_event*>(std::calloc(event.depthExchanges.size(), sizeof(ibkr_depth_exchange_event)));
		if (out.depth_exchanges) {
			out.depth_exchanges_count = event.depthExchanges.size();
			for (std::size_t i = 0; i < event.depthExchanges.size(); i++) {
				out.depth_exchanges[i].exchange = copy_string(event.depthExchanges[i].exchange);
				out.depth_exchanges[i].sec_type = copy_string(event.depthExchanges[i].secType);
				out.depth_exchanges[i].listing_exch = copy_string(event.depthExchanges[i].listingExch);
				out.depth_exchanges[i].service_data_type = copy_string(event.depthExchanges[i].serviceDataType);
				out.depth_exchanges[i].agg_group = event.depthExchanges[i].aggGroup;
			}
		}
	}
	if (!event.newsProviders.empty()) {
		out.news_providers = static_cast<ibkr_news_provider_event*>(std::calloc(event.newsProviders.size(), sizeof(ibkr_news_provider_event)));
		if (out.news_providers) {
			out.news_providers_count = event.newsProviders.size();
			for (std::size_t i = 0; i < event.newsProviders.size(); i++) {
				out.news_providers[i].code = copy_string(event.newsProviders[i].providerCode);
				out.news_providers[i].name = copy_string(event.newsProviders[i].providerName);
			}
		}
	}
	if (!event.softDollarTiers.empty()) {
		out.soft_dollar_tiers = static_cast<ibkr_soft_dollar_tier_event*>(std::calloc(event.softDollarTiers.size(), sizeof(ibkr_soft_dollar_tier_event)));
		if (out.soft_dollar_tiers) {
			out.soft_dollar_tiers_count = event.softDollarTiers.size();
			for (std::size_t i = 0; i < event.softDollarTiers.size(); i++) {
				out.soft_dollar_tiers[i].name = copy_string(event.softDollarTiers[i].name());
				out.soft_dollar_tiers[i].value = copy_string(event.softDollarTiers[i].val());
				out.soft_dollar_tiers[i].display_name = copy_string(event.softDollarTiers[i].displayName());
			}
		}
	}
	if (!event.symbolSamples.empty()) {
		out.symbol_samples = static_cast<ibkr_symbol_sample_event*>(std::calloc(event.symbolSamples.size(), sizeof(ibkr_symbol_sample_event)));
		if (out.symbol_samples) {
			out.symbol_samples_count = event.symbolSamples.size();
			for (std::size_t i = 0; i < event.symbolSamples.size(); i++) {
				const ContractDescription& sample = event.symbolSamples[i];
				out.symbol_samples[i].con_id = sample.contract.conId;
				out.symbol_samples[i].symbol = copy_string(sample.contract.symbol);
				out.symbol_samples[i].sec_type = copy_string(sample.contract.secType);
				out.symbol_samples[i].primary_exchange = copy_string(sample.contract.primaryExchange);
				out.symbol_samples[i].currency = copy_string(sample.contract.currency);
				out.symbol_samples[i].description = copy_string(sample.contract.description);
				out.symbol_samples[i].issuer_id = copy_string(sample.contract.issuerId);
				if (!sample.derivativeSecTypes.empty()) {
					out.symbol_samples[i].derivative_sec_types = static_cast<char**>(std::calloc(sample.derivativeSecTypes.size(), sizeof(char*)));
					if (out.symbol_samples[i].derivative_sec_types) {
						out.symbol_samples[i].derivative_sec_types_count = sample.derivativeSecTypes.size();
						for (std::size_t j = 0; j < sample.derivativeSecTypes.size(); j++) {
							out.symbol_samples[i].derivative_sec_types[j] = copy_string(sample.derivativeSecTypes[j]);
						}
					}
				}
			}
		}
	}
	out.market_rule_id = event.marketRuleID;
	if (!event.priceIncrements.empty()) {
		out.price_increments = static_cast<ibkr_price_increment_event*>(std::calloc(event.priceIncrements.size(), sizeof(ibkr_price_increment_event)));
		if (out.price_increments) {
			out.price_increments_count = event.priceIncrements.size();
			for (std::size_t i = 0; i < event.priceIncrements.size(); i++) {
				out.price_increments[i].low_edge = copy_string(double_to_string(event.priceIncrements[i].lowEdge));
				out.price_increments[i].increment = copy_string(double_to_string(event.priceIncrements[i].increment));
			}
		}
	}
	if (!event.secDefOptParams.empty()) {
		out.sec_def_opt_params = static_cast<ibkr_sec_def_opt_params_event*>(std::calloc(event.secDefOptParams.size(), sizeof(ibkr_sec_def_opt_params_event)));
		if (out.sec_def_opt_params) {
			out.sec_def_opt_params_count = event.secDefOptParams.size();
			for (std::size_t i = 0; i < event.secDefOptParams.size(); i++) {
				const SecDefOptParamsEvent& params = event.secDefOptParams[i];
				out.sec_def_opt_params[i].exchange = copy_string(params.exchange);
				out.sec_def_opt_params[i].underlying_con_id = params.underlyingConID;
				out.sec_def_opt_params[i].trading_class = copy_string(params.tradingClass);
				out.sec_def_opt_params[i].multiplier = copy_string(params.multiplier);
				if (!params.expirations.empty()) {
					out.sec_def_opt_params[i].expirations = static_cast<char**>(std::calloc(params.expirations.size(), sizeof(char*)));
					if (out.sec_def_opt_params[i].expirations) {
						out.sec_def_opt_params[i].expirations_count = params.expirations.size();
						for (std::size_t j = 0; j < params.expirations.size(); j++) {
							out.sec_def_opt_params[i].expirations[j] = copy_string(params.expirations[j]);
						}
					}
				}
				if (!params.strikes.empty()) {
					out.sec_def_opt_params[i].strikes = static_cast<char**>(std::calloc(params.strikes.size(), sizeof(char*)));
					if (out.sec_def_opt_params[i].strikes) {
						out.sec_def_opt_params[i].strikes_count = params.strikes.size();
						for (std::size_t j = 0; j < params.strikes.size(); j++) {
							out.sec_def_opt_params[i].strikes[j] = copy_string(double_to_string(params.strikes[j]));
						}
					}
				}
			}
		}
	}
	if (!event.smartComponents.empty()) {
		out.smart_components = static_cast<ibkr_smart_component_event*>(std::calloc(event.smartComponents.size(), sizeof(ibkr_smart_component_event)));
		if (out.smart_components) {
			out.smart_components_count = event.smartComponents.size();
			std::size_t i = 0;
			for (const auto& component : event.smartComponents) {
				const char exchangeLetter = std::get<1>(component.second);
				out.smart_components[i].bit_number = component.first;
				out.smart_components[i].exchange_name = copy_string(std::get<0>(component.second));
				out.smart_components[i].exchange_letter = copy_string(exchangeLetter == '\0' ? "" : std::string(1, exchangeLetter));
				i++;
			}
		}
	}
	if (!event.histogramData.empty()) {
		out.histogram_data = static_cast<ibkr_histogram_data_event*>(std::calloc(event.histogramData.size(), sizeof(ibkr_histogram_data_event)));
		if (out.histogram_data) {
			out.histogram_data_count = event.histogramData.size();
			for (std::size_t i = 0; i < event.histogramData.size(); i++) {
				out.histogram_data[i].price = copy_string(double_to_string(event.histogramData[i].price));
				out.histogram_data[i].size = copy_string(decimal_to_string(event.histogramData[i].size));
			}
		}
	}
	if (!event.scannerData.empty()) {
		out.scanner_data = static_cast<ibkr_scanner_data_event*>(std::calloc(event.scannerData.size(), sizeof(ibkr_scanner_data_event)));
		if (out.scanner_data) {
			out.scanner_data_count = event.scannerData.size();
			for (std::size_t i = 0; i < event.scannerData.size(); i++) {
				const ScannerDataEvent& entry = event.scannerData[i];
				out.scanner_data[i].rank = entry.rank;
				set_c_contract(out.scanner_data[i].contract, entry.contract);
				out.scanner_data[i].distance = copy_string(entry.distance);
				out.scanner_data[i].benchmark = copy_string(entry.benchmark);
				out.scanner_data[i].projection = copy_string(entry.projection);
				out.scanner_data[i].legs_str = copy_string(entry.legsStr);
			}
		}
	}
	return out;
}

void free_c_event(ibkr_event& event) {
	std::free(event.text);
	std::free(event.account_summary.account);
	std::free(event.account_summary.tag);
	std::free(event.account_summary.value);
	std::free(event.account_summary.currency);
	std::free(event.api_error.message);
	std::free(event.api_error.advanced_order_reject_json);
	free_c_contract(event.contract_details.contract);
	std::free(event.contract_details.market_name);
	std::free(event.contract_details.min_tick);
	std::free(event.contract_details.long_name);
	std::free(event.contract_details.time_zone_id);
	std::free(event.position.account);
	free_c_contract(event.position.contract);
	std::free(event.position.position);
	std::free(event.position.avg_cost);
	std::free(event.account_value.key);
	std::free(event.account_value.value);
	std::free(event.account_value.currency);
	std::free(event.account_value.account);
	std::free(event.portfolio.account);
	free_c_contract(event.portfolio.contract);
	std::free(event.portfolio.position);
	std::free(event.portfolio.market_price);
	std::free(event.portfolio.market_value);
	std::free(event.portfolio.avg_cost);
	std::free(event.portfolio.unrealized_pnl);
	std::free(event.portfolio.realized_pnl);
	std::free(event.account_update_multi.account);
	std::free(event.account_update_multi.model_code);
	std::free(event.account_update_multi.key);
	std::free(event.account_update_multi.value);
	std::free(event.account_update_multi.currency);
	std::free(event.position_multi.account);
	std::free(event.position_multi.model_code);
	free_c_contract(event.position_multi.contract);
	std::free(event.position_multi.position);
	std::free(event.position_multi.avg_cost);
	std::free(event.pnl.daily_pnl);
	std::free(event.pnl.unrealized_pnl);
	std::free(event.pnl.realized_pnl);
	std::free(event.pnl_single.position);
	std::free(event.pnl_single.daily_pnl);
	std::free(event.pnl_single.unrealized_pnl);
	std::free(event.pnl_single.realized_pnl);
	std::free(event.pnl_single.value);
	free_c_open_order(event.open_order);
	free_c_completed_order(event.completed_order);
	std::free(event.news_bulletin.headline);
	std::free(event.news_bulletin.source);
	std::free(event.historical_bar.time);
	std::free(event.historical_bar.open);
	std::free(event.historical_bar.high);
	std::free(event.historical_bar.low);
	std::free(event.historical_bar.close);
	std::free(event.historical_bar.volume);
	std::free(event.historical_bar.wap);
	std::free(event.historical_bar.count);
	std::free(event.real_time_bar.time);
	std::free(event.real_time_bar.open);
	std::free(event.real_time_bar.high);
	std::free(event.real_time_bar.low);
	std::free(event.real_time_bar.close);
	std::free(event.real_time_bar.volume);
	std::free(event.real_time_bar.wap);
	std::free(event.real_time_bar.count);
	std::free(event.tick_by_tick.time);
	std::free(event.tick_by_tick.price);
	std::free(event.tick_by_tick.size);
	std::free(event.tick_by_tick.exchange);
	std::free(event.tick_by_tick.special_conditions);
	std::free(event.tick_by_tick.bid_price);
	std::free(event.tick_by_tick.ask_price);
	std::free(event.tick_by_tick.bid_size);
	std::free(event.tick_by_tick.ask_size);
	std::free(event.tick_by_tick.midpoint);
	std::free(event.market_depth.price);
	std::free(event.market_depth.size);
	std::free(event.market_depth_l2.market_maker);
	std::free(event.market_depth_l2.price);
	std::free(event.market_depth_l2.size);
	std::free(event.tick_price.price);
	std::free(event.tick_price.size);
	std::free(event.tick_size.size);
	std::free(event.tick_generic.value);
	std::free(event.tick_string.value);
	std::free(event.tick_req_params.min_tick);
	std::free(event.tick_req_params.bbo_exchange);
	std::free(event.order_status.status);
	std::free(event.order_status.filled);
	std::free(event.order_status.remaining);
	std::free(event.order_status.avg_fill_price);
	std::free(event.order_status.perm_id);
	std::free(event.order_status.parent_id);
	std::free(event.order_status.last_fill_price);
	std::free(event.order_status.client_id);
	std::free(event.order_status.why_held);
	std::free(event.order_status.mkt_cap_price);
	std::free(event.execution_detail.exec_id);
	std::free(event.execution_detail.account);
	std::free(event.execution_detail.symbol);
	std::free(event.execution_detail.side);
	std::free(event.execution_detail.shares);
	std::free(event.execution_detail.price);
	std::free(event.execution_detail.time);
	std::free(event.commission_report.exec_id);
	std::free(event.commission_report.commission);
	std::free(event.commission_report.currency);
	std::free(event.commission_report.realized_pnl);
	std::free(event.historical_schedule.start_date_time);
	std::free(event.historical_schedule.end_date_time);
	std::free(event.historical_schedule.time_zone);
	for (std::size_t i = 0; i < event.historical_schedule.sessions_count; i++) {
		std::free(event.historical_schedule.sessions[i].start_date_time);
		std::free(event.historical_schedule.sessions[i].end_date_time);
		std::free(event.historical_schedule.sessions[i].ref_date);
	}
	std::free(event.historical_schedule.sessions);
	for (std::size_t i = 0; i < event.historical_ticks_count; i++) {
		std::free(event.historical_ticks[i].time);
		std::free(event.historical_ticks[i].price);
		std::free(event.historical_ticks[i].size);
	}
	std::free(event.historical_ticks);
	for (std::size_t i = 0; i < event.historical_ticks_bid_ask_count; i++) {
		std::free(event.historical_ticks_bid_ask[i].time);
		std::free(event.historical_ticks_bid_ask[i].bid_price);
		std::free(event.historical_ticks_bid_ask[i].ask_price);
		std::free(event.historical_ticks_bid_ask[i].bid_size);
		std::free(event.historical_ticks_bid_ask[i].ask_size);
	}
	std::free(event.historical_ticks_bid_ask);
	for (std::size_t i = 0; i < event.historical_ticks_last_count; i++) {
		std::free(event.historical_ticks_last[i].time);
		std::free(event.historical_ticks_last[i].price);
		std::free(event.historical_ticks_last[i].size);
		std::free(event.historical_ticks_last[i].exchange);
		std::free(event.historical_ticks_last[i].special_conditions);
	}
	std::free(event.historical_ticks_last);
	std::free(event.tick_option_computation.implied_vol);
	std::free(event.tick_option_computation.delta);
	std::free(event.tick_option_computation.opt_price);
	std::free(event.tick_option_computation.pv_dividend);
	std::free(event.tick_option_computation.gamma);
	std::free(event.tick_option_computation.vega);
	std::free(event.tick_option_computation.theta);
	std::free(event.tick_option_computation.und_price);
	for (std::size_t i = 0; i < event.family_codes_count; i++) {
		std::free(event.family_codes[i].account_id);
		std::free(event.family_codes[i].family_code);
	}
	std::free(event.family_codes);
	for (std::size_t i = 0; i < event.depth_exchanges_count; i++) {
		std::free(event.depth_exchanges[i].exchange);
		std::free(event.depth_exchanges[i].sec_type);
		std::free(event.depth_exchanges[i].listing_exch);
		std::free(event.depth_exchanges[i].service_data_type);
	}
	std::free(event.depth_exchanges);
	for (std::size_t i = 0; i < event.news_providers_count; i++) {
		std::free(event.news_providers[i].code);
		std::free(event.news_providers[i].name);
	}
	std::free(event.news_providers);
	for (std::size_t i = 0; i < event.soft_dollar_tiers_count; i++) {
		std::free(event.soft_dollar_tiers[i].name);
		std::free(event.soft_dollar_tiers[i].value);
		std::free(event.soft_dollar_tiers[i].display_name);
	}
	std::free(event.soft_dollar_tiers);
	for (std::size_t i = 0; i < event.symbol_samples_count; i++) {
		std::free(event.symbol_samples[i].symbol);
		std::free(event.symbol_samples[i].sec_type);
		std::free(event.symbol_samples[i].primary_exchange);
		std::free(event.symbol_samples[i].currency);
		for (std::size_t j = 0; j < event.symbol_samples[i].derivative_sec_types_count; j++) {
			std::free(event.symbol_samples[i].derivative_sec_types[j]);
		}
		std::free(event.symbol_samples[i].derivative_sec_types);
		std::free(event.symbol_samples[i].description);
		std::free(event.symbol_samples[i].issuer_id);
	}
	std::free(event.symbol_samples);
	for (std::size_t i = 0; i < event.price_increments_count; i++) {
		std::free(event.price_increments[i].low_edge);
		std::free(event.price_increments[i].increment);
	}
	std::free(event.price_increments);
	for (std::size_t i = 0; i < event.sec_def_opt_params_count; i++) {
		std::free(event.sec_def_opt_params[i].exchange);
		std::free(event.sec_def_opt_params[i].trading_class);
		std::free(event.sec_def_opt_params[i].multiplier);
		for (std::size_t j = 0; j < event.sec_def_opt_params[i].expirations_count; j++) {
			std::free(event.sec_def_opt_params[i].expirations[j]);
		}
		std::free(event.sec_def_opt_params[i].expirations);
		for (std::size_t j = 0; j < event.sec_def_opt_params[i].strikes_count; j++) {
			std::free(event.sec_def_opt_params[i].strikes[j]);
		}
		std::free(event.sec_def_opt_params[i].strikes);
	}
	std::free(event.sec_def_opt_params);
	for (std::size_t i = 0; i < event.smart_components_count; i++) {
		std::free(event.smart_components[i].exchange_name);
		std::free(event.smart_components[i].exchange_letter);
	}
	std::free(event.smart_components);
	std::free(event.historical_news.time);
	std::free(event.historical_news.provider_code);
	std::free(event.historical_news.article_id);
	std::free(event.historical_news.headline);
	for (std::size_t i = 0; i < event.histogram_data_count; i++) {
		std::free(event.histogram_data[i].price);
		std::free(event.histogram_data[i].size);
	}
	std::free(event.histogram_data);
	for (std::size_t i = 0; i < event.scanner_data_count; i++) {
		free_c_contract(event.scanner_data[i].contract);
		std::free(event.scanner_data[i].distance);
		std::free(event.scanner_data[i].benchmark);
		std::free(event.scanner_data[i].projection);
		std::free(event.scanner_data[i].legs_str);
	}
	std::free(event.scanner_data);
}

std::vector<std::string> split_accounts(const std::string& accountsList) {
	std::vector<std::string> accounts;
	std::string current;
	for (char ch : accountsList) {
		if (ch == ',') {
			if (!current.empty()) {
				accounts.push_back(current);
				current.clear();
			}
			continue;
		}
		current.push_back(ch);
	}
	if (!current.empty()) {
		accounts.push_back(current);
	}
	return accounts;
}

std::string protobuf_mode() {
#ifdef GOOGLE_PROTOBUF_VERSION
	return std::string("protobuf ") + IBKR_STRINGIFY(GOOGLE_PROTOBUF_VERSION);
#else
	return "protobuf unknown";
#endif
}

} // namespace

class Adapter {
public:
	class Wrapper final : public DefaultEWrapper {
	public:
		explicit Wrapper(Adapter& adapter) : adapter_(adapter) {}

		void nextValidId(int orderID) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_NEXT_VALID_ID;
			event.integerValue = orderID;
			adapter_.Push(std::move(event));
		}

		void managedAccounts(const std::string& accountsList) override {
			for (const auto& account : split_accounts(accountsList)) {
				AdapterEvent event;
				event.kind = IBKR_EVENT_MANAGED_ACCOUNTS;
				event.text = account;
				adapter_.Push(std::move(event));
			}
		}

		void currentTime(long long time) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_CURRENT_TIME;
			event.integerValue = time;
			adapter_.Push(std::move(event));
		}

		void currentTimeInMillis(time_t timeInMillis) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_CURRENT_TIME_MILLIS;
			event.integerValue = static_cast<long long>(timeInMillis);
			adapter_.Push(std::move(event));
		}

		void openOrder(int orderId, const Contract& contract, const Order& order, const OrderState& orderState) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_OPEN_ORDER;
			event.openOrder = open_order_from_sdk(orderId, contract, order, orderState);
			adapter_.Push(std::move(event));
		}

		void openOrderEnd() override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_OPEN_ORDER_END;
			adapter_.Push(std::move(event));
		}

		void completedOrder(const Contract& contract, const Order& order, const OrderState& orderState) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_COMPLETED_ORDER;
			event.completedOrder = completed_order_from_sdk(contract, order, orderState);
			adapter_.Push(std::move(event));
		}

		void completedOrdersEnd() override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_COMPLETED_ORDER_END;
			adapter_.Push(std::move(event));
		}

		void orderStatus(int orderId, const std::string& status, Decimal filled, Decimal remaining, double avgFillPrice, long long permId, int parentId, double lastFillPrice, int clientId, const std::string& whyHeld, double mktCapPrice) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_ORDER_STATUS;
			event.orderStatus.orderID = orderId;
			event.orderStatus.status = status;
			event.orderStatus.filled = decimal_to_string(filled);
			event.orderStatus.remaining = decimal_to_string(remaining);
			event.orderStatus.avgFillPrice = double_to_string(avgFillPrice);
			event.orderStatus.permID = std::to_string(permId);
			event.orderStatus.parentID = std::to_string(parentId);
			event.orderStatus.lastFillPrice = double_to_string(lastFillPrice);
			event.orderStatus.clientID = std::to_string(clientId);
			event.orderStatus.whyHeld = whyHeld;
			event.orderStatus.mktCapPrice = double_to_string(mktCapPrice);
			adapter_.Push(std::move(event));
		}

		void execDetails(int reqId, const Contract& contract, const Execution& execution) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_EXECUTION_DETAIL;
			event.reqID = reqId;
			event.executionDetail.orderID = execution.orderId;
			event.executionDetail.execID = execution.execId;
			event.executionDetail.account = execution.acctNumber;
			event.executionDetail.symbol = contract.symbol;
			event.executionDetail.side = execution.side;
			event.executionDetail.shares = decimal_to_string(execution.shares);
			event.executionDetail.price = double_to_string(execution.price);
			event.executionDetail.time = execution.time;
			adapter_.Push(std::move(event));
		}

		void execDetailsEnd(int reqId) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_EXECUTIONS_END;
			event.reqID = reqId;
			adapter_.Push(std::move(event));
		}

		void commissionAndFeesReport(const CommissionAndFeesReport& report) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_COMMISSION_REPORT;
			event.commissionReport.execID = report.execId;
			event.commissionReport.commission = double_to_string(report.commissionAndFees);
			event.commissionReport.currency = report.currency;
			event.commissionReport.realizedPNL = double_to_string(report.realizedPNL);
			adapter_.Push(std::move(event));
		}

		void accountSummary(int reqID, const std::string& account, const std::string& tag, const std::string& value, const std::string& currency) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_ACCOUNT_SUMMARY;
			event.reqID = reqID;
			event.account = account;
			event.tag = tag;
			event.value = value;
			event.currency = currency;
			adapter_.Push(std::move(event));
		}

		void accountSummaryEnd(int reqID) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_ACCOUNT_SUMMARY_END;
			event.reqID = reqID;
			adapter_.Push(std::move(event));
		}

		void updateAccountValue(const std::string& key, const std::string& val,
			const std::string& currency, const std::string& accountName) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_UPDATE_ACCOUNT_VALUE;
			event.tag = key;
			event.value = val;
			event.currency = currency;
			event.account = accountName;
			adapter_.Push(std::move(event));
		}

		void updatePortfolio(const Contract& contract, Decimal position, double marketPrice, double marketValue, double averageCost,
			double unrealizedPNL, double realizedPNL, const std::string& accountName) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_UPDATE_PORTFOLIO;
			event.account = accountName;
			event.contract = contract;
			event.position = decimal_to_string(position);
			event.marketPrice = double_to_string(marketPrice);
			event.marketValue = double_to_string(marketValue);
			event.avgCost = double_to_string(averageCost);
			event.unrealizedPNL = double_to_string(unrealizedPNL);
			event.realizedPNL = double_to_string(realizedPNL);
			adapter_.Push(std::move(event));
		}

		void updateAccountTime(const std::string& timeStamp) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_UPDATE_ACCOUNT_TIME;
			event.text = timeStamp;
			adapter_.Push(std::move(event));
		}

		void accountDownloadEnd(const std::string& accountName) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_ACCOUNT_DOWNLOAD_END;
			event.account = accountName;
			adapter_.Push(std::move(event));
		}

		void contractDetails(int reqID, const ContractDetails& details) override {
			PushContractDetails(IBKR_EVENT_CONTRACT_DETAILS, reqID, details);
		}

		void bondContractDetails(int reqID, const ContractDetails& details) override {
			PushContractDetails(IBKR_EVENT_BOND_CONTRACT_DETAILS, reqID, details);
		}

		void contractDetailsEnd(int reqID) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_CONTRACT_DETAILS_END;
			event.reqID = reqID;
			adapter_.Push(std::move(event));
		}

		void position(const std::string& account, const Contract& contract, Decimal position, double avgCost) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_POSITION;
			event.account = account;
			event.contract = contract;
			event.position = decimal_to_string(position);
			event.avgCost = double_to_string(avgCost);
			adapter_.Push(std::move(event));
		}

		void positionEnd() override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_POSITION_END;
			adapter_.Push(std::move(event));
		}

		void positionMulti(int reqId, const std::string& account, const std::string& modelCode, const Contract& contract,
			Decimal pos, double avgCost) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_POSITION_MULTI;
			event.reqID = reqId;
			event.account = account;
			event.modelCode = modelCode;
			event.contract = contract;
			event.position = decimal_to_string(pos);
			event.avgCost = double_to_string(avgCost);
			adapter_.Push(std::move(event));
		}

		void positionMultiEnd(int reqId) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_POSITION_MULTI_END;
			event.reqID = reqId;
			adapter_.Push(std::move(event));
		}

		void accountUpdateMulti(int reqId, const std::string& account, const std::string& modelCode,
			const std::string& key, const std::string& value, const std::string& currency) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_ACCOUNT_UPDATE_MULTI;
			event.reqID = reqId;
			event.account = account;
			event.modelCode = modelCode;
			event.tag = key;
			event.value = value;
			event.currency = currency;
			adapter_.Push(std::move(event));
		}

		void accountUpdateMultiEnd(int reqId) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_ACCOUNT_UPDATE_MULTI_END;
			event.reqID = reqId;
			adapter_.Push(std::move(event));
		}

		void pnl(int reqId, double dailyPnL, double unrealizedPnL, double realizedPnL) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_PNL;
			event.reqID = reqId;
			event.dailyPNL = double_to_string(dailyPnL);
			event.unrealizedPNL = double_to_string(unrealizedPnL);
			event.realizedPNL = double_to_string(realizedPnL);
			adapter_.Push(std::move(event));
		}

		void pnlSingle(int reqId, Decimal pos, double dailyPnL, double unrealizedPnL, double realizedPnL, double value) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_PNL_SINGLE;
			event.reqID = reqId;
			event.position = decimal_to_string(pos);
			event.dailyPNL = double_to_string(dailyPnL);
			event.unrealizedPNL = double_to_string(unrealizedPnL);
			event.realizedPNL = double_to_string(realizedPnL);
			event.value = double_to_string(value);
			adapter_.Push(std::move(event));
		}

		void marketDataType(int reqId, int marketDataType) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_MARKET_DATA_TYPE;
			event.reqID = reqId;
			event.integerValue = marketDataType;
			adapter_.Push(std::move(event));
		}

		void tickPrice(int reqId, TickType field, double price, const TickAttrib& attrib) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_PRICE;
			event.reqID = reqId;
			event.tickPrice.tickType = static_cast<int>(field);
			event.tickPrice.price = double_to_string(price);
			event.tickPrice.attrMask = tick_attrib_price(attrib);
			adapter_.Push(std::move(event));
		}

		void tickSize(int reqId, TickType field, Decimal size) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_SIZE;
			event.reqID = reqId;
			event.tickSize.tickType = static_cast<int>(field);
			event.tickSize.size = decimal_to_string(size);
			adapter_.Push(std::move(event));
		}

		void tickGeneric(int reqId, TickType tickType, double value) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_GENERIC;
			event.reqID = reqId;
			event.tickGeneric.tickType = static_cast<int>(tickType);
			event.tickGeneric.value = double_to_string(value);
			adapter_.Push(std::move(event));
		}

		void tickString(int reqId, TickType tickType, const std::string& value) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_STRING;
			event.reqID = reqId;
			event.tickString.tickType = static_cast<int>(tickType);
			event.tickString.value = value;
			adapter_.Push(std::move(event));
		}

		void tickSnapshotEnd(int reqId) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_SNAPSHOT_END;
			event.reqID = reqId;
			adapter_.Push(std::move(event));
		}

		void tickReqParams(int reqId, double minTick, const std::string& bboExchange, int snapshotPermissions) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_REQ_PARAMS;
			event.reqID = reqId;
			event.tickReqParams.minTick = double_to_string(minTick);
			event.tickReqParams.bboExchange = bboExchange;
			event.tickReqParams.snapshotPermissions = snapshotPermissions;
			adapter_.Push(std::move(event));
		}

		void realtimeBar(int reqId, long long time, double open, double high, double low, double close, Decimal volume, Decimal wap, int count) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_REAL_TIME_BAR;
			event.reqID = reqId;
			event.realTimeBar = HistoricalBarEvent{
				std::to_string(time),
				double_to_string(open),
				double_to_string(high),
				double_to_string(low),
				double_to_string(close),
				decimal_to_string(volume),
				decimal_to_string(wap),
				count == INT_MAX ? "" : std::to_string(count),
			};
			adapter_.Push(std::move(event));
		}

		void tickByTickAllLast(int reqId, int tickType, time_t time, double price, Decimal size, const TickAttribLast& tickAttribLast, const std::string& exchange, const std::string& specialConditions) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_BY_TICK;
			event.reqID = reqId;
			event.tickByTick.tickType = tickType;
			event.tickByTick.time = std::to_string(time);
			event.tickByTick.price = double_to_string(price);
			event.tickByTick.size = decimal_to_string(size);
			event.tickByTick.exchange = exchange;
			event.tickByTick.specialConditions = specialConditions;
			event.tickByTick.tickAttribLast = tick_attrib_last(tickAttribLast);
			adapter_.Push(std::move(event));
		}

		void tickByTickBidAsk(int reqId, time_t time, double bidPrice, double askPrice, Decimal bidSize, Decimal askSize, const TickAttribBidAsk& tickAttribBidAsk) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_BY_TICK;
			event.reqID = reqId;
			event.tickByTick.tickType = 3;
			event.tickByTick.time = std::to_string(time);
			event.tickByTick.bidPrice = double_to_string(bidPrice);
			event.tickByTick.askPrice = double_to_string(askPrice);
			event.tickByTick.bidSize = decimal_to_string(bidSize);
			event.tickByTick.askSize = decimal_to_string(askSize);
			event.tickByTick.tickAttribBidAsk = tick_attrib_bid_ask(tickAttribBidAsk);
			adapter_.Push(std::move(event));
		}

		void tickByTickMidPoint(int reqId, time_t time, double midPoint) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_BY_TICK;
			event.reqID = reqId;
			event.tickByTick.tickType = 4;
			event.tickByTick.time = std::to_string(time);
			event.tickByTick.midPoint = double_to_string(midPoint);
			adapter_.Push(std::move(event));
		}

		void updateMktDepth(int reqId, int position, int operation, int side, double price, Decimal size) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_MARKET_DEPTH;
			event.reqID = reqId;
			event.marketDepth.position = position;
			event.marketDepth.operation = operation;
			event.marketDepth.side = side;
			event.marketDepth.price = double_to_string(price);
			event.marketDepth.size = decimal_to_string(size);
			adapter_.Push(std::move(event));
		}

		void updateMktDepthL2(int reqId, int position, const std::string& marketMaker, int operation, int side, double price, Decimal size, bool isSmartDepth) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_MARKET_DEPTH_L2;
			event.reqID = reqId;
			event.marketDepthL2.position = position;
			event.marketDepthL2.marketMaker = marketMaker;
			event.marketDepthL2.operation = operation;
			event.marketDepthL2.side = side;
			event.marketDepthL2.price = double_to_string(price);
			event.marketDepthL2.size = decimal_to_string(size);
			event.marketDepthL2.isSmartDepth = isSmartDepth;
			adapter_.Push(std::move(event));
		}

		void tickOptionComputation(int reqId, TickType tickType, int tickAttrib, double impliedVol, double delta,
			double optPrice, double pvDividend, double gamma, double vega, double theta, double undPrice) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_TICK_OPTION_COMPUTATION;
			event.reqID = reqId;
			event.tickOptionComputation.tickType = static_cast<int>(tickType);
			event.tickOptionComputation.tickAttrib = tickAttrib;
			event.tickOptionComputation.impliedVol = double_to_string(impliedVol);
			event.tickOptionComputation.delta = double_to_string(delta);
			event.tickOptionComputation.optPrice = double_to_string(optPrice);
			event.tickOptionComputation.pvDividend = double_to_string(pvDividend);
			event.tickOptionComputation.gamma = double_to_string(gamma);
			event.tickOptionComputation.vega = double_to_string(vega);
			event.tickOptionComputation.theta = double_to_string(theta);
			event.tickOptionComputation.undPrice = double_to_string(undPrice);
			adapter_.Push(std::move(event));
		}

		void familyCodes(const std::vector<FamilyCode>& familyCodes) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_FAMILY_CODES;
			event.familyCodes = familyCodes;
			adapter_.Push(std::move(event));
		}

		void mktDepthExchanges(const std::vector<DepthMktDataDescription>& depthMktDataDescriptions) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_MKT_DEPTH_EXCHANGES;
			event.depthExchanges = depthMktDataDescriptions;
			adapter_.Push(std::move(event));
		}

		void newsProviders(const std::vector<NewsProvider>& newsProviders) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_NEWS_PROVIDERS;
			event.newsProviders = newsProviders;
			adapter_.Push(std::move(event));
		}

		void updateNewsBulletin(int msgId, int msgType, const std::string& newsMessage, const std::string& originExch) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_NEWS_BULLETIN;
			event.integerValue = msgId;
			event.messageType = msgType;
			event.text = newsMessage;
			event.source = originExch;
			adapter_.Push(std::move(event));
		}

		void newsArticle(int requestId, int articleType, const std::string& articleText) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_NEWS_ARTICLE;
			event.reqID = requestId;
			event.integerValue = articleType;
			event.text = articleText;
			adapter_.Push(std::move(event));
		}

		void historicalNews(int requestId, const std::string& time, const std::string& providerCode, const std::string& articleId, const std::string& headline) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_NEWS;
			event.reqID = requestId;
			event.marketName = time;
			event.account = providerCode;
			event.tag = articleId;
			event.value = headline;
			adapter_.Push(std::move(event));
		}

		void historicalNewsEnd(int requestId, bool hasMore) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_NEWS_END;
			event.reqID = requestId;
			event.integerValue = hasMore ? 1 : 0;
			adapter_.Push(std::move(event));
		}

		void scannerParameters(const std::string& xml) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_SCANNER_PARAMETERS;
			event.text = xml;
			adapter_.Push(std::move(event));
		}

		void scannerData(int reqId, int rank, const ContractDetails& contractDetails,
			const std::string& distance, const std::string& benchmark, const std::string& projection,
			const std::string& legsStr) override {
			ScannerDataEvent entry;
			entry.rank = rank;
			entry.contract = contractDetails.contract;
			entry.distance = distance;
			entry.benchmark = benchmark;
			entry.projection = projection;
			entry.legsStr = legsStr;
			scannerData_[reqId].push_back(std::move(entry));
		}

		void scannerDataEnd(int reqId) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_SCANNER_DATA;
			event.reqID = reqId;
			auto found = scannerData_.find(reqId);
			if (found != scannerData_.end()) {
				event.scannerData = std::move(found->second);
				scannerData_.erase(found);
			}
			adapter_.Push(std::move(event));
		}

		void receiveFA(faDataType pFaDataType, const std::string& cxml) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_RECEIVE_FA;
			event.integerValue = static_cast<int>(pFaDataType);
			event.text = cxml;
			adapter_.Push(std::move(event));
		}

		void replaceFAEnd(int reqId, const std::string& text) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_REPLACE_FA_END;
			event.reqID = reqId;
			event.text = text;
			adapter_.Push(std::move(event));
		}

		void historicalData(int reqId, const Bar& bar) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_DATA;
			event.reqID = reqId;
			event.historicalBar = historical_bar_from_sdk(bar);
			adapter_.Push(std::move(event));
		}

		void historicalDataEnd(int reqId, const std::string& startDateStr, const std::string& endDateStr) override {
			(void)startDateStr;
			(void)endDateStr;
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_DATA_END;
			event.reqID = reqId;
			adapter_.Push(std::move(event));
		}

		void historicalDataUpdate(int reqId, const Bar& bar) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_DATA_UPDATE;
			event.reqID = reqId;
			event.historicalBar = historical_bar_from_sdk(bar);
			adapter_.Push(std::move(event));
		}

		void historicalSchedule(int reqId, const std::string& startDateTime, const std::string& endDateTime, const std::string& timeZone, const std::vector<HistoricalSession>& sessions) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_SCHEDULE;
			event.reqID = reqId;
			event.historicalSchedule.startDateTime = startDateTime;
			event.historicalSchedule.endDateTime = endDateTime;
			event.historicalSchedule.timeZone = timeZone;
			event.historicalSchedule.sessions = sessions;
			adapter_.Push(std::move(event));
		}

		void historicalTicks(int reqId, const std::vector<HistoricalTick>& ticks, bool done) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_TICKS;
			event.reqID = reqId;
			event.integerValue = done ? 1 : 0;
			event.historicalTicks.reserve(ticks.size());
			for (const HistoricalTick& tick : ticks) {
				event.historicalTicks.push_back(HistoricalTickEvent{
					std::to_string(tick.time),
					double_to_string(tick.price),
					decimal_to_string(tick.size),
				});
			}
			adapter_.Push(std::move(event));
		}

		void historicalTicksBidAsk(int reqId, const std::vector<HistoricalTickBidAsk>& ticks, bool done) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_TICKS_BID_ASK;
			event.reqID = reqId;
			event.integerValue = done ? 1 : 0;
			event.historicalTicksBidAsk.reserve(ticks.size());
			for (const HistoricalTickBidAsk& tick : ticks) {
				event.historicalTicksBidAsk.push_back(HistoricalTickBidAskEvent{
					tick_attrib_bid_ask(tick.tickAttribBidAsk),
					std::to_string(tick.time),
					double_to_string(tick.priceBid),
					double_to_string(tick.priceAsk),
					decimal_to_string(tick.sizeBid),
					decimal_to_string(tick.sizeAsk),
				});
			}
			adapter_.Push(std::move(event));
		}

		void historicalTicksLast(int reqId, const std::vector<HistoricalTickLast>& ticks, bool done) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTORICAL_TICKS_LAST;
			event.reqID = reqId;
			event.integerValue = done ? 1 : 0;
			event.historicalTicksLast.reserve(ticks.size());
			for (const HistoricalTickLast& tick : ticks) {
				event.historicalTicksLast.push_back(HistoricalTickLastEvent{
					tick_attrib_last(tick.tickAttribLast),
					std::to_string(tick.time),
					double_to_string(tick.price),
					decimal_to_string(tick.size),
					tick.exchange,
					tick.specialConditions,
				});
			}
			adapter_.Push(std::move(event));
		}

		void headTimestamp(int reqId, const std::string& headTimestamp) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HEAD_TIMESTAMP;
			event.reqID = reqId;
			event.text = headTimestamp;
			adapter_.Push(std::move(event));
		}

		void histogramData(int reqId, const HistogramDataVector& data) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_HISTOGRAM_DATA;
			event.reqID = reqId;
			event.histogramData = data;
			adapter_.Push(std::move(event));
		}

		void wshMetaData(int reqId, const std::string& dataJson) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_WSH_META_DATA;
			event.reqID = reqId;
			event.text = dataJson;
			adapter_.Push(std::move(event));
		}

		void wshEventData(int reqId, const std::string& dataJson) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_WSH_EVENT_DATA;
			event.reqID = reqId;
			event.text = dataJson;
			adapter_.Push(std::move(event));
		}

		void userInfo(int reqId, const std::string& whiteBrandingId) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_USER_INFO;
			event.reqID = reqId;
			event.text = whiteBrandingId;
			adapter_.Push(std::move(event));
		}

		void softDollarTiers(int reqId, const std::vector<SoftDollarTier>& tiers) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_SOFT_DOLLAR_TIERS;
			event.reqID = reqId;
			event.softDollarTiers = tiers;
			adapter_.Push(std::move(event));
		}

		void displayGroupList(int reqId, const std::string& groups) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_DISPLAY_GROUP_LIST;
			event.reqID = reqId;
			event.text = groups;
			adapter_.Push(std::move(event));
		}

		void displayGroupUpdated(int reqId, const std::string& contractInfo) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_DISPLAY_GROUP_UPDATED;
			event.reqID = reqId;
			event.text = contractInfo;
			adapter_.Push(std::move(event));
		}

		void symbolSamples(int reqId, const std::vector<ContractDescription>& contractDescriptions) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_MATCHING_SYMBOLS;
			event.reqID = reqId;
			event.symbolSamples = contractDescriptions;
			adapter_.Push(std::move(event));
		}

		void marketRule(int marketRuleId, const std::vector<PriceIncrement>& priceIncrements) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_MARKET_RULE;
			event.marketRuleID = marketRuleId;
			event.priceIncrements = priceIncrements;
			adapter_.Push(std::move(event));
		}

		void securityDefinitionOptionalParameter(int reqId, const std::string& exchange, int underlyingConId, const std::string& tradingClass,
			const std::string& multiplier, const std::set<std::string>& expirations, const std::set<double>& strikes) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_SEC_DEF_OPT_PARAMS;
			event.reqID = reqId;
			SecDefOptParamsEvent params;
			params.exchange = exchange;
			params.underlyingConID = underlyingConId;
			params.tradingClass = tradingClass;
			params.multiplier = multiplier;
			params.expirations.assign(expirations.begin(), expirations.end());
			params.strikes.assign(strikes.begin(), strikes.end());
			event.secDefOptParams.push_back(std::move(params));
			adapter_.Push(std::move(event));
		}

		void securityDefinitionOptionalParameterEnd(int reqId) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_SEC_DEF_OPT_PARAMS_END;
			event.reqID = reqId;
			adapter_.Push(std::move(event));
		}

		void smartComponents(int reqId, const SmartComponentsMap& theMap) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_SMART_COMPONENTS;
			event.reqID = reqId;
			event.smartComponents = theMap;
			adapter_.Push(std::move(event));
		}

		void fundamentalData(int reqId, const std::string& data) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_FUNDAMENTAL_DATA;
			event.reqID = reqId;
			event.text = data;
			adapter_.Push(std::move(event));
		}

		void error(int id, time_t errorTime, int errorCode, const std::string& errorString, const std::string& advancedOrderRejectJSON) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_API_ERROR;
			event.reqID = id;
			event.errorTime = static_cast<long long>(errorTime);
			event.code = errorCode;
			event.text = errorString;
			event.advancedOrderRejectJSON = advancedOrderRejectJSON;
			adapter_.Push(std::move(event));
		}

		void error(int id, int errorCode, const std::string& errorString, const std::string& advancedOrderRejectJSON) {
			AdapterEvent event;
			event.kind = IBKR_EVENT_API_ERROR;
			event.reqID = id;
			event.code = errorCode;
			event.text = errorString;
			event.advancedOrderRejectJSON = advancedOrderRejectJSON;
			adapter_.Push(std::move(event));
		}

		void error(const std::string& str) {
			AdapterEvent event;
			event.kind = IBKR_EVENT_API_ERROR;
			event.reqID = -1;
			event.text = str;
			adapter_.Push(std::move(event));
		}

		void connectAck() override {
			adapter_.StartAPIFromConnectAck();
		}

		void connectionClosed() override {
			adapter_.MarkConnectionClosed();
		}

		void ClearScannerData() {
			scannerData_.clear();
		}

	private:
		void PushContractDetails(ibkr_event_kind kind, int reqID, const ContractDetails& details) {
			AdapterEvent event;
			event.kind = kind;
			event.reqID = reqID;
			event.contract = details.contract;
			event.marketName = details.marketName;
			event.minTick = double_to_string(details.minTick);
			event.longName = details.longName;
			event.timeZoneID = details.timeZoneId;
			adapter_.Push(std::move(event));
		}

		Adapter& adapter_;
		std::map<int, std::vector<ScannerDataEvent>> scannerData_;
	};

	explicit Adapter(int queueCapacity)
		: wrapper_(*this),
		  signal_(200),
		  queueCapacity_(std::max(queueCapacity, 1)) {}

	~Adapter() {
		Disconnect();
	}

	bool Connect(const char* host, int port, int clientID, int timeoutMS, ibkr_error* error) {
		try {
			Disconnect();
			{
				std::lock_guard<std::mutex> lock(mu_);
				events_.clear();
				fatal_.clear();
				connected_ = false;
				serverVersion_ = 0;
				connectionTime_.clear();
			}
			wrapper_.ClearScannerData();

			client_.reset(new EClientSocket(&wrapper_, &signal_));
			client_->asyncEConnect(true);

			bool connected = false;
			connected = client_->eConnect(host, port, clientID);
			if (!connected) {
				const std::string detail = ConnectFailureDetail();
				client_.reset();
				set_error(error, "connect", detail);
				return false;
			}
			{
				std::lock_guard<std::mutex> lock(mu_);
				connected_ = true;
			}
			cv_.notify_all();

			reader_.reset(new EReader(client_.get(), &signal_));
			reader_->start();
			readerRunning_.store(true);
			readerThread_ = std::thread([this]() { ReaderLoop(); });

			if (!WaitForConnectionMetadata(timeoutMS, error)) {
				Disconnect();
				return false;
			}
			if (!WithClient("connect_bootstrap", error, [this]() {
				client_->reqManagedAccts();
				client_->reqIds(1);
			})) {
				Disconnect();
				return false;
			}
			return true;
		} catch (const std::exception& err) {
			Disconnect();
			set_error(error, "connect", err);
			return false;
		} catch (...) {
			Disconnect();
			set_error(error, "connect", "unknown C++ exception");
			return false;
		}
	}

	void Disconnect() {
		readerRunning_.store(false);
		if (client_) {
			client_->eDisconnect();
		}
		if (readerThread_.joinable() && std::this_thread::get_id() != readerThread_.get_id()) {
			readerThread_.join();
		}
		reader_.reset();
		client_.reset();
		{
			std::lock_guard<std::mutex> lock(mu_);
			connected_ = false;
		}
		cv_.notify_all();
	}

	bool IsConnected() const {
		std::lock_guard<std::mutex> lock(mu_);
		return connected_;
	}

	int ServerVersion() const {
		std::lock_guard<std::mutex> lock(mu_);
		return serverVersion_;
	}

	std::string ConnectionTime() const {
		std::lock_guard<std::mutex> lock(mu_);
		return connectionTime_;
	}

	bool ReqCurrentTime(ibkr_error* error) {
		return WithClient("current_time", error, [this]() { client_->reqCurrentTime(); });
	}

	bool ReqCurrentTimeMillis(ibkr_error* error) {
		return WithClient("current_time_millis", error, [this]() { client_->reqCurrentTimeInMillis(); });
	}

	bool ReqAccountSummary(int reqID, const char* group, const char* tags, ibkr_error* error) {
		const std::string groupCopy = group ? group : "";
		const std::string tagsCopy = tags ? tags : "";
		return WithClient("account_summary", error, [this, reqID, groupCopy, tagsCopy]() {
			client_->reqAccountSummary(reqID, groupCopy, tagsCopy);
		});
	}

	bool CancelAccountSummary(int reqID, ibkr_error* error) {
		return WithClient("cancel_account_summary", error, [this, reqID]() {
			client_->cancelAccountSummary(reqID);
		});
	}

	bool ReqAccountUpdates(int subscribe, const char* account, ibkr_error* error) {
		const std::string accountCopy = account ? account : "";
		return WithClient("account_updates", error, [this, subscribe, accountCopy]() {
			client_->reqAccountUpdates(subscribe != 0, accountCopy);
		});
	}

	bool ReqAccountUpdatesMulti(int reqID, const char* account, const char* modelCode, ibkr_error* error) {
		const std::string accountCopy = account ? account : "";
		const std::string modelCodeCopy = modelCode ? modelCode : "";
		return WithClient("account_updates_multi", error, [this, reqID, accountCopy, modelCodeCopy]() {
			client_->reqAccountUpdatesMulti(reqID, accountCopy, modelCodeCopy, true);
		});
	}

	bool CancelAccountUpdatesMulti(int reqID, ibkr_error* error) {
		return WithClient("cancel_account_updates_multi", error, [this, reqID]() {
			client_->cancelAccountUpdatesMulti(reqID);
		});
	}

	bool ReqContractDetails(int reqID, const ibkr_contract* contract, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		return WithClient("contract_details", error, [this, reqID, contractCopy]() {
			client_->reqContractDetails(reqID, contractCopy);
		});
	}

	bool ReqPositions(ibkr_error* error) {
		return WithClient("positions", error, [this]() {
			client_->reqPositions();
		});
	}

	bool CancelPositions(ibkr_error* error) {
		return WithClient("cancel_positions", error, [this]() {
			client_->cancelPositions();
		});
	}

	bool ReqPositionsMulti(int reqID, const char* account, const char* modelCode, ibkr_error* error) {
		const std::string accountCopy = account ? account : "";
		const std::string modelCodeCopy = modelCode ? modelCode : "";
		return WithClient("positions_multi", error, [this, reqID, accountCopy, modelCodeCopy]() {
			client_->reqPositionsMulti(reqID, accountCopy, modelCodeCopy);
		});
	}

	bool CancelPositionsMulti(int reqID, ibkr_error* error) {
		return WithClient("cancel_positions_multi", error, [this, reqID]() {
			client_->cancelPositionsMulti(reqID);
		});
	}

	bool ReqPnL(int reqID, const char* account, const char* modelCode, ibkr_error* error) {
		const std::string accountCopy = account ? account : "";
		const std::string modelCodeCopy = modelCode ? modelCode : "";
		return WithClient("pnl", error, [this, reqID, accountCopy, modelCodeCopy]() {
			client_->reqPnL(reqID, accountCopy, modelCodeCopy);
		});
	}

	bool CancelPnL(int reqID, ibkr_error* error) {
		return WithClient("cancel_pnl", error, [this, reqID]() {
			client_->cancelPnL(reqID);
		});
	}

	bool ReqPnLSingle(int reqID, const char* account, const char* modelCode, int conID, ibkr_error* error) {
		const std::string accountCopy = account ? account : "";
		const std::string modelCodeCopy = modelCode ? modelCode : "";
		return WithClient("pnl_single", error, [this, reqID, accountCopy, modelCodeCopy, conID]() {
			client_->reqPnLSingle(reqID, accountCopy, modelCodeCopy, conID);
		});
	}

	bool CancelPnLSingle(int reqID, ibkr_error* error) {
		return WithClient("cancel_pnl_single", error, [this, reqID]() {
			client_->cancelPnLSingle(reqID);
		});
	}

	bool ReqMarketDataType(int marketDataType, ibkr_error* error) {
		return WithClient("market_data_type", error, [this, marketDataType]() {
			client_->reqMarketDataType(marketDataType);
		});
	}

	bool ReqMktData(int reqID, const ibkr_contract* contract, const char* genericTicks, int snapshot, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string genericTicksCopy = genericTicks ? genericTicks : "";
		return WithClient("quote", error, [this, reqID, contractCopy, genericTicksCopy, snapshot]() {
			client_->reqMktData(reqID, contractCopy, genericTicksCopy, snapshot != 0, false, TagValueListSPtr());
		});
	}

	bool CancelMktData(int reqID, ibkr_error* error) {
		return WithClient("cancel_quote", error, [this, reqID]() {
			client_->cancelMktData(reqID);
		});
	}

	bool ReqRealTimeBars(int reqID, const ibkr_contract* contract, const char* whatToShow, int useRTH, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string whatToShowCopy = whatToShow ? whatToShow : "";
		return WithClient("real_time_bars", error, [this, reqID, contractCopy, whatToShowCopy, useRTH]() {
			client_->reqRealTimeBars(reqID, contractCopy, 5, whatToShowCopy, useRTH != 0, TagValueListSPtr());
		});
	}

	bool CancelRealTimeBars(int reqID, ibkr_error* error) {
		return WithClient("cancel_real_time_bars", error, [this, reqID]() {
			client_->cancelRealTimeBars(reqID);
		});
	}

	bool ReqTickByTickData(int reqID, const ibkr_contract* contract, const char* tickType, int numberOfTicks, int ignoreSize, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string tickTypeCopy = tickType ? tickType : "";
		return WithClient("tick_by_tick", error, [this, reqID, contractCopy, tickTypeCopy, numberOfTicks, ignoreSize]() {
			client_->reqTickByTickData(reqID, contractCopy, tickTypeCopy, numberOfTicks, ignoreSize != 0);
		});
	}

	bool CancelTickByTickData(int reqID, ibkr_error* error) {
		return WithClient("cancel_tick_by_tick", error, [this, reqID]() {
			client_->cancelTickByTickData(reqID);
		});
	}

	bool ReqMktDepth(int reqID, const ibkr_contract* contract, int numRows, int isSmartDepth, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		return WithClient("market_depth", error, [this, reqID, contractCopy, numRows, isSmartDepth]() {
			client_->reqMktDepth(reqID, contractCopy, numRows, isSmartDepth != 0, TagValueListSPtr());
		});
	}

	bool CancelMktDepth(int reqID, int isSmartDepth, ibkr_error* error) {
		return WithClient("cancel_market_depth", error, [this, reqID, isSmartDepth]() {
			client_->cancelMktDepth(reqID, isSmartDepth != 0);
		});
	}

	bool CalculateImpliedVolatility(int reqID, const ibkr_contract* contract, const char* optionPrice, const char* underPrice, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const double optionPriceCopy = parse_double(optionPrice);
		const double underPriceCopy = parse_double(underPrice);
		return WithClient("calc_implied_volatility", error, [this, reqID, contractCopy, optionPriceCopy, underPriceCopy]() {
			client_->calculateImpliedVolatility(reqID, contractCopy, optionPriceCopy, underPriceCopy, TagValueListSPtr());
		});
	}

	bool CancelCalculateImpliedVolatility(int reqID, ibkr_error* error) {
		return WithClient("cancel_calc_implied_volatility", error, [this, reqID]() {
			client_->cancelCalculateImpliedVolatility(reqID);
		});
	}

	bool CalculateOptionPrice(int reqID, const ibkr_contract* contract, const char* volatility, const char* underPrice, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const double volatilityCopy = parse_double(volatility);
		const double underPriceCopy = parse_double(underPrice);
		return WithClient("calc_option_price", error, [this, reqID, contractCopy, volatilityCopy, underPriceCopy]() {
			client_->calculateOptionPrice(reqID, contractCopy, volatilityCopy, underPriceCopy, TagValueListSPtr());
		});
	}

	bool CancelCalculateOptionPrice(int reqID, ibkr_error* error) {
		return WithClient("cancel_calc_option_price", error, [this, reqID]() {
			client_->cancelCalculateOptionPrice(reqID);
		});
	}

	bool ExerciseOptions(int reqID, const ibkr_contract* contract, int exerciseAction, int exerciseQuantity, const char* account, int override, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string accountCopy = account ? account : "";
		return WithClient("exercise_options", error, [this, reqID, contractCopy, exerciseAction, exerciseQuantity, accountCopy, override]() {
			client_->exerciseOptions(reqID, contractCopy, exerciseAction, exerciseQuantity, accountCopy, override, "", "", false);
		});
	}

	bool PlaceOrder(const ibkr_place_order_request* request, ibkr_error* error) {
		if (!request) {
			set_error(error, "place_order", "place order request is null");
			return false;
		}
		Contract contractCopy = contract_from_c(&request->contract);
		contractCopy.comboLegs = combo_legs_to_sdk(request->combo_legs, request->combo_legs_count);
		Order orderCopy = order_from_c(request);
		return WithClient("place_order", error, [this, request, contractCopy, orderCopy]() {
			client_->placeOrder(static_cast<int>(request->order_id), contractCopy, orderCopy);
		});
	}

	bool ReqOpenOrders(const char* scope, ibkr_error* error) {
		const std::string scopeCopy = scope ? scope : "";
		if (scopeCopy != "" && scopeCopy != "client" && scopeCopy != "all" && scopeCopy != "auto") {
			set_error(error, "open_orders", "unknown open-order scope");
			return false;
		}
		return WithClient("open_orders", error, [this, scopeCopy]() {
			if (scopeCopy == "all") {
				client_->reqAllOpenOrders();
			} else if (scopeCopy == "auto") {
				client_->reqAutoOpenOrders(true);
			} else {
				client_->reqOpenOrders();
			}
		});
	}

	bool ReqCompletedOrders(int apiOnly, ibkr_error* error) {
		return WithClient("completed_orders", error, [this, apiOnly]() {
			client_->reqCompletedOrders(apiOnly != 0);
		});
	}

	bool CancelOrder(long long orderID, const char* manualOrderCancelTime, const char* extOperator, const char* manualOrderIndicator, ibkr_error* error) {
		OrderCancel orderCancel;
		orderCancel.manualOrderCancelTime = manualOrderCancelTime ? manualOrderCancelTime : "";
		orderCancel.extOperator = extOperator ? extOperator : "";
		orderCancel.manualOrderIndicator = parse_optional_int(manualOrderIndicator);
		return WithClient("cancel_order", error, [this, orderID, orderCancel]() {
			client_->cancelOrder(static_cast<int>(orderID), orderCancel);
		});
	}

	bool ReqGlobalCancel(const char* extOperator, const char* manualOrderIndicator, ibkr_error* error) {
		OrderCancel orderCancel;
		orderCancel.extOperator = extOperator ? extOperator : "";
		orderCancel.manualOrderIndicator = parse_optional_int(manualOrderIndicator);
		return WithClient("global_cancel", error, [this, orderCancel]() {
			client_->reqGlobalCancel(orderCancel);
		});
	}

	bool ReqExecutions(int reqID, const char* account, const char* symbol, ibkr_error* error) {
		ExecutionFilter filter;
		filter.m_acctCode = account ? account : "";
		filter.m_symbol = symbol ? symbol : "";
		return WithClient("executions", error, [this, reqID, filter]() {
			client_->reqExecutions(reqID, filter);
		});
	}

	bool ReqFamilyCodes(ibkr_error* error) {
		return WithClient("family_codes", error, [this]() {
			client_->reqFamilyCodes();
		});
	}

	bool ReqMktDepthExchanges(ibkr_error* error) {
		return WithClient("mkt_depth_exchanges", error, [this]() {
			client_->reqMktDepthExchanges();
		});
	}

	bool ReqNewsProviders(ibkr_error* error) {
		return WithClient("news_providers", error, [this]() {
			client_->reqNewsProviders();
		});
	}

	bool ReqNewsBulletins(int allMessages, ibkr_error* error) {
		return WithClient("news_bulletins", error, [this, allMessages]() {
			client_->reqNewsBulletins(allMessages != 0);
		});
	}

	bool CancelNewsBulletins(ibkr_error* error) {
		return WithClient("cancel_news_bulletins", error, [this]() {
			client_->cancelNewsBulletins();
		});
	}

	bool ReqNewsArticle(int reqID, const char* providerCode, const char* articleID, ibkr_error* error) {
		const std::string providerCodeCopy = providerCode ? providerCode : "";
		const std::string articleIDCopy = articleID ? articleID : "";
		return WithClient("news_article", error, [this, reqID, providerCodeCopy, articleIDCopy]() {
			client_->reqNewsArticle(reqID, providerCodeCopy, articleIDCopy, TagValueListSPtr());
		});
	}

	bool ReqHistoricalNews(int reqID, int conID, const char* providerCodes, const char* startDateTime, const char* endDateTime, int totalResults, ibkr_error* error) {
		const std::string providerCodesCopy = providerCodes ? providerCodes : "";
		const std::string startDateTimeCopy = startDateTime ? startDateTime : "";
		const std::string endDateTimeCopy = endDateTime ? endDateTime : "";
		return WithClient("historical_news", error, [this, reqID, conID, providerCodesCopy, startDateTimeCopy, endDateTimeCopy, totalResults]() {
			client_->reqHistoricalNews(reqID, conID, providerCodesCopy, startDateTimeCopy, endDateTimeCopy, totalResults, TagValueListSPtr());
		});
	}

	bool ReqScannerParameters(ibkr_error* error) {
		return WithClient("scanner_parameters", error, [this]() {
			client_->reqScannerParameters();
		});
	}

	bool ReqScannerSubscription(int reqID, int numberOfRows, const char* instrument, const char* locationCode, const char* scanCode, ibkr_error* error) {
		const std::string instrumentCopy = instrument ? instrument : "";
		const std::string locationCodeCopy = locationCode ? locationCode : "";
		const std::string scanCodeCopy = scanCode ? scanCode : "";
		return WithClient("scanner_subscription", error, [this, reqID, numberOfRows, instrumentCopy, locationCodeCopy, scanCodeCopy]() {
			ScannerSubscription subscription;
			subscription.numberOfRows = numberOfRows;
			subscription.instrument = instrumentCopy;
			subscription.locationCode = locationCodeCopy;
			subscription.scanCode = scanCodeCopy;
			client_->reqScannerSubscription(reqID, subscription, TagValueListSPtr(), TagValueListSPtr());
		});
	}

	bool CancelScannerSubscription(int reqID, ibkr_error* error) {
		return WithClient("cancel_scanner_subscription", error, [this, reqID]() {
			client_->cancelScannerSubscription(reqID);
		});
	}

	bool RequestFA(int faDataTypeValue, ibkr_error* error) {
		return WithClient("request_fa", error, [this, faDataTypeValue]() {
			client_->requestFA(static_cast<faDataType>(faDataTypeValue));
		});
	}

	bool ReplaceFA(int reqID, int faDataTypeValue, const char* xml, ibkr_error* error) {
		const std::string xmlCopy = xml ? xml : "";
		return WithClient("replace_fa", error, [this, reqID, faDataTypeValue, xmlCopy]() {
			client_->replaceFA(reqID, static_cast<faDataType>(faDataTypeValue), xmlCopy);
		});
	}

	bool ReqHistoricalData(int reqID, const ibkr_contract* contract, const char* endDateTime, const char* duration, const char* barSize,
		const char* whatToShow, int useRTH, int keepUpToDate, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string endDateTimeCopy = endDateTime ? endDateTime : "";
		const std::string durationCopy = duration ? duration : "";
		const std::string barSizeCopy = barSize ? barSize : "";
		const std::string whatToShowCopy = whatToShow ? whatToShow : "";
		return WithClient("historical_data", error, [this, reqID, contractCopy, endDateTimeCopy, durationCopy, barSizeCopy, whatToShowCopy, useRTH, keepUpToDate]() {
			client_->reqHistoricalData(reqID, contractCopy, endDateTimeCopy, durationCopy, barSizeCopy, whatToShowCopy, useRTH, 1, keepUpToDate != 0, TagValueListSPtr());
		});
	}

	bool CancelHistoricalData(int reqID, ibkr_error* error) {
		return WithClient("cancel_historical_data", error, [this, reqID]() {
			client_->cancelHistoricalData(reqID);
		});
	}

	bool ReqHistoricalTicks(int reqID, const ibkr_contract* contract, const char* startDateTime, const char* endDateTime,
		int numberOfTicks, const char* whatToShow, int useRTH, int ignoreSize, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string startDateTimeCopy = startDateTime ? startDateTime : "";
		const std::string endDateTimeCopy = endDateTime ? endDateTime : "";
		const std::string whatToShowCopy = whatToShow ? whatToShow : "";
		return WithClient("historical_ticks", error, [this, reqID, contractCopy, startDateTimeCopy, endDateTimeCopy, numberOfTicks, whatToShowCopy, useRTH, ignoreSize]() {
			client_->reqHistoricalTicks(reqID, contractCopy, startDateTimeCopy, endDateTimeCopy, numberOfTicks, whatToShowCopy, useRTH, ignoreSize != 0, TagValueListSPtr());
		});
	}

	bool CancelHistoricalTicks(int reqID, ibkr_error* error) {
		return WithClient("cancel_historical_ticks", error, [this, reqID]() {
			client_->cancelHistoricalTicks(reqID);
		});
	}

	bool ReqHeadTimestamp(int reqID, const ibkr_contract* contract, const char* whatToShow, int useRTH, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string whatToShowCopy = whatToShow ? whatToShow : "";
		return WithClient("head_timestamp", error, [this, reqID, contractCopy, whatToShowCopy, useRTH]() {
			client_->reqHeadTimestamp(reqID, contractCopy, whatToShowCopy, useRTH, 1);
		});
	}

	bool CancelHeadTimestamp(int reqID, ibkr_error* error) {
		return WithClient("cancel_head_timestamp", error, [this, reqID]() {
			client_->cancelHeadTimestamp(reqID);
		});
	}

	bool ReqHistogramData(int reqID, const ibkr_contract* contract, int useRTH, const char* period, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string periodCopy = period ? period : "";
		return WithClient("histogram_data", error, [this, reqID, contractCopy, useRTH, periodCopy]() {
			client_->reqHistogramData(reqID, contractCopy, useRTH != 0, periodCopy);
		});
	}

	bool CancelHistogramData(int reqID, ibkr_error* error) {
		return WithClient("cancel_histogram_data", error, [this, reqID]() {
			client_->cancelHistogramData(reqID);
		});
	}

	bool ReqWshMetaData(int reqID, ibkr_error* error) {
		return WithClient("wsh_meta_data", error, [this, reqID]() {
			client_->reqWshMetaData(reqID);
		});
	}

	bool CancelWshMetaData(int reqID, ibkr_error* error) {
		return WithClient("cancel_wsh_meta_data", error, [this, reqID]() {
			client_->cancelWshMetaData(reqID);
		});
	}

	bool ReqWshEventData(int reqID, int conID, const char* filter, int fillWatchlist, int fillPortfolio, int fillCompetitors,
		const char* startDate, const char* endDate, int totalLimit, ibkr_error* error) {
		const std::string filterCopy = filter ? filter : "";
		const std::string startDateCopy = startDate ? startDate : "";
		const std::string endDateCopy = endDate ? endDate : "";
		return WithClient("wsh_event_data", error, [this, reqID, conID, filterCopy, fillWatchlist, fillPortfolio, fillCompetitors, startDateCopy, endDateCopy, totalLimit]() {
			WshEventData data = conID == 0
				? WshEventData(filterCopy, fillWatchlist != 0, fillPortfolio != 0, fillCompetitors != 0, startDateCopy, endDateCopy, totalLimit)
				: WshEventData(conID, fillWatchlist != 0, fillPortfolio != 0, fillCompetitors != 0, startDateCopy, endDateCopy, totalLimit);
			if (!filterCopy.empty()) {
				data.filter = filterCopy;
			}
			client_->reqWshEventData(reqID, data);
		});
	}

	bool CancelWshEventData(int reqID, ibkr_error* error) {
		return WithClient("cancel_wsh_event_data", error, [this, reqID]() {
			client_->cancelWshEventData(reqID);
		});
	}

	bool ReqUserInfo(int reqID, ibkr_error* error) {
		return WithClient("user_info", error, [this, reqID]() {
			client_->reqUserInfo(reqID);
		});
	}

	bool ReqSoftDollarTiers(int reqID, ibkr_error* error) {
		return WithClient("soft_dollar_tiers", error, [this, reqID]() {
			client_->reqSoftDollarTiers(reqID);
		});
	}

	bool QueryDisplayGroups(int reqID, ibkr_error* error) {
		return WithClient("query_display_groups", error, [this, reqID]() {
			client_->queryDisplayGroups(reqID);
		});
	}

	bool SubscribeToGroupEvents(int reqID, int groupID, ibkr_error* error) {
		return WithClient("subscribe_to_group_events", error, [this, reqID, groupID]() {
			client_->subscribeToGroupEvents(reqID, groupID);
		});
	}

	bool UpdateDisplayGroup(int reqID, const char* contractInfo, ibkr_error* error) {
		const std::string contractInfoCopy = contractInfo ? contractInfo : "";
		return WithClient("update_display_group", error, [this, reqID, contractInfoCopy]() {
			client_->updateDisplayGroup(reqID, contractInfoCopy);
		});
	}

	bool UnsubscribeFromGroupEvents(int reqID, ibkr_error* error) {
		return WithClient("unsubscribe_from_group_events", error, [this, reqID]() {
			client_->unsubscribeFromGroupEvents(reqID);
		});
	}

	bool ReqMatchingSymbols(int reqID, const char* pattern, ibkr_error* error) {
		const std::string patternCopy = pattern ? pattern : "";
		return WithClient("matching_symbols", error, [this, reqID, patternCopy]() {
			client_->reqMatchingSymbols(reqID, patternCopy);
		});
	}

	bool ReqMarketRule(int marketRuleID, ibkr_error* error) {
		return WithClient("market_rule", error, [this, marketRuleID]() {
			client_->reqMarketRule(marketRuleID);
		});
	}

	bool ReqSecDefOptParams(int reqID, const char* underlyingSymbol, const char* futFopExchange, const char* underlyingSecType, int underlyingConID, ibkr_error* error) {
		const std::string underlyingSymbolCopy = underlyingSymbol ? underlyingSymbol : "";
		const std::string futFopExchangeCopy = futFopExchange ? futFopExchange : "";
		const std::string underlyingSecTypeCopy = underlyingSecType ? underlyingSecType : "";
		return WithClient("sec_def_opt_params", error, [this, reqID, underlyingSymbolCopy, futFopExchangeCopy, underlyingSecTypeCopy, underlyingConID]() {
			client_->reqSecDefOptParams(reqID, underlyingSymbolCopy, futFopExchangeCopy, underlyingSecTypeCopy, underlyingConID);
		});
	}

	bool ReqSmartComponents(int reqID, const char* bboExchange, ibkr_error* error) {
		const std::string bboExchangeCopy = bboExchange ? bboExchange : "";
		return WithClient("smart_components", error, [this, reqID, bboExchangeCopy]() {
			client_->reqSmartComponents(reqID, bboExchangeCopy);
		});
	}

	bool ReqFundamentalData(int reqID, const ibkr_contract* contract, const char* reportType, ibkr_error* error) {
		const Contract contractCopy = contract_from_c(contract);
		const std::string reportTypeCopy = reportType ? reportType : "";
		return WithClient("fundamental_data", error, [this, reqID, contractCopy, reportTypeCopy]() {
			client_->reqFundamentalData(reqID, contractCopy, reportTypeCopy, TagValueListSPtr());
		});
	}

	bool CancelFundamentalData(int reqID, ibkr_error* error) {
		return WithClient("cancel_fundamental_data", error, [this, reqID]() {
			client_->cancelFundamentalData(reqID);
		});
	}

	bool Drain(int maxEvents, ibkr_event_batch** out, ibkr_error* error) {
		if (!out) {
			set_error(error, "drain_events", "event batch output is null");
			return false;
		}
		*out = nullptr;

		try {
			std::deque<AdapterEvent> drained;
			{
				std::lock_guard<std::mutex> lock(mu_);
				if (!fatal_.empty() && events_.empty()) {
					set_error(error, "drain_events", fatal_);
					return false;
				}
				const int limit = maxEvents <= 0 ? static_cast<int>(events_.size()) : std::min(maxEvents, static_cast<int>(events_.size()));
				for (int i = 0; i < limit; i++) {
					drained.push_back(std::move(events_.front()));
					events_.pop_front();
				}
			}

			ibkr_event_batch* batch = static_cast<ibkr_event_batch*>(std::calloc(1, sizeof(ibkr_event_batch)));
			if (!batch) {
				set_error(error, "drain_events", "allocate event batch");
				return false;
			}
			batch->count = drained.size();
			if (!drained.empty()) {
				batch->events = static_cast<ibkr_event*>(std::calloc(drained.size(), sizeof(ibkr_event)));
				if (!batch->events) {
					std::free(batch);
					set_error(error, "drain_events", "allocate event batch rows");
					return false;
				}
				for (std::size_t i = 0; i < drained.size(); i++) {
					batch->events[i] = to_c_event(drained[i]);
				}
			}
			*out = batch;
			return true;
		} catch (const std::exception& err) {
			set_error(error, "drain_events", err);
			return false;
		} catch (...) {
			set_error(error, "drain_events", "unknown C++ exception");
			return false;
		}
	}

private:
	template <typename Fn>
	bool WithClient(const std::string& operation, ibkr_error* error, Fn fn) {
		try {
			if (!client_ || !client_->isConnected()) {
				set_error(error, operation, "official SDK client is not connected");
				return false;
			}
			fn();
			return true;
		} catch (const std::exception& err) {
			set_error(error, operation, err);
			return false;
		} catch (...) {
			set_error(error, operation, "unknown C++ exception");
			return false;
		}
	}

	bool WaitForConnectionMetadata(int timeoutMS, ibkr_error* error) {
		const auto timeout = std::chrono::milliseconds(timeoutMS > 0 ? timeoutMS : 1);
		const auto deadline = std::chrono::steady_clock::now() + timeout;
		std::unique_lock<std::mutex> lock(mu_);
		const bool ready = cv_.wait_until(lock, deadline, [this]() {
			return serverVersion_ > 0 || !fatal_.empty() || !connected_;
		});
		if (!ready) {
			set_error(error, "connect", "official SDK server metadata timed out");
			return false;
		}
		if (!fatal_.empty()) {
			set_error(error, "connect", fatal_);
			return false;
		}
		if (serverVersion_ <= 0) {
			set_error(error, "connect", "official SDK connection closed before server metadata");
			return false;
		}
		return true;
	}

	std::string ConnectFailureDetail() const {
		std::lock_guard<std::mutex> lock(mu_);
		if (!fatal_.empty()) {
			return "official SDK eConnect returned false: " + fatal_;
		}
		for (const AdapterEvent& event : events_) {
			if (event.kind == IBKR_EVENT_API_ERROR) {
				std::ostringstream out;
				out << "official SDK eConnect returned false";
				if (event.code != 0) {
					out << "; SDK error " << event.code;
				}
				if (!event.text.empty()) {
					out << ": " << event.text;
				}
				return out.str();
			}
		}
		return "official SDK eConnect returned false";
	}

	void StartAPIFromConnectAck() {
		try {
			EClientSocket* client = client_.get();
			if (!client || !client->isConnected()) {
				return;
			}
			client->startApi();
		} catch (const std::exception& err) {
			PushFatal(err.what());
		} catch (...) {
			PushFatal("official SDK startApi threw an unknown exception");
		}
	}

	void MaybePushConnectionMetadata() {
		EClientSocket* client = client_.get();
		if (!client) {
			return;
		}
		const int serverVersion = static_cast<EClient*>(client)->serverVersion();
		if (serverVersion <= 0) {
			return;
		}
		const std::string connectionTime = static_cast<EClient*>(client)->TwsConnectionTime();
		{
			std::lock_guard<std::mutex> lock(mu_);
			if (serverVersion_ > 0) {
				return;
			}
			serverVersion_ = serverVersion;
			connectionTime_ = connectionTime;
		}

		AdapterEvent metadata;
		metadata.kind = IBKR_EVENT_CONNECTION_METADATA;
		metadata.serverVersion = serverVersion;
		metadata.text = connectionTime;
		Push(std::move(metadata));
		cv_.notify_all();
	}

	void Push(AdapterEvent event) {
		bool overflow = false;
		{
			std::lock_guard<std::mutex> lock(mu_);
			if (!fatal_.empty()) {
				return;
			}
			if (events_.size() >= queueCapacity_) {
				fatal_ = "official SDK adapter event queue overflow";
				connected_ = false;
				overflow = true;
			} else {
				events_.push_back(std::move(event));
			}
		}
		cv_.notify_all();
		if (overflow && client_) {
			client_->eDisconnect();
		}
	}

	void PushFatal(const std::string& message) {
		{
			std::lock_guard<std::mutex> lock(mu_);
			if (fatal_.empty()) {
				fatal_ = message;
			}
			connected_ = false;
			if (events_.size() < queueCapacity_) {
				AdapterEvent event;
				event.kind = IBKR_EVENT_ADAPTER_FATAL;
				event.text = message;
				events_.push_back(std::move(event));
			}
		}
		cv_.notify_all();
	}

	void MarkConnectionClosed() {
		{
			std::lock_guard<std::mutex> lock(mu_);
			connected_ = false;
			if (!fatal_.empty() || events_.size() >= queueCapacity_) {
				cv_.notify_all();
				return;
			}
			AdapterEvent event;
			event.kind = IBKR_EVENT_CONNECTION_CLOSED;
			events_.push_back(std::move(event));
		}
		cv_.notify_all();
	}

	void ReaderLoop() {
		while (readerRunning_.load()) {
			EClientSocket* client = client_.get();
			EReader* reader = reader_.get();
			if (!client || !reader || !client->isConnected()) {
				break;
			}
			signal_.waitForSignal();
			try {
				reader->processMsgs();
				MaybePushConnectionMetadata();
			} catch (const std::exception& err) {
				PushFatal(err.what());
				break;
			} catch (...) {
				PushFatal("official SDK reader threw an unknown exception");
				break;
			}
		}
		MarkConnectionClosed();
	}

	Wrapper wrapper_;
	EReaderOSSignal signal_;
	std::unique_ptr<EClientSocket> client_;
	std::unique_ptr<EReader> reader_;
	std::thread readerThread_;
	std::atomic<bool> readerRunning_{false};

	mutable std::mutex mu_;
	std::condition_variable cv_;
	std::deque<AdapterEvent> events_;
	std::string fatal_;
	std::size_t queueCapacity_;
	bool connected_ = false;
	int serverVersion_ = 0;
	std::string connectionTime_;
};

struct ibkr_adapter {
	explicit ibkr_adapter(int queueCapacity) : impl(queueCapacity) {}
	Adapter impl;
};

extern "C" {

void ibkr_error_clear(ibkr_error* error) {
	if (!error) {
		return;
	}
	std::free(error->operation);
	std::free(error->message);
	std::free(error->advanced_order_reject_json);
	std::free(error->phase);
	*error = ibkr_error{};
}

int ibkr_build_info(ibkr_build_info_result* out, ibkr_error* error) {
	if (!out) {
		set_error(error, "build_info", "output is null");
		return 0;
	}
	*out = ibkr_build_info_result{};
	out->adapter_abi_version = copy_string(kAdapterABIVersion);
	out->sdk_api_version = copy_string(IBKR_STRINGIFY(IBKR_SDK_API_VERSION));
	out->compiler = copy_string(__VERSION__);
	out->protobuf_mode = copy_string(protobuf_mode());
	return 1;
}

void ibkr_build_info_free(ibkr_build_info_result value) {
	std::free(value.adapter_abi_version);
	std::free(value.sdk_api_version);
	std::free(value.compiler);
	std::free(value.protobuf_mode);
}

ibkr_adapter* ibkr_adapter_new(int queue_capacity, ibkr_error* error) {
	try {
		return new ibkr_adapter(queue_capacity);
	} catch (const std::exception& err) {
		set_error(error, "new", err);
		return nullptr;
	} catch (...) {
		set_error(error, "new", "unknown C++ exception");
		return nullptr;
	}
}

int ibkr_adapter_connect(ibkr_adapter* adapter, const char* host, int port, int client_id, int timeout_ms, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "connect", "adapter handle is null");
		return 0;
	}
	if (!host) {
		set_error(error, "connect", "host is null");
		return 0;
	}
	return adapter->impl.Connect(host, port, client_id, timeout_ms, error) ? 1 : 0;
}

void ibkr_adapter_disconnect(ibkr_adapter* adapter) {
	if (adapter) {
		adapter->impl.Disconnect();
	}
}

int ibkr_adapter_is_connected(ibkr_adapter* adapter) {
	return adapter && adapter->impl.IsConnected() ? 1 : 0;
}

int ibkr_adapter_server_version(ibkr_adapter* adapter) {
	if (!adapter) {
		return 0;
	}
	return adapter->impl.ServerVersion();
}

int ibkr_adapter_connection_time(ibkr_adapter* adapter, ibkr_string* out, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "connection_time", "adapter handle is null");
		return 0;
	}
	if (!out) {
		set_error(error, "connection_time", "output string is null");
		return 0;
	}
	out->data = copy_string(adapter->impl.ConnectionTime());
	return 1;
}

int ibkr_adapter_req_current_time(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "current_time", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqCurrentTime(error) ? 1 : 0;
}

int ibkr_adapter_req_current_time_millis(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "current_time_millis", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqCurrentTimeMillis(error) ? 1 : 0;
}

int ibkr_adapter_req_account_summary(ibkr_adapter* adapter, int req_id, const char* group, const char* tags, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "account_summary", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqAccountSummary(req_id, group, tags, error) ? 1 : 0;
}

int ibkr_adapter_cancel_account_summary(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_account_summary", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelAccountSummary(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_account_updates(ibkr_adapter* adapter, int subscribe, const char* account, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "account_updates", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqAccountUpdates(subscribe, account, error) ? 1 : 0;
}

int ibkr_adapter_req_account_updates_multi(ibkr_adapter* adapter, int req_id, const char* account, const char* model_code, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "account_updates_multi", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqAccountUpdatesMulti(req_id, account, model_code, error) ? 1 : 0;
}

int ibkr_adapter_cancel_account_updates_multi(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_account_updates_multi", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelAccountUpdatesMulti(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_contract_details(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "contract_details", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "contract_details", "contract is null");
		return 0;
	}
	return adapter->impl.ReqContractDetails(req_id, contract, error) ? 1 : 0;
}

int ibkr_adapter_req_positions(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "positions", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqPositions(error) ? 1 : 0;
}

int ibkr_adapter_cancel_positions(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_positions", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelPositions(error) ? 1 : 0;
}

int ibkr_adapter_req_positions_multi(ibkr_adapter* adapter, int req_id, const char* account, const char* model_code, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "positions_multi", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqPositionsMulti(req_id, account, model_code, error) ? 1 : 0;
}

int ibkr_adapter_cancel_positions_multi(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_positions_multi", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelPositionsMulti(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_pnl(ibkr_adapter* adapter, int req_id, const char* account, const char* model_code, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "pnl", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqPnL(req_id, account, model_code, error) ? 1 : 0;
}

int ibkr_adapter_cancel_pnl(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_pnl", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelPnL(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_pnl_single(ibkr_adapter* adapter, int req_id, const char* account, const char* model_code, int con_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "pnl_single", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqPnLSingle(req_id, account, model_code, con_id, error) ? 1 : 0;
}

int ibkr_adapter_cancel_pnl_single(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_pnl_single", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelPnLSingle(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_market_data_type(ibkr_adapter* adapter, int market_data_type, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "market_data_type", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqMarketDataType(market_data_type, error) ? 1 : 0;
}

int ibkr_adapter_req_mkt_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* generic_ticks, int snapshot, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "quote", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "quote", "contract is null");
		return 0;
	}
	return adapter->impl.ReqMktData(req_id, contract, generic_ticks, snapshot, error) ? 1 : 0;
}

int ibkr_adapter_cancel_mkt_data(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_quote", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelMktData(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_real_time_bars(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* what_to_show, int use_rth, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "real_time_bars", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "real_time_bars", "contract is null");
		return 0;
	}
	return adapter->impl.ReqRealTimeBars(req_id, contract, what_to_show, use_rth, error) ? 1 : 0;
}

int ibkr_adapter_cancel_real_time_bars(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_real_time_bars", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelRealTimeBars(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_tick_by_tick_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* tick_type, int number_of_ticks, int ignore_size, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "tick_by_tick", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "tick_by_tick", "contract is null");
		return 0;
	}
	return adapter->impl.ReqTickByTickData(req_id, contract, tick_type, number_of_ticks, ignore_size, error) ? 1 : 0;
}

int ibkr_adapter_cancel_tick_by_tick_data(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_tick_by_tick", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelTickByTickData(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_mkt_depth(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, int num_rows, int is_smart_depth, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "market_depth", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "market_depth", "contract is null");
		return 0;
	}
	return adapter->impl.ReqMktDepth(req_id, contract, num_rows, is_smart_depth, error) ? 1 : 0;
}

int ibkr_adapter_cancel_mkt_depth(ibkr_adapter* adapter, int req_id, int is_smart_depth, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_market_depth", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelMktDepth(req_id, is_smart_depth, error) ? 1 : 0;
}

int ibkr_adapter_calc_implied_volatility(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* option_price, const char* under_price, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "calc_implied_volatility", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "calc_implied_volatility", "contract is null");
		return 0;
	}
	return adapter->impl.CalculateImpliedVolatility(req_id, contract, option_price, under_price, error) ? 1 : 0;
}

int ibkr_adapter_cancel_calc_implied_volatility(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_calc_implied_volatility", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelCalculateImpliedVolatility(req_id, error) ? 1 : 0;
}

int ibkr_adapter_calc_option_price(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* volatility, const char* under_price, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "calc_option_price", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "calc_option_price", "contract is null");
		return 0;
	}
	return adapter->impl.CalculateOptionPrice(req_id, contract, volatility, under_price, error) ? 1 : 0;
}

int ibkr_adapter_cancel_calc_option_price(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_calc_option_price", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelCalculateOptionPrice(req_id, error) ? 1 : 0;
}

int ibkr_adapter_exercise_options(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, int exercise_action, int exercise_quantity, const char* account, int override, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "exercise_options", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "exercise_options", "contract is null");
		return 0;
	}
	return adapter->impl.ExerciseOptions(req_id, contract, exercise_action, exercise_quantity, account, override, error) ? 1 : 0;
}

int ibkr_adapter_place_order(ibkr_adapter* adapter, const ibkr_place_order_request* request, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "place_order", "adapter handle is null");
		return 0;
	}
	if (!request) {
		set_error(error, "place_order", "place order request is null");
		return 0;
	}
	return adapter->impl.PlaceOrder(request, error) ? 1 : 0;
}

int ibkr_adapter_req_open_orders(ibkr_adapter* adapter, const char* scope, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "open_orders", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqOpenOrders(scope, error) ? 1 : 0;
}

int ibkr_adapter_req_completed_orders(ibkr_adapter* adapter, int api_only, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "completed_orders", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqCompletedOrders(api_only, error) ? 1 : 0;
}

int ibkr_adapter_cancel_order(ibkr_adapter* adapter, long long order_id, const char* manual_order_cancel_time, const char* ext_operator, const char* manual_order_indicator, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_order", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelOrder(order_id, manual_order_cancel_time, ext_operator, manual_order_indicator, error) ? 1 : 0;
}

int ibkr_adapter_req_global_cancel(ibkr_adapter* adapter, const char* ext_operator, const char* manual_order_indicator, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "global_cancel", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqGlobalCancel(ext_operator, manual_order_indicator, error) ? 1 : 0;
}

int ibkr_adapter_req_executions(ibkr_adapter* adapter, int req_id, const char* account, const char* symbol, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "executions", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqExecutions(req_id, account, symbol, error) ? 1 : 0;
}

int ibkr_adapter_req_family_codes(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "family_codes", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqFamilyCodes(error) ? 1 : 0;
}

int ibkr_adapter_req_mkt_depth_exchanges(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "mkt_depth_exchanges", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqMktDepthExchanges(error) ? 1 : 0;
}

int ibkr_adapter_req_news_providers(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "news_providers", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqNewsProviders(error) ? 1 : 0;
}

int ibkr_adapter_req_news_bulletins(ibkr_adapter* adapter, int all_messages, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "news_bulletins", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqNewsBulletins(all_messages, error) ? 1 : 0;
}

int ibkr_adapter_cancel_news_bulletins(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_news_bulletins", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelNewsBulletins(error) ? 1 : 0;
}

int ibkr_adapter_req_news_article(ibkr_adapter* adapter, int req_id, const char* provider_code, const char* article_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "news_article", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqNewsArticle(req_id, provider_code, article_id, error) ? 1 : 0;
}

int ibkr_adapter_req_historical_news(ibkr_adapter* adapter, int req_id, int con_id, const char* provider_codes, const char* start_date_time, const char* end_date_time, int total_results, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "historical_news", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqHistoricalNews(req_id, con_id, provider_codes, start_date_time, end_date_time, total_results, error) ? 1 : 0;
}

int ibkr_adapter_req_scanner_parameters(ibkr_adapter* adapter, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "scanner_parameters", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqScannerParameters(error) ? 1 : 0;
}

int ibkr_adapter_req_scanner_subscription(ibkr_adapter* adapter, int req_id, int number_of_rows, const char* instrument, const char* location_code, const char* scan_code, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "scanner_subscription", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqScannerSubscription(req_id, number_of_rows, instrument, location_code, scan_code, error) ? 1 : 0;
}

int ibkr_adapter_cancel_scanner_subscription(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_scanner_subscription", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelScannerSubscription(req_id, error) ? 1 : 0;
}

int ibkr_adapter_request_fa(ibkr_adapter* adapter, int fa_data_type, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "request_fa", "adapter handle is null");
		return 0;
	}
	return adapter->impl.RequestFA(fa_data_type, error) ? 1 : 0;
}

int ibkr_adapter_replace_fa(ibkr_adapter* adapter, int req_id, int fa_data_type, const char* xml, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "replace_fa", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReplaceFA(req_id, fa_data_type, xml, error) ? 1 : 0;
}

int ibkr_adapter_req_historical_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* end_date_time, const char* duration, const char* bar_size, const char* what_to_show, int use_rth, int keep_up_to_date, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "historical_data", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "historical_data", "contract is null");
		return 0;
	}
	return adapter->impl.ReqHistoricalData(req_id, contract, end_date_time, duration, bar_size, what_to_show, use_rth, keep_up_to_date, error) ? 1 : 0;
}

int ibkr_adapter_cancel_historical_data(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_historical_data", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelHistoricalData(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_historical_ticks(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* start_date_time, const char* end_date_time, int number_of_ticks, const char* what_to_show, int use_rth, int ignore_size, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "historical_ticks", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "historical_ticks", "contract is null");
		return 0;
	}
	return adapter->impl.ReqHistoricalTicks(req_id, contract, start_date_time, end_date_time, number_of_ticks, what_to_show, use_rth, ignore_size, error) ? 1 : 0;
}

int ibkr_adapter_cancel_historical_ticks(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_historical_ticks", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelHistoricalTicks(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_head_timestamp(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* what_to_show, int use_rth, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "head_timestamp", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "head_timestamp", "contract is null");
		return 0;
	}
	return adapter->impl.ReqHeadTimestamp(req_id, contract, what_to_show, use_rth, error) ? 1 : 0;
}

int ibkr_adapter_cancel_head_timestamp(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_head_timestamp", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelHeadTimestamp(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_histogram_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, int use_rth, const char* period, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "histogram_data", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "histogram_data", "contract is null");
		return 0;
	}
	return adapter->impl.ReqHistogramData(req_id, contract, use_rth, period, error) ? 1 : 0;
}

int ibkr_adapter_cancel_histogram_data(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_histogram_data", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelHistogramData(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_wsh_meta_data(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "wsh_meta_data", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqWshMetaData(req_id, error) ? 1 : 0;
}

int ibkr_adapter_cancel_wsh_meta_data(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_wsh_meta_data", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelWshMetaData(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_wsh_event_data(ibkr_adapter* adapter, int req_id, int con_id, const char* filter, int fill_watchlist, int fill_portfolio, int fill_competitors, const char* start_date, const char* end_date, int total_limit, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "wsh_event_data", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqWshEventData(req_id, con_id, filter, fill_watchlist, fill_portfolio, fill_competitors, start_date, end_date, total_limit, error) ? 1 : 0;
}

int ibkr_adapter_cancel_wsh_event_data(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_wsh_event_data", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelWshEventData(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_user_info(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "user_info", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqUserInfo(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_soft_dollar_tiers(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "soft_dollar_tiers", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqSoftDollarTiers(req_id, error) ? 1 : 0;
}

int ibkr_adapter_query_display_groups(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "query_display_groups", "adapter handle is null");
		return 0;
	}
	return adapter->impl.QueryDisplayGroups(req_id, error) ? 1 : 0;
}

int ibkr_adapter_subscribe_to_group_events(ibkr_adapter* adapter, int req_id, int group_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "subscribe_to_group_events", "adapter handle is null");
		return 0;
	}
	return adapter->impl.SubscribeToGroupEvents(req_id, group_id, error) ? 1 : 0;
}

int ibkr_adapter_update_display_group(ibkr_adapter* adapter, int req_id, const char* contract_info, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "update_display_group", "adapter handle is null");
		return 0;
	}
	return adapter->impl.UpdateDisplayGroup(req_id, contract_info, error) ? 1 : 0;
}

int ibkr_adapter_unsubscribe_from_group_events(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "unsubscribe_from_group_events", "adapter handle is null");
		return 0;
	}
	return adapter->impl.UnsubscribeFromGroupEvents(req_id, error) ? 1 : 0;
}

int ibkr_adapter_req_matching_symbols(ibkr_adapter* adapter, int req_id, const char* pattern, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "matching_symbols", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqMatchingSymbols(req_id, pattern, error) ? 1 : 0;
}

int ibkr_adapter_req_market_rule(ibkr_adapter* adapter, int market_rule_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "market_rule", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqMarketRule(market_rule_id, error) ? 1 : 0;
}

int ibkr_adapter_req_sec_def_opt_params(ibkr_adapter* adapter, int req_id, const char* underlying_symbol, const char* fut_fop_exchange, const char* underlying_sec_type, int underlying_con_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "sec_def_opt_params", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqSecDefOptParams(req_id, underlying_symbol, fut_fop_exchange, underlying_sec_type, underlying_con_id, error) ? 1 : 0;
}

int ibkr_adapter_req_smart_components(ibkr_adapter* adapter, int req_id, const char* bbo_exchange, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "smart_components", "adapter handle is null");
		return 0;
	}
	return adapter->impl.ReqSmartComponents(req_id, bbo_exchange, error) ? 1 : 0;
}

int ibkr_adapter_req_fundamental_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* report_type, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "fundamental_data", "adapter handle is null");
		return 0;
	}
	if (!contract) {
		set_error(error, "fundamental_data", "contract is null");
		return 0;
	}
	return adapter->impl.ReqFundamentalData(req_id, contract, report_type, error) ? 1 : 0;
}

int ibkr_adapter_cancel_fundamental_data(ibkr_adapter* adapter, int req_id, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "cancel_fundamental_data", "adapter handle is null");
		return 0;
	}
	return adapter->impl.CancelFundamentalData(req_id, error) ? 1 : 0;
}

int ibkr_adapter_drain_events(ibkr_adapter* adapter, int max_events, ibkr_event_batch** out, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "drain_events", "adapter handle is null");
		return 0;
	}
	return adapter->impl.Drain(max_events, out, error) ? 1 : 0;
}

void ibkr_adapter_event_batch_free(ibkr_event_batch* batch) {
	if (!batch) {
		return;
	}
	for (std::size_t i = 0; i < batch->count; i++) {
		free_c_event(batch->events[i]);
	}
	std::free(batch->events);
	std::free(batch);
}

void ibkr_string_free(ibkr_string value) {
	std::free(value.data);
}

void ibkr_adapter_free(ibkr_adapter* adapter) {
	delete adapter;
}

} // extern "C"
