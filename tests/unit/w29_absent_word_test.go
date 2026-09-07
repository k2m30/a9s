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
// absent means unknown, never bad. The same is true of a word the fetcher never
// writes: a stale cache entry or a later spelling is unknown, not bad, so an arm
// has to match the values it recognises rather than exclude the good one. This
// pins both for every word a converted classifier reads, and takes another type
// by adding a row to the table.

// w29PostureWords is the derived-word vocabulary each converted classifier
// reads back out of Fields: the lifecycle fields that put the row in its
// healthy state, and the posture words with the value that says "fine".
var w29PostureWords = []struct { //nolint:gochecknoglobals // test-only table
	short   string
	healthy map[string]string
	words   map[string]string
	allBad  map[string]string
	wantBad resource.Color
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
		short: "acm",
		// A certificate not in use is an orphan whatever its key is, so the
		// lifecycle half of the row has to say it is attached before the words
		// can be read on their own.
		healthy: map[string]string{"in_use": "true"},
		words: map[string]string{
			"status":        "issued",
			"key_algorithm": "RSA 2048",
		},
		allBad: map[string]string{
			"status":        "revoked",
			"key_algorithm": "RSA 1024",
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
			"version_support":       "out of standard support",
		},
		wantBad: resource.ColorBroken,
	},
}

func TestW29_AbsentWordIsUnknownNotBad(t *testing.T) {
	for _, tc := range w29PostureWords {
		t.Run(tc.short, func(t *testing.T) {
			td := resource.FindResourceType(tc.short)
			if td == nil {
				t.Fatalf("%s not registered", tc.short)
			}

			// The positive controls. Without them every case below could pass
			// because the classifier ignores the words entirely. The first is
			// per word, since a sweep that only ever sets them together proves
			// nothing about the one word an arm stopped reading; what colour
			// each word carries on its own is pinned by its type's own table,
			// so the control only asks that the word moves the row off healthy.
			for word, badValue := range tc.allBad {
				fields := map[string]string{}
				for k, v := range tc.healthy {
					fields[k] = v
				}
				for k, v := range tc.words {
					fields[k] = v
				}
				fields[word] = badValue
				if got := td.ResolveColor(resource.Resource{ID: tc.short + "-probe", Fields: fields}); got == resource.ColorHealthy {
					t.Fatalf("%q at %q leaves the row healthy; nothing reads that word, so the cases below prove nothing",
						word, badValue)
				}
			}

			// The second is every word at once, which is also where the worst
			// of them has to win.
			bad := map[string]string{}
			for k, v := range tc.healthy {
				bad[k] = v
			}
			for k, v := range tc.allBad {
				bad[k] = v
			}
			if got := td.ResolveColor(resource.Resource{ID: tc.short + "-probe", Fields: bad}); got != tc.wantBad {
				t.Fatalf("every word at its bad value = %v, want %v", got, tc.wantBad)
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
