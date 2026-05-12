package views

// widen_lifecycle_column_internal_test.go — AS-140 / AS-566: pin that
// widenLifecycleColumn sizes the lifecycle/status column using the same
// AS-140 two-layer priority as extractCellValue:
//
//	1. phraseFromFindings(r.Findings)  — composes "<top> (+N)" for stacked
//	   findings (Wave-1 + Wave-2).
//	2. r.Fields[lifecycleKey]          — steady-state fallback.
//
// Regression: prior to this fix, widenLifecycleColumn measured only
// r.Findings[0].Phrase ("stopped", 7 chars) for a stacked row, while
// extractCellValue rendered the merged "stopped (+1)" (12 chars), causing
// the column to truncate the displayed status.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestWidenLifecycleColumn_StackedFindingsSizedToMergedPhrase(t *testing.T) {
	m := ResourceListModel{typeDef: catalog.ResourceTypeDef{LifecycleKey: "state"}}

	cols := []listCol{
		{title: "ID", key: "@id", width: 20},
		{title: "Status", key: "status", width: 8}, // narrow on purpose: "stopped" fits, "stopped (+1)" does not
	}
	rows := []resource.Resource{
		{
			ID: "i-stacked",
			Findings: []domain.Finding{
				{Phrase: "stopped"},
				{Phrase: "maintenance scheduled"},
			},
		},
	}

	out := m.widenLifecycleColumn(cols, rows)

	wantPhrase := "stopped (+1)" // 12 visible chars
	wantWidth := len(wantPhrase)
	if got := out[1].width; got != wantWidth {
		t.Fatalf("widenLifecycleColumn stacked width = %d, want %d (room for %q)", got, wantWidth, wantPhrase)
	}
	if out[0].width != 20 {
		t.Errorf("non-status column width perturbed: got %d, want 20", out[0].width)
	}
}

func TestWidenLifecycleColumn_SingleFindingSizedToBarePhrase(t *testing.T) {
	m := ResourceListModel{typeDef: catalog.ResourceTypeDef{LifecycleKey: "state"}}

	cols := []listCol{
		{title: "Status", key: "status", width: 4},
	}
	rows := []resource.Resource{
		{
			ID:       "i-single",
			Findings: []domain.Finding{{Phrase: "maintenance scheduled"}},
		},
	}

	out := m.widenLifecycleColumn(cols, rows)

	if got, want := out[0].width, len("maintenance scheduled"); got != want {
		t.Fatalf("widenLifecycleColumn single-finding width = %d, want %d", got, want)
	}
}

func TestWidenLifecycleColumn_NoFindingsFallsBackToLifecycleField(t *testing.T) {
	m := ResourceListModel{typeDef: catalog.ResourceTypeDef{LifecycleKey: "state"}}

	cols := []listCol{
		{title: "State", key: "state", width: 4},
	}
	rows := []resource.Resource{
		{ID: "i-healthy", Fields: map[string]string{"state": "available"}},
	}

	out := m.widenLifecycleColumn(cols, rows)

	if got, want := out[0].width, len("available"); got != want {
		t.Fatalf("widenLifecycleColumn lifecycle-fallback width = %d, want %d", got, want)
	}
}

func TestWidenLifecycleColumn_IgnoresLegacyFieldsStatus(t *testing.T) {
	// AS-140: r.Fields["status"] is no longer authoritative — Wave-2 enrichers
	// stopped writing it. widenLifecycleColumn must NOT widen the column to
	// accommodate stale Fields["status"] content when r.Findings is empty and
	// no lifecycle field is set; the column stays at its declared width.
	m := ResourceListModel{typeDef: catalog.ResourceTypeDef{LifecycleKey: "state"}}

	cols := []listCol{
		{title: "Status", key: "status", width: 6},
	}
	rows := []resource.Resource{
		{
			ID:     "i-stale",
			Fields: map[string]string{"status": "stopped (+1)"}, // legacy overlay — ignored
		},
	}

	out := m.widenLifecycleColumn(cols, rows)

	if got, want := out[0].width, 6; got != want {
		t.Fatalf("widenLifecycleColumn must ignore legacy Fields[\"status\"]: got width %d, want %d", got, want)
	}
}
