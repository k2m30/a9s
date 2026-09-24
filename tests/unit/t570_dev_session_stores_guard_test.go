package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/k2m30/a9s/v3/core/session"
)

// TestT570_SessionStoresHaveOneOwner fails when a session store the AWS
// transport reads through is replaced or wired anywhere but the store table,
// the one place that decides which refresh clears it.
func TestT570_SessionStoresHaveOneOwner(t *testing.T) {
	stores := map[string]bool{}
	st := reflect.TypeFor[session.Session]()
	for i := range st.NumField() {
		f := st.Field(i)
		typ := f.Type
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		if f.IsExported() && strings.HasSuffix(typ.Name(), "Store") && unicode.IsLower(rune(typ.Name()[0])) {
			stores[f.Name] = true
		}
	}
	if len(stores) == 0 {
		t.Fatal("no session stores found")
	}
	storeField := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		return ok && stores[sel.Sel.Name]
	}
	owner := filepath.Join("..", "..", "core", "session", "stores.go")
	for _, root := range []string{"core", "internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == owner {
				return err
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.AssignStmt:
					for _, lhs := range n.Lhs {
						if storeField(lhs) {
							t.Errorf("%s: session store replaced outside the store table", fset.Position(n.Pos()))
						}
					}
				case *ast.CallExpr:
					sel, ok := n.Fun.(*ast.SelectorExpr)
					if !ok || !strings.HasPrefix(sel.Sel.Name, "Set") {
						return true
					}
					for _, arg := range n.Args {
						if storeField(arg) {
							t.Errorf("%s: session store wired outside the store table", fset.Position(n.Pos()))
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
