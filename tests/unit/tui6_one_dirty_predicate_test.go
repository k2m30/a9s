// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_one_dirty_predicate_test.go — one rule for what counts as dirty, and it
// covers every string the boundary claims.
//
// Three functions decide dirtiness today and each writes the rule out again:
// the resource-level check, and the finding and attention-row entry points
// beside it. They agree, which is the danger — a rule copied three times
// agrees until one copy is edited, and nothing reports the day it stops.
//
// Copying it three times also made it easy to miss a string. Two of them are
// missed: a finding's Code and an attention row's Tier are strings that reach
// the serialised state and the painter's style selector, and no copy of the
// rule looks at either. One predicate over every string is what closes both
// halves at once.
package unit_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
)

// tui6DirtyEntryPoints are the two finding-level entry points the row names.
var tui6DirtyEntryPoints = []string{"SanitizedFindings", "SanitizedAttentionDetails"} //nolint:gochecknoglobals // test-only lookup

// TestOneDirtyPredicate_EntryPointsHoldNoRuleOfTheirOwn is the gate. A
// dirtiness test written inline is a second copy of the rule, whatever it
// currently says.
func TestOneDirtyPredicate_EntryPointsHoldNoRuleOfTheirOwn(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "core", "domain", "sanitize.go")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing the boundary: %v", err)
	}

	found := map[string]bool{}
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Body == nil {
			continue
		}
		var wanted bool
		for _, name := range tui6DirtyEntryPoints {
			if fn.Name.Name == name {
				wanted = true
			}
		}
		if !wanted {
			continue
		}
		found[fn.Name.Name] = true

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			cmp, isCmp := n.(*ast.BinaryExpr)
			if !isCmp || cmp.Op != token.NEQ {
				return true
			}
			if !tui6CallsSanitize(cmp.X) && !tui6CallsSanitize(cmp.Y) {
				return true
			}
			pos := fset.Position(cmp.Pos())
			t.Errorf("%s decides dirtiness for itself at sanitize.go:%d — one predicate answers that question for every string the boundary cleans, and this is a second copy of it",
				fn.Name.Name, pos.Line)
			return true
		})
	}
	for _, name := range tui6DirtyEntryPoints {
		if !found[name] {
			t.Errorf("%s is not in core/domain/sanitize.go — this gate is reading the wrong file", name)
		}
	}
}

// tui6CallsSanitize reports whether e is a call to Sanitize.
func tui6CallsSanitize(e ast.Expr) bool {
	call, isCall := e.(*ast.CallExpr)
	if !isCall {
		return false
	}
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name == "Sanitize"
	case *ast.SelectorExpr:
		return fn.Sel.Name == "Sanitize"
	}
	return false
}

// TestOneDirtyPredicate_CoversEveryStringOnAFinding pins the coverage half.
// The Code keys the attention block the web lane decodes and the detail body
// groups its rows by; the Tier is the style selector a painter switches on.
// Both are strings on a finding, and neither is cleaned today.
func TestOneDirtyPredicate_CoversEveryStringOnAFinding(t *testing.T) {
	code := domain.FindingCode("ec2.pub\x1b[31mlic")
	findings := []domain.Finding{{
		Code: code, Phrase: "public address", Detail: "The instance holds one.",
		Severity: domain.SevBroken, Source: "wave1",
	}}
	details := map[domain.FindingCode]domain.AttentionDetail{
		code: {Rows: []domain.DetailRow{{Label: "Public address", Value: "54.210.33.112", Tier: "!\x1b[31m"}}},
	}

	clean := domain.SanitizedFindings(findings)
	if got := string(clean[0].Code); tui6HasControl(got) {
		t.Errorf("the finding's Code comes back dirty: %q", got)
	}

	cleanDetails := domain.SanitizedAttentionDetails(details)
	for gotCode, ad := range cleanDetails {
		if tui6HasControl(string(gotCode)) {
			t.Errorf("the attention detail is still keyed by a dirty Code: %q", gotCode)
		}
		for _, row := range ad.Rows {
			if tui6HasControl(row.Tier) {
				t.Errorf("the attention row's Tier comes back dirty: %q", row.Tier)
			}
		}
	}
	if len(cleanDetails) != len(details) {
		t.Errorf("cleaning the keys collapsed %d attention details into %d", len(details), len(cleanDetails))
	}
}

// TestOneDirtyPredicate_ResourceLevelCoversTheSameStrings pins the same
// coverage through the writer every door calls, and through what the other
// lane decodes — the surface the Code and the Tier actually reach.
func TestOneDirtyPredicate_ResourceLevelCoversTheSameStrings(t *testing.T) {
	code := domain.FindingCode("ec2.pub\x1b[31mlic")
	res := domain.Resource{
		ID: "i-0123456789abcdef0", Name: "web-prod", Type: "ec2",
		Fields: map[string]string{"instance_id": "i-0123456789abcdef0"},
		Findings: []domain.Finding{{
			Code: code, Phrase: "public address", Detail: "The instance holds one.",
			Severity: domain.SevBroken, Source: "wave1",
		}},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			code: {Rows: []domain.DetailRow{{Label: "Public address", Value: "54.210.33.112", Tier: "!\x1b[31m"}}},
		},
	}

	if !res.NeedsSanitizing() {
		t.Error("the resource carries a control in its finding Code and its attention Tier, and the predicate says there is nothing to clean")
	}

	clean := res.Sanitized()
	raw, err := json.Marshal(clean)
	if err != nil {
		t.Fatalf("serialising the resource: %v", err)
	}
	if strings.Contains(string(raw), "u001b") {
		t.Errorf("the serialised resource still carries an escape: %s", raw)
	}
}

// TestOneDirtyPredicate_BareC1InAPhraseAlreadyAgrees is the counterpart, and
// it records what this row is NOT about. The three copies of the rule are
// behaviourally identical today — a bare C1 byte in a phrase is recognised by
// every one of them — so the row is about the duplication and the strings the
// duplication let everyone miss, not about a disagreement. If this ever fails,
// the copies HAVE diverged and the row was more urgent than it looked.
func TestOneDirtyPredicate_BareC1InAPhraseAlreadyAgrees(t *testing.T) {
	phrase := "public\u009b31m address"
	findings := []domain.Finding{{Code: "ec2.public-ip", Phrase: phrase, Severity: domain.SevBroken, Source: "wave1"}}

	if got := domain.SanitizedFindings(findings)[0].Phrase; tui6HasControl(got) {
		t.Errorf("the finding entry point left a bare C1 byte in the phrase: %q", got)
	}

	res := domain.Resource{ID: "i-0123456789abcdef0", Findings: findings}
	if !res.NeedsSanitizing() {
		t.Error("the resource-level predicate does not recognise a bare C1 byte in a phrase")
	}
	if got := res.Sanitized().Findings[0].Phrase; tui6HasControl(got) {
		t.Errorf("the resource-level writer left a bare C1 byte in the phrase: %q", got)
	}
}

// tui6HasControl reports whether s carries a control rune or a raw C1 byte.
func tui6HasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
	}
	return strings.IndexByte(s, 0x9b) >= 0
}
