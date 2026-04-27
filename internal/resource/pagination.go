package resource

import "github.com/k2m30/a9s/v3/internal/domain"

// PaginationMeta holds cursor state for paginated fetches.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type PaginationMeta = domain.PaginationMeta

// FetchResult wraps a resource page with pagination state.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type FetchResult = domain.FetchResult
