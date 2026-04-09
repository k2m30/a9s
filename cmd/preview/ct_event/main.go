// Standalone preview for the redesigned CloudTrail event detail view.
// Run with: go run ./cmd/preview/ct_event/
//
// Design source: docs/design/ct-event-detail-v2.md §3 (v2.1 wireframes)
//
// Renders every canonical wireframe case from §3:
//
//	A — Karpenter ec2:DescribeInstances (R, no flags)                (ct-info, dim)
//	B — SSO Console ec2:TerminateInstances (D verb, MFA)             (ct-danger, red)
//	C — IAMUser s3:PutObject AccessDenied (errorCode)                (ct-danger, red)
//	D — KMS kms:RotateKey (AwsServiceEvent)                          (ct-attention, yellow)
//	E — Root s3:PutBucketPolicy (Root + W)                           (ct-attention, yellow)
//	F — IRSA s3:GetObject (WebIdentityUser, R)                       (ct-info, dim)
//	G — Cross-account s3:PutObject (W + cross-account)               (ct-attention, yellow)
//	H — Insight ApiCallRateInsight (no ACTOR)                        (ct-info, dim)
//	I — NetworkActivity VPCE deny s3:PutObject (errorCode)           (ct-danger, red)
//
// No interactivity, no AWS calls. All account IDs are synthetic
// (111111111111, 222222222222, ...).
package main

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// ── Tokyo Night palette (mirror of internal/tui/styles/palette.go) ────────────

var (
	colBorder    = lipgloss.Color("#414868")
	colAccent    = lipgloss.Color("#7aa2f7")
	colDim       = lipgloss.Color("#565f89")
	colHeaderFg  = lipgloss.Color("#c0caf5")
	colDetailVal = lipgloss.Color("#c0caf5")
	colRowAlt    = lipgloss.Color("#1e2030")
	colWarning   = lipgloss.Color("#e0af68") // ct-attention (ColPending)
	colError     = lipgloss.Color("#f7768e") // ct-danger    (ColStopped)
)

// ── Style primitives ──────────────────────────────────────────────────────────

var (
	stCard = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1)

	// stSectionV2: bold-only, no color — per design §1.2.
	// No new token in styles/; constructed inline as specified.
	stSectionV2 = lipgloss.NewStyle().Bold(true)

	stLabel = lipgloss.NewStyle().Foreground(colDim)
	stVal   = lipgloss.NewStyle().Foreground(colDetailVal)
	stDim   = lipgloss.NewStyle().Foreground(colDim)

	// Navigable value: accent-colored + underlined. Unchanged from v1 —
	// matches styles.NavigableField in internal/tui/styles/styles.go (FR-015).
	stNavValue = lipgloss.NewStyle().Foreground(colAccent).Underline(true)

	// Severity styles for the Event: row value (FR-002 / design §1.2 / §5).
	stEventInfo      = lipgloss.NewStyle().Foreground(colDim)     // ct-info
	stEventAttention = lipgloss.NewStyle().Foreground(colWarning) // ct-attention
	stEventDanger    = lipgloss.NewStyle().Foreground(colError)   // ct-danger

	// Right column styles — chrome, unchanged (design §1.1 / §4).
	stRColHdr  = lipgloss.NewStyle().Foreground(colDim)
	stRColRow  = lipgloss.NewStyle().Foreground(colHeaderFg)
	stRColZero = lipgloss.NewStyle().Foreground(colDim)
	stRColSel  = lipgloss.NewStyle().Foreground(colHeaderFg).Background(colRowAlt).Bold(true)
	stRColCard = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colBorder).
			Padding(0, 1)

	// Section label printer for main() case headers.
	stCaseLabel = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
)

// ── Model ─────────────────────────────────────────────────────────────────────

// kv holds a single label/value row. When eventStyle is non-nil it overrides
// the default stVal for the value (used exclusively on the Event: row — FR-002).
type kv struct {
	label      string
	value      string
	eventStyle *lipgloss.Style
	// multi-line values: \n separated; subsequent lines pad to label column.
	// When label is prefixed with navMarker, the row is rendered navigable:
	// value underlined in accent.
}

// navMarker prefixes kv.label to mark the row as navigable at render time.
const navMarker = "\x00NAV\x00"

// navKV builds a navigable row using the label prefix sentinel.
func navKV(label, value string) kv { return kv{label: navMarker + label, value: value} }

// colorKV builds an Event: row whose value renders in the given severity style.
func colorKV(label, value string, style lipgloss.Style) kv {
	return kv{label: label, value: value, eventStyle: &style}
}

type section struct {
	title string
	rows  []kv
}

type event struct {
	id       string
	sections []section
}

// ── Constants ─────────────────────────────────────────────────────────────────

const labelWidth = 14
const contentWidth = 82

// ── Helpers ───────────────────────────────────────────────────────────────────

// padRight pads s (measured by lipgloss.Width) on the right to n cols.
func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func renderRow(k kv) []string {
	nav := strings.HasPrefix(k.label, navMarker)
	label := k.label
	if nav {
		label = strings.TrimPrefix(label, navMarker)
	}
	lines := strings.Split(k.value, "\n")
	out := make([]string, 0, len(lines))
	labelCol := stLabel.Render(padRight(label+":", labelWidth))
	blank := strings.Repeat(" ", labelWidth)
	for i, ln := range lines {
		var styled string
		switch {
		case nav:
			if ln == "" {
				styled = ""
			} else {
				styled = stNavValue.Render(ln)
			}
		case k.eventStyle != nil:
			styled = k.eventStyle.Render(ln)
		default:
			styled = stVal.Render(ln)
		}
		if i == 0 {
			out = append(out, " "+labelCol+" "+styled)
		} else {
			if nav {
				out = append(out, " "+blank+" "+styled)
			} else {
				out = append(out, " "+blank+" "+ln)
			}
		}
	}
	return out
}

func renderSection(s section) []string {
	var lines []string
	head := " " + stSectionV2.Render(strings.ToUpper(s.title))
	lines = append(lines, head)
	for _, r := range s.rows {
		lines = append(lines, renderRow(r)...)
	}
	return lines
}

func renderEvent(e event) string {
	var body []string

	for _, s := range e.sections {
		body = append(body, renderSection(s)...)
	}

	card := stCard.Width(contentWidth).Render(strings.Join(body, "\n"))
	title := stDim.Render("╴ ct-events/" + e.id + " ╶")
	hints := renderHintBorder(contentWidth)
	cardLines := strings.Split(card, "\n")
	cardLines[len(cardLines)-1] = hints
	return title + "\n" + strings.Join(cardLines, "\n") + "\n"
}

// renderHintBorder builds a closing border line matching layout/frame.go
// BottomBorderWithHints. v2 hint set: y yaml / search / tab cols / esc back.
// The `R raw` hint is removed per design §1.5.
func renderHintBorder(w int) string {
	type hint struct{ key, desc string }
	hints := []hint{
		{"y", "yaml"},
		{"/", "search"},
		{"tab", "cols"},
		{"esc", "back"},
	}
	keyStyle := lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colDim)
	borderStyle := lipgloss.NewStyle().Foreground(colBorder)
	dashSep := borderStyle.Render("──")
	var parts []string
	used := 0
	for i, h := range hints {
		rendered := keyStyle.Render(h.key) + " " + descStyle.Render(h.desc)
		hv := lipgloss.Width(rendered)
		sv := 0
		if i > 0 {
			sv = 2
		}
		used += sv + hv
		if i > 0 {
			parts = append(parts, dashSep)
		}
		parts = append(parts, rendered)
	}
	leadingDashes := max(w-1-used-3, 0)
	var sb strings.Builder
	sb.WriteString(borderStyle.Render("╰" + strings.Repeat("─", leadingDashes)))
	for _, p := range parts {
		sb.WriteString(p)
	}
	sb.WriteString(borderStyle.Render("──╯"))
	return sb.String()
}

// ── Right column (mock RELATED panel) ────────────────────────────────────────

// relatedRow mirrors the rightColumnRow shape used by rightColumnModel.
// count == -1 means pivot/FetchFilter row (no "(N)" suffix), count == 0
// dim, count > 0 actionable.
type relatedRow struct {
	label    string
	count    int
	selected bool
}

const rightColWidth = 32 // outer width including border + 1-col padding

func renderRightColumn(rows []relatedRow) string {
	innerW := rightColWidth - 4
	var lines []string

	header := "RELATED"
	pad := max((innerW-lipgloss.Width(header))/2, 0)
	lines = append(lines, stRColHdr.Render(strings.Repeat(" ", pad)+header))

	for _, r := range rows {
		var text string
		var style lipgloss.Style
		switch r.count {
		case -1:
			text = "  " + r.label
			style = stRColRow
		case 0:
			text = "  " + r.label + " (0)"
			style = stRColZero
		default:
			text = fmt.Sprintf("  %s (%d)", r.label, r.count)
			style = stRColRow
		}
		if r.selected {
			lines = append(lines, stRColSel.Width(innerW).Render(text))
		} else {
			lines = append(lines, style.Render(text))
		}
	}
	return stRColCard.Width(rightColWidth).Render(strings.Join(lines, "\n"))
}

// renderEventWithRight renders an event card with a related right column
// joined horizontally. Used for the composite cases that mirror §4.
func renderEventWithRight(e event, related []relatedRow) string {
	left := renderEvent(e)
	right := renderRightColumn(related)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right) + "\n"
}

// ── Fixtures ──────────────────────────────────────────────────────────────────

// Case A — Karpenter ec2:DescribeInstances (read, success → ct-info dim)
// Design §3 Case A. Section order: ACTOR → ACTION → TARGET → CONTEXT → REQUEST.
func caseA() event {
	return event{
		id: "e-a1b2c3d4",
		sections: []section{
			{title: "ACTOR", rows: []kv{
				navKV("Principal", "arn:aws:sts::111111111111:assumed-role/KarpenterNodeRole/\nkarpenter-1759"),
				navKV("Access key", "ASIAY44QH8DCKARPEXMP"),
				{"User agent", "aws-sdk-go-v2/1.30.3", nil},
			}},
			{title: "ACTION", rows: []kv{
				colorKV("Event", "ec2:DescribeInstances", stEventInfo),
			}},
			{title: "TARGET", rows: []kv{
				{"Instances", "(all)", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "us-east-1", nil},
				{"Source IP", "10.0.14.221", nil},
				{"Time", "2026-04-07T14:02:11Z", nil},
			}},
			{title: "REQUEST", rows: []kv{
				{"filters", "[{Name: instance-state-name, Values: [running]}]", nil},
				{"maxResults", "1000", nil},
			}},
		},
	}
}

// Case B — SSO Console ec2:TerminateInstances (D verb, MFA → ct-danger red)
// instancesSet extracted into TARGET → REQUEST omitted. SSO opaque ARN → As: row.
// Design §3 Case B.
func caseB() event {
	return event{
		id: "e-b2c3d4e5",
		sections: []section{
			{title: "ACTOR", rows: []kv{
				navKV("Principal", "arn:aws:sts::222222222222:assumed-role/\nAWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d/alice@corp"),
				{"As", "alice@corp via AWSReservedSSO_AdminAccess", nil},
				{"MFA", "yes", nil},
				navKV("Access key", "ASIAZK7L9PQRSSOXEXMP"),
				{"User agent", "Console (AWS Internal)", nil},
			}},
			{title: "ACTION", rows: []kv{
				colorKV("Event", "ec2:TerminateInstances", stEventDanger),
			}},
			{title: "TARGET", rows: []kv{
				{"Instance", "instance/i-0f1e2d3c4b5a69788", nil},
				{"Instance", "instance/i-0f1e2d3c4b5a69789", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "eu-west-1", nil},
				{"Source IP", "AWS Internal", nil},
				{"Time", "2026-04-07T14:07:42Z", nil},
			}},
			{title: "RESPONSE", rows: []kv{
				{"terminating", "[{i-0f1e2d3c4b5a69788: shutting-down ← running},\n {i-0f1e2d3c4b5a69789: shutting-down ← running}]", nil},
			}},
		},
	}
}

// Case C — s3:PutObject AccessDenied (errorCode → ct-danger red, ERROR hoisted)
// bucketName + key extracted into TARGET → REQUEST omitted.
// ERROR sits between CONTEXT and (omitted) REQUEST. Design §3 Case C.
func caseC() event {
	return event{
		id: "e-c3d4e5f6",
		sections: []section{
			{title: "ACTOR", rows: []kv{
				navKV("Principal", "arn:aws:iam::333333333333:user/bob"),
				navKV("Access key", "AKIAIOSFODNN7BOB1XMP"),
				{"User agent", "aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0", nil},
			}},
			{title: "ACTION", rows: []kv{
				colorKV("Event", "s3:PutObject", stEventDanger),
			}},
			{title: "TARGET", rows: []kv{
				{"Bucket", "prod-logs", nil},
				{"Object", "prod-logs/2026/04/07/app.log", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "us-east-1", nil},
				{"Source IP", "198.51.100.42", nil},
				{"Time", "2026-04-07T14:11:03Z", nil},
			}},
			// ERROR hoisted directly after CONTEXT — per design §1.1 / §3 Case C.
			{title: "ERROR", rows: []kv{
				{"errorCode", "AccessDenied", nil},
				{"errorMessage", "User: arn:aws:iam::333333333333:user/bob is not authorized to\nperform: s3:PutObject on resource:\narn:aws:s3:::prod-logs/2026/04/07/app.log because no identity-\nbased policy allows the s3:PutObject action", nil},
			}},
		},
	}
}

// Case D — KMS kms:RotateKey (AwsServiceEvent → ct-attention yellow)
// No userIdentity ARN → Service: row. Category: row (≠ default). keyId → TARGET.
// Design §3 Case D.
func caseD() event {
	return event{
		id: "e-d4e5f6a7",
		sections: []section{
			{title: "ACTOR", rows: []kv{
				{"Service", "kms.amazonaws.com", nil},
			}},
			{title: "ACTION", rows: []kv{
				colorKV("Event", "kms:RotateKey", stEventAttention),
				{"Category", "Management / AwsServiceEvent", nil},
			}},
			{title: "TARGET", rows: []kv{
				{"Key", "key/2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "us-east-1", nil},
				{"Source IP", "AWS Internal", nil},
				{"Time", "2026-04-07T02:00:07Z", nil},
			}},
			{title: "REQUEST", rows: []kv{
				{"rotationType", "AUTOMATIC", nil},
				{"backingKey", "true", nil},
			}},
		},
	}
}

// Case E — Root s3:PutBucketPolicy (Root + W → ct-attention yellow)
// Root principal ARN. bucketName extracted into TARGET; policy JSON stays in REQUEST.
// Design §3 Case E.
func caseE() event {
	return event{
		id: "e-e5f6a7b8",
		sections: []section{
			{title: "ACTOR", rows: []kv{
				navKV("Principal", "arn:aws:iam::555555555555:root"),
				{"User agent", "Console (Mozilla/5.0 ... Safari/605.1.15)", nil},
			}},
			{title: "ACTION", rows: []kv{
				colorKV("Event", "s3:PutBucketPolicy", stEventAttention),
			}},
			{title: "TARGET", rows: []kv{
				{"Bucket", "prod-artifacts", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "us-east-1", nil},
				{"Source IP", "203.0.113.17", nil},
				{"Time", "2026-04-07T03:42:18Z", nil},
			}},
			{title: "REQUEST", rows: []kv{
				{"policy", `{"Version": "2012-10-17", "Statement": [...]}`, nil},
			}},
		},
	}
}

// Case F — IRSA s3:GetObject (WebIdentityUser, R → ct-info dim)
// Federation: row for IRSA. VPC endpoint in CONTEXT.
// bucketName + key extracted into TARGET → REQUEST omitted. Design §3 Case F.
func caseF() event {
	return event{
		id: "e-f6a7b8c9",
		sections: []section{
			{title: "ACTOR", rows: []kv{
				navKV("Principal", "arn:aws:sts::666666666666:assumed-role/eks-checkout-svc-sa/\n1717156821993453824"),
				{"Federation", "oidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C", nil},
				{"User agent", "aws-sdk-go-v2/1.30.3", nil},
			}},
			{title: "ACTION", rows: []kv{
				colorKV("Event", "s3:GetObject", stEventInfo),
			}},
			{title: "TARGET", rows: []kv{
				{"Bucket", "checkout-config", nil},
				{"Object", "checkout-config/prod/config.json", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "eu-west-1", nil},
				{"Source IP", "10.42.3.18", nil},
				{"VPC endpoint", "vpce-0abc123def456", nil},
				{"Time", "2026-04-07T14:20:21Z", nil},
			}},
		},
	}
}

// Case G — Cross-account s3:PutObject (W + cross-account → ct-attention yellow)
// Caller in 888888888888, recipient 777777777777 → Recipient: row in CONTEXT.
// bucketName + key extracted into TARGET → REQUEST omitted. Design §3 Case G.
func caseG() event {
	return event{
		id: "e-a7b8c9d0",
		sections: []section{
			{title: "ACTOR", rows: []kv{
				navKV("Principal", "arn:aws:sts::888888888888:assumed-role/CiBuildRole/build-4821"),
				navKV("Access key", "ASIAQF3M2N8KCIB1XMPL"),
				{"User agent", "aws-cli/2.17.9", nil},
			}},
			{title: "ACTION", rows: []kv{
				colorKV("Event", "s3:PutObject", stEventAttention),
			}},
			{title: "TARGET", rows: []kv{
				{"Bucket", "shared-artifacts", nil},
				{"Object", "shared-artifacts/build-4821.tar.gz", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "us-east-2", nil},
				{"Source IP", "52.14.88.201", nil},
				{"Recipient", "777777777777 (cross-account)", nil},
				{"Time", "2026-04-07T14:31:55Z", nil},
			}},
		},
	}
}

// Case H — Insight ApiCallRateInsight (no ACTOR → starts at ACTION)
// No userIdentity → ACTOR omitted. Insight metadata folds into ACTION.
// Frame title: standard ╴ ct-events/<eventId> ╶ form. Design §3 Case H.
func caseH() event {
	return event{
		id: "e-b8c9d0e1",
		sections: []section{
			{title: "ACTION", rows: []kv{
				colorKV("Event", "ec2:RunInstances", stEventInfo),
				{"Category", "Insight / AwsApiCall", nil},
				{"Insight type", "ApiCallRateInsight", nil},
				{"State", "Start", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "us-east-1", nil},
				{"Time", "2026-04-07T09:14:00Z", nil},
			}},
			{title: "REQUEST", rows: []kv{
				{"baseline", "0.24 calls/min  (7d window)", nil},
				{"insight", "18.70 calls/min (during anomaly)", nil},
				{"insight prin", "arn:aws:sts::999999999999:assumed-role/DeployRole/ci-41", nil},
				{"baseline prin", "arn:aws:sts::999999999999:assumed-role/DeployRole/ci-*", nil},
			}},
		},
	}
}

// Case I — NetworkActivity VPCE deny s3:PutObject (errorCode → ct-danger red)
// Category: NetworkActivity / AwsVpceEvent in ACTION. VPC endpoint in CONTEXT.
// bucketName + key extracted into TARGET → REQUEST omitted.
// ERROR hoisted after CONTEXT. Design §3 Case I.
func caseI() event {
	return event{
		id: "e-c9d0e1f2",
		sections: []section{
			{title: "ACTOR", rows: []kv{
				navKV("Principal", "arn:aws:sts::111111111111:assumed-role/DataPipelineRole/dp-0719"),
				{"User agent", "aws-sdk-java/2.25.11", nil},
			}},
			{title: "ACTION", rows: []kv{
				colorKV("Event", "s3:PutObject", stEventDanger),
				{"Category", "NetworkActivity / AwsVpceEvent", nil},
			}},
			{title: "TARGET", rows: []kv{
				{"Bucket", "prod-lake", nil},
				{"Object", "prod-lake/landing/2026/04/07/batch-0719.parquet", nil},
			}},
			{title: "CONTEXT", rows: []kv{
				{"Region", "eu-central-1", nil},
				{"Source IP", "10.12.4.77", nil},
				{"VPC endpoint", "vpce-0ff11223344556677", nil},
				{"Time", "2026-04-07T14:44:17Z", nil},
			}},
			// ERROR hoisted directly after CONTEXT — per design §1.1 / §3 Case I.
			{title: "ERROR", rows: []kv{
				{"errorCode", "VpceAccessDenied", nil},
				{"errorMessage", "The VPC endpoint policy denies the s3:PutObject action on\narn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.parquet", nil},
			}},
		},
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	cases := []struct {
		label string
		ev    event
	}{
		{"A — Karpenter ec2:DescribeInstances (R, no flags)               (ct-info, dim)", caseA()},
		{"B — SSO Console ec2:TerminateInstances (D verb, MFA)            (ct-danger, red)", caseB()},
		{"C — IAMUser s3:PutObject AccessDenied (errorCode)               (ct-danger, red)", caseC()},
		{"D — KMS kms:RotateKey (AwsServiceEvent)                         (ct-attention, yellow)", caseD()},
		{"E — Root s3:PutBucketPolicy (Root + W)                          (ct-attention, yellow)", caseE()},
		{"F — IRSA s3:GetObject (WebIdentityUser, R)                      (ct-info, dim)", caseF()},
		{"G — Cross-account s3:PutObject (W + cross-account)              (ct-attention, yellow)", caseG()},
		{"H — Insight ApiCallRateInsight (no ACTOR)                       (ct-info, dim)", caseH()},
		{"I — NetworkActivity VPCE deny s3:PutObject (errorCode)          (ct-danger, red)", caseI()},
	}
	for _, c := range cases {
		fmt.Println()
		fmt.Println(stCaseLabel.Render("▌ " + c.label))
		fmt.Println()
		fmt.Print(renderEvent(c.ev))
	}

	// Composite layouts with right column (mirrors design.md §4).
	fmt.Println()
	fmt.Println(stCaseLabel.Render("▌ §4 — Composite layouts (left card + RELATED right column)"))

	composites := []struct {
		label   string
		ev      event
		related []relatedRow
	}{
		{
			"4b.2 — Case B (SSO TerminateInstances): EC2(2) + IAM Role",
			caseB(),
			[]relatedRow{
				{label: "IAM Roles", count: 1},
				{label: "EC2 Instances", count: 2, selected: true},
				{label: "IAM Users", count: 0},
				{label: "S3 Buckets", count: 0},
				{label: "CT events by AccessKeyId", count: -1},
				{label: "CT events by Username", count: -1},
				{label: "CT events by EventName", count: -1},
			},
		},
		{
			"4b.3 — Case C (PutObject AccessDenied): bucket+object+user",
			caseC(),
			[]relatedRow{
				{label: "IAM Users", count: 1},
				{label: "S3 Buckets", count: 1, selected: true},
				{label: "S3 Objects", count: 1},
				{label: "IAM Roles", count: 0},
				{label: "CT events by AccessKeyId", count: -1},
				{label: "CT events by Username", count: -1},
				{label: "CT events by EventName", count: -1},
			},
		},
		{
			"4b.4 — Case E (Root PutBucketPolicy): bucket only, no AccessKey pivot",
			caseE(),
			[]relatedRow{
				{label: "IAM Roles", count: 0},
				{label: "IAM Users", count: 0},
				{label: "S3 Buckets", count: 1, selected: true},
				{label: "CT events by Username", count: -1},
				{label: "CT events by EventName", count: -1},
			},
		},
	}
	for _, c := range composites {
		fmt.Println()
		fmt.Println(stCaseLabel.Render("▌ " + c.label))
		fmt.Println()
		fmt.Print(renderEventWithRight(c.ev, c.related))
	}
}
