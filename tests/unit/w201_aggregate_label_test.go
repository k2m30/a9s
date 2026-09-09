package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// An inner pass's aggregate keeps its own label when the enricher returns: the
// definition read failed, and the row says so, not that the task describe did.
func TestECSTaskAggregate_NamesTheCallThatFailed(t *testing.T) {
	const badARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/acme-unreadable:1"
	const badID = "0aaaa1111bbbb2222cccc3333dddd4459"
	fake := &pw1ECSTaskFake{
		tasks:  map[string]ecstypes.Task{badID: pw1Task(badID, badARN)},
		defs:   map[string]ecstypes.TaskDefinition{},
		defErr: map[string]error{badARN: errors.New("AccessDeniedException: ecs:DescribeTaskDefinition")},
	}
	res, err := awsclient.EnrichECSTasks(context.Background(), &awsclient.ServiceClients{ECS: fake}, []resource.Resource{pw1ECSTaskResource(badID, badARN)}, nil)
	if err == nil {
		t.Fatal("expected the refused definition read to surface as an error")
	}
	if !strings.HasPrefix(err.Error(), "DescribeTaskDefinition failed") {
		t.Errorf("error = %q, want it to open with the call that failed, DescribeTaskDefinition", err.Error())
	}
	if strings.Contains(err.Error(), "DescribeTasks failed") {
		t.Errorf("error = %q claims DescribeTasks failed; it succeeded", err.Error())
	}
	if !res.Truncated {
		t.Error("a refused definition read still leaves the issue count a lower bound")
	}
}
