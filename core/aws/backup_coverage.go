// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// backup_coverage.go — the backup-plan coverage join, shared by every type
// that reports whether a backup plan selects it.
package aws

import (
	"regexp"
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// backupCoverageDef is one type's half of the join: what it reports, and
// whether its rows carry the tags a plan may select on.
type backupCoverageDef struct {
	code   domain.FindingCode
	detail string
	// evaluatesTags is false for every type whose rows keep their tags behind
	// a separate API call. Those types are judged on selection ARNs alone, and
	// the finding says so when a tag selection is present, because a
	// tag-selecting plan is common enough that treating it as covering
	// everything would silence the signal for the whole type.
	evaluatesTags bool
}

// backupCoverageCodes pairs each type's not-covered code with its sentence.
// The join is one implementation with four call sites, so the per-type wording
// lives here rather than at each caller.
var backupCoverageCodes = map[string]backupCoverageDef{ //nolint:gochecknoglobals // static table, no init()
	"ebs": {CodeEBSNotInBackupPlan, ebsNotInBackupPlanDetail, true},
	"dbi": {CodeDBINotInBackupPlan, dbiNotInBackupPlanDetail, false},
	"dbc": {CodeDBCNotInBackupPlan, dbcNotInBackupPlanDetail, false},
	"ddb": {CodeDDBNotInBackupPlan, ddbNotInBackupPlanDetail, false},
}

// addBackupCoverage reports every resource no cached backup plan selects.
//
// It answers only from the cached backup list, so it inherits that list's
// states: a list nobody fetched and a list cut short both mean a plan this
// resource matches may sit on a page nobody read, so neither reports anything.
// A list read to the end reports, including when it is empty — an account with
// no plans covers nothing, and that is knowledge rather than a gap.
//
// arnAndTags gives the resource's ARN and its tags. The ARN is what selections
// match on and every type builds it differently; tags are nil for the types
// that do not carry them.
func addBackupCoverage(
	cache resource.ResourceCache,
	shortName string,
	resources []resource.Resource,
	arnAndTags func(resource.Resource) (string, map[string]string),
	result *IssueEnricherResult,
) {
	def, ok := backupCoverageCodes[shortName]
	if !ok {
		return
	}
	entry, ok := cache["backup"]
	if !ok || entry.IsTruncated {
		return
	}
	detail := def.detail
	if !def.evaluatesTags && backupPlansSelectByTag(entry.Resources) {
		detail += " " + backupTagSelectionUnreadDetail
	}
	for _, r := range resources {
		if r.ID == "" {
			continue
		}
		arn, tags := arnAndTags(r)
		// A row the fetcher gave no ARN cannot be matched against a selection,
		// so it is not evidence either way.
		if arn == "" || backupPlansCover(entry.Resources, arn, tags, def.evaluatesTags) {
			continue
		}
		setWave2Finding(result, r.ID, def.code, "not covered by a backup plan", "~", shortName,
			[]domain.DetailRow{{Label: "Backup plans", Value: "0"}}, detail)
	}
}

// backupTagSelectionUnreadDetail is appended for the types whose rows carry no
// tags, when some plan selects by one. Without it the operator cannot tell the
// difference between "no plan names this" and "no plan names this and one
// selects by a tag nobody here can read".
const backupTagSelectionUnreadDetail = "One of the plans selects by tag, which this list cannot read for this resource type, so a tag on the resource may already put it in that plan."

// backupPlansSelectByTag reports whether any plan chooses resources by tag.
func backupPlansSelectByTag(plans []resource.Resource) bool {
	for _, plan := range plans {
		if plan.Fields["selection_tags"] != "" {
			return true
		}
	}
	return false
}

// backupPlansCover reports whether any plan's selection takes in the resource.
func backupPlansCover(plans []resource.Resource, arn string, tags map[string]string, evaluatesTags bool) bool {
	for _, plan := range plans {
		if backupPlanCovers(plan, arn, tags, evaluatesTags) {
			return true
		}
	}
	return false
}

// backupPlanCovers applies one plan's selection: an ARN the include list takes
// in, or a tag condition the resource satisfies, minus anything the exclude
// list names.
func backupPlanCovers(plan resource.Resource, arn string, tags map[string]string, evaluatesTags bool) bool {
	if backupARNListMatches(plan.Fields["not_resources"], arn) {
		return false
	}
	if backupARNListMatches(plan.Fields["resources"], arn) {
		return true
	}
	if !evaluatesTags {
		return false
	}
	return backupSelectionTagsMatch(plan.Fields["selection_tags"], tags)
}

// backupARNListMatches reports whether any pattern in a comma-separated
// selection list takes in the ARN.
func backupARNListMatches(list, arn string) bool {
	for pattern := range strings.SplitSeq(list, ",") {
		if pattern = strings.TrimSpace(pattern); pattern != "" && backupARNPatternMatches(pattern, arn) {
			return true
		}
	}
	return false
}

// backupWildcardRun is every character AWS Backup's selection syntax gives a
// meaning the join does not model. A pattern using one is matched as covering,
// so an unfamiliar shape never invents a finding.
var backupWildcardRun = regexp.MustCompile(`[?\[\]]`) //nolint:gochecknoglobals // compiled once

// backupARNPatternMatches applies one selection pattern, where `*` stands for
// any run of characters and everything else is literal.
func backupARNPatternMatches(pattern, arn string) bool {
	if backupWildcardRun.MatchString(pattern) {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == arn
	}
	re, err := regexp.Compile("^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$")
	if err != nil {
		return true
	}
	return re.MatchString(arn)
}

// backupARNFromField is the ARN accessor for every type whose fetcher already
// puts one on the row.
func backupARNFromField(r resource.Resource) (string, map[string]string) {
	return r.Fields["arn"], nil
}
