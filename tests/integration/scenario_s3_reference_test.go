//go:build integration

package integration

// scenario_s3_reference_test.go — standing per-surface gate for s3, the
// project's reference resource. Each subtest names one user-facing surface,
// so a failure identifies the broken surface directly. Subtests share one
// scenario walk and run in order; deep per-fixture render assertions live in
// scenario_s3_visual_test.go, this file proves every surface works end to
// end in demo mode.

import (
	"strconv"
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// s3SortProbeBucket is a namedBuckets fixture from core/demo/fixtures/s3.go,
// lexicographically after the a9s-demo-* buckets so a Bucket Name sort toggle
// provably reorders it relative to the healthy graph-root bucket.
const s3SortProbeBucket = "webapp-assets-prod"

func TestScenario_S3ReferenceSurfaces(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	expectedS3 := demofixtures.ExpectedTopLevelCounts()["s3"]

	t.Run("main_view", func(t *testing.T) {
		scenario.ExpectViewContains("S3 Buckets (" + strconv.Itoa(expectedS3) + ")")
		scenario.ExpectMenuIssueCount("s3", s3ExpectedIssueBkt)
	})

	t.Run("list_view", func(t *testing.T) {
		scenario.Command("s3")
		scenario.ExpectCurrentListType("s3")
		scenario.ExpectFrameContains("s3(" + strconv.Itoa(expectedS3))
		scenario.ExpectViewContains(demofixtures.HealthyBucketName)

		headerLine := ""
		for _, line := range strings.Split(scenario.currentView(), "\n") {
			if strings.Contains(line, "Bucket Name") {
				headerLine = line
				break
			}
		}
		if headerLine == "" {
			t.Fatalf("s3 list header line with %q not found (columns configured in .a9s/views/s3.yaml)\nview:\n%s",
				"Bucket Name", scenario.currentView())
		}
		for _, col := range []string{"Region", "Creation Date", "Status"} {
			if !strings.Contains(headerLine, col) {
				t.Fatalf("s3 list header missing column %q from .a9s/views/s3.yaml: %q", col, headerLine)
			}
		}
	})

	t.Run("list_filter_sort", func(t *testing.T) {
		// Filter target is deliberately NOT the healthy graph-root bucket:
		// opening its detail here would populate relatedCache, and the
		// detail_related re-entry would then never re-emit
		// RelatedCheckResultMsg for it.
		scenario.ApplyFilter(s3SortProbeBucket)
		scenario.ExpectFrameContains("s3(1/")
		scenario.OpenSelectedDetail()
		scenario.ExpectCurrentResourceID(s3SortProbeBucket)
		scenario.Back()
		// Esc on a list with an active filter clears the filter instead of
		// popping the view (per-screen esc semantics, core/app/actions_nav.go).
		scenario.Back()
		scenario.ExpectFrameContains("s3(" + strconv.Itoa(expectedS3))

		// Column 1 = "Bucket Name" per .a9s/views/s3.yaml; the first apply
		// sorts ascending, the second flips to descending.
		scenario.SortByColumn(1)
		expectRenderedOrder(t, scenario, demofixtures.HealthyBucketName, s3SortProbeBucket)
		scenario.SortByColumn(1)
		expectRenderedOrder(t, scenario, s3SortProbeBucket, demofixtures.HealthyBucketName)
	})

	t.Run("issues", func(t *testing.T) {
		scenario.ExpectRowStatusEquals(s3NoPABBucketID, s3S4Phrase)
		// colorS3 resolves color via colorFromAnyFinding (catalog_databases.go);
		// the PAB finding is Severity: SevBroken, so this row renders Broken
		// row color directly instead of staying Healthy-with-`!`-glyph.
		scenario.ExpectRowNoGlyphPrefix(s3NoPABBucketID)
	})

	t.Run("detail_related", func(t *testing.T) {
		root := selectS3ByID(t, scenario, demofixtures.HealthyBucketName)
		scenario.OpenDetailResource("s3", root)
		scenario.ExpectNoAPIError()
		scenario.ExpectRelatedRowCountAtLeast("CloudTrail Trails", 1)
		scenario.ExpectRelatedRowCountAtLeast("KMS Key", 1)
		scenario.ExpectRelatedRow("Lambda (notifications)")
		scenario.ExpectRelatedRow("CloudFormation")
	})

	t.Run("related_drill", func(t *testing.T) {
		// Fresh scenario: re-entering a detail view hits relatedCache and never
		// re-emits RelatedCheckResultMsg (same pattern as
		// related_view_validation_test.go).
		fresh := fullIntegrationNewDemoScenario(t)
		root := selectS3ByID(t, fresh, demofixtures.HealthyBucketName)
		fresh.OpenDetailResource("s3", root)
		fresh.ExpectNoAPIError()
		rows := fresh.DrillRelated("CloudTrail Trails")
		if len(rows) == 0 {
			t.Fatalf("drill into %q from %q landed on an empty list (graph fixtures: core/demo/fixtures/s3.go + cloudtrail fixtures)",
				"CloudTrail Trails", demofixtures.HealthyBucketName)
		}
		fresh.ExpectNoAPIError()
	})

	t.Run("yaml_view", func(t *testing.T) {
		scenario.Back() // detail → s3 list
		scenario.ApplyFilter(demofixtures.HealthyBucketName)
		scenario.OpenYAML()
		scenario.ExpectViewContains(demofixtures.HealthyBucketARN)
		scenario.ApplySearch(demofixtures.HealthyBucketARN)
		scenario.ExpectHeaderContains("matches")
		// Esc first clears the active text search, only the second pops the
		// YAML view (per-view esc semantics, core/app/actions_nav.go).
		scenario.Back()
		scenario.Back() // yaml → filtered list
	})

	t.Run("json_view", func(t *testing.T) {
		scenario.Press("J")
		scenario.ExpectFrameContains("json")
		// JSON quotes string values; the YAML rendering of the same field does
		// not, so the quoted ARN proves json-shaped output.
		scenario.ExpectViewContains("\"" + demofixtures.HealthyBucketARN + "\"")
		scenario.Back() // json → filtered list
		scenario.Back() // clear filter
	})

	t.Run("cache_roundtrip", func(t *testing.T) {
		// Core().ResourceCache is gated to FULL, fetch-origin entries
		// (core/runtime/accessors.go), so ok==true is itself the
		// "full fetch-origin entry exists" assertion.
		entry, ok := scenario.model.Core().ResourceCache("s3")
		if !ok || entry == nil {
			t.Fatalf("no full fetch-origin cache entry for s3 after list load (core/runtime/accessors.go ResourceCache)")
		}
		if got := len(entry.Resources); got != expectedS3 {
			t.Fatalf("s3 cache entry rows = %d, expected %d per core/demo/fixtures/counts.go ExpectedTopLevelCounts",
				got, expectedS3)
		}
		scenario.Back() // list → menu
		scenario.OpenList("s3")
		scenario.ExpectFrameContains("s3(" + strconv.Itoa(expectedS3))
	})

	t.Run("web_ui", func(t *testing.T) {
		t.Skip("covered by tests/integration/web: TestWebAllResourceTypes_List_NavigatesAndShowsColumns parameterizes s3 list/detail/children")
	})
}

// expectRenderedOrder asserts that the row for first renders above the row
// for second in the current view. Both names are fixture buckets from
// core/demo/fixtures/s3.go.
func expectRenderedOrder(t *testing.T, s *fullIntegrationScenario, first, second string) {
	t.Helper()
	view := s.currentView()
	fi := strings.Index(view, first)
	si := strings.Index(view, second)
	if fi < 0 || si < 0 {
		t.Fatalf("sort check: bucket rows %q / %q not both rendered (core/demo/fixtures/s3.go)\nview:\n%s",
			first, second, view)
	}
	if fi > si {
		t.Fatalf("sort check: expected %q above %q after Bucket Name sort toggle\nview:\n%s",
			first, second, view)
	}
}
