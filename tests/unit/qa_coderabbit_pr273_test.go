package unit

// qa_coderabbit_pr273_test.go — regression pins for CodeRabbit PR #273 review findings.
//
// Each test currently FAILS against HEAD (pins a real bug, not a verified fix).
// Do NOT modify production code to make these pass — production fixes belong in
// separate commits that also delete or update the failing test here.
//
// Covered items (skipped items were already fixed in prior commits):
//
//   Item 1:  EKS Node Groups (ng) — AlwaysHealthy=true suppresses real lifecycle states
//            (CREATING/UPDATING/DELETING → Warning; CREATE_FAILED/DEGRADED → Broken).
//   Item 4:  VPC Endpoints (vpce) and Transit Gateways (tgw) — AlwaysHealthy=true is
//            wrong for types with stateful lifecycle (pending/failed/deleting).
//   Item 6:  Missing Gen==0 bypass in handleEnrichmentChecked — Gen=0 test-injection
//            messages are dropped when enrichmentGen>0 after a profile/region switch.
//   Item 12/13: CodeBuild STOPPED state generates an unwanted finding — intentionally
//            cancelled builds should not be flagged as issues.
//   Item 14: Staging EC2 instances (i-0a1b2c3d4e5f60030, i-0a1b2c3d4e5f60031) fall
//            through defaultExtras() and inherit the prod web-ALB security group
//            (sg-0aaa111111111111a) instead of a staging SG.
//
// Skipped items (already fixed in prior commits):
//   Items 2/3:   RDS/DynamoDB — fixed in 35a54d4
//   Item 5:      CloudWatch alarms Color — already correct
//   Items 7/8:   handleRegionSelected / handleProfileSelected — fixed in 2f9a808, aae6860
//   Items 9/10:  isVisibleUnderIssueFilter truncation guard — fixed in aae6860..2e831e1
//   Item 11:     AlwaysHealthy invariant test — already exists in qa_ctrlz_truncated_zero_health_state_test.go

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// =============================================================================
// Item 18: Main-menu ctrl+z — no false positives, no false negatives
// =============================================================================
//
// Expected behavior — ctrl+z on the main menu, after AWS probes return:
//
// The public boundary of a9s against AWS is the set of messages that land on
// the root model: AvailabilityCacheLoadedMsg (restored from cache),
// AvailabilityCheckedMsg (one type's Wave-1 probe result),
// AvailabilityPrefetchedMsg (no-cache prefetch), and EnrichmentCheckedMsg
// (one type's Wave-2 enrichment result). Each of these is the product of a
// real AWS list/describe call. A behavior test against "AWS says X" drives
// the model with these messages; the model's rendered View() is the behavior
// under test.
//
// Contract:
//   * No false negative: if ANY wave reports Issues > 0 for a type, that
//     type MUST be visible under ctrl+z. (The user needs to see issues.)
//   * No false positive: if both Wave-1 and Wave-2 report Issues == 0 for a
//     type, and Wave 2 has actually run for that type (so zero is
//     authoritative, not a lower-bound guess), that type MUST NOT be visible
//     under ctrl+z. (A healthy type has no business under an attention-only
//     filter.)
//   * Confirmed-zero (issues=0, truncated=false) from Wave 1 alone is also
//     authoritative — the probe ran the full page and found nothing.
//
// The user saw "Target Groups (4)" under ctrl+z with no issue badge despite
// every target being healthy — a false positive. The test below reproduces
// it by driving Wave-1 + Wave-2 through public messages with zero issues
// for tg and asserting tg is NOT visible under ctrl+z.

// TestCR273_Item18_MenuCtrlZ_NoFalsePositives_AllTypes drives the happy path
// across every registered resource type: Wave 1 reports zero issues not
// truncated, Wave 2 runs clean for every enricher-backed type. Every type
// must be HIDDEN under ctrl+z.
func TestCR273_Item18_MenuCtrlZ_NoFalsePositives_AllTypes(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	entries := map[string]int{}
	truncated := map[string]bool{}
	issueCounts := map[string]int{}
	issueTruncated := map[string]bool{}
	issueKnown := map[string]bool{}
	for _, td := range resource.AllResourceTypes() {
		if td.ExcludeFromIssueBadge {
			continue
		}
		entries[td.ShortName] = 1
		truncated[td.ShortName] = false
		issueCounts[td.ShortName] = 0
		issueTruncated[td.ShortName] = false
		issueKnown[td.ShortName] = true
	}

	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries:        entries,
		Truncated:      truncated,
		IssueCounts:    issueCounts,
		IssueTruncated: issueTruncated,
		IssueKnown:     issueKnown,
		Expired:        false,
	})

	// Wave 2 clean for every enricher-backed type.
	for _, ent := range awsclient.AllWave2() {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: ent.ShortName,
			Issues:       0,
			Truncated:    false,
			Findings:     map[string]domain.Finding{},
			Err:          nil,
			Gen:          0,
			TypeGen:      0,
		})
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	plain := stripANSI(rootViewContent(m))

	var falsePositives []string
	for _, td := range resource.AllResourceTypes() {
		if td.ExcludeFromIssueBadge {
			continue
		}
		if strings.Contains(plain, td.Name) {
			falsePositives = append(falsePositives, td.ShortName+" ("+td.Name+")")
		}
	}

	if len(falsePositives) > 0 {
		t.Errorf(
			"AWS reported zero issues everywhere (both waves) but these types appear under ctrl+z — false positives:\n  %v\n\nRendered menu:\n%s",
			falsePositives, plain,
		)
	}
}

// TestCR273_Item18_MenuCtrlZ_Wave2AuthoritativeZero_AllEnricherTypes drives
// the truncated-zero → authoritative-zero flip for every enricher-backed
// type. Wave 1 reports issueTruncated=true issues=0 (first-page lower
// bound); Wave 2 runs clean (Truncated=false, Findings={}). Every
// enricher-backed type must flip to HIDDEN under ctrl+z.
func TestCR273_Item18_MenuCtrlZ_Wave2AuthoritativeZero_AllEnricherTypes(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	entries := map[string]int{}
	issueCounts := map[string]int{}
	issueTruncated := map[string]bool{}
	issueKnown := map[string]bool{}
	for _, ent := range awsclient.AllWave2() {
		entries[ent.ShortName] = 1
		issueCounts[ent.ShortName] = 0
		issueTruncated[ent.ShortName] = true // Wave 1 lower bound
		issueKnown[ent.ShortName] = true
	}

	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries:        entries,
		Truncated:      map[string]bool{},
		IssueCounts:    issueCounts,
		IssueTruncated: issueTruncated,
		IssueKnown:     issueKnown,
		Expired:        false,
	})

	// Wave 2: authoritative zero for every enricher-backed type.
	for _, ent := range awsclient.AllWave2() {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: ent.ShortName,
			Issues:       0,
			Truncated:    false,
			Findings:     map[string]domain.Finding{},
			Err:          nil,
			Gen:          0,
			TypeGen:      0,
		})
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	plain := stripANSI(rootViewContent(m))

	var stuckVisible []string
	for _, ent := range awsclient.AllWave2() {
		td := resource.FindResourceType(ent.ShortName)
		if td == nil || td.ExcludeFromIssueBadge {
			continue
		}
		if strings.Contains(plain, td.Name) {
			stuckVisible = append(stuckVisible, ent.ShortName+" ("+td.Name+")")
		}
	}

	if len(stuckVisible) > 0 {
		t.Errorf(
			"Wave 2 confirmed zero for every enricher-backed type (account-wide authoritative) "+
				"but these types remain visible under ctrl+z — false positives:\n  %v\n\n"+
				"Expected: Wave 2 Truncated=false with empty Findings flips the menu from "+
				"truncated-zero lower-bound to confirmed-zero.\n\nRendered menu:\n%s",
			stuckVisible, plain,
		)
	}
}

// TestCR273_Item18_MenuCtrlZ_Wave2ErroredSubCall_AllEnricherTypes pins the
// user's screenshot case across every enricher-backed type: one sub-call
// erred in Wave 2, so the enricher returned Truncated=true with IssueCount=0
// and empty Findings. The result carries no actual issue — the type must
// NOT appear under ctrl+z.
//
// Affected enrichers (per-resource callers that promote error to truncated):
// every entry in awsclient.AllWave2(). This test iterates all of them.
func TestCR273_Item18_MenuCtrlZ_Wave2ErroredSubCall_AllEnricherTypes(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	entries := map[string]int{}
	issueCounts := map[string]int{}
	issueTruncated := map[string]bool{}
	issueKnown := map[string]bool{}
	for _, ent := range awsclient.AllWave2() {
		entries[ent.ShortName] = 1
		issueCounts[ent.ShortName] = 0
		issueTruncated[ent.ShortName] = false
		issueKnown[ent.ShortName] = true
	}

	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries:        entries,
		Truncated:      map[string]bool{},
		IssueCounts:    issueCounts,
		IssueTruncated: issueTruncated,
		IssueKnown:     issueKnown,
		Expired:        false,
	})

	// Wave 2: for each enricher-backed type, a sub-call errored → Truncated=true,
	// but Findings={} and Issues=0 (no actual issue seen).
	for _, ent := range awsclient.AllWave2() {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: ent.ShortName,
			Issues:       0,
			Truncated:    true,
			Findings:     map[string]domain.Finding{},
			Err:          nil,
			Gen:          0,
			TypeGen:      0,
		})
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	plain := stripANSI(rootViewContent(m))

	var falsePositives []string
	for _, ent := range awsclient.AllWave2() {
		td := resource.FindResourceType(ent.ShortName)
		if td == nil || td.ExcludeFromIssueBadge {
			continue
		}
		if strings.Contains(plain, td.Name) {
			falsePositives = append(falsePositives, ent.ShortName+" ("+td.Name+")")
		}
	}

	if len(falsePositives) > 0 {
		t.Errorf(
			"Wave 2 errored on one sub-call per enricher (Truncated=true, Issues=0, Findings={}) "+
				"— the enricher saw zero actual issues but set the truncation flag. These types "+
				"appear under ctrl+z with no issue badge — false positives:\n  %v\n\n"+
				"Contract: when Wave 2 returns IssueCount=0 AND Findings is empty, Truncated must "+
				"NOT promote the type into the attention filter. Truncation signals count "+
				"completeness, not hidden issues — if the enricher had seen an issue, it would "+
				"have produced a Finding.\n\nRendered menu:\n%s",
			falsePositives, plain,
		)
	}
}

// TestCR273_Item18_MenuCtrlZ_NoFalseNegatives_AllEnricherTypes is the
// positive guard: for each enricher-backed type, inject ONE type at a time
// with issues=2 and assert it IS visible under ctrl+z. A missing type under
// ctrl+z while AWS reports issues is a false negative.
func TestCR273_Item18_MenuCtrlZ_NoFalseNegatives_AllEnricherTypes(t *testing.T) {
	tui.Version = "0.6.0"
	var falseNegatives []string
	for _, ent := range awsclient.AllWave2() {
		shortName := ent.ShortName
		td := resource.FindResourceType(shortName)
		if td == nil || td.ExcludeFromIssueBadge {
			continue
		}
		m := newRootSizedModel()
		m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
			Entries:        map[string]int{shortName: 3},
			Truncated:      map[string]bool{},
			IssueCounts:    map[string]int{shortName: 2},
			IssueTruncated: map[string]bool{shortName: false},
			IssueKnown:     map[string]bool{shortName: true},
			Expired:        false,
		})
		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
		plain := stripANSI(rootViewContent(m))
		if !strings.Contains(plain, td.Name) {
			falseNegatives = append(falseNegatives, shortName+" ("+td.Name+")")
		}
	}
	if len(falseNegatives) > 0 {
		t.Errorf(
			"AWS reported 2 issues for these enricher-backed types but they are NOT visible "+
				"under ctrl+z — false negatives:\n  %v",
			falseNegatives,
		)
	}
}

// TestCR273_Item18_MenuCtrlZ_NoFalseNegatives_AllRegisteredTypes is the
// broader positive guard across every registered type (not just
// enricher-backed): if AWS reports issues, the type MUST be visible.
func TestCR273_Item18_MenuCtrlZ_NoFalseNegatives_AllRegisteredTypes(t *testing.T) {
	tui.Version = "0.6.0"
	var falseNegatives []string
	for _, td := range resource.AllResourceTypes() {
		if td.ExcludeFromIssueBadge {
			continue
		}
		m := newRootSizedModel()
		m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
			Entries:        map[string]int{td.ShortName: 3},
			Truncated:      map[string]bool{},
			IssueCounts:    map[string]int{td.ShortName: 2},
			IssueTruncated: map[string]bool{td.ShortName: false},
			IssueKnown:     map[string]bool{td.ShortName: true},
			Expired:        false,
		})
		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
		plain := stripANSI(rootViewContent(m))
		if !strings.Contains(plain, td.Name) {
			falseNegatives = append(falseNegatives, td.ShortName+" ("+td.Name+")")
		}
	}
	if len(falseNegatives) > 0 {
		t.Errorf(
			"AWS reported 2 issues for these types but they are NOT visible under ctrl+z — false negatives:\n  %v",
			falseNegatives,
		)
	}
}

// =============================================================================
// Item 1: EKS Node Groups (ng) — AlwaysHealthy=true suppresses lifecycle states
// =============================================================================

// TestCR273_Item1_NGHasColor asserts that EKS Node Groups have a non-nil Color
// func. Node groups have stateful lifecycle (CREATING/UPDATING/DELETING/
// CREATE_FAILED/DEGRADED); classification must happen.
// (Pre-AH-purge this asserted AlwaysHealthy==false; the field has since been
// removed, so we assert the Color func itself exists.)
func TestCR273_Item1_NGHasColor(t *testing.T) {
	td := resource.FindResourceType("ng")
	if td == nil {
		t.Fatal("resource type 'ng' not registered")
	}
	if td.Color == nil {
		t.Error("ng.Color is nil: node groups have stateful lifecycle and must classify via Color")
	}
}

// TestCR273_Item1_NG_CREATING_ReturnsWarning asserts that a node group in
// CREATING state is classified as ColorWarning (transitioning/degrading).
// Currently FAILS: Color func hardcodes `return ColorHealthy`.
func TestCR273_Item1_NG_CREATING_ReturnsWarning(t *testing.T) {
	td := resource.FindResourceType("ng")
	if td == nil {
		t.Fatal("resource type 'ng' not registered")
	}
	r := resource.Resource{
		ID:     "ng-test-1",
		Name:   "acme-node-group-01",
		Fields: map[string]string{"status": "CREATING"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ng Color(CREATING) = %v, want ColorWarning (%v): transitioning state must be yellow/warning", got, resource.ColorWarning)
	}
}

// TestCR273_Item1_NG_UPDATING_ReturnsWarning asserts that UPDATING → ColorWarning.
// Currently FAILS: Color func hardcodes `return ColorHealthy`.
func TestCR273_Item1_NG_UPDATING_ReturnsWarning(t *testing.T) {
	td := resource.FindResourceType("ng")
	if td == nil {
		t.Fatal("resource type 'ng' not registered")
	}
	r := resource.Resource{
		ID:     "ng-test-2",
		Name:   "acme-node-group-02",
		Fields: map[string]string{"status": "UPDATING"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ng Color(UPDATING) = %v, want ColorWarning (%v)", got, resource.ColorWarning)
	}
}

// TestCR273_Item1_NG_DELETING_ReturnsWarning asserts that DELETING → ColorWarning.
// Currently FAILS: Color func hardcodes `return ColorHealthy`.
func TestCR273_Item1_NG_DELETING_ReturnsWarning(t *testing.T) {
	td := resource.FindResourceType("ng")
	if td == nil {
		t.Fatal("resource type 'ng' not registered")
	}
	r := resource.Resource{
		ID:     "ng-test-3",
		Name:   "acme-node-group-03",
		Fields: map[string]string{"status": "DELETING"},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("ng Color(DELETING) = %v, want ColorWarning (%v)", got, resource.ColorWarning)
	}
}

// TestCR273_Item1_NG_CREATE_FAILED_ReturnsBroken asserts that CREATE_FAILED → ColorBroken.
// Currently FAILS: Color func hardcodes `return ColorHealthy`.
func TestCR273_Item1_NG_CREATE_FAILED_ReturnsBroken(t *testing.T) {
	td := resource.FindResourceType("ng")
	if td == nil {
		t.Fatal("resource type 'ng' not registered")
	}
	r := resource.Resource{
		ID:     "ng-test-4",
		Name:   "acme-node-group-04",
		Fields: map[string]string{"status": "CREATE_FAILED"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("ng Color(CREATE_FAILED) = %v, want ColorBroken (%v): failed creation is a hard error", got, resource.ColorBroken)
	}
}

// TestCR273_Item1_NG_DEGRADED_ReturnsBroken asserts that DEGRADED → ColorBroken.
// Currently FAILS: Color func hardcodes `return ColorHealthy`.
func TestCR273_Item1_NG_DEGRADED_ReturnsBroken(t *testing.T) {
	td := resource.FindResourceType("ng")
	if td == nil {
		t.Fatal("resource type 'ng' not registered")
	}
	r := resource.Resource{
		ID:     "ng-test-5",
		Name:   "acme-node-group-05",
		Fields: map[string]string{"status": "DEGRADED"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("ng Color(DEGRADED) = %v, want ColorBroken (%v): degraded node group is a broken/impaired state", got, resource.ColorBroken)
	}
}

// TestCR273_Item1_NG_ACTIVE_ReturnsHealthy asserts that the nominal ACTIVE state
// remains ColorHealthy after the fix.
func TestCR273_Item1_NG_ACTIVE_ReturnsHealthy(t *testing.T) {
	td := resource.FindResourceType("ng")
	if td == nil {
		t.Fatal("resource type 'ng' not registered")
	}
	r := resource.Resource{
		ID:     "ng-test-6",
		Name:   "acme-node-group-06",
		Fields: map[string]string{"status": "ACTIVE"},
	}
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("ng Color(ACTIVE) = %v, want ColorHealthy (%v): nominal state must not trigger issues", got, resource.ColorHealthy)
	}
}

// =============================================================================
// Item 4: VPCE and TGW — AlwaysHealthy=true wrong for stateful lifecycle types
// =============================================================================

// TestCR273_Item4_VPCEHasColor asserts VPC Endpoints classify via Color.
// VPCEs have a stateful lifecycle (pending/failed/deleting) that must not
// be suppressed. (Pre-AH-purge this asserted AlwaysHealthy==false.)
func TestCR273_Item4_VPCEHasColor(t *testing.T) {
	td := resource.FindResourceType("vpce")
	if td == nil {
		t.Fatal("resource type 'vpce' not registered")
	}
	if td.Color == nil {
		t.Error("vpce.Color is nil: VPC endpoints have stateful lifecycle and must classify via Color")
	}
}

// TestCR273_Item4_TGWHasColor asserts Transit Gateways classify via Color.
// TGWs have a stateful lifecycle (pending/modifying/deleting/deleted).
// (Pre-AH-purge this asserted AlwaysHealthy==false.)
func TestCR273_Item4_TGWHasColor(t *testing.T) {
	td := resource.FindResourceType("tgw")
	if td == nil {
		t.Fatal("resource type 'tgw' not registered")
	}
	if td.Color == nil {
		t.Error("tgw.Color is nil: Transit Gateways have stateful lifecycle and must classify via Color")
	}
}

// =============================================================================
// Item 6: Missing Gen==0 bypass in handleEnrichmentChecked
// =============================================================================

// TestCR273_Item6_Gen0_BypassesSessionGuard asserts that an EnrichmentCheckedMsg
// with Gen=0 is accepted even when enrichmentGen>0 (after a profile/region switch).
//
// The contract: Gen=0 is a reserved test-injection sentinel that bypasses the
// session-wide generation guard. Without this bypass, test doubles that send
// Gen=0 are silently dropped after any profile or region switch, making the
// enrichment system untestable in realistic multi-switch scenarios.
//
// Setup:
//  1. Create a model with enrichmentGen=0.
//  2. Switch profile → bumps enrichmentGen to 1.
//  3. Navigate to EC2 list and load resources.
//  4. Send EnrichmentCheckedMsg{Gen:0, TypeGen:0, ResourceType:"ec2", Issues:1, Findings:{...}}.
//  5. Assert that the issue marker "! " appears in the rendered list view —
//     meaning the message was ACCEPTED, not dropped.
//
// Currently FAILS: line 637 in app_handlers_navigate.go drops Gen=0 when enrichmentGen=1.
func TestCR273_Item6_Gen0_BypassesSessionGuard(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel() // fresh model: enrichmentGen starts at 0

	// Step 2: bump enrichmentGen by switching profile (→ enrichmentGen=1).
	// We don't execute the returned cmd (AWS connect) — only the state update matters.
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "test-profile-switched"})

	genAfterSwitch := m.EnrichmentGen()
	if genAfterSwitch == 0 {
		t.Fatal("pre-condition failed: enrichmentGen must be > 0 after profile switch")
	}

	// Step 3: navigate to EC2 list.
	m = navigateToEC2List(m)

	// Load resources so the list has items to mark.
	resources := rerunEC2Resources()
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources,
	})

	// Step 4: send Gen=0 injection message — must bypass session guard.
	injected := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Issues:       1,
		Truncated:    false,
		Findings: map[string]domain.Finding{
			"i-0abc1111aaa111111": {Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"},
		},
		Gen:     0, // test-injection sentinel
		TypeGen: 0,
	}
	m, _ = rootApplyMsg(m, injected)

	// Step 5: the "! " prefix marker must appear — meaning the finding was applied.
	content := stripANSI(m.View().Content)
	if !strings.Contains(content, "! ") {
		t.Errorf("Gen=0 EnrichmentCheckedMsg must bypass the session guard (enrichmentGen=%d) and apply findings — '! ' marker absent in rendered output:\n%s",
			genAfterSwitch, content)
	}
}

// =============================================================================
// Item 12/13: CodeBuild STOPPED state generates unwanted finding
// =============================================================================

// TestCR273_Item12_CodeBuild_STOPPED_ExcludedFromFindings asserts that a build
// with StatusTypeStopped is NOT flagged as an issue.
//
// STOPPED = intentionally cancelled by a user or automation (e.g. timeout policy,
// manual abort). It is not a failure — treating it as one generates noise and
// inflates the issue badge count.
//
// Currently FAILS: the switch in EnrichCodeBuildBuilds only skips SUCCEEDED and
// IN_PROGRESS; STOPPED falls through and produces a finding with severity "!".
func TestCR273_Item12_CodeBuild_STOPPED_ExcludedFromFindings(t *testing.T) {
	endTime := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{
			"cancelled-pipeline": "cancelled-pipeline:b42",
		},
		builds: map[string]cbtypes.Build{
			"cancelled-pipeline:b42": {
				Id:            aws.String("cancelled-pipeline:b42"),
				BuildStatus:   cbtypes.StatusTypeStopped,
				BuildComplete: true,
				EndTime:       &endTime,
			},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{{ID: "cancelled-pipeline"}}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["cancelled-pipeline"]; ok {
		t.Errorf("STOPPED build must NOT appear in Findings — intentionally cancelled builds are not issues; got finding: %+v", result.Findings["cancelled-pipeline"])
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d for STOPPED build, want 0", result.IssueCount)
	}
}

// TestCR273_Item13_CodeBuild_STOPPED_WithFailed_OnlyFailedCounted asserts that
// when there are both STOPPED and FAILED builds for different projects, only
// the FAILED project appears in Findings, and IssueCount = 1 (not 2).
//
// Currently FAILS: STOPPED falls through and is counted alongside FAILED.
func TestCR273_Item13_CodeBuild_STOPPED_WithFailed_OnlyFailedCounted(t *testing.T) {
	endTime := time.Date(2026, 4, 14, 11, 0, 0, 0, time.UTC)
	fake := &codeBuildEnrichFake{
		projectBuilds: map[string]string{
			"cancelled-job": "cancelled-job:b1",
			"broken-job":    "broken-job:b2",
		},
		builds: map[string]cbtypes.Build{
			"cancelled-job:b1": {
				Id:          aws.String("cancelled-job:b1"),
				BuildStatus: cbtypes.StatusTypeStopped,
				EndTime:     &endTime,
			},
			"broken-job:b2": {
				Id:          aws.String("broken-job:b2"),
				BuildStatus: cbtypes.StatusTypeFailed,
				EndTime:     &endTime,
			},
		},
	}
	clients := &awsclient.ServiceClients{CodeBuild: fake}
	resources := []resource.Resource{
		{ID: "cancelled-job"},
		{ID: "broken-job"},
	}

	result, err := awsclient.EnrichCodeBuildStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["cancelled-job"]; ok {
		t.Errorf("STOPPED build 'cancelled-job' must NOT appear in Findings")
	}
	if _, ok := result.Findings["broken-job"]; !ok {
		t.Errorf("FAILED build 'broken-job' must appear in Findings")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1 (only the FAILED build); STOPPED builds inflate the count", result.IssueCount)
	}
}

// =============================================================================
// Item 14: Staging EC2 instances inherit prod security group in demo fixtures
// =============================================================================

// TestCR273_Item14_StagingInstances_NoProdWebALBSG asserts that EC2 instances
// in the staging VPC (vpc-0def456789abc123d) do not have the prod web-ALB
// security group (sg-0aaa111111111111a) in their SecurityGroups list.
//
// The two staging instances (i-0a1b2c3d4e5f60030, i-0a1b2c3d4e5f60031) are
// absent from namedExtras, so defaultExtras() assigns fixtProdWebALBSGID —
// the prod web-ALB SG — to them. This is a fixture data bug: prod SGs in a
// staging VPC makes no architectural sense.
//
// Currently FAILS: defaultExtras() always assigns fixtProdWebALBSGID as the
// fallback SG regardless of which VPC the instance belongs to.
func TestCR273_Item14_StagingInstances_NoProdWebALBSG(t *testing.T) {
	const (
		fixtStagingVPCID   = "vpc-0def456789abc123d"
		fixtProdWebALBSGID = "sg-0aaa111111111111a"
	)

	fix := fixtures.NewEC2Fixtures()
	if len(fix.Reservations) == 0 {
		t.Fatal("demo EC2 fixtures missing")
	}

	// Scan all reservations for instances in the staging VPC.
	// Security group assignment is on the raw ec2types.Instance struct.
	stagingViolations := 0
	for _, res := range fix.Reservations {
		for _, inst := range res.Instances {
			vpcID := aws.ToString(inst.VpcId)
			if vpcID != fixtStagingVPCID {
				continue
			}
			instanceID := aws.ToString(inst.InstanceId)
			// This instance is in the staging VPC — verify no prod web-ALB SG.
			for _, sg := range inst.SecurityGroups {
				if aws.ToString(sg.GroupId) == fixtProdWebALBSGID {
					stagingViolations++
					t.Errorf("instance %q (VPC %s) has prod web-ALB SG %q — staging instances must not inherit production security groups (fix: add named extras for staging instance IDs in internal/demo/fixtures/ec2.go)",
						instanceID, fixtStagingVPCID, fixtProdWebALBSGID)
				}
			}
		}
	}
	if stagingViolations > 0 {
		t.Logf("staging instances with prod SG: %d — affected IDs: i-0a1b2c3d4e5f60030, i-0a1b2c3d4e5f60031", stagingViolations)
	}
}

// =============================================================================
// Item 2a: DB Instances / DB Clusters — rdsInstanceColor missing "failed" case
// =============================================================================
//
// The color resolver at types_databases.go:44 was fixed to read Fields["status"]
// (the same key the list column renders) instead of "db_instance_status". However
// rdsInstanceColor still has no case for "failed" — a status that can be set
// by the AWS API in certain cluster-level failure scenarios. A resource whose
// status column displays "failed" is incorrectly classified as ColorHealthy,
// hiding it from issue badges and ctrl+z filtering.
//
// Note: the task specification says the bug is the key mismatch ("reads
// db_instance_status"). The key mismatch was fixed in commit 35a54d4. The
// remaining failure is that rdsInstanceColor has no case for "failed", so
// Color(Resource{Fields:{"status":"failed"}}) returns ColorHealthy.
//
// Item 2a for ddb (INACCESSIBLE_ENCRYPTION_CREDENTIALS) is already fixed and
// passes today — pinning is not possible without a different status value.
// DocDB clusters use the same rdsInstanceColor function as dbi, so the same
// "failed" gap applies to "dbc" too (Item 2a DocDB).

// TestCR273_Item2_RDS_ColorReadsStatusKey asserts that a DB instance with
// Fields["status"]="failed" (the key the list column renders) is classified
// as ColorBroken.
//
// Currently FAILS: rdsInstanceColor has no case for "failed" — returns ColorHealthy.
func TestCR273_Item2_RDS_ColorReadsStatusKey(t *testing.T) {
	td := resource.FindResourceType("dbi")
	if td == nil {
		t.Fatal("resource type 'dbi' not registered")
	}
	r := resource.Resource{
		ID:     "db-test-1",
		Name:   "acme-prod-db",
		Fields: map[string]string{"status": "failed"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("dbi Color(status=failed) = %v, want ColorBroken (%v): a resource whose list column shows 'failed' must be classified as broken, not healthy — missing case in rdsInstanceColor",
			got, resource.ColorBroken)
	}
}

// TestCR273_Item2_DocDB_ColorReadsStatusKey asserts that a DB cluster (DocDB)
// in Status=failed renders broken. Post-refactor, Fields["status"] carries the
// §4 phrase ("failed: cluster operation"), not the raw AWS keyword.
func TestCR273_Item2_DocDB_ColorReadsStatusKey(t *testing.T) {
	td := resource.FindResourceType("dbc")
	if td == nil {
		t.Fatal("resource type 'dbc' not registered")
	}
	r := resource.Resource{
		ID:     "cluster-test-1",
		Name:   "acme-docdb-cluster",
		Fields: map[string]string{"status": "failed: cluster operation"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("dbc Color(status=%q) = %v, want ColorBroken (%v): cluster-level failure state must not appear healthy in issue badges or ctrl+z filtering",
			r.Fields["status"], got, resource.ColorBroken)
	}
}

// TestCR273_Item2_DDB_ColorReadsStatusKey pins the post-spec-rewrite contract:
// Fields["status"] carries the §4 phrase (e.g. "kms key inaccessible"), not the
// raw AWS enum. Color must classify the phrase as Broken. Spec: docs/resources/ddb.md §4.
func TestCR273_Item2_DDB_ColorReadsStatusKey(t *testing.T) {
	td := resource.FindResourceType("ddb")
	if td == nil {
		t.Fatal("resource type 'ddb' not registered")
	}
	r := resource.Resource{
		ID:     "table-test-1",
		Name:   "acme-events-table",
		Fields: map[string]string{"status": "kms key inaccessible"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("ddb Color(status=%q) = %v, want ColorBroken (%v): §4 phrase for INACCESSIBLE_ENCRYPTION_CREDENTIALS must classify as Broken",
			r.Fields["status"], got, resource.ColorBroken)
	}
}

// =============================================================================
// Item 3: CloudWatch Alarms — Color reads wrong field key ("state_value" vs "state")
// =============================================================================
//
// The fetcher at internal/aws/alarm.go populates Fields["state"] (stateValue).
// The color resolver at types_monitoring.go:20 reads Fields["state_value"].
// A resource built from real fetcher output has "state" populated but not
// "state_value", so the switch never matches and the alarm defaults to ColorHealthy
// regardless of its actual alarm state.

// TestCR273_Item3_CloudWatchAlarm_ALARM_ReturnsBroken asserts that an alarm
// resource with Fields["state"]="ALARM" (as populated by the fetcher) is
// classified as ColorBroken.
//
// Currently FAILS: Color reads Fields["state_value"] instead of Fields["state"],
// so the switch never matches "ALARM" and returns ColorHealthy.
func TestCR273_Item3_CloudWatchAlarm_ALARM_ReturnsBroken(t *testing.T) {
	td := resource.FindResourceType("alarm")
	if td == nil {
		t.Fatal("resource type 'alarm' not registered")
	}
	// Use the fetcher's real field key ("state"), not the wrong resolver key ("state_value").
	r := resource.Resource{
		ID:     "acme-cpu-high-alarm",
		Name:   "acme-cpu-high-alarm",
		Fields: map[string]string{
			"alarm_name": "acme-cpu-high-alarm",
			"state":      "ALARM",
		},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("alarm Color(Fields[\"state\"]=\"ALARM\") = %v, want ColorBroken (%v): Color reads Fields[\"state_value\"] but fetcher populates Fields[\"state\"] — alarm is hidden from issue badges and ctrl+z filtering",
			got, resource.ColorBroken)
	}
}

// TestCR273_Item3_CloudWatchAlarm_INSUFFICIENT_DATA_ReturnsWarning asserts that
// Fields["state"]="INSUFFICIENT_DATA" is classified as ColorWarning.
//
// Currently FAILS: same key mismatch as above — defaults to ColorHealthy.
func TestCR273_Item3_CloudWatchAlarm_INSUFFICIENT_DATA_ReturnsWarning(t *testing.T) {
	td := resource.FindResourceType("alarm")
	if td == nil {
		t.Fatal("resource type 'alarm' not registered")
	}
	r := resource.Resource{
		ID:     "acme-disk-alarm",
		Name:   "acme-disk-alarm",
		Fields: map[string]string{
			"alarm_name": "acme-disk-alarm",
			"state":      "INSUFFICIENT_DATA",
		},
	}
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("alarm Color(Fields[\"state\"]=\"INSUFFICIENT_DATA\") = %v, want ColorWarning (%v): Color reads wrong key \"state_value\" instead of \"state\"",
			got, resource.ColorWarning)
	}
}

// =============================================================================
// Item 5: CloudFormation — IMPORT_ROLLBACK_COMPLETE falls through to ColorHealthy
// =============================================================================
//
// cfnStackColor at types_cicd.go:14 explicitly handles ROLLBACK_COMPLETE and
// UPDATE_ROLLBACK_COMPLETE as ColorBroken. IMPORT_ROLLBACK_COMPLETE (a terminal
// AWS status for a failed import that rolled back) is not listed and does not
// match the _FAILED suffix check, so it falls through to ColorHealthy. A stack
// that failed an import and rolled back appears healthy in issue badges and
// ctrl+z filtering.

// TestCR273_Item5_CFN_IMPORT_ROLLBACK_COMPLETE_ReturnsBroken asserts that a
// CloudFormation stack with status IMPORT_ROLLBACK_COMPLETE is classified as
// ColorBroken.
//
// Currently FAILS: cfnStackColor has no case for IMPORT_ROLLBACK_COMPLETE —
// it does not end in _FAILED and is not in the explicit case list, so it
// falls through to ColorHealthy.
func TestCR273_Item5_CFN_IMPORT_ROLLBACK_COMPLETE_ReturnsBroken(t *testing.T) {
	td := resource.FindResourceType("cfn")
	if td == nil {
		t.Fatal("resource type 'cfn' not registered")
	}
	r := resource.Resource{
		ID:     "acme-import-stack",
		Name:   "acme-import-stack",
		Fields: map[string]string{"status": "IMPORT_ROLLBACK_COMPLETE"},
	}
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("cfn Color(status=IMPORT_ROLLBACK_COMPLETE) = %v, want ColorBroken (%v): a rolled-back import is a terminal failure — stack must not appear healthy; add IMPORT_ROLLBACK_COMPLETE to the ColorBroken case in cfnStackColor",
			got, resource.ColorBroken)
	}
}

// TestCR273_Item5_CFN_IMPORT_ROLLBACK_COMPLETE_ExplicitCase_NotSuffix confirms
// that the fix must be an explicit case, not a suffix trick. The status ends in
// _COMPLETE which would incorrectly match a hypothetical _COMPLETE=healthy rule.
// This guard ensures the fix adds IMPORT_ROLLBACK_COMPLETE to the Broken cases.
func TestCR273_Item5_CFN_IMPORT_ROLLBACK_COMPLETE_ExplicitCase_NotSuffix(t *testing.T) {
	td := resource.FindResourceType("cfn")
	if td == nil {
		t.Fatal("resource type 'cfn' not registered")
	}
	// IMPORT_COMPLETE (successful import) must stay ColorHealthy.
	// IMPORT_ROLLBACK_COMPLETE (failed import rolled back) must be ColorBroken.
	// These two statuses have the same suffix (_COMPLETE) but opposite colors,
	// so the fix cannot rely on suffix matching alone.
	healthy := resource.Resource{
		Fields: map[string]string{"status": "IMPORT_COMPLETE"},
	}
	broken := resource.Resource{
		Fields: map[string]string{"status": "IMPORT_ROLLBACK_COMPLETE"},
	}
	gotHealthy := td.Color(healthy)
	gotBroken := td.Color(broken)
	if gotHealthy != resource.ColorHealthy {
		t.Errorf("cfn Color(IMPORT_COMPLETE) = %v, want ColorHealthy (%v): successful import must remain healthy", gotHealthy, resource.ColorHealthy)
	}
	if gotBroken != resource.ColorBroken {
		t.Errorf("cfn Color(IMPORT_ROLLBACK_COMPLETE) = %v, want ColorBroken (%v): rolled-back import is broken — must be distinguishable from IMPORT_COMPLETE", gotBroken, resource.ColorBroken)
	}
}

// Items 7, 8, 9, 10, 11, 15 — all marked "Addressed in commits aae6860..2e831e1"
// in the CodeRabbit review. Code inspection confirms:
//
//   Item 7:  probeResources cleared on refresh — app_handlers_navigate.go:333
//            m.ProbeResources = make(map[string][]resource.Resource)
//
//   Item 8:  Per-type Ctrl+R sets menu badge via handleEnrichmentChecked →
//            menu.SetIssues(unified, ...) with no only-increase guard (fixed).
//
//   Item 9:  Title truncation uses m.enrichmentTruncated, not pagination.IsTruncated
//            — resourcelist_helpers.go:357.
//
//   Item 10: applyFilter recalculates m.issueCount from row colors but
//            m.enrichmentIssueCount is kept separate and takes priority in title
//            display — resourcelist_helpers.go:351-352.
//
//   Item 11: isVisibleUnderIssueFilter returns true when m.issueTruncated[shortName]
//            and !td.AlwaysHealthy — mainmenu.go:570-573.
//
//   Item 15: Per-resource API errors set truncated=true in per-resource enrichers.
//
// Tests for these items would PASS today; no failing pins written.

// Items 16, 17 — refactor-only (typed constants replacing string literals).
// Behavior is already correct; no failing tests possible without production-code
// behavior change. Skipped per task specification.

// =============================================================================
// Item 18: Trivial Color causes ghost entries in ctrl+z filter
// =============================================================================
//
// The ctrl+z attention-only filter in the main menu shows a type when its
// truncated-zero count is a "lower bound" (may have issues on unseen pages).
// Per docs/attention-signals.md every registered type has at least a Wave 1
// or Wave 2 signal, so the AlwaysHealthy escape hatch has been removed.
//
// A type whose Color func returns ColorHealthy for every realistic probe is
// a bug: the type can never flag an issue from Wave 1, so its Wave 1 cell in
// the doc must be genuinely empty — and if it IS empty in the doc, the type
// still needs a Wave 2 enricher registered (no-op or real) to make the
// classification contract explicit. This test flags Color funcs that are
// silently trivial across the realistic probe set.

// TestCR273_Item18_TrivialColor_MustClassify iterates registered
// ResourceTypeDefs that have a statusField in typeContracts and verifies
// their Color func returns at least one non-Healthy result across the
// realistic probe set.
//
// Types with statusField="" are skipped — they either have no lifecycle
// state (config-only) or use multi-field Color checks tested by dedicated
// per-type Color tests (qa_*_color_test.go). The doc-grounded test
// TestAttentionSignalsDoc ensures Wave 1/Wave 2 alignment for ALL types.
func TestCR273_Item18_TrivialColor_MustClassify(t *testing.T) {
	statusFieldTypes := make(map[string]string)
	for _, c := range typeContracts {
		if c.statusField != "" {
			statusFieldTypes[c.shortName] = c.statusField
		}
	}

	probeStatuses := []string{
		"", "unknown",
		"running", "failed", "available", "stopped", "deleted", "pending",
		"creating", "updating", "deleting", "modifying", "error", "impaired",
		"terminated", "inactive", "attaching", "detaching", "provisioning",
		"ACTIVE", "FAILED", "CREATING", "UPDATING", "DELETING", "INACTIVE",
		"RUNNING", "STOPPED", "PENDING", "TERMINATED",
		"ALARM", "INSUFFICIENT_DATA", "OK",
		"Active", "Inactive", "Pending", "Failed",
		"Red", "Yellow", "Grey", "Green",
		"PROVISIONING", "ACTIVATING", "DEACTIVATING", "STOPPING", "DEPROVISIONING",
		"Enabled", "Disabled", "PendingDeletion", "PendingImport", "Unavailable",
		"InProgress", "Deployed",
		"Delete in progress",
		"CREATE_FAILED", "DELETE_FAILED", "DEGRADED",
		"PendingAcceptance", "Rejected", "Expired", "Partial",
		"ROLLBACK_COMPLETE", "UPDATE_ROLLBACK_COMPLETE", "IMPORT_ROLLBACK_COMPLETE",
		"INACCESSIBLE_ENCRYPTION_CREDENTIALS", "ARCHIVED", "ARCHIVING",
		"storage-full", "restore-error", "restore-failed", "incompatible-network",
		"EXPIRED", "REVOKED", "VALIDATION_TIMED_OUT",
		"rebooting cluster nodes",
		"false", "true", "0", "1", "No",
		// dbc phrase-based statuses (see docs/resources/dbc.md §4).
		"failed: cluster operation", "encryption key unreachable",
		"parameter group incompatible", "no writer: reads only",
		"modifying: in progress", "delete-protection off",
		"not encrypted at rest", "no automated backups",
		// ses phrase-based statuses (Color reads Fields["status"] derived phrase).
		"verification failed", "verify: temp failure", "verification not started",
		"pending verification", "sending disabled",
	}

	fieldKeys := []string{
		"status", "state", "state_value", "lifecycle",
		"instance_status", "last_status", "cluster_status", "node_group_status",
		"db_instance_status", "table_status",
		"running_count", "desired_count",
		"life_cycle_state",
		"key_state",
		"health",
		"is_logging", "log_file_validation_enabled", "latest_delivery_error",
		"actions_count",
		"wide_open", "dangerous_open_count",
		"rotation_enabled",
		"record_count",
		"subscription_arn",
		"has_console_password",
		"sending_enabled", "verification_status",
		"stream_status",
	}

	var buggy []string

	for _, td := range resource.AllResourceTypes() {
		if td.Color == nil {
			continue
		}
		if _, hasStatusField := statusFieldTypes[td.ShortName]; !hasStatusField {
			continue
		}

		seenNonHealthy := false
		for _, s := range probeStatuses {
			fields := make(map[string]string, len(fieldKeys))
			for _, k := range fieldKeys {
				fields[k] = s
			}
			r := resource.Resource{
				Fields: fields,
			}
			c := td.Color(r)
			if c != resource.ColorHealthy {
				seenNonHealthy = true
				break
			}
		}

		if !seenNonHealthy {
			buggy = append(buggy, td.ShortName)
		}
	}

	if len(buggy) > 0 {
		t.Errorf(
			"the following status-field types have a trivial Color func:\n  %v\n\n"+
				"Each type has statusField set in typeContracts but Color returns only "+
				"ColorHealthy across all probes. Fix the Color func or update typeContracts.",
			buggy,
		)
	}
}
