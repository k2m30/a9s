// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// probes.go — platform-agnostic availability and Wave-2 enrichment probes.
//
// These are (c *Core) methods reading session state via c.session. The
// Bubble Tea adapter in internal/tui/probe_adapter.go wraps them in tea.Cmd
// closures for the TUI runtime.
//
// No Bubble Tea, Lipgloss, or Bubbles imports are permitted in this file.
package runtime

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ProbeAvailabilityResult carries the outcome of a single Wave-1 resource
// availability probe. Adapters convert this into a platform-specific message
// (e.g. messages.AvailabilityChecked for the Bubble Tea adapter).
type ProbeAvailabilityResult struct {
	ResourceType string
	HasResources bool
	Count        int
	Truncated    bool
	Issues       int
	Resources    []resource.Resource
	Err          error
}

// ProbeEnrichmentResult carries the outcome of a single Wave-2 issue
// enrichment probe. Adapters convert this into a platform-specific message
// (e.g. messages.EnrichmentChecked for the Bubble Tea adapter).
//
// Findings is keyed by Resource.ID and carries every independently-evaluated
// Wave-2 Finding per resource (IssueEnricherResult.Findings, unfiltered).
// AttentionDetails is keyed by Resource.ID then by the owning Finding's Code
// (IssueEnricherResult.AttentionDetails, unfiltered) — the fold layer
// (runtime.Core.applyEnrichment) reads it directly against the matching
// r.Findings entry when writing onto cached rows.
type ProbeEnrichmentResult struct {
	ResourceType     string
	Truncated        bool
	Findings         map[string][]domain.Finding
	AttentionDetails map[string]map[domain.FindingCode]domain.AttentionDetail
	FieldUpdates     map[string]map[string]string
	TruncatedIDs     map[string]bool
	Err              error
}

// DemoPrefetchResult carries the combined outcome of a synchronous demo
// prefetch of all registered resource types. Adapters convert this into
// a platform-specific message (e.g. messages.AvailabilityPrefetched for
// the Bubble Tea adapter).
type DemoPrefetchResult struct {
	Entries        map[string]int
	Truncated      map[string]bool
	IssueCounts    map[string]int
	IssueTruncated map[string]bool
	Resources      map[string][]resource.Resource
	Pagination     map[string]*resource.PaginationMeta
	// PrefetchErr aggregates HARD per-type failures (the type yielded no
	// rows) — surfaced as a blocking flash banner.
	PrefetchErr error
	// PrefetchSoftErr aggregates PARTIAL failures (the type still yielded
	// rows alongside a composite per-item error — the E5 contract, e.g. a
	// details-denied environment). Recorded in the `!` error log only; the
	// rows already carry their degraded-state findings on screen.
	PrefetchSoftErr error
}

// LoadAvailabilityCache loads (or reuses the already-loaded) per-type disk
// cache for the CURRENT session pair via EnsureCacheStore and returns it
// converted to the counts-only *cache.File-equivalent shape callers expect:
// a map of per-type TypeFile snapshots. Returns the Store directly —
// callers read it via (*cache.Store).Type/Types.
//
// When session.Region is still unresolved ("", e.g. cold boot with no -r
// flag before the AWS connect settles it), the profile's default region is
// resolved synchronously from the local AWS config file so the disk seed
// does not have to wait on a live connection (C1: cached data renders
// before any AWS activity). This mirrors the resolution
// handleClientsReadySuccess performs post-connect. The resolved region is
// passed directly to Session.EnsureCacheStoreForRegion and is NOT written
// back to c.session.Region — connect owns that field, and if it resolves a
// different region the pair-stamped Store self-corrects on the next call.
//
// This method is itself dispatched as a tea.Cmd (background goroutine), so
// its own Profile/Region read goes through the pairMu-guarded
// EnsureCacheStoreForRegion rather than reading c.session.Region/Profile
// directly — the same cross-goroutine hazard EnsureCacheStore/
// WithCacheStoreSave/ReadCacheStore close (see Session.pairMu's doc comment).
func (c *Core) LoadAvailabilityCache() *cache.Store {
	if c.session.NoCache {
		return nil
	}
	profile, region := c.session.CurrentPair()
	if region == "" {
		region = awsclient.GetDefaultRegion(awsclient.DefaultConfigPath(), profile)
	}
	return c.session.EnsureCacheStoreForRegion(region)
}

// reconcileTypeFile is the SINGLE chokepoint every type-file write goes
// through (task #17 wave 1 — the row-store unification save chokepoint).
// SaveAvailabilityCache (counts-only) and SaveResourceListCache/
// saveProbeResourcesToTypeFiles (rows-carrying) both stage their observation
// through this function before store.Put — no other code path may construct
// a cache.TypeFile to persist. Replaces the previously scattered no-shrink/
// exact-stick/mismatch-drop guards that lived independently in each caller
// and could disagree about which write lane's Rows should survive.
//
// Persisted-pair invariant: a
// pair may only ever show Count > len(Rows) in the COUNTS-ONLY shape (rule
// 2 below) — and it never loses rows it already had. A write never reduces
// row richness merely to make Count and len(Rows) match; C1/goal 3 (a
// stale-marked answer beats an empty screen) outranks that cosmetic
// consistency. D14's exact-shrink-drops-rows regression is the shape this
// chokepoint forbids going forward.
//
// incoming.Rows == nil (rowsProvided=false) marks a counts-only observation
// (SaveAvailabilityCache). incoming.Rows != nil, including an empty
// non-nil slice, marks a rows-carrying observation (SaveResourceListCache /
// saveProbeResourcesToTypeFiles) — such a caller always passes a real
// (possibly zero-length) slice, never nil, so the nil check alone
// distinguishes the two lanes.
//
// rawTruncated/rawCount carry the CURRENT fetch's own observation exactly as
// it came off the wire, BEFORE any caller-side "exactness sticks" adjustment
// — reconcileTypeFile needs the untouched values to detect rule 0 below; a
// caller that pre-collapses a truncated observation into a sticky-exact one
// (as both SaveAvailabilityCache and SaveResourceListCache used to do
// unconditionally) destroys the very information this chokepoint needs to
// tell a genuine still-exact re-observation apart from a poisoned one.
//
// Rules, applied in order:
//
//  0. Contradiction (false-exact self-heal, D14/D7): existing.Exact is stored true,
//     but the CURRENT observation is itself truncated (rawTruncated) AND its
//     own accumulated depth already reaches or exceeds the stored exact
//     count (rawCount >= existing.Count). A truncated fetch cannot, by
//     construction, have exhausted a list that is genuinely done at
//     existing.Count — either a continuation token still exists past that
//     depth, or the verify-depth walk (D7) reached the stored-exact depth
//     and AWS still reports more. The stored Exact was therefore never true
//     for the CURRENT population (a shrink/growth since it was set, or it
//     was poisoned by an old build's false-exact bug) — live contradiction
//     beats a stored claim, so Exact is dropped and the deeper truncated
//     observation's own count/rows become the new (lower-bound) truth. This
//     is the one case where a truncated observation is allowed to REGRESS a
//     stored Exact — every other rule below assumes exactness, once true,
//     only advances (C5), which is the assumption rule 0 exists to correct
//     when it demonstrably no longer holds.
//  1. Rows-carrying, incoming shallower than existing AND incoming's IDs are
//     a subset of existing's (a shallower page of the same list): existing
//     Rows are kept in full — a shallower observation never regresses a
//     deeper one. Count/Exact still advance per C5 (a new EXACT count wins
//     even if its row set is thinner; only a fresh EXACT observation
//     replaces a stored EXACT).
//  2. Counts-only (incoming.Rows == nil): Rows are NEVER touched — existing
//     Rows (if any) are carried forward untouched regardless of whether
//     Count now disagrees with len(Rows). The pair is reconstructable: the
//     row set is the last-known page(s), Count is the authoritative total,
//     and the renderer already treats Rows as stale-until-verified (C1).
//     Only Count/Exact/HasResources/Issues* are written.
//  3. Rows-carrying, incoming has MORE rows than existing: incoming's rows
//     always win (deeper knowledge) — but see the C6b carry note below.
//  4. Rows-carrying, same depth but different content (non-subset — a
//     genuine refresh): incoming wins by recency — but see the C6b carry
//     note below.
//
// C6b Wave-2 carry: rules 3 and 4 let incoming's rows replace existing's
// wholesale, which — for a bare Wave-1 rows-carrying observation (e.g. the
// sweep-completion save) — would silently drop any Wave-2-sourced Findings
// and registered enricher Fields a prior enrichment pass wrote onto
// existing's rows (D17). Unless wave2Authoritative is true (this write IS
// the Wave-2-completion save for shortName, which must supersede carried
// data so healed/resolved issues clear), every row incoming replaces under
// rules 3/4 carries forward its existing counterpart's Wave-2 Findings and
// shortName's IssueEnricherFieldKeys when the incoming row itself has no
// Wave-2 Finding of its own. Wave-1 findings never carry (see
// carryWave2ForRows).
//
// reconcileInput groups the rule-context parameters reconcileTypeFile needs
// beyond the two TypeFile snapshots (existing stays a separate positional
// argument since it is the thing being reconciled AGAINST, not a rule
// input).
type reconcileInput struct {
	Incoming           cache.TypeFile
	RowsProvided       bool
	RawTruncated       bool
	RawCount           int
	ShortName          string
	Wave2Authoritative bool
}

func reconcileTypeFile(existing cache.TypeFile, in reconcileInput) cache.TypeFile {
	incoming := in.Incoming
	if existing.Exact && in.RawTruncated && in.RawCount >= existing.Count && existing.Count > 0 {
		// Rule 0: the stored Exact is provably false — accept the poisoned
		// pair's self-healing observation instead of letting it re-stick.
		incoming.Exact = false
		if in.RawCount > incoming.Count {
			incoming.Count = in.RawCount
		}
	}

	tf := incoming
	tf.HasResources = incoming.Count > 0

	if !in.RowsProvided {
		// Rule 2: counts-only write. Rows are never inspected or dropped.
		tf.Rows = existing.Rows
		if existing.Count > 0 || len(existing.Rows) > 0 {
			tf.HasResources = tf.HasResources || existing.HasResources
		}
		return tf
	}

	switch {
	case len(incoming.Rows) < len(existing.Rows) && rowIDsAreSubset(incoming.Rows, existing.Rows):
		// Rule 1: shallower page of the same list — keep the deeper rows.
		tf.Rows = existing.Rows
		if !incoming.Exact && existing.Count > tf.Count {
			tf.Count = existing.Count
		}
		tf.HasResources = tf.Count > 0 || len(tf.Rows) > 0
	default:
		// Rules 3 & 4: incoming has more rows, or same/differing depth with
		// non-subset content (a genuine refresh) — incoming's rows win,
		// carrying forward any Wave-2 data (C6b) the replaced rows have that
		// incoming itself lacks, unless this write is itself the
		// Wave-2-completion save (which must supersede carried data wholesale
		// so healed/resolved issues clear).
		if !in.Wave2Authoritative {
			tf.Rows = carryWave2ForRows(existing.Rows, tf.Rows, issueEnricherFieldKeysFor(in.ShortName))
		}
	}
	return tf
}

// rowIDsAreSubset reports whether every ID in candidate also appears in
// superset — used by reconcileTypeFile's rule 1 to detect "a shallower page
// of the same list" (e.g. a truncated first-page refetch over an
// already-exact, fuller stored list) as opposed to a genuine content change
// that merely happens to be no longer.
func rowIDsAreSubset(candidate, superset []cache.Row) bool {
	if len(candidate) == 0 {
		return true
	}
	known := make(map[string]struct{}, len(superset))
	for _, r := range superset {
		known[r.ID] = struct{}{}
	}
	for _, r := range candidate {
		if _, ok := known[r.ID]; !ok {
			return false
		}
	}
	return true
}

// SaveAvailabilityCache persists the supplied availability state to disk, one
// type file per resource type (C7: per-type files, no merge logic). Returns
// nil immediately when entries is nil or caching is disabled (NoCache). The
// per-type read-modify-marshal sequence runs inside WithCacheStoreSave (the
// store-lock serialization, D13) so the in-memory half can never interleave
// with a concurrent SaveResourceListCache/SaveAvailabilityCache call for the
// same type file dispatched from another tea.Cmd goroutine (e.g. a
// background availability sweep's save racing a list screen's own
// fetch-completion save) — that race previously let one call's Count and
// another's Rows land in the same on-disk TypeFile as a mismatched pair,
// even though each call's own write was individually consistent. The actual
// disk write happens after WithCacheStoreSave releases pairMu — see its doc
// comment for what changed and the trade-off that split accepts.
// WithCacheStoreSave also covers the initial load (C7 hard invariant: a save
// can never precede that pair's own load).
//
// Row/Findings persistence (C6, all loaded pages) is intentionally NOT done
// here — this method only carries the counts-only availability-probe shape
// callers historically populated it with. Full-row persistence for a type's
// canonical top-level list happens via Core.SaveResourceListCache, called
// from the list-fetch-completion seam (applyResourcesLoaded). Every write
// this method stages goes through reconcileTypeFile (rule 2: a counts-only
// write never touches existing Rows).
func (c *Core) SaveAvailabilityCache(
	entries map[string]int,
	truncated map[string]bool,
	issueCounts map[string]int,
	issueTruncated map[string]bool,
	issueKnown map[string]bool,
) error {
	if entries == nil {
		return nil
	}
	return c.WithCacheStoreSave(func(store *cache.Store) ([]cache.WritePlan, error) {
		if store == nil {
			return nil, nil
		}
		var firstErr error
		var plans []cache.WritePlan
		for rawName, count := range entries {
			name := canonShortName(rawName)
			trunc := false
			if truncated != nil {
				trunc = truncated[rawName]
			}
			existing, hadExisting := store.Type(name)
			incoming := cache.TypeFile{
				Count: count,
				// C5: a truncated first-page probe never downgrades a stored
				// exact total — only replace Exact when this observation is
				// itself untruncated (a genuine exact observation).
				// reconcileTypeFile's rule 0 overrides this stickiness when
				// the raw observation (trunc, count) itself CONTRADICTS the
				// stored exactness (a self-heal for a poisoned pair — see
				// rule 0's doc comment).
				Exact: existing.Exact || !trunc,
			}
			if existing.Exact && trunc && existing.Count > count {
				// Preserve the previously-observed exact count rather than
				// letting a smaller truncated lower-bound regress it. Rule 0
				// overrides this too when the contradiction condition holds.
				incoming.Count = existing.Count
			}
			tf := reconcileTypeFile(existing, reconcileInput{
				Incoming:     incoming,
				RawTruncated: trunc,
				RawCount:     count,
				ShortName:    name,
			})
			if issueKnown[rawName] {
				tf.Issues = issueCounts[rawName]
				tf.IssuesKnown = true
				tf.IssuesTruncated = issueTruncated[rawName]
			} else {
				tf.Issues = existing.Issues
				tf.IssuesKnown = existing.IssuesKnown
				tf.IssuesTruncated = existing.IssuesTruncated
			}
			// Rule 2 (counts-only write) never touches Rows, so scalar
			// equality against the prior on-disk entry means the file
			// content is logically identical — skip the write rather than
			// allocating a fresh inode for no observable change.
			if hadExisting &&
				tf.Count == existing.Count &&
				tf.Exact == existing.Exact &&
				tf.HasResources == existing.HasResources &&
				tf.Issues == existing.Issues &&
				tf.IssuesKnown == existing.IssuesKnown &&
				tf.IssuesTruncated == existing.IssuesTruncated {
				continue
			}
			store.Put(name, tf)
			wp, err := store.PrepareSave(name)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			plans = append(plans, wp)
		}
		return plans, firstErr
	})
}

// materializeListFieldsForSave resolves shortName's list column set (via
// Core.saveColumns when the renderer registered one, else the built-in-
// defaults-only cascade in resolveSaveColumns) and, for every Path-backed
// column whose Fields entry is still empty, extracts the scalar from
// RawStruct and writes it into Fields under the column's resolved key (Key
// when set, else the lowercased Title). Owner decision: "для всех
// ресурсов должны быть закешированы все колонки, которые могут меняться" —
// every renderable list column must be cached, driven by the column CONFIG,
// no hardcode. Shared by both save lanes via SaveTypeRows (task #17 wave 1
// stage 4: one materializer for the list-open lane and the sweep lane).
//
// Unlike the render-time app.MaterializeListFields (which the render path
// uses and intentionally skips every Key-based column, since a live
// Fields-map lookup or a Wave-2 override already covers them while RawStruct
// is still present), this SAVE-seam pass also materializes a Path-backed
// column that additionally declares a Key: cache.Row never carries RawStruct,
// so a Key-based column whose value has so far only ever come from a
// RawStruct fallback needs the exact same one-time materialization a pure
// Path-only column needs, or it goes blank once the row is replayed from
// disk. The status/lifecycle column is always excluded regardless of Path —
// its cell is derived at render time from Findings, so persisting a
// RawStruct-derived value for it would be actively wrong once Findings
// disagree (mirrors app.materializeAllPathFields's exclusion).
//
// A resource whose RawStruct is nil (e.g. a cache-replay round-trip) passes
// through unchanged.
func (c *Core) materializeListFieldsForSave(shortName string, resources []resource.Resource) []resource.Resource {
	if len(resources) == 0 {
		return resources
	}
	var columns []config.ListColumn
	if c.saveColumns != nil {
		columns = c.saveColumns(shortName)
	} else {
		columns = resolveSaveColumns(shortName)
	}
	if len(columns) == 0 {
		return resources
	}
	lifecycleKey := "state"
	if td := resource.FindResourceType(shortName); td != nil && td.LifecycleKey != "" {
		lifecycleKey = td.LifecycleKey
	}
	out := make([]resource.Resource, len(resources))
	for i, r := range resources {
		out[i] = materializeResourceFields(r, columns, lifecycleKey)
	}
	return out
}

// resolveSaveColumns delegates to the shared resource.ResolveListColumnCascade
// with vc=nil — this fallback lane never sees the session's SetViewConfig
// override (see SaveTypeRows' SetSaveColumns comment).
// Used as SaveTypeRows' fallback when no renderer has called SetSaveColumns
// (e.g. a bare Core built directly in a runtime-package test).
func resolveSaveColumns(shortName string) []config.ListColumn {
	td := resource.FindResourceType(shortName)
	return resource.ResolveListColumnCascade(nil, shortName, td)
}

// materializeResourceFields is the single-resource core of
// materializeListFieldsForSave: every Path-backed column (Key-based or not,
// excluding the status/lifecycle column) whose resolved Fields key is not
// already populated gets its RawStruct scalar written in.
func materializeResourceFields(r resource.Resource, columns []config.ListColumn, lifecycleKey string) resource.Resource {
	if r.RawStruct == nil {
		return r
	}
	out := r
	copied := false
	for _, col := range columns {
		if col.Path == "" {
			continue
		}
		if col.Key == "status" || col.Key == lifecycleKey {
			continue
		}
		key := col.Key
		if key == "" {
			key = strings.ToLower(col.Title)
		}
		if key == "" {
			continue
		}
		if v, ok := out.Fields[key]; ok && v != "" {
			continue
		}
		val := fieldpath.ExtractScalar(out.RawStruct, col.Path)
		if val == "" {
			continue
		}
		if !copied {
			fresh := make(map[string]string, len(out.Fields)+1)
			maps.Copy(fresh, out.Fields)
			out.Fields = fresh
			copied = true
		}
		out.Fields[key] = val
	}
	return out
}

// SaveResourceListCache persists rows for one resource type's canonical
// top-level, unfiltered list (C6): every loaded page's rows (ID/Name/Fields/
// Findings — colors/glyphs/status are derived at render time and never
// persisted), the current count, and the exact flag. Callers are responsible
// for the C6 scope gate (only calling this for a top-level unfiltered list,
// never a child/related/filtered view).
//
// The read-modify-marshal sequence runs inside WithCacheStoreSave (the
// store-lock serialization, D13) — see SaveAvailabilityCache's doc comment
// for why obtaining the store via EnsureCacheStore and mutating it
// afterward is not sufficient: that shape only serializes the pointer
// lookup, not the store.Type/Put/PrepareSave sequence, letting two
// concurrent saves for the same type file interleave. The disk write itself
// runs after WithCacheStoreSave releases pairMu — see its doc comment. rows
// is always passed as a real (possibly zero-length, never nil) slice so
// reconcileTypeFile's rows-carrying lane (rules 1/3/4) is the one that
// applies here.
//
// This is the list-open save lane (app.Controller.maybeSaveResourceListCache
// and the executor's per-type sweep loop) — never the Wave-2-completion save,
// so it always runs reconcileTypeFile with wave2Authoritative=false (C6b: a
// bare rows-carrying write here carries forward any Wave-2 data the replaced
// rows have that rows itself lacks). The Wave-2-completion save
// (handleEnrichmentChecked's "all done" branch, via the TaskKindSaveCache
// executor case) uses saveResourceListCacheWave2Complete instead, which is
// reached only through the executor's SaveCachePayload.Wave2Complete tag —
// never through this exported entry point.
func (c *Core) SaveResourceListCache(shortName string, rows []cache.Row, count int, exact bool, issues int, issuesKnown, issuesTruncated bool) error {
	return c.saveResourceListCache(shortName, rows, count, exact, issues, issuesKnown, issuesTruncated, false)
}

// saveResourceListCacheWave2Complete is saveResourceListCache's counterpart
// for the Wave-2-completion save (handleEnrichmentChecked's "all done"
// branch): this observation IS the fresh enrichment result, so it must
// supersede any carried Wave-2 data wholesale for the rows it replaces —
// otherwise a healed/resolved issue could never clear (C6b).
func (c *Core) saveResourceListCacheWave2Complete(shortName string, rows []cache.Row, count int, exact bool, issues int, issuesKnown, issuesTruncated bool) error {
	return c.saveResourceListCache(shortName, rows, count, exact, issues, issuesKnown, issuesTruncated, true)
}

// SaveTypeRows is the single per-type save chokepoint (task #17 wave 1 stage
// 4): materializes resources' Path-backed columns (via
// materializeListFieldsForSave, honoring a registered SetSaveColumns
// resolver), builds the persisted cache.Row projection (ID/Name/Fields/
// Findings — colors/glyphs/status are derived at render time and never
// persisted), and writes through saveResourceListCache/
// saveResourceListCacheWave2Complete depending on wave2Authoritative.
//
// Callers own the C6 scope gate (only a top-level, unfiltered list may call
// this) and the issue-count computation — the list-open lane
// (app.Controller.maybeSaveResourceListCache) and the sweep lane
// (saveProbeResourcesToTypeFiles) each aggregate issues from different
// inputs (per-screen enrichment-store findings vs. a bare Wave-1 probe
// snapshot) and must keep computing issues/issuesKnown/issuesTruncated
// themselves; unifying that computation here would silently change either
// lane's counted total.
func (c *Core) SaveTypeRows(shortName string, resources []resource.Resource, count int, exact bool, issues int, issuesKnown, issuesTruncated, wave2Authoritative bool) error {
	materialized := c.materializeListFieldsForSave(shortName, resources)
	rows := make([]cache.Row, len(materialized))
	for i, r := range materialized {
		rows[i] = cache.Row{
			ID:       r.ID,
			Name:     r.Name,
			Fields:   r.Fields,
			Findings: r.Findings,
		}
	}
	if wave2Authoritative {
		return c.saveResourceListCacheWave2Complete(shortName, rows, count, exact, issues, issuesKnown, issuesTruncated)
	}
	return c.saveResourceListCache(shortName, rows, count, exact, issues, issuesKnown, issuesTruncated, false)
}

func (c *Core) saveResourceListCache(shortName string, rows []cache.Row, count int, exact bool, issues int, issuesKnown, issuesTruncated, wave2Authoritative bool) error {
	// Canonicalize so an alias caller (e.g. "rds") and CachedListDepth's own
	// canonShortName lookup always agree on the stored key — an
	// uncanonicalized Put here would silently miss the depth lookup for
	// every alias caller.
	canon := canonShortName(shortName)
	if rows == nil {
		rows = []cache.Row{}
	}
	return c.WithCacheStoreSave(func(store *cache.Store) ([]cache.WritePlan, error) {
		if store == nil {
			return nil, nil
		}
		existing, _ := store.Type(canon)
		incoming := cache.TypeFile{
			Count: count,
			Exact: exact,
			Rows:  rows,
		}
		if !exact && existing.Exact {
			// C5: exactness only ever advances — a truncated observation
			// never downgrades an already-exact stored total. reconcileTypeFile's
			// rule 0 overrides this stickiness when the raw (exact, count)
			// observation itself contradicts the stored exactness — e.g. the
			// verify-depth walk (D7) reaching the stored-exact depth while
			// AWS still reports truncation (a self-heal for a poisoned pair).
			incoming.Exact = true
			incoming.Count = existing.Count
		}
		tf := reconcileTypeFile(existing, reconcileInput{
			Incoming:           incoming,
			RowsProvided:       true,
			RawTruncated:       !exact,
			RawCount:           count,
			ShortName:          canon,
			Wave2Authoritative: wave2Authoritative,
		})
		// #463: FirstSeen diff runs unconditionally, after reconcileTypeFile
		// (including any Wave-2 carry it performed), against the pre-save
		// on-disk generation (existing.Rows) — the single chokepoint both
		// SaveResourceListCache and saveResourceListCacheWave2Complete share.
		// #463 defect 2: a Wave-2-completion save diffs against the
		// just-written Wave-1 generation, so its own newPairs only ever
		// covers Wave-2-sourced codes — REPLACING the type's delta here
		// would wipe the Wave-1 new-pair counts that same sweep's earlier,
		// non-authoritative save just recorded. Non-authoritative saves keep
		// REPLACE semantics (each is a fresh one-step scan baseline);
		// wave2Authoritative saves MERGE onto whatever the sweep's Wave-1
		// save already recorded this cycle. Cannot double-count: a pair
		// stamps fresh at most once per cycle (stampFindingFirstSeen's own
		// old-vs-new diff), so the two saves' newPairs sets are disjoint.
		var newPairs map[domain.FindingCode]int
		tf.Rows, newPairs = stampFindingFirstSeen(existing.Rows, tf.Rows, time.Now())
		if wave2Authoritative {
			c.session.MergeNewFindingPairs(canon, newPairs)
		} else {
			c.session.SetNewFindingPairs(canon, newPairs)
		}
		if issuesKnown {
			tf.Issues = issues
			tf.IssuesKnown = true
			tf.IssuesTruncated = issuesTruncated
		} else {
			tf.Issues = existing.Issues
			tf.IssuesKnown = existing.IssuesKnown
			tf.IssuesTruncated = existing.IssuesTruncated
		}
		store.Put(canon, tf)
		wp, err := store.PrepareSave(canon)
		if err != nil {
			return nil, err
		}
		return []cache.WritePlan{wp}, nil
	})
}

// CachedListDepth returns the number of rows previously persisted for
// shortName's canonical top-level list, so a background verify-refetch
// (KindFetchResources) can be bounded to at most the depth already shown to
// the user (C1: re-verify must verify the content being shown, not just page
// 1; C5: a truncated first-page fetch must never downgrade a stored exact
// total — paginating up to the prior depth keeps the refetch from silently
// shrinking a wider cached list back to a single page). Returns 0 when
// caching is disabled or no stored rows exist for shortName, in which case
// callers fall back to the un-paginated first-page result.
func (c *Core) CachedListDepth(shortName string) int {
	canon := canonShortName(shortName)
	depth := 0
	_ = c.ReadCacheStore(func(store *cache.Store) error {
		if store == nil {
			return nil
		}
		if tf, ok := store.Type(canon); ok {
			depth = len(tf.Rows)
		}
		return nil
	})
	return depth
}

// ProbeResourceAvailability calls the registered paginated fetcher for
// shortName with a 10-second timeout, applies Wave-1 issue counting, and
// returns the result. Adapters wrap this in platform-specific async
// machinery (e.g. a tea.Cmd for the Bubble Tea adapter).
//
// Paginated fetchers are tried so that truncation can be detected and
// reported as "(N+)" in the main menu.
func (c *Core) ProbeResourceAvailability(ctx context.Context, clients *awsclient.ServiceClients, shortName string) ProbeAvailabilityResult {
	if clients == nil {
		return ProbeAvailabilityResult{
			ResourceType: shortName,
			Err:          fmt.Errorf("AWS clients not initialized"),
		}
	}
	// A registered AvailabilityFetcher (core/resource.GetAvailabilityFetcher)
	// is a cheaper probe-only alternative for types whose real list content
	// is materially more expensive to resolve than an availability/count
	// signal needs (e.g. "policy" skips IAM's per-group inline-policy
	// sweep) — every other type falls back to its ordinary paginated
	// fetcher, unchanged.
	pf := resource.GetAvailabilityFetcher(shortName)
	if pf == nil {
		pf = resource.GetPaginatedFetcher(shortName)
	}
	if pf == nil {
		return ProbeAvailabilityResult{
			ResourceType: shortName,
			Err:          fmt.Errorf("no fetcher for %s", shortName),
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := awsclient.RetryOnThrottle(probeCtx, awsclient.DefaultRetryConfig(), func() (resource.FetchResult, error) {
		return pf(probeCtx, clients, "")
	})

	truncated := result.Pagination != nil && result.Pagination.IsTruncated
	// Count issue-status resources (red/yellow only, not green/dim).
	issues := 0
	td := resource.FindResourceType(shortName)
	for _, r := range result.Resources {
		if td != nil && !td.ExcludeFromIssueBadge && td.ResolveColor(r).IsIssue() {
			issues++
		}
	}

	if err != nil {
		if len(result.Resources) == 0 {
			// Full failure: no resources recovered — treat as unknown.
			return ProbeAvailabilityResult{
				ResourceType: shortName,
				Err:          err,
			}
		}
		// Partial success: some resources returned alongside an error.
		// Surface both so the menu shows what was found and the flash log
		// records the failure (never-silent-skip contract).
		return ProbeAvailabilityResult{
			ResourceType: shortName,
			HasResources: true,
			Count:        len(result.Resources),
			Truncated:    truncated,
			Issues:       issues,
			Resources:    result.Resources,
			Err:          err,
		}
	}
	return ProbeAvailabilityResult{
		ResourceType: shortName,
		HasResources: len(result.Resources) > 0,
		Count:        len(result.Resources),
		Truncated:    truncated,
		Issues:       issues,
		Resources:    result.Resources,
	}
}

// DemoPrefetchCounts synchronously calls all registered paginated fetchers
// and returns a combined result. Used when pre-supplied clients are present
// and no-cache is active so the main menu shows counts immediately without
// the async probe pipeline.
func (c *Core) DemoPrefetchCounts(ctx context.Context, clients *awsclient.ServiceClients) DemoPrefetchResult {
	allNames := resource.AllShortNames()
	entries := make(map[string]int, len(allNames))
	truncated := make(map[string]bool)
	issueCounts := make(map[string]int, len(allNames))
	issueTruncated := make(map[string]bool)
	retainedResources := make(map[string][]resource.Resource, len(allNames))
	pagination := make(map[string]*resource.PaginationMeta, len(allNames))
	var failures []string
	var softFailures []string
	attempted := 0

	for _, shortName := range allNames {
		// Stop early if the app context is done (shutdown or profile/region switch).
		if ctx.Err() != nil {
			break
		}
		pf := resource.GetPaginatedFetcher(shortName)
		if pf == nil {
			continue
		}
		attempted++
		perFetchCtx, perFetchCancel := context.WithTimeout(ctx, 5*time.Second)
		result, err := pf(perFetchCtx, clients, "")
		perFetchCancel()
		// Partial-success: a per-item composite error MAY accompany a non-empty
		// result. Hard failure (no resources) → skip and record for the flash
		// banner. Soft failure (some resources) → record for the error log AND
		// count the resources so the main menu badge isn't blanked by a single
		// per-item failure.
		if err != nil {
			if len(result.Resources) == 0 {
				if awsclient.IsEndpointNotFound(err) {
					// Region gap: the service endpoint's DNS does not resolve
					// — the service is not offered here. Plain language,
					// log-only (the operator can't fix DNS jargon).
					_, region := c.session.CurrentPair()
					softFailures = append(softFailures, fmt.Sprintf("%s: service not available in region %s", shortName, region))
					continue
				}
				failures = append(failures, fmt.Sprintf("%s: %v", shortName, err))
				continue
			}
			softFailures = append(softFailures, fmt.Sprintf("%s: %v", shortName, err))
		}
		entries[shortName] = len(result.Resources)
		// Preserve full pagination meta so the seeded ResourceCache entry's
		// pagination state is authoritative — a later load-more or navigate
		// must be able to advance past page 1.
		if result.Pagination != nil {
			pagination[shortName] = result.Pagination
		}
		isTrunc := result.Pagination != nil && result.Pagination.IsTruncated
		if isTrunc {
			truncated[shortName] = true
			issueTruncated[shortName] = true
		}
		// Count issue-status resources (red/yellow only).
		issues := 0
		td := resource.FindResourceType(shortName)
		for _, r := range result.Resources {
			if td != nil && !td.ExcludeFromIssueBadge && td.ResolveColor(r).IsIssue() {
				issues++
			}
		}
		issueCounts[shortName] = issues
		// Retain first-page resources for Wave 2 enricher consumption.
		retainedResources[shortName] = result.Resources
	}

	return DemoPrefetchResult{
		Entries:         entries,
		Truncated:       truncated,
		IssueCounts:     issueCounts,
		IssueTruncated:  issueTruncated,
		Resources:       retainedResources,
		Pagination:      pagination,
		PrefetchErr:     awsclient.AggregateFailures("availability-prefetch", failures, attempted),
		PrefetchSoftErr: awsclient.AggregateFailures("availability-prefetch (partial)", softFailures, attempted),
	}
}

// buildEnrichQueue returns resource types that have a registered Wave-2 issue
// enricher AND retained probe resources, in dispatch order. Dispatch order
// (priority ascending, then alphabetical) is owned by awsclient.AllWave2 so
// this function only filters by RowStore membership (task #17 wave 1 stage
// 2 — replaces the removed session.ProbeResources membership check).
//
// Deliberately uses tr.Gen != 0 (observed-at-all), NOT ProbeOriginTypeNames'
// len(Rows)>0 gate: the legacy session.ProbeResources map-key-presence check
// this replaces (`_, ok := c.session.ProbeResources[e.ShortName]`) was true
// even for an explicitly-retained, observed-EMPTY slice (a live Wave-1 probe
// confirming zero resources still ran that type's Wave-2 enricher). Reusing
// ProbeOriginTypeNames here would silently skip Wave-2 enrichment for every
// observed-empty type, a real behavior regression this membership test must
// not introduce.
//
// tr.Partial additionally excludes a type that has ONLY ever received a
// lazy related-add (ObservePartial) and no canonical Observe/ObserveCount
// (C6 scope boundary): ObservePartial always bumps Gen even for a
// never-canonically-observed type, so the Gen!=0 check alone is not
// sufficient to keep a lazy-add-only type out of the Wave-2 queue.
func (c *Core) BuildEnrichQueue() []string {
	all := awsclient.AllWave2()
	queue := make([]string, 0, len(all))
	for _, e := range all {
		tr := c.session.RowStore.Snapshot(e.ShortName)
		if tr.Gen == 0 || tr.Partial {
			continue
		}
		queue = append(queue, e.ShortName)
	}
	return queue
}

// ProbeEnrichment runs the registered Wave-2 enricher for shortName and
// returns the enrichment result. The typeGen captured from
// c.session.EnrichmentTypeGen at call time is the caller's responsibility;
// the caller embeds it in the adapter message for stale-result rejection.
//
// Builds a ResourceCache snapshot via BuildResourceCacheSnapshot, backed
// entirely by RowStore (task #17 wave 1 stage 3 — a type's rows live in
// exactly one RowStore entry regardless of which lane wrote them: Wave-1
// probe, top-level fetch, or a sparse FetchByIDs drill). On the normal
// startup path no list has been opened yet, so building from full-only
// entries would leave the first enrichment pass blind to siblings the
// probe alone has retained.
// Regression pin: TestProbeEnrichment_CacheSnapshotMergesProbeResources.
func (c *Core) ProbeEnrichment(ctx context.Context, clients *awsclient.ServiceClients, shortName string) ProbeEnrichmentResult {
	if clients == nil {
		return ProbeEnrichmentResult{
			ResourceType: shortName,
			Err:          fmt.Errorf("AWS clients not initialized"),
		}
	}
	e, ok := awsclient.Wave2EnricherFor(shortName)
	if !ok {
		return ProbeEnrichmentResult{ResourceType: shortName}
	}
	resources, _ := c.ProbeResources(shortName)
	cacheSnap := c.BuildResourceCacheSnapshot()

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := awsclient.RetryOnThrottle(probeCtx, awsclient.DefaultRetryConfig(), func() (awsclient.IssueEnricherResult, error) {
		return e.Fn(probeCtx, clients, resources, cacheSnap)
	})
	// Always populate fields from result regardless of err. RetryOnThrottle
	// preserves partial result on non-retryable errors (partial-success
	// contract: never-silent-skip).
	return ProbeEnrichmentResult{
		ResourceType:     shortName,
		Truncated:        result.Truncated,
		Findings:         result.Findings,
		AttentionDetails: result.AttentionDetails,
		FieldUpdates:     result.FieldUpdates,
		TruncatedIDs:     result.TruncatedIDs,
		Err:              err,
	}
}

// BuildResourceCacheSnapshot returns a read-only snapshot of currently-loaded
// resource lists, keyed by resource short name, so enrichers see the full
// set (including out-of-scope entries pulled via FetchByIDs). Backed
// entirely by RowStore (task #17 wave 1 stage 3 — the former
// ResourceCache/LazyResourceCache maps are gone; a type's rows live in
// exactly one RowStore entry regardless of which lane wrote them, so there
// is no merge-precedence left to apply).
//
// A Partial (lazy-add) entry is marked IsTruncated=true because it is
// sparse (FetchByIDs, not a full first page); a full entry's own
// Pagination.IsTruncated carries through unchanged — first-page-only
// probe/disk rows are marked truncated so the orphan rule in cross-ref
// enrichers treats parent-not-found as "unknown, skip" rather than
// "definitively deleted" per spec §3.1.
//
// A type observed with a zero-length Rows slice (Gen != 0, e.g. a live
// checker's CachedPages write-back reporting a genuinely empty-but-truncated
// or empty-complete first page — issue #233) still gets a snapshot entry:
// only a never-observed type (Gen == 0) is skipped. Dropping an
// observed-empty entry here would make FetchRelatedTarget's `cache[target]`
// lookup miss and fall through to its own live re-fetch, discarding the
// exact IsTruncated signal this method exists to carry — the #233
// regression this comment documents against reintroduction.
func (c *Core) BuildResourceCacheSnapshot() resource.ResourceCache {
	rowStoreAll := c.session.RowStore.SnapshotAll(true)
	snap := make(resource.ResourceCache, len(rowStoreAll))
	for shortName, tr := range rowStoreAll {
		if tr.Gen == 0 {
			continue
		}
		isTruncated := tr.Partial || (tr.Pagination != nil && tr.Pagination.IsTruncated)
		snap[shortName] = resource.ResourceCacheEntry{
			Resources:   tr.Rows,
			IsTruncated: isTruncated,
		}
	}
	return snap
}
