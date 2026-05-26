package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// glueRunFindings returns wave1 findings derived from a Glue job-run state.
// FAILED/TIMEOUT/ERROR → broken; RUNNING/STARTING/STOPPING/WAITING → warn;
// STOPPED → dim. SUCCEEDED and unknown states return nil (healthy).
func glueRunFindings(state gluetypes.JobRunState) []domain.Finding {
	switch state {
	case gluetypes.JobRunStateFailed:
		return []domain.Finding{{Code: CodeGlueRunFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
	case gluetypes.JobRunStateTimeout:
		return []domain.Finding{{Code: CodeGlueRunTimeout, Phrase: "timeout", Severity: domain.SevBroken, Source: "wave1"}}
	case gluetypes.JobRunStateError:
		return []domain.Finding{{Code: CodeGlueRunError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1"}}
	case gluetypes.JobRunStateExpired:
		return []domain.Finding{{Code: CodeGlueRunExpired, Phrase: "expired", Severity: domain.SevBroken, Source: "wave1"}}
	case gluetypes.JobRunStateRunning:
		return []domain.Finding{{Code: CodeGlueRunRunning, Phrase: "running", Severity: domain.SevWarn, Source: "wave1"}}
	case gluetypes.JobRunStateStarting:
		return []domain.Finding{{Code: CodeGlueRunStarting, Phrase: "starting", Severity: domain.SevWarn, Source: "wave1"}}
	case gluetypes.JobRunStateStopping:
		return []domain.Finding{{Code: CodeGlueRunStopping, Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1"}}
	case gluetypes.JobRunStateWaiting:
		return []domain.Finding{{Code: CodeGlueRunWaiting, Phrase: "waiting", Severity: domain.SevWarn, Source: "wave1"}}
	case gluetypes.JobRunStateStopped:
		return []domain.Finding{{Code: CodeGlueRunStopped, Phrase: "stopped", Severity: domain.SevDim, Source: "wave1"}}
	}
	return nil
}

// FetchGlueJobRuns calls the Glue GetJobRuns API and converts the response
// into a FetchResult with pagination support. A single API call is made per
// invocation; IsTruncated and NextToken are forwarded as pagination metadata
// for the caller to request the next page.
func FetchGlueJobRuns(
	ctx context.Context,
	api GlueGetJobRunsAPI,
	jobName string,
	continuationToken string,
) (resource.FetchResult, error) {
	input := &glue.GetJobRunsInput{
		JobName: &jobName,
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.GetJobRuns(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Glue job runs: %w", err)
	}

	var resources []resource.Resource
	for _, run := range output.JobRuns {
		resources = append(resources, convertGlueJobRun(run))
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
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
		startedOn = run.StartedOn.UTC().Format("2006-01-02 15:04")
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
		ID:       runID,
		Name:     startedOn,
		Findings: glueRunFindings(run.JobRunState),
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
