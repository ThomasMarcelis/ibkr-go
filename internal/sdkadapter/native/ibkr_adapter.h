#pragma once

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct ibkr_adapter ibkr_adapter;

typedef struct ibkr_string {
	char* data;
} ibkr_string;

typedef struct ibkr_build_info_result {
	char* adapter_abi_version;
	char* sdk_api_version;
	char* compiler;
	char* protobuf_mode;
} ibkr_build_info_result;

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
	IBKR_EVENT_ADAPTER_FATAL = 9,
	IBKR_EVENT_CONTRACT_DETAILS = 10,
	IBKR_EVENT_CONTRACT_DETAILS_END = 11,
	IBKR_EVENT_POSITION = 12,
	IBKR_EVENT_POSITION_END = 13,
	IBKR_EVENT_CURRENT_TIME_MILLIS = 14,
	IBKR_EVENT_FAMILY_CODES = 15,
	IBKR_EVENT_MKT_DEPTH_EXCHANGES = 16,
	IBKR_EVENT_NEWS_PROVIDERS = 17,
	IBKR_EVENT_SCANNER_PARAMETERS = 18,
	IBKR_EVENT_USER_INFO = 19,
	IBKR_EVENT_SOFT_DOLLAR_TIERS = 20,
	IBKR_EVENT_DISPLAY_GROUP_LIST = 21,
	IBKR_EVENT_MATCHING_SYMBOLS = 22,
	IBKR_EVENT_MARKET_RULE = 23,
	IBKR_EVENT_SMART_COMPONENTS = 24,
	IBKR_EVENT_FUNDAMENTAL_DATA = 25,
	IBKR_EVENT_SEC_DEF_OPT_PARAMS = 26,
	IBKR_EVENT_SEC_DEF_OPT_PARAMS_END = 27,
	IBKR_EVENT_NEWS_ARTICLE = 28,
	IBKR_EVENT_HISTORICAL_NEWS = 29,
	IBKR_EVENT_HISTORICAL_NEWS_END = 30,
	IBKR_EVENT_RECEIVE_FA = 31,
	IBKR_EVENT_HEAD_TIMESTAMP = 32,
	IBKR_EVENT_HISTOGRAM_DATA = 33,
	IBKR_EVENT_WSH_META_DATA = 34,
	IBKR_EVENT_WSH_EVENT_DATA = 35,
	IBKR_EVENT_MARKET_DATA_TYPE = 36,
	IBKR_EVENT_TICK_OPTION_COMPUTATION = 37,
	IBKR_EVENT_SCANNER_DATA = 38,
	IBKR_EVENT_UPDATE_ACCOUNT_VALUE = 39,
	IBKR_EVENT_UPDATE_PORTFOLIO = 40,
	IBKR_EVENT_UPDATE_ACCOUNT_TIME = 41,
	IBKR_EVENT_ACCOUNT_DOWNLOAD_END = 42,
	IBKR_EVENT_ACCOUNT_UPDATE_MULTI = 43,
	IBKR_EVENT_ACCOUNT_UPDATE_MULTI_END = 44,
	IBKR_EVENT_POSITION_MULTI = 45,
	IBKR_EVENT_POSITION_MULTI_END = 46,
	IBKR_EVENT_PNL = 47,
	IBKR_EVENT_PNL_SINGLE = 48,
	IBKR_EVENT_NEWS_BULLETIN = 49,
	IBKR_EVENT_DISPLAY_GROUP_UPDATED = 50,
	IBKR_EVENT_HISTORICAL_DATA = 51,
	IBKR_EVENT_HISTORICAL_DATA_END = 52,
	IBKR_EVENT_HISTORICAL_DATA_UPDATE = 53,
	IBKR_EVENT_HISTORICAL_SCHEDULE = 54,
	IBKR_EVENT_HISTORICAL_TICKS = 55,
	IBKR_EVENT_HISTORICAL_TICKS_BID_ASK = 56,
	IBKR_EVENT_HISTORICAL_TICKS_LAST = 57,
	IBKR_EVENT_REAL_TIME_BAR = 58,
	IBKR_EVENT_TICK_BY_TICK = 59,
	IBKR_EVENT_MARKET_DEPTH = 60,
	IBKR_EVENT_MARKET_DEPTH_L2 = 61,
	IBKR_EVENT_TICK_PRICE = 62,
	IBKR_EVENT_TICK_SIZE = 63,
	IBKR_EVENT_TICK_GENERIC = 64,
	IBKR_EVENT_TICK_STRING = 65,
	IBKR_EVENT_TICK_SNAPSHOT_END = 66,
	IBKR_EVENT_TICK_REQ_PARAMS = 67,
	IBKR_EVENT_REPLACE_FA_END = 68,
	IBKR_EVENT_OPEN_ORDER = 69,
	IBKR_EVENT_OPEN_ORDER_END = 70,
	IBKR_EVENT_COMPLETED_ORDER = 71,
	IBKR_EVENT_COMPLETED_ORDER_END = 72,
	IBKR_EVENT_ORDER_STATUS = 73,
	IBKR_EVENT_EXECUTION_DETAIL = 74,
	IBKR_EVENT_EXECUTIONS_END = 75,
	IBKR_EVENT_COMMISSION_REPORT = 76,
	IBKR_EVENT_BOND_CONTRACT_DETAILS = 77
};

typedef struct ibkr_contract {
	int con_id;
	char* symbol;
	char* sec_type;
	char* expiry;
	char* strike;
	char* right;
	char* multiplier;
	char* exchange;
	char* currency;
	char* local_symbol;
	char* trading_class;
	char* primary_exchange;
} ibkr_contract;

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

typedef struct ibkr_contract_details_event {
	int req_id;
	ibkr_contract contract;
	char* market_name;
	char* min_tick;
	char* long_name;
	char* time_zone_id;
} ibkr_contract_details_event;

typedef struct ibkr_position_event {
	char* account;
	ibkr_contract contract;
	char* position;
	char* avg_cost;
} ibkr_position_event;

typedef struct ibkr_tick_option_computation_event {
	int tick_type;
	int tick_attrib;
	char* implied_vol;
	char* delta;
	char* opt_price;
	char* pv_dividend;
	char* gamma;
	char* vega;
	char* theta;
	char* und_price;
} ibkr_tick_option_computation_event;

typedef struct ibkr_family_code_event {
	char* account_id;
	char* family_code;
} ibkr_family_code_event;

typedef struct ibkr_depth_exchange_event {
	char* exchange;
	char* sec_type;
	char* listing_exch;
	char* service_data_type;
	int agg_group;
} ibkr_depth_exchange_event;

typedef struct ibkr_news_provider_event {
	char* code;
	char* name;
} ibkr_news_provider_event;

typedef struct ibkr_soft_dollar_tier_event {
	char* name;
	char* value;
	char* display_name;
} ibkr_soft_dollar_tier_event;

typedef struct ibkr_symbol_sample_event {
	int con_id;
	char* symbol;
	char* sec_type;
	char* primary_exchange;
	char* currency;
	size_t derivative_sec_types_count;
	char** derivative_sec_types;
	char* description;
	char* issuer_id;
} ibkr_symbol_sample_event;

typedef struct ibkr_price_increment_event {
	char* low_edge;
	char* increment;
} ibkr_price_increment_event;

typedef struct ibkr_smart_component_event {
	int bit_number;
	char* exchange_name;
	char* exchange_letter;
} ibkr_smart_component_event;

typedef struct ibkr_sec_def_opt_params_event {
	char* exchange;
	int underlying_con_id;
	char* trading_class;
	char* multiplier;
	size_t expirations_count;
	char** expirations;
	size_t strikes_count;
	char** strikes;
} ibkr_sec_def_opt_params_event;

typedef struct ibkr_historical_news_event {
	char* time;
	char* provider_code;
	char* article_id;
	char* headline;
} ibkr_historical_news_event;

typedef struct ibkr_histogram_data_event {
	char* price;
	char* size;
} ibkr_histogram_data_event;

typedef struct ibkr_scanner_data_event {
	int rank;
	ibkr_contract contract;
	char* distance;
	char* benchmark;
	char* projection;
	char* legs_str;
} ibkr_scanner_data_event;

typedef struct ibkr_account_value_event {
	char* key;
	char* value;
	char* currency;
	char* account;
} ibkr_account_value_event;

typedef struct ibkr_portfolio_event {
	char* account;
	ibkr_contract contract;
	char* position;
	char* market_price;
	char* market_value;
	char* avg_cost;
	char* unrealized_pnl;
	char* realized_pnl;
} ibkr_portfolio_event;

typedef struct ibkr_account_update_multi_event {
	char* account;
	char* model_code;
	char* key;
	char* value;
	char* currency;
} ibkr_account_update_multi_event;

typedef struct ibkr_position_multi_event {
	char* account;
	char* model_code;
	ibkr_contract contract;
	char* position;
	char* avg_cost;
} ibkr_position_multi_event;

typedef struct ibkr_pnl_event {
	char* daily_pnl;
	char* unrealized_pnl;
	char* realized_pnl;
} ibkr_pnl_event;

typedef struct ibkr_pnl_single_event {
	char* position;
	char* daily_pnl;
	char* unrealized_pnl;
	char* realized_pnl;
	char* value;
} ibkr_pnl_single_event;

typedef struct ibkr_combo_leg_event {
	int con_id;
	int ratio;
	char* action;
	char* exchange;
	char* open_close;
	char* short_sale_slot;
	char* designated_location;
	char* exempt_code;
} ibkr_combo_leg_event;

typedef struct ibkr_tag_value_event {
	char* tag;
	char* value;
} ibkr_tag_value_event;

typedef struct ibkr_order_condition_event {
	int condition_type;
	char* conjunction;
	int con_id;
	char* exchange;
	int operator_value;
	char* value;
	int trigger_method;
	char* sec_type;
	char* symbol;
} ibkr_order_condition_event;

typedef struct ibkr_open_order_event {
	long long order_id;
	ibkr_contract contract;
	char* action;
	char* quantity;
	char* order_type;
	char* lmt_price;
	char* aux_price;
	char* tif;
	char* oca_group;
	char* account;
	char* open_close;
	char* origin;
	char* order_ref;
	char* client_id;
	char* perm_id;
	char* outside_rth;
	char* hidden;
	char* discretion_amt;
	char* good_after_time;
	size_t combo_legs_count;
	ibkr_combo_leg_event* combo_legs;
	size_t order_combo_leg_prices_count;
	char** order_combo_leg_prices;
	size_t smart_combo_routing_count;
	ibkr_tag_value_event* smart_combo_routing;
	char* algo_strategy;
	size_t algo_params_count;
	ibkr_tag_value_event* algo_params;
	size_t conditions_count;
	ibkr_order_condition_event* conditions;
	char* conditions_ignore_rth;
	char* conditions_cancel_order;
	char* status;
	char* init_margin_before;
	char* maint_margin_before;
	char* equity_with_loan_before;
	char* init_margin_change;
	char* maint_margin_change;
	char* equity_with_loan_change;
	char* init_margin_after;
	char* maint_margin_after;
	char* equity_with_loan_after;
	char* commission;
	char* min_commission;
	char* max_commission;
	char* commission_currency;
	char* warning_text;
	char* filled;
	char* remaining;
	char* parent_id;
} ibkr_open_order_event;

typedef struct ibkr_completed_order_event {
	ibkr_contract contract;
	char* action;
	char* order_type;
	char* status;
	char* quantity;
	char* filled;
	char* remaining;
} ibkr_completed_order_event;

typedef struct ibkr_place_order_request {
	long long order_id;
	ibkr_contract contract;
	char* action;
	char* total_quantity;
	char* order_type;
	char* lmt_price;
	char* aux_price;
	char* tif;
	char* oca_group;
	char* account;
	char* open_close;
	char* origin;
	char* order_ref;
	char* transmit;
	char* parent_id;
	char* block_order;
	char* sweep_to_fill;
	char* display_size;
	char* trigger_method;
	char* outside_rth;
	char* hidden;
	size_t combo_legs_count;
	ibkr_combo_leg_event* combo_legs;
	size_t order_combo_leg_prices_count;
	char** order_combo_leg_prices;
	size_t smart_combo_routing_params_count;
	ibkr_tag_value_event* smart_combo_routing_params;
	char* fa_group;
	char* fa_method;
	char* fa_percentage;
	char* model_code;
	char* short_sale_slot;
	char* designated_location;
	char* exempt_code;
	char* discretionary_amt;
	char* good_after_time;
	char* good_till_date;
	char* oca_type;
	char* rule80a;
	char* settling_firm;
	char* all_or_none;
	char* min_qty;
	char* percent_offset;
	char* auction_strategy;
	char* starting_price;
	char* stock_ref_price;
	char* delta;
	char* stock_range_lower;
	char* stock_range_upper;
	char* override_percentage_constraints;
	char* volatility;
	char* volatility_type;
	char* delta_neutral_order_type;
	char* delta_neutral_aux_price;
	char* continuous_update;
	char* reference_price_type;
	char* trail_stop_price;
	char* trailing_percent;
	char* scale_init_level_size;
	char* scale_subs_level_size;
	char* scale_price_increment;
	char* scale_table;
	char* active_start_time;
	char* active_stop_time;
	char* hedge_type;
	char* hedge_param;
	char* opt_out_smart_routing;
	char* clearing_account;
	char* clearing_intent;
	char* not_held;
	char* delta_neutral_contract_present;
	char* algo_strategy;
	size_t algo_params_count;
	ibkr_tag_value_event* algo_params;
	char* algo_id;
	char* what_if;
	char* order_misc_options;
	char* solicited;
	char* randomize_size;
	char* randomize_price;
	size_t conditions_count;
	ibkr_order_condition_event* conditions;
	char* conditions_ignore_rth;
	char* conditions_cancel_order;
	char* adjusted_order_type;
	char* trigger_price;
	char* lmt_price_offset;
	char* adjusted_stop_price;
	char* adjusted_stop_limit_price;
	char* adjusted_trailing_amount;
	char* adjustable_trailing_unit;
	char* ext_operator;
	char* soft_dollar_name;
	char* soft_dollar_value;
	char* cash_qty;
	char* mifid2_decision_maker;
	char* mifid2_decision_algo;
	char* mifid2_execution_trader;
	char* mifid2_execution_algo;
	char* dont_use_auto_price_for_hedge;
	char* is_oms_container;
	char* discretionary_up_to_limit_price;
	char* use_price_mgmt_algo;
	char* duration;
	char* post_to_ats;
	char* auto_cancel_parent;
	char* advanced_error_override;
	char* manual_order_time;
	char* customer_account;
	char* professional_customer;
	char* include_overnight;
	char* manual_order_indicator;
	char* imbalance_only;
} ibkr_place_order_request;

typedef struct ibkr_order_status_event {
	long long order_id;
	char* status;
	char* filled;
	char* remaining;
	char* avg_fill_price;
	char* perm_id;
	char* parent_id;
	char* last_fill_price;
	char* client_id;
	char* why_held;
	char* mkt_cap_price;
} ibkr_order_status_event;

typedef struct ibkr_execution_detail_event {
	long long order_id;
	char* exec_id;
	char* account;
	char* symbol;
	char* side;
	char* shares;
	char* price;
	char* time;
} ibkr_execution_detail_event;

typedef struct ibkr_commission_report_event {
	char* exec_id;
	char* commission;
	char* currency;
	char* realized_pnl;
} ibkr_commission_report_event;

typedef struct ibkr_news_bulletin_event {
	int msg_id;
	int msg_type;
	char* headline;
	char* source;
} ibkr_news_bulletin_event;

typedef struct ibkr_historical_bar_event {
	char* time;
	char* open;
	char* high;
	char* low;
	char* close;
	char* volume;
	char* wap;
	char* count;
} ibkr_historical_bar_event;

typedef struct ibkr_historical_schedule_session_event {
	char* start_date_time;
	char* end_date_time;
	char* ref_date;
} ibkr_historical_schedule_session_event;

typedef struct ibkr_historical_schedule_event {
	char* start_date_time;
	char* end_date_time;
	char* time_zone;
	size_t sessions_count;
	ibkr_historical_schedule_session_event* sessions;
} ibkr_historical_schedule_event;

typedef struct ibkr_historical_tick_event {
	char* time;
	char* price;
	char* size;
} ibkr_historical_tick_event;

typedef struct ibkr_historical_tick_bid_ask_event {
	int tick_attrib;
	char* time;
	char* bid_price;
	char* ask_price;
	char* bid_size;
	char* ask_size;
} ibkr_historical_tick_bid_ask_event;

typedef struct ibkr_historical_tick_last_event {
	int tick_attrib;
	char* time;
	char* price;
	char* size;
	char* exchange;
	char* special_conditions;
} ibkr_historical_tick_last_event;

typedef struct ibkr_tick_by_tick_event {
	int tick_type;
	char* time;
	char* price;
	char* size;
	char* exchange;
	char* special_conditions;
	char* bid_price;
	char* ask_price;
	char* bid_size;
	char* ask_size;
	char* midpoint;
	int tick_attrib_last;
	int tick_attrib_bid_ask;
} ibkr_tick_by_tick_event;

typedef struct ibkr_market_depth_event {
	int position;
	int operation;
	int side;
	char* price;
	char* size;
} ibkr_market_depth_event;

typedef struct ibkr_market_depth_l2_event {
	int position;
	char* market_maker;
	int operation;
	int side;
	char* price;
	char* size;
	int is_smart_depth;
} ibkr_market_depth_l2_event;

typedef struct ibkr_tick_price_event {
	int tick_type;
	char* price;
	char* size;
	int attr_mask;
} ibkr_tick_price_event;

typedef struct ibkr_tick_size_event {
	int tick_type;
	char* size;
} ibkr_tick_size_event;

typedef struct ibkr_tick_value_event {
	int tick_type;
	char* value;
} ibkr_tick_value_event;

typedef struct ibkr_tick_req_params_event {
	char* min_tick;
	char* bbo_exchange;
	int snapshot_permissions;
} ibkr_tick_req_params_event;

typedef struct ibkr_event {
	int kind;
	int req_id;
	int server_version;
	long long integer_value;
	char* text;
	ibkr_account_summary_event account_summary;
	ibkr_api_error_event api_error;
	ibkr_contract_details_event contract_details;
	ibkr_position_event position;
	ibkr_tick_option_computation_event tick_option_computation;
	size_t family_codes_count;
	ibkr_family_code_event* family_codes;
	size_t depth_exchanges_count;
	ibkr_depth_exchange_event* depth_exchanges;
	size_t news_providers_count;
	ibkr_news_provider_event* news_providers;
	size_t soft_dollar_tiers_count;
	ibkr_soft_dollar_tier_event* soft_dollar_tiers;
	size_t symbol_samples_count;
	ibkr_symbol_sample_event* symbol_samples;
	int market_rule_id;
	size_t price_increments_count;
	ibkr_price_increment_event* price_increments;
	size_t smart_components_count;
	ibkr_smart_component_event* smart_components;
	size_t sec_def_opt_params_count;
	ibkr_sec_def_opt_params_event* sec_def_opt_params;
	ibkr_historical_news_event historical_news;
	size_t histogram_data_count;
	ibkr_histogram_data_event* histogram_data;
	size_t scanner_data_count;
	ibkr_scanner_data_event* scanner_data;
	ibkr_account_value_event account_value;
	ibkr_portfolio_event portfolio;
	ibkr_account_update_multi_event account_update_multi;
	ibkr_position_multi_event position_multi;
	ibkr_pnl_event pnl;
	ibkr_pnl_single_event pnl_single;
	ibkr_open_order_event open_order;
	ibkr_completed_order_event completed_order;
	ibkr_order_status_event order_status;
	ibkr_execution_detail_event execution_detail;
	ibkr_commission_report_event commission_report;
	ibkr_news_bulletin_event news_bulletin;
	ibkr_historical_bar_event historical_bar;
	ibkr_historical_bar_event real_time_bar;
	ibkr_tick_by_tick_event tick_by_tick;
	ibkr_market_depth_event market_depth;
	ibkr_market_depth_l2_event market_depth_l2;
	ibkr_tick_price_event tick_price;
	ibkr_tick_size_event tick_size;
	ibkr_tick_value_event tick_generic;
	ibkr_tick_value_event tick_string;
	ibkr_tick_req_params_event tick_req_params;
	ibkr_historical_schedule_event historical_schedule;
	size_t historical_ticks_count;
	ibkr_historical_tick_event* historical_ticks;
	size_t historical_ticks_bid_ask_count;
	ibkr_historical_tick_bid_ask_event* historical_ticks_bid_ask;
	size_t historical_ticks_last_count;
	ibkr_historical_tick_last_event* historical_ticks_last;
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
int ibkr_adapter_req_current_time_millis(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_req_account_summary(ibkr_adapter* adapter, int req_id, const char* group, const char* tags, ibkr_error* error);
int ibkr_adapter_cancel_account_summary(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_account_updates(ibkr_adapter* adapter, int subscribe, const char* account, ibkr_error* error);
int ibkr_adapter_req_account_updates_multi(ibkr_adapter* adapter, int req_id, const char* account, const char* model_code, ibkr_error* error);
int ibkr_adapter_cancel_account_updates_multi(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_contract_details(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, ibkr_error* error);
int ibkr_adapter_req_positions(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_cancel_positions(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_req_positions_multi(ibkr_adapter* adapter, int req_id, const char* account, const char* model_code, ibkr_error* error);
int ibkr_adapter_cancel_positions_multi(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_pnl(ibkr_adapter* adapter, int req_id, const char* account, const char* model_code, ibkr_error* error);
int ibkr_adapter_cancel_pnl(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_pnl_single(ibkr_adapter* adapter, int req_id, const char* account, const char* model_code, int con_id, ibkr_error* error);
int ibkr_adapter_cancel_pnl_single(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_market_data_type(ibkr_adapter* adapter, int market_data_type, ibkr_error* error);
int ibkr_adapter_req_mkt_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* generic_ticks, int snapshot, ibkr_error* error);
int ibkr_adapter_cancel_mkt_data(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_real_time_bars(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* what_to_show, int use_rth, ibkr_error* error);
int ibkr_adapter_cancel_real_time_bars(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_tick_by_tick_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* tick_type, int number_of_ticks, int ignore_size, ibkr_error* error);
int ibkr_adapter_cancel_tick_by_tick_data(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_mkt_depth(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, int num_rows, int is_smart_depth, ibkr_error* error);
int ibkr_adapter_cancel_mkt_depth(ibkr_adapter* adapter, int req_id, int is_smart_depth, ibkr_error* error);
int ibkr_adapter_calc_implied_volatility(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* option_price, const char* under_price, ibkr_error* error);
int ibkr_adapter_cancel_calc_implied_volatility(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_calc_option_price(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* volatility, const char* under_price, ibkr_error* error);
int ibkr_adapter_cancel_calc_option_price(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_exercise_options(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, int exercise_action, int exercise_quantity, const char* account, int override, ibkr_error* error);
int ibkr_adapter_place_order(ibkr_adapter* adapter, const ibkr_place_order_request* request, ibkr_error* error);
int ibkr_adapter_req_open_orders(ibkr_adapter* adapter, const char* scope, ibkr_error* error);
int ibkr_adapter_req_completed_orders(ibkr_adapter* adapter, int api_only, ibkr_error* error);
int ibkr_adapter_cancel_order(ibkr_adapter* adapter, long long order_id, const char* manual_order_cancel_time, const char* ext_operator, const char* manual_order_indicator, ibkr_error* error);
int ibkr_adapter_req_global_cancel(ibkr_adapter* adapter, const char* ext_operator, const char* manual_order_indicator, ibkr_error* error);
int ibkr_adapter_req_executions(ibkr_adapter* adapter, int req_id, const char* account, const char* symbol, ibkr_error* error);
int ibkr_adapter_req_family_codes(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_req_mkt_depth_exchanges(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_req_news_providers(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_req_news_bulletins(ibkr_adapter* adapter, int all_messages, ibkr_error* error);
int ibkr_adapter_cancel_news_bulletins(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_req_news_article(ibkr_adapter* adapter, int req_id, const char* provider_code, const char* article_id, ibkr_error* error);
int ibkr_adapter_req_historical_news(ibkr_adapter* adapter, int req_id, int con_id, const char* provider_codes, const char* start_date_time, const char* end_date_time, int total_results, ibkr_error* error);
int ibkr_adapter_req_scanner_parameters(ibkr_adapter* adapter, ibkr_error* error);
int ibkr_adapter_req_scanner_subscription(ibkr_adapter* adapter, int req_id, int number_of_rows, const char* instrument, const char* location_code, const char* scan_code, ibkr_error* error);
int ibkr_adapter_cancel_scanner_subscription(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_request_fa(ibkr_adapter* adapter, int fa_data_type, ibkr_error* error);
int ibkr_adapter_replace_fa(ibkr_adapter* adapter, int req_id, int fa_data_type, const char* xml, ibkr_error* error);
int ibkr_adapter_req_historical_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* end_date_time, const char* duration, const char* bar_size, const char* what_to_show, int use_rth, int keep_up_to_date, ibkr_error* error);
int ibkr_adapter_cancel_historical_data(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_historical_ticks(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* start_date_time, const char* end_date_time, int number_of_ticks, const char* what_to_show, int use_rth, int ignore_size, ibkr_error* error);
int ibkr_adapter_cancel_historical_ticks(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_head_timestamp(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* what_to_show, int use_rth, ibkr_error* error);
int ibkr_adapter_cancel_head_timestamp(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_histogram_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, int use_rth, const char* period, ibkr_error* error);
int ibkr_adapter_cancel_histogram_data(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_wsh_meta_data(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_cancel_wsh_meta_data(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_wsh_event_data(ibkr_adapter* adapter, int req_id, int con_id, const char* filter, int fill_watchlist, int fill_portfolio, int fill_competitors, const char* start_date, const char* end_date, int total_limit, ibkr_error* error);
int ibkr_adapter_cancel_wsh_event_data(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_user_info(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_soft_dollar_tiers(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_query_display_groups(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_subscribe_to_group_events(ibkr_adapter* adapter, int req_id, int group_id, ibkr_error* error);
int ibkr_adapter_update_display_group(ibkr_adapter* adapter, int req_id, const char* contract_info, ibkr_error* error);
int ibkr_adapter_unsubscribe_from_group_events(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_req_matching_symbols(ibkr_adapter* adapter, int req_id, const char* pattern, ibkr_error* error);
int ibkr_adapter_req_market_rule(ibkr_adapter* adapter, int market_rule_id, ibkr_error* error);
int ibkr_adapter_req_sec_def_opt_params(ibkr_adapter* adapter, int req_id, const char* underlying_symbol, const char* fut_fop_exchange, const char* underlying_sec_type, int underlying_con_id, ibkr_error* error);
int ibkr_adapter_req_smart_components(ibkr_adapter* adapter, int req_id, const char* bbo_exchange, ibkr_error* error);
int ibkr_adapter_req_fundamental_data(ibkr_adapter* adapter, int req_id, const ibkr_contract* contract, const char* report_type, ibkr_error* error);
int ibkr_adapter_cancel_fundamental_data(ibkr_adapter* adapter, int req_id, ibkr_error* error);
int ibkr_adapter_drain_events(ibkr_adapter* adapter, int max_events, ibkr_event_batch** out, ibkr_error* error);
void ibkr_adapter_event_batch_free(ibkr_event_batch* batch);
void ibkr_string_free(ibkr_string value);
int ibkr_build_info(ibkr_build_info_result* out, ibkr_error* error);
void ibkr_build_info_free(ibkr_build_info_result value);
void ibkr_adapter_free(ibkr_adapter* adapter);

#ifdef __cplusplus
}
#endif
