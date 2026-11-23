// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
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
// "first page" and returns the zero cursor. Any other input — one with
// unknown fields, or one that decodes without error to the exact zero value
// (a bare "null" or "{}", both of which json.Unmarshal accepts silently but
// which encode() itself never produces: a token is only ever encoded when
// there is real state — a non-empty Next or a Done flag — to resume from) —
// has no legitimate origin other than this same encoder, so it is always a
// bug elsewhere (a foreign cursor, a caller wiring mistake, a token that
// outlived a field-tag change) — never external input worth tolerating
// silently. Restarting from page 1 in that case would duplicate or drop
// tasks with no signal, so it is reported as an error instead.
func decodeEcsSvcTasksCursor(token string) (ecsSvcTasksCursor, error) {
	var cur ecsSvcTasksCursor
	if token == "" {
		return cur, nil
	}
	dec := json.NewDecoder(strings.NewReader(token))
	dec.DisallowUnknownFields()
	var trailing json.RawMessage
	if err := dec.Decode(&cur); err != nil || cur == (ecsSvcTasksCursor{}) || dec.Decode(&trailing) != io.EOF {
		return ecsSvcTasksCursor{}, fmt.Errorf("decoding ecs svc tasks continuation token: invalid or foreign cursor %q", token)
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
// further NextToken for this status — a non-nil but empty NextToken is
// exhaustion, not "one more page", the same nil-or-empty contract every
// AWS-generated paginator in this SDK enforces; ListTasks has no generated
// paginator of its own to inherit it from. Without this, a status that
// returns a non-nil empty NextToken would never latch done, and the compound
// cursor's Done flag for that status would never become true.
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
	if output.NextToken != nil && *output.NextToken != "" {
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
	var listFailures []Failure
	var firstErr error
	listed := 0
	// A status whose tasks could not be listed costs its own tasks, recorded
	// as a failure, and is not asked again on a later page; the other status
	// still lists.
	listStatus := func(status ecstypes.DesiredStatus, token string) (string, bool) {
		listed++
		arns, nextTok, done, err := fetchEcsTaskArnsPage(ctx, listAPI, cluster, serviceName, status, token)
		if err != nil {
			if !IsNotFoundErr(err) {
				listFailures = append(listFailures, FailedCall(string(status), err))
				firstErr = cmp.Or(firstErr, err)
			}
			return "", true
		}
		allTaskArns = append(allTaskArns, arns...)
		return nextTok, done
	}
	if !cur.RunningDone {
		next.RunningNext, next.RunningDone = listStatus(ecstypes.DesiredStatusRunning, cur.RunningNext)
	}
	if !cur.StoppedDone {
		next.StoppedNext, next.StoppedDone = listStatus(ecstypes.DesiredStatusStopped, cur.StoppedNext)
	}
	// Every status refused is no list at all: the failure is the page's.
	if listed > 0 && len(listFailures) == listed {
		return resource.FetchResult{}, fmt.Errorf("listing ECS tasks for %s: %w", serviceName, firstErr)
	}

	isTruncated := !next.RunningDone || !next.StoppedDone

	// DescribeTasks API accepts max 100 ARNs per call — batch if needed.
	const descBatchSize = 100
	var resources []resource.Resource
	var failures []Failure
	for i := 0; i < len(allTaskArns); i += descBatchSize {
		chunk := allTaskArns[i:min(i+descBatchSize, len(allTaskArns))]
		// Tags are returned only when named in Include.
		descOutput, err := describeAPI.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   chunk,
			Include: []ecstypes.TaskField{ecstypes.TaskFieldTags},
		})
		var ans listedAnswer
		if err == nil {
			ans.gone = ecsGone(descOutput.Failures)
			for _, task := range descOutput.Tasks {
				ans.rows = append(ans.rows, convertEcsTask(task))
			}
		}
		rows, chunkFailures := keepListed("DescribeTasks", chunk, ecsTaskRowID, ans, err, func(arn string, err error) resource.Resource {
			taskID := ecsTaskRowID(arn)
			r := DegradedDetails("ecs_tasks", taskID, err)
			r.Fields["task_id"], r.Fields["task_arn"] = taskID, arn
			return r
		})
		resources = append(resources, rows...)
		failures = append(failures, chunkFailures...)
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
		}, JoinAggregates(
			AggregateFailures("ecs_tasks: ListTasks", listFailures, listed),
			AggregateFailures("ecs_tasks: DescribeTasks", failures, len(allTaskArns)),
		)
}

// convertEcsTask converts a single ECS Task into a generic Resource.
func convertEcsTask(task ecstypes.Task) resource.Resource {
	taskArn := ""
	taskIDShort := ""
	if task.TaskArn != nil {
		taskArn = *task.TaskArn
		taskIDShort, _ = ecsTaskRefToID(taskArn, domain.RefContext{})
	}

	status := ""
	if task.LastStatus != nil {
		status = *task.LastStatus
	}

	health := ecsTaskHealthWords(task.HealthStatus)

	taskDefShort := ""
	if task.TaskDefinitionArn != nil {
		taskDefShort = taskDefRevision(*task.TaskDefinitionArn)
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

	return withWave1("ecs_tasks", resource.Resource{
		ID:   taskIDShort,
		Name: taskIDShort,
		Fields: map[string]string{
			"task_id":        taskIDShort,
			"status":         status,
			"health":         health,
			"task_def_short": taskDefShort,
			"started_at":     startedAt,
			"stopped_reason": stoppedReason,
			"stop_code":      stopCode,
			"task_arn":       taskArn,
		},
		RawStruct: task,
	}, findings...)
}
