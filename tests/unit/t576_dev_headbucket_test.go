package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
)

// headBucketAnswering serves every S3 call from the demo fake except
// HeadBucket, which fails with code.
type headBucketAnswering struct {
	awsclient.S3API
	code string
}

func (h headBucketAnswering) HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return nil, &smithy.OperationError{ServiceID: "S3", OperationName: "HeadBucket", Err: &smithy.GenericAPIError{Code: h.code, Fault: smithy.FaultClient}}
}

// HeadBucket proves a bucket gone only with a 404 and proves it present with
// a 403 or a redirect. Any other error, such as a 400 for an expired token,
// answers nothing: the distribution is not inspected and the failure is
// reported, never read as an origin bucket that exists.
func TestT576Dev_HeadBucketErrorThatIsNoAnswerMarksTheDistribution(t *testing.T) {
	_, rows, cache := t576Bench(t, "cf")
	fn := t576Enricher(t, "cf").Fn

	run := func(code string) awsclient.IssueEnricherResult {
		t.Helper()
		clients := demo.NewServiceClients()
		clients.S3 = headBucketAnswering{S3API: clients.S3, code: code}
		res, err := fn(context.Background(), clients, rows, cache)
		if answers := code == "Forbidden" || code == "MovedPermanently"; answers != (err == nil) {
			t.Errorf("HeadBucket %s: enricher error = %v", code, err)
		}
		return res
	}
	missing := func(res awsclient.IssueEnricherResult) bool {
		return slices.ContainsFunc(res.Findings[fixtures.CFOriginBucketMissing], func(f domain.Finding) bool {
			return f.Code == awsclient.CodeCFOriginBucketMissing
		})
	}

	for _, code := range []string{"BadRequest", "ExpiredToken", "InternalError"} {
		res := run(code)
		if check := res.TruncatedIDs[fixtures.CFOriginBucketMissing]; check != "HeadBucket" {
			t.Errorf("HeadBucket %s: not-inspected check = %q, want %q", code, check, "HeadBucket")
		}
		if missing(res) {
			t.Errorf("HeadBucket %s: origin reported missing though nothing said so", code)
		}
	}
	for _, code := range []string{"Forbidden", "MovedPermanently"} {
		res := run(code)
		if check, ok := res.TruncatedIDs[fixtures.CFOriginBucketMissing]; ok {
			t.Errorf("HeadBucket %s: the bucket exists, yet the row is marked %q", code, check)
		}
		if missing(res) {
			t.Errorf("HeadBucket %s: an existing bucket reported missing", code)
		}
	}
}
