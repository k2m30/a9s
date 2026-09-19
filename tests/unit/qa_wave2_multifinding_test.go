// Multi-finding rows.
// IssueEnricherResult.Findings is map[string][]domain.Finding: an enricher may
// emit several independently evaluated findings per resource, and all of them
// fold onto domain.Resource.Findings. The worst severity drives
// td.ResolveColor (colorFromAnyFinding scans the whole r.Findings slice,
// order-independent), and the detail Attention block lists each finding as its
// own entry with its own Phrase and Detail (buildAttentionEntries emits one
// attentionEntry per issue-severity finding).
//
// The fold tests build a two-finding IssueEnricherResult entry and run it
// through runtime.ApplyWave2ToRow. The carry tests seed a row with two
// wave2-sourced findings directly: cache.Row.Findings and
// domain.Resource.Findings are plain slices that carryWave2/carryWave2ForRows/
// wave2FindingsOf (core/runtime/wave2_carry.go) copy whole, so a disk refresh
// and a restart seed keep both.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

const multiFindingResourceID = "ecs-svc-fold-multi"

// multiFindingBang and multiFindingTilde are two independent Wave-2 findings
// an ecs-svc enricher emits for the same resource.
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
// entry the way a two-finding ecs-svc enricher does — both findings held in
// a single []domain.Finding slice value under the shared resource-ID key,
// bang (SevBroken, the worse of the two) ordered first — then folds it
// through the REAL runtime.ApplyWave2ToRow.
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
	// capitalizeFirstDetail (core/app/detail_body.go), which upper-cases
	// the first letter for display ("deployment failed" -> "Deployment
	// failed"). The contract this pins is that BOTH findings' phrases survive
	// the fold and each surfaces as its own Attention entry — not the exact
	// casing of a display convention.
	joined := strings.ToLower(strings.Join(attentionText, " | "))
	if !strings.Contains(joined, multiFindingBang.Phrase) {
		t.Errorf("detail Attention block missing bang phrase %q — buildAttentionEntries (core/app/detail_body.go) builds one entry per issue-severity r.Findings element with its OWN Phrase/Detail; this fails only if ApplyWave2ToRow still drops a finding during the fold (#52); got Attention rows: %v", multiFindingBang.Phrase, attentionText)
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

	// "(+1)" is listPhraseFromFindings's render rule (core/app/list_columns.go):
	// "<top phrase> (+N)" for a row with more than one finding, top being the
	// first issue-severity entry. bang is first in the slice and the worse
	// severity, so both readings of "top" agree.
	want := multiFindingBang.Phrase + " (+1)"
	if cell != want {
		t.Errorf("Status cell = %q, want %q — the worst (SevBroken) finding must lead the stacked phrase once both findings survive the fold; this fails only if ApplyWave2ToRow still drops the tilde finding (#52), since listPhraseFromFindings itself needs no change", cell, want)
	}
}

// cache.Row.Findings and domain.Resource.Findings are plain []domain.Finding
// slices, so the carry tests seed the multi-finding row directly, without
// IssueEnricherResult or ApplyWave2ToRow.

const (
	wave2CarryMultiShortName  = "wave2carrymulti"
	wave2CarryMultiResourceID = "ecs-svc-carry-multi"
)

func TestReconcileTypeFile_Wave2Carry_MultiFindingRowSurvivesRefresh(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	// newSaveCacheRegressionCore bootstraps its Core against
	// saveRegProfile/saveRegRegion (runtime_savecache_regressions_test.go) —
	// the seed must land in the same pair the Core under test reads/writes.
	store := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
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
	// sweep-completion save produces.
	freshRows := []cache.Row{
		{ID: wave2CarryMultiResourceID, Name: wave2CarryMultiResourceID},
	}
	if err := c.SaveResourceListCache(c.Pair(), wave2CarryMultiShortName, freshRows, 1, true, 0, false, false); err != nil {
		t.Fatalf("SaveResourceListCache: %v", err)
	}

	reloaded := cache.LoadDirForTest(saveRegProfile, saveRegRegion)
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
	// Construction touches no disk; EnsureCacheStore's first call, below, loads
	// the cache after the helper's t.Setenv("A9S_CONFIG_FOLDER", t.TempDir()), so
	// seeding after construction and before ctrl.Apply's cache read is equivalent
	// to seeding before it.
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
	// Whole-row colour follows the worst carried finding.
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

// opensearchMultiUpdateForcedCode and opensearchMultiEncryptionOffCode mirror
// the unexported opensearchCodeUpdateForced / opensearchCodeEncryptionOff
// constants in core/aws/opensearch_issue_enrichment.go.
const (
	opensearchMultiUpdateForcedCode  domain.FindingCode = "opensearch.update-forced"
	opensearchMultiEncryptionOffCode domain.FindingCode = "opensearch.encryption-off"
)
