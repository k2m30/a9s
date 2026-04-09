// preview renders static mockups of the a9s TUI design using Lipgloss v2.
// Run with: go run ./cmd/preview/
//
// Layout (every view):
//
//	HEADER   (1 line, unframed) — left: "a9s v0.x.x  profile:region"
//	                            — right: "? for help"  (or filter/cmd input, or flash message)
//	┌──────────────── TITLE ─────────────────────────────────────┐
//	│ content                                                     │
//	└─────────────────────────────────────────────────────────────┘
package main

import (
	"fmt"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// ── Palette (Tokyo Night Dark) ────────────────────────────────────────────────

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

	colYAMLStr  = lipgloss.Color("#9ece6a")
	colYAMLNum  = lipgloss.Color("#ff9e64")
	colYAMLBool = lipgloss.Color("#bb9af7")

	colHelpKey = lipgloss.Color("#9ece6a")
	colHelpCat = lipgloss.Color("#e0af68")

	colFilter  = lipgloss.Color("#e0af68")
	colSuccess = lipgloss.Color("#9ece6a")
	colError   = lipgloss.Color("#f7768e")
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func padOrTrunc(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		if w <= 1 {
			return s[:w]
		}
		r := []rune(s)
		if len(r) > w-1 {
			return string(r[:w-1]) + "…"
		}
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

// ── Header ────────────────────────────────────────────────────────────────────
// One unframed line:
//   LEFT:  "a9s v0.x.x  profile:region"
//   RIGHT: "? for help"  (or a custom right string for input/flash states)
//
// rightContent variants:
//   normal:          "? for help"  (dim)
//   filter active:   "/search-text█"  (amber) — same table, fewer rows, count in title
//   command mode:    ":ec2█"  (amber)
//   transient flash: "Copied!" or "Error: ..."  (green or red)

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

	// padding: 1 char on each side
	innerW := w - 2
	gap := max(innerW-leftW-rightW, 1)

	content := left + strings.Repeat(" ", gap) + rightContent
	return lipgloss.NewStyle().
		Foreground(colHeaderFg).
		Width(w).Padding(0, 1).Render(content)
}

// renderHeaderNormal returns the standard header with "? for help" on the right.
func renderHeaderNormal(profile, region, version string, w int) string {
	right := lipgloss.NewStyle().Foreground(colDim).Render("? for help")
	return renderHeader(profile, region, version, w, right)
}

// renderHeaderCommand returns the header with ":cmd█" on the right.
func renderHeaderCommand(profile, region, version, cmdText string, w int) string {
	right := lipgloss.NewStyle().Foreground(colFilter).Bold(true).Render(":"+cmdText) +
		lipgloss.NewStyle().Foreground(colFilter).Render("█")
	return renderHeader(profile, region, version, w, right)
}

// renderHeaderFlash returns the header with a transient message on the right.
// msgType: "success" | "error"
func renderHeaderFlash(profile, region, version, msg, msgType string, w int) string {
	var right string
	switch msgType {
	case "success":
		right = lipgloss.NewStyle().Foreground(colSuccess).Bold(true).Render(msg)
	case "error":
		right = lipgloss.NewStyle().Foreground(colError).Bold(true).Render(msg)
	default:
		right = lipgloss.NewStyle().Foreground(colHeaderFg).Render(msg)
	}
	return renderHeader(profile, region, version, w, right)
}

// ── Table column helpers ──────────────────────────────────────────────────────

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

// ── Framed box with centered title in top border ──────────────────────────────
//
// Produces exactly:
//
//	┌──────────────── title ─────────────────────────────────┐
//	│ content line padded to fill inner width                │
//	│ content line                                           │
//	└────────────────────────────────────────────────────────┘
//
// The title is centered between the two corner characters.
// w is the total frame width including the two border characters.
// title is embedded in the top border (pass "" for a plain top border).
func renderFramedBox(lines []string, title string, w int) string {
	borderStyle := lipgloss.NewStyle().Foreground(colBorderNormal)
	innerW := w - 2 // space between left │ and right │

	// top border
	var topBorder string
	if title == "" {
		topBorder = borderStyle.Render("┌" + strings.Repeat("─", w-2) + "┐")
	} else {
		titleRendered := lipgloss.NewStyle().Foreground(colHeaderFg).Bold(true).Render(title)
		titleVis := lipgloss.Width(titleRendered)

		// Total dashes available = (w - 2) - titleVis - 2 spaces around title
		// "┌" + leftDashes + " " + title + " " + rightDashes + "┐"
		// leftDashes + rightDashes = w - 2 - titleVis - 2
		totalDashes := max(w-2-titleVis-2, 2)
		leftDashes := totalDashes / 2
		rightDashes := totalDashes - leftDashes

		prefix := "┌" + strings.Repeat("─", leftDashes) + " "
		suffix := " " + strings.Repeat("─", rightDashes) + "┐"
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
		sb.WriteString(borderStyle.Render("│"))
		sb.WriteString(padded)
		sb.WriteString(borderStyle.Render("│"))
	}

	sb.WriteString("\n")
	sb.WriteString(borderStyle.Render("└" + strings.Repeat("─", w-2) + "┘"))

	return sb.String()
}

// ── Preview section divider ───────────────────────────────────────────────────

func divider(label string) string {
	line := strings.Repeat("━", 38)
	return "\n" +
		lipgloss.NewStyle().Foreground(colSep).Render(line) +
		"  " +
		lipgloss.NewStyle().Foreground(colAccent).Bold(true).Render(label) +
		"  " +
		lipgloss.NewStyle().Foreground(colSep).Render(line) +
		"\n\n"
}

// ── VIEW 1: Main Menu ─────────────────────────────────────────────────────────

func renderMainMenu() string {
	const w = 82

	type menuItem struct {
		cmd  string // command alias, e.g. "ec2"
		name string // display name, e.g. "EC2 Instances"
	}
	items := []menuItem{
		{"ec2", "EC2 Instances"},
		{"s3", "S3 Buckets"},
		{"rds", "RDS Instances"},
		{"redis", "ElastiCache Redis"},
		{"docdb", "DocumentDB Clusters"},
		{"eks", "EKS Clusters"},
		{"secrets", "Secrets Manager"},
	}

	innerW := w - 2 // space between the two │ border characters

	dimStyle := lipgloss.NewStyle().Foreground(colDim)

	var lines []string

	for i, item := range items {
		// Right side: dimmed ":alias" — fixed width 9 chars (":secrets  " is 9 visible)
		aliasStr := ":" + item.cmd
		aliasW := 9 // widest alias ":secrets" = 8 + 1 trailing space = 9
		aliasPadded := padOrTrunc(aliasStr, aliasW)

		// Layout: "  " + name + gap + alias
		// "  " prefix = 2, alias field = aliasW, trailing " " = 1  → name field = innerW - 2 - aliasW - 1
		nameFieldW := innerW - 2 - aliasW - 1
		namePadded := padOrTrunc(item.name, nameFieldW)

		if i == 0 {
			// Selected row: full blue background; alias stays dimmed on the right.
			selectedAlias := dimStyle.Render(aliasPadded)
			selectedName := "  " + namePadded + " "
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(selectedName + selectedAlias)
			lines = append(lines, line)
		} else {
			alias := dimStyle.Render(aliasPadded)
			name := lipgloss.NewStyle().Foreground(colHeaderFg).Render("  " + namePadded + " ")
			lines = append(lines, name+alias)
		}
	}

	lines = append(lines, "")
	lines = append(lines,
		lipgloss.NewStyle().Foreground(colDim).Render("  7 resource types"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("default", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "resource-types(7)", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 2: Resource List — normal state ─────────────────────────────────────

func renderResourceListNormal() string {
	const w = 110

	cols := []col{
		{"NAME\u2191", 22},
		{"STATUS", 11},
		{"TYPE", 10},
		{"AZ", 12},
		{"AMI", 18},
		{"LAUNCH TIME", 17},
	}

	type ec2row struct {
		name, status, itype, az, ami, launch string
	}
	rows := []ec2row{
		{"api-prod-01", "running", "t3.medium", "us-east-1a", "ami-0abcdef01234", "2024-01-15 09:22"},
		{"api-prod-02", "running", "t3.medium", "us-east-1b", "ami-0abcdef01234", "2024-01-15 09:25"},
		{"worker-01", "running", "t3.large", "us-east-1a", "ami-0abcdef01234", "2024-01-10 14:30"},
		{"worker-02", "pending", "t3.large", "us-east-1b", "ami-0abcdef01234", "2024-03-17 08:00"},
		{"bastion", "running", "t2.micro", "us-east-1a", "ami-0zzz11122233", "2023-11-01 10:00"},
		{"old-worker", "stopped", "t3.medium", "us-east-1c", "ami-0abcdef01234", "2023-06-20 16:45"},
		{"legacy-app", "terminated", "t2.small", "us-east-1a", "ami-0000111222333", "2022-12-01 12:00"},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerText := " " + strings.Join(headerParts, "  ")
	headerLine := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render(headerText)

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.name, cols[0].width),
			padOrTrunc(r.status, cols[1].width),
			padOrTrunc(r.itype, cols[2].width),
			padOrTrunc(r.az, cols[3].width),
			padOrTrunc(r.ami, cols[4].width),
			padOrTrunc(r.launch, cols[5].width),
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

	lines = append(lines,
		lipgloss.NewStyle().Foreground(colDim).Render("  · · · (35 more rows)"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ec2-instances(42)", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 3b: Resource List — command mode active ──────────────────────────────

func renderResourceListCommand() string {
	const w = 110

	cols := []col{
		{"NAME\u2191", 22},
		{"STATUS", 11},
		{"TYPE", 10},
		{"AZ", 12},
		{"AMI", 18},
		{"LAUNCH TIME", 17},
	}

	type ec2row struct {
		name, status, itype, az, ami, launch string
	}
	rows := []ec2row{
		{"api-prod-01", "running", "t3.medium", "us-east-1a", "ami-0abcdef01234", "2024-01-15 09:22"},
		{"api-prod-02", "running", "t3.medium", "us-east-1b", "ami-0abcdef01234", "2024-01-15 09:25"},
		{"worker-01", "running", "t3.large", "us-east-1a", "ami-0abcdef01234", "2024-01-10 14:30"},
	}

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().Foreground(colAccent).Bold(true).
		Render(" " + strings.Join(headerParts, "  "))

	innerW := w - 2

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.name, cols[0].width),
			padOrTrunc(r.status, cols[1].width),
			padOrTrunc(r.itype, cols[2].width),
			padOrTrunc(r.az, cols[3].width),
			padOrTrunc(r.ami, cols[4].width),
			padOrTrunc(r.launch, cols[5].width),
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

	var sb strings.Builder
	// Header shows the live command input on the right
	sb.WriteString(renderHeaderCommand("prod", "us-east-1", "0.5.0", "ec2", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ec2-instances(42)", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 3c: Resource List — flash message (Copied!) ─────────────────────────

func renderResourceListFlash() string {
	const w = 110

	cols := []col{
		{"NAME\u2191", 22},
		{"STATUS", 11},
		{"TYPE", 10},
		{"AZ", 12},
		{"AMI", 18},
		{"LAUNCH TIME", 17},
	}

	type ec2row struct {
		name, status, itype, az, ami, launch string
	}
	rows := []ec2row{
		{"api-prod-01", "running", "t3.medium", "us-east-1a", "ami-0abcdef01234", "2024-01-15 09:22"},
		{"api-prod-02", "running", "t3.medium", "us-east-1b", "ami-0abcdef01234", "2024-01-15 09:25"},
		{"worker-01", "running", "t3.large", "us-east-1a", "ami-0abcdef01234", "2024-01-10 14:30"},
	}

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().Foreground(colAccent).Bold(true).
		Render(" " + strings.Join(headerParts, "  "))

	innerW := w - 2

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.name, cols[0].width),
			padOrTrunc(r.status, cols[1].width),
			padOrTrunc(r.itype, cols[2].width),
			padOrTrunc(r.az, cols[3].width),
			padOrTrunc(r.ami, cols[4].width),
			padOrTrunc(r.launch, cols[5].width),
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

	var sb strings.Builder
	// Header shows a transient success flash on the right
	sb.WriteString(renderHeaderFlash("prod", "us-east-1", "0.5.0", "Copied!", "success", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ec2-instances(42)", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 4: Detail View ───────────────────────────────────────────────────────

func renderDetailView() string {
	const w = 84

	secStyle := lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	kStyle := lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	kw := 24

	kv := func(key, val string) string {
		return "   " + kStyle.Render(padOrTrunc(key, kw)) + vStyle.Render(val)
	}
	kvStatus := func(key, val string) string {
		return "   " + kStyle.Render(padOrTrunc(key, kw)) + rowColorStyle(val).Render(val)
	}
	sec := func(s string) string {
		return " " + secStyle.Render(s)
	}

	var lines []string
	lines = append(lines, sec("Identity"))
	lines = append(lines, kv("InstanceId", "i-0abc123def456789a"))
	lines = append(lines, kv("InstanceType", "t3.medium"))
	lines = append(lines, kv("ImageId", "ami-0abcdef01234567"))
	lines = append(lines, kv("KeyName", "prod-keypair"))
	lines = append(lines, "")
	lines = append(lines, sec("Network"))
	lines = append(lines, kv("VpcId", "vpc-0123456789abcdef0"))
	lines = append(lines, kv("SubnetId", "subnet-0123456789abcde"))
	lines = append(lines, kv("PrivateIpAddress", "10.0.1.42"))
	lines = append(lines, kv("PublicIpAddress", "54.123.45.67"))
	lines = append(lines, "")
	lines = append(lines, sec("State"))
	lines = append(lines, kvStatus("State.Name", "running"))
	lines = append(lines, kv("LaunchTime", "2024-01-15T09:22:31Z"))
	lines = append(lines, kv("Placement.Zone", "us-east-1a"))
	lines = append(lines, "")
	lines = append(lines, sec("Tags"))
	lines = append(lines, kv("Name", "api-prod-01"))
	lines = append(lines, kv("Environment", "production"))
	lines = append(lines, kv("Team", "platform"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "i-0abc123def456789a", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 5: Help Screen ───────────────────────────────────────────────────────

func renderHelpScreen() string {
	const w = 84

	catStyle := lipgloss.NewStyle().Foreground(colHelpCat).Bold(true)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	bind := func(k, d string) string {
		return hkStyle.Render(padOrTrunc(k, 9)) + descStyle.Render(d)
	}

	colW := 20
	catRow := catStyle.Render(padOrTrunc("RESOURCE", colW)) +
		catStyle.Render(padOrTrunc("GENERAL", colW)) +
		catStyle.Render(padOrTrunc("NAVIGATION", colW)) +
		catStyle.Render("HOTKEYS")

	type bindRow struct {
		c1, c2, c3, c4 string
	}
	bindRows := []bindRow{
		{bind("<esc>", "Back  "), bind("<ctrl-r>", "Refresh  "), bind("<j>", "Down      "), bind("<?>", "Help")},
		{bind("<q>", "Quit  "), bind("<q>", "Quit     "), bind("<k>", "Up        "), bind("<:>", "Command")},
		{"", bind("<:>", "Command  "), bind("<g>", "Top       "), ""},
		{"", bind("</>", "Filter   "), bind("<G>", "Bottom    "), ""},
		{"", "", bind("<h/l>", "Cols      "), ""},
		{"", "", bind("<enter>", "Open      "), ""},
		{"", "", bind("<d>", "Detail    "), ""},
		{"", "", bind("<y>", "YAML      "), ""},
		{"", "", bind("<c>", "Copy ID   "), ""},
		{"", "", bind("<N/S/A>", "Sort      "), ""},
	}

	var lines []string
	lines = append(lines, " "+catRow)
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
			lipgloss.NewStyle().Foreground(colDim).Render("Press any key to close")))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Help", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 6: YAML View ─────────────────────────────────────────────────────────

func renderYAMLView() string {
	const w = 84

	yamlKey := lipgloss.NewStyle().Foreground(colDetailKey)
	yamlStr := lipgloss.NewStyle().Foreground(colYAMLStr)
	yamlNum := lipgloss.NewStyle().Foreground(colYAMLNum)
	yamlBool := lipgloss.NewStyle().Foreground(colYAMLBool)
	yamlTree := lipgloss.NewStyle().Foreground(colSep)
	i1 := " " + yamlTree.Render("│") + "   "
	i2 := " " + yamlTree.Render("│") + "     "

	l := func(s string) string { return " " + s }

	var lines []string
	lines = append(lines, l(yamlKey.Render("AmiLaunchIndex")+": "+yamlNum.Render("0")))
	lines = append(lines, l(yamlKey.Render("Architecture")+": "+yamlStr.Render("x86_64")))
	lines = append(lines, l(yamlKey.Render("BlockDeviceMappings")+":"))
	lines = append(lines, i1+"- "+yamlKey.Render("DeviceName")+": "+yamlStr.Render("/dev/xvda"))
	lines = append(lines, i1+"  "+yamlKey.Render("Ebs")+":")
	lines = append(lines, i2+yamlKey.Render("AttachTime")+": "+yamlStr.Render("2024-01-15T09:22:45Z"))
	lines = append(lines, i2+yamlKey.Render("DeleteOnTermination")+": "+yamlBool.Render("true"))
	lines = append(lines, i2+yamlKey.Render("Status")+": "+yamlStr.Render("attached"))
	lines = append(lines, i2+yamlKey.Render("VolumeId")+": "+yamlStr.Render("vol-0abc123def456789a"))
	lines = append(lines, l(yamlKey.Render("ImageId")+": "+yamlStr.Render("ami-0abcdef01234567")))
	lines = append(lines, l(yamlKey.Render("InstanceId")+": "+yamlStr.Render("i-0abc123def456789a")))
	lines = append(lines, l(yamlKey.Render("InstanceType")+": "+yamlStr.Render("t3.medium")))
	lines = append(lines, l(yamlKey.Render("KeyName")+": "+yamlStr.Render("prod-keypair")))
	lines = append(lines, l(yamlKey.Render("LaunchTime")+": "+yamlStr.Render("2024-01-15T09:22:31Z")))
	lines = append(lines, l(yamlKey.Render("Placement")+":"))
	lines = append(lines, i1+yamlKey.Render("AvailabilityZone")+": "+yamlStr.Render("us-east-1a"))
	lines = append(lines, i1+yamlKey.Render("GroupName")+": "+yamlStr.Render(`""`))
	lines = append(lines, i1+yamlKey.Render("Tenancy")+": "+yamlStr.Render("default"))
	lines = append(lines, l(yamlKey.Render("PrivateIpAddress")+": "+yamlStr.Render("10.0.1.42")))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "i-0abc123def456789a yaml", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 7: Lambda Function Code — Normal (Python handler) ──────────────────

func renderLambdaCodeNormal() string {
	const w = 88

	lineNumStyle := lipgloss.NewStyle().Foreground(colDim)
	pipeStyle := lipgloss.NewStyle().Foreground(colSep)
	codeStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	sourceLines := []string{
		"import json",
		"import boto3",
		"from datetime import datetime",
		"",
		"s3 = boto3.client('s3')",
		"dynamodb = boto3.resource('dynamodb')",
		"",
		"def process(event, context):",
		`    """Process incoming payment events."""`,
		"    order_id = event['detail']['order_id']",
		"    amount = event['detail']['amount']",
		"    currency = event['detail']['currency']",
		"",
		"    # Validate payment details",
		"    if amount <= 0:",
		`        raise ValueError(f"Invalid amount: {amount}")`,
		"",
		"    # Call Stripe API",
		"    stripe_client = boto3.client('secretsmanager')",
		"    api_key = stripe_client.get_secret_value(",
	}

	pipe := pipeStyle.Render(" \u2502 ")

	var lines []string
	for i, src := range sourceLines {
		num := fmt.Sprintf("%2d", i+1)
		numRendered := lineNumStyle.Render(num)
		codeRendered := codeStyle.Render(src)
		lines = append(lines, " "+numRendered+pipe+codeRendered)
	}

	lines = append(lines,
		lipgloss.NewStyle().Foreground(colDim).Render("    · · · (scroll for more)"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "lambda-code \u2014 payment-processor/handler.py", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 7b: Lambda Function Code — Container Image Lambda ──────────────────

func renderLambdaCodeContainerImage() string {
	const w = 88

	msgStyle := lipgloss.NewStyle().Foreground(colPending).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(colDim)
	valStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, " "+msgStyle.Render("Container image Lambda \u2014 source code not viewable"))
	lines = append(lines, "")
	lines = append(lines, " "+labelStyle.Render("Package type:  ")+valStyle.Render("Image"))
	lines = append(lines, " "+labelStyle.Render("Image URI:     ")+valStyle.Render("123456789012.dkr.ecr.us-east-1.amazonaws.com/payment:latest"))
	lines = append(lines, "")
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "lambda-code \u2014 payment-processor", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 7c: Lambda Function Code — Package Too Large ───────────────────────

func renderLambdaCodeTooLarge() string {
	const w = 88

	msgStyle := lipgloss.NewStyle().Foreground(colPending).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(colDim)
	valStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	sizeStyle := lipgloss.NewStyle().Foreground(colError)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, " "+msgStyle.Render("Package too large for inline viewing (23.4 MB)"))
	lines = append(lines, "")
	lines = append(lines, " "+labelStyle.Render("Handler:   ")+valStyle.Render("handler.process"))
	lines = append(lines, " "+labelStyle.Render("Runtime:   ")+valStyle.Render("python3.12"))
	lines = append(lines, " "+labelStyle.Render("Code size: ")+sizeStyle.Render("23.4 MB")+valStyle.Render(" (limit: 5 MB)"))
	lines = append(lines, "")
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "lambda-code \u2014 payment-processor", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 7d: Lambda Function Code — Help Screen ────────────────────────────

func renderLambdaCodeHelp() string {
	const w = 84

	catStyle := lipgloss.NewStyle().Foreground(colHelpCat).Bold(true)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	bind := func(k, d string) string {
		return hkStyle.Render(padOrTrunc(k, 9)) + descStyle.Render(d)
	}

	colW := 20
	catRow := catStyle.Render(padOrTrunc("FUNCTION CODE", colW)) +
		catStyle.Render(padOrTrunc("GENERAL", colW)) +
		catStyle.Render(padOrTrunc("NAVIGATION", colW)) +
		catStyle.Render("HOTKEYS")

	type bindRow struct {
		c1, c2, c3, c4 string
	}
	bindRows := []bindRow{
		{bind("<esc>", "Back  "), bind("<ctrl-r>", "Refresh  "), bind("<j>", "Down      "), bind("<?>", "Help")},
		{bind("<c>", "Copy  "), "", bind("<k>", "Up        "), bind("<:>", "Command")},
		{bind("<w>", "Wrap  "), "", bind("<g>", "Top       "), ""},
		{"", "", bind("<G>", "Bottom    "), ""},
		{"", "", bind("<pgup/dn>", "Page      "), ""},
	}

	var lines []string
	lines = append(lines, " "+catRow)
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
			lipgloss.NewStyle().Foreground(colDim).Render("Press any key to close")))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.5.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Help", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── Header with account badge ──────────────────────────────────────────────

// renderHeaderWithBadge returns the header with account alias/ID badge after profile:region.
func renderHeaderWithBadge(profile, region, version, badge string, w int, rightContent string) string {
	accent := lipgloss.NewStyle().
		Foreground(colAccent).Bold(true).Render("a9s")
	ver := lipgloss.NewStyle().
		Foreground(colDim).Render(" v" + version)
	ctx := lipgloss.NewStyle().
		Foreground(colHeaderFg).Bold(true).
		Render("  " + profile + ":" + region)

	var badgeRendered string
	if badge != "" {
		badgeRendered = lipgloss.NewStyle().
			Foreground(colDim).Render(" (" + badge + ")")
	}

	left := accent + ver + ctx + badgeRendered
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(rightContent)

	innerW := w - 2
	gap := max(innerW-leftW-rightW, 1)

	content := left + strings.Repeat(" ", gap) + rightContent
	return lipgloss.NewStyle().
		Foreground(colHeaderFg).
		Width(w).Padding(0, 1).Render(content)
}

// renderHeaderBadgeNormal returns the header with account badge and "? for help" on right.
func renderHeaderBadgeNormal(profile, region, version, badge string, w int) string {
	right := lipgloss.NewStyle().Foreground(colDim).Render("? for help")
	return renderHeaderWithBadge(profile, region, version, badge, w, right)
}

// renderHeaderBadgeExpiry returns the header with account badge and session expiry warning.
func renderHeaderBadgeExpiry(profile, region, version, badge, expiryText string, expiryLevel string, w int) string {
	var expiryRendered string
	switch expiryLevel {
	case "warning":
		expiryRendered = lipgloss.NewStyle().Foreground(colPending).Render(expiryText + " ")
	case "critical":
		expiryRendered = lipgloss.NewStyle().Foreground(colError).Bold(true).Render(expiryText + " ")
	}
	helpHint := lipgloss.NewStyle().Foreground(colDim).Render("? for help")
	right := expiryRendered + helpHint
	return renderHeaderWithBadge(profile, region, version, badge, w, right)
}

// ── VIEW 8: Header with account badge (various states) ──────────────────────

// ── VIEW 7e: EC2 Status Checks — Mixed (some failing) ─────────────────────

func renderEC2StatusChecksMixed() string {
	const w = 114

	cols := []col{
		{"NAME\u2191", 22},
		{"STATE", 12},
		{"LIFECYCLE", 10},
		{"TYPE", 10},
		{"PRIVATE IP", 16},
		{"INSTANCE ID", 22},
	}

	type ec2row struct {
		name, state, lifecycle, itype, ip, id string
		checkStatus                           string // "ok", "impaired", "initializing", ""
	}
	rows := []ec2row{
		{"api-prod-01", "running", "on-demand", "t3.medium", "10.0.1.42", "i-0abc123def456789a", "ok"},
		{"api-prod-02", "running", "on-demand", "t3.medium", "10.0.1.43", "i-0abc123def456789b", "ok"},
		{"worker-01", "running", "on-demand", "t3.large", "10.0.2.10", "i-0def456789abcdef0", "impaired"},
		{"worker-02", "running", "on-demand", "t3.large", "10.0.2.11", "i-0def456789abcdef1", "initializing"},
		{"bastion", "running", "on-demand", "t2.micro", "10.0.0.5", "i-0111222333aaabbb0", "ok"},
		{"old-worker", "stopped", "on-demand", "t3.medium", "10.0.1.99", "i-0aaa111222bbbccc0", ""},
		{"legacy-app", "terminated", "on-demand", "t2.small", "-", "i-0000111222cccddd0", ""},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().Foreground(colAccent).Bold(true).
		Render(" " + strings.Join(headerParts, "  "))

	warnStyle := lipgloss.NewStyle().Foreground(colStopped).Bold(true)
	initStyle := lipgloss.NewStyle().Foreground(colPending)

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		// Build the STATE cell with optional prefix
		stateCell := r.state
		if r.state == "running" {
			switch r.checkStatus {
			case "impaired":
				stateCell = warnStyle.Render("!") + " " + r.state
			case "initializing":
				stateCell = initStyle.Render("~") + " " + r.state
			}
		}

		cells := []string{
			padOrTrunc(r.name, cols[0].width),
			padOrTrunc(stateCell, cols[1].width),
			padOrTrunc(r.lifecycle, cols[2].width),
			padOrTrunc(r.itype, cols[3].width),
			padOrTrunc(r.ip, cols[4].width),
			padOrTrunc(r.id, cols[5].width),
		}
		rowText := " " + strings.Join(cells, "  ")

		switch i {
		case 0:
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		case 2:
			// worker-01: selected row with failed check — ! stays red on blue bg
			// Compose: row bg is blue, but the ! glyph keeps its red fg
			rowBase := rowColorStyle(r.state)
			stateWithWarn := warnStyle.Render("!") + " " + rowBase.Render(r.state)
			cellsCustom := []string{
				padOrTrunc(rowBase.Render(r.name), cols[0].width),
				padOrTrunc(stateWithWarn, cols[1].width),
				padOrTrunc(rowBase.Render(r.lifecycle), cols[2].width),
				padOrTrunc(rowBase.Render(r.itype), cols[3].width),
				padOrTrunc(rowBase.Render(r.ip), cols[4].width),
				padOrTrunc(rowBase.Render(r.id), cols[5].width),
			}
			rowTextCustom := " " + strings.Join(cellsCustom, "  ")
			lines = append(lines, rowTextCustom)
		default:
			style := rowColorStyle(r.state)
			lines = append(lines, style.Render(rowText))
		}
	}

	lines = append(lines,
		lipgloss.NewStyle().Foreground(colDim).Render("  · · · (35 more rows)"))

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.8.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ec2-instances(42)", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 7f: EC2 Status Checks — All Healthy ────────────────────────────────

func renderEC2StatusChecksHealthy() string {
	const w = 114

	cols := []col{
		{"NAME\u2191", 22},
		{"STATE", 12},
		{"LIFECYCLE", 10},
		{"TYPE", 10},
		{"PRIVATE IP", 16},
		{"INSTANCE ID", 22},
	}

	type ec2row struct {
		name, state, lifecycle, itype, ip, id string
	}
	rows := []ec2row{
		{"api-prod-01", "running", "on-demand", "t3.medium", "10.0.1.42", "i-0abc123def456789a"},
		{"api-prod-02", "running", "on-demand", "t3.medium", "10.0.1.43", "i-0abc123def456789b"},
		{"worker-01", "running", "on-demand", "t3.large", "10.0.2.10", "i-0def456789abcdef0"},
		{"worker-02", "running", "on-demand", "t3.large", "10.0.2.11", "i-0def456789abcdef1"},
	}

	innerW := w - 2

	headerParts := make([]string, len(cols))
	for i, c := range cols {
		headerParts[i] = padOrTrunc(c.title, c.width)
	}
	headerLine := lipgloss.NewStyle().Foreground(colAccent).Bold(true).
		Render(" " + strings.Join(headerParts, "  "))

	var lines []string
	lines = append(lines, headerLine)

	for i, r := range rows {
		cells := []string{
			padOrTrunc(r.name, cols[0].width),
			padOrTrunc(r.state, cols[1].width),
			padOrTrunc(r.lifecycle, cols[2].width),
			padOrTrunc(r.itype, cols[3].width),
			padOrTrunc(r.ip, cols[4].width),
			padOrTrunc(r.id, cols[5].width),
		}
		rowText := " " + strings.Join(cells, "  ")

		if i == 0 {
			line := lipgloss.NewStyle().
				Background(colRowSelected).Foreground(colRowSelectedFg).Bold(true).
				Width(innerW).Render(rowText)
			lines = append(lines, line)
		} else {
			style := rowColorStyle(r.state)
			lines = append(lines, style.Render(rowText))
		}
	}

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.8.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "ec2-instances(42)", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 7g: EC2 Status Checks — Detail View (failed check) ─────────────────

func renderEC2StatusChecksDetail() string {
	const w = 84

	secStyle := lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	kStyle := lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	kw := 24

	kv := func(key, val string) string {
		return " " + kStyle.Render(padOrTrunc(key+":", kw)) + vStyle.Render(val)
	}
	kvColor := func(key, val string, valColor color.Color) string {
		return " " + kStyle.Render(padOrTrunc(key+":", kw)) +
			lipgloss.NewStyle().Foreground(valColor).Render(val)
	}
	sec := func(s string) string {
		return " " + secStyle.Render(s)
	}
	subKV := func(key, val string, valColor color.Color) string {
		return "     " + kStyle.Render(padOrTrunc(key+":", 12)) +
			lipgloss.NewStyle().Foreground(valColor).Render(val)
	}

	var lines []string
	lines = append(lines, sec("Identity"))
	lines = append(lines, kv("InstanceId", "i-0def456789abcdef0"))
	lines = append(lines, kv("InstanceType", "t3.large"))
	lines = append(lines, kv("ImageId", "ami-0abcdef01234567"))
	lines = append(lines, "")
	lines = append(lines, sec("State"))
	lines = append(lines, kvColor("State.Name", "running", colRunning))
	lines = append(lines, "")
	lines = append(lines, sec("Status Checks"))
	lines = append(lines, subKV("System", "ok", colRunning))
	lines = append(lines, subKV("Instance", "impaired", colStopped))
	lines = append(lines, "")
	lines = append(lines, sec("Network"))
	lines = append(lines, kv("VpcId", "vpc-0123456789abcdef0"))
	lines = append(lines, kv("SubnetId", "subnet-0123456789abcde"))
	lines = append(lines, kv("PrivateIpAddress", "10.0.2.10"))
	lines = append(lines, "")
	lines = append(lines, sec("Tags"))
	lines = append(lines, kv("Name", "worker-01"))
	lines = append(lines, kv("Environment", "production"))

	_ = kvColor // used above

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "0.8.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "i-0def456789abcdef0", w))
	sb.WriteString("\n")

	return sb.String()
}

func renderHeaderVariants() string {
	const w = 110

	var sb strings.Builder

	// Variant 1: Normal with alias
	sb.WriteString("  Header with account alias:\n")
	sb.WriteString(renderHeaderBadgeNormal("prod-admin", "us-east-1", "3.11.0", "acme-prod", w))
	sb.WriteString("\n\n")

	// Variant 2: Normal with account ID (no alias)
	sb.WriteString("  Header with account ID (no alias):\n")
	sb.WriteString(renderHeaderBadgeNormal("prod-admin", "us-east-1", "3.11.0", "123456789012", w))
	sb.WriteString("\n\n")

	// Variant 3: Session expiry warning (yellow, 12min)
	sb.WriteString("  Session expiry warning (12m remaining):\n")
	sb.WriteString(renderHeaderBadgeExpiry("prod-admin", "us-east-1", "3.11.0", "acme-prod", "12m left", "warning", w))
	sb.WriteString("\n\n")

	// Variant 4: Session expiry critical (red, 3min)
	sb.WriteString("  Session expiry critical (3m remaining):\n")
	sb.WriteString(renderHeaderBadgeExpiry("prod-admin", "us-east-1", "3.11.0", "acme-prod", "3m left", "critical", w))
	sb.WriteString("\n\n")

	// Variant 5: Session expired
	sb.WriteString("  Session expired:\n")
	sb.WriteString(renderHeaderBadgeExpiry("prod-admin", "us-east-1", "3.11.0", "acme-prod", "EXPIRED", "critical", w))
	sb.WriteString("\n\n")

	// Variant 6: Narrow terminal (80 cols, badge still fits)
	sb.WriteString("  Narrow terminal (80 cols):\n")
	sb.WriteString(renderHeaderBadgeNormal("prod", "us-east-1", "3.11.0", "acme-prod", 80))
	sb.WriteString("\n\n")

	// Variant 7: Very narrow terminal (70 cols, badge omitted)
	sb.WriteString("  Very narrow terminal (70 cols, badge omitted):\n")
	sb.WriteString(renderHeaderNormal("prod", "us-east-1", "3.11.0", 70))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 9: Identity Detail View — Assumed Role ──────────────────────────────

func renderIdentityAssumedRole() string {
	const w = 100

	secStyle := lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	kStyle := lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colDim)

	kw := 22

	kv := func(key, val string) string {
		return "     " + kStyle.Render(padOrTrunc(key, kw)) + vStyle.Render(val)
	}
	sec := func(s string) string {
		return "  " + secStyle.Render(s)
	}

	expiryVal := vStyle.Render("2024-01-15 14:30:00 UTC ") +
		lipgloss.NewStyle().Foreground(colPending).Render("(12m remaining)")

	var lines []string
	lines = append(lines, "")
	lines = append(lines, sec("Account:"))
	lines = append(lines, kv("Alias:", "acme-prod"))
	lines = append(lines, kv("Account ID:", "123456789012"))
	lines = append(lines, "")
	lines = append(lines, sec("Caller:"))
	lines = append(lines, kv("ARN:", "arn:aws:sts::123456789012:assumed-role/admin-role/session"))
	lines = append(lines, kv("Role:", "admin-role"))
	lines = append(lines, kv("Session:", "session"))
	lines = append(lines, "")
	lines = append(lines, sec("Session:"))
	lines = append(lines, "     "+kStyle.Render(padOrTrunc("Expiry:", kw))+expiryVal)
	lines = append(lines, kv("Profile:", "prod-admin"))
	lines = append(lines, kv("Region:", "us-east-1"))
	lines = append(lines, kv("Credential Source:", "SSO"))
	lines = append(lines, "")
	lines = append(lines, "")
	hintLine := hkStyle.Render("c") + dimStyle.Render(" copy ARN  ") +
		hkStyle.Render("esc/any") + dimStyle.Render(" close")
	lines = append(lines, lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top, hintLine))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderBadgeNormal("prod-admin", "us-east-1", "3.11.0", "acme-prod", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Identity", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 9b: Identity Detail View — IAM User ────────────────────────────────

func renderIdentityIAMUser() string {
	const w = 100

	secStyle := lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	kStyle := lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colDim)

	kw := 22

	kv := func(key, val string) string {
		return "     " + kStyle.Render(padOrTrunc(key, kw)) + vStyle.Render(val)
	}
	sec := func(s string) string {
		return "  " + secStyle.Render(s)
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, sec("Account:"))
	lines = append(lines, kv("Alias:", dimStyle.Render("--")))
	lines = append(lines, kv("Account ID:", "123456789012"))
	lines = append(lines, "")
	lines = append(lines, sec("Caller:"))
	lines = append(lines, kv("ARN:", "arn:aws:iam::123456789012:user/deploy-bot"))
	lines = append(lines, kv("User:", "deploy-bot"))
	lines = append(lines, "")
	lines = append(lines, sec("Session:"))
	lines = append(lines, kv("Expiry:", dimStyle.Render("-- (no session token)")))
	lines = append(lines, kv("Profile:", "dev"))
	lines = append(lines, kv("Region:", "us-west-2"))
	lines = append(lines, kv("Credential Source:", "profile"))
	lines = append(lines, "")
	lines = append(lines, "")
	hintLine := hkStyle.Render("c") + dimStyle.Render(" copy ARN  ") +
		hkStyle.Render("esc/any") + dimStyle.Render(" close")
	lines = append(lines, lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top, hintLine))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderBadgeNormal("dev", "us-west-2", "3.11.0", "123456789012", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Identity", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 9c: Identity Detail View — Session Expiring (Critical) ─────────────

func renderIdentityExpiring() string {
	const w = 100

	secStyle := lipgloss.NewStyle().Foreground(colDetailSec).Bold(true)
	kStyle := lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colDim)
	critStyle := lipgloss.NewStyle().Foreground(colError).Bold(true)

	kw := 22

	kv := func(key, val string) string {
		return "     " + kStyle.Render(padOrTrunc(key, kw)) + vStyle.Render(val)
	}
	sec := func(s string) string {
		return "  " + secStyle.Render(s)
	}

	expiryVal := critStyle.Render("2024-01-15 14:30:00 UTC (3m remaining)")

	var lines []string
	lines = append(lines, "")
	lines = append(lines, sec("Account:"))
	lines = append(lines, kv("Alias:", "acme-prod"))
	lines = append(lines, kv("Account ID:", "123456789012"))
	lines = append(lines, "")
	lines = append(lines, sec("Caller:"))
	lines = append(lines, kv("ARN:", "arn:aws:sts::123456789012:assumed-role/admin-role/session"))
	lines = append(lines, kv("Role:", "admin-role"))
	lines = append(lines, kv("Session:", "session"))
	lines = append(lines, "")
	lines = append(lines, sec("Session:"))
	lines = append(lines, "     "+kStyle.Render(padOrTrunc("Expiry:", kw))+expiryVal)
	lines = append(lines, kv("Profile:", "prod-admin"))
	lines = append(lines, kv("Region:", "us-east-1"))
	lines = append(lines, kv("Credential Source:", "SSO"))
	lines = append(lines, "")
	lines = append(lines, "")
	hintLine := hkStyle.Render("c") + dimStyle.Render(" copy ARN  ") +
		hkStyle.Render("esc/any") + dimStyle.Render(" close")
	lines = append(lines, lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top, hintLine))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderBadgeExpiry("prod-admin", "us-east-1", "3.11.0", "acme-prod", "3m left", "critical", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Identity", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 9d: Identity Detail View — Error State ─────────────────────────────

func renderIdentityError() string {
	const w = 100

	kStyle := lipgloss.NewStyle().Foreground(colDetailKey)
	vStyle := lipgloss.NewStyle().Foreground(colDetailVal)
	errStyle := lipgloss.NewStyle().Foreground(colError).Bold(true)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colDim)

	kw := 22

	kv := func(key, val string) string {
		return "  " + kStyle.Render(padOrTrunc(key, kw)) + vStyle.Render(val)
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, "  "+errStyle.Render("Error: Unable to locate credentials (NoCredentialProviders)"))
	lines = append(lines, "")
	lines = append(lines, kv("Profile:", "prod-admin"))
	lines = append(lines, kv("Region:", "us-east-1"))
	lines = append(lines, "")
	lines = append(lines, "")
	hintLine := hkStyle.Render("esc/any") + dimStyle.Render(" close")
	lines = append(lines, lipgloss.Place(w-2, 1, lipgloss.Center, lipgloss.Top, hintLine))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal("prod-admin", "us-east-1", "3.11.0", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Identity", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── VIEW 9e: Identity Detail View — Loading State ───────────────────────────

func renderIdentityLoading() string {
	const w = 100

	spinStyle := lipgloss.NewStyle().Foreground(colAccent)
	msgStyle := lipgloss.NewStyle().Foreground(colDetailVal)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, "")
	loadingText := spinStyle.Render("\u28bf") + msgStyle.Render(" Fetching identity...")
	lines = append(lines, "        "+loadingText)
	lines = append(lines, "")
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderBadgeNormal("prod-admin", "us-east-1", "3.11.0", "", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Identity", w))
	sb.WriteString("\n")

	return sb.String()
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	fmt.Println(divider("VIEW 1: Main Menu"))
	fmt.Println(renderMainMenu())

	fmt.Println(divider("VIEW 2: Resource List — Normal (EC2)"))
	fmt.Println(renderResourceListNormal())

	fmt.Println(divider("VIEW 3: Resource List — Command Mode"))
	fmt.Println(renderResourceListCommand())

	fmt.Println(divider("VIEW 3c: Resource List — Flash Message"))
	fmt.Println(renderResourceListFlash())

	fmt.Println(divider("VIEW 4: Detail View (EC2 instance)"))
	fmt.Println(renderDetailView())

	fmt.Println(divider("VIEW 5: Help Screen (k9s 4-column)"))
	fmt.Println(renderHelpScreen())

	fmt.Println(divider("VIEW 6: YAML View"))
	fmt.Println(renderYAMLView())

	fmt.Println(divider("VIEW 7: Lambda Code — Python Handler"))
	fmt.Println(renderLambdaCodeNormal())

	fmt.Println(divider("VIEW 7b: Lambda Code — Container Image"))
	fmt.Println(renderLambdaCodeContainerImage())

	fmt.Println(divider("VIEW 7c: Lambda Code — Package Too Large"))
	fmt.Println(renderLambdaCodeTooLarge())

	fmt.Println(divider("VIEW 7d: Lambda Code — Help Screen"))
	fmt.Println(renderLambdaCodeHelp())

	fmt.Println(divider("VIEW 7e: EC2 Status Checks — Mixed"))
	fmt.Println(renderEC2StatusChecksMixed())

	fmt.Println(divider("VIEW 7f: EC2 Status Checks — All Healthy"))
	fmt.Println(renderEC2StatusChecksHealthy())

	fmt.Println(divider("VIEW 7g: EC2 Status Checks — Detail (Failed)"))
	fmt.Println(renderEC2StatusChecksDetail())

	fmt.Println(divider("VIEW 8: Header Variants (Account Badge + Session Expiry)"))
	fmt.Println(renderHeaderVariants())

	fmt.Println(divider("VIEW 9: Identity — Assumed Role"))
	fmt.Println(renderIdentityAssumedRole())

	fmt.Println(divider("VIEW 9b: Identity — IAM User"))
	fmt.Println(renderIdentityIAMUser())

	fmt.Println(divider("VIEW 9c: Identity — Session Expiring (Critical)"))
	fmt.Println(renderIdentityExpiring())

	fmt.Println(divider("VIEW 9d: Identity — Error State"))
	fmt.Println(renderIdentityError())

	fmt.Println(divider("VIEW 9e: Identity — Loading"))
	fmt.Println(renderIdentityLoading())
}
