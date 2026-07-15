package unit

// aws_degraded_resource_test.go — the shared degraded-row classifier contract.
// DegradedDetails renders "details denied" for an authorization failure and
// the neutral "details unavailable" for anything else, so "you can't see it"
// (denied) and "it didn't come back" (unavailable) never collapse into one
// phrase. This classifier feeds every N+1 fetcher's degraded row across
// mwaa/transfer/lt/ddb/opensearch/eks/ng, so pinning it here covers all of
// them at the source (CodeRabbit / docs-resources honest-degradation).

import (
	"errors"
	"testing"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

func TestDegradedDetails_ClassifiesAuthVsNonAuth(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantPhrase string
	}{
		{"nil-body", nil, "details unavailable"},
		{"access-denied", &smithy.GenericAPIError{Code: "AccessDenied"}, "details denied"},
		{"access-denied-exception", &smithy.GenericAPIError{Code: "AccessDeniedException"}, "details denied"},
		// EC2 (lt's DescribeLaunchTemplateVersions) uses UnauthorizedOperation,
		// never AccessDenied — the classifier must treat it as a denial too.
		{"ec2-unauthorized-operation", &smithy.GenericAPIError{Code: "UnauthorizedOperation"}, "details denied"},
		{"not-found", &smithy.GenericAPIError{Code: "ResourceNotFoundException"}, "details unavailable"},
		{"throttling", &smithy.GenericAPIError{Code: "ThrottlingException"}, "details unavailable"},
		{"plain-error", errors.New("connection reset by peer"), "details unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := awsclient.DegradedDetails("svc", "res-1", nil, tc.err)
			if len(r.Findings) != 1 {
				t.Fatalf("want exactly 1 finding, got %+v", r.Findings)
			}
			if got := r.Findings[0].Phrase; got != tc.wantPhrase {
				t.Errorf("Findings[0].Phrase = %q, want %q", got, tc.wantPhrase)
			}
			if got := r.Fields["status"]; got != tc.wantPhrase {
				t.Errorf("Fields[\"status\"] = %q, want %q", got, tc.wantPhrase)
			}
			// A degraded row must never vanish — the id is always preserved.
			if r.ID != "res-1" {
				t.Errorf("ID = %q, want res-1", r.ID)
			}
		})
	}
}
