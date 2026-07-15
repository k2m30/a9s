// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import "github.com/k2m30/a9s/v3/core/domain"

// PaginationMeta holds cursor state for paginated fetches.
// Declaration lives in core/domain/contracts.go; this alias keeps
// existing consumers compiling.
type PaginationMeta = domain.PaginationMeta

// FetchResult wraps a resource page with pagination state.
// Declaration lives in core/domain/contracts.go; this alias keeps
// existing consumers compiling.
type FetchResult = domain.FetchResult
