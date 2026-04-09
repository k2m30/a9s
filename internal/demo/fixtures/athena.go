package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
)

// AthenaFixtures holds typed fixture data for Athena.
type AthenaFixtures struct {
	WorkGroups []athenatypes.WorkGroupSummary
}

func mustParseAthenaTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewAthenaFixtures constructs AthenaFixtures from the canonical demo data.
func NewAthenaFixtures() *AthenaFixtures {
	return &AthenaFixtures{
		WorkGroups: []athenatypes.WorkGroupSummary{
			{
				Name:         aws.String("primary"),
				State:        athenatypes.WorkGroupStateEnabled,
				Description:  aws.String("Default Athena workgroup"),
				CreationTime: aws.Time(mustParseAthenaTime("2024-06-01T08:00:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("AUTO"),
				},
			},
			{
				Name:         aws.String("acme-analytics"),
				State:        athenatypes.WorkGroupStateEnabled,
				Description:  aws.String("Analytics team workgroup with query cost controls"),
				CreationTime: aws.Time(mustParseAthenaTime("2025-02-15T10:30:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("Athena engine version 3"),
				},
			},
			{
				Name:         aws.String("acme-data-science"),
				State:        athenatypes.WorkGroupStateDisabled,
				Description:  aws.String("Data science workgroup (suspended for cost review)"),
				CreationTime: aws.Time(mustParseAthenaTime("2025-08-01T14:00:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("AUTO"),
				},
			},
		},
	}
}
