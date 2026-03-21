package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/fieldpath"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/styles"
	"github.com/k2m30/a9s/internal/tui/views"
)

// TestDetailPaths_AllConfiguredFieldsRendered verifies that EVERY detail path
// from views.yaml appears in the rendered detail view for each resource type.
// Uses realistic SDK struct fixtures for all resource types.
// This catches: wrong field names in views.yaml, nil fields being skipped,
// and wrong ViewDef being selected.
func TestDetailPaths_AllConfiguredFieldsRendered(t *testing.T) {
	styles.Reinit() // ensure styles are initialized

	cfg, err := config.LoadFrom([]string{"../../.a9s/views.yaml"})
	if err != nil {
		t.Fatalf("failed to load views.yaml: %v", err)
	}

	k := keys.Default()

	// Map resource type to a fixture resource with RawStruct populated.
	// Each entry uses the realistic SDK struct builder from the _test package.
	allFixtures := map[string]resource.Resource{
		"ec2":          buildResource("i-0abcdef1234567890", "web-server-prod", realisticEC2Instance()),
		"dbi":          buildResource("prod-db-01", "prod-db-01", realisticRDSInstance()),
		"redis":        buildResource("redis-prod-001", "redis-prod-001", realisticRedisCacheCluster()),
		"dbc":          buildResource("docdb-prod-cluster", "docdb-prod-cluster", realisticDocDBCluster()),
		"eks":          buildResource("prod-cluster", "prod-cluster", realisticEKSCluster()),
		"secrets":      buildResource("prod/database/password", "prod/database/password", realisticSecretListEntry()),
		"s3":           buildResource("my-production-bucket", "my-production-bucket", realisticS3Bucket()),
		"s3_objects":   buildResource("data/report-2025.csv", "data/report-2025.csv", realisticS3ObjectFile()),
		"lambda":       buildResource("my-api-handler", "my-api-handler", realisticLambdaFunction()),
		"alarm":        buildResource("HighCPUAlarm", "HighCPUAlarm", realisticAlarm()),
		"sns":          buildResource("my-notifications", "my-notifications", realisticSNSTopic()),
		"elb":          buildResource("my-app-alb", "my-app-alb", realisticELB()),
		"tg":           buildResource("my-app-tg", "my-app-tg", realisticTargetGroup()),
		"ecs":          buildResource("prod-cluster", "prod-cluster", realisticECSClusterStruct()),
		"ecs-svc":      buildResource("api-service", "api-service", realisticECSService()),
		"ecs-task":     buildResource("abc123def456", "abc123def456", realisticECSTask()),
		"cfn":          buildResource("my-app-stack", "my-app-stack", realisticCFNStack()),
		"role":         buildResource("lambda-exec-role", "lambda-exec-role", realisticIAMRole()),
		"logs":         buildResource("/aws/lambda/my-api-handler", "/aws/lambda/my-api-handler", realisticLogGroup()),
		"ssm":          buildResource("/app/config/db-host", "/app/config/db-host", realisticSSMParameter()),
		"ddb":          buildResource("users-table", "users-table", realisticDDBTable()),
		"acm":          buildResource("example.com", "example.com", realisticACMCertificate()),
		"asg":          buildResource("my-app-asg", "my-app-asg", realisticASG()),
		"vpc":          buildResource("vpc-0abc1234def56789a", "prod-vpc", realisticVPC()),
		"sg":           buildResource("sg-0abc1234def56789a", "web-sg", realisticSecurityGroup()),
		"ng":           buildResource("prod-ng-01", "prod-ng-01", realisticNodeGroup()),
		"subnet":       buildResource("subnet-0abc1234def56789a", "public-subnet-1a", realisticSubnet()),
		"rtb":          buildResource("rtb-0abc1234def56789a", "public-rtb", realisticRouteTable()),
		"nat":          buildResource("nat-0abc1234def56789a", "prod-nat", realisticNATGateway()),
		"igw":          buildResource("igw-0abc1234def56789a", "prod-igw", realisticInternetGateway()),
		"eip":          buildResource("eipalloc-0abc1234def56789a", "prod-eip", realisticEIP()),
		"tgw":          buildResource("tgw-0abc1234def56789a", "prod-tgw", realisticTransitGateway()),
		"vpce":         buildResource("vpce-0abc1234def56789a", "s3-endpoint", realisticVPCEndpoint()),
		"eni":          buildResource("eni-0abc1234def56789a", "prod-eni", realisticENI()),
		"rds-snap":     buildResource("rds-snap-prod-20250615", "rds-snap-prod-20250615", realisticRDSSnapshot()),
		"docdb-snap":   buildResource("docdb-snap-prod-20250615", "docdb-snap-prod-20250615", realisticDocDBSnapshot()),
		"sns-sub":      buildResource("sub-12345", "sub-12345", realisticSNSSubscription()),
		"policy":       buildResource("ReadOnlyAccess", "ReadOnlyAccess", realisticIAMPolicy()),
		"iam-user":     buildResource("deploy-user", "deploy-user", realisticIAMUser()),
		"iam-group":    buildResource("developers", "developers", realisticIAMGroup()),
		"cf":           buildResource("E1A2B3C4D5E6F7", "E1A2B3C4D5E6F7", realisticCFDistribution()),
		"r53":          buildResource("/hostedzone/Z1234567890ABC", "example.com.", realisticR53Zone()),
		"apigw":        buildResource("abc123def4", "prod-api", realisticAPIGW()),
		"ecr":          buildResource("my-app", "my-app", realisticECR()),
		"efs":          buildResource("fs-0abc1234def56789a", "prod-efs", realisticEFS()),
		"eb-rule":      buildResource("daily-backup-rule", "daily-backup-rule", realisticEBRule()),
		"sfn":          buildResource("order-processing", "order-processing", realisticSFN()),
		"pipeline":     buildResource("deploy-pipeline", "deploy-pipeline", realisticPipeline()),
		"kinesis":      buildResource("events-stream", "events-stream", realisticKinesis()),
		"waf":          buildResource("prod-waf-acl", "prod-waf-acl", realisticWAF()),
		"glue":         buildResource("etl-daily-job", "etl-daily-job", realisticGlueJob()),
		"eb":           buildResource("prod-api-env", "prod-api-env", realisticEB()),
		"ses":          buildResource("example.com", "example.com", realisticSESIdentity()),
		"redshift":     buildResource("analytics-cluster", "analytics-cluster", realisticRedshift()),
		"trail":        buildResource("org-trail", "org-trail", realisticTrail()),
		"athena":       buildResource("analytics-wg", "analytics-wg", realisticAthena()),
		"codeartifact": buildResource("shared-libs", "shared-libs", realisticCodeArtifact()),
		"cb":           buildResource("build-project", "build-project", realisticCodeBuild()),
		"opensearch":   buildResource("search-prod", "search-prod", realisticOpenSearch()),
		"kms":          buildResource("12345678-1234-1234-1234-123456789012", "prod-key", realisticKMS()),
		"msk":          buildResource("events-kafka", "events-kafka", realisticMSK()),
		"backup":       buildResource("daily-backup-plan", "daily-backup-plan", realisticBackup()),
	}

	// Auto-discover all resource types plus s3_objects
	shortNames := resource.AllShortNames()
	shortNames = append(shortNames, "s3_objects")

	for _, shortName := range shortNames {
		t.Run(shortName, func(t *testing.T) {
			vd := config.GetViewDef(cfg, shortName)
			if len(vd.Detail) == 0 {
				t.Skipf("no detail paths configured for %s", shortName)
			}

			res, ok := allFixtures[shortName]
			if !ok {
				t.Skipf("no fixture for %s", shortName)
			}

			// First: check that configured paths actually resolve against the struct
			if res.RawStruct != nil {
				for _, path := range vd.Detail {
					val := fieldpath.ExtractSubtree(res.RawStruct, path)
					t.Logf("  %s.%s = %q", shortName, path, truncateStr(val, 60))
				}
			}

			// Create detail model and render
			m := views.NewDetail(res, shortName, cfg, k)
			m.SetSize(120, 40)
			view := m.View()
			plain := stripAnsi(view)

			// Every configured detail path should appear as a label in the view
			for _, path := range vd.Detail {
				// The path name (or a truncated version) should be visible.
				// PadOrTrunc receives "path:" (len+1), truncates to 22 with ellipsis.
				// So if len(path)+1 > 22 (i.e. len >= 22), the label gets truncated.
				label := path
				if len(label) >= 22 {
					label = label[:20] // PadOrTrunc truncates "path:" to 22 visible chars
				}
				if !strings.Contains(plain, label) {
					t.Errorf("detail view for %s missing field %q in output:\n%s",
						shortName, path, plain[:min(500, len(plain))])
				}
			}
		})
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
