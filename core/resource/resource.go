// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package resource defines the generic resource model used across all AWS resource types.
package resource

import "github.com/k2m30/a9s/v3/core/domain"

// Resource is the generic AWS resource instance. Declaration lives in
// core/domain; this alias re-exports it.
type Resource = domain.Resource

// DedupByID returns rs keyed by resource ID — the first row carrying an ID
// wins — and the IDs that more than one row carried, in the order they
// collapsed. Rows are keyed by their ID everywhere downstream (findings,
// attention details, related sets, the cursor), so two rows with one ID
// cannot both be shown; dups is what the screen owes the operator.
func DedupByID(rs []Resource) (out []Resource, dups []string) {
	if len(rs) == 0 {
		return rs, nil
	}
	seen := make(map[string]bool, len(rs))
	reported := make(map[string]bool)
	out = make([]Resource, 0, len(rs))
	for _, r := range rs {
		if seen[r.ID] {
			if !reported[r.ID] {
				reported[r.ID] = true
				dups = append(dups, r.ID)
			}
			continue
		}
		seen[r.ID] = true
		out = append(out, r)
	}
	return out, dups
}
