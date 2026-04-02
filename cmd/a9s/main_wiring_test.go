package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Regression guard: main must import internal/aws for registry side effects
// (related defs + navigable fields). Without this, real app runs lose RELATED.
func TestMain_ImportsAWSRegistrySideEffects(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse main.go imports: %v", err)
	}

	want := "github.com/k2m30/a9s/v3/internal/aws"
	found := false
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		path := imp.Path.Value
		if path == `"`+want+`"` {
			if imp.Name != nil && imp.Name.Name == "_" {
				found = true
				break
			}
		}
	}
	if !found {
		var got []string
		for _, imp := range file.Imports {
			if imp.Path == nil {
				continue
			}
			if imp.Name != nil {
				got = append(got, imp.Name.Name+" "+imp.Path.Value)
			} else {
				got = append(got, imp.Path.Value)
			}
		}
		t.Fatalf("main.go must include blank import _ %q; imports=%v", want, got)
	}

	// Keep ast import used to satisfy lints in some environments.
	_ = ast.ImportSpec{}
}
