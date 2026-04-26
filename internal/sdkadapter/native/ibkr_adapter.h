#pragma once

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct ibkr_adapter ibkr_adapter;

typedef struct ibkr_string {
	char* data;
} ibkr_string;

typedef struct ibkr_error {
	char* operation;
	char* message;
	int req_id;
	long long order_id;
	int code;
	char* advanced_order_reject_json;
	char* phase;
} ibkr_error;

enum ibkr_event_kind {
	IBKR_EVENT_CONNECTION_METADATA = 1,
	IBKR_EVENT_CONNECTION_CLOSED = 2,
	IBKR_EVENT_NEXT_VALID_ID = 3,
	IBKR_EVENT_MANAGED_ACCOUNTS = 4,
	IBKR_EVENT_CURRENT_TIME = 5,
	IBKR_EVENT_ACCOUNT_SUMMARY = 6,
	IBKR_EVENT_ACCOUNT_SUMMARY_END = 7,
	IBKR_EVENT_API_ERROR = 8,
	IBKR_EVENT_ADAPTER_FATAL = 9
};

typedef struct ibkr_account_summary_event {
	int req_id;
	char* account;
	char* tag;
	char* value;
	char* currency;
} ibkr_account_summary_event;

typedef struct ibkr_api_error_event {
	int req_id;
	long long order_id;
	long long error_time;
	int code;
	char* message;
	char* advanced_order_reject_json;
} ibkr_api_error_event;

typedef struct ibkr_event {
	int kind;
	int req_id;
	int server_version;
	long long integer_value;
	char* text;
	ibkr_account_summary_event account_summary;
	ibkr_api_error_event api_error;
} ibkr_event;

typedef struct ibkr_event_batch {
	size_t count;
	ibkr_event* events;
} ibkr_event_batch;

void ibkr_error_clear(ibkr_error* error);

ibkr_adapter* ibkr_adapter_new(int queue_capacity, ibkr_error* error);
int ibkr_adapter_connect(ibkr_adapter* adapter, const char* host, int port, int client_id, int timeout_ms, ibkr_error* error);
void ibkr_adapter_disconnect(ibkr_adapter* adapter);
int ibkr_adapter_is_connected(ibkr_adapter* adapter);
int ibkr_adapter_server_version(ibkr_adapter* adapter);
int ibkr_adapter_connection_time(ibkr_adapter* adapter, ibkr_string* out, ibkr_error* error);
int ibkr_adapter_req_current_time(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_req_account_summary(ibkr_adapter* adapter, int req_id, const char* group, const char* tags, ibkr_error* error);
int ibkr_adapter_cancel_account_summary(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_drain_events(ibkr_adapter* adapter, int max_events, ibkr_event_batch** out, ibkr_error* error);
void ibkr_adapter_event_batch_free(ibkr_event_batch* batch);
void ibkr_string_free(ibkr_string value);
void ibkr_adapter_free(ibkr_adapter* adapter);

#ifdef __cplusplus
}
#endif
