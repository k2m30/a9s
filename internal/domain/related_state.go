package domain

// RelatedRowState classifies the resolution state of a related-resource
// result or row (RelatedCheckResult / DetailRelatedRow / RelatedBlock /
// rightColumnRow), replacing the former Count == -1 sentinel. RelatedResolved
// is the zero value so a resolved result (Count 0..N is authoritative) needs
// no explicit State assignment.
type RelatedRowState uint8

const (
	// RelatedResolved means Count (0..N) is authoritative. Zero value.
	RelatedResolved RelatedRowState = iota
	// RelatedLoading means the checker is still in flight.
	RelatedLoading
	// RelatedError means the checker (or a prerequisite lookup) failed.
	RelatedError
	// RelatedUnknown means the result resolved but the count could not be
	// computed (e.g. a two-hop checker whose source lookup missed in a
	// truncated cache). Renders as "(?)".
	RelatedUnknown
	// RelatedDeferred means navigation uses a server-side FetchFilter pivot
	// instead of a local count. Renders with a blank count badge.
	RelatedDeferred
)

// String returns the debug name of the state.
func (s RelatedRowState) String() string {
	switch s {
	case RelatedResolved:
		return "resolved"
	case RelatedLoading:
		return "loading"
	case RelatedError:
		return "error"
	case RelatedUnknown:
		return "unknown"
	case RelatedDeferred:
		return "deferred"
	default:
		return "invalid"
	}
}
