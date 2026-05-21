package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Regression guard: main must import internal/aws AND call aws.Install() so
// the catalog (and any remaining init()-driven registry side effects) is wired
// before the TUI starts.
//
// Before AS-795a this test asserted a blank import (side-effect-only). After
// AS-795a the import is named so main() can call aws.Install(); we verify both
// the import and the explicit call to keep the wiring honest.
func TestMain_ImportsAWSRegistrySideEffects(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	want := "github.com/k2m30/a9s/v3/internal/aws"
	foundImport := false
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		if imp.Path.Value == `"`+want+`"` {
			foundImport = true
			break
		}
	}
	if !foundImport {
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
		t.Fatalf("main.go must import %q; imports=%v", want, got)
	}

	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(b), "aws.Install()") {
		t.Fatalf("main.go must call aws.Install() to populate the catalog before TUI start")
	}

	// Keep ast import used to satisfy lints in some environments.
	_ = ast.ImportSpec{}
}
