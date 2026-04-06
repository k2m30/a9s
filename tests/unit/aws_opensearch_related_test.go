package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func opensearchCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("opensearch") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("opensearch related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("opensearch related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_OpenSearch_None(t *testing.T) {
	nav := resource.IsFieldNavigable("opensearch", "DomainName")
	if nav != nil {
		t.Errorf("expected no navigable fields for opensearch, but DomainName resolved to %v", nav)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, DomainName dimension) ---

func TestRelated_OpenSearch_Alarms_Found(t *testing.T) {
	const domainName = "acme-logs"

	alarmRes := resource.Resource{
		ID: "opensearch-cluster-status-red",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("opensearch-cluster-status-red"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DomainName"), Value: aws.String(domainName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   domainName,
		Name: domainName,
		Fields: map[string]string{
			"domain_name": domainName,
		},
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String(domainName),
		},
	}

	checker := opensearchCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "opensearch-cluster-status-red" {
		t.Errorf("ResourceIDs = %v, want [opensearch-cluster-status-red]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_OpenSearch_Alarms_NotFound(t *testing.T) {
	const domainName = "acme-logs"

	alarmRes := resource.Resource{
		ID: "other-domain-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-domain-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DomainName"), Value: aws.String("different-domain")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   domainName,
		Name: domainName,
		Fields: map[string]string{
			"domain_name": domainName,
		},
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String(domainName),
		},
	}

	checker := opensearchCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_OpenSearch_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-logs",
		Name: "acme-logs",
		Fields: map[string]string{
			"domain_name": "acme-logs",
		},
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
		},
	}

	checker := opensearchCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- opensearch→cfn: undeterminable from cache, returns Count: 0 ---

func TestRelated_OpenSearch_CFN_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-logs",
		Name: "acme-logs",
	}
	checker := opensearchCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_OpenSearch_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("opensearch")
	if checker == nil {
		t.Fatal("no demo checker registered for opensearch")
	}

	results := checker(resource.Resource{ID: "acme-logs"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"alarm": false, "cfn": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}
}
