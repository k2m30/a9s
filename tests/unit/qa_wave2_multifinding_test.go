// qa_wave2_multifinding_test.go — pins OWNER CONTRACT #52 (multi-finding).
//
// #52's contract: IssueEnricherResult.Findings is map[string][]domain.Finding.
// Enrichers may emit N independently-evaluated findings per resource; ALL fold
// onto domain.Resource.Findings (a plain []domain.Finding slice); the worst
// severity drives td.ResolveColor (colorFromAnyFinding scans the WHOLE
// r.Findings slice, order-independent); and the detail-view Attention block
// lists each finding as its own entry — own Phrase, own Detail — never a
// demoted row of another finding (buildAttentionEntries iterates ds.Findings,
// one attentionEntry per issue-severity finding).
//
// This suite drives the REAL, exported runtime.ApplyWave2ToRow fold and the
// real awsclient.EnrichOpenSearchDomains against the landed
// map[string][]domain.Finding shape, asserting the downstream behavior below.
//
// Section map:
//
//  1. TestFold_TwoWave2FindingsSameResource_* (4 subtests) — construct the
//     IssueEnricherResult.Findings entry the way a hypothetical two-finding
//     ecs-svc enricher WOULD under the new contract (mock-style map value, no
//     real IssueEnricher invoked, mirroring the existing TestEnrich*
//     seam-level tests), fold it through the REAL runtime.ApplyWave2ToRow,
//     and assert the intended downstream shape: both findings land in
//     r.Findings, the worst severity drives td.ResolveColor, the detail
//     Attention block lists both phrases, and the list Status cell shows the
//     worst finding's phrase stacked with "(+1)" — this suffix is NOT
//     speculative: internal/app/list_columns.go's listPhraseFromFindings
//     already renders "<top phrase> (+N)" for any len(findings)>1 row
//     (verified by reading its source, and independently exercised GREEN by
//     this file's own Section 2 carry tests, which seed r.Findings directly
//     and already observe the same suffix at HEAD).
//  2. TestReconcileTypeFile_Wave2Carry_MultiFinding* / TestRestartSeed_MultiFinding_*
//     (2 tests) — mirror runtime_wave2_carry_test.go's Test 1 (disk refresh)
//     and Test 4 (restart seed) with a row directly seeded with TWO
//     wave2-sourced domain.Finding entries. VERIFIED EMPIRICALLY (not
//     assumed from the dispatch): cache.Row.Findings and domain.Resource.
//     Findings are plain []domain.Finding slices — carryWave2/
//     carryWave2ForRows/wave2FindingsOf (internal/runtime/wave2_carry.go)
//     iterate and copy the WHOLE slice, with no per-resource cap — so the
//     C6b carry path (disk reconcile + restart seed) is already
//     multi-finding-safe and touches NO part of the #52 seam
//     (IssueEnricherResult.Findings / ApplyWave2ToRow are never referenced by
//     either test in this section). Kept here as a GREEN regression guard,
//     not a RED framework-limit pin — it documents the fix boundary so a
//     future enricher-side change does not need to touch the carry layer,
//     and would be caught here if it accidentally did. (Collateral: both
//     tests are swept into this FILE's package-level compile failure by
//     Section 1/3's intentional seam errors above — that is a build-system
//     fact about `go test` compiling a package atomically, not a defect in
//     these two tests' own logic.)
//  3. TestOpenSearch_Enrich_MultiBackground_BothConditionsSurfaceAsOwnFindings
//     — the concrete opensearch case: a MultiBackgroundDomain resource (both
//     forced-update and encryption-off) must produce TWO Finding entries in
//     result.Findings[id] (as a []domain.Finding under the #52 contract),
//     each carrying its own Code/Phrase/Detail, and must NOT cram the hidden
//     condition's phrase into a generic {Label:"Additional"} DetailRow.
//     Companion note: the prior pin TestOpenSearch_Enrich_MultiBackground_
//     TopWinsHiddenSurfacesAsRow (aws_opensearch_issue_enrichment_test.go),
//     which asserted exactly the CURRENT (limited) "! wins, ~ hides in a
//     row" shape this test says must change, has been RETIRED (deleted, with
//     a pointer comment back to this file) as part of this same change.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// ────────────────────────────────────────────────────────────────────────────
// Section 1 — fold: two intended Wave-2 findings for one resource
// ────────────────────────────────────────────────────────────────────────────

const multiFindingResourceID = "ecs-svc-fold-multi"

// multiFindingBang and multiFindingTilde are the two independent Wave-2
// findings an ecs-svc enricher WOULD emit for the same resource once #52
// lifts. Constructed mock-style — no real IssueEnricher invoked — mirroring
// the existing TestEnrich* seam-level tests (aws_opensearch_issue_enrichment_test.go
// et al.).
var (
	multiFindingBang = domain.Finding{
		Code:     domain.FindingCode("ecs-svc.deployment-failed"),
		Phrase:   "deployment failed",
		Detail:   "The service's latest deployment rolled back after failing health checks.",
		Severity: domain.SevBroken,
		Source:   "wave2:ecs-svc",
	}
	multiFindingTilde = domain.Finding{
		Code:     domain.FindingCode("ecs-svc.task-def-drift"),
		Phrase:   "task definition drifted from desired revision",
		Detail:   "Running tasks reference an older task definition revision than the service's desired one.",
		Severity: domain.SevWarn,
		Source:   "wave2:ecs-svc",
	}
)

// buildFoldedMultiFindingRow constructs the IssueEnricherResult.Findings
// entry the way a hypothetical two-finding ecs-svc enricher WOULD under the
// #52 contract — both findings held in a single []domain.Finding slice value
// under the shared resource-ID key, bang (SevBroken, the worse of the two)
// ordered first — then folds it through the REAL runtime.ApplyWave2ToRow.
//
// COMPILE-RED (intended, see file header): findings here is
// map[string][]domain.Finding; ApplyWave2ToRow at HEAD still declares its
// findings parameter as map[string]domain.Finding. This mismatch is #52's
// exact fix boundary, not a bug in this helper.
func buildFoldedMultiFindingRow(t *testing.T) (*resource.ResourceTypeDef, domain.Resource) {
	t.Helper()
	td := resource.FindResourceType("ecs-svc")
	if td == nil {
		t.Fatal(`resource.FindResourceType("ecs-svc") = nil — ecs-svc must stay registered for this pin`)
	}

	findings := map[string][]domain.Finding{
		multiFindingResourceID: {multiFindingBang, multiFindingTilde},
	}
	attentionDetails := map[string]map[domain.FindingCode]domain.AttentionDetail{}

	r := domain.Resource{ID: multiFindingResourceID, Name: multiFindingResourceID, Type: "ecs-svc"}
	runtime.ApplyWave2ToRow(&r, *td, findings, attentionDetails)
	return td, r
}

func TestFold_TwoWave2FindingsSameResource_BothFindingsSurviveInRFindings(t *testing.T) {
	_, r := buildFoldedMultiFindingRow(t)
	if len(r.Findings) != 2 {
		t.Fatalf("r.Findings has %d entries (%+v), want 2 — once ApplyWave2ToRow folds a map[string][]domain.Finding (#52), it must append EVERY finding for the resource ID onto r.Findings, not just one", len(r.Findings), r.Findings)
	}
	var haveBang, haveTilde bool
	for _, f := range r.Findings {
		if f.Code == multiFindingBang.Code {
			haveBang = true
		}
		if f.Code == multiFindingTilde.Code {
			haveTilde = true
		}
	}
	if !haveBang {
		t.Errorf("r.Findings missing Code %q (%q) — %+v", multiFindingBang.Code, multiFindingBang.Phrase, r.Findings)
	}
	if !haveTilde {
		t.Errorf("r.Findings missing Code %q (%q) — %+v", multiFindingTilde.Code, multiFindingTilde.Phrase, r.Findings)
	}
}

func TestFold_TwoWave2FindingsSameResource_WorstSeverityDrivesResolveColor(t *testing.T) {
	td, r := buildFoldedMultiFindingRow(t)
	color := td.ResolveColor(r)
	if color != domain.ColorBroken {
		t.Errorf("td.ResolveColor(r) = %d, want %d (ColorBroken — SevBroken %q is the worst of the two intended findings) — colorFromAnyFinding already scans the WHOLE r.Findings slice for the worst severity (order-independent); this fails only if ApplyWave2ToRow drops one of the two findings during the fold instead of appending both (#52)",
			color, domain.ColorBroken, multiFindingBang.Phrase)
	}
}

func TestFold_TwoWave2FindingsSameResource_DetailAttentionListsBothPhrases(t *testing.T) {
	_, r := buildFoldedMultiFindingRow(t)

	ctrl := newTestController(t)
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	ctrl.EnsureDetailState(r, "ecs-svc")

	body := ctrl.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after EnsureDetailState")
	}
	var attentionText []string
	for _, f := range body.Fields {
		if f.Path == "Attention" {
			attentionText = append(attentionText, f.Value)
		}
	}
	// Case-insensitive: buildAttentionEntries renders through
	// capitalizeFirstDetail (internal/app/detail_body.go), which upper-cases
	// the first letter for display ("deployment failed" -> "Deployment
	// failed"). The contract this pins is that BOTH findings' phrases survive
	// the fold and each surfaces as its own Attention entry — not the exact
	// casing of a display convention.
	joined := strings.ToLower(strings.Join(attentionText, " | "))
	if !strings.Contains(joined, multiFindingBang.Phrase) {
		t.Errorf("detail Attention block missing bang phrase %q — buildAttentionEntries (internal/app/detail_body.go) builds one entry per issue-severity r.Findings element with its OWN Phrase/Detail; this fails only if ApplyWave2ToRow still drops a finding during the fold (#52); got Attention rows: %v", multiFindingBang.Phrase, attentionText)
	}
	if !strings.Contains(joined, multiFindingTilde.Phrase) {
		t.Errorf("detail Attention block missing tilde phrase %q; got Attention rows: %v", multiFindingTilde.Phrase, attentionText)
	}
}

func TestFold_TwoWave2FindingsSameResource_ListStatusCellShowsWorstPhrase(t *testing.T) {
	_, r := buildFoldedMultiFindingRow(t)

	ctrl := newTestController(t)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ecs-svc"})
	ctrl.ApplyResourcesLoaded("ecs-svc", []resource.Resource{r}, nil, false)

	body := ctrl.Snapshot().Body.List
	if body == nil {
		t.Fatal("Body.List is nil after opening ecs-svc list")
	}
	if body.StatusCol < 0 {
		t.Fatal("ecs-svc list has no Status column — test assumption broken, cannot exercise the Status-cell pin")
	}

	var cell string
	found := false
	for _, row := range body.Rows {
		if row.ResourceID != multiFindingResourceID {
			continue
		}
		found = true
		if body.StatusCol < len(row.Cells) {
			cell = row.Cells[body.StatusCol]
		}
		break
	}
	if !found {
		t.Fatalf("row %q not found in ecs-svc list body", multiFindingResourceID)
	}

	// "(+1)" is the REAL render rule, not a guess: listPhraseFromFindings
	// (internal/app/list_columns.go) already renders "<top phrase> (+N)" for
	// any row whose r.Findings has more than one entry, and picks the FIRST
	// issue-severity entry as <top phrase> — bang is ordered first in
	// buildFoldedMultiFindingRow's slice AND is the worse severity, so both
	// readings of "top" agree here. This exact suffix is independently
	// exercised GREEN today by Section 2's carry tests, which seed
	// r.Findings=[]domain.Finding{bang, tilde} directly (bypassing the #52
	// enricher-map seam entirely) and already observe "<bang phrase> (+1)".
	want := multiFindingBang.Phrase + " (+1)"
	if cell != want {
		t.Errorf("Status cell = %q, want %q — the worst (SevBroken) finding must lead the stacked phrase once both findings survive the fold; this fails only if ApplyWave2ToRow still drops the tilde finding (#52), since listPhraseFromFindings itself needs no change", cell, want)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Section 2 — C6b cache carry: does the multi-finding shape survive a
// row-replacing refresh and a restart seed? cache.Row.Findings and
// domain.Resource.Findings are plain []domain.Finding slices — UNCHANGED by
// #52 (only IssueEnricherResult.Findings, the enricher-result type, changes
// shape) — so these tests seed the multi-finding row directly, never
// touching IssueEnricherResult or ApplyWave2ToRow, and stay GREEN
// characterization tests, not RED framework-limit pins. They scope the
// coder's fix to the enricher-result type + fold (Section 1) and act as a
// regression guard so a future fix there does not regress the carry layer.
// (Both are nonetheless swept into this file's package-level compile
// failure by Section 1/3's intentional seam errors — see file header.)
// ────────────────────────────────────────────────────────────────────────────

const (
	wave2CarryMultiShortName  = "wave2carrymulti"
	wave2CarryMultiResourceID = "ecs-svc-carry-multi"
)

func TestReconcileTypeFile_Wave2Carry_MultiFindingRowSurvivesRefresh(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	// newSaveCacheRegressionCore bootstraps its Core against
	// saveRegProfile/saveRegRegion (runtime_savecache_regressions_test.go) —
	// the seed must land in the same pair the Core under test reads/writes.
	store := cache.LoadDir(saveRegProfile, saveRegRegion)
	existingRows := []cache.Row{
		{
			ID:   wave2CarryMultiResourceID,
			Name: wave2CarryMultiResourceID,
			Fields: map[string]string{
				"status": multiFindingBang.Phrase + " (+1)",
			},
			Findings: []domain.Finding{multiFindingBang, multiFindingTilde},
		},
	}
	store.Put(wave2CarryMultiShortName, cache.TypeFile{
		HasResources: true, Count: 1, Exact: true, Rows: existingRows,
	})
	if err := store.SaveType(wave2CarryMultiShortName); err != nil {
		t.Fatalf("seed SaveType(%s): %v", wave2CarryMultiShortName, err)
	}

	c := newSaveCacheRegressionCore(t, false)
	// Bare same-ID refresh, no Findings/status — the shape a fresh Wave-1
	// sweep-completion save produces at HEAD.
	freshRows := []cache.Row{
		{ID: wave2CarryMultiResourceID, Name: wave2CarryMultiResourceID},
	}
	if err := c.SaveResourceListCache(wave2CarryMultiShortName, freshRows, 1, true, 0, false, false); err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDir(saveRegProfile, saveRegRegion)
	tf, ok := reloaded.Type(wave2CarryMultiShortName)
	if !ok {
		t.Fatal("TypeFile missing after multi-finding Wave-2-carry refresh")
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("TypeFile.Rows has %d entries, want 1", len(tf.Rows))
	}
	row := tf.Rows[0]
	if len(row.Findings) != 2 {
		t.Errorf("row Findings has %d entries (%+v), want 2 — a bare refresh must carry forward BOTH of the replaced row's wave2-sourced findings (C6b multi-finding carry), not just one", len(row.Findings), row.Findings)
	}
	if got := row.Fields["status"]; got != multiFindingBang.Phrase+" (+1)" {
		t.Errorf(`row Fields["status"] = %q, want %q (C6b: enricher Fields carry alongside the findings)`, got, multiFindingBang.Phrase+" (+1)")
	}
}

func TestRestartSeed_MultiFinding_BothFindingsVisibleOnFirstRender(t *testing.T) {
	// newTestControllerWithCore constructs via runtime.New(session, nil) —
	// functionally identical to the runtime.Bootstrap(profile, region, nil)
	// this test used before (Bootstrap is a thin New(sessionWithPairSet, ...)
	// wrapper — internal/runtime/accessors.go). Neither touches disk at
	// construction time; EnsureCacheStore's first call is what triggers
	// cache.LoadDir, and that first call happens below, AFTER the helper's
	// t.Setenv("A9S_CONFIG_FOLDER", t.TempDir()) has already pinned this
	// test's own isolated config root — so seeding after construction (but
	// before ctrl.Apply's cache read) is equivalent to seeding before it.
	core, ctrl := newTestControllerWithCore(t)

	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("ecs-svc", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{
				ID:   wave2CarryMultiResourceID,
				Name: wave2CarryMultiResourceID,
				Fields: map[string]string{
					"status": multiFindingBang.Phrase + " (+1)",
				},
				Findings: []domain.Finding{multiFindingBang, multiFindingTilde},
			},
		},
	})
	if err := store.SaveType("ecs-svc"); err != nil {
		t.Fatalf("seed fixture SaveType(ecs-svc): %v", err)
	}

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ecs-svc"})
	snap := ctrl.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ecs-svc list with a disk-store multi-finding seed")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — a populated on-disk per-type cache must seed the list immediately")
	}
	if len(lb.Rows) != 1 {
		t.Fatalf("Body.List.Rows has %d entries, want 1", len(lb.Rows))
	}
	row := lb.Rows[0]
	if row.ResourceID != wave2CarryMultiResourceID {
		t.Fatalf("Rows[0].ResourceID = %q, want %q", row.ResourceID, wave2CarryMultiResourceID)
	}
	if row.Decorator != app.DecoratorNormal {
		t.Errorf("Rows[0].Decorator = %q, want %q — colorECSSvc resolves ColorBroken directly from the worst carried finding, giving whole-row color (no glyph prefix), same precedent as the single-finding D17/C6b restart-seed pin", row.Decorator, app.DecoratorNormal)
	}
	if row.Severity != "issue" {
		t.Errorf(`Rows[0].Severity = %q, want "issue" — the FIRST render must already reflect the carried worst finding, no enrichment needed`, row.Severity)
	}
	if lb.StatusCol < 0 || lb.StatusCol >= len(row.Cells) {
		t.Fatalf("ecs-svc list has no resolvable Status column on the seeded row (StatusCol=%d, len(Cells)=%d)", lb.StatusCol, len(row.Cells))
	}
	want := multiFindingBang.Phrase + " (+1)"
	if got := row.Cells[lb.StatusCol]; got != want {
		t.Errorf("Rows[0] Status cell = %q, want %q — the restart-seeded row must show BOTH carried findings stacked, worst first, on the FIRST rendered frame (C6b multi-finding carry)", got, want)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Section 3 — opensearch concrete: both conditions must surface as their own
// findings, not one Finding plus a hidden condition crammed into a generic
// "Additional" DetailRow. Retires (see aws_opensearch_issue_enrichment_test.go)
// TestOpenSearch_Enrich_MultiBackground_TopWinsHiddenSurfacesAsRow, which
// pinned exactly the "! wins, ~ hides in a row" shape this test says must
// change.
// ────────────────────────────────────────────────────────────────────────────

// opensearchMultiUpdateForcedCode and opensearchMultiEncryptionOffCode mirror
// the unexported opensearchCodeUpdateForced / opensearchCodeEncryptionOff
// constants in internal/aws/opensearch_issue_enrichment.go.
const (
	opensearchMultiUpdateForcedCode  domain.FindingCode = "opensearch.update-forced"
	opensearchMultiEncryptionOffCode domain.FindingCode = "opensearch.encryption-off"
)

// opensearchUpdateForcedOperatorSentence and
// opensearchEncryptionOffOperatorSentence mirror the unexported
// opensearchUpdateForcedDetail / opensearchEncryptionOffDetail constants in
// internal/aws/opensearch_issue_enrichment.go — the S5 operator sentences
// that must stay reachable on each condition's OWN Finding.Detail even when
// both fire on the same domain.
const (
	opensearchUpdateForcedOperatorSentence  = "AWS will apply this update automatically once the scheduled date passes; upgrade on your own schedule before then to control the maintenance window."
	opensearchEncryptionOffOperatorSentence = "Data at rest is stored unencrypted. Enabling encryption at rest requires creating a new domain and migrating data — it cannot be turned on in place."
)

func TestOpenSearch_Enrich_MultiBackground_BothConditionsSurfaceAsOwnFindings(t *testing.T) {
	resources := []resource.Resource{
		{
			ID:   fixtures.MultiBackgroundDomain,
			Name: fixtures.MultiBackgroundDomain,
			Fields: map[string]string{
				"service_software_update_available": "true",
				"encryption_at_rest_enabled":        "false",
			},
		},
	}

	result, err := awsclient.EnrichOpenSearchDomains(t.Context(), nil, resources, nil)
	if err != nil {
		t.Fatalf("EnrichOpenSearchDomains error: %v", err)
	}

	id := fixtures.MultiBackgroundDomain
	findings := result.Findings[id]
	if len(findings) != 2 {
		t.Fatalf("result.Findings[%q] has %d entries, want 2 — each independently-evaluated Wave-2 condition (update-forced, encryption-off) must survive as its OWN Finding once IssueEnricherResult.Findings is map[string][]domain.Finding (#52); got %+v", id, len(findings), findings)
	}

	byCode := map[domain.FindingCode]domain.Finding{}
	for _, f := range findings {
		byCode[f.Code] = f
	}

	updateForced, ok := byCode[opensearchMultiUpdateForcedCode]
	if !ok {
		t.Fatalf("no Finding with Code %q in result.Findings[%q]; got %+v", opensearchMultiUpdateForcedCode, id, findings)
	}
	if updateForced.Phrase != "software update forced soon" {
		t.Errorf("update-forced Finding.Phrase = %q, want %q", updateForced.Phrase, "software update forced soon")
	}
	if updateForced.Severity != domain.SevBroken {
		t.Errorf("update-forced Finding.Severity = %v, want SevBroken", updateForced.Severity)
	}
	if updateForced.Detail != opensearchUpdateForcedOperatorSentence {
		t.Errorf("update-forced Finding.Detail = %q, want %q — its own S5 operator sentence must stay reachable on its own Finding", updateForced.Detail, opensearchUpdateForcedOperatorSentence)
	}

	encOff, ok := byCode[opensearchMultiEncryptionOffCode]
	if !ok {
		t.Fatalf("no Finding with Code %q in result.Findings[%q]; got %+v", opensearchMultiEncryptionOffCode, id, findings)
	}
	if encOff.Phrase != "encryption at rest off" {
		t.Errorf("encryption-off Finding.Phrase = %q, want %q", encOff.Phrase, "encryption at rest off")
	}
	if encOff.Severity != domain.SevWarn {
		t.Errorf("encryption-off Finding.Severity = %v, want SevWarn", encOff.Severity)
	}
	if encOff.Detail != opensearchEncryptionOffOperatorSentence {
		t.Errorf("encryption-off Finding.Detail = %q, want %q — its own S5 operator sentence must stay reachable as its own Finding.Detail, not a generic Additional row", encOff.Detail, opensearchEncryptionOffOperatorSentence)
	}

	for _, ad := range result.AttentionDetails[id] {
		for _, row := range ad.Rows {
			if row.Label == "Additional" {
				t.Errorf(`Rows contains {Label:%q, Value:%q} — a generic "Additional" row means the second condition was demoted into a row instead of surfacing as its own Finding (#52)`, row.Label, row.Value)
			}
		}
	}
}
