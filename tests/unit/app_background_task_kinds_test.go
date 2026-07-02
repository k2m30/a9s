// app_background_task_kinds_test.go — contract pin for app.IsBackgroundTaskKind.
//
// Contract: IsBackgroundTaskKind classifies a
// runtime.TaskKind as "background" (its result feeds internal/session state
// that a later render consumes, rather than being the screen content itself)
// versus "blocking" (its result IS the screen content, or it is a
// renderer-only adapter task that must complete before the screen is usable).
//
// Background kinds (exactly 4):
//
//	runtime.KindRelatedCheck     — related-panel fan-out; result feeds RelatedCache/RelatedRows async.
//	runtime.KindEnrichDetail     — Wave-2 detail enrichment; result patches DetailState async.
//	runtime.TaskKindProbeEnrich  — Wave-2 menu enrichment probe; result patches menu badges async.
//	runtime.TaskKindSaveCache    — disk cache persistence; no screen content at all.
//
// Every other enumerated TaskKind (18 total, read from internal/runtime/tasks.go,
// related.go, enrich.go, handlers_navigate.go, handlers_related.go) is blocking:
// the caller's request should stall until the task completes because the
// task's own result IS the screen the user is waiting on, or the adapter has
// no meaningful state to render before the task lands.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

func TestIsBackgroundTaskKind_TableAllKnownKinds(t *testing.T) {
	tests := []struct {
		name string
		kind runtime.TaskKind
		want bool
	}{
		// -- background (4) --
		{"KindRelatedCheck", runtime.KindRelatedCheck, true},
		{"KindEnrichDetail", runtime.KindEnrichDetail, true},
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

		// -- blocking: renderer-only adapter kinds (no screen meaning headless,
		// but not part of the 4-kind background allowlist either) --
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
