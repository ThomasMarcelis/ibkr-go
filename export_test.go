package ibkr

// SetAdvertisedServerVersionMaxForTest caps the version range advertised in
// the handshake so live tests can force a lower supported wire layout. It
// returns a restore func. Tests using it mutate package
// state and must not run in parallel.
func SetAdvertisedServerVersionMaxForTest(v int) (restore func()) {
	prev := advertisedServerVersionMax
	advertisedServerVersionMax = v
	return func() { advertisedServerVersionMax = prev }
}
