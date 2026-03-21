package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	demoData["glue"] = glueFixtures
	demoData["athena"] = athenaFixtures
}

// glueFixtures returns demo Glue Job fixtures.
func glueFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-etl-orders",
			Name:   "acme-etl-orders",
			Status: "",
			Fields: map[string]string{
				"job_name":      "acme-etl-orders",
				"role":          "arn:aws:iam::123456789012:role/acme-glue-role",
				"glue_version":  "4.0",
				"worker_type":   "G.1X",
				"num_workers":   "10",
				"max_retries":   "1",
				"created_on":    "2025-05-10 09:00:00",
				"last_modified": "2026-03-15 14:30:00",
				"command":       "glueetl",
			},
			RawStruct: gluetypes.Job{
				Name:            aws.String("acme-etl-orders"),
				Role:            aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
				GlueVersion:    aws.String("4.0"),
				WorkerType:     gluetypes.WorkerTypeG1x,
				NumberOfWorkers: aws.Int32(10),
				MaxRetries:     1,
				CreatedOn:      aws.Time(mustParseTime("2025-05-10T09:00:00+00:00")),
				LastModifiedOn: aws.Time(mustParseTime("2026-03-15T14:30:00+00:00")),
				Command: &gluetypes.JobCommand{
					Name: aws.String("glueetl"),
				},
			},
		},
		{
			ID:     "acme-etl-clickstream",
			Name:   "acme-etl-clickstream",
			Status: "",
			Fields: map[string]string{
				"job_name":      "acme-etl-clickstream",
				"role":          "arn:aws:iam::123456789012:role/acme-glue-role",
				"glue_version":  "4.0",
				"worker_type":   "G.2X",
				"num_workers":   "20",
				"max_retries":   "2",
				"created_on":    "2025-07-20 11:00:00",
				"last_modified": "2026-03-10 09:15:00",
				"command":       "gluestreaming",
			},
			RawStruct: gluetypes.Job{
				Name:            aws.String("acme-etl-clickstream"),
				Role:            aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
				GlueVersion:    aws.String("4.0"),
				WorkerType:     gluetypes.WorkerTypeG2x,
				NumberOfWorkers: aws.Int32(20),
				MaxRetries:     2,
				CreatedOn:      aws.Time(mustParseTime("2025-07-20T11:00:00+00:00")),
				LastModifiedOn: aws.Time(mustParseTime("2026-03-10T09:15:00+00:00")),
				Command: &gluetypes.JobCommand{
					Name: aws.String("gluestreaming"),
				},
			},
		},
		{
			ID:     "acme-data-catalog-crawler",
			Name:   "acme-data-catalog-crawler",
			Status: "",
			Fields: map[string]string{
				"job_name":      "acme-data-catalog-crawler",
				"role":          "arn:aws:iam::123456789012:role/acme-glue-crawler-role",
				"glue_version":  "3.0",
				"worker_type":   "Standard",
				"num_workers":   "5",
				"max_retries":   "0",
				"created_on":    "2025-03-15 08:30:00",
				"last_modified": "2026-01-20 16:00:00",
				"command":       "pythonshell",
			},
			RawStruct: gluetypes.Job{
				Name:            aws.String("acme-data-catalog-crawler"),
				Role:            aws.String("arn:aws:iam::123456789012:role/acme-glue-crawler-role"),
				GlueVersion:    aws.String("3.0"),
				WorkerType:     gluetypes.WorkerTypeStandard,
				NumberOfWorkers: aws.Int32(5),
				MaxRetries:     0,
				CreatedOn:      aws.Time(mustParseTime("2025-03-15T08:30:00+00:00")),
				LastModifiedOn: aws.Time(mustParseTime("2026-01-20T16:00:00+00:00")),
				Command: &gluetypes.JobCommand{
					Name: aws.String("pythonshell"),
				},
			},
		},
	}
}

// athenaFixtures returns demo Athena WorkGroup fixtures.
func athenaFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "primary",
			Name:   "primary",
			Status: "ENABLED",
			Fields: map[string]string{
				"workgroup_name": "primary",
				"state":          "ENABLED",
				"description":    "Default Athena workgroup",
				"creation_time":  "2024-06-01 08:00:00",
				"engine_version": "Athena engine version 3",
			},
			RawStruct: athenatypes.WorkGroupSummary{
				Name:         aws.String("primary"),
				State:        athenatypes.WorkGroupStateEnabled,
				Description:  aws.String("Default Athena workgroup"),
				CreationTime: aws.Time(mustParseTime("2024-06-01T08:00:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("AUTO"),
				},
			},
		},
		{
			ID:     "acme-analytics",
			Name:   "acme-analytics",
			Status: "ENABLED",
			Fields: map[string]string{
				"workgroup_name": "acme-analytics",
				"state":          "ENABLED",
				"description":    "Analytics team workgroup with query cost controls",
				"creation_time":  "2025-02-15 10:30:00",
				"engine_version": "Athena engine version 3",
			},
			RawStruct: athenatypes.WorkGroupSummary{
				Name:         aws.String("acme-analytics"),
				State:        athenatypes.WorkGroupStateEnabled,
				Description:  aws.String("Analytics team workgroup with query cost controls"),
				CreationTime: aws.Time(mustParseTime("2025-02-15T10:30:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("Athena engine version 3"),
				},
			},
		},
		{
			ID:     "acme-data-science",
			Name:   "acme-data-science",
			Status: "DISABLED",
			Fields: map[string]string{
				"workgroup_name": "acme-data-science",
				"state":          "DISABLED",
				"description":    "Data science workgroup (suspended for cost review)",
				"creation_time":  "2025-08-01 14:00:00",
				"engine_version": "Athena engine version 3",
			},
			RawStruct: athenatypes.WorkGroupSummary{
				Name:         aws.String("acme-data-science"),
				State:        athenatypes.WorkGroupStateDisabled,
				Description:  aws.String("Data science workgroup (suspended for cost review)"),
				CreationTime: aws.Time(mustParseTime("2025-08-01T14:00:00+00:00")),
				EngineVersion: &athenatypes.EngineVersion{
					EffectiveEngineVersion: aws.String("Athena engine version 3"),
					SelectedEngineVersion:  aws.String("AUTO"),
				},
			},
		},
	}
}
