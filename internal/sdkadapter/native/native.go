//go:build ibkr_sdk && cgo && linux

package native

/*
#include <stdlib.h>
#include "ibkr_adapter.h"
*/
import "C"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter"
)

type Adapter struct {
	mu     sync.Mutex
	handle *C.ibkr_adapter
	closed bool
}

func New(queueCapacity int) (*Adapter, error) {
	var cErr C.ibkr_error
	handle := C.ibkr_adapter_new(C.int(queueCapacity), &cErr)
	if handle == nil {
		defer C.ibkr_error_clear(&cErr)
		return nil, fromCError(cErr)
	}
	return &Adapter{handle: handle}, nil
}

func (a *Adapter) Connect(ctx context.Context, req sdkadapter.ConnectRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return sdkadapter.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	host := C.CString(req.Host)
	defer C.free(unsafe.Pointer(host))
	timeoutMS := int(req.Timeout / time.Millisecond)
	if timeoutMS <= 0 {
		timeoutMS = 1
	}
	var cErr C.ibkr_error
	ok := C.ibkr_adapter_connect(a.handle, host, C.int(req.Port), C.int(req.ClientID), C.int(timeoutMS), &cErr)
	if ok == 0 {
		defer C.ibkr_error_clear(&cErr)
		return fromCError(cErr)
	}
	return nil
}

func (a *Adapter) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	C.ibkr_adapter_disconnect(a.handle)
	return nil
}

func (a *Adapter) IsConnected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return false
	}
	return C.ibkr_adapter_is_connected(a.handle) != 0
}

func (a *Adapter) ServerVersion() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return 0
	}
	return int(C.ibkr_adapter_server_version(a.handle))
}

func (a *Adapter) ConnectionTime() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ""
	}
	var out C.ibkr_string
	var cErr C.ibkr_error
	ok := C.ibkr_adapter_connection_time(a.handle, &out, &cErr)
	if ok == 0 {
		C.ibkr_error_clear(&cErr)
		return ""
	}
	defer C.ibkr_string_free(out)
	return goString(out.data)
}

func (a *Adapter) Submit(ctx context.Context, command sdkadapter.Command) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return sdkadapter.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var cErr C.ibkr_error
	var ok C.int
	switch command.Kind {
	case sdkadapter.CommandCurrentTime:
		ok = C.ibkr_adapter_req_current_time(a.handle, &cErr)
	case sdkadapter.CommandAccountSummary:
		group := C.CString(command.Group)
		tags := C.CString(strings.Join(command.Tags, ","))
		ok = C.ibkr_adapter_req_account_summary(a.handle, C.int(command.ReqID), group, tags, &cErr)
		C.free(unsafe.Pointer(group))
		C.free(unsafe.Pointer(tags))
	case sdkadapter.CommandCancelAccountSummary:
		ok = C.ibkr_adapter_cancel_account_summary(a.handle, C.int(command.ReqID), &cErr)
	default:
		return sdkadapter.ErrUnsupportedCommand
	}
	if ok == 0 {
		defer C.ibkr_error_clear(&cErr)
		return fromCError(cErr)
	}
	return nil
}

func (a *Adapter) DrainEvents(ctx context.Context, maxEvents int) ([]sdkadapter.Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, sdkadapter.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var batch *C.ibkr_event_batch
	var cErr C.ibkr_error
	ok := C.ibkr_adapter_drain_events(a.handle, C.int(maxEvents), &batch, &cErr)
	if ok == 0 {
		defer C.ibkr_error_clear(&cErr)
		return nil, fromCError(cErr)
	}
	if batch == nil {
		return nil, nil
	}
	defer C.ibkr_adapter_event_batch_free(batch)
	rows := unsafe.Slice(batch.events, int(batch.count))
	events := make([]sdkadapter.Event, 0, len(rows))
	var managedAccounts []string
	for _, row := range rows {
		event := fromCEvent(row)
		if event.Kind == sdkadapter.EventManagedAccounts {
			managedAccounts = append(managedAccounts, event.Accounts...)
			continue
		}
		if len(managedAccounts) > 0 {
			events = append(events, sdkadapter.Event{Kind: sdkadapter.EventManagedAccounts, Accounts: managedAccounts})
			managedAccounts = nil
		}
		events = append(events, event)
	}
	if len(managedAccounts) > 0 {
		events = append(events, sdkadapter.Event{Kind: sdkadapter.EventManagedAccounts, Accounts: managedAccounts})
	}
	return events, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	C.ibkr_adapter_free(a.handle)
	a.handle = nil
	a.closed = true
	return nil
}

func fromCEvent(row C.ibkr_event) sdkadapter.Event {
	event := sdkadapter.Event{
		ReqID:          int(row.req_id),
		ServerVersion:  int(row.server_version),
		ConnectionTime: goString(row.text),
		CurrentTime:    int64(row.integer_value),
		NextValidID:    int64(row.integer_value),
	}
	switch row.kind {
	case C.IBKR_EVENT_CONNECTION_METADATA:
		event.Kind = sdkadapter.EventConnectionMetadata
	case C.IBKR_EVENT_CONNECTION_CLOSED:
		event.Kind = sdkadapter.EventConnectionClosed
	case C.IBKR_EVENT_NEXT_VALID_ID:
		event.Kind = sdkadapter.EventNextValidID
	case C.IBKR_EVENT_MANAGED_ACCOUNTS:
		event.Kind = sdkadapter.EventManagedAccounts
		if account := goString(row.text); account != "" {
			event.Accounts = []string{account}
		}
	case C.IBKR_EVENT_CURRENT_TIME:
		event.Kind = sdkadapter.EventCurrentTime
	case C.IBKR_EVENT_ACCOUNT_SUMMARY:
		event.Kind = sdkadapter.EventAccountSummary
		event.AccountSummary = sdkadapter.AccountSummaryValue{
			Account:  goString(row.account_summary.account),
			Tag:      goString(row.account_summary.tag),
			Value:    goString(row.account_summary.value),
			Currency: goString(row.account_summary.currency),
		}
	case C.IBKR_EVENT_ACCOUNT_SUMMARY_END:
		event.Kind = sdkadapter.EventAccountSummaryEnd
	case C.IBKR_EVENT_API_ERROR:
		event.Kind = sdkadapter.EventAPIError
		event.APIError = sdkadapter.Error{
			Op:                      "api",
			ReqID:                   int(row.api_error.req_id),
			OrderID:                 int64(row.api_error.order_id),
			Code:                    int(row.api_error.code),
			Message:                 goString(row.api_error.message),
			AdvancedOrderRejectJSON: goString(row.api_error.advanced_order_reject_json),
		}
	case C.IBKR_EVENT_ADAPTER_FATAL:
		event.Kind = sdkadapter.EventAdapterFatal
		event.FatalMessage = goString(row.text)
	default:
		event.Kind = sdkadapter.EventAdapterFatal
		event.FatalMessage = fmt.Sprintf("unknown native adapter event kind %d", int(row.kind))
	}
	return event
}

func fromCError(cErr C.ibkr_error) error {
	err := sdkadapter.Error{
		Op:                      goString(cErr.operation),
		ReqID:                   int(cErr.req_id),
		OrderID:                 int64(cErr.order_id),
		Code:                    int(cErr.code),
		Message:                 goString(cErr.message),
		AdvancedOrderRejectJSON: goString(cErr.advanced_order_reject_json),
		Phase:                   goString(cErr.phase),
	}
	if err.Message == "" {
		err.Message = "native SDK adapter error"
	}
	return err
}

func goString(value *C.char) string {
	if value == nil {
		return ""
	}
	return C.GoString(value)
}
