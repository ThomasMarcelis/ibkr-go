//go:build ibkr_swig && cgo && linux

package ibkrsdkprobe

/*
#cgo CXXFLAGS: -std=c++14
#cgo linux LDFLAGS: -lstdc++
*/
import "C"

// CompileProbeVersion returns a marker from the SWIG-generated wrapper. Calling
// it proves that Go, cgo, SWIG, the local adapter, and the official SDK headers
// were compiled into the same package.
func CompileProbeVersion() string {
	return IbkrSdkProbeCompileVersion()
}

func cgoEnabled() {
	_ = C.int(0)
}
