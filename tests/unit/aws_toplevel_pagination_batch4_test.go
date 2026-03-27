package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ===========================================================================
// 1. CloudFormation — DescribeStacks (NextToken)
// ===========================================================================

type mockCFNPaginatedClient struct {
	outputs []*cloudformation.DescribeStacksOutput
	err     error
	callIdx int
}

func (m *mockCFNPaginatedClient) DescribeStacks(
	ctx context.Context,
	params *cloudformation.DescribeStacksInput,
	optFns ...func(*cloudformation.Options),
) (*cloudformation.DescribeStacksOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &cloudformation.DescribeStacksOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchCloudFormationStacks_Pagination(t *testing.T) {
	mock := &mockCFNPaginatedClient{
		outputs: []*cloudformation.DescribeStacksOutput{
			{
				NextToken: aws.String("page2-token"),
				Stacks: []cfntypes.Stack{
					{StackName: aws.String("page1-stack-1"), StackStatus: cfntypes.StackStatusCreateComplete},
					{StackName: aws.String("page1-stack-2"), StackStatus: cfntypes.StackStatusUpdateComplete},
				},
			},
			{
				Stacks: []cfntypes.Stack{
					{StackName: aws.String("page2-stack-1"), StackStatus: cfntypes.StackStatusCreateComplete},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudFormationStacks(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-stack-1" {
			t.Errorf("expected %q, got %q", "page1-stack-1", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-stack-1" {
			t.Errorf("expected %q, got %q", "page2-stack-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 2. CloudWatch Alarms — DescribeAlarms (NextToken)
// ===========================================================================

type mockCWAlarmPaginatedClient struct {
	outputs []*cloudwatch.DescribeAlarmsOutput
	err     error
	callIdx int
}

func (m *mockCWAlarmPaginatedClient) DescribeAlarms(
	ctx context.Context,
	params *cloudwatch.DescribeAlarmsInput,
	optFns ...func(*cloudwatch.Options),
) (*cloudwatch.DescribeAlarmsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &cloudwatch.DescribeAlarmsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchCloudWatchAlarms_Pagination(t *testing.T) {
	mock := &mockCWAlarmPaginatedClient{
		outputs: []*cloudwatch.DescribeAlarmsOutput{
			{
				NextToken: aws.String("page2-token"),
				MetricAlarms: []cwtypes.MetricAlarm{
					{AlarmName: aws.String("page1-alarm-1"), StateValue: cwtypes.StateValueOk, MetricName: aws.String("CPUUtilization")},
					{AlarmName: aws.String("page1-alarm-2"), StateValue: cwtypes.StateValueAlarm, MetricName: aws.String("MemoryUsage")},
				},
			},
			{
				MetricAlarms: []cwtypes.MetricAlarm{
					{AlarmName: aws.String("page2-alarm-1"), StateValue: cwtypes.StateValueOk, MetricName: aws.String("DiskUsage")},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudWatchAlarms(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-alarm-1" {
			t.Errorf("expected %q, got %q", "page1-alarm-1", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-alarm-1" {
			t.Errorf("expected %q, got %q", "page2-alarm-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 3. Auto Scaling Groups — DescribeAutoScalingGroups (NextToken)
// ===========================================================================

type mockASGPaginatedClient struct {
	outputs []*autoscaling.DescribeAutoScalingGroupsOutput
	err     error
	callIdx int
}

func (m *mockASGPaginatedClient) DescribeAutoScalingGroups(
	ctx context.Context,
	params *autoscaling.DescribeAutoScalingGroupsInput,
	optFns ...func(*autoscaling.Options),
) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &autoscaling.DescribeAutoScalingGroupsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchAutoScalingGroups_Pagination(t *testing.T) {
	mock := &mockASGPaginatedClient{
		outputs: []*autoscaling.DescribeAutoScalingGroupsOutput{
			{
				NextToken: aws.String("page2-token"),
				AutoScalingGroups: []asgtypes.AutoScalingGroup{
					{AutoScalingGroupName: aws.String("page1-asg-1"), MinSize: aws.Int32(1), MaxSize: aws.Int32(10), DesiredCapacity: aws.Int32(3)},
					{AutoScalingGroupName: aws.String("page1-asg-2"), MinSize: aws.Int32(2), MaxSize: aws.Int32(20), DesiredCapacity: aws.Int32(5)},
				},
			},
			{
				AutoScalingGroups: []asgtypes.AutoScalingGroup{
					{AutoScalingGroupName: aws.String("page2-asg-1"), MinSize: aws.Int32(0), MaxSize: aws.Int32(5), DesiredCapacity: aws.Int32(1)},
				},
			},
		},
	}

	resources, err := awsclient.FetchAutoScalingGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-asg-1" {
			t.Errorf("expected %q, got %q", "page1-asg-1", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-asg-1" {
			t.Errorf("expected %q, got %q", "page2-asg-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 4. ACM Certificates — ListCertificates (NextToken)
// ===========================================================================

type mockACMPaginatedClient struct {
	outputs []*acm.ListCertificatesOutput
	err     error
	callIdx int
}

func (m *mockACMPaginatedClient) ListCertificates(
	ctx context.Context,
	params *acm.ListCertificatesInput,
	optFns ...func(*acm.Options),
) (*acm.ListCertificatesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &acm.ListCertificatesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchACMCertificates_Pagination(t *testing.T) {
	mock := &mockACMPaginatedClient{
		outputs: []*acm.ListCertificatesOutput{
			{
				NextToken: aws.String("page2-token"),
				CertificateSummaryList: []acmtypes.CertificateSummary{
					{DomainName: aws.String("page1-cert-1.example.com"), Status: acmtypes.CertificateStatusIssued},
					{DomainName: aws.String("page1-cert-2.example.com"), Status: acmtypes.CertificateStatusIssued},
				},
			},
			{
				CertificateSummaryList: []acmtypes.CertificateSummary{
					{DomainName: aws.String("page2-cert-1.example.com"), Status: acmtypes.CertificateStatusIssued},
				},
			},
		},
	}

	resources, err := awsclient.FetchACMCertificates(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-cert-1.example.com" {
			t.Errorf("expected %q, got %q", "page1-cert-1.example.com", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-cert-1.example.com" {
			t.Errorf("expected %q, got %q", "page2-cert-1.example.com", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 5. ECR Repositories — DescribeRepositories (NextToken)
// ===========================================================================

type mockECRPaginatedClient struct {
	outputs []*ecr.DescribeRepositoriesOutput
	err     error
	callIdx int
}

func (m *mockECRPaginatedClient) DescribeRepositories(
	ctx context.Context,
	params *ecr.DescribeRepositoriesInput,
	optFns ...func(*ecr.Options),
) (*ecr.DescribeRepositoriesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ecr.DescribeRepositoriesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchECRRepositories_Pagination(t *testing.T) {
	mock := &mockECRPaginatedClient{
		outputs: []*ecr.DescribeRepositoriesOutput{
			{
				NextToken: aws.String("page2-token"),
				Repositories: []ecrtypes.Repository{
					{RepositoryName: aws.String("page1-repo-1"), RepositoryUri: aws.String("111111111111.dkr.ecr.us-east-1.amazonaws.com/page1-repo-1")},
					{RepositoryName: aws.String("page1-repo-2"), RepositoryUri: aws.String("111111111111.dkr.ecr.us-east-1.amazonaws.com/page1-repo-2")},
				},
			},
			{
				Repositories: []ecrtypes.Repository{
					{RepositoryName: aws.String("page2-repo-1"), RepositoryUri: aws.String("111111111111.dkr.ecr.us-east-1.amazonaws.com/page2-repo-1")},
				},
			},
		},
	}

	resources, err := awsclient.FetchECRRepositories(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-repo-1" {
			t.Errorf("expected %q, got %q", "page1-repo-1", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-repo-1" {
			t.Errorf("expected %q, got %q", "page2-repo-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 6. EventBridge Rules — ListRules (NextToken)
// ===========================================================================

type mockEBRulePaginatedClient struct {
	outputs []*eventbridge.ListRulesOutput
	err     error
	callIdx int
}

func (m *mockEBRulePaginatedClient) ListRules(
	ctx context.Context,
	params *eventbridge.ListRulesInput,
	optFns ...func(*eventbridge.Options),
) (*eventbridge.ListRulesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &eventbridge.ListRulesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchEventBridgeRules_Pagination(t *testing.T) {
	mock := &mockEBRulePaginatedClient{
		outputs: []*eventbridge.ListRulesOutput{
			{
				NextToken: aws.String("page2-token"),
				Rules: []ebtypes.Rule{
					{Name: aws.String("page1-rule-1"), State: ebtypes.RuleStateEnabled, EventBusName: aws.String("default")},
					{Name: aws.String("page1-rule-2"), State: ebtypes.RuleStateDisabled, EventBusName: aws.String("custom-bus")},
				},
			},
			{
				Rules: []ebtypes.Rule{
					{Name: aws.String("page2-rule-1"), State: ebtypes.RuleStateEnabled, EventBusName: aws.String("default")},
				},
			},
		},
	}

	resources, err := awsclient.FetchEventBridgeRules(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-rule-1" {
			t.Errorf("expected %q, got %q", "page1-rule-1", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-rule-1" {
			t.Errorf("expected %q, got %q", "page2-rule-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 7. Secrets Manager — ListSecrets (NextToken)
// ===========================================================================

type mockSecretsPaginatedClient struct {
	outputs []*secretsmanager.ListSecretsOutput
	err     error
	callIdx int
}

func (m *mockSecretsPaginatedClient) ListSecrets(
	ctx context.Context,
	params *secretsmanager.ListSecretsInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.ListSecretsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &secretsmanager.ListSecretsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchSecrets_Pagination(t *testing.T) {
	mock := &mockSecretsPaginatedClient{
		outputs: []*secretsmanager.ListSecretsOutput{
			{
				NextToken: aws.String("page2-token"),
				SecretList: []smtypes.SecretListEntry{
					{Name: aws.String("page1-secret-1"), Description: aws.String("DB password")},
					{Name: aws.String("page1-secret-2"), Description: aws.String("API key")},
				},
			},
			{
				SecretList: []smtypes.SecretListEntry{
					{Name: aws.String("page2-secret-1"), Description: aws.String("SSH key")},
				},
			},
		},
	}

	resources, err := awsclient.FetchSecrets(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-secret-1" {
			t.Errorf("expected %q, got %q", "page1-secret-1", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-secret-1" {
			t.Errorf("expected %q, got %q", "page2-secret-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 8. Step Functions — ListStateMachines (NextToken)
// ===========================================================================

type mockSFNPaginatedClient struct {
	outputs []*sfn.ListStateMachinesOutput
	err     error
	callIdx int
}

func (m *mockSFNPaginatedClient) ListStateMachines(
	ctx context.Context,
	params *sfn.ListStateMachinesInput,
	optFns ...func(*sfn.Options),
) (*sfn.ListStateMachinesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &sfn.ListStateMachinesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchStepFunctions_Pagination(t *testing.T) {
	mock := &mockSFNPaginatedClient{
		outputs: []*sfn.ListStateMachinesOutput{
			{
				NextToken: aws.String("page2-token"),
				StateMachines: []sfntypes.StateMachineListItem{
					{Name: aws.String("page1-sfn-1"), StateMachineArn: aws.String("arn:aws:states:us-east-1:111111111111:stateMachine:page1-sfn-1"), Type: sfntypes.StateMachineTypeStandard},
					{Name: aws.String("page1-sfn-2"), StateMachineArn: aws.String("arn:aws:states:us-east-1:111111111111:stateMachine:page1-sfn-2"), Type: sfntypes.StateMachineTypeExpress},
				},
			},
			{
				StateMachines: []sfntypes.StateMachineListItem{
					{Name: aws.String("page2-sfn-1"), StateMachineArn: aws.String("arn:aws:states:us-east-1:111111111111:stateMachine:page2-sfn-1"), Type: sfntypes.StateMachineTypeStandard},
				},
			},
		},
	}

	resources, err := awsclient.FetchStepFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-sfn-1" {
			t.Errorf("expected %q, got %q", "page1-sfn-1", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-sfn-1" {
			t.Errorf("expected %q, got %q", "page2-sfn-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 9. SSM Parameters — DescribeParameters (NextToken)
// ===========================================================================

type mockSSMPaginatedClient struct {
	outputs []*ssm.DescribeParametersOutput
	err     error
	callIdx int
}

func (m *mockSSMPaginatedClient) DescribeParameters(
	ctx context.Context,
	params *ssm.DescribeParametersInput,
	optFns ...func(*ssm.Options),
) (*ssm.DescribeParametersOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ssm.DescribeParametersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchSSMParameters_Pagination(t *testing.T) {
	mock := &mockSSMPaginatedClient{
		outputs: []*ssm.DescribeParametersOutput{
			{
				NextToken: aws.String("page2-token"),
				Parameters: []ssmtypes.ParameterMetadata{
					{Name: aws.String("page1-param-1"), Type: ssmtypes.ParameterTypeString, Description: aws.String("Database host")},
					{Name: aws.String("page1-param-2"), Type: ssmtypes.ParameterTypeString, Description: aws.String("API endpoint")},
				},
			},
			{
				Parameters: []ssmtypes.ParameterMetadata{
					{Name: aws.String("page2-param-1"), Type: ssmtypes.ParameterTypeStringList, Description: aws.String("Allowed CIDRs")},
				},
			},
		},
	}

	resources, err := awsclient.FetchSSMParameters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-param-1" {
			t.Errorf("expected %q, got %q", "page1-param-1", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-param-1" {
			t.Errorf("expected %q, got %q", "page2-param-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 10. Route53 Hosted Zones — ListHostedZones (Marker/IsTruncated/NextMarker)
// ===========================================================================

type mockR53PaginatedClient struct {
	outputs []*route53.ListHostedZonesOutput
	err     error
	callIdx int
}

func (m *mockR53PaginatedClient) ListHostedZones(
	ctx context.Context,
	params *route53.ListHostedZonesInput,
	optFns ...func(*route53.Options),
) (*route53.ListHostedZonesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &route53.ListHostedZonesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchHostedZones_Pagination(t *testing.T) {
	mock := &mockR53PaginatedClient{
		outputs: []*route53.ListHostedZonesOutput{
			{
				IsTruncated: true,
				NextMarker:  aws.String("page2-marker"),
				HostedZones: []r53types.HostedZone{
					{
						Id:                     aws.String("/hostedzone/Z1PAGE1001"),
						Name:                   aws.String("page1-zone1.example.com."),
						ResourceRecordSetCount: aws.Int64(10),
						Config:                 &r53types.HostedZoneConfig{PrivateZone: false, Comment: aws.String("Public zone")},
					},
					{
						Id:                     aws.String("/hostedzone/Z1PAGE1002"),
						Name:                   aws.String("page1-zone2.internal."),
						ResourceRecordSetCount: aws.Int64(5),
						Config:                 &r53types.HostedZoneConfig{PrivateZone: true},
					},
				},
			},
			{
				IsTruncated: false,
				HostedZones: []r53types.HostedZone{
					{
						Id:                     aws.String("/hostedzone/Z2PAGE2001"),
						Name:                   aws.String("page2-zone1.example.com."),
						ResourceRecordSetCount: aws.Int64(25),
						Config:                 &r53types.HostedZoneConfig{PrivateZone: false},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchHostedZones(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "/hostedzone/Z1PAGE1001" {
			t.Errorf("expected %q, got %q", "/hostedzone/Z1PAGE1001", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "/hostedzone/Z2PAGE2001" {
			t.Errorf("expected %q, got %q", "/hostedzone/Z2PAGE2001", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// 11. CloudFront Distributions — ListDistributions (Marker/NextMarker/IsTruncated)
// ===========================================================================

type mockCloudFrontPaginatedClient struct {
	outputs []*cloudfront.ListDistributionsOutput
	err     error
	callIdx int
}

func (m *mockCloudFrontPaginatedClient) ListDistributions(
	ctx context.Context,
	params *cloudfront.ListDistributionsInput,
	optFns ...func(*cloudfront.Options),
) (*cloudfront.ListDistributionsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &cloudfront.ListDistributionsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchCloudFrontDistributions_Pagination(t *testing.T) {
	mock := &mockCloudFrontPaginatedClient{
		outputs: []*cloudfront.ListDistributionsOutput{
			{
				DistributionList: &cftypes.DistributionList{
					IsTruncated: aws.Bool(true),
					NextMarker:  aws.String("page2-marker"),
					Marker:      aws.String(""),
					MaxItems:    aws.Int32(100),
					Quantity:    aws.Int32(2),
					Items: []cftypes.DistributionSummary{
						{
							Id:         aws.String("EDFDVBD111111"),
							DomainName: aws.String("d111111.cloudfront.net"),
							Status:     aws.String("Deployed"),
							Enabled:    aws.Bool(true),
						},
						{
							Id:         aws.String("EDFDVBD222222"),
							DomainName: aws.String("d222222.cloudfront.net"),
							Status:     aws.String("Deployed"),
							Enabled:    aws.Bool(true),
						},
					},
				},
			},
			{
				DistributionList: &cftypes.DistributionList{
					IsTruncated: aws.Bool(false),
					Marker:      aws.String("page2-marker"),
					MaxItems:    aws.Int32(100),
					Quantity:    aws.Int32(1),
					Items: []cftypes.DistributionSummary{
						{
							Id:         aws.String("EDFDVBD333333"),
							DomainName: aws.String("d333333.cloudfront.net"),
							Status:     aws.String("InProgress"),
							Enabled:    aws.Bool(false),
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudFrontDistributions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "EDFDVBD111111" {
			t.Errorf("expected %q, got %q", "EDFDVBD111111", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "EDFDVBD333333" {
			t.Errorf("expected %q, got %q", "EDFDVBD333333", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ===========================================================================
// Suppress unused import warnings — fmt is used in error messages
// ===========================================================================
var _ = fmt.Sprintf
