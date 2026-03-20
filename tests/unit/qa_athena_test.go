package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T-ATH-001 - Test Athena Workgroups response parsing
// ---------------------------------------------------------------------------

func TestFetchAthenaWorkgroups_ParsesMultipleWorkgroups(t *testing.T) {
	now := time.Now()
	mock := &mockAthenaClient{
		output: &athena.ListWorkGroupsOutput{
			WorkGroups: []athenatypes.WorkGroupSummary{
				{
					Name:         aws.String("primary"),
					State:        athenatypes.WorkGroupStateEnabled,
					Description:  aws.String("Primary workgroup"),
					CreationTime: &now,
					EngineVersion: &athenatypes.EngineVersion{
						SelectedEngineVersion:  aws.String("Athena engine version 3"),
						EffectiveEngineVersion: aws.String("Athena engine version 3"),
					},
				},
				{
					Name:         aws.String("analytics"),
					State:        athenatypes.WorkGroupStateDisabled,
					Description:  aws.String("Analytics workgroup"),
					CreationTime: &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchAthenaWorkgroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "primary" {
		t.Errorf("expected Name 'primary', got %q", r.Name)
	}
	if r.ID != "primary" {
		t.Errorf("expected ID 'primary', got %q", r.ID)
	}
	if r.Status != "ENABLED" {
		t.Errorf("expected Status 'ENABLED', got %q", r.Status)
	}
	if r.Fields["workgroup_name"] != "primary" {
		t.Errorf("expected Fields[workgroup_name] 'primary', got %q", r.Fields["workgroup_name"])
	}
	if r.Fields["state"] != "ENABLED" {
		t.Errorf("expected Fields[state] 'ENABLED', got %q", r.Fields["state"])
	}
	if r.Fields["description"] != "Primary workgroup" {
		t.Errorf("expected Fields[description] 'Primary workgroup', got %q", r.Fields["description"])
	}
	if r.Fields["engine_version"] != "Athena engine version 3" {
		t.Errorf("expected Fields[engine_version] 'Athena engine version 3', got %q", r.Fields["engine_version"])
	}

	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}

	// Second workgroup
	r2 := resources[1]
	if r2.Status != "DISABLED" {
		t.Errorf("expected Status 'DISABLED', got %q", r2.Status)
	}
}

func TestFetchAthenaWorkgroups_EmptyResponse(t *testing.T) {
	mock := &mockAthenaClient{
		output: &athena.ListWorkGroupsOutput{
			WorkGroups: []athenatypes.WorkGroupSummary{},
		},
	}

	resources, err := awsclient.FetchAthenaWorkgroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchAthenaWorkgroups_APIError(t *testing.T) {
	mock := &mockAthenaClient{
		err: &mockAPIError{code: "InternalServerException", message: "internal error"},
	}

	_, err := awsclient.FetchAthenaWorkgroups(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchAthenaWorkgroups_NilEngineVersion(t *testing.T) {
	mock := &mockAthenaClient{
		output: &athena.ListWorkGroupsOutput{
			WorkGroups: []athenatypes.WorkGroupSummary{
				{
					Name:  aws.String("no-engine"),
					State: athenatypes.WorkGroupStateEnabled,
				},
			},
		},
	}

	resources, err := awsclient.FetchAthenaWorkgroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].Fields["engine_version"] != "" {
		t.Errorf("expected empty engine_version, got %q", resources[0].Fields["engine_version"])
	}
}
