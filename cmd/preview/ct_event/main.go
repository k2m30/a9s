// Standalone preview for the redesigned CloudTrail event detail view.
// Run with: go run ./cmd/preview/ct_event/
//
// Renders every canonical wireframe case from docs/design/ct-event-detail.md §4:
//
//	A — AssumedRole service role, read, success   (Karpenter DescribeInstances)
//	B — SSO AssumedRole console write, MFA         (TerminateInstances)
//	C — IAMUser long-lived key, AccessDenied       (S3 PutObject)
//	D — AWSService event                           (KMS RotateKey)
//	E — Root user action                           (PutBucketPolicy)
//	F — WebIdentityUser / IRSA                     (S3 GetObject)
//	G — Cross-account (recipient != caller)        (S3 PutObject)
//	H — Insight event                              (ApiCallRateInsight)
//	I — NetworkActivity VPCE deny                  (PutObject VpceAccessDenied)
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
	colSuccess   = lipgloss.Color("#9ece6a")
	colError     = lipgloss.Color("#f7768e")
	colWarning   = lipgloss.Color("#e0af68")
	colOrange    = lipgloss.Color("#ff9e64")
	colPurple    = lipgloss.Color("#bb9af7")
	colRowAlt    = lipgloss.Color("#1e2030")
)

// ── Style primitives ──────────────────────────────────────────────────────────

var (
	stCard = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1)

	stSection    = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	stSectionAlt = lipgloss.NewStyle().Foreground(colPurple).Bold(true)
	stSectionErr = lipgloss.NewStyle().Foreground(colError).Bold(true)

	stLabel = lipgloss.NewStyle().Foreground(colDim)
	stVal   = lipgloss.NewStyle().Foreground(colDetailVal)
	stDim   = lipgloss.NewStyle().Foreground(colDim)

	stActor   = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	stOK      = lipgloss.NewStyle().Foreground(colSuccess)
	stFailed  = lipgloss.NewStyle().Foreground(colError).Bold(true)
	stErrBody = lipgloss.NewStyle().Foreground(colDetailVal).Background(colRowAlt)

	// Verb glyphs + eventName coloring
	stVerbRead    = lipgloss.NewStyle().Foreground(colDim)
	stVerbWrite   = lipgloss.NewStyle().Foreground(colOrange).Bold(true)
	stVerbDelete  = lipgloss.NewStyle().Foreground(colError).Bold(true)
	stVerbService = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	stVerbInsight = lipgloss.NewStyle().Foreground(colPurple).Bold(true)

	// Badges
	stBadgeGood = lipgloss.NewStyle().Foreground(colSuccess)                          // [MFA] [CONSOLE]
	stBadgeInfo = lipgloss.NewStyle().Foreground(colAccent)                           // [SERVICE] [IMDSv2] [IRSA] [VPCE]
	stBadgeWarn = lipgloss.NewStyle().Foreground(colWarning)                          // [X-ACCT] [LONG-LIVED-KEY]
	stBadgeDim  = lipgloss.NewStyle().Foreground(colDim)                              // [AWS-INTERNAL]
	stBadgeRoot = lipgloss.NewStyle().Foreground(colError).Bold(true)                 // [ROOT]
	stRootBar   = lipgloss.NewStyle().Foreground(colHeaderFg).Background(colError).Bold(true)
	stHintBar   = lipgloss.NewStyle().Foreground(colDim)

	// Navigable value: accent-colored + underlined. No trailing glyph —
	// matches styles.NavigableField in internal/tui/styles/styles.go:195.
	stNavValue = lipgloss.NewStyle().Foreground(colAccent).Underline(true)
)

// ── Model ─────────────────────────────────────────────────────────────────────

type verb int

const (
	verbRead verb = iota
	verbWrite
	verbDelete
	verbService
	verbInsight
)

type kv struct {
	label string
	value string // may contain inline ANSI (badges)
	// multi-line values: \n separated; subsequent lines pad to label column.
	// When label is prefixed with navMarker, the row is rendered navigable:
	// value underlined in accent, trailing " →" glyph appended.
}

// navMarker prefixes kv.label to mark the row as navigable at render time.
// Kept in the label (not the value) so multi-line values render uniformly.
// Navigable rows render their value underlined+accent, no trailing arrow.
const navMarker = "\x00NAV\x00"

// navKV builds a navigable row using the label prefix sentinel.
func navKV(label, value string) kv { return kv{label: navMarker + label, value: value} }

type section struct {
	title string
	alt   bool // purple header
	err   bool // red header
	rows  []kv
	// freeform extra lines appended after rows (e.g. ERROR body)
	extra []string
}

type event struct {
	id       string
	verb     verb
	name     string // eventName
	actor    string
	badges   []string // pre-rendered badge strings (with brackets)
	target   string   // short target, shown on header line 2
	ok       bool
	errCode  string
	time     string
	region   string
	rootBar  bool // draw big red banner above header
	sections []section
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func badge(name, kind string) string {
	s := "[" + name + "]"
	switch kind {
	case "good":
		return stBadgeGood.Render(s)
	case "info":
		return stBadgeInfo.Render(s)
	case "warn":
		return stBadgeWarn.Render(s)
	case "dim":
		return stBadgeDim.Render(s)
	case "root":
		return stBadgeRoot.Render(s)
	}
	return s
}

func verbGlyph(v verb) string {
	switch v {
	case verbRead:
		return stVerbRead.Render("R")
	case verbWrite:
		return stVerbWrite.Render("W")
	case verbDelete:
		return stVerbDelete.Render("D")
	case verbService:
		return stVerbService.Render("S")
	case verbInsight:
		return stVerbInsight.Render("I")
	}
	return "?"
}

func verbName(v verb, name string) string {
	switch v {
	case verbRead:
		return stVerbRead.Render(name)
	case verbWrite:
		return stVerbWrite.Render(name)
	case verbDelete:
		return stVerbDelete.Render(name)
	case verbService:
		return stVerbService.Render(name)
	case verbInsight:
		return stVerbInsight.Render(name)
	}
	return name
}

const labelWidth = 14
const contentWidth = 82

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
	labelCol := stLabel.Render(padRight(label, labelWidth))
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
		default:
			styled = stVal.Render(ln)
		}
		if i == 0 {
			out = append(out, "  "+labelCol+styled)
		} else {
			if nav {
				out = append(out, "  "+blank+styled)
			} else {
				// continuation lines: pass through unchanged so embedded
				// badge spans remain intact.
				out = append(out, "  "+blank+ln)
			}
		}
	}
	return out
}

func renderSection(s section) []string {
	var lines []string
	var head string
	switch {
	case s.err:
		head = stSectionErr.Render(s.title)
	case s.alt:
		head = stSectionAlt.Render(s.title)
	default:
		head = stSection.Render(s.title)
	}
	lines = append(lines, head)
	for _, r := range s.rows {
		lines = append(lines, renderRow(r)...)
	}
	for _, e := range s.extra {
		lines = append(lines, "  "+e)
	}
	return lines
}

func renderEvent(e event) string {
	var body []string

	if e.rootBar {
		// Card outer width = contentWidth; inner = -2 border -2 padding.
		bw := contentWidth - 4
		bar := strings.Repeat("█", bw)
		body = append(body, stRootBar.Render(bar))
		body = append(body, stRootBar.Render(padRight("█ ROOT USER ACTION — account 555555555555", bw-1)+"█"))
		body = append(body, stRootBar.Render(padRight("█ "+e.name+"   "+e.target+"   OK", bw-1)+"█"))
		body = append(body, stRootBar.Render(bar))
		body = append(body, "")
	}

	// Header lines
	badges := strings.Join(e.badges, " ")
	header1 := verbGlyph(e.verb) + "  " + verbName(e.verb, e.name) + "   " + stActor.Render(e.actor)
	if badges != "" {
		header1 += " " + badges
	}
	body = append(body, header1)

	outcome := stOK.Render("OK")
	if !e.ok {
		outcome = stFailed.Render("FAILED (" + e.errCode + ")")
	}
	tgt := stDim.Render("(no resource)")
	if e.target != "" {
		tgt = stVal.Render(e.target)
	}
	header2 := "   → " + tgt + "   " + outcome + "   " + stDim.Render(e.time+"  "+e.region)
	body = append(body, header2)
	body = append(body, "")

	for i, s := range e.sections {
		if i > 0 {
			body = append(body, "")
		}
		body = append(body, renderSection(s)...)
	}

	// Box with fixed content width for deterministic rendering.
	card := stCard.Width(contentWidth).Render(strings.Join(body, "\n"))
	title := stDim.Render("╴ ct-events/" + e.id + " ╶")
	// Bottom hint bar mirrors layout/frame.go BottomBorderWithHints:
	// "key desc" pairs separated by ── inside the closing border line.
	hints := renderHintBorder(contentWidth)
	// The card already includes its own bottom border; we replace it
	// by trimming the last line of the card and appending the hint border.
	cardLines := strings.Split(card, "\n")
	cardLines[len(cardLines)-1] = hints
	return title + "\n" + strings.Join(cardLines, "\n") + "\n"
}

// renderHintBorder builds a closing border line "└──...──key desc──key desc──┘"
// matching internal/tui/layout/frame.go BottomBorderWithHints (line 149).
func renderHintBorder(w int) string {
	type hint struct{ key, desc string }
	hints := []hint{
		{"R", "raw"},
		{"y", "copy"},
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
	// Layout: └ + leadingDashes + parts + ──┘   total = w
	leadingDashes := w - 1 - used - 3
	if leadingDashes < 0 {
		leadingDashes = 0
	}
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

// Styles mirror rightColumnModel.View() in internal/tui/views/rightcolumn.go:
//   - header "RELATED" centered, DimText
//   - normal row: stRColRow
//   - zero/loading/error row: DimText
//   - selected row: RowSelected, full-width background
//
// The frame around the column is added externally — we replicate it here
// with stRColCard so the preview shows the complete visual.
var (
	stRColHdr  = lipgloss.NewStyle().Foreground(colDim)
	stRColRow  = lipgloss.NewStyle().Foreground(colHeaderFg)
	stRColZero = lipgloss.NewStyle().Foreground(colDim)
	stRColSel  = lipgloss.NewStyle().Foreground(colHeaderFg).Background(colRowAlt).Bold(true)
	stRColCard = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colBorder).
			Padding(0, 1)
)

func renderRightColumn(rows []relatedRow) string {
	// Card width = outer; inner content width = outer - 2 (border) - 2 (padding).
	innerW := rightColWidth - 4
	var lines []string

	// Centered "RELATED" header — exact match to rightColumnModel.View()
	// lines 172-176.
	header := "RELATED"
	pad := (innerW - lipgloss.Width(header)) / 2
	if pad < 0 {
		pad = 0
	}
	lines = append(lines, stRColHdr.Render(strings.Repeat(" ", pad)+header))

	for _, r := range rows {
		// Row text identical to rightColumnModel.View() switch (lines 195-214):
		//   loading/err          -> "  name"  (dim)
		//   count == -1 + filter -> "  name"  (normal — actionable pivot)
		//   count == 0           -> "  name (0)" (dim)
		//   default              -> "  name (N)" (normal)
		var text string
		var style lipgloss.Style
		switch {
		case r.count == -1:
			text = "  " + r.label
			style = stRColRow
		case r.count == 0:
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
// joined horizontally. Used for the composite cases that mirror §4b.
func renderEventWithRight(e event, related []relatedRow) string {
	left := renderEvent(e)
	right := renderRightColumn(related)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right) + "\n"
}

// ── Fixtures ──────────────────────────────────────────────────────────────────

func caseA() event {
	return event{
		id:     "e-a1b2c3d4",
		verb:   verbRead,
		name:   "DescribeInstances",
		actor:  "KarpenterNodeRole → karpenter-1759",
		badges: []string{badge("SERVICE", "info"), badge("IMDSv2", "info")},
		target: "",
		ok:     true,
		time:   "14:02:11Z",
		region: "us-east-1",
		sections: []section{
			{title: "WHO", rows: []kv{
				{"Actor", "KarpenterNodeRole → karpenter-1759  " + badge("SERVICE", "info") + " " + badge("IMDSv2", "info")},
				{"Account", "111111111111"},
				navKV("Principal", "arn:aws:iam::111111111111:role/KarpenterNodeRole"),
				navKV("Issuer role", "KarpenterNodeRole"),
				{"Session", "karpenter-1759  (started 13:44:02Z, 18m ago)"},
				{"MFA", "no"},
				navKV("Access key", "ASIAY44QH8DCKARPEXMP"),
				{"User agent", "Go SDK v2  (aws-sdk-go-v2/1.30.3)"},
				navKV("— 47 more events from this principal", ""),
			}},
			{title: "WHAT", rows: []kv{
				{"Event", "DescribeInstances"},
				{"Source", "ec2.amazonaws.com"},
				{"Category", "Management      Type   AwsApiCall"},
				{"Read only", "true"},
				{"Resources", stDim.Render("(no resource)")},
			}},
			{title: "WHERE", rows: []kv{
				{"Region", "us-east-1"},
				{"Source IP", "10.0.14.221"},
				{"TLS", "TLSv1.3  TLS_AES_128_GCM_SHA256"},
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T14:02:11Z"},
				{"Session age", "00:18:09"},
			}},
			{title: "REQUEST", rows: []kv{
				{"filters", `[ { Name: "instance-state-name", Values: ["running"] } ]`},
				{"maxResults", "1000"},
			}},
		},
	}
}

func caseB() event {
	return event{
		id:     "e-b2c3d4e5",
		verb:   verbDelete,
		name:   "TerminateInstances",
		actor:  "sso:alice@corp (via AdminAccess)",
		badges: []string{badge("CONSOLE", "good"), badge("MFA", "good")},
		target: "AWS::EC2::Instance i-0f1e2d3c4b5a69788",
		ok:     true,
		time:   "14:07:42Z",
		region: "eu-west-1",
		sections: []section{
			{title: "WHO", rows: []kv{
				{"Actor", "sso:alice@corp (via AdminAccess)  " + badge("CONSOLE", "good") + " " + badge("MFA", "good")},
				{"Account", "222222222222"},
				navKV("Principal", "arn:aws:iam::222222222222:role/aws-reserved/sso.amazonaws.com/\nAWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d"),
				{"Session", "alice@corp  (started 13:58:00Z, 9m ago)"},
				{"MFA", "yes"},
				{"Access key", "ASIAZK7L9PQRSSOXEXMP"},
				{"Source ident", "alice@corp"},
				{"User agent", "Console  (AWS Internal)"},
			}},
			{title: "WHAT", rows: []kv{
				{"Event", "TerminateInstances"},
				{"Source", "ec2.amazonaws.com"},
				{"Category", "Management      Type   AwsApiCall"},
				{"Read only", "false"},
				navKV("Resources", "AWS::EC2::Instance  arn:aws:ec2:eu-west-1:222222222222:\ninstance/i-0f1e2d3c4b5a69788"),
			}},
			{title: "WHERE", rows: []kv{
				{"Region", "eu-west-1"},
				{"Source IP", "AWS Internal  " + badge("AWS-INTERNAL", "dim")},
				{"TLS", "TLSv1.3  TLS_AES_128_GCM_SHA256"},
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T14:07:42Z"},
				{"Session age", "00:09:42"},
			}},
			{title: "REQUEST", rows: []kv{
				{"instancesSet", ""},
				navKV("  [0]", "i-0f1e2d3c4b5a69788"),
				navKV("  [1]", "i-0f1e2d3c4b5a69789"),
			}},
			{title: "RESPONSE", rows: []kv{
				navKV("terminating", "[ i-0f1e2d3c4b5a69788: shutting-down ← running ]"),
			}},
		},
	}
}

func caseC() event {
	return event{
		id:      "e-c3d4e5f6",
		verb:    verbWrite,
		name:    "PutObject",
		actor:   "bob",
		badges:  []string{badge("LONG-LIVED-KEY", "warn")},
		target:  "AWS::S3::Object arn:aws:s3:::prod-logs/2026/04/07/app.log",
		ok:      false,
		errCode: "AccessDenied",
		time:    "14:11:03Z",
		region:  "us-east-1",
		sections: []section{
			{title: "WHO", rows: []kv{
				{"Actor", "bob  " + badge("LONG-LIVED-KEY", "warn")},
				{"Account", "333333333333"},
				navKV("Principal", "arn:aws:iam::333333333333:user/bob"),
				{"MFA", "no"},
				{"Access key", "AKIAIOSFODNN7BOB1XMP"},
				{"User agent", "AWS CLI v2  (aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0)"},
			}},
			{title: "WHAT", rows: []kv{
				{"Event", "PutObject"},
				{"Source", "s3.amazonaws.com"},
				{"Category", "Data            Type   AwsApiCall"},
				{"Read only", "false"},
				{"Resources", ""},
				navKV("  Bucket", "arn:aws:s3:::prod-logs"),
				navKV("  Object", "arn:aws:s3:::prod-logs/2026/04/07/app.log"),
			}},
			{title: "WHERE", rows: []kv{
				{"Region", "us-east-1"},
				{"Source IP", "198.51.100.42"},
				{"TLS", "TLSv1.3"},
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T14:11:03Z"},
			}},
			{title: "REQUEST", rows: []kv{
				navKV("bucketName", "prod-logs"),
				navKV("key", "2026/04/07/app.log"),
			}},
			{
				title: "ERROR",
				err:   true,
				extra: []string{
					stErrBody.Render("AccessDenied"),
					stErrBody.Render("User: arn:aws:iam::333333333333:user/bob is not authorized to perform:"),
					stErrBody.Render("s3:PutObject on resource: arn:aws:s3:::prod-logs/2026/04/07/app.log"),
					stErrBody.Render("because no identity-based policy allows the s3:PutObject action"),
				},
			},
		},
	}
}

func caseD() event {
	return event{
		id:     "e-d4e5f6a7",
		verb:   verbService,
		name:   "RotateKey",
		actor:  "kms.amazonaws.com",
		badges: []string{badge("SERVICE", "info")},
		target: "AWS::KMS::Key (2f7e9a5b-…)",
		ok:     true,
		time:   "02:00:07Z",
		region: "us-east-1",
		sections: []section{
			{title: "WHO", rows: []kv{
				{"Actor", "kms.amazonaws.com  " + badge("SERVICE", "info")},
				{"Account", "444444444444"},
				{"Invoked by", "kms.amazonaws.com"},
			}},
			{title: "WHAT", rows: []kv{
				{"Event", "RotateKey"},
				{"Source", "kms.amazonaws.com"},
				{"Category", "Management      Type   AwsServiceEvent"},
				navKV("Resources", "AWS::KMS::Key  arn:aws:kms:us-east-1:444444444444:key/\n               2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b"),
			}},
			{title: "WHERE", rows: []kv{
				{"Region", "us-east-1"},
				{"Source IP", "AWS Internal  " + badge("AWS-INTERNAL", "dim")},
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T02:00:07Z"},
			}},
			{title: "SERVICE EVENT DETAILS", alt: true, rows: []kv{
				navKV("keyId", "2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b"),
				{"rotationType", "AUTOMATIC"},
				{"backingKey", "true"},
			}},
		},
	}
}

func caseE() event {
	return event{
		id:      "e-e5f6a7b8",
		verb:    verbWrite,
		name:    "PutBucketPolicy",
		actor:   "ROOT (account 555555555555)",
		badges:  []string{badge("ROOT", "root")},
		target:  "AWS::S3::Bucket arn:aws:s3:::prod-artifacts",
		ok:      true,
		time:    "03:42:18Z",
		region:  "us-east-1",
		rootBar: true,
		sections: []section{
			{title: "WHO", rows: []kv{
				{"Actor", "ROOT (account 555555555555)  " + badge("ROOT", "root")},
				{"Account", "555555555555"},
				{"Principal", "arn:aws:iam::555555555555:root"},
				{"MFA", "no"},
				{"Access key", stDim.Render("(signed with root credentials)")},
				{"User agent", "Console  (Mozilla/5.0 ... Safari/605.1.15)"},
			}},
			{title: "WHAT", rows: []kv{
				{"Event", "PutBucketPolicy"},
				{"Source", "s3.amazonaws.com"},
				{"Category", "Management      Type   AwsApiCall"},
				{"Read only", "false"},
				navKV("Resources", "AWS::S3::Bucket  arn:aws:s3:::prod-artifacts"),
			}},
			{title: "WHERE", rows: []kv{
				{"Region", "us-east-1"},
				{"Source IP", "203.0.113.17"},
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T03:42:18Z"},
			}},
			{title: "REQUEST", rows: []kv{
				navKV("bucketName", "prod-artifacts"),
				{"policy", `{ "Version": "2012-10-17", "Statement": [ ... ] }`},
			}},
		},
	}
}

func caseF() event {
	return event{
		id:     "e-f6a7b8c9",
		verb:   verbRead,
		name:   "GetObject",
		actor:  "checkout-svc-sa → 1717156821...",
		badges: []string{badge("SERVICE", "info"), badge("IRSA", "info")},
		target: "AWS::S3::Object arn:aws:s3:::checkout-config/prod/config.json",
		ok:     true,
		time:   "14:20:21Z",
		region: "eu-west-1",
		sections: []section{
			{title: "WHO", rows: []kv{
				{"Actor", "checkout-svc-sa → 1717156821...  " + badge("SERVICE", "info") + " " + badge("IRSA", "info")},
				{"Account", "666666666666"},
				navKV("Principal", "arn:aws:iam::666666666666:role/eks-checkout-svc-sa"),
				{"Session", "1717156821993453824"},
				{"MFA", "no"},
				{"Web federation", "arn:aws:iam::666666666666:oidc-provider/\noidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C\n" + stDim.Render("(not navigable — OIDC providers not in a9s)")},
				{"User agent", "aws-sdk-go-v2/1.30.3"},
			}},
			{title: "WHAT", rows: []kv{
				{"Event", "GetObject"},
				{"Source", "s3.amazonaws.com"},
				{"Category", "Data            Type   AwsApiCall"},
				{"Read only", "true"},
				navKV("Resources", "AWS::S3::Object  arn:aws:s3:::checkout-config/prod/config.json"),
			}},
			{title: "WHERE", rows: []kv{
				{"Region", "eu-west-1"},
				{"Source IP", "10.42.3.18"},
				{"VPC endpoint", "vpce-0abc123def456 (acct 666666666666)  " + badge("VPCE", "info")},
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T14:20:21Z"},
			}},
			{title: "REQUEST", rows: []kv{
				navKV("bucketName", "checkout-config"),
				navKV("key", "prod/config.json"),
			}},
		},
	}
}

func caseG() event {
	return event{
		id:     "e-a7b8c9d0",
		verb:   verbWrite,
		name:   "PutObject",
		actor:  "CiBuildRole → build-4821 (from 888888888888)",
		badges: []string{badge("X-ACCT", "warn")},
		target: "AWS::S3::Object arn:aws:s3:::shared-artifacts/build-4821.tar.gz",
		ok:     true,
		time:   "14:31:55Z",
		region: "us-east-2",
		sections: []section{
			{title: "WHO", rows: []kv{
				{"Actor", "CiBuildRole → build-4821  " + badge("X-ACCT", "warn")},
				{"Account", "888888888888  (caller)"},
				{"Recipient", "777777777777"},
				{"Principal", stDim.Render("arn:aws:iam::777777777777:role/CiBuildRole   (cross-acct, §8 q10)")},
				{"Session", "build-4821  (started 14:28:10Z, 3m ago)"},
				{"MFA", "no"},
				{"Access key", "ASIAQF3M2N8KCIB1XMPL"},
				{"User agent", "aws-cli/2.17.9"},
			}},
			{title: "WHAT", rows: []kv{
				{"Event", "PutObject"},
				{"Source", "s3.amazonaws.com"},
				{"Category", "Data            Type   AwsApiCall"},
				{"Resources", ""},
				navKV("  Bucket", "arn:aws:s3:::shared-artifacts"),
				navKV("  Object", "arn:aws:s3:::shared-artifacts/build-4821.tar.gz"),
			}},
			{title: "WHERE", rows: []kv{
				{"Region", "us-east-2"},
				{"Source IP", "52.14.88.201"},
				{"Recipient", "777777777777  " + badge("X-ACCT", "warn")},
				{"Shared event", "f1e2d3c4-b5a6-7890-1234-567890abcdef"},
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T14:31:55Z"},
			}},
			{title: "REQUEST", rows: []kv{
				navKV("bucketName", "shared-artifacts"),
				navKV("key", "build-4821.tar.gz"),
			}},
		},
	}
}

func caseH() event {
	return event{
		id:     "e-b8c9d0e1",
		verb:   verbInsight,
		name:   "RunInstances",
		actor:  "INSIGHT  ApiCallRateInsight  Start",
		target: stDim.Render("(statistical)"),
		ok:     true,
		time:   "09:14:00Z",
		region: "us-east-1",
		sections: []section{
			{title: "INSIGHT", alt: true, rows: []kv{
				{"Type", "ApiCallRateInsight"},
				{"State", "Start"},
				{"Event source", "ec2.amazonaws.com"},
				{"Event name", "RunInstances"},
			}},
			{title: "STATISTICS", alt: true, rows: []kv{
				{"Baseline", "average  0.24 calls/min  (7d window)"},
				{"Insight", "average 18.70 calls/min  (during anomaly)"},
			}},
			{title: "ATTRIBUTIONS", alt: true, rows: []kv{
				{"userIdentityArn", ""},
				navKV("  insight", "arn:aws:sts::999999999999:assumed-role/DeployRole/ci-41"),
				navKV("  baseline", "arn:aws:sts::999999999999:assumed-role/DeployRole/ci-*"),
				{"userAgent", "insight   aws-sdk-go-v2/1.30.3\nbaseline  Terraform/1.8.5"},
				{"errorCode", "insight   (none)\nbaseline  (none)"},
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T09:14:00Z"},
			}},
		},
	}
}

func caseI() event {
	return event{
		id:      "e-c9d0e1f2",
		verb:    verbWrite,
		name:    "PutObject",
		actor:   "DataPipelineRole → dp-0719",
		badges:  []string{badge("VPCE", "info")},
		target:  "AWS::S3::Object arn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.parquet",
		ok:      false,
		errCode: "VpceAccessDenied",
		time:    "14:44:17Z",
		region:  "eu-central-1",
		sections: []section{
			{title: "WHO", rows: []kv{
				{"Actor", "DataPipelineRole → dp-0719"},
				{"Account", "111111111111"},
				navKV("Principal", "arn:aws:iam::111111111111:role/DataPipelineRole"),
				{"User agent", "aws-sdk-java/2.25.11"},
			}},
			{title: "WHAT", rows: []kv{
				{"Event", "PutObject"},
				{"Source", "s3.amazonaws.com"},
				{"Category", "NetworkActivity  Type   AwsVpceEvent"},
			}},
			{title: "WHERE", rows: []kv{
				{"Region", "eu-central-1"},
				{"Source IP", "10.12.4.77"},
				navKV("VPC endpoint", "vpce-0ff11223344556677 (acct 111111111111)  "+badge("VPCE", "info")),
			}},
			{title: "WHEN", rows: []kv{
				{"Event time", "2026-04-07T14:44:17Z"},
			}},
			{title: "REQUEST", rows: []kv{
				navKV("bucketName", "prod-lake"),
				navKV("key", "landing/2026/04/07/batch-0719.parquet"),
			}},
			{
				title: "ERROR",
				err:   true,
				extra: []string{
					stErrBody.Render("VpceAccessDenied"),
					stErrBody.Render("The VPC endpoint policy denies the s3:PutObject action on"),
					stErrBody.Render("arn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.parquet"),
				},
			},
		},
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	cases := []struct {
		label string
		ev    event
	}{
		{"A — AssumedRole service role, read, success      (Karpenter DescribeInstances)", caseA()},
		{"B — SSO AssumedRole console write, MFA           (TerminateInstances)", caseB()},
		{"C — IAMUser long-lived key, AccessDenied         (S3 PutObject)", caseC()},
		{"D — AWSService event                             (KMS RotateKey)", caseD()},
		{"E — Root user action                             (PutBucketPolicy)", caseE()},
		{"F — WebIdentityUser / IRSA                       (S3 GetObject)", caseF()},
		{"G — Cross-account (recipient != caller)          (S3 PutObject)", caseG()},
		{"H — Insight event                                (ApiCallRateInsight)", caseH()},
		{"I — NetworkActivity VPCE deny                    (PutObject VpceAccessDenied)", caseI()},
	}
	for _, c := range cases {
		fmt.Println()
		fmt.Println(stSection.Render("▌ " + c.label))
		fmt.Println()
		fmt.Print(renderEvent(c.ev))
	}

	// Composite layouts with right column (mirrors design.md §4b).
	fmt.Println()
	fmt.Println(stSection.Render("▌ §4b — Composite layouts (left card + RELATED right column)"))

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
		fmt.Println(stSection.Render("▌ " + c.label))
		fmt.Println()
		fmt.Print(renderEventWithRight(c.ev, c.related))
	}
}
