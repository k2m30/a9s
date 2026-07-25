package unit_test

// aws_transfer_related_test.go — related-resource checker tests for transfer
// (docs/resources/transfer.md §2, docs/resources/transfer-impl-plan.md §1
// "related_targets"). Checkers live in core/aws/transfer_related.go.
//
// All 8 real pivots (acm, eip, lambda, logs, role, subnet, vpc, vpce) are
// field-driven (read a field on the DescribedServer, no API call) — these
// are exercised against the REAL FetchTransferServersPage output for the
// relevant demo fixtures, avoiding any guess at internal Fields/RawStruct
// shape (mirrors TestRelated_MWAA_GraphRootCounts). ct-events is the
// universal ctEventsCheckerFor("transfer") pivot. sg/apigw/s3/efs are
// explicitly excluded per docs/resources/transfer.md §2.
//
// checkerByTarget is shared package-scope test tooling, defined in
// aws_iam_policies_related_test.go.

import (
	"context"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// transferResourceByID fetches the real demo page via FetchTransferServersPage
// and returns the Resource with the given id, failing the test if absent.
func transferResourceByID(t *testing.T, id string) resource.Resource {
	t.Helper()
	clients := &awsclient.ServiceClients{Transfer: fakes.NewTransfer()}
	result, err := awsclient.FetchTransferServersPage(context.Background(), clients, "")
	if err == nil || !strings.Contains(err.Error(), fixtures.WarnTransferDetailsDeniedID) {
		// The demo set includes the details-denied witness, so the fetch
		// must legitimately return rows + a composite error naming that
		// witness (E5 partial success) — any other outcome is unexpected.
		t.Fatalf("expected the details-denied composite error naming %q, got %v", fixtures.WarnTransferDetailsDeniedID, err)
	}
	for _, r := range result.Resources {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("resource %q not found in fetch result", id)
	return resource.Resource{}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestRelated_Transfer_Registered(t *testing.T) {
	defs := resource.GetRelated("transfer")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for transfer")
	}

	expected := map[string]string{
		"acm":       "ACM Certificates",
		"eip":       "Elastic IPs",
		"lambda":    "Lambda Functions",
		"logs":      "Log Groups",
		"role":      "IAM Roles",
		"subnet":    "Subnets",
		"vpc":       "VPC",
		"vpce":      "VPC Endpoints",
		"ct-events": "CloudTrail Events",
	}
	if len(defs) != len(expected) {
		t.Errorf("transfer: len(GetRelated) = %d, want exactly %d (spec §2's excluded targets — sg/apigw/s3/efs — must not sneak in as extra registrations)", len(defs), len(expected))
	}
	seen := map[string]bool{}
	for _, def := range defs {
		wantDisplay, ok := expected[def.TargetType]
		if !ok {
			continue
		}
		seen[def.TargetType] = true
		if def.Checker == nil {
			t.Errorf("transfer %q: Checker should not be nil", def.TargetType)
		}
		if def.DisplayName != wantDisplay {
			t.Errorf("transfer %q: DisplayName = %q, want %q", def.TargetType, def.DisplayName, wantDisplay)
		}
	}
	for target := range expected {
		if !seen[target] {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func TestRelated_Transfer_ExcludedTargetsNotRegistered(t *testing.T) {
	defs := resource.GetRelated("transfer")
	for _, excluded := range []string{"sg", "apigw", "s3", "efs"} {
		for _, def := range defs {
			if def.TargetType == excluded {
				t.Errorf("transfer: target %q must not be registered (spec §2 explicitly excludes it)", excluded)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Field-driven checkers — graph root (prod-as2-gateway) counts per
// transfer-impl-plan.md §1 "related_targets": role 1, vpc 1, subnet 3,
// vpce 1, logs 2, acm 1, eip 3.
// ---------------------------------------------------------------------------

func TestRelated_Transfer_GraphRootCounts(t *testing.T) {
	res := transferResourceByID(t, fixtures.ProdAS2GatewayID)
	for _, tc := range []struct {
		target string
		want   int
		field  string
	}{
		{"role", 1, "LoggingRole"},
		{"vpc", 1, "EndpointDetails.VpcId"},
		{"subnet", 3, "EndpointDetails.SubnetIds"},
		{"vpce", 1, "EndpointDetails.VpcEndpointId"},
		{"logs", 2, "StructuredLogDestinations"},
		{"acm", 1, "Certificate"},
		{"eip", 3, "EndpointDetails.AddressAllocationIds"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			checker := checkerByTarget(t, "transfer", tc.target)
			result := checker(context.Background(), nil, res, resource.ResourceCache{})
			if result.Count() != tc.want {
				t.Errorf("Count = %d, want %d (%s)", result.Count(), tc.want, tc.field)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// lambda — custom-authorizer pivot, only present on the AWS_LAMBDA IdP
// fixture (sftp-lambda-auth), not on the graph root.
// ---------------------------------------------------------------------------

func TestRelated_Transfer_LambdaOnAuthFixture(t *testing.T) {
	res := transferResourceByID(t, fixtures.SftpLambdaAuthID)
	checker := checkerByTarget(t, "transfer", "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (IdentityProviderDetails.Function)", result.Count())
	}
}

// ---------------------------------------------------------------------------
// Conditional pivots absent cleanly — a PUBLIC endpoint (no EndpointDetails,
// no Certificate, no Lambda IdP) must resolve those pivots to 0, not error
// or panic. role stays 1 (LoggingRole is present regardless of endpoint
// type) as a positive control that the checkers are actually running.
// ---------------------------------------------------------------------------

func TestRelated_Transfer_PublicEndpointConditionalPivotsAbsent(t *testing.T) {
	res := transferResourceByID(t, fixtures.SftpUsersProdID)
	for _, tc := range []struct {
		target string
		want   int
	}{
		{"vpc", 0},
		{"subnet", 0},
		{"vpce", 0},
		{"acm", 0},
		{"lambda", 0},
		{"logs", 0},
		{"role", 1},
		{"eip", 0},
	} {
		t.Run(tc.target, func(t *testing.T) {
			checker := checkerByTarget(t, "transfer", tc.target)
			result := checker(context.Background(), nil, res, resource.ResourceCache{})
			if result.Count() != tc.want {
				t.Errorf("Count = %d, want %d", result.Count(), tc.want)
			}
			if result.Err() != nil {
				t.Errorf("Err = %v, want nil", result.Err())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// eip — internet-facing endpoint static addresses. Only the AS2 gateway
// carries AddressAllocationIds; every other endpoint shape (PUBLIC, or VPC
// without static addresses) must resolve to a clean 0, and a degraded
// ListedServer row (no DescribeServer access) must resolve to
// RelatedUnknown, not panic.
//
// SftpUsersProdID (PUBLIC endpoint) is covered by the {"eip", 0} case in
// TestRelated_Transfer_PublicEndpointConditionalPivotsAbsent — only the VPC
// endpoint shape (SftpLambdaAuthID) is unique here.
// ---------------------------------------------------------------------------

func TestRelated_Transfer_EIP_ZeroOnFixturesWithoutAddressAllocation(t *testing.T) {
	res := transferResourceByID(t, fixtures.SftpLambdaAuthID)
	checker := checkerByTarget(t, "transfer", "eip")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no EndpointDetails.AddressAllocationIds on %s)", result.Count(), fixtures.SftpLambdaAuthID)
	}
	if result.Err() != nil {
		t.Errorf("Err = %v, want nil", result.Err())
	}
}

func TestRelated_Transfer_EIP_DegradedRowUnknown(t *testing.T) {
	res := transferResourceByID(t, fixtures.WarnTransferDetailsDeniedID)
	checker := checkerByTarget(t, "transfer", "eip")

	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (degraded ListedServer row carries no EndpointDetails field)", result.State())
	}
}

// ---------------------------------------------------------------------------
// ct-events — universal ctEventsCheckerFor("transfer") pivot: deferred,
// server-side FetchFilter, drillable.
// ---------------------------------------------------------------------------

func TestRelated_Transfer_CtEvents_Drillable(t *testing.T) {
	res := transferResourceByID(t, fixtures.ProdAS2GatewayID)
	checker := checkerByTarget(t, "transfer", "ct-events")

	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedDeferred {
		t.Errorf("State = %v, want RelatedDeferred (ct-events is a universal server-side pivot, drillable by resource name)",
			result.State())
	}
	if len(result.FetchFilter()) == 0 {
		t.Error("FetchFilter is empty, want a CloudTrail LookupEvents filter keyed on the server id")
	}
}
