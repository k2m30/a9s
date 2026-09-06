// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// ecsSvcTasksCursor encodes the two independent ListTasks continuation
// cursors (RUNNING and STOPPED) into a single opaque token, since one
// invocation of FetchEcsSvcTasks combines one page of each status. Done
// marks a status as fully drained so a later page neither re-fetches nor
// re-appends its already-seen first page.
type ecsSvcTasksCursor struct {
	RunningNext string `json:"rn,omitempty"`
	RunningDone bool   `json:"rd,omitempty"`
	StoppedNext string `json:"sn,omitempty"`
	StoppedDone bool   `json:"sd,omitempty"`
}

// decodeEcsSvcTasksCursor decodes a compound continuation token previously
// produced by ecsSvcTasksCursor.encode(). An empty token legitimately means
// "first page" and returns the zero cursor. A non-empty token that fails to
// decode has no legitimate origin other than this same encoder, so it is
// always a bug elsewhere (a foreign cursor, a caller wiring mistake, a token
// that outlived a field-tag change) — never external input worth tolerating
// silently. Restarting from page 1 in that case would duplicate or drop
// tasks with no signal, so it is reported as an error instead.
func decodeEcsSvcTasksCursor(token string) (ecsSvcTasksCursor, error) {
	var cur ecsSvcTasksCursor
	if token == "" {
		return cur, nil
	}
	if err := json.Unmarshal([]byte(token), &cur); err != nil {
		return ecsSvcTasksCursor{}, fmt.Errorf("decoding ecs svc tasks continuation token: %w", err)
	}
	return cur, nil
}

func (c ecsSvcTasksCursor) encode() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

// fetchEcsTaskArnsPage fetches a single ListTasks page for one DesiredStatus,
// resuming from nextToken when non-empty. done is true when AWS reported no
// further NextToken for this status.
func fetchEcsTaskArnsPage(ctx context.Context, listAPI ECSListTasksAPI, cluster, serviceName string, status ecstypes.DesiredStatus, nextToken string) (arns []string, newNextToken string, done bool, err error) {
	input := &ecs.ListTasksInput{
		Cluster:       aws.String(cluster),
		ServiceName:   aws.String(serviceName),
		DesiredStatus: status,
	}
	if nextToken != "" {
		input.NextToken = aws.String(nextToken)
	}

	output, err := listAPI.ListTasks(ctx, input)
	if err != nil {
		return nil, "", false, err
	}
	if output.NextToken != nil {
		return output.TaskArns, *output.NextToken, false, nil
	}
	return output.TaskArns, "", true, nil
}

// FetchEcsSvcTasks calls ListTasks for RUNNING and STOPPED statuses (one page
// each), then DescribeTasks for full details. A single ListTasks call is made
// per status per invocation. continuationToken is a compound cursor (see
// ecsSvcTasksCursor) that resumes each status independently; a status already
// marked done is skipped rather than re-fetched.
func FetchEcsSvcTasks(
	ctx context.Context,
	listAPI ECSListTasksAPI,
	describeAPI ECSDescribeTasksAPI,
	cluster, serviceName string,
	continuationToken string,
) (resource.FetchResult, error) {
	cur, err := decodeEcsSvcTasksCursor(continuationToken)
	if err != nil {
		return resource.FetchResult{}, err
	}
	next := cur

	var allTaskArns []string

	if !cur.RunningDone {
		arns, nextTok, done, err := fetchEcsTaskArnsPage(ctx, listAPI, cluster, serviceName, ecstypes.DesiredStatusRunning, cur.RunningNext)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing ECS tasks for %s: %w", serviceName, err)
		}
		allTaskArns = append(allTaskArns, arns...)
		next.RunningNext, next.RunningDone = nextTok, done
	}
	if !cur.StoppedDone {
		arns, nextTok, done, err := fetchEcsTaskArnsPage(ctx, listAPI, cluster, serviceName, ecstypes.DesiredStatusStopped, cur.StoppedNext)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing ECS tasks for %s: %w", serviceName, err)
		}
		allTaskArns = append(allTaskArns, arns...)
		next.StoppedNext, next.StoppedDone = nextTok, done
	}

	isTruncated := !next.RunningDone || !next.StoppedDone

	// DescribeTasks API accepts max 100 ARNs per call — batch if needed.
	const descBatchSize = 100
	var allTasks []ecstypes.Task
	for i := 0; i < len(allTaskArns); i += descBatchSize {
		end := min(i+descBatchSize, len(allTaskArns))
		descOutput, err := describeAPI.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   allTaskArns[i:end],
		})
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("describing ECS tasks for %s: %w", serviceName, err)
		}
		allTasks = append(allTasks, descOutput.Tasks...)
	}

	var resources []resource.Resource
	for _, task := range allTasks {
		resources = append(resources, convertEcsTask(task))
	}

	totalHint := len(resources)
	nextToken := ""
	if isTruncated {
		totalHint = -1
		nextToken = next.encode()
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

// convertEcsTask converts a single ECS Task into a generic Resource.
func convertEcsTask(task ecstypes.Task) resource.Resource {
	taskArn := ""
	taskIDShort := ""
	if task.TaskArn != nil {
		taskArn = *task.TaskArn
		parts := strings.Split(taskArn, "/")
		taskIDShort = parts[len(parts)-1]
	}

	status := ""
	if task.LastStatus != nil {
		status = *task.LastStatus
	}

	health := ecsTaskHealthWords(task.HealthStatus)

	taskDefShort := ""
	if task.TaskDefinitionArn != nil {
		// Extract "family:revision" from ARN like
		// "arn:aws:ecs:us-east-1:123456789012:task-definition/web-app:5"
		parts := strings.Split(*task.TaskDefinitionArn, "/")
		if len(parts) > 0 {
			taskDefShort = parts[len(parts)-1]
		}
	}

	startedAt := ""
	if task.StartedAt != nil {
		startedAt = task.StartedAt.UTC().Format("2006-01-02 15:04")
	}

	stoppedReason := ""
	if task.StoppedReason != nil {
		stoppedReason = strings.ReplaceAll(*task.StoppedReason, "\n", " ")
	}

	stopCode := string(task.StopCode)

	findings := ecsTaskStructuralFindings(status, stopCode, health)

	return resource.Resource{
		ID:   taskIDShort,
		Name: taskIDShort,
		Fields: map[string]string{
			"task_id_short":  taskIDShort,
			"status":         status,
			"health":         health,
			"task_def_short": taskDefShort,
			"started_at":     startedAt,
			"stopped_reason": stoppedReason,
			"stop_code":      stopCode,
			"task_arn":       taskArn,
		},
		Findings:  findings,
		RawStruct: task,
	}
}
