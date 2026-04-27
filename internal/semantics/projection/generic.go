// Package projection provides DetailProjector implementations for resource types.
package projection

import "github.com/k2m30/a9s/v3/internal/domain"

// Generic is the default DetailProjector. STUB: real implementation lands in
// PR-01 step 3. For now returns an empty slice so callers compile.
func Generic(r domain.Resource) []domain.Section { return nil }
