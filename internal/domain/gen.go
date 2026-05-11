package domain

// Gen is the program-wide generation-counter type used by async-result
// staleness guards. Every session-rotation counter (ConnectGen,
// AvailabilityGen, EnrichmentGen, RelatedGen, EnrichGen, per-type
// EnrichmentTypeGen) and every message field that carries one of those
// values (Gen, TypeGen, NewGen, CurrentGen, Generation) uses this single
// type — there is one generation-counter type across the program.
//
// A handler captures the current session Gen at dispatch time and stamps
// it onto an outgoing async tea.Cmd. When the async result returns, the
// handler compares the carried Gen against the live session Gen; a
// mismatch means the user switched profile/region (or triggered a
// refresh) while the work was in flight, so the result is stale and
// must be dropped.
//
// Thread-safety contract: Gen is read and written exclusively on the
// Bubble Tea Update goroutine — Session.Rotate(), refresh handlers, and
// per-handler stamp/check sites all run there. Async tea.Cmd closures
// capture a Gen by value at dispatch time and never mutate the live
// session Gen from a goroutine; the captured value is compared inside
// the handler when the result arrives. Because the Bubble Tea runtime
// serializes Update, no atomic operations are required.
//
// Zero value: a freshly-constructed Session sets every Gen to 1 (via
// session.New). The zero value 0 is reserved for "pre-guard dispatch"
// sentinels — handlers that see a message with Gen == 0 accept it
// unconditionally so synthetic test messages and early-return paths
// don't require a live session to round-trip a gen.
type Gen uint64

// Bump increments the generation in place and returns the new value.
// Use from Session.Rotate and refresh handlers; the returned value is
// the new "current" Gen that subsequent dispatches will stamp onto
// outgoing tea.Cmds.
func (g *Gen) Bump() Gen {
	*g++
	return *g
}
