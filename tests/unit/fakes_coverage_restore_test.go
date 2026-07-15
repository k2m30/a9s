// fakes_coverage_restore_test.go contains fake AWS service client implementations
// used by the coverage-restoration tests across athena, eventbridge_rule, backup,
// redshift, tg, and pipeline related checkers.
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// ---------------------------------------------------------------------------
// fakeAthenaCR — implements AthenaAPI (ListWorkGroups + GetWorkGroup)
// Used by: athena coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeAthenaCR struct {
	workGroupOutput *athena.GetWorkGroupOutput
	err             error
}

func (f *fakeAthenaCR) ListWorkGroups(_ context.Context, _ *athena.ListWorkGroupsInput, _ ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error) {
	return &athena.ListWorkGroupsOutput{}, nil
}

func (f *fakeAthenaCR) GetWorkGroup(_ context.Context, _ *athena.GetWorkGroupInput, _ ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.workGroupOutput != nil {
		return f.workGroupOutput, nil
	}
	return &athena.GetWorkGroupOutput{}, nil
}

var _ awsclient.AthenaAPI = (*fakeAthenaCR)(nil)

// newFakeAthenaWithS3URI returns a fakeAthenaCR whose GetWorkGroup response
// carries an OutputLocation pointing at the given s3URI.
func newFakeAthenaWithS3URI(s3URI string) *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{
					ResultConfiguration: &athenatypes.ResultConfiguration{
						OutputLocation: aws.String(s3URI),
					},
				},
			},
		},
	}
}

// newFakeAthenaWithKMSKey returns a fakeAthenaCR whose GetWorkGroup response
// carries a KMS key ARN in the EncryptionConfiguration.
func newFakeAthenaWithKMSKey(kmsKeyARN string) *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{
					ResultConfiguration: &athenatypes.ResultConfiguration{
						EncryptionConfiguration: &athenatypes.EncryptionConfiguration{
							KmsKey: aws.String(kmsKeyARN),
						},
					},
				},
			},
		},
	}
}

// newFakeAthenaWithCWLogsEnabled returns a fakeAthenaCR whose GetWorkGroup response
// has PublishCloudWatchMetricsEnabled=true.
func newFakeAthenaWithCWLogsEnabled() *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{
					PublishCloudWatchMetricsEnabled: aws.Bool(true),
				},
			},
		},
	}
}

// newFakeAthenaWithExecutionRole returns a fakeAthenaCR whose GetWorkGroup response
// carries an ExecutionRole ARN.
func newFakeAthenaWithExecutionRole(roleARN string) *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{
					ExecutionRole: aws.String(roleARN),
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// fakeEventBridgeCR — implements EventBridgeAPI with controllable ListTargetsByRule.
// Used by: eventbridge_rule coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeEventBridgeCR struct {
	targets []eventbridgetypes.Target
	err     error
}

func (f *fakeEventBridgeCR) ListRules(_ context.Context, _ *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	return &eventbridge.ListRulesOutput{}, nil
}

func (f *fakeEventBridgeCR) ListTargetsByRule(_ context.Context, _ *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &eventbridge.ListTargetsByRuleOutput{Targets: f.targets}, nil
}

func (f *fakeEventBridgeCR) ListRuleNamesByTarget(_ context.Context, _ *eventbridge.ListRuleNamesByTargetInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error) {
	return &eventbridge.ListRuleNamesByTargetOutput{}, nil
}

var _ awsclient.EventBridgeAPI = (*fakeEventBridgeCR)(nil)

// ---------------------------------------------------------------------------
// fakeRedshiftCR — implements RedshiftAPI (DescribeClusters, DescribeLoggingStatus,
// DescribeClusterSubnetGroups).
// Used by: redshift coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeRedshiftCR struct {
	loggingOutput *redshift.DescribeLoggingStatusOutput
	loggingErr    error
	subnetOutput  *redshift.DescribeClusterSubnetGroupsOutput
	subnetErr     error
}

func (f *fakeRedshiftCR) DescribeClusters(_ context.Context, _ *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	return &redshift.DescribeClustersOutput{}, nil
}

func (f *fakeRedshiftCR) DescribeLoggingStatus(_ context.Context, _ *redshift.DescribeLoggingStatusInput, _ ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error) {
	if f.loggingErr != nil {
		return nil, f.loggingErr
	}
	if f.loggingOutput != nil {
		return f.loggingOutput, nil
	}
	return &redshift.DescribeLoggingStatusOutput{}, nil
}

func (f *fakeRedshiftCR) DescribeClusterSubnetGroups(_ context.Context, _ *redshift.DescribeClusterSubnetGroupsInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterSubnetGroupsOutput, error) {
	if f.subnetErr != nil {
		return nil, f.subnetErr
	}
	if f.subnetOutput != nil {
		return f.subnetOutput, nil
	}
	return &redshift.DescribeClusterSubnetGroupsOutput{}, nil
}

var _ awsclient.RedshiftAPI = (*fakeRedshiftCR)(nil)

// ---------------------------------------------------------------------------
// fakeELBv2CR — implements ELBv2API + ELBv2DescribeTagsAPI for TG checker tests.
// Used by: tg coverage-restore tests.
// ---------------------------------------------------------------------------

type fakeELBv2CR struct {
	describeTagsOutput         *elbv2.DescribeTagsOutput
	describeTagsErr            error
	describeTargetHealthOutput *elbv2.DescribeTargetHealthOutput
	describeTargetHealthErr    error
}

func (f *fakeELBv2CR) DescribeLoadBalancers(_ context.Context, _ *elbv2.DescribeLoadBalancersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	return &elbv2.DescribeLoadBalancersOutput{}, nil
}

func (f *fakeELBv2CR) DescribeTargetGroups(_ context.Context, _ *elbv2.DescribeTargetGroupsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	return &elbv2.DescribeTargetGroupsOutput{}, nil
}

func (f *fakeELBv2CR) DescribeTargetHealth(_ context.Context, _ *elbv2.DescribeTargetHealthInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	if f.describeTargetHealthErr != nil {
		return nil, f.describeTargetHealthErr
	}
	if f.describeTargetHealthOutput != nil {
		return f.describeTargetHealthOutput, nil
	}
	return &elbv2.DescribeTargetHealthOutput{}, nil
}

func (f *fakeELBv2CR) DescribeListeners(_ context.Context, _ *elbv2.DescribeListenersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	return &elbv2.DescribeListenersOutput{}, nil
}

func (f *fakeELBv2CR) DescribeRules(_ context.Context, _ *elbv2.DescribeRulesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	return &elbv2.DescribeRulesOutput{}, nil
}

func (f *fakeELBv2CR) DescribeLoadBalancerAttributes(_ context.Context, _ *elbv2.DescribeLoadBalancerAttributesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error) {
	return &elbv2.DescribeLoadBalancerAttributesOutput{}, nil
}

// DescribeTags satisfies ELBv2DescribeTagsAPI (used by checkTGCFN via type assertion).
func (f *fakeELBv2CR) DescribeTags(_ context.Context, _ *elbv2.DescribeTagsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error) {
	if f.describeTagsErr != nil {
		return nil, f.describeTagsErr
	}
	if f.describeTagsOutput != nil {
		return f.describeTagsOutput, nil
	}
	return &elbv2.DescribeTagsOutput{}, nil
}

var _ awsclient.ELBv2API = (*fakeELBv2CR)(nil)

// newFakeELBv2CRWithCFNTag returns a fakeELBv2CR whose DescribeTags returns
// the aws:cloudformation:stack-name tag for the TG resource.
func newFakeELBv2CRWithCFNTag(stackName string) *fakeELBv2CR {
	return &fakeELBv2CR{
		describeTagsOutput: &elbv2.DescribeTagsOutput{
			TagDescriptions: []elbv2types.TagDescription{
				{
					Tags: []elbv2types.Tag{
						{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String(stackName)},
					},
				},
			},
		},
	}
}

// newFakeELBv2CRWithTargetHealth returns a fakeELBv2CR whose DescribeTargetHealth
// returns the given target health descriptions.
func newFakeELBv2CRWithTargetHealth(targets []elbv2types.TargetHealthDescription) *fakeELBv2CR {
	return &fakeELBv2CR{
		describeTargetHealthOutput: &elbv2.DescribeTargetHealthOutput{
			TargetHealthDescriptions: targets,
		},
	}
}

// newFakeAthenaWithEmptyConfig returns a fakeAthenaCR whose GetWorkGroup returns a
// WorkGroup with a non-nil Configuration but no OutputLocation, KMS key,
// ExecutionRole, or PublishCloudWatchMetrics. Used by "no value" branch tests that
// need cfg != nil so checkers fall through to Count=0 (not Count=-1).
func newFakeAthenaWithEmptyConfig() *fakeAthenaCR {
	return &fakeAthenaCR{
		workGroupOutput: &athena.GetWorkGroupOutput{
			WorkGroup: &athenatypes.WorkGroup{
				Configuration: &athenatypes.WorkGroupConfiguration{},
			},
		},
	}
}
