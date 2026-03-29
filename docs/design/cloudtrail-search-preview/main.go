// CloudTrail Search/Debug View — static preview mockups.
// Run with: go run ./docs/design/cloudtrail-search-preview/
//
// Renders all key states of the ct-search view using Lipgloss v2:
//   1. Search Form (empty)
//   2. Search Form (with filters filled in)
//   3. Loading state
//   4. Results List (write events, last 1h)
//   5. Results List (error events only)
//   6. Results List (root activity)
//   7. Event Detail (normal write event)
//   8. Event Detail (AccessDenied error)
//   9. Event Detail (Root login)
//  10. Help Screens (form, results, detail)
package main

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// ── Palette (Tokyo Night Dark) ──────────────────────────────────────────────────

var (
	colHeaderFg     = lipgloss.Color("#c0caf5")
	colAccent       = lipgloss.Color("#7aa2f7")
	colDim          = lipgloss.Color("#565f89")
	colSep          = lipgloss.Color("#414868")
	colBorderNormal = lipgloss.Color("#414868")

	colRowSelected   = lipgloss.Color("#7aa2f7")
	colRowSelectedFg = lipgloss.Color("#1a1b26")

	colRunning = lipgloss.Color("#9ece6a") // green
	colPending = lipgloss.Color("#e0af68") // yellow/amber
	colPurple  = lipgloss.Color("#bb9af7") // purple

	colDetailKey = lipgloss.Color("#7aa2f7")
	colDetailSec = lipgloss.Color("#e0af68")
	colDetailVal = lipgloss.Color("#c0caf5")

	colHelpKey = lipgloss.Color("#9ece6a")
	colHelpCat = lipgloss.Color("#e0af68")

	colError = lipgloss.Color("#f7768e")

	colDarkBg = lipgloss.Color("#1a1b26") // maps to ColOverlayBg in main design spec palette
	colFormBg = lipgloss.Color("#24283b") // maps to ColKeyHintBg in main design spec palette
)

// ── Helpers ─────────────────────────────────────────────────────────────────────

// NOTE: This rune-slicing truncation is ANSI-unsafe -- it will break strings
// containing ANSI escape sequences (e.g., styled lipgloss output). For the
// static preview this is fine because all inputs are plain strings. The real
// implementation MUST use ansi.Truncate() from charm.land/x/ansi instead of
// rune slicing to correctly handle styled/colored strings.
func padOrTrunc(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		if w <= 1 {
			return s[:w]
		}
		r := []rune(s)
		if len(r) > w-1 {
			return string(r[:w-1]) + "\u2026"
		}
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

// ── Header ──────────────────────────────────────────────────────────────────────

func renderHeader(profile, region, version string, w int, rightContent string) string {
	accent := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render("a9s")
	ver := lipgloss.NewStyle().
		Foreground(colDim).Render(" v" + version)
	ctx := lipgloss.NewStyle().
		Foreground(colHeaderFg).Bold(true).
		Render("  " + profile + ":" + region)

	left := accent + ver + ctx
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(rightContent)

	innerW := w - 2
	gap := max(innerW-leftW-rightW, 1)

	content := left + strings.Repeat(" ", gap) + rightContent
	return lipgloss.NewStyle().
		Foreground(colHeaderFg).
		Width(w).Padding(0, 1).Render(content)
}

func renderHeaderNormal(profile, region, version string, w int) string {
	right := lipgloss.NewStyle().Foreground(colDim).Render("? for help")
	return renderHeader(profile, region, version, w, right)
}

// ── Framed box with centered title in top border ────────────────────────────────

func renderFramedBox(lines []string, title string, w int) string {
	borderStyle := lipgloss.NewStyle().Foreground(colBorderNormal)
	innerW := w - 2

	var topBorder string
	if title == "" {
		topBorder = borderStyle.Render("\u250c" + strings.Repeat("\u2500", w-2) + "\u2510")
	} else {
		titleRendered := lipgloss.NewStyle().Foreground(colHeaderFg).Bold(true).Render(title)
		titleVis := lipgloss.Width(titleRendered)

		totalDashes := max(w-2-titleVis-2, 2)
		leftDashes := totalDashes / 2
		rightDashes := totalDashes - leftDashes

		prefix := "\u250c" + strings.Repeat("\u2500", leftDashes) + " "
		suffix := " " + strings.Repeat("\u2500", rightDashes) + "\u2510"
		topBorder = borderStyle.Render(prefix) + titleRendered + borderStyle.Render(suffix)
	}

	var sb strings.Builder
	sb.WriteString(topBorder)

	for _, line := range lines {
		sb.WriteString("\n")
		visW := lipgloss.Width(line)
		var padded string
		if visW < innerW {
			padded = line + strings.Repeat(" ", innerW-visW)
		} else {
			padded = line
		}
		sb.WriteString(borderStyle.Render("\u2502"))
		sb.WriteString(padded)
		sb.WriteString(borderStyle.Render("\u2502"))
	}

	sb.WriteString("\n")
	sb.WriteString(borderStyle.Render("\u2514" + strings.Repeat("\u2500", w-2) + "\u2518"))

	return sb.String()
}

// ── Divider ─────────────────────────────────────────────────────────────────────

func divider(label string) string {
	line := strings.Repeat("\u2501", 38)
	return "\n" +
		lipgloss.NewStyle().Foreground(colSep).Render(line) +
		"  " +
		lipgloss.NewStyle().Foreground(colAccent).Bold(true).Render(label) +
		"  " +
		lipgloss.NewStyle().Foreground(colSep).Render(line) +
		"\n\n"
}

// ── Shared styles ───────────────────────────────────────────────────────────────

var (
	secStyle   = lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	kStyle     = lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle     = lipgloss.NewStyle().Foreground(colDetailVal)
	dimStyle   = lipgloss.NewStyle().Foreground(colDim)
	errStyle   = lipgloss.NewStyle().Foreground(colError)
	errBold   = lipgloss.NewStyle().Foreground(colError).Bold(true)
	greenBold = lipgloss.NewStyle().Foreground(colRunning).Bold(true)
	svcStyle  = lipgloss.NewStyle().Foreground(colDim)
	chipActive = lipgloss.NewStyle().
			Foreground(colDarkBg).
			Background(colAccent).
			Bold(true).
			Padding(0, 1)
	chipInactive = lipgloss.NewStyle().
			Foreground(colDim).
			Background(colFormBg).
			Padding(0, 1)
	toggleOn  = lipgloss.NewStyle().Foreground(colRunning).Bold(true)
	toggleOff = lipgloss.NewStyle().Foreground(colDim)
	presetNum = lipgloss.NewStyle().Foreground(colPending).Bold(true)
	presetNm  = lipgloss.NewStyle().Foreground(colPending).Bold(true)
	presetDsc = lipgloss.NewStyle().Foreground(colDim)
	labelSty  = lipgloss.NewStyle().Foreground(colDetailKey)
	inputSty  = lipgloss.NewStyle().Foreground(colDetailVal).
			Background(colFormBg)
	inputEmpty = lipgloss.NewStyle().Foreground(colDim).
			Background(colFormBg)
	apiBadge    = lipgloss.NewStyle().Foreground(colPurple).Bold(true)
	clientBadge = lipgloss.NewStyle().Foreground(colDim)
	warnStyle   = lipgloss.NewStyle().Foreground(colPending).Italic(true)
	hintStyle   = lipgloss.NewStyle().Foreground(colDim)
	headerCol   = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
)

// ── SCREEN 1a: Search Form (Empty) ─────────────────────────────────────────────

func renderSearchFormEmpty() string {
	const w = 100

	var lines []string
	lines = append(lines, "")

	// Time range
	lines = append(lines, "  "+secStyle.Render("TIME RANGE"))
	timeRow := "  " +
		chipInactive.Render("15m") + " " +
		chipActive.Render(" 1h ") + " " +
		chipInactive.Render(" 4h ") + " " +
		chipInactive.Render("24h ") + " " +
		chipInactive.Render(" 7d ") + " " +
		chipInactive.Render("30d ") + " " +
		chipInactive.Render("custom")
	lines = append(lines, timeRow)
	lines = append(lines, "")

	// Filters
	filterLabelW := 16
	inputW := 42
	blankInput := inputEmpty.Width(inputW).Render(strings.Repeat("\u2500", inputW))

	filterHeader := "  " + secStyle.Render("FILTERS") +
		strings.Repeat(" ", 46) +
		apiBadge.Render("API") + "  " + clientBadge.Render("local")
	lines = append(lines, filterHeader)

	addFilter := func(label string, isAPI bool) {
		badge := clientBadge.Render("      local")
		if isAPI {
			badge = apiBadge.Render("API") + "        "
		}
		line := "  " + labelSty.Render(padOrTrunc(label+":", filterLabelW)) + blankInput + "   " + badge
		lines = append(lines, line)
	}

	addFilter("Event Name", true)
	addFilter("Username", true)
	addFilter("Event Source", true)
	addFilter("Resource ARN", true)
	addFilter("Access Key", true)
	addFilter("Error Code", false)
	addFilter("Source IP", false)
	lines = append(lines, "")

	// Toggles
	lines = append(lines, "  "+secStyle.Render("TOGGLES"))
	toggleRow := "  " +
		toggleOn.Render("[x]") + " " + vStyle.Render("Write events only") +
		"    " +
		toggleOff.Render("[ ]") + " " + dimStyle.Render("Error events only")
	lines = append(lines, toggleRow)
	lines = append(lines, "")

	// Presets
	lines = append(lines, "  "+secStyle.Render("PRESETS"))
	presets := []struct {
		num, name, desc string
	}{
		{"1", "Recent Changes", "write events, last 2h"},
		{"2", "Error Investigation", "error events, last 1h"},
		{"3", "Console Logins", "ConsoleLogin events, last 7d"},
		{"4", "IAM Changes", "iam.amazonaws.com, write, last 24h"},
		{"5", "Root Activity", "Root identity, last 30d"},
		{"6", "Dangerous Ops", "write events + dangerous op filter, last 4h"},
	}
	for _, p := range presets {
		line := "  " +
			presetNum.Render("["+p.num+"]") + " " +
			presetNm.Render(padOrTrunc(p.name, 22)) +
			presetDsc.Render(p.desc)
		lines = append(lines, line)
	}
	lines = append(lines, "")

	// Data events warning
	lines = append(lines, "  "+warnStyle.Render("Data events (S3 GetObject, Lambda Invoke) require a configured trail"))
	lines = append(lines, "")

	// Action hints
	hintLine := lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top,
		hintStyle.Render("Enter: search") + "  " + hintStyle.Render("Esc: cancel"))
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── SCREEN 1b: Search Form (With Filters) ──────────────────────────────────────

func renderSearchFormFilled() string {
	const w = 100

	var lines []string
	lines = append(lines, "")

	// Time range - 7d selected
	lines = append(lines, "  "+secStyle.Render("TIME RANGE"))
	timeRow := "  " +
		chipInactive.Render("15m") + " " +
		chipInactive.Render(" 1h ") + " " +
		chipInactive.Render(" 4h ") + " " +
		chipInactive.Render("24h ") + " " +
		chipActive.Render(" 7d ") + " " +
		chipInactive.Render("30d ") + " " +
		chipInactive.Render("custom")
	lines = append(lines, timeRow)
	lines = append(lines, "")

	// Filters with some filled in
	filterLabelW := 16
	inputW := 42

	filterHeader := "  " + secStyle.Render("FILTERS") +
		strings.Repeat(" ", 46) +
		apiBadge.Render("API") + "  " + clientBadge.Render("local")
	lines = append(lines, filterHeader)

	addFilter := func(label, value string, isAPI bool) {
		badge := clientBadge.Render("      local")
		if isAPI {
			badge = apiBadge.Render("API") + "        "
		}
		var input string
		if value == "" {
			input = inputEmpty.Width(inputW).Render(strings.Repeat("\u2500", inputW))
		} else {
			filled := inputSty.Width(inputW).Render(value)
			input = filled
		}
		line := "  " + labelSty.Render(padOrTrunc(label+":", filterLabelW)) + input + "   " + badge
		lines = append(lines, line)
	}

	addFilter("Event Name", "", true)
	addFilter("Username", "deploy-bot", true)
	addFilter("Event Source", "", true)
	addFilter("Resource ARN", "", true)
	addFilter("Access Key", "", true)
	addFilter("Error Code", "", false)
	addFilter("Source IP", "", false)
	lines = append(lines, "")

	// Toggles - both off for security sweep
	lines = append(lines, "  "+secStyle.Render("TOGGLES"))
	toggleRow := "  " +
		toggleOff.Render("[ ]") + " " + dimStyle.Render("Write events only") +
		"    " +
		toggleOff.Render("[ ]") + " " + dimStyle.Render("Error events only")
	lines = append(lines, toggleRow)
	lines = append(lines, "")

	// Info about API filter selection
	lines = append(lines, "  "+dimStyle.Render("The most selective API filter will be sent to CloudTrail."))
	lines = append(lines, "  "+dimStyle.Render("All other filters are applied locally on fetched results."))
	lines = append(lines, "")

	// Action hints
	hintLine := lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top,
		hintStyle.Render("Enter: search") + "  " + hintStyle.Render("Esc: cancel"))
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── LOADING STATE ───────────────────────────────────────────────────────────────

func renderSearchLoading() string {
	const w = 100

	var lines []string
	for range 3 {
		lines = append(lines, "")
	}

	spinnerStyle := lipgloss.NewStyle().Foreground(colAccent)
	countStyle := lipgloss.NewStyle().Foreground(colRunning).Bold(true)
	lines = append(lines, "            "+spinnerStyle.Render("\u28bf")+"  "+vStyle.Render("Searching CloudTrail events..."))
	lines = append(lines, "               "+countStyle.Render("Loaded 150 events, 3 match filters"))
	lines = append(lines, "")
	lines = append(lines, "               "+dimStyle.Render("write events, last 1h"))
	lines = append(lines, "               "+dimStyle.Render("API filter: Username = deploy-bot"))
	lines = append(lines, "               "+dimStyle.Render("Client filters: Source IP"))
	lines = append(lines, "")
	lines = append(lines,
		lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top,
			hintStyle.Render("Esc: stop and show matches")))

	for range 2 {
		lines = append(lines, "")
	}

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search \u2014 Searching...", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── SCREEN 2a: Results (Write Events, Last 1h) ─────────────────────────────────

func renderResultsWriteEvents() string {
	const w = 100

	type col struct {
		title string
		width int
	}
	cols := []col{
		{"TIME", 22},
		{"EVENT NAME", 26},
		{"USER", 18},
		{"SOURCE", 24},
	}

	type row struct {
		time, event, user, source string
		isError, isRoot, isRead  bool
	}
	rows := []row{
		{"2026-03-29 14:58:31", "DeleteBucket", "admin", "s3.amazonaws.com", false, false, false},
		{"2026-03-29 14:57:12", "PutBucketPolicy", "ci-deploy", "s3.amazonaws.com", false, false, false},
		{"2026-03-29 14:55:03", "AuthorizeSecurityGroupI\u2026", "platform-bot", "ec2.amazonaws.com", false, false, false},
		{"2026-03-29 14:52:44", "RunInstances", "ci-deploy", "ec2.amazonaws.com", false, false, false},
		{"2026-03-29 14:50:19", "UpdateFunctionConfigur\u2026", "ci-deploy", "lambda.amazonaws.com", false, false, false},
		{"2026-03-29 14:47:33", "CreateAccessKey", "admin", "iam.amazonaws.com", false, false, false},
		{"2026-03-29 14:45:01", "AssumeRole", "ci-deploy", "sts.amazonaws.com", false, false, false},
		{"2026-03-29 14:42:18", "ConsoleLogin", "admin", "signin.amazonaws.com", false, false, false},
	}

	// Mark service-triggered rows (invokedBy = AWS service)
	svcRows := map[int]bool{4: true} // UpdateFunctionConfigur... triggered by cloudformation

	innerW := w - 2

	// Column headers
	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := headerCol.Render(" " + strings.Join(headerParts, " "))

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		userCell := r.user
		if svcRows[i] {
			userCell = r.user + " " + svcStyle.Render("[svc]")
		}
		cells := []string{
			padOrTrunc(r.time, cols[0].width),
			padOrTrunc(r.event, cols[1].width),
			padOrTrunc(userCell, cols[2].width),
			padOrTrunc(r.source, cols[3].width),
		}
		rowText := " " + strings.Join(cells, " ")

		switch {
		case i == 0:
			// Selected row: blue background
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		case r.event == "DeleteBucket" || strings.HasPrefix(r.event, "Terminate"):
			// Dangerous ops: red
			lines = append(lines, errStyle.Render(rowText))
		default:
			lines = append(lines, vStyle.Render(rowText))
		}
	}

	lines = append(lines, dimStyle.Render("  \u00b7 \u00b7 \u00b7 (39 more)"))
	lines = append(lines, dimStyle.Render("\u2500\u2500 M: load more \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search(47+) \u2014 write events, last 1h", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── SCREEN 2b: Results (Error Events Only) ──────────────────────────────────────

func renderResultsErrorEvents() string {
	const w = 100

	type col struct {
		title string
		width int
	}
	cols := []col{
		{"TIME", 22},
		{"EVENT NAME", 24},
		{"ERROR", 20},
		{"USER", 20},
	}

	type row struct {
		time, event, errCode, user string
	}
	rows := []row{
		{"2026-03-29 14:58:31", "PutBucketPolicy", "AccessDenied", "lambda-role"},
		{"2026-03-29 14:55:03", "AssumeRole", "AccessDenied", "ci-deploy"},
		{"2026-03-29 14:52:44", "GetSecretValue", "AccessDenied", "app-svc-role"},
		{"2026-03-29 14:50:19", "DescribeInstances", "ThrottlingExcepti\u2026", "monitoring"},
		{"2026-03-29 14:47:33", "ListBuckets", "AccessDenied", "intern-role"},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := headerCol.Render(" " + strings.Join(headerParts, " "))

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.time, cols[0].width),
			padOrTrunc(r.event, cols[1].width),
			padOrTrunc(r.errCode, cols[2].width),
			padOrTrunc(r.user, cols[3].width),
		}
		rowText := " " + strings.Join(cells, " ")

		if i == 0 {
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		} else {
			// All error events in red
			lines = append(lines, errStyle.Render(rowText))
		}
	}

	lines = append(lines, dimStyle.Render("  \u00b7 \u00b7 \u00b7 (18 more)"))
	lines = append(lines, dimStyle.Render("\u2500\u2500 M: load more \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search(23+) \u2014 errors only, last 1h", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── SCREEN 2c: Results (Root Activity) ──────────────────────────────────────────

func renderResultsRootActivity() string {
	const w = 100

	type col struct {
		title string
		width int
	}
	cols := []col{
		{"TIME", 22},
		{"EVENT NAME", 24},
		{"SOURCE", 26},
		{"SOURCE IP", 16},
	}

	type row struct {
		time, event, source, ip string
	}
	rows := []row{
		{"2026-03-25 03:14:22", "ConsoleLogin", "signin.amazonaws.com", "198.51.100.1"},
		{"2026-03-18 11:02:05", "CreateAccessKey", "iam.amazonaws.com", "198.51.100.1"},
		{"2026-03-10 22:45:33", "ConsoleLogin", "signin.amazonaws.com", "203.0.113.50"},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := headerCol.Render(" " + strings.Join(headerParts, " "))

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.time, cols[0].width),
			padOrTrunc(r.event, cols[1].width),
			padOrTrunc(r.source, cols[2].width),
			padOrTrunc(r.ip, cols[3].width),
		}
		rowText := " " + strings.Join(cells, " ")

		if i == 0 {
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		} else {
			// Root events: bold red
			lines = append(lines, errBold.Render(rowText))
		}
	}

	// Sparse results - add empty lines
	for range 4 {
		lines = append(lines, "")
	}

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search(3) \u2014 root activity, last 30d", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── SCREEN 2d: Results (Console Logins) ────────────────────────────────────────

func renderResultsConsoleLogins() string {
	const w = 100

	type col struct {
		title string
		width int
	}
	cols := []col{
		{"TIME", 22},
		{"EVENT NAME", 18},
		{"USER", 20},
		{"SOURCE IP", 16},
		{"MFA", 6},
	}

	type row struct {
		time, event, user, ip, mfa string
		isRoot                     bool
	}
	rows := []row{
		{"2026-03-29 09:15:42", "ConsoleLogin", "admin", "198.51.100.1", "Yes", false},
		{"2026-03-28 22:03:17", "ConsoleLogin", "dev-user", "10.0.1.42", "Yes", false},
		{"2026-03-28 14:45:33", "ConsoleLogin", "ci-deploy", "203.0.113.50", "No", false},
		{"2026-03-27 11:20:05", "ConsoleLogin", "intern-role", "192.0.2.100", "No", false},
		{"2026-03-25 03:14:22", "ConsoleLogin", "Root", "198.51.100.1", "Yes", true},
		{"2026-03-24 16:30:11", "ConsoleLogin", "dev-user", "10.0.1.42", "Yes", false},
		{"2026-03-23 08:55:44", "ConsoleLogin", "platform-bot", "10.0.1.42", "Yes", false},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := headerCol.Render(" " + strings.Join(headerParts, " "))

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.time, cols[0].width),
			padOrTrunc(r.event, cols[1].width),
			padOrTrunc(r.user, cols[2].width),
			padOrTrunc(r.ip, cols[3].width),
			padOrTrunc(r.mfa, cols[4].width),
		}
		rowText := " " + strings.Join(cells, " ")

		switch {
		case i == 0:
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		case r.isRoot:
			// Root login: bold red
			lines = append(lines, errBold.Render(rowText))
		case r.mfa == "No":
			// No MFA: amber warning
			lines = append(lines, warnStyle.Render(rowText))
		default:
			lines = append(lines, vStyle.Render(rowText))
		}
	}

	lines = append(lines, dimStyle.Render("  \u00b7 \u00b7 \u00b7 (5 more)"))
	lines = append(lines, dimStyle.Render("\u2500\u2500 M: load more \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search(12+) \u2014 ConsoleLogin, last 7d", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── SCREEN 3a: Event Detail (Normal Write Event) ───────────────────────────────

func renderEventDetailNormal() string {
	const w = 100
	kw := 22

	kv := func(key, val string) string {
		return "  " + kStyle.Render(padOrTrunc(key+":", kw)) + vStyle.Render(val)
	}
	sec := func(s string) string {
		return " " + secStyle.Render(s)
	}

	// Navigable resource: underlined blue
	navStyle := lipgloss.NewStyle().Foreground(colAccent).Underline(true)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, sec("Event:"))
	lines = append(lines, kv("EventName", "RunInstances"))
	lines = append(lines, kv("EventSource", "ec2.amazonaws.com"))
	lines = append(lines, kv("EventTime", "2026-03-29 14:52:44"))
	lines = append(lines, kv("AwsRegion", "us-east-1"))
	lines = append(lines, kv("ReadOnly", "false"))
	lines = append(lines, "")
	lines = append(lines, sec("Identity:"))
	lines = append(lines, kv("Type", "AssumedRole"))
	lines = append(lines, kv("PrincipalId", "AROA3XFRBF23COEXAMPLE:ci-deploy"))
	lines = append(lines, kv("Arn", "arn:aws:sts::123456789012:assumed-role/ci-deploy-role/ci-d\u2026"))
	lines = append(lines, kv("AccessKeyId", "ASIA3XFRBF23EXAMPLE"))
	lines = append(lines, kv("SourceIpAddress", "10.0.1.42"))
	lines = append(lines, kv("UserAgent", "aws-cli/2.15.0 Python/3.11.6"))
	lines = append(lines, kv("SharedEventID", "a1b2c3d4-1234-5678-abcd-example12345"))
	lines = append(lines, "")
	lines = append(lines, sec("Resources:"))
	lines = append(lines, "  "+dimStyle.Render("[1]")+" "+dimStyle.Render("AWS::EC2::Instance")+"   "+navStyle.Render("i-0abc123def456789a"))
	lines = append(lines, "")
	lines = append(lines, sec("Request Parameters:"))
	lines = append(lines, kv("instanceType", "m5.xlarge"))
	lines = append(lines, kv("imageId", "ami-0abcdef01234567"))
	lines = append(lines, kv("minCount", "2"))
	lines = append(lines, kv("maxCount", "2"))
	lines = append(lines, kv("subnetId", "subnet-0123456789abcde"))
	lines = append(lines, "")
	lines = append(lines, sec("Response:"))
	lines = append(lines, kv("instancesSet", "{items: [{instanceId: i-0abc123def456789a, ...}]}"))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search \u2014 RunInstances", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── SCREEN 3b: Event Detail (AccessDenied Error) ───────────────────────────────

func renderEventDetailError() string {
	const w = 100
	kw := 22

	kv := func(key, val string) string {
		return "  " + kStyle.Render(padOrTrunc(key+":", kw)) + vStyle.Render(val)
	}
	kvErr := func(key, val string) string {
		return "  " + kStyle.Render(padOrTrunc(key+":", kw)) + errStyle.Render(val)
	}
	sec := func(s string) string {
		return " " + secStyle.Render(s)
	}
	secErr := func(s string) string {
		return " " + errBold.Render(s)
	}

	navStyle := lipgloss.NewStyle().Foreground(colAccent).Underline(true)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, sec("Event:"))
	lines = append(lines, kv("EventName", "PutBucketPolicy"))
	lines = append(lines, kv("EventSource", "s3.amazonaws.com"))
	lines = append(lines, kv("EventTime", "2026-03-29 14:58:31"))
	lines = append(lines, kv("AwsRegion", "us-east-1"))
	lines = append(lines, kv("ReadOnly", "false"))
	lines = append(lines, "")
	lines = append(lines, sec("Identity:"))
	lines = append(lines, kv("Type", "AssumedRole"))
	lines = append(lines, kv("Arn", "arn:aws:sts::123456789012:assumed-role/lambda-role/funct\u2026"))
	lines = append(lines, kv("SourceIpAddress", "10.0.2.55"))
	lines = append(lines, kv("UserAgent", "aws-sdk-python/1.34.0"))
	crossAcctStyle := lipgloss.NewStyle().Foreground(colPending).Italic(true)
	lines = append(lines, kv("SourceAccount", "987654321098 "+crossAcctStyle.Render("(cross-account)")))
	lines = append(lines, "")

	// Error section - red
	lines = append(lines, secErr("Error:"))
	lines = append(lines, kvErr("ErrorCode", "AccessDenied"))
	lines = append(lines, kvErr("ErrorMessage", "User: arn:aws:sts::123456789012:assumed-role/lambda-rol\u2026"))
	lines = append(lines, "")

	lines = append(lines, sec("Resources:"))
	lines = append(lines, "  "+dimStyle.Render("[1]")+" "+dimStyle.Render("AWS::S3::Bucket")+"      "+navStyle.Render("prod-data-bucket"))
	lines = append(lines, "")
	lines = append(lines, sec("Request Parameters:"))
	lines = append(lines, kv("bucketName", "prod-data-bucket"))
	lines = append(lines, kv("bucketPolicy", "{\"Version\":\"2012-10-17\",\"Statement\":[...]}"))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search \u2014 PutBucketPolicy", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── SCREEN 3c: Event Detail (Root Login) ────────────────────────────────────────

func renderEventDetailRoot() string {
	const w = 100
	kw := 22

	kv := func(key, val string) string {
		return "  " + kStyle.Render(padOrTrunc(key+":", kw)) + vStyle.Render(val)
	}
	kvDanger := func(key, val string) string {
		return "  " + kStyle.Render(padOrTrunc(key+":", kw)) + errBold.Render(val)
	}
	sec := func(s string) string {
		return " " + secStyle.Render(s)
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, sec("Event:"))
	lines = append(lines, kv("EventName", "ConsoleLogin"))
	lines = append(lines, kv("EventSource", "signin.amazonaws.com"))
	lines = append(lines, kv("EventTime", "2026-03-25 03:14:22"))
	lines = append(lines, kv("AwsRegion", "us-east-1"))
	lines = append(lines, kv("ReadOnly", "false"))
	lines = append(lines, "")
	lines = append(lines, sec("Identity:"))
	lines = append(lines, kvDanger("Type", "Root"))
	lines = append(lines, kv("PrincipalId", "123456789012"))
	lines = append(lines, kv("Arn", "arn:aws:iam::123456789012:root"))
	lines = append(lines, kv("SourceIpAddress", "198.51.100.1"))
	lines = append(lines, kv("UserAgent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"))
	lines = append(lines, "")
	lines = append(lines, sec("Additional:"))
	lines = append(lines, kv("MFAUsed", greenBold.Render("Yes")))
	lines = append(lines, kv("LoginTo", "https://console.aws.amazon.com/console/home"))
	lines = append(lines, "")
	lines = append(lines, "  "+dimStyle.Render("No resources referenced"))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search \u2014 ConsoleLogin", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── HELP SCREENS ────────────────────────────────────────────────────────────────

func renderHelpSearchForm() string {
	const w = 100

	catStyle := lipgloss.NewStyle().Foreground(colHelpCat).Bold(true)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	bind := func(k, d string) string {
		return hkStyle.Render(padOrTrunc(k, 9)) + descStyle.Render(d)
	}

	colW := 22
	catRow := " " +
		catStyle.Render(padOrTrunc("SEARCH FORM", colW)) +
		catStyle.Render(padOrTrunc("GENERAL", colW)) +
		catStyle.Render(padOrTrunc("NAVIGATION", colW)) +
		catStyle.Render("PRESETS")

	type bindRow struct {
		c1, c2, c3, c4 string
	}
	bindRows := []bindRow{
		{bind("<enter>", "Search    "), bind("<ctrl-r>", "Refresh   "), bind("<tab>", "Next field "), bind("<1>", "Recent")},
		{bind("<esc>", "Cancel    "), bind("<?>", "Help      "), bind("<s-tab>", "Prev field "), bind("<2>", "Errors")},
		{bind("<W>", "Write tog "), bind("<q>", "Quit      "), bind("<h/l>", "Time range "), bind("<3>", "Logins")},
		{bind("<E>", "Error tog "), "", bind("<space>", "Toggle     "), bind("<4>", "IAM")},
		{"", "", "", bind("<5>", "Root")},
		{"", "", "", bind("<6>", "Danger")},
	}

	var lines []string
	lines = append(lines, catRow)
	lines = append(lines, "")

	for _, row := range bindRows {
		c1 := padOrTrunc(row.c1, colW)
		c2 := padOrTrunc(row.c2, colW)
		c3 := padOrTrunc(row.c3, colW)
		c4 := row.c4
		lines = append(lines, " "+c1+c2+c3+c4)
	}

	lines = append(lines, "")
	lines = append(lines,
		lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top,
			dimStyle.Render("Press any key to close")))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Help", w))
	sb.WriteString("\n")
	return sb.String()
}

func renderHelpResults() string {
	const w = 100

	catStyle := lipgloss.NewStyle().Foreground(colHelpCat).Bold(true)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	bind := func(k, d string) string {
		return hkStyle.Render(padOrTrunc(k, 9)) + descStyle.Render(d)
	}

	colW := 22
	catRow := " " +
		catStyle.Render(padOrTrunc("RESULTS", colW)) +
		catStyle.Render(padOrTrunc("GENERAL", colW)) +
		catStyle.Render(padOrTrunc("NAVIGATION", colW)) +
		catStyle.Render("COPY")

	type bindRow struct {
		c1, c2, c3, c4 string
	}
	bindRows := []bindRow{
		{bind("<enter>", "Detail    "), bind("<ctrl-r>", "Re-search "), bind("<j>", "Down       "), bind("<c>", "Event ID")},
		{bind("<esc>", "Back form "), bind("</>", "Filter    "), bind("<k>", "Up         "), bind("<C>", "Full JSON")},
		{bind("<f>", "Edit filt "), bind("<:>", "Command   "), bind("<g>", "Top        "), bind("<Y>", "All JSON")},
		{bind("<y>", "YAML      "), bind("<?>", "Help      "), bind("<G>", "Bottom     "), ""},
		{bind("<M>", "Load more "), "", bind("<N>", "Sort Name  "), ""},
		{"", "", bind("<A>", "Sort Time  "), ""},
	}

	var lines []string
	lines = append(lines, catRow)
	lines = append(lines, "")

	for _, row := range bindRows {
		c1 := padOrTrunc(row.c1, colW)
		c2 := padOrTrunc(row.c2, colW)
		c3 := padOrTrunc(row.c3, colW)
		c4 := row.c4
		lines = append(lines, " "+c1+c2+c3+c4)
	}

	lines = append(lines, "")
	lines = append(lines,
		lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top,
			dimStyle.Render("Press any key to close")))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Help", w))
	sb.WriteString("\n")
	return sb.String()
}

func renderHelpEventDetail() string {
	const w = 100

	catStyle := lipgloss.NewStyle().Foreground(colHelpCat).Bold(true)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	bind := func(k, d string) string {
		return hkStyle.Render(padOrTrunc(k, 9)) + descStyle.Render(d)
	}

	colW := 22
	catRow := " " +
		catStyle.Render(padOrTrunc("EVENT DETAIL", colW)) +
		catStyle.Render(padOrTrunc("GENERAL", colW)) +
		catStyle.Render(padOrTrunc("NAVIGATION", colW)) +
		catStyle.Render("COPY")

	type bindRow struct {
		c1, c2, c3, c4 string
	}
	bindRows := []bindRow{
		{bind("<esc>", "Back      "), bind("<ctrl-r>", "Refresh   "), bind("<j>", "Down       "), bind("<c>", "Principal")},
		{bind("<enter>", "Navigate  "), bind("<?>", "Help      "), bind("<k>", "Up         "), bind("<C>", "Full JSON")},
		{bind("<y>", "YAML      "), "", bind("<g>", "Top        "), bind("<R>", "Resource")},
		{bind("<w>", "Word wrap "), "", bind("<G>", "Bottom     "), bind("<E>", "Error msg")},
		{"", "", bind("<pgup>", "Page up    "), ""},
		{"", "", bind("<pgdn>", "Page down  "), ""},
	}

	var lines []string
	lines = append(lines, catRow)
	lines = append(lines, "")

	for _, row := range bindRows {
		c1 := padOrTrunc(row.c1, colW)
		c2 := padOrTrunc(row.c2, colW)
		c3 := padOrTrunc(row.c3, colW)
		c4 := row.c4
		lines = append(lines, " "+c1+c2+c3+c4)
	}

	lines = append(lines, "")
	lines = append(lines,
		lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top,
			dimStyle.Render("Press any key to close")))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Help", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── MAIN ────────────────────────────────────────────────────────────────────────

func main() {
	fmt.Println(divider("SCREEN 1a: Search Form (Empty, Default)"))
	fmt.Println(renderSearchFormEmpty())

	fmt.Println(divider("SCREEN 1b: Search Form (Filters Filled)"))
	fmt.Println(renderSearchFormFilled())

	fmt.Println(divider("SCREEN 1c: Loading State"))
	fmt.Println(renderSearchLoading())

	fmt.Println(divider("SCREEN 2a: Results -- Write Events, Last 1h"))
	fmt.Println(renderResultsWriteEvents())

	fmt.Println(divider("SCREEN 2b: Results -- Error Events Only"))
	fmt.Println(renderResultsErrorEvents())

	fmt.Println(divider("SCREEN 2c: Results -- Root Activity"))
	fmt.Println(renderResultsRootActivity())

	fmt.Println(divider("SCREEN 2d: Results -- Console Logins"))
	fmt.Println(renderResultsConsoleLogins())

	fmt.Println(divider("SCREEN 3a: Event Detail -- RunInstances (Normal)"))
	fmt.Println(renderEventDetailNormal())

	fmt.Println(divider("SCREEN 3b: Event Detail -- AccessDenied Error"))
	fmt.Println(renderEventDetailError())

	fmt.Println(divider("SCREEN 3c: Event Detail -- Root ConsoleLogin"))
	fmt.Println(renderEventDetailRoot())

	fmt.Println(divider("HELP: Search Form"))
	fmt.Println(renderHelpSearchForm())

	fmt.Println(divider("HELP: Results List"))
	fmt.Println(renderHelpResults())

	fmt.Println(divider("HELP: Event Detail"))
	fmt.Println(renderHelpEventDetail())
}
