// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package resource defines the generic resource model used across all AWS resource types.
package resource

import "github.com/k2m30/a9s/v3/core/domain"

// Resource is the generic AWS resource instance. Declaration lives in
// core/domain; this alias lets existing consumers compile without changes.
type Resource = domain.Resource

// DedupByID returns the subset of incoming whose ID is not already present in
// existing, preserving incoming's order. Rows are keyed by their stable
// resource ID.
func DedupByID(existing, incoming []Resource) []Resource {
	if len(incoming) == 0 {
		return incoming
	}
	seen := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		seen[r.ID] = struct{}{}
	}
	out := make([]Resource, 0, len(incoming))
	for _, r := range incoming {
		if _, dup := seen[r.ID]; dup {
			continue
		}
		seen[r.ID] = struct{}{}
		out = append(out, r)
	}
	return out
}
