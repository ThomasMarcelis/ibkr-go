package ibkr

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

// Option configures a [Client] at [DialContext] time.
type Option func(*config)

// SubscriptionOption configures a single subscription at the point it is
// created, overriding the client-level defaults.
type SubscriptionOption func(*subscriptionConfig)

// Dialer establishes the TCP connection to the Gateway or TWS. The standard
// library's [*net.Dialer] satisfies this interface, so most callers pass one
// directly; supply a custom implementation to route through a proxy or an
// in-process pipe. Pass it with [WithDialer].
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type config struct {
	host                           string
	port                           int
	clientID                       ClientID
	dialer                         Dialer
	logger                         *slog.Logger
	reconnect                      ReconnectPolicy
	tcpKeepAlive                   time.Duration
	sendRate                       int
	eventBuffer                    int
	subscriptionBuffer             int
	orderEventBuffer               int
	orderExecutionCorrelationLimit int
	maxInboundFrameBytes           int
}

type subscriptionConfig struct {
	resume                       ResumePolicy
	buffer                       int
	collectSnapshot              bool
	executionCorrelationLimit    int
	executionCorrelationLimitSet bool
}

// This is an operational memory ceiling, not an IBKR protocol limit. It leaves
// more than 270x headroom over the largest checked-in live-derived 15-row
// execution snapshot while keeping every retained event family finite.
const defaultExecutionCorrelationLimit = 1 << 12

func defaultConfig() config {
	return config{
		host:                           "127.0.0.1",
		port:                           7497,
		clientID:                       1,
		dialer:                         &net.Dialer{},
		logger:                         slog.New(slog.NewTextHandler(io.Discard, nil)),
		reconnect:                      ReconnectAuto,
		tcpKeepAlive:                   30 * time.Second,
		sendRate:                       50,
		eventBuffer:                    64,
		subscriptionBuffer:             64,
		orderEventBuffer:               64,
		orderExecutionCorrelationLimit: defaultExecutionCorrelationLimit,
		maxInboundFrameBytes:           64 << 20,
	}
}

func defaultSubscriptionConfig(cfg config) subscriptionConfig {
	return subscriptionConfig{
		resume:                    ResumeNever,
		buffer:                    cfg.subscriptionBuffer,
		executionCorrelationLimit: defaultExecutionCorrelationLimit,
	}
}

func applyOptions(opts []Option) (config, error) {
	cfg := defaultConfig()
	for i, opt := range opts {
		if opt == nil {
			return config{}, &ValidationError{Field: "Option", Value: strconv.Itoa(i), Message: "must not be nil"}
		}
		opt(&cfg)
	}
	if err := validateConfig(cfg); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func validateConfig(cfg config) error {
	if strings.TrimSpace(cfg.host) == "" {
		return &ValidationError{Field: "Host", Message: "must not be empty"}
	}
	if cfg.port < 1 || cfg.port > 65535 {
		return &ValidationError{Field: "Port", Value: strconv.Itoa(cfg.port), Message: "must be between 1 and 65535"}
	}
	if cfg.clientID < 0 {
		return &ValidationError{Field: "ClientID", Value: strconv.FormatInt(int64(cfg.clientID), 10), Message: "must be >= 0"}
	}
	if cfg.eventBuffer < 1 {
		return &ValidationError{Field: "EventBuffer", Value: strconv.Itoa(cfg.eventBuffer), Message: "must be >= 1"}
	}
	if cfg.subscriptionBuffer < 1 {
		return &ValidationError{Field: "SubscriptionBuffer", Value: strconv.Itoa(cfg.subscriptionBuffer), Message: "must be >= 1"}
	}
	if cfg.orderEventBuffer < 1 {
		return &ValidationError{Field: "OrderEventBuffer", Value: strconv.Itoa(cfg.orderEventBuffer), Message: "must be >= 1"}
	}
	if cfg.orderExecutionCorrelationLimit < 1 {
		return &ValidationError{Field: "OrderExecutionCorrelationLimit", Value: strconv.Itoa(cfg.orderExecutionCorrelationLimit), Message: "must be >= 1"}
	}
	if cfg.maxInboundFrameBytes < 1 || cfg.maxInboundFrameBytes > 64<<20 {
		return &ValidationError{Field: "MaxInboundFrameBytes", Value: strconv.Itoa(cfg.maxInboundFrameBytes), Message: "must be between 1 and 67108864"}
	}
	if !cfg.reconnect.valid() {
		return &ValidationError{Field: "ReconnectPolicy", Value: string(cfg.reconnect), Message: "must be ReconnectOff or ReconnectAuto"}
	}
	return nil
}

func applySubscriptionOptions(client config, opts []SubscriptionOption) (subscriptionConfig, error) {
	cfg := defaultSubscriptionConfig(client)
	for i, opt := range opts {
		if opt == nil {
			return subscriptionConfig{}, &ValidationError{Field: "SubscriptionOption", Value: strconv.Itoa(i), Message: "must not be nil"}
		}
		opt(&cfg)
	}
	if cfg.buffer < 1 {
		return subscriptionConfig{}, &ValidationError{Field: "QueueSize", Value: strconv.Itoa(cfg.buffer), Message: "must be >= 1"}
	}
	if !cfg.resume.valid() {
		return subscriptionConfig{}, &ValidationError{Field: "ResumePolicy", Value: string(cfg.resume), Message: "must be ResumeNever or ResumeAuto"}
	}
	if cfg.executionCorrelationLimit < 1 {
		return subscriptionConfig{}, &ValidationError{
			Field: "ExecutionCorrelationLimit", Value: strconv.Itoa(cfg.executionCorrelationLimit), Message: "must be >= 1",
		}
	}
	return cfg, nil
}

func applySubscriptionOptionsFor(client config, opKind OpKind, opts []SubscriptionOption) (subscriptionConfig, error) {
	cfg, err := applySubscriptionOptions(client, opts)
	if err != nil {
		return subscriptionConfig{}, err
	}
	if err := validateResumePolicy(opKind, cfg.resume); err != nil {
		return subscriptionConfig{}, err
	}
	if cfg.executionCorrelationLimitSet && opKind != OpExecutions {
		return subscriptionConfig{}, &ValidationError{
			Field: "ExecutionCorrelationLimit", Message: "only applies to execution subscriptions",
		}
	}
	return cfg, nil
}

// WithHost sets the Gateway/TWS host. Default: "127.0.0.1". An empty host is
// rejected by [DialContext] with a [ValidationError] before dialing.
func WithHost(host string) Option {
	return func(cfg *config) {
		cfg.host = host
	}
}

// WithPort sets the Gateway/TWS port. Default: 7497 (TWS paper). IB Gateway
// commonly listens on 4001 (live) or 4002 (paper). Valid ports are 1..65535.
func WithPort(port int) Option {
	return func(cfg *config) {
		cfg.port = port
	}
}

// WithClientID sets the TWS API client ID. Default: 1. Each concurrent
// connection to the same Gateway needs a distinct ID; ID 0 additionally binds
// manually entered TWS orders (see [OpenOrdersScopeAuto]).
func WithClientID(clientID ClientID) Option {
	return func(cfg *config) {
		cfg.clientID = clientID
	}
}

// WithMaxInboundFrameBytes lowers the per-frame inbound allocation ceiling.
// The default and hard maximum are 64 MiB. The limit applies to the server
// handshake and every subsequent raw frame, before decoded expansion.
func WithMaxInboundFrameBytes(limit int) Option {
	return func(cfg *config) {
		cfg.maxInboundFrameBytes = limit
	}
}

// WithDialer sets a custom [Dialer] for the TCP connection, for example to
// route through a proxy or an in-process pipe. Default: a standard [net.Dialer].
// A nil dialer is ignored, leaving the default [net.Dialer] in place.
func WithDialer(dialer Dialer) Option {
	return func(cfg *config) {
		if dialer != nil {
			cfg.dialer = dialer
		}
	}
}

// WithLogger sets the structured logger. A nil logger is ignored, leaving the
// default no-op logger in place.
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) {
		if logger != nil {
			cfg.logger = logger
		}
	}
}

// WithReconnectPolicy controls automatic reconnection after a dropped
// connection. Default: [ReconnectAuto].
func WithReconnectPolicy(policy ReconnectPolicy) Option {
	return func(cfg *config) {
		cfg.reconnect = policy
	}
}

// WithTCPKeepAlive configures OS TCP keepalive for Gateway/TWS connections.
// A positive duration enables keepalive with that period. A non-positive duration
// disables keepalive after dialing.
func WithTCPKeepAlive(period time.Duration) Option {
	return func(cfg *config) {
		cfg.tcpKeepAlive = period
	}
}

// WithSendRate caps outbound requests per second to respect IBKR pacing.
// Default: 50. A non-positive rate disables pacing; rates above 1e9/s are
// clamped to 1e9/s (the point where the pacing interval would round to zero),
// so extreme values are safe rather than a panic.
func WithSendRate(rate int) Option {
	return func(cfg *config) {
		cfg.sendRate = rate
	}
}

// WithEventBuffer sets the capacity of the session [Event] channel. Default: 64.
func WithEventBuffer(size int) Option {
	return func(cfg *config) {
		cfg.eventBuffer = size
	}
}

// WithSubscriptionBuffer sets the default per-subscription event queue capacity.
// Default: 64. Override per subscription with [WithQueueSize].
func WithSubscriptionBuffer(size int) Option {
	return func(cfg *config) {
		cfg.subscriptionBuffer = size
	}
}

// WithOrderEventBuffer sets the capacity of every [OrderHandle] event queue.
// Default: 64. Order events are never silently dropped: when this queue fills,
// local observation ends with [ErrSlowConsumer] while the live order may keep
// executing at IBKR.
func WithOrderEventBuffer(size int) Option {
	return func(cfg *config) {
		cfg.orderEventBuffer = size
	}
}

// WithOrderExecutionCorrelationLimit bounds retained execution IDs and, separately,
// pending fee-report versions across all local order and exercise handles.
// Default: 4096; limit must be positive. Identical consecutive fees consume no
// additional capacity. Correlation survives reconnects while handles remain open.
// Overflow ends affected observation with both [ErrExecutionCorrelationOverflow]
// and [ErrOrderRecoveryRequired]. An unmatched fee can belong to any handle, so
// its overflow ends all order observation. Orders at IBKR are not cancelled;
// execution queries and [OrdersClient.SubscribeExecutionEvents] remain available
// for reconciliation. This limit is independent of [WithExecutionCorrelationLimit].
func WithOrderExecutionCorrelationLimit(limit int) Option {
	return func(cfg *config) { cfg.orderExecutionCorrelationLimit = limit }
}

// WithResumePolicy overrides the [ResumePolicy] for a single subscription. The
// default is [ResumeNever]. [ResumeAuto] is accepted only by streaming
// [MarketDataClient.SubscribeQuotes] and
// [MarketDataClient.SubscribeRealTimeBars]; every other operation returns a
// [*ValidationError]. It reissues the request after an automatic transport
// reconnect and after a data-lost restoration (Gateway code 1101), including
// when 1101 arrives on the existing socket. It does not make one-shots or other
// subscription families replayable.
func WithResumePolicy(policy ResumePolicy) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.resume = policy
	}
}

// WithQueueSize overrides the event queue capacity for a single subscription.
// Raising it gives a bursty high-rate stream (market depth, tick-by-tick) more
// slack before the subscription closes with [ErrSlowConsumer]. The size must be positive.
func WithQueueSize(size int) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.buffer = size
	}
}

// WithExecutionCorrelationLimit caps both the distinct execution IDs retained
// by [OrdersClient.SubscribeExecutions] and the total fee-report versions held
// while waiting for their execution details. The default 4096 is a client
// memory ceiling, not an IBKR protocol maximum. Immediately repeated identical
// fee reports do not consume additional capacity. Reaching the limit is
// accepted; the next event that would exceed it closes the subscription with
// [ErrExecutionCorrelationOverflow]. This option is rejected for other
// subscription types.
func WithExecutionCorrelationLimit(limit int) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.executionCorrelationLimit = limit
		cfg.executionCorrelationLimitSet = true
	}
}
