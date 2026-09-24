package unit_test

// A checker reads every place its relation can live in and counts what it
// finds there. A place it could not read leaves the row unknown; a proven 0
// says every place was read and held nothing.

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const t567MSKServerlessARN = "arn:aws:kafka:us-east-1:123456789012:cluster/acme-events-serverless/1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d-s1"

// TestT567MSKServerless_VpcConfigsHoldTheNetwork: a serverless cluster has no
// broker node group; its subnets and security groups are in
// Serverless.VpcConfigs.
func TestT567MSKServerless_VpcConfigsHoldTheNetwork(t *testing.T) {
	b := newRefBench(t)
	cluster := resource.Resource{ID: "acme-events-serverless", Name: "acme-events-serverless", Type: "msk", RawStruct: kafkatypes.Cluster{
		ClusterName: aws.String("acme-events-serverless"),
		ClusterArn:  aws.String(t567MSKServerlessARN),
		ClusterType: kafkatypes.ClusterTypeServerless,
		State:       kafkatypes.ClusterStateActive,
		Serverless: &kafkatypes.Serverless{
			VpcConfigs: []kafkatypes.VpcConfig{{
				SubnetIds:        []string{"subnet-0ccc333333333333c", "subnet-0ddd444444444444d"},
				SecurityGroupIds: []string{"sg-0bbb222222222222b"},
			}},
			ClientAuthentication: &kafkatypes.ServerlessClientAuthentication{Sasl: &kafkatypes.ServerlessSasl{Iam: &kafkatypes.Iam{Enabled: aws.Bool(true)}}},
		},
	}}
	want := map[string][]string{
		"sg":     {"sg-0bbb222222222222b"},
		"subnet": {"subnet-0ccc333333333333c", "subnet-0ddd444444444444d"},
		"vpc":    {"vpc-0abc123def456789a"},
	}
	for target, ids := range want {
		got := refChecker(t, "msk", target)(context.Background(), refClients(), cluster, b.cache)
		if !slices.Equal(sortedIDs(got), ids) || got.Truncated() {
			t.Errorf("serverless msk → %s = %s %v, want (%d) %v", target, t567Badge(got), sortedIDs(got), len(ids), ids)
		}
	}
}

// t567MSKFake answers ListScramSecrets from secrets, or refuses it.
type t567MSKFake struct {
	awsclient.MSKAPI
	secrets []string
	refuse  bool
	calls   int
}

func (f *t567MSKFake) ListScramSecrets(context.Context, *kafka.ListScramSecretsInput, ...func(*kafka.Options)) (*kafka.ListScramSecretsOutput, error) {
	f.calls++
	if f.refuse {
		return nil, t567Denied("kafka:ListScramSecrets on resource: " + t567MSKServerlessARN)
	}
	return &kafka.ListScramSecretsOutput{SecretArnList: f.secrets}, nil
}

func t567ProvisionedMSK(auth *kafkatypes.ClientAuthentication) resource.Resource {
	return resource.Resource{ID: "acme-events-prod", Name: "acme-events-prod", Type: "msk", RawStruct: kafkatypes.Cluster{
		ClusterName: aws.String("acme-events-prod"),
		ClusterArn:  aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/acme-events-prod/9f8e7d6c-5b4a-3921-8f7e-6d5c4b3a2910-3"),
		ClusterType: kafkatypes.ClusterTypeProvisioned,
		State:       kafkatypes.ClusterStateActive,
		Provisioned: &kafkatypes.Provisioned{
			NumberOfBrokerNodes: aws.Int32(3),
			BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
				ClientSubnets:  []string{"subnet-0ccc333333333333c"},
				SecurityGroups: []string{"sg-0aaa111111111111a"},
				InstanceType:   aws.String("kafka.m5.large"),
			},
			ClientAuthentication: auth,
		},
	}}
}

// TestT567MSKSecrets_OnlyASCRAMClusterHasSecrets: MSK associates Secrets
// Manager secrets with a cluster only for SASL/SCRAM, so a cluster without it
// has none and nothing needs to be asked.
func TestT567MSKSecrets_OnlyASCRAMClusterHasSecrets(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "msk", "secrets")
	scram := func(on bool) *kafkatypes.ClientAuthentication {
		return &kafkatypes.ClientAuthentication{Sasl: &kafkatypes.Sasl{
			Scram: &kafkatypes.Scram{Enabled: aws.Bool(on)},
			Iam:   &kafkatypes.Iam{Enabled: aws.Bool(!on)},
		}}
	}
	serverless := resource.Resource{ID: "acme-events-serverless", Name: "acme-events-serverless", Type: "msk", RawStruct: kafkatypes.Cluster{
		ClusterName: aws.String("acme-events-serverless"),
		ClusterArn:  aws.String(t567MSKServerlessARN),
		ClusterType: kafkatypes.ClusterTypeServerless,
		State:       kafkatypes.ClusterStateActive,
		Serverless: &kafkatypes.Serverless{
			ClientAuthentication: &kafkatypes.ServerlessClientAuthentication{Sasl: &kafkatypes.ServerlessSasl{Iam: &kafkatypes.Iam{Enabled: aws.Bool(true)}}},
		},
	}}
	for name, cluster := range map[string]resource.Resource{
		"IAM only":             t567ProvisionedMSK(scram(false)),
		"no client auth":       t567ProvisionedMSK(nil),
		"TLS only":             t567ProvisionedMSK(&kafkatypes.ClientAuthentication{Tls: &kafkatypes.Tls{Enabled: aws.Bool(true)}}),
		"serverless, IAM only": serverless,
	} {
		clients := refClients()
		fake := &t567MSKFake{MSKAPI: clients.MSK, refuse: true}
		clients.MSK = fake
		got := check(context.Background(), clients, cluster, b.cache)
		if badge := t567Badge(got); badge != "(0)" || fake.calls != 0 {
			t.Errorf("%s cluster → Secrets = %q after %d ListScramSecrets calls, want (0) after none", name, badge, fake.calls)
		}
	}

	clients := refClients()
	clients.MSK = &t567MSKFake{MSKAPI: clients.MSK, refuse: true}
	if got := check(context.Background(), clients, t567ProvisionedMSK(scram(true)), b.cache); got.State() != domain.RelatedUnknown {
		t.Errorf("SCRAM cluster with ListScramSecrets refused → Secrets = %q (state %s), want unknown", t567Badge(got), got.State())
	}

	clients = refClients()
	clients.MSK = &t567MSKFake{MSKAPI: clients.MSK, secrets: []string{"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/kafka/sasl-password-AbCdEf"}}
	got := check(context.Background(), clients, t567ProvisionedMSK(scram(true)), b.cache)
	if badge := t567Badge(got); badge != "(1)" || !slices.Equal(got.ResourceIDs(), []string{"prod/kafka/sasl-password"}) {
		t.Errorf("SCRAM cluster → Secrets = %s %v, want (1) [prod/kafka/sasl-password]", badge, got.ResourceIDs())
	}
}

func t567NodeGroup(lt *ekstypes.LaunchTemplateSpecification, remoteSG string) resource.Resource {
	ng := ekstypes.Nodegroup{
		NodegroupName:  aws.String("general-pool"),
		ClusterName:    aws.String("acme-prod"),
		NodegroupArn:   aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/general-pool/5ec8b6a2-1111-2222-3333-444455556666"),
		Status:         ekstypes.NodegroupStatusActive,
		CapacityType:   ekstypes.CapacityTypesOnDemand,
		NodeRole:       aws.String("arn:aws:iam::123456789012:role/acme-eks-node-role"),
		Subnets:        []string{"subnet-0ccc333333333333c"},
		LaunchTemplate: lt,
	}
	if lt == nil {
		ng.AmiType = ekstypes.AMITypesAl2023X8664Standard
		ng.ReleaseVersion = aws.String("1.31.3-20241121")
		ng.InstanceTypes = []string{"m5.large"}
		ng.DiskSize = aws.Int32(20)
	} else {
		ng.AmiType = ekstypes.AMITypesCustom
	}
	if remoteSG != "" {
		ng.RemoteAccess = &ekstypes.RemoteAccessConfig{Ec2SshKey: aws.String("acme-ops")}
		ng.Resources = &ekstypes.NodegroupResources{RemoteAccessSecurityGroup: aws.String(remoteSG)}
	}
	return resource.Resource{ID: "acme-prod/general-pool", Name: "general-pool", Type: "ng", RawStruct: ng,
		Fields: map[string]string{"nodegroup_name": "general-pool", "cluster_name": "acme-prod"}}
}

func t567LTClients(lt *t567LTEC2Fake) *awsclient.ServiceClients {
	clients := refClients()
	lt.EC2API = clients.EC2
	clients.EC2 = lt
	return clients
}

// TestT567NGSG_LaunchTemplateSecurityGroupsCount: a node group on a launch
// template gets its security groups from the template, not from a remote
// access config.
func TestT567NGSG_LaunchTemplateSecurityGroupsCount(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "ng", "sg")
	spec := &ekstypes.LaunchTemplateSpecification{Id: aws.String("lt-0a1b2c3d4e5f60001"), Name: aws.String("acme-general-pool"), Version: aws.String("3")}

	clients := t567LTClients(&t567LTEC2Fake{data: map[string]ec2types.ResponseLaunchTemplateData{
		"lt-0a1b2c3d4e5f60001": {ImageId: aws.String("ami-0eks111111111111a"), InstanceType: ec2types.InstanceTypeM5Large, SecurityGroupIds: []string{"sg-0bbb222222222222b"}},
	}})
	got := check(context.Background(), clients, t567NodeGroup(spec, ""), b.cache)
	if badge := t567Badge(got); badge != "(1)" || !slices.Equal(got.ResourceIDs(), []string{"sg-0bbb222222222222b"}) {
		t.Errorf("launch-template ng → Security Groups = %s %v, want (1) [sg-0bbb222222222222b]", badge, got.ResourceIDs())
	}

	clients = t567LTClients(&t567LTEC2Fake{denied: map[string]bool{"lt-0a1b2c3d4e5f60001": true}})
	if t567ProvenZero(check(context.Background(), clients, t567NodeGroup(spec, ""), b.cache)) {
		t.Errorf("launch-template ng with the template unreadable → Security Groups = (0), want no proven zero")
	}

	got = check(context.Background(), refClients(), t567NodeGroup(nil, "sg-0ddd444444444444d"), b.cache)
	if !slices.Contains(got.ResourceIDs(), "sg-0ddd444444444444d") || got.Truncated() {
		t.Errorf("remote-access ng → Security Groups = %s %v, want sg-0ddd444444444444d counted", t567Badge(got), got.ResourceIDs())
	}
}

// TestT567NGAMI_DefaultAMIIsNotNone: a node group with no launch template, or
// a template that names no image, runs the EKS-optimised AMI for its AmiType
// and release version. It has an image; the pivot cannot say it has none.
func TestT567NGAMI_DefaultAMIIsNotNone(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "ng", "ami")
	spec := &ekstypes.LaunchTemplateSpecification{Id: aws.String("lt-0a1b2c3d4e5f60001"), Name: aws.String("acme-general-pool"), Version: aws.String("3")}

	if got := check(context.Background(), refClients(), t567NodeGroup(nil, ""), b.cache); t567ProvenZero(got) {
		t.Errorf("ng on the EKS default AMI → AMIs = (0), want no proven zero")
	}

	clients := t567LTClients(&t567LTEC2Fake{data: map[string]ec2types.ResponseLaunchTemplateData{
		"lt-0a1b2c3d4e5f60001": {InstanceType: ec2types.InstanceTypeM5Large, SecurityGroupIds: []string{"sg-0bbb222222222222b"}},
	}})
	noImage := t567NodeGroup(spec, "")
	ng := noImage.RawStruct.(ekstypes.Nodegroup)
	ng.AmiType = ekstypes.AMITypesAl2023X8664Standard
	ng.ReleaseVersion = aws.String("1.31.3-20241121")
	noImage.RawStruct = ng
	if got := check(context.Background(), clients, noImage, b.cache); t567ProvenZero(got) {
		t.Errorf("ng whose template names no image → AMIs = (0), want no proven zero")
	}

	clients = t567LTClients(&t567LTEC2Fake{data: map[string]ec2types.ResponseLaunchTemplateData{
		"lt-0a1b2c3d4e5f60001": {ImageId: aws.String("ami-0eks111111111111a"), InstanceType: ec2types.InstanceTypeM5Large},
	}})
	got := check(context.Background(), clients, t567NodeGroup(spec, ""), b.cache)
	if badge := t567Badge(got); badge != "(1)" || !slices.Equal(got.ResourceIDs(), []string{"ami-0eks111111111111a"}) {
		t.Errorf("custom-AMI ng → AMIs = %s %v, want (1) [ami-0eks111111111111a]", badge, got.ResourceIDs())
	}
}

// t567S3EncFake answers GetBucketEncryption with one default-encryption rule.
type t567S3EncFake struct {
	awsclient.S3API
	rule s3types.ServerSideEncryptionByDefault
}

func (f *t567S3EncFake) GetBucketEncryption(context.Context, *s3.GetBucketEncryptionInput, ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error) {
	return &s3.GetBucketEncryptionOutput{ServerSideEncryptionConfiguration: &s3types.ServerSideEncryptionConfiguration{
		Rules: []s3types.ServerSideEncryptionRule{{ApplyServerSideEncryptionByDefault: &f.rule, BucketKeyEnabled: aws.Bool(true)}},
	}}, nil
}

// TestT567S3KMS_ManagedKeyWhenNoKeyIsNamed: SSE-KMS with no KMSMasterKeyID
// encrypts with the AWS managed key aws/s3, the same key the rule names when
// it spells alias/aws/s3 out.
func TestT567S3KMS_ManagedKeyWhenNoKeyIsNamed(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "s3", "kms")
	bucket := b.row(t, "s3", "a9s-demo-healthy")
	run := func(keyID *string) resource.RelatedCheckResult {
		clients := refClients()
		clients.S3 = &t567S3EncFake{S3API: clients.S3, rule: s3types.ServerSideEncryptionByDefault{SSEAlgorithm: s3types.ServerSideEncryptionAwsKms, KMSMasterKeyID: keyID}}
		return check(context.Background(), clients, bucket, b.cache)
	}

	named := run(aws.String("arn:aws:kms:us-east-1:123456789012:alias/aws/s3"))
	if t567ProvenZero(named) {
		t.Fatalf("bucket naming alias/aws/s3 → KMS Keys = (0); the managed key must be countable when named")
	}
	implicit := run(nil)
	if t567ProvenZero(implicit) || t567Badge(implicit) != t567Badge(named) || !slices.Equal(sortedIDs(implicit), sortedIDs(named)) {
		t.Errorf("SSE-KMS bucket with no key named → KMS Keys = %q %v, want %q %v as for alias/aws/s3", t567Badge(implicit), sortedIDs(implicit), t567Badge(named), sortedIDs(named))
	}

	if got := run(aws.String("arn:aws:kms:us-east-1:123456789012:key/c3d4e5f6-7890-12ab-cdef-333333333333")); !slices.Equal(sortedIDs(got), []string{"c3d4e5f6-7890-12ab-cdef-333333333333"}) {
		t.Errorf("bucket naming its customer key → KMS Keys = %s %v, want [c3d4e5f6-7890-12ab-cdef-333333333333]", t567Badge(got), sortedIDs(got))
	}
}

// TestT567AMICFN_NoStackTagIsNoSearch: CloudFormation records no stack on an
// image, and finding one means reading every stack's template. Without the
// stack-name tag nothing was searched.
func TestT567AMICFN_NoStackTagIsNoSearch(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "ami", "cfn")
	for _, ami := range b.byType["ami"] {
		img, ok := ami.RawStruct.(ec2types.Image)
		if !ok {
			if p, isPtr := ami.RawStruct.(*ec2types.Image); isPtr {
				img, ok = *p, true
			}
		}
		if !ok {
			t.Fatalf("demo ami %s carries no ec2types.Image", ami.ID)
		}
		tagged := slices.ContainsFunc(img.Tags, func(tag ec2types.Tag) bool { return aws.ToString(tag.Key) == "aws:cloudformation:stack-name" })
		got := check(context.Background(), refClients(), ami, b.cache)
		switch {
		case tagged && got.Count() != 1:
			t.Errorf("ami %s tagged with its stack → CloudFormation Stacks = %s, want (1)", ami.ID, t567Badge(got))
		case !tagged && got.State() != domain.RelatedUnknown:
			t.Errorf("ami %s with no stack tag → CloudFormation Stacks = %q (state %s), want unknown", ami.ID, t567Badge(got), got.State())
		}
	}
}

// TestT567PipelineEbRule_NoARNIsNoSearch: EventBridge finds the rules
// targeting a pipeline by the pipeline's ARN; without one nothing was asked.
func TestT567PipelineEbRule_NoARNIsNoSearch(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "pipeline", "eb-rule")
	pipeline := b.row(t, "pipeline", "acme-api-deploy")
	if got := check(context.Background(), refClients(), pipeline, b.cache); t567Badge(got) != "(1)" {
		t.Fatalf("demo pipeline acme-api-deploy → EventBridge Rules = %s, want (1)", t567Badge(got))
	}
	pipeline.Fields = maps.Clone(pipeline.Fields)
	pipeline.Fields["arn"] = ""
	if got := check(context.Background(), refClients(), pipeline, b.cache); got.State() != domain.RelatedUnknown {
		t.Errorf("pipeline with no ARN → EventBridge Rules = %q (state %s), want unknown", t567Badge(got), got.State())
	}
}

// t567APIGWFake answers GetIntegrations per API.
type t567APIGWFake struct {
	awsclient.APIGatewayV2API
	uris map[string][]string
}

func (f *t567APIGWFake) GetIntegrations(_ context.Context, in *apigatewayv2.GetIntegrationsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	out := &apigatewayv2.GetIntegrationsOutput{}
	for i, u := range f.uris[aws.ToString(in.ApiId)] {
		out.Items = append(out.Items, apigwv2types.Integration{
			IntegrationId:        aws.String("7h2k9q" + string(rune('a'+i))),
			IntegrationType:      apigwv2types.IntegrationTypeAwsProxy,
			IntegrationUri:       aws.String(u),
			PayloadFormatVersion: aws.String("2.0"),
		})
	}
	return out, nil
}

// TestT567LambdaAPIGW_IntegrationNamesTheFunction: which function an API
// invokes is written in its integrations. An API whose name shares nothing
// with the function still invokes it when an integration says so.
func TestT567LambdaAPIGW_IntegrationNamesTheFunction(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "lambda", "apigw")
	fn := b.row(t, "lambda", "process-orders")
	clients := refClients()
	clients.APIGatewayV2 = &t567APIGWFake{APIGatewayV2API: clients.APIGatewayV2, uris: map[string][]string{
		"klm901nop2": {refLambdaInvokeURI("arn:aws:lambda:us-east-1:123456789012:function:process-orders")},
	}}
	got := check(context.Background(), clients, fn, b.cache)
	if t567ProvenZero(got) {
		t.Fatalf("lambda process-orders → API Gateways = (0), but internal-service-api integrates it")
	}
	if got.State() == domain.RelatedResolved && !slices.Contains(got.ResourceIDs(), "klm901nop2") {
		t.Errorf("lambda process-orders → API Gateways = %s %v, want internal-service-api (klm901nop2) among them", t567Badge(got), got.ResourceIDs())
	}

	other := b.row(t, "lambda", "image-thumbnail-gen")
	if got := check(context.Background(), clients, other, b.cache); slices.Contains(got.ResourceIDs(), "klm901nop2") {
		t.Errorf("lambda image-thumbnail-gen → API Gateways %v names an API whose integration invokes another function", got.ResourceIDs())
	}
}
