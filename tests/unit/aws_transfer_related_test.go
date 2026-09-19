package unit_test

// The field-driven pivots run against the real FetchTransferServersPage output
// for the demo fixtures, so no test guesses the internal Fields/RawStruct shape.

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

func transferResourceByID(t *testing.T, id string) resource.Resource {
	t.Helper()
	clients := &awsclient.ServiceClients{Transfer: fakes.NewTransfer()}
	result, err := awsclient.FetchTransferServersPage(context.Background(), clients, "")
	if err == nil || !strings.Contains(err.Error(), fixtures.WarnTransferDetailsDeniedID) {
		// The demo set includes a server whose DescribeServer is denied, so the
		// fetch returns rows plus a composite error naming it.
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

// The lambda pivot is the custom authorizer of an AWS_LAMBDA identity provider.

func TestRelated_Transfer_LambdaOnAuthFixture(t *testing.T) {
	res := transferResourceByID(t, fixtures.SftpLambdaAuthID)
	checker := checkerByTarget(t, "transfer", "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (IdentityProviderDetails.Function)", result.Count())
	}
}

// A PUBLIC endpoint carries no EndpointDetails, Certificate or Lambda identity
// provider; LoggingRole is present on every endpoint type.

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

// AddressAllocationIds are the endpoint's static addresses; a ListedServer row
// without DescribeServer access resolves eip to unknown.

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
