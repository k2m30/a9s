// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// backup_coverage.go — the backup-plan coverage join, shared by every type
// that reports whether a backup plan selects it.
package aws

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// addBackupCoverage reports every resource no cached backup plan selects.
//
// It answers from the cached backup list, so it inherits that list's states: a
// list nobody fetched and a list cut short both mean a plan this resource
// matches may sit on a page nobody read, so every row is marked not inspected
// instead. A list read to the end reports, including when it is empty — an
// account with no plans covers nothing, and that is knowledge rather than a
// gap.
//
// targetOf gives what a selection is evaluated against for each row. A row
// no plan covers and some plan might is marked not inspected, naming the check
// that could still decide it.
func addBackupCoverage(
	cache resource.ResourceCache,
	code domain.FindingCode,
	resources []resource.Resource,
	targetOf func(resource.Resource) backupTarget,
	result *IssueEnricherResult,
) {
	entry, ok := cache["backup"]
	if !ok || entry.IsTruncated {
		for _, r := range resources {
			markUninspected(result, r.ID, checkListIncomplete("backup"))
		}
		return
	}
	for _, r := range resources {
		if r.ID == "" {
			continue
		}
		t := targetOf(r)
		if t.arn == "" {
			if len(entry.Resources) > 0 {
				markUninspected(result, r.ID, t.unread)
			}
			continue
		}
		ids, undecided := backupPlansCovering(entry.Resources, t)
		switch {
		case len(ids) > 0:
		case undecided == "":
			setWave2Finding(result, r.ID, code, []domain.DetailRow{{Label: "Backup plans", Value: "0"}})
		default:
			markUninspected(result, r.ID, undecided)
		}
	}
}

// backupTagReader answers one resource's tags from its own service, for the
// types whose list call does not return them.
type backupTagReader func(ctx context.Context, arn string) (map[string]string, error)

// backupTagsAccessor builds the coverage join's targetOf for a type whose rows
// carry an ARN but no tags; op is the call read performs.
//
// It reads tags only where they can change the answer: when a plan's own
// selection list could not be read to the end (no tag read can then prove a
// resource uncovered), and for every resource whose verdict no selection's tag
// clause decides, the read is skipped. The rest are read one call each, in
// parallel, up to EnrichmentCap. A resource whose tags were not read names op
// as its unread check.
func backupTagsAccessor(
	ctx context.Context,
	cache resource.ResourceCache,
	resources []resource.Resource,
	targetOf func(resource.Resource) backupTarget,
	read backupTagReader,
	result *IssueEnricherResult,
	op string,
) (func(resource.Resource) backupTarget, error) {
	untagged := func(r resource.Resource) backupTarget {
		t := targetOf(r)
		if t.arn != "" {
			t.unread = op
		}
		return t
	}
	entry, ok := cache["backup"]
	if !ok || entry.IsTruncated || read == nil || backupPlansIncomplete(entry.Resources) {
		return untagged, nil
	}

	tags := make(map[string]map[string]string, len(resources))
	var pending []resource.Resource
	for _, r := range resources {
		t := untagged(r)
		if r.ID == "" || t.arn == "" {
			continue
		}
		if ids, undecided := backupPlansCovering(entry.Resources, t); len(ids) > 0 || undecided == "" {
			continue
		}
		pending = append(pending, r)
	}
	pending = capAtEnrichmentCap(result, pending, nil, resourceIDsOf)
	n := len(pending)

	var mu sync.Mutex
	var failures []Failure
	walkErr := ForEachRow(ctx, result, resourceIDs(pending), EnrichmentParallelism, func(i int) {
		r := pending[i]
		t, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (map[string]string, error) {
			return read(ctx, targetOf(r).arn)
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(result, r.ID, &failures, err)
			return
		}
		tags[r.ID] = t
	})

	return func(r resource.Resource) backupTarget {
		t := untagged(r)
		if read, ok := tags[r.ID]; ok {
			t.tags, t.unread = read, ""
		}
		return t
	}, errors.Join(walkErr, AggregateFailures(op, failures, n))
}

// backupPlansIncomplete reports whether any plan's selections were not all
// read. Such a plan may select anything, so it cannot support "not covered".
func backupPlansIncomplete(plans []resource.Resource) bool {
	return slices.ContainsFunc(plans, func(plan resource.Resource) bool {
		_, complete := BackupPlanSelections(plan)
		return !complete
	})
}

// backupTargetFromFields is targetOf for every type whose fetcher puts the
// ARN, and for a cluster the engine, on the row; describe is the call that
// returns the ARN.
func backupTargetFromFields(describe string) func(resource.Resource) backupTarget {
	return func(r resource.Resource) backupTarget {
		t := backupTarget{arn: r.Fields["arn"], engine: r.Fields["engine"]}
		if t.arn == "" {
			t.unread = describe
		}
		return t
	}
}
