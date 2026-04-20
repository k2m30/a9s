// Related Resources: Two-Column Detail View -- static preview mockups.
// Run with: go run ./docs/design/related-resources-preview/
//
// The related-resources feature embeds a right column INSIDE the detail view.
// Left column = detail fields with navigable links (underlined).
// Right column = reverse & algorithmic related resource types.
//
// Mockups:
//  1. EC2 detail -- two-column, left focused, dim separator (120 cols)
//  2. EC2 detail -- two-column, right column focused, accent separator (120 cols)
//  3. EC2 detail -- right column hidden (r toggled off, 120 cols)
//  4. RDS detail -- different resource type (120 cols)
//  5. VPC detail -- heavy right column with scroll (120 cols)
//  6. Stacked layout (80 cols, narrow terminal)
//  7. Initial load -- right column all dim (120 cols)
//  8. Deep navigation -- depth indicator (120 cols)
//  9. Lambda detail -- algorithmic relationships (120 cols)
//
// 10. Smart Enter: filtered list from right column (120 cols)
// 11. Smart Enter: direct to detail from left column navigable field
// 12. Help screen for two-column detail view (120 cols)
// 13. Search active in left column -- "vpc" match 1/3 (120 cols)
// 14. Filter active in right column -- "/cloud" filtering types (120 cols)
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
	colYellow = lipgloss.Color("#e0af68") //nolint:unused // design palette — reserved for future use in preview

	colDetailKey = lipgloss.Color("#7aa2f7")
	colDetailSec = lipgloss.Color("#e0af68")
	colDetailVal = lipgloss.Color("#c0caf5")

	colHelpKey = lipgloss.Color("#9ece6a")
	colHelpCat = lipgloss.Color("#e0af68")

	// Search match colors (QA-26)
	colMatchBg    = lipgloss.Color("#e0af68") // amber background for non-current matches
	colMatchCurBg = lipgloss.Color("#ff9e64") // orange background for current match
	colMatchFg    = lipgloss.Color("#1a1b26") // dark foreground on match background
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
	navStyle     = lipgloss.NewStyle().Foreground(colAccent).Underline(true) // navigable value
	//lint:ignore U1000 design style — reserved for future use in preview
	greenStyle   = lipgloss.NewStyle().Foreground(colGreen)
	helpKeyStyle = lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	helpCatStyle = lipgloss.NewStyle().Foreground(colHelpCat).Bold(true)

	// Search match styles
	matchStyle    = lipgloss.NewStyle().Foreground(colMatchFg).Background(colMatchBg)
	matchCurStyle = lipgloss.NewStyle().Foreground(colMatchFg).Background(colMatchCurBg).Bold(true)
	matchIndStyle = lipgloss.NewStyle().Foreground(colDim) // match indicator "[1/3 matches]"
)

// -- Constants --

const rightColWidth = 32 // fixed width for the right column

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

func renderHeaderFilter(profile, region, version string, w int, filterText string) string {
	right := lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render("/" + filterText)
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
	gap := max(innerW-leftW-rightW, 1)

	content := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().
		Foreground(colHeaderFg).
		Width(w).Padding(0, 1).Render(content)
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

// -- Detail field types --

type fieldKind int

const (
	fieldPlain     fieldKind = iota // non-navigable scalar
	fieldNavigable                  // navigable scalar (value contains a resource ID/ARN)
	fieldSection                    // section header (e.g., "State:", "Tags:")
	fieldSubPlain                   // sub-field plain (indented)
	fieldSubNav                     // sub-field navigable
	fieldSubText                    // sub-field plain text (no key, just value like array items)
)

type detailField struct {
	key   string    // field key (e.g., "InstanceId", "VpcId")
	value string    // field value (e.g., "i-0abc123", "vpc-0aaa111")
	kind  fieldKind // determines rendering
}

// renderDetailField renders a single detail field row padded to leftW.
// When selected=true, the entire row gets the cursor highlight.
func renderDetailField(f detailField, leftW int, selected bool) string {
	keyColW := 22

	var rendered string
	switch f.kind {
	case fieldSection:
		rendered = " " + secStyle.Render(f.key+":")
	case fieldSubPlain:
		rendered = "     " + kStyle.Render(padOrTrunc(f.key+":", keyColW-4)) + vStyle.Render(f.value)
	case fieldSubNav:
		rendered = "     " + kStyle.Render(padOrTrunc(f.key+":", keyColW-4)) + navStyle.Render(f.value)
	case fieldSubText:
		rendered = "     " + vStyle.Render(f.value)
	case fieldNavigable:
		rendered = " " + kStyle.Render(padOrTrunc(f.key+":", keyColW)) + navStyle.Render(f.value)
	default: // fieldPlain
		rendered = " " + kStyle.Render(padOrTrunc(f.key+":", keyColW)) + vStyle.Render(f.value)
	}

	if selected {
		// Build plain text for selection highlighting
		var plainText string
		switch f.kind {
		case fieldSection:
			plainText = " " + f.key + ":"
		case fieldSubPlain, fieldSubNav:
			plainText = "     " + padOrTrunc(f.key+":", keyColW-4) + f.value
		case fieldSubText:
			plainText = "     " + f.value
		default:
			plainText = " " + padOrTrunc(f.key+":", keyColW) + f.value
		}
		return cellSelected.Render(pad(plainText, leftW))
	}

	return pad(rendered, leftW)
}

// -- Detail field with search highlights --
// Renders a field with specific substrings highlighted as search matches.
// matchPositions: list of {offset, length, isCurrent} for the plain text.
// For simplicity in this preview, we take a pre-built styled string approach:
// the caller provides the rendered text with embedded match markers.

// renderSearchField renders a detail field where certain value substrings
// are highlighted as search matches. For the preview, this is done by
// manually constructing the styled text with match highlights inline.
func renderSearchField(key string, preMatch string, match string, postMatch string, isCurrent bool, leftW int, isSelected bool, isSub bool) string {
	keyColW := 22
	indent := " "
	keyW := keyColW
	if isSub {
		indent = "     "
		keyW = keyColW - 4
	}

	mStyle := matchStyle
	if isCurrent {
		mStyle = matchCurStyle
	}

	if isSelected {
		// Selected row: build with match highlight inside selection
		plainPre := indent + padOrTrunc(key+":", keyW) + preMatch
		matchPart := mStyle.Render(match)
		plainPost := postMatch

		// We need to combine: selected prefix + match (with match bg) + selected suffix
		// For the preview, render the selected row with the match highlight embedded
		return cellSelected.Render(plainPre) + matchPart + cellSelected.Render(pad(plainPost, leftW-lipgloss.Width(plainPre)-lipgloss.Width(match)))
	}

	// Normal row with search highlight
	keyPart := indent + kStyle.Render(padOrTrunc(key+":", keyW))
	valuePart := vStyle.Render(preMatch) + mStyle.Render(match) + vStyle.Render(postMatch)
	return pad(keyPart+valuePart, leftW)
}

// renderSearchSectionField renders a section header with a search match in the key.
// The key argument is unused — kept for signature parity with renderSearchKeyValueField.
func renderSearchSectionField(_ string, preMatch string, match string, postMatch string, isCurrent bool, leftW int) string {
	mStyle := matchStyle
	if isCurrent {
		mStyle = matchCurStyle
	}
	rendered := " " + secStyle.Render(preMatch) + mStyle.Render(match) + secStyle.Render(postMatch+":")
	return pad(rendered, leftW)
}

// -- Related row in right column --

type rowState int

const (
	rowAvailable rowState = iota
	rowDim
	rowSelected
	rowHeader // RELATED header -- dim, not selectable, cursor skips
)

type relatedRow struct {
	label string
	count string // "(3)", "(1)", etc. or "" for expensive/no-count
	state rowState
}

func renderRelatedRow(r relatedRow, rightW int) string {
	indent := "  "
	text := indent + r.label
	if r.count != "" {
		text += " " + r.count
	}

	switch r.state {
	case rowSelected:
		return cellSelected.Render(pad(text, rightW))
	case rowDim:
		return cellDim.Render(pad(text, rightW))
	case rowHeader:
		return cellDim.Render(pad(text, rightW))
	default:
		return cellNormal.Render(pad(text, rightW))
	}
}

// -- Two-column framed box --
// Renders a two-column layout inside a framed box.
// Left column: detail fields. Right column: related resource types.
// A thin separator (|) divides them. Top/bottom borders span full width.

func renderTwoColBox(
	leftFields []detailField,
	selectedIdx int, // which left field has cursor (-1 = none)
	rightRows []relatedRow,
	title string,
	w int,
	rightFocused bool, // true = right column focused, changes separator color
) string {
	// Separator color: dim when left focused, accent when right focused
	sepColor := colSep // #414868 dim
	if rightFocused {
		sepColor = colAccent // #7aa2f7 accent
	}
	sepChar := lipgloss.NewStyle().Foreground(sepColor).Render("\u2502")
	innerW := w - 2
	rightW := rightColWidth
	leftW := innerW - rightW - 1 // -1 for separator

	// Determine the number of content lines (max of left and right)
	numLines := max(len(rightRows), len(leftFields))
	numLines++ // bottom padding line

	// Build content lines
	var contentLines []string
	for i := 0; i < numLines; i++ {
		var leftPart string
		if i < len(leftFields) {
			leftPart = renderDetailField(leftFields[i], leftW, i == selectedIdx)
		} else {
			leftPart = strings.Repeat(" ", leftW)
		}

		var rightPart string
		if i < len(rightRows) {
			rightPart = renderRelatedRow(rightRows[i], rightW)
		} else {
			rightPart = strings.Repeat(" ", rightW)
		}

		contentLines = append(contentLines, leftPart+sepChar+rightPart)
	}

	return renderFramedBox(contentLines, title, w)
}

// -- Two-column framed box with pre-rendered left lines --
// Used when left column has custom rendering (e.g., search highlights).

func renderTwoColBoxCustomLeft(
	leftLines []string, // pre-rendered, must be padded to leftW
	rightRows []relatedRow,
	title string,
	w int,
	rightFocused bool,
) string {
	sepColor := colSep
	if rightFocused {
		sepColor = colAccent
	}
	sepChar := lipgloss.NewStyle().Foreground(sepColor).Render("\u2502")
	innerW := w - 2
	rightW := rightColWidth
	leftW := innerW - rightW - 1 //nolint:unused // needed for proportion reference

	numLines := max(len(rightRows), len(leftLines))
	numLines++

	_ = leftW // suppress unused warning

	var contentLines []string
	for i := 0; i < numLines; i++ {
		var leftPart string
		if i < len(leftLines) {
			leftPart = leftLines[i]
		} else {
			leftPart = strings.Repeat(" ", innerW-rightW-1)
		}

		var rightPart string
		if i < len(rightRows) {
			rightPart = renderRelatedRow(rightRows[i], rightW)
		} else {
			rightPart = strings.Repeat(" ", rightW)
		}

		contentLines = append(contentLines, leftPart+sepChar+rightPart)
	}

	return renderFramedBox(contentLines, title, w)
}

// -- Single-column detail box (right column hidden) --

func renderSingleColBox(
	fields []detailField,
	selectedIdx int,
	title string,
	w int,
) string {
	innerW := w - 2
	var lines []string
	for i, f := range fields {
		lines = append(lines, renderDetailField(f, innerW, i == selectedIdx))
	}
	lines = append(lines, "") // bottom padding
	return renderFramedBox(lines, title, w)
}

// -- Stacked layout (narrow terminal) --

func renderStackedBox(
	fields []detailField,
	selectedFieldIdx int,
	relRows []relatedRow,
	title string,
	w int,
) string {
	innerW := w - 2
	var lines []string
	for i, f := range fields {
		lines = append(lines, renderDetailField(f, innerW, i == selectedFieldIdx))
	}

	// Separator line
	sepLine := dimStyle.Render(" -- Related " + strings.Repeat("\u2500", innerW-13))
	lines = append(lines, "")
	lines = append(lines, sepLine)
	lines = append(lines, "")

	// Related rows (full width, same indent as right column)
	for _, r := range relRows {
		text := "  " + r.label
		if r.count != "" {
			text += " " + r.count
		}
		switch r.state {
		case rowSelected:
			lines = append(lines, cellSelected.Render(pad(text, innerW)))
		case rowDim:
			lines = append(lines, cellDim.Render(pad(text, innerW)))
		default:
			lines = append(lines, cellNormal.Render(pad(text, innerW)))
		}
	}

	lines = append(lines, "") // bottom padding
	return renderFramedBox(lines, title, w)
}

// ============================================================
// Mockups
// ============================================================

// -- Mockup 1: EC2 detail, two-column, left focused, cursor on InstanceId --

func mockEC2LeftFocused() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "InstanceId", value: "i-0abc123def456789a", kind: fieldPlain},
		{key: "InstanceType", value: "t3.large", kind: fieldPlain},
		{key: "State", value: "running", kind: fieldPlain},
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldNavigable},
		{key: "SubnetId", value: "subnet-0bbb222ccc333dd", kind: fieldNavigable},
		{key: "SecurityGroups", value: "", kind: fieldSection},
		{key: "GroupId", value: "sg-0ccc333ddd444ee", kind: fieldSubNav},
		{key: "GroupName", value: "web-sg", kind: fieldSubPlain},
		{key: "GroupId", value: "sg-0ddd444eee555ff", kind: fieldSubNav},
		{key: "GroupName", value: "db-access-sg", kind: fieldSubPlain},
		{key: "IamInstanceProfile", value: "", kind: fieldSection},
		{key: "Arn", value: "arn:aws:iam::123456:role/web-role", kind: fieldSubNav},
		{key: "ImageId", value: "ami-0aaa111222333", kind: fieldNavigable},
		{key: "KeyName", value: "prod-keypair", kind: fieldPlain},
		{key: "PrivateIpAddress", value: "10.0.48.175", kind: fieldPlain},
		{key: "PublicIpAddress", value: "203.0.113.10", kind: fieldPlain},
		{key: "LaunchTime", value: "2026-03-15 09:22:45", kind: fieldPlain},
		{key: "Architecture", value: "x86_64", kind: fieldPlain},
		{key: "Placement", value: "", kind: fieldSection},
		{key: "AvailabilityZone", value: "us-east-1a", kind: fieldSubPlain},
		{key: "Tenancy", value: "default", kind: fieldSubPlain},
		{key: "Tags", value: "", kind: fieldSection},
		{key: "Key", value: "Name", kind: fieldSubPlain},
		{key: "Value", value: "web-prod", kind: fieldSubPlain},
		{key: "Key", value: "Environment", kind: fieldSubPlain},
		{key: "Value", value: "production", kind: fieldSubPlain},
	}

	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "Auto Scaling Groups", state: rowAvailable},
		{label: "Target Groups", state: rowAvailable},
		{label: "CloudWatch Alarms", state: rowAvailable},
		{label: "CloudFormation Stacks", state: rowAvailable},
		{label: "EBS Snapshots", state: rowAvailable},
		{label: "Elastic IPs", count: "(1)", state: rowAvailable},
		{label: "CloudTrail Events", state: rowAvailable},
		{label: "EKS Node Groups", state: rowDim},
		{label: "Elastic Beanstalk", state: rowDim},
	}

	box := renderTwoColBox(fields, 0, related, "detail -- i-0abc123 (web-prod)", w, false)
	return header + "\n" + box
}

// -- Mockup 2: EC2 detail, right column focused --

func mockEC2RightFocused() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "InstanceId", value: "i-0abc123def456789a", kind: fieldPlain},
		{key: "InstanceType", value: "t3.large", kind: fieldPlain},
		{key: "State", value: "running", kind: fieldPlain},
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldNavigable},
		{key: "SubnetId", value: "subnet-0bbb222ccc333dd", kind: fieldNavigable},
		{key: "SecurityGroups", value: "", kind: fieldSection},
		{key: "GroupId", value: "sg-0ccc333ddd444ee", kind: fieldSubNav},
		{key: "GroupName", value: "web-sg", kind: fieldSubPlain},
		{key: "GroupId", value: "sg-0ddd444eee555ff", kind: fieldSubNav},
		{key: "GroupName", value: "db-access-sg", kind: fieldSubPlain},
		{key: "IamInstanceProfile", value: "", kind: fieldSection},
		{key: "Arn", value: "arn:aws:iam::123456:role/web-role", kind: fieldSubNav},
		{key: "ImageId", value: "ami-0aaa111222333", kind: fieldNavigable},
		{key: "KeyName", value: "prod-keypair", kind: fieldPlain},
		{key: "PrivateIpAddress", value: "10.0.48.175", kind: fieldPlain},
		{key: "PublicIpAddress", value: "203.0.113.10", kind: fieldPlain},
	}

	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "Auto Scaling Groups", state: rowSelected},
		{label: "Target Groups", state: rowAvailable},
		{label: "CloudWatch Alarms", state: rowAvailable},
		{label: "CloudFormation Stacks", state: rowAvailable},
		{label: "EBS Snapshots", state: rowAvailable},
		{label: "Elastic IPs", count: "(1)", state: rowAvailable},
		{label: "CloudTrail Events", state: rowAvailable},
		{label: "EKS Node Groups", state: rowDim},
		{label: "Elastic Beanstalk", state: rowDim},
	}

	// No cursor in left column (right is focused); separator is accent color
	box := renderTwoColBox(fields, -1, related, "detail -- i-0abc123 (web-prod)", w, true)
	return header + "\n" + box
}

// -- Mockup 3: Right column hidden (r toggled off) --

func mockRightHidden() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "InstanceId", value: "i-0abc123def456789a", kind: fieldPlain},
		{key: "InstanceType", value: "t3.large", kind: fieldPlain},
		{key: "State", value: "running", kind: fieldPlain},
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldNavigable},
		{key: "SubnetId", value: "subnet-0bbb222ccc333dd", kind: fieldNavigable},
		{key: "SecurityGroups", value: "", kind: fieldSection},
		{key: "GroupId", value: "sg-0ccc333ddd444ee", kind: fieldSubNav},
		{key: "GroupName", value: "web-sg", kind: fieldSubPlain},
		{key: "GroupId", value: "sg-0ddd444eee555ff", kind: fieldSubNav},
		{key: "GroupName", value: "db-access-sg", kind: fieldSubPlain},
		{key: "IamInstanceProfile", value: "", kind: fieldSection},
		{key: "Arn", value: "arn:aws:iam::123456:role/web-role", kind: fieldSubNav},
		{key: "ImageId", value: "ami-0aaa111222333", kind: fieldNavigable},
		{key: "KeyName", value: "prod-keypair", kind: fieldPlain},
		{key: "PrivateIpAddress", value: "10.0.48.175", kind: fieldPlain},
		{key: "PublicIpAddress", value: "203.0.113.10", kind: fieldPlain},
		{key: "LaunchTime", value: "2026-03-15 09:22:45", kind: fieldPlain},
		{key: "Architecture", value: "x86_64", kind: fieldPlain},
	}

	box := renderSingleColBox(fields, 0, "detail -- i-0abc123 (web-prod)", w)
	return header + "\n" + box
}

// -- Mockup 4: RDS detail --

func mockRDSDetail() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "DBInstanceIdentifier", value: "mydb-prod", kind: fieldPlain},
		{key: "Engine", value: "postgres", kind: fieldPlain},
		{key: "EngineVersion", value: "15.4", kind: fieldPlain},
		{key: "DBInstanceStatus", value: "available", kind: fieldPlain},
		{key: "DBInstanceClass", value: "db.t3.medium", kind: fieldPlain},
		{key: "Endpoint", value: "", kind: fieldSection},
		{key: "Address", value: "mydb-prod.abc123.rds.amazonaws.com", kind: fieldSubPlain},
		{key: "Port", value: "5432", kind: fieldSubPlain},
		{key: "MultiAZ", value: "true", kind: fieldPlain},
		{key: "VpcSecurityGroups", value: "", kind: fieldSection},
		{key: "VpcSecurityGroupId", value: "sg-0abc123def456", kind: fieldSubNav},
		{key: "Status", value: "active", kind: fieldSubPlain},
		{key: "DBSubnetGroup", value: "", kind: fieldSection},
		{key: "DBSubnetGroupName", value: "prod-db-subnets", kind: fieldSubPlain},
		{key: "Subnets", value: "", kind: fieldSection},
		{key: "SubnetId", value: "subnet-0aaa111bbb222", kind: fieldSubNav},
		{key: "SubnetId", value: "subnet-0ccc333ddd444", kind: fieldSubNav},
		{key: "KmsKeyId", value: "arn:aws:kms:us-east-1:123:key/abc", kind: fieldNavigable},
		{key: "StorageEncrypted", value: "true", kind: fieldPlain},
	}

	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "CloudWatch Alarms", state: rowAvailable},
		{label: "RDS Snapshots", count: "(5)", state: rowAvailable},
		{label: "Secrets Manager", state: rowAvailable},
		{label: "CW Log Groups", count: "(2)", state: rowAvailable},
		{label: "CloudTrail Events", state: rowAvailable},
		{label: "CloudFormation Stacks", state: rowDim},
	}

	box := renderTwoColBox(fields, 0, related, "detail -- mydb-prod", w, false)
	return header + "\n" + box
}

// -- Mockup 5: VPC detail -- heavy right column --

func mockVPCDetail() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldPlain},
		{key: "CidrBlock", value: "10.0.0.0/16", kind: fieldPlain},
		{key: "State", value: "available", kind: fieldPlain},
		{key: "IsDefault", value: "false", kind: fieldPlain},
		{key: "DhcpOptionsId", value: "dopt-0abc123def456", kind: fieldPlain},
		{key: "InstanceTenancy", value: "default", kind: fieldPlain},
		{key: "Tags", value: "", kind: fieldSection},
		{key: "Key", value: "Name", kind: fieldSubPlain},
		{key: "Value", value: "production-vpc", kind: fieldSubPlain},
		{key: "Key", value: "Environment", kind: fieldSubPlain},
		{key: "Value", value: "prod", kind: fieldSubPlain},
	}

	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "EC2 Instances", state: rowAvailable},
		{label: "Subnets", count: "(6)", state: rowAvailable},
		{label: "Security Groups", count: "(12)", state: rowAvailable},
		{label: "Route Tables", count: "(3)", state: rowAvailable},
		{label: "NAT Gateways", count: "(2)", state: rowAvailable},
		{label: "Internet Gateways", count: "(1)", state: rowAvailable},
		{label: "VPC Endpoints", count: "(4)", state: rowAvailable},
		{label: "Transit Gateways", state: rowAvailable},
		{label: "Load Balancers", state: rowAvailable},
		{label: "Lambda Functions", state: rowAvailable},
		{label: "EKS Clusters", state: rowAvailable},
		{label: "DB Instances", state: rowAvailable},
		{label: "ElastiCache", state: rowAvailable},
		{label: "OpenSearch", state: rowDim},
		{label: "Redshift", state: rowDim},
		{label: "MSK Clusters", state: rowDim},
		{label: "CloudTrail Events", state: rowAvailable},
	}

	// Right column is taller than left -- the box extends to fit both.
	// In real TUI, the right column would scroll. For this preview, we show
	// the full list to demonstrate the height.
	box := renderTwoColBox(fields, 0, related, "detail -- vpc-0aaa111 (production-vpc)", w, false)
	return header + "\n" + box
}

// -- Mockup 6: Stacked layout (80 cols) --

func mockStacked80() string {
	w := 80
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "InstanceId", value: "i-0abc123def456789a", kind: fieldPlain},
		{key: "InstanceType", value: "t3.large", kind: fieldPlain},
		{key: "State", value: "running", kind: fieldPlain},
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldNavigable},
		{key: "SubnetId", value: "subnet-0bbb222ccc333dd", kind: fieldNavigable},
		{key: "SecurityGroups", value: "", kind: fieldSection},
		{key: "GroupId", value: "sg-0ccc333ddd444ee", kind: fieldSubNav},
		{key: "GroupId", value: "sg-0ddd444eee555ff", kind: fieldSubNav},
		{key: "LaunchTime", value: "2026-03-15 09:22:45", kind: fieldPlain},
	}

	related := []relatedRow{
		{label: "Auto Scaling Groups", state: rowAvailable},
		{label: "CloudWatch Alarms", state: rowAvailable},
		{label: "CloudFormation Stacks", state: rowAvailable},
		{label: "EKS Node Groups", state: rowDim},
		{label: "CloudTrail Events", state: rowAvailable},
	}

	box := renderStackedBox(fields, 0, related, "detail -- i-0abc123 (web-prod)", w)
	return header + "\n" + box
}

// -- Mockup 7: Initial load (right column all dim) --

func mockInitialLoad() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "InstanceId", value: "i-0abc123def456789a", kind: fieldPlain},
		{key: "InstanceType", value: "t3.large", kind: fieldPlain},
		{key: "State", value: "running", kind: fieldPlain},
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldNavigable},
		{key: "SubnetId", value: "subnet-0bbb222ccc333dd", kind: fieldNavigable},
		{key: "SecurityGroups", value: "", kind: fieldSection},
		{key: "GroupId", value: "sg-0ccc333ddd444ee", kind: fieldSubNav},
		{key: "GroupId", value: "sg-0ddd444eee555ff", kind: fieldSubNav},
		{key: "ImageId", value: "ami-0aaa111222333", kind: fieldNavigable},
		{key: "LaunchTime", value: "2026-03-15 09:22:45", kind: fieldPlain},
	}

	// All right column rows dim (checks in progress); RELATED header always dim
	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "Target Groups", state: rowDim},
		{label: "Auto Scaling Groups", state: rowDim},
		{label: "CloudWatch Alarms", state: rowDim},
		{label: "EKS Node Groups", state: rowDim},
		{label: "CloudFormation Stacks", state: rowDim},
		{label: "Elastic Beanstalk", state: rowDim},
		{label: "EBS Snapshots", state: rowDim},
		{label: "Elastic IPs", state: rowDim},
		{label: "CloudTrail Events", state: rowDim},
	}

	box := renderTwoColBox(fields, 0, related, "detail -- i-0abc123 (web-prod)", w, false)
	return header + "\n" + box
}

// -- Mockup 8: Deep navigation with depth indicator --

func mockDeepNavigation() string {
	w := 120
	header := renderHeaderDepth(5, "prod", "us-east-1", w)

	fields := []detailField{
		{key: "SubnetId", value: "subnet-0bbb222ccc333dd", kind: fieldPlain},
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldNavigable},
		{key: "CidrBlock", value: "10.0.1.0/24", kind: fieldPlain},
		{key: "AvailabilityZone", value: "us-east-1a", kind: fieldPlain},
		{key: "AvailableIpAddressCount", value: "251", kind: fieldPlain},
		{key: "MapPublicIpOnLaunch", value: "false", kind: fieldPlain},
		{key: "State", value: "available", kind: fieldPlain},
		{key: "Tags", value: "", kind: fieldSection},
		{key: "Key", value: "Name", kind: fieldSubPlain},
		{key: "Value", value: "private-us-east-1a", kind: fieldSubPlain},
	}

	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "EC2 Instances", state: rowAvailable},
		{label: "NAT Gateways", state: rowAvailable},
		{label: "Network Interfaces", state: rowAvailable},
		{label: "Route Tables", state: rowAvailable},
		{label: "CloudTrail Events", state: rowAvailable},
		{label: "Load Balancers", state: rowDim},
	}

	box := renderTwoColBox(fields, 0, related, "detail -- subnet-0bbb222 (private-us-east-1a)", w, false)
	return header + "\n" + box
}

// -- Mockup 9: Lambda detail (algorithmic relationships) --

func mockLambdaDetail() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "FunctionName", value: "process-orders", kind: fieldPlain},
		{key: "Runtime", value: "python3.11", kind: fieldPlain},
		{key: "Handler", value: "handler.main", kind: fieldPlain},
		{key: "MemorySize", value: "256", kind: fieldPlain},
		{key: "Timeout", value: "30", kind: fieldPlain},
		{key: "Role", value: "arn:aws:iam::123456:role/lambda-exec", kind: fieldNavigable},
		{key: "VpcConfig", value: "", kind: fieldSection},
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldSubNav},
		{key: "SubnetIds", value: "", kind: fieldSection},
		{key: "", value: "- subnet-0aaa111bbb222", kind: fieldSubNav},
		{key: "", value: "- subnet-0ccc333ddd444", kind: fieldSubNav},
		{key: "SecurityGroupIds", value: "", kind: fieldSection},
		{key: "", value: "- sg-0abc123def456", kind: fieldSubNav},
		{key: "CodeSize", value: "1048576", kind: fieldPlain},
		{key: "LastModified", value: "2026-03-28 14:30:00", kind: fieldPlain},
	}

	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "CW Log Group", state: rowAvailable},
		{label: "SQS Event Sources", count: "(2)", state: rowAvailable},
		{label: "EventBridge Rules", state: rowAvailable},
		{label: "SNS Subscriptions", state: rowAvailable},
		{label: "CloudWatch Alarms", state: rowAvailable},
		{label: "API Gateway", state: rowAvailable},
		{label: "CloudTrail Events", state: rowAvailable},
		{label: "S3 Notifications", state: rowDim},
		{label: "Step Functions", state: rowDim},
		{label: "Target Groups", state: rowDim},
	}

	box := renderTwoColBox(fields, 0, related, "detail -- process-orders (process-orders)", w, false)
	return header + "\n" + box
}

// -- Mockup 10: Smart Enter from right column -> filtered list --

func mockFilteredList() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	colHdr := lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	sel := cellSelected
	rowGreen := lipgloss.NewStyle().Foreground(colGreen)

	nameW := 26
	stateW := 14
	metricW := 30
	dimW := 24

	hdrLine := " " +
		colHdr.Render(padOrTrunc("NAME", nameW)) +
		colHdr.Render(padOrTrunc("STATE", stateW)) +
		colHdr.Render(padOrTrunc("METRIC", metricW)) +
		colHdr.Render(padOrTrunc("DIMENSIONS", dimW))

	row1 := sel.Render(pad(
		" "+padOrTrunc("ec2-cpu-high-i-0abc", nameW)+
			padOrTrunc("OK", stateW)+
			padOrTrunc("CPUUtilization", metricW)+
			padOrTrunc("InstanceId:i-0abc123", dimW),
		w-2))
	row2 := " " +
		rowGreen.Render(padOrTrunc("ec2-status-i-0abc", nameW)) +
		rowGreen.Render(padOrTrunc("OK", stateW)) +
		rowGreen.Render(padOrTrunc("StatusCheckFailed", metricW)) +
		rowGreen.Render(padOrTrunc("InstanceId:i-0abc123", dimW))
	row3 := " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render(padOrTrunc("ec2-mem-high-i-0abc", nameW)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render(padOrTrunc("ALARM", stateW)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render(padOrTrunc("MemoryUtilization", metricW)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Render(padOrTrunc("InstanceId:i-0abc123", dimW))

	countLine := dimStyle.Render(" 3 alarms")

	lines := []string{hdrLine, row1, row2, row3, "", countLine, ""}

	box := renderFramedBox(lines, "alarms(3) -- i-0abc123 (web-prod)", w)
	return header + "\n" + box
}

// -- Mockup 11: Smart Enter from left navigable field -> direct detail --

func mockDirectDetail() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	fields := []detailField{
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldPlain},
		{key: "CidrBlock", value: "10.0.0.0/16", kind: fieldPlain},
		{key: "State", value: "available", kind: fieldPlain},
		{key: "IsDefault", value: "false", kind: fieldPlain},
		{key: "DhcpOptionsId", value: "dopt-0abc123def456", kind: fieldPlain},
		{key: "InstanceTenancy", value: "default", kind: fieldPlain},
		{key: "Tags", value: "", kind: fieldSection},
		{key: "Key", value: "Name", kind: fieldSubPlain},
		{key: "Value", value: "production-vpc", kind: fieldSubPlain},
	}

	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "EC2 Instances", state: rowAvailable},
		{label: "Subnets", count: "(6)", state: rowAvailable},
		{label: "Security Groups", count: "(12)", state: rowAvailable},
		{label: "Route Tables", count: "(3)", state: rowAvailable},
		{label: "NAT Gateways", count: "(2)", state: rowAvailable},
		{label: "Internet Gateways", count: "(1)", state: rowAvailable},
		{label: "CloudTrail Events", state: rowAvailable},
	}

	box := renderTwoColBox(fields, 0, related, "detail -- vpc-0aaa111 (production-vpc)", w, false)
	return header + "\n" + box
}

// -- Mockup 13: Search active in left column --
// Search for "vpc" confirmed, 3 matches found. Left column focused.
// Current match (1/3) is on the VpcId value row.

func mockSearchLeftColumn() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	innerW := w - 2
	rightW := rightColWidth
	leftW := innerW - rightW - 1

	// Build left lines manually to show search highlights
	var leftLines []string

	// Row 0: InstanceId (no match, no selection)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "InstanceId", value: "i-0abc123def456789a", kind: fieldPlain},
		leftW, false))

	// Row 1: InstanceType (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "InstanceType", value: "t3.large", kind: fieldPlain},
		leftW, false))

	// Row 2: State (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "State", value: "running", kind: fieldPlain},
		leftW, false))

	// Row 3: VpcId -- CURRENT MATCH (1/3), cursor on this row
	// "vpc" in "vpc-0aaa111bbb222cc" is the match. Show with cursor + orange highlight.
	leftLines = append(leftLines, renderSearchField(
		"VpcId", "", "vpc", "-0aaa111bbb222cc", true, leftW, true, false))

	// Row 4: SubnetId (no match, navigable but no search hit)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "SubnetId", value: "subnet-0bbb222ccc333dd", kind: fieldNavigable},
		leftW, false))

	// Row 5: SecurityGroups section (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "SecurityGroups", value: "", kind: fieldSection},
		leftW, false))

	// Row 6: GroupId sub-field (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "GroupId", value: "sg-0ccc333ddd444ee", kind: fieldSubNav},
		leftW, false))

	// Row 7: GroupId sub-field (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "GroupId", value: "sg-0ddd444eee555ff", kind: fieldSubNav},
		leftW, false))

	// Row 8: IamInstanceProfile section (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "IamInstanceProfile", value: "", kind: fieldSection},
		leftW, false))

	// Row 9: Arn sub-field (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "Arn", value: "arn:aws:iam::123456:role/web-role", kind: fieldSubNav},
		leftW, false))

	// Row 10: ImageId (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "ImageId", value: "ami-0aaa111222333", kind: fieldNavigable},
		leftW, false))

	// Row 11: VpcConfig section -- MATCH 2/3, amber highlight on "Vpc" in the key
	leftLines = append(leftLines, renderSearchSectionField(
		"VpcConfig", "", "Vpc", "Config", false, leftW))

	// Row 12: Sub-field VpcId -- MATCH 3/3, amber highlight
	leftLines = append(leftLines, renderSearchField(
		"VpcId", "", "vpc", "-0aaa111bbb222cc", false, leftW, false, true))

	// Row 13: AvailabilityZone sub-field (no match)
	leftLines = append(leftLines, renderDetailField(
		detailField{key: "AvailabilityZone", value: "us-east-1a", kind: fieldSubPlain},
		leftW, false))

	// Match indicator at bottom
	leftLines = append(leftLines, pad(matchIndStyle.Render(" [1/3 matches]"), leftW))

	// Right column (unaffected by search)
	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "Auto Scaling Groups", state: rowAvailable},
		{label: "CloudWatch Alarms", state: rowAvailable},
		{label: "CloudFormation Stacks", state: rowAvailable},
		{label: "EBS Snapshots", state: rowAvailable},
		{label: "Elastic IPs", count: "(1)", state: rowAvailable},
		{label: "CloudTrail Events", state: rowAvailable},
		{label: "EKS Node Groups", state: rowDim},
		{label: "Elastic Beanstalk", state: rowDim},
	}

	box := renderTwoColBoxCustomLeft(leftLines, related, "detail -- i-0abc123 (web-prod)", w, false)
	return header + "\n" + box
}

// -- Mockup 14: Filter active in right column --
// Right column focused, filter "/cloud" active. Only matching types shown.

func mockFilterRightColumn() string {
	w := 120
	header := renderHeaderFilter("prod", "us-east-1", "3.28.0", w, "cloud")

	fields := []detailField{
		{key: "InstanceId", value: "i-0abc123def456789a", kind: fieldPlain},
		{key: "InstanceType", value: "t3.large", kind: fieldPlain},
		{key: "State", value: "running", kind: fieldPlain},
		{key: "VpcId", value: "vpc-0aaa111bbb222cc", kind: fieldNavigable},
		{key: "SubnetId", value: "subnet-0bbb222ccc333dd", kind: fieldNavigable},
		{key: "SecurityGroups", value: "", kind: fieldSection},
		{key: "GroupId", value: "sg-0ccc333ddd444ee", kind: fieldSubNav},
		{key: "GroupName", value: "web-sg", kind: fieldSubPlain},
		{key: "GroupId", value: "sg-0ddd444eee555ff", kind: fieldSubNav},
		{key: "GroupName", value: "db-access-sg", kind: fieldSubPlain},
		{key: "IamInstanceProfile", value: "", kind: fieldSection},
		{key: "Arn", value: "arn:aws:iam::123456:role/web-role", kind: fieldSubNav},
		{key: "ImageId", value: "ami-0aaa111222333", kind: fieldNavigable},
		{key: "KeyName", value: "prod-keypair", kind: fieldPlain},
	}

	// Right column: filtered to only types containing "cloud"
	// RELATED header persists during filtering (structural, not a filterable row)
	related := []relatedRow{
		{label: "RELATED", state: rowHeader},
		{label: "CloudWatch Alarms", state: rowSelected}, // cursor on first match
		{label: "CloudFormation Stacks", state: rowAvailable},
		{label: "CloudTrail Events", state: rowAvailable},
	}

	box := renderTwoColBox(fields, -1, related, "detail -- i-0abc123 (web-prod)", w, true)
	return header + "\n" + box
}

// -- Mockup 12: Help screen --

func mockHelpScreen() string {
	w := 120
	header := renderHeaderNormal("prod", "us-east-1", "3.28.0", w)

	colW := 27
	hk := func(key, desc string) string {
		return helpKeyStyle.Render(padOrTrunc(key, 10)) +
			lipgloss.NewStyle().Foreground(colHeaderFg).Render(padOrTrunc(desc, colW-10))
	}

	blank := strings.Repeat(" ", colW)

	lines := []string{
		" " + helpCatStyle.Render(padOrTrunc("DETAIL", colW)) +
			helpCatStyle.Render(padOrTrunc("RELATED", colW)) +
			helpCatStyle.Render(padOrTrunc("NAVIGATION", colW)) +
			helpCatStyle.Render(padOrTrunc("HOTKEYS", colW)),
		"",
		" " + hk("<enter>", "Open link") +
			hk("<tab>", "Switch col") +
			hk("<j>", "Down") +
			hk("<?>", "Help"),
		" " + hk("<esc>", "Go back") +
			hk("<r>", "Toggle col") +
			hk("<k>", "Up") +
			hk("<:>", "Command"),
		" " + hk("<h/l>", "Switch col") +
			hk("</>", "Filter list") +
			hk("<g>", "Top") +
			blank,
		" " + hk("<c>", "Copy value") +
			blank +
			hk("<G>", "Bottom") +
			blank,
		" " + hk("<y>", "YAML view") +
			blank +
			hk("<pgdn>", "Page down") +
			blank,
		" " + hk("</>", "Search") +
			blank +
			hk("<pgup>", "Page up") +
			blank,
		" " + hk("<n>", "Next match") +
			blank +
			blank +
			blank,
		" " + hk("<N>", "Prev match") +
			blank +
			blank +
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

	// 1. EC2 detail -- two-column, left focused, dim separator
	fmt.Print(divider("1. EC2 detail -- left focused, dim separator (120 cols)"))
	fmt.Println(mockEC2LeftFocused())

	// 2. EC2 detail -- right column focused, accent separator
	fmt.Print(divider("2. EC2 detail -- right focused, accent separator (120 cols)"))
	fmt.Println(mockEC2RightFocused())

	// 3. Right column hidden
	fmt.Print(divider("3. EC2 detail -- right column hidden (r off, 120 cols)"))
	fmt.Println(mockRightHidden())

	// 4. RDS detail
	fmt.Print(divider("4. RDS detail -- different resource type (120 cols)"))
	fmt.Println(mockRDSDetail())

	// 5. VPC detail -- heavy right column
	fmt.Print(divider("5. VPC detail -- heavy right column, 17 types (120 cols)"))
	fmt.Println(mockVPCDetail())

	// 6. Stacked layout (80 cols)
	fmt.Print(divider("6. Stacked layout -- narrow terminal (80 cols)"))
	fmt.Println(mockStacked80())

	// 7. Initial load -- right column all dim
	fmt.Print(divider("7. Initial load -- right column all dim (120 cols)"))
	fmt.Println(mockInitialLoad())

	// 8. Deep navigation with depth indicator
	fmt.Print(divider("8. Deep navigation -- depth [5], subnet detail (120 cols)"))
	fmt.Println(mockDeepNavigation())

	// 9. Lambda detail (algorithmic relationships)
	fmt.Print(divider("9. Lambda detail -- algorithmic relationships (120 cols)"))
	fmt.Println(mockLambdaDetail())

	// 10. Smart Enter: filtered list from right column
	fmt.Print(divider("10. Smart Enter -- filtered alarm list from right col"))
	fmt.Println(mockFilteredList())

	// 11. Smart Enter: direct to detail from navigable field
	fmt.Print(divider("11. Smart Enter -- direct to VPC detail from left col"))
	fmt.Println(mockDirectDetail())

	// 12. Help screen
	fmt.Print(divider("12. Help screen for two-column detail view"))
	fmt.Println(mockHelpScreen())

	// 13. Search active in left column
	fmt.Print(divider("13. Search active -- left col, 'vpc', match 1/3 (120 cols)"))
	fmt.Println(mockSearchLeftColumn())

	// 14. Filter active in right column
	fmt.Print(divider("14. Filter active -- right col, '/cloud', 3 types (120 cols)"))
	fmt.Println(mockFilterRightColumn())

	fmt.Println()
}
