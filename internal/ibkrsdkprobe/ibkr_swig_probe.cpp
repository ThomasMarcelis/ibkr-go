//go:build ibkr_swig && cgo && linux

#include "ibkr_swig_probe.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <condition_variable>
#include <ctime>
#include <exception>
#include <memory>
#include <mutex>
#include <sstream>
#include <string>
#include <thread>
#include <vector>

#include "DefaultEWrapper.h"
#include "EClientSocket.h"
#include "EReader.h"
#include "EReaderOSSignal.h"

namespace {

constexpr const char* kCompileVersion = "ibkr-swig-official-sdk-probe";

struct SDKError {
	int id = 0;
	long errorTime = 0;
	int code = 0;
	std::string message;
	std::string advancedOrderRejectJSON;
};

struct AccountSummaryRow {
	int reqID = 0;
	std::string account;
	std::string tag;
	std::string value;
	std::string currency;
};

template <typename T>
const T* atOrNull(const std::vector<T>& values, int index) {
	if (index < 0) {
		return nullptr;
	}
	const auto unsignedIndex = static_cast<std::size_t>(index);
	if (unsignedIndex >= values.size()) {
		return nullptr;
	}
	return &values[unsignedIndex];
}

} // namespace

class OfficialSDKProbe::Impl {
public:
	class Wrapper final : public DefaultEWrapper {
	public:
		explicit Wrapper(Impl& impl) : impl_(impl) {}

		void nextValidId(int orderID) override {
			std::lock_guard<std::mutex> lock(impl_.mu_);
			impl_.nextValidID_ = static_cast<int>(orderID);
			impl_.haveNextValidID_ = true;
			impl_.cv_.notify_all();
		}

		void managedAccounts(const std::string& accountsList) override {
			std::lock_guard<std::mutex> lock(impl_.mu_);
			impl_.managedAccountsCSV_ = accountsList;
			impl_.haveManagedAccounts_ = true;
			impl_.cv_.notify_all();
		}

		void currentTime(long long time) override {
			std::lock_guard<std::mutex> lock(impl_.mu_);
			impl_.currentTime_ = time;
			impl_.haveCurrentTime_ = true;
			impl_.cv_.notify_all();
		}

		void accountSummary(int reqID, const std::string& account, const std::string& tag, const std::string& value, const std::string& currency) override {
			std::lock_guard<std::mutex> lock(impl_.mu_);
			impl_.accountSummaryRows_.push_back(AccountSummaryRow{
				reqID,
				account,
				tag,
				value,
				currency,
			});
			impl_.cv_.notify_all();
		}

		void accountSummaryEnd(int reqID) override {
			std::lock_guard<std::mutex> lock(impl_.mu_);
			impl_.accountSummaryEndReqID_ = reqID;
			impl_.cv_.notify_all();
		}

		void error(int id, time_t errorTime, int errorCode, const std::string& errorString, const std::string& advancedOrderRejectJSON) {
			impl_.RecordError(id, static_cast<long>(errorTime), errorCode, errorString, advancedOrderRejectJSON);
		}

		void error(int id, int errorCode, const std::string& errorString, const std::string& advancedOrderRejectJSON) {
			impl_.RecordError(id, 0, errorCode, errorString, advancedOrderRejectJSON);
		}

		void error(const std::string& str) {
			impl_.RecordError(-1, 0, 0, str, "");
		}

		void connectionClosed() override {
			std::lock_guard<std::mutex> lock(impl_.mu_);
			impl_.connected_ = false;
			impl_.connectionClosed_ = true;
			impl_.cv_.notify_all();
		}

	private:
		Impl& impl_;
	};

	Impl() : wrapper_(*this), signal_(200) {}

	~Impl() {
		Disconnect();
	}

	bool Connect(const std::string& host, int port, int clientID, int timeoutMS) {
		Disconnect();
		ResetState();

		client_.reset(new EClientSocket(&wrapper_, &signal_));
		const bool connected = client_->eConnect(host.c_str(), port, clientID);
		if (!connected) {
			std::lock_guard<std::mutex> lock(mu_);
			blockedReason_ = "official SDK eConnect returned false";
			client_.reset();
			return false;
		}

		{
			std::lock_guard<std::mutex> lock(mu_);
			connected_ = true;
			serverVersion_ = static_cast<EClient*>(client_.get())->serverVersion();
			connectionTime_ = static_cast<EClient*>(client_.get())->TwsConnectionTime();
		}

		reader_.reset(new EReader(client_.get(), &signal_));
		reader_->start();
		readerRunning_.store(true);
		readerThread_ = std::thread([this]() { ReaderLoop(); });

		const bool bootstrapped = WaitFor(timeoutMS, [this]() {
			return (haveNextValidID_ && haveManagedAccounts_) || connectionClosed_;
		});
		if (bootstrapped) {
			std::lock_guard<std::mutex> lock(mu_);
			if (BootstrapCompleteLocked()) {
				return true;
			}
		}

		std::lock_guard<std::mutex> lock(mu_);
		if (connectionClosed_) {
			blockedReason_ = WithLastSDKErrorLocked("connection closed before SDK bootstrap completed");
		} else {
			blockedReason_ = WithLastSDKErrorLocked("timed out waiting for nextValidId and managedAccounts");
		}
		return false;
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

		std::lock_guard<std::mutex> lock(mu_);
		connected_ = false;
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

	bool BootstrapComplete() const {
		std::lock_guard<std::mutex> lock(mu_);
		return BootstrapCompleteLocked();
	}

	std::string BlockedReason() const {
		std::lock_guard<std::mutex> lock(mu_);
		return blockedReason_;
	}

	int NextValidID() const {
		std::lock_guard<std::mutex> lock(mu_);
		return nextValidID_;
	}

	std::string ManagedAccountsCSV() const {
		std::lock_guard<std::mutex> lock(mu_);
		return managedAccountsCSV_;
	}

	bool RequestCurrentTime(int timeoutMS) {
		{
			std::lock_guard<std::mutex> lock(mu_);
			haveCurrentTime_ = false;
			currentTime_ = 0;
		}
		if (!ConnectedForRequest("current time")) {
			return false;
		}

		client_->reqCurrentTime();
		const bool received = WaitFor(timeoutMS, [this]() {
			return haveCurrentTime_ || connectionClosed_;
		});
		if (received) {
			std::lock_guard<std::mutex> lock(mu_);
			if (CurrentTimeReadyLocked()) {
				return true;
			}
		}

		std::lock_guard<std::mutex> lock(mu_);
		blockedReason_ = WithLastSDKErrorLocked("timed out waiting for currentTime");
		return false;
	}

	long CurrentTime() const {
		std::lock_guard<std::mutex> lock(mu_);
		return currentTime_;
	}

	bool RequestAccountSummary(int reqID, const std::string& group, const std::string& tags, int timeoutMS) {
		{
			std::lock_guard<std::mutex> lock(mu_);
			accountSummaryRows_.clear();
			accountSummaryEndReqID_ = -1;
		}
		if (!ConnectedForRequest("account summary")) {
			return false;
		}

		client_->reqAccountSummary(reqID, group, tags);
		const bool completed = WaitFor(timeoutMS, [this, reqID]() {
			return accountSummaryEndReqID_ == reqID || connectionClosed_;
		});
		client_->cancelAccountSummary(reqID);
		if (completed) {
			std::lock_guard<std::mutex> lock(mu_);
			if (AccountSummaryCompleteLocked(reqID)) {
				return true;
			}
		}

		std::lock_guard<std::mutex> lock(mu_);
		blockedReason_ = WithLastSDKErrorLocked("timed out waiting for accountSummaryEnd");
		return false;
	}

	int AccountSummaryCount() const {
		std::lock_guard<std::mutex> lock(mu_);
		return static_cast<int>(accountSummaryRows_.size());
	}

	int AccountSummaryReqID(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto row = atOrNull(accountSummaryRows_, index);
		return row ? row->reqID : 0;
	}

	std::string AccountSummaryAccount(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto row = atOrNull(accountSummaryRows_, index);
		return row ? row->account : "";
	}

	std::string AccountSummaryTag(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto row = atOrNull(accountSummaryRows_, index);
		return row ? row->tag : "";
	}

	std::string AccountSummaryValue(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto row = atOrNull(accountSummaryRows_, index);
		return row ? row->value : "";
	}

	std::string AccountSummaryCurrency(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto row = atOrNull(accountSummaryRows_, index);
		return row ? row->currency : "";
	}

	int ErrorCount() const {
		std::lock_guard<std::mutex> lock(mu_);
		return static_cast<int>(errors_.size());
	}

	int ErrorID(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto err = atOrNull(errors_, index);
		return err ? err->id : 0;
	}

	long ErrorTime(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto err = atOrNull(errors_, index);
		return err ? err->errorTime : 0;
	}

	int ErrorCode(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto err = atOrNull(errors_, index);
		return err ? err->code : 0;
	}

	std::string ErrorMessage(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto err = atOrNull(errors_, index);
		return err ? err->message : "";
	}

	std::string ErrorAdvancedOrderRejectJSON(int index) const {
		std::lock_guard<std::mutex> lock(mu_);
		const auto err = atOrNull(errors_, index);
		return err ? err->advancedOrderRejectJSON : "";
	}

private:
	void ResetState() {
		std::lock_guard<std::mutex> lock(mu_);
		connected_ = false;
		connectionClosed_ = false;
		serverVersion_ = 0;
		connectionTime_.clear();
		haveNextValidID_ = false;
		nextValidID_ = 0;
		haveManagedAccounts_ = false;
		managedAccountsCSV_.clear();
		haveCurrentTime_ = false;
		currentTime_ = 0;
		accountSummaryRows_.clear();
		accountSummaryEndReqID_ = -1;
		errors_.clear();
		blockedReason_.clear();
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
				RecordError(-1, 0, 0, err.what(), "");
			} catch (...) {
				RecordError(-1, 0, 0, "official SDK reader threw an unknown exception", "");
			}
		}
		wrapper_.connectionClosed();
	}

	bool ConnectedForRequest(const std::string& operation) {
		if (client_ && client_->isConnected()) {
			return true;
		}
		std::lock_guard<std::mutex> lock(mu_);
		blockedReason_ = "cannot request " + operation + ": official SDK client is not connected";
		return false;
	}

	void RecordError(int id, long errorTime, int errorCode, const std::string& message, const std::string& advancedOrderRejectJSON) {
		std::lock_guard<std::mutex> lock(mu_);
		errors_.push_back(SDKError{
			id,
			errorTime,
			errorCode,
			message,
			advancedOrderRejectJSON,
		});
		cv_.notify_all();
	}

	template <typename Predicate>
	bool WaitFor(int timeoutMS, Predicate predicate) {
		const auto timeout = std::chrono::milliseconds(std::max(timeoutMS, 1));
		std::unique_lock<std::mutex> lock(mu_);
		return cv_.wait_for(lock, timeout, predicate);
	}

	bool BootstrapCompleteLocked() const {
		return haveNextValidID_ && haveManagedAccounts_;
	}

	bool CurrentTimeReadyLocked() const {
		return haveCurrentTime_;
	}

	bool AccountSummaryCompleteLocked(int reqID) const {
		return accountSummaryEndReqID_ == reqID;
	}

	std::string WithLastSDKErrorLocked(const std::string& prefix) const {
		if (errors_.empty()) {
			return prefix;
		}
		const auto& err = errors_.back();
		std::ostringstream out;
		out << prefix << "; last SDK error id=" << err.id << " time=" << err.errorTime << " code=" << err.code << " msg=" << err.message;
		if (!err.advancedOrderRejectJSON.empty()) {
			out << " advancedOrderRejectJSON=" << err.advancedOrderRejectJSON;
		}
		return out.str();
	}

	Wrapper wrapper_;
	EReaderOSSignal signal_;
	std::unique_ptr<EClientSocket> client_;
	std::unique_ptr<EReader> reader_;
	std::thread readerThread_;
	std::atomic<bool> readerRunning_{false};

	mutable std::mutex mu_;
	std::condition_variable cv_;
	bool connected_ = false;
	bool connectionClosed_ = false;
	int serverVersion_ = 0;
	std::string connectionTime_;
	bool haveNextValidID_ = false;
	int nextValidID_ = 0;
	bool haveManagedAccounts_ = false;
	std::string managedAccountsCSV_;
	bool haveCurrentTime_ = false;
	long currentTime_ = 0;
	std::vector<AccountSummaryRow> accountSummaryRows_;
	int accountSummaryEndReqID_ = -1;
	std::vector<SDKError> errors_;
	std::string blockedReason_;
};

OfficialSDKProbe::OfficialSDKProbe() : impl_(new Impl()) {}

OfficialSDKProbe::~OfficialSDKProbe() {
	delete impl_;
}

std::string OfficialSDKProbe::CompileVersion() const {
	return kCompileVersion;
}

bool OfficialSDKProbe::Connect(const std::string& host, int port, int clientID, int timeoutMS) {
	return impl_->Connect(host, port, clientID, timeoutMS);
}

void OfficialSDKProbe::Disconnect() {
	impl_->Disconnect();
}

bool OfficialSDKProbe::IsConnected() const {
	return impl_->IsConnected();
}

int OfficialSDKProbe::ServerVersion() const {
	return impl_->ServerVersion();
}

std::string OfficialSDKProbe::ConnectionTime() const {
	return impl_->ConnectionTime();
}

bool OfficialSDKProbe::BootstrapComplete() const {
	return impl_->BootstrapComplete();
}

std::string OfficialSDKProbe::BlockedReason() const {
	return impl_->BlockedReason();
}

int OfficialSDKProbe::NextValidID() const {
	return impl_->NextValidID();
}

std::string OfficialSDKProbe::ManagedAccountsCSV() const {
	return impl_->ManagedAccountsCSV();
}

bool OfficialSDKProbe::RequestCurrentTime(int timeoutMS) {
	return impl_->RequestCurrentTime(timeoutMS);
}

long OfficialSDKProbe::CurrentTime() const {
	return impl_->CurrentTime();
}

bool OfficialSDKProbe::RequestAccountSummary(int reqID, const std::string& group, const std::string& tags, int timeoutMS) {
	return impl_->RequestAccountSummary(reqID, group, tags, timeoutMS);
}

int OfficialSDKProbe::AccountSummaryCount() const {
	return impl_->AccountSummaryCount();
}

int OfficialSDKProbe::AccountSummaryReqID(int index) const {
	return impl_->AccountSummaryReqID(index);
}

std::string OfficialSDKProbe::AccountSummaryAccount(int index) const {
	return impl_->AccountSummaryAccount(index);
}

std::string OfficialSDKProbe::AccountSummaryTag(int index) const {
	return impl_->AccountSummaryTag(index);
}

std::string OfficialSDKProbe::AccountSummaryValue(int index) const {
	return impl_->AccountSummaryValue(index);
}

std::string OfficialSDKProbe::AccountSummaryCurrency(int index) const {
	return impl_->AccountSummaryCurrency(index);
}

int OfficialSDKProbe::ErrorCount() const {
	return impl_->ErrorCount();
}

int OfficialSDKProbe::ErrorID(int index) const {
	return impl_->ErrorID(index);
}

long OfficialSDKProbe::ErrorTime(int index) const {
	return impl_->ErrorTime(index);
}

int OfficialSDKProbe::ErrorCode(int index) const {
	return impl_->ErrorCode(index);
}

std::string OfficialSDKProbe::ErrorMessage(int index) const {
	return impl_->ErrorMessage(index);
}

std::string OfficialSDKProbe::ErrorAdvancedOrderRejectJSON(int index) const {
	return impl_->ErrorAdvancedOrderRejectJSON(index);
}

std::string IbkrSdkProbeCompileVersion() {
	return kCompileVersion;
}
