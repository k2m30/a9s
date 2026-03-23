package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Target Health fetcher tests (child of Target Groups)
// ---------------------------------------------------------------------------

// TestFetchTargetHealth_Basic verifies parsing of 4 targets with varied health
// states, checking ID, Name, Status, all Fields, and RawStruct.
func TestFetchTargetHealth_Basic(t *testing.T) {
	port80 := int32(80)
	port443 := int32(443)
	port8080 := int32(8080)
	port3000 := int32(3000)

	mock := &mockELBv2DescribeTargetHealthClient{
		output: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
				{
					Target: &elbtypes.TargetDescription{
						Id:               aws.String("i-0abc1234def56789a"),
						Port:             &port80,
						AvailabilityZone: aws.String("us-east-1a"),
					},
					TargetHealth: &elbtypes.TargetHealth{
						State:       elbtypes.TargetHealthStateEnumHealthy,
						Description: aws.String("Health checks passed"),
					},
				},
				{
					Target: &elbtypes.TargetDescription{
						Id:               aws.String("i-0bcd2345efg67890b"),
						Port:             &port443,
						AvailabilityZone: aws.String("us-east-1b"),
					},
					TargetHealth: &elbtypes.TargetHealth{
						State:       elbtypes.TargetHealthStateEnumHealthy,
						Description: aws.String("Health checks passed"),
					},
				},
				{
					Target: &elbtypes.TargetDescription{
						Id:               aws.String("i-0cde3456fgh78901c"),
						Port:             &port8080,
						AvailabilityZone: aws.String("us-east-1c"),
					},
					TargetHealth: &elbtypes.TargetHealth{
						State:       elbtypes.TargetHealthStateEnumUnhealthy,
						Reason:      elbtypes.TargetHealthReasonEnumFailedHealthChecks,
						Description: aws.String("Health checks failed with 503"),
					},
				},
				{
					Target: &elbtypes.TargetDescription{
						Id:               aws.String("i-0def4567ghi89012d"),
						Port:             &port3000,
						AvailabilityZone: aws.String("us-east-1a"),
					},
					TargetHealth: &elbtypes.TargetHealth{
						State:       elbtypes.TargetHealthStateEnumDraining,
						Reason:      elbtypes.TargetHealthReasonEnumDeregistrationInProgress,
						Description: aws.String("Target deregistration in progress"),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchTargetHealth(
		context.Background(),
		mock,
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abc123",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(resources))
	}

	t.Run("target_0_ID", func(t *testing.T) {
		if resources[0].ID != "i-0abc1234def56789a" {
			t.Errorf("ID: expected %q, got %q", "i-0abc1234def56789a", resources[0].ID)
		}
	})

	t.Run("target_0_Name", func(t *testing.T) {
		if resources[0].Name != "i-0abc1234def56789a" {
			t.Errorf("Name: expected %q, got %q", "i-0abc1234def56789a", resources[0].Name)
		}
	})

	t.Run("target_0_Status", func(t *testing.T) {
		if resources[0].Status != "healthy" {
			t.Errorf("Status: expected %q, got %q", "healthy", resources[0].Status)
		}
	})

	t.Run("target_0_fields", func(t *testing.T) {
		r := resources[0]
		if r.Fields["target_id"] != "i-0abc1234def56789a" {
			t.Errorf("Fields[target_id]: expected %q, got %q", "i-0abc1234def56789a", r.Fields["target_id"])
		}
		if r.Fields["port"] != "80" {
			t.Errorf("Fields[port]: expected %q, got %q", "80", r.Fields["port"])
		}
		if r.Fields["az"] != "us-east-1a" {
			t.Errorf("Fields[az]: expected %q, got %q", "us-east-1a", r.Fields["az"])
		}
		if r.Fields["health"] != "healthy" {
			t.Errorf("Fields[health]: expected %q, got %q", "healthy", r.Fields["health"])
		}
		if r.Fields["reason"] != "" {
			t.Errorf("Fields[reason]: expected empty for healthy target, got %q", r.Fields["reason"])
		}
		if r.Fields["description"] != "Health checks passed" {
			t.Errorf("Fields[description]: expected %q, got %q", "Health checks passed", r.Fields["description"])
		}
	})

	t.Run("target_0_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(elbtypes.TargetHealthDescription)
		if !ok {
			t.Fatalf("RawStruct should be elbtypes.TargetHealthDescription, got %T", r.RawStruct)
		}
		if raw.Target == nil || raw.Target.Id == nil || *raw.Target.Id != "i-0abc1234def56789a" {
			t.Errorf("RawStruct.Target.Id not preserved correctly")
		}
	})

	t.Run("target_2_unhealthy", func(t *testing.T) {
		r := resources[2]
		if r.Status != "unhealthy" {
			t.Errorf("Status: expected %q, got %q", "unhealthy", r.Status)
		}
		if r.Fields["reason"] != "Target.FailedHealthChecks" {
			t.Errorf("Fields[reason]: expected %q, got %q", "Target.FailedHealthChecks", r.Fields["reason"])
		}
		if r.Fields["description"] != "Health checks failed with 503" {
			t.Errorf("Fields[description]: expected %q, got %q", "Health checks failed with 503", r.Fields["description"])
		}
	})

	t.Run("target_3_draining", func(t *testing.T) {
		r := resources[3]
		if r.Status != "draining" {
			t.Errorf("Status: expected %q, got %q", "draining", r.Status)
		}
		if r.Fields["reason"] != "Target.DeregistrationInProgress" {
			t.Errorf("Fields[reason]: expected %q, got %q", "Target.DeregistrationInProgress", r.Fields["reason"])
		}
	})

	// Verify all targets have required fields
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"target_id", "port", "az", "health", "reason", "description"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchTargetHealth_Empty verifies that an empty response returns an empty
// slice with no error.
func TestFetchTargetHealth_Empty(t *testing.T) {
	mock := &mockELBv2DescribeTargetHealthClient{
		output: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: []elbtypes.TargetHealthDescription{},
		},
	}

	resources, err := awsclient.FetchTargetHealth(
		context.Background(),
		mock,
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/empty-tg/xyz",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchTargetHealth_APIError verifies that API errors are propagated correctly.
func TestFetchTargetHealth_APIError(t *testing.T) {
	mock := &mockELBv2DescribeTargetHealthClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchTargetHealth(
		context.Background(),
		mock,
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/err-tg/xyz",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

// TestFetchTargetHealth_AllStates verifies one target per health state:
// healthy, unhealthy, draining, initial, unavailable, unused.
func TestFetchTargetHealth_AllStates(t *testing.T) {
	port := int32(80)

	states := []struct {
		state    elbtypes.TargetHealthStateEnum
		expected string
	}{
		{elbtypes.TargetHealthStateEnumHealthy, "healthy"},
		{elbtypes.TargetHealthStateEnumUnhealthy, "unhealthy"},
		{elbtypes.TargetHealthStateEnumDraining, "draining"},
		{elbtypes.TargetHealthStateEnumInitial, "initial"},
		{elbtypes.TargetHealthStateEnumUnavailable, "unavailable"},
		{elbtypes.TargetHealthStateEnumUnused, "unused"},
	}

	descs := make([]elbtypes.TargetHealthDescription, len(states))
	for i, s := range states {
		descs[i] = elbtypes.TargetHealthDescription{
			Target: &elbtypes.TargetDescription{
				Id:   aws.String(fmt.Sprintf("i-%02d", i)),
				Port: &port,
			},
			TargetHealth: &elbtypes.TargetHealth{
				State: s.state,
			},
		}
	}

	mock := &mockELBv2DescribeTargetHealthClient{
		output: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: descs,
		},
	}

	resources, err := awsclient.FetchTargetHealth(
		context.Background(),
		mock,
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/all-states-tg/xyz",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != len(states) {
		t.Fatalf("expected %d resources, got %d", len(states), len(resources))
	}

	for i, s := range states {
		t.Run(s.expected, func(t *testing.T) {
			if resources[i].Status != s.expected {
				t.Errorf("Status: expected %q, got %q", s.expected, resources[i].Status)
			}
			if resources[i].Fields["health"] != s.expected {
				t.Errorf("Fields[health]: expected %q, got %q", s.expected, resources[i].Fields["health"])
			}
		})
	}
}

// TestFetchTargetHealth_IPTargets verifies IP-based targets instead of instance IDs.
func TestFetchTargetHealth_IPTargets(t *testing.T) {
	port8080 := int32(8080)
	port9090 := int32(9090)

	mock := &mockELBv2DescribeTargetHealthClient{
		output: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
				{
					Target: &elbtypes.TargetDescription{
						Id:               aws.String("10.0.1.47"),
						Port:             &port8080,
						AvailabilityZone: aws.String("us-west-2a"),
					},
					TargetHealth: &elbtypes.TargetHealth{
						State: elbtypes.TargetHealthStateEnumHealthy,
					},
				},
				{
					Target: &elbtypes.TargetDescription{
						Id:               aws.String("10.0.2.103"),
						Port:             &port9090,
						AvailabilityZone: aws.String("us-west-2b"),
					},
					TargetHealth: &elbtypes.TargetHealth{
						State:       elbtypes.TargetHealthStateEnumUnhealthy,
						Reason:      elbtypes.TargetHealthReasonEnumTimeout,
						Description: aws.String("Request timed out"),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchTargetHealth(
		context.Background(),
		mock,
		"arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/ip-tg/xyz",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("ip_target_0", func(t *testing.T) {
		r := resources[0]
		if r.ID != "10.0.1.47" {
			t.Errorf("ID: expected %q, got %q", "10.0.1.47", r.ID)
		}
		if r.Name != "10.0.1.47" {
			t.Errorf("Name: expected %q, got %q", "10.0.1.47", r.Name)
		}
		if r.Fields["target_id"] != "10.0.1.47" {
			t.Errorf("Fields[target_id]: expected %q, got %q", "10.0.1.47", r.Fields["target_id"])
		}
		if r.Fields["port"] != "8080" {
			t.Errorf("Fields[port]: expected %q, got %q", "8080", r.Fields["port"])
		}
		if r.Fields["az"] != "us-west-2a" {
			t.Errorf("Fields[az]: expected %q, got %q", "us-west-2a", r.Fields["az"])
		}
	})

	t.Run("ip_target_1_unhealthy", func(t *testing.T) {
		r := resources[1]
		if r.ID != "10.0.2.103" {
			t.Errorf("ID: expected %q, got %q", "10.0.2.103", r.ID)
		}
		if r.Status != "unhealthy" {
			t.Errorf("Status: expected %q, got %q", "unhealthy", r.Status)
		}
		if r.Fields["reason"] != "Target.Timeout" {
			t.Errorf("Fields[reason]: expected %q, got %q", "Target.Timeout", r.Fields["reason"])
		}
	})
}

// TestFetchTargetHealth_NilFields verifies that nil TargetHealth, nil Reason,
// nil Description do not cause a panic and produce empty strings.
func TestFetchTargetHealth_NilFields(t *testing.T) {
	port := int32(80)

	mock := &mockELBv2DescribeTargetHealthClient{
		output: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
				{
					// Target with nil TargetHealth
					Target: &elbtypes.TargetDescription{
						Id:   aws.String("i-nil-health"),
						Port: &port,
					},
					TargetHealth: nil,
				},
				{
					// Target with TargetHealth but zero-value Reason (no reason set)
					Target: &elbtypes.TargetDescription{
						Id:   aws.String("i-no-reason"),
						Port: &port,
					},
					TargetHealth: &elbtypes.TargetHealth{
						State: elbtypes.TargetHealthStateEnumHealthy,
						// Reason is zero value (empty string enum)
						// Description is nil
					},
				},
				{
					// Target with nil AvailabilityZone
					Target: &elbtypes.TargetDescription{
						Id:   aws.String("i-no-az"),
						Port: &port,
						// AvailabilityZone is nil
					},
					TargetHealth: &elbtypes.TargetHealth{
						State: elbtypes.TargetHealthStateEnumHealthy,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchTargetHealth(
		context.Background(),
		mock,
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/nil-tg/xyz",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("nil_TargetHealth", func(t *testing.T) {
		r := resources[0]
		if r.ID != "i-nil-health" {
			t.Errorf("ID: expected %q, got %q", "i-nil-health", r.ID)
		}
		// With nil TargetHealth, status/health/reason/description should be empty
		if r.Fields["health"] != "" {
			t.Errorf("Fields[health]: expected empty with nil TargetHealth, got %q", r.Fields["health"])
		}
		if r.Fields["reason"] != "" {
			t.Errorf("Fields[reason]: expected empty with nil TargetHealth, got %q", r.Fields["reason"])
		}
		if r.Fields["description"] != "" {
			t.Errorf("Fields[description]: expected empty with nil TargetHealth, got %q", r.Fields["description"])
		}
	})

	t.Run("no_reason", func(t *testing.T) {
		r := resources[1]
		if r.Fields["reason"] != "" {
			t.Errorf("Fields[reason]: expected empty for healthy target, got %q", r.Fields["reason"])
		}
		if r.Fields["description"] != "" {
			t.Errorf("Fields[description]: expected empty with nil Description, got %q", r.Fields["description"])
		}
	})

	t.Run("nil_az", func(t *testing.T) {
		r := resources[2]
		if r.Fields["az"] != "" {
			t.Errorf("Fields[az]: expected empty with nil AvailabilityZone, got %q", r.Fields["az"])
		}
	})
}

// TestFetchTargetHealth_RawStruct verifies that RawStruct preserves the original
// elbtypes.TargetHealthDescription, including all sub-structs.
func TestFetchTargetHealth_RawStruct(t *testing.T) {
	port := int32(443)

	mock := &mockELBv2DescribeTargetHealthClient{
		output: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: []elbtypes.TargetHealthDescription{
				{
					Target: &elbtypes.TargetDescription{
						Id:               aws.String("i-raw-struct-test"),
						Port:             &port,
						AvailabilityZone: aws.String("eu-west-1a"),
					},
					TargetHealth: &elbtypes.TargetHealth{
						State:       elbtypes.TargetHealthStateEnumUnhealthy,
						Reason:      elbtypes.TargetHealthReasonEnumResponseCodeMismatch,
						Description: aws.String("Health check returned 502"),
					},
					HealthCheckPort: aws.String("443"),
				},
			},
		},
	}

	resources, err := awsclient.FetchTargetHealth(
		context.Background(),
		mock,
		"arn:aws:elasticloadbalancing:eu-west-1:123456789012:targetgroup/raw-tg/xyz",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(elbtypes.TargetHealthDescription)
	if !ok {
		t.Fatalf("RawStruct should be elbtypes.TargetHealthDescription, got %T", r.RawStruct)
	}

	t.Run("Target_preserved", func(t *testing.T) {
		if raw.Target == nil {
			t.Fatal("RawStruct.Target must not be nil")
		}
		if raw.Target.Id == nil || *raw.Target.Id != "i-raw-struct-test" {
			t.Errorf("RawStruct.Target.Id not preserved correctly")
		}
		if raw.Target.Port == nil || *raw.Target.Port != 443 {
			t.Errorf("RawStruct.Target.Port not preserved correctly")
		}
		if raw.Target.AvailabilityZone == nil || *raw.Target.AvailabilityZone != "eu-west-1a" {
			t.Errorf("RawStruct.Target.AvailabilityZone not preserved correctly")
		}
	})

	t.Run("TargetHealth_preserved", func(t *testing.T) {
		if raw.TargetHealth == nil {
			t.Fatal("RawStruct.TargetHealth must not be nil")
		}
		if raw.TargetHealth.State != elbtypes.TargetHealthStateEnumUnhealthy {
			t.Errorf("RawStruct.TargetHealth.State not preserved: got %q", raw.TargetHealth.State)
		}
		if raw.TargetHealth.Reason != elbtypes.TargetHealthReasonEnumResponseCodeMismatch {
			t.Errorf("RawStruct.TargetHealth.Reason not preserved: got %q", raw.TargetHealth.Reason)
		}
		if raw.TargetHealth.Description == nil || *raw.TargetHealth.Description != "Health check returned 502" {
			t.Errorf("RawStruct.TargetHealth.Description not preserved correctly")
		}
	})

	t.Run("HealthCheckPort_preserved", func(t *testing.T) {
		if raw.HealthCheckPort == nil || *raw.HealthCheckPort != "443" {
			t.Errorf("RawStruct.HealthCheckPort not preserved correctly")
		}
	})
}

// TestTargetHealthColumns verifies that TargetHealthColumns returns the expected
// 6 columns: target_id, port, az, health, reason, description.
func TestTargetHealthColumns(t *testing.T) {
	cols := resource.TargetHealthColumns()

	expectedKeys := []string{"target_id", "port", "az", "health", "reason", "description"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 6 {
			t.Fatalf("expected 6 columns, got %d", len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("columns_have_titles", func(t *testing.T) {
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("column[%d] (%s) has empty Title", i, col.Key)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})
}
