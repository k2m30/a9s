//go:build integration

package integration

// scenario_vpcpeer_visual_test.go checks the rendered TUI output (not
// fetcher return values) for vpc-peer against the universal UI rules and
// docs/resources/vpc-peer.md.
//
// vpc-peer is a single-call type (DescribeVpcPeeringConnections carries the
// whole story — no N+1, no degraded rows). Every finding is color-bearing
// (the fleet color invariant); the two derived route checks (no-local-route,
// blackholed) come from the zero-API rtb cache-scan enricher as `~`-class
// findings — Warning-colored rows that do not bump the menu badge.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	vpcPeerPhraseProvisioning = "provisioning"
	vpcPeerPhraseInitiating   = "initiating"
	vpcPeerPhrasePending      = "pending acceptance: expires in 3d"
	vpcPeerPhraseExpired      = "expired: never accepted"
	vpcPeerPhraseRejected     = "rejected"
	vpcPeerPhraseFailed       = "failed"
	vpcPeerPhraseDeleting     = "deleting"
	vpcPeerPhraseDeleted      = "deleted"
	vpcPeerPhraseOverlap      = "CIDR overlap with peer"
	vpcPeerPhraseNoRoute      = "no local route to peer"
	vpcPeerPhraseBlackhole    = "route to peer blackholed"

	// Detail-view Attention entries: the first letter is capitalized for display.
	vpcPeerDetailOverlap   = "CIDR overlap with peer"
	vpcPeerDetailRejected  = "Rejected"
	vpcPeerDetailNoRoute   = "No local route to peer"
	vpcPeerDetailBlackhole = "Route to peer blackholed"
)

func TestScenario_VpcPeerVisual(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// Menu badge — issue-COLORED rows: provisioning, initiating, pending,
	// expired, deleting, overlap (6 Warning) + rejected, failed (2 Broken)
	// = 8. Dim (deleted) and the two `~` route checks do not bump.
	scenario.ExpectMenuIssueCount("vpc-peer", 8)

	scenario.OpenList("vpc-peer")

	for _, jargon := range []string{
		"CIS", " Flags", " Issues ", "NOBKP", "UNENC", "NOPROT", "PUB ",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// The healthy row is active, routed, with disjoint CIDRs.
	scenario.ExpectRowStatusBlank(demofixtures.ProdPeerSharedID)

	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerProvisioningID, vpcPeerPhraseProvisioning)
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerInitiatingID, vpcPeerPhraseInitiating)
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerPendingID, vpcPeerPhrasePending)
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerExpiredID, vpcPeerPhraseExpired)
	scenario.ExpectRowStatusEquals(demofixtures.BrokenPeerRejectedID, vpcPeerPhraseRejected)
	scenario.ExpectRowStatusEquals(demofixtures.BrokenPeerFailedID, vpcPeerPhraseFailed)
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerDeletingID, vpcPeerPhraseDeleting)
	scenario.ExpectRowStatusEquals(demofixtures.DimPeerDeletedID, vpcPeerPhraseDeleted)

	// Config-derived warning: active-only CIDR overlap.
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerOverlapID, vpcPeerPhraseOverlap)

	// The two `~`-class route background checks colour their rows Warning;
	// the `~` class only keeps them out of the menu badge.
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerNoRouteID, vpcPeerPhraseNoRoute)
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerBlackholeID, vpcPeerPhraseBlackhole)

	for _, id := range []string{
		demofixtures.ProdPeerSharedID,
		demofixtures.WarnPeerProvisioningID,
		demofixtures.WarnPeerInitiatingID,
		demofixtures.WarnPeerPendingID,
		demofixtures.WarnPeerExpiredID,
		demofixtures.BrokenPeerRejectedID,
		demofixtures.BrokenPeerFailedID,
		demofixtures.WarnPeerDeletingID,
		demofixtures.DimPeerDeletedID,
		demofixtures.WarnPeerOverlapID,
		demofixtures.WarnPeerNoRouteID,
		demofixtures.WarnPeerBlackholeID,
	} {
		scenario.ExpectRowNoGlyphPrefix(id)
	}

	// Related panel — graph root: rtb 2 (two tables route to it), vpc 1
	// (the LOCAL side only — the cross-account accepter renders as plain
	// OwnerId/VpcId facts, never a pivot entry).
	root := selectVpcPeerByID(t, scenario, demofixtures.ProdPeerSharedID)
	scenario.OpenDetailResource("vpc-peer", root)
	scenario.ExpectNoAPIError()
	scenario.ExpectRelatedRowCountAtLeast("Route Tables", 2)
	scenario.ExpectRelatedRowCountAtLeast("VPC", 1)

	scenario.Back()

	// The rejected row's detail carries Status.Message verbatim in the
	// Attention entry's sentence — phrase stays the short "rejected".
	rejected := selectVpcPeerByID(t, scenario, demofixtures.BrokenPeerRejectedID)
	scenario.OpenDetailResource("vpc-peer", rejected)
	scenario.ExpectNoAPIError()

	t.Log("\n" + scenario.currentView())

	scenario.ExpectViewContains(vpcPeerDetailRejected)
	scenario.ExpectViewContains("Rejected by accepter")

	scenario.Back()

	blackhole := selectVpcPeerByID(t, scenario, demofixtures.WarnPeerBlackholeID)
	scenario.OpenDetailResource("vpc-peer", blackhole)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(vpcPeerDetailBlackhole)

	scenario.Back()

	// The no-route `~` check surfaces in its row's detail too.
	noRoute := selectVpcPeerByID(t, scenario, demofixtures.WarnPeerNoRouteID)
	scenario.OpenDetailResource("vpc-peer", noRoute)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(vpcPeerDetailNoRoute)

	scenario.Back()

	// The CIDR-overlap warning's detail names the finding and the ranges.
	overlap := selectVpcPeerByID(t, scenario, demofixtures.WarnPeerOverlapID)
	scenario.OpenDetailResource("vpc-peer", overlap)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(vpcPeerDetailOverlap)

	scenario.AssertNoEnrichmentErrors()
}

// TestScenario_VpcPeerVisual_HealthySilence — the showroom row renders no
// Attention section and no finding phrase; the
// cross-account accepter renders as plain facts (OwnerId visible), never a
// signal.
func TestScenario_VpcPeerVisual_HealthySilence(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)
	scenario.OpenList("vpc-peer")

	res := selectVpcPeerByID(t, scenario, demofixtures.ProdPeerSharedID)
	scenario.OpenDetailResource("vpc-peer", res)
	scenario.ExpectNoAPIError()
	view := scenario.currentView()
	t.Log("\n" + view)

	expectNoAttentionSection(t, view)
	for _, phrase := range []string{
		vpcPeerPhraseOverlap, vpcPeerPhraseNoRoute, vpcPeerPhraseBlackhole,
	} {
		scenario.ExpectViewNotContains(phrase)
	}
	// Cross-account fact rendering: the remote owner is visible as data.
	scenario.ExpectViewContains("210987654321")
}

// selectVpcPeerByID looks up a concrete vpc-peer resource from the demo
// clients so the scenario can call OpenDetailResource with a real resource.
func selectVpcPeerByID(t *testing.T, s *fullIntegrationScenario, id string) resource.Resource {
	t.Helper()
	return fullIntegrationMustFindResourceByID(t, s.clients, "vpc-peer", id)
}
