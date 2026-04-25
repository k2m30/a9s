// related_uncovered_struct_test.go tests stub checkers (constant results) and
// struct-extraction checkers (extract IDs from RawStruct, no cache needed)
// that were previously uncovered.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkerByTargetUncovered finds the RelatedChecker registered for shortName→target.
// (checkerByTarget is already declared in aws_iam_policies_related_test.go)
func checkerByTargetUncovered(t *testing.T, shortName, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated(shortName) {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("%s related checker for %s is nil", shortName, target)
			}
			return def.Checker
		}
	}
	t.Fatalf("%s related checker for %s not found", shortName, target)
	return nil
}

// ---------------------------------------------------------------------------
// STUB CHECKERS — return constant results regardless of input
// ---------------------------------------------------------------------------

// TestRelated_DBI_Secrets_MatchesByARN verifies the dbi→secrets checker resolves
// the managed master-user secret via DBInstance.MasterUserSecret.SecretArn by
// matching against the secrets cache by ARN (Fields["arn"] or SecretListEntry.ARN).
func TestRelated_DBI_Secrets_MatchesByARN(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-abc-defghij"
	checker := checkerByTargetUncovered(t, "dbi", "secrets")

	source := resource.Resource{
		ID:     "acme-prod-db",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("acme-prod-db"),
			MasterUserSecret: &rdstypes.MasterUserSecret{
				SecretArn: aws.String(secretARN),
			},
		},
	}
	secretRes := resource.Resource{
		ID:   "rds!db-abc-defghij",
		Name: "rds!db-abc-defghij",
		Fields: map[string]string{
			"arn": secretARN,
		},
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("rds!db-abc-defghij"),
			ARN:  aws.String(secretARN),
		},
	}
	otherSecret := resource.Resource{
		ID:     "some-other-secret",
		Fields: map[string]string{"arn": "arn:aws:secretsmanager:us-east-1:123456789012:secret:other-XYZ"},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes, otherSecret}},
	}

	got := checker(context.Background(), nil, source, cache)
	if got.Count != 1 {
		t.Errorf("expected Count=1, got %d", got.Count)
	}
	if got.TargetType != "secrets" {
		t.Errorf("expected TargetType=secrets, got %q", got.TargetType)
	}
	if len(got.ResourceIDs) != 1 || got.ResourceIDs[0] != "rds!db-abc-defghij" {
		t.Errorf("expected ResourceIDs=[rds!db-abc-defghij], got %v", got.ResourceIDs)
	}
}

// TestRelated_DBI_Secrets_NoManagedSecret verifies that when DBInstance has no
// MasterUserSecret (self-managed credentials) the checker returns Count=0.
func TestRelated_DBI_Secrets_NoManagedSecret(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbi", "secrets")

	source := resource.Resource{
		ID:     "self-managed-db",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("self-managed-db"),
			MasterUserSecret:     nil,
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "some-secret", Fields: map[string]string{"arn": "arn:aws:secretsmanager:us-east-1:123456789012:secret:some-secret"}},
		}},
	}
	got := checker(context.Background(), nil, source, cache)
	if got.Count != 0 {
		t.Errorf("expected Count=0 for no MasterUserSecret, got %d", got.Count)
	}
	if got.TargetType != "secrets" {
		t.Errorf("expected TargetType=secrets, got %q", got.TargetType)
	}
}

// TestRelated_Pipeline_CB_ReturnsUnknown verifies pipeline→cb reports Count=-1 because
// the pipeline list RawStruct (cptypes.PipelineSummary) carries no stages/actions —
// resolving CodeBuild project references would require GetPipeline per pipeline (N+1).
func TestRelated_Pipeline_CB_ReturnsUnknown(t *testing.T) {
	checker := checkerByTargetUncovered(t, "pipeline", "cb")
	res := resource.Resource{ID: "my-pipeline", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 (undeterminable — no stages on PipelineSummary), got %d", got.Count)
	}
	if got.TargetType != "cb" {
		t.Errorf("expected TargetType=cb, got %q", got.TargetType)
	}
}

// TestRelated_Pipeline_Role_ReturnsUnknown verifies pipeline→role reports Count=-1 because
// the pipeline list RawStruct has no RoleArn — it is only on GetPipelineOutput.
func TestRelated_Pipeline_Role_ReturnsUnknown(t *testing.T) {
	checker := checkerByTargetUncovered(t, "pipeline", "role")
	res := resource.Resource{ID: "my-pipeline", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 (undeterminable — no RoleArn on PipelineSummary), got %d", got.Count)
	}
	if got.TargetType != "role" {
		t.Errorf("expected TargetType=role, got %q", got.TargetType)
	}
}

// TestRelated_Lambda_SQS_UnknownWithoutClients verifies the lambda→sqs checker
// returns Count=-1 when live Lambda clients are unavailable (requires
// lambda:ListEventSourceMappings — FunctionConfiguration does not embed event
// sources, so there is no cache-only path).
func TestRelated_Lambda_SQS_UnknownWithoutClients(t *testing.T) {
	checker := checkerByTargetUncovered(t, "lambda", "sqs")
	res := resource.Resource{ID: "my-function", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 (requires live API), got %d", got.Count)
	}
	if got.TargetType != "sqs" {
		t.Errorf("expected TargetType=sqs, got %q", got.TargetType)
	}
}

// TestRelated_Lambda_SQS_EmptyIDReturnsZero verifies the checker short-circuits
// with Count=0 when the function has no identifier, avoiding any API call.
func TestRelated_Lambda_SQS_EmptyIDReturnsZero(t *testing.T) {
	checker := checkerByTargetUncovered(t, "lambda", "sqs")
	res := resource.Resource{ID: "", Name: "", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0 for empty function id, got %d", got.Count)
	}
}

// TestRelated_Lambda_CFN_UnknownWithoutClients verifies the lambda→cfn checker
// returns Count=-1 when live Lambda clients are unavailable (requires
// lambda:ListTags because FunctionConfiguration does not carry tags).
func TestRelated_Lambda_CFN_UnknownWithoutClients(t *testing.T) {
	checker := checkerByTargetUncovered(t, "lambda", "cfn")
	res := resource.Resource{
		ID:     "my-function",
		Fields: map[string]string{},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-function"),
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 (requires live API), got %d", got.Count)
	}
	if got.TargetType != "cfn" {
		t.Errorf("expected TargetType=cfn, got %q", got.TargetType)
	}
}

// TestRelated_Lambda_CFN_NoARNReturnsZero verifies the checker reports Count=0
// when the function has no ARN — tags cannot be resolved, and there is no
// link to return.
func TestRelated_Lambda_CFN_NoARNReturnsZero(t *testing.T) {
	checker := checkerByTargetUncovered(t, "lambda", "cfn")
	res := resource.Resource{
		ID:     "my-function",
		Fields: map[string]string{},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0 (no function ARN), got %d", got.Count)
	}
}

// TestRelated_Lambda_EbRule_UnknownWithoutClients verifies the lambda→eb-rule
// checker returns Count=-1 without live clients. Rule structs do not include
// targets (those come from events:ListTargetsByRule), so the checker has no
// cache-only signal to use.
func TestRelated_Lambda_EbRule_UnknownWithoutClients(t *testing.T) {
	checker := checkerByTargetUncovered(t, "lambda", "eb-rule")
	res := resource.Resource{
		ID:     "my-function",
		Fields: map[string]string{},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-function"),
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1, got %d", got.Count)
	}
	if got.TargetType != "eb-rule" {
		t.Errorf("expected TargetType=eb-rule, got %q", got.TargetType)
	}
}

// TestRelated_ELB_R53_WithDNSReturnsUnknown: real ELB with dns_name → -1 (records per-zone).
func TestRelated_ELB_R53_WithDNSReturnsUnknown(t *testing.T) {
	checker := checkerByTargetUncovered(t, "elb", "r53")
	res := resource.Resource{
		ID: "my-alb",
		Fields: map[string]string{
			"dns_name": "my-alb-1234.us-east-1.elb.amazonaws.com",
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 (alias records per-zone), got %d", got.Count)
	}
	if got.TargetType != "r53" {
		t.Errorf("expected TargetType=r53, got %q", got.TargetType)
	}
}

// TestRelated_ELB_R53_EmptyDNSReturnsZero: ELB without dns_name → 0 (nothing to resolve).
func TestRelated_ELB_R53_EmptyDNSReturnsZero(t *testing.T) {
	checker := checkerByTargetUncovered(t, "elb", "r53")
	res := resource.Resource{ID: "my-alb", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0 (empty dns_name), got %d", got.Count)
	}
}

// TestRelated_SFN_EbRule_ReturnsZeroOnEmptyARN verifies sfn→eb-rule reports Count=0
// when the state machine ARN field is empty. checkSFNEbRule uses a live
// ListRuleNamesByTarget call; without an ARN there is nothing to look up.
func TestRelated_SFN_EbRule_ReturnsZeroOnEmptyARN(t *testing.T) {
	checker := checkerByTargetUncovered(t, "sfn", "eb-rule")
	res := resource.Resource{ID: "my-state-machine", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0 (no ARN — nothing to look up), got %d", got.Count)
	}
	if got.TargetType != "eb-rule" {
		t.Errorf("expected TargetType=eb-rule, got %q", got.TargetType)
	}
}

// TestRelated_R53_ELB_ZoneReturnsUnknown: real zone → -1 (alias records live per-zone).
func TestRelated_R53_ELB_ZoneReturnsUnknown(t *testing.T) {
	checker := checkerByTargetUncovered(t, "r53", "elb")
	res := resource.Resource{ID: "Z1234ABCDEFG", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 (alias records per-zone), got %d", got.Count)
	}
	if got.TargetType != "elb" {
		t.Errorf("expected TargetType=elb, got %q", got.TargetType)
	}
}

// TestRelated_R53_ELB_EmptyZoneReturnsZero: missing zone id → 0.
func TestRelated_R53_ELB_EmptyZoneReturnsZero(t *testing.T) {
	checker := checkerByTargetUncovered(t, "r53", "elb")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0 (empty zone id), got %d", got.Count)
	}
}

// TestRelated_R53_CF_ZoneReturnsUnknown: real zone → -1 (alias records per-zone).
func TestRelated_R53_CF_ZoneReturnsUnknown(t *testing.T) {
	checker := checkerByTargetUncovered(t, "r53", "cf")
	res := resource.Resource{ID: "Z1234ABCDEFG", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 (alias records per-zone), got %d", got.Count)
	}
	if got.TargetType != "cf" {
		t.Errorf("expected TargetType=cf, got %q", got.TargetType)
	}
}

// TestRelated_R53_CF_EmptyZoneReturnsZero: missing zone id → 0.
func TestRelated_R53_CF_EmptyZoneReturnsZero(t *testing.T) {
	checker := checkerByTargetUncovered(t, "r53", "cf")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0 (empty zone id), got %d", got.Count)
	}
}

// TestRelated_R53_ACM_ZoneReturnsUnknown: real zone → -1 (validation records per-zone).
func TestRelated_R53_ACM_ZoneReturnsUnknown(t *testing.T) {
	checker := checkerByTargetUncovered(t, "r53", "acm")
	res := resource.Resource{ID: "Z1234ABCDEFG", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 (validation records per-zone), got %d", got.Count)
	}
	if got.TargetType != "acm" {
		t.Errorf("expected TargetType=acm, got %q", got.TargetType)
	}
}

// TestRelated_R53_ACM_EmptyZoneReturnsZero: missing zone id → 0.
func TestRelated_R53_ACM_EmptyZoneReturnsZero(t *testing.T) {
	checker := checkerByTargetUncovered(t, "r53", "acm")
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0 (empty zone id), got %d", got.Count)
	}
}

func TestRelated_KMS_S3_EmptyID(t *testing.T) {
	checker := checkerByTargetUncovered(t, "kms", "s3")

	// Empty ID → Count:0 (cannot search without a key ID)
	empty := resource.Resource{ID: "", Fields: map[string]string{}}
	got := checker(context.Background(), nil, empty, nil)
	if got.Count != 0 {
		t.Errorf("empty ID: expected Count=0, got %d", got.Count)
	}
	if got.TargetType != "s3" {
		t.Errorf("empty ID: expected TargetType=s3, got %q", got.TargetType)
	}

	// Non-empty ID → Count:-1 (S3 resources don't expose KMS key IDs)
	withID := resource.Resource{ID: "abc-def-1234-5678-abcd", Fields: map[string]string{}}
	got2 := checker(context.Background(), nil, withID, nil)
	if got2.Count != -1 {
		t.Errorf("non-empty ID: expected Count=-1, got %d", got2.Count)
	}
	if got2.TargetType != "s3" {
		t.Errorf("non-empty ID: expected TargetType=s3, got %q", got2.TargetType)
	}
}

// ---------------------------------------------------------------------------
// STRUCT EXTRACTION CHECKERS
// ---------------------------------------------------------------------------

// --- DBI → SG ---

func TestRelated_DBI_SG_ExtractsSecurityGroups(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbi", "sg")
	res := resource.Resource{
		ID:     "db-instance-1",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String("sg-abc123")},
			},
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 1 {
		t.Errorf("expected Count=1, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 1 || got.ResourceIDs[0] != "sg-abc123" {
		t.Errorf("expected ResourceIDs=[sg-abc123], got %v", got.ResourceIDs)
	}
}

func TestRelated_DBI_SG_NoSecurityGroups(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbi", "sg")
	res := resource.Resource{
		ID:        "db-instance-1",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{}},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0, got %d", got.Count)
	}
}

func TestRelated_DBI_SG_WrongType(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbi", "sg")
	res := resource.Resource{
		ID:        "db-instance-1",
		Fields:    map[string]string{},
		RawStruct: "not-a-struct",
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != -1 {
		t.Errorf("expected Count=-1 for wrong type, got %d", got.Count)
	}
}

// --- DBC → SG ---

func TestRelated_DBC_SG_ExtractsSecurityGroups(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbc", "sg")
	res := resource.Resource{
		ID:     "my-docdb-cluster",
		Fields: map[string]string{},
		RawStruct: docdbtypes.DBCluster{
			VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String("sg-docdb1")},
			},
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 1 {
		t.Errorf("expected Count=1, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 1 || got.ResourceIDs[0] != "sg-docdb1" {
		t.Errorf("expected ResourceIDs=[sg-docdb1], got %v", got.ResourceIDs)
	}
}

// --- DocDB Snapshot → DBC ---

func TestRelated_DbcSnap_DBC_ExtractsCluster(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbc-snap", "dbc")
	res := resource.Resource{
		ID:     "snap-001",
		Fields: map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{
			DBClusterIdentifier: aws.String("my-cluster"),
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 1 {
		t.Errorf("expected Count=1, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 1 || got.ResourceIDs[0] != "my-cluster" {
		t.Errorf("expected ResourceIDs=[my-cluster], got %v", got.ResourceIDs)
	}
}

func TestRelated_DbcSnap_DBC_NoCluster(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbc-snap", "dbc")
	res := resource.Resource{
		ID:        "snap-001",
		Fields:    map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{DBClusterIdentifier: nil},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0, got %d", got.Count)
	}
}

// --- DocDB Snapshot → KMS ---

func TestRelated_DbcSnap_KMS_ExtractsKey(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbc-snap", "kms")
	res := resource.Resource{
		ID:     "snap-001",
		Fields: map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/abc-def"),
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 1 {
		t.Errorf("expected Count=1, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 1 || got.ResourceIDs[0] != "abc-def" {
		t.Errorf("expected ResourceIDs=[abc-def], got %v", got.ResourceIDs)
	}
}

func TestRelated_DbcSnap_KMS_NoKey(t *testing.T) {
	checker := checkerByTargetUncovered(t, "dbc-snap", "kms")
	res := resource.Resource{
		ID:        "snap-001",
		Fields:    map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{KmsKeyId: nil},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0, got %d", got.Count)
	}
}

// --- MSK → SG ---

func TestRelated_MSK_SG_ExtractsFromProvisioned(t *testing.T) {
	checker := checkerByTargetUncovered(t, "msk", "sg")
	res := resource.Resource{
		ID:     "my-msk-cluster",
		Fields: map[string]string{},
		RawStruct: kafkatypes.Cluster{
			Provisioned: &kafkatypes.Provisioned{
				BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
					SecurityGroups: []string{"sg-msk1", "sg-msk2"},
				},
			},
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 2 {
		t.Errorf("expected Count=2, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 2 {
		t.Errorf("expected 2 ResourceIDs, got %v", got.ResourceIDs)
	}
}

func TestRelated_MSK_SG_NilProvisioned(t *testing.T) {
	checker := checkerByTargetUncovered(t, "msk", "sg")
	res := resource.Resource{
		ID:        "my-msk-cluster",
		Fields:    map[string]string{},
		RawStruct: kafkatypes.Cluster{Provisioned: nil},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0, got %d", got.Count)
	}
}

// --- EventBridge Rule → Role ---

func TestRelated_EbRule_Role_ExtractsRoleName(t *testing.T) {
	checker := checkerByTargetUncovered(t, "eb-rule", "role")
	res := resource.Resource{
		ID:     "my-rule",
		Fields: map[string]string{},
		RawStruct: eventbridgetypes.Rule{
			RoleArn: aws.String("arn:aws:iam::123456789012:role/my-role"),
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 1 {
		t.Errorf("expected Count=1, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 1 || got.ResourceIDs[0] != "my-role" {
		t.Errorf("expected ResourceIDs=[my-role], got %v", got.ResourceIDs)
	}
}

func TestRelated_EbRule_Role_NoRoleArn(t *testing.T) {
	checker := checkerByTargetUncovered(t, "eb-rule", "role")
	res := resource.Resource{
		ID:        "my-rule",
		Fields:    map[string]string{},
		RawStruct: eventbridgetypes.Rule{RoleArn: nil},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0, got %d", got.Count)
	}
}

// --- OpenSearch → Logs ---

func TestRelated_OpenSearch_Logs_ExtractsLogGroups(t *testing.T) {
	checker := checkerByTargetUncovered(t, "opensearch", "logs")
	res := resource.Resource{
		ID:     "my-domain",
		Fields: map[string]string{},
		RawStruct: opensearchtypes.DomainStatus{
			LogPublishingOptions: map[string]opensearchtypes.LogPublishingOption{
				string(opensearchtypes.LogTypeIndexSlowLogs): {
					CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/opensearch/domains/my-domain/index-slow-logs:*"),
				},
			},
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 1 {
		t.Errorf("expected Count=1, got %d", got.Count)
	}
	if len(got.ResourceIDs) != 1 {
		t.Fatalf("expected 1 ResourceID, got %v", got.ResourceIDs)
	}
	want := "/aws/opensearch/domains/my-domain/index-slow-logs"
	if got.ResourceIDs[0] != want {
		t.Errorf("expected ResourceIDs[0]=%q, got %q", want, got.ResourceIDs[0])
	}
}

func TestRelated_OpenSearch_Logs_Empty(t *testing.T) {
	checker := checkerByTargetUncovered(t, "opensearch", "logs")
	res := resource.Resource{
		ID:     "my-domain",
		Fields: map[string]string{},
		RawStruct: opensearchtypes.DomainStatus{
			LogPublishingOptions: map[string]opensearchtypes.LogPublishingOption{},
		},
	}
	got := checker(context.Background(), nil, res, nil)
	if got.Count != 0 {
		t.Errorf("expected Count=0, got %d", got.Count)
	}
}
