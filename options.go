package ibkr

import (
	"context"
	"io"
	"log/slog"
	"net"
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
	resume       ResumePolicy
	slowConsumer SlowConsumerPolicy
	buffer       int
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

// WithHost sets the Gateway/TWS host. Default: "127.0.0.1".
func WithHost(host string) Option {
	return func(cfg *config) {
		cfg.host = host
	}
}

// WithPort sets the Gateway/TWS port. Default: 7497 (TWS paper). IB Gateway
// commonly listens on 4001 (live) or 4002 (paper).
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
func WithDialer(dialer Dialer) Option {
	return func(cfg *config) {
		cfg.dialer = dialer
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
// Default: 50.
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
// subscriptions. Default: [SlowConsumerClose]. High-rate streams (market depth,
// tick-by-tick) can trip this; for them prefer [SlowConsumerDropOldest] or a
// larger queue. Override per subscription with [WithSlowConsumerPolicy].
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
// subscription when the consumer falls behind; high-rate market-depth or
// tick-by-tick consumers should pass [SlowConsumerDropOldest] or raise the
// queue with [WithQueueSize].
func WithSlowConsumerPolicy(policy SlowConsumerPolicy) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.slowConsumer = policy
	}
}

// WithQueueSize overrides the event queue capacity for a single subscription.
// Raising it gives a bursty high-rate stream (market depth, tick-by-tick) more
// slack before the [SlowConsumerPolicy] takes effect.
func WithQueueSize(size int) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.buffer = size
	}
}
