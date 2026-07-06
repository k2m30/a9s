package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
)

// AthenaFixtures holds typed fixture data for Athena.
type AthenaFixtures struct {
	WorkGroups       []athenatypes.WorkGroupSummary
	WorkGroupDetails map[string]*athena.GetWorkGroupOutput
}

func mustParseAthenaTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewAthenaFixtures constructs AthenaFixtures from the canonical demo data.
var sharedAthenaFixtures = sync.OnceValue(func() *AthenaFixtures {
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
			// S3 healthy-bucket Athena workgroup (checkS3Athena pivot).
			// Full config (ResultConfiguration.OutputLocation) lives in
			// WorkGroupDetails below so the fetcher can populate
			// Fields["result_output_location"] via GetWorkGroup.
			{
				Name:         aws.String("a9s-demo-s3-queries"),
				State:        athenatypes.WorkGroupStateEnabled,
				Description:  aws.String("Athena workgroup writing query results to s3://" + HealthyBucketName + "/athena-results/"),
				CreationTime: aws.Time(mustParseAthenaTime("2025-02-01T08:00:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("Athena engine version 3"),
				},
			},
			// Spark workgroup — required for athena:kms, athena:logs, and
			// athena:role related-panel pivots. checkAthenaKMS/Logs/Role all
			// read Configuration fields only populated on Spark workgroups
			// (ExecutionRole, PublishCloudWatchMetricsEnabled,
			// EncryptionConfiguration.KmsKey) via GetWorkGroup.
			{
				Name:         aws.String("acme-spark-analytics"),
				State:        athenatypes.WorkGroupStateEnabled,
				Description:  aws.String("PySpark workgroup for ad-hoc analytics notebooks"),
				CreationTime: aws.Time(mustParseAthenaTime("2025-05-12T09:00:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("PySpark engine version 3"),
					SelectedEngineVersion:  aws.String("PySpark engine version 3"),
				},
			},
			// acme-etl-orders workgroup — required for the glue:athena
			// related-panel pivot. checkGlueAthena matches wg.ID == jobName;
			// this workgroup is provisioned for analysts querying the ETL
			// job's output tables via Athena (glue.go acme-etl-orders job).
			{
				Name:         aws.String("acme-etl-orders"),
				State:        athenatypes.WorkGroupStateEnabled,
				Description:  aws.String("Athena workgroup for querying acme-etl-orders output tables"),
				CreationTime: aws.Time(mustParseAthenaTime("2025-05-11T09:00:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("Athena engine version 3"),
				},
			},
		},
		WorkGroupDetails: map[string]*athena.GetWorkGroupOutput{
			"a9s-demo-s3-queries": {
				WorkGroup: &athenatypes.WorkGroup{
					Name:  aws.String("a9s-demo-s3-queries"),
					State: athenatypes.WorkGroupStateEnabled,
					Configuration: &athenatypes.WorkGroupConfiguration{
						ResultConfiguration: &athenatypes.ResultConfiguration{
							OutputLocation: aws.String("s3://" + HealthyBucketName + "/athena-results/"),
						},
					},
				},
			},
			"acme-spark-analytics": {
				WorkGroup: &athenatypes.WorkGroup{
					Name:  aws.String("acme-spark-analytics"),
					State: athenatypes.WorkGroupStateEnabled,
					Configuration: &athenatypes.WorkGroupConfiguration{
						EngineVersion: &athenatypes.EngineVersion{
							EffectiveEngineVersion: aws.String("PySpark engine version 3"),
							SelectedEngineVersion:  aws.String("PySpark engine version 3"),
						},
						ExecutionRole:                   aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
						PublishCloudWatchMetricsEnabled: aws.Bool(true),
						ResultConfiguration: &athenatypes.ResultConfiguration{
							OutputLocation: aws.String("s3://" + HealthyBucketName + "/spark-results/"),
							EncryptionConfiguration: &athenatypes.EncryptionConfiguration{
								EncryptionOption: athenatypes.EncryptionOptionSseKms,
								KmsKey:           aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
							},
						},
					},
				},
			},
		},
	}
})

func NewAthenaFixtures() *AthenaFixtures {
	return sharedAthenaFixtures()
}
