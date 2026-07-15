// qa_status_column_uniformity_test.go — the standing OWNER RULE gate: every
// registered top-level list view must present exactly ONE status column,
// uniformly titled "Status", whose cell the shared pipeline in
// core/app/list_columns.go (listExtractCellValue's isStatusCol cascade)
// renders. Per-type duplicates — ng/eks "Issues"/"Health Issues"
// (key=health_issues), Key-less columns titled "State" (rather than
// "Status"), sg-style "Risk" phrase columns, any "State Reason"-like cause
// columns — are architecture violations: the cause belongs in the Status
// cell and, in full, in the detail Attention block, not in a second
// standing column.
//
// RATCHET semantics (identical contract to knownVisibilityGaps /
// knownStateCoverageGaps / knownDisconnectedPivots in the sibling gates):
//   - A violation NOT in knownStatusColumnDebt is a NEW regression — always
//     fails, unconditionally.
//   - An allowlisted violation that NOW conforms (single Status-titled
//     status column, no duplicate cause column) fails with a "remove from
//     allowlist" message — the burn-down signal for the conversion coder.
//   - An allowlisted violation still violating is skipped (logged),
//     pre-existing debt this gate exists to track down.
//
// Two independent rules are checked per type, each with its own allowlist
// key suffix so a type can be pinned for one violation without masking the
// other:
//
//  1. "status-title": exactly one column must qualify as the status column
//     per a mirror of listExtractCellValue's own isStatusCol predicate
//     (Key=="status" OR Key==LifecycleKey OR (Key=="" AND Title
//     case-insensitively "status" or "state")), AND that column's Title
//     must be exactly "Status" (not "State" or any other casing/spelling).
//     Zero qualifying columns, more than one, or a qualifying column titled
//     anything but "Status" all count as a violation.
//
//  2. "duplicate-cause": no OTHER column (besides the one true status
//     column identified above) may carry problem/cause text — i.e. its key
//     or title case-insensitively matches "issues", "health_issues",
//     "risk", "state", "state reason", or a "status"-prefixed variant with
//     a numeric/alpha suffix (status2-style). This is checked independently
//     of rule 1 so a type with zero OR multiple status-qualifying columns
//     can still separately be flagged for a second duplicate cause column.
package unit_test

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
)

// knownStatusColumnDebt pins the exact inventory of (shortName, ruleKey)
// violations found by this gate at seeding time. Key shape:
// "<shortName>:<status-title|duplicate-cause>". Same burn-down semantics as
// knownVisibilityGaps in qa_issue_visibility_gate_test.go:
//   - present + still violating today   -> skip (logged), expected
//     pre-existing debt for the conversion coder to burn down.
//   - present + now conformant           -> FAIL ("remove from allowlist").
//   - a violation NOT present here       -> FAIL unconditionally, a new
//     regression the allowlist was never told about.
//
// SEEDED CENSUS (2026-07-07, first run of this gate): every entry originally
// pinned here was discovered by driving the REAL production
// column-resolution path (app.ResolveListColumns, which mirrors
// resolveColumns in table_render.go) against every registered top-level
// type's default view, back when the OWNER RULE required a status-qualifying
// column to be Key-less to have its Title checked. The single-Status-column
// conversion (list_columns.go's listExtractCellValue / resolveListStatusCol
// title-based cascade, applied regardless of Key) burned down every single
// pinned entry — all 36 status-title violations (State→Status column-title
// renames, and previously keyed columns like cb's "Last Status" that now
// route to the shared Status cell by Title) plus all 5 duplicate-cause
// violations (eks/ng's "Issues", elb's "State Reason", sg's/ssm's "Risk" —
// each folded into the Status cell's finding phrase). The allowlist is
// empty: any violation below is a NEW regression, not pre-existing debt.
var knownStatusColumnDebt = map[string]bool{}

// statusColumnUniformityRuleKeys enumerates the two independent violation
// suffixes this gate tracks, so both the seeded map and the live drive
// agree on the exact key shape.
const (
	ruleStatusTitle    = "status-title"
	ruleDuplicateCause = "duplicate-cause"
)

// duplicateCausePattern mirrors the OWNER RULE's enumerated duplicate/cause
// signal list: issues, health_issues, risk, state, state reason, and a
// status2-style numeric-suffixed variant of "status". Applied
// case-insensitively against both column Key and Title.
var duplicateCausePattern = regexp.MustCompile(`(?i)^(issues|health_issues|risk|state|state[_ ]?reason|status[0-9]+)$`)

// isStatusQualifyingColumn mirrors listExtractCellValue's isStatusCol
// predicate in core/app/list_columns.go byte-for-byte (Key=="status" OR
// Key==lifecycleKey OR Title case-insensitively "status" or "state",
// REGARDLESS of Key — the OWNER CONTRACT lets a keyed column like
// {Key:"last_status", Title:"Status"} qualify too), operating on the
// exported app.ColumnDef + the type's LifecycleKey (default "state" when
// unset, matching production's own fallback).
func isStatusQualifyingColumn(col app.ColumnDef, lifecycleKey string) bool {
	if lifecycleKey == "" {
		lifecycleKey = "state"
	}
	if col.Key == "status" || col.Key == lifecycleKey {
		return true
	}
	if strings.EqualFold(col.Title, "status") || strings.EqualFold(col.Title, "state") {
		return true
	}
	return false
}

// isDuplicateCauseColumn reports whether col's Key or Title
// case-insensitively matches the OWNER RULE's enumerated duplicate/cause
// signal set (issues, health_issues, risk, state, state reason,
// status2-style variants).
func isDuplicateCauseColumn(col app.ColumnDef) bool {
	if col.Key != "" && duplicateCausePattern.MatchString(col.Key) {
		return true
	}
	if col.Title != "" && duplicateCausePattern.MatchString(col.Title) {
		return true
	}
	return false
}

// statusColumnCensus reports, for one type's resolved column set, the
// indices of every status-qualifying column and the indices of every
// column (excluding the FIRST status-qualifying column, mirroring
// resolveListStatusCol's own first-match-wins cascade in
// core/app/list_columns.go) that separately matches the duplicate/cause
// pattern.
func statusColumnCensus(columns []app.ColumnDef, lifecycleKey string) (statusIdx []int, duplicateIdx []int) {
	for i, col := range columns {
		if isStatusQualifyingColumn(col, lifecycleKey) {
			statusIdx = append(statusIdx, i)
		}
	}
	firstStatus := -1
	if len(statusIdx) > 0 {
		firstStatus = statusIdx[0]
	}
	for i, col := range columns {
		if i == firstStatus {
			continue
		}
		if isDuplicateCauseColumn(col) {
			duplicateIdx = append(duplicateIdx, i)
		}
	}
	return statusIdx, duplicateIdx
}

// TestStatusColumnUniformityGate_ExactlyOneStatusColumnTitledStatus is the
// standing OWNER RULE gate. For every registered top-level type, resolves
// the DEFAULT list columns via app.ResolveListColumns (which mirrors
// resolveColumns in table_render.go — the same column set the TUI renders),
// then checks:
//
//  1. "status-title": exactly one column qualifies as the status column
//     (isStatusQualifyingColumn), and its Title is exactly "Status".
//  2. "duplicate-cause": no other column duplicates problem/cause text
//     (isDuplicateCauseColumn), checked against every column except the
//     first status-qualifying one.
//
// Each rule is independently allowlisted via knownStatusColumnDebt under
// "<shortName>:<ruleKey>" — see that map's doc comment for the ratchet
// semantics.
func TestStatusColumnUniformityGate_ExactlyOneStatusColumnTitledStatus(t *testing.T) {
	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var newlyRegressed []string
	var readyForBurnDown []string
	var stillGapped []string

	checkRule := func(t *testing.T, shortName, ruleKey string, violated bool, detail string) {
		t.Helper()
		key := shortName + ":" + ruleKey
		allowlisted := knownStatusColumnDebt[key]

		switch {
		case !violated && allowlisted:
			readyForBurnDown = append(readyForBurnDown, key)
			t.Errorf(
				"BURN-DOWN: %s (%s) no longer violates the status-column-uniformity rule "+
					"but is still pinned in knownStatusColumnDebt — remove %q from the allowlist in this PR",
				shortName, ruleKey, key,
			)
		case !violated:
			// Conformant and not allowlisted — expected steady state.
		case allowlisted:
			stillGapped = append(stillGapped, key)
			t.Skipf(
				"KNOWN DEBT (allowlisted): %s (%s): %s — pre-existing debt, see knownStatusColumnDebt",
				shortName, ruleKey, detail,
			)
		default:
			newlyRegressed = append(newlyRegressed, key)
			t.Errorf(
				"NEW VIOLATION (not allowlisted): %s (%s): %s. Either fix the column layout "+
					"or, if this is pre-existing debt, add %q to knownStatusColumnDebt",
				shortName, ruleKey, detail, key,
			)
		}
	}

	for _, td := range types {
		td := td
		columns := app.ResolveListColumns(td.ShortName)
		if len(columns) == 0 {
			continue
		}

		t.Run(td.ShortName, func(t *testing.T) {
			statusIdx, duplicateIdx := statusColumnCensus(columns, td.LifecycleKey)

			var titleViolation bool
			var titleDetail string
			switch len(statusIdx) {
			case 0:
				titleViolation = true
				titleDetail = "no column qualifies as the status column (no Key==\"status\", Key==LifecycleKey, or Key-less Title matching \"status\"/\"state\")"
			case 1:
				gotTitle := columns[statusIdx[0]].Title
				if gotTitle != "Status" {
					titleViolation = true
					titleDetail = fmt.Sprintf("the sole status-qualifying column (index %d) is titled %q, not \"Status\"", statusIdx[0], gotTitle)
				}
			default:
				titleViolation = true
				var titles []string
				for _, i := range statusIdx {
					titles = append(titles, fmt.Sprintf("%d:%q", i, columns[i].Title))
				}
				titleDetail = fmt.Sprintf("%d columns qualify as the status column simultaneously: %s", len(statusIdx), strings.Join(titles, ", "))
			}
			checkRule(t, td.ShortName, ruleStatusTitle, titleViolation, titleDetail)

			dupViolation := len(duplicateIdx) > 0
			var dupDetail string
			if dupViolation {
				var dups []string
				for _, i := range duplicateIdx {
					dups = append(dups, fmt.Sprintf("%d:{Key:%q,Title:%q}", i, columns[i].Key, columns[i].Title))
				}
				dupDetail = fmt.Sprintf("column(s) beyond the status column duplicate problem/cause text: %s", strings.Join(dups, ", "))
			}
			checkRule(t, td.ShortName, ruleDuplicateCause, dupViolation, dupDetail)
		})
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW VIOLATION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}
}
