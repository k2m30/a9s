package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ═══════════════════════════════════════════════════════════════════════════
// Help view tests — issue #247: CloudTrail "t" key appears in help
// ═══════════════════════════════════════════════════════════════════════════

// TestHelp_ResourceList_ShowsCloudTrailKey verifies that the ResourceList
// help view renders the "t" key with "cloudtrail" description.
func TestHelp_ResourceList_ShowsCloudTrailKey(t *testing.T) {
	m := views.NewHelp(keys.Default(), views.HelpFromResourceList)
	m.SetSize(120, 40)
	output := m.View()

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "cloudtrail") {
		t.Errorf("Help (ResourceList) View() does not contain 'cloudtrail'; got:\n%s", output)
	}
	if !strings.Contains(output, "t") {
		t.Errorf("Help (ResourceList) View() does not contain 't'")
	}
}

// TestHelp_Detail_ShowsCloudTrailKey verifies that the Detail help view
// renders the "t" key with "cloudtrail" description.
func TestHelp_Detail_ShowsCloudTrailKey(t *testing.T) {
	m := views.NewHelp(keys.Default(), views.HelpFromDetail)
	m.SetSize(120, 40)
	output := m.View()

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "cloudtrail") {
		t.Errorf("Help (Detail) View() does not contain 'cloudtrail'; got:\n%s", output)
	}
	if !strings.Contains(output, "t") {
		t.Errorf("Help (Detail) View() does not contain 't'")
	}
}

// TestHelp_YAML_ShowsCloudTrailKey verifies that the YAML help view renders
// the "t" key with "cloudtrail" description.
func TestHelp_YAML_ShowsCloudTrailKey(t *testing.T) {
	m := views.NewHelp(keys.Default(), views.HelpFromYAML)
	m.SetSize(120, 40)
	output := m.View()

	lower := strings.ToLower(output)
	if !strings.Contains(lower, "cloudtrail") {
		t.Errorf("Help (YAML) View() does not contain 'cloudtrail'; got:\n%s", output)
	}
	if !strings.Contains(output, "t") {
		t.Errorf("Help (YAML) View() does not contain 't'")
	}
}
