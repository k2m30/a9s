package unit

// GetHostedZone on a zone deleted after ListHostedZones surfaces as either the
// typed *route53/types.NoSuchHostedZone or a
// smithy.GenericAPIError{Code: "NoSuchHostedZone"}, depending on whether the
// SDK's deserializer matched a modeled shape. Both are a silent truncation,
// not an aggregated failure.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

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
	// A check that issues several calls per resource names the check, never
	// one of its calls; the denial's own cause still quotes the action, which
	// is where the operator reads which call the role lacks. The type is not
	// in the label either: it comes from the registry key at the surface, and
	// a type here would render twice ("enrich r53: r53-enrich: ...").
	const r53AggregateOp = "hosted zone posture and records"
	if !strings.Contains(err.Error(), r53AggregateOp) {
		t.Errorf("composite error must name the check, %q, got: %q", r53AggregateOp, err.Error())
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
