package unit

// aws_enrichment_wave4b_test.go — Targets the 1-4 uncovered branches in each
// Wave 2 enricher: EnrichIAMRoleLastUsed, EnrichIAMPolicy, EnrichIAMGroup,
// EnrichASGScalingActivities, EnrichCodePipelineStatus, plus full coverage of
// the pure helpers isMSKVersionOutdated and parseVersionPart.
//
// Each test covers exactly one previously-uncovered branch; no tautologies.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ─── EnrichIAMRoleLastUsed — uncovered branches ──────────────────────────────

// iamGetRoleFakeWithNilRole implements IAMAPI returning GetRoleOutput with
// Role=nil for the named role, simulating an empty but non-error response.
type iamGetRoleFakeWithNilRole struct {
	awsclient.IAMAPI
	nilRoleName string
}

func (f *iamGetRoleFakeWithNilRole) GetRole(
	_ context.Context,
	in *iam.GetRoleInput,
	_ ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	if in != nil && in.RoleName != nil && *in.RoleName == f.nilRoleName {
		return &iam.GetRoleOutput{Role: nil}, nil
	}
	return &iam.GetRoleOutput{}, nil
}

var _ awsclient.IAMAPI = (*iamGetRoleFakeWithNilRole)(nil)

// iamBareAPI implements IAMAPI but does NOT implement IAMGetRoleAPI (no GetRole).
// Used to exercise the !ok type-assertion branch in EnrichIAMRoleLastUsed.
type iamBareAPI struct {
	awsclient.IAMAPI
}

var _ awsclient.IAMAPI = (*iamBareAPI)(nil)

// TestEnrichIAMRoleLastUsed_NilRoleOutputSkipped verifies that when GetRole
// returns a non-error response with Role=nil, no finding is produced for that
// role. This exercises the `if out.Role == nil { continue }` branch.
func TestEnrichIAMRoleLastUsed_NilRoleOutputSkipped(t *testing.T) {
	fake := &iamGetRoleFakeWithNilRole{nilRoleName: "ghost-role"}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{
		{
			ID:   "ghost-role",
			Name: "ghost-role",
			Fields: map[string]string{
				"role_name": "ghost-role",
				"path":      "/",
			},
		},
	}

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings when Role is nil, got %d", len(result.Findings))
	}
}

// TestEnrichIAMRoleLastUsed_APIErrorSetsTruncatedContinues verifies that a
// GetRole API error marks Truncated=true, adds the role to TruncatedIDs, and
// does not propagate the error. A second role that is dormant still produces a
// finding — confirming the loop continues past the error.
func TestEnrichIAMRoleLastUsed_APIErrorSetsTruncatedContinues(t *testing.T) {
	// broken-role errors on GetRole; dormant-role has nil RoleLastUsed → finding.
	combo := &iamGetRoleFakeCombo{
		errForRole:     "broken-role",
		nilLastUsedFor: map[string]bool{"dormant-role": true},
	}
	clients := &awsclient.ServiceClients{IAM: combo}
	resources := []resource.Resource{
		{
			ID:   "broken-role",
			Name: "broken-role",
			Fields: map[string]string{
				"role_name": "broken-role",
				"path":      "/",
			},
		},
		{
			ID:   "dormant-role",
			Name: "dormant-role",
			Fields: map[string]string{
				"role_name": "dormant-role",
				"path":      "/",
			},
		},
	}

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("enricher must not propagate GetRole errors: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when at least one GetRole call failed")
	}
	if _, ok := result.TruncatedIDs["broken-role"]; !ok {
		t.Error("TruncatedIDs must contain broken-role")
	}
	if _, ok := result.Findings["dormant-role"]; !ok {
		t.Error("dormant-role must still produce a finding even after broken-role errored")
	}
}

// iamGetRoleFakeCombo handles both error-for-a-role and nil-last-used-for-a-role cases.
type iamGetRoleFakeCombo struct {
	awsclient.IAMAPI
	errForRole     string
	nilLastUsedFor map[string]bool
}

func (f *iamGetRoleFakeCombo) GetRole(
	_ context.Context,
	in *iam.GetRoleInput,
	_ ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	name := ""
	if in != nil && in.RoleName != nil {
		name = *in.RoleName
	}
	if name == f.errForRole {
		return nil, errors.New("iam: NoSuchEntity for " + name)
	}
	role := &iamtypes.Role{RoleName: aws.String(name)}
	if f.nilLastUsedFor[name] {
		role.RoleLastUsed = nil
	} else {
		recent := time.Now().Add(-1 * time.Hour)
		role.RoleLastUsed = &iamtypes.RoleLastUsed{LastUsedDate: &recent}
	}
	return &iam.GetRoleOutput{Role: role}, nil
}

var _ awsclient.IAMAPI = (*iamGetRoleFakeCombo)(nil)

// TestEnrichIAMRoleLastUsed_RoleNameFallsBackToID verifies that when a resource
// has no "role_name" field, the enricher uses r.ID as the role name for the
// GetRole call. The dormant role is identified by its ID.
func TestEnrichIAMRoleLastUsed_RoleNameFallsBackToID(t *testing.T) {
	fake := &iamGetRoleFakeCombo{
		nilLastUsedFor: map[string]bool{"role-by-id-only": true},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	// No "role_name" in Fields — only r.ID is set.
	resources := []resource.Resource{
		{
			ID:     "role-by-id-only",
			Name:   "role-by-id-only",
			Fields: map[string]string{"path": "/"},
		},
	}

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["role-by-id-only"]; !ok {
		t.Errorf("expected finding for role whose name was resolved from r.ID, got %d findings", len(result.Findings))
	}
}

// TestEnrichIAMRoleLastUsed_EmptyIDAndNameSkipped verifies that a resource with
// both an empty role_name field and an empty r.ID is silently skipped — no
// finding, no panic.
func TestEnrichIAMRoleLastUsed_EmptyIDAndNameSkipped(t *testing.T) {
	fake := &iamGetRoleFakeCombo{}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{
		{
			ID:     "",
			Name:   "no-id-role",
			Fields: map[string]string{"path": "/"},
		},
	}

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for empty-ID resource, got %d", len(result.Findings))
	}
}

// TestEnrichIAMRoleLastUsed_IAMClientNotGetRoleAPI verifies that when the IAM
// client does not implement IAMGetRoleAPI, the enricher returns empty findings
// with no error (type-assertion guard).
func TestEnrichIAMRoleLastUsed_IAMClientNotGetRoleAPI(t *testing.T) {
	bare := &iamBareAPI{}
	clients := &awsclient.ServiceClients{IAM: bare}

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, iamRoleResources())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings when IAM client lacks GetRole, got %d", len(result.Findings))
	}
}

// ─── EnrichIAMPolicy — uncovered branches ────────────────────────────────────

// TestEnrichIAMPolicy_APIErrorSetsTruncatedNoError verifies that when
// FetchManagedPolicyDocument returns an error for a policy, that policy is added
// to TruncatedIDs, Truncated is set, but the enricher does not return an error.
// A second safe policy is still processed.
func TestEnrichIAMPolicy_APIErrorSetsTruncatedNoError(t *testing.T) {
	fake := &iamPolicyFake{
		getPolicyResults: map[string]*iam.GetPolicyOutput{
			iamPolicyARN1: iamPolicyGetPolicyOutput(iamPolicyARN1, "v1"),
		},
		getPolicyVersionResults: map[string]*iam.GetPolicyVersionOutput{
			iamPolicyARN1: iamPolicyGetVersionOutput(safePolicyDoc),
		},
		errByArn: map[string]error{
			iamPolicyARN2: errors.New("iam: NoSuchEntity for " + iamPolicyARN2),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{
		iamPolicyResource(iamPolicyARN2, "BrokenPolicy"),
		iamPolicyResource(iamPolicyARN1, "SafePolicy"),
	}

	result, err := awsclient.EnrichIAMPolicy(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("enricher must not propagate FetchManagedPolicyDocument errors: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when at least one policy fetch failed")
	}
	if _, ok := result.TruncatedIDs[iamPolicyARN2]; !ok {
		t.Errorf("TruncatedIDs must contain %s", iamPolicyARN2)
	}
	// Safe policy was processed OK — no finding.
	if _, ok := result.Findings[iamPolicyARN1]; ok {
		t.Error("safe policy must NOT appear in Findings")
	}
}

// TestEnrichIAMPolicy_NoRawStructARNFallbackFromID verifies the ARN fallback
// path: when r.RawStruct is not an iamtypes.Policy (extractIAMPolicyARN returns
// false) but r.ID itself starts with "arn:", the enricher uses r.ID as the ARN
// and still detects the admin-star policy.
func TestEnrichIAMPolicy_NoRawStructARNFallbackFromID(t *testing.T) {
	fake := &iamPolicyFake{
		getPolicyResults: map[string]*iam.GetPolicyOutput{
			iamPolicyARN1: iamPolicyGetPolicyOutput(iamPolicyARN1, "v1"),
		},
		getPolicyVersionResults: map[string]*iam.GetPolicyVersionOutput{
			iamPolicyARN1: iamPolicyGetVersionOutput(adminStarPolicyDoc),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	// RawStruct is nil — forces extractIAMPolicyARN to return ("", false).
	// r.ID is a valid ARN so the fallback kicks in.
	resources := []resource.Resource{
		{
			ID:     iamPolicyARN1,
			Name:   "ArbitraryPolicy",
			Fields: map[string]string{"attachment_count": "1"},
			// RawStruct intentionally left nil.
		},
	}

	result, err := awsclient.EnrichIAMPolicy(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[iamPolicyARN1]; !ok {
		t.Errorf("expected admin-star finding via ARN fallback from r.ID, got %d findings", len(result.Findings))
	}
	if result.Findings[iamPolicyARN1].Severity != "!" {
		t.Errorf("severity = %q, want %q", result.Findings[iamPolicyARN1].Severity, "!")
	}
}

// TestEnrichIAMPolicy_EmptyARNSkipped verifies that a resource whose
// extractIAMPolicyARN returns empty and whose r.ID does NOT start with "arn:"
// is silently skipped — no finding, no panic.
func TestEnrichIAMPolicy_EmptyARNSkipped(t *testing.T) {
	fake := &iamPolicyFake{
		getPolicyResults:        map[string]*iam.GetPolicyOutput{},
		getPolicyVersionResults: map[string]*iam.GetPolicyVersionOutput{},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{
		{
			ID:     "not-an-arn",
			Name:   "UnresolvablePolicy",
			Fields: map[string]string{},
			// RawStruct nil, r.ID not an ARN → policyARN stays empty.
		},
	}

	result, err := awsclient.EnrichIAMPolicy(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for unresolvable ARN resource, got %d", len(result.Findings))
	}
}

// ─── EnrichIAMGroup — uncovered branches ────────────────────────────────────

// iamGroupErrorFake is like iamGroupFake but can return errors per group name.
type iamGroupErrorFake struct {
	awsclient.IAMAPI

	usersByGroup            map[string][]iamtypes.User
	attachedPoliciesByGroup map[string][]iamtypes.AttachedPolicy
	inlinePoliciesByGroup   map[string][]string

	getGroupErrFor            string
	listAttachedErrFor        string
	listInlineErrFor          string
}

func (f *iamGroupErrorFake) GetGroup(
	_ context.Context,
	in *iam.GetGroupInput,
	_ ...func(*iam.Options),
) (*iam.GetGroupOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	if f.getGroupErrFor == name {
		return nil, errors.New("iam: NoSuchEntity GetGroup for " + name)
	}
	users := f.usersByGroup[name]
	return &iam.GetGroupOutput{
		Group: &iamtypes.Group{GroupName: aws.String(name)},
		Users: users,
	}, nil
}

func (f *iamGroupErrorFake) ListAttachedGroupPolicies(
	_ context.Context,
	in *iam.ListAttachedGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListAttachedGroupPoliciesOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	if f.listAttachedErrFor == name {
		return nil, errors.New("iam: AccessDenied ListAttachedGroupPolicies for " + name)
	}
	policies := f.attachedPoliciesByGroup[name]
	return &iam.ListAttachedGroupPoliciesOutput{AttachedPolicies: policies}, nil
}

func (f *iamGroupErrorFake) ListGroupPolicies(
	_ context.Context,
	in *iam.ListGroupPoliciesInput,
	_ ...func(*iam.Options),
) (*iam.ListGroupPoliciesOutput, error) {
	name := ""
	if in != nil && in.GroupName != nil {
		name = *in.GroupName
	}
	if f.listInlineErrFor == name {
		return nil, errors.New("iam: AccessDenied ListGroupPolicies for " + name)
	}
	names := f.inlinePoliciesByGroup[name]
	return &iam.ListGroupPoliciesOutput{PolicyNames: names}, nil
}

var _ awsclient.IAMAPI = (*iamGroupErrorFake)(nil)

// TestEnrichIAMGroup_GetGroupAPIErrorSkipsGroup verifies that when GetGroup
// returns an error for a group, that group is skipped (no finding produced),
// Truncated is set, and processing continues for the next group.
func TestEnrichIAMGroup_GetGroupAPIErrorSkipsGroup(t *testing.T) {
	fake := &iamGroupErrorFake{
		getGroupErrFor: "broken-group",
		usersByGroup: map[string][]iamtypes.User{
			"ok-group": {},
		},
		attachedPoliciesByGroup: map[string][]iamtypes.AttachedPolicy{
			"ok-group": {iamAttachedPolicy("arn:aws:iam::aws:policy/ReadOnlyAccess", "ReadOnlyAccess")},
		},
		inlinePoliciesByGroup: map[string][]string{},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamGroupResources("broken-group", "ok-group")

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("enricher must not propagate GetGroup errors: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when at least one GetGroup call failed")
	}
	// broken-group errored on first call → memberFirstCallErrd=true → skipped from findings.
	if _, ok := result.Findings["broken-group"]; ok {
		t.Error("broken-group must NOT appear in Findings when GetGroup errored")
	}
	// ok-group has members but no members (0 users) → finds orphan finding.
	// Actually ok-group has 0 users → should produce a "no members" finding.
	if _, ok := result.Findings["ok-group"]; !ok {
		t.Error("ok-group with 0 members should still produce a finding")
	}
}

// TestEnrichIAMGroup_EmptyGroupNameSkipped verifies that a resource with both
// an empty group_name field and an empty r.ID is silently skipped — no finding,
// no panic.
func TestEnrichIAMGroup_EmptyGroupNameSkipped(t *testing.T) {
	fake := &iamGroupFake{
		usersByGroup:            map[string][]iamtypes.User{},
		attachedPoliciesByGroup: map[string][]iamtypes.AttachedPolicy{},
		inlinePoliciesByGroup:   map[string][]string{},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{
		{
			ID:     "",
			Name:   "no-id-group",
			Fields: map[string]string{},
		},
	}

	result, err := awsclient.EnrichIAMGroup(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for empty-ID group resource, got %d", len(result.Findings))
	}
}

// ─── EnrichASGScalingActivities — uncovered branches ─────────────────────────

// TestEnrichASGScalingActivities_EmptyActivitiesProducesNoFindings verifies
// that an ASG whose DescribeScalingActivities returns an empty Activities slice
// produces no finding. This exercises the `len(out.Activities) == 0` branch.
func TestEnrichASGScalingActivities_EmptyActivitiesProducesNoFindings(t *testing.T) {
	fake := &asgScalingActivitiesFake{
		activities: map[string][]asgtypes.Activity{
			"quiet-asg": {}, // API returns success but no activities yet.
		},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		{ID: "quiet-asg", Name: "quiet-asg"},
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for ASG with no activities, got %d", len(result.Findings))
	}
}

// TestEnrichASGScalingActivities_FailedWithStatusMessageSummarized verifies
// that when a failed activity has a non-nil StatusMessage, the summary includes
// that message and a "Message" row is appended.
func TestEnrichASGScalingActivities_FailedWithStatusMessageSummarized(t *testing.T) {
	startTime := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	cause := "At 2025-03-15T00:00:00Z an instance was started."
	statusMsg := "InsufficientInstanceCapacity"

	fake := &asgScalingActivitiesFake{
		activities: map[string][]asgtypes.Activity{
			"capacity-asg": {
				{
					ActivityId:           aws.String("act-cap-1"),
					AutoScalingGroupName: aws.String("capacity-asg"),
					StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
					StatusMessage:        aws.String(statusMsg),
					Cause:                aws.String(cause),
					StartTime:            &startTime,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		{ID: "capacity-asg", Name: "capacity-asg"},
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["capacity-asg"]
	if !ok {
		t.Fatalf("expected finding for capacity-asg, got none")
	}
	// Summary must contain the status message.
	if !strings.Contains(f.Summary, statusMsg) {
		t.Errorf("summary %q must contain status message %q", f.Summary, statusMsg)
	}
	// Rows must include a "Message" row.
	var hasMessageRow, hasCauseRow, hasStartedRow bool
	for _, row := range f.Rows {
		switch row.Label {
		case "Message":
			hasMessageRow = true
			if row.Value != statusMsg {
				t.Errorf("Message row value = %q, want %q", row.Value, statusMsg)
			}
		case "Cause":
			hasCauseRow = true
			if row.Value != cause {
				t.Errorf("Cause row value = %q, want %q", row.Value, cause)
			}
		case "Started":
			hasStartedRow = true
			if row.Value != "2025-03-15" {
				t.Errorf("Started row value = %q, want %q", row.Value, "2025-03-15")
			}
		}
	}
	if !hasMessageRow {
		t.Error("rows must contain a Message label when StatusMessage is set")
	}
	if !hasCauseRow {
		t.Error("rows must contain a Cause label when Cause is set")
	}
	if !hasStartedRow {
		t.Error("rows must contain a Started label when StartTime is set")
	}
}

// TestEnrichASGScalingActivities_FailedWithoutStatusMessagePlainSummary verifies
// that when a failed activity has StatusMessage=nil, the summary is the plain
// "latest scaling activity failed" (no colon+message) and no "Message" row is appended.
func TestEnrichASGScalingActivities_FailedWithoutStatusMessagePlainSummary(t *testing.T) {
	fake := &asgScalingActivitiesFake{
		activities: map[string][]asgtypes.Activity{
			"silent-fail-asg": {
				{
					ActivityId:           aws.String("act-silent-1"),
					AutoScalingGroupName: aws.String("silent-fail-asg"),
					StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
					StatusMessage:        nil,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		{ID: "silent-fail-asg", Name: "silent-fail-asg"},
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["silent-fail-asg"]
	if !ok {
		t.Fatalf("expected finding for silent-fail-asg")
	}
	if f.Summary != "latest scaling activity failed" {
		t.Errorf("summary = %q, want %q", f.Summary, "latest scaling activity failed")
	}
	for _, row := range f.Rows {
		if row.Label == "Message" {
			t.Error("rows must NOT contain a Message label when StatusMessage is nil")
		}
	}
}

// TestEnrichASGScalingActivities_EmptyIDSkipped verifies that a resource with
// an empty ID is silently skipped — the ASG name is derived from r.ID and an
// empty name would produce an invalid API call.
func TestEnrichASGScalingActivities_EmptyIDSkipped(t *testing.T) {
	fake := &asgScalingActivitiesFake{
		activities: map[string][]asgtypes.Activity{},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fake}
	resources := []resource.Resource{
		{ID: "", Name: "unnamed-asg"}, // empty ID → must be skipped
	}

	result, err := awsclient.EnrichASGScalingActivities(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings when r.ID is empty, got %d", len(result.Findings))
	}
}

// ─── EnrichCodePipelineStatus — uncovered branches ───────────────────────────

// TestEnrichCodePipelineStatus_EmptyNameSkipped verifies that a resource with
// an empty Name is silently skipped and produces no finding.
func TestEnrichCodePipelineStatus_EmptyNameSkipped(t *testing.T) {
	fake := &pipelineStateFake{states: map[string]*codepipeline.GetPipelineStateOutput{}}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{
		{ID: "pipe-id", Name: ""}, // empty name → must be skipped
	}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for empty-name pipeline, got %d", len(result.Findings))
	}
}

// TestEnrichCodePipelineStatus_EmptyIDKeyedByName verifies that when r.ID is
// empty, the finding is keyed by r.Name (the fallback key path). This tests the
// `key := r.Name` fallback branch.
func TestEnrichCodePipelineStatus_EmptyIDKeyedByName(t *testing.T) {
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"nameless-id-pipeline": {
				StageStates: []cptypes.StageState{
					stageState("Deploy", cptypes.StageExecutionStatusFailed),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{
		{ID: "", Name: "nameless-id-pipeline"},
	}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["nameless-id-pipeline"]; !ok {
		t.Errorf("expected finding keyed by r.Name %q when r.ID is empty, keys: %v",
			"nameless-id-pipeline", result.Findings)
	}
}

// TestEnrichCodePipelineStatus_ActionErrorDetailsAppended verifies that when a
// failed stage has an action with LatestExecution.Status=Failed and non-nil
// ErrorDetails.Message, an "Error" row is appended to the finding rows.
func TestEnrichCodePipelineStatus_ActionErrorDetailsAppended(t *testing.T) {
	errMsg := "Build step exited with code 1"
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"err-detail-pipeline": {
				StageStates: []cptypes.StageState{
					{
						StageName: aws.String("Build"),
						LatestExecution: &cptypes.StageExecution{
							Status: cptypes.StageExecutionStatusFailed,
						},
						ActionStates: []cptypes.ActionState{
							{
								LatestExecution: &cptypes.ActionExecution{
									Status: cptypes.ActionExecutionStatusFailed,
									ErrorDetails: &cptypes.ErrorDetails{
										Message: aws.String(errMsg),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{
		{ID: "err-pipe-id", Name: "err-detail-pipeline"},
	}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["err-pipe-id"]
	if !ok {
		t.Fatalf("expected finding for err-pipe-id, got none")
	}
	var hasErrorRow bool
	for _, row := range f.Rows {
		if row.Label == "Error" {
			hasErrorRow = true
			if row.Value != errMsg {
				t.Errorf("Error row value = %q, want %q", row.Value, errMsg)
			}
		}
	}
	if !hasErrorRow {
		t.Error("finding rows must contain an Error label when action has ErrorDetails.Message")
	}
}

// TestEnrichCodePipelineStatus_ActionNilExecutionSkipped verifies that action
// states with nil LatestExecution are skipped without panic.
func TestEnrichCodePipelineStatus_ActionNilExecutionSkipped(t *testing.T) {
	fake := &pipelineStateFake{
		states: map[string]*codepipeline.GetPipelineStateOutput{
			"nil-exec-pipeline": {
				StageStates: []cptypes.StageState{
					{
						StageName: aws.String("Deploy"),
						LatestExecution: &cptypes.StageExecution{
							Status: cptypes.StageExecutionStatusFailed,
						},
						ActionStates: []cptypes.ActionState{
							{LatestExecution: nil}, // must not panic
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{CodePipeline: fake}
	resources := []resource.Resource{
		{ID: "nil-exec-pipe", Name: "nil-exec-pipeline"},
	}

	result, err := awsclient.EnrichCodePipelineStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["nil-exec-pipe"]
	if !ok {
		t.Fatalf("expected finding for nil-exec-pipe")
	}
	for _, row := range f.Rows {
		if row.Label == "Error" {
			t.Error("no Error row should be appended when action LatestExecution is nil")
		}
	}
}

// ─── isMSKVersionOutdated / parseVersionPart — via EnrichMSKCluster ──────────
//
// isMSKVersionOutdated and parseVersionPart are unexported. They are exercised
// indirectly via EnrichMSKCluster, which calls isMSKVersionOutdated for each
// provisioned cluster's KafkaVersion string. A finding with Summary
// "broker software outdated" is produced only when the version is outdated.

const mskARNForVersionTests = "arn:aws:kafka:us-east-1:123456789012:cluster/msk-vtest/vvvvvvvv"

// TestEnrichMSKCluster_VersionBoundaries exercises all branches of the internal
// isMSKVersionOutdated and parseVersionPart helpers via EnrichMSKCluster with
// different KafkaVersion strings.
//
// Cutoff contract: major < 2 OR (major == 2 AND minor < 8) → outdated.
// Malformed or single-part versions are treated as up-to-date (return false).
func TestEnrichMSKCluster_VersionBoundaries(t *testing.T) {
	cases := []struct {
		name         string
		version      string
		wantOutdated bool
	}{
		// modern — no outdated finding
		{"modern_3_5_1", "3.5.1", false},
		{"cutoff_2_8_0", "2.8.0", false},
		{"above_cutoff_2_8_1", "2.8.1", false},
		{"minor_above_cutoff_2_9_0", "2.9.0", false},
		// outdated — finding expected
		{"just_below_cutoff_2_7_1", "2.7.1", true},
		{"old_2_0_0", "2.0.0", true},
		{"major_1", "1.11.13", true},
		{"major_0", "0.11.0", true},
		// malformed — parseVersionPart error → treated as modern
		{"malformed_alpha_major", "not.a.version", false},
		{"malformed_alpha_minor", "2.x.0", false},
		// too few parts (len(parts) < 2) → return false
		{"single_part", "2", false},
		{"empty_string", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			arn := mskARNForVersionTests
			// Use TLS so no encryption finding masks the version result.
			fake := &mskDescribeClusterV2Fake{
				results: map[string]*kafkatypes.Cluster{
					arn: provisionedCluster(arn, tc.version, kafkatypes.ClientBrokerTls),
				},
			}
			clients := &awsclient.ServiceClients{MSK: fake}
			resources := mskClusterResources(arn)

			result, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			f, hasFinding := result.Findings[arn]
			isOutdatedFinding := hasFinding && f.Summary == "broker software outdated"

			if tc.wantOutdated && !isOutdatedFinding {
				summary := ""
				if hasFinding {
					summary = f.Summary
				}
				t.Errorf("version %q: expected 'broker software outdated' finding, got hasFinding=%v summary=%q",
					tc.version, hasFinding, summary)
			}
			if !tc.wantOutdated && isOutdatedFinding {
				t.Errorf("version %q: expected no outdated finding, got one with summary %q",
					tc.version, f.Summary)
			}
		})
	}
}
