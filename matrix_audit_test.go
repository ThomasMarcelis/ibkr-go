package ibkr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

func TestCaptureScenarioMessageIDsExistInProtocolRegistry(t *testing.T) {
	t.Parallel()

	known := make(map[int]struct{})
	for _, message := range protocol.Messages() {
		known[message.ID] = struct{}{}
	}
	for scenario, ids := range captureScenarioMessageIDs(t) {
		for _, id := range ids {
			if _, ok := known[id]; !ok {
				t.Errorf("capture scenario %q refers to unknown classic message ID %d", scenario, id)
			}
		}
	}
}

func captureScenarioMessageIDs(t *testing.T) map[string][]int {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cmd/ibkr-capture/catalog.go", nil, 0)
	if err != nil {
		t.Fatalf("ParseFile(catalog.go) error = %v", err)
	}

	scenarios := make(map[string][]int)
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) == 0 || spec.Names[0].Name != "scenarioMetadataByName" || len(spec.Values) != 1 {
			return true
		}
		catalog, ok := spec.Values[0].(*ast.CompositeLit)
		if !ok {
			return false
		}
		for _, elt := range catalog.Elts {
			entry, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			nameLiteral, ok := entry.Key.(*ast.BasicLit)
			if !ok || nameLiteral.Kind != token.STRING {
				continue
			}
			name, err := strconv.Unquote(nameLiteral.Value)
			if err != nil {
				continue
			}
			ast.Inspect(entry.Value, func(n ast.Node) bool {
				list, ok := n.(*ast.CompositeLit)
				if !ok || !isIntSlice(list.Type) {
					return true
				}
				for _, item := range list.Elts {
					literal, ok := item.(*ast.BasicLit)
					if !ok || literal.Kind != token.INT {
						continue
					}
					id, err := strconv.Atoi(literal.Value)
					if err == nil {
						scenarios[name] = append(scenarios[name], id)
					}
				}
				return false
			})
		}
		return false
	})
	if len(scenarios) == 0 {
		t.Fatal("no scenario message IDs found")
	}
	return scenarios
}

func isIntSlice(expr ast.Expr) bool {
	array, ok := expr.(*ast.ArrayType)
	if !ok {
		return false
	}
	element, ok := array.Elt.(*ast.Ident)
	return ok && element.Name == "int"
}
