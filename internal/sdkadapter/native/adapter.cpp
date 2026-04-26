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
#include "EClientSocket.h"
#include "EReader.h"
#include "EReaderOSSignal.h"

namespace {

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
};

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
