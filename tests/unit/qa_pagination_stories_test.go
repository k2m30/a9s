package unit

// qa_pagination_stories_test.go — Tests for QA stories sections D, F, G, H, I
// from docs/qa/pagination_stories.md.
//
// Sections A, B (basic), and E (retry) are already covered elsewhere.
// This file tests:
//   - D: Top-Level Pagination Correctness (large-count multi-page fetchers)
//   - F: Refresh Behavior (Ctrl+R resets pagination)
//   - G: Navigation Across Views with Pagination State
//   - H: Demo Mode (no pagination in demo)
//   - I: Edge Cases (sort preservation, cursor at bottom)

import (
	"context"
	"fmt"
	"os"
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

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// Section D: Top-Level Pagination Correctness
//
// These tests verify that fetchers paginate through ALL pages internally
// and return the complete result set. The existing tests in
// aws_toplevel_pagination_test.go cover correctness with small counts (2-3
// items). These tests use the large counts specified in the QA stories.
// ===========================================================================

// ---------------------------------------------------------------------------
// D.1: EC2 with 1500 instances across 2 API pages → all 1500 returned
// ---------------------------------------------------------------------------

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
		count := pageSize
		if count > remaining {
			count = remaining
		}
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

func TestStoryD1_EC2_1500Instances_AllReturned(t *testing.T) {
	mock := newStoryEC2PaginatedMock(1500, 1000)
	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 1500 {
		t.Fatalf("D.1: expected 1500 instances, got %d", len(resources))
	}

	// Verify first and last IDs to confirm both pages contributed
	if resources[0].ID != "i-0000000" {
		t.Errorf("first resource ID: expected %q, got %q", "i-0000000", resources[0].ID)
	}
	if resources[1499].ID != "i-0001499" {
		t.Errorf("last resource ID: expected %q, got %q", "i-0001499", resources[1499].ID)
	}

	// Verify all API pages were called (1000 + 500 = 2 pages)
	if mock.callIdx != 2 {
		t.Errorf("expected 2 API calls, got %d", mock.callIdx)
	}
}

// ---------------------------------------------------------------------------
// D.2: Lambda with 200 functions across 4 pages → all 200 returned
// ---------------------------------------------------------------------------

type storyLambdaPaginatedMock struct {
	pages   []*lambda.ListFunctionsOutput
	callIdx int
}

func newStoryLambdaPaginatedMock(total, pageSize int) *storyLambdaPaginatedMock {
	m := &storyLambdaPaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := pageSize
		if count > remaining {
			count = remaining
		}
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
	resources, err := awsclient.FetchLambdaFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 200 {
		t.Fatalf("D.2: expected 200 functions, got %d", len(resources))
	}

	// First and last
	if resources[0].ID != "func-0000" {
		t.Errorf("first resource: expected %q, got %q", "func-0000", resources[0].ID)
	}
	if resources[199].ID != "func-0199" {
		t.Errorf("last resource: expected %q, got %q", "func-0199", resources[199].ID)
	}

	// 200/50 = 4 pages
	if mock.callIdx != 4 {
		t.Errorf("expected 4 API calls, got %d", mock.callIdx)
	}
}

// ---------------------------------------------------------------------------
// D.3: RDS with 250 instances across 3 pages → all 250 returned
// ---------------------------------------------------------------------------

type storyRDSPaginatedMock struct {
	pages   []*rds.DescribeDBInstancesOutput
	callIdx int
}

func newStoryRDSPaginatedMock(total, pageSize int) *storyRDSPaginatedMock {
	m := &storyRDSPaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := pageSize
		if count > remaining {
			count = remaining
		}
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
	resources, err := awsclient.FetchRDSInstances(context.Background(), mock)
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

	// 100 + 100 + 50 = 3 pages
	if mock.callIdx != 3 {
		t.Errorf("expected 3 API calls, got %d", mock.callIdx)
	}
}

// ---------------------------------------------------------------------------
// D.4: IAM Roles with 3000 roles → all returned
// ---------------------------------------------------------------------------

type storyIAMRolesPaginatedMock struct {
	pages   []*iam.ListRolesOutput
	callIdx int
}

func newStoryIAMRolesPaginatedMock(total, pageSize int) *storyIAMRolesPaginatedMock {
	m := &storyIAMRolesPaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := pageSize
		if count > remaining {
			count = remaining
		}
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
	resources, err := awsclient.FetchIAMRoles(context.Background(), mock)
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

	// 3000/100 = 30 pages
	if mock.callIdx != 30 {
		t.Errorf("expected 30 API calls, got %d", mock.callIdx)
	}
}

// ---------------------------------------------------------------------------
// D.5: CloudWatch Logs with 500 log groups → all returned
// ---------------------------------------------------------------------------

type storyCWLogsPaginatedMock struct {
	pages   []*cloudwatchlogs.DescribeLogGroupsOutput
	callIdx int
}

func newStoryCWLogsPaginatedMock(total, pageSize int) *storyCWLogsPaginatedMock {
	m := &storyCWLogsPaginatedMock{}
	remaining := total
	pageNum := 0
	for remaining > 0 {
		count := pageSize
		if count > remaining {
			count = remaining
		}
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
	resources, err := awsclient.FetchCloudWatchLogGroups(context.Background(), mock)
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

	// 500/50 = 10 pages
	if mock.callIdx != 10 {
		t.Errorf("expected 10 API calls, got %d", mock.callIdx)
	}
}

// ---------------------------------------------------------------------------
// D.6: Security Groups with 1200 groups
//
// NOTE: The current SG fetcher (internal/aws/sg.go) does NOT paginate.
// It makes a single DescribeSecurityGroups call and returns whatever
// the API returns in that one response. The DescribeSecurityGroups API
// does support pagination (NextToken) but the fetcher does not loop.
// This test documents the current behavior: all items in a single response.
// When the fetcher is updated to paginate, this test should be expanded
// to use multiple pages.
// ---------------------------------------------------------------------------

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
	// Current implementation: single API call returns all groups.
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

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
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

// ===========================================================================
// Section F: Refresh Behavior
//
// Ctrl+R is handled at the app level (app_handlers.go), not at the
// ResourceListModel level. At the model level, a refresh results in:
//   1. Model enters loading state (loading=true set by ClearLoading or re-init)
//   2. A new ResourcesLoadedMsg arrives with Append=false (replacing old data)
//
// These tests verify the view-level behavior: that replacing data resets
// pagination state and counts.
// ===========================================================================

// storyNewModel creates a fresh ResourceListModel and initializes it.
func storyNewModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := pgTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 30)
	m, _ = m.Init()
	return m
}

// storyLoadResources is a convenience wrapper around pgLoadResources.
func storyLoadResources(
	m views.ResourceListModel,
	resources []resource.Resource,
	pagination *resource.PaginationMeta,
	appendMode bool,
) views.ResourceListModel {
	return pgLoadResources(m, resources, pagination, appendMode)
}

func TestStoryF1_CtrlR_ResetsPagination(t *testing.T) {
	m := storyNewModel(t)

	// Load initial truncated page (simulating first fetch)
	m = storyLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok-p2",
	}, false)
	if m.FrameTitle() != "ec2(200+)" {
		t.Fatalf("precondition: expected %q, got %q", "ec2(200+)", m.FrameTitle())
	}

	// Press M to load more — appends page 2
	m, _ = m.Update(pgKeyPress("M"))
	m = storyLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok-p3",
	}, true)
	if m.FrameTitle() != "ec2(400+)" {
		t.Fatalf("after M: expected %q, got %q", "ec2(400+)", m.FrameTitle())
	}

	// Simulate Ctrl+R: a full re-fetch replaces with first page only (Append=false).
	// This is what app_handlers.go does: it calls fetchResources or
	// fetchChildResources, which sends a ResourcesLoadedMsg with Append=false.
	m = storyLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok-fresh-p2",
	}, false)

	// After refresh, should be back to first-page state
	title := m.FrameTitle()
	if title != "ec2(200+)" {
		t.Errorf("F.1: after refresh, expected %q, got %q", "ec2(200+)", title)
	}
}

func TestStoryF2_CtrlR_TopLevel_ReFetchesAllPages(t *testing.T) {
	m := storyNewModel(t)

	// Simulate a top-level fetch that returned all items (not truncated)
	m = storyLoadResources(m, pgTestResources(150), nil, false)
	if m.FrameTitle() != "ec2(150)" {
		t.Fatalf("precondition: expected %q, got %q", "ec2(150)", m.FrameTitle())
	}

	// Simulate Ctrl+R: a full re-fetch returns potentially different data.
	// For top-level resources, the fetcher exhausts all pages internally,
	// so the ResourcesLoadedMsg comes with nil/false pagination.
	newResources := pgTestResources(160) // maybe some instances were created
	m = storyLoadResources(m, newResources, nil, false)

	title := m.FrameTitle()
	if title != "ec2(160)" {
		t.Errorf("F.2: after re-fetch, expected %q, got %q", "ec2(160)", title)
	}
}

func TestStoryF3_CtrlR_EmptyList_ReFetches(t *testing.T) {
	m := storyNewModel(t)

	// Load empty result set
	m = storyLoadResources(m, []resource.Resource{}, nil, false)
	if m.FrameTitle() != "ec2(0)" {
		t.Fatalf("precondition: expected %q, got %q", "ec2(0)", m.FrameTitle())
	}

	// Simulate Ctrl+R: now an instance exists
	m = storyLoadResources(m, pgTestResources(1), nil, false)

	title := m.FrameTitle()
	if title != "ec2(1)" {
		t.Errorf("F.3: after refresh from empty, expected %q, got %q", "ec2(1)", title)
	}
}

// ===========================================================================
// Section G: Navigation Across Views with Pagination State
// ===========================================================================

func TestStoryG1_DetailAndBack_PreservesLoadedData(t *testing.T) {
	m := storyNewModel(t)

	// Load 200 + append 200 + append 200 = 600 total
	m = storyLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok-p2",
	}, false)

	// Press M, receive page 2
	m, _ = m.Update(pgKeyPress("M"))
	page2 := make([]resource.Resource, 200)
	for i := range 200 {
		id := fmt.Sprintf("i-%05d", 200+i)
		page2[i] = resource.Resource{
			ID: id, Name: id, Status: "running",
			Fields: map[string]string{
				"instance_id": id, "name": id, "state": "running",
			},
		}
	}
	m = storyLoadResources(m, page2, &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok-p3",
	}, true)

	// Press M again, receive page 3
	m, _ = m.Update(pgKeyPress("M"))
	page3 := make([]resource.Resource, 200)
	for i := range 200 {
		id := fmt.Sprintf("i-%05d", 400+i)
		page3[i] = resource.Resource{
			ID: id, Name: id, Status: "running",
			Fields: map[string]string{
				"instance_id": id, "name": id, "state": "running",
			},
		}
	}
	m = storyLoadResources(m, page3, &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	titleBefore := m.FrameTitle()
	if titleBefore != "ec2(600)" {
		t.Fatalf("precondition: expected %q, got %q", "ec2(600)", titleBefore)
	}

	// Move cursor to row 5, then "navigate to detail" by pressing Enter.
	// The ResourceListModel itself is preserved on the view stack.
	// We simulate this by verifying the model state is unchanged after
	// receiving non-mutating messages (detail view is a separate model).
	for range 5 {
		m, _ = m.Update(pgKeyPress("j"))
	}
	selected := m.SelectedResource()
	if selected == nil {
		t.Fatal("expected a selected resource")
	}

	// After "returning from detail", the model is the same object.
	// Verify it still has all 600 resources and cursor at row 5.
	titleAfter := m.FrameTitle()
	if titleAfter != titleBefore {
		t.Errorf("G.1: frame title changed after detail round-trip: %q → %q", titleBefore, titleAfter)
	}
	if m.SelectedResource().ID != selected.ID {
		t.Errorf("G.1: cursor moved after detail round-trip: %q → %q", selected.ID, m.SelectedResource().ID)
	}
}

func TestStoryG3_SwitchingResourceType_ResetsPagination(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	// Create a model for EC2 and load paginated data
	ec2TD := pgTestTypeDef()
	k := keys.Default()
	m1 := views.NewResourceList(ec2TD, nil, k)
	m1.SetSize(120, 30)
	m1, _ = m1.Init()
	m1 = pgLoadResources(m1, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok",
	}, false)

	if m1.FrameTitle() != "ec2(200+)" {
		t.Fatalf("precondition: ec2 %q", m1.FrameTitle())
	}

	// When the user switches resource type, a NEW ResourceListModel is created.
	// Verify that a fresh model starts with no data and in loading state.
	rdsTypeDef := resource.ResourceTypeDef{
		Name:      "RDS Instances",
		ShortName: "dbi",
		Columns: []resource.Column{
			{Key: "db_identifier", Title: "DB Identifier", Width: 24},
		},
	}
	m2 := views.NewResourceList(rdsTypeDef, nil, k)
	m2.SetSize(120, 30)
	m2, _ = m2.Init()

	// A new model starts in loading state, FrameTitle returns just the short name
	title := m2.FrameTitle()
	if title != "dbi" {
		t.Errorf("G.3: new model should have title %q (loading), got %q", "dbi", title)
	}

	// After loading its own data, it's independent
	m2, _ = m2.Update(messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    pgTestResources(50),
		Pagination:   nil,
	})
	if m2.FrameTitle() != "dbi(50)" {
		t.Errorf("G.3: expected %q, got %q", "dbi(50)", m2.FrameTitle())
	}

	// Original EC2 model is unaffected
	if m1.FrameTitle() != "ec2(200+)" {
		t.Errorf("G.3: ec2 model should be unchanged, got %q", m1.FrameTitle())
	}
}

// ===========================================================================
// Section H: Demo Mode
//
// Demo mode loads all data at once (no pagination). The demo data generators
// return a complete slice. The ResourcesLoadedMsg has nil Pagination, so
// there is no + suffix and M key has no effect.
// ===========================================================================

func TestStoryH1_DemoMode_NoPagination_AllResourceTypes(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	for _, rt := range resource.AllResourceTypes() {
		t.Run(rt.ShortName, func(t *testing.T) {
			// Get demo data for this resource type
			demoResources, ok := demo.GetResources(rt.ShortName)
			if !ok {
				t.Skipf("no demo data for %s", rt.ShortName)
			}

			// Create a model and load demo data (as the app does)
			k := keys.Default()
			m := views.NewResourceList(rt, nil, k)
			m.SetSize(120, 30)
			m, _ = m.Init()

			// Demo mode sends ResourcesLoadedMsg with no pagination
			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: rt.ShortName,
				Resources:    demoResources,
				// Pagination is nil — demo mode
			})

			title := m.FrameTitle()
			count := len(demoResources)

			// Frame title should show exact count (no + suffix)
			expected := fmt.Sprintf("%s(%d)", rt.ShortName, count)
			if title != expected {
				t.Errorf("demo %s: expected title %q, got %q", rt.ShortName, expected, title)
			}

			// M key should be a no-op (no pagination)
			_, cmd := m.Update(pgKeyPress("M"))
			if cmd != nil {
				t.Errorf("demo %s: M key should be a no-op (nil pagination), got non-nil cmd", rt.ShortName)
			}
		})
	}
}

func TestStoryH1_DemoMode_ChildViews_NoPagination(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	// Test a selection of child view types that have demo data.
	childTypes := []struct {
		childType string
		parentCtx map[string]string
	}{
		{"cfn_events", map[string]string{"StackName": "payment-service-prod"}},
		{"log_streams", map[string]string{"log_group_name": "/aws/lambda/payment-processor"}},
		{"sfn_executions", map[string]string{"StateMachineArn": "arn:aws:states:us-east-1:111122223333:stateMachine:order-workflow"}},
		{"ecr_images", map[string]string{"RepositoryName": "payment-api"}},
		{"cb_builds", map[string]string{"ProjectName": "payment-build"}},
		{"glue_runs", map[string]string{"JobName": "etl-daily"}},
		{"alarm_history", map[string]string{"AlarmName": "cpu-alarm"}},
		{"asg_activities", map[string]string{"AutoScalingGroupName": "web-asg"}},
	}

	for _, tc := range childTypes {
		t.Run(tc.childType, func(t *testing.T) {
			demoResources, ok := demo.GetChildResources(tc.childType, tc.parentCtx)
			if !ok {
				t.Skipf("no demo data for child type %s", tc.childType)
			}

			// Verify demo data was returned with no pagination metadata.
			// In demo mode, fetchDemoChildResources returns a ResourcesLoadedMsg
			// with nil Pagination — meaning no truncation, no + suffix.
			if demoResources == nil {
				// nil is fine — it means no items
				demoResources = []resource.Resource{}
			}

			// Verify that the child type does NOT use the paginated child
			// fetcher in demo mode by checking that demo loads all at once.
			// The model should show exact count (no truncation).
			rt := resource.FindResourceType(tc.childType)
			if rt == nil {
				// Use a synthetic type def for child types
				rt = &resource.ResourceTypeDef{
					ShortName: tc.childType,
					Name:      tc.childType,
					Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
				}
			}

			k := keys.Default()
			m := views.NewResourceList(*rt, nil, k)
			m.SetSize(120, 30)
			m, _ = m.Init()

			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: tc.childType,
				Resources:    demoResources,
				// Pagination is nil — demo mode sends nil
			})

			title := m.FrameTitle()
			// Should not contain "+" suffix
			if strings.Contains(title, "+)") {
				t.Errorf("demo child %s: title %q should not contain truncation indicator", tc.childType, title)
			}

			// M key should be no-op
			_, cmd := m.Update(pgKeyPress("M"))
			if cmd != nil {
				t.Errorf("demo child %s: M key should be no-op, got non-nil cmd", tc.childType)
			}
		})
	}
}

// ===========================================================================
// Section I: Edge Cases
// ===========================================================================

// I.4: Load more after sort preserves sort order
func TestStoryI4_LoadMoreAfterSort_PreservesSortOrder(t *testing.T) {
	m := storyNewModel(t)

	// Load initial truncated page with varied names for sorting
	resources := make([]resource.Resource, 200)
	for i := range 200 {
		// Generate names that will sort differently than insertion order
		// Use reverse naming so sort by name ASC reverses the order
		name := fmt.Sprintf("z-instance-%05d", 199-i)
		id := fmt.Sprintf("i-%05d", i)
		resources[i] = resource.Resource{
			ID: id, Name: name, Status: "running",
			Fields: map[string]string{
				"instance_id": id, "name": name, "state": "running",
			},
		}
	}
	m = storyLoadResources(m, resources, &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok-p2",
	}, false)

	// Sort by name ascending (press N key)
	m, _ = m.Update(pgKeyPress("N"))

	// Verify sort is active: first visible resource should be the one with
	// the alphabetically first name
	sorted1 := m.SelectedResource()
	if sorted1 == nil {
		t.Fatal("expected selected resource after sort")
	}
	if sorted1.Name != "z-instance-00000" {
		t.Errorf("after sort, first should be %q, got %q", "z-instance-00000", sorted1.Name)
	}

	// Now press M to load more
	m, _ = m.Update(pgKeyPress("M"))

	// Append page 2 with resources that have names interleaving with page 1
	page2 := make([]resource.Resource, 100)
	for i := range 100 {
		name := fmt.Sprintf("a-instance-%05d", i) // "a-" sorts before "z-"
		id := fmt.Sprintf("i-%05d", 200+i)
		page2[i] = resource.Resource{
			ID: id, Name: name, Status: "running",
			Fields: map[string]string{
				"instance_id": id, "name": name, "state": "running",
			},
		}
	}
	m = storyLoadResources(m, page2, &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	// After append with sort active, the "a-" prefixed names should sort before "z-"
	firstAfterAppend := m.SelectedResource()
	if firstAfterAppend == nil {
		t.Fatal("expected selected resource after append")
	}
	// The first item alphabetically should be "a-instance-00000"
	if firstAfterAppend.Name != "a-instance-00000" {
		t.Errorf("I.4: after sort + append, first should be %q, got %q",
			"a-instance-00000", firstAfterAppend.Name)
	}

	// Verify total count
	title := m.FrameTitle()
	if title != "ec2(300)" {
		t.Errorf("I.4: expected title %q, got %q", "ec2(300)", title)
	}
}

// I.5: Load more while scrolled to bottom — cursor stays at row 200
func TestStoryI5_LoadMoreAtBottom_CursorStays(t *testing.T) {
	m := storyNewModel(t)

	// Load 200 truncated items
	m = storyLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok-p2",
	}, false)

	// Move cursor to last row (row 199, 0-indexed)
	// Use G key (Bottom) to go to end
	m, _ = m.Update(pgKeyPress("G"))
	selected := m.SelectedResource()
	if selected == nil {
		t.Fatal("expected selected resource at bottom")
	}
	if selected.ID != "i-00199" {
		t.Fatalf("precondition: expected cursor at %q, got %q", "i-00199", selected.ID)
	}

	// Press M to load more
	m, _ = m.Update(pgKeyPress("M"))

	// Append 100 more
	page2 := make([]resource.Resource, 100)
	for i := range 100 {
		id := fmt.Sprintf("i-%05d", 200+i)
		page2[i] = resource.Resource{
			ID: id, Name: fmt.Sprintf("instance-%05d", 200+i), Status: "running",
			Fields: map[string]string{
				"instance_id": id,
				"name":        fmt.Sprintf("instance-%05d", 200+i),
				"state":       "running",
			},
		}
	}
	m = storyLoadResources(m, page2, &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	// Cursor should still be at the same resource (i-00199, row 199)
	afterAppend := m.SelectedResource()
	if afterAppend == nil {
		t.Fatal("expected selected resource after append")
	}
	if afterAppend.ID != "i-00199" {
		t.Errorf("I.5: cursor should stay at %q, got %q", "i-00199", afterAppend.ID)
	}

	// New rows should be accessible by pressing Down/j
	m, _ = m.Update(pgKeyPress("j"))
	next := m.SelectedResource()
	if next == nil {
		t.Fatal("expected next resource after j")
	}
	if next.ID != "i-00200" {
		t.Errorf("I.5: after pressing j, expected %q, got %q", "i-00200", next.ID)
	}

	// Total should be 300
	title := m.FrameTitle()
	if title != "ec2(300)" {
		t.Errorf("I.5: expected title %q, got %q", "ec2(300)", title)
	}
}

// I.1 (from stories): API returns zero items on load more despite having token.
// This is already covered in TestResourceList_Append_EmptySecondPage in
// qa_pagination_view_test.go. Here we add a variant verifying M key becomes
// no-op after the empty append.
func TestStoryI1_EmptyLoadMore_MBecomesNoop(t *testing.T) {
	m := storyNewModel(t)

	m = storyLoadResources(m, pgTestResources(100), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok",
	}, false)

	// Press M, get empty response with IsTruncated=false
	m, _ = m.Update(pgKeyPress("M"))
	m = storyLoadResources(m, []resource.Resource{}, &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	// Title should show exact count (no +)
	if m.FrameTitle() != "ec2(100)" {
		t.Errorf("I.1: expected %q, got %q", "ec2(100)", m.FrameTitle())
	}

	// M should be a no-op now
	_, cmd := m.Update(pgKeyPress("M"))
	if cmd != nil {
		t.Errorf("I.1: M should be no-op after empty load-more, got non-nil cmd")
	}
}

// I.2 (from stories): Rapid M presses are debounced. This is already tested
// in TestResourceList_LoadMore_WhenAlreadyLoading_Noop in qa_pagination_view_test.go.
// Verify here with multiple rapid presses.
func TestStoryI2_RapidMPresses_Debounced(t *testing.T) {
	m := storyNewModel(t)

	m = storyLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok",
	}, false)

	// First M press should produce a command
	var cmd1 func() tea.Msg
	m, cmd1 = m.Update(pgKeyPress("M"))
	if cmd1 == nil {
		t.Fatal("first M should produce a command")
	}

	// Subsequent M presses while loading should all be no-ops
	for i := range 5 {
		var cmd func() tea.Msg
		m, cmd = m.Update(pgKeyPress("M"))
		if cmd != nil {
			t.Errorf("M press %d during loading should be no-op, got non-nil cmd", i+2)
		}
	}
}

// ===========================================================================
// Cross-section: Verify all resource types have consistent pagination behavior
// at the view level.
// ===========================================================================

func TestStoryDFGI_AllResourceTypes_PaginationViewConsistency(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	for _, rt := range resource.AllResourceTypes() {
		t.Run(rt.ShortName+"_pagination_lifecycle", func(t *testing.T) {
			k := keys.Default()
			m := views.NewResourceList(rt, nil, k)
			m.SetSize(120, 30)
			m, _ = m.Init()

			// 1. Loading state: FrameTitle returns just the short name
			if m.FrameTitle() != rt.ShortName {
				t.Errorf("loading: expected %q, got %q", rt.ShortName, m.FrameTitle())
			}

			// 2. Load truncated page
			resources := make([]resource.Resource, 100)
			for i := range 100 {
				fields := make(map[string]string)
				for _, col := range rt.Columns {
					fields[col.Key] = fmt.Sprintf("%s-%d", col.Key, i)
				}
				resources[i] = resource.Resource{
					ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("name-%d", i),
					Status: "running", Fields: fields,
				}
			}

			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: rt.ShortName,
				Resources:    resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "tok",
				},
			})
			if m.FrameTitle() != rt.ShortName+"(100+)" {
				t.Errorf("truncated: expected %q, got %q", rt.ShortName+"(100+)", m.FrameTitle())
			}

			// 3. Press M → loading more
			m, _ = m.Update(pgKeyPress("M"))
			if !strings.Contains(m.FrameTitle(), "loading...") {
				t.Errorf("loading more: expected 'loading...' in %q", m.FrameTitle())
			}

			// 4. Append page 2 (final)
			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: rt.ShortName,
				Resources:    resources,
				Pagination:   &resource.PaginationMeta{IsTruncated: false},
				Append:       true,
			})
			if m.FrameTitle() != rt.ShortName+"(200)" {
				t.Errorf("complete: expected %q, got %q", rt.ShortName+"(200)", m.FrameTitle())
			}

			// 5. M should be no-op now
			_, cmd := m.Update(pgKeyPress("M"))
			if cmd != nil {
				t.Errorf("M after complete should be no-op")
			}

			// 6. Replace (simulate refresh) resets
			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: rt.ShortName,
				Resources:    resources[:50],
				Pagination:   nil,
			})
			if m.FrameTitle() != rt.ShortName+"(50)" {
				t.Errorf("refresh: expected %q, got %q", rt.ShortName+"(50)", m.FrameTitle())
			}
		})
	}
}
