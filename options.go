package ibkr

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
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
	host                string
	port                int
	clientID            int
	dialer              transport.Dialer
	logger              *slog.Logger
	reconnect           ReconnectPolicy
	tcpKeepAlive        time.Duration
	sendRate            int
	eventBuffer         int
	subscriptionBuffer  int
	defaultResume       ResumePolicy
	defaultSlowConsumer SlowConsumerPolicy
}

type subscriptionConfig struct {
	resume          ResumePolicy
	slowConsumer    SlowConsumerPolicy
	buffer          int
	collectSnapshot bool
}

func defaultConfig() config {
	return config{
		host:                "127.0.0.1",
		port:                7497,
		clientID:            1,
		dialer:              &net.Dialer{},
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		reconnect:           ReconnectAuto,
		tcpKeepAlive:        30 * time.Second,
		sendRate:            50,
		eventBuffer:         64,
		subscriptionBuffer:  64,
		defaultResume:       ResumeNever,
		defaultSlowConsumer: SlowConsumerClose,
	}
}

func defaultSubscriptionConfig(cfg config) subscriptionConfig {
	return subscriptionConfig{
		resume:       cfg.defaultResume,
		slowConsumer: cfg.defaultSlowConsumer,
		buffer:       cfg.subscriptionBuffer,
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
		return &ValidationError{Field: "ClientID", Value: strconv.Itoa(cfg.clientID), Message: "must be >= 0"}
	}
	if cfg.eventBuffer < 1 {
		return &ValidationError{Field: "EventBuffer", Value: strconv.Itoa(cfg.eventBuffer), Message: "must be >= 1"}
	}
	if cfg.subscriptionBuffer < 1 {
		return &ValidationError{Field: "SubscriptionBuffer", Value: strconv.Itoa(cfg.subscriptionBuffer), Message: "must be >= 1"}
	}
	if !cfg.reconnect.valid() {
		return &ValidationError{Field: "ReconnectPolicy", Value: string(cfg.reconnect), Message: "must be ReconnectOff or ReconnectAuto"}
	}
	if !cfg.defaultResume.valid() {
		return &ValidationError{Field: "DefaultResumePolicy", Value: string(cfg.defaultResume), Message: "must be ResumeNever or ResumeAuto"}
	}
	if !cfg.defaultSlowConsumer.valid() {
		return &ValidationError{Field: "DefaultSlowConsumerPolicy", Value: string(cfg.defaultSlowConsumer), Message: "must be SlowConsumerClose or SlowConsumerDropOldest"}
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
	if !cfg.slowConsumer.valid() {
		return subscriptionConfig{}, &ValidationError{Field: "SlowConsumerPolicy", Value: string(cfg.slowConsumer), Message: "must be SlowConsumerClose or SlowConsumerDropOldest"}
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
func WithClientID(clientID int) Option {
	return func(cfg *config) {
		cfg.clientID = clientID
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

// WithDefaultResumePolicy sets the default [ResumePolicy] for subscriptions.
// Default: [ResumeNever]. Override per subscription with [WithResumePolicy].
func WithDefaultResumePolicy(policy ResumePolicy) Option {
	return func(cfg *config) {
		cfg.defaultResume = policy
	}
}

// WithDefaultSlowConsumerPolicy sets the default [SlowConsumerPolicy] for
// subscriptions. Default: [SlowConsumerClose]. Drop-oldest is suitable only
// when losing individual events is acceptable; stateful market-depth streams
// reject it. Override per subscription with [WithSlowConsumerPolicy].
func WithDefaultSlowConsumerPolicy(policy SlowConsumerPolicy) Option {
	return func(cfg *config) {
		cfg.defaultSlowConsumer = policy
	}
}

// WithResumePolicy overrides the [ResumePolicy] for a single subscription.
func WithResumePolicy(policy ResumePolicy) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.resume = policy
	}
}

// WithSlowConsumerPolicy overrides the [SlowConsumerPolicy] for a single
// subscription. The default is [SlowConsumerClose], which fails the
// subscription when the consumer falls behind. Use [SlowConsumerDropOldest]
// only for streams where losing individual events is acceptable; market depth
// rejects it because each event mutates book state.
func WithSlowConsumerPolicy(policy SlowConsumerPolicy) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.slowConsumer = policy
	}
}

// WithQueueSize overrides the event queue capacity for a single subscription.
// Raising it gives a bursty high-rate stream (market depth, tick-by-tick) more
// slack before the [SlowConsumerPolicy] takes effect. The size must be positive.
func WithQueueSize(size int) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.buffer = size
	}
}
