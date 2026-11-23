package main

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	backupsvc "github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

const (
	t11aActiveSecretARN   = "arn:aws:secretsmanager:eu-west-1:123456789012:secret:prod/db-password-AbCdEf"
	t11aRetiringSecretARN = "arn:aws:secretsmanager:eu-west-1:123456789012:secret:legacy/api-key-GhIjKl"
)

// Secrets Manager leaves a secret scheduled for deletion out of ListSecrets
// unless IncludePlannedDeletion is set; the app lists it (with a Broken
// finding), so the oracle must hold it too, with the date AWS deletes it.
func TestT11a_CaptureSecrets_KeepsASecretScheduledForDeletion(t *testing.T) {
	deleted := time.Date(2026, 12, 1, 9, 30, 0, 0, time.UTC)
	cfg := answeringConfig(func(_, op string, in any) (any, error) {
		switch op {
		case "ListSecrets":
			list := []smtypes.SecretListEntry{{
				Name:            aws.String("prod/db-password"),
				ARN:             aws.String(t11aActiveSecretARN),
				RotationEnabled: aws.Bool(true),
			}}
			if aws.ToBool(in.(*secretsmanager.ListSecretsInput).IncludePlannedDeletion) {
				list = append(list, smtypes.SecretListEntry{
					Name:        aws.String("legacy/api-key"),
					ARN:         aws.String(t11aRetiringSecretARN),
					DeletedDate: aws.Time(deleted),
				})
			}
			return &secretsmanager.ListSecretsOutput{SecretList: list}, nil
		case "DescribeSecret":
			return &secretsmanager.DescribeSecretOutput{
				ARN:                in.(*secretsmanager.DescribeSecretInput).SecretId,
				VersionIdsToStages: map[string][]string{"a1b2c3d4-5678-90ab-cdef-EXAMPLE11111": {"AWSCURRENT"}},
			}, nil
		}
		return nil, fmt.Errorf("unexpected %s", op)
	})

	got, err := captureSecrets(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureSecrets: %v", err)
	}
	deletedDates := map[string]string{}
	for _, s := range got.(secretsData).Secrets {
		deletedDates[s.Arn] = s.DeletedDate
	}
	want := map[string]string{
		t11aActiveSecretARN:   "",
		t11aRetiringSecretARN: "2026-12-01 09:30",
	}
	if fmt.Sprint(deletedDates) != fmt.Sprint(want) {
		t.Errorf("captured secrets (ARN -> deleted_date) = %v, want %v", deletedDates, want)
	}
}

const (
	t11aTwoPagePlanID = "8a7b6c5d-1111-4222-8333-444455556666"
	t11aOnePagePlanID = "9b8c7d6e-7777-4888-8999-aaaabbbbcccc"
	t11aRoleA         = "arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"
	t11aRoleB         = "arn:aws:iam::123456789012:role/backup-tagged-volumes"
	t11aRoleC         = "arn:aws:iam::123456789012:role/backup-databases"
)

// ListBackupSelections returns a NextToken while more selections remain, so
// the roles a plan's selections assume are the roles of every page.
func TestT11a_CaptureBackup_ReadsEverySelectionsPage(t *testing.T) {
	cfg := answeringConfig(func(_, op string, in any) (any, error) {
		switch op {
		case "ListBackupPlans":
			return &backupsvc.ListBackupPlansOutput{BackupPlansList: []backuptypes.BackupPlansListMember{
				{BackupPlanId: aws.String(t11aTwoPagePlanID), BackupPlanName: aws.String("daily-volumes")},
				{BackupPlanId: aws.String(t11aOnePagePlanID), BackupPlanName: aws.String("nightly-databases")},
			}}, nil
		case "ListBackupSelections":
			sel := in.(*backupsvc.ListBackupSelectionsInput)
			switch {
			case aws.ToString(sel.BackupPlanId) == t11aOnePagePlanID && sel.NextToken == nil:
				return &backupsvc.ListBackupSelectionsOutput{BackupSelectionsList: []backuptypes.BackupSelectionsListMember{
					{BackupPlanId: sel.BackupPlanId, IamRoleArn: aws.String(t11aRoleC), SelectionName: aws.String("rds")},
				}}, nil
			case aws.ToString(sel.BackupPlanId) == t11aTwoPagePlanID && sel.NextToken == nil:
				return &backupsvc.ListBackupSelectionsOutput{
					BackupSelectionsList: []backuptypes.BackupSelectionsListMember{
						{BackupPlanId: sel.BackupPlanId, IamRoleArn: aws.String(t11aRoleA), SelectionName: aws.String("all-ebs")},
					},
					NextToken: aws.String("selections-page-2"),
				}, nil
			case aws.ToString(sel.BackupPlanId) == t11aTwoPagePlanID && aws.ToString(sel.NextToken) == "selections-page-2":
				return &backupsvc.ListBackupSelectionsOutput{BackupSelectionsList: []backuptypes.BackupSelectionsListMember{
					{BackupPlanId: sel.BackupPlanId, IamRoleArn: aws.String(t11aRoleB), SelectionName: aws.String("tagged-ebs")},
				}}, nil
			}
			return nil, fmt.Errorf("unexpected ListBackupSelections %s token %q", aws.ToString(sel.BackupPlanId), aws.ToString(sel.NextToken))
		case "ListBackupJobs":
			return &backupsvc.ListBackupJobsOutput{}, nil
		}
		return nil, fmt.Errorf("unexpected %s", op)
	})

	got, err := captureBackup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureBackup: %v", err)
	}
	roles := map[string][]string{}
	outcomes := map[string]string{}
	for _, p := range got.(backupData).Plans {
		roles[p.BackupPlanId] = slices.Sorted(slices.Values(p.IamRoleArns))
		outcomes[p.BackupPlanId] = p.SelectionsOutcome
	}
	wantRoles := map[string][]string{
		t11aTwoPagePlanID: slices.Sorted(slices.Values([]string{t11aRoleA, t11aRoleB})),
		t11aOnePagePlanID: {t11aRoleC},
	}
	if fmt.Sprint(roles) != fmt.Sprint(wantRoles) {
		t.Errorf("selection roles per plan = %v, want %v", roles, wantRoles)
	}
	if want := map[string]string{t11aTwoPagePlanID: "ok", t11aOnePagePlanID: "ok"}; fmt.Sprint(outcomes) != fmt.Sprint(want) {
		t.Errorf("selections_outcome per plan = %v, want %v", outcomes, want)
	}
}

// BatchGetBuilds names in BuildsNotFound a build that ListBuildsForProject
// listed and that was deleted in between: the project has build history the
// collector could not read, which is "not_found" (the collector's word for a
// batch describe that left a listed item out), never "none" (no build at all).
func TestT11a_CaptureCB_ABuildDeletedAfterTheListIsNotFoundNotNone(t *testing.T) {
	const deletedBuild = "web-build:0f1e2d3c-4b5a-6978-8a9b-EXAMPLE00001"
	const keptBuild = "api-build:1a2b3c4d-5e6f-7a8b-9c0d-EXAMPLE00002"
	ended := time.Date(2026, 11, 20, 14, 5, 0, 0, time.UTC)
	cfg := answeringConfig(func(_, op string, in any) (any, error) {
		switch op {
		case "ListProjects":
			return &codebuild.ListProjectsOutput{Projects: []string{"web-build", "api-build", "docs-build"}}, nil
		case "BatchGetProjects":
			var ps []cbtypes.Project
			for _, n := range in.(*codebuild.BatchGetProjectsInput).Names {
				ps = append(ps, cbtypes.Project{
					Name:        aws.String(n),
					Arn:         aws.String("arn:aws:codebuild:eu-west-1:123456789012:project/" + n),
					ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-" + n),
				})
			}
			return &codebuild.BatchGetProjectsOutput{Projects: ps}, nil
		case "ListBuildsForProject":
			switch aws.ToString(in.(*codebuild.ListBuildsForProjectInput).ProjectName) {
			case "web-build":
				return &codebuild.ListBuildsForProjectOutput{Ids: []string{deletedBuild}}, nil
			case "api-build":
				return &codebuild.ListBuildsForProjectOutput{Ids: []string{keptBuild}}, nil
			}
			return &codebuild.ListBuildsForProjectOutput{}, nil
		case "BatchGetBuilds":
			out := &codebuild.BatchGetBuildsOutput{}
			for _, id := range in.(*codebuild.BatchGetBuildsInput).Ids {
				if id == keptBuild {
					out.Builds = append(out.Builds, cbtypes.Build{
						Id:           aws.String(id),
						BuildStatus:  cbtypes.StatusTypeSucceeded,
						CurrentPhase: aws.String("COMPLETED"),
						EndTime:      aws.Time(ended),
					})
				} else {
					out.BuildsNotFound = append(out.BuildsNotFound, id)
				}
			}
			return out, nil
		}
		return nil, fmt.Errorf("unexpected %s", op)
	})

	got, err := captureCB(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureCB: %v", err)
	}
	outcomes := map[string]string{}
	for _, p := range got.(cbData).Projects {
		if p.LatestBuild == nil {
			t.Fatalf("project %s has no latest_build entry", p.Name)
		}
		outcomes[p.Name] = p.LatestBuild.Outcome
	}
	want := map[string]string{"web-build": "not_found", "api-build": "found", "docs-build": "none"}
	if fmt.Sprint(outcomes) != fmt.Sprint(want) {
		t.Errorf("latest_build outcome per project = %v, want %v", outcomes, want)
	}
}
