// preview-policy-doc renders static mockups of the Policy Document child view.
// Run with: go run ./cmd/preview-policy-doc/
//
// This preview shows all states of the policy-doc view:
//  1. Managed policy with Allow/Deny statements (full syntax highlighting)
//  2. Search active with highlighted matches
//  3. Inline policy with Deny + wildcard Resource "*"
//  4. Help screen
//  5. Loading state
//  6. Error state
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

	// JSON syntax highlighting
	colJSONKey   = lipgloss.Color("#7aa2f7") // same as ColDetailKey / ColYAMLKey
	colJSONStr   = lipgloss.Color("#9ece6a") // same as ColYAMLStr
	colJSONBrace = lipgloss.Color("#565f89") // brackets, braces, commas
	// Additional JSON type colors used in the real implementation but not
	// exercised in this preview (IAM policies are all strings):
	//   colJSONNum  = lipgloss.Color("#ff9e64") // numbers (Condition blocks)
	//   colJSONBool = lipgloss.Color("#bb9af7") // booleans
	//   colJSONNull = lipgloss.Color("#565f89") // null values
	colJSONAllow = lipgloss.Color("#73daca") // "Allow" -- bright green
	colJSONDeny  = lipgloss.Color("#f7768e") // "Deny" -- bright red
	colJSONArn   = lipgloss.Color("#7dcfff") // ARN strings -- cyan
	colJSONWild  = lipgloss.Color("#f7768e") // "*" wildcard -- red

	// Search highlighting
	colSearchMatchBg  = lipgloss.Color("#e0af68") // amber bg for matches
	colSearchMatchFg  = lipgloss.Color("#1a1b26") // dark fg for matches
	colSearchActiveBg = lipgloss.Color("#ff9e64") // orange bg for current match
	colSearchActiveFg = lipgloss.Color("#1a1b26") // dark fg for current match

	// Header metadata
	colMetaKey = lipgloss.Color("#565f89")
	colMetaVal = lipgloss.Color("#c0caf5")

	// Help screen
	colHelpKey = lipgloss.Color("#9ece6a")
	colHelpCat = lipgloss.Color("#e0af68")

	// Status
	colFilter  = lipgloss.Color("#e0af68")
	colSuccess = lipgloss.Color("#9ece6a")
	colError   = lipgloss.Color("#f7768e")
	colSpinner = lipgloss.Color("#7aa2f7")
)

// -- Helpers -------------------------------------------------------------------

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

// -- Header (reuses a9s standard pattern) --------------------------------------

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

func renderHeaderSearch(query string, w int) string {
	right := lipgloss.NewStyle().Foreground(colFilter).Bold(true).Render("/"+query) +
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

// -- Divider between preview sections -----------------------------------------

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

// -- JSON syntax highlighting helpers -----------------------------------------

// Styled renders for JSON tokens
var (
	jsonKey   = lipgloss.NewStyle().Foreground(colJSONKey)
	jsonStr   = lipgloss.NewStyle().Foreground(colJSONStr)
	jsonBrace = lipgloss.NewStyle().Foreground(colJSONBrace)
	jsonAllow = lipgloss.NewStyle().Foreground(colJSONAllow).Bold(true)
	jsonDeny  = lipgloss.NewStyle().Foreground(colJSONDeny).Bold(true)
	jsonArn   = lipgloss.NewStyle().Foreground(colJSONArn)
	jsonWild  = lipgloss.NewStyle().Foreground(colJSONWild).Bold(true)
)

// jk renders a JSON key: "key":
func jk(indent, key string) string {
	return jsonBrace.Render(indent) + jsonKey.Render(`"`+key+`"`) + jsonBrace.Render(": ")
}

// jkOnly renders a JSON key with trailing content (no value on same line)
func jkOnly(indent, key, trail string) string {
	return jsonBrace.Render(indent) + jsonKey.Render(`"`+key+`"`) + jsonBrace.Render(": ") + jsonBrace.Render(trail)
}

// js renders a JSON string value with quotes
func js(val string) string {
	return jsonStr.Render(`"` + val + `"`)
}

// jArn renders an ARN string value in cyan
func jArn(val string) string {
	return jsonArn.Render(`"` + val + `"`)
}

// jComma renders a trailing comma
func jComma(s string) string {
	return s + jsonBrace.Render(",")
}

// -- Metadata header ----------------------------------------------------------

func renderManagedMeta() []string {
	mk := lipgloss.NewStyle().Foreground(colMetaKey)
	mv := lipgloss.NewStyle().Foreground(colMetaVal)
	arnStyle := lipgloss.NewStyle().Foreground(colJSONArn)

	return []string{
		" " + mk.Render("Policy:   ") + mv.Render("AmazonS3ReadOnlyAccess"),
		" " + mk.Render("ARN:      ") + arnStyle.Render("arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"),
		" " + mk.Render("Version:  ") + mv.Render("v1 (default)") + mk.Render(" \u2014 1 version(s)"),
		" " + lipgloss.NewStyle().Foreground(colSep).Render(strings.Repeat("\u2500", 76)),
	}
}

func renderInlineMeta() []string {
	mk := lipgloss.NewStyle().Foreground(colMetaKey)
	mv := lipgloss.NewStyle().Foreground(colMetaVal)

	return []string{
		" " + mk.Render("Policy:   ") + mv.Render("admin-override-policy"),
		" " + mk.Render("Type:     ") + mv.Render("Inline Policy") + mk.Render(" (attached to ") + mv.Render("emergency-access-role") + mk.Render(")"),
		" " + lipgloss.NewStyle().Foreground(colSep).Render(strings.Repeat("\u2500", 76)),
	}
}

// -- VIEW 1: Managed Policy Document (main view) -----------------------------

func renderPolicyDocManaged() string {
	const w = 84

	meta := renderManagedMeta()

	// Build the JSON lines with syntax highlighting
	json := []string{
		" " + jsonBrace.Render("{"),
		"   " + jk("", "Version") + jComma(js("2012-10-17")),
		"   " + jkOnly("", "Statement", "["),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Sid") + jComma(js("AllowS3Read")),
		"       " + jk("", "Effect") + jComma(jsonAllow.Render(`"Allow"`)),
		"       " + jkOnly("", "Action", "["),
		"         " + jComma(js("s3:GetObject")),
		"         " + js("s3:ListBucket"),
		"       " + jsonBrace.Render("],"),
		"       " + jkOnly("", "Resource", "["),
		"         " + jComma(jArn("arn:aws:s3:::my-bucket")),
		"         " + jArn("arn:aws:s3:::my-bucket/*"),
		"       " + jsonBrace.Render("]"),
		"     " + jsonBrace.Render("},"),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Sid") + jComma(js("DenyDeleteBucket")),
		"       " + jk("", "Effect") + jComma(jsonDeny.Render(`"Deny"`)),
		"       " + jk("", "Action") + jComma(js("s3:DeleteBucket")),
		"       " + jk("", "Resource") + jsonWild.Render(`"*"`),
		"     " + jsonBrace.Render("}"),
		"   " + jsonBrace.Render("]"),
		" " + jsonBrace.Render("}"),
	}

	var lines []string
	lines = append(lines, meta...)
	lines = append(lines, json...)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "policy-doc \u2014 AmazonS3ReadOnlyAccess (Managed v1)", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- VIEW 2: Search active with highlighted match ----------------------------

func renderPolicyDocSearch() string {
	const w = 84

	meta := renderManagedMeta()

	// Search highlighting styles
	matchBg := lipgloss.NewStyle().
		Background(colSearchActiveBg).Foreground(colSearchActiveFg).Bold(true)
	otherMatchBg := lipgloss.NewStyle().
		Background(colSearchMatchBg).Foreground(colSearchMatchFg)

	// Build the JSON with search matches highlighted
	// Search term: "s3:GetObject" -- appears once in Action array, once if anywhere else
	json := []string{
		" " + jsonBrace.Render("{"),
		"   " + jk("", "Version") + jComma(js("2012-10-17")),
		"   " + jkOnly("", "Statement", "["),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Sid") + jComma(js("AllowS3Read")),
		"       " + jk("", "Effect") + jComma(jsonAllow.Render(`"Allow"`)),
		"       " + jkOnly("", "Action", "["),
		// This line has the current match: "s3:GetObject" highlighted
		"         " + jsonStr.Render(`"`) + matchBg.Render("s3:GetObject") + jsonStr.Render(`"`) + jsonBrace.Render(","),
		"         " + js("s3:ListBucket"),
		"       " + jsonBrace.Render("],"),
		"       " + jkOnly("", "Resource", "["),
		"         " + jComma(jArn("arn:aws:s3:::my-bucket")),
		"         " + jArn("arn:aws:s3:::my-bucket/*"),
		"       " + jsonBrace.Render("]"),
		"     " + jsonBrace.Render("},"),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Sid") + jComma(js("DenyDeleteBucket")),
		"       " + jk("", "Effect") + jComma(jsonDeny.Render(`"Deny"`)),
		"       " + jk("", "Action") + jComma(js("s3:DeleteBucket")),
		"       " + jk("", "Resource") + jsonWild.Render(`"*"`),
		"     " + jsonBrace.Render("}"),
		"   " + jsonBrace.Render("]"),
		" " + jsonBrace.Render("}"),
		"",
		" " + lipgloss.NewStyle().Foreground(colDim).Render("[1/1 matches]"),
	}
	_ = otherMatchBg // would be used for non-current matches in multi-match scenarios

	var lines []string
	lines = append(lines, meta...)
	lines = append(lines, json...)

	var sb strings.Builder
	sb.WriteString(renderHeaderSearch("s3:GetObject", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "policy-doc \u2014 AmazonS3ReadOnlyAccess (Managed v1)", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- VIEW 3: Inline policy with Deny + wildcard (danger view) ----------------

func renderPolicyDocInlineDeny() string {
	const w = 84

	meta := renderInlineMeta()

	json := []string{
		" " + jsonBrace.Render("{"),
		"   " + jk("", "Version") + jComma(js("2012-10-17")),
		"   " + jkOnly("", "Statement", "["),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Sid") + jComma(js("DenyAllS3Deletes")),
		"       " + jk("", "Effect") + jComma(jsonDeny.Render(`"Deny"`)),
		"       " + jkOnly("", "Action", "["),
		"         " + jComma(js("s3:DeleteObject")),
		"         " + js("s3:DeleteBucket"),
		"       " + jsonBrace.Render("],"),
		"       " + jk("", "Resource") + jsonWild.Render(`"*"`),
		"     " + jsonBrace.Render("},"),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Effect") + jComma(jsonAllow.Render(`"Allow"`)),
		"       " + jk("", "Action") + jComma(js("s3:GetObject")),
		"       " + jk("", "Resource") + jArn("arn:aws:s3:::logs-bucket/*"),
		"     " + jsonBrace.Render("}"),
		"   " + jsonBrace.Render("]"),
		" " + jsonBrace.Render("}"),
	}

	var lines []string
	lines = append(lines, meta...)
	lines = append(lines, json...)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "policy-doc \u2014 admin-override-policy (Inline)", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- VIEW 4: Multi-statement policy with multiple matches --------------------

func renderPolicyDocMultiMatch() string {
	const w = 84

	meta := renderManagedMeta()

	matchBg := lipgloss.NewStyle().
		Background(colSearchMatchBg).Foreground(colSearchMatchFg)
	activeBg := lipgloss.NewStyle().
		Background(colSearchActiveBg).Foreground(colSearchActiveFg).Bold(true)

	// Search term: "s3:" -- appears in multiple places
	json := []string{
		" " + jsonBrace.Render("{"),
		"   " + jk("", "Version") + jComma(js("2012-10-17")),
		"   " + jkOnly("", "Statement", "["),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Sid") + jComma(js("AllowS3Read")),
		"       " + jk("", "Effect") + jComma(jsonAllow.Render(`"Allow"`)),
		"       " + jkOnly("", "Action", "["),
		// match 1 (non-current): s3: highlighted in amber
		"         " + jsonStr.Render(`"`) + matchBg.Render("s3:") + jsonStr.Render(`GetObject"`) + jsonBrace.Render(","),
		// match 2 (current): s3: highlighted in orange
		"         " + jsonStr.Render(`"`) + activeBg.Render("s3:") + jsonStr.Render(`ListBucket"`),
		"       " + jsonBrace.Render("],"),
		"       " + jkOnly("", "Resource", "["),
		"         " + jComma(jArn("arn:aws:s3:::my-bucket")),
		"         " + jArn("arn:aws:s3:::my-bucket/*"),
		"       " + jsonBrace.Render("]"),
		"     " + jsonBrace.Render("},"),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Effect") + jComma(jsonDeny.Render(`"Deny"`)),
		// match 3 (non-current): s3: highlighted in amber
		"       " + jk("", "Action") + jComma(jsonStr.Render(`"`)+matchBg.Render("s3:")+jsonStr.Render(`DeleteBucket"`)),
		"       " + jk("", "Resource") + jsonWild.Render(`"*"`),
		"     " + jsonBrace.Render("}"),
		"   " + jsonBrace.Render("]"),
		" " + jsonBrace.Render("}"),
		"",
		" " + lipgloss.NewStyle().Foreground(colDim).Render("[2/3 matches]"),
	}

	var lines []string
	lines = append(lines, meta...)
	lines = append(lines, json...)

	var sb strings.Builder
	sb.WriteString(renderHeaderSearch("s3:", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "policy-doc \u2014 AmazonS3ReadOnlyAccess (Managed v1)", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- VIEW 5: Help screen -----------------------------------------------------

func renderPolicyDocHelp() string {
	const w = 84

	catStyle := lipgloss.NewStyle().Foreground(colHelpCat).Bold(true)
	hkStyle := lipgloss.NewStyle().Foreground(colHelpKey).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colHeaderFg)

	bind := func(k, d string) string {
		return hkStyle.Render(padOrTrunc(k, 9)) + descStyle.Render(d)
	}

	colW := 20
	catRow := catStyle.Render(padOrTrunc("POLICY DOCUMENT", colW)) +
		catStyle.Render(padOrTrunc("GENERAL", colW)) +
		catStyle.Render(padOrTrunc("NAVIGATION", colW)) +
		catStyle.Render("HOTKEYS")

	type bindRow struct {
		c1, c2, c3, c4 string
	}
	bindRows := []bindRow{
		{bind("<esc>", "Back  "), bind("<ctrl-r>", "Refresh  "), bind("<j>", "Down      "), bind("<?>", "Help")},
		{bind("<c>", "Copy  "), bind("<q>", "Quit     "), bind("<k>", "Up        "), bind("<:>", "Command")},
		{bind("<w>", "Wrap  "), "", bind("<g>", "Top       "), ""},
		{bind("</>", "Search"), "", bind("<G>", "Bottom    "), ""},
		{bind("<n>", "Next  "), "", bind("<pgup>", "Page Up   "), ""},
		{bind("<N>", "Prev  "), "", bind("<pgdn>", "Page Down "), ""},
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
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "Help", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- VIEW 6: Loading state ----------------------------------------------------

func renderPolicyDocLoading() string {
	const w = 84

	var lines []string
	lines = append(lines, "")
	lines = append(lines,
		"       "+lipgloss.NewStyle().Foreground(colSpinner).Render("\u28ff")+" "+
			lipgloss.NewStyle().Foreground(colHeaderFg).Render("Fetching policy document..."))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "policy-doc \u2014 AmazonS3ReadOnlyAccess (Managed v1)", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- VIEW 7: Error state ------------------------------------------------------

func renderPolicyDocError() string {
	const w = 84

	errStyle := lipgloss.NewStyle().Foreground(colError).Bold(true)

	var lines []string
	lines = append(lines, "")
	lines = append(lines,
		" "+errStyle.Render("Error: NoSuchEntity \u2014 Policy arn:aws:iam::123456789012:policy/deleted-policy"))
	lines = append(lines,
		" "+errStyle.Render("was not found."))
	lines = append(lines, "")
	lines = append(lines,
		" "+lipgloss.NewStyle().Foreground(colDim).Render("Press Esc to go back."))
	lines = append(lines, "")

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "policy-doc \u2014 deleted-policy (Managed v1)", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- VIEW 8: Larger realistic policy (AdministratorAccess-like) ---------------

func renderPolicyDocLarge() string {
	const w = 96

	mk := lipgloss.NewStyle().Foreground(colMetaKey)
	mv := lipgloss.NewStyle().Foreground(colMetaVal)
	arnStyle := lipgloss.NewStyle().Foreground(colJSONArn)
	warnStyle := lipgloss.NewStyle().Foreground(colError).Bold(true)

	meta := []string{
		" " + mk.Render("Policy:   ") + warnStyle.Render("AdministratorAccess"),
		" " + mk.Render("ARN:      ") + arnStyle.Render("arn:aws:iam::aws:policy/AdministratorAccess"),
		" " + mk.Render("Version:  ") + mv.Render("v1 (default)") + mk.Render(" \u2014 1 version(s)"),
		" " + lipgloss.NewStyle().Foreground(colSep).Render(strings.Repeat("\u2500", 88)),
	}

	// AdministratorAccess is the simplest and most dangerous policy
	json := []string{
		" " + jsonBrace.Render("{"),
		"   " + jk("", "Version") + jComma(js("2012-10-17")),
		"   " + jkOnly("", "Statement", "["),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Effect") + jComma(jsonAllow.Render(`"Allow"`)),
		"       " + jk("", "Action") + jComma(jsonWild.Render(`"*"`)),
		"       " + jk("", "Resource") + jsonWild.Render(`"*"`),
		"     " + jsonBrace.Render("}"),
		"   " + jsonBrace.Render("]"),
		" " + jsonBrace.Render("}"),
	}

	var lines []string
	lines = append(lines, meta...)
	lines = append(lines, json...)

	var sb strings.Builder
	sb.WriteString(renderHeaderNormal(w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "policy-doc \u2014 AdministratorAccess (Managed v1)", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- VIEW 9: Flash "Copied!" after pressing c --------------------------------

func renderPolicyDocCopied() string {
	const w = 84

	meta := renderManagedMeta()

	json := []string{
		" " + jsonBrace.Render("{"),
		"   " + jk("", "Version") + jComma(js("2012-10-17")),
		"   " + jkOnly("", "Statement", "["),
		"     " + jsonBrace.Render("{"),
		"       " + jk("", "Effect") + jComma(jsonAllow.Render(`"Allow"`)),
		"       " + jkOnly("", "Action", "["),
		"         " + jComma(js("s3:GetObject")),
		"         " + js("s3:ListBucket"),
		"       " + jsonBrace.Render("],"),
		"       " + jkOnly("", "Resource", "["),
		"         " + jComma(jArn("arn:aws:s3:::my-bucket")),
		"         " + jArn("arn:aws:s3:::my-bucket/*"),
		"       " + jsonBrace.Render("]"),
		"     " + jsonBrace.Render("}"),
		"   " + jsonBrace.Render("]"),
		" " + jsonBrace.Render("}"),
	}

	var lines []string
	lines = append(lines, meta...)
	lines = append(lines, json...)

	var sb strings.Builder
	sb.WriteString(renderHeaderFlash("Copied!", w))
	sb.WriteString("\n")
	sb.WriteString(renderFramedBox(lines, "policy-doc \u2014 AmazonS3ReadOnlyAccess (Managed v1)", w))
	sb.WriteString("\n")

	return sb.String()
}

// -- Main ---------------------------------------------------------------------

func main() {
	fmt.Println(divider("VIEW 1: Managed Policy Document (Allow + Deny)"))
	fmt.Println(renderPolicyDocManaged())

	fmt.Println(divider("VIEW 2: Search Active (/s3:GetObject)"))
	fmt.Println(renderPolicyDocSearch())

	fmt.Println(divider("VIEW 3: Inline Policy with Deny + Wildcard"))
	fmt.Println(renderPolicyDocInlineDeny())

	fmt.Println(divider("VIEW 4: Multi-Match Search (/s3:)"))
	fmt.Println(renderPolicyDocMultiMatch())

	fmt.Println(divider("VIEW 5: Help Screen"))
	fmt.Println(renderPolicyDocHelp())

	fmt.Println(divider("VIEW 6: Loading State"))
	fmt.Println(renderPolicyDocLoading())

	fmt.Println(divider("VIEW 7: Error State"))
	fmt.Println(renderPolicyDocError())

	fmt.Println(divider("VIEW 8: AdministratorAccess (Danger Policy)"))
	fmt.Println(renderPolicyDocLarge())

	fmt.Println(divider("VIEW 9: Copy Flash (Copied!)"))
	fmt.Println(renderPolicyDocCopied())
}
