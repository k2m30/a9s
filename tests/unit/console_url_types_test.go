// console_url_types_test.go — per-type contract for catalog.ResourceTypeDef.
// ConsoleURL (spec: console-url-spec.md, per-type builder table). Covers all
// 70 top-level catalog types: a completeness gate (every type except
// ct-events has a non-nil builder), a table-driven expected-URL test per
// type driven by the shared demo fixtures, the aurora/docdb and
// REGIONAL/CLOUDFRONT branch cases, nil-safety for RawStruct-dependent
// builders, and hostile-input percent-encoding.
//
// Expected URLs are built from the spec's literal per-type URL shape plus
// the fixture's own real ID/Name/Fields values captured via the type's real
// Wave-1 Fetcher (drainDemoFixtures, shared with qa_demo_pivot_coverage_test.go
// and qa_demo_state_coverage_test.go) — never by calling consolelink helpers,
// so a broken builder cannot pass by tautology.
package unit_test

import (
	"fmt"
	"net/url"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const consoleTestAccountID = "123456789012"

// ─── Shared helpers ──────────────────────────────────────────────────────────

// consoleURLTypeDef returns the installed catalog entry for shortName,
// failing loudly if the type is not registered.
func consoleURLTypeDef(t *testing.T, shortName string) resource.ResourceTypeDef {
	t.Helper()
	td := catalog.Find(shortName)
	if td == nil {
		t.Fatalf("catalog.Find(%q) returned nil — type not registered", shortName)
	}
	return *td
}

// consoleURLRows drains shortName's real demo fixtures via its own Wave-1
// Fetcher (same mechanism as drainDemoFixtures in
// qa_demo_pivot_coverage_test.go).
func consoleURLRows(t *testing.T, clients *awsclient.ServiceClients, shortName string) []resource.Resource {
	t.Helper()
	td := consoleURLTypeDef(t, shortName)
	rows, ok := drainDemoFixtures(t, td, clients)
	if !ok || len(rows) == 0 {
		t.Fatalf("%s: no demo fixtures available to drive this test", shortName)
	}
	return rows
}

// consoleURLRowByID finds the fixture row with the given ID, failing loudly
// if it is absent — a silent fallback to rows[0] would make the branch-case
// assertions below meaningless if the named fixture is ever renamed/removed.
func consoleURLRowByID(t *testing.T, rows []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("fixture row with ID %q not found among %d rows", id, len(rows))
	return resource.Resource{}
}

// ─── Completeness: every top-level type except ct-events has a builder ─────

func TestConsoleURL_RegisteredForEveryTopLevelType_ExceptCtEvents(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		if td.ShortName == "ct-events" {
			if td.ConsoleURL != nil {
				t.Error("ct-events: ConsoleURL should be nil (no per-event console page exists) — deliberate per spec")
			}
			continue
		}
		if td.ConsoleURL == nil {
			t.Errorf("%s: ConsoleURL is nil; every top-level type except ct-events must define one", td.ShortName)
		}
	}
}

// ─── Per-type expected-URL table ────────────────────────────────────────────

type consoleURLCase struct {
	name      string // subtest name
	shortName string
	pickID    string // fixture row ID to select; "" means use rows[0]
	want      func(r resource.Resource) string
}

func TestConsoleURL_PerType(t *testing.T) {
	clients := demo.NewServiceClients()
	r := demo.DemoRegion // "us-east-1" — must match the region literal baked into fixture ARNs
	acct := consoleTestAccountID

	cases := []consoleURLCase{
		{
			name: "ec2", shortName: "ec2", pickID: "i-0a1b2c3d4e5f60001",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#InstanceDetails:instanceId=%s", r, r, row.ID)
			},
		},
		{
			name: "asg", shortName: "asg", pickID: "acme-web-prod-asg",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#AutoScalingGroupDetails:id=%s;view=details", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "ebs", shortName: "ebs", pickID: "vol-0a1b2c3d4e5f60001",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#VolumeDetails:volumeId=%s", r, r, row.ID)
			},
		},
		{
			name: "ebs-snap", shortName: "ebs-snap", pickID: "snap-0a1b2c3d4e5f60001",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#Snapshots:snapshotId=%s", r, r, row.ID)
			},
		},
		{
			name: "ami", shortName: "ami", pickID: "ami-0a1b2c3d4e5f60001",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#ImageDetails:imageId=%s", r, r, row.ID)
			},
		},
		{
			name: "lt", shortName: "lt", pickID: "lt-0prodweb1111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#LaunchTemplateDetails:launchTemplateId=%s", r, r, row.ID)
			},
		},
		{
			name: "lambda", shortName: "lambda", pickID: "api-gateway-authorizer",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/lambda/home?region=%s#/functions/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "ecs", shortName: "ecs", pickID: "acme-services",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/clusters/%s?region=%s", r, url.PathEscape(row.ID), r)
			},
		},
		{
			// ServiceArn is only present on RawStruct (ecstypes.Service), not
			// in Fields — the literal ARN below was captured from the same
			// demo fixture (core/demo/fixtures/ecs.go) building "api-gateway".
			name: "ecs-svc", shortName: "ecs-svc", pickID: "api-gateway",
			want: func(row resource.Resource) string {
				arn := "arn:aws:ecs:us-east-1:123456789012:service/acme-services/api-gateway"
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/redirect?arn=%s&region=%s", r, url.QueryEscape(arn), r)
			},
		},
		{
			// TaskArn is only present on RawStruct (ecstypes.Task), not Fields.
			name: "ecs-task", shortName: "ecs-task", pickID: "a1b2c3d4e5f6a1b2c3d4e5f6",
			want: func(row resource.Resource) string {
				arn := "arn:aws:ecs:us-east-1:123456789012:task/acme-services/a1b2c3d4e5f6a1b2c3d4e5f6"
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/redirect?arn=%s&region=%s", r, url.QueryEscape(arn), r)
			},
		},
		{
			name: "eks", shortName: "eks", pickID: "acme-prod",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/eks/home?region=%s#/clusters/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "ng", shortName: "ng", pickID: "general-pool",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/eks/home?region=%s#/clusters/%s/nodegroups/%s", r, r, url.PathEscape(row.Fields["cluster_name"]), url.PathEscape(row.ID))
			},
		},
		{
			name: "elb", shortName: "elb", pickID: "acme-prod-web",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#LoadBalancer:loadBalancerArn=%s", r, r, row.Fields["load_balancer_arn"])
			},
		},
		{
			name: "tg", shortName: "tg", pickID: "acme-web-tg",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#TargetGroup:targetGroupArn=%s", r, r, row.Fields["target_group_arn"])
			},
		},
		{
			name: "sg", shortName: "sg", pickID: "sg-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#SecurityGroup:groupId=%s", r, r, row.ID)
			},
		},
		{
			name: "vpc", shortName: "vpc", pickID: "vpc-0abc123def456789a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#VpcDetails:VpcId=%s", r, r, row.ID)
			},
		},
		{
			name: "subnet", shortName: "subnet", pickID: "subnet-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#SubnetDetails:subnetId=%s", r, r, row.ID)
			},
		},
		{
			name: "rtb", shortName: "rtb", pickID: "rtb-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#RouteTableDetails:RouteTableId=%s", r, r, row.ID)
			},
		},
		{
			name: "nat", shortName: "nat", pickID: "nat-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#NatGatewayDetails:natGatewayId=%s", r, r, row.ID)
			},
		},
		{
			name: "igw", shortName: "igw", pickID: "igw-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#InternetGateway:internetGatewayId=%s", r, r, row.ID)
			},
		},
		{
			name: "eip", shortName: "eip", pickID: "eipalloc-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#ElasticIpDetails:AllocationId=%s", r, r, row.ID)
			},
		},
		{
			name: "vpce", shortName: "vpce", pickID: "vpce-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#EndpointDetails:vpcEndpointId=%s", r, r, row.ID)
			},
		},
		{
			name: "tgw", shortName: "tgw", pickID: "tgw-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#TransitGateways:filter=%s", r, r, row.ID)
			},
		},
		{
			name: "eni", shortName: "eni", pickID: "eni-0aaa111111111111a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/home?region=%s#NetworkInterface:networkInterfaceId=%s", r, r, row.ID)
			},
		},
		{
			name: "vpc-peer", shortName: "vpc-peer", pickID: "pcx-0prodpeershared1a",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/vpc/home?region=%s#PeeringConnectionDetails:VpcPeeringConnectionId=%s", r, r, row.ID)
			},
		},
		{
			// No ?region= param at all — as emitted (spec's documented gap).
			name: "transfer", shortName: "transfer", pickID: "broken-transfer-start-failed",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/transfer/home#/servers/%s", r, row.ID)
			},
		},
		{
			name: "dbi", shortName: "dbi", pickID: "prod-dbi-1",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/rds/home?region=%s#database:id=%s;is-cluster=false", r, r, url.PathEscape(row.ID))
			},
		},
		{
			// docdb branch: RS docdbtypes.DBCluster.
			name: "dbc (docdb)", shortName: "dbc", pickID: "acme-docdb-prod",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/docdb/home?region=%s#cluster-details/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			// aurora branch: RS rdstypes.DBCluster, Engine != neptune-prefixed.
			name: "dbc (aurora)", shortName: "dbc", pickID: "prod-aurora-cluster",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/rds/home?region=%s#database:id=%s;is-cluster=true", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "dbi-snap", shortName: "dbi-snap", pickID: "rds:prod-dbi-1-2026-04-15",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/rds/home?region=%s#db-snapshot:id=%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "dbc-snap", shortName: "dbc-snap", pickID: "rds:acme-docdb-prod-2026-03-20",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/rds/home?region=%s#db-snapshot:id=%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "s3", shortName: "s3", pickID: "a9s-demo-healthy",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://console.aws.amazon.com/s3/buckets/%s", url.PathEscape(row.ID))
			},
		},
		{
			name: "redis", shortName: "redis", pickID: "prod-redis-sessions",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/elasticache/home?region=%s#/redis/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "ddb", shortName: "ddb", pickID: "analytics-deleting",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/dynamodbv2/home?region=%s#table?name=%s", r, r, url.QueryEscape(row.ID))
			},
		},
		{
			name: "opensearch", shortName: "opensearch", pickID: "staging-analytics",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/aos/home?region=%s#opensearch/domains/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "redshift", shortName: "redshift", pickID: "acme-warehouse",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/redshiftv2/home?region=%s#cluster-details:cluster=%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "efs", shortName: "efs", pickID: "fs-0prod1234abcd5678",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/efs/home?region=%s#/file-systems/%s", r, r, row.ID)
			},
		},
		{
			name: "alarm", shortName: "alarm", pickID: "api-high-error-rate",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#alarmsV2:alarm/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			// Slash encoding: pesc(id) turns "/" into "%2F" — as emitted.
			name: "logs", shortName: "logs", pickID: "/aws/lambda/api-gateway-authorizer",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/cloudwatch/home?region=%s#logsV2:log-groups/log-group/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "trail", shortName: "trail", pickID: "acme-management-trail",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/cloudtrailv2/home?region=%s#/trails/%s", r, r, row.Fields["trail_arn"])
			},
		},
		{
			name: "sqs", shortName: "sqs", pickID: "order-processing-queue",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/sqs/v3/home?region=%s#/queues/%s", r, r, row.Fields["arn"])
			},
		},
		{
			// sns: id IS the topic ARN.
			name: "sns", shortName: "sns", pickID: "arn:aws:sns:us-east-1:123456789012:alarm-notifications",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/sns/v3/home?region=%s#/topic/%s", r, r, row.ID)
			},
		},
		{
			// sns-sub: id IS the subscription ARN.
			name: "sns-sub", shortName: "sns-sub", pickID: "arn:aws:sns:us-east-1:123456789012:alarm-notifications:a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/sns/v3/home?region=%s#/subscription/%s", r, r, row.ID)
			},
		},
		{
			name: "eb", shortName: "eb", pickID: "e-acmeprodapi",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/elasticbeanstalk/home?region=%s#/environment/dashboard?applicationName=%s&environmentName=%s",
					r, r, url.QueryEscape(row.Fields["application_name"]), url.QueryEscape(row.Fields["environment_name"]))
			},
		},
		{
			name: "eb-rule", shortName: "eb-rule", pickID: "nightly-db-backup",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/events/home?region=%s#/eventbus/%s/rules/%s",
					r, r, url.PathEscape(row.Fields["event_bus"]), url.PathEscape(row.ID))
			},
		},
		{
			name: "kinesis", shortName: "kinesis", pickID: "clickstream-ingest",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/kinesis/home?region=%s#/streams/details/%s/monitoring", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "msk", shortName: "msk", pickID: "acme-events-prod",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/msk/home?region=%s#/cluster/%s/view?tabId=details", r, r, url.QueryEscape(row.Fields["cluster_arn"]))
			},
		},
		{
			name: "sfn", shortName: "sfn", pickID: "order-fulfillment-workflow",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/states/home?region=%s#/statemachines/view/%s", r, r, row.Fields["arn"])
			},
		},
		{
			name: "ses", shortName: "ses", pickID: "acme-corp.com",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ses/home?region=%s#/identities/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			// Friendly name only — no "-XXXXXX" AWS-generated suffix (secrets.go:69-70).
			name: "secrets", shortName: "secrets", pickID: "prod/docdb/acme-docdb-prod",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/secretsmanager/secret?region=%s&name=%s", r, r, url.QueryEscape(row.ID))
			},
		},
		{
			// Leading "/" trimmed, remaining slashes kept raw — as emitted.
			name: "ssm", shortName: "ssm", pickID: "/acme/prod/app/config",
			want: func(row resource.Resource) string {
				n := row.ID[1:] // TrimPrefix(id, "/") — id is known to start with "/"
				return fmt.Sprintf("https://%s.console.aws.amazon.com/systems-manager/parameters/%s/description?region=%s", r, n, r)
			},
		},
		{
			name: "kms", shortName: "kms", pickID: "d4e5f6a7-bcde-1234-5678-aabbccddeeff",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/kms/home?region=%s#/kms/keys/%s", r, r, row.ID)
			},
		},
		{
			// "/hostedzone/" prefix stripped (fetcher keeps it, r53.go:40-42); Global (no region subdomain).
			name: "r53", shortName: "r53", pickID: "/hostedzone/Z0123456789ABCDEFGHIJ",
			want: func(row resource.Resource) string {
				z := row.ID[len("/hostedzone/"):]
				return fmt.Sprintf("https://console.aws.amazon.com/route53/v2/hostedzones#ListRecordSets/%s", z)
			},
		},
		{
			name: "cf", shortName: "cf", pickID: "E1A2B3C4D5E6F7",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://console.aws.amazon.com/cloudfront/v4/home#/distributions/%s", row.ID)
			},
		},
		{
			// id IS the cert ARN; uuid = last "/" segment.
			name: "acm", shortName: "acm", pickID: "arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/acm/home?region=%s#/certificates/a1b2c3d4-5678-90ab-cdef-111111111111", r, r)
			},
		},
		{
			// HTTP protocol -> the non-REST ("else") branch. The REST branch
			// has no demo fixture witness (all demo apigw rows are HTTP or
			// WEBSOCKET) — see TestConsoleURL_Apigw_RestProtocolBranch below
			// for a synthetic-row test of that branch.
			name: "apigw (HTTP, else branch)", shortName: "apigw", pickID: "abc123def4",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/apigateway/main/api-detail?api=%s&region=%s", r, row.ID, r)
			},
		},
		{
			name: "role", shortName: "role", pickID: "acme-eks-node-role",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://console.aws.amazon.com/iam/home#/roles/details/%s", url.PathEscape(row.ID))
			},
		},
		{
			// Arn is only present on RawStruct (iamtypes.Policy), not Fields —
			// the literal ARN below was captured from the same demo fixture
			// (core/demo/fixtures/iam.go) building "acme-s3-read-only".
			name: "policy", shortName: "policy", pickID: "acme-s3-read-only",
			want: func(row resource.Resource) string {
				arn := "arn:aws:iam::123456789012:policy/acme-s3-read-only"
				return fmt.Sprintf("https://console.aws.amazon.com/iam/home#/policies/details/%s", url.QueryEscape(arn))
			},
		},
		{
			name: "iam-user", shortName: "iam-user", pickID: "alice.johnson",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://console.aws.amazon.com/iam/home#/users/details/%s", url.PathEscape(row.ID))
			},
		},
		{
			name: "iam-group", shortName: "iam-group", pickID: "admins",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://console.aws.amazon.com/iam/home#/groups/details/%s", url.PathEscape(row.ID))
			},
		},
		{
			// StackId is only present on RawStruct (cfntypes.StackSummary),
			// not Fields — literal captured from core/demo/fixtures/cfn.go
			// building "acme-vpc-stack".
			name: "cfn", shortName: "cfn", pickID: "acme-vpc-stack",
			want: func(row resource.Resource) string {
				stackArn := "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-vpc-stack/11111111-1111-1111-1111-111111111111"
				return fmt.Sprintf("https://%s.console.aws.amazon.com/cloudformation/home?region=%s#/stacks/stackinfo?stackId=%s", r, r, stackArn)
			},
		},
		{
			name: "pipeline", shortName: "pipeline", pickID: "acme-api-deploy",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/codesuite/codepipeline/pipelines/%s/view?region=%s", r, url.PathEscape(row.ID), r)
			},
		},
		{
			// Arn (for AccountFromARN) is only present on RawStruct
			// (cbtypes.Project), not Fields — literal captured from
			// core/demo/fixtures/codebuild.go building "acme-api-build".
			name: "cb", shortName: "cb", pickID: "acme-api-build",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/codesuite/codebuild/%s/projects/%s", r, consoleTestAccountID, url.PathEscape(row.ID))
			},
		},
		{
			// Repo name contains a "/" — path-escaped.
			name: "ecr", shortName: "ecr", pickID: "acme/api-service",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/ecr/repositories/%s/?region=%s", r, url.PathEscape(row.ID), r)
			},
		},
		{
			// domain_owner present directly in Fields — the AccountFromARN
			// fallback branch is covered separately by
			// TestConsoleURL_Codeartifact_AccountFallsBackToARNWhenDomainOwnerMissing.
			name: "codeartifact", shortName: "codeartifact", pickID: "acme-npm",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/codesuite/codeartifact/d/%s/%s/r/%s",
					r, row.Fields["domain_owner"], url.PathEscape(row.Fields["domain_name"]), url.PathEscape(row.ID))
			},
		},
		{
			name: "glue", shortName: "glue", pickID: "acme-etl-orders",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/gluestudio/home?region=%s#/editor/job/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "athena", shortName: "athena", pickID: "primary",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/athena/home?region=%s#/workgroups/details/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "mwaa", shortName: "mwaa", pickID: "broken-airflow-create-failed",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/mwaa/home?region=%s#environments/%s", r, r, url.PathEscape(row.ID))
			},
		},
		{
			name: "backup", shortName: "backup", pickID: "11111111-1111-1111-1111-111111111111",
			want: func(row resource.Resource) string {
				return fmt.Sprintf("https://%s.console.aws.amazon.com/backup/home?region=%s#/backupplan/details/%s", r, r, row.ID)
			},
		},
	}

	seenShortNames := map[string]bool{
		"ct-events": true, // deliberately excluded (nil ConsoleURL)
		"waf":       true, // covered separately: TestConsoleURL_Waf_RegionalUsesGivenRegion / TestConsoleURL_Waf_CloudfrontForcesUsEast1RegardlessOfSessionRegion
	}
	for _, c := range cases {
		seenShortNames[c.shortName] = true
	}
	for _, td := range resource.AllResourceTypes() {
		if !seenShortNames[td.ShortName] {
			t.Errorf("type %q has no case in TestConsoleURL_PerType's table — every top-level type must be covered", td.ShortName)
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := consoleURLRows(t, clients, c.shortName)
			row := rows[0]
			if c.pickID != "" {
				row = consoleURLRowByID(t, rows, c.pickID)
			}
			td := consoleURLTypeDef(t, c.shortName)
			if td.ConsoleURL == nil {
				t.Fatalf("%s: ConsoleURL is nil", c.shortName)
			}
			got := td.ConsoleURL(row, r, acct)
			want := c.want(row)
			if got != want {
				t.Errorf("%s: ConsoleURL(%q) =\n  %q\nwant\n  %q", c.shortName, row.ID, got, want)
			}
		})
	}
}

// ─── waf: REGIONAL vs CLOUDFRONT branch (region is NOT us-east-1, to prove
// the CLOUDFRONT branch truly hardcodes us-east-1 rather than merely
// echoing back whatever region happened to be passed in) ───────────────────

func TestConsoleURL_Waf_RegionalUsesGivenRegion(t *testing.T) {
	clients := demo.NewServiceClients()
	rows := consoleURLRows(t, clients, "waf")
	row := consoleURLRowByID(t, rows, "a1b2c3d4-5678-90ab-cdef-111111111111") // acme-prod-api-waf, scope=REGIONAL

	td := consoleURLTypeDef(t, "waf")
	got := td.ConsoleURL(row, "eu-west-1", consoleTestAccountID)
	want := fmt.Sprintf("https://eu-west-1.console.aws.amazon.com/wafv2-pro/protections/%s/%s?panel=protectionPackHome&region=eu-west-1&scope=regional",
		url.PathEscape(row.Name), row.ID)
	if got != want {
		t.Errorf("waf REGIONAL ConsoleURL =\n  %q\nwant\n  %q", got, want)
	}
}

func TestConsoleURL_Waf_CloudfrontForcesUsEast1RegardlessOfSessionRegion(t *testing.T) {
	clients := demo.NewServiceClients()
	rows := consoleURLRows(t, clients, "waf")
	row := consoleURLRowByID(t, rows, "a1b2c3d4-5678-90ab-cdef-222222222222") // acme-cloudfront-waf, scope=CLOUDFRONT

	td := consoleURLTypeDef(t, "waf")
	// Session region is eu-west-1 — the CLOUDFRONT branch must still force
	// us-east-1 in both host and query, per spec.
	got := td.ConsoleURL(row, "eu-west-1", consoleTestAccountID)
	want := fmt.Sprintf("https://us-east-1.console.aws.amazon.com/wafv2-pro/protections/%s/%s?panel=protectionPackHome&region=us-east-1&scope=global",
		url.PathEscape(row.Name), row.ID)
	if got != want {
		t.Errorf("waf CLOUDFRONT ConsoleURL =\n  %q\nwant\n  %q", got, want)
	}
}

// ─── apigw: REST protocol branch (no demo fixture reaches it — synthetic row) ─

func TestConsoleURL_Apigw_RestProtocolBranch(t *testing.T) {
	td := consoleURLTypeDef(t, "apigw")
	row := domain.Resource{
		ID:     "rest123abc",
		Name:   "acme-legacy-rest-api",
		Fields: map[string]string{"api_id": "rest123abc", "protocol": "REST"},
	}
	got := td.ConsoleURL(row, "us-east-1", consoleTestAccountID)
	want := "https://us-east-1.console.aws.amazon.com/apigateway/home?region=us-east-1#/apis/rest123abc"
	if got != want {
		t.Errorf("apigw REST branch ConsoleURL = %q, want %q", got, want)
	}
}

// ─── codeartifact: AccountFromARN fallback when domain_owner is absent ──────

func TestConsoleURL_Codeartifact_AccountFallsBackToARNWhenDomainOwnerMissing(t *testing.T) {
	td := consoleURLTypeDef(t, "codeartifact")
	row := domain.Resource{
		ID:   "acme-terraform",
		Name: "acme-terraform",
		Fields: map[string]string{
			"repo_name":   "acme-terraform",
			"domain_name": "acme-artifacts",
			"arn":         "arn:aws:codeartifact:us-east-1:987654321098:repository/acme-artifacts/acme-terraform",
			// domain_owner deliberately absent.
		},
	}
	got := td.ConsoleURL(row, "us-east-1", consoleTestAccountID)
	want := "https://us-east-1.console.aws.amazon.com/codesuite/codeartifact/d/987654321098/acme-artifacts/r/acme-terraform"
	if got != want {
		t.Errorf("codeartifact AccountFromARN-fallback ConsoleURL = %q, want %q", got, want)
	}
}

// ─── Nil-safety for RawStruct-dependent builders ────────────────────────────

func consoleURLNoPanic(t *testing.T, shortName string, r domain.Resource) string {
	t.Helper()
	td := consoleURLTypeDef(t, shortName)
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("%s: ConsoleURL panicked on a resource with nil RawStruct/missing Fields: %v", shortName, rec)
		}
	}()
	return td.ConsoleURL(r, "us-east-1", consoleTestAccountID)
}

// TestConsoleURL_NilSafety_MissingFieldsReturnsEmpty covers the negative
// half of the RawStruct-to-Fields migration contract: ecs-svc, elb, cfn, and
// eb all now read their identifying ARN from r.Fields (RawStruct doesn't
// survive the on-disk cache, so Fields is the single truth source) instead
// of a RawStruct type assertion. A Resource missing the required Fields key
// (nil RawStruct here too, since neither is consulted for identity anymore)
// must still return "" and never panic.
func TestConsoleURL_NilSafety_MissingFieldsReturnsEmpty(t *testing.T) {
	cases := []struct {
		shortName string
		resource  domain.Resource
	}{
		{"ecs-svc", domain.Resource{ID: "svc-missing-arn", RawStruct: nil, Fields: nil}},
		{"elb", domain.Resource{ID: "elb-missing-arn", RawStruct: nil, Fields: nil}},
		{"cfn", domain.Resource{ID: "stack-missing-id", RawStruct: nil, Fields: nil}},
		{"eb", domain.Resource{ID: "env-missing-names", RawStruct: nil, Fields: nil}},
	}
	for _, c := range cases {
		if got := consoleURLNoPanic(t, c.shortName, c.resource); got != "" {
			t.Errorf("%s: ConsoleURL on a resource with nil RawStruct/missing Fields = %q, want \"\"", c.shortName, got)
		}
	}
}

// TestConsoleURL_NilSafety_PopulatedFieldsResolvesWithNilRawStruct covers the
// positive half of the same contract: since RawStruct doesn't survive the
// on-disk cache, a cache-loaded Resource always has RawStruct == nil — the
// whole point of the migration is that ConsoleURL must still resolve for
// such a row as long as its Fields carry the identifying ARN.
func TestConsoleURL_NilSafety_PopulatedFieldsResolvesWithNilRawStruct(t *testing.T) {
	cases := []struct {
		shortName string
		resource  domain.Resource
		want      string
	}{
		{
			shortName: "ecs-svc",
			resource: domain.Resource{ID: "svc-cached", RawStruct: nil, Fields: map[string]string{
				"arn": "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-svc",
			}},
			want: "https://us-east-1.console.aws.amazon.com/ecs/v2/redirect?arn=" +
				url.QueryEscape("arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-svc") + "&region=us-east-1",
		},
		{
			// elb reads Fields["load_balancer_arn"], not "arn" — that
			// duplicate key was deliberately removed for a single truth
			// source (core/aws/elb.go / catalog_networking.go). This is
			// also the exact shape an old cached row carries: it was
			// written before "arn" ever existed, so a resolver that
			// depended on "arn" would silently break every pre-existing
			// cache file.
			shortName: "elb",
			resource: domain.Resource{ID: "elb-cached", RawStruct: nil, Fields: map[string]string{
				"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-lb/abc123",
			}},
			want: "https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#LoadBalancer:loadBalancerArn=" +
				"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-lb/abc123",
		},
		{
			shortName: "cfn",
			resource: domain.Resource{ID: "stack-cached", RawStruct: nil, Fields: map[string]string{
				"arn": "arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/def456",
			}},
			want: "https://us-east-1.console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/stackinfo?stackId=" +
				"arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/def456",
		},
		{
			shortName: "eb",
			resource: domain.Resource{ID: "env-cached", RawStruct: nil, Fields: map[string]string{
				"environment_arn": "arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/my-app/my-env",
			}},
			want: "https://us-east-1.console.aws.amazon.com/elasticbeanstalk/home?region=us-east-1#/environment/dashboard?applicationName=" +
				url.QueryEscape("my-app") + "&environmentName=" + url.QueryEscape("my-env"),
		},
	}
	for _, c := range cases {
		got := consoleURLNoPanic(t, c.shortName, c.resource)
		if got != c.want {
			t.Errorf("%s: ConsoleURL on a nil-RawStruct/populated-Fields resource =\n  %q\nwant\n  %q", c.shortName, got, c.want)
		}
	}
}

// ─── Hostile-input encoding: spaces, unicode, slashes ───────────────────────

func TestConsoleURL_HostileInput_AlarmNameEncoding(t *testing.T) {
	td := consoleURLTypeDef(t, "alarm")
	row := domain.Resource{ID: "My Über Alarm/Test", Name: "My Über Alarm/Test"}
	got := td.ConsoleURL(row, "us-east-1", consoleTestAccountID)
	want := "https://us-east-1.console.aws.amazon.com/cloudwatch/home?region=us-east-1#alarmsV2:alarm/" + url.PathEscape(row.ID)
	if got != want {
		t.Errorf("alarm hostile-input ConsoleURL = %q, want %q", got, want)
	}
}

func TestConsoleURL_HostileInput_SSMParameterNameEncoding(t *testing.T) {
	td := consoleURLTypeDef(t, "ssm")
	row := domain.Resource{ID: "/acme/prod/app/config with space/ключ", Name: "config with space/ключ"}
	got := td.ConsoleURL(row, "us-east-1", consoleTestAccountID)
	// ssm keeps slashes AND everything else raw (as emitted) — only the
	// leading "/" is trimmed.
	want := "https://us-east-1.console.aws.amazon.com/systems-manager/parameters/acme/prod/app/config with space/ключ/description?region=us-east-1"
	if got != want {
		t.Errorf("ssm hostile-input ConsoleURL = %q, want %q", got, want)
	}
}

func TestConsoleURL_HostileInput_SecretsNameEncoding(t *testing.T) {
	td := consoleURLTypeDef(t, "secrets")
	row := domain.Resource{ID: "café secret/data with space", Name: "café secret/data with space"}
	got := td.ConsoleURL(row, "us-east-1", consoleTestAccountID)
	want := "https://us-east-1.console.aws.amazon.com/secretsmanager/secret?region=us-east-1&name=" + url.QueryEscape(row.ID)
	if got != want {
		t.Errorf("secrets hostile-input ConsoleURL = %q, want %q", got, want)
	}
}

// ─── Incomplete-row hardening (Codex review) ────────────────────────────────
//
// A related-panel stub/ID-only resource (no StubCreator registered for its
// type) carries just ID+Type — Name and Fields are zero-valued. Before the
// Codex-driven fix, waf/codeartifact/dbc's builders would either panic-free
// but WRONG-guess a URL (dbc defaulting to the RDS console for an empty
// engine) or build a structurally-empty-but-non-empty path segment
// (waf/codeartifact with an empty Name/domain_name). All three now guard
// explicitly and return "" instead.

func TestConsoleURL_IncompleteRow_Waf_EmptyNameOrScopeReturnsEmpty(t *testing.T) {
	td := consoleURLTypeDef(t, "waf")
	cases := []struct {
		name string
		row  domain.Resource
	}{
		{"empty Name", domain.Resource{ID: "acl-uuid-1", Name: "", Fields: map[string]string{"scope": "REGIONAL"}}},
		{"empty scope", domain.Resource{ID: "acl-uuid-2", Name: "some-acl", Fields: map[string]string{"scope": ""}}},
		{"both empty (bare ID-only stub)", domain.Resource{ID: "acl-uuid-3"}},
	}
	for _, c := range cases {
		if got := td.ConsoleURL(c.row, "us-east-1", consoleTestAccountID); got != "" {
			t.Errorf("waf %s: ConsoleURL = %q, want \"\"", c.name, got)
		}
	}
}

func TestConsoleURL_IncompleteRow_Codeartifact_EmptyDomainNameReturnsEmpty(t *testing.T) {
	td := consoleURLTypeDef(t, "codeartifact")
	// domain_owner and arn are present (account resolves fine) — only
	// domain_name is missing, which used to still build a URL with an empty
	// path segment ("d/123456789012//r/my-repo").
	row := domain.Resource{ID: "my-repo", Fields: map[string]string{
		"domain_owner": "123456789012",
		"domain_name":  "",
	}}
	if got := td.ConsoleURL(row, "us-east-1", consoleTestAccountID); got != "" {
		t.Errorf("codeartifact empty domain_name: ConsoleURL = %q, want \"\"", got)
	}
}

func TestConsoleURL_IncompleteRow_Dbc_EmptyEngineNeverGuessesRDS(t *testing.T) {
	td := consoleURLTypeDef(t, "dbc")
	// Before the guard, an empty engine fell through the switch's default
	// case and produced an RDS-console URL for a resource that might not
	// even be an RDS cluster (docdb/neptune both prefix-match "engine").
	row := domain.Resource{ID: "acme-docdb-prod", Fields: map[string]string{"engine": ""}}
	if got := td.ConsoleURL(row, "us-east-1", consoleTestAccountID); got != "" {
		t.Errorf("dbc empty engine: ConsoleURL = %q, want \"\" (must never guess the RDS console)", got)
	}
}

// ─── elb cache-restored shape (Codex review) ────────────────────────────────

// TestConsoleURL_Elb_CacheRestoredShape_LoadBalancerArnOnlyResolves proves
// the exact row shape an on-disk cache file written before this feature
// carries: Fields["load_balancer_arn"] only, no "arn" key (that duplicate
// was deliberately deleted for a single truth source — see
// core/aws/elb.go). A resolver that regressed to reading Fields["arn"]
// would silently break ConsoleURL for every already-cached elb row.
func TestConsoleURL_Elb_CacheRestoredShape_LoadBalancerArnOnlyResolves(t *testing.T) {
	td := consoleURLTypeDef(t, "elb")
	arn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/cached-lb/9876543210"
	row := domain.Resource{
		ID:        "cached-lb",
		RawStruct: nil,
		Fields:    map[string]string{"load_balancer_arn": arn},
	}
	got := td.ConsoleURL(row, "us-east-1", consoleTestAccountID)
	want := "https://us-east-1.console.aws.amazon.com/ec2/home?region=us-east-1#LoadBalancer:loadBalancerArn=" + arn
	if got != want {
		t.Errorf("elb cache-restored shape: ConsoleURL = %q, want %q", got, want)
	}
}

// ─── Related-panel wiring: stub rows never yield a malformed URL ───────────

// TestConsoleURL_RelatedPanelStub_FieldHungryTypeYieldsNoLink is the lighter-
// weight equivalent of driving full related-panel focus state through the
// TUI (build a detail view, run related checks, focus the right column,
// press "o") to reach handleOpenConsole's consoleTargetFromRelatedRow path.
// Chose this over the full TUI drive: neither waf nor dbc registers a
// StubCreator (confirmed via core/aws/catalog_security.go /
// catalog_databases.go), so consoleTargetFromRelatedRow's fallback for a
// single-ID related row is exactly domain.Resource{ID: id, Type: targetType}
// — a bare stub with no Name and no Fields. Driving consolelink.Resolve
// directly on that exact shape exercises the identical call
// handleOpenConsole makes (ConsoleURL, then the Fields["arn"]/GoView
// fallback) without needing related-checker fakes, right-column focus
// state, or a live detail screen — the TUI scaffolding would only add
// indirection around this same call, not additional coverage.
func TestConsoleURL_RelatedPanelStub_FieldHungryTypeYieldsNoLink(t *testing.T) {
	cases := []struct {
		shortName string
		id        string
	}{
		{"waf", "a1b2c3d4-5678-90ab-cdef-111111111111"},
		{"dbc", "acme-docdb-prod"},
	}
	for _, c := range cases {
		td := consoleURLTypeDef(t, c.shortName)
		stub := domain.Resource{ID: c.id, Type: c.shortName} // no Name, no Fields — the no-StubCreator fallback shape
		got, ok := consolelink.Resolve(td, stub, "us-east-1", consoleTestAccountID)
		if ok || got != "" {
			t.Errorf("%s: Resolve() on a related-panel stub (ID-only, no Fields) = (%q, %v), want (\"\", false)", c.shortName, got, ok)
		}
	}
}
