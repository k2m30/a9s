//go:build integration

package integration

import (
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// TestCostsScenario_GridPivotDrillAndBackToMenu is the Cost Explorer's first
// tests/integration/ scenario-harness coverage: `make integration` (the
// release gate) previously exercised internal/costs, internal/costs/screen
// and internal/app/costs_*.go only via tests/unit, never end-to-end through
// the real tui.Model.Update() loop the way every other resource screen is
// covered here.
//
// Walks: open Cost Explorer via the same Navigate{Target: TargetCosts}
// message the ":costs"/":ce" colon-command dispatches (internal/app/
// actions_view.go, internal/tui/app_input.go) -> assert the SERVICE grid
// renders with the planted growth-story anomaly cell -> pivot to REGION and
// back to SERVICE via digit keys -> drill the anomaly cell down to
// USAGE_TYPE -> back up -> drill the CURRENT month's SERVICE cell all the
// way to a RESOURCE_ID row and into the real EC2 detail view -> Esc chain
// back through every drill frame to the grid and out to the main menu.
func TestCostsScenario_GridPivotDrillAndBackToMenu(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	// growthLabel mirrors stripCostsServiceVendorPrefix (internal/app/
	// costs_body.go): the SERVICE row label strips the "Amazon "/"AWS "
	// vendor prefix a real Cost Explorer SERVICE dimension value carries.
	growthLabel := strings.TrimPrefix(fixtures.CostsGrowthService, "Amazon ")
	growthLabel = strings.TrimPrefix(growthLabel, "AWS ")

	// --- open costs via the command path ---
	scenario.beginAction("command :costs")
	scenario.applyAndDrain(messages.Navigate{Target: messages.TargetCosts})
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("Costs: by service")
	scenario.ExpectFrameContains("SERVICE")
	scenario.ExpectFrameContains(growthLabel)
	scenario.ExpectFrameContains("▲") // the planted growth-story anomaly mark

	// --- pivot to region and back (digit keys) ---
	scenario.Press("2") // CostPivot digit 2 -> DimensionRegion
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("REGION")
	scenario.ExpectFrameContains(fixtures.CostsDemoRegion)

	scenario.Press("1") // CostPivot digit 1 -> DimensionService (back)
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("Costs: by service")
	scenario.ExpectFrameContains(growthLabel)
	scenario.ExpectFrameContains("▲")

	// --- drill the growth cell to usage types ---
	// The root frame opens with the cursor on the newest (rightmost) of the
	// 12 trailing months (FR-002 "open at today"); fixtures.CostsGrowthMonth
	// is fixed, by fixtures/costs.go's own construction, at exactly 6 months
	// before the anchor month -> exactly 6 ScrollLeft presses from the
	// newest column.
	const growthMonthColumnsBack = 6
	for range growthMonthColumnsBack {
		scenario.Press("left")
	}
	scenario.Press("enter") // Select: SERVICE growth cell -> USAGE_TYPE (growth month)
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("USAGE_TYPE")
	scenario.ExpectFrameContains(growthLabel) // breadcrumb: pinned SERVICE value
	scenario.ExpectFrameContains(fixtures.CostsGrowthUsageType)

	// --- return ---
	scenario.Back()
	scenario.ExpectFrameContains("Costs: by service")

	// --- drill the CURRENT month down to resource rows and into EC2 detail ---
	// applyCostsBack restores the parent frame's cursor exactly as it was
	// before the child drill (still on the growth cell) — scroll back right
	// to the newest (current) month before drilling again.
	for range growthMonthColumnsBack {
		scenario.Press("right")
	}
	scenario.Press("enter") // Select: SERVICE current-month cell -> USAGE_TYPE (current month)
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("USAGE_TYPE")
	scenario.ExpectFrameContains(growthLabel)

	// A fresh drill frame's cursor starts on the OLDEST column of its own
	// (weekly) window; over-scroll right to the newest week — ScrollRight
	// clamps at the window's own end, so this is safe regardless of how many
	// weeks the current month actually has.
	const overScrollWeeks = 8
	for range overScrollWeeks {
		scenario.Press("right")
	}
	scenario.Press("enter") // Select: USAGE_TYPE row -> RESOURCE_ID
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("RESOURCE_ID")

	// The resource-level drill dataset (fixtures.CostsResourceRowsByService)
	// is keyed by SERVICE only, not by usage type — resolve the row the grid
	// will actually sort to the top (largest DailyAmount) via the fixture
	// data itself, then look up its real Name via the discovery helper so
	// this test never hardcodes a fixture literal.
	ec2Rows := fixtures.CostsResourceRowsByService[awsclient.CostExplorerServiceNameEC2]
	if len(ec2Rows) == 0 {
		t.Fatal("precondition: fixtures.CostsResourceRowsByService has no EC2 rows")
	}
	target := ec2Rows[0]
	for _, row := range ec2Rows {
		if row.DailyAmount > target.DailyAmount {
			target = row
		}
	}
	ec2Resource := fullIntegrationMustFindResourceByID(t, scenario.clients, "ec2", target.ResourceID)

	scenario.Press("enter") // Select: RESOURCE_ID row -> EC2 detail (KindFetchByIDDetail)
	scenario.ExpectNoAPIError()
	scenario.ExpectCurrentResourceType("ec2")
	scenario.ExpectCurrentResourceID(target.ResourceID)
	// The detail frame title renders Resource.Name when present, falling
	// back to the raw ID only when Name is empty (internal/app/
	// detail_state.go's detailFrameTitleLocked) — the fixture instance
	// always carries a Name, so the frame title itself shows the name, not
	// the "i-..." string; ExpectCurrentResourceID above pins the exact
	// fixture instance ID.
	scenario.ExpectFrameContains(ec2Resource.Name)

	// --- Esc chain back to the grid and menu ---
	scenario.Back() // EC2 detail -> costs screen (RESOURCE_ID frame)
	scenario.ExpectFrameContains("Costs:")
	scenario.Back() // RESOURCE_ID -> USAGE_TYPE (current month)
	scenario.ExpectFrameContains("USAGE_TYPE")
	scenario.Back() // USAGE_TYPE -> SERVICE root
	scenario.ExpectFrameContains("Costs: by service")
	scenario.Back() // SERVICE root -> main menu
	scenario.ExpectFrameContains("Cost Explorer")
}
