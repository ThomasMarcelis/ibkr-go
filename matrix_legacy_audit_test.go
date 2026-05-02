//go:build legacy_native_socket

package ibkr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestIBKRAPIInventoryMentionsCodecMessageConstants(t *testing.T) {
	t.Parallel()

	inventory := readText(t, "docs/ibkr-api-inventory.md")
	for _, name := range codecMessageConstantNames(t) {
		if !strings.Contains(inventory, "`"+name+"`") {
			t.Errorf("API inventory missing codec message constant %s", name)
		}
	}
}

func TestLiveCoverageMatrixMentionsCaptureScenarios(t *testing.T) {
	t.Parallel()

	matrix := readText(t, "docs/live-coverage-matrix.md")
	for _, name := range captureScenarioNames(t) {
		if !strings.Contains(matrix, "`"+name+"`") {
			t.Errorf("live coverage matrix missing capture scenario %q", name)
		}
	}
}

func codecMessageConstantNames(t *testing.T) []string {
	t.Helper()

	text := readText(t, "internal/codec/msgid.go")
	re := regexp.MustCompile(`\b(?:Out|In)[A-Za-z0-9_]+\b`)
	seen := map[string]bool{}
	var out []string
	for _, match := range re.FindAllString(text, -1) {
		if match == "Outbound" || match == "Inbound" {
			continue
		}
		if seen[match] {
			continue
		}
		seen[match] = true
		out = append(out, match)
	}
	return out
}

func captureScenarioNames(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cmd/ibkr-capture/catalog.go", nil, 0)
	if err != nil {
		t.Fatalf("ParseFile(catalog.go) error = %v", err)
	}

	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) == 0 || spec.Names[0].Name != "scenarioMetadataByName" {
			return true
		}
		if len(spec.Values) != 1 {
			return false
		}
		cl, ok := spec.Values[0].(*ast.CompositeLit)
		if !ok {
			return false
		}
		for _, elt := range cl.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			lit, ok := kv.Key.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			unquoted, err := strconv.Unquote(lit.Value)
			if err != nil {
				continue
			}
			names = append(names, unquoted)
		}
		return false
	})
	if len(names) == 0 {
		t.Fatal("no scenario metadata entries found")
	}
	return names
}
