// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// phrase_multi_item_verify_test.go — the multi-item case on three more of the
// converted emitters.
//
// Moving an item out of the phrase is only half the fix. The other half is
// that every item the emitter inspected and found wrong reaches a supporting
// row, so the reader can act on all of them. Each site below is given more
// than one offending item, which is the input no demo fixture supplies and
// the one that tells a converted emitter apart from an emitter that merely
// stopped naming the item it kept.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rowsUnder returns the supporting rows recorded for one (resource, code) pair.
func rowsUnder(result awsclient.IssueEnricherResult, resourceID string, code domain.FindingCode) []domain.DetailRow {
	return result.AttentionDetails[resourceID][code].Rows
}

// hasRow reports whether any supporting row carries value.
func hasRow(rows []domain.DetailRow, value string) bool {
	for _, row := range rows {
		if row.Value == value || row.Label == value {
			return true
		}
	}
	return false
}

func TestCFNTwoFailedResourcesNameBothUnderOnePhrase(t *testing.T) {
	const code domain.FindingCode = "cfn.recent-resource-failure"
	stackID := "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-api/abc"
	fake := &fakeCFNEnricher{
		stackEvents: []cfntypes.StackEvent{
			{
				ResourceStatus:    cfntypes.ResourceStatusCreateFailed,
				LogicalResourceId: aws.String("ApiBucket"),
				ResourceType:      aws.String("AWS::S3::Bucket"),
			},
			{
				ResourceStatus:    cfntypes.ResourceStatusUpdateFailed,
				LogicalResourceId: aws.String("ApiQueue"),
				ResourceType:      aws.String("AWS::SQS::Queue"),
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{{
		ID:     stackID,
		Name:   "acme-api",
		Fields: map[string]string{"stack_name": "acme-api"},
	}}

	result, err := awsclient.EnrichCFNStackEvents(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs := result.Findings[stackID]
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want exactly 1 — two failed resources are one condition", len(fs))
	}
	if want := catalog.Phrase(code); fs[0].Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", fs[0].Phrase, want)
	}
	rows := rowsUnder(result, stackID, code)
	if !hasRow(rows, "AWS::S3::Bucket/ApiBucket") || !hasRow(rows, "AWS::SQS::Queue/ApiQueue") {
		t.Errorf("rows = %v, want a row for each failed resource — naming only the first leaves "+
			"the second unsayable", rows)
	}
}

func TestCFNOneFailedResourceNamesOnlyThatResource(t *testing.T) {
	const code domain.FindingCode = "cfn.recent-resource-failure"
	stackID := "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-web/def"
	fake := &fakeCFNEnricher{
		stackEvents: []cfntypes.StackEvent{
			{
				ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
				LogicalResourceId: aws.String("WebBucket"),
				ResourceType:      aws.String("AWS::S3::Bucket"),
			},
			{
				ResourceStatus:    cfntypes.ResourceStatusUpdateFailed,
				LogicalResourceId: aws.String("WebQueue"),
				ResourceType:      aws.String("AWS::SQS::Queue"),
			},
		},
	}
	clients := &awsclient.ServiceClients{CloudFormation: fake}
	resources := []resource.Resource{{
		ID:     stackID,
		Name:   "acme-web",
		Fields: map[string]string{"stack_name": "acme-web"},
	}}

	result, err := awsclient.EnrichCFNStackEvents(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows := rowsUnder(result, stackID, code)
	if hasRow(rows, "AWS::S3::Bucket/WebBucket") {
		t.Errorf("rows = %v, name a resource whose event succeeded", rows)
	}
	if !hasRow(rows, "AWS::SQS::Queue/WebQueue") {
		t.Errorf("rows = %v, want the failed resource", rows)
	}
}

func TestTGWEveryAttachmentReachesItsConditionsRows(t *testing.T) {
	const (
		failedCode       domain.FindingCode = "tgw.attachment-failed"
		transitionalCode domain.FindingCode = "tgw.attachment-transitional"
	)
	fake := &tgwAttachmentFake{
		results: map[string][]ec2types.TransitGatewayAttachment{
			"tgw-00000001": {
				tgwAttachment("tgw-00000001", "tgw-attach-f001", ec2types.TransitGatewayAttachmentStateFailed),
				tgwAttachment("tgw-00000001", "tgw-attach-f002", ec2types.TransitGatewayAttachmentStateFailed),
				tgwAttachment("tgw-00000001", "tgw-attach-m003", ec2types.TransitGatewayAttachmentStateModifying),
				tgwAttachment("tgw-00000001", "tgw-attach-a004", ec2types.TransitGatewayAttachmentStateAvailable),
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}

	result, err := awsclient.EnrichTGWAttachments(context.Background(), clients, tgwResources("tgw-00000001"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Two independently-evaluated conditions on one gateway, each keeping its
	// own severity: an attachment that failed is not an attachment mid-change.
	sev := map[domain.FindingCode]domain.Severity{}
	for _, f := range result.Findings["tgw-00000001"] {
		if _, dup := sev[f.Code]; dup {
			t.Errorf("%s emitted twice on one gateway", f.Code)
		}
		sev[f.Code] = f.Severity
		if want := catalog.Phrase(f.Code); f.Phrase != want {
			t.Errorf("%s Phrase = %q, want the catalog's %q", f.Code, f.Phrase, want)
		}
	}
	if sev[failedCode] != domain.SevBroken {
		t.Errorf("%s severity = %v, want SevBroken", failedCode, sev[failedCode])
	}
	if sev[transitionalCode] != domain.SevWarn {
		t.Errorf("%s severity = %v, want SevWarn", transitionalCode, sev[transitionalCode])
	}

	failedRows := rowsUnder(result, "tgw-00000001", failedCode)
	if !hasRow(failedRows, "tgw-attach-f001") || !hasRow(failedRows, "tgw-attach-f002") {
		t.Errorf("failed rows = %v, want both failed attachments — keeping only the worst one "+
			"left the other with nothing to say it was inspected", failedRows)
	}
	if hasRow(failedRows, "tgw-attach-m003") || hasRow(failedRows, "tgw-attach-a004") {
		t.Errorf("failed rows = %v, name an attachment that did not fail", failedRows)
	}
	transitionalRows := rowsUnder(result, "tgw-00000001", transitionalCode)
	if !hasRow(transitionalRows, "tgw-attach-m003") {
		t.Errorf("transitional rows = %v, want the modifying attachment", transitionalRows)
	}
	if hasRow(transitionalRows, "tgw-attach-a004") {
		t.Errorf("transitional rows = %v, name an available attachment", transitionalRows)
	}
}

func TestECSClusterBothConditionsNameBothUnderOnePhrase(t *testing.T) {
	const code domain.FindingCode = "ecs.cluster-issue"
	clusterName := "acme-batch-cluster"
	fake := &fakeECSEnricher{
		descClustersOut: &ecs.DescribeClustersOutput{
			Clusters: []ecstypes.Cluster{{
				ClusterName:                       aws.String(clusterName),
				PendingTasksCount:                 4,
				RunningTasksCount:                 0,
				RegisteredContainerInstancesCount: 2,
			}},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{{
		ID:     clusterName,
		Name:   clusterName,
		Fields: map[string]string{"cluster_name": clusterName},
	}}

	result, err := awsclient.EnrichECSClusters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs := result.Findings[clusterName]
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want exactly 1", len(fs))
	}
	if want := catalog.Phrase(code); fs[0].Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q — the two conditions were joined into the "+
			"wording before, which is the shape this row removes", fs[0].Phrase, want)
	}
	rows := rowsUnder(result, clusterName, code)
	if !hasRow(rows, "4 tasks pending") {
		t.Errorf("rows = %v, want the pending-task count", rows)
	}
	if !hasRow(rows, "no running tasks (2 container instances registered)") {
		t.Errorf("rows = %v, want the idle-instances row", rows)
	}
}

func TestECSClusterPendingOnlyNamesOnlyPending(t *testing.T) {
	const code domain.FindingCode = "ecs.cluster-issue"
	clusterName := "acme-web-cluster"
	fake := &fakeECSEnricher{
		descClustersOut: &ecs.DescribeClustersOutput{
			Clusters: []ecstypes.Cluster{{
				ClusterName:                       aws.String(clusterName),
				PendingTasksCount:                 4,
				RunningTasksCount:                 9,
				RegisteredContainerInstancesCount: 2,
			}},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{{
		ID:     clusterName,
		Name:   clusterName,
		Fields: map[string]string{"cluster_name": clusterName},
	}}

	result, err := awsclient.EnrichECSClusters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows := rowsUnder(result, clusterName, code)
	if len(rows) != 1 || !hasRow(rows, "4 tasks pending") {
		t.Errorf("rows = %v, want only the pending-task row — a cluster running its tasks has no "+
			"idle-instances problem", rows)
	}
}
