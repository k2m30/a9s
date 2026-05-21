package unit

// catalog_mutator_test.go — AS-726 PR-04i — unit tests for the eight new
// catalog mutators added to internal/catalog/catalog.go.
//
// Contract under test (per docs/refactor/AS-726-pr04i-messaging.md §2):
//   - RegisterFetcher / RegisterWave2 / RegisterRelated / RegisterNavigable /
//     RegisterFieldKeys / RegisterChildView — panic on duplicate non-nil register.
//   - RegisterIssueEnricherFieldKeys — idempotent; dedups across calls.
//   - FindChild — returns nil for unregistered, non-nil + populated for registered.
//
// Catalog state for the top-level Register* mutators is keyed by ShortName and
// looks up an existing row in catalog.ResourceTypes. To avoid polluting real
// resource rows the tests use synthetic shortNames added via test helpers the
// Coder must expose alongside the mutators:
//
//   func catalog.AddForTest(def ResourceTypeDef)       // appends to ResourceTypes
//   func catalog.RemoveForTest(shortName string)       // removes by ShortName
//
// These are the minimal scaffolding needed to test mutator semantics in
// isolation — they are NOT consumed by production code. See spec §2 hint:
// "Catalog state is package-level; if you need a reset, add catalog.ResetForTest()
// and ask the Coder to expose it."

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
)

// noopFetcher is a minimal PaginatedFetcher used as the mutator's argument
// where the value's content is irrelevant — only its presence/absence.
func noopFetcher(_ context.Context, _ any, _ string) (domain.FetchResult, error) {
	return domain.FetchResult{}, nil
}

func noopChildFetcher(_ context.Context, _ any, _ domain.ParentContext, _ string) (domain.FetchResult, error) {
	return domain.FetchResult{}, nil
}

// expectPanic runs fn and reports a test failure if it does NOT panic.
func expectPanic(t *testing.T, label string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got nil", label)
		}
	}()
	fn()
}

// ---------------------------------------------------------------------------
// RegisterFetcher
// ---------------------------------------------------------------------------

func TestRegisterFetcher(t *testing.T) {
	t.Run("happy_path_register_then_find", func(t *testing.T) {
		sn := "_qa_test_fetcher_happy"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterFetcher(sn, noopFetcher)

		ct := catalog.Find(sn)
		if ct == nil {
			t.Fatalf("catalog.Find(%q) returned nil after RegisterFetcher", sn)
		}
		if ct.Fetcher == nil {
			t.Errorf("RegisterFetcher did not set Fetcher on the catalog row")
		}
	})

	t.Run("panic_on_duplicate", func(t *testing.T) {
		sn := "_qa_test_fetcher_dup"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterFetcher(sn, noopFetcher)
		expectPanic(t, "RegisterFetcher second call on populated row", func() {
			catalog.RegisterFetcher(sn, noopFetcher)
		})
	})
}

// ---------------------------------------------------------------------------
// RegisterWave2
// ---------------------------------------------------------------------------

func TestRegisterWave2(t *testing.T) {
	t.Run("happy_path_register_then_find", func(t *testing.T) {
		sn := "_qa_test_wave2_happy"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		marker := struct{ Tag string }{Tag: "marker"}
		catalog.RegisterWave2(sn, marker)

		ct := catalog.Find(sn)
		if ct == nil {
			t.Fatalf("catalog.Find(%q) returned nil after RegisterWave2", sn)
		}
		if ct.Wave2 == nil {
			t.Errorf("RegisterWave2 did not set Wave2 on the catalog row")
		}
	})

	t.Run("panic_on_duplicate", func(t *testing.T) {
		sn := "_qa_test_wave2_dup"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterWave2(sn, struct{ Tag string }{Tag: "first"})
		expectPanic(t, "RegisterWave2 second call on populated row", func() {
			catalog.RegisterWave2(sn, struct{ Tag string }{Tag: "second"})
		})
	})
}

// ---------------------------------------------------------------------------
// RegisterRelated
// ---------------------------------------------------------------------------

func TestRegisterRelated(t *testing.T) {
	t.Run("happy_path_register_then_find", func(t *testing.T) {
		sn := "_qa_test_related_happy"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		defs := []domain.RelatedDef{{TargetType: "_qa_target", DisplayName: "QA Target"}}
		catalog.RegisterRelated(sn, defs)

		ct := catalog.Find(sn)
		if ct == nil {
			t.Fatalf("catalog.Find(%q) returned nil after RegisterRelated", sn)
		}
		if len(ct.Related) != 1 || ct.Related[0].TargetType != "_qa_target" {
			t.Errorf("RegisterRelated did not set Related on the catalog row; got %+v", ct.Related)
		}
	})

	t.Run("panic_on_duplicate", func(t *testing.T) {
		sn := "_qa_test_related_dup"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		first := []domain.RelatedDef{{TargetType: "_qa_target_a"}}
		second := []domain.RelatedDef{{TargetType: "_qa_target_b"}}
		catalog.RegisterRelated(sn, first)
		expectPanic(t, "RegisterRelated second call on populated row", func() {
			catalog.RegisterRelated(sn, second)
		})
	})
}

// ---------------------------------------------------------------------------
// RegisterNavigable
// ---------------------------------------------------------------------------

func TestRegisterNavigable(t *testing.T) {
	t.Run("happy_path_register_then_find", func(t *testing.T) {
		sn := "_qa_test_nav_happy"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		fields := []domain.NavigableField{{FieldPath: "RoleArn", TargetType: "role"}}
		catalog.RegisterNavigable(sn, fields)

		ct := catalog.Find(sn)
		if ct == nil {
			t.Fatalf("catalog.Find(%q) returned nil after RegisterNavigable", sn)
		}
		if len(ct.Navigable) != 1 || ct.Navigable[0].FieldPath != "RoleArn" {
			t.Errorf("RegisterNavigable did not set Navigable on the catalog row; got %+v", ct.Navigable)
		}
	})

	t.Run("panic_on_duplicate", func(t *testing.T) {
		sn := "_qa_test_nav_dup"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterNavigable(sn, []domain.NavigableField{{FieldPath: "A", TargetType: "x"}})
		expectPanic(t, "RegisterNavigable second call on populated row", func() {
			catalog.RegisterNavigable(sn, []domain.NavigableField{{FieldPath: "B", TargetType: "y"}})
		})
	})
}

// ---------------------------------------------------------------------------
// RegisterFieldKeys
// ---------------------------------------------------------------------------

func TestRegisterFieldKeys(t *testing.T) {
	t.Run("happy_path_register_then_find", func(t *testing.T) {
		sn := "_qa_test_fk_happy"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterFieldKeys(sn, []string{"alpha", "beta"})

		ct := catalog.Find(sn)
		if ct == nil {
			t.Fatalf("catalog.Find(%q) returned nil after RegisterFieldKeys", sn)
		}
		if len(ct.FieldKeys) != 2 || ct.FieldKeys[0] != "alpha" || ct.FieldKeys[1] != "beta" {
			t.Errorf("RegisterFieldKeys did not set FieldKeys correctly; got %v", ct.FieldKeys)
		}
	})

	t.Run("panic_on_duplicate", func(t *testing.T) {
		sn := "_qa_test_fk_dup"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterFieldKeys(sn, []string{"alpha"})
		expectPanic(t, "RegisterFieldKeys second call on populated row", func() {
			catalog.RegisterFieldKeys(sn, []string{"beta"})
		})
	})
}

// ---------------------------------------------------------------------------
// RegisterIssueEnricherFieldKeys — IDEMPOTENT (dedups)
// ---------------------------------------------------------------------------

func TestRegisterIssueEnricherFieldKeys(t *testing.T) {
	t.Run("happy_path_register_then_find", func(t *testing.T) {
		sn := "_qa_test_iefk_happy"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterIssueEnricherFieldKeys(sn, []string{"dlq"})

		ct := catalog.Find(sn)
		if ct == nil {
			t.Fatalf("catalog.Find(%q) returned nil after RegisterIssueEnricherFieldKeys", sn)
		}
		if len(ct.IssueEnricherFieldKeys) != 1 || ct.IssueEnricherFieldKeys[0] != "dlq" {
			t.Errorf("RegisterIssueEnricherFieldKeys did not set keys; got %v", ct.IssueEnricherFieldKeys)
		}
	})

	t.Run("dedups_across_calls", func(t *testing.T) {
		sn := "_qa_test_iefk_dedup"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterIssueEnricherFieldKeys(sn, []string{"alpha", "beta"})
		catalog.RegisterIssueEnricherFieldKeys(sn, []string{"beta", "gamma"})

		ct := catalog.Find(sn)
		if ct == nil {
			t.Fatalf("catalog.Find(%q) returned nil", sn)
		}
		got := ct.IssueEnricherFieldKeys
		if len(got) != 3 {
			t.Fatalf("expected union of 3 keys after dedup, got %d: %v", len(got), got)
		}
		// Order: original order preserved, new (non-dup) keys appended.
		want := []string{"alpha", "beta", "gamma"}
		for i, k := range want {
			if got[i] != k {
				t.Errorf("IssueEnricherFieldKeys[%d] = %q, want %q (full: %v)", i, got[i], k, got)
			}
		}
	})

	t.Run("idempotent_second_call_same_keys", func(t *testing.T) {
		sn := "_qa_test_iefk_idem"
		catalog.AddForTest(catalog.ResourceTypeDef{ShortName: sn})
		defer catalog.RemoveForTest(sn)

		catalog.RegisterIssueEnricherFieldKeys(sn, []string{"x"})
		catalog.RegisterIssueEnricherFieldKeys(sn, []string{"x"})

		ct := catalog.Find(sn)
		if ct == nil {
			t.Fatalf("catalog.Find(%q) returned nil", sn)
		}
		if len(ct.IssueEnricherFieldKeys) != 1 {
			t.Errorf("idempotent register should yield 1 key, got %v", ct.IssueEnricherFieldKeys)
		}
	})
}

// ---------------------------------------------------------------------------
// RegisterChildView / FindChild
// ---------------------------------------------------------------------------

func TestRegisterChildView(t *testing.T) {
	t.Run("happy_path_register_then_find", func(t *testing.T) {
		sn := "_qa_test_childview_happy"
		defer catalog.RemoveForTest(sn)

		catalog.RegisterChildView(catalog.ResourceTypeDef{
			Name:         "QA Child Happy",
			ShortName:    sn,
			ChildFetcher: noopChildFetcher,
			Columns:      []domain.Column{{Key: "id", Title: "ID"}},
			CopyField:    "id",
		})

		got := catalog.FindChild(sn)
		if got == nil {
			t.Fatalf("FindChild(%q) returned nil after RegisterChildView", sn)
		}
		if got.ShortName != sn {
			t.Errorf("FindChild ShortName = %q, want %q", got.ShortName, sn)
		}
		if got.ChildFetcher == nil {
			t.Errorf("FindChild returned entry with nil ChildFetcher")
		}
		if len(got.Columns) == 0 {
			t.Errorf("FindChild returned entry with empty Columns")
		}
		if got.CopyField != "id" {
			t.Errorf("FindChild CopyField = %q, want %q", got.CopyField, "id")
		}
	})

	t.Run("panic_on_duplicate", func(t *testing.T) {
		sn := "_qa_test_childview_dup"
		defer catalog.RemoveForTest(sn)
		catalog.RegisterChildView(catalog.ResourceTypeDef{ShortName: sn, ChildFetcher: noopChildFetcher})
		expectPanic(t, "RegisterChildView second call with same shortName", func() {
			catalog.RegisterChildView(catalog.ResourceTypeDef{ShortName: sn, ChildFetcher: noopChildFetcher})
		})
	})
}

func TestFindChild_ReturnsNilForUnregistered(t *testing.T) {
	got := catalog.FindChild("_qa_test_unregistered_child_xyz")
	if got != nil {
		t.Errorf("FindChild on unregistered shortName returned %+v, want nil", got)
	}
}
