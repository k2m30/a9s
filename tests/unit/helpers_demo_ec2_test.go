package unit

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// mustDemoEC2 fetches EC2 instances from the demo fake client.
// It fails the test immediately if the fixtures are missing or empty.
func mustDemoEC2(t *testing.T) []resource.Resource {
	t.Helper()
	ec2Client := fakes.NewEC2()
	ec2, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	return ec2
}
