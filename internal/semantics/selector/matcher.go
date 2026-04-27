// Package selector provides resource matching primitives.
package selector

import (
	"github.com/k2m30/a9s/v3/internal/domain"
)

// Matcher reports whether a Resource satisfies a selection condition.
type Matcher interface {
	Matches(r domain.Resource) bool
}

// WildcardARN returns a Matcher that matches resources whose ID or ARN
// matches the given glob-style pattern. Real implementation lands later.
func WildcardARN(_ string) Matcher { return nil }

// Tags returns a Matcher that matches resources satisfying all provided
// tag conditions. Real implementation lands later.
func Tags(_ []TagCondition) Matcher { return nil }
