package unit

// help_test.go — the CloudTrail "t" key appears in help: the "t"/cloudtrail
// keybinding entry itself (not the ct-events data-format legend, which is
// covered by views_help_ct_events_legend_test.go /
// views_help_resource_wiring_test.go) appears in the general keybinding list
// for ResourceList/Detail/YAML help contexts, through the live
// NewHelpWithResource constructor (buildGroups/domainContext).

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ═══════════════════════════════════════════════════════════════════════════
// Help view tests — the CloudTrail "t" key appears in help
// ═══════════════════════════════════════════════════════════════════════════

// TestHelp_ResourceList_ShowsCloudTrailKey verifies that the ResourceList
// help view renders the "t" key with "cloudtrail" description.
func TestHelp_ResourceList_ShowsCloudTrailKey(t *testing.T) {
	m := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceList, "ec2")
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
	m := views.NewHelpWithResource(keys.Default(), views.HelpFromDetail, "ec2")
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
	m := views.NewHelpWithResource(keys.Default(), views.HelpFromYAML, "ec2")
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
