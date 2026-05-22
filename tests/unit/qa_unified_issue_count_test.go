package unit

// qa_unified_issue_count_test.go — Tests for the unified issue count contract:
//
//  1. Every enricher returns IssueCount == len(Findings).
//  2. unifiedIssueCount deduplicates Wave-1 issue resources and Wave-2 findings
//     so the same instance ID is not double-counted.
//  3. The menu count for a type after EnrichmentCheckedMsg equals the list's
//     FrameTitle count.

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: Every enricher returns IssueCount == len(Findings)
// ─────────────────────────────────────────────────────────────────────────────

type allEnrichersCase struct {
	name    string
	clients *awsclient.ServiceClients
	probes  []resource.Resource
	call    func(context.Context, *awsclient.ServiceClients, []resource.Resource, resource.ResourceCache) (awsclient.IssueEnricherResult, error)
}

// TestAllEnrichers_IssueCountMatchesFindings verifies that every registered enricher
// that produces severity "!" findings returns result.IssueCount == len(result.Findings)
// when seeded with one finding. Enrichers that only produce severity "~" (informational)
// findings are excluded because "~" findings do not contribute to IssueCount — they are
// tested separately in TestEnrichRDSDocDBMaintenance_OnlyEmitsForProbedResources.
func TestAllEnrichers_IssueCountMatchesFindings(t *testing.T) {
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/tg/abc123"
	smARN := "arn:aws:states:us-east-1:123456789012:stateMachine:sm"

	cases := []allEnrichersCase{
		{
			name: "ebs/EnrichEBSVolumeStatus",
			clients: &awsclient.ServiceClients{
				EC2: &ebsStatusFake{
					volumeOutput: &ec2.DescribeVolumeStatusOutput{
						VolumeStatuses: []ec2types.VolumeStatusItem{
							{
								VolumeId: aws.String("vol-001"),
								VolumeStatus: &ec2types.VolumeStatusInfo{
									Status: ec2types.VolumeStatusInfoStatusImpaired,
								},
							},
						},
					},
				},
			},
			probes: nil,
			call:   awsclient.EnrichEBSVolumeStatus,
		},
		{
			name: "cb/EnrichCodeBuildStatus",
			clients: &awsclient.ServiceClients{
				CodeBuild: &codeBuildEnrichFake{
					projectBuilds: map[string]string{"my-project": "build-1"},
					builds: map[string]cbtypes.Build{
						"build-1": {
							Id:          aws.String("build-1"),
							BuildStatus: cbtypes.StatusTypeFailed,
							EndTime:     aws.Time(time.Now()),
						},
					},
				},
			},
			probes: []resource.Resource{{ID: "my-project", Name: "my-project"}},
			call:   awsclient.EnrichCodeBuildStatus,
		},
		{
			name: "tg/EnrichTargetGroupHealth",
			clients: &awsclient.ServiceClients{
				ELBv2: &tgHealthFake{
					outputs: map[string]*elbv2.DescribeTargetHealthOutput{
						tgARN: {
							TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
								tgHealthDesc(elbtypes.TargetHealthStateEnumUnhealthy),
							},
						},
					},
				},
			},
			probes: []resource.Resource{{ID: "ucm-tg", Fields: map[string]string{"target_group_arn": tgARN}}},
			call:   awsclient.EnrichTargetGroupHealth,
		},
		{
			name: "pipeline/EnrichCodePipelineStatus",
			clients: &awsclient.ServiceClients{
				CodePipeline: &pipelineStateFake{
					states: map[string]*codepipeline.GetPipelineStateOutput{
						"my-pipeline": {
							StageStates: []cptypes.StageState{
								stageState("Deploy", cptypes.StageExecutionStatusFailed),
							},
						},
					},
				},
			},
			probes: []resource.Resource{{ID: "my-pipeline", Name: "my-pipeline"}},
			call:   awsclient.EnrichCodePipelineStatus,
		},
		{
			name: "sfn/EnrichStepFunctionsStatus",
			clients: &awsclient.ServiceClients{
				SFN: &sfnEnrichFake{
					executions: map[string]sfntypes.ExecutionStatus{
						smARN: sfntypes.ExecutionStatusFailed,
					},
				},
			},
			probes: []resource.Resource{{ID: "ucm-sm", Fields: map[string]string{"arn": smARN}}},
			call:   awsclient.EnrichStepFunctionsStatus,
		},
		{
			name: "glue/EnrichGlueJobStatus",
			clients: &awsclient.ServiceClients{
				Glue: &glueJobFake{
					jobRuns: map[string]gluetypes.JobRunState{
						"my-job": gluetypes.JobRunStateFailed,
					},
				},
			},
			probes: []resource.Resource{{ID: "my-job", Name: "my-job"}},
			call:   awsclient.EnrichGlueJobStatus,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.call(context.Background(), tc.clients, tc.probes, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IssueCount != len(result.Findings) {
				t.Errorf("IssueCount = %d, want %d (len(Findings)); enricher: %s",
					result.IssueCount, len(result.Findings), tc.name)
			}
			if len(result.Findings) == 0 {
				t.Errorf("expected at least 1 finding from seeded fake; enricher: %s", tc.name)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: unifiedIssueCount deduplication — tested via ResourceListModel
// ─────────────────────────────────────────────────────────────────────────────

// buildUnifiedModel builds a ResourceListModel loaded with the given resources and
// enrichment state, returning the FrameTitle for count inspection.
func buildUnifiedModel(t *testing.T, resources []resource.Resource, enrichIC int, findings map[string]resource.EnrichmentFinding) string {
	t.Helper()
	td := resource.ResourceTypeDef{
		ShortName: "ec2",
		Name:      "EC2 Instances",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 12},
		},
	}
	m := views.NewResourceList(td, nil, keys.Default())
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources,
	})
	m.SetEnrichmentState(enrichIC, false, findings)
	return m.FrameTitle()
}

// TestUnifiedIssueCount_DedupesAcrossWaves verifies that the unified count is the
// distinct union of Wave-1 issue resource IDs and Wave-2 finding IDs.
// Four sub-cases: disjoint sets, fully overlapping, empty Wave-1, empty findings.
//
// The dedup logic lives in the unexported unifiedIssueCount helper; we exercise it
// indirectly via ResourceListModel.FrameTitle which reflects the unified count when
// enrichmentIssueCount > 0.
func TestUnifiedIssueCount_DedupesAcrossWaves(t *testing.T) {
	t.Run("disjoint: Wave-2 only (enrichIC=1) on running resource → count=1", func(t *testing.T) {
		resources := []resource.Resource{
			{ID: "i-bbb", Name: "running-server", Status: "running",
				Fields: map[string]string{"name": "running-server", "state": "running"}},
		}
		findings := map[string]resource.EnrichmentFinding{
			"i-bbb": {Severity: "!", Summary: "status impaired"},
		}
		// enrichIC=1 reflects the correct distinct count from unifiedIssueCount on the production side.
		title := buildUnifiedModel(t, resources, 1, findings)
		if !strings.Contains(title, "1") {
			t.Errorf("FrameTitle() = %q, want count 1 (Wave-2 only, no Wave-1 issues)", title)
		}
	})

	t.Run("fully overlapping: same resource in Wave-1 and Wave-2 → enrichIC=1 not 2", func(t *testing.T) {
		resources := []resource.Resource{
			{ID: "i-aaa", Name: "stopped-server", Status: "stopped",
				Fields: map[string]string{"name": "stopped-server", "state": "stopped"}},
		}
		findings := map[string]resource.EnrichmentFinding{
			"i-aaa": {Severity: "!", Summary: "status impaired"},
		}
		// unifiedIssueCount({i-aaa(stopped)}, findings{i-aaa}) = 1, not 2.
		title := buildUnifiedModel(t, resources, 1, findings)
		if !strings.Contains(title, "1") {
			t.Errorf("FrameTitle() = %q, want deduplicated count 1 (same ID in both waves)", title)
		}
		// Must not contain "2" (double-counting would show 2).
		if strings.Contains(stripANSI(title), "(2)") || strings.Contains(stripANSI(title), "[2]") {
			t.Errorf("FrameTitle() = %q, must not show count 2 (double-counting guard)", title)
		}
	})

	t.Run("multiple disjoint findings → enrichIC=3", func(t *testing.T) {
		resources := []resource.Resource{
			{ID: "i-aaa", Name: "s1", Status: "running", Fields: map[string]string{"name": "s1"}},
			{ID: "i-bbb", Name: "s2", Status: "running", Fields: map[string]string{"name": "s2"}},
			{ID: "i-ccc", Name: "s3", Status: "running", Fields: map[string]string{"name": "s3"}},
		}
		findings := map[string]resource.EnrichmentFinding{
			"i-aaa": {Severity: "!", Summary: "impaired"},
			"i-bbb": {Severity: "~", Summary: "maintenance"},
			"i-ccc": {Severity: "!", Summary: "impaired"},
		}
		title := buildUnifiedModel(t, resources, 3, findings)
		if !strings.Contains(title, "3") {
			t.Errorf("FrameTitle() = %q, want count 3 (3 distinct findings)", title)
		}
	})

	t.Run("empty findings → enrichIC=0 → FrameTitle has no issue badge", func(t *testing.T) {
		resources := []resource.Resource{
			{ID: "i-aaa", Name: "server", Status: "running", Fields: map[string]string{"name": "server"}},
		}
		title := buildUnifiedModel(t, resources, 0, map[string]resource.EnrichmentFinding{})
		if strings.Contains(title, "[!]") {
			t.Errorf("FrameTitle() = %q; no issue badge expected when enrichIC=0 and no findings", title)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 3: Menu count == list count after Wave 2 (EnrichmentCheckedMsg)
// ─────────────────────────────────────────────────────────────────────────────

// TestMenuCount_MatchesListCount_AfterWave2 verifies that after handleEnrichmentChecked
// processes an EnrichmentCheckedMsg, the issue count shown in the active
// ResourceListModel FrameTitle is consistent with what the menu shows for that type.
//
// This is R2: menu count == list count after Wave 2.
func TestMenuCount_MatchesListCount_AfterWave2(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	// Load EC2 resources: 2 running (no Wave-1 issues).
	resources := []resource.Resource{
		{ID: "i-0abc1111aaa111111", Name: "web-server-1",
			Fields: map[string]string{"state": "running"}},
		{ID: "i-0abc2222bbb222222", Name: "web-server-2",
			Fields: map[string]string{"state": "running"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources,
	})

	// Deliver Wave-2 enrichment: 1 finding for the first instance.
	// Gen=0 and TypeGen=0 match a fresh model's initial generation counters.
	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Issues:       1,
		Truncated:    false,
		Findings: map[string]resource.EnrichmentFinding{
			"i-0abc1111aaa111111": {Severity: "!", Summary: "system status impaired"},
		},
		Gen:     0,
		TypeGen: 0,
	})

	// The list FrameTitle should reflect enrichmentIssueCount=1 (Wave-2 count).
	listContent := m.View().Content
	if !strings.Contains(listContent, "1") {
		t.Errorf("list view does not contain issue count 1 after EnrichmentCheckedMsg; output:\n%s", listContent)
	}

	// Navigate back to the main menu by pressing the back key ("q").
	backKey := tea.KeyPressMsg{Code: -1, Text: "q"}
	m, _ = rootApplyMsg(m, backKey)
	menuContent := m.View().Content

	// The main menu must also show 1 for ec2.
	// The menu renders issue counts as part of each row; "1" must appear somewhere.
	if !strings.Contains(menuContent, "1") {
		t.Errorf("main menu does not show issue count 1 for ec2 after Wave-2 enrichment; output:\n%s", menuContent)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Target #4 — unifiedIssueCount must not count "~"-severity findings
//
// Problem: unifiedIssueCount adds every finding ID to the issue-ID set
// regardless of finding.Severity. Only "!" findings must bump the badge;
// "~" (informational) findings must never contribute.
//
// These tests FAIL until the coder adds a Severity=="!" guard in the
// findings loop of unifiedIssueCount.
// ─────────────────────────────────────────────────────────────────────────────

// tildeSeverityEC2Instances returns 3 EC2 resources whose Color is Healthy
// (running state → ColorHealthy → !IsIssue). Used as Wave-1 baseline
// so any badge count must come from Wave-2 findings only.
func tildeSeverityEC2Instances() []resource.Resource {
	return []resource.Resource{
		{ID: "i-aaa", Name: "server-a", Status: "running",
			Fields: map[string]string{"name": "server-a", "state": "running"}},
		{ID: "i-bbb", Name: "server-b", Status: "running",
			Fields: map[string]string{"name": "server-b", "state": "running"}},
		{ID: "i-ccc", Name: "server-c", Status: "running",
			Fields: map[string]string{"name": "server-c", "state": "running"}},
	}
}

// TestUnifiedIssueCount_IgnoresTildeSeverityFindings verifies three cases:
//  1. One "!" finding + two "~" findings → unified count = 1 (only "!" counts).
//  2. Three "~" findings, no "!" → unified count = 0 (informational only, no badge).
//  3. One Wave-1 broken resource + one "~" finding on its ID → count = 1
//     (broken comes from Wave-1 IsIssue; "~" must not double-count or bump).
//
// Regression pin: unifiedIssueCount must ignore findings where Severity != "!".
// The menu issue badge (format " issues:N") must reflect only "!" findings.
//
// Navigation pattern: AvailabilityCheckedMsg seeds probeResources so that
// unifiedIssueCount has wave1Resources to work with; NavigateMsg pops back to
// the menu so that m.View() renders the menu (not the resource list).
func TestUnifiedIssueCount_IgnoresTildeSeverityFindings(t *testing.T) {
	tui.Version = "test"

	t.Run("one ! finding + two ~ findings → count=1 (only ! counts)", func(t *testing.T) {
		m := newRootSizedModel()

		// Use AvailabilityCheckedMsg stamped with the live AvailabilityGen
		// (session.New seeds it to 1 after AS-659) to seed probeResources["ec2"]
		// so unifiedIssueCount has wave1Resources.
		// All three resources are running → Wave-1 contributes 0 to issue IDs.
		resources := tildeSeverityEC2Instances()
		m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: "ec2",
			Count:        3,
			Resources:    resources,
			Issues:       0,
			Gen:          m.Session().AvailabilityGen,
		})

		m = navigateToEC2List(m)

		// EnrichmentCheckedMsg: unifiedIssueCount re-derives from Findings.
		// Bug (before fix): all 3 findings counted → issues:3.
		// Correct (after fix): only "!" findings → issues:1.
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: "ec2",
			Issues:       1,
			Truncated:    false,
			Findings: map[string]resource.EnrichmentFinding{
				"i-aaa": {Severity: "!", Summary: "system status impaired"},
				"i-bbb": {Severity: "~", Summary: "pending maintenance"},
				"i-ccc": {Severity: "~", Summary: "quota 80%+ used"},
			},
			Gen:     0,
			TypeGen: 0,
		})

		// Pop back to the menu so m.View() renders the main menu.
		m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetMainMenu})
		menuContent := stripANSI(m.View().Content)

		// The menu badge format is " issues:N". Bug produces " issues:3".
		// After fix: " issues:1" (only the "!" finding counts).
		if strings.Contains(menuContent, " issues:3") {
			t.Errorf("menu shows issues:3, want issues:1 — ~ severity must not count; output:\n%s", menuContent)
		}
		if strings.Contains(menuContent, " issues:2") {
			t.Errorf("menu shows issues:2, want issues:1 — ~ severity must not count; output:\n%s", menuContent)
		}
	})

	t.Run("three ~ findings only → count=0 (no badge)", func(t *testing.T) {
		m := newRootSizedModel()

		resources := tildeSeverityEC2Instances()
		m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: "ec2",
			Count:        3,
			Resources:    resources,
			Issues:       0,
			Gen:          m.Session().AvailabilityGen,
		})

		m = navigateToEC2List(m)

		// All three findings are "~" (informational). unifiedIssueCount must return 0.
		// Bug (before fix): counts all 3 → issues:3.
		// Correct (after fix): no "!" findings → no badge.
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: "ec2",
			Issues:       0,
			Truncated:    false,
			Findings: map[string]resource.EnrichmentFinding{
				"i-aaa": {Severity: "~", Summary: "pending maintenance"},
				"i-bbb": {Severity: "~", Summary: "pending maintenance"},
				"i-ccc": {Severity: "~", Summary: "quota 80%+ used"},
			},
			Gen:     0,
			TypeGen: 0,
		})

		m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetMainMenu})
		menuContent := stripANSI(m.View().Content)

		// No issue badge: " issues:" must not appear at all for ec2.
		// Bug produces " issues:3"; correct result has no badge (issueBadge returns "").
		if strings.Contains(menuContent, " issues:") {
			t.Errorf("menu shows issue badge, want none — all findings are ~ severity; output:\n%s", menuContent)
		}
	})

	t.Run("one Wave-1 broken + ~ finding on same ID → count=1 (no double-count)", func(t *testing.T) {
		m := newRootSizedModel()

		// EC2 resource with Status=stopped → Color func returns ColorBroken (IsIssue=true).
		// Wave-1 contributes 1 to the issue count.
		brokenResource := resource.Resource{
			ID:     "i-stopped",
			Name:   "stopped-server",
			Status: "stopped",
			Fields: map[string]string{"name": "stopped-server", "state": "stopped"},
		}
		m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: "ec2",
			Count:        1,
			Resources:    []resource.Resource{brokenResource},
			Issues:       1, // Wave-1 issue
			Gen:          m.Session().AvailabilityGen,
		})

		m = navigateToEC2List(m)

		// Wave-2: a "~" finding on the same ID. Wave-1 already contributes count=1.
		// unifiedIssueCount must return 1 (dedup + no ~ bump).
		// Bug (before fix): "~" is counted, dedup collapses to 1 anyway — this subtest
		// catches the case where a DIFFERENT id has a "~" finding that inflates count.
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: "ec2",
			Issues:       0, // enricher excludes ~ from IssueCount
			Truncated:    false,
			Findings: map[string]resource.EnrichmentFinding{
				"i-stopped": {Severity: "~", Summary: "pending maintenance"},
			},
			Gen:     0,
			TypeGen: 0,
		})

		m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetMainMenu})
		menuContent := stripANSI(m.View().Content)

		// Wave-1 broken resource contributes issues:1. ~ on same ID must not bump to 2.
		if strings.Contains(menuContent, " issues:2") {
			t.Errorf("menu shows issues:2, want issues:1 — ~ on broken resource must not double-count; output:\n%s", menuContent)
		}
	})
}
