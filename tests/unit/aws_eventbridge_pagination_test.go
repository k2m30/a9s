package unit

// aws_eventbridge_pagination_test.go — Failing tests for EnrichEventBridgeRuleTargets pagination.
//
// These tests document the REQUIRED behavior after the coder implements
// pagination for ListTargetsByRule.
//
// ListTargetsByRule uses the NextToken pagination pattern (up to 100 targets
// per page). After full pagination, counts are exact. If pagination exceeds
// PerParentPageCap = 10 pages per rule, the count is capped and marked "1000+".
// DLQ checks must be applied to targets on ALL pages, not just the first.
//
// Contract assertions:
//   - 2 pages (100 + 50 targets) → Fields["target_count"] == "150"; called twice
//   - NextToken always non-nil (huge rule) → capped at PerParentPageCap; "1000+"
//   - DLQ check applied to targets on page 2 even when page 1 has DLQ configured
//   - ENABLED rule, all pages return 0 targets → "enabled rule has no targets" finding

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Pagination-aware fake for EventBridge ListTargetsByRule
// ---------------------------------------------------------------------------

// ebPaginatedFake implements EventBridgeAPI and serves paginated
// ListTargetsByRule responses. Pages are keyed by rule name.
type ebPaginatedFake struct {
	awsclient.EventBridgeAPI

	// pages maps ruleName → ordered pages of ListTargetsByRuleOutput.
	pages map[string][]*eventbridge.ListTargetsByRuleOutput

	// callCounts tracks how many times ListTargetsByRule was called per rule.
	callCounts map[string]int
}

func newEBPaginatedFake() *ebPaginatedFake {
	return &ebPaginatedFake{
		pages:      make(map[string][]*eventbridge.ListTargetsByRuleOutput),
		callCounts: make(map[string]int),
	}
}

func (f *ebPaginatedFake) ListTargetsByRule(
	_ context.Context,
	in *eventbridge.ListTargetsByRuleInput,
	_ ...func(*eventbridge.Options),
) (*eventbridge.ListTargetsByRuleOutput, error) {
	rule := ""
	if in != nil && in.Rule != nil {
		rule = *in.Rule
	}
	idx := f.callCounts[rule]
	f.callCounts[rule] = idx + 1

	pages := f.pages[rule]
	if idx >= len(pages) {
		return &eventbridge.ListTargetsByRuleOutput{
			Targets:   []ebtypes.Target{},
			NextToken: nil,
		}, nil
	}
	return pages[idx], nil
}

// Compile-time check: ebPaginatedFake satisfies EventBridgeAPI.
var _ awsclient.EventBridgeAPI = (*ebPaginatedFake)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeEBTargetsWithDLQ builds n targets, each with a DeadLetterConfig.
func makeEBTargetsWithDLQ(n int) []ebtypes.Target {
	targets := make([]ebtypes.Target, n)
	for i := range targets {
		targets[i] = ebtypes.Target{
			Id:  aws.String(fmt.Sprintf("target-dlq-%d", i)),
			Arn: aws.String(fmt.Sprintf("arn:aws:lambda:us-east-1:123456789012:function:fn-%d", i)),
			DeadLetterConfig: &ebtypes.DeadLetterConfig{
				Arn: aws.String(fmt.Sprintf("arn:aws:sqs:us-east-1:123456789012:dlq-%d", i)),
			},
		}
	}
	return targets
}

// makeEBTargetsNoDLQ builds n targets without a DeadLetterConfig.
func makeEBTargetsNoDLQ(n int) []ebtypes.Target {
	targets := make([]ebtypes.Target, n)
	for i := range targets {
		targets[i] = ebtypes.Target{
			Id:               aws.String(fmt.Sprintf("target-nodlq-%d", i)),
			Arn:              aws.String(fmt.Sprintf("arn:aws:lambda:us-east-1:123456789012:function:no-dlq-%d", i)),
			DeadLetterConfig: nil,
		}
	}
	return targets
}

// ebRuleResources builds a slice of EventBridge rule Resource stubs.
func ebRuleResources(rules ...struct {
	name  string
	state string
	bus   string
}) []resource.Resource {
	res := make([]resource.Resource, 0, len(rules))
	for _, r := range rules {
		res = append(res, resource.Resource{
			ID:   r.name,
			Name: r.name,
			Fields: map[string]string{
				"name":      r.name,
				"state":     r.state,
				"event_bus": r.bus,
				"status":    r.state,
			},
		})
	}
	return res
}

// ---------------------------------------------------------------------------
// Test: ListTargetsByRule pagination — 2 pages (100 + 50 targets)
// ---------------------------------------------------------------------------

// TestEnrichEventBridgeRule_PaginatesTargets verifies that the enricher
// follows NextToken across two pages and writes the total count (150) to
// Fields["target_count"].
func TestEnrichEventBridgeRule_PaginatesTargets(t *testing.T) {
	const ruleName = "daily-backup-rule"

	fake := newEBPaginatedFake()

	// Page 1: 100 targets with DLQ, NextToken="t1"
	// Page 2: 50 targets with DLQ, NextToken nil (last page)
	fake.pages[ruleName] = []*eventbridge.ListTargetsByRuleOutput{
		{
			Targets:   makeEBTargetsWithDLQ(100),
			NextToken: aws.String("t1"),
		},
		{
			Targets:   makeEBTargetsWithDLQ(50),
			NextToken: nil,
		},
	}

	rules := ebRuleResources(struct {
		name  string
		state string
		bus   string
	}{ruleName, "ENABLED", "default"})

	clients := &awsclient.ServiceClients{EventBridge: fake}

	result, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), clients, rules, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// target_count must reflect both pages
	updates, ok := result.FieldUpdates[ruleName]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", ruleName)
	}
	wantCount := "150"
	if updates["target_count"] != wantCount {
		t.Errorf("target_count = %q, want %q", updates["target_count"], wantCount)
	}

	// ListTargetsByRule must have been called twice
	calls := fake.callCounts[ruleName]
	if calls != 2 {
		t.Errorf("ListTargetsByRule called %d times, want 2", calls)
	}

	// No findings — all targets have DLQ, rule is ENABLED with targets
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

// ---------------------------------------------------------------------------
// Test: ListTargetsByRule capped at PerParentPageCap → "1000+"
// ---------------------------------------------------------------------------

// TestEnrichEventBridgeRule_CappedAtPerParentPageCap verifies that when
// NextToken is always non-nil (simulating a huge rule), the enricher stops
// after PerParentPageCap pages and writes "1000+" to target_count.
func TestEnrichEventBridgeRule_CappedAtPerParentPageCap(t *testing.T) {
	const ruleName = "huge-rule"

	fake := newEBPaginatedFake()

	// Build PerParentPageCap+2 pages, all with NextToken set, 100 targets/page.
	pages := make([]*eventbridge.ListTargetsByRuleOutput, awsclient.PerParentPageCap+2)
	for i := range pages {
		pages[i] = &eventbridge.ListTargetsByRuleOutput{
			Targets:   makeEBTargetsWithDLQ(100),
			NextToken: aws.String(fmt.Sprintf("token-%d", i+1)),
		}
	}
	fake.pages[ruleName] = pages

	rules := ebRuleResources(struct {
		name  string
		state string
		bus   string
	}{ruleName, "ENABLED", "default"})

	clients := &awsclient.ServiceClients{EventBridge: fake}

	result, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), clients, rules, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ListTargetsByRule must be called exactly PerParentPageCap times
	calls := fake.callCounts[ruleName]
	if calls != awsclient.PerParentPageCap {
		t.Errorf("ListTargetsByRule called %d times, want exactly %d (PerParentPageCap)", calls, awsclient.PerParentPageCap)
	}

	// target_count must carry "+" suffix
	updates, ok := result.FieldUpdates[ruleName]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", ruleName)
	}
	tc := updates["target_count"]
	if !strings.HasSuffix(tc, "+") {
		t.Errorf("target_count = %q, want suffix \"+\" (approximate)", tc)
	}
	wantPrefix := fmt.Sprintf("%d+", awsclient.PerParentPageCap*100)
	if tc != wantPrefix {
		t.Errorf("target_count = %q, want %q", tc, wantPrefix)
	}
}

// ---------------------------------------------------------------------------
// Test: DLQ check applied to targets on all pages
// ---------------------------------------------------------------------------

// TestEnrichEventBridgeRule_DLQCheckAcrossAllPages verifies that targets
// without DeadLetterConfig on page 2 produce a finding even when page 1
// targets all have DLQ configured.
func TestEnrichEventBridgeRule_DLQCheckAcrossAllPages(t *testing.T) {
	const ruleName = "mixed-dlq-rule"

	fake := newEBPaginatedFake()

	// Page 1: all targets have DLQ
	// Page 2: all targets lack DLQ → should trigger findings
	fake.pages[ruleName] = []*eventbridge.ListTargetsByRuleOutput{
		{
			Targets:   makeEBTargetsWithDLQ(2),
			NextToken: aws.String("dlq-token"),
		},
		{
			Targets:   makeEBTargetsNoDLQ(2),
			NextToken: nil,
		},
	}

	rules := ebRuleResources(struct {
		name  string
		state string
		bus   string
	}{ruleName, "ENABLED", "default"})

	clients := &awsclient.ServiceClients{EventBridge: fake}

	result, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), clients, rules, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Findings must exist for the targets-without-DLQ from page 2
	if _, ok := result.Findings[ruleName]; !ok {
		t.Fatalf("expected finding for %q (targets on page 2 lack DLQ), none produced", ruleName)
	}

	// Verify at least one row references the no-DLQ issue
	foundNoDLQ := false
	for _, row := range result.AttentionDetails[ruleName].Rows {
		if strings.Contains(row.Value, "no dead-letter config") {
			foundNoDLQ = true
			break
		}
	}
	if !foundNoDLQ {
		t.Errorf("no row with 'no dead-letter config' in finding rows: %v", result.AttentionDetails[ruleName].Rows)
	}
}

// ---------------------------------------------------------------------------
// Test: ENABLED rule with zero targets across all pages → finding
// ---------------------------------------------------------------------------

// TestEnrichEventBridgeRule_EnabledWithZeroTargetsAcrossPages verifies that
// when all pages return 0 targets and the rule is ENABLED, the
// "enabled rule has no targets" finding is emitted and target_count is "0".
func TestEnrichEventBridgeRule_EnabledWithZeroTargetsAcrossPages(t *testing.T) {
	const ruleName = "orphan-rule"

	fake := newEBPaginatedFake()

	// Single page, 0 targets, no NextToken
	fake.pages[ruleName] = []*eventbridge.ListTargetsByRuleOutput{
		{
			Targets:   []ebtypes.Target{},
			NextToken: nil,
		},
	}

	rules := ebRuleResources(struct {
		name  string
		state string
		bus   string
	}{ruleName, "ENABLED", "default"})

	clients := &awsclient.ServiceClients{EventBridge: fake}

	result, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), clients, rules, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// target_count must be "0"
	updates, ok := result.FieldUpdates[ruleName]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", ruleName)
	}
	if updates["target_count"] != "0" {
		t.Errorf("target_count = %q, want \"0\"", updates["target_count"])
	}

	// Finding "enabled rule has no targets" must be emitted with severity "!"
	f, ok := result.Findings[ruleName]
	if !ok {
		t.Fatalf("expected finding for %q (enabled rule with no targets), none produced", ruleName)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("finding severity = %v, want SevBroken", f.Severity)
	}
	if !strings.Contains(f.Phrase, "no targets") {
		t.Errorf("finding summary %q must contain \"no targets\"", f.Phrase)
	}

	// IssueCount must be 1
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}
