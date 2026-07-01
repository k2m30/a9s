// Package resource defines the generic resource model used across all AWS resource types.
package resource

import "github.com/k2m30/a9s/v3/internal/domain"

// Resource is the generic AWS resource instance. Declaration lives in
// internal/domain; this alias lets existing consumers compile without changes.
type Resource = domain.Resource
