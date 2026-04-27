// Package selector provides resource matching primitives.
package selector

import (
	"github.com/k2m30/a9s/v3/internal/domain"
)

// Matcher reports whether a Resource satisfies a selection condition.
type Matcher interface {
	Matches(r domain.Resource) bool
}

// noopMatcher is a safe stub that always returns false. Used as the
// return value for constructors whose real implementations have not yet
// landed, so callers never receive a nil Matcher and cannot NPE.
type noopMatcher struct{}

func (noopMatcher) Matches(_ domain.Resource) bool { return false }

// WildcardARN returns a Matcher that matches resources whose ID or ARN
// matches the given glob-style pattern. Real implementation lands later;
// until then it returns a no-op matcher that always returns false.
//
// TODO(phase-04): wire glob matching to Resource.ARN/ID per docs/refactor/04-catalog.md.
// Until then any caller that constructs a WildcardARN will see all resources
// fail to match — fail-closed is intentional.
func WildcardARN(_ string) Matcher { return noopMatcher{} }

// Tags returns a Matcher that matches resources satisfying all provided
// tag conditions. Real implementation lands later; until then it returns
// a no-op matcher that always returns false.
//
// TODO(phase-04): implement TagCondition evaluation against Resource.Tags
// per docs/refactor/04-catalog.md. Fail-closed until then.
func Tags(_ []TagCondition) Matcher { return noopMatcher{} }
