package unit

// backup_selection_coverage_test.go — one backup coverage evaluator.
//
// AWS Backup evaluates each selection of a plan on its own:
//
//	(Resources match OR ListOfTags match) AND every Conditions clause AND NOT NotResources match
//
// with an empty Resources and an empty ListOfTags standing for every eligible
// resource. A plan covers a resource when any one of its selections does.
// Sources: API_BackupSelection (Resources OR, ListOfTags OR, Conditions AND)
// and "Assign resources with AWS CLI" (the JSON examples every case below is
// drawn from, including `*` matching zero or more non-whitespace characters).
//
// Plan rows are built by FetchBackupPlansPage over a fake Backup API, so the
// selections reach the evaluator in whatever structure the fetcher keeps them.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const (
	bk551Account = "123456789012"
	bk551Region  = "us-east-1"
	bk551PlanID  = "5a5b5c5d-0551-4551-8551-000000000551"
	bk551Role    = "arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"

	bk551VolumeID    = "vol-0a1b2c3d4e5f65510"
	bk551VolumeARN   = "arn:aws:ec2:us-east-1:123456789012:volume/vol-0a1b2c3d4e5f65510"
	bk551InstanceID  = "i-0a1b2c3d4e5f65510"
	bk551InstanceARN = "arn:aws:ec2:us-east-1:123456789012:instance/i-0a1b2c3d4e5f65510"
	bk551TableARN    = "arn:aws:dynamodb:us-east-1:123456789012:table/acme-ledger"
	bk551DBARN       = "arn:aws:rds:us-east-1:123456789012:db:acme-ledger-db"
	bk551ClusterARN  = "arn:aws:rds:us-east-1:123456789012:cluster:acme-ledger-cluster"
	bk551EFSARN      = "arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0a1b2c3d4e5f65510"
	bk551Bucket      = "acme-ledger-exports"
	bk551BucketARN   = "arn:aws:s3:::acme-ledger-exports"
	bk551FSxARN      = "arn:aws:fsx:us-east-1:123456789012:file-system/fs-0123456789abcdef0"
)

// ── fake Backup API and plan builder ─────────────────────────────────────

type bk551BackupFake struct {
	awsclient.BackupAPI

	selections []backuptypes.BackupSelection
	failGetAt  int // index of the selection whose GetBackupSelection fails; -1 for none

	optIn       map[string]bool // nil: every resource type opted in
	settingsErr error
}

// bk551AllOptedIn is the ResourceTypeOptInPreference of a Region where every
// resource type AWS Backup supports is switched on.
func bk551AllOptedIn() map[string]bool {
	return map[string]bool{
		"Aurora": true, "CloudFormation": true, "DocumentDB": true, "DynamoDB": true, "EBS": true,
		"EC2": true, "EFS": true, "FSx": true, "Neptune": true, "RDS": true, "Redshift": true,
		"S3": true, "SAP HANA on Amazon EC2": true, "Storage Gateway": true, "Timestream": true,
		"VirtualMachine": true,
	}
}

func (f *bk551BackupFake) DescribeRegionSettings(_ context.Context, _ *backup.DescribeRegionSettingsInput, _ ...func(*backup.Options)) (*backup.DescribeRegionSettingsOutput, error) {
	if f.settingsErr != nil {
		return nil, f.settingsErr
	}
	optIn := f.optIn
	if optIn == nil {
		optIn = bk551AllOptedIn()
	}
	return &backup.DescribeRegionSettingsOutput{
		ResourceTypeOptInPreference:      optIn,
		ResourceTypeManagementPreference: map[string]bool{"DynamoDB": true, "EFS": true},
	}, nil
}

func (f *bk551BackupFake) ListBackupPlans(_ context.Context, _ *backup.ListBackupPlansInput, _ ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	return &backup.ListBackupPlansOutput{
		BackupPlansList: []backuptypes.BackupPlansListMember{{
			BackupPlanId:   aws.String(bk551PlanID),
			BackupPlanName: aws.String("acme-ledger-nightly"),
			BackupPlanArn:  aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:" + bk551PlanID),
			CreationDate:   aws.Time(time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)),
			VersionId:      aws.String("ZjYxMDk4ZTQtNTU1MQ=="),
		}},
	}, nil
}

func (f *bk551BackupFake) ListBackupSelections(_ context.Context, in *backup.ListBackupSelectionsInput, _ ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	out := &backup.ListBackupSelectionsOutput{}
	for i, sel := range f.selections {
		out.BackupSelectionsList = append(out.BackupSelectionsList, backuptypes.BackupSelectionsListMember{
			BackupPlanId:  in.BackupPlanId,
			SelectionId:   aws.String(bk551SelectionID(i)),
			SelectionName: sel.SelectionName,
			IamRoleArn:    sel.IamRoleArn,
			CreationDate:  aws.Time(time.Date(2026, 3, 1, 9, 5, 0, 0, time.UTC)),
		})
	}
	return out, nil
}

func (f *bk551BackupFake) GetBackupSelection(_ context.Context, in *backup.GetBackupSelectionInput, _ ...func(*backup.Options)) (*backup.GetBackupSelectionOutput, error) {
	for i, sel := range f.selections {
		if bk551SelectionID(i) != aws.ToString(in.SelectionId) {
			continue
		}
		if i == f.failGetAt {
			return nil, errors.New("operation error Backup: GetBackupSelection, https response error StatusCode: 400, ThrottlingException: Rate exceeded")
		}
		return &backup.GetBackupSelectionOutput{
			BackupPlanId:    in.BackupPlanId,
			SelectionId:     in.SelectionId,
			BackupSelection: &sel,
		}, nil
	}
	return nil, &backuptypes.ResourceNotFoundException{Message: aws.String("selection not found")}
}

func bk551SelectionID(i int) string {
	return fmt.Sprintf("0551%04d-aaaa-4bbb-8ccc-0000000005%02d", i, i)
}

// bk551Fetch runs the real fetcher over the given selections and returns the
// one plan row it produces. failGetAt names a selection whose read fails, so
// the plan's selection set is incomplete; -1 reads every selection.
func bk551Fetch(t *testing.T, failGetAt int, sels ...backuptypes.BackupSelection) resource.Resource {
	t.Helper()
	res, err := awsclient.FetchBackupPlansPage(context.Background(), bk551NewFake(failGetAt, sels), "")
	if err != nil {
		t.Fatalf("FetchBackupPlansPage: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("FetchBackupPlansPage returned %d plans, want 1", len(res.Resources))
	}
	return res.Resources[0]
}

func bk551NewFake(failGetAt int, sels []backuptypes.BackupSelection) *bk551BackupFake {
	for i := range sels {
		if sels[i].SelectionName == nil {
			sels[i].SelectionName = aws.String(fmt.Sprintf("acme-selection-%d", i))
		}
		if sels[i].IamRoleArn == nil {
			sels[i].IamRoleArn = aws.String(bk551Role)
		}
	}
	return &bk551BackupFake{selections: sels, failGetAt: failGetAt}
}

func bk551Plan(t *testing.T, sels ...backuptypes.BackupSelection) resource.Resource {
	t.Helper()
	return bk551Fetch(t, -1, sels...)
}

func bk551Cache(plans ...resource.Resource) resource.ResourceCache {
	return resource.ResourceCache{"backup": resource.ResourceCacheEntry{Resources: plans}}
}

func bk551Cond(key, value string) backuptypes.ConditionParameter {
	return backuptypes.ConditionParameter{ConditionKey: aws.String("aws:ResourceTag/" + key), ConditionValue: aws.String(value)}
}

func bk551ListTag(key, value string) backuptypes.Condition {
	return backuptypes.Condition{
		ConditionType:  backuptypes.ConditionTypeStringequals,
		ConditionKey:   aws.String(key),
		ConditionValue: aws.String(value),
	}
}

// ── BackupSelectionCovers ────────────────────────────────────────────────

func TestBackupSelectionCovers_AWSRule(t *testing.T) {
	allVolumes := "arn:aws:ec2:*:*:volume/*"
	tests := []struct {
		name string
		sel  backuptypes.BackupSelection
		arn  string
		tags map[string]string
		want bool
	}{
		{"exact ARN", backuptypes.BackupSelection{Resources: []string{bk551VolumeARN}}, bk551VolumeARN, nil, true},
		{"exact ARN of another resource", backuptypes.BackupSelection{Resources: []string{bk551TableARN}}, bk551VolumeARN, nil, false},
		{"one of several ARNs (OR)", backuptypes.BackupSelection{Resources: []string{bk551TableARN, bk551VolumeARN}}, bk551VolumeARN, nil, true},
		{"resource-type wildcard", backuptypes.BackupSelection{Resources: []string{allVolumes}}, bk551VolumeARN, nil, true},
		{"resource-type wildcard of another type", backuptypes.BackupSelection{Resources: []string{allVolumes}}, bk551InstanceARN, nil, false},
		{"wildcard in the middle takes db", backuptypes.BackupSelection{Resources: []string{"arn:aws:rds:*:*:db:*"}}, bk551DBARN, nil, true},
		{"wildcard in the middle leaves cluster", backuptypes.BackupSelection{Resources: []string{"arn:aws:rds:*:*:db:*"}}, bk551ClusterARN, nil, false},
		{"service prefix", backuptypes.BackupSelection{Resources: []string{"arn:aws:fsx:*"}}, bk551FSxARN, nil, true},
		{"service prefix of another service", backuptypes.BackupSelection{Resources: []string{"arn:aws:fsx:*"}}, bk551VolumeARN, nil, false},
		{"name prefix", backuptypes.BackupSelection{Resources: []string{"arn:aws:s3:::acme-ledger-*"}}, bk551BucketARN, nil, true},
		{"name prefix not matching", backuptypes.BackupSelection{Resources: []string{"arn:aws:s3:::acme-audit-*"}}, bk551BucketARN, nil, false},
		{"star alone", backuptypes.BackupSelection{Resources: []string{"*"}}, bk551TableARN, nil, true},

		{"star minus volumes: volume", backuptypes.BackupSelection{Resources: []string{"*"}, NotResources: []string{allVolumes}}, bk551VolumeARN, nil, false},
		{"star minus volumes: table", backuptypes.BackupSelection{Resources: []string{"*"}, NotResources: []string{allVolumes}}, bk551TableARN, nil, true},
		{"NotResources entries OR", backuptypes.BackupSelection{Resources: []string{"*"}, NotResources: []string{"arn:aws:fsx:*", "arn:aws:rds:*"}}, bk551ClusterARN, nil, false},
		{"NotResources exact ARN", backuptypes.BackupSelection{Resources: []string{allVolumes}, NotResources: []string{bk551VolumeARN}}, bk551VolumeARN, nil, false},

		{"empty selection takes every resource", backuptypes.BackupSelection{}, bk551TableARN, nil, true},
		{"NotResources only: others", backuptypes.BackupSelection{NotResources: []string{bk551VolumeARN}}, bk551TableARN, nil, true},
		{"NotResources only: the excluded one", backuptypes.BackupSelection{NotResources: []string{bk551VolumeARN}}, bk551VolumeARN, nil, false},
		{"NotResources beats a tag match", backuptypes.BackupSelection{
			ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")}, NotResources: []string{bk551VolumeARN},
		}, bk551VolumeARN, map[string]string{"backup": "true"}, false},

		{"Conditions only: tagged", backuptypes.BackupSelection{Conditions: &backuptypes.Conditions{
			StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")},
		}}, bk551TableARN, map[string]string{"backup": "true"}, true},
		{"Conditions only: untagged", backuptypes.BackupSelection{Conditions: &backuptypes.Conditions{
			StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")},
		}}, bk551TableARN, nil, false},
		{"Conditions only: other value", backuptypes.BackupSelection{Conditions: &backuptypes.Conditions{
			StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")},
		}}, bk551TableARN, map[string]string{"backup": "false"}, false},
		{"StringEquals is case sensitive", backuptypes.BackupSelection{Conditions: &backuptypes.Conditions{
			StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")},
		}}, bk551TableARN, map[string]string{"backup": "True"}, false},

		{"Resources and Conditions: tagged volume", backuptypes.BackupSelection{
			Resources:  []string{allVolumes},
			Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
		}, bk551VolumeARN, map[string]string{"backup": "true"}, true},
		{"Resources and Conditions: untagged volume", backuptypes.BackupSelection{
			Resources:  []string{allVolumes},
			Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
		}, bk551VolumeARN, map[string]string{"Name": "acme-ledger-data"}, false},
		{"Resources and Conditions: tagged table outside Resources", backuptypes.BackupSelection{
			Resources:  []string{allVolumes},
			Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
		}, bk551TableARN, map[string]string{"backup": "true"}, false},

		{"two StringEquals AND: both", backuptypes.BackupSelection{Resources: []string{allVolumes}, Conditions: &backuptypes.Conditions{
			StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true"), bk551Cond("stage", "prod")},
		}}, bk551VolumeARN, map[string]string{"backup": "true", "stage": "prod"}, true},
		{"two StringEquals AND: one", backuptypes.BackupSelection{Resources: []string{allVolumes}, Conditions: &backuptypes.Conditions{
			StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true"), bk551Cond("stage", "prod")},
		}}, bk551VolumeARN, map[string]string{"backup": "true"}, false},

		{"StringNotEquals: excluded value", backuptypes.BackupSelection{Resources: []string{allVolumes}, Conditions: &backuptypes.Conditions{
			StringEquals:    []backuptypes.ConditionParameter{bk551Cond("backup", "true")},
			StringNotEquals: []backuptypes.ConditionParameter{bk551Cond("stage", "test")},
		}}, bk551VolumeARN, map[string]string{"backup": "true", "stage": "test"}, false},
		{"StringNotEquals: other value", backuptypes.BackupSelection{Resources: []string{allVolumes}, Conditions: &backuptypes.Conditions{
			StringEquals:    []backuptypes.ConditionParameter{bk551Cond("backup", "true")},
			StringNotEquals: []backuptypes.ConditionParameter{bk551Cond("stage", "test")},
		}}, bk551VolumeARN, map[string]string{"backup": "true", "stage": "prod"}, true},
		{"StringNotEquals: key absent", backuptypes.BackupSelection{Resources: []string{allVolumes}, Conditions: &backuptypes.Conditions{
			StringEquals:    []backuptypes.ConditionParameter{bk551Cond("backup", "true")},
			StringNotEquals: []backuptypes.ConditionParameter{bk551Cond("stage", "test")},
		}}, bk551VolumeARN, map[string]string{"backup": "true"}, true},

		{"StringLike prefix: match", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike: []backuptypes.ConditionParameter{bk551Cond("key1", "include*")},
		}}, bk551TableARN, map[string]string{"key1": "include-ledger"}, true},
		{"StringLike prefix: no match", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike: []backuptypes.ConditionParameter{bk551Cond("key1", "include*")},
		}}, bk551TableARN, map[string]string{"key1": "skip-ledger"}, false},
		{"StringLike any value: present", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike: []backuptypes.ConditionParameter{bk551Cond("backup", "*")},
		}}, bk551TableARN, map[string]string{"backup": "weekly"}, true},
		{"StringLike any value: absent", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike: []backuptypes.ConditionParameter{bk551Cond("backup", "*")},
		}}, bk551TableARN, map[string]string{"stage": "prod"}, false},
		{"StringLike star stops at whitespace", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike: []backuptypes.ConditionParameter{bk551Cond("tier", "prod*")},
		}}, bk551TableARN, map[string]string{"tier": "prod server"}, false},
		{"StringLike star within a word", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike: []backuptypes.ConditionParameter{bk551Cond("tier", "prod*")},
		}}, bk551TableARN, map[string]string{"tier": "prod_server"}, true},
		{"StringNotLike: excluded", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike:    []backuptypes.ConditionParameter{bk551Cond("key1", "include*")},
			StringNotLike: []backuptypes.ConditionParameter{bk551Cond("key2", "*exclude*")},
		}}, bk551TableARN, map[string]string{"key1": "include-ledger", "key2": "please-exclude-me"}, false},
		{"StringNotLike: kept", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike:    []backuptypes.ConditionParameter{bk551Cond("key1", "include*")},
			StringNotLike: []backuptypes.ConditionParameter{bk551Cond("key2", "*exclude*")},
		}}, bk551TableARN, map[string]string{"key1": "include-ledger", "key2": "keep"}, true},
		{"StringNotLike: key absent", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringLike:    []backuptypes.ConditionParameter{bk551Cond("key1", "include*")},
			StringNotLike: []backuptypes.ConditionParameter{bk551Cond("key2", "*exclude*")},
		}}, bk551TableARN, map[string]string{"key1": "include-ledger"}, true},

		{"ListOfTags entries OR", backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{
			bk551ListTag("backup", "daily"), bk551ListTag("backup", "weekly"),
		}}, bk551TableARN, map[string]string{"backup": "weekly"}, true},
		{"ListOfTags no entry matches", backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{
			bk551ListTag("backup", "daily"), bk551ListTag("backup", "weekly"),
		}}, bk551TableARN, map[string]string{"backup": "monthly"}, false},
		{"ListOfTags key in aws:ResourceTag form", backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{
			bk551ListTag("aws:ResourceTag/backup", "daily"),
		}}, bk551TableARN, map[string]string{"backup": "daily"}, true},

		// Resources OR ListOfTags, then AND Conditions — the CLI guide's
		// "all FSx, plus everything tagged backup:true, except stage:test".
		{"Resources or ListOfTags: untagged FSx", backuptypes.BackupSelection{
			Resources: []string{"arn:aws:fsx:*"}, ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")},
			Conditions: &backuptypes.Conditions{StringNotEquals: []backuptypes.ConditionParameter{bk551Cond("stage", "test")}},
		}, bk551FSxARN, nil, true},
		{"Resources or ListOfTags: tagged volume", backuptypes.BackupSelection{
			Resources: []string{"arn:aws:fsx:*"}, ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")},
			Conditions: &backuptypes.Conditions{StringNotEquals: []backuptypes.ConditionParameter{bk551Cond("stage", "test")}},
		}, bk551VolumeARN, map[string]string{"backup": "true"}, true},
		{"Resources or ListOfTags: untagged volume", backuptypes.BackupSelection{
			Resources: []string{"arn:aws:fsx:*"}, ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")},
			Conditions: &backuptypes.Conditions{StringNotEquals: []backuptypes.ConditionParameter{bk551Cond("stage", "test")}},
		}, bk551VolumeARN, nil, false},
		{"Resources or ListOfTags, AND Conditions: stage test", backuptypes.BackupSelection{
			Resources: []string{"arn:aws:fsx:*"}, ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")},
			Conditions: &backuptypes.Conditions{StringNotEquals: []backuptypes.ConditionParameter{bk551Cond("stage", "test")}},
		}, bk551VolumeARN, map[string]string{"backup": "true", "stage": "test"}, false},

		// AWS tag keys may contain "=", so a key and a value can never be
		// recovered from one "k=v" string.
		{"ListOfTags key containing '='", backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{
			bk551ListTag("cost=center", "ops"),
		}}, bk551TableARN, map[string]string{"cost=center": "ops"}, true},
		{"ListOfTags key containing '=' is not a prefix split", backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{
			bk551ListTag("cost=center", "ops"),
		}}, bk551TableARN, map[string]string{"cost": "center=ops"}, false},
		{"Conditions key containing '='", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringEquals: []backuptypes.ConditionParameter{bk551Cond("cost=center", "ops")},
		}}, bk551TableARN, map[string]string{"cost=center": "ops"}, true},
		{"Conditions key containing '=' is not a prefix split", backuptypes.BackupSelection{Resources: []string{"*"}, Conditions: &backuptypes.Conditions{
			StringEquals: []backuptypes.ConditionParameter{bk551Cond("cost=center", "ops")},
		}}, bk551TableARN, map[string]string{"cost": "center=ops"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := awsclient.BackupSelectionCovers(tc.sel, tc.arn, tc.tags); got != tc.want {
				t.Errorf("BackupSelectionCovers(%s) = %v, want %v", tc.arn, got, tc.want)
			}
		})
	}
}

// ── BackupPlanCovers ─────────────────────────────────────────────────────

// One selection's NotResources removes a resource from that selection only.
// Selection A still takes vol-1 in, so the plan covers it.
func TestBackupPlanCovers_ExclusionInOneSelectionDoesNotUndoAnother(t *testing.T) {
	plan := bk551Plan(t,
		backuptypes.BackupSelection{Resources: []string{"arn:aws:ec2:*:*:volume/*"}},
		backuptypes.BackupSelection{NotResources: []string{bk551VolumeARN}},
	)
	covered, known := awsclient.BackupPlanCovers(plan, bk551VolumeARN, nil, true)
	if !covered || !known {
		t.Errorf("BackupPlanCovers = (%v, %v), want (true, true): selection A takes every volume in", covered, known)
	}

	only := bk551Plan(t, backuptypes.BackupSelection{
		Resources: []string{"arn:aws:ec2:*:*:volume/*"}, NotResources: []string{bk551VolumeARN},
	})
	covered, known = awsclient.BackupPlanCovers(only, bk551VolumeARN, nil, true)
	if covered || !known {
		t.Errorf("BackupPlanCovers = (%v, %v), want (false, true): the plan's only selection excludes vol-1", covered, known)
	}
}

func TestBackupPlanCovers_Knowledge(t *testing.T) {
	byTag := backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")}}
	volumesIfTagged := backuptypes.BackupSelection{
		Resources:  []string{"arn:aws:ec2:*:*:volume/*"},
		Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
	}
	tablesIfTagged := backuptypes.BackupSelection{
		Resources:  []string{"arn:aws:dynamodb:*:*:table/*"},
		Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
	}
	tests := []struct {
		name        string
		plan        resource.Resource
		arn         string
		tags        map[string]string
		tagsKnown   bool
		wantCovered bool
		wantKnown   bool
	}{
		{"ARN selection, tags unknown", bk551Plan(t, backuptypes.BackupSelection{Resources: []string{bk551VolumeARN}}),
			bk551VolumeARN, nil, false, true, true},
		{"ARN selection of another resource, tags unknown", bk551Plan(t, backuptypes.BackupSelection{Resources: []string{bk551TableARN}}),
			bk551VolumeARN, nil, false, false, true},
		{"ListOfTags decides, tags unknown", bk551Plan(t, byTag), bk551VolumeARN, nil, false, false, false},
		{"ListOfTags decides, tags known and absent", bk551Plan(t, byTag), bk551VolumeARN, map[string]string{}, true, false, true},
		{"ListOfTags decides, tags known and matching", bk551Plan(t, byTag), bk551VolumeARN, map[string]string{"backup": "true"}, true, true, true},
		{"Conditions decide, tags unknown", bk551Plan(t, volumesIfTagged), bk551VolumeARN, nil, false, false, false},
		// The ARN is outside Resources, so no tag value could bring it in.
		{"Conditions cannot decide, tags unknown", bk551Plan(t, tablesIfTagged), bk551VolumeARN, nil, false, false, true},
		{"excluded by NotResources, tags unknown", bk551Plan(t, backuptypes.BackupSelection{
			ListOfTags: []backuptypes.Condition{bk551ListTag("backup", "true")}, NotResources: []string{bk551VolumeARN},
		}), bk551VolumeARN, nil, false, false, true},
		{"selections incomplete, nothing read covers", bk551Fetch(t, 1,
			backuptypes.BackupSelection{Resources: []string{bk551TableARN}},
			backuptypes.BackupSelection{Resources: []string{bk551VolumeARN}},
		), bk551VolumeARN, nil, true, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			covered, known := awsclient.BackupPlanCovers(tc.plan, tc.arn, tc.tags, tc.tagsKnown)
			if covered != tc.wantCovered || known != tc.wantKnown {
				t.Errorf("BackupPlanCovers = (%v, %v), want (%v, %v)", covered, known, tc.wantCovered, tc.wantKnown)
			}
		})
	}
}

// A selection that was read and covers the resource answers regardless of the
// selection that could not be read.
func TestBackupPlanCovers_IncompletePlanStillCoversWhatItRead(t *testing.T) {
	plan := bk551Fetch(t, 1,
		backuptypes.BackupSelection{Resources: []string{bk551VolumeARN}},
		backuptypes.BackupSelection{Resources: []string{bk551TableARN}},
	)
	if covered, _ := awsclient.BackupPlanCovers(plan, bk551VolumeARN, nil, true); !covered {
		t.Error("BackupPlanCovers covered = false, want true: the selection that was read names vol-1")
	}
}

// ── the fetcher keeps selections whole ──────────────────────────────────

func TestFetchBackupPlansPage_KeepsSelectionsStructurally(t *testing.T) {
	a := backuptypes.BackupSelection{
		SelectionName: aws.String("acme-volumes-tagged"),
		IamRoleArn:    aws.String(bk551Role),
		Resources:     []string{"arn:aws:ec2:*:*:volume/*"},
		Conditions: &backuptypes.Conditions{
			StringEquals:    []backuptypes.ConditionParameter{bk551Cond("backup", "true")},
			StringNotEquals: []backuptypes.ConditionParameter{bk551Cond("stage", "test")},
		},
	}
	b := backuptypes.BackupSelection{
		SelectionName: aws.String("acme-everything-else"),
		IamRoleArn:    aws.String(bk551Role),
		NotResources:  []string{bk551VolumeARN},
		ListOfTags:    []backuptypes.Condition{bk551ListTag("cost=center", "ops")},
	}
	plan := bk551Plan(t, a, b)

	sels, complete := awsclient.BackupPlanSelections(plan)
	if !complete {
		t.Error("BackupPlanSelections complete = false, want true: every selection was read")
	}
	if len(sels) != 2 {
		t.Fatalf("BackupPlanSelections returned %d selections, want 2", len(sels))
	}
	if got := aws.ToString(sels[0].SelectionName); got != "acme-volumes-tagged" {
		t.Errorf("selection 0 name = %q, want acme-volumes-tagged", got)
	}
	if !slices.Equal(sels[0].Resources, a.Resources) || len(sels[0].NotResources) != 0 {
		t.Errorf("selection 0 Resources/NotResources = %v/%v, want %v/[]", sels[0].Resources, sels[0].NotResources, a.Resources)
	}
	if c := sels[0].Conditions; c == nil || len(c.StringEquals) != 1 || len(c.StringNotEquals) != 1 ||
		aws.ToString(c.StringNotEquals[0].ConditionValue) != "test" {
		t.Errorf("selection 0 Conditions = %+v, want StringEquals backup=true and StringNotEquals stage=test", c)
	}
	if !slices.Equal(sels[1].NotResources, b.NotResources) || len(sels[1].Resources) != 0 {
		t.Errorf("selection 1 Resources/NotResources = %v/%v, want []/%v", sels[1].Resources, sels[1].NotResources, b.NotResources)
	}
	if len(sels[1].ListOfTags) != 1 || aws.ToString(sels[1].ListOfTags[0].ConditionKey) != "cost=center" {
		t.Errorf("selection 1 ListOfTags = %+v, want one entry keyed cost=center", sels[1].ListOfTags)
	}

	// A plan-wide flattening loses which selection an ARN, an exclusion or a
	// tag belongs to, so the row carries none.
	for _, key := range []string{"resources", "not_resources", "selection_tags", "selection_resources", "selection_not_resources"} {
		if v, ok := plan.Fields[key]; ok {
			t.Errorf("plan Fields[%q] = %q, want the key absent", key, v)
		}
	}
}

func TestFetchBackupPlansPage_SelectionReadFailureIsIncomplete(t *testing.T) {
	plan := bk551Fetch(t, 0,
		backuptypes.BackupSelection{Resources: []string{bk551TableARN}},
		backuptypes.BackupSelection{Resources: []string{bk551VolumeARN}},
	)
	if _, complete := awsclient.BackupPlanSelections(plan); complete {
		t.Error("BackupPlanSelections complete = true, want false: a selection could not be read")
	}
}

// ── the coverage join ────────────────────────────────────────────────────

func bk551Volume(tags map[string]string) resource.Resource {
	vol := ec2types.Volume{
		VolumeId:         aws.String(bk551VolumeID),
		State:            ec2types.VolumeStateInUse,
		AvailabilityZone: aws.String(bk551Region + "a"),
		Size:             aws.Int32(100),
		VolumeType:       ec2types.VolumeTypeGp3,
		Encrypted:        aws.Bool(true),
	}
	for k, v := range tags {
		vol.Tags = append(vol.Tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return resource.Resource{
		ID:   bk551VolumeID,
		Name: "acme-ledger-data",
		Fields: map[string]string{
			"volume_id": bk551VolumeID,
			"name":      "acme-ledger-data",
			"state":     "in-use",
			"az":        bk551Region + "a",
		},
		RawStruct: vol,
	}
}

func bk551Clients() *awsclient.ServiceClients {
	clients := &awsclient.ServiceClients{Region: bk551Region}
	store := session.NewIdentityStore()
	store.Set(bk551Account, nil)
	clients.SetIdentityStore(store)
	return clients
}

func bk551EnrichEBS(t *testing.T, vol resource.Resource, cache resource.ResourceCache) awsclient.IssueEnricherResult {
	t.Helper()
	res, err := awsclient.EnrichEBSVolumeStatus(context.Background(), bk551Clients(), []resource.Resource{vol}, cache)
	if err != nil && res.Findings == nil {
		t.Fatalf("EnrichEBSVolumeStatus: %v", err)
	}
	return res
}

func TestBackupCoverageJoin_ExclusionInOneSelectionOnly(t *testing.T) {
	plan := bk551Plan(t,
		backuptypes.BackupSelection{Resources: []string{"arn:aws:ec2:*:*:volume/*"}},
		backuptypes.BackupSelection{NotResources: []string{bk551VolumeARN}},
	)
	res := bk551EnrichEBS(t, bk551Volume(nil), bk551Cache(plan))
	w4AssertNoCode(t, res.Findings[bk551VolumeID], awsclient.CodeEBSNotInBackupPlan)

	excluded := bk551Plan(t, backuptypes.BackupSelection{
		Resources: []string{"arn:aws:ec2:*:*:volume/*"}, NotResources: []string{bk551VolumeARN},
	})
	res = bk551EnrichEBS(t, bk551Volume(nil), bk551Cache(excluded))
	w4AssertFinding(t, res.Findings[bk551VolumeID], awsclient.CodeEBSNotInBackupPlan,
		"not covered by a backup plan", domain.SevWarn, "wave2")
}

// Resources AND Conditions: a volume inside Resources but without the tag the
// Conditions demand is not selected.
func TestBackupCoverageJoin_ConditionsNarrowResources(t *testing.T) {
	plan := bk551Plan(t, backuptypes.BackupSelection{
		Resources:  []string{"arn:aws:ec2:*:*:volume/*"},
		Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
	})

	untagged := bk551EnrichEBS(t, bk551Volume(map[string]string{"Name": "acme-ledger-data"}), bk551Cache(plan))
	w4AssertFinding(t, untagged.Findings[bk551VolumeID], awsclient.CodeEBSNotInBackupPlan,
		"not covered by a backup plan", domain.SevWarn, "wave2")
	w4AssertRows(t, untagged.AttentionDetails[bk551VolumeID], awsclient.CodeEBSNotInBackupPlan,
		[]domain.DetailRow{{Label: "Backup plans", Value: "0"}})

	tagged := bk551EnrichEBS(t, bk551Volume(map[string]string{"Name": "acme-ledger-data", "backup": "true"}), bk551Cache(plan))
	w4AssertNoCode(t, tagged.Findings[bk551VolumeID], awsclient.CodeEBSNotInBackupPlan)
}

// A plan whose selections were only partly read may select the resource on
// the selection nobody read, so nothing is reported.
func TestBackupCoverageJoin_IncompletePlanReportsNothing(t *testing.T) {
	plan := bk551Fetch(t, 1,
		backuptypes.BackupSelection{Resources: []string{"arn:aws:dynamodb:*:*:table/acme-other-*"}},
		backuptypes.BackupSelection{Resources: []string{bk551TableARN}},
	)
	table := resource.Resource{
		ID: "acme-ledger", Name: "acme-ledger",
		Fields:    map[string]string{"table_name": "acme-ledger", "arn": bk551TableARN},
		RawStruct: ddbtypes.TableDescription{TableName: aws.String("acme-ledger"), TableArn: aws.String(bk551TableARN)},
	}
	res, err := awsclient.EnrichDynamoDBPITR(context.Background(), &awsclient.ServiceClients{}, []resource.Resource{table}, bk551Cache(plan))
	if err != nil && res.Findings == nil {
		t.Fatalf("EnrichDynamoDBPITR: %v", err)
	}
	w4AssertNoCode(t, res.Findings["acme-ledger"], awsclient.CodeDDBNotInBackupPlan)
}

// ── every backup pivot ───────────────────────────────────────────────────

type bk551Pivot struct {
	short    string
	res      resource.Resource
	arn      string // the ARN a selection names to cover res
	wildcard string // a selection pattern taking every resource of that type
	cache    resource.ResourceCache
	tagsSeen bool // the pivot row carries the resource's tags
}

func bk551Pivots() []bk551Pivot {
	return []bk551Pivot{
		{
			short: "ddb", arn: bk551TableARN, wildcard: "arn:aws:dynamodb:*:*:table/*",
			res: resource.Resource{
				ID: "acme-ledger", Name: "acme-ledger",
				Fields:    map[string]string{"table_name": "acme-ledger", "arn": bk551TableARN},
				RawStruct: ddbtypes.TableDescription{TableName: aws.String("acme-ledger"), TableArn: aws.String(bk551TableARN)},
			},
		},
		{
			short: "dbi-snap", arn: bk551DBARN, wildcard: "arn:aws:rds:*:*:db:*",
			res: resource.Resource{
				ID: "rds:acme-ledger-db-2026-09-18-03-00", Name: "rds:acme-ledger-db-2026-09-18-03-00",
				Fields: map[string]string{"db_instance_identifier": "acme-ledger-db"},
				RawStruct: rdstypes.DBSnapshot{
					DBSnapshotIdentifier: aws.String("rds:acme-ledger-db-2026-09-18-03-00"),
					DBInstanceIdentifier: aws.String("acme-ledger-db"),
					SnapshotType:         aws.String("automated"),
				},
			},
			cache: resource.ResourceCache{"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{{
				ID: "acme-ledger-db", Name: "acme-ledger-db",
				Fields: map[string]string{"db_identifier": "acme-ledger-db", "arn": bk551DBARN},
				RawStruct: rdstypes.DBInstance{
					DBInstanceIdentifier: aws.String("acme-ledger-db"),
					DBInstanceArn:        aws.String(bk551DBARN),
				},
			}}}},
		},
		{
			short: "dbc-snap", arn: bk551ClusterARN, wildcard: "arn:aws:rds:*:*:cluster:*",
			res: resource.Resource{
				ID: "rds:acme-ledger-cluster-2026-09-18-04-00", Name: "rds:acme-ledger-cluster-2026-09-18-04-00",
				Fields: map[string]string{"db_cluster_identifier": "acme-ledger-cluster"},
				RawStruct: rdstypes.DBClusterSnapshot{
					DBClusterSnapshotIdentifier: aws.String("rds:acme-ledger-cluster-2026-09-18-04-00"),
					DBClusterIdentifier:         aws.String("acme-ledger-cluster"),
					SnapshotType:                aws.String("automated"),
				},
			},
			cache: resource.ResourceCache{"dbc": resource.ResourceCacheEntry{Resources: []resource.Resource{{
				ID: "acme-ledger-cluster", Name: "acme-ledger-cluster",
				Fields: map[string]string{"cluster_id": "acme-ledger-cluster", "arn": bk551ClusterARN},
				RawStruct: rdstypes.DBCluster{
					DBClusterIdentifier: aws.String("acme-ledger-cluster"),
					DBClusterArn:        aws.String(bk551ClusterARN),
				},
			}}}},
		},
		{
			short: "efs", arn: bk551EFSARN, wildcard: "arn:aws:elasticfilesystem:*:*:file-system/*",
			res: resource.Resource{
				ID: "fs-0a1b2c3d4e5f65510", Name: "acme-ledger-share",
				Fields: map[string]string{"file_system_id": "fs-0a1b2c3d4e5f65510"},
				RawStruct: efstypes.FileSystemDescription{
					FileSystemId:  aws.String("fs-0a1b2c3d4e5f65510"),
					FileSystemArn: aws.String(bk551EFSARN),
					Name:          aws.String("acme-ledger-share"),
				},
			},
			tagsSeen: true,
		},
		{
			short: "ec2", arn: bk551InstanceARN, wildcard: "arn:aws:ec2:*:*:instance/*",
			res: resource.Resource{
				ID: bk551InstanceID, Name: "acme-ledger-api",
				Fields: map[string]string{"instance_id": bk551InstanceID, "arn": bk551InstanceARN},
				RawStruct: ec2types.Instance{
					InstanceId: aws.String(bk551InstanceID),
					Tags:       []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("acme-ledger-api")}},
				},
			},
			tagsSeen: true,
		},
		{
			short: "s3", arn: bk551BucketARN, wildcard: "arn:aws:s3:::*",
			res: resource.Resource{
				ID: bk551Bucket, Name: bk551Bucket,
				Fields:    map[string]string{"name": bk551Bucket},
				RawStruct: s3types.Bucket{Name: aws.String(bk551Bucket), BucketRegion: aws.String(bk551Region)},
			},
		},
		{
			short: "ebs", arn: bk551VolumeARN, wildcard: "arn:aws:ec2:*:*:volume/*",
			res:      bk551Volume(map[string]string{"Name": "acme-ledger-data"}),
			tagsSeen: true,
		},
		{
			// AWS Backup tags the snapshots it creates with the ARN of the
			// resource it backed up; that source volume is what a selection names.
			short: "ebs-snap", arn: bk551VolumeARN, wildcard: "arn:aws:ec2:*:*:volume/*",
			res: resource.Resource{
				ID: "snap-0a1b2c3d4e5f65510", Name: "snap-0a1b2c3d4e5f65510",
				Fields: map[string]string{"snapshot_id": "snap-0a1b2c3d4e5f65510", "volume_id": bk551VolumeID},
				RawStruct: ec2types.Snapshot{
					SnapshotId:  aws.String("snap-0a1b2c3d4e5f65510"),
					VolumeId:    aws.String(bk551VolumeID),
					Description: aws.String("Created by AWS Backup for job 0551"),
					State:       ec2types.SnapshotStateCompleted,
					Tags: []ec2types.Tag{
						{Key: aws.String("aws:backup:source-resource"), Value: aws.String(bk551VolumeARN)},
					},
				},
			},
		},
	}
}

func bk551RunPivot(t *testing.T, p bk551Pivot, plan resource.Resource) resource.RelatedCheckResult {
	t.Helper()
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated(p.short) {
		if def.TargetType == "backup" {
			checker = def.Checker
		}
	}
	if checker == nil {
		t.Fatalf("no %s→backup checker registered", p.short)
	}
	cache := bk551Cache(plan)
	for k, v := range p.cache {
		cache[k] = v
	}
	return checker(context.Background(), bk551Clients(), p.res, cache)
}

func bk551AssertPlans(t *testing.T, got resource.RelatedCheckResult, want []string) {
	t.Helper()
	if got.State() != domain.RelatedResolved {
		t.Fatalf("State = %v, want resolved with plans %v", got.State(), want)
	}
	if !slices.Equal(got.ResourceIDs(), want) || got.Count() != len(want) || got.Truncated() {
		t.Errorf("plans = %v (count %d, truncated %v), want %v exactly", got.ResourceIDs(), got.Count(), got.Truncated(), want)
	}
}

// A selection pattern takes the resource in, and a different selection of the
// same plan excludes it; the plan is still listed.
func TestBackupPivots_ExclusionInOneSelectionOnly(t *testing.T) {
	for _, p := range bk551Pivots() {
		t.Run(p.short, func(t *testing.T) {
			plan := bk551Plan(t,
				backuptypes.BackupSelection{Resources: []string{p.wildcard}},
				backuptypes.BackupSelection{NotResources: []string{p.arn}},
			)
			bk551AssertPlans(t, bk551RunPivot(t, p, plan), []string{bk551PlanID})
		})
	}
}

func TestBackupPivots_ExcludedByItsOnlySelection(t *testing.T) {
	for _, p := range bk551Pivots() {
		t.Run(p.short, func(t *testing.T) {
			plan := bk551Plan(t, backuptypes.BackupSelection{
				Resources: []string{p.wildcard}, NotResources: []string{p.arn},
			})
			bk551AssertPlans(t, bk551RunPivot(t, p, plan), nil)
		})
	}
}

// A selection whose Resources take the resource in but whose Conditions
// demand a tag the resource does not carry never lists the plan. Where the
// pivot row carries the tags the answer is a proven zero; elsewhere it may
// only be unknown.
func TestBackupPivots_ConditionsNarrowResources(t *testing.T) {
	for _, p := range bk551Pivots() {
		t.Run(p.short, func(t *testing.T) {
			plan := bk551Plan(t, backuptypes.BackupSelection{
				Resources:  []string{p.wildcard},
				Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
			})
			got := bk551RunPivot(t, p, plan)
			if slices.Contains(got.ResourceIDs(), bk551PlanID) {
				t.Fatalf("plans = %v, want the plan absent: the resource lacks backup=true", got.ResourceIDs())
			}
			if p.tagsSeen {
				bk551AssertPlans(t, got, nil)
			}
		})
	}
}

// The tag-gated selection lists the plan once the tag is there, on the pivots
// whose row carries its tags.
func TestBackupPivots_ConditionsSatisfiedByRowTags(t *testing.T) {
	plan := bk551Plan(t,
		backuptypes.BackupSelection{
			Resources:  []string{"arn:aws:ec2:*:*:instance/*"},
			Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
		},
		backuptypes.BackupSelection{
			Resources:  []string{"arn:aws:ec2:*:*:volume/*"},
			Conditions: &backuptypes.Conditions{StringEquals: []backuptypes.ConditionParameter{bk551Cond("backup", "true")}},
		},
	)
	for _, p := range bk551Pivots() {
		switch p.short {
		case "ec2":
			inst := p.res.RawStruct.(ec2types.Instance)
			inst.Tags = append(inst.Tags, ec2types.Tag{Key: aws.String("backup"), Value: aws.String("true")})
			p.res.RawStruct = inst
		case "ebs":
			p.res = bk551Volume(map[string]string{"Name": "acme-ledger-data", "backup": "true"})
		default:
			continue
		}
		t.Run(p.short, func(t *testing.T) {
			bk551AssertPlans(t, bk551RunPivot(t, p, plan), []string{bk551PlanID})
		})
	}
}

// The plan's second selection could not be read, and the first does not take
// the resource in: the pivot cannot say zero.
func TestBackupPivots_IncompletePlanIsUnknown(t *testing.T) {
	for _, p := range bk551Pivots() {
		t.Run(p.short, func(t *testing.T) {
			plan := bk551Fetch(t, 1,
				backuptypes.BackupSelection{Resources: []string{"arn:aws:fsx:*"}},
				backuptypes.BackupSelection{Resources: []string{p.arn}},
			)
			got := bk551RunPivot(t, p, plan)
			if got.State() != domain.RelatedUnknown {
				t.Errorf("State = %v (plans %v), want unknown: a selection of the plan was never read", got.State(), got.ResourceIDs())
			}
		})
	}
}

// ── the flattened matchers ───────────────────────────────────────────────

func TestBackupCoverage_FlattenedMatchersRemoved(t *testing.T) {
	root := filepath.Join("..", "..")
	pattern := regexp.MustCompile(`BackupPlanCoversARN|backupSelectionTagsMatch|selection_resources|selection_not_resources`)
	var hits []string
	for _, dir := range []string{"core", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for i, line := range strings.Split(string(src), "\n") {
				if pattern.MatchString(line) {
					hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	if len(hits) > 0 {
		t.Errorf("flattened backup matchers still referenced:\n%s", strings.Join(hits, "\n"))
	}
}
