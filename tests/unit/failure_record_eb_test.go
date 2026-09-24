package unit_test

// failure_record_eb_test.go — a failure record says which call failed,
// on what, and why.
//
// Lives beside the Elastic Beanstalk related fakes, which are in this
// package; the other failure-record pins are in failure_record_test.go.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestRelated_Eb_TG_TimeoutOnLoadBalancer_NamesTheCall: the environment's
// only load balancer could not be resolved, so nothing was read and the row is
// unknown. The failure travels beside the answer and says which call failed,
// on which load balancer, and why.
func TestRelated_Eb_TG_TimeoutOnLoadBalancer_NamesTheCall(t *testing.T) {
	const envName = "acme-web-env"
	const lbName = "awseb-AWSEBLB-ABCDEF123456"

	fakeELBv2 := &fakeELBv2ForEB{
		describeLoadBalancersFn: func(_ *elbv2.DescribeLoadBalancersInput) (*elbv2.DescribeLoadBalancersOutput, error) {
			return nil, &smithy.OperationError{
				ServiceID: "Elastic Load Balancing v2", OperationName: "DescribeLoadBalancers",
				Err: context.DeadlineExceeded,
			}
		},
	}
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: newFakeEBWithEnvironmentResources(ebtypes.EnvironmentResourceDescription{
			EnvironmentName: aws.String(envName),
			LoadBalancers:   []ebtypes.LoadBalancer{{Name: aws.String(lbName)}},
		}),
		ELBv2: fakeELBv2,
	}
	res := resource.Resource{ID: envName, Name: envName, Fields: map[string]string{},
		RawStruct: ebtypes.EnvironmentDescription{EnvironmentName: aws.String(envName)}}

	result := ebCheckerByTarget(t, "tg")(context.Background(), clients, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("state = %s, want unknown: the only load balancer was not read", result.State())
	}
	if result.Failure() == nil {
		t.Fatal("a timed-out load balancer read carried no failure")
	}
	for _, want := range []string{"DescribeLoadBalancers", "timeout", lbName} {
		if !strings.Contains(result.Failure().Error(), want) {
			t.Errorf("failure line %q does not carry %q", result.Failure(), want)
		}
	}
}

// A denial's cause already names the action, so the operation is not
// prepended a second time.
func TestFailedCall_DeniedAction_NamesTheOperationOnce(t *testing.T) {
	denied := &smithy.OperationError{
		ServiceID: "Elastic Load Balancing v2", OperationName: "DescribeListeners",
		Err: &smithy.GenericAPIError{Code: "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/a9s is not authorized to perform: elasticloadbalancing:DescribeListeners"},
	}
	f := awsclient.FailedCall("awseb-AWSEBLB-ABCDEF123456", denied)
	if n := strings.Count(f.Cause, "DescribeListeners"); n != 1 {
		t.Errorf("cause %q names the operation %d times, want 1", f.Cause, n)
	}
}
