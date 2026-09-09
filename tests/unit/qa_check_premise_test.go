// qa_check_premise_test.go — six conditions whose premise does not match what
// AWS documents, each with the healthy case that must stop firing and the
// genuinely bad case that must keep firing.
//
// A finding's condition and its sentence are one fact. When the sentence has
// to hedge — "may still apply", "unless the account default", "or it might be
// the managed key" — the condition is what is wrong, not the wording. Each
// test below fixes the fact in the negative case, so a fix that only softens
// the sentence still fails.
package unit

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// hasFinding reports whether the enricher keyed a finding with code against id.
func hasFinding(res awsclient.IssueEnricherResult, id string, code domain.FindingCode) (domain.Finding, bool) {
	for _, f := range res.Findings[id] {
		if f.Code == code {
			return f, true
		}
	}
	return domain.Finding{}, false
}

// ---------------------------------------------------------------------------
// 1. SQS — SSE-SQS is encryption
// ---------------------------------------------------------------------------

// sqsAttrFake serves GetQueueAttributes per queue URL.
type sqsAttrFake struct {
	awsclient.SQSAPI
	attrs map[string]map[string]string
}

func (f *sqsAttrFake) GetQueueAttributes(
	_ context.Context, in *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options),
) (*sqs.GetQueueAttributesOutput, error) {
	url := aws.ToString(in.QueueUrl)
	return &sqs.GetQueueAttributesOutput{Attributes: f.attrs[url]}, nil
}

// TestSQSManagedEncryptionIsEncryption holds the fact AWS documents: a queue
// with SqsManagedSseEnabled true is encrypted at rest with an AWS-owned key,
// and carries no KmsMasterKeyId precisely because it is not using a customer
// key. Reading the absence of that one attribute as "messages sit unencrypted"
// calls an encrypted queue unencrypted.
func TestSQSManagedEncryptionIsEncryption(t *testing.T) {
	const (
		sseQueue    = "https://sqs.us-east-1.amazonaws.com/123456789012/orders-sse-sqs"
		bareQueue   = "https://sqs.us-east-1.amazonaws.com/123456789012/orders-plaintext"
		queueARNPfx = "arn:aws:sqs:us-east-1:123456789012:"
	)
	fake := &sqsAttrFake{attrs: map[string]map[string]string{
		sseQueue: {
			"QueueArn":             queueARNPfx + "orders-sse-sqs",
			"SqsManagedSseEnabled": "true",
			"RedrivePolicy":        `{"deadLetterTargetArn":"` + queueARNPfx + `dlq","maxReceiveCount":5}`,
		},
		bareQueue: {
			"QueueArn":             queueARNPfx + "orders-plaintext",
			"SqsManagedSseEnabled": "false",
			"RedrivePolicy":        `{"deadLetterTargetArn":"` + queueARNPfx + `dlq","maxReceiveCount":5}`,
		},
	}}
	rows := []resource.Resource{
		{ID: sseQueue, Name: "orders-sse-sqs", Fields: map[string]string{"queue_url": sseQueue}},
		{ID: bareQueue, Name: "orders-plaintext", Fields: map[string]string{"queue_url": bareQueue}},
	}

	res, err := awsclient.EnrichSQSAttributes(context.Background(), &awsclient.ServiceClients{SQS: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichSQSAttributes: %v", err)
	}

	if f, ok := hasFinding(res, sseQueue, "sqs.no-kms"); ok {
		t.Errorf("a queue with SSE-SQS enabled is flagged %q (%q). AWS encrypts it "+
			"at rest with an AWS-owned key; KmsMasterKeyId is empty because no "+
			"customer key is in use, not because nothing is encrypting it",
			f.Code, f.Phrase)
	}
	if _, ok := hasFinding(res, bareQueue, "sqs.no-kms"); !ok {
		t.Error("a queue with neither SSE-SQS nor a KMS key is not flagged; the " +
			"narrowed condition still has to catch the queue that really is unencrypted")
	}
}

// ---------------------------------------------------------------------------
// 2. SNS — a topic without a customer key is not a topic on unencrypted disks
// ---------------------------------------------------------------------------

type snsAttrFake struct {
	awsclient.SNSAPI
	attrs map[string]map[string]string
}

func (f *snsAttrFake) GetTopicAttributes(
	_ context.Context, in *sns.GetTopicAttributesInput, _ ...func(*sns.Options),
) (*sns.GetTopicAttributesOutput, error) {
	return &sns.GetTopicAttributesOutput{Attributes: f.attrs[aws.ToString(in.TopicArn)]}, nil
}

func (f *snsAttrFake) ListSubscriptionsByTopic(
	_ context.Context, _ *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options),
) (*sns.ListSubscriptionsByTopicOutput, error) {
	return &sns.ListSubscriptionsByTopicOutput{}, nil
}

// TestSNSNoCustomerKeyDoesNotClaimUnencryptedStorage holds what the absence of
// KmsMasterKeyId actually means: SNS stores messages on encrypted volumes
// whatever the topic's own setting, and the key adds server-side encryption of
// the message itself. The finding may stand — a customer key is worth having —
// but it must not tell the operator their messages are lying about in the
// clear, because that is the sentence that sends someone to check a disk.
func TestSNSNoCustomerKeyDoesNotClaimUnencryptedStorage(t *testing.T) {
	const topicARN = "arn:aws:sns:us-east-1:123456789012:orders-events"
	fake := &snsAttrFake{attrs: map[string]map[string]string{
		topicARN: {"TopicArn": topicARN},
	}}
	rows := []resource.Resource{{ID: topicARN, Name: "orders-events"}}

	res, err := awsclient.EnrichSNSSubscriptions(context.Background(),
		&awsclient.ServiceClients{SNS: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichSNSSubscriptions: %v", err)
	}

	f, ok := hasFinding(res, topicARN, "sns.no-kms")
	if !ok {
		t.Fatal("a topic with no customer key emits no finding at all; the check " +
			"about message-level encryption is the one that should stand")
	}
	for _, claim := range []string{"unencrypted", "backing storage"} {
		if strings.Contains(strings.ToLower(f.Detail), claim) {
			t.Errorf("the sns.no-kms sentence says %q: %q\n    SNS encrypts message "+
				"storage with AWS-owned keys regardless of this attribute. The finding "+
				"is about server-side encryption of the message with a customer key, "+
				"and saying otherwise sends the operator to look at a disk that is "+
				"already encrypted", claim, f.Detail)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. EKS — 1.28 and later encrypt secrets without a customer key
// ---------------------------------------------------------------------------

func eksClusterOut(name, version string, enc []ekstypes.EncryptionConfig) *eks.DescribeClusterOutput {
	return &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{
		Name:             aws.String(name),
		Arn:              aws.String("arn:aws:eks:us-east-1:123456789012:cluster/" + name),
		Version:          aws.String(version),
		Status:           ekstypes.ClusterStatusActive,
		EncryptionConfig: enc,
		ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
			EndpointPublicAccess:  false,
			EndpointPrivateAccess: true,
		},
		Logging: &ekstypes.Logging{ClusterLogging: []ekstypes.LogSetup{{
			Enabled: aws.Bool(true),
			Types: []ekstypes.LogType{
				ekstypes.LogTypeApi, ekstypes.LogTypeAudit, ekstypes.LogTypeAuthenticator,
				ekstypes.LogTypeControllerManager, ekstypes.LogTypeScheduler,
			},
		}}},
	}}
}

// TestEKSDefaultEnvelopeEncryptionIsNotMissingEncryption holds the change AWS
// made at Kubernetes 1.28: every cluster from that version on envelope-encrypts
// Kubernetes secrets with an AWS-owned key, with no configuration. An empty
// EncryptionConfig on such a cluster means "no customer-managed key", which is
// a different and much smaller thing than "secrets are not encrypted", and the
// finding as written tells the operator etcd is readable when it is not.
//
// If the catalog wants to keep saying something about the absent customer key,
// that is a separate code at a lower tier. The observable pinned here is that
// the cluster no longer carries this one.
func TestEKSDefaultEnvelopeEncryptionIsNotMissingEncryption(t *testing.T) {
	fake := &eksDescribeFailFake{
		clusters: []string{"prod-128", "legacy-127"},
		outputs: map[string]*eks.DescribeClusterOutput{
			"prod-128":   eksClusterOut("prod-128", "1.28", nil),
			"legacy-127": eksClusterOut("legacy-127", "1.27", nil),
		},
	}
	out, err := awsclient.FetchEKSClustersPage(context.Background(),
		&awsclient.ServiceClients{EKS: fake}, "")
	if err != nil {
		t.Fatalf("FetchEKSClustersPage: %v", err)
	}

	byName := map[string]resource.Resource{}
	for _, r := range out.Resources {
		byName[r.Name] = r
	}
	carries := func(name string) (domain.Finding, bool) {
		for _, f := range byName[name].Findings {
			if f.Code == "eks.secrets-not-kms" {
				return f, true
			}
		}
		return domain.Finding{}, false
	}

	if f, ok := carries("prod-128"); ok {
		t.Errorf("a 1.28 cluster with no customer key is flagged %q (%q). From 1.28 "+
			"AWS envelope-encrypts Kubernetes secrets with an AWS-owned key on every "+
			"cluster; an empty EncryptionConfig means no customer-managed key, not "+
			"no encryption", f.Code, f.Phrase)
	}
	if _, ok := carries("legacy-127"); !ok {
		t.Error("a 1.27 cluster with no EncryptionConfig is not flagged; before 1.28 " +
			"there is no default envelope encryption, so that cluster is the one the " +
			"finding was written for")
	}
}

// ---------------------------------------------------------------------------
// 4. VPC — a flow log on a subnet or an ENI is a flow log
// ---------------------------------------------------------------------------

// vpcScopedFlowLogFake answers DescribeFlowLogs the way AWS does: with no
// resource-id filter it returns every flow log in the account, and with one it
// returns only the logs whose ResourceId matches. A fake that ignores the
// filter would pass an implementation that still asks only about the VPC.
type vpcScopedFlowLogFake struct {
	awsclient.EC2API
	logs []ec2types.FlowLog
}

func (f *vpcScopedFlowLogFake) DescribeFlowLogs(
	_ context.Context, in *ec2svc.DescribeFlowLogsInput, _ ...func(*ec2svc.Options),
) (*ec2svc.DescribeFlowLogsOutput, error) {
	wanted := map[string]bool{}
	for _, filter := range in.Filter {
		if aws.ToString(filter.Name) == "resource-id" {
			for _, v := range filter.Values {
				wanted[v] = true
			}
		}
	}
	if len(wanted) == 0 {
		return &ec2svc.DescribeFlowLogsOutput{FlowLogs: f.logs}, nil
	}
	var out []ec2types.FlowLog
	for _, fl := range f.logs {
		if wanted[aws.ToString(fl.ResourceId)] {
			out = append(out, fl)
		}
	}
	return &ec2svc.DescribeFlowLogsOutput{FlowLogs: out}, nil
}

// TestSubnetScopedFlowLogCountsAsCoverage holds what a flow log is: a capture
// attached to a VPC, a subnet, or a network interface, all three writing the
// same records. Asking only for logs whose resource-id is the VPC misses the
// two narrower scopes and reports a VPC with full subnet coverage as having no
// record of what connected to what.
func TestSubnetScopedFlowLogCountsAsCoverage(t *testing.T) {
	const (
		coveredVPC = "vpc-0a1b2c3d4e5f60001"
		bareVPC    = "vpc-0a1b2c3d4e5f60002"
		subnetID   = "subnet-0a1b2c3d4e5f60001"
	)
	fake := &vpcScopedFlowLogFake{logs: []ec2types.FlowLog{{
		FlowLogId:     aws.String("fl-0a1b2c3d4e5f60001"),
		ResourceId:    aws.String(subnetID),
		FlowLogStatus: aws.String("ACTIVE"),
		TrafficType:   ec2types.TrafficTypeAll,
	}}}
	rows := []resource.Resource{
		{ID: coveredVPC, Name: "prod", Fields: map[string]string{
			"vpc_id": coveredVPC, "subnet_ids": subnetID,
		}},
		{ID: bareVPC, Name: "sandbox", Fields: map[string]string{"vpc_id": bareVPC}},
	}

	res, err := awsclient.EnrichVPCFlowLogs(context.Background(),
		&awsclient.ServiceClients{EC2: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichVPCFlowLogs: %v", err)
	}

	if f, ok := hasFinding(res, coveredVPC, "vpc.no-flow-logs"); ok {
		t.Errorf("a VPC whose subnet carries an ACTIVE flow log is flagged %q (%q). "+
			"A subnet- or interface-scoped log writes the same records; only a query "+
			"filtered to the VPC's own id misses it", f.Code, f.Phrase)
	}
	if got := res.FieldUpdates[coveredVPC]["flow_logs"]; got != "yes" {
		t.Errorf("flow_logs field for the covered VPC = %q, want %q — the column and "+
			"the finding read the same fact and must not disagree", got, "yes")
	}
	if _, ok := hasFinding(res, bareVPC, "vpc.no-flow-logs"); !ok {
		t.Error("a VPC with no flow log at any scope is not flagged; the widened " +
			"query still has to find the VPC that genuinely captures nothing")
	}
}

// ---------------------------------------------------------------------------
// 5. ECS — the scheduler stopping a task is not the task failing
// ---------------------------------------------------------------------------

// TestSchedulerAndSpotStopsAreNotFailures holds what an ECS stop code says.
// ServiceSchedulerInitiated is the service replacing a task during a
// deployment or a scale-in, and SpotInterruption is capacity being reclaimed on
// two minutes' notice — both are the platform doing its job, and neither means
// something stopped the task. Treating every code other than UserInitiated as a
// failure paints a normal deployment red.
func TestSchedulerAndSpotStopsAreNotFailures(t *testing.T) {
	const clusterARN = "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"
	taskARN := func(name string) string {
		return clusterARN[:len(clusterARN)-len("cluster/prod-cluster")] + "task/prod-cluster/" + name
	}

	stopped := func(name string, code ecstypes.TaskStopCode, reason string) ecstypes.Task {
		return ecstypes.Task{
			TaskArn:           aws.String(taskARN(name)),
			LastStatus:        aws.String("STOPPED"),
			DesiredStatus:     aws.String("STOPPED"),
			HealthStatus:      ecstypes.HealthStatusUnknown,
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/app:7"),
			StoppedReason:     aws.String(reason),
			StopCode:          code,
			LaunchType:        ecstypes.LaunchTypeFargate,
		}
	}

	listMock := &mockECSListTasksClient{outputs: map[string]*ecs.ListTasksOutput{
		clusterARN: {TaskArns: []string{
			taskARN("scheduler"), taskARN("spot"), taskARN("crashed"),
		}},
	}}
	describeMock := &mockECSDescribeTasksClient{output: &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{
		stopped("scheduler", ecstypes.TaskStopCodeServiceSchedulerInitiated,
			"Scaling activity initiated by (deployment ecs-svc/1234567890123456789)"),
		stopped("spot", ecstypes.TaskStopCodeSpotInterruption,
			"Your Spot Task was interrupted"),
		stopped("crashed", ecstypes.TaskStopCodeEssentialContainerExited,
			"Essential container in task exited"),
	}}}

	out, err := awsclient.FetchEcsSvcTasks(context.Background(), listMock, describeMock,
		clusterARN, "web-service", "")
	if err != nil {
		t.Fatalf("FetchEcsSvcTasks: %v", err)
	}

	broken := map[string]domain.Finding{}
	for _, r := range out.Resources {
		for _, f := range r.Findings {
			if f.Code == "ecs-task.stop-code.failed" {
				broken[r.Fields["stop_code"]] = f
			}
		}
	}

	for _, code := range []string{
		string(ecstypes.TaskStopCodeServiceSchedulerInitiated),
		string(ecstypes.TaskStopCodeSpotInterruption),
	} {
		if f, ok := broken[code]; ok {
			t.Errorf("a task stopped with %s is flagged broken as %q (%q). The "+
				"scheduler replacing a task and Spot reclaiming capacity are the "+
				"platform working, not something stopping the task", code, f.Code, f.Phrase)
		}
	}
	if _, ok := broken[string(ecstypes.TaskStopCodeEssentialContainerExited)]; !ok {
		t.Error("a task whose essential container exited is not flagged broken; " +
			"narrowing the condition must not retire the code for the stop that " +
			"really is a failure")
	}
}

// ---------------------------------------------------------------------------
// 6. CloudFront — an S3 website endpoint cannot serve HTTPS
// ---------------------------------------------------------------------------

// TestS3WebsiteOriginIsNotToldToUseHTTPS holds the constraint AWS documents:
// an S3 static-website endpoint serves HTTP only, so http-only is the only
// origin protocol policy that works and CloudFront rejects https-only against
// it. Telling the operator to set HTTPS to the origin is advice that cannot be
// followed; the answer for that architecture is a REST endpoint with origin
// access control, which is a different finding.
func TestS3WebsiteOriginIsNotToldToUseHTTPS(t *testing.T) {
	websiteOrigin := cfHealthyForW6ARows(&cftypes.DistributionConfig{
		Comment: aws.String("static site"),
		Enabled: aws.Bool(true),
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
			TargetOriginId:       aws.String("site-origin"),
		},
		Origins: &cftypes.Origins{Quantity: aws.Int32(1), Items: []cftypes.Origin{{
			Id:         aws.String("site-origin"),
			DomainName: aws.String("marketing-site.s3-website-us-east-1.amazonaws.com"),
			CustomOriginConfig: &cftypes.CustomOriginConfig{
				HTTPPort:             aws.Int32(80),
				HTTPSPort:            aws.Int32(443),
				OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpOnly,
			},
		}}},
	})
	customOrigin := cfHealthyForW6ARows(&cftypes.DistributionConfig{
		Comment: aws.String("api"),
		Enabled: aws.Bool(true),
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
			TargetOriginId:       aws.String("api-origin"),
		},
		Origins: &cftypes.Origins{Quantity: aws.Int32(1), Items: []cftypes.Origin{{
			Id:         aws.String("api-origin"),
			DomainName: aws.String("api.example.com"),
			CustomOriginConfig: &cftypes.CustomOriginConfig{
				HTTPPort:             aws.Int32(80),
				HTTPSPort:            aws.Int32(443),
				OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpOnly,
			},
		}}},
	})

	fake := &cfGetDistributionConfigFake{results: map[string]*cftypes.DistributionConfig{
		cfDistroID1: websiteOrigin,
		cfDistroID2: customOrigin,
	}}
	res, err := awsclient.EnrichCloudFrontDistribution(context.Background(),
		&awsclient.ServiceClients{CloudFront: fake},
		cfDistroResources(cfDistroID1, cfDistroID2), nil)
	if err != nil {
		t.Fatalf("EnrichCloudFrontDistribution: %v", err)
	}

	if f, ok := hasFinding(res, cfDistroID1, "cf.insecure-protocol"); ok {
		t.Errorf("a distribution whose origin is an S3 website endpoint is flagged "+
			"%q (%q). That endpoint serves HTTP only, so http-only is the sole policy "+
			"CloudFront accepts for it and the sentence asks for something that cannot "+
			"be configured", f.Code, f.Phrase)
	}
	if _, ok := hasFinding(res, cfDistroID2, "cf.insecure-protocol"); !ok {
		t.Error("a distribution reaching an ordinary custom origin over http-only is " +
			"not flagged; that origin can serve HTTPS, so it is the one the finding " +
			"was written for")
	}
}
