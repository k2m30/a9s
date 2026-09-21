package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

// b2HasCode reports whether a resource carries the given finding code.
func b2HasCode(findings []domain.Finding, code domain.FindingCode) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// DescribeTable omits BillingModeSummary on a table that has never switched
// billing mode, and PROVISIONED is what such a table runs on.
func TestDDB_BillingMode_AbsentSummaryReadsProvisioned(t *testing.T) {
	const name = "acme-orders"
	listStub := &ddbListStub{names: []string{name}}
	descStub := &ddbDescribeStub{tables: map[string]*ddbtypes.TableDescription{
		name: {
			TableName:   aws.String(name),
			TableStatus: ddbtypes.TableStatusActive,
			TableArn:    aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/" + name),
		},
	}}

	out, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	if err != nil {
		t.Fatalf("FetchDynamoDBTablesPage: %v", err)
	}
	if len(out.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(out.Resources))
	}
	if got := out.Resources[0].Fields["billing_mode"]; got != "provisioned" {
		t.Errorf("billing_mode = %q, want %q", got, "provisioned")
	}
}

func b2Ago(d time.Duration) *time.Time {
	t := time.Now().Add(-d)
	return &t
}

// A throttled first call is a transient answer, not an empty snapshot list.
func TestDocDBClusterSnapshots_ThrottleIsRetried(t *testing.T) {
	fake := &fakeDocDBDescribeDBClusterSnapshots{
		PageFunc: func(call int) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
			if call == 1 {
				return nil, &smithy.GenericAPIError{Code: "ThrottlingException", Message: "Rate exceeded"}
			}
			return &docdb.DescribeDBClusterSnapshotsOutput{
				DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{{
					DBClusterSnapshotIdentifier: aws.String("acme-docdb-snap-1"),
					DBClusterIdentifier:         aws.String("acme-docdb"),
					Status:                      aws.String("available"),
				}},
			}, nil
		},
	}

	out, err := awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchDocDBClusterSnapshotsPage: %v", err)
	}
	if len(out.Resources) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(out.Resources))
	}
	if fake.Calls < 2 {
		t.Errorf("DescribeDBClusterSnapshots called %d times; a throttle must be retried", fake.Calls)
	}
}

// StorageEncryptionType carries the effective at-rest state; the bool is false
// on groups that are nonetheless encrypted.
func TestRedis_AtRest_StorageEncryptionTypeDecides(t *testing.T) {
	for _, tc := range []struct {
		name    string
		encType elasticachetypes.StorageEncryptionType
		flag    *bool
		wantOff bool
	}{
		{"sse-kms with the flag false", elasticachetypes.StorageEncryptionTypeSseKms, aws.Bool(false), false},
		{"sse-elasticache with the flag false", elasticachetypes.StorageEncryptionTypeSseElasticache, aws.Bool(false), false},
		{"none with the flag false", elasticachetypes.StorageEncryptionTypeNone, aws.Bool(false), true},
		{"no type reported, flag false", "", aws.Bool(false), true},
		{"no type reported, flag true", "", aws.Bool(true), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockRedisRGClient{output: rgOutput(elasticachetypes.ReplicationGroup{
				ReplicationGroupId:      aws.String("acme-cache"),
				Status:                  aws.String("available"),
				AtRestEncryptionEnabled: tc.flag,
				StorageEncryptionType:   tc.encType,
			})}
			out, err := awsclient.FetchRedisPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchRedisPage: %v", err)
			}
			got := b2HasCode(out.Resources[0].Findings, awsclient.CodeRedisAtRestOff)
			if got != tc.wantOff {
				t.Errorf("at-rest-off finding = %v, want %v", got, tc.wantOff)
			}
		})
	}
}

// ElastiCache refuses a user group on a group with an AUTH token enabled, so
// an RBAC group authenticates its clients precisely by having no token.
func TestRedis_NoAuth_RBACGroupIsAuthenticated(t *testing.T) {
	for _, tc := range []struct {
		name     string
		groups   []string
		wantFind bool
	}{
		{"no user group and no token", nil, true},
		{"user group instead of a token", []string{"acme-app-users"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockRedisRGClient{output: rgOutput(elasticachetypes.ReplicationGroup{
				ReplicationGroupId:       aws.String("acme-cache"),
				Status:                   aws.String("available"),
				TransitEncryptionEnabled: aws.Bool(true),
				AuthTokenEnabled:         aws.Bool(false),
				UserGroupIds:             tc.groups,
			})}
			out, err := awsclient.FetchRedisPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchRedisPage: %v", err)
			}
			got := b2HasCode(out.Resources[0].Findings, awsclient.CodeRedisNoAuth)
			if got != tc.wantFind {
				t.Errorf("no-auth finding = %v, want %v", got, tc.wantFind)
			}
		})
	}
}

// b2TGWEnrich runs the registered tgw enricher over one gateway's attachments.
func b2TGWEnrich(t *testing.T, atts ...ec2types.TransitGatewayAttachment) awsclient.IssueEnricherResult {
	t.Helper()
	const tgwID = "tgw-00000001"
	fake := &tgwAttachmentFake{results: map[string][]ec2types.TransitGatewayAttachment{tgwID: atts}}
	res, err := awsclient.EnrichTGWAttachments(context.Background(),
		&awsclient.ServiceClients{EC2: fake}, tgwResources(tgwID), nil)
	if err != nil {
		t.Fatalf("EnrichTGWAttachments: %v", err)
	}
	return res
}

// A refused cross-account attachment carries no traffic and never will
// without someone acting on it, which is the failed tier, not silence.
func TestTGW_RejectedAttachmentIsBroken(t *testing.T) {
	for _, state := range []ec2types.TransitGatewayAttachmentState{
		ec2types.TransitGatewayAttachmentStateRejected,
		ec2types.TransitGatewayAttachmentStateRejecting,
	} {
		t.Run(string(state), func(t *testing.T) {
			res := b2TGWEnrich(t, tgwAttachment("tgw-00000001", "tgw-attach-r001", state))
			if !b2HasCode(res.Findings["tgw-00000001"], domain.FindingCode("tgw.attachment-failed")) {
				t.Errorf("state %q produced findings %v, want tgw.attachment-failed", state, res.Findings)
			}
		})
	}
}

// An attachment request the owner has not answered yet is the normal opening
// of every cross-account attachment; only one left waiting is worth a colour.
func TestTGW_PendingAcceptanceWarnsOnlyAfterADay(t *testing.T) {
	fresh := tgwAttachment("tgw-00000001", "tgw-attach-p001", ec2types.TransitGatewayAttachmentStatePendingAcceptance)
	fresh.CreationTime = b2Ago(time.Hour)
	if res := b2TGWEnrich(t, fresh); len(res.Findings) != 0 {
		t.Errorf("an hour-old request produced %v, want no finding", res.Findings)
	}

	stale := tgwAttachment("tgw-00000001", "tgw-attach-p002", ec2types.TransitGatewayAttachmentStatePendingAcceptance)
	stale.CreationTime = b2Ago(25 * time.Hour)
	res := b2TGWEnrich(t, stale)
	if !b2HasCode(res.Findings["tgw-00000001"], domain.FindingCode("tgw.attachment-transitional")) {
		t.Errorf("a day-old request produced %v, want tgw.attachment-transitional", res.Findings)
	}
}
