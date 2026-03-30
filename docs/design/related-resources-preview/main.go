// Related Resources View -- static preview mockups.
// Run with: go run ./docs/design/related-resources-preview/
//
// Renders all key states of the related-resources list using Lipgloss v2.
// The related-resources view reuses the EXACT main menu pattern:
//   - Resource type name on the left
//   - Optional count in parentheses inline after the name (cheap lookups only)
//   - Entire row dimmed if unavailable (cursor skips)
//   - Normal rendering if available
//   - NO "Checking...", NO "Unavailable", NO "Search >", NO spinners
//
// Mockups:
//   1. List -- Initial load (all rows dim, checks in progress)
//   2. List -- Partially loaded (some checks complete)
//   3. List -- Fully loaded (120 cols)
//   4. List -- Fully loaded (80 cols)
//   5. List -- Filter active
//   6. Smart Enter: filtered resource list (multiple results)
//   7. Smart Enter: direct to detail (single result)
//   8. Deep navigation with depth indicator (VPC, 18 types)
//   9. All unavailable state
//  10. Long list with scroll indicator (27 types, 80 cols)
//  11. Help screen for related list
package main

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// -- Palette (Tokyo Night Dark) --

var (
	colHeaderFg     = lipgloss.Color("#c0caf5")
	colAccent       = lipgloss.Color("#7aa2f7")
	colDim          = lipgloss.Color("#565f89")
	colSep          = lipgloss.Color("#414868")
	colBorderNormal = lipgloss.Color("#414868")

	colRowSelected   = lipgloss.Color("#7aa2f7")
	colRowSelectedFg = lipgloss.Color("#1a1b26")

	colGreen  = lipgloss.Color("#9ece6a")
	colYellow = lipgloss.Color("#e0af68")

	colDetailKey = lipgloss.Color("#7aa2f7")
	colDetailSec = lipgloss.Color("#e0af68")
	colDetailVal = lipgloss.Color("#c0caf5")

	colHelpKey = lipgloss.Color("#9ece6a")
	colHelpCat = lipgloss.Color("#e0af68")
)

// -- Styles --

var (
	cellNormal   = lipgloss.NewStyle().Foreground(colHeaderFg)
	cellDim      = lipgloss.NewStyle().Foreground(colDim)
	cellSelected = lipgloss.NewStyle().Foreground(colRowSelectedFg).Background(colRowSelected).Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(colDim)
	secStyle     = lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	kStyle       = lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle       = lipgloss.NewStyle().Foreground(colDetailVal)
	greenStyle   = lipgloss.NewStyle().Foreground(colGreen)
	helpKeyStyle = lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	helpCatStyle = lipgloss.NewStyle().Foreground(colHelpCat).Bold(true)
)

// -- Helpers --

func padOrTrunc(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		if w <= 1 {
			r := []rune(s)
			if len(r) > w {
				return string(r[:w])
			}
			return s
		}
		r := []rune(s)
		if len(r) > w-1 {
			return string(r[:w-1]) + "\u2026"
		}
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

func pad(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

// -- Header --

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

func renderHeaderDepth(depth int, profile, region string, w int) string {
	accent := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render("a9s")
	depthStr := lipgloss.NewStyle().
		Foreground(colDim).Render(fmt.Sprintf(" [%d]", depth))
	ctx := lipgloss.NewStyle().
		Foreground(colHeaderFg).Bold(true).
		Render("  " + profile + ":" + region)

	left := accent + depthStr + ctx
	right := lipgloss.NewStyle().Foreground(colDim).Render("? for help")
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)

	innerW := w - 2
	gap := innerW - leftW - rightW
	if gap < 1 {
		gap = 1
	}

	content := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().
		Foreground(colHeaderFg).
		Width(w).Padding(0, 1).Render(content)
}

func renderHeaderFilter(profile, region, version string, w int, filterText string) string {
	right := lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render("/" + filterText + "\u2588")
	return renderHeader(profile, region, version, w, right)
}

// -- Framed box with centered title in top border --

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

// -- Divider between sections --

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

// -- Row state --
// Only three visual states, matching the main menu exactly.

type rowState int

const (
	rowAvailable rowState = iota // normal text, has related resources
	rowDim                       // dimmed, no related resources or not yet checked
	rowSelected                  // blue background, cursor is here
)

type listRow struct {
	label string
	count string // "(3)", "(1)", "(20+)" for cheap lookups; "" for expensive/no-count
	state rowState
}

// renderListRow renders a single row: label left-aligned with count inline after the name.
// Matches the main menu pattern exactly: "    Security Groups (3)" left-aligned, nothing on the right.
func renderListRow(r listRow, innerW int) string {
	indent := "    "

	// Build the text: "    Label" or "    Label (N)"
	text := indent + r.label
	if r.count != "" {
		text += " " + r.count
	}

	switch r.state {
	case rowSelected:
		return cellSelected.Render(pad(text, innerW))
	case rowDim:
		return cellDim.Render(pad(text, innerW))
	default: // rowAvailable
		return cellNormal.Render(pad(text, innerW))
	}
}

// renderList renders a complete list of rows inside a framed box
func renderList(rows []listRow, title string, w int, extraLines []string) string {
	innerW := w - 2
	var lines []string
	for _, r := range rows {
		lines = append(lines, renderListRow(r, innerW))
	}
	if len(extraLines) > 0 {
		lines = append(lines, extraLines...)
	}
	lines = append(lines, "") // bottom padding
	return renderFramedBox(lines, title, w)
}

// -- Mockup 1: Initial Load -- all rows dim (checks in progress) --

func mockInitialLoad() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	rows := []listRow{
		{label: "Security Groups", state: rowDim},
		{label: "VPC", state: rowDim},
		{label: "Subnet", state: rowDim},
		{label: "Elastic IPs", state: rowDim},
		{label: "Network Interfaces", state: rowDim},
		{label: "Auto Scaling Groups", state: rowDim},
		{label: "Target Groups", state: rowDim},
		{label: "CloudWatch Alarms", state: rowDim},
		{label: "IAM Role", state: rowDim},
		{label: "EKS Node Groups", state: rowDim},
		{label: "Elastic Beanstalk", state: rowDim},
		{label: "CloudTrail Events", state: rowDim},
	}

	box := renderList(rows, "related -- i-0abc123 (web-prod)", w, nil)
	return header + "\n" + box
}

// -- Mockup 2: Partially Loaded -- some forward checks resolved --

func mockPartiallyLoaded() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	rows := []listRow{
		// Forward lookups resolved (cheap -- counts shown)
		{label: "Security Groups", count: "(3)", state: rowSelected},
		{label: "VPC", count: "(1)", state: rowAvailable},
		{label: "Subnet", count: "(1)", state: rowAvailable},
		// Still checking (dim)
		{label: "Elastic IPs", state: rowDim},
		{label: "Network Interfaces", state: rowDim},
		{label: "Auto Scaling Groups", state: rowDim},
		{label: "Target Groups", state: rowDim},
		{label: "CloudWatch Alarms", state: rowDim},
		{label: "IAM Role", state: rowDim},
		{label: "EKS Node Groups", state: rowDim},
		{label: "Elastic Beanstalk", state: rowDim},
		{label: "CloudTrail Events", state: rowDim},
	}

	box := renderList(rows, "related -- i-0abc123 (web-prod)", w, nil)
	return header + "\n" + box
}

// -- Mockup 3: Fully Loaded (120 cols) --

func mockFullyLoaded120() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	rows := []listRow{
		// Forward relationships -- cheap, counts shown
		{label: "Security Groups", count: "(3)", state: rowSelected},
		{label: "VPC", count: "(1)", state: rowAvailable},
		{label: "Subnet", count: "(1)", state: rowAvailable},
		{label: "Elastic IPs", count: "(1)", state: rowAvailable},
		{label: "Network Interfaces", count: "(2)", state: rowAvailable},
		// Reverse/expensive relationships -- no counts, just available
		{label: "Auto Scaling Groups", state: rowAvailable},
		{label: "Target Groups", state: rowAvailable},
		{label: "CloudWatch Alarms", state: rowAvailable},
		{label: "IAM Role", state: rowAvailable},
		// Unavailable -- dim, no text
		{label: "EKS Node Groups", state: rowDim},
		{label: "Elastic Beanstalk", state: rowDim},
		// CloudTrail -- just a normal row
		{label: "CloudTrail Events", state: rowAvailable},
	}

	box := renderList(rows, "related -- i-0abc123 (web-prod)", w, nil)
	return header + "\n" + box
}

// -- Mockup 4: Fully Loaded (80 cols) --

func mockFullyLoaded80() string {
	w := 80
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	rows := []listRow{
		{label: "Security Groups", count: "(3)", state: rowSelected},
		{label: "VPC", count: "(1)", state: rowAvailable},
		{label: "Subnet", count: "(1)", state: rowAvailable},
		{label: "Elastic IPs", count: "(1)", state: rowAvailable},
		{label: "Network Interfaces", count: "(2)", state: rowAvailable},
		{label: "Auto Scaling Groups", state: rowAvailable},
		{label: "Target Groups", state: rowAvailable},
		{label: "CloudWatch Alarms", state: rowAvailable},
		{label: "IAM Role", state: rowAvailable},
		{label: "EKS Node Groups", state: rowDim},
		{label: "Elastic Beanstalk", state: rowDim},
		{label: "CloudTrail Events", state: rowAvailable},
	}

	box := renderList(rows, "related -- i-0abc123 (web-prod)", w, nil)
	return header + "\n" + box
}

// -- Mockup 5: Filter active --

func mockFilterActive() string {
	w := 120
	header := renderHeaderFilter("prod", "us-east-1", "3.26.0", w, "sec")

	rows := []listRow{
		{label: "Security Groups", count: "(3)", state: rowSelected},
	}

	box := renderList(rows, "related -- i-0abc123 (web-prod)", w, []string{"", ""})
	return header + "\n" + box
}

// -- Mockup 6: Filtered resource list (multiple SGs) --

func mockFilteredList() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	colHdr := lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	sel := cellSelected
	rowGreen := lipgloss.NewStyle().Foreground(colGreen)

	nameW := 24
	gidW := 26
	vpcW := 26
	descW := 38

	hdrLine := " " +
		colHdr.Render(padOrTrunc("NAME", nameW)) +
		colHdr.Render(padOrTrunc("GROUP ID", gidW)) +
		colHdr.Render(padOrTrunc("VPC ID", vpcW)) +
		colHdr.Render(padOrTrunc("DESCRIPTION", descW))

	row1 := sel.Render(pad(
		" "+padOrTrunc("web-sg", nameW)+
			padOrTrunc("sg-0abc111222333444", gidW)+
			padOrTrunc("vpc-0aaa111bbb222cc", vpcW)+
			padOrTrunc("Web server security group", descW),
		w-2))
	row2 := " " +
		rowGreen.Render(padOrTrunc("db-access-sg", nameW)) +
		rowGreen.Render(padOrTrunc("sg-0def555666777888", gidW)) +
		rowGreen.Render(padOrTrunc("vpc-0aaa111bbb222cc", vpcW)) +
		rowGreen.Render(padOrTrunc("Database access from web tier", descW))
	row3 := " " +
		rowGreen.Render(padOrTrunc("monitoring-sg", nameW)) +
		rowGreen.Render(padOrTrunc("sg-0ghi123456789012", gidW)) +
		rowGreen.Render(padOrTrunc("vpc-0aaa111bbb222cc", vpcW)) +
		rowGreen.Render(padOrTrunc("Monitoring agent inbound", descW))

	countLine := dimStyle.Render(" 3 security groups")

	lines := []string{hdrLine, row1, row2, row3, "", countLine, ""}

	box := renderFramedBox(lines, "sg-instances(3) -- i-0abc123 (web-prod)", w)
	return header + "\n" + box
}

// -- Mockup 7: Direct to detail (single VPC) --

func mockDirectDetail() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	keyColW := 22
	kvLine := func(key, val string) string {
		return " " + kStyle.Render(padOrTrunc(key+":", keyColW)) + vStyle.Render(val)
	}

	lines := []string{
		kvLine("VpcId", "vpc-0aaa111bbb222cc"),
		kvLine("CidrBlock", "10.0.0.0/16"),
		kvLine("State", greenStyle.Render("available")),
		kvLine("IsDefault", "false"),
		kvLine("DhcpOptionsId", "dopt-0abc123def456"),
		kvLine("InstanceTenancy", "default"),
		" " + secStyle.Render("Tags:"),
		"     " + dimStyle.Render("- Key: Name"),
		"       " + dimStyle.Render("Value: production-vpc"),
		"     " + dimStyle.Render("- Key: Environment"),
		"       " + dimStyle.Render("Value: prod"),
		"",
	}

	box := renderFramedBox(lines, "vpc-0aaa111bbb222cc", w)
	return header + "\n" + box
}

// -- Mockup 8: Deep navigation with depth indicator (VPC, 18 types) --

func mockDeepNavigation() string {
	w := 120
	header := renderHeaderDepth(7, "prod", "us-east-1", w)

	rows := []listRow{
		// P0 -- critical networking (forward, cheap, counts shown)
		{label: "Subnets", count: "(6)", state: rowSelected},
		{label: "Security Groups", count: "(12)", state: rowAvailable},
		{label: "Route Tables", count: "(3)", state: rowAvailable},
		// P1 -- forward networking (cheap, counts shown)
		{label: "NAT Gateways", count: "(2)", state: rowAvailable},
		{label: "Internet Gateways", count: "(1)", state: rowAvailable},
		{label: "VPC Endpoints", count: "(4)", state: rowAvailable},
		// P1 -- reverse lookups (expensive, no counts)
		{label: "Transit Gateways", state: rowAvailable},
		{label: "EC2 Instances", state: rowAvailable},
		{label: "Load Balancers", state: rowAvailable},
		{label: "Lambda Functions", state: rowAvailable},
		{label: "EKS Clusters", state: rowAvailable},
		{label: "DB Instances", state: rowAvailable},
		{label: "ElastiCache", state: rowAvailable},
		{label: "Target Groups", state: rowAvailable},
		// P2 -- unavailable (dim, no text)
		{label: "OpenSearch", state: rowDim},
		{label: "Redshift", state: rowDim},
		{label: "MSK Clusters", state: rowDim},
		// Always last
		{label: "CloudTrail Events", state: rowAvailable},
	}

	box := renderList(rows, "related -- vpc-0aaa111 (production-vpc)", w, nil)
	return header + "\n" + box
}

// -- Mockup 9: All unavailable --

func mockAllUnavailable() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	rows := []listRow{
		{label: "EC2 Instances", state: rowDim},
		{label: "Auto Scaling Groups", state: rowDim},
		{label: "EBS Snapshots", state: rowDim},
		{label: "CloudTrail Events", state: rowDim},
	}

	innerW := w - 2
	msg := dimStyle.Render("No related resources found. Press ctrl+r to refresh or esc to go back.")
	msgW := lipgloss.Width(msg)
	leftPad := (innerW - msgW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	centeredMsg := strings.Repeat(" ", leftPad) + msg

	box := renderList(rows, "related -- ami-0abc123 (my-custom-ami)", w, []string{"", centeredMsg})
	return header + "\n" + box
}

// -- Mockup 10: Long list with scroll indicator (VPC, 27 types, 80 cols) --

func mockLongListScroll() string {
	w := 80
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	// Show only the first 10 visible rows + scroll indicator
	rows := []listRow{
		{label: "Subnets", count: "(6)", state: rowSelected},
		{label: "Security Groups", count: "(12)", state: rowAvailable},
		{label: "Route Tables", count: "(3)", state: rowAvailable},
		{label: "NAT Gateways", count: "(2)", state: rowAvailable},
		{label: "Internet Gateways", count: "(1)", state: rowAvailable},
		{label: "VPC Endpoints", count: "(4)", state: rowAvailable},
		{label: "Transit Gateways", state: rowAvailable},
		{label: "EC2 Instances", state: rowAvailable},
		{label: "Load Balancers", state: rowAvailable},
		{label: "Lambda Functions", state: rowAvailable},
	}

	innerW := w - 2
	var lines []string
	for _, r := range rows {
		lines = append(lines, renderListRow(r, innerW))
	}

	// Scroll indicator
	scrollIndicator := dimStyle.Render(strings.Repeat(" ", innerW-20) + "v 17 more below")
	lines = append(lines, scrollIndicator)

	box := renderFramedBox(lines, "related -- vpc-0aaa111 (production-vpc)", w)
	return header + "\n" + box
}

// -- Mockup 11: Help screen --

func mockHelpScreen() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.26.0", w)

	colW := 27
	hk := func(key, desc string) string {
		return helpKeyStyle.Render(padOrTrunc(key, 10)) + lipgloss.NewStyle().Foreground(colHeaderFg).Render(padOrTrunc(desc, colW-10))
	}

	blank := strings.Repeat(" ", colW)

	lines := []string{
		" " + helpCatStyle.Render(padOrTrunc("RELATED", colW)) +
			helpCatStyle.Render(padOrTrunc("GENERAL", colW)) +
			helpCatStyle.Render(padOrTrunc("NAVIGATION", colW)) +
			helpCatStyle.Render(padOrTrunc("HOTKEYS", colW)),
		"",
		" " + hk("<enter>", "Open type") +
			hk("<ctrl-r>", "Refresh") +
			hk("<j>", "Down") +
			hk("<?>", "Help"),
		" " + hk("<esc>", "Go back") +
			hk("<q>", "Quit") +
			hk("<k>", "Up") +
			hk("<:>", "Command"),
		" " + blank +
			hk("</>", "Filter") +
			hk("<g>", "Top") +
			hk("<r>", "Related"),
		" " + blank +
			blank +
			hk("<G>", "Bottom") +
			blank,
		" " + blank +
			blank +
			hk("<pgdn>", "Page down") +
			blank,
		" " + blank +
			blank +
			hk("<pgup>", "Page up") +
			blank,
		"",
		lipgloss.NewStyle().Foreground(colDim).Render(
			strings.Repeat(" ", (w-30)/2) + "Press any key to close"),
		"",
	}

	box := renderFramedBox(lines, "Help", w)
	return header + "\n" + box
}

func main() {
	fmt.Println()

	// 1. Initial Load -- all dim
	fmt.Print(divider("1. List -- Initial load (all rows dim, checks in progress)"))
	fmt.Println(mockInitialLoad())

	// 2. Partially Loaded -- some forward checks done
	fmt.Print(divider("2. List -- Partially loaded (forward checks resolved)"))
	fmt.Println(mockPartiallyLoaded())

	// 3. Fully Loaded (120 cols)
	fmt.Print(divider("3. List -- Fully loaded (120 cols)"))
	fmt.Println(mockFullyLoaded120())

	// 4. Fully Loaded (80 cols)
	fmt.Print(divider("4. List -- Fully loaded (80 cols)"))
	fmt.Println(mockFullyLoaded80())

	// 5. Filter active
	fmt.Print(divider("5. List -- Filter active (/sec)"))
	fmt.Println(mockFilterActive())

	// 6. Filtered resource list
	fmt.Print(divider("6. Smart Enter -- Filtered resource list (SG, count > 1)"))
	fmt.Println(mockFilteredList())

	// 7. Direct to detail
	fmt.Print(divider("7. Smart Enter -- Direct to detail (VPC, count = 1)"))
	fmt.Println(mockDirectDetail())

	// 8. Deep navigation (VPC, 18 types)
	fmt.Print(divider("8. Deep navigation -- Depth [7], VPC 18 related types"))
	fmt.Println(mockDeepNavigation())

	// 9. All unavailable
	fmt.Print(divider("9. All dim state (no related resources found)"))
	fmt.Println(mockAllUnavailable())

	// 10. Long list with scroll indicator
	fmt.Print(divider("10. Long list with scroll indicator (VPC, 27 types, 80 cols)"))
	fmt.Println(mockLongListScroll())

	// 11. Help screen
	fmt.Print(divider("11. Help screen for related list"))
	fmt.Println(mockHelpScreen())

	fmt.Println()
}
