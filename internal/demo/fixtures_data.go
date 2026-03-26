package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["glue"] = glueFixtures
	demoData["athena"] = athenaFixtures

	RegisterChildDemo("glue_runs", func(parentCtx map[string]string) []resource.Resource {
		return glueRunFixtures(parentCtx["job_name"])
	})
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

// glueRunFixtures returns demo Glue Job Run fixtures for the given job name.
func glueRunFixtures(jobName string) []resource.Resource {
	dpuSucceeded := 45000.0
	dpuFailed := 12000.0
	dpuTimeout := 72000.0

	return []resource.Resource{
		{
			ID:     "jr_aaa11111-1111-1111-1111-111111111111",
			Name:   "2026-03-20 08:00:00",
			Status: "SUCCEEDED",
			Fields: map[string]string{
				"run_id_short":         "jr_aaa11",
				"job_run_state":        "SUCCEEDED",
				"started_on":           "2026-03-20 08:00:00",
				"execution_time_human": "47m 23s",
				"error_message":        "",
				"dpu_hours":            "12.5",
				"run_id":               "jr_aaa11111-1111-1111-1111-111111111111",
				"job_name":             jobName,
			},
			RawStruct: gluetypes.JobRun{
				Id:            aws.String("jr_aaa11111-1111-1111-1111-111111111111"),
				JobName:       aws.String(jobName),
				JobRunState:   gluetypes.JobRunStateSucceeded,
				StartedOn:     aws.Time(mustParseTime("2026-03-20T08:00:00+00:00")),
				CompletedOn:   aws.Time(mustParseTime("2026-03-20T08:47:23+00:00")),
				ExecutionTime: 2843,
				DPUSeconds:    &dpuSucceeded,
			},
		},
		{
			ID:     "jr_bbb22222-2222-2222-2222-222222222222",
			Name:   "2026-03-19 14:30:00",
			Status: "FAILED",
			Fields: map[string]string{
				"run_id_short":         "jr_bbb22",
				"job_run_state":        "FAILED",
				"started_on":           "2026-03-19 14:30:00",
				"execution_time_human": "5m 12s",
				"error_message":        "An error occurred while calling o42.pyWriteDynamicFrame: Connection refused",
				"dpu_hours":            "3.3",
				"run_id":               "jr_bbb22222-2222-2222-2222-222222222222",
				"job_name":             jobName,
			},
			RawStruct: gluetypes.JobRun{
				Id:            aws.String("jr_bbb22222-2222-2222-2222-222222222222"),
				JobName:       aws.String(jobName),
				JobRunState:   gluetypes.JobRunStateFailed,
				StartedOn:     aws.Time(mustParseTime("2026-03-19T14:30:00+00:00")),
				CompletedOn:   aws.Time(mustParseTime("2026-03-19T14:35:12+00:00")),
				ExecutionTime: 312,
				ErrorMessage:  aws.String("An error occurred while calling o42.pyWriteDynamicFrame: Connection refused"),
				DPUSeconds:    &dpuFailed,
			},
		},
		{
			ID:     "jr_ccc33333-3333-3333-3333-333333333333",
			Name:   "2026-03-21 02:00:00",
			Status: "RUNNING",
			Fields: map[string]string{
				"run_id_short":         "jr_ccc33",
				"job_run_state":        "RUNNING",
				"started_on":           "2026-03-21 02:00:00",
				"execution_time_human": "",
				"error_message":        "",
				"dpu_hours":            "",
				"run_id":               "jr_ccc33333-3333-3333-3333-333333333333",
				"job_name":             jobName,
			},
			RawStruct: gluetypes.JobRun{
				Id:          aws.String("jr_ccc33333-3333-3333-3333-333333333333"),
				JobName:     aws.String(jobName),
				JobRunState: gluetypes.JobRunStateRunning,
				StartedOn:   aws.Time(mustParseTime("2026-03-21T02:00:00+00:00")),
			},
		},
		{
			ID:     "jr_ddd44444-4444-4444-4444-444444444444",
			Name:   "2026-03-18 22:00:00",
			Status: "TIMEOUT",
			Fields: map[string]string{
				"run_id_short":         "jr_ddd44",
				"job_run_state":        "TIMEOUT",
				"started_on":           "2026-03-18 22:00:00",
				"execution_time_human": "2h 0m",
				"error_message":        "Job execution exceeded timeout of 7200 seconds",
				"dpu_hours":            "20.0",
				"run_id":               "jr_ddd44444-4444-4444-4444-444444444444",
				"job_name":             jobName,
			},
			RawStruct: gluetypes.JobRun{
				Id:            aws.String("jr_ddd44444-4444-4444-4444-444444444444"),
				JobName:       aws.String(jobName),
				JobRunState:   gluetypes.JobRunStateTimeout,
				StartedOn:     aws.Time(mustParseTime("2026-03-18T22:00:00+00:00")),
				CompletedOn:   aws.Time(mustParseTime("2026-03-19T00:00:00+00:00")),
				ExecutionTime: 7200,
				ErrorMessage:  aws.String("Job execution exceeded timeout of 7200 seconds"),
				DPUSeconds:    &dpuTimeout,
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
