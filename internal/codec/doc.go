//go:build legacy_native_socket

// Package codec owns the real IBKR message encode/decode layer on top of
// package wire, including handshake parsing and server-version-aware field
// layouts validated against grounded captures.
package codec
