package unit

// Fetcher pagination over large counts, help-view load-more visibility, and
// log-event fetch parameters.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// storyEC2PaginatedMock produces N instances split into pages of pageSize.
type storyEC2PaginatedMock struct {
	pages   []*ec2.DescribeInstancesOutput
	callIdx int
}

func newStoryEC2PaginatedMock(total, pageSize int) *storyEC2PaginatedMock {
	m := &storyEC2PaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := min(pageSize, remaining)
		instances := make([]ec2types.Instance, count)
		for i := range count {
			idx := pageNum*pageSize + i
			instances[i] = ec2types.Instance{
				InstanceId:   aws.String(fmt.Sprintf("i-%07d", idx)),
				InstanceType: ec2types.InstanceTypeT3Micro,
				State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			}
		}
		out := &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{{Instances: instances}},
		}
		remaining -= count
		if remaining > 0 {
			out.NextToken = aws.String(fmt.Sprintf("page-%d-token", pageNum+1))
		}
		m.pages = append(m.pages, out)
		pageNum++
	}
	return m
}

func (m *storyEC2PaginatedMock) DescribeInstances(
	ctx context.Context,
	params *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	if m.callIdx >= len(m.pages) {
		return &ec2.DescribeInstancesOutput{}, nil
	}
	out := m.pages[m.callIdx]
	m.callIdx++
	return out, nil
}

func (m *storyEC2PaginatedMock) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

func TestStoryD1_EC2_1500Instances_AllReturned(t *testing.T) {
	mock := newStoryEC2PaginatedMock(1500, 1000)
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 1500 {
		t.Fatalf("D.1: expected 1500 instances, got %d", len(resources))
	}

	if resources[0].ID != "i-0000000" {
		t.Errorf("first resource ID: expected %q, got %q", "i-0000000", resources[0].ID)
	}
	if resources[1499].ID != "i-0001499" {
		t.Errorf("last resource ID: expected %q, got %q", "i-0001499", resources[1499].ID)
	}

	if mock.callIdx != 2 {
		t.Errorf("expected 2 API calls, got %d", mock.callIdx)
	}
}

type storyLambdaPaginatedMock struct {
	pages   []*lambda.ListFunctionsOutput
	callIdx int
}

func newStoryLambdaPaginatedMock(total, pageSize int) *storyLambdaPaginatedMock {
	m := &storyLambdaPaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := min(pageSize, remaining)
		funcs := make([]lambdatypes.FunctionConfiguration, count)
		for i := range count {
			idx := pageNum*pageSize + i
			funcs[i] = lambdatypes.FunctionConfiguration{
				FunctionName: aws.String(fmt.Sprintf("func-%04d", idx)),
				Runtime:      lambdatypes.RuntimeNodejs18x,
				MemorySize:   aws.Int32(128),
				Timeout:      aws.Int32(30),
				Handler:      aws.String("index.handler"),
				PackageType:  lambdatypes.PackageTypeZip,
			}
		}
		out := &lambda.ListFunctionsOutput{Functions: funcs}
		remaining -= count
		if remaining > 0 {
			out.NextMarker = aws.String(fmt.Sprintf("page-%d-marker", pageNum+1))
		}
		m.pages = append(m.pages, out)
		pageNum++
	}
	return m
}

func (m *storyLambdaPaginatedMock) ListFunctions(
	ctx context.Context,
	params *lambda.ListFunctionsInput,
	optFns ...func(*lambda.Options),
) (*lambda.ListFunctionsOutput, error) {
	if m.callIdx >= len(m.pages) {
		return &lambda.ListFunctionsOutput{}, nil
	}
	out := m.pages[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestStoryD2_Lambda_200Functions_AllReturned(t *testing.T) {
	mock := newStoryLambdaPaginatedMock(200, 50)
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchLambdaFunctionsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 200 {
		t.Fatalf("D.2: expected 200 functions, got %d", len(resources))
	}

	if resources[0].ID != "func-0000" {
		t.Errorf("first resource: expected %q, got %q", "func-0000", resources[0].ID)
	}
	if resources[199].ID != "func-0199" {
		t.Errorf("last resource: expected %q, got %q", "func-0199", resources[199].ID)
	}

	if mock.callIdx != 4 {
		t.Errorf("expected 4 API calls, got %d", mock.callIdx)
	}
}

type storyRDSPaginatedMock struct {
	pages   []*rds.DescribeDBInstancesOutput
	callIdx int
}

func newStoryRDSPaginatedMock(total, pageSize int) *storyRDSPaginatedMock {
	m := &storyRDSPaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := min(pageSize, remaining)
		instances := make([]rdstypes.DBInstance, count)
		for i := range count {
			idx := pageNum*pageSize + i
			instances[i] = rdstypes.DBInstance{
				DBInstanceIdentifier: aws.String(fmt.Sprintf("db-%05d", idx)),
				Engine:               aws.String("mysql"),
				EngineVersion:        aws.String("8.0"),
				DBInstanceStatus:     aws.String("available"),
				DBInstanceClass:      aws.String("db.t3.micro"),
			}
		}
		out := &rds.DescribeDBInstancesOutput{DBInstances: instances}
		remaining -= count
		if remaining > 0 {
			out.Marker = aws.String(fmt.Sprintf("page-%d-marker", pageNum+1))
		}
		m.pages = append(m.pages, out)
		pageNum++
	}
	return m
}

func (m *storyRDSPaginatedMock) DescribeDBInstances(
	ctx context.Context,
	params *rds.DescribeDBInstancesInput,
	optFns ...func(*rds.Options),
) (*rds.DescribeDBInstancesOutput, error) {
	if m.callIdx >= len(m.pages) {
		return &rds.DescribeDBInstancesOutput{}, nil
	}
	out := m.pages[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestStoryD3_RDS_250Instances_AllReturned(t *testing.T) {
	mock := newStoryRDSPaginatedMock(250, 100)
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchRDSInstancesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 250 {
		t.Fatalf("D.3: expected 250 instances, got %d", len(resources))
	}

	if resources[0].ID != "db-00000" {
		t.Errorf("first: expected %q, got %q", "db-00000", resources[0].ID)
	}
	if resources[249].ID != "db-00249" {
		t.Errorf("last: expected %q, got %q", "db-00249", resources[249].ID)
	}

	if mock.callIdx != 3 {
		t.Errorf("expected 3 API calls, got %d", mock.callIdx)
	}
}

type storyIAMRolesPaginatedMock struct {
	pages   []*iam.ListRolesOutput
	callIdx int
}

func newStoryIAMRolesPaginatedMock(total, pageSize int) *storyIAMRolesPaginatedMock {
	m := &storyIAMRolesPaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := min(pageSize, remaining)
		roles := make([]iamtypes.Role, count)
		for i := range count {
			idx := pageNum*pageSize + i
			roles[i] = iamtypes.Role{
				RoleName: aws.String(fmt.Sprintf("role-%05d", idx)),
				RoleId:   aws.String(fmt.Sprintf("AROAEXAMPLE%05d", idx)),
				Path:     aws.String("/"),
			}
		}
		out := &iam.ListRolesOutput{
			Roles:       roles,
			IsTruncated: remaining > count,
		}
		remaining -= count
		if remaining > 0 {
			out.Marker = aws.String(fmt.Sprintf("page-%d-marker", pageNum+1))
		}
		m.pages = append(m.pages, out)
		pageNum++
	}
	return m
}

func (m *storyIAMRolesPaginatedMock) ListRoles(
	ctx context.Context,
	params *iam.ListRolesInput,
	optFns ...func(*iam.Options),
) (*iam.ListRolesOutput, error) {
	if m.callIdx >= len(m.pages) {
		return &iam.ListRolesOutput{}, nil
	}
	out := m.pages[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestStoryD4_IAMRoles_3000Roles_AllReturned(t *testing.T) {
	mock := newStoryIAMRolesPaginatedMock(3000, 100)
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchIAMRolesPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 3000 {
		t.Fatalf("D.4: expected 3000 roles, got %d", len(resources))
	}

	if resources[0].ID != "role-00000" {
		t.Errorf("first: expected %q, got %q", "role-00000", resources[0].ID)
	}
	if resources[2999].ID != "role-02999" {
		t.Errorf("last: expected %q, got %q", "role-02999", resources[2999].ID)
	}

	if mock.callIdx != 30 {
		t.Errorf("expected 30 API calls, got %d", mock.callIdx)
	}
}

type storyCWLogsPaginatedMock struct {
	pages   []*cloudwatchlogs.DescribeLogGroupsOutput
	callIdx int
}

func newStoryCWLogsPaginatedMock(total, pageSize int) *storyCWLogsPaginatedMock {
	m := &storyCWLogsPaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := min(pageSize, remaining)
		groups := make([]cwlogstypes.LogGroup, count)
		for i := range count {
			idx := pageNum*pageSize + i
			groups[i] = cwlogstypes.LogGroup{
				LogGroupName: aws.String(fmt.Sprintf("/aws/lambda/func-%04d", idx)),
				StoredBytes:  aws.Int64(int64(1024 * (idx + 1))),
			}
		}
		out := &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: groups}
		remaining -= count
		if remaining > 0 {
			out.NextToken = aws.String(fmt.Sprintf("page-%d-token", pageNum+1))
		}
		m.pages = append(m.pages, out)
		pageNum++
	}
	return m
}

func (m *storyCWLogsPaginatedMock) DescribeLogGroups(
	ctx context.Context,
	params *cloudwatchlogs.DescribeLogGroupsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	if m.callIdx >= len(m.pages) {
		return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
	}
	out := m.pages[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestStoryD5_CWLogs_500LogGroups_AllReturned(t *testing.T) {
	mock := newStoryCWLogsPaginatedMock(500, 50)
	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchCloudWatchLogGroupsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 500 {
		t.Fatalf("D.5: expected 500 log groups, got %d", len(resources))
	}

	if resources[0].Name != "/aws/lambda/func-0000" {
		t.Errorf("first: expected %q, got %q", "/aws/lambda/func-0000", resources[0].Name)
	}
	if resources[499].Name != "/aws/lambda/func-0499" {
		t.Errorf("last: expected %q, got %q", "/aws/lambda/func-0499", resources[499].Name)
	}

	if mock.callIdx != 10 {
		t.Errorf("expected 10 API calls, got %d", mock.callIdx)
	}
}

type storySGSinglePageMock struct {
	output *ec2.DescribeSecurityGroupsOutput
}

func (m *storySGSinglePageMock) DescribeSecurityGroups(
	ctx context.Context,
	params *ec2.DescribeSecurityGroupsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeSecurityGroupsOutput, error) {
	return m.output, nil
}

func TestStoryD6_SG_1200Groups_CurrentBehavior(t *testing.T) {
	sgs := make([]ec2types.SecurityGroup, 1200)
	for i := range 1200 {
		sgs[i] = ec2types.SecurityGroup{
			GroupId:     aws.String(fmt.Sprintf("sg-%07d", i)),
			GroupName:   aws.String(fmt.Sprintf("sg-name-%04d", i)),
			VpcId:       aws.String("vpc-0abc123"),
			Description: aws.String("test security group"),
		}
	}
	mock := &storySGSinglePageMock{
		output: &ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: sgs,
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchSecurityGroupsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 1200 {
		t.Fatalf("D.6: expected 1200 security groups, got %d", len(resources))
	}

	if resources[0].ID != "sg-0000000" {
		t.Errorf("first: expected %q, got %q", "sg-0000000", resources[0].ID)
	}
	if resources[1199].ID != "sg-0001199" {
		t.Errorf("last: expected %q, got %q", "sg-0001199", resources[1199].ID)
	}
}

func TestStoryC1_HelpView_ShowsMKey_WhenTruncated(t *testing.T) {
	tuitest.ForceColor(t)

	help := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceListPaginated, "ec2")
	help.SetSize(120, 30)

	output := help.View()
	outputLower := strings.ToLower(output)

	if !strings.Contains(outputLower, "load more") {
		t.Errorf("C.1: paginated help view must contain 'load more', got:\n%s", output)
	}
	if !strings.Contains(output, "M") {
		t.Errorf("C.1: paginated help view must contain 'M' key, got:\n%s", output)
	}

	expectedSections := []string{"NAVIGATION", "ACTIONS", "SORT", "OTHER"}
	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("C.1: help view missing expected section %q", section)
		}
	}

	expectedBindings := []string{"refresh", "back", "filter", "yaml", "copy id", "help"}
	for _, binding := range expectedBindings {
		if !strings.Contains(output, binding) {
			t.Errorf("C.1: help view missing expected binding %q", binding)
		}
	}
}

func TestStoryC2_HelpView_HidesMKey_WhenNotTruncated(t *testing.T) {
	tuitest.ForceColor(t)

	help := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceList, "ec2")
	help.SetSize(120, 30)

	output := help.View()

	if strings.Contains(output, "Load More") || strings.Contains(output, "load more") {
		t.Errorf("C.2: help view should NOT show 'Load More' for non-truncated list, but it does")
	}

	helpMenu := views.NewHelpWithResource(keys.Default(), views.HelpFromMainMenu, "")
	helpMenu.SetSize(120, 30)
	menuOutput := helpMenu.View()
	if strings.Contains(menuOutput, "Load More") || strings.Contains(menuOutput, "load more") {
		t.Errorf("C.2: main menu help view should NOT show 'Load More'")
	}
}

func TestStoryK1_LogEvents_DefaultFetch(t *testing.T) {
	events := make([]cwlogstypes.OutputLogEvent, 5)
	for i := range 5 {
		ts := int64(1711100000000 + int64(i)*1000)
		msg := fmt.Sprintf("2026-03-22T10:00:0%d.000Z INFO Test message %d", i, i)
		events[i] = cwlogstypes.OutputLogEvent{
			Timestamp:     &ts,
			Message:       &msg,
			IngestionTime: &ts,
		}
	}

	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: events,
		},
	}

	result, err := awsclient.FetchLogEvents(
		context.Background(), mock,
		"/aws/lambda/test-func",
		"2026/03/22/test-stream",
		"",
	)
	if err != nil {
		t.Fatalf("K.1: unexpected error: %v", err)
	}

	if len(result.Resources) != 5 {
		t.Errorf("K.1: expected 5 resources, got %d", len(result.Resources))
	}

	if mock.lastInput == nil {
		t.Fatal("K.1: expected lastInput to be set")
	}
	if mock.lastInput.LogGroupName == nil || *mock.lastInput.LogGroupName != "/aws/lambda/test-func" {
		t.Errorf("K.1: expected log group name %q, got %v",
			"/aws/lambda/test-func", mock.lastInput.LogGroupName)
	}
	if mock.lastInput.LogStreamName == nil || *mock.lastInput.LogStreamName != "2026/03/22/test-stream" {
		t.Errorf("K.1: expected log stream name %q, got %v",
			"2026/03/22/test-stream", mock.lastInput.LogStreamName)
	}

	// StartFromHead=false returns the newest events first.
	if mock.lastInput.StartFromHead == nil || *mock.lastInput.StartFromHead {
		t.Errorf("K.1: expected StartFromHead=false, got %v", mock.lastInput.StartFromHead)
	}

	if result.Pagination == nil {
		t.Fatal("K.1: expected pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Log("K.1: FetchLogEvents returned IsTruncated=true — time range pagination may be implemented")
	}

	for i, r := range result.Resources {
		if r.Fields["timestamp"] == "" {
			t.Errorf("K.1: event %d has empty timestamp", i)
		}
		if r.Fields["message"] == "" {
			t.Errorf("K.1: event %d has empty message", i)
		}
	}
}

func TestStoryK2_LogEvents_ContinuationToken(t *testing.T) {
	ts := int64(1711100000000)
	msg := "2026-03-22T10:00:00.000Z INFO Older event"
	mock := &mockCWLogsGetLogEventsClient{
		output: &cloudwatchlogs.GetLogEventsOutput{
			Events: []cwlogstypes.OutputLogEvent{
				{Timestamp: &ts, Message: &msg, IngestionTime: &ts},
			},
		},
	}

	result, err := awsclient.FetchLogEvents(
		context.Background(), mock,
		"/aws/lambda/test-func",
		"2026/03/22/test-stream",
		"some-continuation-token",
	)
	if err != nil {
		t.Fatalf("K.2: unexpected error: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Errorf("K.2: expected 1 resource, got %d", len(result.Resources))
	}

}
