// IsBackgroundTaskKind classifies a runtime.TaskKind as background (its result
// feeds core/session state that a later render consumes) or blocking (its
// result is the screen content, or it is a renderer-only adapter task that must
// complete before the screen is usable).
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
)

func TestIsBackgroundTaskKind_TableAllKnownKinds(t *testing.T) {
	tests := []struct {
		name string
		kind runtime.TaskKind
		want bool
	}{
		// -- background --
		{"KindRelatedCheck", runtime.KindRelatedCheck, true},
		{"KindEnrichDetail", runtime.KindEnrichDetail, true},
		{"KindEnrichRow", runtime.KindEnrichRow, true},
		{"TaskKindProbeEnrich", runtime.TaskKindProbeEnrich, true},
		{"TaskKindSaveCache", runtime.TaskKindSaveCache, true},

		// -- blocking: result IS the screen content --
		{"KindFetchResources", runtime.KindFetchResources, false},
		{"KindFetchFiltered", runtime.KindFetchFiltered, false},
		{"KindFetchMore", runtime.KindFetchMore, false},
		{"KindFetchByIDDetail", runtime.KindFetchByIDDetail, false},
		{"TaskKindFetchChildResources", runtime.TaskKindFetchChildResources, false},
		{"KindFetchProfiles", runtime.KindFetchProfiles, false},
		{"KindFetchReveal", runtime.KindFetchReveal, false},

		// -- blocking: connect / identity / cache-load gate the first render --
		{"TaskKindProbeAvailability", runtime.TaskKindProbeAvailability, false},
		{"TaskKindConnect", runtime.TaskKindConnect, false},
		{"TaskKindFetchIdentity", runtime.TaskKindFetchIdentity, false},
		{"TaskKindLoadAvailCache", runtime.TaskKindLoadAvailCache, false},
		{"TaskKindDemoPrefetchCounts", runtime.TaskKindDemoPrefetchCounts, false},

		// -- blocking: renderer-only adapter kinds --
		{"TaskKindFlashTick", runtime.TaskKindFlashTick, false},
		{"TaskKindEmitNavigate", runtime.TaskKindEmitNavigate, false},
		{"TaskKindEmitAPIError", runtime.TaskKindEmitAPIError, false},
		{"TaskKindReadThemeFile", runtime.TaskKindReadThemeFile, false},
		{"TaskKindSaveThemeConfig", runtime.TaskKindSaveThemeConfig, false},

		// -- unknown kind defaults to blocking (fail safe: an unrecognised
		// kind must not silently get deferred and dropped from a sync render) --
		{"UnknownKind", runtime.TaskKind("totally-unknown-kind"), false},
		{"EmptyKind", runtime.TaskKind(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := app.IsBackgroundTaskKind(tc.kind)
			if got != tc.want {
				t.Errorf("IsBackgroundTaskKind(%q) = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}
