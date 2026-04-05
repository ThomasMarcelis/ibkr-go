// Package ibkr is the public entry point for the ibkr-go client library.
//
// The public surface is built around a ready-session dial path, typed one-shot
// request methods, and typed subscriptions with explicit lifecycle events.
// Business data flows through Events(), while lifecycle and reconnect semantics
// flow through State() and SessionEvents().
//
// DialContext returns only after the session is ready for use. Managed accounts,
// negotiated server version, and connection sequence live on SessionSnapshot
// rather than behind request-shaped APIs.
package ibkr
