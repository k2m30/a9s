//go:build integration

package integration

// scenario_detail_focus_keys_test.go — regression pin for the focused-related
// key routing defect (user-reported 2026-07-14): y (YAML), J (JSON) and
// t (CloudTrail) were dead while the related column held the cursor, because
// handleDetailKeyMsg's focused default case fed every unowned key to the
// right-column widget as filter input. The fix routes unowned keys to the
// detail screen's own cases whenever the filter input is not active.

import (
	"strings"
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

func TestScenario_DetailFocusKeys_NavKeysWorkWithRelatedFocused(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	scenario.OpenList("transfer")
	root := fullIntegrationMustFindResourceByID(t, scenario.clients, "transfer", demofixtures.ProdAS2GatewayID)

	// y: YAML opens with the related column focused.
	scenario.OpenDetailResource("transfer", root)
	scenario.ExpectNoAPIError()
	scenario.Press("l")
	scenario.Press("y")
	if !strings.Contains(scenario.currentView(), "yaml") {
		t.Fatalf("y with related column focused did not open the YAML view:\n%s", scenario.currentView())
	}
	scenario.Back()

	// J: JSON opens with the related column focused.
	scenario.OpenDetailResource("transfer", root)
	scenario.Press("l")
	scenario.Press("J")
	if !strings.Contains(scenario.currentView(), "json") {
		t.Fatalf("J with related column focused did not open the JSON view:\n%s", scenario.currentView())
	}
	scenario.Back()

	// t: CloudTrail drill navigates with the related column focused.
	scenario.OpenDetailResource("transfer", root)
	scenario.Press("l")
	scenario.Press("t")
	if !strings.Contains(scenario.currentView(), "ct-events") {
		t.Fatalf("t with related column focused did not open CloudTrail events:\n%s", scenario.currentView())
	}
	scenario.Back()

	// '/' while focused still starts the related filter (the one key the
	// widget owns outside filter-input mode) — guard against over-fixing.
	scenario.OpenDetailResource("transfer", root)
	scenario.Press("l")
	scenario.Type("/")
	scenario.Type("vpc")
	view := scenario.currentView()
	if !strings.Contains(view, "vpc") {
		t.Fatalf("related filter via / stopped working with focus held:\n%s", view)
	}
}
