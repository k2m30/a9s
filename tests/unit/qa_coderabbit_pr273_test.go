package unit

// qa_coderabbit_pr273_test.go — regression pins for CodeRabbit PR #273 review findings.
//
// Covered items still pinned here (skipped items were already fixed in prior
// commits, and Items 1/2/3/4/5/14's dedicated pins were retired once the
// per-type qa_<type>_color_test.go files and the qa_coderabbit_pr273_all_types_test.go
// typeContracts table converged on the same coverage):
//
//   Item 6:  Missing Gen==0 bypass in handleEnrichmentChecked — Gen=0 test-injection
//            messages are dropped when enrichmentGen>0 after a profile/region switch.
//   Item 12/13: CodeBuild STOPPED state generates an unwanted finding — intentionally
//            cancelled builds should not be flagged as issues.
//   Item 18: Main-menu ctrl+z false-positive/false-negative coverage, and the
//            TrivialColor gate below.
//
// Skipped items (already fixed in prior commits):
//   Items 1/4:   ng/vpce/tgw stateful-lifecycle Color coverage — now pinned by
//                qa_ng_color_test.go, TestColorRefactor_AllTypes_NonNilColorFunc
//                (qa_resource_color_test.go), and the typeContracts table.
//   Items 2/3:   RDS/DocDB/DynamoDB/CloudWatch alarm Color — fixed in 35a54d4,
//                now pinned by qa_dbi_color_test.go, qa_dbc_color_test.go,
//                qa_alarm_color_test.go, and the typeContracts table (dbi/dbc/ddb/alarm rows).
//   Item 5:      CloudFormation IMPORT_ROLLBACK_COMPLETE — now pinned by the
//                typeContracts table's cfn row (both IMPORT_COMPLETE and
//                IMPORT_ROLLBACK_COMPLETE cases).
//   Items 7/8:   handleRegionSelected / handleProfileSelected — fixed in 2f9a808, aae6860
//   Items 9/10:  isVisibleUnderIssueFilter truncation guard — fixed in aae6860..2e831e1
//   Item 11:     AlwaysHealthy invariant test — already exists in qa_ctrlz_truncated_zero_health_state_test.go
//   Item 14:     Staging EC2 instance SG fixture-data check — retired as a fixture
//                lint concern now covered by the demo-graph gates
//                (qa_demo_pivot_coverage_test.go, qa_demo_related_ids_resolve_test.go).

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
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
	})

	// Wave 2 clean for every enricher-backed type.
	for _, ent := range awsclient.AllWave2() {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: ent.ShortName,
			Truncated:    false,
			Findings:     map[string][]domain.Finding{},
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
	})

	// Wave 2: authoritative zero for every enricher-backed type.
	for _, ent := range awsclient.AllWave2() {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: ent.ShortName,
			Truncated:    false,
			Findings:     map[string][]domain.Finding{},
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
	})

	// Wave 2: for each enricher-backed type, a sub-call errored → Truncated=true,
	// but Findings={} and Issues=0 (no actual issue seen).
	for _, ent := range awsclient.AllWave2() {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: ent.ShortName,
			Truncated:    true,
			Findings:     map[string][]domain.Finding{},
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
// Regression pin (fixed): app_handlers_navigate.go must accept Gen=0 as the
// test-injection sentinel even when enrichmentGen>0.
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
		Truncated:    false,
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     0, // test-injection sentinel
		TypeGen: 0,
	}
	m, _ = rootApplyMsg(m, injected)

	// Step 5: the finding must have been applied. Since the
	// color-findings-conformance wave, colorEC2 derives ColorBroken directly
	// from this Finding (colorFromAnyFinding) — resolveListDecoratorFull no
	// longer emits the "! " glyph prefix for a non-Healthy row (that branch
	// only fires when ResolveColor()==ColorHealthy; see
	// core/app/list_columns.go). The stronger, renderer-agnostic contract
	// is that the row is now a counted issue: it must survive the ctrl+z
	// attention filter, which only the real Wave-2 Finding could cause here
	// (the fixture's Fields carry no lifecycle signal of their own).
	m, _ = rootApplyMsg(m, ctrlZ())
	content := stripANSI(m.View().Content)
	if !strings.Contains(content, "web-server-1") {
		t.Errorf("Gen=0 EnrichmentCheckedMsg must bypass the session guard (enrichmentGen=%d) and apply findings — "+
			"the impaired instance must remain visible under ctrl+z (it is now an issue row):\n%s",
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
// Regression pin (fixed): the switch in EnrichCodeBuildBuilds must skip
// STOPPED alongside SUCCEEDED and IN_PROGRESS.
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
}

// TestCR273_Item13_CodeBuild_STOPPED_WithFailed_OnlyFailedCounted asserts that
// when there are both STOPPED and FAILED builds for different projects, only
// the FAILED project appears in Findings, and IssueCount = 1 (not 2).
//
// Regression pin (fixed): STOPPED must not be counted alongside FAILED.
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
	if len(result.Findings) != 1 {
		t.Errorf("len(Findings) = %d, want 1 (only the FAILED build); STOPPED builds must not inflate the count", len(result.Findings))
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
		// redis phrase-based statuses (colorRedis reads Fields["status"] derived
		// phrase, not the raw ReplicationGroup.Status enum — see docs/resources/redis.md §4).
		"create failed — see events", "creating — new group",
		"modifying — config change", "snapshotting — backup running",
		"deleting — teardown", "multi-AZ without auto-failover",
		"shard 0002 degraded",
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
			// findingsOnlyColorTypes (ec2/lambda/ami/ebs-snap, since the
			// color-findings-conformance wave) have NO raw-field fallback at
			// all — a bare Fields probe can never produce non-Healthy for
			// them. Attach a representative SevBroken Finding so this probe
			// still exercises "can this type ever classify non-Healthy" for
			// its real (Findings-driven) mechanism.
			if findingsOnlyColorTypes[td.ShortName] {
				r.Findings = []domain.Finding{
					{Code: domain.FindingCode(td.ShortName + ".test.probe"), Phrase: s, Severity: domain.SevBroken, Source: "wave1"},
				}
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
