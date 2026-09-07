package unit_test

// failure_record_eb_test.go — spec row 3: one aggregate covering two different
// calls says which call failed.
//
// Lives beside the Elastic Beanstalk related fakes, which are in this
// package; the rest of the "skipped" pins are in failure_record_test.go.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestRelated_Eb_TG_TimeoutOnListeners_NamesTheCall pins spec row 3: one
// aggregate covers two different calls (resolving the load balancer's name,
// then reading its listeners), so a failure whose cause does not already name
// what it was doing says which call refused. A denial names its action itself
// and is not decorated twice.
func TestRelated_Eb_TG_TimeoutOnListeners_NamesTheCall(t *testing.T) {
	const envName = "acme-web-env"
	const lbName = "awseb-AWSEBLB-ABCDEF123456"
	const lbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/awseb-AWSEBLB-ABCDEF123456/0123456789abcdef"

	fakeELBv2 := &fakeELBv2ForEB{
		describeLoadBalancersFn: func(_ *elbv2.DescribeLoadBalancersInput) (*elbv2.DescribeLoadBalancersOutput, error) {
			return &elbv2.DescribeLoadBalancersOutput{LoadBalancers: []elbv2types.LoadBalancer{
				{LoadBalancerName: aws.String(lbName), LoadBalancerArn: aws.String(lbARN)},
			}}, nil
		},
		describeListenersFn: func(_ *elbv2.DescribeListenersInput) (*elbv2.DescribeListenersOutput, error) {
			return nil, &smithy.OperationError{
				ServiceID: "Elastic Load Balancing v2", OperationName: "DescribeListeners",
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
	if result.Err() == nil {
		t.Fatal("a timed-out listener read returned no error")
	}
	for _, want := range []string{"DescribeListeners", "timeout", lbName} {
		if !strings.Contains(result.Err().Error(), want) {
			t.Errorf("failure line %q does not carry %q", result.Err(), want)
		}
	}
}

// TestFailedCall_DeniedAction_NamesTheOperationOnce pins the other half of row
// 3: a denial's cause already names the action, so the operation is not
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
