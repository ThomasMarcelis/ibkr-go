#pragma once

#include <string>

class OfficialSDKProbe {
public:
	OfficialSDKProbe();
	~OfficialSDKProbe();

	OfficialSDKProbe(const OfficialSDKProbe&) = delete;
	OfficialSDKProbe& operator=(const OfficialSDKProbe&) = delete;

	std::string CompileVersion() const;

	bool Connect(const std::string& host, int port, int clientID, int timeoutMS);
	void Disconnect();
	bool IsConnected() const;
	int ServerVersion() const;
	std::string ConnectionTime() const;
	bool BootstrapComplete() const;
	std::string BlockedReason() const;
	int NextValidID() const;
	std::string ManagedAccountsCSV() const;

	bool RequestCurrentTime(int timeoutMS);
	long CurrentTime() const;

	bool RequestAccountSummary(int reqID, const std::string& group, const std::string& tags, int timeoutMS);
	int AccountSummaryCount() const;
	int AccountSummaryReqID(int index) const;
	std::string AccountSummaryAccount(int index) const;
	std::string AccountSummaryTag(int index) const;
	std::string AccountSummaryValue(int index) const;
	std::string AccountSummaryCurrency(int index) const;

	int ErrorCount() const;
	int ErrorID(int index) const;
	long ErrorTime(int index) const;
	int ErrorCode(int index) const;
	std::string ErrorMessage(int index) const;
	std::string ErrorAdvancedOrderRejectJSON(int index) const;

private:
	class Impl;
	Impl* impl_;
};

std::string IbkrSdkProbeCompileVersion();
