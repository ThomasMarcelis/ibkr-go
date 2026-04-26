//go:build ibkr_swig && cgo && linux

#include "ibkr_swig_probe.hpp"

namespace {

int forceOfficialSDKLink(EWrapper *wrapper, EReaderSignal *signal) {
	EClientSocket client(wrapper, signal);
	return 1;
}

volatile auto officialSDKLinkAnchor = &forceOfficialSDKLink;

} // namespace

std::string IbkrSdkProbeCompileVersion() {
	return "ibkr-swig-probe";
}

