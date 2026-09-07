// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// phrase_source_pin_test.go — the source-level counterpart of the demo-bench
// phrase gate.
//
// The bench gate only sees a wording that some demo fixture actually
// triggers. A per-item phrase on a condition no fixture reaches is just as
// wrong and invisible there, so the shape is pinned at the call site too: an
// emitter passes the phrase its code declares, never one assembled with
// Sprintf, joined from a list, or read out of the first element of one.
//
// A registered phrase that carries a "<…>" placeholder is the exception the
// catalog itself declares: the code's wording contains one value the emitter
// supplies, the phrase is still built once per resource, and there is nothing
// for the same-code collapse to drop. Those sites are allowed by their own
// declaration rather than by a list kept in this file, so a site stops being
// allowed the moment its declaration stops carrying the placeholder.
//
// The second pin is the seam itself: a Wave-1 finding whose phrase is built
// by a call must go through addWave1Finding, not a hand-rolled
// domain.Finding literal. A literal bypasses the one place a Wave-1 phrase
// could ever be observed or checked.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// phraseArgIndex gives, per emitter, the argument positions of the finding
// code and of the phrase.
var phraseArgIndex = map[string]struct{ code, phrase int }{
	// setWave2Finding(r, resourceID, code, phrase, glyph, shortName, rows)
	"setWave2Finding": {code: 2, phrase: 3},
	// addWave1Finding(r, code, phrase, severity)
	"addWave1Finding": {code: 1, phrase: 2},
}

// parseAWSPackage parses every non-test Go file under core/aws.
func parseAWSPackage(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	root, err := filepath.Abs("../../core/aws")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatalf("filepath.Glob: %v", err)
	}
	if len(paths) < 100 {
		t.Fatalf("only %d files under %s — the scan is not seeing the package", len(paths), root)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		files[path] = src
	}
	return fset, files
}

// stringConstants maps every package-level string constant name to its value,
// so a call site passing a FindingCode constant can be resolved to the code
// the catalog declares.
func stringConstants(files map[string]*ast.File) map[string]string {
	consts := map[string]string{}
	for _, src := range files {
		ast.Inspect(src, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for i, name := range spec.Names {
				if i >= len(spec.Values) {
					continue
				}
				if lit, isLit := spec.Values[i].(*ast.BasicLit); isLit && lit.Kind == token.STRING {
					consts[name.Name] = strings.Trim(lit.Value, `"`)
				}
			}
			return true
		})
	}
	return consts
}

// The two ways an emitter can put something other than its code's own wording
// into a phrase, which the pin judges differently.
const (
	// phraseIndexesItem — the wording is read out of one element of a
	// collection the emitter walked. Whatever else the resource had wrong is
	// then unsayable, whichever element happens to be first. No catalog
	// declaration makes that right.
	phraseIndexesItem = "reads one item out of a collection"
	// phraseAssembled — the wording is composed at emit time from values the
	// emitter measured. Legitimate when the code declares the shape, which is
	// what a "<…>" placeholder in the registered phrase says.
	phraseAssembled = "assembled at emit time"
)

// perItemNames maps every name bound anywhere in a file to a value that is
// itself item-derived — `firstCause := out.Causes[0]`, a struct field
// `summary: fmt.Sprintf(…)` — to how it was derived. A phrase argument naming
// one of these is the same defect written in two statements. File scope,
// matched on the name alone: the lexical convention the package's other source
// gates use.
func perItemNames(src *ast.File) map[string]string {
	names := map[string]string{}
	bind := func(lhs ast.Expr, rhs ast.Expr) {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			return
		}
		if kind := builtFromItem(rhs, nil); kind != "" {
			names[id.Name] = kind
		}
	}
	ast.Inspect(src, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if len(node.Lhs) == len(node.Rhs) {
					bind(lhs, node.Rhs[i])
				}
			}
		case *ast.KeyValueExpr:
			bind(node.Key, node.Value)
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if i < len(node.Values) {
					bind(name, node.Values[i])
				}
			}
		}
		return true
	})
	return names
}

// builtFromItem classifies expr as one of the two shapes above, or "" when it
// is the code's own wording — a literal, a constant, a concatenation of those.
// Indexing wins over assembling: a Sprintf whose argument is rows[0].Label is
// still first-item-wins.
func builtFromItem(expr ast.Expr, itemNames map[string]string) string {
	kind := ""
	promote := func(k string) {
		if k != "" && kind != phraseIndexesItem {
			kind = k
		}
	}
	ast.Inspect(expr, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IndexExpr:
			promote(phraseIndexesItem)
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				pkg, isIdent := sel.X.(*ast.Ident)
				if isIdent && (pkg.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Sprint") ||
					pkg.Name == "strings" && sel.Sel.Name == "Join") {
					promote(phraseAssembled)
				}
			}
		case *ast.Ident:
			promote(itemNames[node.Name])
		case *ast.SelectorExpr:
			promote(itemNames[node.Sel.Name])
		}
		return true
	})
	return kind
}

func TestNoEmitterBuildsItsPhraseFromAnItem(t *testing.T) {
	registered := map[string]string{}
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, f := range td.Findings {
			registered[string(f.Code)] = f.Phrase
		}
	}
	if len(registered) < 300 {
		t.Fatalf("only %d declared finding codes; the pin is not seeing the catalog", len(registered))
	}

	fset, files := parseAWSPackage(t)
	consts := stringConstants(files)

	var offenders, unresolved []string
	for path, src := range files {
		itemNames := perItemNames(src)
		ast.Inspect(src, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			idx, watched := phraseArgIndex[fn.Name]
			if !watched || len(call.Args) <= idx.phrase {
				return true
			}
			kind := builtFromItem(call.Args[idx.phrase], itemNames)
			if kind == "" {
				return true
			}
			pos := fset.Position(call.Pos())
			site := fmt.Sprintf("%s:%d %s", filepath.Base(path), pos.Line, fn.Name)

			codeIdent, isIdent := call.Args[idx.code].(*ast.Ident)
			if !isIdent {
				unresolved = append(unresolved, site+" — code argument is not a constant")
				return true
			}
			code, known := consts[codeIdent.Name]
			if !known {
				unresolved = append(unresolved, site+" — code constant "+codeIdent.Name+" has no string value")
				return true
			}
			phrase := registered[code]
			if kind == phraseAssembled && strings.Contains(phrase, "<") {
				// The catalog declares this wording as one phrase with a value
				// the emitter fills in per resource; nothing is dropped, and the
				// demo-bench gate holds the emitted text to the declared shape.
				return true
			}
			offenders = append(offenders, fmt.Sprintf("%s under %s: %s (catalog phrase %q)",
				site, code, kind, phrase))
			return true
		})
	}

	sort.Strings(offenders)
	sort.Strings(unresolved)
	for _, o := range offenders {
		t.Errorf("phrase built from an item: %s — pass the phrase the code declares and put the "+
			"item in a supporting row", o)
	}
	for _, u := range unresolved {
		t.Errorf("phrase built from an item, code not statically known: %s — a call site that "+
			"picks its code at runtime also picks its wording at runtime; give each condition its "+
			"own code and its own registered phrase", u)
	}
}

func TestWave1PhrasesGoThroughTheSeam(t *testing.T) {
	fset, files := parseAWSPackage(t)

	var offenders []string
	for path, src := range files {
		// Matched on the field name alone rather than on the literal's type:
		// the Finding literals that build a phrase sit inside an elided
		// []domain.Finding{{…}} element, whose CompositeLit carries no type at
		// all. "Phrase" names one field in this package.
		ast.Inspect(src, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, isIdent := kv.Key.(*ast.Ident)
			if !isIdent || key.Name != "Phrase" {
				return true
			}
			if _, isCall := kv.Value.(*ast.CallExpr); !isCall {
				return true
			}
			pos := fset.Position(kv.Pos())
			offenders = append(offenders, fmt.Sprintf("%s:%d", filepath.Base(path), pos.Line))
			return true
		})
	}

	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("Wave-1 finding builds its phrase in a literal: %s — call addWave1Finding so "+
			"wave 1 has the one seam a phrase can be observed and checked at, the way "+
			"setWave2Finding is for wave 2", o)
	}
}
