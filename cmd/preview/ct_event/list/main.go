// Standalone preview for the redesigned CloudTrail events LIST view.
// Run with: go run ./cmd/preview/ct_event/list/
//
// Renders the four wireframes from docs/design/ct-event-list.md §4:
//
//	4a — busy mixed list (default columns, 132 cols)
//	4b — filtered to errors (/FAILED)
//	4c — filtered to a single principal (/bob)
//	4d — narrow terminal (80 cols, drop priority)
//
// No interactivity, no AWS calls. All values synthetic.
package main

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// ── Tokyo Night palette (mirror of internal/tui/styles/palette.go) ────────────

var (
	colBorder   = lipgloss.Color("#414868")
	colAccent   = lipgloss.Color("#7aa2f7")
	colDim      = lipgloss.Color("#565f89")
	colHeaderFg = lipgloss.Color("#c0caf5")
	colSuccess  = lipgloss.Color("#9ece6a")
	colError    = lipgloss.Color("#f7768e")
	colWarning  = lipgloss.Color("#e0af68")
	colOrange   = lipgloss.Color("#ff9e64")
	colPurple   = lipgloss.Color("#bb9af7")
	// EC2-style row-color tokens (match internal/tui/styles/palette.go):
	// ColStopped=red, ColPending=yellow, ColTerminated=dim.
	colStopped    = colError
	colPending    = colWarning
	colTerminated = colDim
)

// ── Cell styles ───────────────────────────────────────────────────────────────

var (
	stCard = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colBorder).
		Padding(0, 1)

	stHeader = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	stDim    = lipgloss.NewStyle().Foreground(colDim)
	stVal    = lipgloss.NewStyle().Foreground(colHeaderFg)

	stOK   = lipgloss.NewStyle().Foreground(colSuccess)
	stFail = lipgloss.NewStyle().Foreground(colError).Bold(true)
	stWarn = lipgloss.NewStyle().Foreground(colWarning)

	stVerbR = lipgloss.NewStyle().Foreground(colDim)               // read
	stVerbW = lipgloss.NewStyle().Foreground(colOrange).Bold(true) // write/mutating
	stVerbD = lipgloss.NewStyle().Foreground(colError).Bold(true)  // destructive
	stVerbS = lipgloss.NewStyle().Foreground(colAccent).Bold(true) // service event
	stVerbI = lipgloss.NewStyle().Foreground(colPurple).Bold(true) // insight
	stVerbN = lipgloss.NewStyle().Foreground(colAccent).Bold(true) // network activity

	stEvR = stVerbR
	stEvW = lipgloss.NewStyle().Foreground(colOrange)
	stEvD = lipgloss.NewStyle().Foreground(colError).Bold(true)
	stEvS = lipgloss.NewStyle().Foreground(colAccent)
	stEvI = lipgloss.NewStyle().Foreground(colPurple)
	stEvN = lipgloss.NewStyle().Foreground(colDim)

	stActorRoot = lipgloss.NewStyle().Foreground(colError).Bold(true)
	stActorSvc  = lipgloss.NewStyle().Foreground(colDim)
	stActorXAct = lipgloss.NewStyle().Foreground(colWarning)
	stActor     = lipgloss.NewStyle().Foreground(colHeaderFg)

	// Row tints — keyed by EC2-style Resource.Status value. See
	// docs/design/ct-event-list.md §5 and internal/tui/styles/styles.go:109.
	//
	//   "ct-root"    (NEW)        → fg header on red bg, bold
	//   "error"      (existing)   → red fg
	//   "pending"    (existing)   → yellow fg (cross-account)
	//   "terminated" (existing)   → dim fg (service event)
	//   "running"    (existing)   → green fg (default success)
	stTintRoot    = lipgloss.NewStyle().Background(colStopped).Foreground(colHeaderFg).Bold(true)
	stTintError   = lipgloss.NewStyle().Foreground(colStopped)
	stTintXAcct   = lipgloss.NewStyle().Foreground(colPending)
	stTintService = lipgloss.NewStyle().Foreground(colTerminated)

	stHint   = lipgloss.NewStyle().Foreground(colDim)
	stFilter = lipgloss.NewStyle().Foreground(colWarning).Bold(true)
)

// ── Row model ────────────────────────────────────────────────────────────────

type row struct {
	verb      string // "R" "W" "D" "S" "I" "N"
	timestamp string // "2006-01-02 15:04:05"
	actor     string
	origin    string
	event     string
	target    string
	outcome   string // "OK", "FAILED <code>", "START", "END"
	status    string // EC2-style: "ct-root" | "error" | "pending" | "terminated" | "running"
}

// pad truncates or space-pads to width w (end-elide with …).
func pad(s string, w int) string {
	if lipgloss.Width(s) == w {
		return s
	}
	if lipgloss.Width(s) > w {
		runes := []rune(s)
		if w <= 1 {
			return strings.Repeat(" ", w)
		}
		return string(runes[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

func midElide(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return pad(s, w)
	}
	if w < 5 {
		return pad(s, w)
	}
	keep := w - 1
	left := keep / 2
	right := keep - left
	r := []rune(s)
	return string(r[:left]) + "…" + string(r[len(r)-right:])
}

// ── Per-cell classifiers (match the proposed ListColumn.Color hook) ───────────

func styleVerb(v string) string {
	switch v {
	case "R":
		return stVerbR.Render(v)
	case "W":
		return stVerbW.Render(v)
	case "D":
		return stVerbD.Render(v)
	case "S":
		return stVerbS.Render(v)
	case "I":
		return stVerbI.Render(v)
	case "N":
		return stVerbN.Render(v)
	}
	return v
}

func styleEvent(v, name string) string {
	switch v {
	case "R":
		return stEvR.Render(name)
	case "W":
		return stEvW.Render(name)
	case "D":
		return stEvD.Render(name)
	case "S":
		return stEvS.Render(name)
	case "I":
		return stEvI.Render(name)
	case "N":
		return stEvN.Render(name)
	}
	return name
}

func styleActor(r row, padded string) string {
	if strings.HasPrefix(r.actor, "ROOT") {
		return stActorRoot.Render(padded)
	}
	switch r.status {
	case "pending":
		return stActorXAct.Render(padded)
	case "terminated":
		return stActorSvc.Render(padded)
	}
	return stActor.Render(padded)
}

func styleOutcome(o string, w int) string {
	padded := pad(o, w)
	if o == "OK" {
		return stOK.Render(padded)
	}
	if strings.HasPrefix(o, "START") || strings.HasPrefix(o, "END") {
		return stWarn.Render(padded)
	}
	return stFail.Render(padded)
}

func styleOrigin(o string, w int) string {
	switch o {
	case "Service", "—":
		return stDim.Render(pad(o, w))
	case "Console":
		return stHeader.Render(pad(o, w))
	}
	return stVal.Render(pad(o, w))
}

// ── Column widths (120-col default layout) ────────────────────────────────────
//
// verb 1 · time 19 · actor 26 · origin 7 · event 22 · target 28 · outcome 14
// = 117 chars + 6 single-space gutters = 123 chars of content.

const (
	wTime    = 19
	wActor   = 26
	wOrigin  = 7
	wEvent   = 22
	wTarget  = 28
	wOutcome = 14
)

func renderRow(r row) string {
	verb := styleVerb(r.verb)
	timeCell := stDim.Render(pad(r.timestamp, wTime))
	actorCell := styleActor(r, pad(midElide(r.actor, wActor), wActor))
	originCell := styleOrigin(r.origin, wOrigin)
	eventCell := styleEvent(r.verb, pad(midElide(r.event, wEvent), wEvent))
	targetCell := stVal.Render(pad(midElide(r.target, wTarget), wTarget))
	outcomeCell := styleOutcome(r.outcome, wOutcome)

	line := strings.Join([]string{
		verb, timeCell, actorCell, originCell, eventCell, targetCell, outcomeCell,
	}, " ")

	// Row-wide tint is applied by flattening the raw text and re-rendering
	// with a single style. The per-cell colors are swallowed — this mirrors
	// how styles.RowColorStyle paints a whole row in the real renderer.
	raw := func() string {
		return r.verb + " " + pad(r.timestamp, wTime) + " " +
			pad(midElide(r.actor, wActor), wActor) + " " +
			pad(r.origin, wOrigin) + " " +
			pad(midElide(r.event, wEvent), wEvent) + " " +
			pad(midElide(r.target, wTarget), wTarget) + " " +
			pad(r.outcome, wOutcome)
	}
	switch r.status {
	case "ct-root":
		return stTintRoot.Render(raw())
	case "error":
		return stTintError.Render(raw())
	case "pending":
		return stTintXAcct.Render(raw())
	case "terminated":
		return stTintService.Render(raw())
	case "running":
		// Default green row — keep per-cell colors, don't flatten.
		return line
	}
	return line
}

func headerRow() string {
	parts := []string{
		" ",
		stHeader.Render(pad("TIME", wTime)),
		stHeader.Render(pad("ACTOR", wActor)),
		stHeader.Render(pad("ORIGIN", wOrigin)),
		stHeader.Render(pad("EVENT", wEvent)),
		stHeader.Render(pad("TARGET", wTarget)),
		stHeader.Render(pad("OUTCOME", wOutcome)),
	}
	return strings.Join(parts, " ")
}

// ── Sample data (time DESC — newest first, matches fetch order) ───────────────

func sampleRows() []row {
	return []row{
		{"D", "2026-04-07 14:31:12", "sso:alice@corp (AdminAcc)", "Console", "TerminateInstances", "ec2/i-0f1e2d3c4b5a69788", "OK", "running"},
		{"W", "2026-04-07 14:30:37", "bob", "CLI", "PutObject", "s3/prod-logs/2026/04/07/app.log", "FAILED AccessDenied", "error"},
		{"D", "2026-04-07 14:30:05", "ROOT", "Console", "PutBucketPolicy", "s3/billing-archive", "OK", "ct-root"},
		{"R", "2026-04-07 14:29:41", "KarpenterNodeRole/k-1759", "SDK", "DescribeInstances", "(none)", "OK", "running"},
		{"S", "2026-04-07 14:28:52", "ec2.amazonaws.com", "Service", "TerminateInstanceInASG", "ec2/i-0a1b2c3d4e5f60718", "OK", "terminated"},
		{"W", "2026-04-07 14:27:44", "terraform/CI", "TF", "UpdateFunctionCode", "lambda/api-prod", "OK", "running"},
		{"R", "2026-04-07 14:25:13", "bob", "CLI", "ListBuckets", "(none)", "OK", "running"},
		{"D", "2026-04-07 14:19:02", "ops-deployer/build-9821", "SDK", "DeleteLogGroup", "logs//aws/lambda/api-prod", "OK", "running"},
		{"I", "2026-04-07 14:17:30", "—", "—", "ApiCallRateInsight", "ApiCallRateInsight ×4.2", "START", "running"},
		{"W", "2026-04-07 14:13:11", "federated:saml/dave", "Browser", "AssumeRoleWithSAML", "iam/AdminAccess", "OK", "running"},
		{"R", "2026-04-07 14:09:47", "ReadOnlyAuditor/sess-22a", "SDK", "DescribeSecurityGroups", "vpc/sg-08feab23", "OK", "running"},
		{"N", "2026-04-07 14:06:21", "vpce-0fab12/account-444444", "VPCE", "PutObject", "vpce-0fab12 → s3", "FAILED VpceAccessDenied", "pending"},
		{"W", "2026-04-07 14:00:58", "karpenter-controller/k-99", "SDK", "CreateFleet", "ec2/eks-prod-ng", "FAILED UnauthorizedOperation", "error"},
		{"R", "2026-04-07 13:50:33", "bob", "CLI", "GetCallerIdentity", "(none)", "OK", "running"},
		{"R", "2026-04-07 13:30:12", "ReadOnlyAuditor/sess-22a", "SDK", "DescribeStackResources", "cfn/billing-pipeline", "OK", "running"},
	}
}

func filterRows(rows []row, pred func(row) bool) []row {
	out := make([]row, 0, len(rows))
	for _, r := range rows {
		if pred(r) {
			out = append(out, r)
		}
	}
	return out
}

// ── Card frame ───────────────────────────────────────────────────────────────

func card(title string, body string, statusBar string, width int) string {
	c := stCard.Width(width)
	titleStyled := stHeader.Render(title)
	hint := stHint.Render(statusBar)
	inner := titleStyled + "\n" + body + "\n" + hint
	return c.Render(inner)
}

// ── 4a busy mixed list ────────────────────────────────────────────────────────

func render4a() string {
	rows := sampleRows()
	var b strings.Builder
	b.WriteString(headerRow())
	b.WriteString("\n")
	for _, r := range rows {
		b.WriteString(renderRow(r))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(stHint.Render("── m: load more (showing 25/114) ──"))
	status := "/filter  s sort  tab cols  enter detail  r related  esc back  ? help"
	return card("ct-events [114]", b.String(), status, 134)
}

// ── 4b filtered to errors ─────────────────────────────────────────────────────

func render4b() string {
	rows := filterRows(sampleRows(), func(r row) bool {
		return strings.HasPrefix(r.outcome, "FAILED")
	})
	// One more synthetic error for a fuller wireframe.
	rows = append(rows, row{
		"W", "2026-04-07 12:41:09", "ops-deployer/build-9817", "SDK",
		"AttachRolePolicy", "iam/role/ops-runner", "FAILED AccessDenied", "error",
	})

	var b strings.Builder
	b.WriteString(headerRow())
	b.WriteString("\n")
	for _, r := range rows {
		b.WriteString(renderRow(r))
		b.WriteString("\n")
	}
	status := "filter: " + stFilter.Render("FAILED") + "  ── enter to clear ──"
	return card(fmt.Sprintf("ct-events [%d of 114, filter: FAILED]", len(rows)), b.String(), status, 134)
}

// ── 4c single principal ───────────────────────────────────────────────────────

func render4c() string {
	rows := filterRows(sampleRows(), func(r row) bool {
		return strings.Contains(strings.ToLower(r.actor), "bob")
	})
	var b strings.Builder
	b.WriteString(headerRow())
	b.WriteString("\n")
	for _, r := range rows {
		b.WriteString(renderRow(r))
		b.WriteString("\n")
	}
	status := "filter: " + stFilter.Render("bob")
	return card(fmt.Sprintf("ct-events [%d of 114, filter: bob]", len(rows)), b.String(), status, 134)
}

// ── 4d narrow 80-col (TIME collapses to HH:MM:SS, ORIGIN dropped) ─────────────

const (
	wTimeN   = 8 // HH:MM:SS
	wActorN  = 14
	wEventN  = 16
	wTargetN = 16
	wOutN    = 11
)

func renderRowNarrow(r row) string {
	// Extract HH:MM:SS from "2006-01-02 15:04:05"
	hhmmss := r.timestamp
	if len(hhmmss) >= 19 {
		hhmmss = hhmmss[11:19]
	}

	verb := styleVerb(r.verb)
	timeCell := stDim.Render(pad(hhmmss, wTimeN))
	actorCell := styleActor(r, pad(midElide(r.actor, wActorN), wActorN))
	eventCell := styleEvent(r.verb, pad(midElide(r.event, wEventN), wEventN))
	targetCell := stVal.Render(pad(midElide(r.target, wTargetN), wTargetN))
	outCell := styleOutcome(midElide(r.outcome, wOutN), wOutN)

	line := strings.Join([]string{verb, timeCell, actorCell, eventCell, targetCell, outCell}, " ")

	raw := func() string {
		return r.verb + " " + pad(hhmmss, wTimeN) + " " +
			pad(midElide(r.actor, wActorN), wActorN) + " " +
			pad(midElide(r.event, wEventN), wEventN) + " " +
			pad(midElide(r.target, wTargetN), wTargetN) + " " +
			pad(midElide(r.outcome, wOutN), wOutN)
	}
	switch r.status {
	case "ct-root":
		return stTintRoot.Render(raw())
	case "error":
		return stTintError.Render(raw())
	case "pending":
		return stTintXAcct.Render(raw())
	case "terminated":
		return stTintService.Render(raw())
	}
	return line
}

func headerRowNarrow() string {
	return strings.Join([]string{
		" ",
		stHeader.Render(pad("TIME", wTimeN)),
		stHeader.Render(pad("ACTOR", wActorN)),
		stHeader.Render(pad("EVENT", wEventN)),
		stHeader.Render(pad("TARGET", wTargetN)),
		stHeader.Render(pad("OUTCOME", wOutN)),
	}, " ")
}

func render4d() string {
	rows := sampleRows()[:5]
	var b strings.Builder
	b.WriteString(headerRowNarrow())
	b.WriteString("\n")
	for _, r := range rows {
		b.WriteString(renderRowNarrow(r))
		b.WriteString("\n")
	}
	status := "/filter  s sort  tab cols  enter detail  esc back  ? help"
	return card("ct-events [114]", b.String(), status, 80)
}

// ── Driver ───────────────────────────────────────────────────────────────────

func main() {
	sections := []struct {
		title string
		body  string
	}{
		{"4a — busy mixed list (default columns, 132 cols)", render4a()},
		{"4b — filtered to errors (/FAILED)", render4b()},
		{"4c — filtered to a single principal (/bob)", render4c()},
		{"4d — narrow terminal (80 cols, drop priority)", render4d()},
	}
	for _, s := range sections {
		fmt.Println()
		fmt.Println(stHeader.Render(s.title))
		fmt.Println()
		fmt.Println(s.body)
	}
	fmt.Println()
}
