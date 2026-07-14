//go:build integration

package integration

// scenario_vpcpeer_visual_test.go — Phase 8 render-gate for the vpc-peer
// resource. Verifies the rendered TUI output (not fetcher return values)
// matches the universal UI rules and the §4 contract in
// docs/resources/vpc-peer.md.
//
// vpc-peer is a single-call type (DescribeVpcPeeringConnections carries the
// whole story — no N+1, no degraded rows). State findings are fetcher-written
// and color-bearing; the two derived route checks (no-local-route,
// blackholed) are `~` background checks from the zero-API rtb cache-scan
// enricher — green rows carrying the S4 phrase, no menu-badge bump.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// §4 phrases pinned locally — any drift in the fetcher/enricher surfaces here
// instead of in unit tests that could be rewritten without noticing.
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

	// Detail-view Attention entries (capitalizeFirst applied at render).
	vpcPeerDetailOverlap   = "CIDR overlap with peer"
	vpcPeerDetailRejected  = "Rejected"
	vpcPeerDetailNoRoute   = "No local route to peer"
	vpcPeerDetailBlackhole = "Route to peer blackholed"
)

func TestScenario_VpcPeerVisual(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	// S1 menu badge — issue-COLORED rows: provisioning, initiating, pending,
	// expired, deleting, overlap (6 Warning) + rejected, failed (2 Broken)
	// = 8. Dim (deleted) and the two `~` route checks do not bump.
	scenario.ExpectMenuIssueCount("vpc-peer", 8)

	scenario.OpenList("vpc-peer")

	// Universal column rules — no jargon columns.
	for _, jargon := range []string{
		"CIS", " Flags", " Issues ", "NOBKP", "UNENC", "NOPROT", "PUB ",
	} {
		scenario.ExpectViewNotContains(jargon)
	}

	// Healthy row: blank Status — active, routed, disjoint CIDRs.
	scenario.ExpectRowStatusBlank(demofixtures.ProdPeerSharedID)

	// §4 state phrases.
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

	// The two `~` route background checks: green rows carrying the phrase.
	// (The S3 `~` glyph does not render fleet-wide when a list opens after
	// the sweep — tracked as https://github.com/k2m30/a9s/issues/458; pin
	// the glyph here once that lands.)
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerNoRouteID, vpcPeerPhraseNoRoute)
	scenario.ExpectRowStatusEquals(demofixtures.WarnPeerBlackholeID, vpcPeerPhraseBlackhole)

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

	// 8.4 user-visible sanity render (mandatory).
	t.Log("\n" + scenario.currentView())

	scenario.ExpectViewContains(vpcPeerDetailRejected)
	scenario.ExpectViewContains("Rejected by accepter")

	scenario.Back()

	// Both `~` findings surface as their own S5 Attention entries.
	blackhole := selectVpcPeerByID(t, scenario, demofixtures.WarnPeerBlackholeID)
	scenario.OpenDetailResource("vpc-peer", blackhole)
	scenario.ExpectNoAPIError()
	scenario.ExpectViewContains(vpcPeerDetailBlackhole)

	scenario.AssertNoEnrichmentErrors()
}

// TestScenario_VpcPeerVisual_HealthySilence — spec §4 "Healthy silence": the
// showroom row renders no Attention section and no finding phrase; the
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
