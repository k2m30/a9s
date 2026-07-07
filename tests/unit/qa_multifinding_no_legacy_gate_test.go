// qa_multifinding_no_legacy_gate_test.go — owner directive: the v3.47.0
// multi-finding work (setWave2Finding as an append-only builder,
// IssueEnricherResult.Findings as map[string][]domain.Finding, per
// .claude/agent-memory/a9s-qa/project_framework_limit_52_carry_layer_already_multifinding_safe.md)
// must not coexist indefinitely with the single-finding compat layer
// Reshape A/B deliberately preserved as an interim bridge (see
// .claude/agent-memory/a9s-qa/reference_attentiondetails_nested_map_test_migration.md).
// These four gates enforce the "no legacy" end state: each is RED today
// against the shim-laden code and must go GREEN once the coder purges every
// violation this file lists.
//
// DESIGN CHOICE (stated once — applies to all four gates below): no
// allowlist. Each gate does a fresh scan every run and unconditionally
// fails when it finds ANY matching violation, listing every one in the
// failure message. This is deliberately different from this package's
// other ratchet-style gates (qa_controller_construction_discipline_test.go,
// qa_enricher_finding_builder_discipline_test.go), which allowlist known
// debt with t.Skipf and stay green around it — those track slow multi-PR
// burn-down of pre-existing debt. These four gates instead pin a hard purge
// deadline on NEW, deliberately-scoped debt: the coder's very next change
// either removes every violation below or the gate stays red. Zero
// allowlist also means zero test-file edits are needed for the fix to turn
// these green (see
// .claude/agent-memory/a9s-qa/feedback_compile_red_shared_package_blast_radius.md
// on why that property is worth choosing deliberately) — and, since none of
// the four gates requires a new production symbol to exist, this file
// compiles clean against HEAD today; "RED" here means "runs and fails via
// t.Errorf", never a compile break, so it carries zero blast radius for any
// concurrent sibling work in this package.
//
// VERIFIED CENSUS (2026-07-07, this exact file's scanners run against HEAD
// by hand before this file was written; corrections to the originating
// dispatch's claims are called out inline per gate):
//
//   GATE 1 (4 violations): internal/runtime/intent.go:36
//   (ListEnrichmentPatch.Findings), internal/runtime/state.go:18
//   (RuntimeState.EnrichmentFindings), internal/app/viewstate.go:148
//   (ListBody.EnrichmentFindings), internal/runtime/messages/event.go:244
//   (EnrichmentChecked.Findings). Dispatch cited intent.go:31 and
//   state.go:16 — both are the doc-comment lines directly above the real
//   field declarations (31 is ListEnrichmentPatch's struct-level doc start;
//   16 is EnrichmentFindings' own leading comment); the field declarations
//   themselves are at 36 and 18 respectively, verified by direct read
//   (mirrors the same dispatch-cites-the-comment-not-the-code pattern noted
//   in qa_enricher_finding_builder_discipline_test.go's own header).
//
//   GATE 2 (2 violations): internal/runtime/intent.go:37
//   (ListEnrichmentPatch.AttentionDetails), internal/runtime/state.go:22
//   (RuntimeState.EnrichmentAttentionDetails).
//
//   GATE 3 (2 violations): internal/aws/snapshot_cross_ref.go:217
//   (result.Findings[res.ID] = ...) and
//   internal/aws/snapshot_cross_ref.go:224
//   (result.AttentionDetails[res.ID] = ...) — both inside
//   EnrichSnapshotCrossRef. Zero violations found in any
//   internal/aws/*_issue_enrichment.go file — the append-only builder
//   discipline is already clean there (matches
//   qa_enricher_finding_builder_discipline_test.go's own green census after
//   the MSK fix landed), verified by grep before writing this gate rather
//   than assumed.
//
//   GATE 4 (3 violations, of 4 candidate patterns): internal/tui/
//   app_enrich_fold.go:3 (go/ast merges the file's two leading
//   package-level comment blocks — lines 3-4 and 6-8, despite the blank
//   line 5 between them — into one CommentGroup anchored at line 3; its
//   merged text contains "applyEnrichment is the canonical write path for
//   Wave 2 results.", matching the "canonical ... wave 2 ... path"
//   unordered pattern; verified against the actual t.Errorf output, not
//   assumed from source line numbers alone), and internal/tui/
//   app_enrich_fold.go:131 and :155 (each "break // at most one wave2
//   finding per resource", in findingsFromRows and attentionDetailsFromRows
//   respectively).
//   CORRECTION to the originating dispatch: it also named
//   internal/aws/issue_enrichment.go and "others" as carrying these
//   phrases, and named two more literal patterns — "worse-severity wins the
//   slot" and "attach to first finding". A case-insensitive scan of the
//   whole internal/ tree (non-test files) at HEAD found ZERO occurrences of
//   either phrase anywhere, and ZERO occurrences of any of the four
//   patterns in issue_enrichment.go specifically — its setWave2Finding doc
//   comment already describes the CURRENT append-style multi-finding
//   contract ("every independently-evaluated condition survives as its own
//   Finding"), not a stale single-finding one. Both patterns remain
//   implemented in mfnlDocPatterns below — they will catch a real future
//   regression — but neither seeds a violation today, and this file does
//   not invent one to force a match (see
//   .claude/agent-memory/a9s-qa/feedback_verify_dispatch_claims_against_code.md).
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
	"../../internal/runtime/intent.go",
	"../../internal/runtime/state.go",
	"../../internal/app/viewstate.go",
	"../../internal/runtime/messages/event.go",
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
// GATE 3: no internal/aws/*_issue_enrichment.go enricher, and no
// internal/aws/snapshot_cross_ref.go cross-ref enricher, may assign
// directly into a "*.Findings[id]" or "*.AttentionDetails[id]" index
// expression. setWave2Finding is today's sole append-only builder for both
// fields; this gate does not require a specific replacement name (the
// coder may introduce a new builder, e.g. an AddFinding method) — it only
// bans a NEW/remaining file writing the index expression directly.
//
// Glob scope deliberately mirrors
// qa_enricher_finding_builder_discipline_test.go's: "*_issue_enrichment.go"
// does not match "issue_enrichment.go" itself (20 characters, shorter than
// the 21-character "_issue_enrichment.go" suffix required) — setWave2Finding's
// own legitimate r.Findings[resourceID]/r.AttentionDetails[resourceID]
// writes (issue_enrichment.go:144/153) are never in this gate's scan scope
// either. This gate additionally scans snapshot_cross_ref.go explicitly
// because its filename does not match that glob and, unlike every
// *_issue_enrichment.go file, it has real violations today — the sibling
// read/range-access gate does not scan it at all.
//
// Verified 2-violation census (see this file's header comment): zero
// violations exist in any *_issue_enrichment.go file today — the
// append-only discipline is already clean there.
func TestMultiFindingNoLegacyGate3_NoDirectResultFieldWriteOutsideBuilder(t *testing.T) {
	root, err := filepath.Abs("../../internal/aws")
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
			"Route every write through setWave2Finding (internal/aws/issue_enrichment.go) or an "+
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
// Verified 3-violation census (see this file's header comment for the full
// correction against the originating dispatch's broader claim): all three
// live in internal/tui/app_enrich_fold.go — line 6 (the file's own
// "canonical write path" package-level note) and lines 131/155 (matching
// "break // at most one wave2 finding per resource" trailing comments in
// findingsFromRows/attentionDetailsFromRows respectively). The other two
// patterns ("worse-severity wins the slot", "attach to first finding") are
// implemented but currently match nothing anywhere in internal/ — kept
// active to catch a future regression, not to force today's count higher
// than what a direct scan actually finds.
func TestMultiFindingNoLegacyGate4_NoStaleSingleFindingDocComments(t *testing.T) {
	root, err := filepath.Abs("../../internal")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	fset := token.NewFileSet()
	var violations []mfnlDocViolation
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
		t.Fatalf("walk internal/ failed: %v", walkErr)
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
