package app

import "github.com/k2m30/a9s/v3/internal/runtime"

// IsBackgroundTaskKind classifies a runtime.TaskKind as "background" (its
// result feeds internal/session state that a later render consumes, rather
// than being the screen content itself) versus "blocking" (its result IS the
// screen content, or it is a renderer-only adapter task that must complete
// before the screen is usable).
//
// Exactly 4 kinds are background:
//
//	runtime.KindRelatedCheck    — related-panel fan-out; result feeds RelatedCache/RelatedRows async.
//	runtime.KindEnrichDetail    — Wave-2 detail enrichment; result patches DetailState async.
//	runtime.TaskKindProbeEnrich — Wave-2 menu enrichment probe; result patches menu badges async.
//	runtime.TaskKindSaveCache   — disk cache persistence; no screen content at all.
//
// Every other kind, including unrecognised/empty kinds, is blocking: an
// unknown kind must not silently be deferred and dropped from a sync render.
func IsBackgroundTaskKind(kind runtime.TaskKind) bool {
	switch kind {
	case runtime.KindRelatedCheck, runtime.KindEnrichDetail, runtime.TaskKindProbeEnrich, runtime.TaskKindSaveCache:
		return true
	default:
		return false
	}
}
