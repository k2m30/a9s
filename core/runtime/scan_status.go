// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	"sort"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// ProbeOutcome classifies the result of a resource type's most recent
// Wave-1 availability probe, folded together with any Wave-2 enrichment
// probe that ran for the same type (see degradeForEnrichment).
type ProbeOutcome string

const (
	ProbeOK             ProbeOutcome = "ok"
	ProbePartial        ProbeOutcome = "partial"
	ProbeFailed         ProbeOutcome = "failed"
	ProbeSkippedNoRules ProbeOutcome = "skipped-no-rules"
)

// ProbeStatus is one resource type's most recent scan outcome, returned by
// Core.ScanStatus for hosts that surface per-type scan health (#462).
type ProbeStatus struct {
	ShortName string
	Outcome   ProbeOutcome
	// Duration is the availability probe's wall time, plus the enrichment
	// probe's wall time when one ran for this type — the two are summed,
	// not tracked separately, so this is total probe time for the type's
	// most recent scan, not either probe's individual duration.
	Duration time.Duration
	// Err is "throttled" | "timeout" | "access-denied" | "expired" | a raw
	// AWS error code, classified via classifyProbeErr. Empty for
	// ok/skipped-no-rules.
	Err string
	// At is the completion time of the most recent contributing probe —
	// the enrichment probe's completion time when one ran for this type
	// after availability, otherwise the availability probe's.
	At time.Time
}

// ScanStatus returns the most recent scan outcome for every resource type
// this session has probed, sorted by ShortName. Session.AllProbeStatus is
// lock-guarded, so unlike most Core accessors this is safe to call from a
// goroutine other than the one draining the handler loop (a host's own
// status-rendering goroutine, for example).
func (c *Core) ScanStatus() []ProbeStatus {
	records := c.session.AllProbeStatus()
	out := make([]ProbeStatus, 0, len(records))
	for shortName, rec := range records {
		out = append(out, ProbeStatus{
			ShortName: shortName,
			Outcome:   ProbeOutcome(rec.Outcome),
			Duration:  rec.Duration,
			Err:       rec.Err,
			At:        rec.At,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ShortName < out[j].ShortName })
	return out
}

// classifyProbeErr maps a probe error to the short machine-readable
// classification ProbeStatus.Err carries. The class table itself lives with
// the other AWS error classification in core/aws, so the cause a failure
// renders as (aws.CauseOf) and the class a probe records here can never drift
// apart; every surface that phrases the failure (ScanStatus.Err, the menu
// row's cause word, the account-wide title) reads the class, never the error
// text again.
func classifyProbeErr(err error) string {
	return awsclient.ErrClass(err)
}

// probeHasNoRules reports whether shortName has neither a declarative
// Wave-1 finding rule nor a registered Wave-2 issue enricher — the
// precondition for the "skipped-no-rules" outcome. Only meaningful when the
// probe itself did not fail; callers apply it after the failed/partial
// checks (a hard-failed probe on a no-rules type is still "failed").
func probeHasNoRules(c *Core, shortName string) bool {
	td := resource.FindResourceType(shortName)
	if td == nil {
		return false
	}
	return len(td.Findings) == 0 && !c.HasIssueEnricher(shortName)
}

// availabilityOutcome classifies a single Wave-1 AvailabilityChecked result
// per the #462 outcome-mapping contract: failed (error, no resources
// returned) beats partial (truncated, or error with resources present)
// beats skipped-no-rules (probe succeeded on a type with no findings rules
// and no Wave-2 enricher) beats ok.
func availabilityOutcome(c *Core, shortName string, resourcesReturned bool, truncated bool, err error) (ProbeOutcome, string) {
	switch {
	case err != nil && !resourcesReturned:
		return ProbeFailed, classifyProbeErr(err)
	case truncated || (err != nil && resourcesReturned):
		return ProbePartial, classifyProbeErr(err)
	case probeHasNoRules(c, shortName):
		return ProbeSkippedNoRules, ""
	default:
		return ProbeOK, ""
	}
}

// degradeForEnrichment folds a Wave-2 EnrichmentChecked result onto a
// type's existing outcome. Enrichment never turns a Wave-1 success into
// "failed" — the availability data is already on screen — so an
// enrichment error or truncation only ever downgrades ok/skipped-no-rules
// to "partial"; a Wave-1 "failed" outcome has nothing worse to degrade to
// and passes through unchanged, and "partial" stays "partial".
func degradeForEnrichment(prev ProbeOutcome, err error, truncated bool) ProbeOutcome {
	if prev == "" {
		prev = ProbeOK
	}
	if err == nil && !truncated {
		return prev
	}
	if prev == ProbeFailed {
		return prev
	}
	return ProbePartial
}

// setProbeStatus is the shared write path both handleAvailabilityChecked
// and handleEnrichmentChecked use to update a type's scan-status record.
// outcome/duration/errClass are the AGGREGATE fields (ScanStatus's public
// shape); availOutcome/availDuration/availErr are the Wave-1 baseline this
// aggregate was computed from — handleAvailabilityChecked passes its own
// values for both (a fresh probe IS the new baseline), while
// handleEnrichmentChecked passes through the baseline it read from the PRIOR
// record, unchanged, so a later enrichment rerun folds from the same
// baseline rather than the previous rerun's aggregate (#462/#463 defect 1).
func (c *Core) setProbeStatus(shortName string, outcome ProbeOutcome, duration time.Duration, errClass string, at time.Time, availOutcome ProbeOutcome, availDuration time.Duration, availErr string) {
	c.session.SetProbeStatus(shortName, session.ProbeStatusRecord{
		Outcome:       string(outcome),
		Duration:      duration,
		Err:           errClass,
		At:            at,
		AvailOutcome:  string(availOutcome),
		AvailDuration: availDuration,
		AvailErr:      availErr,
	})
}
