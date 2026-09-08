// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_cap_independence_test.go pins the class rule for a per-parent
// page cap: hitting the cap on one walk says the count from THAT walk is a
// lower bound. It does not make the resource uninspected, so it must never
// suppress a check computed from a different API call, nor discard the
// findings already derived from the pages that did arrive.
//
// The mechanism the rule protects against is runtime.FoldWave2Rows: every ID
// in IssueEnricherResult.TruncatedIDs is skipped wholesale, so a row marked
// there receives none of the findings the same enricher just computed for it.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// --- EFS -------------------------------------------------------------------

// efsEndlessMountTargetsFake always hands back another Marker, so the
// mount-target walk can only ever end at PerParentPageCap. The two policy
// calls are independent of that walk and answer normally.
type efsEndlessMountTargetsFake struct {
	awsclient.EFSAPI
	mtCalls     int
	policy      string
	policyCalls int
}

func (f *efsEndlessMountTargetsFake) DescribeMountTargets(
	_ context.Context, in *efs.DescribeMountTargetsInput, _ ...func(*efs.Options),
) (*efs.DescribeMountTargetsOutput, error) {
	f.mtCalls++
	fsID := aws.ToString(in.FileSystemId)
	return &efs.DescribeMountTargetsOutput{
		MountTargets: []efstypes.MountTargetDescription{{
			FileSystemId:   aws.String(fsID),
			MountTargetId:  aws.String("fsmt-0000000000000000a"),
			SubnetId:       aws.String("subnet-00000001"),
			LifeCycleState: efstypes.LifeCycleStateAvailable,
		}},
		NextMarker: aws.String("more"),
	}, nil
}

func (f *efsEndlessMountTargetsFake) DescribeFileSystemPolicy(
	_ context.Context, _ *efs.DescribeFileSystemPolicyInput, _ ...func(*efs.Options),
) (*efs.DescribeFileSystemPolicyOutput, error) {
	f.policyCalls++
	return &efs.DescribeFileSystemPolicyOutput{Policy: aws.String(f.policy)}, nil
}

func (f *efsEndlessMountTargetsFake) DescribeBackupPolicy(
	_ context.Context, _ *efs.DescribeBackupPolicyInput, _ ...func(*efs.Options),
) (*efs.DescribeBackupPolicyOutput, error) {
	return &efs.DescribeBackupPolicyOutput{
		BackupPolicy: &efstypes.BackupPolicy{Status: efstypes.StatusEnabled},
	}, nil
}

const efsWideOpenPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["elasticfilesystem:ClientMount","elasticfilesystem:ClientWrite"],"Resource":"*"}]}`

// TestEnrichEFS_CappedMountTargetWalkKeepsPolicyFinding pins row 31's first
// site: the file-system policy check runs before the mount-target walk
// precisely so a pagination problem cannot swallow it, and marking the file
// system uninspected when the walk hits its page cap swallows it anyway.
func TestEnrichEFS_CappedMountTargetWalkKeepsPolicyFinding(t *testing.T) {
	fake := &efsEndlessMountTargetsFake{policy: efsWideOpenPolicy}
	clients := &awsclient.ServiceClients{EFS: fake}
	rows := wipfixEFSRows("fs-0123456789abcdef0")

	res, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("EnrichEFSMountTargets returned an error for a page-capped walk: %v — "+
			"a cap is not a failure", err)
	}
	if fake.mtCalls != awsclient.PerParentPageCap {
		t.Fatalf("precondition: DescribeMountTargets called %d times, want the cap %d",
			fake.mtCalls, awsclient.PerParentPageCap)
	}
	if fake.policyCalls != 1 {
		t.Fatalf("precondition: DescribeFileSystemPolicy called %d times, want 1", fake.policyCalls)
	}

	if res.TruncatedIDs["fs-0123456789abcdef0"] {
		t.Errorf("TruncatedIDs[fs-0123456789abcdef0] = true — a mount-target page cap must not " +
			"mark the file system uninspected: FoldWave2Rows skips every id in this map, " +
			"so the efs.public-policy finding below never reaches the row")
	}
	if !hasFindingCode(res.Findings["fs-0123456789abcdef0"], "efs.public-policy") {
		t.Errorf("findings for the capped file system = %v, want efs.public-policy — "+
			"a policy open to anyone stays open however many mount targets it has",
			codesOf(res.Findings["fs-0123456789abcdef0"]))
	}

	// The user-visible observable: the finding must survive the fold onto the row.
	td := resource.FindResourceType("efs")
	if td == nil {
		t.Fatal("resource type efs is not registered")
	}
	folded := wipfixEFSRows("fs-0123456789abcdef0")
	runtime.FoldWave2Rows(folded, *td, res.Findings, res.AttentionDetails, res.TruncatedIDs)
	if !hasFindingCode(folded[0].Findings, "efs.public-policy") {
		t.Errorf("after FoldWave2Rows the row carries %v, want efs.public-policy — "+
			"this is the drop the uninspected mark causes", codesOf(folded[0].Findings))
	}
}

// TestEnrichEFS_HealthyPolicyOnCappedWalkRaisesNothing is the negative half:
// the cap must not invent a finding either. A file system whose policy is
// absent and whose backups are on stays clean even when its mount-target
// walk is capped.
func TestEnrichEFS_HealthyPolicyOnCappedWalkRaisesNothing(t *testing.T) {
	fake := &efsEndlessMountTargetsFake{
		policy: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"elasticfilesystem:ClientMount","Resource":"*"}]}`,
	}
	clients := &awsclient.ServiceClients{EFS: fake}
	res, err := awsclient.EnrichEFSMountTargets(context.Background(), clients,
		wipfixEFSRows("fs-0123456789abcdef1"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := codesOf(res.Findings["fs-0123456789abcdef1"]); len(got) != 0 {
		t.Errorf("findings for a healthy, page-capped file system = %v, want none", got)
	}
	if res.TruncatedIDs["fs-0123456789abcdef1"] {
		t.Errorf("TruncatedIDs[fs-0123456789abcdef1] = true on a page cap alone, want false")
	}
}

// --- EventBridge rule ------------------------------------------------------

// ebEndlessTargetsFake always returns another NextToken, so the target walk
// ends at PerParentPageCap. Every target it hands back lacks a
// DeadLetterConfig, so each arrived page carries a real, already-derived
// finding that the uninspected mark would discard.
type ebEndlessTargetsFake struct {
	awsclient.EventBridgeAPI
	calls int
}

func (f *ebEndlessTargetsFake) ListTargetsByRule(
	_ context.Context, _ *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options),
) (*eventbridge.ListTargetsByRuleOutput, error) {
	f.calls++
	return &eventbridge.ListTargetsByRuleOutput{
		Targets: []eventbridgetypes.Target{{
			Id:  aws.String("target-" + string(rune('a'+f.calls%26))),
			Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:example-consumer"),
		}},
		NextToken: aws.String("more"),
	}, nil
}

// TestEnrichEventBridgeRule_CappedTargetWalkKeepsFoundRows pins row 31's
// second site: the dead-letter verdicts derived from the pages that DID
// arrive are facts about real targets. Capping the walk means the count is a
// lower bound, not that those verdicts are unknown.
func TestEnrichEventBridgeRule_CappedTargetWalkKeepsFoundRows(t *testing.T) {
	fake := &ebEndlessTargetsFake{}
	clients := &awsclient.ServiceClients{EventBridge: fake}
	rows := []resource.Resource{{
		ID:     "example-nightly-rule",
		Name:   "example-nightly-rule",
		Fields: map[string]string{"name": "example-nightly-rule", "state": "ENABLED", "event_bus": "default"},
	}}

	res, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), clients, rows, nil)
	if err != nil {
		t.Fatalf("EnrichEventBridgeRuleTargets returned an error for a page-capped walk: %v", err)
	}
	if fake.calls != awsclient.PerParentPageCap {
		t.Fatalf("precondition: ListTargetsByRule called %d times, want the cap %d",
			fake.calls, awsclient.PerParentPageCap)
	}

	if res.TruncatedIDs["example-nightly-rule"] {
		t.Errorf("TruncatedIDs[example-nightly-rule] = true — a target-walk page cap must not " +
			"mark the rule uninspected: the dead-letter verdicts below were computed from " +
			"targets that really exist, and FoldWave2Rows drops every one of them")
	}
	if !hasFindingCode(res.Findings["example-nightly-rule"], "eb-rule.target-issue") {
		t.Errorf("findings for the capped rule = %v, want eb-rule.target-issue — "+
			"the targets on the pages that arrived have no dead-letter config",
			codesOf(res.Findings["example-nightly-rule"]))
	}
	if got := res.FieldUpdates["example-nightly-rule"]["target_count"]; got != "10+" {
		t.Errorf("target_count = %q, want %q — the cap reports itself as the count's lower bound, "+
			"which is the whole of what it knows", got, "10+")
	}

	td := resource.FindResourceType("eb-rule")
	if td == nil {
		t.Fatal("resource type eb-rule is not registered")
	}
	folded := []resource.Resource{{ID: "example-nightly-rule", Name: "example-nightly-rule"}}
	runtime.FoldWave2Rows(folded, *td, res.Findings, res.AttentionDetails, res.TruncatedIDs)
	if !hasFindingCode(folded[0].Findings, "eb-rule.target-issue") {
		t.Errorf("after FoldWave2Rows the row carries %v, want eb-rule.target-issue",
			codesOf(folded[0].Findings))
	}
}

// TestEnrichEventBridgeRule_CappedWalkNeverClaimsNoTargets is the negative
// half: an ENABLED rule whose walk was capped has definitely got targets, so
// the "fires but goes nowhere" finding must stay off.
func TestEnrichEventBridgeRule_CappedWalkNeverClaimsNoTargets(t *testing.T) {
	clients := &awsclient.ServiceClients{EventBridge: &ebEndlessTargetsFake{}}
	res, err := awsclient.EnrichEventBridgeRuleTargets(context.Background(), clients,
		[]resource.Resource{{
			ID:     "example-nightly-rule",
			Fields: map[string]string{"name": "example-nightly-rule", "state": "ENABLED"},
		}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasFindingCode(res.Findings["example-nightly-rule"], "eb-rule.no-targets") {
		t.Errorf("a rule whose target walk was capped raised eb-rule.no-targets — " +
			"it demonstrably has targets")
	}
}

// --- shared assertions -----------------------------------------------------

// wipfixEFSRows returns EFS list rows in the shape the efs fetcher writes.
func wipfixEFSRows(ids ...string) []resource.Resource {
	out := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		out = append(out, resource.Resource{
			ID:   id,
			Name: "example-" + id,
			Fields: map[string]string{
				"file_system_id":   id,
				"life_cycle_state": "available",
				"mount_targets":    "1",
			},
		})
	}
	return out
}

func hasFindingCode(fs []domain.Finding, code domain.FindingCode) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

func codesOf(fs []domain.Finding) []domain.FindingCode {
	out := make([]domain.FindingCode, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}
