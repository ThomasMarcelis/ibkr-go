package ibkr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/ThomasMarcelis/ibkr-go/internal/protocol"
)

func TestLiveCoverageMatrixMentionsPublicFacadeMethods(t *testing.T) {
	t.Parallel()

	matrix := readText(t, "docs/live-coverage-matrix.md")
	for _, label := range publicFacadeLabels(t) {
		if !strings.Contains(matrix, label) {
			t.Errorf("live coverage matrix missing public API label %q", label)
		}
	}
}

func TestIBKRAPIInventoryMentionsCodecMessageConstants(t *testing.T) {
	t.Parallel()

	inventory := readText(t, "docs/ibkr-api-inventory.md")
	for _, message := range protocol.Messages() {
		row := "| `" + message.Name + "` | " + strconv.Itoa(message.ID) + " |"
		if !strings.Contains(inventory, row) {
			t.Errorf("API inventory missing or misnumbering protocol message %s (%s %d)", message.Name, message.Direction, message.ID)
		}
	}
}

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

func TestLiveCoverageMatrixMentionsCaptureScenarios(t *testing.T) {
	t.Parallel()

	matrix := readText(t, "docs/live-coverage-matrix.md")
	for _, name := range captureScenarioNames(t) {
		if !strings.Contains(matrix, "`"+name+"`") {
			t.Errorf("live coverage matrix missing capture scenario %q", name)
		}
	}
}

func publicFacadeLabels(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "client.go", nil, 0)
	if err != nil {
		t.Fatalf("ParseFile(client.go) error = %v", err)
	}

	receiverLabels := map[string]string{
		"AccountsClient":   "Accounts()",
		"ContractsClient":  "Contracts()",
		"MarketDataClient": "MarketData()",
		"HistoryClient":    "History()",
		"OrdersClient":     "Orders()",
		"OptionsClient":    "Options()",
		"NewsClient":       "News()",
		"ScannerClient":    "Scanner()",
		"AdvisorsClient":   "Advisors()",
		"WSHClient":        "WSH()",
		"TWSClient":        "TWS()",
	}
	rootClientMethods := map[string]struct{}{
		"Close": {}, "Done": {}, "Wait": {}, "Session": {}, "SessionEvents": {},
		"CurrentTime": {}, "CurrentTimeMillis": {},
	}

	var labels []string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil || len(fn.Recv.List) != 1 {
			return true
		}
		receiver := receiverName(fn.Recv.List[0].Type)
		if label, ok := receiverLabels[receiver]; ok {
			labels = append(labels, label+"."+fn.Name.Name)
			return true
		}
		if receiver == "Client" {
			if _, ok := rootClientMethods[fn.Name.Name]; ok {
				labels = append(labels, "Client."+fn.Name.Name)
			}
		}
		return true
	})
	return labels
}

func receiverName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return receiverName(v.X)
	default:
		return ""
	}
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

func readText(t *testing.T, path string) string {
	t.Helper()

	// #nosec G304 -- callers pass repository-owned audit inputs.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}
