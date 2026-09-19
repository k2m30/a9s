package unit

// Tests for the CloudTrail Events legend in the help view. The legend is
// gated on the "ct-events" short name and the resource-list help context, it
// names the verb glyphs (R/W/D/S/I/N) and the severity tiers, and it carries
// no CELL COLORS block.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// helpWithCTEvents constructs a HelpModel scoped to ct-events.
func helpWithCTEvents(ctx views.HelpContext) views.HelpModel {
	return views.NewHelpWithResource(keys.Default(), ctx, "ct-events")
}

// helpWithOtherResource constructs a HelpModel for a non-ct-events resource.
func helpWithOtherResource(ctx views.HelpContext, shortName string) views.HelpModel {
	return views.NewHelpWithResource(keys.Default(), ctx, shortName)
}

// renderHelp sets a reasonable terminal size and renders the HelpModel.
func renderHelp(m views.HelpModel) string {
	m.SetSize(120, 40)
	return m.View()
}

func TestHelpCTEventsLegend_VisibleFromResourceList(t *testing.T) {
	h := helpWithCTEvents(views.HelpFromResourceList)
	out := renderHelp(h)

	if !strings.Contains(out, "CloudTrail") {
		t.Error("help legend missing 'CloudTrail' section header when context=HelpFromResourceList + ct-events")
	}
}

func TestHelpCTEventsLegend_VisibleFromResourceListPaginated(t *testing.T) {
	h := helpWithCTEvents(views.HelpFromResourceListPaginated)
	out := renderHelp(h)

	if !strings.Contains(out, "CloudTrail") {
		t.Error("help legend missing 'CloudTrail' section header when context=HelpFromResourceListPaginated + ct-events")
	}
}

func TestHelpCTEventsLegend_ContainsAllVerbGlyphs(t *testing.T) {
	h := helpWithCTEvents(views.HelpFromResourceList)
	out := renderHelp(h)

	plain := stripANSI(out)

	for _, verb := range []string{"R", "W", "D", "S", "I", "N"} {
		if !strings.Contains(plain, verb) {
			t.Errorf("help legend missing verb glyph %q in ct-events legend", verb)
		}
	}
}

func TestHelpCTEventsLegend_ContainsSeverityTierLabels(t *testing.T) {
	h := helpWithCTEvents(views.HelpFromResourceList)
	out := renderHelp(h)
	plain := stripANSI(out)

	// The legend must name all three severity tiers.
	for _, want := range []string{"ct-info", "ct-attention", "ct-danger"} {
		if !strings.Contains(plain, want) {
			t.Errorf("help legend missing severity tier %q in ct-events legend", want)
		}
	}
	for _, banned := range []string{"ct-write", "ct-read"} {
		if strings.Contains(plain, banned) {
			t.Errorf("help legend still contains obsolete status name %q; must use ct-info/ct-attention/ct-danger", banned)
		}
	}
}

func TestHelpCTEventsLegend_NoCellColorsSection(t *testing.T) {
	h := helpWithCTEvents(views.HelpFromResourceList)
	out := renderHelp(h)
	plain := stripANSI(out)

	if strings.Contains(plain, "CELL COLORS") {
		t.Error("help legend must NOT contain 'CELL COLORS' section — obsolete block deleted in P3 tear-down")
	}
	// Per-cell color labels that live only in the CELL COLORS block must be
	// absent. "ROOT" may appear in other contexts (ACTOR column description),
	// so the section-header guard above is the specific check; "cross-acct" is
	// unique to that block.
	if strings.Contains(plain, "cross-acct") {
		t.Error("help legend must NOT contain 'cross-acct' — obsolete CELL COLORS entry")
	}
}

func TestHelpCTEventsLegend_HiddenForEC2ResourceList(t *testing.T) {
	h := helpWithOtherResource(views.HelpFromResourceList, "ec2")
	out := renderHelp(h)
	plain := stripANSI(out)

	if strings.Contains(plain, "CloudTrail") {
		t.Error("help legend must NOT show 'CloudTrail' section when resource short name is 'ec2'")
	}
}

func TestHelpCTEventsLegend_HiddenForS3ResourceList(t *testing.T) {
	h := helpWithOtherResource(views.HelpFromResourceList, "s3")
	out := renderHelp(h)
	plain := stripANSI(out)

	if strings.Contains(plain, "CloudTrail") {
		t.Error("help legend must NOT show 'CloudTrail' section when resource short name is 's3'")
	}
}

func TestHelpCTEventsLegend_HiddenForEmptyShortName(t *testing.T) {
	h := helpWithOtherResource(views.HelpFromResourceList, "")
	out := renderHelp(h)
	plain := stripANSI(out)

	if strings.Contains(plain, "CloudTrail") {
		t.Error("help legend must NOT show 'CloudTrail' section when resource short name is empty")
	}
}

func TestHelpCTEventsLegend_AllNonCTResourceTypes(t *testing.T) {
	nonCTTypes := []string{
		"ec2", "s3", "rds", "lambda", "eks", "role", "iam-user",
		"sg", "vpc", "elb", "kms", "secrets", "logs", "alarm",
	}
	for _, shortName := range nonCTTypes {
		h := helpWithOtherResource(views.HelpFromResourceList, shortName)
		out := renderHelp(h)
		plain := stripANSI(out)
		if strings.Contains(plain, "CloudTrail") {
			t.Errorf("help legend shows 'CloudTrail' section for resource short name %q; must only appear for ct-events", shortName)
		}
	}
}

func TestHelpCTEventsLegend_HiddenFromMainMenu(t *testing.T) {
	// Even if the caller passes "ct-events", the legend must not appear
	// from HelpFromMainMenu context (wrong view).
	h := helpWithCTEvents(views.HelpFromMainMenu)
	out := renderHelp(h)
	plain := stripANSI(out)

	if strings.Contains(plain, "CloudTrail") {
		t.Error("help legend must NOT show 'CloudTrail' section when context=HelpFromMainMenu")
	}
}

func TestHelpCTEventsLegend_HiddenFromDetailView(t *testing.T) {
	h := helpWithCTEvents(views.HelpFromDetail)
	out := renderHelp(h)
	plain := stripANSI(out)

	if strings.Contains(plain, "CloudTrail") {
		t.Error("help legend must NOT show 'CloudTrail' section when context=HelpFromDetail")
	}
}

func TestHelpCTEventsLegend_HiddenFromYAMLView(t *testing.T) {
	h := helpWithCTEvents(views.HelpFromYAML)
	out := renderHelp(h)
	plain := stripANSI(out)

	if strings.Contains(plain, "CloudTrail") {
		t.Error("help legend must NOT show 'CloudTrail' section when context=HelpFromYAML")
	}
}

func TestHelpCTEventsLegend_HiddenFromSelectorView(t *testing.T) {
	h := helpWithCTEvents(views.HelpFromSelector)
	out := renderHelp(h)
	plain := stripANSI(out)

	if strings.Contains(plain, "CloudTrail") {
		t.Error("help legend must NOT show 'CloudTrail' section when context=HelpFromSelector")
	}
}

// stripANSI is defined in helpers_test.go (same package).
