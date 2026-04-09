// preview-pagination renders static mockups of paginated resource list states.
// Run with: go run ./cmd/preview-pagination/
//
// Shows all states from docs/design/pagination-interactions.md:
//  1. Truncated list (200+ items, first page)
//  2. Loading more (in progress)
//  3. All pages loaded (no truncation)
//  4. Filter active on truncated list
//  5. Filter active, zero matches, truncated
//  6. After refresh (Ctrl+R)
//  7. Non-paginated list (legacy fetcher, no changes)
//  8. Sorted truncated list
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

	colFilter  = lipgloss.Color("#e0af68")
	colSuccess = lipgloss.Color("#9ece6a")
)

// -- Helpers ------------------------------------------------------------------

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
	gap := max(innerW-leftW-rightW, 1)

	content := left + strings.Repeat(" ", gap) + rightContent
	return lipgloss.NewStyle().
		Foreground(colHeaderFg).
		Width(w).Padding(0, 1).Render(content)
}

func renderHeaderNormal(w int) string {
	right := lipgloss.NewStyle().Foreground(colDim).Render("? for help")
	return renderHeader("prod", "us-east-1", "0.5.0", w, right)
}

func renderHeaderFilter(filterText string, w int) string {
	right := lipgloss.NewStyle().Foreground(colFilter).Bold(true).Render("/"+filterText) +
		lipgloss.NewStyle().Foreground(colFilter).Render("\u2588")
	return renderHeader("prod", "us-east-1", "0.5.0", w, right)
}

func renderHeaderFlash(msg string, w int) string {
	right := lipgloss.NewStyle().Foreground(colSuccess).Bold(true).Render(msg)
	return renderHeader("prod", "us-east-1", "0.5.0", w, right)
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

// -- Section divider ----------------------------------------------------------

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

// -- Column / row helpers -----------------------------------------------------

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
		return lipgloss.NewStyle().Foreground(colHeaderFg)
	}
}

// -- CloudTrail row data used across multiple views ---------------------------

type ctRow struct {
	event, time, user, source string
}

var ctCols = []col{
	{"EVENT NAME", 22},
	{"TIME", 21},
	{"USER", 16},
	{"SOURCE", 18},
}

var ctColsSorted = []col{
	{"EVENT NAME\u2191", 22},
	{"TIME", 21},
	{"USER", 16},
	{"SOURCE", 18},
}

var ctColsTimeSorted = []col{
	{"EVENT NAME", 22},
	{"TIME\u2193", 21},
	{"USER", 16},
	{"SOURCE", 18},
}

var ctRows = []ctRow{
	{"AssumeRole", "2024-03-17 08:58", "ci-deploy", "sts"},
	{"ConsoleLogin", "2024-03-17 09:00", "admin", "signin"},
	{"CreateBucket", "2024-03-17 08:12", "admin", "s3"},
	{"CreateFunction", "2024-03-16 14:22", "ci-deploy", "lambda"},
	{"DeleteBucket", "2024-03-17 07:55", "admin", "s3"},
	{"DescribeInstances", "2024-03-17 08:45", "monitoring", "ec2"},
	{"GetSecretValue", "2024-03-17 08:30", "app-service", "secretsmanager"},
	{"PutBucketPolicy", "2024-03-16 22:30", "ci-deploy", "s3"},
	{"RunInstances", "2024-03-16 20:15", "ci-deploy", "ec2"},
	{"StopInstances", "2024-03-16 19:00", "admin", "ec2"},
}

func renderCTHeader(cols []col) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = padOrTrunc(c.title, c.width)
	}
	return lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).
		Render(" " + strings.Join(parts, "  "))
}

func renderCTRow(cols []col, r ctRow, selected bool, innerW int) string {
	cells := []string{
		padOrTrunc(r.event, cols[0].width),
		padOrTrunc(r.time, cols[1].width),
		padOrTrunc(r.user, cols[2].width),
		padOrTrunc(r.source, cols[3].width),
	}
	rowText := " " + strings.Join(cells, "  ")

	if selected {
		return lipgloss.NewStyle().
			Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
			Width(innerW).Render(rowText)
	}
	return lipgloss.NewStyle().Foreground(colHeaderFg).Render(rowText)
}

func dimHint(text string) string {
	return lipgloss.NewStyle().Foreground(colDim).Render(text)
}

// -- STATE 1: Truncated list (200+, first page) ------------------------------

func renderTruncatedList() string {
	const w = 84
	innerW := w - 2

	var lines []string
	lines = append(lines, renderCTHeader(ctColsSorted))
	for i, r := range ctRows {
		lines = append(lines, renderCTRow(ctColsSorted, r, i == 0, innerW))
	}
	lines = append(lines, dimHint("\u2500\u2500 M: load more \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(200+)", w))
	return sb.String()
}

// -- STATE 2: Loading more (in progress) -------------------------------------

func renderLoadingMore() string {
	const w = 84
	innerW := w - 2

	var lines []string
	lines = append(lines, renderCTHeader(ctColsSorted))
	for i, r := range ctRows {
		lines = append(lines, renderCTRow(ctColsSorted, r, i == 0, innerW))
	}
	lines = append(lines, dimHint("\u2500\u2500 loading... \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(200+ loading...)", w))
	return sb.String()
}

// -- STATE 3: All pages loaded -----------------------------------------------

func renderAllLoaded() string {
	const w = 84
	innerW := w - 2

	var lines []string
	lines = append(lines, renderCTHeader(ctColsSorted))
	for i, r := range ctRows {
		lines = append(lines, renderCTRow(ctColsSorted, r, i == 0, innerW))
	}
	// No load-more hint -- all data loaded
	lines = append(lines, dimHint("  ... (1837 more rows)"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(1847)", w))
	return sb.String()
}

// -- STATE 4: Filter active on truncated list --------------------------------

func renderFilteredTruncated() string {
	const w = 84
	innerW := w - 2

	filteredRows := []ctRow{
		{"CreateBucket", "2024-03-17 08:12", "admin", "s3"},
		{"DeleteBucket", "2024-03-17 07:55", "admin", "s3"},
		{"PutBucketPolicy", "2024-03-16 22:30", "ci-deploy", "s3"},
	}

	var lines []string
	lines = append(lines, renderCTHeader(ctCols))
	for i, r := range filteredRows {
		lines = append(lines, renderCTRow(ctCols, r, i == 0, innerW))
	}
	// Empty rows to show sparse result
	for range 6 {
		lines = append(lines, "")
	}
	lines = append(lines, dimHint("\u2500\u2500 M: load more (filter applies to loaded data only) \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderFilter("bucket", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(3/200+)", w))
	return sb.String()
}

// -- STATE 5: Filter active, zero matches, truncated -------------------------

func renderFilteredZeroMatches() string {
	const w = 84

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(colHeaderFg).Render(" No resources found"))
	for range 4 {
		lines = append(lines, "")
	}
	lines = append(lines, dimHint("\u2500\u2500 M: load more (filter applies to loaded data only) \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderFilter("xyz123abc", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(0/200+)", w))
	return sb.String()
}

// -- STATE 6: After refresh (Ctrl+R) -----------------------------------------

func renderAfterRefresh() string {
	const w = 84
	innerW := w - 2

	refreshedRows := []ctRow{
		{"ConsoleLogin", "2024-03-17 09:05", "admin", "signin"},
		{"AssumeRole", "2024-03-17 09:03", "ci-deploy", "sts"},
		{"PutObject", "2024-03-17 09:01", "app-service", "s3"},
		{"DescribeInstances", "2024-03-17 09:00", "monitoring", "ec2"},
		{"GetSecretValue", "2024-03-17 08:58", "app-service", "secretsmanager"},
		{"CreateLogStream", "2024-03-17 08:55", "lambda-exec", "logs"},
		{"InvokeFunction", "2024-03-17 08:52", "api-gw", "lambda"},
		{"AssumeRole", "2024-03-17 08:50", "ci-deploy", "sts"},
		{"DescribeAlarms", "2024-03-17 08:48", "monitoring", "cloudwatch"},
		{"GetParameter", "2024-03-17 08:45", "app-service", "ssm"},
	}

	var lines []string
	lines = append(lines, renderCTHeader(ctCols))
	for i, r := range refreshedRows {
		lines = append(lines, renderCTRow(ctCols, r, i == 0, innerW))
	}
	lines = append(lines, dimHint("\u2500\u2500 M: load more \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderFlash("Refreshing...", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(200+)", w))
	return sb.String()
}

// -- STATE 7: Non-paginated list (legacy fetcher) ----------------------------

func renderNonPaginated() string {
	const w = 84

	type ec2Row struct {
		name, status, itype, az, launch string
	}
	ec2Cols := []col{
		{"NAME\u2191", 20},
		{"STATUS", 11},
		{"TYPE", 11},
		{"AZ", 14},
		{"LAUNCH TIME", 17},
	}
	ec2Rows := []ec2Row{
		{"api-prod-01", "running", "t3.medium", "us-east-1a", "2024-01-15 09:22"},
		{"api-prod-02", "running", "t3.medium", "us-east-1b", "2024-01-15 09:25"},
		{"worker-01", "running", "t3.large", "us-east-1a", "2024-01-10 14:30"},
		{"worker-02", "pending", "t3.large", "us-east-1b", "2024-03-17 08:00"},
		{"bastion", "running", "t2.micro", "us-east-1a", "2023-11-01 10:00"},
		{"old-worker", "stopped", "t3.medium", "us-east-1c", "2023-06-20 16:45"},
	}

	innerW := w - 2

	parts := make([]string, len(ec2Cols))
	for i, c := range ec2Cols {
		parts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().Foreground(colAccent).Bold(true).
		Render(" " + strings.Join(parts, "  "))

	var lines []string
	lines = append(lines, headerLine)
	for i, r := range ec2Rows {
		cells := []string{
			padOrTrunc(r.name, ec2Cols[0].width),
			padOrTrunc(r.status, ec2Cols[1].width),
			padOrTrunc(r.itype, ec2Cols[2].width),
			padOrTrunc(r.az, ec2Cols[3].width),
			padOrTrunc(r.launch, ec2Cols[4].width),
		}
		rowText := " " + strings.Join(cells, "  ")
		if i == 0 {
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		} else {
			lines = append(lines, rowColorStyle(r.status).Render(rowText))
		}
	}
	// No load-more hint
	lines = append(lines, dimHint("  ... (36 more rows)"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ec2(42)", w))
	return sb.String()
}

// -- STATE 8: Sorted truncated list ------------------------------------------

func renderSortedTruncated() string {
	const w = 84
	innerW := w - 2

	timeSortedRows := []ctRow{
		{"ConsoleLogin", "2024-03-17 09:00", "admin", "signin"},
		{"AssumeRole", "2024-03-17 08:58", "ci-deploy", "sts"},
		{"DescribeInstances", "2024-03-17 08:45", "monitoring", "ec2"},
		{"GetSecretValue", "2024-03-17 08:30", "app-service", "secretsmanager"},
		{"CreateBucket", "2024-03-17 08:12", "admin", "s3"},
		{"DeleteBucket", "2024-03-17 07:55", "admin", "s3"},
		{"PutBucketPolicy", "2024-03-16 22:30", "ci-deploy", "s3"},
		{"RunInstances", "2024-03-16 20:15", "ci-deploy", "ec2"},
		{"StopInstances", "2024-03-16 19:00", "admin", "ec2"},
		{"CreateFunction", "2024-03-16 14:22", "ci-deploy", "lambda"},
	}

	var lines []string
	lines = append(lines, renderCTHeader(ctColsTimeSorted))
	for i, r := range timeSortedRows {
		lines = append(lines, renderCTRow(ctColsTimeSorted, r, i == 0, innerW))
	}
	lines = append(lines, dimHint("\u2500\u2500 M: load more \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(200+)", w))
	return sb.String()
}

// -- STATE 9: Filter + Load More result (count updated) ----------------------

func renderFilterAfterLoadMore() string {
	const w = 84
	innerW := w - 2

	filteredRows := []ctRow{
		{"CreateBucket", "2024-03-17 08:12", "admin", "s3"},
		{"DeleteBucket", "2024-03-17 07:55", "admin", "s3"},
		{"PutBucketPolicy", "2024-03-16 22:30", "ci-deploy", "s3"},
		{"ListBuckets", "2024-03-15 11:00", "monitoring", "s3"},
		{"GetBucketAcl", "2024-03-14 09:22", "audit", "s3"},
	}

	var lines []string
	lines = append(lines, renderCTHeader(ctCols))
	for i, r := range filteredRows {
		lines = append(lines, renderCTRow(ctCols, r, i == 0, innerW))
	}
	for range 4 {
		lines = append(lines, "")
	}
	lines = append(lines, dimHint("\u2500\u2500 M: load more (filter applies to loaded data only) \u2500\u2500"))

	var sb strings.Builder
	sb.WriteString(renderHeaderFilter("bucket", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(5/400+)", w))
	return sb.String()
}

// -- STATE 10: Filter active, all loaded -------------------------------------

func renderFilterAllLoaded() string {
	const w = 84
	innerW := w - 2

	filteredRows := []ctRow{
		{"CreateBucket", "2024-03-17 08:12", "admin", "s3"},
		{"DeleteBucket", "2024-03-17 07:55", "admin", "s3"},
		{"PutBucketPolicy", "2024-03-16 22:30", "ci-deploy", "s3"},
		{"ListBuckets", "2024-03-15 11:00", "monitoring", "s3"},
		{"GetBucketAcl", "2024-03-14 09:22", "audit", "s3"},
		{"PutBucketLogging", "2024-03-13 16:45", "admin", "s3"},
		{"GetBucketPolicy", "2024-03-13 14:00", "ci-deploy", "s3"},
		{"DeleteBucketPolicy", "2024-03-12 20:30", "admin", "s3"},
		{"CreateBucket", "2024-03-12 10:15", "ci-deploy", "s3"},
		{"PutBucketVersioning", "2024-03-11 08:00", "admin", "s3"},
	}

	var lines []string
	lines = append(lines, renderCTHeader(ctCols))
	for i, r := range filteredRows {
		lines = append(lines, renderCTRow(ctCols, r, i == 0, innerW))
	}
	lines = append(lines, dimHint("  ... (2 more matching rows)"))
	// No load-more hint -- all pages loaded

	var sb strings.Builder
	sb.WriteString(renderHeaderFilter("bucket", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ct-events(12/1847)", w))
	return sb.String()
}

// -- main ---------------------------------------------------------------------

func main() {
	fmt.Println(divider("STATE 1: Truncated List (200+, page 1)"))
	fmt.Println(renderTruncatedList())

	fmt.Println(divider("STATE 2: Loading More (M pressed)"))
	fmt.Println(renderLoadingMore())

	fmt.Println(divider("STATE 3: All Pages Loaded (1847 total)"))
	fmt.Println(renderAllLoaded())

	fmt.Println(divider("STATE 4: Filter Active + Truncated"))
	fmt.Println(renderFilteredTruncated())

	fmt.Println(divider("STATE 5: Filter Active, Zero Matches"))
	fmt.Println(renderFilteredZeroMatches())

	fmt.Println(divider("STATE 6: After Refresh (Ctrl+R)"))
	fmt.Println(renderAfterRefresh())

	fmt.Println(divider("STATE 7: Non-Paginated List (legacy)"))
	fmt.Println(renderNonPaginated())

	fmt.Println(divider("STATE 8: Sorted + Truncated"))
	fmt.Println(renderSortedTruncated())

	fmt.Println(divider("STATE 9: Filter + After Load More"))
	fmt.Println(renderFilterAfterLoadMore())

	fmt.Println(divider("STATE 10: Filter + All Loaded"))
	fmt.Println(renderFilterAllLoaded())
}
