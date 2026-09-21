package unit_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

func pairChecker(t *testing.T, src, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(src) {
		if def.TargetType == target {
			return def.Checker
		}
	}
	t.Fatalf("%s registers no %s pivot", src, target)
	return nil
}

func pairDef(t *testing.T, src, target string) resource.RelatedDef {
	t.Helper()
	for _, def := range resource.GetRelated(src) {
		if def.TargetType == target {
			return def
		}
	}
	t.Fatalf("%s registers no %s pivot", src, target)
	return resource.RelatedDef{}
}

// ─── acm ↔ elb: the SNI certificates of a listener ─────────────────────────

const (
	t548bELBARN      = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/shared-ingress/1111222233334444"
	t548bListenerARN = t548bELBARN + "/listener/aaaa1111"
	t548bDefaultCert = "arn:aws:acm:us-east-1:123456789012:certificate/11111111-1111-1111-1111-111111111111"
	t548bSNICert     = "arn:aws:acm:us-east-1:123456789012:certificate/22222222-2222-2222-2222-222222222222"
)

// t548bListenerStub answers DescribeListeners with one HTTPS listener and
// DescribeListenerCertificates with that listener's whole certificate list.
type t548bListenerStub struct {
	awsclient.ELBv2API
	sni     []string
	certErr error
	calls   int
}

func (s *t548bListenerStub) DescribeListeners(context.Context, *elbv2.DescribeListenersInput, ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	return &elbv2.DescribeListenersOutput{Listeners: []elbv2types.Listener{{
		ListenerArn:     aws.String(t548bListenerARN),
		LoadBalancerArn: aws.String(t548bELBARN),
		Protocol:        elbv2types.ProtocolEnumHttps,
		Port:            aws.Int32(443),
		Certificates:    []elbv2types.Certificate{{CertificateArn: aws.String(t548bDefaultCert)}},
	}}}, nil
}

func (s *t548bListenerStub) DescribeListenerCertificates(_ context.Context, in *elbv2.DescribeListenerCertificatesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeListenerCertificatesOutput, error) {
	s.calls++
	if s.certErr != nil {
		return nil, s.certErr
	}
	certs := []elbv2types.Certificate{{CertificateArn: aws.String(t548bDefaultCert), IsDefault: aws.Bool(true)}}
	for _, arn := range s.sni {
		certs = append(certs, elbv2types.Certificate{CertificateArn: aws.String(arn), IsDefault: aws.Bool(false)})
	}
	if aws.ToString(in.ListenerArn) != t548bListenerARN {
		return &elbv2.DescribeListenerCertificatesOutput{}, nil
	}
	return &elbv2.DescribeListenerCertificatesOutput{Certificates: certs}, nil
}

func t548bELBRow() resource.Resource {
	return resource.Resource{
		ID:     "shared-ingress",
		Name:   "shared-ingress",
		Fields: map[string]string{"load_balancer_arn": t548bELBARN, "type": "application"},
	}
}

// TestT548b_ELBNamesEverySNICertificate pins that a load balancer names every
// certificate its listeners serve. Listener.Certificates carries the
// listener's default certificate alone, and a multi-domain listener serves
// the rest by SNI, which ACM records the load balancer under just the same.
func TestT548b_ELBNamesEverySNICertificate(t *testing.T) {
	stub := &t548bListenerStub{sni: []string{t548bSNICert}}
	got := pairChecker(t, "elb", "acm")(context.Background(), &awsclient.ServiceClients{ELBv2: stub}, t548bELBRow(), resource.ResourceCache{})

	ids := slices.Clone(got.ResourceIDs())
	slices.Sort(ids)
	want := []string{t548bDefaultCert, t548bSNICert}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Errorf("load balancer lists %v, want %v: an SNI certificate is on its listener too", ids, want)
	}
	if got.Truncated() {
		t.Errorf("both certificate lists were read, so the count is exact, not a lower bound")
	}
	if stub.calls == 0 {
		t.Errorf("DescribeListenerCertificates was not called for an HTTPS listener")
	}
}

// TestT548b_ELBCertificatesAreALowerBoundWhenUnreadable pins that a listener
// whose certificate list could not be read leaves the count a lower bound:
// it may serve a certificate this answer does not name.
func TestT548b_ELBCertificatesAreALowerBoundWhenUnreadable(t *testing.T) {
	failing := &t548bListenerStub{certErr: errors.New("AccessDenied")}
	got := pairChecker(t, "elb", "acm")(context.Background(), &awsclient.ServiceClients{ELBv2: failing}, t548bELBRow(), resource.ResourceCache{})
	if !got.Truncated() {
		t.Errorf("a refused DescribeListenerCertificates leaves the certificates a lower bound, got an exact %d", got.Count())
	}
	if !slices.Contains(got.ResourceIDs(), t548bDefaultCert) {
		t.Errorf("the default certificate was read and must still be listed, got %v", got.ResourceIDs())
	}
}

// ─── the lambda reference predicate ────────────────────────────────────────

// TestT548b_LambdaTriggerNamesQualifiedAndForeignARNs pins how a reference to
// a function is read wherever another service records one: an alias- or
// version-qualified ARN names the function it belongs to, and an ARN of
// another account names a different function of the same name.
func TestT548b_LambdaTriggerNamesQualifiedAndForeignARNs(t *testing.T) {
	const fnName = "orders"
	fnRow := resource.Resource{ID: fnName, Name: fnName}
	sub := func(endpoint string) resource.Resource {
		return resource.Resource{
			ID:     "arn:aws:sns:us-east-1:123456789012:orders-events:11111111-2222-3333-4444-555555555555",
			Fields: map[string]string{"protocol": "lambda", "endpoint": endpoint, "topic_arn": "arn:aws:sns:us-east-1:123456789012:orders-events"},
		}
	}
	cases := []struct {
		name     string
		endpoint string
		want     bool
	}{
		{"unqualified", "arn:aws:lambda:us-east-1:123456789012:function:orders", true},
		{"alias qualified", "arn:aws:lambda:us-east-1:123456789012:function:orders:PROD", true},
		{"version qualified", "arn:aws:lambda:us-east-1:123456789012:function:orders:7", true},
		{"another account", "arn:aws:lambda:us-east-1:999999999999:function:orders", false},
		{"another function", "arn:aws:lambda:us-east-1:123456789012:function:orders-archiver", false},
	}
	for _, tc := range cases {
		cache := resource.ResourceCache{
			"sns-sub": resource.ResourceCacheEntry{Resources: []resource.Resource{sub(tc.endpoint)}},
			"lambda":  resource.ResourceCacheEntry{Resources: []resource.Resource{fnRow}},
		}
		store := session.NewIdentityStore()
		store.Set("123456789012", nil)
		clients := &awsclient.ServiceClients{Region: "us-east-1"}
		clients.SetIdentityStore(store)
		got := pairChecker(t, "lambda", "sns-sub")(context.Background(), clients, fnRow, cache)
		if listed := len(got.ResourceIDs()) > 0; listed != tc.want {
			t.Errorf("%s: function %s lists %v, want listed=%v for endpoint %s", tc.name, fnName, got.ResourceIDs(), tc.want, tc.endpoint)
		}
	}
}

// ─── ec2 ↔ ng: one tag rule from both ends ─────────────────────────────────

func t548bNodeGroupRow(cluster, group string) resource.Resource {
	return resource.Resource{
		ID:     cluster + "/" + group,
		Fields: map[string]string{"cluster_name": cluster, "nodegroup_name": group},
		RawStruct: ekstypes.Nodegroup{
			ClusterName:   aws.String(cluster),
			NodegroupName: aws.String(group),
		},
	}
}

func t548bInstanceRow(id string, tags map[string]string) resource.Resource {
	inst := ec2types.Instance{InstanceId: aws.String(id)}
	for k, v := range tags {
		inst.Tags = append(inst.Tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return resource.Resource{ID: id, Fields: map[string]string{"instance_id": id}, RawStruct: inst}
}

// TestT548b_NodeGroupMembershipIsOneRuleFromBothEnds pins that an instance
// and a node group agree about membership: EKS writes eks:cluster-name and
// eks:nodegroup-name on every instance a managed node group launches, so an
// instance missing one of them is a node of no group rather than a node of
// every group of that name.
func TestT548b_NodeGroupMembershipIsOneRuleFromBothEnds(t *testing.T) {
	ctx := context.Background()
	groupA := t548bNodeGroupRow("cluster-a", "workers")
	groupB := t548bNodeGroupRow("cluster-b", "workers")
	tagged := t548bInstanceRow("i-0a1b2c3d4e5f60001", map[string]string{"eks:cluster-name": "cluster-a", "eks:nodegroup-name": "workers"})
	halfTagged := t548bInstanceRow("i-0a1b2c3d4e5f60002", map[string]string{"eks:nodegroup-name": "workers"})
	clusterOnly := t548bInstanceRow("i-0a1b2c3d4e5f60003", map[string]string{"eks:cluster-name": "cluster-a"})

	cache := resource.ResourceCache{
		"ng":  resource.ResourceCacheEntry{Resources: []resource.Resource{groupA, groupB}},
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{tagged, halfTagged, clusterOnly}},
	}
	ec2ToNG := pairChecker(t, "ec2", "ng")
	ngToEC2 := pairChecker(t, "ng", "ec2")

	if got := ec2ToNG(ctx, nil, tagged, cache).ResourceIDs(); !slices.Equal(got, []string{groupA.ID}) {
		t.Errorf("a fully tagged instance lists %v, want [%s]", got, groupA.ID)
	}
	if got := ngToEC2(ctx, nil, groupA, cache).ResourceIDs(); !slices.Equal(got, []string{tagged.ID}) {
		t.Errorf("node group %s lists %v, want [%s]", groupA.ID, got, tagged.ID)
	}
	if got := ngToEC2(ctx, nil, groupB, cache).ResourceIDs(); len(got) != 0 {
		t.Errorf("node group %s lists %v, want none: no instance carries its cluster", groupB.ID, got)
	}
	if got := ec2ToNG(ctx, nil, halfTagged, cache).ResourceIDs(); len(got) != 0 {
		t.Errorf("an instance with no eks:cluster-name lists %v, want none: no node group claims it back", got)
	}
	if got := ec2ToNG(ctx, nil, clusterOnly, cache).ResourceIDs(); len(got) != 0 {
		t.Errorf("an instance with no eks:nodegroup-name lists %v, want none: it is a node of no group", got)
	}
}

// ─── a failed read is unknown, never a zero ────────────────────────────────

// TestT548b_UnreadWorkgroupConfigurationIsALowerBound pins that a bucket does
// not claim no workgroup writes to it over a workgroup whose configuration
// was never read: the result location lives on the configuration, and an
// empty one on such a row says nothing either way.
func TestT548b_UnreadWorkgroupConfigurationIsALowerBound(t *testing.T) {
	unread := resource.Resource{
		ID:     "analytics",
		Name:   "analytics",
		Fields: map[string]string{"workgroup_name": "analytics", "result_config_unread": "true"},
	}
	cache := resource.ResourceCache{"athena": resource.ResourceCacheEntry{Resources: []resource.Resource{unread}}}
	bucket := resource.Resource{ID: "build-results", Name: "build-results"}

	got := pairChecker(t, "s3", "athena")(context.Background(), nil, bucket, cache)
	if !got.Truncated() {
		t.Errorf("a workgroup whose configuration was not read leaves the bucket's count a lower bound, got an exact %d", got.Count())
	}
}

// TestT548b_UnreadTaskDefinitionIsUnknown pins that a task whose definition
// the list fetcher could not read reports what it consumes as unknown: its
// secrets, its SSM parameters and its roles live on that definition.
func TestT548b_UnreadTaskDefinitionIsUnknown(t *testing.T) {
	unjoined := resource.Resource{
		ID:     "task-1111",
		Fields: map[string]string{"task_id": "task-1111", "task_def_join_error": "true"},
	}
	for _, target := range []string{"secrets", "ssm", "role"} {
		got := pairChecker(t, "ecs-task", target)(context.Background(), nil, unjoined, resource.ResourceCache{})
		if got.Coverage() != domain.CoverageNoPath {
			t.Errorf("%s over an unread task definition has coverage %v, want nothing searched", target, got.Coverage())
		}
	}
}

// ─── ddb ↔ lambda: the table, not the stream generation ────────────────────

// TestT548b_TableNamesConsumersOfAnEarlierStream pins that a table names the
// functions consuming it whichever of its streams the mapping was built
// against: disabling and re-enabling streams mints a new stream ARN, and a
// mapping keeps the ARN it was created with.
func TestT548b_TableNamesConsumersOfAnEarlierStream(t *testing.T) {
	const (
		table       = "orders-prod"
		oldStream   = "arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod/stream/2025-01-01T00:00:00.000"
		otherTable  = "arn:aws:dynamodb:us-east-1:123456789012:table/payments/stream/2026-01-01T00:00:00.000"
		consumerARN = "arn:aws:lambda:us-east-1:123456789012:function:orders-projector"
	)
	row := resource.Resource{
		ID:   table,
		Name: table,
		RawStruct: ddbtypes.TableDescription{
			TableName:       aws.String(table),
			TableArn:        aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod"),
			LatestStreamArn: aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod/stream/2026-06-01T00:00:00.000"),
		},
	}
	stub := &t548bESMStub{mappings: []lambdatypes.EventSourceMappingConfiguration{
		{FunctionArn: aws.String(consumerARN), EventSourceArn: aws.String(oldStream)},
		{FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:payments-projector"), EventSourceArn: aws.String(otherTable)},
	}}
	got := pairChecker(t, "ddb", "lambda")(context.Background(), &awsclient.ServiceClients{Lambda: stub, Region: "us-east-1"}, row, resource.ResourceCache{})
	if !slices.Contains(got.ResourceIDs(), "orders-projector") {
		t.Errorf("table %s lists %v, want the consumer of its earlier stream", table, got.ResourceIDs())
	}
	if len(got.ResourceIDs()) != 1 {
		t.Errorf("table %s lists %v, want only its own consumers", table, got.ResourceIDs())
	}
}

// ─── the asg → ami phrase ──────────────────────────────────────────────────

// TestT548b_ASGImagePhraseNamesBothFields pins that the contract text names
// every field the direction reads: a group on a launch configuration takes
// its image from LaunchConfiguration.ImageId, one on a launch template from
// LaunchTemplateData.ImageId.
func TestT548b_ASGImagePhraseNamesBothFields(t *testing.T) {
	phrase := pairDef(t, "asg", "ami").Distinct
	for _, field := range []string{"LaunchConfiguration.ImageId", "LaunchTemplateData.ImageId"} {
		if !strings.Contains(phrase, field) {
			t.Errorf("asg → ami reads %s but its phrase does not name it: %q", field, phrase)
		}
	}
}

// t548bESMStub answers ListEventSourceMappings with a fixed set, whatever is
// asked for.
type t548bESMStub struct {
	awsclient.LambdaAPI
	mappings []lambdatypes.EventSourceMappingConfiguration
}

func (s *t548bESMStub) ListEventSourceMappings(context.Context, *lambda.ListEventSourceMappingsInput, ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	return &lambda.ListEventSourceMappingsOutput{EventSourceMappings: s.mappings}, nil
}

// ─── a scan that read most of the population is a lower bound ──────────────

// t548bKinesisDDBStub answers DescribeKinesisStreamingDestination from a
// fixed table, and refuses the tables named in denied.
type t548bKinesisDDBStub struct {
	awsclient.DynamoDBAPI
	destinations map[string][]string
	denied       map[string]bool
}

func (s *t548bKinesisDDBStub) DescribeKinesisStreamingDestination(_ context.Context, in *dynamodb.DescribeKinesisStreamingDestinationInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	table := aws.ToString(in.TableName)
	if s.denied[table] {
		return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "denied " + table}
	}
	out := &dynamodb.DescribeKinesisStreamingDestinationOutput{TableName: in.TableName}
	for _, arn := range s.destinations[table] {
		out.KinesisDataStreamDestinations = append(out.KinesisDataStreamDestinations,
			ddbtypes.KinesisDataStreamDestination{StreamArn: aws.String(arn)})
	}
	return out, nil
}

// TestT548b_OneRefusedReadIsALowerBoundNotAFailedRow pins what a per-item
// scan answers when one item refuses its read: the tables it did read are a
// proven subset, so the count is a lower bound and the row stays drillable.
// Only a scan where every item refused establishes nothing and is an error.
func TestT548b_OneRefusedReadIsALowerBoundNotAFailedRow(t *testing.T) {
	const streamARN = "arn:aws:kinesis:us-east-1:123456789012:stream/orders-events"
	stream := resource.Resource{ID: "orders-events", Name: "orders-events", Fields: map[string]string{"stream_arn": streamARN}}
	ddbRows := []resource.Resource{{ID: "orders"}, {ID: "payments"}}
	cache := resource.ResourceCache{"ddb": resource.ResourceCacheEntry{Resources: ddbRows}}
	checker := pairChecker(t, "kinesis", "ddb")

	partial := &t548bKinesisDDBStub{
		destinations: map[string][]string{"orders": {streamARN}},
		denied:       map[string]bool{"payments": true},
	}
	got := checker(context.Background(), &awsclient.ServiceClients{DynamoDB: partial, Region: "us-east-1"}, stream, cache)
	if got.Err() != nil {
		t.Errorf("one refused table must not fail the row: %v", got.Err())
	}
	if !slices.Contains(got.ResourceIDs(), "orders") {
		t.Errorf("the table that answered is listed, got %v", got.ResourceIDs())
	}
	if !got.Truncated() {
		t.Errorf("the table that refused leaves the count a lower bound, got an exact %d", got.Count())
	}

	allDenied := &t548bKinesisDDBStub{denied: map[string]bool{"orders": true, "payments": true}}
	got = checker(context.Background(), &awsclient.ServiceClients{DynamoDB: allDenied, Region: "us-east-1"}, stream, cache)
	if got.Err() == nil {
		t.Errorf("every table refused its read, which establishes nothing: want an error, got %d", got.Count())
	}
}
