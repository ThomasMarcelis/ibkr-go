package ibkr

import (
	"context"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

type Option func(*config)

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

func WithHost(host string) Option {
	return func(cfg *config) {
		cfg.host = host
	}
}

func WithPort(port int) Option {
	return func(cfg *config) {
		cfg.port = port
	}
}

func WithClientID(clientID int) Option {
	return func(cfg *config) {
		cfg.clientID = clientID
	}
}

func WithDialer(dialer Dialer) Option {
	return func(cfg *config) {
		cfg.dialer = dialer
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) {
		if logger != nil {
			cfg.logger = logger
		}
	}
}

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

func WithSendRate(rate int) Option {
	return func(cfg *config) {
		cfg.sendRate = rate
	}
}

func WithEventBuffer(size int) Option {
	return func(cfg *config) {
		cfg.eventBuffer = size
	}
}

func WithSubscriptionBuffer(size int) Option {
	return func(cfg *config) {
		cfg.subscriptionBuffer = size
	}
}

func WithDefaultResumePolicy(policy ResumePolicy) Option {
	return func(cfg *config) {
		cfg.defaultResume = policy
	}
}

func WithDefaultSlowConsumerPolicy(policy SlowConsumerPolicy) Option {
	return func(cfg *config) {
		cfg.defaultSlowConsumer = policy
	}
}

func WithResumePolicy(policy ResumePolicy) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.resume = policy
	}
}

func WithSlowConsumerPolicy(policy SlowConsumerPolicy) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.slowConsumer = policy
	}
}

func WithQueueSize(size int) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.buffer = size
	}
}
