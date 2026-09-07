package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

// w29_absent_word_test.go — the defect that has now bitten the same way three
// times: a classifier reading a derived word out of Fields treats a missing one
// as the bad value and reports a posture finding for a row that never said
// anything. KMS's default arm called a stateless row unavailable; the first cut
// of the EKS posture predicate called a wordless row unencrypted and out of
// support.
//
// A word the fetcher always writes is absent only on a row built outside it, so
// absent means unknown, never bad. This pins that for every word a converted
// classifier reads, and the shape takes a second type by adding a row to the
// table — cfn's termination-protection and output-secret words land here when
// the late group converts it.

// w29PostureWords is the derived-word vocabulary each converted classifier
// reads back out of Fields: the lifecycle fields that put the row in its
// healthy state, and the posture words with the value that says "fine".
var w29PostureWords = []struct { //nolint:gochecknoglobals // test-only table
	short   string
	healthy map[string]string
	words   map[string]string
	allBad  map[string]string
	wantBad resource.Color
	// openVocabulary are words whose bad set is open by design, so "not the
	// good value" is the honest arm and an unrecognised token cannot be told
	// from a real one. They sit outside the unknown-token sweep.
	openVocabulary map[string]string
}{
	{
		short:   "cfn",
		healthy: map[string]string{"status": "CREATE_COMPLETE"},
		words: map[string]string{
			"termination_protection": "on",
			"output_secret":          "no",
		},
		allBad: map[string]string{
			"termination_protection": "off",
			"output_secret":          "yes",
		},
		wantBad: resource.ColorBroken,
	},
	{
		short:   "logs",
		healthy: map[string]string{"stored_bytes": "1024"},
		words: map[string]string{
			"retention":  "expires",
			"encryption": "kms",
		},
		allBad: map[string]string{
			"retention":  "never expire",
			"encryption": "none",
		},
		wantBad: resource.ColorWarning,
	},
	{
		short:   "eks",
		healthy: map[string]string{"status": "ACTIVE", "health_issues_count": "0"},
		words: map[string]string{
			"public_endpoint":       "no",
			"control_plane_logging": "complete",
			"secrets_encryption":    "kms",
			"version_support":       "standard",
		},
		allBad: map[string]string{
			"public_endpoint":       "open",
			"control_plane_logging": "incomplete",
			"secrets_encryption":    "none",
			"version_support":       "extended support",
		},
		wantBad: resource.ColorBroken,
		openVocabulary: map[string]string{
			"version_support": "the catalogue names every status that is not standard support, " +
				"so the arm cannot enumerate the bad ones",
		},
	},
}

func TestW29_AbsentWordIsUnknownNotBad(t *testing.T) {
	for _, tc := range w29PostureWords {
		t.Run(tc.short, func(t *testing.T) {
			td := resource.FindResourceType(tc.short)
			if td == nil {
				t.Fatalf("%s not registered", tc.short)
			}

			// The positive control. Without it every case below could pass
			// because the classifier ignores the words entirely.
			bad := map[string]string{}
			for k, v := range tc.healthy {
				bad[k] = v
			}
			for k, v := range tc.allBad {
				bad[k] = v
			}
			if got := td.ResolveColor(resource.Resource{ID: tc.short + "-probe", Fields: bad}); got != tc.wantBad {
				t.Fatalf("every word at its bad value = %v, want %v — the rest of this test would prove nothing",
					got, tc.wantBad)
			}

			t.Run("no_fields_at_all", func(t *testing.T) {
				if got := td.ResolveColor(resource.Resource{ID: tc.short + "-probe"}); got != resource.ColorHealthy {
					t.Errorf("a row with no fields = %v, want ColorHealthy", got)
				}
			})

			t.Run("lifecycle_only", func(t *testing.T) {
				if got := td.ResolveColor(resource.Resource{ID: tc.short + "-probe", Fields: tc.healthy}); got != resource.ColorHealthy {
					t.Errorf("a row with no posture words = %v, want ColorHealthy", got)
				}
			})

			// A word the fetcher never writes is unknown too. An arm that
			// matches "not the good value" reads it as the bad one.
			for unknown := range tc.words {
				if _, open := tc.openVocabulary[unknown]; open {
					continue
				}
				t.Run("unknown_"+unknown, func(t *testing.T) {
					fields := map[string]string{}
					for k, v := range tc.healthy {
						fields[k] = v
					}
					for k, v := range tc.words {
						fields[k] = v
					}
					fields[unknown] = "not-a-word-the-fetcher-writes"
					if got := td.ResolveColor(resource.Resource{ID: tc.short + "-probe", Fields: fields}); got != resource.ColorHealthy {
						t.Errorf("a row whose %q is an unrecognised token = %v, want ColorHealthy", unknown, got)
					}
				})
			}

			// Partially filled: every word present and healthy except one.
			for absent := range tc.words {
				t.Run("without_"+absent, func(t *testing.T) {
					fields := map[string]string{}
					for k, v := range tc.healthy {
						fields[k] = v
					}
					for k, v := range tc.words {
						if k != absent {
							fields[k] = v
						}
					}
					if got := td.ResolveColor(resource.Resource{ID: tc.short + "-probe", Fields: fields}); got != resource.ColorHealthy {
						t.Errorf("a row missing only %q = %v, want ColorHealthy", absent, got)
					}
				})
			}
		})
	}
}
