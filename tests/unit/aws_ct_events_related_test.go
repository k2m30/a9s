package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_CtEvents_Registered(t *testing.T) {
	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ct-events")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"role":     {"IAM Roles", true},
		"iam-user": {"IAM Users", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ct-events %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ct-events %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ct-events %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// ctEventsCheckerByTarget returns the RelatedChecker for the given target type registered
// under "ct-events". It fails the test immediately if the checker is nil or not found.
func ctEventsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ct-events") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ct-events related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ct-events related checker for %s not found", target)
	return nil
}

// --- IAM User checker tests (match by Fields["user"] == resource.Name) ---

func TestRelated_CtEvents_User_MatchByUsername(t *testing.T) {
	userRes := resource.Resource{
		ID:     "AIDAEXAMPLE",
		Name:   "admin-user",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"iam-user": resource.ResourceCacheEntry{Resources: []resource.Resource{userRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60001",
		Fields: map[string]string{"user": "admin-user"},
	}

	checker := ctEventsCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CtEvents_User_NoMatch(t *testing.T) {
	userRes := resource.Resource{
		ID:     "AIDAEXAMPLE",
		Name:   "other-user",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"iam-user": resource.ResourceCacheEntry{Resources: []resource.Resource{userRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60002",
		Fields: map[string]string{"user": "admin-user"},
	}

	checker := ctEventsCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CtEvents_User_EmptyUser(t *testing.T) {
	userRes := resource.Resource{
		ID:     "AIDAEXAMPLE",
		Name:   "admin-user",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"iam-user": resource.ResourceCacheEntry{Resources: []resource.Resource{userRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60003",
		Fields: map[string]string{"user": ""},
	}

	checker := ctEventsCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty user field)", result.Count)
	}
}

func TestRelated_CtEvents_User_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60004",
		Fields: map[string]string{"user": "admin-user"},
	}

	checker := ctEventsCheckerByTarget(t, "iam-user")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

// --- IAM Role checker tests (match by RawStruct Resources list — AWS::IAM::Role) ---

func TestRelated_CtEvents_Role_MatchByResource(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "my-role",
		Name:   "my-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60005",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceType: aws.String("AWS::IAM::Role"),
					ResourceName: aws.String("arn:aws:iam::123:role/my-role"),
				},
			},
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CtEvents_Role_NoMatch(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "other-role",
		Name:   "other-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60006",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceType: aws.String("AWS::IAM::Role"),
					ResourceName: aws.String("arn:aws:iam::123:role/my-role"),
				},
			},
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CtEvents_Role_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "evt-0a1b2c3d4e5f60007",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceType: aws.String("AWS::IAM::Role"),
					ResourceName: aws.String("arn:aws:iam::123:role/my-role"),
				},
			},
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

func TestRelatedDemo_CtEvents_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ct-events")
	if checker == nil {
		t.Fatal("no demo checker registered for ct-events")
	}

	results := checker(resource.Resource{ID: "evt-0a1b2c3d4e5f60001"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}

// ---------------------------------------------------------------------------
// FetchFilter propagation: checkIAMUserCtEvents (iam-user → ct-events)
// ---------------------------------------------------------------------------

// TestRelated_CtEvents_IAMUser_FetchFilterSet verifies that checkIAMUserCtEvents
// sets FetchFilter["Username"] to the user's ID on all return paths (match found).
func TestRelated_CtEvents_IAMUser_FetchFilterSet(t *testing.T) {
	userName := "alice"
	eventRes := resource.Resource{
		ID:     "evt-iam-user-001",
		Fields: map[string]string{"user": userName},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{eventRes}},
	}
	iamUser := resource.Resource{
		ID:   userName,
		Name: userName,
	}

	checker := iamUserCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, iamUser, cache)

	if result.Count <= 0 {
		t.Errorf("Count = %d, want > 0 (event matched)", result.Count)
	}
	if result.FetchFilter == nil {
		t.Fatal("FetchFilter is nil, want non-nil")
	}
	if result.FetchFilter["Username"] != userName {
		t.Errorf("FetchFilter[Username] = %q, want %q", result.FetchFilter["Username"], userName)
	}
}

// TestRelated_CtEvents_IAMUser_FetchFilterSet_NoMatch_Truncated verifies that
// FetchFilter["Username"] is set even when no match is found in a truncated cache.
func TestRelated_CtEvents_IAMUser_FetchFilterSet_NoMatch_Truncated(t *testing.T) {
	userName := "alice"
	// Cache has no matching event but is marked truncated.
	unrelatedEvent := resource.Resource{
		ID:     "evt-iam-user-002",
		Fields: map[string]string{"user": "other-user"},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{unrelatedEvent},
			IsTruncated: true,
		},
	}
	iamUser := resource.Resource{
		ID:   userName,
		Name: userName,
	}

	checker := iamUserCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, iamUser, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no match, truncated)", result.Count)
	}
	if result.FetchFilter == nil {
		t.Fatal("FetchFilter is nil, want non-nil")
	}
	if result.FetchFilter["Username"] != userName {
		t.Errorf("FetchFilter[Username] = %q, want %q", result.FetchFilter["Username"], userName)
	}
}

// TestRelated_CtEvents_IAMUser_FetchFilterSet_EmptyUsername verifies that
// FetchFilter is nil (not set) when the source resource has an empty ID.
func TestRelated_CtEvents_IAMUser_FetchFilterSet_EmptyUsername(t *testing.T) {
	iamUser := resource.Resource{
		ID:   "",
		Name: "",
	}
	cache := resource.ResourceCache{}

	checker := iamUserCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, iamUser, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty username)", result.Count)
	}
	if len(result.FetchFilter) != 0 {
		t.Errorf("FetchFilter = %v, want nil/empty (no filter for empty username)", result.FetchFilter)
	}
}

// TestRelated_CtEvents_IAMUser_FetchFilterSet_Match_Truncated is a regression
// test for the bug where checkIAMUserCtEvents returned the in-cache match count
// instead of -1 when the cache was truncated. A truncated cache means the real
// count is unknown, so -1 must be returned regardless of in-cache matches.
func TestRelated_CtEvents_IAMUser_FetchFilterSet_Match_Truncated(t *testing.T) {
	userName := "alice"
	matchingEvent := resource.Resource{
		ID:     "evt-match-001",
		Fields: map[string]string{"user": userName},
		RawStruct: cloudtrailtypes.Event{
			Username: aws.String(userName),
		},
	}
	matchingEvent2 := resource.Resource{
		ID:     "evt-match-002",
		Fields: map[string]string{"user": userName},
		RawStruct: cloudtrailtypes.Event{
			Username: aws.String(userName),
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingEvent, matchingEvent2},
			IsTruncated: true,
		},
	}
	iamUser := resource.Resource{
		ID:   userName,
		Name: userName,
	}

	checker := iamUserCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, iamUser, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache truncated, real count unknown)", result.Count)
	}
	if result.FetchFilter == nil {
		t.Fatal("FetchFilter is nil, want non-nil")
	}
	if result.FetchFilter["Username"] != userName {
		t.Errorf("FetchFilter[Username] = %q, want %q", result.FetchFilter["Username"], userName)
	}
}

// ---------------------------------------------------------------------------
// FetchFilter propagation: checkEC2CloudTrailEvents (ec2 → ct-events)
// ---------------------------------------------------------------------------

// ec2CtEventsRelatedChecker captures the ec2→ct-events checker at package init time,
// before any test can mutate the ec2 related registry.
var ec2CtEventsRelatedChecker resource.RelatedChecker

func init() {
	for _, def := range resource.GetRelated("ec2") {
		if def.TargetType == "ct-events" {
			ec2CtEventsRelatedChecker = def.Checker
			break
		}
	}
}

// ec2CtEventsCheckerByTarget returns the RelatedChecker for "ct-events" registered
// under "ec2". Defined here (unit_test pkg) to avoid conflict with ec2CheckerByTarget
// in aws_ec2_related_test.go (unit pkg).
func ec2CtEventsCheckerByTarget(t *testing.T) resource.RelatedChecker {
	t.Helper()
	if ec2CtEventsRelatedChecker == nil {
		t.Fatal("ec2 ct-events checker not captured at init — verify ec2 RegisterRelated includes ct-events")
	}
	return ec2CtEventsRelatedChecker
}

// TestRelated_CtEvents_EC2_FetchFilterSet verifies that checkEC2CloudTrailEvents
// sets FetchFilter["ResourceName"] to the instance ID when a match is found.
func TestRelated_CtEvents_EC2_FetchFilterSet(t *testing.T) {
	instanceID := "i-0abcdef1234567890"
	eventRes := resource.Resource{
		ID:        "evt-ec2-001",
		Fields:    map[string]string{"resource_name": instanceID},
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String(instanceID),
					ResourceType: aws.String("AWS::EC2::Instance"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{eventRes}},
	}
	ec2Res := resource.Resource{
		ID: instanceID,
	}

	checker := ec2CtEventsCheckerByTarget(t)
	result := checker(context.Background(), nil, ec2Res, cache)

	if result.Count <= 0 {
		t.Errorf("Count = %d, want > 0 (event matched)", result.Count)
	}
	if result.FetchFilter == nil {
		t.Fatal("FetchFilter is nil, want non-nil")
	}
	if result.FetchFilter["ResourceName"] != instanceID {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q", result.FetchFilter["ResourceName"], instanceID)
	}
}

// TestRelated_CtEvents_EC2_FetchFilterSet_NoMatch_Truncated verifies that
// FetchFilter["ResourceName"] is set even when no match is found in a truncated cache.
func TestRelated_CtEvents_EC2_FetchFilterSet_NoMatch_Truncated(t *testing.T) {
	instanceID := "i-0abcdef1234567890"
	unrelatedEvent := resource.Resource{
		ID:     "evt-ec2-002",
		Fields: map[string]string{"resource_name": "i-other"},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{unrelatedEvent},
			IsTruncated: true,
		},
	}
	ec2Res := resource.Resource{
		ID: instanceID,
	}

	checker := ec2CtEventsCheckerByTarget(t)
	result := checker(context.Background(), nil, ec2Res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no match, truncated)", result.Count)
	}
	if result.FetchFilter == nil {
		t.Fatal("FetchFilter is nil, want non-nil")
	}
	if result.FetchFilter["ResourceName"] != instanceID {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q", result.FetchFilter["ResourceName"], instanceID)
	}
}

// TestRelated_CtEvents_EC2_FetchFilterSet_Match_Truncated is a regression test
// for the bug where checkEC2CloudTrailEvents returned the in-cache match count
// instead of -1 when the cache was truncated. A truncated cache means the real
// count is unknown, so -1 must be returned regardless of in-cache matches.
func TestRelated_CtEvents_EC2_FetchFilterSet_Match_Truncated(t *testing.T) {
	instanceID := "i-abc123"
	matchingEvent := resource.Resource{
		ID:     "evt-ec2-match-001",
		Fields: map[string]string{"resource_name": instanceID},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingEvent},
			IsTruncated: true,
		},
	}
	ec2Res := resource.Resource{
		ID: instanceID,
	}

	checker := ec2CtEventsCheckerByTarget(t)
	result := checker(context.Background(), nil, ec2Res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache truncated, real count unknown)", result.Count)
	}
	if result.FetchFilter == nil {
		t.Fatal("FetchFilter is nil, want non-nil")
	}
	if result.FetchFilter["ResourceName"] != instanceID {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q", result.FetchFilter["ResourceName"], instanceID)
	}
}

// ---------------------------------------------------------------------------
// AssumedRole JSON extraction: checkCtEventsRole via CloudTrailEvent JSON
// ---------------------------------------------------------------------------

// TestRelated_CtEvents_Role_AssumedRoleViaCTEventJSON verifies that when
// Resources is empty and Username has no "/", the role name is extracted from
// CloudTrailEvent JSON at userIdentity.sessionContext.sessionIssuer.userName.
func TestRelated_CtEvents_Role_AssumedRoleViaCTEventJSON(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "my-role",
		Name:   "my-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	ctEventJSON := `{"userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::123:assumed-role/my-role/session","sessionContext":{"sessionIssuer":{"type":"Role","principalId":"AROAEXAMPLE","arn":"arn:aws:iam::123:role/my-role","accountId":"123","userName":"my-role"}}}}`

	res := resource.Resource{
		ID:     "evt-assumed-role-001",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			EventId:         aws.String("evt-assumed-role-001"),
			Username:        aws.String("session-name"), // no "/" — not a service role path
			CloudTrailEvent: aws.String(ctEventJSON),
			Resources:       []cloudtrailtypes.Resource{}, // empty
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (AssumedRole extracted from CloudTrailEvent JSON)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_CtEvents_Role_AssumedRoleNoMatch verifies that when the role
// extracted from CloudTrailEvent JSON is not in the cache, Count == 0.
func TestRelated_CtEvents_Role_AssumedRoleNoMatch(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "other-role",
		Name:   "other-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	ctEventJSON := `{"userIdentity":{"type":"AssumedRole","sessionContext":{"sessionIssuer":{"userName":"my-role"}}}}`

	res := resource.Resource{
		ID:     "evt-assumed-role-002",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			EventId:         aws.String("evt-assumed-role-002"),
			Username:        aws.String("session-name"),
			CloudTrailEvent: aws.String(ctEventJSON),
			Resources:       []cloudtrailtypes.Resource{},
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (role not in cache)", result.Count)
	}
}

// TestRelated_CtEvents_Role_AssumedRole_NotRoleType verifies that when
// CloudTrailEvent JSON has userIdentity.type == "IAMUser" (not AssumedRole/Role),
// no role is extracted and Count == 0.
func TestRelated_CtEvents_Role_AssumedRole_NotRoleType(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "my-role",
		Name:   "my-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	// IAMUser type — no sessionContext.sessionIssuer.userName path applies
	ctEventJSON := `{"userIdentity":{"type":"IAMUser","userName":"alice","sessionContext":{"sessionIssuer":{"userName":"my-role"}}}}`

	res := resource.Resource{
		ID:     "evt-iam-user-type-001",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Event{
			EventId:         aws.String("evt-iam-user-type-001"),
			Username:        aws.String("alice"),        // no "/" — would match service role path check
			CloudTrailEvent: aws.String(ctEventJSON),
			Resources:       []cloudtrailtypes.Resource{},
		},
	}

	checker := ctEventsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (IAMUser type should not extract role from JSON)", result.Count)
	}
}
