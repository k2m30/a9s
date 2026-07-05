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

// IsBackgroundFetchTask is a context-aware sibling of IsBackgroundTaskKind for
// classifying a single TaskRequest, adding one additional case on top of the
// existing table: a runtime.KindFetchResources task is background exactly
// when screenAlreadyRenderable is true (the target screen already has cached
// rows seeded, or a Loading shell is already on screen) — the transport must
// never block the response on a fetch whose result the response doesn't need
// to already show SOMETHING. A genuinely cold fetch (nothing renderable yet,
// screenAlreadyRenderable=false) stays blocking, since the synchronous half
// of the partition is what produces the `Loading…` shell in the same
// response (DEF-1/C4, pilot step 2).
//
// Every other TaskKind's classification is delegated unchanged to
// IsBackgroundTaskKind — this function only changes behavior for
// KindFetchResources; it never diverges from the existing table for any
// other kind, regardless of screenAlreadyRenderable.
func IsBackgroundFetchTask(req runtime.TaskRequest, screenAlreadyRenderable bool) bool {
	if req.Key.Kind == runtime.KindFetchResources {
		return screenAlreadyRenderable
	}
	return IsBackgroundTaskKind(req.Key.Kind)
}
