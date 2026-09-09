// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// tui6_display_id_gate_test.go — every painted use of an identifier asks for
// its displayed form.
//
// Resource.ID is the one string the text boundary leaves alone: the next AWS
// call and the clipboard need the bytes AWS knows, and an S3 object key is
// operator-supplied text that arrives as an ID. That makes every place which
// PAINTS an identifier a place that has to ask resource.DisplayID for it —
// and "every place" is a rule about a set of functions, which is a comment
// unless something enumerates them.
//
// So this reads the frame, title and label builders out of the source and
// fails when one of them puts a raw .ID into the string it returns. It is the
// same shape as the doors gate: the exceptions are listed by name with their
// reason, so a new one is a decision somebody makes rather than a hole.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// displayIDPackages are the packages that build what the screen shows.
var displayIDPackages = []string{ //nolint:gochecknoglobals // test-only lookup
	filepath.Join("core", "app"),
	filepath.Join("core", "resource"),
	filepath.Join("internal", "tui"),
	filepath.Join("internal", "tui", "views"),
}

// paintedBuilders matches the functions whose result is painted: a frame
// title, a screen title, a label, a heading, a breadcrumb.
func isPaintedBuilder(name string) bool {
	lower := strings.ToLower(name)
	for _, mark := range []string{"frametitle", "title", "label", "heading", "breadcrumb"} {
		if strings.Contains(lower, mark) {
			return true
		}
	}
	return false
}

// paintedBuildersThatKeepTheRawID are the exceptions: a builder whose result
// is not painted, or one that hands the identifier on to a caller that paints
// it through DisplayID. Each needs its reason.
var paintedBuildersThatKeepTheRawID = map[string]string{ //nolint:gochecknoglobals // test-only lookup
	"TextFrameTitle": "paints the screen's own identifier (a runtime.ScreenID: yaml, json, errors), not a resource's",
}

// readsRawID reports whether fn reads a .ID field into a string it builds,
// without that read being wrapped in DisplayID. A read inside a DisplayID call
// is the compliant form; a read compared against another value, indexed with,
// or passed to a non-painting call is not a paint and is not flagged, because
// only string building reaches a screen.
func readsRawID(fn *ast.FuncDecl) []string {
	var raw []string
	wrapped := map[ast.Node]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}
		fun, isSel := call.Fun.(*ast.SelectorExpr)
		if isSel && fun.Sel.Name == "DisplayID" || isIdent(call.Fun, "DisplayID") {
			for _, arg := range call.Args {
				ast.Inspect(arg, func(inner ast.Node) bool {
					wrapped[inner] = true
					return true
				})
			}
		}
		return true
	})
	// Only string-building positions count: a binary + with a string operand,
	// or a return of the read itself.
	flag := func(e ast.Expr) {
		ast.Inspect(e, func(n ast.Node) bool {
			// Handing the identifier to another painted builder is not a
			// paint: that builder is in this gate's own set and answers for
			// it. Only the last hop can call DisplayID, and this is not it.
			if call, isCall := n.(*ast.CallExpr); isCall && callsPaintedBuilder(call) {
				return false
			}
			sel, isSel := n.(*ast.SelectorExpr)
			if !isSel || sel.Sel.Name != "ID" || wrapped[n] {
				return true
			}
			raw = append(raw, exprString(sel))
			return true
		})
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ReturnStmt:
			for _, res := range node.Results {
				flag(res)
			}
		case *ast.BinaryExpr:
			if node.Op.String() == "+" {
				flag(node.X)
				flag(node.Y)
			}
		}
		return true
	})
	return raw
}

// callsPaintedBuilder reports whether call's callee is itself one of the
// builders this gate checks.
func callsPaintedBuilder(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return isPaintedBuilder(fun.Name)
	case *ast.SelectorExpr:
		return isPaintedBuilder(fun.Sel.Name)
	}
	return false
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

// exprString renders a selector as it is written, for the failure message.
func exprString(sel *ast.SelectorExpr) string {
	var b strings.Builder
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch t := e.(type) {
		case *ast.Ident:
			b.WriteString(t.Name)
		case *ast.SelectorExpr:
			walk(t.X)
			b.WriteString("." + t.Sel.Name)
		case *ast.CallExpr:
			walk(t.Fun)
			b.WriteString("()")
		default:
			b.WriteString("?")
		}
	}
	walk(sel)
	return b.String()
}

func notATestFile(fi fs.FileInfo) bool {
	return !strings.HasSuffix(fi.Name(), "_test.go")
}

// TestDisplayIDGate_EveryPaintedIDGoesThroughIt is the enforcement half of
// row 16: the painted-identifier sites are read out of the source, not listed
// in prose.
func TestDisplayIDGate_EveryPaintedIDGoesThroughIt(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	var builders []string
	offenders := map[string][]string{}
	positions := map[string]string{}
	fset := token.NewFileSet()
	for _, pkgDir := range displayIDPackages {
		pkgs, err := parser.ParseDir(fset, filepath.Join(repoRoot, pkgDir), notATestFile, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", pkgDir, err)
		}
		for _, pkg := range pkgs {
			for _, file := range pkg.Files {
				for _, decl := range file.Decls {
					fn, isFunc := decl.(*ast.FuncDecl)
					if !isFunc || fn.Body == nil || !isPaintedBuilder(fn.Name.Name) {
						continue
					}
					name := fn.Name.Name
					builders = append(builders, name)
					pos := fset.Position(fn.Pos())
					positions[name] = filepath.Base(pos.Filename) + ":" + strconv.Itoa(pos.Line)
					if reads := readsRawID(fn); len(reads) > 0 {
						offenders[name] = reads
					}
				}
			}
		}
	}
	sort.Strings(builders)
	if len(builders) < 5 {
		t.Fatalf("only %d painted builders found (%v) — the gate is not reading the source", len(builders), builders)
	}

	for name, reads := range offenders {
		if reason, exempt := paintedBuildersThatKeepTheRawID[name]; exempt {
			t.Logf("%s keeps a raw identifier by exception: %s", name, reason)
			continue
		}
		t.Errorf("%s (%s) builds a painted string out of %v without resource.DisplayID.\n"+
			"An identifier is left raw by the boundary for the clipboard and for AWS calls, so a painted one has to ask for its displayed form.\n"+
			"Builders read: %v", name, positions[name], reads, builders)
	}

	for name := range paintedBuildersThatKeepTheRawID {
		if _, found := positions[name]; found && offenders[name] == nil {
			t.Errorf("paintedBuildersThatKeepTheRawID names %q, which no longer reads a raw identifier — delete the entry", name)
		}
	}
}
