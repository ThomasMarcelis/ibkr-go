//go:build ibkr_sdk && cgo && linux

#include "ibkr_adapter.h"

#include <algorithm>
#include <atomic>
#include <cstdlib>
#include <cstring>
#include <deque>
#include <exception>
#include <memory>
#include <mutex>
#include <sstream>
#include <string>
#include <thread>
#include <utility>
#include <vector>

#include "DefaultEWrapper.h"
#include "Contract.h"
#include "Decimal.h"
#include "EClientSocket.h"
#include "EReader.h"
#include "EReaderOSSignal.h"
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

	long long orderID = 0;
	long long errorTime = 0;
	int code = 0;
	std::string advancedOrderRejectJSON;

	Contract contract;
	std::string marketName;
	std::string minTick;
	std::string longName;
	std::string timeZoneID;
	std::string position;
	std::string avgCost;
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

double parse_double(const char* value) {
	if (!value || std::strlen(value) == 0) {
		return 0;
	}
	return std::strtod(value, nullptr);
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

ibkr_event to_c_event(const AdapterEvent& event) {
	ibkr_event out{};
	out.kind = event.kind;
	out.req_id = event.reqID;
	out.server_version = event.serverVersion;
	out.integer_value = event.integerValue;
	out.text = copy_string(event.text);
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

		void contractDetails(int reqID, const ContractDetails& details) override {
			AdapterEvent event;
			event.kind = IBKR_EVENT_CONTRACT_DETAILS;
			event.reqID = reqID;
			event.contract = details.contract;
			event.marketName = details.marketName;
			event.minTick = double_to_string(details.minTick);
			event.longName = details.longName;
			event.timeZoneID = details.timeZoneId;
			adapter_.Push(std::move(event));
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

		void connectionClosed() override {
			adapter_.MarkConnectionClosed();
		}

	private:
		Adapter& adapter_;
	};

	explicit Adapter(int queueCapacity)
		: wrapper_(*this),
		  signal_(200),
		  queueCapacity_(std::max(queueCapacity, 1)) {}

	~Adapter() {
		Disconnect();
	}

	bool Connect(const char* host, int port, int clientID, ibkr_error* error) {
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

			client_.reset(new EClientSocket(&wrapper_, &signal_));
			if (!client_->eConnect(host, port, clientID)) {
				client_.reset();
				set_error(error, "connect", "official SDK eConnect returned false");
				return false;
			}

			serverVersion_ = static_cast<EClient*>(client_.get())->serverVersion();
			connectionTime_ = static_cast<EClient*>(client_.get())->TwsConnectionTime();
			connected_ = true;

			AdapterEvent metadata;
			metadata.kind = IBKR_EVENT_CONNECTION_METADATA;
			metadata.serverVersion = serverVersion_;
			metadata.text = connectionTime_;
			Push(std::move(metadata));

			reader_.reset(new EReader(client_.get(), &signal_));
			reader_->start();
			readerRunning_.store(true);
			readerThread_ = std::thread([this]() { ReaderLoop(); });
			return true;
		} catch (const std::exception& err) {
			set_error(error, "connect", err);
			return false;
		} catch (...) {
			set_error(error, "connect", "unknown C++ exception");
			return false;
		}
	}

	void Disconnect() {
		readerRunning_.store(false);
		if (client_) {
			client_->eDisconnect();
		}
		if (readerThread_.joinable()) {
			readerThread_.join();
		}
		reader_.reset();
		client_.reset();
		{
			std::lock_guard<std::mutex> lock(mu_);
			connected_ = false;
		}
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

	void Push(AdapterEvent event) {
		std::lock_guard<std::mutex> lock(mu_);
		if (!fatal_.empty()) {
			return;
		}
		if (events_.size() >= queueCapacity_) {
			fatal_ = "official SDK adapter event queue overflow";
			connected_ = false;
			if (client_) {
				client_->eDisconnect();
			}
			return;
		}
		events_.push_back(std::move(event));
	}

	void PushFatal(const std::string& message) {
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

	void MarkConnectionClosed() {
		std::lock_guard<std::mutex> lock(mu_);
		connected_ = false;
		if (!fatal_.empty() || events_.size() >= queueCapacity_) {
			return;
		}
		AdapterEvent event;
		event.kind = IBKR_EVENT_CONNECTION_CLOSED;
		events_.push_back(std::move(event));
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

int ibkr_adapter_connect(ibkr_adapter* adapter, const char* host, int port, int client_id, int, ibkr_error* error) {
	if (!adapter) {
		set_error(error, "connect", "adapter handle is null");
		return 0;
	}
	if (!host) {
		set_error(error, "connect", "host is null");
		return 0;
	}
	return adapter->impl.Connect(host, port, client_id, error) ? 1 : 0;
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
