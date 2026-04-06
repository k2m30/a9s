package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Paginated mock: EC2 DescribeInstances
// ---------------------------------------------------------------------------

type mockEC2PaginatedClient struct {
	outputs []*ec2.DescribeInstancesOutput
	err     error
	callIdx int
}

func (m *mockEC2PaginatedClient) DescribeInstances(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeInstancesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func (m *mockEC2PaginatedClient) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

func TestFetchEC2Instances_Pagination(t *testing.T) {
	mock := &mockEC2PaginatedClient{
		outputs: []*ec2.DescribeInstancesOutput{
			{
				NextToken: aws.String("page2-token"),
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-page1-001"),
								InstanceType: ec2types.InstanceTypeT3Micro,
								State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
							},
							{
								InstanceId:   aws.String("i-page1-002"),
								InstanceType: ec2types.InstanceTypeT3Small,
								State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
							},
						},
					},
				},
			},
			{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-page2-001"),
								InstanceType: ec2types.InstanceTypeM5Large,
								State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
							},
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if resources[0].ID != "i-page1-001" {
			t.Errorf("expected %q, got %q", "i-page1-001", resources[0].ID)
		}
	})

	t.Run("page1_second", func(t *testing.T) {
		if resources[1].ID != "i-page1-002" {
			t.Errorf("expected %q, got %q", "i-page1-002", resources[1].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if resources[2].ID != "i-page2-001" {
			t.Errorf("expected %q, got %q", "i-page2-001", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: Lambda ListFunctions
// ---------------------------------------------------------------------------

type mockLambdaPaginatedClient struct {
	outputs []*lambda.ListFunctionsOutput
	err     error
	callIdx int
}

func (m *mockLambdaPaginatedClient) ListFunctions(
	ctx context.Context,
	params *lambda.ListFunctionsInput,
	optFns ...func(*lambda.Options),
) (*lambda.ListFunctionsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &lambda.ListFunctionsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchLambdaFunctions_Pagination(t *testing.T) {
	mock := &mockLambdaPaginatedClient{
		outputs: []*lambda.ListFunctionsOutput{
			{
				NextMarker: aws.String("page2-marker"),
				Functions: []lambdatypes.FunctionConfiguration{
					{
						FunctionName: aws.String("page1-func-1"),
						Runtime:      lambdatypes.RuntimeNodejs18x,
						MemorySize:   aws.Int32(128),
						Timeout:      aws.Int32(30),
						Handler:      aws.String("index.handler"),
						PackageType:  lambdatypes.PackageTypeZip,
					},
				},
			},
			{
				Functions: []lambdatypes.FunctionConfiguration{
					{
						FunctionName: aws.String("page2-func-1"),
						Runtime:      lambdatypes.RuntimePython312,
						MemorySize:   aws.Int32(256),
						Timeout:      aws.Int32(60),
						Handler:      aws.String("app.handler"),
						PackageType:  lambdatypes.PackageTypeZip,
					},
					{
						FunctionName: aws.String("page2-func-2"),
						Runtime:      lambdatypes.RuntimeGo1x,
						MemorySize:   aws.Int32(512),
						Timeout:      aws.Int32(120),
						Handler:      aws.String("main"),
						PackageType:  lambdatypes.PackageTypeZip,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchLambdaFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_func", func(t *testing.T) {
		if resources[0].ID != "page1-func-1" {
			t.Errorf("expected %q, got %q", "page1-func-1", resources[0].ID)
		}
	})

	t.Run("page2_funcs", func(t *testing.T) {
		if resources[1].ID != "page2-func-1" {
			t.Errorf("expected %q, got %q", "page2-func-1", resources[1].ID)
		}
		if resources[2].ID != "page2-func-2" {
			t.Errorf("expected %q, got %q", "page2-func-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: RDS DescribeDBInstances
// ---------------------------------------------------------------------------

type mockRDSPaginatedClient struct {
	outputs []*rds.DescribeDBInstancesOutput
	err     error
	callIdx int
}

func (m *mockRDSPaginatedClient) DescribeDBInstances(
	ctx context.Context,
	params *rds.DescribeDBInstancesInput,
	optFns ...func(*rds.Options),
) (*rds.DescribeDBInstancesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &rds.DescribeDBInstancesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchRDSInstances_Pagination(t *testing.T) {
	mock := &mockRDSPaginatedClient{
		outputs: []*rds.DescribeDBInstancesOutput{
			{
				Marker: aws.String("page2-marker"),
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("page1-db-1"),
						Engine:               aws.String("mysql"),
						EngineVersion:        aws.String("8.0"),
						DBInstanceStatus:     aws.String("available"),
						DBInstanceClass:      aws.String("db.t3.micro"),
					},
				},
			},
			{
				DBInstances: []rdstypes.DBInstance{
					{
						DBInstanceIdentifier: aws.String("page2-db-1"),
						Engine:               aws.String("postgres"),
						EngineVersion:        aws.String("15.2"),
						DBInstanceStatus:     aws.String("available"),
						DBInstanceClass:      aws.String("db.r5.large"),
					},
					{
						DBInstanceIdentifier: aws.String("page2-db-2"),
						Engine:               aws.String("aurora"),
						EngineVersion:        aws.String("3.0"),
						DBInstanceStatus:     aws.String("creating"),
						DBInstanceClass:      aws.String("db.r6g.xlarge"),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchRDSInstances(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_db", func(t *testing.T) {
		if resources[0].ID != "page1-db-1" {
			t.Errorf("expected %q, got %q", "page1-db-1", resources[0].ID)
		}
	})

	t.Run("page2_dbs", func(t *testing.T) {
		if resources[1].ID != "page2-db-1" {
			t.Errorf("expected %q, got %q", "page2-db-1", resources[1].ID)
		}
		if resources[2].ID != "page2-db-2" {
			t.Errorf("expected %q, got %q", "page2-db-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: IAM ListRoles
// ---------------------------------------------------------------------------

type mockIAMListRolesPaginatedClient struct {
	outputs []*iam.ListRolesOutput
	err     error
	callIdx int
}

func (m *mockIAMListRolesPaginatedClient) ListRoles(
	ctx context.Context,
	params *iam.ListRolesInput,
	optFns ...func(*iam.Options),
) (*iam.ListRolesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &iam.ListRolesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchIAMRoles_Pagination(t *testing.T) {
	mock := &mockIAMListRolesPaginatedClient{
		outputs: []*iam.ListRolesOutput{
			{
				IsTruncated: true,
				Marker:      aws.String("page2-marker"),
				Roles: []iamtypes.Role{
					{RoleName: aws.String("page1-role-1"), RoleId: aws.String("AROAEXAMPLE1"), Path: aws.String("/")},
					{RoleName: aws.String("page1-role-2"), RoleId: aws.String("AROAEXAMPLE2"), Path: aws.String("/")},
				},
			},
			{
				IsTruncated: false,
				Roles: []iamtypes.Role{
					{RoleName: aws.String("page2-role-1"), RoleId: aws.String("AROAEXAMPLE3"), Path: aws.String("/service-role/")},
				},
			},
		},
	}

	resources, err := awsclient.FetchIAMRoles(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_roles", func(t *testing.T) {
		if resources[0].ID != "page1-role-1" {
			t.Errorf("expected %q, got %q", "page1-role-1", resources[0].ID)
		}
		if resources[1].ID != "page1-role-2" {
			t.Errorf("expected %q, got %q", "page1-role-2", resources[1].ID)
		}
	})

	t.Run("page2_role", func(t *testing.T) {
		if resources[2].ID != "page2-role-1" {
			t.Errorf("expected %q, got %q", "page2-role-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: IAM ListUsers
// ---------------------------------------------------------------------------

type mockIAMListUsersPaginatedClient struct {
	outputs []*iam.ListUsersOutput
	err     error
	callIdx int
}

func (m *mockIAMListUsersPaginatedClient) ListUsers(
	ctx context.Context,
	params *iam.ListUsersInput,
	optFns ...func(*iam.Options),
) (*iam.ListUsersOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &iam.ListUsersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchIAMUsers_Pagination(t *testing.T) {
	mock := &mockIAMListUsersPaginatedClient{
		outputs: []*iam.ListUsersOutput{
			{
				IsTruncated: true,
				Marker:      aws.String("page2-marker"),
				Users: []iamtypes.User{
					{UserName: aws.String("page1-user-1"), UserId: aws.String("AIDAEXAMPLE1"), Path: aws.String("/")},
				},
			},
			{
				IsTruncated: false,
				Users: []iamtypes.User{
					{UserName: aws.String("page2-user-1"), UserId: aws.String("AIDAEXAMPLE2"), Path: aws.String("/")},
					{UserName: aws.String("page2-user-2"), UserId: aws.String("AIDAEXAMPLE3"), Path: aws.String("/developers/")},
				},
			},
		},
	}

	resources, err := awsclient.FetchIAMUsers(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_user", func(t *testing.T) {
		if resources[0].ID != "page1-user-1" {
			t.Errorf("expected %q, got %q", "page1-user-1", resources[0].ID)
		}
	})

	t.Run("page2_users", func(t *testing.T) {
		if resources[1].ID != "page2-user-1" {
			t.Errorf("expected %q, got %q", "page2-user-1", resources[1].ID)
		}
		if resources[2].ID != "page2-user-2" {
			t.Errorf("expected %q, got %q", "page2-user-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: IAM ListPolicies
// ---------------------------------------------------------------------------

type mockIAMListPoliciesPaginatedClient struct {
	outputs []*iam.ListPoliciesOutput
	err     error
	callIdx int
}

func (m *mockIAMListPoliciesPaginatedClient) ListPolicies(
	ctx context.Context,
	params *iam.ListPoliciesInput,
	optFns ...func(*iam.Options),
) (*iam.ListPoliciesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &iam.ListPoliciesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchIAMPolicies_Pagination(t *testing.T) {
	mock := &mockIAMListPoliciesPaginatedClient{
		outputs: []*iam.ListPoliciesOutput{
			{
				IsTruncated: true,
				Marker:      aws.String("page2-marker"),
				Policies: []iamtypes.Policy{
					{PolicyName: aws.String("page1-policy-1"), PolicyId: aws.String("ANPAEXAMPLE1"), Path: aws.String("/")},
				},
			},
			{
				IsTruncated: false,
				Policies: []iamtypes.Policy{
					{PolicyName: aws.String("page2-policy-1"), PolicyId: aws.String("ANPAEXAMPLE2"), Path: aws.String("/")},
				},
			},
		},
	}

	resources, err := awsclient.FetchIAMPolicies(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 2 {
			t.Fatalf("expected 2 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_policy", func(t *testing.T) {
		if resources[0].ID != "page1-policy-1" {
			t.Errorf("expected %q, got %q", "page1-policy-1", resources[0].ID)
		}
	})

	t.Run("page2_policy", func(t *testing.T) {
		if resources[1].ID != "page2-policy-1" {
			t.Errorf("expected %q, got %q", "page2-policy-1", resources[1].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: CloudWatch Logs DescribeLogGroups
// ---------------------------------------------------------------------------

type mockCWLogsPaginatedClient struct {
	outputs []*cloudwatchlogs.DescribeLogGroupsOutput
	err     error
	callIdx int
}

func (m *mockCWLogsPaginatedClient) DescribeLogGroups(
	ctx context.Context,
	params *cloudwatchlogs.DescribeLogGroupsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchCloudWatchLogGroups_Pagination(t *testing.T) {
	mock := &mockCWLogsPaginatedClient{
		outputs: []*cloudwatchlogs.DescribeLogGroupsOutput{
			{
				NextToken: aws.String("page2-token"),
				LogGroups: []cwlogstypes.LogGroup{
					{LogGroupName: aws.String("/aws/lambda/page1-func-1"), StoredBytes: aws.Int64(1024)},
					{LogGroupName: aws.String("/aws/lambda/page1-func-2"), StoredBytes: aws.Int64(2048)},
				},
			},
			{
				LogGroups: []cwlogstypes.LogGroup{
					{LogGroupName: aws.String("/aws/lambda/page2-func-1"), StoredBytes: aws.Int64(4096)},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudWatchLogGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_groups", func(t *testing.T) {
		if resources[0].Name != "/aws/lambda/page1-func-1" {
			t.Errorf("expected %q, got %q", "/aws/lambda/page1-func-1", resources[0].Name)
		}
		if resources[1].Name != "/aws/lambda/page1-func-2" {
			t.Errorf("expected %q, got %q", "/aws/lambda/page1-func-2", resources[1].Name)
		}
	})

	t.Run("page2_group", func(t *testing.T) {
		if resources[2].Name != "/aws/lambda/page2-func-1" {
			t.Errorf("expected %q, got %q", "/aws/lambda/page2-func-1", resources[2].Name)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: DynamoDB ListTables
// ---------------------------------------------------------------------------

type mockDDBListTablesPaginatedClient struct {
	outputs []*dynamodb.ListTablesOutput
	err     error
	callIdx int
}

func (m *mockDDBListTablesPaginatedClient) ListTables(
	ctx context.Context,
	params *dynamodb.ListTablesInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.ListTablesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &dynamodb.ListTablesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchDynamoDBTables_Pagination(t *testing.T) {
	listMock := &mockDDBListTablesPaginatedClient{
		outputs: []*dynamodb.ListTablesOutput{
			{
				LastEvaluatedTableName: aws.String("page1-table-2"),
				TableNames:             []string{"page1-table-1", "page1-table-2"},
			},
			{
				TableNames: []string{"page2-table-1"},
			},
		},
	}

	describeMock := &mockDDBDescribeTableClient{
		outputs: map[string]*dynamodb.DescribeTableOutput{
			"page1-table-1": {
				Table: &ddbtypes.TableDescription{
					TableName:   aws.String("page1-table-1"),
					TableStatus: ddbtypes.TableStatusActive,
					ItemCount:   aws.Int64(100),
				},
			},
			"page1-table-2": {
				Table: &ddbtypes.TableDescription{
					TableName:   aws.String("page1-table-2"),
					TableStatus: ddbtypes.TableStatusActive,
					ItemCount:   aws.Int64(200),
				},
			},
			"page2-table-1": {
				Table: &ddbtypes.TableDescription{
					TableName:   aws.String("page2-table-1"),
					TableStatus: ddbtypes.TableStatusCreating,
					ItemCount:   aws.Int64(0),
				},
			},
		},
	}

	resources, err := awsclient.FetchDynamoDBTables(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_tables", func(t *testing.T) {
		if resources[0].ID != "page1-table-1" {
			t.Errorf("expected %q, got %q", "page1-table-1", resources[0].ID)
		}
		if resources[1].ID != "page1-table-2" {
			t.Errorf("expected %q, got %q", "page1-table-2", resources[1].ID)
		}
	})

	t.Run("page2_table", func(t *testing.T) {
		if resources[2].ID != "page2-table-1" {
			t.Errorf("expected %q, got %q", "page2-table-1", resources[2].ID)
		}
	})

	t.Run("list_api_called_twice", func(t *testing.T) {
		if listMock.callIdx != 2 {
			t.Errorf("expected 2 ListTables API calls, got %d", listMock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: SQS ListQueues
// ---------------------------------------------------------------------------

type mockSQSListQueuesPaginatedClient struct {
	outputs []*sqs.ListQueuesOutput
	err     error
	callIdx int
}

func (m *mockSQSListQueuesPaginatedClient) ListQueues(
	ctx context.Context,
	params *sqs.ListQueuesInput,
	optFns ...func(*sqs.Options),
) (*sqs.ListQueuesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &sqs.ListQueuesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchSQSQueues_Pagination(t *testing.T) {
	listMock := &mockSQSListQueuesPaginatedClient{
		outputs: []*sqs.ListQueuesOutput{
			{
				NextToken: aws.String("page2-token"),
				QueueUrls: []string{
					"https://sqs.us-east-1.amazonaws.com/111122223333/page1-queue-1",
				},
			},
			{
				QueueUrls: []string{
					"https://sqs.us-east-1.amazonaws.com/111122223333/page2-queue-1",
					"https://sqs.us-east-1.amazonaws.com/111122223333/page2-queue-2",
				},
			},
		},
	}

	attrMock := &mockSQSGetQueueAttributesClient{
		outputs: map[string]*sqs.GetQueueAttributesOutput{
			"https://sqs.us-east-1.amazonaws.com/111122223333/page1-queue-1": {
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "10",
					"ApproximateNumberOfMessagesNotVisible": "2",
					"DelaySeconds":                          "0",
				},
			},
			"https://sqs.us-east-1.amazonaws.com/111122223333/page2-queue-1": {
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "5",
					"ApproximateNumberOfMessagesNotVisible": "1",
					"DelaySeconds":                          "30",
				},
			},
			"https://sqs.us-east-1.amazonaws.com/111122223333/page2-queue-2": {
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "0",
					"ApproximateNumberOfMessagesNotVisible": "0",
					"DelaySeconds":                          "0",
				},
			},
		},
	}

	resources, err := awsclient.FetchSQSQueues(context.Background(), listMock, attrMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_queue", func(t *testing.T) {
		if resources[0].ID != "page1-queue-1" {
			t.Errorf("expected %q, got %q", "page1-queue-1", resources[0].ID)
		}
	})

	t.Run("page2_queues", func(t *testing.T) {
		if resources[1].ID != "page2-queue-1" {
			t.Errorf("expected %q, got %q", "page2-queue-1", resources[1].ID)
		}
		if resources[2].ID != "page2-queue-2" {
			t.Errorf("expected %q, got %q", "page2-queue-2", resources[2].ID)
		}
	})

	t.Run("list_api_called_twice", func(t *testing.T) {
		if listMock.callIdx != 2 {
			t.Errorf("expected 2 ListQueues API calls, got %d", listMock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mock: SNS ListTopics
// ---------------------------------------------------------------------------

type mockSNSPaginatedClient struct {
	outputs []*sns.ListTopicsOutput
	err     error
	callIdx int
}

func (m *mockSNSPaginatedClient) ListTopics(
	ctx context.Context,
	params *sns.ListTopicsInput,
	optFns ...func(*sns.Options),
) (*sns.ListTopicsOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &sns.ListTopicsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchSNSTopics_Pagination(t *testing.T) {
	mock := &mockSNSPaginatedClient{
		outputs: []*sns.ListTopicsOutput{
			{
				NextToken: aws.String("page2-token"),
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:aws:sns:us-east-1:111122223333:page1-topic-1")},
					{TopicArn: aws.String("arn:aws:sns:us-east-1:111122223333:page1-topic-2")},
				},
			},
			{
				Topics: []snstypes.Topic{
					{TopicArn: aws.String("arn:aws:sns:us-east-1:111122223333:page2-topic-1")},
				},
			},
		},
	}

	resources, err := awsclient.FetchSNSTopics(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_topics", func(t *testing.T) {
		if resources[0].Name != "page1-topic-1" {
			t.Errorf("expected %q, got %q", "page1-topic-1", resources[0].Name)
		}
		if resources[1].Name != "page1-topic-2" {
			t.Errorf("expected %q, got %q", "page1-topic-2", resources[1].Name)
		}
	})

	t.Run("page2_topic", func(t *testing.T) {
		if resources[2].Name != "page2-topic-1" {
			t.Errorf("expected %q, got %q", "page2-topic-1", resources[2].Name)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Paginated mocks: KMS ListKeys + ListAliases
// ---------------------------------------------------------------------------

type mockKMSListKeysPaginatedClient struct {
	outputs []*kms.ListKeysOutput
	err     error
	callIdx int
}

func (m *mockKMSListKeysPaginatedClient) ListKeys(
	ctx context.Context,
	params *kms.ListKeysInput,
	optFns ...func(*kms.Options),
) (*kms.ListKeysOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &kms.ListKeysOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

type mockKMSListAliasesPaginatedClient struct {
	outputs []*kms.ListAliasesOutput
	err     error
	callIdx int
}

func (m *mockKMSListAliasesPaginatedClient) ListAliases(
	ctx context.Context,
	params *kms.ListAliasesInput,
	optFns ...func(*kms.Options),
) (*kms.ListAliasesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &kms.ListAliasesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchKMSKeys_Pagination(t *testing.T) {
	listKeysMock := &mockKMSListKeysPaginatedClient{
		outputs: []*kms.ListKeysOutput{
			{
				Truncated:  true,
				NextMarker: aws.String("page2-marker"),
				Keys: []kmstypes.KeyListEntry{
					{KeyId: aws.String("key-page1-001")},
				},
			},
			{
				Truncated: false,
				Keys: []kmstypes.KeyListEntry{
					{KeyId: aws.String("key-page2-001")},
				},
			},
		},
	}

	listAliasesMock := &mockKMSListAliasesPaginatedClient{
		outputs: []*kms.ListAliasesOutput{
			{
				Truncated:  true,
				NextMarker: aws.String("alias-page2"),
				Aliases: []kmstypes.AliasListEntry{
					{TargetKeyId: aws.String("key-page1-001"), AliasName: aws.String("alias/page1-key")},
				},
			},
			{
				Truncated: false,
				Aliases: []kmstypes.AliasListEntry{
					{TargetKeyId: aws.String("key-page2-001"), AliasName: aws.String("alias/page2-key")},
				},
			},
		},
	}

	describeKeyMock := &mockKMSDescribeKeyClient{
		outputs: map[string]*kms.DescribeKeyOutput{
			"key-page1-001": {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:      aws.String("key-page1-001"),
					KeyState:   kmstypes.KeyStateEnabled,
					KeyManager: kmstypes.KeyManagerTypeCustomer,
				},
			},
			"key-page2-001": {
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:      aws.String("key-page2-001"),
					KeyState:   kmstypes.KeyStateEnabled,
					KeyManager: kmstypes.KeyManagerTypeCustomer,
				},
			},
		},
	}

	resources, err := awsclient.FetchKMSKeys(context.Background(), listKeysMock, describeKeyMock, listAliasesMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 2 {
			t.Fatalf("expected 2 customer-managed keys across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_key", func(t *testing.T) {
		if resources[0].ID != "key-page1-001" {
			t.Errorf("expected %q, got %q", "key-page1-001", resources[0].ID)
		}
		if resources[0].Name != "alias/page1-key" {
			t.Errorf("expected alias %q, got %q", "alias/page1-key", resources[0].Name)
		}
	})

	t.Run("page2_key", func(t *testing.T) {
		if resources[1].ID != "key-page2-001" {
			t.Errorf("expected %q, got %q", "key-page2-001", resources[1].ID)
		}
		if resources[1].Name != "alias/page2-key" {
			t.Errorf("expected alias %q, got %q", "alias/page2-key", resources[1].Name)
		}
	})

	t.Run("list_keys_called_twice", func(t *testing.T) {
		if listKeysMock.callIdx != 2 {
			t.Errorf("expected 2 ListKeys API calls, got %d", listKeysMock.callIdx)
		}
	})

	t.Run("list_aliases_called_twice", func(t *testing.T) {
		if listAliasesMock.callIdx != 2 {
			t.Errorf("expected 2 ListAliases API calls, got %d", listAliasesMock.callIdx)
		}
	})
}

// ---------------------------------------------------------------------------
// Pagination error on second page tests
// ---------------------------------------------------------------------------

func TestFetchEC2Instances_PaginationErrorOnSecondPage(t *testing.T) {
	errMock := &mockEC2PaginatedErrorOnPage2Client{
		firstOutput: &ec2.DescribeInstancesOutput{
			NextToken: aws.String("page2-token"),
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{
						{
							InstanceId:   aws.String("i-first-page"),
							InstanceType: ec2types.InstanceTypeT3Micro,
							State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
						},
					},
				},
			},
		},
		secondErr: fmt.Errorf("throttling exception"),
	}

	_, err := awsclient.FetchEC2Instances(context.Background(), errMock)
	if err == nil {
		t.Fatal("expected error on second page, got nil")
	}
}

type mockEC2PaginatedErrorOnPage2Client struct {
	firstOutput *ec2.DescribeInstancesOutput
	secondErr   error
	callIdx     int
}

func (m *mockEC2PaginatedErrorOnPage2Client) DescribeInstances(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	m.callIdx++
	if m.callIdx == 1 {
		return m.firstOutput, nil
	}
	return nil, m.secondErr
}

func (m *mockEC2PaginatedErrorOnPage2Client) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}
