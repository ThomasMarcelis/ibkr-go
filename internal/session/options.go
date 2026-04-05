package session

import (
	"log/slog"
	"net"
	"time"

	"github.com/ThomasMarcelis/ibkr-go/internal/transport"
)

type Option func(*config)

type SubscriptionOption func(*subscriptionConfig)

type config struct {
	host                string
	port                int
	dialer              transport.Dialer
	logger              *slog.Logger
	reconnect           ReconnectPolicy
	sendRate            int
	eventBuffer         int
	subscriptionBuffer  int
	defaultResume       ResumePolicy
	defaultSlowConsumer SlowConsumerPolicy
	reconnectBackoff    time.Duration
	minServerVersion    int
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
		dialer:              &net.Dialer{},
		logger:              slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		reconnect:           ReconnectAuto,
		sendRate:            50,
		eventBuffer:         64,
		subscriptionBuffer:  64,
		defaultResume:       ResumeNever,
		defaultSlowConsumer: SlowConsumerClose,
		reconnectBackoff:    100 * time.Millisecond,
		minServerVersion:    1,
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

func WithDialer(dialer transport.Dialer) Option {
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

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
