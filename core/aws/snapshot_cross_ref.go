// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// snapshot_cross_ref.go — generic Wave-1 cross-ref enricher pattern shared
// across snapshot resource types (dbi-snap, dbc-snap, future ebs-snap).
//
// Two signals fire from sibling-cache scans (zero AWS API calls):
//
//  1. orphan: snapshot's parent identifier NOT found in the loaded parent
//     cache, AND the cache is not truncated.
//     Phrase: configurable per-type (e.g. "orphan: source DB deleted").
//
//  2. past-retention: automated snapshot older than the parent's
//     `BackupRetentionPeriod` (1.0× — no multiplier; the operator's
//     declared retention IS the policy). Only fires when the parent IS in
//     the cache, the snapshot is "automated", and the parent retention > 0.
//     Phrase: configurable per-type
//     (e.g. "automated, <N>d past retention").
//
// Wave classification stays Wave 1 (zero SDK calls) — the helper scans the
// in-memory ResourceCache only. Both signals route through
// `IssueEnricherResult.Findings` (which surfaces in S5 Attention and, via
// `applyEnrichment` → `applyWave2ToRow`, in the S4 status column via
// `domain.StatusPhrase(r.Findings)` at render time). Re-runs are idempotent:
// Findings is map-keyed so a second pass overwrites the first.
//
// Retention-rule-disabled mode: when a future consumer's parent type has no
// retention concept (e.g. ebs-snap on ec2.Volume), set
// `RetentionEnabled: false` and only the orphan rule fires.
package aws

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// SnapshotCrossRefConfig parameterizes the cross-ref enricher pattern shared
// across snapshot resource types.
type SnapshotCrossRefConfig struct {
	// ParentShortName is the resource type whose ResourceCache entry the
	// helper scans for parents (e.g. "dbi", "dbc", "ec2").
	ParentShortName string

	// GetParentID extracts the parent identifier from a snapshot's RawStruct.
	// Returns ("", false) when the snapshot has no parent reference — the
	// helper skips the row in that case (no orphan, no retention).
	GetParentID func(snapRaw any) (string, bool)

	// GetCreatedAt extracts the snapshot creation time from RawStruct.
	// Returns (zero, false) if absent — past-retention rule skips this row.
	// Required when RetentionEnabled is true; may be nil otherwise.
	GetCreatedAt func(snapRaw any) (time.Time, bool)

	// GetSnapshotType extracts the snapshot type ("automated"/"manual"/etc).
	// Returns ("", false) if absent — past-retention rule skips this row.
	// Required when RetentionEnabled is true; may be nil otherwise.
	GetSnapshotType func(snapRaw any) (string, bool)

	// GetParentRetention extracts the BackupRetentionPeriod (in days) from
	// the parent's RawStruct. Returns (0, false) when absent or zero — the
	// past-retention rule will not fire on this row.
	// Required when RetentionEnabled is true; may be nil otherwise.
	GetParentRetention func(parentRaw any) (int32, bool)

	// OrphanPhrase is the §4 status phrase emitted when the parent is missing
	// from the loaded cache (and the cache is NOT truncated).
	OrphanPhrase string

	// ParentRowLabel is the FindingRow label used to cite the parent in the
	// detail-view Attention section (e.g. "Source DB" for dbi, "Source
	// Cluster" for dbc). It is reused for both the orphan citation row AND
	// the past-retention parent-cite row.
	ParentRowLabel string

	// RetentionPhrase formats the past-retention status phrase given days-over.
	// Example: func(d int) string { return fmt.Sprintf("automated, %dd past retention", d) }
	// Required when RetentionEnabled is true; may be nil otherwise.
	RetentionPhrase func(daysOver int) string

	// RetentionEnabled gates the past-retention rule. Set false for snapshot
	// types whose parent has no retention concept (e.g. future ebs-snap, where
	// ec2.Volume has no BackupRetentionPeriod). When false, only the orphan
	// rule fires; GetParentRetention/GetSnapshotType/GetCreatedAt/RetentionPhrase
	// may be nil.
	RetentionEnabled bool

	// Severity is the severity tier emitted on every FindingRow and on the
	// domain.Finding.Severity for this enricher's output. "!" for the
	// existing snapshot consumers (orphan + past-retention are operator-
	// actionable). Future consumers may use "~" for informational signals.
	// Internally mapped to domain.Severity via glyphToSeverity.
	Severity string

	// OrphanCode is the canonical FindingCode emitted when the orphan rule
	// fires (e.g. "dbi-snap.orphan"). Required.
	OrphanCode domain.FindingCode
	// PastRetentionCode is the canonical FindingCode emitted when the past-
	// retention rule fires (e.g. "dbi-snap.past-retention"). Required when
	// RetentionEnabled is true.
	PastRetentionCode domain.FindingCode
	// ShortName is the resource short name used to stamp the Source field
	// ("wave2:<short>") on every emitted Finding. Required.
	ShortName string

	// PublicAttr, when non-nil, reads one snapshot's share attributes from
	// AWS so the helper can report a snapshot shared with every account.
	// This is the one part of the helper that makes an API call; consumers
	// that leave it nil stay a zero-call cross-ref scan. The returned
	// attributes are normalized so a single parser answers for the rds and
	// docdb SDKs alike.
	PublicAttr func(ctx context.Context, clients *ServiceClients, snap resource.Resource) ([]snapshotAttribute, error)
	// PublicCode / PublicPhrase describe the finding emitted when PublicAttr
	// reports a restore grant to the "all" group. Required when PublicAttr is
	// non-nil.
	PublicCode   domain.FindingCode
	PublicPhrase string
}

// snapshotAttribute is one snapshot share attribute, normalized away from the
// rds / docdb SDK types that carry the same two fields under different names.
type snapshotAttribute struct {
	Name   string
	Values []string
}

// restoreSharedWithAll reports whether attrs grants restore to the "all"
// group — AWS's spelling of "every AWS account on earth". This is the single
// parser behind both the dbi-snap and dbc-snap public checks.
func restoreSharedWithAll(attrs []snapshotAttribute) bool {
	for _, a := range attrs {
		if !strings.EqualFold(a.Name, "restore") {
			continue
		}
		if slices.ContainsFunc(a.Values, func(v string) bool { return strings.EqualFold(v, "all") }) {
			return true
		}
	}
	return false
}

// EnrichSnapshotCrossRef returns an IssueEnricherFunc that applies the
// orphan + past-retention pattern parameterized by cfg. The returned closure
// is wired into the catalog.ResourceTypeDef.Wave2 field of the snapshot type
// (catalog_databases.go entries for dbi-snap, dbc-snap, etc.).
//
// Contract: zero API calls, idempotent on repeated runs (Findings overwrites
// per resource ID).
func EnrichSnapshotCrossRef(cfg SnapshotCrossRefConfig) IssueEnricherFunc {
	return func(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
		// Default severity to "!" (operator-actionable) when callers omit the field.
		severity := cfg.Severity
		if severity == "" {
			severity = "!"
		}

		result := IssueEnricherResult{
			Findings:         make(map[string][]domain.Finding),
			AttentionDetails: make(map[string]map[domain.FindingCode]domain.AttentionDetail),
			TruncatedIDs:     make(map[string]bool),
			FieldUpdates:     make(map[string]map[string]string),
		}

		publicErr := enrichSnapshotPublicShare(ctx, cfg, clients, resources, &result)

		// Skip rule per spec §3.1: the cross-ref enricher requires the parent
		// list to be loaded. If absent, both rules silently skip.
		parentEntry, parentLoaded := cache[cfg.ParentShortName]
		if !parentLoaded {
			return result, publicErr
		}

		// Build a lookup map: parent ID → RawStruct for O(1) access.
		parentByID := make(map[string]any, len(parentEntry.Resources))
		for _, p := range parentEntry.Resources {
			parentByID[p.ID] = p.RawStruct
		}

		for _, res := range resources {
			parentID, ok := cfg.GetParentID(res.RawStruct)
			if !ok || parentID == "" {
				continue
			}

			parentRaw, parentFound := parentByID[parentID]

			var phrase string
			var code domain.FindingCode
			var rows []domain.DetailRow

			switch {
			case !parentFound:
				// Orphan rule: when the cache is truncated AND the parent
				// isn't in the visible window, absence is non-definitive —
				// skip rather than emit a false positive.
				if parentEntry.IsTruncated {
					continue
				}
				phrase = cfg.OrphanPhrase
				code = cfg.OrphanCode
				rows = append(rows, domain.DetailRow{
					Label: cfg.ParentRowLabel,
					Value: parentID + " (not in loaded list)",
					Tier:  severity,
				})
			case cfg.RetentionEnabled:
				// Past-retention rule: only for "automated" snapshots whose
				// parent has a positive BackupRetentionPeriod.
				snapType, hasType := cfg.GetSnapshotType(res.RawStruct)
				createdAt, hasCreated := cfg.GetCreatedAt(res.RawStruct)
				retention, hasRetention := cfg.GetParentRetention(parentRaw)
				if !hasType || !hasCreated || !hasRetention {
					continue
				}
				if snapType != "automated" || retention <= 0 {
					continue
				}
				ageD := int(time.Since(createdAt).Hours() / 24)
				retentionD := int(retention)
				if ageD <= retentionD {
					continue
				}
				overD := ageD - retentionD
				phrase = cfg.RetentionPhrase(overD)
				code = cfg.PastRetentionCode
				rows = append(rows, domain.DetailRow{
					Label: cfg.ParentRowLabel,
					Value: parentID,
					Tier:  severity,
				})
				rows = append(rows, domain.DetailRow{
					Label: "Retention",
					Value: fmt.Sprintf("%d days", retention),
					Tier:  severity,
				})
				rows = append(rows, domain.DetailRow{
					Label: "Created",
					Value: createdAt.Format("2006-01-02"),
					Tier:  severity,
				})
			default:
				// Parent found but retention rule disabled — nothing to emit.
				continue
			}

			if phrase == "" {
				continue
			}

			// setWave2Finding is the sole append-only builder for both Findings
			// and AttentionDetails (core/aws/issue_enrichment.go) — it
			// drives the detail-view Attention section AND the S4 status
			// column at render time via domain.StatusPhrase(r.Findings).
			setWave2Finding(&result, res.ID, code, phrase, severity, cfg.ShortName, rows)
		}

		return result, publicErr
	}
}

// enrichSnapshotPublicShare runs cfg.PublicAttr over the snapshots (cap
// EnrichmentCap) and emits the "shared with all AWS accounts" finding for
// each one whose restore attribute names the "all" group. A per-snapshot
// failure marks that snapshot skipped and never hides the others.
func enrichSnapshotPublicShare(
	ctx context.Context,
	cfg SnapshotCrossRefConfig,
	clients *ServiceClients,
	resources []resource.Resource,
	result *IssueEnricherResult,
) error {
	if cfg.PublicAttr == nil || clients == nil {
		return nil
	}
	resources = capAtEnrichmentCap(result, resources, resourceIDsOf)
	n := len(resources)
	if n < len(resources) {
		SetTruncated(result, true)
	}
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		res := resources[i]
		if res.ID == "" {
			return
		}
		attrs, err := cfg.PublicAttr(ctx, clients, res)
		mu.Lock()
		defer mu.Unlock()
		switch {
		case err != nil && IsNotFoundErr(err):
			// The snapshot went away between the list call and this one —
			// data incomplete for that row, not a failure to log.
			result.TruncatedIDs[res.ID] = true
			return
		case err != nil:
			MarkSkipped(result, res.ID, &failures, err)
			return
		}
		if !restoreSharedWithAll(attrs) {
			return
		}
		setWave2Finding(result, res.ID, cfg.PublicCode, cfg.PublicPhrase, "!", cfg.ShortName,
			[]domain.DetailRow{{Label: "Restore", Value: "all", Tier: "!"}})

	})
	return Finish(result, failures, n, cfg.ShortName+"-enrich: snapshot share attributes")
}

// glyphToSeverity maps a legacy "!" / "~" / "" severity glyph to the canonical
// domain.Severity. Used by snapshot_cross_ref.go (and any other enricher
// callsite that still parameterizes via glyph strings).
func glyphToSeverity(s string) domain.Severity {
	switch s {
	case "!":
		return domain.SevBroken
	case "~":
		return domain.SevWarn
	default:
		return domain.SevDim
	}
}
