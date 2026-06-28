// text_render_parity_test.go — byte-parity gate for the YAML/JSON text flip.
//
// Asserts that YAMLModel.RenderText(body) and JSONModel.RenderText(body)
// produce output byte-identical to their respective View() for the same
// logical state, across a comprehensive set of scenarios.
//
// Strategy:
//   - Both sides share the SAME model instance (m) so viewport geometry
//     (width/height/scroll) is identical — RenderText reads width/height
//     from the model's viewport set by SetSize.
//   - Legacy side: drive m via Update(KeyPressMsg) to activate wrap/search/
//     scroll, then capture legacy := m.View().
//   - Controller side: construct a TextBody whose Lines are the same
//     syntax-colored content lines that m.renderContent() (called by SetSize)
//     places in the viewport. Scroll, wrap, and search come from the state
//     driven above. Call got := m.RenderText(body).
//   - Assert got == legacy EXACTLY (byte-parity). Any difference is a bug in
//     RenderText and must be reported, not suppressed.
//
// Drive path: direct TextBody construction (no controller stack needed).
// The controller's EnsureTextState / buildTextBody path requires a YAML/JSON
// screen on the stack, which is not wired end-to-end yet. Instead, this test
// constructs TextBody directly from the Lines that the model produces —
// matching exactly what buildTextBody(ts) returns for an equivalent TextState.
// This is the same approach the selector parity test uses for its body.
package unit_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Fixtures — realistic resource data used by both YAML and JSON tests.
// ---------------------------------------------------------------------------

// textParityResource returns a realistic resource with multiple fields and
// a RawStruct so that marshal output has a predictable number of lines.
// Using a small fixed struct keeps the test self-contained (no demo imports).
func textParityResource() resource.Resource {
	type innerBlock struct {
		Protocol string `yaml:"protocol" json:"protocol"`
		Port     int    `yaml:"port"     json:"port"`
	}
	type rawData struct {
		InstanceID   string     `yaml:"instance_id"   json:"instance_id"`
		InstanceType string     `yaml:"instance_type" json:"instance_type"`
		State        string     `yaml:"state"         json:"state"`
		LaunchTime   string     `yaml:"launch_time"   json:"launch_time"`
		PublicIP     string     `yaml:"public_ip"     json:"public_ip"`
		PrivateIP    string     `yaml:"private_ip"    json:"private_ip"`
		VPCID        string     `yaml:"vpc_id"        json:"vpc_id"`
		SubnetID     string     `yaml:"subnet_id"     json:"subnet_id"`
		Monitoring   bool       `yaml:"monitoring"    json:"monitoring"`
		Tags         []string   `yaml:"tags"          json:"tags"`
		Network      innerBlock `yaml:"network"       json:"network"`
	}
	raw := rawData{
		InstanceID:   "i-0abc123def456gh78",
		InstanceType: "t3.medium",
		State:        "running",
		LaunchTime:   "2024-01-15T10:30:00Z",
		PublicIP:     "203.0.113.42",
		PrivateIP:    "10.0.1.100",
		VPCID:        "vpc-0abc12345def67890",
		SubnetID:     "subnet-0abc12345def67890",
		Monitoring:   true,
		Tags:         []string{"prod", "backend", "eu-west-1"},
		Network:      innerBlock{Protocol: "tcp", Port: 443},
	}
	return resource.Resource{
		ID:        "i-0abc123def456gh78",
		Name:      "prod-backend-01",
		RawStruct: raw,
		Fields: map[string]string{
			"instance_id":   "i-0abc123def456gh78",
			"instance_type": "t3.medium",
			"state":         "running",
		},
	}
}

// textParityResourceEmpty returns a resource with no fields or raw struct
// to exercise the "no data available" branch.
func textParityResourceEmpty() resource.Resource {
	return resource.Resource{
		ID:   "i-empty",
		Name: "",
	}
}

// textParityResourceShort returns a resource with minimal fields
// producing only a few lines of output.
func textParityResourceShort() resource.Resource {
	return resource.Resource{
		ID: "sg-0abc123",
		Fields: map[string]string{
			"group_id":    "sg-0abc123",
			"description": "default",
		},
	}
}

// textParityResourceLong returns a resource with many fields
// to force viewport scrolling.
func textParityResourceLong() resource.Resource {
	fields := map[string]string{}
	for i := 0; i < 60; i++ {
		fields[fmt.Sprintf("field_%02d", i)] = fmt.Sprintf("value-%d", i)
	}
	return resource.Resource{
		ID:     "arn:aws:iam::123456789012:role/LongRole",
		Name:   "LongRole",
		Fields: fields,
	}
}

// ---------------------------------------------------------------------------
// Content helpers — produce the syntax-colored content lines that both
// YAMLModel and JSONModel place in their viewports via refreshViewportContent.
// ---------------------------------------------------------------------------

// yamlContentLinesAt returns the syntax-colored YAML lines for a resource at
// the given display width.  A very large height ensures all content lines are
// visible (none clipped).  Trailing spaces — added by the viewport renderer
// as per-line padding — are stripped so that strings.Join in RenderText
// reconstructs the same raw content string that was originally passed to
// viewport.SetContent, making both sides of the parity assertion identical.
// Returns nil when the model renders "No YAML data available".
func yamlContentLinesAt(res resource.Resource, width int) []string {
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(width, 9999)
	rawLines := strings.Split(m.View(), "\n")
	lines := make([]string, len(rawLines))
	for i, l := range rawLines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// yamlContentLines returns syntax-colored YAML lines at width 80.
func yamlContentLines(res resource.Resource) []string {
	return yamlContentLinesAt(res, 80)
}

// jsonContentLinesAt returns the syntax-colored JSON lines for a resource at
// the given display width.  See yamlContentLinesAt for rationale.
func jsonContentLinesAt(res resource.Resource, width int) []string {
	m := views.NewJSON(res, "ec2", keys.Default())
	m.SetSize(width, 9999)
	rawLines := strings.Split(m.View(), "\n")
	lines := make([]string, len(rawLines))
	for i, l := range rawLines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// jsonContentLines returns syntax-colored JSON lines at width 80.
func jsonContentLines(res resource.Resource) []string {
	return jsonContentLinesAt(res, 80)
}

// ---------------------------------------------------------------------------
// TextBody construction — mirrors buildTextBody(ts) from internal/app/text.go.
// ---------------------------------------------------------------------------

// textBodyFromLines constructs a TextBody for the given lines with default
// (no wrap, no search, scroll=0) state — equivalent to buildTextBody on a
// fresh TextState initialized with those lines.
func textBodyFromLines(lines []string) app.TextBody {
	return app.TextBody{
		Lines:   lines,
		Wrap:    false,
		ScrollY: 0,
	}
}

// textBodyWithWrap returns a TextBody with wrap enabled.
func textBodyWithWrap(lines []string) app.TextBody {
	return app.TextBody{
		Lines: lines,
		Wrap:  true,
	}
}

// textBodyWithSearch returns a TextBody with an active search query and the
// given cursor. SearchMatches is not required by RenderText (it recomputes).
func textBodyWithSearch(lines []string, query string, cursor int) app.TextBody {
	return app.TextBody{
		Lines:        lines,
		Search:       query,
		SearchCursor: cursor,
	}
}

// textBodyWithScrollY returns a TextBody scrolled to the given Y offset.
func textBodyWithScrollY(lines []string, scrollY int) app.TextBody {
	return app.TextBody{
		Lines:   lines,
		ScrollY: scrollY,
	}
}

// ---------------------------------------------------------------------------
// Assertion helper
// ---------------------------------------------------------------------------

// assertTextParity calls m.View() and m.RenderText(body) on a YAMLModel and
// asserts byte-identical output, reporting a line-by-line diff on mismatch.
func assertTextParityYAML(t *testing.T, m *views.YAMLModel, body app.TextBody, kind, scenario string) {
	t.Helper()
	legacy := m.View()
	got := m.RenderText(body)
	if got == legacy {
		return
	}
	reportTextParityMismatch(t, legacy, got, kind, scenario)
}

// assertTextParityJSON calls m.View() and m.RenderText(body) on a JSONModel and
// asserts byte-identical output, reporting a line-by-line diff on mismatch.
func assertTextParityJSON(t *testing.T, m *views.JSONModel, body app.TextBody, kind, scenario string) {
	t.Helper()
	legacy := m.View()
	got := m.RenderText(body)
	if got == legacy {
		return
	}
	reportTextParityMismatch(t, legacy, got, kind, scenario)
}

// reportTextParityMismatch emits a structured line-by-line diff failure message.
func reportTextParityMismatch(t *testing.T, legacy, got, kind, scenario string) {
	t.Helper()
	legacyLines := strings.Split(legacy, "\n")
	gotLines := strings.Split(got, "\n")
	maxLines := len(legacyLines)
	if len(gotLines) > maxLines {
		maxLines = len(gotLines)
	}
	var diff strings.Builder
	diff.WriteString(fmt.Sprintf(
		"kind=%s scenario=%s — RenderText differs from View():\n  legacy lines=%d  RenderText lines=%d\n",
		kind, scenario, len(legacyLines), len(gotLines),
	))
	for i := 0; i < maxLines; i++ {
		legLine, gotLine := "", ""
		if i < len(legacyLines) {
			legLine = legacyLines[i]
		}
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if legLine != gotLine {
			diff.WriteString(fmt.Sprintf(
				"  line %d:\n    legacy:     %q\n    RenderText: %q\n",
				i+1, legLine, gotLine,
			))
		}
	}
	t.Errorf("byte-parity FAILED:\n%s", diff.String())
}

// ---------------------------------------------------------------------------
// YAML parity scenarios
// ---------------------------------------------------------------------------

// TestTextRenderParity_YAML covers all YAML scenarios.
func TestTextRenderParity_YAML(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textParityResource()
	lines := yamlContentLines(res)

	// S1: Default — no wrap, no search, scroll=0, standard terminal.
	t.Run("S1_Default", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		body := textBodyFromLines(lines)
		assertTextParityYAML(t, &m, body, "yaml", "S1_Default")
	})

	// S2: Wrap ON — toggle via Update (the "w" key matches keys.ToggleWrap).
	t.Run("S2_WrapOn", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "w"})

		body := textBodyWithWrap(lines)
		assertTextParityYAML(t, &m, body, "yaml", "S2_WrapOn")
	})

	// S3: Search active with matches — query "instance" which appears in the resource.
	t.Run("S3_SearchWithMatches", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		// Activate search and type a query.
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		for _, ch := range "instance" {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: string(ch)})
		}
		// Confirm search (Enter).
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		body := textBodyWithSearch(lines, "instance", 0)
		assertTextParityYAML(t, &m, body, "yaml", "S3_SearchWithMatches")
	})

	// S4: Search with no matches — query has no occurrences in content.
	t.Run("S4_SearchNoMatch", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		for _, ch := range "zzznomatch" {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: string(ch)})
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		body := textBodyWithSearch(lines, "zzznomatch", 0)
		assertTextParityYAML(t, &m, body, "yaml", "S4_SearchNoMatch")
	})

	// S5: Scrolled — scroll down 3 lines.
	// Height=5 < content line count (~12) so the viewport overflows and j scrolls.
	t.Run("S5_Scrolled", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(80, 5)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		body := textBodyWithScrollY(lines, 3)
		assertTextParityYAML(t, &m, body, "yaml", "S5_Scrolled")
	})

	// S6: Narrow width (40).
	t.Run("S6_NarrowWidth40", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(40, 24)
		body := textBodyFromLines(yamlContentLinesAt(res, 40))
		assertTextParityYAML(t, &m, body, "yaml", "S6_NarrowWidth40")
	})

	// S7: Wide width (200).
	t.Run("S7_WideWidth200", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(200, 24)
		body := textBodyFromLines(yamlContentLinesAt(res, 200))
		assertTextParityYAML(t, &m, body, "yaml", "S7_WideWidth200")
	})

	// S8: Empty/no-data resource — model renders "No YAML data available".
	t.Run("S8_EmptyResource", func(t *testing.T) {
		empty := textParityResourceEmpty()
		m := views.NewYAML(empty, "ec2", keys.Default())
		m.SetSize(80, 24)
		// For empty resources, RenderText receives nil/empty lines.
		// The legacy View() renders the viewport content (dim text).
		// Build body with the lines from the viewport (which holds the dim text line).
		emptyLines := yamlContentLines(empty)
		// emptyLines may be nil (no data); RenderText handles nil Lines as empty string.
		body := textBodyFromLines(emptyLines)
		assertTextParityYAML(t, &m, body, "yaml", "S8_EmptyResource")
	})

	// S9: Long document — forces viewport to only show a portion.
	t.Run("S9_LongDocument", func(t *testing.T) {
		long := textParityResourceLong()
		longLines := yamlContentLines(long)
		m := views.NewYAML(long, "ec2", keys.Default())
		m.SetSize(80, 10)
		body := textBodyFromLines(longLines)
		assertTextParityYAML(t, &m, body, "yaml", "S9_LongDocument")
	})

	// S10: Long document scrolled to mid-document.
	t.Run("S10_LongDocumentScrolled", func(t *testing.T) {
		long := textParityResourceLong()
		longLines := yamlContentLines(long)
		m := views.NewYAML(long, "ec2", keys.Default())
		m.SetSize(80, 10)
		// Scroll down 15 lines.
		for i := 0; i < 15; i++ {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		}
		body := textBodyWithScrollY(longLines, 15)
		assertTextParityYAML(t, &m, body, "yaml", "S10_LongDocumentScrolled")
	})

	// S11: Short document (few lines).
	t.Run("S11_ShortDocument", func(t *testing.T) {
		short := textParityResourceShort()
		shortLines := yamlContentLines(short)
		m := views.NewYAML(short, "ec2", keys.Default())
		m.SetSize(80, 24)
		body := textBodyFromLines(shortLines)
		assertTextParityYAML(t, &m, body, "yaml", "S11_ShortDocument")
	})

	// S12: Small viewport height (5).
	t.Run("S12_SmallViewportHeight5", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(80, 5)
		body := textBodyFromLines(lines)
		assertTextParityYAML(t, &m, body, "yaml", "S12_SmallViewportHeight5")
	})

	// S13: Search with matches, cursor advanced to second match.
	t.Run("S13_SearchCursorAdvanced", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		// Activate search, type "e" which should produce multiple matches.
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "e"})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		// Advance to next match (n = SearchNext key).
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "n"})

		// Body: search="e", cursor=1 (second match, 0-indexed).
		body := textBodyWithSearch(lines, "e", 1)
		assertTextParityYAML(t, &m, body, "yaml", "S13_SearchCursorAdvanced")
	})

	// S14: Wrap ON + search active.
	t.Run("S14_WrapPlusSearch", func(t *testing.T) {
		m := views.NewYAML(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		// Toggle wrap.
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "w"})
		// Activate search.
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		for _, ch := range "running" {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: string(ch)})
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		body := app.TextBody{
			Lines:        lines,
			Wrap:         true,
			Search:       "running",
			SearchCursor: 0,
		}
		assertTextParityYAML(t, &m, body, "yaml", "S14_WrapPlusSearch")
	})
}

// ---------------------------------------------------------------------------
// JSON parity scenarios
// ---------------------------------------------------------------------------

// TestTextRenderParity_JSON covers all JSON scenarios.
func TestTextRenderParity_JSON(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	res := textParityResource()
	lines := jsonContentLines(res)

	// S1: Default — no wrap, no search, scroll=0, standard terminal.
	t.Run("S1_Default", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		body := textBodyFromLines(lines)
		assertTextParityJSON(t, &m, body, "json", "S1_Default")
	})

	// S2: Wrap ON.
	t.Run("S2_WrapOn", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "w"})

		body := textBodyWithWrap(lines)
		assertTextParityJSON(t, &m, body, "json", "S2_WrapOn")
	})

	// S3: Search active with matches — "instance_id" appears in JSON keys.
	t.Run("S3_SearchWithMatches", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		for _, ch := range "instance" {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: string(ch)})
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		body := textBodyWithSearch(lines, "instance", 0)
		assertTextParityJSON(t, &m, body, "json", "S3_SearchWithMatches")
	})

	// S4: Search with no matches.
	t.Run("S4_SearchNoMatch", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		for _, ch := range "zzznomatch" {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: string(ch)})
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		body := textBodyWithSearch(lines, "zzznomatch", 0)
		assertTextParityJSON(t, &m, body, "json", "S4_SearchNoMatch")
	})

	// S5: Scrolled — scroll down 3 lines.
	// Height=5 < JSON content line count (~22) so the viewport overflows and j scrolls.
	t.Run("S5_Scrolled", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(80, 5)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		body := textBodyWithScrollY(lines, 3)
		assertTextParityJSON(t, &m, body, "json", "S5_Scrolled")
	})

	// S6: Narrow width (40).
	t.Run("S6_NarrowWidth40", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(40, 24)
		body := textBodyFromLines(jsonContentLinesAt(res, 40))
		assertTextParityJSON(t, &m, body, "json", "S6_NarrowWidth40")
	})

	// S7: Wide width (200).
	t.Run("S7_WideWidth200", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(200, 24)
		body := textBodyFromLines(jsonContentLinesAt(res, 200))
		assertTextParityJSON(t, &m, body, "json", "S7_WideWidth200")
	})

	// S8: Empty/no-data resource.
	t.Run("S8_EmptyResource", func(t *testing.T) {
		empty := textParityResourceEmpty()
		m := views.NewJSON(empty, "ec2", keys.Default())
		m.SetSize(80, 24)
		emptyLines := jsonContentLines(empty)
		body := textBodyFromLines(emptyLines)
		assertTextParityJSON(t, &m, body, "json", "S8_EmptyResource")
	})

	// S9: Long document (60 fields).
	t.Run("S9_LongDocument", func(t *testing.T) {
		long := textParityResourceLong()
		longLines := jsonContentLines(long)
		m := views.NewJSON(long, "ec2", keys.Default())
		m.SetSize(80, 10)
		body := textBodyFromLines(longLines)
		assertTextParityJSON(t, &m, body, "json", "S9_LongDocument")
	})

	// S10: Long document scrolled.
	t.Run("S10_LongDocumentScrolled", func(t *testing.T) {
		long := textParityResourceLong()
		longLines := jsonContentLines(long)
		m := views.NewJSON(long, "ec2", keys.Default())
		m.SetSize(80, 10)
		for i := 0; i < 15; i++ {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		}
		body := textBodyWithScrollY(longLines, 15)
		assertTextParityJSON(t, &m, body, "json", "S10_LongDocumentScrolled")
	})

	// S11: Short document.
	t.Run("S11_ShortDocument", func(t *testing.T) {
		short := textParityResourceShort()
		shortLines := jsonContentLines(short)
		m := views.NewJSON(short, "ec2", keys.Default())
		m.SetSize(80, 24)
		body := textBodyFromLines(shortLines)
		assertTextParityJSON(t, &m, body, "json", "S11_ShortDocument")
	})

	// S12: Small viewport height (5).
	t.Run("S12_SmallViewportHeight5", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(80, 5)
		body := textBodyFromLines(lines)
		assertTextParityJSON(t, &m, body, "json", "S12_SmallViewportHeight5")
	})

	// S13: Search cursor advanced to second match.
	t.Run("S13_SearchCursorAdvanced", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "e"})
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "n"})

		body := textBodyWithSearch(lines, "e", 1)
		assertTextParityJSON(t, &m, body, "json", "S13_SearchCursorAdvanced")
	})

	// S14: Wrap ON + search active.
	t.Run("S14_WrapPlusSearch", func(t *testing.T) {
		m := views.NewJSON(res, "ec2", keys.Default())
		m.SetSize(80, 24)
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "w"})
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		for _, ch := range "running" {
			m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: string(ch)})
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		body := app.TextBody{
			Lines:        lines,
			Wrap:         true,
			Search:       "running",
			SearchCursor: 0,
		}
		assertTextParityJSON(t, &m, body, "json", "S14_WrapPlusSearch")
	})
}
