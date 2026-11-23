package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// t9bDevTextRouteBanned are the entry points of the second text-screen route:
// a model built from a resource that marshals and colours it itself, beside
// the controller's TextBody that production renders.
var t9bDevTextRouteBanned = map[string]bool{
	"NewYAMLWithCtrl":     true,
	"NewJSONWithCtrl":     true,
	"RawYAMLFromResource": true,
	"ContentLines":        true,
	"RawContent":          true,
	"YAMLModel":           true,
	"JSONModel":           true,
}

func TestT9bDev_OneTextScreenRoute(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "tui", "views")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var problems []string
	sawTextModel := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if t9bDevTextRouteBanned[d.Name.Name] {
					problems = append(problems, fset.Position(d.Pos()).String()+": declares "+d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if t9bDevTextRouteBanned[ts.Name.Name] {
						problems = append(problems, fset.Position(ts.Pos()).String()+": declares type "+ts.Name.Name)
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok || ts.Name.Name != "TextModel" {
						continue
					}
					sawTextModel = true
					for _, fld := range st.Fields.List {
						typ := t9bDevTypeName(fld.Type)
						if strings.Contains(typ, "resource.Resource") || strings.Contains(typ, "app.Controller") {
							problems = append(problems, fset.Position(fld.Pos()).String()+": TextModel holds "+typ)
						}
					}
				}
			}
		}
	}
	if !sawTextModel {
		t.Fatal("internal/tui/views declares no TextModel; the guard checked nothing")
	}
	t9bReport(t, "second text-screen route", problems)
}

func t9bDevTypeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return "*" + t9bDevTypeName(x.X)
	case *ast.SelectorExpr:
		return t9bDevTypeName(x.X) + "." + x.Sel.Name
	case *ast.Ident:
		return x.Name
	default:
		return ""
	}
}
