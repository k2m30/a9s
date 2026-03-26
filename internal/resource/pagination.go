package resource

// PaginationMeta holds cursor state for paginated fetches.
type PaginationMeta struct {
	// IsTruncated is true when more pages exist beyond what was returned.
	IsTruncated bool
	// NextToken is an opaque continuation token for the next page.
	NextToken string
	// TotalHint is the known or estimated total count. -1 means unknown.
	TotalHint int
	// PageSize is the number of items returned in this page.
	PageSize int
}

// FetchResult wraps a resource page with pagination state.
type FetchResult struct {
	Resources  []Resource
	Pagination *PaginationMeta // nil means unpaginated (legacy compatibility)
}
