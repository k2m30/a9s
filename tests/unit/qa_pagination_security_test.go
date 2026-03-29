package unit

// qa_pagination_security_test.go — pagination tests for security fetchers:
// sg, iam-role, iam-policy, secrets, ssm

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeSecurityGroups (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeSecurityGroupsAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*ec2.DescribeSecurityGroupsOutput, error)
}

func (m *mockEC2DescribeSecurityGroupsAPIPaginated) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchSecurityGroupsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchSecurityGroupsPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSecurityGroupsOutput, error) {
			return &ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []ec2types.SecurityGroup{
					{
						GroupId:     aws.String("sg-0abc111222333444a"),
						GroupName:   aws.String("web-server-sg"),
						VpcId:       aws.String("vpc-0abc111222333444a"),
						Description: aws.String("Web server security group"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchSecurityGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "sg-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "sg-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchSecurityGroupsPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSecurityGroupsOutput, error) {
			return &ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []ec2types.SecurityGroup{
					{
						GroupId:     aws.String("sg-0xyz999888777666b"),
						GroupName:   aws.String("db-server-sg"),
						VpcId:       aws.String("vpc-0abc111222333444a"),
						Description: aws.String("Database security group"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSecurityGroupsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchSecurityGroupsPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSecurityGroupsOutput, error) {
			return &ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []ec2types.SecurityGroup{},
				NextToken:      nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSecurityGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchSecurityGroupsPage_Error(t *testing.T) {
	mock := &mockEC2DescribeSecurityGroupsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSecurityGroupsOutput, error) {
			return nil, errors.New("describe security groups failed")
		},
	}

	_, err := awsclient.FetchSecurityGroupsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: IAM ListRoles (paginated)
// ---------------------------------------------------------------------------

type mockIAMListRolesAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*iam.ListRolesOutput, error)
}

func (m *mockIAMListRolesAPIPaginated) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchIAMRolesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchIAMRolesPage_FirstPage(t *testing.T) {
	mock := &mockIAMListRolesAPIPaginated{
		PageFunc: func(_ int) (*iam.ListRolesOutput, error) {
			return &iam.ListRolesOutput{
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("my-lambda-role"),
						RoleId:   aws.String("AROA111222333444AAAAA"),
						Path:     aws.String("/"),
					},
				},
				IsTruncated: true,
				Marker:      aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchIAMRolesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with Marker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-lambda-role" {
		t.Errorf("resource ID: expected %q, got %q", "my-lambda-role", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchIAMRolesPage_Continuation(t *testing.T) {
	mock := &mockIAMListRolesAPIPaginated{
		PageFunc: func(_ int) (*iam.ListRolesOutput, error) {
			return &iam.ListRolesOutput{
				Roles: []iamtypes.Role{
					{
						RoleName: aws.String("my-ec2-role"),
						RoleId:   aws.String("AROA111222333444BBBBB"),
						Path:     aws.String("/"),
					},
				},
				IsTruncated: false,
				Marker:      nil,
			}, nil
		},
	}

	result, err := awsclient.FetchIAMRolesPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (IsTruncated=false)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchIAMRolesPage_Empty(t *testing.T) {
	mock := &mockIAMListRolesAPIPaginated{
		PageFunc: func(_ int) (*iam.ListRolesOutput, error) {
			return &iam.ListRolesOutput{
				Roles:       []iamtypes.Role{},
				IsTruncated: false,
				Marker:      nil,
			}, nil
		},
	}

	result, err := awsclient.FetchIAMRolesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchIAMRolesPage_Error(t *testing.T) {
	mock := &mockIAMListRolesAPIPaginated{
		PageFunc: func(_ int) (*iam.ListRolesOutput, error) {
			return nil, errors.New("list roles failed")
		},
	}

	_, err := awsclient.FetchIAMRolesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: IAM ListPolicies (paginated)
// ---------------------------------------------------------------------------

type mockIAMListPoliciesAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*iam.ListPoliciesOutput, error)
}

func (m *mockIAMListPoliciesAPIPaginated) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchIAMPoliciesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchIAMPoliciesPage_FirstPage(t *testing.T) {
	attachCount := int32(3)
	mock := &mockIAMListPoliciesAPIPaginated{
		PageFunc: func(_ int) (*iam.ListPoliciesOutput, error) {
			return &iam.ListPoliciesOutput{
				Policies: []iamtypes.Policy{
					{
						PolicyName:      aws.String("MyCustomPolicy"),
						PolicyId:        aws.String("ANPA111222333444AAAAA"),
						Path:            aws.String("/"),
						AttachmentCount: &attachCount,
					},
				},
				IsTruncated: true,
				Marker:      aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchIAMPoliciesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with Marker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "MyCustomPolicy" {
		t.Errorf("resource ID: expected %q, got %q", "MyCustomPolicy", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchIAMPoliciesPage_Continuation(t *testing.T) {
	attachCount := int32(1)
	mock := &mockIAMListPoliciesAPIPaginated{
		PageFunc: func(_ int) (*iam.ListPoliciesOutput, error) {
			return &iam.ListPoliciesOutput{
				Policies: []iamtypes.Policy{
					{
						PolicyName:      aws.String("AnotherPolicy"),
						PolicyId:        aws.String("ANPA111222333444BBBBB"),
						Path:            aws.String("/"),
						AttachmentCount: &attachCount,
					},
				},
				IsTruncated: false,
				Marker:      nil,
			}, nil
		},
	}

	result, err := awsclient.FetchIAMPoliciesPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (IsTruncated=false)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchIAMPoliciesPage_Empty(t *testing.T) {
	mock := &mockIAMListPoliciesAPIPaginated{
		PageFunc: func(_ int) (*iam.ListPoliciesOutput, error) {
			return &iam.ListPoliciesOutput{
				Policies:    []iamtypes.Policy{},
				IsTruncated: false,
				Marker:      nil,
			}, nil
		},
	}

	result, err := awsclient.FetchIAMPoliciesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchIAMPoliciesPage_Error(t *testing.T) {
	mock := &mockIAMListPoliciesAPIPaginated{
		PageFunc: func(_ int) (*iam.ListPoliciesOutput, error) {
			return nil, errors.New("list policies failed")
		},
	}

	_, err := awsclient.FetchIAMPoliciesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SecretsManager ListSecrets (paginated)
// ---------------------------------------------------------------------------

type mockSecretsManagerListSecretsAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*secretsmanager.ListSecretsOutput, error)
}

func (m *mockSecretsManagerListSecretsAPIPaginated) ListSecrets(_ context.Context, _ *secretsmanager.ListSecretsInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchSecretsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchSecretsPage_FirstPage(t *testing.T) {
	mock := &mockSecretsManagerListSecretsAPIPaginated{
		PageFunc: func(_ int) (*secretsmanager.ListSecretsOutput, error) {
			rotEnabled := true
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{
						Name:            aws.String("prod/db/password"),
						Description:     aws.String("Production database password"),
						RotationEnabled: &rotEnabled,
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchSecretsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "prod/db/password" {
		t.Errorf("resource ID: expected %q, got %q", "prod/db/password", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchSecretsPage_Continuation(t *testing.T) {
	mock := &mockSecretsManagerListSecretsAPIPaginated{
		PageFunc: func(_ int) (*secretsmanager.ListSecretsOutput, error) {
			rotEnabled := false
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{
					{
						Name:            aws.String("staging/api/key"),
						Description:     aws.String("Staging API key"),
						RotationEnabled: &rotEnabled,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSecretsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchSecretsPage_Empty(t *testing.T) {
	mock := &mockSecretsManagerListSecretsAPIPaginated{
		PageFunc: func(_ int) (*secretsmanager.ListSecretsOutput, error) {
			return &secretsmanager.ListSecretsOutput{
				SecretList: []smtypes.SecretListEntry{},
				NextToken:  nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSecretsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchSecretsPage_Error(t *testing.T) {
	mock := &mockSecretsManagerListSecretsAPIPaginated{
		PageFunc: func(_ int) (*secretsmanager.ListSecretsOutput, error) {
			return nil, errors.New("list secrets failed")
		},
	}

	_, err := awsclient.FetchSecretsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SSM DescribeParameters (paginated)
// ---------------------------------------------------------------------------

type mockSSMDescribeParametersAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*ssm.DescribeParametersOutput, error)
}

func (m *mockSSMDescribeParametersAPIPaginated) DescribeParameters(_ context.Context, _ *ssm.DescribeParametersInput, _ ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchSSMParametersPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchSSMParametersPage_FirstPage(t *testing.T) {
	mock := &mockSSMDescribeParametersAPIPaginated{
		PageFunc: func(_ int) (*ssm.DescribeParametersOutput, error) {
			return &ssm.DescribeParametersOutput{
				Parameters: []ssmtypes.ParameterMetadata{
					{
						Name:        aws.String("/prod/db/host"),
						Type:        ssmtypes.ParameterTypeString,
						Version:     1,
						Description: aws.String("Production database host"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchSSMParametersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "/prod/db/host" {
		t.Errorf("resource ID: expected %q, got %q", "/prod/db/host", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchSSMParametersPage_Continuation(t *testing.T) {
	mock := &mockSSMDescribeParametersAPIPaginated{
		PageFunc: func(_ int) (*ssm.DescribeParametersOutput, error) {
			return &ssm.DescribeParametersOutput{
				Parameters: []ssmtypes.ParameterMetadata{
					{
						Name:        aws.String("/prod/db/port"),
						Type:        ssmtypes.ParameterTypeString,
						Version:     2,
						Description: aws.String("Production database port"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSSMParametersPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchSSMParametersPage_Empty(t *testing.T) {
	mock := &mockSSMDescribeParametersAPIPaginated{
		PageFunc: func(_ int) (*ssm.DescribeParametersOutput, error) {
			return &ssm.DescribeParametersOutput{
				Parameters: []ssmtypes.ParameterMetadata{},
				NextToken:  nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSSMParametersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchSSMParametersPage_Error(t *testing.T) {
	mock := &mockSSMDescribeParametersAPIPaginated{
		PageFunc: func(_ int) (*ssm.DescribeParametersOutput, error) {
			return nil, errors.New("describe parameters failed")
		},
	}

	_, err := awsclient.FetchSSMParametersPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
