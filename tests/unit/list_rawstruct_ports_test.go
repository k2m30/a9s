// list_rawstruct_ports_test.go — live-seam port for
// qa_list_rawstruct_test.go + qa_list_rawstruct_child_views_test.go
// (specs/022-codebase-cleanup/wave3-status.md: "RawStruct-over-Fields
// precedence + Humanize column flag pins — UNIQUE, exist nowhere else; port
// before any deletion.").
//
// Both legacy files drive views.NewResourceList(...).Update(...).View() —
// dead code in production (the controller/ViewState render path is the only
// live consumer of core/app/list_columns.go's ExtractCellValue,
// exactly as list_ports_test.go's header already documents for the
// filter/checker-carry/marker-col pins). This file re-pins every RawStruct/
// Humanize assertion the two legacy files made, driven instead through the
// live seam: Controller.ApplyResourcesLoaded -> buildListBody ->
// extractListCells -> ExtractCellValue, read back via
// Snapshot().Body.List.Rows[i].Cells.
//
// Assertion style is deliberately unchanged from the legacy files
// (substring-contains, not exact per-cell equality): the legacy tests never
// pinned column indices, only that a RawStruct/Humanize value reaches SOME
// rendered cell. Joining Cells (not the full View()) is strictly tighter
// than the original — no borders/help-text/header chrome to coincidentally
// match — while preserving every original pass/fail case.
package unit_test

import (
	"path/filepath"
	"strings"
	"testing"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// Helpers (distinctly named — openListController already exists in
// list_ports_test.go and is reused directly for top-level types).
// ---------------------------------------------------------------------------

// wave3RowCellsJoined applies resources to c under typeName and returns the
// first row's Cells joined with "|" — the live-path equivalent of the legacy
// files' `strings.Contains(view, ...)` target string, but sourced from the
// exact data RenderList consumes (ListBody.Rows[i].Cells), not a full
// re-rendered frame.
func wave3RowCellsJoined(t *testing.T, c *app.Controller, typeName string, resources []resource.Resource) string {
	t.Helper()
	c.ApplyResourcesLoaded(typeName, resources, nil, false)
	lb := c.Snapshot().Body.List
	if lb == nil || len(lb.Rows) == 0 {
		t.Fatalf("%s: no rows in list body after ApplyResourcesLoaded", typeName)
	}
	return strings.Join(lb.Rows[0].Cells, "|")
}

// wave3ChildListController builds a Controller pre-navigated to a
// ScreenChildList for a resource.GetChildType-registered shortName —
// child types (log_streams, tg_health, ecs_svc_events, ...) never appear in
// the main catalog/menu, so they cannot go through openListController's
// ActionCommand navigation. Mirrors PushChildListScreen's own doc: "used by
// NewChildResourceList to ensure topListState() is non-nil before Patch*
// calls" — the same construction the live child-list code path uses.
//
// SetViewConfig(configForType(shortName)) matters here, not just cosmetics:
// resolveListColumnsForBuild only takes the vc-sourced (Key-less, Path-based)
// column list when vc != nil. With vc == nil it falls back to the child
// type's own td.Columns, which — like several catalog ResourceTypeDefs —
// sets Key on columns the default view config deliberately leaves Key-less
// so RawStruct/Path wins. Skipping SetViewConfig here would silently swap
// every child type onto the wrong (Fields-priority) column layout and
// produce false passes/failures unrelated to the behavior under test — the
// exact class of harness bug this file's own TestWave3ListRawStruct_
// AllTypes_OverridesFields caught for "lambda"/"sg"/"ecs"/"ses" during
// development.
func wave3ChildListController(t *testing.T, shortName string) *app.Controller {
	t.Helper()
	td := resource.GetChildType(shortName)
	if td == nil {
		t.Fatalf("%s: child resource type not registered", shortName)
	}
	c := newTestController(t)
	c.SetViewConfig(configForType(shortName))
	c.RegisterFallbackTypeDef(*td)
	c.PushChildListScreen(shortName)
	return c
}

// openListControllerWithConfig mirrors openListController but injects a
// caller-supplied *config.ViewsConfig via SetViewConfig before navigating —
// SetViewConfig's own doc requires it be called "before the first
// Snapshot()", so it must run before Apply(ActionCommand), which is the
// first Apply/Snapshot call in the sequence. Every AllTypes/OverridesFields/
// standalone-type test in this file uses configForType(shortName) here
// (never nil) to exactly replicate the legacy harness's `cfg :=
// configForType(shortName)` — see wave3ChildListController's doc for why a
// nil vc silently swaps some types onto the wrong column layout.
func openListControllerWithConfig(t *testing.T, shortName string, cfg *config.ViewsConfig) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.SetViewConfig(cfg)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	return c
}

// ===========================================================================
// 1. TestListRawStruct_AllTypes — port of TestQA_ListRawStruct_AllTypes.
// Subsumes the legacy file's 7 individual per-type tests (EC2/RDS/Redis/
// DocDB/EKS/Secrets/S3), which that file's own table comment says are
// "already covered individually above, included for completeness".
// ===========================================================================

func TestListRawStruct_AllTypes(t *testing.T) {
	tests := []struct {
		shortName   string
		rawStruct   any
		expectInRow []string
	}{
		{"ec2", realisticEC2Instance(), []string{"i-0abcdef1234567890", "running", "t3.medium"}},
		{"dbi", realisticRDSInstance(), []string{"prod-db-01", "mysql", "db.r5.large"}},
		{"redis", realisticRedisReplicationGroup(), []string{"redis-prod-001", "cache.r6g.large"}},
		{"dbc", realisticDocDBCluster(), []string{"docdb-prod-cluster", "5.0.0"}},
		{"eks", realisticEKSCluster(), []string{"prod-cluster", "1.28"}},
		{"secrets", realisticSecretListEntry(), []string{"prod/database/password", "Production database password"}},
		{"s3", realisticS3Bucket(), []string{"my-production-bucket", "2025-06-15"}},
		{"lambda", realisticLambdaFunction(), []string{"my-api-handler", "python3.12"}},
		{"alarm", realisticAlarm(), []string{"HighCPUAlarm", "alarm", "CPUUtilization"}},
		{"sns", realisticSNSTopic(), []string{"arn:aws:sns:us-east-1:123456789012:my-notifications"}},
		{"elb", realisticELB(), []string{"my-app-alb", "application", "internet-faci"}},
		{"tg", realisticTargetGroup(), []string{"my-app-tg", "8080", "http", "/health"}},
		{"ecs", realisticECSClusterStruct(), []string{"prod-cluster", "active"}},
		{"ecs-svc", realisticECSService(), []string{"api-service", "active", "fargate"}},
		{"ecs-task", realisticECSTask(), []string{"running", "256", "512"}},
		{"cfn", realisticCFNStack(), []string{"my-app-stack", "create complete"}},
		{"role", realisticIAMRole(), []string{"lambda-exec-role", "/"}},
		{"logs", realisticLogGroup(), []string{"/aws/lambda/my-api-handler"}},
		{"ssm", realisticSSMParameter(), []string{"/app/config/db-host", "string"}},
		{"ddb", realisticDDBTable(), []string{"users-table"}},
		{"acm", realisticACMCertificate(), []string{"example.com", "issued", "amazon issued"}},
		{"asg", realisticASG(), []string{"my-app-asg"}},
		{"vpc", realisticVPC(), []string{"vpc-0abc1234def56789a", "10.0.0.0/16", "available"}},
		{"sg", realisticSecurityGroup(), []string{"sg-0abc1234def56789a", "web-sg", "vpc-0abc1234"}},
		{"ng", realisticNodeGroup(), []string{"prod-ng-01", "prod-cluster", "active"}},
		{"subnet", realisticSubnet(), []string{"subnet-0abc1234def56789a", "10.0.1.0/24", "us-east-1a"}},
		{"nat", realisticNATGateway(), []string{"nat-0abc1234def56789a", "available"}},
		{"igw", realisticInternetGateway(), []string{"igw-0abc1234def56789a"}},
		{"eip", realisticEIP(), []string{"eipalloc-0abc1234def56789a", "54.123.45.67"}},
		{"tgw", realisticTransitGateway(), []string{"tgw-0abc1234def56789a", "available"}},
		{"vpce", realisticVPCEndpoint(), []string{"vpce-0abc1234def56789a", "com.amazonaws.us-east-1.s3"}},
		{"eni", realisticENI(), []string{"eni-0abc1234def56789a", "in-use", "10.0.1.42"}},
		{"dbi-snap", realisticDBISnapshot(), []string{"dbi-snap-prod-20250615", "prod-db-01"}},
		{"dbc-snap", realisticDBCSnapshot(), []string{"dbc-snap-prod-20250615", "docdb-prod-cluster"}},
		{"sns-sub", realisticSNSSubscription(), []string{"email", "user@example.com"}},
		{"iam-user", realisticIAMUser(), []string{"deploy-user", "AIDAEXAMPLEUSERID"}},
		{"iam-group", realisticIAMGroup(), []string{"developers", "AGPAEXAMPLEGROUPID"}},
		{"cf", realisticCFDistribution(), []string{"E1A2B3C4D5E6F7", "d1234abcdef.cloudfront.net", "deployed"}},
		{"r53", realisticR53Zone(), []string{"/hostedzone/Z1234567890ABC", "example.com."}},
		// apigw's API ID, Protocol and Endpoint columns read mapped fields,
		// not RawStruct paths: the list merges the REST (v1) and HTTP (v2)
		// lanes, whose SDK structs share no field names, and only the mapped
		// keys answer for both. Name and Description still resolve through a
		// RawStruct path, so the case pins the rule on the cells that still
		// exercise it — the ses precedent below, not a weakened rule.
		{"apigw", realisticAPIGW(), []string{"prod-api", "Production REST API"}},
		{"ecr", realisticECR(), []string{"my-app", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app"}},
		{"efs", realisticEFS(), []string{"fs-0abc1234def56789a"}},
		{"eb-rule", realisticEBRule(), []string{"daily-backup-rule", "enabled"}},
		{"sfn", realisticSFN(), []string{"order-processing", "standard"}},
		{"pipeline", realisticPipeline(), []string{"deploy-pipeline", "v2"}},
		{"kinesis", realisticKinesis(), []string{"events-stream", "active"}},
		{"waf", realisticWAF(), []string{"prod-waf-acl", "a1b2c3d4-5678-90ab-cdef-EXAMPLE11111"}},
		{"glue", realisticGlueJob(), []string{"etl-daily-job", "4.0", "G.2X"}},
		{"eb", realisticEB(), []string{"prod-api-env", "my-web-app", "ready"}},
		// INVERTED for aws6 rows 4-6 and 10 (one humanize owner). Six cells
		// above name a value in the readable form rather than the SDK
		// constant — tg's protocol, ecs-svc's launch type, ssm's and sfn's
		// type, pipeline's type, msk's cluster type. The rule this table
		// pins is unchanged: those words are on screen only because the
		// column read the field off RawStruct, and a column that stopped
		// reading it renders nothing at all. The SDK-constant spellings are
		// not to be restored — each of those fields is now declared on its
		// type (ResourceTypeDef.HumanizeFields), and restoring them would
		// mean the column shows a constant the detail shows as words.
		//
		// ses's Identity column still takes a RawStruct path
		// (.a9s/views/ses.yaml: path: IdentityName). Its Type column no
		// longer does — it reads the mapped identity_type field, because the
		// rendered column shows words and not the SDK enum — so the example
		// moved to a cell that still exercises the rule rather than the rule
		// being weakened to accommodate the column.
		{"ses", realisticSESIdentity(), []string{"example.com"}},
		{"redshift", realisticRedshift(), []string{"analytics-cluster", "dc2.large"}},
		{"trail", realisticTrail(), []string{"org-trail", "cloudtrail-logs-bucket"}},
		{"athena", realisticAthena(), []string{"analytics-wg", "enabled"}},
		{"codeartifact", realisticCodeArtifact(), []string{"shared-libs", "my-domain"}},
		{"cb", realisticCodeBuild(), []string{"build-project", "codecommit"}},
		{"opensearch", realisticOpenSearch(), []string{"search-prod", "OpenSearch_2.11"}},
		{"kms", realisticKMS(), []string{"12345678-1234-1234-1234-123456789012", "enabled"}},
		{"msk", realisticMSK(), []string{"events-kafka", "provisioned", "active"}},
		{"backup", realisticBackup(), []string{"daily-backup-plan", "abc12345-1234-1234-1234-123456789012"}},
	}

	for _, tc := range tests {
		t.Run(tc.shortName, func(t *testing.T) {
			c := openListControllerWithConfig(t, tc.shortName, configForType(tc.shortName))
			res := resource.Resource{ID: "test-id", Name: "test-name", RawStruct: tc.rawStruct}
			joined := wave3RowCellsJoined(t, c, tc.shortName, []resource.Resource{res})
			for _, expected := range tc.expectInRow {
				if !strings.Contains(joined, expected) {
					t.Errorf("%s row cells should contain %q from RawStruct, got: %q", tc.shortName, expected, joined)
				}
			}
		})
	}
}

// ===========================================================================
// 2. TestListRawStruct_AllTypes_OverridesFields — port of
// TestQA_ListRawStruct_AllTypes_OverridesFields (RawStruct-over-Fields
// precedence, plus the documented status-column exception where Fields wins).
// ===========================================================================

func TestListRawStruct_AllTypes_OverridesFields(t *testing.T) {
	tests := []struct {
		shortName   string
		rawStruct   any
		wrongFields map[string]string
		expectInRow []string
	}{
		{
			"ec2",
			ec2types.Instance{
				InstanceId:   new("i-correct"),
				InstanceType: ec2types.InstanceTypeT3Medium,
				State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			},
			map[string]string{"instance_id": "WRONG-ID", "state": "WRONG-STATE"},
			[]string{"i-correct", "wrong- state"},
		},
		{
			"lambda",
			realisticLambdaFunction(),
			map[string]string{"function_name": "WRONG-FN", "runtime": "WRONG-RT"},
			[]string{"my-api-handler", "python3.12"},
		},
		{
			"alarm",
			realisticAlarm(),
			map[string]string{"alarm_name": "WRONG-ALARM", "state_value": "WRONG-STATE"},
			[]string{"HighCPUAlarm", "alarm"},
		},
		{
			"vpc",
			realisticVPC(),
			map[string]string{"vpc_id": "WRONG-VPC", "cidr_block": "WRONG-CIDR"},
			[]string{"vpc-0abc1234def56789a", "10.0.0.0/16"},
		},
		{
			"sg",
			realisticSecurityGroup(),
			map[string]string{"group_id": "WRONG-SG", "group_name": "WRONG-NAME"},
			[]string{"sg-0abc1234def56789a", "web-sg"},
		},
		{
			"subnet",
			realisticSubnet(),
			map[string]string{"subnet_id": "WRONG-SUB", "cidr_block": "WRONG-CIDR"},
			[]string{"subnet-0abc1234def56789a", "10.0.1.0/24"},
		},
		{
			"eip",
			realisticEIP(),
			map[string]string{"allocation_id": "WRONG-ALLOC", "public_ip": "WRONG-IP"},
			[]string{"eipalloc-0abc1234def56789a", "54.123.45.67"},
		},
		{
			"ecs",
			realisticECSClusterStruct(),
			map[string]string{"cluster_name": "WRONG-CLS", "status": "WRONG-STATUS"},
			[]string{"prod-cluster", "wrong- status"},
		},
		{
			"cfn",
			realisticCFNStack(),
			map[string]string{"stack_name": "WRONG-STACK", "stack_status": "WRONG-STATUS"},
			[]string{"my-app-stack", "create complete"},
		},
		{
			"role",
			realisticIAMRole(),
			map[string]string{"role_name": "WRONG-ROLE", "path": "WRONG-PATH"},
			[]string{"lambda-exec-role", "/"},
		},
		{
			"cf",
			realisticCFDistribution(),
			map[string]string{"id": "WRONG-ID", "domain_name": "WRONG-DN"},
			[]string{"E1A2B3C4D5E6F7", "d1234abcdef.cloudfront.net"},
		},
		{
			"r53",
			realisticR53Zone(),
			map[string]string{"id": "WRONG-ID", "name": "WRONG-NAME"},
			[]string{"/hostedzone/Z1234567890ABC", "example.com."},
		},
		{
			// api_id is deliberately absent from wrongFields: the API ID
			// column reads that field by key (the v1/v2 lanes share no SDK
			// field names), so Fields is SUPPOSED to win there. Name still
			// resolves through a RawStruct path and is the cell this case
			// pins.
			"apigw",
			realisticAPIGW(),
			map[string]string{"name": "WRONG-NAME"},
			[]string{"prod-api"},
		},
		{
			// identity_type is deliberately absent from wrongFields: the Type
			// column reads that field by key now, so Fields is SUPPOSED to
			// win there. Identity still resolves through a RawStruct path and
			// is the cell this case pins.
			"ses",
			realisticSESIdentity(),
			map[string]string{"identity_name": "WRONG-NAME"},
			[]string{"example.com"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.shortName, func(t *testing.T) {
			c := openListControllerWithConfig(t, tc.shortName, configForType(tc.shortName))
			res := resource.Resource{ID: "test-id", Name: "test-name", Fields: tc.wrongFields, RawStruct: tc.rawStruct}
			joined := wave3RowCellsJoined(t, c, tc.shortName, []resource.Resource{res})

			for _, expected := range tc.expectInRow {
				if !strings.Contains(joined, expected) {
					t.Errorf("%s row cells should contain %q from RawStruct, got: %q", tc.shortName, expected, joined)
				}
			}
			for _, wrong := range tc.wrongFields {
				if strings.Contains(joined, wrong) {
					t.Errorf("%s row cells should NOT contain %q from Fields when RawStruct is set, got: %q", tc.shortName, wrong, joined)
				}
			}
		})
	}
}

// ===========================================================================
// 3. TestListRawStruct_WithProductionViewsYAML — port of
// TestQA_ListRawStruct_WithProductionViewsYAML: validates against the real
// on-disk .a9s/views/ config (config.LoadFromDirs), not the built-in
// config.DefaultConfig() every other test in this file uses — a genuine
// drift guard between the generated YAML and the Go defaults it's generated
// from (go run ./cmd/viewsgen/).
// ===========================================================================

func TestListRawStruct_WithProductionViewsYAML(t *testing.T) {
	cfg, err := config.LoadFromDirs([]string{filepath.Join("..", "..", ".a9s", "views")})
	if err != nil {
		t.Fatalf("failed to load production views dir: %v", err)
	}
	if cfg == nil {
		t.Fatal(".a9s/views/ directory not found or returned nil config")
	}

	t.Run("EC2", func(t *testing.T) {
		inst := ec2types.Instance{
			InstanceId:       new("i-prod-config-test"),
			InstanceType:     ec2types.InstanceTypeM5Large,
			PrivateIpAddress: new("172.16.0.100"),
			State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
		}
		c := openListControllerWithConfig(t, "ec2", cfg)
		res := resource.Resource{ID: "i-prod-config-test", Name: "prod-test", Fields: map[string]string{"state": "WRONG"}, RawStruct: inst}
		joined := wave3RowCellsJoined(t, c, "ec2", []resource.Resource{res})
		if !strings.Contains(joined, "wrong") {
			t.Errorf("EC2 with production config should show 'wrong' (humanized Fields[state], wins over RawStruct for status col), got: %q", joined)
		}
		if strings.Contains(joined, "WRONG") {
			t.Error("EC2 with production config should NOT show the raw uppercase 'WRONG'")
		}
		if strings.Contains(joined, "stopped") {
			t.Error("EC2 with production config should NOT show 'stopped' from RawStruct.State.Name")
		}
	})

	// The regression row 16 fixed lived in this file's subject: the catalog
	// column was already keyed to the mapped field, but the GENERATED YAML
	// still carried a RawStruct path to the SDK enum, and the YAML wins. Only
	// a render through the on-disk views directory sees that — every other
	// test in this file builds its config from the Go defaults, which were
	// never wrong.
	t.Run("SES", func(t *testing.T) {
		ident := sesv2types.IdentityInfo{
			IdentityName:   new("acme-corp.com"),
			IdentityType:   sesv2types.IdentityTypeDomain,
			SendingEnabled: true,
		}
		c := openListControllerWithConfig(t, "ses", cfg)
		res := resource.Resource{
			ID:        "acme-corp.com",
			Name:      "acme-corp.com",
			Fields:    map[string]string{"identity_type": "domain"},
			RawStruct: ident,
		}
		joined := wave3RowCellsJoined(t, c, "ses", []resource.Resource{res})
		if !strings.Contains(joined, "domain") {
			t.Errorf("SES with production config should show the mapped word %q, got: %q", "domain", joined)
		}
		if strings.Contains(joined, "DOMAIN") {
			t.Errorf("SES with production config shows the raw SDK enum %q; the Type column reads the mapped field, and the generated YAML must not carry a RawStruct path back to the enum. got: %q", "DOMAIN", joined)
		}
	})

	t.Run("RDS", func(t *testing.T) {
		db := rdstypes.DBInstance{
			DBInstanceIdentifier: new("prod-rds-test"),
			Engine:               new("aurora-mysql"),
			EngineVersion:        new("3.04.0"),
			DBInstanceStatus:     new("available"),
			DBInstanceClass:      new("db.r6g.2xlarge"),
			MultiAZ:              new(true),
			Endpoint:             &rdstypes.Endpoint{Address: new("prod-rds-test.cluster-xyz.us-west-2.rds.amazonaws.com")},
		}
		c := openListControllerWithConfig(t, "dbi", cfg)
		res := resource.Resource{ID: "prod-rds-test", Name: "prod-rds-test", Fields: map[string]string{"endpoint": "WRONG-EP"}, RawStruct: db}
		joined := wave3RowCellsJoined(t, c, "dbi", []resource.Resource{res})
		if !strings.Contains(joined, "prod-rds-test.cluster-xyz") {
			t.Errorf("RDS with production config should show endpoint prefix, got: %q", joined)
		}
		if strings.Contains(joined, "WRONG-EP") {
			t.Error("RDS with production config should NOT show WRONG-EP from Fields")
		}
	})

	t.Run("Redis", func(t *testing.T) {
		rg := elasticachetypes.ReplicationGroup{
			ReplicationGroupId:    new("prod-redis-test"),
			Status:                new("available"),
			CacheNodeType:         new("cache.m7g.large"),
			MemberClusters:        []string{"prod-redis-test-001", "prod-redis-test-002"},
			ConfigurationEndpoint: &elasticachetypes.Endpoint{Address: new("prod-redis-test.clustercfg.usw2.cache.amazonaws.com")},
		}
		c := openListControllerWithConfig(t, "redis", cfg)
		res := resource.Resource{ID: "prod-redis-test", Name: "prod-redis-test", Fields: map[string]string{"endpoint": "WRONG-EP"}, RawStruct: rg}
		joined := wave3RowCellsJoined(t, c, "redis", []resource.Resource{res})
		if !strings.Contains(joined, "prod-redis-test.clustercfg") {
			t.Errorf("Redis with production config should show endpoint prefix, got: %q", joined)
		}
		if strings.Contains(joined, "WRONG-EP") {
			t.Error("Redis with production config should NOT show WRONG-EP from Fields")
		}
	})

	t.Run("DocDB", func(t *testing.T) {
		cluster := docdbtypes.DBCluster{
			DBClusterIdentifier: new("prod-docdb-test"),
			EngineVersion:       new("5.0.0"),
			Status:              new("available"),
			Endpoint:            new("prod-docdb-test.cluster-abc.us-west-2.docdb.amazonaws.com"),
		}
		c := openListControllerWithConfig(t, "dbc", cfg)
		res := resource.Resource{ID: "prod-docdb-test", Name: "prod-docdb-test", Fields: map[string]string{"endpoint": "WRONG-EP"}, RawStruct: cluster}
		joined := wave3RowCellsJoined(t, c, "dbc", []resource.Resource{res})
		if !strings.Contains(joined, "prod-docdb-test.cluster-abc") {
			t.Errorf("DocDB with production config should show endpoint prefix, got: %q", joined)
		}
	})

	t.Run("EKS", func(t *testing.T) {
		cluster := &ekstypes.Cluster{
			Name:            new("prod-eks-test"),
			Version:         new("1.30"),
			Status:          ekstypes.ClusterStatusActive,
			Endpoint:        new("https://prod-eks-test.gr7.us-west-2.eks.amazonaws.com"),
			PlatformVersion: new("eks.9"),
		}
		c := openListControllerWithConfig(t, "eks", cfg)
		res := resource.Resource{ID: "prod-eks-test", Name: "prod-eks-test", Fields: map[string]string{"endpoint": "WRONG-EP"}, RawStruct: cluster}
		joined := wave3RowCellsJoined(t, c, "eks", []resource.Resource{res})
		if !strings.Contains(joined, "prod-eks-test.gr7") {
			t.Errorf("EKS with production config should show endpoint prefix, got: %q", joined)
		}
	})

	t.Run("Secrets", func(t *testing.T) {
		secret := smtypes.SecretListEntry{Name: new("prod/test/secret"), Description: new("Production test secret")}
		c := openListControllerWithConfig(t, "secrets", cfg)
		res := resource.Resource{ID: "prod/test/secret", Name: "prod/test/secret", Fields: map[string]string{"description": "WRONG-DESC"}, RawStruct: secret}
		joined := wave3RowCellsJoined(t, c, "secrets", []resource.Resource{res})
		if !strings.Contains(joined, "Production test secret") {
			t.Errorf("Secrets with production config should show description from RawStruct, got: %q", joined)
		}
		if strings.Contains(joined, "WRONG-DESC") {
			t.Error("Secrets with production config should NOT show WRONG-DESC from Fields")
		}
	})

	t.Run("S3", func(t *testing.T) {
		bucket := s3types.Bucket{Name: new("prod-config-bucket"), CreationDate: new(testTime)}
		c := openListControllerWithConfig(t, "s3", cfg)
		res := resource.Resource{ID: "prod-config-bucket", Name: "prod-config-bucket", Fields: map[string]string{"creation_date": "WRONG-DATE"}, RawStruct: bucket}
		joined := wave3RowCellsJoined(t, c, "s3", []resource.Resource{res})
		if !strings.Contains(joined, "2025-06-15") {
			t.Errorf("S3 with production config should show creation date from RawStruct, got: %q", joined)
		}
		if strings.Contains(joined, "WRONG-DATE") {
			t.Error("S3 with production config should NOT show WRONG-DATE from Fields")
		}
	})
}

// ===========================================================================
// 4. TestListRawStruct_FieldsFallbackWhenNoRawStruct — port of
// TestQA_ListRawStruct_FieldsFallbackWhenNoRawStruct.
// ===========================================================================

func TestListRawStruct_FieldsFallbackWhenNoRawStruct(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))
	res := resource.Resource{
		ID:   "i-fallback",
		Name: "fallback-instance",
		Fields: map[string]string{
			"instance_id": "i-fallback",
			"state":       "terminated",
			"type":        "t2.micro",
			"private_ip":  "10.0.0.1",
		},
	}
	joined := wave3RowCellsJoined(t, c, "ec2", []resource.Resource{res})
	if joined == "" || strings.Trim(joined, "|") == "" {
		t.Error("list body cells should not be empty when Fields are provided without RawStruct")
	}
}

// ===========================================================================
// 4a. TestListRawStruct_HumanizeColumn_FieldsFallbackWhenNoRawStruct —
// a humanized, Path-only (Key-less) column must still route through
// domain.HumanizeStatusPhrase when RawStruct is nil and the raw AWS enum is
// only reachable via the title-match Fields fallback (a cache-warm row:
// RawStruct stripped, value materialized into Fields). "transfer"'s Endpoint/
// Identity Provider columns (core/config/defaults_networking.go) are real,
// registered Key-less/Path-based columns their type declares as humanized —
// the title-match loop (list_columns.go's ExtractCellValue) looks them up by
// their OWN title-derived key ("endpoint", "identity_provider"), which is
// what a Fields-only (RawStruct-stripped) row must carry for that fallback to
// find them at all.
//
// INVERTED for aws6 row 10: the Domain column used to be the negative case,
// pinning that a column WITHOUT the flag stays raw. transfer's domain was one
// of the fields row 10 declares, so it reads as words now and cannot play
// that part. The negative case it carried has moved to the Server ID cell,
// which no declaration names and which must stay verbatim because it is an
// identifier. The "EFS" assertion is not to be restored — restoring it would
// mean the column shows a constant the detail shows as words.
// ===========================================================================

func TestListRawStruct_HumanizeColumn_FieldsFallbackWhenNoRawStruct(t *testing.T) {
	c := openListControllerWithConfig(t, "transfer", configForType("transfer"))
	res := resource.Resource{
		ID:   "s-0abc1234def56789a",
		Name: "s-0abc1234def56789a",
		Fields: map[string]string{
			"status":            "ONLINE",
			"domain":            "EFS",
			"endpoint":          "VPC_ENDPOINT",
			"identity_provider": "SERVICE_MANAGED",
		},
	}
	joined := wave3RowCellsJoined(t, c, "transfer", []resource.Resource{res})

	if !strings.Contains(joined, "vpc endpoint") {
		t.Errorf(`transfer row cells (RawStruct nil, Fields-only) should humanize the Endpoint column (Humanize:true, Path:"EndpointType") to "vpc endpoint", got: %q — a warm-cache row must render identically to a live one`, joined)
	}
	if strings.Contains(joined, "VPC_ENDPOINT") {
		t.Errorf(`transfer row cells should NOT show the raw enum "VPC_ENDPOINT" when RawStruct is nil, got: %q`, joined)
	}
	if !strings.Contains(joined, "service managed") {
		t.Errorf(`transfer row cells (RawStruct nil, Fields-only) should humanize the Identity Provider column (Humanize:true, Path:"IdentityProviderType") to "service managed", got: %q`, joined)
	}
	if strings.Contains(joined, "SERVICE_MANAGED") {
		t.Errorf(`transfer row cells should NOT show the raw enum "SERVICE_MANAGED" when RawStruct is nil, got: %q`, joined)
	}

	if !strings.Contains(joined, "efs") {
		t.Errorf(`transfer row cells should humanize the Domain column to "efs", got: %q`, joined)
	}
	// The negative case: humanizing is per declared field, not per column. An
	// identifier the type declares nothing about reaches the cell verbatim.
	if !strings.Contains(joined, "s-0abc1234def56789a") {
		t.Errorf(`transfer row cells should show the server id verbatim — no declaration names it, got: %q`, joined)
	}
}

// ===========================================================================
// 5. Standalone top-level types NOT covered by the AllTypes table: SQS
// (string RawStruct), EBS volume/snapshot, AMI, CloudTrail events.
// ===========================================================================

func TestListRawStruct_SQS_StringRawStruct(t *testing.T) {
	c := openListControllerWithConfig(t, "sqs", configForType("sqs"))
	res := resource.Resource{
		ID:   "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		Name: "my-queue",
		Fields: map[string]string{
			"queue_name":         "my-queue",
			"approx_messages":    "42",
			"approx_not_visible": "3",
			"delay_seconds":      "0",
			"queue_url":          "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		},
		RawStruct: "map[ApproximateNumberOfMessages:42 ApproximateNumberOfMessagesNotVisible:3]",
	}
	joined := wave3RowCellsJoined(t, c, "sqs", []resource.Resource{res})
	if !strings.Contains(joined, "my-queue") {
		t.Errorf("SQS row cells should contain queue name from Fields, got: %q", joined)
	}
	if !strings.Contains(joined, "42") {
		t.Errorf("SQS row cells should contain message count from Fields, got: %q", joined)
	}
}

func TestListRawStruct_EBSVolume(t *testing.T) {
	c := openListControllerWithConfig(t, "ebs", configForType("ebs"))
	res := resource.Resource{
		ID:   "vol-111aabbcc",
		Name: "prod-data-vol",
		Fields: map[string]string{
			"volume_id": "vol-111aabbcc", "name": "prod-data-vol", "state": "in-use",
			"size": "100", "type": "gp3", "iops": "3000", "encrypted": "true",
			"attached_to": "i-0abc123456def789", "az": "us-east-1a", "created": "2025-03-10 14:00",
		},
		RawStruct: realisticVolume(),
	}
	joined := wave3RowCellsJoined(t, c, "ebs", []resource.Resource{res})
	if !strings.Contains(joined, "vol-111aabbcc") {
		t.Errorf("EBS Volume row cells should contain volume ID from RawStruct, got: %q", joined)
	}
	if !strings.Contains(joined, "in-use") {
		t.Errorf("EBS Volume row cells should contain 'in-use' state from RawStruct, got: %q", joined)
	}
	if !strings.Contains(joined, "us-east-1a") {
		t.Errorf("EBS Volume row cells should contain AZ from RawStruct, got: %q", joined)
	}
}

func TestListRawStruct_EBSSnapshot(t *testing.T) {
	c := openListControllerWithConfig(t, "ebs-snap", configForType("ebs-snap"))
	res := resource.Resource{
		ID:   "snap-0aabb11cc",
		Name: "prod-snap-daily",
		Fields: map[string]string{
			"snapshot_id": "snap-0aabb11cc", "name": "prod-snap-daily", "state": "completed",
			"volume_id": "vol-111aabbcc", "size": "100", "encrypted": "true",
			"description": "Daily backup snapshot", "started": "2025-02-20 09:15", "progress": "100%",
		},
		RawStruct: realisticSnapshot(),
	}
	joined := wave3RowCellsJoined(t, c, "ebs-snap", []resource.Resource{res})
	if !strings.Contains(joined, "snap-0aabb11cc") {
		t.Errorf("EBS Snapshot row cells should contain snapshot ID from RawStruct, got: %q", joined)
	}
	if !strings.Contains(joined, "completed") {
		t.Errorf("EBS Snapshot row cells should contain 'completed' state from RawStruct, got: %q", joined)
	}
	if !strings.Contains(joined, "vol-111aabbcc") {
		t.Errorf("EBS Snapshot row cells should contain volume ID from RawStruct, got: %q", joined)
	}
}

func TestListRawStruct_AMI(t *testing.T) {
	c := openListControllerWithConfig(t, "ami", configForType("ami"))
	res := resource.Resource{
		ID:   "ami-0abc111222333444a",
		Name: "my-web-server-ami",
		Fields: map[string]string{
			"image_id": "ami-0abc111222333444a", "name": "my-web-server-ami", "state": "available",
			"architecture": "x86_64", "platform": "Linux/UNIX", "root_device_type": "ebs",
			"creation_date": "2025-01-15T10:30:00.000Z", "public": "false",
		},
		RawStruct: realisticImage(),
	}
	joined := wave3RowCellsJoined(t, c, "ami", []resource.Resource{res})
	if !strings.Contains(joined, "my-web-server-ami") {
		t.Errorf("AMI row cells should contain AMI name from RawStruct, got: %q", joined)
	}
	if !strings.Contains(joined, "ami-0abc111222333444a") {
		t.Errorf("AMI row cells should contain image ID from RawStruct, got: %q", joined)
	}
	if !strings.Contains(joined, "available") {
		t.Errorf("AMI row cells should contain 'available' state from RawStruct, got: %q", joined)
	}
}

func TestListRawStruct_CloudTrailEvent(t *testing.T) {
	c := openListControllerWithConfig(t, "ct-events", configForType("ct-events"))
	res := resource.Resource{
		ID:   "evt-0001-abcd-1234-5678-abcdef012345",
		Name: "RunInstances",
		Fields: map[string]string{
			"event_name": "RunInstances", "time": "2025-03-15 12:00:00", "event_time": "2025-03-15 12:00:00",
			"user": "admin", "source": "ec2.amazonaws.com", "resource_type": "AWS::EC2::Instance",
			"resource_name": "i-0abc123456def789", "read_only": "false",
			"_ct.verb": "W", "_ct.actor": "admin", "_ct.origin": "?", "_ct.target": "i-0abc123456def789", "_ct.outcome": "OK",
		},
		RawStruct: realisticCloudTrailEvent(),
	}
	joined := wave3RowCellsJoined(t, c, "ct-events", []resource.Resource{res})
	if !strings.Contains(joined, "RunInstances") {
		t.Errorf("CloudTrail Event row cells should contain event name from RawStruct (EVENT column), got: %q", joined)
	}
	if !strings.Contains(joined, "admin") {
		t.Errorf("CloudTrail Event row cells should contain actor 'admin' from _ct.actor field, got: %q", joined)
	}
}

// ===========================================================================
// 6. TestListRawStruct_ChildViews — table-driven port of the 22 child
// list-view tests in qa_list_rawstruct_child_views_test.go. None of these
// shortNames appear in the AllTypes table above — every one is unique.
// ===========================================================================

func TestListRawStruct_ChildViews(t *testing.T) {
	ts := testTime

	tests := []struct {
		name        string
		shortName   string
		res         resource.Resource
		expectInRow []string
		notInRow    []string
	}{
		{
			"LogStreams", "log_streams",
			resource.Resource{
				ID: "2024/03/22/[$LATEST]abcdef1234567890", Name: "2024/03/22/[$LATEST]abcdef1234567890",
				Fields: map[string]string{
					"stream_name": "2024/03/22/[$LATEST]abcdef1234567890",
					"last_event":  "2024-03-23 00:00",
					"first_event": "2024-03-22 00:00",
				},
				RawStruct: cwlogstypes.LogStream{LogStreamName: new("2024/03/22/[$LATEST]abcdef1234567890")},
			},
			[]string{"abcdef1234567890"}, nil,
		},
		{
			"LogEvents", "log_events",
			resource.Resource{
				ID: "evt-1711065600000-0", Name: "ERROR Failed to connect to database",
				Fields: map[string]string{
					"timestamp":      "2024-03-22 00:00",
					"message":        "ERROR Failed to connect to database",
					"ingestion_time": "2024-03-22 00:00",
				},
				RawStruct: cwlogstypes.OutputLogEvent{Message: new("ERROR Failed to connect to database")},
			},
			[]string{"Failed to connect"}, nil,
		},
		{
			"TargetHealth", "tg_health",
			resource.Resource{
				ID: "i-0abc1234def56789a", Name: "i-0abc1234def56789a",
				Fields: map[string]string{
					"target_id": "i-0abc1234def56789a", "port": "8080", "az": "us-east-1a",
					"health": "unhealthy", "reason": "Target.FailedHealthChecks",
					"description": "Health checks failed with 503",
				},
				RawStruct: elbtypes.TargetHealthDescription{
					Target:       &elbtypes.TargetDescription{Id: new("i-0abc1234def56789a")},
					TargetHealth: &elbtypes.TargetHealth{State: elbtypes.TargetHealthStateEnumUnhealthy},
				},
			},
			[]string{"i-0abc1234def56789a"}, nil,
		},
		{
			"LambdaInvocations", "lambda_invocations",
			resource.Resource{
				ID: "12345678-1234-1234-1234-123456789012", Name: "12345678-1234-1234-1234-123456789012",
				Fields: map[string]string{
					"request_id": "12345678-1234-1234-1234-123456789012", "timestamp": "2024-03-22 00:00",
					"status": "OK", "duration_ms": "2103 ms", "memory_used": "128/256 MB", "cold_start": "no",
				},
				RawStruct: cwlogstypes.FilteredLogEvent{EventId: new("evt-001")},
			},
			[]string{"12345678"}, nil,
		},
		{
			"LambdaInvocationLogs", "lambda_invocation_logs",
			resource.Resource{
				ID: "log-002", Name: "INFO Processing request for user abc-123",
				Fields: map[string]string{
					"timestamp": "2024-03-22 00:00", "message": "INFO Processing request for user abc-123",
				},
				RawStruct: cwlogstypes.FilteredLogEvent{Message: new("INFO Processing request for user abc-123")},
			},
			[]string{"Processing request"}, nil,
		},
		{
			"EcsSvcEvents", "ecs_svc_events",
			resource.Resource{
				ID: "evt-list-001", Name: "(service web-service) has reached a steady state.",
				Fields: map[string]string{
					"timestamp": "2024-03-22 10:00", "message": "(service web-service) has reached a steady state.",
				},
				RawStruct: ecstypes.ServiceEvent{Message: new("(service web-service) has reached a steady state.")},
			},
			[]string{"steady state"}, nil,
		},
		{
			"EcsSvcTasks", "ecs_tasks",
			resource.Resource{
				ID: "abc123def456", Name: "abc123def456",
				Fields: map[string]string{
					"task_id_short": "abc123def456", "status": "RUNNING", "health": "HEALTHY",
					"task_def_short": "web-app:5", "started_at": "2024-03-22 10:00", "stopped_reason": "",
				},
				RawStruct: ecstypes.Task{TaskArn: new("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456"), LastStatus: new("RUNNING")},
			},
			[]string{"abc123def456"}, nil,
		},
		{
			"EcsSvcLogs", "ecs_svc_logs",
			resource.Resource{
				ID: "evt-svc-log-list", Name: "INFO Starting application server on port 8080",
				Fields: map[string]string{
					"timestamp": "2024-03-21 16:00", "stream_short": "web/abc123de",
					"message": "INFO Starting application server on port 8080",
				},
				RawStruct: cwlogstypes.FilteredLogEvent{Message: new("INFO Starting application server on port 8080")},
			},
			[]string{"Starting application"}, nil,
		},
		{
			"CfnEvents", "cfn_events",
			resource.Resource{
				ID: "evt-list-cfn-001", Name: "2024-03-22 10:00",
				Fields: map[string]string{
					"timestamp": "2024-03-22 10:00", "logical_resource_id": "MyBucket",
					"resource_type": "AWS::S3::Bucket", "resource_status": "CREATE_COMPLETE",
					"resource_status_reason": "Resource creation complete",
				},
				RawStruct: cfntypes.StackEvent{LogicalResourceId: new("MyBucket"), ResourceStatus: cfntypes.ResourceStatusCreateComplete},
			},
			[]string{"MyBucket"}, nil,
		},
		{
			"CfnResources", "cfn_resources",
			resource.Resource{
				ID: "MyBucket", Name: "MyBucket",
				Fields: map[string]string{
					"logical_resource_id": "MyBucket", "physical_resource_id": "my-stack-mybucket-abc123",
					"resource_type": "AWS::S3::Bucket", "resource_status": "CREATE_COMPLETE",
					"drift_status": "IN_SYNC", "last_updated": "2024-03-22 10:00",
				},
				RawStruct: cfntypes.StackResourceSummary{LogicalResourceId: new("MyBucket"), ResourceStatus: cfntypes.ResourceStatusCreateComplete},
			},
			[]string{"MyBucket"}, nil,
		},
		{
			"AsgActivities", "asg_activities",
			resource.Resource{
				ID: "act-list-001", Name: "2024-03-22 10:00",
				Fields: map[string]string{
					"start_time": "2024-03-22 10:00", "status_code": "Successful",
					"description": "Launching a new EC2 instance: i-0abc1234",
					"cause":       "At 2024-03-22T10:00:00Z an instance was started",
				},
				RawStruct: asgtypes.Activity{ActivityId: new("act-list-001"), StatusCode: asgtypes.ScalingActivityStatusCodeSuccessful},
			},
			// The "Status"-titled column routes through domain.HumanizeStatusPhrase
			// (title-based Status cascade): "Successful" -> "successful".
			[]string{"successful"}, nil,
		},
		{
			"AlarmHistory", "alarm_history",
			resource.Resource{
				ID: "2024-03-22 10:00", Name: "2024-03-22 10:00",
				Fields: map[string]string{
					"timestamp": "2024-03-22 10:00", "history_item_type": "StateUpdate",
					"history_summary": "Alarm updated from OK to ALARM",
				},
				RawStruct: cwtypes.AlarmHistoryItem{AlarmName: new("HighCPUAlarm"), HistoryItemType: cwtypes.HistoryItemTypeStateUpdate},
			},
			[]string{"StateUpdate"}, nil,
		},
		{
			"ELBListeners", "elb_listeners",
			resource.Resource{
				ID: "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456", Name: "443",
				Fields: map[string]string{
					"port": "443", "protocol": "HTTPS", "default_action_type": "forward",
					"default_action_target": "api-prod-tg", "ssl_policy": "ELBSecurityPolicy-TLS13-1-2-2021-06",
					"certificate_short": "abc-def-123",
				},
				RawStruct: elbtypes.Listener{
					Port: new(int32(443)), Protocol: elbtypes.ProtocolEnumHttps,
					DefaultActions: []elbtypes.Action{{Type: elbtypes.ActionTypeEnumForward}},
				},
			},
			[]string{"443", "HTTPS", "forward"}, nil,
		},
		{
			"CBBuilds", "cb_builds",
			resource.Resource{
				ID: "my-project:build-id-001", Name: "#142",
				Fields: map[string]string{
					"build_number": "142", "build_status": "SUCCEEDED", "start_time": "2024-06-15 10:00",
					"duration": "4m 12s", "source_version_short": "abc123de", "initiator": "codepipeline/my-pipeline",
				},
				RawStruct: cbtypes.Build{BuildNumber: new(int64(142)), BuildStatus: cbtypes.StatusTypeSucceeded},
			},
			// Status column humanizes the all-caps AWS enum "SUCCEEDED" -> "succeeded".
			[]string{"succeeded", "142"}, nil,
		},
		{
			"CBBuildLogs", "cb_build_logs",
			resource.Resource{
				ID: "evt-1718445600000-0", Name: "[Container] Running command echo hello",
				Fields: map[string]string{
					"timestamp": "2024-06-15 10:00", "message": "[Container] Running command echo hello",
					"ingestion_time": "2024-06-15 10:00", "event_id": "evt-1718445600000-0",
				},
				RawStruct: cwlogstypes.OutputLogEvent{Message: new("[Container] Running command echo hello")},
			},
			[]string{"Running command"}, nil,
		},
		{
			"ECRImages", "ecr_images",
			resource.Resource{
				ID: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", Name: "latest, v1.0.0",
				Fields: map[string]string{
					"image_tags": "latest, v1.0.0", "digest_short": "abcdef123456", "pushed_at": "2024-06-15 10:00",
					"image_size": "50.0 MB", "scan_status": "COMPLETE", "finding_counts": "3H 5M",
				},
				RawStruct: ecrtypes.ImageDetail{
					ImageDigest: new("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
					ImageTags:   []string{"latest", "v1.0.0"},
				},
			},
			[]string{"latest", "abcdef123456"}, nil,
		},
		{
			"PipelineStages", "pipeline_stages",
			resource.Resource{
				ID: "Source/GitHub", Name: "GitHub",
				Fields: map[string]string{
					"stage_name": "Source", "stage_status": "Succeeded", "action_name": "GitHub",
					"action_status": "Succeeded", "last_change_time": "2024-06-15 10:00",
					"external_url": "https://github.com/org/repo/commit/abc123",
				},
				RawStruct: awsclient.PipelineStageRow{StageName: "Source", StageStatus: "Succeeded", ActionName: "GitHub", ActionStatus: "Succeeded"},
			},
			[]string{"Source", "GitHub", "Succeeded"}, nil,
		},
		{
			"RolePolicies", "role_policies",
			resource.Resource{
				ID: "arn:aws:iam::aws:policy/ReadOnlyAccess", Name: "ReadOnlyAccess",
				Fields: map[string]string{
					"policy_name": "ReadOnlyAccess", "policy_arn": "arn:aws:iam::aws:policy/ReadOnlyAccess", "policy_type": "Managed",
				},
				RawStruct: awsclient.RolePolicyRow{PolicyName: "ReadOnlyAccess", PolicyArn: "arn:aws:iam::aws:policy/ReadOnlyAccess", PolicyType: "Managed"},
			},
			[]string{"ReadOnlyAccess", "Managed"}, nil,
		},
		{
			"ELBListenerRules", "elb_listener_rules",
			resource.Resource{
				ID: "arn:rule/1", Name: "100",
				Fields: map[string]string{
					"priority": "100", "conditions_summary": "path: /api/*", "action_type": "forward", "action_target": "api-tg",
				},
				RawStruct: elbtypes.Rule{RuleArn: new("arn:rule/1"), Priority: new("100")},
			},
			[]string{"100", "forward"}, nil,
		},
		{
			"DbiEvents", "dbi_events",
			resource.Resource{
				ID: "2024-06-15 10:00/my-db-instance", Name: "2024-06-15 10:00",
				Fields: map[string]string{
					"timestamp": "2024-06-15 10:00", "event_categories": "maintenance",
					"message": "Applying offline patches to DB instance", "source_identifier": "my-db-instance",
					"source_type": "db-instance", "source_arn": "arn:aws:rds:us-east-1:123456789012:db:my-db-instance",
				},
				RawStruct: rdstypes.Event{
					EventCategories: []string{"maintenance"}, Message: new("Applying offline patches to DB instance"),
					SourceIdentifier: new("my-db-instance"), SourceType: rdstypes.SourceTypeDbInstance,
				},
			},
			[]string{"maintenance", "Applying offline patches"}, nil,
		},
		{
			"EbRuleTargets", "eb_rule_targets",
			resource.Resource{
				ID: "lambda-target-1", Name: "lambda-target-1",
				Fields: map[string]string{
					"target_id": "lambda-target-1", "target_arn": "arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-daily",
					"resource_type_name": "Lambda: data-pipeline-daily", "input_summary": "—",
				},
				RawStruct: ebtypes.Target{
					Id:  new("lambda-target-1"),
					Arn: new("arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-daily"),
				},
			},
			[]string{"lambda-target-1", "Lambda"}, nil,
		},
		{
			"GlueRuns", "glue_runs",
			resource.Resource{
				ID: "jr_abc12345-6789-0abc-def0-123456789012", Name: "2024-08-10 14:30",
				Fields: map[string]string{
					"run_id_short": "jr_abc12", "job_run_state": "SUCCEEDED", "started_on": "2024-08-10 14:30",
					"execution_time_human": "47m 23s", "error_message": "", "dpu_hours": "12.5",
					"run_id": "jr_abc12345-6789-0abc-def0-123456789012", "job_name": "etl-daily-load",
				},
				RawStruct: gluetypes.JobRun{
					Id: new("jr_abc12345-6789-0abc-def0-123456789012"), JobName: new("etl-daily-load"),
					JobRunState: gluetypes.JobRunStateSucceeded, StartedOn: &ts,
				},
			},
			// glue_runs's config-driven State column is {Title:"State",
			// Path:"JobRunState"} — Key-less, Path-based — so it routes RawStruct
			// through domain.HumanizeStatusPhrase; the raw AWS enum must not reach
			// the row.
			[]string{"jr_abc12", "succeeded"}, []string{"SUCCEEDED"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := wave3ChildListController(t, tc.shortName)
			joined := wave3RowCellsJoined(t, c, tc.shortName, []resource.Resource{tc.res})
			for _, expected := range tc.expectInRow {
				if !strings.Contains(joined, expected) {
					t.Errorf("%s row cells should contain %q, got: %q", tc.shortName, expected, joined)
				}
			}
			for _, forbidden := range tc.notInRow {
				if strings.Contains(joined, forbidden) {
					t.Errorf("%s row cells should NOT contain raw enum %q, got: %q", tc.shortName, forbidden, joined)
				}
			}
		})
	}
}

// ===========================================================================
// 2. TestListRawStruct_S3ObjectSort_UsesNumericByteOrder — port of
// qa_s3_test.go's TestQA_S3_B10_3_ObjectList_SortBySize_UsesNumericByteOrder.
// s3_objects's default view config sets {Key:"size", SortKey:"size_raw"} on
// the Size column (core/config/defaults_databases.go), so the sort reads the
// byte count the fetcher stored — not the display string ("1 KB" vs "900 B"),
// which would sort lexicographically wrong ('1' < '9'). The sort_path this
// test was written against read the AWS struct instead, which the warm-cache
// frame does not have, so the same list came back in a different order until
// the fetch landed; the rows below now carry the stored byte count the way the
// fetcher writes it. Do not restore a RawStruct-only version of this case.
// Neither core/app/list_test.go's TestListSort_* (Name-only) nor this file's
// AllTypes/OverridesFields cases (cell VALUE, not sort ORDER) cover a
// sort-key-driven numeric sort — this was otherwise unpinned at the
// controller level.
// ===========================================================================

func TestListRawStruct_S3ObjectSort_UsesNumericByteOrder(t *testing.T) {
	objects := []resource.Resource{
		{
			ID: "medium.bin", Name: "medium.bin",
			Fields:    map[string]string{"key": "medium.bin", "size_raw": "1024", "size": "1 KB", "last_modified": "2025-01-02"},
			RawStruct: s3types.Object{Key: wave3StrPtr("medium.bin"), Size: wave3Int64Ptr(1024)},
		},
		{
			ID: "small.bin", Name: "small.bin",
			Fields:    map[string]string{"key": "small.bin", "size_raw": "900", "size": "900 B", "last_modified": "2025-01-01"},
			RawStruct: s3types.Object{Key: wave3StrPtr("small.bin"), Size: wave3Int64Ptr(900)},
		},
		{
			ID: "large.bin", Name: "large.bin",
			Fields:    map[string]string{"key": "large.bin", "size_raw": "2048", "size": "2 KB", "last_modified": "2025-01-03"},
			RawStruct: s3types.Object{Key: wave3StrPtr("large.bin"), Size: wave3Int64Ptr(2048)},
		},
	}

	c := wave3ChildListController(t, "s3_objects")
	c.ApplyResourcesLoaded("s3_objects", objects, nil, false)

	c.Apply(app.Action{Kind: app.ActionSort, Arg: "size"})
	lb := *c.Snapshot().Body.List
	if len(lb.Rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(lb.Rows))
	}
	gotAsc := []string{lb.Rows[0].ResourceID, lb.Rows[1].ResourceID, lb.Rows[2].ResourceID}
	wantAsc := []string{"small.bin", "medium.bin", "large.bin"}
	for i := range wantAsc {
		if gotAsc[i] != wantAsc[i] {
			t.Fatalf("size ascending sort must use numeric byte order (RawStruct.Size), got order %v, want %v", gotAsc, wantAsc)
		}
	}

	// Toggle to descending.
	c.Apply(app.Action{Kind: app.ActionSort, Arg: "size"})
	lb = *c.Snapshot().Body.List
	gotDesc := []string{lb.Rows[0].ResourceID, lb.Rows[1].ResourceID, lb.Rows[2].ResourceID}
	wantDesc := []string{"large.bin", "medium.bin", "small.bin"}
	for i := range wantDesc {
		if gotDesc[i] != wantDesc[i] {
			t.Fatalf("size descending sort must use numeric byte order (RawStruct.Size), got order %v, want %v", gotDesc, wantDesc)
		}
	}
}

func wave3StrPtr(s string) *string { return &s }
func wave3Int64Ptr(n int64) *int64 { return &n }
