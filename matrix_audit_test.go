package ibkr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
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

func TestSDKMigrationMatrixMentionsPublicFacadeMethods(t *testing.T) {
	t.Parallel()

	matrix := readText(t, "docs/sdk-migration-matrix.md")
	for _, label := range publicFacadeLabels(t) {
		if !strings.Contains(matrix, label) {
			t.Errorf("SDK migration matrix missing public API label %q", label)
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
		"CurrentTime": {},
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

func readText(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}
