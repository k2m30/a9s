package views

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestCapTierToRowBucket pins the universal Attention-entry color-cap rule:
// a `!` severity tier renders as `!` (red) ONLY when the row's S2 color bucket
// is Broken. On any other row bucket (Healthy, Warning, Dim) the color caps to
// `~` (yellow) so the detail view never contradicts the list row.
//
// The glyph on the rendered line is constructed separately from the severity
// tier (see injectAttentionSection); this helper is the color seam only.
func TestCapTierToRowBucket(t *testing.T) {
	cases := []struct {
		name   string
		tier   string
		bucket resource.Color
		want   string
	}{
		// !-tier entries — capped unless row is Broken.
		{"!_on_Healthy_caps_to_~", "!", resource.ColorHealthy, "~"},
		{"!_on_Warning_caps_to_~", "!", resource.ColorWarning, "~"},
		{"!_on_Dim_caps_to_~", "!", resource.ColorDim, "~"},
		{"!_on_Broken_stays_!", "!", resource.ColorBroken, "!"},

		// ~-tier entries — always pass through unchanged.
		{"~_on_Healthy_stays_~", "~", resource.ColorHealthy, "~"},
		{"~_on_Warning_stays_~", "~", resource.ColorWarning, "~"},
		{"~_on_Dim_stays_~", "~", resource.ColorDim, "~"},
		{"~_on_Broken_stays_~", "~", resource.ColorBroken, "~"},

		// Other tiers used by the detail renderer (EC2 status checks, ct-events,
		// plain informational) are not capped — only the ! → ~ severity downgrade
		// is in scope for this rule. These pass through unchanged on every bucket.
		{"ok_on_Healthy_stays_ok", "ok", resource.ColorHealthy, "ok"},
		{"ok_on_Broken_stays_ok", "ok", resource.ColorBroken, "ok"},
		{"impaired_on_Broken_stays_impaired", "impaired", resource.ColorBroken, "impaired"},
		{"ct-danger_on_Healthy_stays_ct-danger", "ct-danger", resource.ColorHealthy, "ct-danger"},
		{"initializing_on_Warning_stays_initializing", "initializing", resource.ColorWarning, "initializing"},

		// Empty tier passes through — renders neutral via TierColorStyle's default.
		{"empty_on_Healthy_stays_empty", "", resource.ColorHealthy, ""},
		{"empty_on_Broken_stays_empty", "", resource.ColorBroken, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := capTierToRowBucket(tc.tier, tc.bucket)
			if got != tc.want {
				t.Errorf("capTierToRowBucket(%q, %v) = %q, want %q", tc.tier, tc.bucket, got, tc.want)
			}
		})
	}
}

// TestResolveRowColorBucket_UnknownTypeFallsBackToHealthy pins the safe default
// for unregistered resource types — treat as Healthy so any ! entries cap to ~.
func TestResolveRowColorBucket_UnknownTypeFallsBackToHealthy(t *testing.T) {
	got := resolveRowColorBucket("no-such-type", resource.Resource{ID: "x"})
	if got != resource.ColorHealthy {
		t.Errorf("resolveRowColorBucket(unknown) = %v, want ColorHealthy", got)
	}
}

// TestResolveRowColorBucket_DBCPhrases exercises the happy-path for the dbc
// type that triggered this rule: Healthy phrases (blank, maintenance overdue)
// return ColorHealthy; Warning phrases return ColorWarning; Broken phrases
// return ColorBroken. If the dbc Color func ever regresses, this pins the
// contract at the exact place the Attention color-cap depends on it.
func TestResolveRowColorBucket_DBCPhrases(t *testing.T) {
	cases := []struct {
		phrase string
		want   resource.Color
	}{
		{"", resource.ColorHealthy},
		{"maintenance overdue", resource.ColorHealthy}, // Wave-2 on Healthy row
		{"delete-protection off", resource.ColorWarning},
		{"not encrypted at rest", resource.ColorWarning},
		{"no automated backups", resource.ColorWarning},
		{"no automated backups (+1)", resource.ColorWarning}, // suffix stripped
		{"modifying: in progress", resource.ColorWarning},
		{"failed: cluster operation", resource.ColorBroken},
		{"encryption key unreachable", resource.ColorBroken},
		{"parameter group incompatible", resource.ColorBroken},
		{"no writer: reads only", resource.ColorBroken},
	}
	for _, tc := range cases {
		t.Run(tc.phrase, func(t *testing.T) {
			r := resource.Resource{
				ID:     "dbc-test",
				Fields: map[string]string{"status": tc.phrase},
			}
			got := resolveRowColorBucket("dbc", r)
			if got != tc.want {
				t.Errorf("resolveRowColorBucket(dbc, status=%q) = %v, want %v", tc.phrase, got, tc.want)
			}
		})
	}
}
