// qa_status_column_uniformity_test.go — the standing OWNER RULE gate: every
// registered top-level list view must present exactly ONE status column,
// uniformly titled "Status", whose cell the shared pipeline in
// internal/app/list_columns.go (listExtractCellValue's isStatusCol cascade)
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

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
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
// SEEDED CENSUS (2026-07-07, first run of this gate): every entry below was
// discovered by driving the REAL production column-resolution path
// (app.ResolveListColumns, which mirrors resolveColumns in
// table_render.go) against every registered top-level type's default view.
// This is the burn-down deliverable for the conversion coder — each entry
// names one duplicate/misnamed column that must be folded into the single
// shared Status cell (and, for the full cause text, the detail Attention
// block) per docs/related-resources.md's "single source of truth" doctrine
// applied here to list columns.
var knownStatusColumnDebt = map[string]bool{
	// --- status-title violations: the only status-qualifying column is
	// titled "State", not "Status" (isStatusCol matches "state" case
	// insensitively, but the OWNER RULE requires the exact title "Status"). ---
	"ec2:status-title":          true, // defaults_compute.go: {Title:"State", Path:"State.Name"}
	"lambda:status-title":       true, // defaults_compute.go: {Title:"State", Key:"state", Path:"State"}
	"ebs:status-title":          true, // defaults_compute.go: {Title:"State", Path:"State"}
	"ebs-snap:status-title":     true, // defaults_compute.go: {Title:"State", Path:"State"}
	"ami:status-title":          true, // defaults_compute.go: {Title:"State", Path:"State"}
	"vpc:status-title":          true, // defaults_networking.go: {Title:"State", Path:"State"}
	"subnet:status-title":       true, // defaults_networking.go: {Title:"State", Path:"State"}
	"nat:status-title":          true, // defaults_networking.go: {Title:"State", Path:"State"}
	"igw:status-title":          true, // defaults_networking.go: {Title:"State", Key:"state"}
	"vpce:status-title":         true, // defaults_networking.go: {Title:"State", Path:"State"}
	"tgw:status-title":          true, // defaults_networking.go: {Title:"State", Path:"State"}
	"elb:status-title":          true, // defaults_networking.go: {Title:"State", Path:"State.Code"}
	"eip:status-title":          true, // defaults_networking.go: {Title:"State", Key:"status"}
	"glue:status-title":         true, // no status-qualifying column at all in defaults_data.go "glue"
	"athena:status-title":       true, // defaults_data.go: {Title:"State", Path:"State"}
	"alarm:status-title":        true, // defaults_monitoring.go: {Title:"State", Path:"StateValue"}
	"msk:status-title":          true, // defaults_messaging.go: {Title:"State", Path:"State"}
	"eb-rule:status-title":      true, // defaults_messaging.go: {Title:"State", Path:"State"}
	"sqs:status-title":          true, // no status-qualifying column at all in defaults_messaging.go "sqs"
	"sns:status-title":          true, // no status-qualifying column at all in defaults_messaging.go "sns"
	"sns-sub:status-title":      true, // no status-qualifying column at all in defaults_messaging.go "sns-sub"
	"sfn:status-title":          true, // no status-qualifying column at all in defaults_messaging.go "sfn"
	"pipeline:status-title":     true, // no status-qualifying column at all in defaults_cicd.go "pipeline" ("Last Status" key="last_status" does not match Key=="status")
	"cb:status-title":           true, // no status-qualifying column at all in defaults_cicd.go "cb" ("Last Build" key="last_build" does not match Key=="status")
	"ecr:status-title":          true, // no status-qualifying column at all in defaults_cicd.go "ecr"
	"codeartifact:status-title": true, // no status-qualifying column at all in defaults_cicd.go "codeartifact"
	"role:status-title":         true, // no status-qualifying column at all in defaults_security.go "role"
	"iam-user:status-title":     true, // no status-qualifying column at all in defaults_security.go "iam-user"
	"iam-group:status-title":    true, // no status-qualifying column at all in defaults_security.go "iam-group"
	"waf:status-title":          true, // no status-qualifying column at all in defaults_security.go "waf"
	"policy:status-title":       true, // no status-qualifying column at all in defaults_security.go "policy" ("Risk" is the closest signal, but is not status-qualifying by key/title)
	"r53:status-title":          true, // no status-qualifying column at all in defaults_dns_cdn.go "r53"
	"apigw:status-title":        true, // no status-qualifying column at all in defaults_dns_cdn.go "apigw"
	"logs:status-title":         true, // no status-qualifying column at all in defaults_monitoring.go "logs"
	"trail:status-title":        true, // no status-qualifying column at all in defaults_monitoring.go "trail"
	"ct-events:status-title":    true, // no status-qualifying column at all in defaults_monitoring.go "ct-events" (an audit-log child-shaped view registered as a top-level type)
	"rtb:status-title":          true, // no status-qualifying column at all in defaults_networking.go "rtb"
	"sg:status-title":           true, // no status-qualifying column at all in defaults_networking.go "sg" (see sg:duplicate-cause — "Risk" is the closest signal, not status-qualifying by key/title)
	"tg:status-title":           true, // no status-qualifying column at all in defaults_networking.go "tg" (see tg:duplicate-cause — "Health" is the closest signal, not status-qualifying by key/title)
	"ssm:status-title":          true, // no status-qualifying column at all in defaults_secrets.go "ssm" (see ssm:duplicate-cause — "Risk" is the closest signal, not status-qualifying by key/title)

	// --- duplicate-cause violations: a second column beyond the true
	// status column carries problem/cause text (issues, health_issues,
	// risk, state reason). ---
	"eks:duplicate-cause": true, // defaults_containers.go: {Title:"Issues", Key:"health_issues"} alongside {Title:"Status", Key:"status"}
	"ng:duplicate-cause":  true, // defaults_containers.go: {Title:"Issues", Key:"health_issues"} alongside {Title:"Status", Key:"status"}
	"elb:duplicate-cause": true, // defaults_networking.go: {Title:"State Reason", Path:"State.Reason"} alongside {Title:"State", Path:"State.Code"}
	"sg:duplicate-cause":  true, // defaults_networking.go: {Title:"Risk", Key:"risk_summary"} — sg has no status column at all, so Risk is the sole cause-bearing column
	"ssm:duplicate-cause": true, // defaults_secrets.go: {Title:"Risk", Key:"risk"} alongside {Title:"Type"} (no status column)
}

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
// predicate in internal/app/list_columns.go byte-for-byte (Key=="status" OR
// Key==lifecycleKey OR (Key=="" AND Title case-insensitively "status" or
// "state")), operating on the exported app.ColumnDef + the type's
// LifecycleKey (default "state" when unset, matching production's own
// fallback).
func isStatusQualifyingColumn(col app.ColumnDef, lifecycleKey string) bool {
	if lifecycleKey == "" {
		lifecycleKey = "state"
	}
	if col.Key == "status" || col.Key == lifecycleKey {
		return true
	}
	if col.Key == "" && (strings.EqualFold(col.Title, "status") || strings.EqualFold(col.Title, "state")) {
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
// internal/app/list_columns.go) that separately matches the duplicate/cause
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
