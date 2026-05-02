package ibkr

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSDKOwnedProtocolFoundationGuards(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"internal/sdkadapter/messages.go",
		"internal/sdkadapter/adapter.go",
		"internal/sdkadapter/fixture.go",
		"internal/sdkadapter/replay.go",
		"internal/sdkadapter/native/native.go",
		"internal/sdkadapter/native/native_stub.go",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("required SDK foundation file %s: %v", path, err)
		}
	}
	for _, path := range []string{
		"internal/" + "sdkmodel",
		"internal/" + "sdkfixture",
		"internal/" + "native",
	} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("removed SDK split root still exists: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat removed SDK split root %s: %v", path, err)
		}
	}

	disallowedPackageImports := []string{
		"internal/" + "sdkmodel",
		"internal/" + "sdkfixture",
		"internal/" + "native",
	}
	disallowedBackendSwitchStrings := []string{
		"IBKR_USE" + "_OFFICIAL_SDK",
		"sdkRuntime" + "Requested",
		"sdkRuntime" + "Available",
	}
	disallowedPublicOptions := []*regexp.Regexp{
		regexp.MustCompile(`\bWith` + `Dialer\b`),
		regexp.MustCompile(`\bWith` + `TCPKeepAlive\b`),
		regexp.MustCompile(`\bWith` + `SendRate\b`),
	}
	legacyImports := []string{
		`"github.com/ThomasMarcelis/ibkr-go/internal/` + `codec"`,
		`"github.com/ThomasMarcelis/ibkr-go/internal/` + `wire"`,
		`"github.com/ThomasMarcelis/ibkr-go/internal/` + `transport"`,
		`"github.com/ThomasMarcelis/ibkr-go/internal/` + `capturelog"`,
		`"github.com/ThomasMarcelis/ibkr-go/testing/` + `testhost"`,
	}
	nativeSDKImport := `"github.com/ThomasMarcelis/ibkr-go/internal/sdkadapter/` + `native"`
	for _, path := range goSourceFiles(t) {
		text := readText(t, path)
		legacySocketPath := isLegacyNativeSocketPath(path) || strings.HasPrefix(text, "//go:build legacy_native_socket")
		if legacySocketPath && !strings.HasPrefix(text, "//go:build legacy_native_socket") {
			t.Errorf("%s is legacy socket tooling but is not quarantined behind legacy_native_socket", path)
		}
		if strings.Contains(text, nativeSDKImport) && !isAllowedNativeSDKImportPath(path) {
			t.Errorf("%s imports internal/sdkadapter/native outside the SDK runtime boundary", path)
		}
		for _, removedImport := range disallowedPackageImports {
			if strings.Contains(text, removedImport) {
				t.Errorf("%s imports removed SDK split package %q", path, removedImport)
			}
		}
		if !legacySocketPath {
			for _, legacyImport := range legacyImports {
				if strings.Contains(text, legacyImport) {
					t.Errorf("%s imports legacy socket package %s in the default build", path, legacyImport)
				}
			}
		}
		for _, removedString := range disallowedBackendSwitchStrings {
			if strings.Contains(text, removedString) {
				t.Errorf("%s contains removed backend switch string %q", path, removedString)
			}
		}
		for _, removedOption := range disallowedPublicOptions {
			if removedOption.MatchString(text) {
				t.Errorf("%s contains removed socket-era public option %q", path, removedOption.String())
			}
		}
		if strings.HasPrefix(path, "internal/sdkadapter/") {
			if strings.Contains(text, "type Message interface") {
				t.Errorf("%s reintroduces sdkadapter.Message", path)
			}
			if strings.Contains(text, "messageName(") {
				t.Errorf("%s reintroduces symbolic message names", path)
			}
		}
	}

	engine := readText(t, "engine.go")
	if strings.Contains(engine, "sdkadapter.Message") {
		t.Fatal("engine route/send path depends on sdkadapter.Message")
	}
}

func goSourceFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch path {
			case ".git", ".external", ".claude", "captures", "tmp_decode_completed":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, strings.TrimPrefix(path, "./"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk Go source files: %v", err)
	}
	return files
}

func isLegacyNativeSocketPath(path string) bool {
	for _, prefix := range []string{
		"internal/codec/",
		"internal/wire/",
		"internal/transport/",
		"internal/capturelog/",
		"testing/testhost/",
		"cmd/ibkr-capture/",
		"cmd/ibkr-normalize/",
		"cmd/ibkr-recorder/",
		"cmd/ibkr-probe/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func isAllowedNativeSDKImportPath(path string) bool {
	switch path {
	case "sdk_runtime_sdk.go", "sdk_runtime_stub.go", "cmd/ibkr-sdk-fixture/main.go":
		return true
	default:
		return strings.HasPrefix(path, "internal/sdkadapter/native/")
	}
}
