package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["backup"] = backupFixtures
	demoData["ses"] = sesFixtures
}

// backupFixtures returns demo AWS Backup Plan fixtures.
func backupFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "1a2b3c4d-5e6f-7890-abcd-111111111111",
			Name:   "acme-daily-backup",
			Status: "",
			Fields: map[string]string{
				"plan_name":      "acme-daily-backup",
				"plan_id":        "1a2b3c4d-5e6f-7890-abcd-111111111111",
				"creation_date":  "2025-01-15T09:00:00+00:00",
				"last_execution": "2026-03-21T02:00:00+00:00",
			},
			RawStruct: backuptypes.BackupPlansListMember{
				BackupPlanName:    aws.String("acme-daily-backup"),
				BackupPlanId:      aws.String("1a2b3c4d-5e6f-7890-abcd-111111111111"),
				BackupPlanArn:     aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:1a2b3c4d-5e6f-7890-abcd-111111111111"),
				CreationDate:      aws.Time(mustParseTime("2025-01-15T09:00:00+00:00")),
				LastExecutionDate: aws.Time(mustParseTime("2026-03-21T02:00:00+00:00")),
			},
		},
		{
			ID:     "1a2b3c4d-5e6f-7890-abcd-222222222222",
			Name:   "acme-weekly-full-backup",
			Status: "",
			Fields: map[string]string{
				"plan_name":      "acme-weekly-full-backup",
				"plan_id":        "1a2b3c4d-5e6f-7890-abcd-222222222222",
				"creation_date":  "2025-01-15T09:15:00+00:00",
				"last_execution": "2026-03-16T03:00:00+00:00",
			},
			RawStruct: backuptypes.BackupPlansListMember{
				BackupPlanName:    aws.String("acme-weekly-full-backup"),
				BackupPlanId:      aws.String("1a2b3c4d-5e6f-7890-abcd-222222222222"),
				BackupPlanArn:     aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:1a2b3c4d-5e6f-7890-abcd-222222222222"),
				CreationDate:      aws.Time(mustParseTime("2025-01-15T09:15:00+00:00")),
				LastExecutionDate: aws.Time(mustParseTime("2026-03-16T03:00:00+00:00")),
			},
		},
		{
			ID:     "1a2b3c4d-5e6f-7890-abcd-333333333333",
			Name:   "acme-compliance-30day",
			Status: "",
			Fields: map[string]string{
				"plan_name":      "acme-compliance-30day",
				"plan_id":        "1a2b3c4d-5e6f-7890-abcd-333333333333",
				"creation_date":  "2025-06-01T10:00:00+00:00",
				"last_execution": "2026-03-20T04:30:00+00:00",
			},
			RawStruct: backuptypes.BackupPlansListMember{
				BackupPlanName:    aws.String("acme-compliance-30day"),
				BackupPlanId:      aws.String("1a2b3c4d-5e6f-7890-abcd-333333333333"),
				BackupPlanArn:     aws.String("arn:aws:backup:us-east-1:123456789012:backup-plan:1a2b3c4d-5e6f-7890-abcd-333333333333"),
				CreationDate:      aws.Time(mustParseTime("2025-06-01T10:00:00+00:00")),
				LastExecutionDate: aws.Time(mustParseTime("2026-03-20T04:30:00+00:00")),
			},
		},
	}
}

// sesFixtures returns demo SES Identity fixtures.
func sesFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acmecorp.com",
			Name:   "acmecorp.com",
			Status: "",
			Fields: map[string]string{
				"identity_name":       "acmecorp.com",
				"identity_type":       "DOMAIN",
				"sending_enabled":     "true",
				"verification_status": "SUCCESS",
			},
			RawStruct: sesv2types.IdentityInfo{
				IdentityName:       aws.String("acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeDomain,
				SendingEnabled:     true,
				VerificationStatus: sesv2types.VerificationStatusSuccess,
			},
		},
		{
			ID:     "noreply@acmecorp.com",
			Name:   "noreply@acmecorp.com",
			Status: "",
			Fields: map[string]string{
				"identity_name":       "noreply@acmecorp.com",
				"identity_type":       "EMAIL_ADDRESS",
				"sending_enabled":     "true",
				"verification_status": "SUCCESS",
			},
			RawStruct: sesv2types.IdentityInfo{
				IdentityName:       aws.String("noreply@acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeEmailAddress,
				SendingEnabled:     true,
				VerificationStatus: sesv2types.VerificationStatusSuccess,
			},
		},
		{
			ID:     "alerts@acmecorp.com",
			Name:   "alerts@acmecorp.com",
			Status: "",
			Fields: map[string]string{
				"identity_name":       "alerts@acmecorp.com",
				"identity_type":       "EMAIL_ADDRESS",
				"sending_enabled":     "false",
				"verification_status": "PENDING",
			},
			RawStruct: sesv2types.IdentityInfo{
				IdentityName:       aws.String("alerts@acmecorp.com"),
				IdentityType:       sesv2types.IdentityTypeEmailAddress,
				SendingEnabled:     false,
				VerificationStatus: sesv2types.VerificationStatusPending,
			},
		},
	}
}
