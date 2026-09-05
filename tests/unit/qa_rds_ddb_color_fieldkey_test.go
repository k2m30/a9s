package unit

// qa_rds_ddb_color_fieldkey_test.go — the status → colour table for DynamoDB.
//
// This file used to pin which Fields key the dbi and ddb classifiers read
// first, canonical "status" over the legacy "db_instance_status" /
// "table_status". No classifier reads a Fields key any more, so the key
// precedence it guarded no longer exists and its dbi cases now live in
// qa_dbi_color_test.go. What survives is the mapping itself, driven the way
// production drives it: an SDK TableDescription through the ddb fetcher, and a
// colour computed from the worst finding's severity.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d1DDBBaseline is a healthy table: active with deletion protection on. The
// deletion-protection row stacks on top of any lifecycle state, so a baseline
// without it would make every case below warn for a reason it did not name.
func d1DDBBaseline() *ddbtypes.TableDescription {
	return &ddbtypes.TableDescription{
		TableName:                 aws.String("acme-orders"),
		TableArn:                  aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders"),
		TableStatus:               ddbtypes.TableStatusActive,
		DeletionProtectionEnabled: aws.Bool(true),
	}
}

func d1FetchDDBRow(t *testing.T, table *ddbtypes.TableDescription) resource.Resource {
	t.Helper()
	name := aws.ToString(table.TableName)
	listStub := &ddbListStub{names: []string{name}}
	descStub := &ddbDescribeStub{tables: map[string]*ddbtypes.TableDescription{name: table}}
	page, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	if err != nil {
		t.Fatalf("FetchDynamoDBTablesPage: %v", err)
	}
	if len(page.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(page.Resources))
	}
	return page.Resources[0]
}

// TestDDBColor pins the table status → colour mapping for DynamoDB.
//
// The wave-2 point-in-time-recovery row and the "(+N)" suffix row are gone: the
// enricher owns the first and prowler_w2_ddb covers it, and the suffix is added
// for display after the severity that decides colour has been read.
func TestDDBColor(t *testing.T) {
	cases := []struct {
		name   string
		status ddbtypes.TableStatus
		mutate func(*ddbtypes.TableDescription)
		want   resource.Color
	}{
		{name: "active", status: ddbtypes.TableStatusActive, want: resource.ColorHealthy},

		{name: "creating", status: ddbtypes.TableStatusCreating, want: resource.ColorWarning},
		{name: "updating", status: ddbtypes.TableStatusUpdating, want: resource.ColorWarning},
		{name: "deleting", status: ddbtypes.TableStatusDeleting, want: resource.ColorWarning},
		{name: "archiving", status: ddbtypes.TableStatusArchiving, want: resource.ColorWarning},
		{
			name:   "deletion_protection_off",
			status: ddbtypes.TableStatusActive,
			mutate: func(td *ddbtypes.TableDescription) { td.DeletionProtectionEnabled = aws.Bool(false) },
			want:   resource.ColorWarning,
		},

		{name: "kms_key_inaccessible", status: ddbtypes.TableStatusInaccessibleEncryptionCredentials, want: resource.ColorBroken},
		{name: "archived_kms_lost", status: ddbtypes.TableStatusArchived, want: resource.ColorBroken},
		{
			name:   "broken_status_outranks_deletion_protection_off",
			status: ddbtypes.TableStatusArchived,
			mutate: func(td *ddbtypes.TableDescription) { td.DeletionProtectionEnabled = aws.Bool(false) },
			want:   resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := d1DDBBaseline()
			table.TableStatus = tc.status
			if tc.mutate != nil {
				tc.mutate(table)
			}
			d1AssertColor(t, d1FetchDDBRow(t, table), tc.want)
		})
	}
}
