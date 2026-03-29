// Resource-to-CloudTrail Navigation -- static preview mockups.
// Run with: go run ./docs/design/resource-to-cloudtrail-preview/
//
// Renders the key states of the T-key CloudTrail navigation flow:
//   1. EC2 Instance list (before pressing T) -- shows selected row
//   2. ct-search results (after pressing T on EC2) -- ARN-filtered results
//   3. IAM Role list (before pressing T) -- shows selected row
//   4. ct-search results (after pressing T on IAM Role) -- Username-filtered results
//   5. S3 Bucket detail (before pressing T) -- resource detail view
//   6. ct-search results (after pressing T on S3) -- ARN-filtered results
//   7. ct-search form (after pressing f from results) -- pre-filled filters visible
//   8. Security Group ct-search results -- change audit scenario
package main

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// -- Palette (Tokyo Night Dark) -----------------------------------------------

var (
	colHeaderFg     = lipgloss.Color("#c0caf5")
	colAccent       = lipgloss.Color("#7aa2f7")
	colDim          = lipgloss.Color("#565f89")
	colSep          = lipgloss.Color("#414868")
	colBorderNormal = lipgloss.Color("#414868")

	colRowSelected   = lipgloss.Color("#7aa2f7")
	colRowSelectedFg = lipgloss.Color("#1a1b26")

	colRunning = lipgloss.Color("#9ece6a")
	colStopped = lipgloss.Color("#f7768e")
	colPending = lipgloss.Color("#e0af68")

	colDetailKey = lipgloss.Color("#7aa2f7")
	colDetailSec = lipgloss.Color("#e0af68")
	colDetailVal = lipgloss.Color("#c0caf5")

	colHelpKey = lipgloss.Color("#9ece6a")

	colError = lipgloss.Color("#f7768e")
)

// -- Helpers ------------------------------------------------------------------

// NOTE: This rune-slicing truncation is ANSI-unsafe -- it will break strings
// containing ANSI escape sequences. For static preview this is fine. The real
// implementation MUST use ansi.Truncate() from charm.land/x/ansi.
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

// -- Header -------------------------------------------------------------------

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
	gap := innerW - leftW - rightW
	if gap < 1 {
		gap = 1
	}

	content := left + strings.Repeat(" ", gap) + rightContent
	return lipgloss.NewStyle().
		Foreground(colHeaderFg).
		Width(w).Padding(0, 1).Render(content)
}

func renderHeaderNormal(profile, region, version string, w int) string {
	right := lipgloss.NewStyle().Foreground(colDim).Render("? for help")
	return renderHeader(profile, region, version, w, right)
}

// -- Table column helpers -----------------------------------------------------

type col struct {
	title string
	width int
}

func rowColorStyle(status string) lipgloss.Style {
	v := strings.ToLower(status)
	switch {
	case strings.Contains(v, "running") || strings.Contains(v, "available") || strings.Contains(v, "active"):
		return lipgloss.NewStyle().Foreground(colRunning)
	case strings.Contains(v, "terminat"):
		return lipgloss.NewStyle().Foreground(colDim)
	case strings.Contains(v, "stop") || strings.Contains(v, "fail"):
		return lipgloss.NewStyle().Foreground(colStopped)
	case strings.Contains(v, "pend") || strings.Contains(v, "start") || strings.Contains(v, "creat"):
		return lipgloss.NewStyle().Foreground(colPending)
	default:
		return lipgloss.NewStyle().Foreground(colDetailVal)
	}
}

// -- Framed box with centered title -------------------------------------------

func renderFramedBox(lines []string, title string, w int) string {
	borderStyle := lipgloss.NewStyle().Foreground(colBorderNormal)
	innerW := w - 2

	var topBorder string
	if title == "" {
		topBorder = borderStyle.Render("\u250c" + strings.Repeat("\u2500", w-2) + "\u2510")
	} else {
		titleRendered := lipgloss.NewStyle().Foreground(colHeaderFg).Bold(true).Render(title)
		titleVis := lipgloss.Width(titleRendered)

		totalDashes := w - 2 - titleVis - 2
		if totalDashes < 2 {
			totalDashes = 2
		}
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

// -- Preview section divider --------------------------------------------------

func divider(label string) string {
	line := strings.Repeat("\u2501", 35)
	return "\n" +
		lipgloss.NewStyle().Foreground(colSep).Render(line) +
		"  " +
		lipgloss.NewStyle().Foreground(colAccent).Bold(true).Render(label) +
		"  " +
		lipgloss.NewStyle().Foreground(colSep).Render(line) +
		"\n\n"
}

// =============================================================================
// VIEW 1: EC2 Instance List (before pressing T)
// =============================================================================

func renderEC2ListBeforeT() string {
	const w = 90

	cols := []col{
		{"NAME\u2191", 20},
		{"STATUS", 12},
		{"TYPE", 10},
		{"AZ", 14},
		{"LAUNCH TIME", 18},
	}

	type ec2row struct {
		name, status, itype, az, launch string
	}
	rows := []ec2row{
		{"api-prod-01", "running", "t3.medium", "us-east-1a", "2026-03-28 09:22"},
		{"api-prod-02", "running", "t3.medium", "us-east-1b", "2026-03-28 09:22"},
		{"worker-01", "running", "m5.large", "us-east-1a", "2026-03-15 14:30"},
		{"bastion", "stopped", "t3.micro", "us-east-1a", "2026-01-10 08:00"},
		{"dev-test-03", "terminated", "t3.small", "us-east-1c", "2026-03-20 11:15"},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render(" " + strings.Join(headerParts, "  "))

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.name, cols[0].width),
			padOrTrunc(r.status, cols[1].width),
			padOrTrunc(r.itype, cols[2].width),
			padOrTrunc(r.az, cols[3].width),
			padOrTrunc(r.launch, cols[4].width),
		}
		rowText := " " + strings.Join(cells, "  ")

		if i == 0 {
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		} else {
			style := rowColorStyle(r.status)
			lines = append(lines, style.Render(rowText))
		}
	}

	lines = append(lines, "")
	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	lines = append(lines, dimStyle.Render("  . . . (37 more rows)"))
	lines = append(lines, "")
	// Key hint showing T
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	hintLine := "  " + hkStyle.Render("T") + dimStyle.Render(" CloudTrail  ") +
		hkStyle.Render("d") + dimStyle.Render(" detail  ") +
		hkStyle.Render("y") + dimStyle.Render(" yaml  ") +
		hkStyle.Render("c") + dimStyle.Render(" copy ID")
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.25.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ec2-instances(42)", w))
	sb.WriteString("\n")

	return sb.String()
}

// =============================================================================
// VIEW 2: ct-search results after pressing T on EC2 instance
// =============================================================================

func renderCTSearchEC2Results() string {
	const w = 90

	cols := []col{
		{"TIME", 22},
		{"EVENT NAME", 24},
		{"USER", 18},
		{"SOURCE", 16},
	}

	type ctRow struct {
		time, event, user, source string
	}
	rows := []ctRow{
		{"2026-03-29 06:15:42", "ModifyInstanceAttribute", "ci-deploy", "ec2.amazonaws.c\u2026"},
		{"2026-03-29 04:35:22", "StartInstances", "admin", "ec2.amazonaws.c\u2026"},
		{"2026-03-29 04:30:11", "StopInstances", "admin", "ec2.amazonaws.c\u2026"},
		{"2026-03-28 22:10:05", "ModifyInstanceAttribute", "platform-bot", "ec2.amazonaws.c\u2026"},
		{"2026-03-28 18:45:33", "CreateTags", "ci-deploy", "ec2.amazonaws.c\u2026"},
		{"2026-03-28 14:20:15", "RunInstances", "ci-deploy", "ec2.amazonaws.c\u2026"},
		{"2026-03-28 11:05:44", "AuthorizeSecurityGroup\u2026", "platform-bot", "ec2.amazonaws.c\u2026"},
		{"2026-03-28 09:22:01", "RunInstances", "ci-deploy", "ec2.amazonaws.c\u2026"},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render(" " + strings.Join(headerParts, "  "))

	normalStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	dangerEvents := map[string]bool{
		"StopInstances": true,
	}

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.time, cols[0].width),
			padOrTrunc(r.event, cols[1].width),
			padOrTrunc(r.user, cols[2].width),
			padOrTrunc(r.source, cols[3].width),
		}
		rowText := " " + strings.Join(cells, "  ")

		switch {
		case i == 0:
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		case dangerEvents[r.event]:
			lines = append(lines, lipgloss.NewStyle().Foreground(colError).Render(rowText))
		default:
			lines = append(lines, normalStyle.Render(rowText))
		}
	}

	// Fill remaining lines to show full frame
	for i := 0; i < 5; i++ {
		lines = append(lines, "")
	}

	// Bottom hint
	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	hintLine := "  " + hkStyle.Render("f") + dimStyle.Render(" edit filters  ") +
		hkStyle.Render("Enter") + dimStyle.Render(" detail  ") +
		hkStyle.Render("Esc") + dimStyle.Render(" back to ec2")
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.25.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search(8) \u2014 i-0abc123\u2026, write events, last 24h", w))
	sb.WriteString("\n")

	return sb.String()
}

// =============================================================================
// VIEW 3: IAM Role List (before pressing T)
// =============================================================================

func renderIAMRoleListBeforeT() string {
	const w = 90

	cols := []col{
		{"NAME\u2191", 32},
		{"CREATED", 20},
		{"LAST USED", 20},
	}

	type roleRow struct {
		name, created, lastUsed string
	}
	rows := []roleRow{
		{"ci-deploy-role", "2025-06-15 10:00", "2026-03-29 14:52"},
		{"lambda-api-handler-role", "2025-09-01 08:30", "2026-03-29 14:50"},
		{"admin-role", "2024-01-01 00:00", "2026-03-29 12:00"},
		{"monitoring-role", "2025-03-10 09:15", "2026-03-29 14:47"},
		{"intern-role", "2026-01-15 11:00", "2026-03-28 16:30"},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render(" " + strings.Join(headerParts, "  "))

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.name, cols[0].width),
			padOrTrunc(r.created, cols[1].width),
			padOrTrunc(r.lastUsed, cols[2].width),
		}
		rowText := " " + strings.Join(cells, "  ")

		if i == 0 {
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		} else {
			lines = append(lines, lipgloss.NewStyle().Foreground(colDetailVal).Render(rowText))
		}
	}

	lines = append(lines, "")
	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	lines = append(lines, dimStyle.Render("  . . . (151 more rows)"))
	lines = append(lines, "")
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	hintLine := "  " + hkStyle.Render("T") + dimStyle.Render(" CloudTrail  ") +
		hkStyle.Render("d") + dimStyle.Render(" detail  ") +
		hkStyle.Render("Enter") + dimStyle.Render(" policies")
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.25.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "iam-roles(156)", w))
	sb.WriteString("\n")

	return sb.String()
}

// =============================================================================
// VIEW 4: ct-search results after pressing T on IAM Role (Username filter)
// =============================================================================

func renderCTSearchIAMRoleResults() string {
	const w = 90

	// When filtering by username, show SOURCE and RESOURCE instead of USER
	cols := []col{
		{"TIME", 22},
		{"EVENT NAME", 24},
		{"SOURCE", 18},
		{"RESOURCE", 16},
	}

	type ctRow struct {
		time, event, source, resource string
	}
	rows := []ctRow{
		{"2026-03-29 14:52:44", "RunInstances", "ec2.amazonaws.com", "i-0abc123\u2026"},
		{"2026-03-29 14:50:19", "UpdateFunctionConfigu\u2026", "lambda.amazonaws\u2026", "api-handler"},
		{"2026-03-29 14:45:01", "AssumeRole", "sts.amazonaws.com", "ci-deploy\u2026"},
		{"2026-03-29 12:30:00", "PutBucketPolicy", "s3.amazonaws.com", "prod-data\u2026"},
		{"2026-03-29 10:15:22", "CreateDeployment", "codedeploy.amazo\u2026", "prod-app"},
		{"2026-03-29 08:00:11", "UpdateService", "ecs.amazonaws.com", "api/prod"},
		{"2026-03-28 22:10:45", "UpdateFunctionCode", "lambda.amazonaws\u2026", "api-handler"},
		{"2026-03-28 20:03:11", "CreateDeployment", "codedeploy.amazo\u2026", "prod-app"},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render(" " + strings.Join(headerParts, "  "))

	normalStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.time, cols[0].width),
			padOrTrunc(r.event, cols[1].width),
			padOrTrunc(r.source, cols[2].width),
			padOrTrunc(r.resource, cols[3].width),
		}
		rowText := " " + strings.Join(cells, "  ")

		if i == 0 {
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		} else {
			lines = append(lines, normalStyle.Render(rowText))
		}
	}

	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	lines = append(lines, dimStyle.Render("  . . . (15 more)"))
	lines = append(lines, dimStyle.Render("  \u2500\u2500 M: load more \u2500\u2500"))
	lines = append(lines, "")
	// IAM dual-mode hint
	iamHint := lipgloss.NewStyle().Foreground(colDim).Italic(true).
		Render("  Showing events BY ci-deploy-role. Press f to switch to events ON this role.")
	lines = append(lines, iamHint)
	lines = append(lines, "")

	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	hintLine := "  " + hkStyle.Render("f") + dimStyle.Render(" edit filters  ") +
		hkStyle.Render("Enter") + dimStyle.Render(" detail  ") +
		hkStyle.Render("Esc") + dimStyle.Render(" back to roles")
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.25.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search(23) \u2014 ci-deploy-role, write events, last 24h", w))
	sb.WriteString("\n")

	return sb.String()
}

// =============================================================================
// VIEW 5: S3 Bucket Detail (before pressing T)
// =============================================================================

func renderS3DetailBeforeT() string {
	const w = 90

	secStyle := lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	kStyle := lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	kw := 20

	kv := func(key, val string) string {
		return "  " + kStyle.Render(padOrTrunc(key, kw)) + vStyle.Render(val)
	}
	sec := func(s string) string {
		return " " + secStyle.Render(s)
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, sec("Bucket:"))
	lines = append(lines, kv("Name:", "prod-data-bucket"))
	lines = append(lines, kv("Region:", "us-east-1"))
	lines = append(lines, kv("CreationDate:", "2025-03-15 10:30:00"))
	lines = append(lines, kv("Versioning:", "Enabled"))
	lines = append(lines, kv("Encryption:", "AES256"))
	lines = append(lines, kv("PublicAccess:", "All blocked"))
	lines = append(lines, "")
	lines = append(lines, sec("Tags:"))
	lines = append(lines, kv("Environment:", "production"))
	lines = append(lines, kv("Team:", "platform"))
	lines = append(lines, kv("CostCenter:", "engineering"))
	lines = append(lines, "")

	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	hintLine := "  " + hkStyle.Render("T") + dimStyle.Render(" CloudTrail  ") +
		hkStyle.Render("y") + dimStyle.Render(" yaml  ") +
		hkStyle.Render("Enter") + dimStyle.Render(" objects  ") +
		hkStyle.Render("c") + dimStyle.Render(" copy name")
	lines = append(lines, hintLine)
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.25.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "s3 \u2014 prod-data-bucket", w))
	sb.WriteString("\n")

	return sb.String()
}

// =============================================================================
// VIEW 6: ct-search results after pressing T on S3 bucket
// =============================================================================

func renderCTSearchS3Results() string {
	const w = 90

	cols := []col{
		{"TIME", 22},
		{"EVENT NAME", 22},
		{"USER", 18},
		{"ERROR", 16},
	}

	type ctRow struct {
		time, event, user, err string
	}
	rows := []ctRow{
		{"2026-03-29 14:58:31", "PutBucketPolicy", "lambda-role", "AccessDenied"},
		{"2026-03-29 11:20:05", "PutBucketTagging", "admin", ""},
		{"2026-03-28 22:45:12", "PutBucketVersioning", "platform-bot", ""},
		{"2026-03-28 16:10:33", "PutBucketEncryption", "ci-deploy", ""},
		{"2026-03-28 09:30:00", "CreateBucket", "ci-deploy", ""},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render(" " + strings.Join(headerParts, "  "))

	normalStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	errorStyle := lipgloss.NewStyle().Foreground(colError)

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.time, cols[0].width),
			padOrTrunc(r.event, cols[1].width),
			padOrTrunc(r.user, cols[2].width),
			padOrTrunc(r.err, cols[3].width),
		}
		rowText := " " + strings.Join(cells, "  ")

		switch {
		case i == 0:
			// First row has an error -- show error style with selected bg
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		case r.err != "":
			lines = append(lines, errorStyle.Render(rowText))
		default:
			lines = append(lines, normalStyle.Render(rowText))
		}
	}

	// Fill remaining space
	for i := 0; i < 7; i++ {
		lines = append(lines, "")
	}

	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	hintLine := "  " + hkStyle.Render("f") + dimStyle.Render(" edit filters  ") +
		hkStyle.Render("Enter") + dimStyle.Render(" detail  ") +
		hkStyle.Render("Esc") + dimStyle.Render(" back to s3 detail")
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.25.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search(5) \u2014 prod-data-bucket, write events, last 24h", w))
	sb.WriteString("\n")

	return sb.String()
}

// =============================================================================
// VIEW 7: ct-search form (pre-filled, after pressing f from results)
// =============================================================================

func renderCTSearchPreFilledForm() string {
	const w = 90

	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	labelStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	valueStyle := lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	chipOnStyle := lipgloss.NewStyle().Foreground(colRowSelectedFg).Background(colAccent).Bold(true)
	chipOffStyle := lipgloss.NewStyle().Foreground(colDim)
	toggleOnStyle := lipgloss.NewStyle().Foreground(colRunning).Bold(true)
	toggleOffStyle := lipgloss.NewStyle().Foreground(colDim)
	apiStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")).Bold(true)
	localStyle := dimStyle
	breadcrumbStyle := lipgloss.NewStyle().Foreground(colDim).Italic(true)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)

	// Filter field label width
	lw := 16
	fw := 38 // field width

	filterLine := func(label, value, badge string) string {
		var valRendered string
		if value == "" {
			valRendered = dimStyle.Render(strings.Repeat("_", fw))
		} else {
			padding := fw - len(value)
			if padding < 0 {
				padding = 0
			}
			valRendered = valueStyle.Render(value) + dimStyle.Render(strings.Repeat("_", padding))
		}
		var badgeRendered string
		if badge == "API" {
			badgeRendered = apiStyle.Render("API")
		} else {
			badgeRendered = localStyle.Render("local")
		}
		return "  " + labelStyle.Render(padOrTrunc(label, lw)) + valRendered + "   " + badgeRendered
	}

	var lines []string
	lines = append(lines, "")

	// Time range
	lines = append(lines, "  "+sectionStyle.Render("TIME RANGE"))
	chipRow := "  " +
		chipOffStyle.Render("[15m]") + " " +
		chipOffStyle.Render("[ 1h ]") + " " +
		chipOffStyle.Render("[ 4h ]") + " " +
		chipOnStyle.Render(" 24h ") + " " +
		chipOffStyle.Render("[ 7d ]") + " " +
		chipOffStyle.Render("[30d ]") + " " +
		chipOffStyle.Render("[custom]")
	lines = append(lines, chipRow)

	lines = append(lines, "")

	// Filters
	badgeHeader := strings.Repeat(" ", 46) + apiStyle.Render("API") + "  " + localStyle.Render("local")
	lines = append(lines, "  "+sectionStyle.Render("FILTERS")+badgeHeader)
	lines = append(lines, filterLine("Event Name:", "", "API"))
	lines = append(lines, filterLine("Username:", "", "API"))
	lines = append(lines, filterLine("Event Source:", "", "API"))
	lines = append(lines, filterLine("Resource ARN:", "arn:aws:s3:::prod-data-bucket", "API"))
	lines = append(lines, filterLine("Access Key:", "", "API"))
	lines = append(lines, filterLine("Error Code:", "", "local"))
	lines = append(lines, filterLine("Source IP:", "", "local"))

	lines = append(lines, "")

	// Toggles
	lines = append(lines, "  "+sectionStyle.Render("TOGGLES"))
	toggleRow := "  " + toggleOnStyle.Render("[x] Write events only") +
		"    " + toggleOffStyle.Render("[ ] Error events only")
	lines = append(lines, toggleRow)

	lines = append(lines, "")

	// Breadcrumb
	lines = append(lines, "  "+breadcrumbStyle.Render("Navigated from: s3 / prod-data-bucket"))

	lines = append(lines, "")

	// Action hints
	hintLine := lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top,
		hkStyle.Render("Enter")+dimStyle.Render(": search  ")+
			hkStyle.Render("Esc")+dimStyle.Render(": back to results"))
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.25.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search", w))
	sb.WriteString("\n")

	return sb.String()
}

// =============================================================================
// VIEW 8: Security Group ct-search results (change audit scenario)
// =============================================================================

func renderCTSearchSGResults() string {
	const w = 90

	cols := []col{
		{"TIME", 22},
		{"EVENT NAME", 26},
		{"USER", 16},
		{"SOURCE", 16},
	}

	type ctRow struct {
		time, event, user, source string
		isDanger                  bool
	}
	rows := []ctRow{
		{"2026-03-29 14:55:03", "AuthorizeSecurityGroupIn\u2026", "platform-bot", "ec2.amazonaws.\u2026", false},
		{"2026-03-29 11:30:22", "RevokeSecurityGroupIngr\u2026", "admin", "ec2.amazonaws.\u2026", true},
		{"2026-03-28 22:15:11", "AuthorizeSecurityGroupIn\u2026", "ci-deploy", "ec2.amazonaws.\u2026", false},
		{"2026-03-28 16:40:33", "ModifySecurityGroupRule\u2026", "platform-bot", "ec2.amazonaws.\u2026", false},
		{"2026-03-28 10:05:44", "CreateSecurityGroup", "ci-deploy", "ec2.amazonaws.\u2026", false},
		{"2026-03-28 08:20:00", "AuthorizeSecurityGroupIn\u2026", "platform-bot", "ec2.amazonaws.\u2026", false},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render(" " + strings.Join(headerParts, "  "))

	normalStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	dangerStyle := lipgloss.NewStyle().Foreground(colError)

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.time, cols[0].width),
			padOrTrunc(r.event, cols[1].width),
			padOrTrunc(r.user, cols[2].width),
			padOrTrunc(r.source, cols[3].width),
		}
		rowText := " " + strings.Join(cells, "  ")

		switch {
		case i == 0:
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		case r.isDanger:
			lines = append(lines, dangerStyle.Render(rowText))
		default:
			lines = append(lines, normalStyle.Render(rowText))
		}
	}

	for i := 0; i < 6; i++ {
		lines = append(lines, "")
	}

	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	hintLine := "  " + hkStyle.Render("f") + dimStyle.Render(" edit filters  ") +
		hkStyle.Render("Enter") + dimStyle.Render(" detail  ") +
		hkStyle.Render("Esc") + dimStyle.Render(" back to sg")
	lines = append(lines, hintLine)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.25.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-search(6) \u2014 sg-0abc123\u2026, write events, last 24h", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- Main ---------------------------------------------------------------------

func main() {
	fmt.Println(divider("1: EC2 Instance List (cursor on api-prod-01, T hint visible)"))
	fmt.Println(renderEC2ListBeforeT())

	fmt.Println(divider("2: After pressing T -- CloudTrail events for EC2 instance"))
	fmt.Println(renderCTSearchEC2Results())

	fmt.Println(divider("3: IAM Role List (cursor on ci-deploy-role, T hint visible)"))
	fmt.Println(renderIAMRoleListBeforeT())

	fmt.Println(divider("4: After pressing T -- CloudTrail events BY ci-deploy-role"))
	fmt.Println(renderCTSearchIAMRoleResults())

	fmt.Println(divider("5: S3 Bucket Detail (T hint visible in detail view)"))
	fmt.Println(renderS3DetailBeforeT())

	fmt.Println(divider("6: After pressing T -- CloudTrail events for S3 bucket"))
	fmt.Println(renderCTSearchS3Results())

	fmt.Println(divider("7: Editing filters (f from results) -- form shows pre-filled ARN"))
	fmt.Println(renderCTSearchPreFilledForm())

	fmt.Println(divider("8: Security Group change audit -- who modified this SG?"))
	fmt.Println(renderCTSearchSGResults())
}
