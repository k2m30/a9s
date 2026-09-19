package unit

// Backup plan rows for coverage tests. A plan row carries its selections on
// RawStruct in a type only the fetcher can fill, so every such row is fetched
// over a fake.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type plan551Fake struct {
	awsclient.BackupAPI

	id         string
	name       string
	selections []backuptypes.BackupSelection
	failLast   bool
}

func (f *plan551Fake) ListBackupPlans(context.Context, *backup.ListBackupPlansInput, ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error) {
	name := f.name
	if name == "" {
		name = f.id
	}
	return &backup.ListBackupPlansOutput{BackupPlansList: []backuptypes.BackupPlansListMember{{
		BackupPlanId:   aws.String(f.id),
		BackupPlanName: aws.String(name),
	}}}, nil
}

func (f *plan551Fake) DescribeRegionSettings(context.Context, *backup.DescribeRegionSettingsInput, ...func(*backup.Options)) (*backup.DescribeRegionSettingsOutput, error) {
	return &backup.DescribeRegionSettingsOutput{ResourceTypeOptInPreference: bk551AllOptedIn()}, nil
}

func (f *plan551Fake) ListBackupSelections(context.Context, *backup.ListBackupSelectionsInput, ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error) {
	out := &backup.ListBackupSelectionsOutput{}
	for i := range f.selections {
		out.BackupSelectionsList = append(out.BackupSelectionsList, backuptypes.BackupSelectionsListMember{
			SelectionId: aws.String(fmt.Sprint(i)),
		})
	}
	return out, nil
}

func (f *plan551Fake) GetBackupSelection(_ context.Context, in *backup.GetBackupSelectionInput, _ ...func(*backup.Options)) (*backup.GetBackupSelectionOutput, error) {
	var i int
	_, _ = fmt.Sscan(aws.ToString(in.SelectionId), &i)
	if f.failLast && i == len(f.selections)-1 {
		return nil, errors.New("AccessDeniedException: not authorized to perform backup:GetBackupSelection")
	}
	return &backup.GetBackupSelectionOutput{BackupSelection: &f.selections[i]}, nil
}

func plan551Fetch(t *testing.T, f *plan551Fake) resource.Resource {
	t.Helper()
	res, err := awsclient.FetchBackupPlansPage(context.Background(), f, "")
	if err != nil || len(res.Resources) != 1 {
		t.Fatalf("FetchBackupPlansPage = %d plans, %v; want 1", len(res.Resources), err)
	}
	return res.Resources[0]
}

// BackupPlanRow is the plan row the fetcher produces for a plan whose
// selections were all read.
func BackupPlanRow(t *testing.T, id string, sels ...backuptypes.BackupSelection) resource.Resource {
	t.Helper()
	return plan551Fetch(t, &plan551Fake{id: id, selections: sels})
}

// BackupPartialPlanRow is the plan row for a plan whose last selection could
// not be read.
func BackupPartialPlanRow(t *testing.T, id string, sels ...backuptypes.BackupSelection) resource.Resource {
	t.Helper()
	return plan551Fetch(t, &plan551Fake{id: id, selections: sels, failLast: true})
}

// BackupTagSelection selects by one ListOfTags entry.
func BackupTagSelection(key, value string) backuptypes.BackupSelection {
	return backuptypes.BackupSelection{ListOfTags: []backuptypes.Condition{{
		ConditionType:  backuptypes.ConditionTypeStringequals,
		ConditionKey:   aws.String(key),
		ConditionValue: aws.String(value),
	}}}
}
