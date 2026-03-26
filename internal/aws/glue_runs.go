package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

const maxGlueJobRuns = 200

func init() {
	resource.RegisterFieldKeys("glue_runs", []string{
		"run_id_short", "job_run_state", "started_on",
		"execution_time_human", "error_message", "dpu_hours",
		"run_id", "job_name",
	})

	resource.RegisterChildFetcher("glue_runs", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchGlueJobRuns(ctx, c.Glue, parentCtx["job_name"])
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Job Runs",
		ShortName: "glue_runs",
		Columns:   resource.GlueRunColumns(),
		CopyField: "error_message",
	})
}

// FetchGlueJobRuns calls the Glue GetJobRuns API and converts the response
// into a slice of generic Resource structs. Pagination is followed via
// NextToken, capped at maxGlueJobRuns (200) items.
func FetchGlueJobRuns(
	ctx context.Context,
	api GlueGetJobRunsAPI,
	jobName string,
) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		input := &glue.GetJobRunsInput{
			JobName:   &jobName,
			NextToken: nextToken,
		}

		output, err := api.GetJobRuns(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, run := range output.JobRuns {
			resources = append(resources, convertGlueJobRun(run))

			if len(resources) >= maxGlueJobRuns {
				return resources, nil
			}
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}

// convertGlueJobRun converts a single Glue JobRun into a generic Resource.
func convertGlueJobRun(run gluetypes.JobRun) resource.Resource {
	runID := ""
	if run.Id != nil {
		runID = *run.Id
	}

	runIDShort := runID
	if len(runIDShort) > 8 {
		runIDShort = runIDShort[:8]
	}

	jobRunState := string(run.JobRunState)

	startedOn := ""
	if run.StartedOn != nil {
		startedOn = run.StartedOn.UTC().Format("2006-01-02 15:04:05")
	}

	executionTimeHuman := ""
	if run.ExecutionTime != 0 {
		secs := time.Duration(run.ExecutionTime) * time.Second
		executionTimeHuman = FormatHumanDuration(secs)
	}

	errorMessage := ""
	if run.ErrorMessage != nil {
		errorMessage = *run.ErrorMessage
		errorMessage = strings.ReplaceAll(errorMessage, "\r\n", " ")
		errorMessage = strings.ReplaceAll(errorMessage, "\n", " ")
		errorMessage = strings.ReplaceAll(errorMessage, "\r", " ")
	}

	dpuHours := ""
	if run.DPUSeconds != nil && *run.DPUSeconds != 0 {
		dpuHours = fmt.Sprintf("%.1f", *run.DPUSeconds/3600.0)
	}

	jobName := ""
	if run.JobName != nil {
		jobName = *run.JobName
	}

	return resource.Resource{
		ID:     runID,
		Name:   startedOn,
		Status: jobRunState,
		Fields: map[string]string{
			"run_id_short":         runIDShort,
			"job_run_state":        jobRunState,
			"started_on":           startedOn,
			"execution_time_human": executionTimeHuman,
			"error_message":        errorMessage,
			"dpu_hours":            dpuHours,
			"run_id":               runID,
			"job_name":             jobName,
		},
		RawStruct: run,
	}
}
