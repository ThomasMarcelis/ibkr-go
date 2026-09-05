package ibkr

import (
	"errors"
	"testing"
)

func TestApplyOptionsRejectsInvalidConfigurationBeforeDial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		opts  []Option
		field string
	}{
		{name: "nil option", opts: []Option{nil}, field: "Option"},
		{name: "empty host", opts: []Option{WithHost(" \t")}, field: "Host"},
		{name: "zero port", opts: []Option{WithPort(0)}, field: "Port"},
		{name: "large port", opts: []Option{WithPort(65536)}, field: "Port"},
		{name: "negative client ID", opts: []Option{WithClientID(-1)}, field: "ClientID"},
		{name: "zero event buffer", opts: []Option{WithEventBuffer(0)}, field: "EventBuffer"},
		{name: "zero subscription buffer", opts: []Option{WithSubscriptionBuffer(0)}, field: "SubscriptionBuffer"},
		{name: "zero order event buffer", opts: []Option{WithOrderEventBuffer(0)}, field: "OrderEventBuffer"},
		{name: "zero order correlation limit", opts: []Option{WithOrderExecutionCorrelationLimit(0)}, field: "OrderExecutionCorrelationLimit"},
		{name: "negative order correlation limit", opts: []Option{WithOrderExecutionCorrelationLimit(-1)}, field: "OrderExecutionCorrelationLimit"},
		{name: "zero inbound frame limit", opts: []Option{WithMaxInboundFrameBytes(0)}, field: "MaxInboundFrameBytes"},
		{name: "large inbound frame limit", opts: []Option{WithMaxInboundFrameBytes(64<<20 + 1)}, field: "MaxInboundFrameBytes"},
		{name: "unknown reconnect", opts: []Option{WithReconnectPolicy("sometimes")}, field: "ReconnectPolicy"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := applyOptions(test.opts)
			validationErr, ok := errors.AsType[*ValidationError](err)
			if !ok || validationErr.Field != test.field {
				t.Fatalf("applyOptions() error = %v, want %s ValidationError", err, test.field)
			}
		})
	}
}

func TestApplySubscriptionOptionsRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		opts  []SubscriptionOption
		field string
	}{
		{name: "nil option", opts: []SubscriptionOption{nil}, field: "SubscriptionOption"},
		{name: "zero queue", opts: []SubscriptionOption{WithQueueSize(0)}, field: "QueueSize"},
		{name: "unknown resume", opts: []SubscriptionOption{WithResumePolicy("sometimes")}, field: "ResumePolicy"},
		{name: "zero execution correlation limit", opts: []SubscriptionOption{WithExecutionCorrelationLimit(0)}, field: "ExecutionCorrelationLimit"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := applySubscriptionOptions(defaultConfig(), test.opts)
			validationErr, ok := errors.AsType[*ValidationError](err)
			if !ok || validationErr.Field != test.field {
				t.Fatalf("applySubscriptionOptions() error = %v, want %s ValidationError", err, test.field)
			}
		})
	}
}

func TestExecutionCorrelationLimitIsScopedToExecutionSubscriptions(t *testing.T) {
	t.Parallel()

	cfg, err := applySubscriptionOptionsFor(
		defaultConfig(),
		OpExecutions,
		[]SubscriptionOption{WithExecutionCorrelationLimit(7)},
	)
	if err != nil {
		t.Fatalf("applySubscriptionOptionsFor(executions): %v", err)
	}
	if cfg.executionCorrelationLimit != 7 {
		t.Fatalf("execution correlation limit = %d, want 7", cfg.executionCorrelationLimit)
	}

	_, err = applySubscriptionOptionsFor(
		defaultConfig(),
		OpQuotes,
		[]SubscriptionOption{WithExecutionCorrelationLimit(7)},
	)
	validationErr, ok := errors.AsType[*ValidationError](err)
	if !ok || validationErr.Field != "ExecutionCorrelationLimit" {
		t.Fatalf("applySubscriptionOptionsFor(quotes) error = %v, want ExecutionCorrelationLimit ValidationError", err)
	}
}

func TestExecutionCorrelationLimitHasFiniteDefault(t *testing.T) {
	t.Parallel()

	cfg, err := applySubscriptionOptionsFor(defaultConfig(), OpExecutions, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.executionCorrelationLimit != defaultExecutionCorrelationLimit || cfg.executionCorrelationLimit <= 0 {
		t.Fatalf(
			"default execution correlation limit = %d, want finite default %d",
			cfg.executionCorrelationLimit,
			defaultExecutionCorrelationLimit,
		)
	}
}
