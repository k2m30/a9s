package unit

// Tests for the CloudTrail Events legend in the help view (T050Q).
//
// These tests are written BEFORE implementation exists (TDD).
// They will fail to compile until the coder:
//   - Adds a ResourceShortName field (or equivalent) to HelpModel so the
//     legend can be gated on "ct-events"
//   - Adds a NewHelpWithResource(k, ctx, shortName) constructor (or extends
//     NewHelp) so callers can pass the resource short name
//   - Implements the CloudTrail Events legend block in help.go per §8a
//
// Bug vectors covered:
//   - Legend shown on ALL resource lists (not gated on ct-events short name)
//   - Legend shown from main-menu context (wrong context gate)
//   - Legend missing required verb glyphs (R/W/D/S/I/N)
//   - Legend missing severity-tier labels (ct-info / ct-attention / ct-danger)
//   - "CloudTrail" section header absent from legend
//   - Obsolete CELL COLORS section still present (ROOT/OK/FAILED per-cell colors
//     were removed in the P3 tear-down; the block must NOT appear)

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// helpWithCTEvents constructs a HelpModel scoped to ct-events via
// the to-be-implemented NewHelpWithResource constructor.
// Signature expected: views.NewHelpWithResource(keys.Map, views.HelpContext, string) views.HelpModel
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

// ===========================================================================
// Legend visible from ct-events list (HelpFromResourceList)
// ===========================================================================

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

	// Strip ANSI escapes for content checks — we only care about the text, not styling.
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

	// The legend must describe all three severity tiers per §1.1.
	// Must contain all new severity-tier status names.
	for _, want := range []string{"ct-info", "ct-attention", "ct-danger"} {
		if !strings.Contains(plain, want) {
			t.Errorf("help legend missing severity tier %q in ct-events legend", want)
		}
	}
	// Must NOT contain the obsolete binary status names.
	for _, banned := range []string{"ct-write", "ct-read"} {
		if strings.Contains(plain, banned) {
			t.Errorf("help legend still contains obsolete status name %q; must use ct-info/ct-attention/ct-danger", banned)
		}
	}
}

func TestHelpCTEventsLegend_NoCellColorsSection(t *testing.T) {
	// The CELL COLORS block (ROOT actor / OK / FAILED outcome per-cell colors)
	// was removed in the P3 redesign tear-down. The coder deletes lines 485–505
	// from help.go. This test asserts those labels are ABSENT so a future
	// accidental re-addition is caught.
	h := helpWithCTEvents(views.HelpFromResourceList)
	out := renderHelp(h)
	plain := stripANSI(out)

	// "CELL COLORS" section header must not appear.
	if strings.Contains(plain, "CELL COLORS") {
		t.Error("help legend must NOT contain 'CELL COLORS' section — obsolete block deleted in P3 tear-down")
	}
	// Per-cell color labels that lived only in the CELL COLORS block must be absent.
	// NOTE: "ROOT" may appear in other contexts (ACTOR column description), so we
	// specifically check the section-header guard above. "cross-acct" is unique to
	// the deleted block.
	if strings.Contains(plain, "cross-acct") {
		t.Error("help legend must NOT contain 'cross-acct' — obsolete CELL COLORS entry")
	}
}

// ===========================================================================
// Legend hidden from non-ct-events resource lists
// ===========================================================================

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
	// Exhaustive check: a representative set of resource short names must NOT
	// show the CloudTrail legend.
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

// ===========================================================================
// Legend hidden when context is not HelpFromResourceList*
// ===========================================================================

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

// ===========================================================================
// NewHelp (no resource) still works — backwards-compatibility
// ===========================================================================

func TestHelpCTEventsLegend_NewHelpNoResourceCompatible(t *testing.T) {
	// The existing NewHelp constructor (without resource name) must still compile
	// and render without the CloudTrail legend, regardless of context.
	// This guards against the coder breaking the existing API.
	h := views.NewHelp(keys.Default(), views.HelpFromResourceList)
	h.SetSize(120, 40)
	out := h.View()
	plain := stripANSI(out)

	// Without a resource short name, the legend must NOT appear.
	if strings.Contains(plain, "CloudTrail") {
		t.Error("NewHelp (no resource name) must NOT show CloudTrail legend; legend requires explicit ct-events short name")
	}
}

// stripANSI is defined in helpers_test.go (same package).
