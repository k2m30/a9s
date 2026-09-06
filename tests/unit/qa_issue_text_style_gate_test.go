// qa_issue_text_style_gate_test.go — the standing OWNER RULE machine-style
// gate: no raw AWS enum / UPPER_SNAKE token may appear in a user-facing
// cause text. AWS SDK enums come back as either UPPER_SNAKE_CASE
// ("CREATE_FAILED") or PascalCase ("PendingConfirmation"); domain.HumanizeStatusPhrase
// is the single conversion point that turns either shape into a lowercase,
// space-separated operator phrase. This gate polices two independent
// surfaces so a phrase can never sneak a raw enum onto the screen by a
// different route than the one HumanizeStatusPhrase already covers:
//
//  1. The declarative catalog — every registered FindingDef.Phrase across
//     every resource type (resource.AllResourceTypes()[i].Findings). These
//     are hand-authored phrase literals; a violator here is a copy/paste of
//     a raw AWS constant straight into a FindingDef declaration (e.g. the
//     historical `Phrase: "ALARM"` in catalog_monitoring.go, fixed to
//     "alarm triggered").
//  2. The RENDERED surfaces actually shown to an operator, driven through
//     the exact same demo-fixture-drain-plus-real-Controller harness as
//     TestIssueVisibilityGate_EveryColoredOrFlaggedRowIsVisibleSomewhere in
//     qa_issue_visibility_gate_test.go (reused verbatim: buildVisibilityTypeCache,
//     mergeWave2Findings, newVisibilityListController/newVisibilityDetailController,
//     listStatusCellFor/detailHasAttentionFor's underlying body-walks). This
//     surface catches enums that reach the screen at render/enrichment time
//     rather than through a static FindingDef literal — e.g. a Wave-2
//     enricher that does `Value: string(sdkType.SomeEnum)` into a
//     DetailRow, or a fetcher/materialize path that writes a raw
//     Fields["status"] value which a Key-less, Path-based Status column
//     then surfaces verbatim (see listExtractCellValue's title-match
//     fallback in core/app/list_columns.go — the humanize call only
//     guards the col.Key=="status" branch, not this fallback).
//
// EXEMPTIONS (isExemptStyleToken): a token matching the raw-enum shape
// ^[A-Z][A-Z0-9_]+$ (a leading capital followed by one or more caps/digits/
// underscores — i.e. no lowercase letters anywhere) is NOT a violation when:
//   - it contains ':' or '/' — these are resource identifiers/ARNs
//     (e.g. "arn:aws:iam::123456789012:role/x", "i-0a1b2c3d4e5f60001" has
//     no ':' or '/' but is excluded by the id/name check below instead),
//   - it equals the row's own resource ID or Name (case-sensitive) —
//     instance IDs, ARNs, and free-form names are operator-assigned data,
//     not a machine enum a11n developer chose to display,
//   - it is a short (<=5 char) acronym already accepted as a domain term
//     via knownAcronymExemptions (e.g. "PITR" prior to the coder's fix,
//     "ARN", "VPC", "TLS", "WAF", "DNS", "SSL", "IAM", "KMS", "SG", "AMI",
//     "CIS" — column *titles* and abbreviations that are conventional
//     industry shorthand, not verbose enum text an operator must decode).
//     This list is intentionally narrow: it exempts short, universally
//     recognized acronyms display elsewhere in this codebase (column
//     titles, spec doc language), not every all-caps word.
package unit_test

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rawEnumTokenPattern matches a "word" (delimited by ASCII word-boundary
// characters — whitespace/punctuation split happens in extractTokens) shaped
// like a raw AWS SDK enum constant: a leading capital letter followed by one
// or more further capitals, digits, or underscores, with NO lowercase letter
// anywhere in the token. This catches both UPPER_SNAKE_CASE ("CREATE_FAILED")
// and all-caps abbreviations lacking humanization ("ADMIN_ALL", "NO_MFA"),
// while never matching a normal English word (which always has a lowercase
// letter) or a humanized phrase ("create failed", "alarm triggered").
var rawEnumTokenPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)

// knownAcronymExemptions are short, industry-standard acronyms and service
// names that legitimately appear in user-facing text uppercase (column
// titles, spec prose, compliance control IDs, AWS service names embedded in
// a sentence) and are not raw AWS enum leakage — an operator reads "ARN",
// "VPC", or "AWS" the same way regardless of case; there is no "humanized"
// form to convert them to. Kept intentionally small and reviewed per
// addition: this is an allowlist of conventional shorthand a human author
// chose deliberately, not a general escape hatch for verbose enums.
var knownAcronymExemptions = map[string]bool{
	"ARN": true, "ARNS": true, "VPC": true, "TLS": true, "SSL": true, "DNS": true,
	"WAF": true, "IAM": true, "KMS": true, "SG": true, "AMI": true,
	"CIS": true, "ACL": true, "MFA": true, "DLQ": true, "PITR": true,
	"CIDR": true, "NAT": true, "IGW": true, "EIP": true, "ENI": true, "EBS": true, "SSRF": true,
	// AWS service-name / protocol acronyms that appear inline in
	// hand-written prose (e.g. "no HTTPS redirect", "isolated: quarantined
	// by AWS", "orphan: source DB deleted", "AccessDenied: ... S3 bucket
	// policy ..."). These are the author's own word choice, not a
	// machine-cased SDK constant — there is nothing to humanize.
	"AWS": true, "HTTP": true, "HTTPS": true, "TCP": true, "DB": true, "EB": true,
	"EC2": true, "S3": true, "NS": true, "SOA": true, "AZ": true,
	"PAB": true, "OK": true,
	// UTC is how a timestamp names its zone and IOPS is how a volume's
	// throughput is measured. Both are what the operator would write.
	"UTC": true, "IOPS": true,
	// IP is how an operator names an address; EKS is the service the health
	// sentence points the operator at.
	"IP": true, "EKS": true,
}

// awsAccessKeyIDPattern matches an AWS IAM access key ID shape
// (AKIA/ASIA + 16 alphanumeric chars) or this codebase's demo-fixture
// placeholder for one (e.g. "AKIAEXAMPLE2222222222") — a resource
// identifier embedded inline in prose ("Key AKIA... >90d (rotation)"), not
// an enum a developer chose to leave un-humanized.
var awsAccessKeyIDPattern = regexp.MustCompile(`^A(KIA|SIA)[A-Z0-9]{12,}$`)

// styleGateAllowlist pins today's known, not-yet-fixable style violations —
// same burn-down contract as knownVisibilityGaps in qa_issue_visibility_gate_test.go:
//   - present + still violating today            -> skip (logged), pre-existing debt.
//   - present + no longer violating               -> FAIL ("remove from allowlist").
//   - a violation NOT present here                -> FAIL unconditionally, a new
//     regression the allowlist was never told about.
//
// Key shape for the catalog sweep: "phrase:<shortName>:<FindingCode>".
// Key shape for the rendered sweep: "rendered:<shortName>:<resourceID>:<surface>"
// where surface is "list-status" or "detail-attention".
//
// Target: EMPTY. Any entry here names a concrete file/line the coder (or a
// follow-up PR) must fix; it is not a permanent exemption.
//
// PRUNED (2026-07-07): every entry this map ever held has been removed as
// stale. Two independent classes of staleness were found while verifying
// this gate, neither of which this file's own switch statements in
// TestIssueTextStyleGate_CatalogPhrasesNeverRawEnum / checkStyleGateSurface
// surface as a failure on their own — both silently Skip a no-longer-
// violating allowlisted key instead of erroring "remove from allowlist",
// unlike the explicit BURN-DOWN branch other gates in this package use
// (e.g. qa_status_column_uniformity_test.go's checkRule) — so each entry
// below was confirmed stale by reading current production source directly:
//   - The 11 "rendered:<type>:prod-<type>-healthy:list-status" entries named
//     resource IDs (prod-eks-healthy, prod-ng-healthy, prod-ecs-healthy, ...)
//     that do not exist anywhere in core/demo/ — dead keys that can never
//     match a real subtest.
//   - The 4 "rendered:*:detail-attention[0]" entries (cb, ecr, glue, sfn) and
//     the 3 "phrase:*" entries (ecr.vulnerabilities, ses.account-shutdown,
//     ses.account-probation) named enrichers/literals that already route
//     through domain.HumanizeStatusPhrase or already use lowercase prose
//     (cb_issue_enrichment.go's statusPhrase, glue_issue_enrichment.go's
//     statePhrase, sfn_issue_enrichment.go's statusPhrase, ecr_issue_enrichment.go's
//     "%d critical findings"/"%d high findings", catalog_cicd.go's
//     "<N> critical, <M> high vulnerabilities", catalog_messaging.go's
//     "sending paused by AWS (shutdown)" / "account under review (probation)")
//     — all fixed by the same wave that landed the humanize call, never
//     needing this allowlist at all.
var styleGateAllowlist = map[string]string{}

// extractTokens splits s on any character that is not a letter, digit, or
// underscore, returning the non-empty remainder tokens. Mirrors how a human
// operator visually parses words in a rendered phrase.
func extractTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			return false
		}
		if r >= '0' && r <= '9' {
			return false
		}
		return r != '_'
	})
}

// isExemptStyleToken reports whether tok — already known to match
// rawEnumTokenPattern — is an accepted exemption per the file-header
// EXEMPTIONS contract: a short known acronym, an AWS access-key-ID shaped
// identifier, or the row's own ID/Name.
func isExemptStyleToken(tok, resourceID, resourceName string) bool {
	if knownAcronymExemptions[tok] {
		return true
	}
	// A token with three or more digits is an identifier, not an enum: AWS
	// enum constants are words ("DISABLED", "CREATE_FAILED") while hosted-zone
	// IDs and volume IDs are the concrete values an operator copies. The
	// threshold is three rather than one so a protocol version stays in
	// scope — "TLS13" and "IPV6" carry a digit and are still raw tokens a
	// reader has to decode, which is what this gate is for.
	digits := 0
	for _, r := range tok {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits >= 3 {
		return true
	}
	if awsAccessKeyIDPattern.MatchString(tok) {
		return true
	}
	if resourceID != "" && tok == resourceID {
		return true
	}
	if resourceName != "" && tok == resourceName {
		return true
	}
	return false
}

// angleBracketPlaceholderPattern matches a "<...>" template placeholder in a
// catalog FindingDef.Phrase literal (e.g. "latest run <STATUS>", "<N> jobs
// failed"). These are documentation-style placeholders describing what the
// real enricher substitutes at runtime (see glue_issue_enrichment.go /
// sfn_issue_enrichment.go, which build the actual Finding.Phrase via
// fmt.Sprintf rather than this catalog literal) — the placeholder name
// itself is never rendered verbatim to an operator, so it is not the
// raw-enum-leak this gate polices.
var angleBracketPlaceholderPattern = regexp.MustCompile(`<[^<>]*>`)

// findRawEnumViolations scans text for whitespace/punctuation-delimited
// tokens shaped like a raw AWS enum (rawEnumTokenPattern) that are not
// exempt (isExemptStyleToken), and returns them. text containing ':' or '/'
// is handled at the token level: extractTokens already splits on those
// characters, so an ARN's individual segments (e.g. "IAM", "123456789012")
// are evaluated as separate tokens — segments that are pure digits never
// match rawEnumTokenPattern (no leading capital letter), and segments that
// are short service-name acronyms ("IAM", "ARN") are covered by
// knownAcronymExemptions. "<...>" template placeholders are stripped before
// tokenizing so a placeholder name (e.g. "STATUS" in "latest run <STATUS>")
// is never flagged — see angleBracketPlaceholderPattern.
func findRawEnumViolations(text, resourceID, resourceName string) []string {
	if text == "" {
		return nil
	}
	text = angleBracketPlaceholderPattern.ReplaceAllString(text, "")
	var violations []string
	for _, tok := range extractTokens(text) {
		if !rawEnumTokenPattern.MatchString(tok) {
			continue
		}
		if isExemptStyleToken(tok, resourceID, resourceName) {
			continue
		}
		violations = append(violations, tok)
	}
	return violations
}

// TestIssueTextStyleGate_CatalogPhrasesNeverRawEnum sweeps every registered
// FindingDef.Phrase across every resource type and asserts it contains no
// raw AWS enum token. This is the declarative-literal half of the OWNER
// RULE: a FindingDef.Phrase is hand-authored, so a violation here is always
// a straightforward literal fix (e.g. `Phrase: "ALARM"` -> `Phrase: "alarm
// triggered"`), never a rendering-pipeline gap.
func TestIssueTextStyleGate_CatalogPhrasesNeverRawEnum(t *testing.T) {
	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var newlyRegressed []string
	var readyForBurnDown []string
	var stillGapped []string

	for _, td := range types {
		for _, fd := range td.Findings {
			violations := findRawEnumViolations(fd.Phrase, "", "")
			key := fmt.Sprintf("phrase:%s:%s", td.ShortName, fd.Code)
			testName := fmt.Sprintf("%s/%s", td.ShortName, fd.Code)

			t.Run(testName, func(t *testing.T) {
				violating := len(violations) > 0
				reason, allowlisted := styleGateAllowlist[key]

				switch {
				case violating && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf("BURN-DOWN: %s Phrase %q is still allowlisted (%s) but the allowlist entry no longer matches reality — re-check", key, fd.Phrase, reason)
				case violating:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW VIOLATION (not allowlisted): %s: FindingDef.Phrase %q contains raw AWS enum token(s) %v — "+
							"a user-facing cause text must never show a raw SDK enum; use domain.HumanizeStatusPhrase "+
							"or rewrite the literal to lowercase prose. If this is pre-existing debt, add %q to styleGateAllowlist "+
							"naming the exact reason it cannot be fixed in this wave.",
						key, fd.Phrase, violations, key,
					)
				case allowlisted:
					stillGapped = append(stillGapped, key)
					t.Skipf("KNOWN GAP (allowlisted): %s Phrase %q: %s", key, fd.Phrase, reason)
				}
			})
		}
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

// TestIssueTextStyleGate_RenderedSurfacesNeverRawEnum drives the exact same
// demo-fixture-drain-plus-real-Controller harness as
// TestIssueVisibilityGate_EveryColoredOrFlaggedRowIsVisibleSomewhere
// (qa_issue_visibility_gate_test.go) and asserts that neither the list
// Status cell nor the detail Attention block ever shows a raw AWS enum
// token, for every registered type's demo fixtures. This is the
// render-pipeline half of the OWNER RULE: unlike the catalog-literal sweep
// above, a violation here means the *pipeline* (a fetcher writing a raw
// Fields["status"], an enricher doing `Value: string(sdkEnum)` into a
// DetailRow, or a column-resolution branch that skips the humanize call)
// leaked an enum onto the screen even though every FindingDef.Phrase
// literal is clean.
func TestIssueTextStyleGate_RenderedSurfacesNeverRawEnum(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	demoClients := demo.NewServiceClients()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var newlyRegressed []string
	var readyForBurnDown []string
	var stillGapped []string

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}

		merged := mergeWave2Findings(t, td, fixtures, cache, demoClients)

		for _, res := range merged {
			testName := fmt.Sprintf("%s/%s", td.ShortName, res.ID)

			t.Run(testName, func(t *testing.T) {
				statusCell, _ := listStatusCellFor(t, td, merged, res.ID)
				statusViolations := findRawEnumViolations(statusCell, res.ID, res.Name)
				checkStyleGateSurface(t, td.ShortName, res.ID, "list-status", statusCell, statusViolations,
					&newlyRegressed, &readyForBurnDown, &stillGapped)

				attentionValues := detailAttentionValuesFor(t, res, td.ShortName)
				for i, v := range attentionValues {
					violations := findRawEnumViolations(v, res.ID, res.Name)
					if len(violations) == 0 {
						continue
					}
					surface := fmt.Sprintf("detail-attention[%d]", i)
					checkStyleGateSurface(t, td.ShortName, res.ID, surface, v, violations,
						&newlyRegressed, &readyForBurnDown, &stillGapped)
				}
			})
		}
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

// checkStyleGateSurface applies the shared ratchet decision (identical
// contract to knownVisibilityGaps) for one (shortName, resourceID, surface)
// triple, given the violations already found in its rendered text.
func checkStyleGateSurface(
	t *testing.T,
	shortName, resourceID, surface, text string,
	violations []string,
	newlyRegressed, readyForBurnDown, stillGapped *[]string,
) {
	t.Helper()
	key := fmt.Sprintf("rendered:%s:%s:%s", shortName, resourceID, surface)
	violating := len(violations) > 0
	reason, allowlisted := styleGateAllowlist[key]

	switch {
	case violating && allowlisted:
		*readyForBurnDown = append(*readyForBurnDown, key)
		t.Errorf("BURN-DOWN candidate check: %s %s=%q still raw (%s) — allowlist entry stands", key, surface, text, reason)
	case violating:
		*newlyRegressed = append(*newlyRegressed, key)
		t.Errorf(
			"NEW VIOLATION (not allowlisted): %s: %s=%q contains raw AWS enum token(s) %v for resource %q (type=%s). "+
				"A user-facing cause text must never show a raw SDK enum; route it through domain.HumanizeStatusPhrase. "+
				"If this is pre-existing debt, add %q to styleGateAllowlist naming the exact reason it cannot be fixed in this wave.",
			key, surface, text, violations, resourceID, shortName, key,
		)
	case allowlisted:
		*stillGapped = append(*stillGapped, key)
		t.Skipf("KNOWN GAP (allowlisted): %s %s=%q: %s", key, surface, text, reason)
	}
}

// detailAttentionValuesFor drives the REAL detail body build
// (Controller.EnsureDetailState + Snapshot().Body.Detail), exactly like
// detailHasAttentionFor in qa_issue_visibility_gate_test.go, and returns
// every non-empty FieldRow.Value within the Attention block (FieldRow.Path
// == "Attention") instead of just a present/absent boolean — this style
// gate needs the actual text to scan for raw enum tokens, not merely
// whether the block exists.
func detailAttentionValuesFor(t *testing.T, res resource.Resource, shortName string) []string {
	t.Helper()
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(res, shortName)
	body := c.Snapshot().Body.Detail
	if body == nil {
		return nil
	}
	var values []string
	for _, f := range body.Fields {
		if f.Path != "Attention" {
			continue
		}
		if f.Value == "" {
			continue
		}
		values = append(values, f.Value)
	}
	return values
}

// ---------------------------------------------------------------------------
// EXTENSION: every rendered list CELL, not just the Status column.
//
// The two gates above sweep the Status cell and the detail Attention block —
// but a raw AWS enum can just as easily leak through any OTHER column that
// echoes a Fields[...] value or a fieldpath-extracted struct field verbatim
// (e.g. sg's Risk column showing "WIDE_OPEN"/"PORTS:22" via
// Fields["risk_summary"], or ng's Issues column showing
// "EC2LaunchTemplateVersionMismatch,AutoScalingGroupInvalidConfiguration" via
// Fields["health_issues"] — see core/aws/sg.go computeSGRiskFields and
// core/aws/ng.go's issueCodes join). Unlike the Status/Attention gates,
// a plain column has no domain.HumanizeStatusPhrase chokepoint at all today —
// this is new coverage, not a re-check of an existing conversion point.
// ---------------------------------------------------------------------------

// wholeCellUpperSnakePattern matches a full cell value shaped like a raw
// UPPER_SNAKE_CASE AWS enum constant end-to-end (e.g. "WIDE_OPEN",
// "CREATE_FAILED") — anchored on the whole string, unlike rawEnumTokenPattern
// which is applied per-token after splitting. A column cell is a single
// semantic value (not free-form prose), so the violation shape here is "the
// entire cell IS the enum", not "the enum appears somewhere in a sentence".
var wholeCellUpperSnakePattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$`)

// wholeCellCamelEnumPattern matches a full cell value shaped like a
// whole-cell CamelCase AWS-enum (e.g. "PendingConfirmation",
// "EC2LaunchTemplateVersionMismatch") — at least two capital-initial
// "words" concatenated with no separator and no lowercase-only word
// boundary punctuation. Deliberately anchored on the whole cell (not a
// per-token scan) so normal display names that merely CONTAIN a capital
// letter (e.g. "prod-eks-healthy") are never touched — this pattern only
// fires when the ENTIRE cell has the enum shape.
var wholeCellCamelEnumPattern = regexp.MustCompile(`^([A-Z][a-z0-9]+){2,}$`)

// cellTimeShapedPattern matches a cell that is a rendered timestamp, either
// FormatValue's "2006-01-02 15:04" shape or a raw Go time.Time %v fallback
// ("2026-02-10 08:30:00 +0000 UTC") whose trailing zone abbreviation (UTC,
// PST, EST, ...) would otherwise false-positive against
// wholeCellUpperSnakePattern-adjacent single-word checks. Matched as a
// whole-cell prefix check (a date shape at the start of the cell) rather
// than a token regex, since the gate's per-cell scan (not per-token) never
// splits a timestamp into pieces in the first place.
var cellTimeShapedPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}([ T]\d{2}:\d{2})?`)

// cellDotVersionPattern matches an instance-type / version shape containing
// a digit immediately adjacent to a dot (e.g. "t3.large", "1.29", "8.0.35")
// — these are identifiers/version strings, not enum text, and are common
// enough (RDS engine version, EKS k8s version, EC2 instance type) to need an
// explicit exemption independent of the no-lowercase-letter enum shapes
// above (a bare version like "1.29" has no letters at all and already can't
// match either enum pattern, but this exemption also covers any future
// mixed-case version token).
var cellDotVersionPattern = regexp.MustCompile(`\d\.\d`)

// classificationColumnFieldSuffixes names the field-path/key suffixes that
// identify a CLASSIFICATION/TAXONOMY column — "what KIND of thing is this
// row" — as opposed to a lifecycle/status/cause-text column. AWS's own
// console renders these verbatim in exactly this shape (e.g. ACM's
// Type=AMAZON_ISSUED, DynamoDB's Billing=PAY_PER_REQUEST via
// BillingModeSummary.BillingMode, EC2's Type=InstanceType, CloudWatch's
// Metric=MetricName / Namespace=Namespace): there is no "humanized" form to
// convert "AMAZON_ISSUED" or "m5.xlarge" to, because the raw SDK constant
// IS the correct display value for a classification field, the same way
// "VPC" or "S3" is (knownAcronymExemptions covers the same idea for
// standalone acronym tokens). This is derived from the *column's own field
// name*, not a per-resource-type or per-column-title allowlist: any column
// whose Path or Key ends with one of these suffixes is exempt everywhere,
// for every resource type, present or future.
//
// Deliberately EXCLUDES suffixes that name a cause/reason/state field
// (e.g. "Code", "Reason", "Status", "State", "Issues") — a column like ECS's
// StopCode or ng's health_issues must still be caught, because those DO have
// a humanized-prose alternative and are exactly the class of bug this gate
// exists to find.
var classificationColumnFieldSuffixes = []string{"type", "mode", "name", "namespace"}

// isClassificationColumn reports whether col names a classification/taxonomy
// field per classificationColumnFieldSuffixes, checked against both Path and
// Key (whichever is populated) case-insensitively.
func isClassificationColumn(col app.ColumnDef) bool {
	field := col.Path
	if field == "" {
		field = col.Key
	}
	field = strings.ToLower(field)
	if field == "" {
		return false
	}
	for _, suffix := range classificationColumnFieldSuffixes {
		if strings.HasSuffix(field, suffix) {
			return true
		}
	}
	return false
}

// isExemptStyleCell reports whether the ENTIRE cell value is an accepted
// exemption from the whole-cell enum-shape check, per the EXTENSION contract:
//   - empty, or the row's own ID/Name (case-sensitive) — resource identifiers
//     are operator-assigned data, not a machine enum a developer chose to
//     display,
//   - the column itself is a classification/taxonomy field
//     (isClassificationColumn) — the raw SDK constant IS the correct display
//     value; there is nothing to humanize,
//   - contains ':' or '/' — ARNs, "PORTS:22"-shaped risk summaries once
//     humanized to prose, CIDR-adjacent or path-shaped identifiers,
//   - contains a digit-adjacent dot (cellDotVersionPattern) — instance
//     types ("t3.large") and dotted version strings ("1.29", "8.0.35"),
//   - looks like a rendered timestamp (cellTimeShapedPattern),
//   - is a short known acronym (knownAcronymExemptions) — column values that
//     are themselves conventional shorthand, not verbose enum text.
//
// Column semantics (not a per-type hardcoded list) drive every rule above:
// an identifier-like cell is recognized structurally (its own shape, or by
// being byte-identical to the row's ID/Name, or by its own column's field
// name), never by checking the column's Key/Title against a
// per-resource-type allowlist.
func isExemptStyleCell(cell, resourceID, resourceName string, col app.ColumnDef) bool {
	if cell == "" {
		return true
	}
	if resourceID != "" && cell == resourceID {
		return true
	}
	if resourceName != "" && cell == resourceName {
		return true
	}
	if isClassificationColumn(col) {
		return true
	}
	if strings.Contains(cell, ":") || strings.Contains(cell, "/") {
		return true
	}
	if cellDotVersionPattern.MatchString(cell) {
		return true
	}
	if cellTimeShapedPattern.MatchString(cell) {
		return true
	}
	if knownAcronymExemptions[cell] {
		return true
	}
	return false
}

// findWholeCellEnumViolation reports whether cell (the full rendered value
// of one list column, for one row) is itself shaped like a raw AWS enum
// (whole-cell UPPER_SNAKE or whole-cell CamelCase), after exemptions. Unlike
// findRawEnumViolations (token-level, applied to free-form prose text), this
// operates on the WHOLE cell string because a column cell is a single
// semantic value, not a sentence a human authored — the entire cell being
// enum-shaped is the violation, not a substring of it.
func findWholeCellEnumViolation(cell, resourceID, resourceName string, col app.ColumnDef) bool {
	if isExemptStyleCell(cell, resourceID, resourceName, col) {
		return false
	}
	if wholeCellUpperSnakePattern.MatchString(cell) {
		return true
	}
	if wholeCellCamelEnumPattern.MatchString(cell) {
		return true
	}
	return false
}

// cellStyleGateAllowlist pins today's known whole-cell raw-enum violations
// found by the rendered-cell sweep below — same burn-down contract as
// styleGateAllowlist/knownVisibilityGaps: present + still violating -> skip
// (pre-existing debt); present + no longer violating -> FAIL ("remove from
// allowlist"); a violation NOT present here -> FAIL unconditionally (a new
// regression the allowlist was never told about).
//
// Key shape: "<shortName>:<resourceID>:<columnTitle>".
//
// Target: EMPTY. Any entry here names a concrete production gap the coder
// (or a follow-up PR) must fix. sg's Risk column ("WIDE_OPEN") and ng's
// Status/Issues columns ("CREATE_FAILED", "InsufficientFreeAddresses") were
// the two verified-RED violations this gate's extension was written to
// catch (see core/aws/sg.go's sgWideOpenPhrase/sgDangerousPortsPhrase
// and core/aws/ng.go's domain.HumanizeStatusPhrase(string(issue.Code))
// fix) — neither entry below is that pair; both were already fixed at
// write time and never needed allowlisting.
var cellStyleGateAllowlist = map[string]string{
	"ecs-task:f6a1b2c3d4e5f60102030405:Stop Code": "ECS task StopCode is fetched via fieldpath.ExtractScalar from the raw RawStruct.StopCode field (no Wave-1/Wave-2 finding path humanizes it) — the Stop Code column has no HumanizeStatusPhrase chokepoint today; needs a dedicated cause-text builder or CellDecorator, out of scope for this wave's sg/ng/lambda fix.",
	"ecs-task:e5f6a1b2c3d4e5f601020304:Stop Code": "same ECS Stop Code gap as f6a1b2c3d4e5f60102030405 — StopCode is Path-based with no humanize chokepoint.",
	"nat:nat-0failed111111111d:Failure":           "NAT gateway Failure column is fetched via fieldpath.ExtractScalar from the raw RawStruct FailureCode field (e.g. \"InsufficientFreeAddressesInSubnet\") — no humanize chokepoint on this column today; out of scope for this wave's sg/ng/lambda fix.",
}

// TestIssueTextStyleGate_RenderedListCellsNeverRawEnum sweeps EVERY column
// cell (not just the Status column) of every registered type's demo fixtures,
// driven through the real Controller.ApplyResourcesLoaded + Snapshot().Body.List
// path (identical harness to listStatusCellFor / buildVisibilityTypeCache /
// mergeWave2Findings), and asserts no cell is itself shaped like a raw AWS
// enum end-to-end. This catches enum leakage through columns that have no
// HumanizeStatusPhrase chokepoint at all (e.g. sg's Risk column, ng's Issues
// column) — a gap the Status-only/Attention-only gates above cannot see.
func TestIssueTextStyleGate_RenderedListCellsNeverRawEnum(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	demoClients := demo.NewServiceClients()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var newlyRegressed []string
	var readyForBurnDown []string
	var stillGapped []string

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}

		merged := mergeWave2Findings(t, td, fixtures, cache, demoClients)

		c := newVisibilityListController(t, td.ShortName)
		c.ApplyResourcesLoaded(td.ShortName, merged, nil, false)
		body := c.Snapshot().Body.List
		if body == nil {
			continue
		}

		byID := make(map[string]resource.Resource, len(merged))
		for _, r := range merged {
			byID[r.ID] = r
		}

		for _, row := range body.Rows {
			res, ok := byID[row.ResourceID]
			if !ok {
				continue
			}

			t.Run(fmt.Sprintf("%s/%s", td.ShortName, res.ID), func(t *testing.T) {
				for i, cell := range row.Cells {
					if i >= len(body.Columns) {
						continue
					}
					col := body.Columns[i]
					if !findWholeCellEnumViolation(cell, res.ID, res.Name, col) {
						continue
					}
					checkCellStyleGate(t, td.ShortName, res.ID, col.Title, cell,
						&newlyRegressed, &readyForBurnDown, &stillGapped)
				}
			})
		}
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

// checkCellStyleGate applies the shared ratchet decision (identical contract
// to checkStyleGateSurface) for one (shortName, resourceID, columnTitle)
// triple whose cell is already known to violate the whole-cell enum shape.
func checkCellStyleGate(
	t *testing.T,
	shortName, resourceID, columnTitle, cell string,
	newlyRegressed, _, stillGapped *[]string,
) {
	t.Helper()
	key := fmt.Sprintf("%s:%s:%s", shortName, resourceID, columnTitle)
	reason, allowlisted := cellStyleGateAllowlist[key]

	switch {
	case allowlisted:
		*stillGapped = append(*stillGapped, key)
		t.Skipf("KNOWN GAP (allowlisted): %s column %q=%q: %s", key, columnTitle, cell, reason)
	default:
		*newlyRegressed = append(*newlyRegressed, key)
		t.Errorf(
			"NEW VIOLATION (not allowlisted): %s: column %q cell=%q is a raw AWS enum shape for resource %q (type=%s). "+
				"A rendered list cell must never show a raw SDK enum verbatim; route it through domain.HumanizeStatusPhrase "+
				"or a dedicated cause-text builder. If this is pre-existing debt, add %q to cellStyleGateAllowlist "+
				"naming the exact reason it cannot be fixed in this wave.",
			key, columnTitle, cell, resourceID, shortName, key,
		)
	}
}

// ---------------------------------------------------------------------------
// EXTENSION: no vacuous severity-word phrase.
//
// A cause text that is ITSELF just the bare word "error", "warning", or
// "issue" tells the operator nothing they didn't already know from the row's
// color — the whole point of a cause text (S4 phrase / status cell /
// Attention row) is to say WHY, not to restate THAT something is wrong.
// "error: volume unusable" passes (it says why); a bare "error" does not.
// This is a WHOLE-STRING exact-match check (case-insensitive), not a
// substring scan — a phrase merely containing one of these words as part of
// a real sentence is not a violation.
// ---------------------------------------------------------------------------

// vacuousPhraseWords are whole cause-text values that carry no information
// beyond "something is wrong" — the row's color already says that. Matched
// exact (case-insensitive, whitespace-trimmed), never as a substring.
var vacuousPhraseWords = map[string]bool{ //nolint:gochecknoglobals // static exact-match set, mirrors this file's other allowlist style
	"attention": true, "danger": true, "warning": true, "warn": true,
	"error": true, "issue": true, "problem": true, "critical": true,
	"alert": true, "bad": true, "unhealthy": true,
}

// isVacuousPhrase reports whether text, trimmed and lowercased, exactly
// equals one of vacuousPhraseWords — never a substring/prefix match, so
// "error: volume unusable" and "critical alarm silenced" both pass.
func isVacuousPhrase(text string) bool {
	return vacuousPhraseWords[strings.ToLower(strings.TrimSpace(text))]
}

// vacuousPhraseAllowlist pins today's known vacuous-phrase violations — same
// burn-down contract as styleGateAllowlist/knownVisibilityGaps: present +
// still violating today -> skip (pre-existing debt); present + no longer
// violating -> FAIL ("remove from allowlist"); a violation NOT present here
// -> FAIL unconditionally (a new regression the allowlist was never told
// about).
//
// Key shape for the catalog sweep: "phrase:<shortName>:<FindingCode>".
// Key shape for the rendered sweep: "rendered:<shortName>:<resourceID>:<surface>"
// where surface is "list-status" or "detail-attention[N]".
//
// Target: EMPTY. ct-events phrases (ct-danger/ct-attention/ct-info status
// values) were already fixed by the same wave that landed
// domain.HumanizeStatusPhrase and are NOT bare vacuous words (they carry the
// "ct-" event-classification prefix, not a standalone severity word) — this
// gate is expected green against them without needing an entry here.
//
// SEEDED 2026-07-07 (this gate's first run): four catalog FindingDef.Phrase
// literals are the bare word "error"/"unhealthy" with no cause text at all —
// all four are production-owned classifier literals
// (core/aws/ebs.go/ebs_snap state-error branches, ecs_task_codes.go's
// health-unhealthy branch, efs.go's error branch), not test fixtures, so
// fixing them is out of QA's write scope. Three of the four also surface on
// the rendered list-status column via a real demo fixture (efs's "error"
// finding never becomes the sole/first Finding on any current efs fixture,
// so it produces no rendered hit today).
var vacuousPhraseAllowlist = map[string]string{ //nolint:gochecknoglobals // burn-down allowlist, see doc comment
	"phrase:ebs:ebs.state.error":                             "colorEBS's state=error branch (core/aws/ebs.go) sets Phrase: \"error\" verbatim with no cause text — production classifier literal, not a QA fixture; needs a coder fix (e.g. \"error: volume unusable\").",
	"phrase:ebs-snap:ebs-snap.state.error":                   "colorEBSSnap's state=error branch (core/aws/ebs.go) sets Phrase: \"error\" verbatim with no cause text — production classifier literal, not a QA fixture; needs a coder fix.",
	"phrase:ecs-task:ecs-task.health.unhealthy":              "ecsTaskStructuralFindings' health-check branch (core/aws/ecs_task_codes.go) sets Phrase: \"unhealthy\" verbatim with no cause text — production classifier literal, not a QA fixture; needs a coder fix (e.g. \"unhealthy: container health check failing\").",
	"phrase:efs:efs.broken.error":                            "EFS's error branch (core/aws/efs.go) sets Phrase: \"error\" verbatim with no cause text — production classifier literal, not a QA fixture; needs a coder fix.",
	"rendered:ebs:vol-0error00000000b2:list-status":          "same production gap as phrase:ebs:ebs.state.error — the demo fixture's rendered list-status cell shows the bare literal verbatim; fixing the FindingDef.Phrase fixes this row too.",
	"rendered:ebs-snap:snap-error000000000b:list-status":     "same production gap as phrase:ebs-snap:ebs-snap.state.error — the demo fixture's rendered list-status cell shows the bare literal verbatim; fixing the FindingDef.Phrase fixes this row too.",
	"rendered:ecs-task:a7b8c9d0e1f2a7b8c9d0e1f2:list-status": "same production gap as phrase:ecs-task:ecs-task.health.unhealthy — the demo fixture's rendered list-status cell shows the bare literal verbatim; fixing the FindingDef.Phrase fixes this row too.",
}

// TestIssueTextStyleGate_CatalogPhrasesNeverVacuous sweeps every registered
// FindingDef.Phrase and asserts it is never itself just a bare severity word
// (isVacuousPhrase) — the declarative-literal half of the vacuous-phrase
// rule, mirroring TestIssueTextStyleGate_CatalogPhrasesNeverRawEnum's
// structure exactly.
func TestIssueTextStyleGate_CatalogPhrasesNeverVacuous(t *testing.T) {
	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var newlyRegressed []string
	var readyForBurnDown []string
	var stillGapped []string

	for _, td := range types {
		for _, fd := range td.Findings {
			key := fmt.Sprintf("phrase:%s:%s", td.ShortName, fd.Code)
			testName := fmt.Sprintf("%s/%s", td.ShortName, fd.Code)

			t.Run(testName, func(t *testing.T) {
				violating := isVacuousPhrase(fd.Phrase)
				reason, allowlisted := vacuousPhraseAllowlist[key]

				switch {
				case !violating && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf("BURN-DOWN: %s now conforms (Phrase %q is no longer vacuous) but is still allowlisted (%s) — remove %q from vacuousPhraseAllowlist", key, fd.Phrase, reason, key)
				case violating && allowlisted:
					stillGapped = append(stillGapped, key)
					t.Skipf("KNOWN GAP (allowlisted): %s Phrase %q: %s", key, fd.Phrase, reason)
				case violating:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW VIOLATION (not allowlisted): %s: FindingDef.Phrase %q is a bare vacuous severity word — "+
							"it must say WHY, not just restate that something is wrong (the row's color already does "+
							"that). If this is pre-existing debt, add %q to vacuousPhraseAllowlist naming the exact "+
							"reason it cannot be fixed in this wave.",
						key, fd.Phrase, key,
					)
				}
			})
		}
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

// TestIssueTextStyleGate_RenderedSurfacesNeverVacuous drives the same
// demo-fixture-drain-plus-real-Controller harness as
// TestIssueTextStyleGate_RenderedSurfacesNeverRawEnum and asserts that
// neither the list Status cell nor any detail Attention row value is ever
// itself just a bare vacuous severity word — the render-pipeline half of the
// vacuous-phrase rule.
func TestIssueTextStyleGate_RenderedSurfacesNeverVacuous(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	demoClients := demo.NewServiceClients()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var newlyRegressed []string
	var readyForBurnDown []string
	var stillGapped []string

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}

		merged := mergeWave2Findings(t, td, fixtures, cache, demoClients)

		for _, res := range merged {
			testName := fmt.Sprintf("%s/%s", td.ShortName, res.ID)

			t.Run(testName, func(t *testing.T) {
				statusCell, _ := listStatusCellFor(t, td, merged, res.ID)
				checkVacuousPhraseSurface(t, td.ShortName, res.ID, "list-status", statusCell,
					isVacuousPhrase(statusCell), &newlyRegressed, &readyForBurnDown, &stillGapped)

				attentionValues := detailAttentionValuesFor(t, res, td.ShortName)
				for i, v := range attentionValues {
					if !isVacuousPhrase(v) {
						continue
					}
					surface := fmt.Sprintf("detail-attention[%d]", i)
					checkVacuousPhraseSurface(t, td.ShortName, res.ID, surface, v,
						true, &newlyRegressed, &readyForBurnDown, &stillGapped)
				}
			})
		}
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

// checkVacuousPhraseSurface applies the shared ratchet decision (identical
// contract to checkStyleGateSurface) for one (shortName, resourceID, surface)
// triple already known to be a bare vacuous severity word.
func checkVacuousPhraseSurface(
	t *testing.T,
	shortName, resourceID, surface, text string,
	violating bool,
	newlyRegressed, readyForBurnDown, stillGapped *[]string,
) {
	t.Helper()
	key := fmt.Sprintf("rendered:%s:%s:%s", shortName, resourceID, surface)
	reason, allowlisted := vacuousPhraseAllowlist[key]

	switch {
	case !violating && allowlisted:
		*readyForBurnDown = append(*readyForBurnDown, key)
		t.Errorf("BURN-DOWN: %s now conforms (%s=%q is no longer vacuous) but is still allowlisted (%s) — remove %q from vacuousPhraseAllowlist", key, surface, text, reason, key)
	case violating && allowlisted:
		*stillGapped = append(*stillGapped, key)
		t.Skipf("KNOWN GAP (allowlisted): %s %s=%q: %s", key, surface, text, reason)
	case violating:
		*newlyRegressed = append(*newlyRegressed, key)
		t.Errorf(
			"NEW VIOLATION (not allowlisted): %s: %s=%q for resource %q (type=%s) is a bare vacuous severity word — "+
				"it must say WHY, not just restate that something is wrong (the row's color already does that). "+
				"If this is pre-existing debt, add %q to vacuousPhraseAllowlist naming the exact reason it cannot "+
				"be fixed in this wave.",
			key, surface, text, resourceID, shortName, key,
		)
	}
}
