package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/glue"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("glue", []string{"job_name", "glue_version", "worker_type", "num_workers", "last_modified"})
	resource.Register("glue", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchGlueJobs(ctx, c.Glue)
	})
}

// FetchGlueJobs calls the Glue GetJobs API and converts the response
// into a slice of generic Resource structs.
func FetchGlueJobs(ctx context.Context, api GlueGetJobsAPI) ([]resource.Resource, error) {
	output, err := api.GetJobs(ctx, &glue.GetJobsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching Glue jobs: %w", err)
	}

	var resources []resource.Resource

	for _, job := range output.Jobs {
		jobName := ""
		if job.Name != nil {
			jobName = *job.Name
		}

		role := ""
		if job.Role != nil {
			role = *job.Role
		}

		glueVersion := ""
		if job.GlueVersion != nil {
			glueVersion = *job.GlueVersion
		}

		workerType := string(job.WorkerType)

		numWorkers := ""
		if job.NumberOfWorkers != nil {
			numWorkers = strconv.Itoa(int(*job.NumberOfWorkers))
		}

		maxRetries := strconv.Itoa(int(job.MaxRetries))

		createdOn := ""
		if job.CreatedOn != nil {
			createdOn = job.CreatedOn.Format("2006-01-02 15:04:05")
		}

		lastModified := ""
		if job.LastModifiedOn != nil {
			lastModified = job.LastModifiedOn.Format("2006-01-02 15:04:05")
		}

		commandName := ""
		if job.Command != nil && job.Command.Name != nil {
			commandName = *job.Command.Name
		}

		r := resource.Resource{
			ID:     jobName,
			Name:   jobName,
			Status: "",
			Fields: map[string]string{
				"job_name":      jobName,
				"role":          role,
				"glue_version":  glueVersion,
				"worker_type":   workerType,
				"num_workers":   numWorkers,
				"max_retries":   maxRetries,
				"created_on":    createdOn,
				"last_modified": lastModified,
				"command":       commandName,
			},
			RawStruct: job,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
