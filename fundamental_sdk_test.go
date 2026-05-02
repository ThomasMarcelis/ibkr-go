package ibkr

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

func TestSDKFundamentalDataPublicRouteReplaysOfficialFixture(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan struct {
		report XMLDocument
		err    error
	}, 1)
	go func() {
		report, err := client.Contracts().FundamentalData(ctx, FundamentalDataRequest{
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			ReportType: FundamentalReportSnapshot,
		})
		resultCh <- struct {
			report XMLDocument
			err    error
		}{report: report, err: err}
	}()

	runNextEngineCommand(t, e)
	command := onlySDKCommand(t, adapter)
	if command.Kind != sdkadapter.CommandFundamentalData {
		t.Fatalf("command kind = %s, want %s", command.Kind, sdkadapter.CommandFundamentalData)
	}
	if command.FundamentalData.ReportType != "ReportSnapshot" ||
		command.FundamentalData.Contract.Symbol != "AAPL" ||
		command.FundamentalData.Contract.Exchange != "SMART" {
		t.Fatalf("fundamental data command = %+v, want AAPL ReportSnapshot", command.FundamentalData)
	}

	event := fixtureEvent(t, "internal/sdkadapter/testdata/fixtures/official_sdk_fundamental_data_snapshot_20260502.json", sdkadapter.EventFundamentalData, 501)
	event.ReqID = command.FundamentalData.ReqID
	dispatchSDKFixtureEvent(t, e, event)

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("Contracts().FundamentalData() error = %v", result.err)
		}
		report := string(result.report)
		for _, want := range []string{
			"<ReportSnapshot",
			`<CoID Type="CompanyName">Apple Inc</CoID>`,
			`<IssueID Type="Ticker">AAPL</IssueID>`,
		} {
			if !strings.Contains(report, want) {
				t.Fatalf("Contracts().FundamentalData() report does not contain %q", want)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("Contracts().FundamentalData() did not return")
	}
}

func TestSDKFundamentalDataContextCancelSendsCancel(t *testing.T) {
	t.Parallel()

	adapter := sdkadapter.NewReplayAdapter(nil)
	e := newManualReadySDKEngine(adapter)
	client := &Client{engine: e}

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := client.Contracts().FundamentalData(ctx, FundamentalDataRequest{
			Contract: Contract{
				Symbol:   "AAPL",
				SecType:  SecTypeStock,
				Exchange: "SMART",
				Currency: "USD",
			},
			ReportType: FundamentalReportSnapshot,
		})
		resultCh <- err
	}()

	runNextEngineCommand(t, e)
	first := onlySDKCommand(t, adapter)
	if first.Kind != sdkadapter.CommandFundamentalData {
		t.Fatalf("command kind = %s, want %s", first.Kind, sdkadapter.CommandFundamentalData)
	}
	cancel()

	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal("Contracts().FundamentalData() error = nil, want context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("Contracts().FundamentalData() did not return after context cancel")
	}

	runNextEngineCommand(t, e)
	commands := adapter.Commands()
	if len(commands) != 2 {
		t.Fatalf("commands len = %d, want fundamental request plus cancel: %+v", len(commands), commands)
	}
	if commands[1].Kind != sdkadapter.CommandCancelFundamentalData {
		t.Fatalf("cancel command kind = %s, want %s", commands[1].Kind, sdkadapter.CommandCancelFundamentalData)
	}
	if commands[1].CancelFundamentalData.ReqID != first.FundamentalData.ReqID {
		t.Fatalf("cancel reqID = %d, want %d", commands[1].CancelFundamentalData.ReqID, first.FundamentalData.ReqID)
	}
}
