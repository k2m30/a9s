package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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

	// Nil clients + empty cache is the demo/test scenario: the target list is
	// definitively empty (not "unknown"). Checker must return Count=0, not Count=-1.
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cache, nil clients → definitively empty)", result.Count)
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

	// Nil clients + empty cache is the demo/test scenario: the target list is
	// definitively empty (not "unknown"). Checker must return Count=0, not Count=-1.
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cache, nil clients → definitively empty)", result.Count)
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
		ID:     "evt-ec2-001",
		Fields: map[string]string{"resource_name": instanceID},
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

// ---------------------------------------------------------------------------
// §7b.10 completeness: all 13 typed RelatedDef entries must be registered
// ---------------------------------------------------------------------------

// TestCtEventsRelatedGroups_AllTypedRegistered asserts that the "ct-events" related
// registry contains entries for every resource type listed in §7b.10 of
// docs/design/ct-event-detail.md. The test is intentionally expected to FAIL until
// all 11 missing registrations are added to ct_events.go.
func TestCtEventsRelatedGroups_AllTypedRegistered(t *testing.T) {
	expected := []string{
		"role", "iam-user", "ec2", "s3", "s3_objects", "lambda",
		"rds", "kms", "secrets", "vpce", "sg", "ddb", "cfn",
	}

	defs := resource.GetRelated("ct-events")

	registered := make(map[string]bool, len(defs))
	for _, def := range defs {
		registered[def.TargetType] = true
	}

	var missing []string
	for _, target := range expected {
		if !registered[target] {
			missing = append(missing, target)
		}
	}

	var unexpected []string
	for _, def := range defs {
		if def.TargetType == "ct-events" {
			continue // skip self-pivots — covered by TestCtEventsRelatedGroups_PivotsRegistered
		}
		found := slices.Contains(expected, def.TargetType)
		if !found {
			unexpected = append(unexpected, def.TargetType)
		}
	}

	current := make([]string, 0, len(defs))
	for _, def := range defs {
		current = append(current, def.TargetType)
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Errorf(
			"ct-events related registry mismatch:\n  missing (%d):    %v\n  unexpected (%d): %v\n  current set:     %v",
			len(missing), missing,
			len(unexpected), unexpected,
			current,
		)
	}
}

// ---------------------------------------------------------------------------
// §7b.10 self-pivot rows: ct-events → ct-events (4 pivot RelatedDefs)
// ---------------------------------------------------------------------------

// TestCtEventsRelatedGroups_PivotsRegistered asserts that the "ct-events" related
// registry contains exactly 4 self-pivot entries (TargetType == "ct-events") with
// the DisplayNames specified in §7b.10 of docs/design/ct-event-detail.md.
// The test is expected to FAIL until all 4 pivot registrations are added.
func TestCtEventsRelatedGroups_PivotsRegistered(t *testing.T) {
	expectedPivots := []string{
		"CT events by AccessKeyId",
		"CT events by Username",
		"CT events by EventName",
		"CT events by SharedEventId",
	}

	defs := resource.GetRelated("ct-events")

	// Collect only self-pivot entries.
	var pivots []resource.RelatedDef
	for _, def := range defs {
		if def.TargetType == "ct-events" {
			pivots = append(pivots, def)
		}
	}

	// Index pivots by DisplayName for O(1) lookup.
	pivotByName := make(map[string]bool, len(pivots))
	for _, p := range pivots {
		pivotByName[p.DisplayName] = true
	}

	// Build expected set for reverse lookup.
	expectedSet := make(map[string]bool, len(expectedPivots))
	for _, name := range expectedPivots {
		expectedSet[name] = true
	}

	var missing []string
	for _, name := range expectedPivots {
		if !pivotByName[name] {
			missing = append(missing, name)
		}
	}

	var unexpected []string
	for _, p := range pivots {
		if !expectedSet[p.DisplayName] {
			unexpected = append(unexpected, p.DisplayName)
		}
	}

	currentNames := make([]string, 0, len(pivots))
	for _, p := range pivots {
		currentNames = append(currentNames, p.DisplayName)
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Errorf(
			"ct-events self-pivot registry mismatch:\n  missing (%d):    %v\n  unexpected (%d): %v\n  current set:     %v",
			len(missing), missing,
			len(unexpected), unexpected,
			currentNames,
		)
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
			Username:        aws.String("alice"), // no "/" — would match service role path check
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
