// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// IsIssueRow is the one answer to "is this row an issue", behind the menu
// badge, the disk count, the list title, the issues-only filter and both
// probes. A Wave-1 signal — an issue colour or an issue-severity finding —
// counts at warning and broken alike; a Wave-2 finding counts only at "!"
// severity, so a row whose only signal is a Wave-2 "~" is not an issue.
// Wave-2 findings are stripped before the colour is resolved because a colour
// func reading the findings would otherwise turn a Wave-2 "~" into an issue
// colour.
func IsIssueRow(td resource.ResourceTypeDef, r resource.Resource) bool {
	for _, f := range r.Findings {
		if f.Severity == domain.SevBroken || (!f.IsWave2Sourced() && f.Severity.IsIssue()) {
			return true
		}
	}
	return td.ResolveColor(wave1Only(r)).IsIssue()
}

func wave1Only(r resource.Resource) resource.Resource {
	if len(r.Findings) == 0 {
		return r
	}
	kept := make([]domain.Finding, 0, len(r.Findings))
	for _, f := range r.Findings {
		if !f.IsWave2Sourced() {
			kept = append(kept, f)
		}
	}
	r.Findings = kept
	return r
}

// IssueCount is the number of rows the issues-only filter keeps, which is the
// type's issue count. It counts per row, not per ID: two distinct resources
// that share an ID (two ACM certificates for one domain) are two issues.
func IssueCount(td resource.ResourceTypeDef, rows []resource.Resource) int {
	n := 0
	for _, r := range rows {
		if IsIssueRow(td, r) {
			n++
		}
	}
	return n
}

// ShowsIssueBadge reports whether a type's issue count is drawn as a number,
// on the menu badge and the list title. An ExcludeFromIssueBadge type (CloudTrail
// events, whose severity is per event, not per resource) keeps its count —
// the issues-only menu and filter read it — and shows no number.
func ShowsIssueBadge(td resource.ResourceTypeDef) bool {
	return !td.ExcludeFromIssueBadge
}

// raisesWave2Issue reports whether the type has a Wave-2 check that raises
// an issue at all: a row that check left unread can hide one, while an
// unread "~" check hides none.
func raisesWave2Issue(td resource.ResourceTypeDef) bool {
	for _, f := range td.Findings {
		if f.Source == "wave2" && f.Severity == domain.SevBroken {
			return true
		}
	}
	return false
}
