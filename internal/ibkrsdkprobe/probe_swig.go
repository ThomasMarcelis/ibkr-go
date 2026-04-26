//go:build ibkr_swig && cgo && linux

package ibkrsdkprobe

/*
#cgo CXXFLAGS: -std=c++14
#cgo linux LDFLAGS: -lstdc++
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const defaultProbeTimeout = 15 * time.Second

type Probe struct {
	sdk OfficialSDKProbe
}

type Snapshot struct {
	Connected          bool
	ServerVersion      int
	ConnectionTime     string
	BootstrapComplete  bool
	BlockedReason      string
	NextValidID        int
	ManagedAccountsCSV string
	CurrentTimeUnix    int64
	AccountSummary     []AccountSummaryRow
	Errors             []SDKError
}

type SDKError struct {
	ID                      int
	Time                    int64
	Code                    int
	Message                 string
	AdvancedOrderRejectJSON string
}

type AccountSummaryRow struct {
	ReqID    int
	Account  string
	Tag      string
	Value    string
	Currency string
}

// CompileProbeVersion returns a marker from the SWIG-generated wrapper. Calling
// it proves that Go, cgo, SWIG, the local adapter, and the official SDK headers
// were compiled into the same package.
func CompileProbeVersion() string {
	return IbkrSdkProbeCompileVersion()
}

func NewProbe() *Probe {
	return &Probe{sdk: NewOfficialSDKProbe()}
}

func (p *Probe) Close() {
	if p == nil || p.sdk == nil {
		return
	}
	p.sdk.Disconnect()
	DeleteOfficialSDKProbe(p.sdk)
	p.sdk = nil
}

func (p *Probe) Connect(ctx context.Context, host string, port int, clientID int) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if !p.sdk.Connect(host, port, clientID, timeoutMS(ctx)) {
		snapshot := p.Snapshot()
		return snapshot, fmt.Errorf("official SDK bootstrap blocked: %s", snapshot.BlockedReason)
	}
	return p.Snapshot(), nil
}

func (p *Probe) Disconnect() {
	if p == nil || p.sdk == nil {
		return
	}
	p.sdk.Disconnect()
}

func (p *Probe) RequestCurrentTime(ctx context.Context) (time.Time, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	if !p.sdk.RequestCurrentTime(timeoutMS(ctx)) {
		return time.Time{}, fmt.Errorf("official SDK current time blocked: %s", p.sdk.BlockedReason())
	}
	return time.Unix(p.sdk.CurrentTime(), 0).UTC(), nil
}

func (p *Probe) RequestAccountSummary(ctx context.Context, reqID int, group string, tags []string) ([]AccountSummaryRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if group == "" {
		group = "All"
	}
	if !p.sdk.RequestAccountSummary(reqID, group, strings.Join(tags, ","), timeoutMS(ctx)) {
		return nil, fmt.Errorf("official SDK account summary blocked: %s", p.sdk.BlockedReason())
	}
	return p.accountSummary(), nil
}

func (p *Probe) Snapshot() Snapshot {
	if p == nil || p.sdk == nil {
		return Snapshot{}
	}
	return Snapshot{
		Connected:          p.sdk.IsConnected(),
		ServerVersion:      p.sdk.ServerVersion(),
		ConnectionTime:     p.sdk.ConnectionTime(),
		BootstrapComplete:  p.sdk.BootstrapComplete(),
		BlockedReason:      p.sdk.BlockedReason(),
		NextValidID:        p.sdk.NextValidID(),
		ManagedAccountsCSV: p.sdk.ManagedAccountsCSV(),
		CurrentTimeUnix:    p.sdk.CurrentTime(),
		AccountSummary:     p.accountSummary(),
		Errors:             p.errors(),
	}
}

func cgoEnabled() {
	_ = C.int(0)
}

func timeoutMS(ctx context.Context) int {
	deadline, ok := ctx.Deadline()
	if !ok {
		return int(defaultProbeTimeout / time.Millisecond)
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 1
	}
	return int(remaining / time.Millisecond)
}

func (p *Probe) accountSummary() []AccountSummaryRow {
	count := p.sdk.AccountSummaryCount()
	rows := make([]AccountSummaryRow, 0, count)
	for i := range count {
		rows = append(rows, AccountSummaryRow{
			ReqID:    p.sdk.AccountSummaryReqID(i),
			Account:  p.sdk.AccountSummaryAccount(i),
			Tag:      p.sdk.AccountSummaryTag(i),
			Value:    p.sdk.AccountSummaryValue(i),
			Currency: p.sdk.AccountSummaryCurrency(i),
		})
	}
	return rows
}

func (p *Probe) errors() []SDKError {
	count := p.sdk.ErrorCount()
	errors := make([]SDKError, 0, count)
	for i := range count {
		errors = append(errors, SDKError{
			ID:                      p.sdk.ErrorID(i),
			Time:                    p.sdk.ErrorTime(i),
			Code:                    p.sdk.ErrorCode(i),
			Message:                 p.sdk.ErrorMessage(i),
			AdvancedOrderRejectJSON: p.sdk.ErrorAdvancedOrderRejectJSON(i),
		})
	}
	return errors
}
