package unit

// aws_r53_notfound_classification_test.go — a hosted zone
// deleted between ListHostedZones and enrichment must not spam the r53
// failure aggregate. GetHostedZone on a deleted zone can surface either as
// the typed *route53/types.NoSuchHostedZone error or as the generic
// smithy.GenericAPIError{Code: "NoSuchHostedZone"} shape (depending on
// whether the SDK's response deserializer matched a modeled shape) — both
// must classify as a silent truncation: TruncatedIDs[id]=true, no aggregate
// entry, composite error nil when it is the only failure. A different error
// code (e.g. AccessDenied) must still aggregate exactly as before.
//
// Reuses r53GetHostedZoneFake / r53ZoneResources / r53ZoneID1 / r53ZoneID2
// from aws_r53_enricher_test.go (same package unit).

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// TestEnrichRoute53Zone_TypedNoSuchHostedZone_SilentTruncation pins the
// typed-error shape: GetHostedZone returns *route53types.NoSuchHostedZone.
func TestEnrichRoute53Zone_TypedNoSuchHostedZone_SilentTruncation(t *testing.T) {
	fake := &r53GetHostedZoneFake{
		errByID: map[string]error{
			r53ZoneID1: &r53types.NoSuchHostedZone{Message: aws.String("No hosted zone found with ID: " + r53ZoneID1)},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fake}
	resources := r53ZoneResources(r53ZoneID1)

	result, err := awsclient.EnrichRoute53Zone(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("composite error must be nil when NoSuchHostedZone is the only failure; got %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for a deleted zone, got %d: %v", len(result.Findings), result.Findings)
	}
	if _, marked := result.TruncatedIDs[r53ZoneID1]; !marked {
		t.Errorf("TruncatedIDs[%q] must be true — zone deleted between list and enrichment", r53ZoneID1)
	}
}

// TestEnrichRoute53Zone_GenericNoSuchHostedZone_SilentTruncation pins the
// alternate shape: GetHostedZone returns a smithy.GenericAPIError carrying
// the same Code, rather than the typed route53types.NoSuchHostedZone.
func TestEnrichRoute53Zone_GenericNoSuchHostedZone_SilentTruncation(t *testing.T) {
	fake := &r53GetHostedZoneFake{
		errByID: map[string]error{
			r53ZoneID1: &smithy.GenericAPIError{Code: "NoSuchHostedZone", Message: "No hosted zone found"},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fake}
	resources := r53ZoneResources(r53ZoneID1)

	result, err := awsclient.EnrichRoute53Zone(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("composite error must be nil when NoSuchHostedZone is the only failure; got %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for a deleted zone, got %d: %v", len(result.Findings), result.Findings)
	}
	if _, marked := result.TruncatedIDs[r53ZoneID1]; !marked {
		t.Errorf("TruncatedIDs[%q] must be true — zone deleted between list and enrichment", r53ZoneID1)
	}
}

// TestEnrichRoute53Zone_AccessDenied_StillAggregates is the negative-space
// guard: a different error code must not be swallowed by the new
// NoSuchHostedZone silent-truncation branch.
func TestEnrichRoute53Zone_AccessDenied_StillAggregates(t *testing.T) {
	fake := &r53GetHostedZoneFake{
		errByID: map[string]error{
			r53ZoneID1: &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized to perform route53:GetHostedZone"},
		},
	}
	clients := &awsclient.ServiceClients{Route53: fake}
	resources := r53ZoneResources(r53ZoneID1)

	result, err := awsclient.EnrichRoute53Zone(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("expected non-nil composite error for AccessDenied; NoSuchHostedZone silent-truncation must not swallow other errors")
	}
	// The aggregate names the call, not the type: the type comes from the
	// registry key at the surface, and a type in the label would render it
	// twice ("enrich r53: r53-enrich: ...").
	if !strings.Contains(err.Error(), "GetHostedZone") {
		t.Errorf("composite error must name the call, GetHostedZone, got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), r53ZoneID1) {
		t.Errorf("composite error must name the failing zone %q, got: %q", r53ZoneID1, err.Error())
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on AccessDenied, got %d", len(result.Findings))
	}
	if _, marked := result.TruncatedIDs[r53ZoneID1]; !marked {
		t.Errorf("TruncatedIDs[%q] must be true", r53ZoneID1)
	}
}
