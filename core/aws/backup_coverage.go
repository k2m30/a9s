// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// backup_coverage.go — the backup-plan coverage join, shared by every type
// that reports whether a backup plan selects it.
package aws

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// addBackupCoverage reports every resource no cached backup plan selects.
//
// It answers from the cached backup list, so it inherits that list's states: a
// list nobody fetched and a list cut short both mean a plan this resource
// matches may sit on a page nobody read, so neither reports anything. A list
// read to the end reports, including when it is empty — an account with no
// plans covers nothing, and that is knowledge rather than a gap.
//
// arnAndTags gives the resource's ARN and the tags a selection may condition
// on. Its third value is false when those tags could not be read. Unknown tags
// only silence the finding where they could change it — when some plan selects
// by tag; where every selection names ARNs, tags decide nothing and the
// verdict stands.
//
// A plan whose own selection list could not be read to the end is the same
// gap one level up: it may select anything, so nothing in the account can be
// called uncovered while it is in the cache.
func addBackupCoverage(
	cache resource.ResourceCache,
	shortName string,
	code domain.FindingCode,
	resources []resource.Resource,
	arnAndTags func(resource.Resource) (string, map[string]string, bool),
	result *IssueEnricherResult,
) {
	entry, ok := cache["backup"]
	if !ok || entry.IsTruncated || backupPlansIncomplete(entry.Resources) {
		return
	}
	tagsDecide := backupPlansSelectByTag(entry.Resources)
	for _, r := range resources {
		if r.ID == "" {
			continue
		}
		arn, tags, known := arnAndTags(r)
		// A row the fetcher gave no ARN cannot be matched against a selection,
		// so it is not evidence either way.
		if arn == "" || (!known && tagsDecide) || backupPlansCover(entry.Resources, arn, tags) {
			continue
		}
		setWave2Finding(result, r.ID, code, "~", shortName, []domain.DetailRow{{Label: "Backup plans", Value: "0"}})
	}
}

// backupTagReader answers one resource's tags from its own service, for the
// types whose list call does not return them.
type backupTagReader func(ctx context.Context, arn string) (map[string]string, error)

// backupTagsAccessor builds the coverage join's accessor for a type whose rows
// carry an ARN but no tags.
//
// It reads tags only where they can change the answer: when no cached plan
// selects by tag, when a plan's own selection list could not be read to the
// end (addBackupCoverage reports nothing at all in that case), and for every
// resource a selection's ARNs already take in, the read is skipped. The rest
// are read one call each, in parallel, up to EnrichmentCap. A resource whose
// read fails or falls outside the cap has unknown tags, which the join leaves
// alone.
func backupTagsAccessor(
	ctx context.Context,
	cache resource.ResourceCache,
	resources []resource.Resource,
	read backupTagReader,
	result *IssueEnricherResult,
	op string,
) (func(resource.Resource) (string, map[string]string, bool), error) {
	entry, ok := cache["backup"]
	if !ok || entry.IsTruncated || read == nil ||
		backupPlansIncomplete(entry.Resources) || !backupPlansSelectByTag(entry.Resources) {
		return backupARNFromField, nil
	}

	tags := make(map[string]map[string]string, len(resources))
	var pending []resource.Resource
	for _, r := range resources {
		arn := r.Fields["arn"]
		if r.ID == "" || arn == "" {
			continue
		}
		if backupPlansCover(entry.Resources, arn, nil) {
			tags[r.ID] = nil
			continue
		}
		pending = append(pending, r)
	}
	pending = capAtEnrichmentCap(result, pending, resourceIDsOf)
	n := len(pending)

	var mu sync.Mutex
	var failures []Failure
	walkErr := ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := pending[i]
		t, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (map[string]string, error) {
			return read(ctx, r.Fields["arn"])
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(result, r.ID, &failures, err)
			return
		}
		tags[r.ID] = t
	})

	return func(r resource.Resource) (string, map[string]string, bool) {
		t, known := tags[r.ID]
		return r.Fields["arn"], t, known
	}, errors.Join(walkErr, AggregateFailures(op, failures, n))
}

// backupPlansIncomplete reports whether any plan's selection enumeration
// stopped short of the whole list. The plan row then names some of what the
// plan protects and no way to tell how much is missing, so it cannot support
// "not covered" for anything.
func backupPlansIncomplete(plans []resource.Resource) bool {
	for _, plan := range plans {
		if plan.Fields[backupSelectionsPartialField] != "" {
			return true
		}
	}
	return false
}

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
func backupPlansCover(plans []resource.Resource, arn string, tags map[string]string) bool {
	for _, plan := range plans {
		if backupPlanCovers(plan, arn, tags) {
			return true
		}
	}
	return false
}

// backupPlanCovers applies one plan's selection: an ARN the include list takes
// in, or a tag condition the resource satisfies, minus anything the exclude
// list names.
func backupPlanCovers(plan resource.Resource, arn string, tags map[string]string) bool {
	if backupARNListMatches(plan.Fields["not_resources"], arn) {
		return false
	}
	if backupARNListMatches(plan.Fields["resources"], arn) {
		return true
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

// backupARNPatternMatches applies one selection pattern, where `*` stands for
// any run of characters and everything else is literal. `?`, `[` and `]` are
// every character AWS Backup gives a meaning the join does not model, and a
// pattern using one is matched as covering, so an unfamiliar shape never
// invents a finding.
func backupARNPatternMatches(pattern, arn string) bool {
	if strings.ContainsAny(pattern, "?[]") {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == arn
	}
	return regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$").MatchString(arn)
}

// backupARNFromField is the accessor for every type whose fetcher already puts
// an ARN on the row and whose tags no selection needs read.
func backupARNFromField(r resource.Resource) (string, map[string]string, bool) {
	return r.Fields["arn"], nil, true
}
