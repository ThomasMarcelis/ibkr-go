// Package testhost provides an in-process replay and fault-injection harness
// driven by checked-in scenario transcripts. It uses the production wire
// framing and codec paths in both directions so deterministic tests exercise
// the same stack as the client, but it is not a source of truth for defining
// IBKR protocol semantics.
package testhost
