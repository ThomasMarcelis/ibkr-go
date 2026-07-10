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
		{name: "unknown reconnect", opts: []Option{WithReconnectPolicy("sometimes")}, field: "ReconnectPolicy"},
		{name: "unknown default resume", opts: []Option{WithDefaultResumePolicy("sometimes")}, field: "DefaultResumePolicy"},
		{name: "unknown default slow consumer", opts: []Option{WithDefaultSlowConsumerPolicy("sometimes")}, field: "DefaultSlowConsumerPolicy"},
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
		{name: "unknown slow consumer", opts: []SubscriptionOption{WithSlowConsumerPolicy("sometimes")}, field: "SlowConsumerPolicy"},
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
