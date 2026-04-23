package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
)

// GlueFixtures holds typed fixture data for Glue.
type GlueFixtures struct {
	Jobs    []gluetypes.Job
	JobRuns map[string][]gluetypes.JobRun // key: job name
}

func mustParseGlueTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewGlueFixtures constructs GlueFixtures from the canonical demo data.
func NewGlueFixtures() *GlueFixtures {
	dpuSucceeded := 45000.0
	dpuFailed := 12000.0
	dpuTimeout := 72000.0
	dpuError := 5000.0

	return &GlueFixtures{
		Jobs: []gluetypes.Job{
			{
				Name:            aws.String("acme-etl-orders"),
				Role:            aws.String("acme-glue-role"),
				GlueVersion:     aws.String("4.0"),
				WorkerType:      gluetypes.WorkerTypeG1x,
				NumberOfWorkers: aws.Int32(10),
				MaxRetries:      1,
				CreatedOn:       aws.Time(mustParseGlueTime("2025-05-10T09:00:00+00:00")),
				LastModifiedOn:  aws.Time(mustParseGlueTime("2026-03-15T14:30:00+00:00")),
				Command: &gluetypes.JobCommand{
					Name: aws.String("glueetl"),
				},
			},
			{
				Name:            aws.String("acme-etl-clickstream"),
				Role:            aws.String("acme-glue-role"),
				GlueVersion:     aws.String("4.0"),
				WorkerType:      gluetypes.WorkerTypeG2x,
				NumberOfWorkers: aws.Int32(20),
				MaxRetries:      2,
				CreatedOn:       aws.Time(mustParseGlueTime("2025-07-20T11:00:00+00:00")),
				LastModifiedOn:  aws.Time(mustParseGlueTime("2026-03-10T09:15:00+00:00")),
				Command: &gluetypes.JobCommand{
					Name: aws.String("gluestreaming"),
				},
			},
			{
				Name:            aws.String("acme-data-catalog-crawler"),
				Role:            aws.String("acme-glue-crawler-role"),
				GlueVersion:     aws.String("3.0"),
				WorkerType:      gluetypes.WorkerTypeStandard,
				NumberOfWorkers: aws.Int32(5),
				MaxRetries:      0,
				CreatedOn:       aws.Time(mustParseGlueTime("2025-03-15T08:30:00+00:00")),
				LastModifiedOn:  aws.Time(mustParseGlueTime("2026-01-20T16:00:00+00:00")),
				Command: &gluetypes.JobCommand{
					Name: aws.String("pythonshell"),
				},
			},
			// S3 healthy-bucket ETL job (checkS3Glue pivot).
			// checkS3Glue uses assertStruct[gluetypes.Job] and reads Command.ScriptLocation.
			// bucketFromS3URI("s3://a9s-demo-healthy/scripts/etl.py") == "a9s-demo-healthy".
			{
				Name:            aws.String("a9s-demo-s3-etl"),
				Role:            aws.String("acme-glue-role"),
				GlueVersion:     aws.String("4.0"),
				WorkerType:      gluetypes.WorkerTypeG1x,
				NumberOfWorkers: aws.Int32(5),
				MaxRetries:      1,
				CreatedOn:       aws.Time(mustParseGlueTime("2025-01-10T10:00:00+00:00")),
				LastModifiedOn:  aws.Time(mustParseGlueTime("2026-01-15T09:00:00+00:00")),
				Command: &gluetypes.JobCommand{
					Name:           aws.String("glueetl"),
					ScriptLocation: aws.String("s3://" + HealthyBucketName + "/scripts/etl.py"),
					PythonVersion:  aws.String("3"),
				},
				Description: aws.String("ETL job reading from and writing to a9s-demo-healthy S3 bucket"),
			},
			// Issue: latest JobRun=ERROR → Broken (job script threw an unhandled exception)
			{
				Name:            aws.String("glue-error-run"),
				Role:            aws.String("acme-glue-role"),
				GlueVersion:     aws.String("4.0"),
				WorkerType:      gluetypes.WorkerTypeG1x,
				NumberOfWorkers: aws.Int32(5),
				MaxRetries:      0,
				CreatedOn:       aws.Time(mustParseGlueTime("2025-09-01T09:00:00+00:00")),
				LastModifiedOn:  aws.Time(mustParseGlueTime("2026-04-01T10:00:00+00:00")),
				Command: &gluetypes.JobCommand{
					Name: aws.String("glueetl"),
				},
			},
		},
		JobRuns: map[string][]gluetypes.JobRun{
			"acme-etl-orders": {
				{
					Id:            aws.String("jr_aaa11111-1111-1111-1111-111111111111"),
					JobName:       aws.String("acme-etl-orders"),
					JobRunState:   gluetypes.JobRunStateSucceeded,
					StartedOn:     aws.Time(mustParseGlueTime("2026-03-20T08:00:00+00:00")),
					CompletedOn:   aws.Time(mustParseGlueTime("2026-03-20T08:47:23+00:00")),
					ExecutionTime: 2843,
					DPUSeconds:    &dpuSucceeded,
				},
				{
					Id:            aws.String("jr_bbb22222-2222-2222-2222-222222222222"),
					JobName:       aws.String("acme-etl-orders"),
					JobRunState:   gluetypes.JobRunStateFailed,
					StartedOn:     aws.Time(mustParseGlueTime("2026-03-19T14:30:00+00:00")),
					CompletedOn:   aws.Time(mustParseGlueTime("2026-03-19T14:35:12+00:00")),
					ExecutionTime: 312,
					ErrorMessage:  aws.String("An error occurred while calling o42.pyWriteDynamicFrame: Connection refused"),
					DPUSeconds:    &dpuFailed,
				},
				{
					Id:          aws.String("jr_ccc33333-3333-3333-3333-333333333333"),
					JobName:     aws.String("acme-etl-orders"),
					JobRunState: gluetypes.JobRunStateRunning,
					StartedOn:   aws.Time(mustParseGlueTime("2026-03-21T02:00:00+00:00")),
				},
				{
					Id:            aws.String("jr_ddd44444-4444-4444-4444-444444444444"),
					JobName:       aws.String("acme-etl-orders"),
					JobRunState:   gluetypes.JobRunStateTimeout,
					StartedOn:     aws.Time(mustParseGlueTime("2026-03-18T22:00:00+00:00")),
					CompletedOn:   aws.Time(mustParseGlueTime("2026-03-19T00:00:00+00:00")),
					ExecutionTime: 7200,
					ErrorMessage:  aws.String("Job execution exceeded timeout of 7200 seconds"),
					DPUSeconds:    &dpuTimeout,
				},
			},
			"glue-error-run": {
				{
					Id:            aws.String("jr_eee55555-5555-5555-5555-555555555555"),
					JobName:       aws.String("glue-error-run"),
					JobRunState:   gluetypes.JobRunStateError,
					StartedOn:     aws.Time(mustParseGlueTime("2026-04-18T06:00:00+00:00")),
					CompletedOn:   aws.Time(mustParseGlueTime("2026-04-18T06:03:00+00:00")),
					ExecutionTime: 180,
					ErrorMessage:  aws.String("An error occurred: java.lang.NullPointerException at line 47 of transform script"),
					DPUSeconds:    &dpuError,
				},
			},
		},
	}
}
