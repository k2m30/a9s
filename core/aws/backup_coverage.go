// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// backup_coverage.go — the backup-plan coverage join, shared by every type
// that reports whether a backup plan selects it.
package aws

import (
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// backupCoverageCodes pairs each type's not-covered code with its sentence.
// The join is one implementation with four call sites, so the per-type wording
// lives here rather than at each caller.
var backupCoverageCodes = map[string]struct { //nolint:gochecknoglobals // static table, no init()
	code   domain.FindingCode
	detail string
}{
	"ebs": {CodeEBSNotInBackupPlan, ebsNotInBackupPlanDetail},
	"dbi": {CodeDBINotInBackupPlan, dbiNotInBackupPlanDetail},
	"dbc": {CodeDBCNotInBackupPlan, dbcNotInBackupPlanDetail},
	"ddb": {CodeDDBNotInBackupPlan, ddbNotInBackupPlanDetail},
}

// backupCoverageFindings reports the not-covered finding for one resource when
// no backup plan selection matches its ARN or its tags.
//
// It answers only from the backup list, so it inherits that list's three
// states: a list that was never fetched, one that was cut short, and one whose
// call failed all mean "cannot prove this resource is uncovered", and each
// yields no finding. Only a list read to the end with no match does.
//
// shortName selects the type's code and sentence from backupCoverageCodes; an
// unknown one yields nothing.
func backupCoverageFindings(
	cache resource.ResourceCache,
	shortName string,
	resourceARN string,
	tags map[string]string,
) []domain.Finding {
	if _, ok := backupCoverageCodes[shortName]; !ok || resourceARN == "" {
		return nil
	}
	// Cache-only, and read from the cache directly rather than through
	// relatedResourcesFor, which fetches the list when the cache has no entry.
	// An enricher must not turn a missing list into an API call, and a missing
	// entry is exactly the "cannot tell" state that reports nothing. A cut-short
	// entry is a lower bound on the plans, so it cannot prove a resource
	// uncovered either.
	entry, ok := cache["backup"]
	if !ok || entry.IsTruncated {
		return nil
	}
	_ = entry.Resources
	// Matching a plan's selections against the ARN and the tags is the
	// implementation round's work. Until it lands nothing is reported
	// uncovered, which is the direction that invents no findings.
	return nil
}

// addBackupCoverage appends each resource's coverage finding to result. It is
// the one place the join is folded into an enricher, so the four call sites
// differ only in their type name.
func addBackupCoverage(
	cache resource.ResourceCache,
	shortName string,
	resources []resource.Resource,
	result *IssueEnricherResult,
) {
	for _, r := range resources {
		if r.ID == "" {
			continue
		}
		// Tags stay nil until the join reads them: each type keeps them on a
		// different SDK struct, so pulling them out is the join's own work.
		fs := backupCoverageFindings(cache, shortName, r.Fields["arn"], nil)
		if len(fs) > 0 {
			result.Findings[r.ID] = append(result.Findings[r.ID], fs...)
		}
	}
}
