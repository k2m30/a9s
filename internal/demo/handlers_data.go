package demo

import (
	"net/http"

	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
)

// registerDataHandlers registers Glue and Athena handlers.
func registerDataHandlers(t *Transport) {
	registerGlueHandlers(t)
	registerAthenaHandlers(t)
}

// ---------------------------------------------------------------------------
// Glue (awsjson11)
// ---------------------------------------------------------------------------

func registerGlueHandlers(t *Transport) {
	t.Handle("glue", "GetJobs", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["glue"]()
		jobs := ExtractSDK[gluetypes.Job](resources)

		out := &glue.GetJobsOutput{
			Jobs: jobs,
		}
		return JSONResponse(out)
	})

	t.Handle("glue", "GetJobRuns", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["glue"]()
		if len(resources) == 0 {
			out := &glue.GetJobRunsOutput{}
			return JSONResponse(out)
		}
		jobName := resources[0].Fields["job_name"]
		runResources := childDemoData["glue_runs"](map[string]string{"job_name": jobName})
		runs := ExtractSDK[gluetypes.JobRun](runResources)

		out := &glue.GetJobRunsOutput{
			JobRuns: runs,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// Athena (awsjson11)
// ---------------------------------------------------------------------------

func registerAthenaHandlers(t *Transport) {
	t.Handle("athena", "ListWorkGroups", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["athena"]()
		wgs := ExtractSDK[athenatypes.WorkGroupSummary](resources)

		out := &athena.ListWorkGroupsOutput{
			WorkGroups: wgs,
		}
		return JSONResponse(out)
	})
}
