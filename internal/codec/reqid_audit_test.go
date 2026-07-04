package codec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestReqIDAuditInboundMessagesImplementRequestID guards the ReqIDer
// contract documented in reqid.go: every inbound (server -> client) message
// struct that carries a ReqID field must implement RequestID() int, or the
// generic keyed dispatch in engine_route.go (handleIncoming's
// `msg.(codec.ReqIDer)` type assertion) silently drops it and the caller
// waiting on that request hangs until context timeout.
//
// The rule is derived from the package's own source, not a hand-maintained
// list of message names:
//
//  1. Find every struct with a `ReqID int` field.
//  2. Classify each such struct's direction by reading the msg_id constant
//     its own encodeWire method writes onto the wire: constants are named
//     InXxx for messages the Gateway/testhost sends downstream, OutXxx for
//     requests the client sends upstream (see msgid.go). Outbound
//     Request/Cancel structs are writer-only: they are never returned by an
//     inboundDecoders entry, so they can never reach handleIncoming's keyed
//     dispatch and are correctly exempt -- exempt because of what their own
//     code says about them, not because of a name pattern like "Request".
//  3. Every inbound-direction struct must implement RequestID(), except the
//     documented exception below.
//
// Documented exception:
//
//   - APIError: ReqID is frequently -1 (unsolicited or connectivity-wide
//     errors have no originating request) so it cannot key a lookup the way
//     a real per-request ID can. engine_route.go special-cases codec.APIError
//     in handleIncoming ahead of the generic ReqIDer branch and routes it
//     through handleAPIError instead.
//
// This is a different exception class from the one already documented on
// ReqIDer in reqid.go (OpenOrder, OrderStatus, CompletedOrder): those route
// by OrderID and never have a ReqID field at all, so they never enter this
// scan's candidate set.
//
// When this test fails after adding a new inbound message: either add a
// `func (m NewType) RequestID() int { return m.ReqID }` in reqid.go, or, if
// the type is deliberately routed some other way, add it to
// wantReqIDerExceptions below with a comment explaining the routing path --
// the same bar as the APIError exception above.
func TestReqIDAuditInboundMessagesImplementRequestID(t *testing.T) {
	wantReqIDerExceptions := map[string]string{
		"APIError": "special-cased in engine_route.go's handleIncoming (handleAPIError); ReqID may be -1 for unsolicited errors",
	}

	reqIDStructs := map[string]bool{}     // struct name -> has a `ReqID int` field
	requestIDMethods := map[string]bool{} // struct name -> implements RequestID() int
	direction := map[string]string{}      // struct name -> "In" or "Out", from its own encodeWire

	dirConst := regexp.MustCompile(`^(In|Out)[A-Z]`)

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok || st.Fields == nil {
						continue
					}
					for _, field := range st.Fields.List {
						ident, ok := field.Type.(*ast.Ident)
						if !ok || ident.Name != "int" {
							continue
						}
						for _, fname := range field.Names {
							if fname.Name == "ReqID" {
								reqIDStructs[ts.Name.Name] = true
							}
						}
					}
				}

			case *ast.FuncDecl:
				recv := receiverTypeName(d)
				if recv == "" {
					continue
				}
				switch d.Name.Name {
				case "RequestID":
					requestIDMethods[recv] = true
				case "encodeWire":
					if d.Body == nil {
						continue
					}
					ast.Inspect(d.Body, func(n ast.Node) bool {
						if _, done := direction[recv]; done {
							return false
						}
						call, ok := n.(*ast.CallExpr)
						if !ok || len(call.Args) == 0 {
							return true
						}
						fnName := calleeName(call.Fun)
						if fnName != "itoa" && fnName != "i64toa" && fnName != "WriteInt" {
							return true
						}
						argIdent, ok := call.Args[0].(*ast.Ident)
						if !ok || !dirConst.MatchString(argIdent.Name) {
							return true
						}
						if strings.HasPrefix(argIdent.Name, "In") {
							direction[recv] = "In"
						} else {
							direction[recv] = "Out"
						}
						return false
					})
				}
			}
		}
	}

	var violations []string
	var unclassified []string
	for typeName := range reqIDStructs {
		dir, ok := direction[typeName]
		if !ok {
			// A struct with a ReqID field but no discoverable direction: the
			// scan itself is incomplete for this type, which is worth
			// failing loudly on rather than silently skipping.
			unclassified = append(unclassified, typeName)
			continue
		}
		if dir != "In" {
			continue // outbound request/cancel: never routed back by ReqID
		}
		if requestIDMethods[typeName] {
			continue
		}
		if _, exempt := wantReqIDerExceptions[typeName]; exempt {
			continue
		}
		violations = append(violations, typeName)
	}

	sort.Strings(unclassified)
	sort.Strings(violations)

	if len(unclassified) > 0 {
		t.Fatalf("could not determine wire direction for types with a ReqID field (encodeWire not found or msg_id constant not recognized): %v", unclassified)
	}

	if len(violations) > 0 {
		t.Fatalf("inbound message struct(s) with a ReqID field but no RequestID() method, so handleIncoming's keyed dispatch cannot route them: %v\n"+
			"either add `func (m T) RequestID() int { return m.ReqID }` in reqid.go, or document the routing exception in wantReqIDerExceptions", violations)
	}

	// Sanity check on the scan itself: if these come back empty, the AST
	// walk above stopped matching the source shape and the test is no
	// longer exercising anything.
	if len(reqIDStructs) == 0 {
		t.Fatal("scan found no structs with a ReqID field; the AST walk is broken")
	}
	if len(requestIDMethods) == 0 {
		t.Fatal("scan found no RequestID() methods; the AST walk is broken")
	}
}

// receiverTypeName returns the bare type name a method is declared on
// (value or pointer receiver), or "" for free functions.
func receiverTypeName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) != 1 {
		return ""
	}
	expr := d.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

// calleeName returns the identifier name of a call expression's function,
// handling both bare calls (itoa(...)) and selector calls (w.WriteInt(...)).
func calleeName(fn ast.Expr) string {
	switch e := fn.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	default:
		return ""
	}
}
