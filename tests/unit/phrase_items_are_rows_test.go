// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// phrase_items_are_rows_test.go — one condition found on several items of one
// resource states the condition once and lists every item.
//
// A pipeline with two failed stages, a user with two keys past rotation and a
// task with two containers that exited non-zero each carry one condition
// found more than once. The resource says that condition once, in the code's
// registered phrase, and each offending item is a supporting row. Building the
// phrase out of the first item instead makes the wording a property of the
// item, and the loop that then stops at the first item leaves the reader with
// no way to know the others were even inspected.
//
// The demo bench cannot see this: every demo witness carries exactly one
// offending item, by the one-witness-per-finding rule, so the same-code
// collapse never has a second emission to drop. These are the hand-built
// multi-item cases.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// registeredPhrase returns the phrase the installed catalog declares for code
// and fails the test when that phrase still carries a "<…>" placeholder. A
// placeholder means the wording is still assembled per item at emit time,
// which is the shape these tests exist to rule out.
func registeredPhrase(t *testing.T, code domain.FindingCode) string {
	t.Helper()
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, f := range td.Findings {
			if f.Code != code {
				continue
			}
			if strings.Contains(f.Phrase, "<") {
				t.Errorf("catalog declares %s with phrase %q — the placeholder makes the wording a "+
					"property of the item; register the condition's own phrase and put the item in "+
					"a supporting row", code, f.Phrase)
			}
			return f.Phrase
		}
	}
	t.Fatalf("no catalog.FindingDef declares %s", code)
	return ""
}

// onlyFinding returns the single Finding emitted for resourceID, failing when
// the resource carries none or more than one under code.
func onlyFinding(t *testing.T, result awsclient.IssueEnricherResult, resourceID string, code domain.FindingCode) domain.Finding {
	t.Helper()
	var matched []domain.Finding
	for _, f := range result.Findings[resourceID] {
		if f.Code == code {
			matched = append(matched, f)
		}
	}
	if len(matched) != 1 {
		t.Fatalf("%s on %s: got %d finding(s), want exactly 1 — one condition found on several "+
			"items is still one condition; findings: %v", code, resourceID, len(matched), result.Findings[resourceID])
	}
	return matched[0]
}

// rowValues returns the values of every supporting row labelled label on the
// (resourceID, code) AttentionDetail entry.
func phraseRowValues(result awsclient.IssueEnricherResult, resourceID string, code domain.FindingCode, label string) []string {
	var values []string
	for _, row := range result.AttentionDetails[resourceID][code].Rows {
		if row.Label == label {
			values = append(values, row.Value)
		}
	}
	return values
}

func TestPipelineTwoFailedStagesNamesBothUnderOnePhrase(t *testing.T) {
	const code domain.FindingCode = "pipeline.stage-failed"
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"acme-release": {
				StageStates: []cptypes.StageState{
					stageState("Source", cptypes.StageExecutionStatusSucceeded),
					stageState("Build", cptypes.StageExecutionStatusFailed),
					stageState("Deploy", cptypes.StageExecutionStatusFailed),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{{ID: "acme-release", Name: "acme-release"}}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := onlyFinding(t, result, "acme-release", code)
	if f.Phrase != "stage failed" {
		t.Errorf("Phrase = %q, want %q — the wording belongs to the code, the stage belongs in a row",
			f.Phrase, "stage failed")
	}
	if want := registeredPhrase(t, code); f.Phrase != want {
		t.Errorf("Phrase = %q but the catalog declares %q — the emitter and the signals page must "+
			"say the same thing", f.Phrase, want)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want SevBroken", f.Severity)
	}
	if f.Source != "wave2:pipeline" {
		t.Errorf("Source = %q, want %q", f.Source, "wave2:pipeline")
	}

	stages := phraseRowValues(result, "acme-release", code, "Failed Stage")
	if len(stages) != 2 || stages[0] != "Build" || stages[1] != "Deploy" {
		t.Errorf("Failed Stage rows = %v, want [Build Deploy] — a stage the enricher inspected and "+
			"found failed must be listed, not dropped because an earlier one already failed", stages)
	}
}

func TestPipelineOneFailedStageNamesOnlyThatStage(t *testing.T) {
	const code domain.FindingCode = "pipeline.stage-failed"
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"acme-nightly": {
				StageStates: []cptypes.StageState{
					stageState("Source", cptypes.StageExecutionStatusSucceeded),
					stageState("Build", cptypes.StageExecutionStatusFailed),
					stageState("Deploy", cptypes.StageExecutionStatusSucceeded),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{{ID: "acme-nightly", Name: "acme-nightly"}}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := onlyFinding(t, result, "acme-nightly", code)
	if f.Phrase != "stage failed" {
		t.Errorf("Phrase = %q, want %q", f.Phrase, "stage failed")
	}
	stages := phraseRowValues(result, "acme-nightly", code, "Failed Stage")
	if len(stages) != 1 || stages[0] != "Build" {
		t.Errorf("Failed Stage rows = %v, want [Build] — the succeeded stages must not be listed", stages)
	}
}

func TestIAMUserTwoKeysPastRotationNameBothUnderOnePhrase(t *testing.T) {
	const code domain.FindingCode = "iam-user.old-key"
	old := time.Now().Add(-200 * 24 * time.Hour)
	fake := &iamUserMFAFake{
		accessKeysByUser: map[string][]iamtypes.AccessKeyMetadata{
			"ci-deployer": {
				{AccessKeyId: aws.String("AKIAIOSFODNN7EXAMPLE"), Status: iamtypes.StatusTypeActive, CreateDate: &old},
				{AccessKeyId: aws.String("AKIAI44QH8DHBEXAMPLE"), Status: iamtypes.StatusTypeActive, CreateDate: &old},
			},
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{{ID: "ci-deployer", Fields: map[string]string{"user_name": "ci-deployer"}}}

	result, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := onlyFinding(t, result, "ci-deployer", code)
	want := registeredPhrase(t, code)
	if f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q — the key that triggered it belongs in a row",
			f.Phrase, want)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}

	keys := phraseRowValues(result, "ci-deployer", code, "Access key")
	if len(keys) != 2 {
		t.Errorf("Access key rows = %v, want one per key past rotation (2) — a second key past "+
			"rotation is a second thing to rotate, not a duplicate of the first", keys)
	}
}

func TestIAMUserOneKeyPastRotationNamesOnlyThatKey(t *testing.T) {
	const code domain.FindingCode = "iam-user.old-key"
	old := time.Now().Add(-200 * 24 * time.Hour)
	fresh := time.Now().Add(-2 * 24 * time.Hour)
	fake := &iamUserMFAFake{
		accessKeysByUser: map[string][]iamtypes.AccessKeyMetadata{
			"build-agent": {
				{AccessKeyId: aws.String("AKIAIOSFODNN7EXAMPLE"), Status: iamtypes.StatusTypeActive, CreateDate: &old},
				{AccessKeyId: aws.String("AKIAI44QH8DHBEXAMPLE"), Status: iamtypes.StatusTypeActive, CreateDate: &fresh},
			},
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{{ID: "build-agent", Fields: map[string]string{"user_name": "build-agent"}}}

	result, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := onlyFinding(t, result, "build-agent", code)
	if want := registeredPhrase(t, code); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	keys := phraseRowValues(result, "build-agent", code, "Access key")
	if len(keys) != 1 {
		t.Errorf("Access key rows = %v, want only the key past rotation (1) — a key created two "+
			"days ago is not due for rotation", keys)
	}
}

func TestECSTaskTwoFailedContainersNameBothUnderOnePhrase(t *testing.T) {
	const code domain.FindingCode = "ecs-task.task-failed"
	taskID := "a1b2c3d4e5f60708090a0b0c0d0e0f10"
	exit137 := int32(137)
	exit1 := int32(1)
	fake := &fakeECSEnricher{
		descTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{{
				TaskArn:  aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-cluster/" + taskID),
				StopCode: ecstypes.TaskStopCodeEssentialContainerExited,
				Containers: []ecstypes.Container{
					{Name: aws.String("web"), ExitCode: &exit137},
					{Name: aws.String("sidecar"), ExitCode: &exit1},
				},
			}},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{{
		ID:   taskID,
		Name: taskID,
		Fields: map[string]string{
			"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster",
			"task_id": taskID,
		},
	}}

	result, err := awsclient.EnrichECSTasks(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := onlyFinding(t, result, taskID, code)
	want := registeredPhrase(t, code)
	if f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q — the container that exited belongs in a row",
			f.Phrase, want)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want SevBroken", f.Severity)
	}
	if f.Source != "wave2:ecs-task" {
		t.Errorf("Source = %q, want %q", f.Source, "wave2:ecs-task")
	}

	containers := phraseRowValues(result, taskID, code, "Container")
	if len(containers) != 2 {
		t.Errorf("Container rows = %v, want one per container that exited non-zero (2) — a second "+
			"dead container is a second thing to look at", containers)
	}
}

func TestECSTaskOneFailedContainerNamesOnlyThatContainer(t *testing.T) {
	const code domain.FindingCode = "ecs-task.task-failed"
	taskID := "b1b2c3d4e5f60708090a0b0c0d0e0f11"
	exit137 := int32(137)
	exit0 := int32(0)
	fake := &fakeECSEnricher{
		descTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{{
				TaskArn:  aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-cluster/" + taskID),
				StopCode: ecstypes.TaskStopCodeEssentialContainerExited,
				Containers: []ecstypes.Container{
					{Name: aws.String("web"), ExitCode: &exit137},
					{Name: aws.String("sidecar"), ExitCode: &exit0},
				},
			}},
		},
	}
	clients := &awsclient.ServiceClients{ECS: fake}
	resources := []resource.Resource{{
		ID:   taskID,
		Name: taskID,
		Fields: map[string]string{
			"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/acme-cluster",
			"task_id": taskID,
		},
	}}

	result, err := awsclient.EnrichECSTasks(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := onlyFinding(t, result, taskID, code)
	if want := registeredPhrase(t, code); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	containers := phraseRowValues(result, taskID, code, "Container")
	if len(containers) != 1 || !strings.Contains(containers[0], "web") {
		t.Errorf("Container rows = %v, want only the container that exited non-zero — a container "+
			"that exited 0 did its job", containers)
	}
}
