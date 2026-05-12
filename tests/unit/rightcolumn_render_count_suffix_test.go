package unit_test

// rightcolumn_render_count_suffix_test.go — AS-378 acceptance #2 regression
// pin for the related-panel renderer "(N)" suffix contract.
//
// Failure mode that motivated this file (AS-377 / AS-378):
//   `TestLiveFullIntegration_AllResourcesBaseline` greps the rendered
//   RELATED panel for the literal substring `"<Pivot> (<N>)"` for every
//   defined pivot whose checker returned `Count >= 0`. Pre-AS-378 the
//   renderer emitted `(0+)` / `(N+)` for `Approximate==true` rows, so the
//   integration assertion failed for ~6 pivots across 6 unrelated primaries
//   (`lambda`, `dbi`, `dbc`, `s3`, `ddb`, `ecr`).
//
// Contract pinned here (post-AS-378):
//   actual = -1, FetchFilter present       → "DisplayName"            (no parens, navigable)
//   actual = -1, FetchFilter absent        → "DisplayName"            (no parens, dim)
//   actual = 0,  approximate = false       → "DisplayName (0)"        (dim, confirmed zero)
//   actual = 0,  approximate = true        → "DisplayName (0)"        (normal, lower bound)
//   actual = N>0, approximate = false      → "DisplayName (N)"        (normal)
//   actual = N>0, approximate = true       → "DisplayName (N)"        (normal, lower bound)
//
// Acceptance criterion #2 from AS-378 explicitly enumerates the affected
// pivots (CT Events / CW Alarms / Glue Jobs / Network Interfaces / CT
// Trails); the named cases below exercise each one against the same
// parametrized renderer path.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// buildSuffixDetail registers a single RelatedDef for resource type
// "suffix-test" using the supplied displayName and target type, and returns
// a DetailModel sized so the right column is auto-shown. Each test case
// gets its own (name, target) pair so concurrent test execution does not
// stomp the shared registry.
func buildSuffixDetail(t *testing.T, displayName, targetType string) (views.DetailModel, func()) {
	t.Helper()
	resource.RegisterRelated("suffix-test", []resource.RelatedDef{
		{TargetType: targetType, DisplayName: displayName, Checker: noopChecker},
	})
	cleanup := func() { resource.UnregisterRelated("suffix-test") }

	res := resource.Resource{
		ID:   "suffix-test-id",
		Name: "suffix-test-name",
		Fields: map[string]string{
			"id": "suffix-test-id",
		},
	}
	d := views.NewDetail(res, "suffix-test", nil, keys.Default())
	d.SetSize(140, 30)
	return d, cleanup
}

// injectSuffixResult delivers a RelatedCheckResultMsg for the named target
// and waits for the model to apply it.
func injectSuffixResult(d views.DetailModel, targetType string, count int, approximate bool, fetchFilter map[string]string) views.DetailModel {
	msg := messages.RelatedCheckResult{
		ResourceType: "suffix-test",
		Result: resource.RelatedCheckResult{
			TargetType:  targetType,
			Count:       count,
			Approximate: approximate,
			FetchFilter: fetchFilter,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

// TestRightColumn_RenderCountSuffix_Matrix is the parametrized acceptance
// table for the AS-378 renderer contract. Every row asserts both the
// expected presence and the absence of any "+" marker that would break the
// integration test's literal-substring contract.
func TestRightColumn_RenderCountSuffix_Matrix(t *testing.T) {
	ensureNoColor(t)

	type renderCase struct {
		name        string
		displayName string
		targetType  string
		count       int
		approximate bool
		fetchFilter map[string]string

		// wantContains is a literal substring the rendered panel MUST
		// contain (typically `"<Display> (<N>)"` or just `<Display>`).
		wantContains string
		// mustNotContain are literal substrings the renderer MUST NOT
		// emit for this case (defends against accidental reintroduction
		// of the "+" marker).
		mustNotContain []string
	}

	// AS-378 acceptance #2 explicitly names CT Events / CW Alarms / Glue
	// Jobs / Network Interfaces / CT Trails as the pivots that broke at
	// Stage 6.5. Each row below exercises one of those display labels
	// against one of the three `actual` axes (-1, 0, >0) with the
	// approximate flag flipped where relevant.
	cases := []renderCase{
		// CloudTrail Events — actual=-1 with FetchFilter, the production
		// path for any resource whose CloudTrailKey resolves. Renderer
		// emits the bare displayName (no suffix); test contract requires
		// just the displayName, not "(?)" or "(N)".
		{
			name:           "CT_Events_minus_one_with_filter",
			displayName:    "CloudTrail Events",
			targetType:     "ct-events",
			count:          -1,
			approximate:    false,
			fetchFilter:    map[string]string{"ResourceName": "my-resource"},
			wantContains:   "CloudTrail Events",
			mustNotContain: []string{"CloudTrail Events (0)", "CloudTrail Events (?)"},
		},
		// CloudWatch Alarms — actual=0 with approximate=true (truncated
		// cache + 0 matches). This is the AS-378 root-cause path:
		// pre-fix the renderer emitted "(0+)" and the integration test
		// failed.
		{
			name:           "CW_Alarms_zero_approximate",
			displayName:    "CloudWatch Alarms",
			targetType:     "alarm",
			count:          0,
			approximate:    true,
			fetchFilter:    nil,
			wantContains:   "CloudWatch Alarms (0)",
			mustNotContain: []string{"CloudWatch Alarms (0+)"},
		},
		// CloudWatch Alarms — actual=0 with approximate=false (cache
		// fully scanned, confirmed zero). Renderer must emit "(0)".
		{
			name:           "CW_Alarms_zero_confirmed",
			displayName:    "CloudWatch Alarms",
			targetType:     "alarm",
			count:          0,
			approximate:    false,
			fetchFilter:    nil,
			wantContains:   "CloudWatch Alarms (0)",
			mustNotContain: []string{"CloudWatch Alarms (0+)"},
		},
		// CloudWatch Alarms — actual>0 with approximate=false. Renderer
		// must emit "(N)".
		{
			name:           "CW_Alarms_positive_exact",
			displayName:    "CloudWatch Alarms",
			targetType:     "alarm",
			count:          3,
			approximate:    false,
			fetchFilter:    nil,
			wantContains:   "CloudWatch Alarms (3)",
			mustNotContain: []string{"CloudWatch Alarms (3+)"},
		},
		// CloudWatch Alarms — actual>0 with approximate=true (truncated
		// cache lower-bound). Renderer must emit "(N)" without the "+"
		// marker post-AS-378.
		{
			name:           "CW_Alarms_positive_approximate",
			displayName:    "CloudWatch Alarms",
			targetType:     "alarm",
			count:          7,
			approximate:    true,
			fetchFilter:    nil,
			wantContains:   "CloudWatch Alarms (7)",
			mustNotContain: []string{"CloudWatch Alarms (7+)"},
		},
		// Glue Jobs — actual=0 confirmed (s3 / ecr pivot). Confirmed zero
		// renders "(0)" dim.
		{
			name:           "Glue_Jobs_zero_confirmed",
			displayName:    "Glue Jobs",
			targetType:     "glue-job",
			count:          0,
			approximate:    false,
			fetchFilter:    nil,
			wantContains:   "Glue Jobs (0)",
			mustNotContain: []string{"Glue Jobs (0+)"},
		},
		// Glue Jobs — actual=0 approximate (lower bound).
		{
			name:           "Glue_Jobs_zero_approximate",
			displayName:    "Glue Jobs",
			targetType:     "glue-job",
			count:          0,
			approximate:    true,
			fetchFilter:    nil,
			wantContains:   "Glue Jobs (0)",
			mustNotContain: []string{"Glue Jobs (0+)"},
		},
		// Network Interfaces — lambda's ENI pivot with actual=-1 (cache
		// miss, no fetch fallback). Renderer emits the bare displayName.
		{
			name:           "Network_Interfaces_minus_one",
			displayName:    "Network Interfaces",
			targetType:     "eni",
			count:          -1,
			approximate:    false,
			fetchFilter:    nil,
			wantContains:   "Network Interfaces",
			mustNotContain: []string{"Network Interfaces (0)", "Network Interfaces (0+)"},
		},
		// Network Interfaces — actual=0 approximate (truncated ENI
		// cache, no matches for this Lambda's hyperplane).
		{
			name:           "Network_Interfaces_zero_approximate",
			displayName:    "Network Interfaces",
			targetType:     "eni",
			count:          0,
			approximate:    true,
			fetchFilter:    nil,
			wantContains:   "Network Interfaces (0)",
			mustNotContain: []string{"Network Interfaces (0+)"},
		},
		// CloudTrail Trails — s3 pivot, actual=0 confirmed.
		{
			name:           "CT_Trails_zero_confirmed",
			displayName:    "CloudTrail Trails",
			targetType:     "trail",
			count:          0,
			approximate:    false,
			fetchFilter:    nil,
			wantContains:   "CloudTrail Trails (0)",
			mustNotContain: []string{"CloudTrail Trails (0+)"},
		},
		// CloudTrail Trails — actual=2 exact.
		{
			name:           "CT_Trails_positive_exact",
			displayName:    "CloudTrail Trails",
			targetType:     "trail",
			count:          2,
			approximate:    false,
			fetchFilter:    nil,
			wantContains:   "CloudTrail Trails (2)",
			mustNotContain: []string{"CloudTrail Trails (2+)"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d, cleanup := buildSuffixDetail(t, tc.displayName, tc.targetType)
			defer cleanup()

			if !strings.Contains(stripAnsi(d.View()), "RELATED") {
				t.Skip("right column not visible at width=140; cannot exercise suffix rendering")
			}

			d = injectSuffixResult(d, tc.targetType, tc.count, tc.approximate, tc.fetchFilter)
			plain := stripAnsi(d.View())

			if !strings.Contains(plain, tc.wantContains) {
				t.Errorf("rendered RELATED panel missing %q (count=%d approximate=%v); got:\n%s",
					tc.wantContains, tc.count, tc.approximate, plain)
			}
			for _, banned := range tc.mustNotContain {
				if strings.Contains(plain, banned) {
					t.Errorf("rendered RELATED panel must not contain %q (count=%d approximate=%v); got:\n%s",
						banned, tc.count, tc.approximate, plain)
				}
			}
		})
	}
}
