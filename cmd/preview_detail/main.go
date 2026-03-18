// preview_detail renders static BEFORE/AFTER mockups of the detail view
// indentation fix.
//
// Run with: go run ./cmd/preview_detail/
//
// Design only — no Bubbletea, no interactivity.
// Uses Lipgloss v2 for real terminal colors.
package main

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// ── Palette (Tokyo Night Dark) ────────────────────────────────────────────────

var (
	colAccent   = lipgloss.Color("#7aa2f7")
	colDim      = lipgloss.Color("#565f89")
	colBorder   = lipgloss.Color("#414868")
	colHeaderFg = lipgloss.Color("#c0caf5")

	colKey    = lipgloss.Color("#7aa2f7") // all top-level keys
	colVal    = lipgloss.Color("#c0caf5") // scalar values
	colOK     = lipgloss.Color("#9ece6a") // running / available / active
	colErr    = lipgloss.Color("#f7768e") // stopped / failed
	colWarn   = lipgloss.Color("#e0af68") // pending / starting (kept for completeness)
	colSub    = lipgloss.Color("#565f89") // sub-field lines inside sections
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func padRight(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

func truncate(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	r := []rune(s)
	if len(r) > w-1 {
		return string(r[:w-1]) + "…"
	}
	return s
}

func statusStyle(v string) lipgloss.Style {
	low := strings.ToLower(v)
	switch {
	case strings.Contains(low, "running") ||
		strings.Contains(low, "available") ||
		strings.Contains(low, "active"):
		return lipgloss.NewStyle().Foreground(colOK)
	case strings.Contains(low, "stop") || strings.Contains(low, "fail"):
		return lipgloss.NewStyle().Foreground(colErr)
	default:
		return lipgloss.NewStyle().Foreground(colWarn)
	}
}

// ── Frame helpers ─────────────────────────────────────────────────────────────

func renderHeader(w int) string {
	accent := lipgloss.NewStyle().Foreground(colAccent).Bold(true).Render("a9s")
	ver    := lipgloss.NewStyle().Foreground(colDim).Render(" v0.6.0")
	ctx    := lipgloss.NewStyle().Foreground(colHeaderFg).Bold(true).Render("  prod:us-east-1")
	right  := lipgloss.NewStyle().Foreground(colDim).Render("? for help")

	left  := accent + ver + ctx
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	innerW := w - 2
	gap := innerW - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	content := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().Foreground(colHeaderFg).Width(w).Padding(0, 1).Render(content)
}

// renderBox draws a titled box around the given content lines.
// w is the TOTAL width including the border characters.
func renderBox(lines []string, title string, w int) string {
	borderSt := lipgloss.NewStyle().Foreground(colBorder)
	innerW   := w - 2

	titleRendered := lipgloss.NewStyle().Foreground(colHeaderFg).Bold(true).Render(title)
	titleVis      := lipgloss.Width(titleRendered)
	totalDashes   := w - 2 - titleVis - 2
	if totalDashes < 2 {
		totalDashes = 2
	}
	leftD  := totalDashes / 2
	rightD := totalDashes - leftD
	topBorder := borderSt.Render("┌"+strings.Repeat("─", leftD)+" ") +
		titleRendered +
		borderSt.Render(" "+strings.Repeat("─", rightD)+"┐")

	var sb strings.Builder
	sb.WriteString(topBorder)
	for _, line := range lines {
		sb.WriteString("\n")
		visW := lipgloss.Width(line)
		padded := line
		if visW < innerW {
			padded = line + strings.Repeat(" ", innerW-visW)
		}
		sb.WriteString(borderSt.Render("│"))
		sb.WriteString(padded)
		sb.WriteString(borderSt.Render("│"))
	}
	sb.WriteString("\n")
	sb.WriteString(borderSt.Render("└" + strings.Repeat("─", w-2) + "┘"))
	return sb.String()
}

func divider(label string) string {
	line := strings.Repeat("━", 34)
	return "\n" +
		lipgloss.NewStyle().Foreground(colBorder).Render(line) +
		"  " +
		lipgloss.NewStyle().Foreground(colAccent).Bold(true).Render(label) +
		"  " +
		lipgloss.NewStyle().Foreground(colBorder).Render(line) +
		"\n\n"
}

// ── BEFORE: current broken layout ─────────────────────────────────────────────
//
// Scalars use 3-space left margin.
// Section headers (struct / slice fields) use 1-space left margin.
// Sub-fields use 5-space indent.
// Result: scalars hang 2 columns to the right of their parent sections.

func renderBefore() string {
	const w = 84

	kStyle  := lipgloss.NewStyle().Foreground(colKey)
	vStyle  := lipgloss.NewStyle().Foreground(colVal)
	// The existing code uses a distinct amber color for section headers,
	// making the misalignment even more visually obvious.
	secStyle := lipgloss.NewStyle().Foreground(colWarn).Bold(true)
	subStyle := lipgloss.NewStyle().Foreground(colSub)

	// Scalar: 3-space margin + key padded to 22 chars + value
	kv := func(key, val string) string {
		return "   " + kStyle.Render(padRight(key+":", 22)) + vStyle.Render(val)
	}
	// Section header: 1-space margin (the broken part)
	sec := func(s string) string {
		return " " + secStyle.Render(s)
	}
	// Sub-field: 5-space indent
	sub := func(s string) string {
		return "     " + subStyle.Render(s)
	}

	var lines []string
	lines = append(lines, kv("InstanceId", "i-0bbb222222222222b"))
	lines = append(lines, sec("State:"))
	lines = append(lines, sub(`Code: "16"`))
	lines = append(lines, sub("Name: running"))
	lines = append(lines, kv("InstanceType", "t3.large"))
	lines = append(lines, kv("ImageId", "ami-0aaa111111111111a"))
	lines = append(lines, kv("VpcId", "vpc-0aaa1111bbb2222cc"))
	lines = append(lines, kv("SubnetId", "subnet-0ddd444444444444d"))
	lines = append(lines, kv("PrivateIpAddress", "10.0.48.175"))
	lines = append(lines, kv("PublicIpAddress", "203.0.113.10"))
	lines = append(lines, sec("SecurityGroups:"))
	lines = append(lines, sub("- GroupId: sg-0aa000000000000f2"))
	lines = append(lines, sub("  GroupName: vpn-sg"))
	lines = append(lines, kv("LaunchTime", "2025-07-25 12:26:50"))
	lines = append(lines, kv("Architecture", "x86_64"))
	lines = append(lines, kv("Platform", "-"))
	lines = append(lines, sec("Tags:"))
	lines = append(lines, sub("- Key: Name"))
	lines = append(lines, sub("  Value: VPN"))

	var sb strings.Builder
	sb.WriteString(renderHeader(w))
	sb.WriteString("\n")
	sb.WriteString(renderBox(lines, "i-0bbb222222222222b", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── AFTER: fixed layout ────────────────────────────────────────────────────────
//
// All top-level keys (scalar AND section header) use a uniform 1-space left
// margin. Sub-fields keep their 5-space indent. Nothing else changes.

const keyColW = 22 // key column width including the trailing colon

// top-level scalar field: 1-space margin
func kvScalar(key, val string) string {
	kStyle := lipgloss.NewStyle().Foreground(colKey)
	vStyle := lipgloss.NewStyle().Foreground(colVal)
	return " " + kStyle.Render(padRight(key+":", keyColW)) + vStyle.Render(val)
}

// top-level scalar field with status coloring on the value
func kvStatus(key, val string) string {
	kStyle := lipgloss.NewStyle().Foreground(colKey)
	return " " + kStyle.Render(padRight(key+":", keyColW)) + statusStyle(val).Render(val)
}

// top-level section header (struct / slice): 1-space margin — same as scalar
func kvSection(key string) string {
	kStyle := lipgloss.NewStyle().Foreground(colKey)
	return " " + kStyle.Render(key+":")
}

// sub-field line inside a section body: 5-space indent (unchanged)
func subLine(s string) string {
	subStyle := lipgloss.NewStyle().Foreground(colSub)
	return "     " + subStyle.Render(s)
}

func renderAfter() string {
	const w = 84

	var lines []string
	lines = append(lines, kvScalar("InstanceId", "i-0bbb222222222222b"))
	// State is a struct section — header now at 1-space, same as scalars
	lines = append(lines, kvSection("State"))
	lines = append(lines, subLine(`Code: "16"`))
	lines = append(lines, subLine("Name: running"))
	lines = append(lines, kvScalar("InstanceType", "t3.large"))
	lines = append(lines, kvScalar("ImageId", "ami-0aaa111111111111a"))
	lines = append(lines, kvScalar("VpcId", "vpc-0aaa1111bbb2222cc"))
	lines = append(lines, kvScalar("SubnetId", "subnet-0ddd444444444444d"))
	lines = append(lines, kvScalar("PrivateIpAddress", "10.0.48.175"))
	lines = append(lines, kvScalar("PublicIpAddress", "203.0.113.10"))
	// SecurityGroups is a slice section — same 1-space margin
	lines = append(lines, kvSection("SecurityGroups"))
	lines = append(lines, subLine("- GroupId: sg-0aa000000000000f2"))
	lines = append(lines, subLine("  GroupName: vpn-sg"))
	lines = append(lines, kvScalar("LaunchTime", "2025-07-25 12:26:50"))
	lines = append(lines, kvScalar("Architecture", "x86_64"))
	lines = append(lines, kvScalar("Platform", "-"))
	// Tags section — same 1-space margin
	lines = append(lines, kvSection("Tags"))
	lines = append(lines, subLine("- Key: Name"))
	lines = append(lines, subLine("  Value: VPN"))

	var sb strings.Builder
	sb.WriteString(renderHeader(w))
	sb.WriteString("\n")
	sb.WriteString(renderBox(lines, "i-0bbb222222222222b", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── AFTER: side-by-side ruler showing margin positions ────────────────────────
//
// This extra panel makes it easy to verify column positions visually.

func renderRuler(w int) string {
	// Columns 1-based, showing every 5th position
	ruler := " "
	for i := 2; i <= w-2; i++ {
		if i%10 == 0 {
			ruler += fmt.Sprintf("%d", (i/10)%10)
		} else if i%5 == 0 {
			ruler += "┊"
		} else {
			ruler += "·"
		}
	}
	return lipgloss.NewStyle().Foreground(colDim).Render(ruler)
}

// ── AFTER: additional resource types to confirm rule holds ────────────────────

func renderAfterRDS() string {
	const w = 84

	var lines []string
	lines = append(lines, kvScalar("DBInstanceIdentifier", "mydb-prod"))
	lines = append(lines, kvScalar("Engine", "mysql"))
	lines = append(lines, kvScalar("EngineVersion", "8.0.35"))
	lines = append(lines, kvStatus("DBInstanceStatus", "available"))
	lines = append(lines, kvScalar("DBInstanceClass", "db.t3.medium"))
	lines = append(lines, kvScalar("Endpoint", "mydb-prod.abcdef.us-east-1.rds.amazonaws.com"))
	lines = append(lines, kvScalar("MultiAZ", "No"))
	lines = append(lines, kvScalar("AllocatedStorage", "20 GiB"))
	lines = append(lines, kvScalar("StorageType", "gp2"))
	// Endpoint is a nested struct in the real SDK — show as section
	lines = append(lines, kvSection("EndpointDetails"))
	lines = append(lines, subLine("Address: mydb-prod.abcdef.us-east-1.rds.amazonaws.com"))
	lines = append(lines, subLine("Port: 3306"))
	lines = append(lines, kvScalar("AvailabilityZone", "us-east-1a"))

	var sb strings.Builder
	sb.WriteString(renderHeader(w))
	sb.WriteString("\n")
	sb.WriteString(renderBox(lines, "mydb-prod", w))
	sb.WriteString("\n")
	return sb.String()
}

func renderAfterSecrets() string {
	const w = 84

	var lines []string
	lines = append(lines, kvScalar("Name", "prod/api/database-password"))
	lines = append(lines, kvScalar("Description", "Production database password"))
	lines = append(lines, kvScalar("LastAccessedDate", "2025-07-25 00:00:00"))
	lines = append(lines, kvScalar("LastChangedDate", "2025-07-10 14:23:00"))
	lines = append(lines, kvScalar("RotationEnabled", "Yes"))
	lines = append(lines, kvScalar("ARN", truncate("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/database-password-AbCdEf", 54)))
	lines = append(lines, kvScalar("KmsKeyId", "-"))
	lines = append(lines, kvSection("Tags"))
	lines = append(lines, subLine("- Key: Environment"))
	lines = append(lines, subLine("  Value: production"))
	lines = append(lines, subLine("- Key: Team"))
	lines = append(lines, subLine("  Value: platform"))

	var sb strings.Builder
	sb.WriteString(renderHeader(w))
	sb.WriteString("\n")
	sb.WriteString(renderBox(lines, "prod/api/database-password", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── AFTER: EC2 with deeply nested fields ──────────────────────────────────────
//
// Indentation rules demonstrated:
//   1 space  — top-level keys (scalar and section header)
//   5 spaces — first-level sub-fields (inside a section, or array item opener)
//   9 spaces — second-level sub-fields (nested inside an array item's struct)

// subLine2 renders a second-level sub-field: 9-space indent.
func subLine2(s string) string {
	subStyle := lipgloss.NewStyle().Foreground(colSub)
	return "         " + subStyle.Render(s)
}

func renderAfterEC2Nested() string {
	const w = 84

	var lines []string
	lines = append(lines, kvScalar("InstanceId", "i-0bbb222222222222b"))

	// State — struct section with scalar sub-fields
	lines = append(lines, kvSection("State"))
	lines = append(lines, subLine(`Code: "16"`))
	lines = append(lines, subLine("Name: running"))

	lines = append(lines, kvScalar("InstanceType", "t3.large"))

	// Placement — struct section
	lines = append(lines, kvSection("Placement"))
	lines = append(lines, subLine("AvailabilityZone: eu-west-2a"))
	lines = append(lines, subLine(`GroupName: ""`))
	lines = append(lines, subLine("Tenancy: default"))

	// NetworkInterfaces — slice section; each item opens with "- Key:" at 5-space,
	// continuation fields of that same item at 5-space with "  " prefix.
	lines = append(lines, kvSection("NetworkInterfaces"))
	lines = append(lines, subLine("- NetworkInterfaceId: eni-0abc123"))
	lines = append(lines, subLine("  PrivateIpAddress: 10.0.1.5"))
	lines = append(lines, subLine("  SubnetId: subnet-0ddd444"))
	lines = append(lines, subLine("- NetworkInterfaceId: eni-0def456"))
	lines = append(lines, subLine("  PrivateIpAddress: 10.0.2.10"))
	lines = append(lines, subLine("  SubnetId: subnet-0abc456"))

	// BlockDeviceMappings — slice where each item itself has a nested struct (Ebs).
	// The nested struct fields drop one more level to 9-space indent.
	lines = append(lines, kvSection("BlockDeviceMappings"))
	lines = append(lines, subLine("- DeviceName: /dev/xvda"))
	lines = append(lines, subLine("  Ebs:"))
	lines = append(lines, subLine2("VolumeId: vol-0abc123"))
	lines = append(lines, subLine2("Status: attached"))

	// Tags — simple key/value slice
	lines = append(lines, kvSection("Tags"))
	lines = append(lines, subLine("- Key: Name"))
	lines = append(lines, subLine("  Value: web-server"))
	lines = append(lines, subLine("- Key: Environment"))
	lines = append(lines, subLine("  Value: production"))

	var sb strings.Builder
	sb.WriteString(renderHeader(w))
	sb.WriteString("\n")
	sb.WriteString(renderBox(lines, "i-0bbb222222222222b  (EC2 — deeply nested)", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── AFTER: RDS with nested Endpoint struct + VpcSecurityGroups slice ──────────

func renderAfterRDSNested() string {
	const w = 84

	var lines []string
	lines = append(lines, kvScalar("DBInstanceIdentifier", "mydb-prod"))
	lines = append(lines, kvScalar("Engine", "postgres"))
	lines = append(lines, kvScalar("EngineVersion", "15.4"))
	lines = append(lines, kvStatus("DBInstanceStatus", "available"))

	// Endpoint — nested struct section
	lines = append(lines, kvSection("Endpoint"))
	lines = append(lines, subLine("Address: mydb-prod.cluster-abc123.eu-west-2.rds.amazonaws.com"))
	lines = append(lines, subLine("Port: 5432"))
	lines = append(lines, subLine("HostedZoneId: Z1TTGA775OQIAX"))

	lines = append(lines, kvStatus("MultiAZ", "true"))

	// VpcSecurityGroups — slice section
	lines = append(lines, kvSection("VpcSecurityGroups"))
	lines = append(lines, subLine("- VpcSecurityGroupId: sg-0abc123"))
	lines = append(lines, subLine("  Status: active"))

	var sb strings.Builder
	sb.WriteString(renderHeader(w))
	sb.WriteString("\n")
	sb.WriteString(renderBox(lines, "mydb-prod  (RDS — nested Endpoint + slice)", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── AFTER: indentation legend panel ──────────────────────────────────────────
//
// Renders a self-contained reference showing all three indent levels with
// visual markers so the column positions are unambiguous.

func renderIndentLegend() string {
	const w = 84

	dimSt := lipgloss.NewStyle().Foreground(colDim)
	accSt := lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	marker := func(col int, label string) string {
		// col is 1-based position of the first content character
		prefix := strings.Repeat(" ", col-1)
		return dimSt.Render(prefix+"↳ col "+fmt.Sprintf("%d", col)+"  ") + accSt.Render(label)
	}

	lines := []string{
		marker(1, "top-level key (scalar or section header)   kvScalar / kvSection"),
		marker(5, "first-level sub-field                      subLine"),
		marker(9, "second-level sub-field (inside array item) subLine2"),
	}

	var sb strings.Builder
	sb.WriteString(renderBox(lines, "Indentation legend", w))
	sb.WriteString("\n")
	return sb.String()
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	fmt.Println(divider("BEFORE — Current broken layout (scalars at 3-space, sections at 1-space)"))
	fmt.Println(renderBefore())

	fmt.Println(divider("Column ruler  (· = 1 col, ┊ = 5th col, digit = 10th col)"))
	fmt.Println(renderRuler(84))
	fmt.Println()

	fmt.Println(divider("AFTER — Fixed layout (all top-level keys at 1-space)"))
	fmt.Println(renderAfter())

	fmt.Println(divider("AFTER — RDS instance (scalar + nested struct section)"))
	fmt.Println(renderAfterRDS())

	fmt.Println(divider("AFTER — Secrets Manager (Tags section)"))
	fmt.Println(renderAfterSecrets())

	fmt.Println(divider("AFTER — EC2 deeply nested  (State · Placement · NetworkInterfaces · BlockDeviceMappings · Tags)"))
	fmt.Println(renderAfterEC2Nested())

	fmt.Println(divider("AFTER — RDS nested Endpoint + VpcSecurityGroups slice"))
	fmt.Println(renderAfterRDSNested())

	fmt.Println(divider("Indentation legend  (col positions for all three levels)"))
	fmt.Println(renderIndentLegend())
}
