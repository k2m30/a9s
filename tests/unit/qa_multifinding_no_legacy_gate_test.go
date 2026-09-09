// qa_multifinding_no_legacy_gate_test.go — no single-finding compat layer
// beside the multi-finding work (setWave2Finding as an append-only builder,
// IssueEnricherResult.Findings as map[string][]domain.Finding).
//
// No allowlist: each gate does a fresh scan every run and unconditionally
// fails when it finds ANY matching violation, listing every one in the
// failure message. This is deliberately different from this package's
// ratchet-style gates (qa_controller_construction_discipline_test.go,
// qa_enricher_finding_builder_discipline_test.go), which allowlist known
// debt with t.Skipf and stay green around it. None of the four gates
// requires a new production symbol to exist, so a violation is a t.Errorf,
// never a compile break.
//
//	GATE 1: no single-Finding-typed Findings field on a runtime/app/messages
//	struct.
//
//	GATE 2: no single-value AttentionDetails field on those structs.
//
//	GATE 3: no direct per-resource result.Findings[id] = ... /
//	result.AttentionDetails[id] = ... write outside the builder.
//
//	GATE 4: no doc comment describing a single-finding contract ("canonical
//	... wave 2 ... path", "at most one wave2 finding per resource",
//	"worse-severity wins the slot", "attach to first finding"); the patterns
//	are mfnlDocPatterns below.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// GATE 1 + GATE 2 shared scan: struct-field-type detection over the
// enrichment-contract file surface.
// ---------------------------------------------------------------------------

// mfnlContractFiles is the enrichment-contract surface GATE 1 and GATE 2
// scan. Every struct declared in each of these four files is examined
// field-by-field, not just the specific structs the originating dispatch
// named — PatchDetail (intent.go) and EnrichmentChecked (event.go) are
// covered generically this way, along with any future struct added to the
// same files carrying the same anti-pattern. Verified by direct grep before
// writing this gate: no OTHER struct in any of these four files references
// domain.Finding or domain.AttentionDetail at all, so scanning whole-file
// cannot produce a false positive against an unrelated field.
var mfnlContractFiles = []string{
	"../../core/runtime/intent.go",
	"../../core/runtime/state.go",
	"../../core/app/viewstate.go",
	"../../core/runtime/messages/event.go",
}

// mfnlIsStringIdent reports whether expr is the bare identifier "string" —
// the required key type at every map[string]... layer of an allowed or
// banned finding/attention-detail map chain.
func mfnlIsStringIdent(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "string"
}

// mfnlIsDomainSelector reports whether expr is the exact selector
// "domain.<name>" — e.g. domain.Finding or domain.AttentionDetail.
func mfnlIsDomainSelector(expr ast.Expr, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "domain"
}

// mfnlHasBareLeaf reports whether typ is one or more map[string]-keyed
// layers whose FINAL value type is the bare "domain.<leafName>" selector —
// the legacy single-value shape GATE 1 (leafName="Finding") and GATE 2
// (leafName="AttentionDetail") ban. Recurses through nested map[string]
// layers (so map[string]map[string]domain.Finding is caught, matching
// RuntimeState.EnrichmentFindings' actual shape) but returns false the
// moment a layer's key is anything other than the bare "string" identifier
// (so map[string]map[domain.FindingCode]domain.AttentionDetail — the target
// shape — is correctly NOT flagged) or the terminal value is not the bare
// selector (so map[string][]domain.Finding — the other target shape — is
// correctly NOT flagged either).
func mfnlHasBareLeaf(typ ast.Expr, leafName string) bool {
	mt, ok := typ.(*ast.MapType)
	if !ok {
		return false
	}
	if !mfnlIsStringIdent(mt.Key) {
		return false
	}
	if mfnlIsDomainSelector(mt.Value, leafName) {
		return true
	}
	if inner, ok := mt.Value.(*ast.MapType); ok {
		return mfnlHasBareLeaf(inner, leafName)
	}
	return false
}

// mfnlFieldViolation is one struct-field-type violation found by
// mfnlScanFileForBareLeaf.
type mfnlFieldViolation struct {
	file       string
	line       int
	structName string
	fieldName  string
}

func (v mfnlFieldViolation) String() string {
	return fmt.Sprintf("%s:%d: %s.%s", v.file, v.line, v.structName, v.fieldName)
}

// mfnlScanFileForBareLeaf parses path and returns one mfnlFieldViolation for
// every struct field (across every struct declared in the file) whose type
// matches mfnlHasBareLeaf for leafName.
func mfnlScanFileForBareLeaf(fset *token.FileSet, path, leafName string) ([]mfnlFieldViolation, error) {
	src, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	rel := filepath.Base(path)
	var violations []mfnlFieldViolation
	for _, decl := range src.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			for _, field := range st.Fields.List {
				if !mfnlHasBareLeaf(field.Type, leafName) {
					continue
				}
				pos := fset.Position(field.Pos())
				for _, name := range field.Names {
					violations = append(violations, mfnlFieldViolation{
						file:       rel,
						line:       pos.Line,
						structName: ts.Name.Name,
						fieldName:  name.Name,
					})
				}
			}
		}
	}
	return violations, nil
}

// mfnlScanContractFiles runs mfnlScanFileForBareLeaf(leafName) over every
// file in mfnlContractFiles, failing the test via t.Fatalf on any missing
// file or parse error rather than silently under-scanning.
func mfnlScanContractFiles(t *testing.T, leafName string) []mfnlFieldViolation {
	t.Helper()
	fset := token.NewFileSet()
	var violations []mfnlFieldViolation
	for _, rel := range mfnlContractFiles {
		abs, err := filepath.Abs(rel)
		if err != nil {
			t.Fatalf("filepath.Abs(%q): %v", rel, err)
		}
		if _, statErr := os.Stat(abs); statErr != nil {
			t.Fatalf("scan target %s does not exist: %v", rel, statErr)
		}
		found, scanErr := mfnlScanFileForBareLeaf(fset, abs, leafName)
		if scanErr != nil {
			t.Fatalf("parse error in %s: %v", rel, scanErr)
		}
		violations = append(violations, found...)
	}
	return violations
}

// TestMultiFindingNoLegacyGate1_NoSingleFindingCompatFieldInEnrichmentContracts
// is GATE 1: no struct field in the enrichment-contract surface
// (mfnlContractFiles) may be typed as a map[string]-keyed chain (flat or
// nested) whose terminal value is the bare domain.Finding — the single-
// representative compat shape the v3.47.0 append-only builder work was
// supposed to retire. The only allowed finding-map shape in these contracts
// is map[string][]domain.Finding (e.g. ListEnrichmentPatch.AllFindings,
// PatchDetail.EnrichmentFindings, EnrichmentChecked.AllFindings — all
// correctly NOT flagged). domain.Resource.Findings ([]domain.Finding, not a
// map at all) is out of scope entirely — this gate only ever inspects the
// four files in mfnlContractFiles. See this file's header comment for the
// verified 4-violation census.
func TestMultiFindingNoLegacyGate1_NoSingleFindingCompatFieldInEnrichmentContracts(t *testing.T) {
	violations := mfnlScanContractFiles(t, "Finding")
	if len(violations) == 0 {
		return
	}
	lines := make([]string, len(violations))
	for i, v := range violations {
		lines[i] = v.String()
	}
	sort.Strings(lines)
	t.Errorf(
		"GATE 1 — %d single-finding compat field(s) found in the enrichment-contract surface "+
			"(the only allowed finding-map shape here is map[string][]domain.Finding):\n%s\n\n"+
			"Remove/replace each field above (and its call sites) so no enrichment-contract struct "+
			"can carry only ONE Finding per resource — see the AllFindings/EnrichmentFindings "+
			"siblings already present in the same structs for the correct multi-finding shape to "+
			"converge every caller onto.",
		len(violations), strings.Join(lines, "\n"),
	)
}

// TestMultiFindingNoLegacyGate2_NoSingleAttentionDetailCompatFieldInEnrichmentContracts
// is GATE 2: no struct field in the enrichment-contract surface
// (mfnlContractFiles) may be typed as a map[string]-keyed chain (flat or
// nested) whose terminal value is the bare domain.AttentionDetail — one
// row-set per resource, unable to represent a second independently-
// evaluated finding's own supporting rows. The only allowed shape is
// map[string]map[domain.FindingCode]domain.AttentionDetail (Resource.ID,
// then FindingCode) — e.g. ListEnrichmentPatch.AttentionDetailsAll,
// PatchDetail.EnrichmentAttentionDetails, EnrichmentChecked.AttentionDetails
// are all correctly NOT flagged, as are the single-resource
// map[domain.FindingCode]domain.AttentionDetail fields (PatchDetail.
// Attention, RuntimeState.DetailAttention) which have no outer Resource.ID
// layer at all. See this file's header comment for the verified
// 2-violation census.
func TestMultiFindingNoLegacyGate2_NoSingleAttentionDetailCompatFieldInEnrichmentContracts(t *testing.T) {
	violations := mfnlScanContractFiles(t, "AttentionDetail")
	if len(violations) == 0 {
		return
	}
	lines := make([]string, len(violations))
	for i, v := range violations {
		lines[i] = v.String()
	}
	sort.Strings(lines)
	t.Errorf(
		"GATE 2 — %d single-AttentionDetail compat field(s) found in the enrichment-contract "+
			"surface (the only allowed shape here is "+
			"map[string]map[domain.FindingCode]domain.AttentionDetail):\n%s\n\n"+
			"Remove/replace each field above (and its call sites) so no enrichment-contract struct "+
			"can carry only ONE AttentionDetail row-set per resource regardless of how many "+
			"independently-evaluated findings that resource has.",
		len(violations), strings.Join(lines, "\n"),
	)
}

// ---------------------------------------------------------------------------
// GATE 3: no direct IssueEnricherResult.Findings/.AttentionDetails index
// WRITE outside the builder.
// ---------------------------------------------------------------------------

// mfnlWriteViolation is one direct-write violation found by
// mfnlScanFileForDirectWrite.
type mfnlWriteViolation struct {
	file      string
	line      int
	fieldName string
}

func (v mfnlWriteViolation) String() string {
	return fmt.Sprintf("%s:%d: direct write to .%s[...] bypasses setWave2Finding", v.file, v.line, v.fieldName)
}

// mfnlScanFileForDirectWrite parses path and returns one mfnlWriteViolation
// for every assignment statement whose left-hand side indexes a
// "<expr>.Findings" or "<expr>.AttentionDetails" selector — matched purely
// on the selector name (not a resolved static type), the same
// lexical-matching convention
// qa_enricher_finding_builder_discipline_test.go's efbdIsFindingsSelector
// uses. This intentionally only inspects *ast.AssignStmt (a WRITE); it
// never inspects *ast.RangeStmt or a bare read IndexExpr, so the four
// benign post-append IssueCount range-aggregation loops
// qa_enricher_finding_builder_discipline_test.go already allowlists
// (eb_rule/ec2/ecr/elb) are structurally outside this gate's scan and need
// no allowlist entry here.
func mfnlScanFileForDirectWrite(fset *token.FileSet, path string) ([]mfnlWriteViolation, error) {
	src, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	rel := filepath.Base(path)
	var violations []mfnlWriteViolation
	ast.Inspect(src, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assign.Lhs {
			idx, ok := lhs.(*ast.IndexExpr)
			if !ok {
				continue
			}
			sel, ok := idx.X.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if sel.Sel.Name != "Findings" && sel.Sel.Name != "AttentionDetails" {
				continue
			}
			pos := fset.Position(lhs.Pos())
			violations = append(violations, mfnlWriteViolation{
				file:      rel,
				line:      pos.Line,
				fieldName: sel.Sel.Name,
			})
		}
		return true
	})
	return violations, nil
}

// TestMultiFindingNoLegacyGate3_NoDirectResultFieldWriteOutsideBuilder is
// GATE 3: no core/aws/*_issue_enrichment.go enricher, and no
// core/aws/snapshot_cross_ref.go cross-ref enricher, may assign
// directly into a "*.Findings[id]" or "*.AttentionDetails[id]" index
// expression. setWave2Finding is today's sole append-only builder for both
// fields; this gate does not require a specific builder name — it only
// bans a file writing the index expression directly.
//
// Glob scope deliberately mirrors
// qa_enricher_finding_builder_discipline_test.go's: "*_issue_enrichment.go"
// does not match "issue_enrichment.go" itself (20 characters, shorter than
// the 21-character "_issue_enrichment.go" suffix required) — setWave2Finding's
// own legitimate r.Findings[resourceID]/r.AttentionDetails[resourceID]
// writes (issue_enrichment.go:144/153) are never in this gate's scan scope
// either. This gate additionally scans snapshot_cross_ref.go explicitly
// because its filename does not match that glob — the sibling
// read/range-access gate does not scan it at all.
func TestMultiFindingNoLegacyGate3_NoDirectResultFieldWriteOutsideBuilder(t *testing.T) {
	root, err := filepath.Abs("../../core/aws")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	pattern := filepath.Join(root, "*_issue_enrichment.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q): %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no *_issue_enrichment.go files found under %s — check path/glob", root)
	}
	crossRef := filepath.Join(root, "snapshot_cross_ref.go")
	if _, statErr := os.Stat(crossRef); statErr != nil {
		t.Fatalf("scan target %s does not exist: %v", crossRef, statErr)
	}
	files = append(files, crossRef)

	fset := token.NewFileSet()
	var violations []mfnlWriteViolation
	for _, path := range files {
		found, scanErr := mfnlScanFileForDirectWrite(fset, path)
		if scanErr != nil {
			t.Fatalf("parse error in %s: %v", path, scanErr)
		}
		violations = append(violations, found...)
	}
	if len(violations) == 0 {
		return
	}
	lines := make([]string, len(violations))
	for i, v := range violations {
		lines[i] = v.String()
	}
	sort.Strings(lines)
	t.Errorf(
		"GATE 3 — %d direct IssueEnricherResult field write(s) found outside the builder:\n%s\n\n"+
			"Route every write through setWave2Finding (core/aws/issue_enrichment.go) or an "+
			"equivalent append-only builder instead of assigning result.Findings[id]/"+
			"result.AttentionDetails[id] directly.",
		len(violations), strings.Join(lines, "\n"),
	)
}

// ---------------------------------------------------------------------------
// GATE 4: no stale single-finding doc comments.
// ---------------------------------------------------------------------------

// mfnlDocPattern is one stale-comment pattern GATE 4 watches for.
type mfnlDocPattern struct {
	label   string
	matches func(lowerText string) bool
}

// mfnlDocPatterns lists the four stale single-finding doc-comment patterns
// GATE 4 watches for, matched case-insensitively against each comment
// group's stripped text (ast.CommentGroup.Text(), obtained via
// parser.ParseComments) — never against source code, so a helper function
// or variable merely *named* similarly cannot false-positive this gate.
//
// The second pattern is an unordered co-occurrence check (all three tokens
// present somewhere in the same comment group) rather than a strict
// substring, because the one real occurrence at seeding time
// (internal/tui/app_enrich_fold.go:3, in the phrase "applyEnrichment is the
// canonical write path for Wave 2 results.") has "path" appearing BEFORE
// "Wave 2", not after — Go's RE2 regexp engine has no lookaround, so this
// is three independent Contains checks rather than one ordered regex.
var mfnlDocPatterns = []mfnlDocPattern{
	{
		label: `"at most one wave2 finding"`,
		matches: func(lowerText string) bool {
			return strings.Contains(lowerText, "at most one wave2 finding")
		},
	},
	{
		label: `"canonical ... wave 2 ... path" (unordered co-occurrence)`,
		matches: func(lowerText string) bool {
			return strings.Contains(lowerText, "canonical") &&
				strings.Contains(lowerText, "path") &&
				(strings.Contains(lowerText, "wave 2") || strings.Contains(lowerText, "wave2"))
		},
	},
	{
		label: `"worse-severity wins the slot"`,
		matches: func(lowerText string) bool {
			return strings.Contains(lowerText, "worse-severity wins the slot")
		},
	},
	{
		label: `"attach to first finding"`,
		matches: func(lowerText string) bool {
			return strings.Contains(lowerText, "attach to first finding")
		},
	},
}

// mfnlDocViolation is one stale doc-comment violation found by
// TestMultiFindingNoLegacyGate4_NoStaleSingleFindingDocComments.
type mfnlDocViolation struct {
	file    string
	line    int
	pattern string
	snippet string
}

func (v mfnlDocViolation) String() string {
	return fmt.Sprintf("%s:%d: comment matches %s: %q", v.file, v.line, v.pattern, v.snippet)
}

// TestMultiFindingNoLegacyGate4_NoStaleSingleFindingDocComments is GATE 4:
// no non-test .go file under internal/ may carry a comment describing the
// retired single-finding-per-resource contract as though it were still
// true. Scans every comment group (parser.ParseComments) — never source
// code — for each of the four case-insensitive patterns in
// mfnlDocPatterns. Mirrors architecture_conformance_test.go's existing
// internal/-tree walk convention (skip directories, skip non-.go files,
// skip _test.go files) so a test's own regression-check strings (e.g.
// controller_regression_test.go's "first finding still in Attention after
// second apply" failure message, which asserts the OLD behavior does NOT
// happen) can never trip this gate.
//
// All four patterns stay active whether or not they currently match: the
// gate exists to catch a regression, not to report today's count.
func TestMultiFindingNoLegacyGate4_NoStaleSingleFindingDocComments(t *testing.T) {
	fset := token.NewFileSet()
	var violations []mfnlDocViolation
	for _, scanRoot := range []string{"../../core", "../../internal"} {
		root, err := filepath.Abs(scanRoot)
		if err != nil {
			t.Fatalf("filepath.Abs: %v", err)
		}
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if perr != nil {
				return perr
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			for _, cg := range src.Comments {
				text := cg.Text()
				if strings.TrimSpace(text) == "" {
					continue
				}
				lower := strings.ToLower(text)
				pos := fset.Position(cg.Pos())
				for _, p := range mfnlDocPatterns {
					if !p.matches(lower) {
						continue
					}
					violations = append(violations, mfnlDocViolation{
						file:    rel,
						line:    pos.Line,
						pattern: p.label,
						snippet: strings.TrimSpace(strings.ReplaceAll(text, "\n", " ")),
					})
				}
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s failed: %v", root, walkErr)
		}
	}
	if len(violations) == 0 {
		return
	}
	lines := make([]string, len(violations))
	for i, v := range violations {
		lines[i] = v.String()
	}
	sort.Strings(lines)
	t.Errorf(
		"GATE 4 — %d stale single-finding doc comment(s) found:\n%s\n\n"+
			"Update or delete each comment above to describe the current append-only, "+
			"multi-finding-per-resource contract (setWave2Finding / IssueEnricherResult.Findings "+
			"map[string][]domain.Finding) instead of the retired single-finding one.",
		len(violations), strings.Join(lines, "\n"),
	)
}
