package unit

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// TestCTDetailSummarizeIAM_CreateRole verifies that SummarizeIAM emits rows for residual
// CreateRole fields after TARGET extraction. roleName may or may not be lifted upstream
// depending on implementation — the test asserts that the remaining fields ARE emitted
// and that the summarizer does not panic.
func TestCTDetailSummarizeIAM_CreateRole(t *testing.T) {
	// cleaned params: roleName may be lifted by catch-all (ends in "Name"),
	// or may remain. Either is valid. Test the residual non-identity fields.
	params := map[string]any{
		"assumeRolePolicyDocument": `{"Version":"2012-10-17","Statement":[]}`,
		"path":                     "/",
		"description":              "Admin access",
	}
	rows := ctevent.SummarizeIAM("CreateRole", params)
	if rows == nil {
		t.Fatal("SummarizeIAM(CreateRole) returned nil; want non-nil slice")
	}
	wantKeys := map[string]bool{
		"assumeRolePolicyDocument": false,
		"path":                     false,
		"description":              false,
	}
	for _, r := range rows {
		if _, ok := wantKeys[r.Key]; ok {
			wantKeys[r.Key] = true
		}
	}
	for k, seen := range wantKeys {
		if !seen {
			t.Errorf("CreateRole: expected row with Key=%q but it was not emitted; rows=%v", k, rows)
		}
	}
}

// TestCTDetailSummarizeIAM_DeleteRole verifies that when roleName is lifted upstream
// (catch-all: ends in "Name"), the summarizer receives empty params and returns []Row{}.
func TestCTDetailSummarizeIAM_DeleteRole(t *testing.T) {
	// cleaned params after TARGET extraction removes roleName
	rows := ctevent.SummarizeIAM("DeleteRole", map[string]any{})
	if rows == nil {
		t.Fatal("SummarizeIAM(DeleteRole, {}) returned nil; want non-nil []Row{}")
	}
	if len(rows) != 0 {
		t.Errorf("SummarizeIAM(DeleteRole, {}): expected 0 rows; got %d: %v", len(rows), rows)
	}
}

// TestCTDetailSummarizeIAM_AttachRolePolicy verifies that policyArn is emitted as a row.
// roleName is lifted by TARGET extraction. policyArn should be emitted; if the implementation
// marks it navigable to "policy", that navigability is verified.
func TestCTDetailSummarizeIAM_AttachRolePolicy(t *testing.T) {
	// cleaned params: roleName lifted by catch-all, policyArn remains
	params := map[string]any{
		"policyArn": "arn:aws:iam::aws:policy/AdministratorAccess",
	}
	rows := ctevent.SummarizeIAM("AttachRolePolicy", params)
	if rows == nil {
		t.Fatal("SummarizeIAM(AttachRolePolicy) returned nil; want non-nil slice")
	}
	var policyRow *ctevent.Row
	for i := range rows {
		if rows[i].Key == "policyArn" {
			policyRow = &rows[i]
			break
		}
	}
	if policyRow == nil {
		t.Fatalf("AttachRolePolicy: expected row with Key=policyArn; got rows=%v", rows)
	}
	if policyRow.Value == "" {
		t.Errorf("AttachRolePolicy: policyArn row has empty Value")
	}
	// If the implementation marks policyArn as navigable, the target must be "policy".
	// If not navigable, TargetType must be empty. Both are valid — enforce consistency.
	if policyRow.IsNavigable && policyRow.TargetType != "policy" {
		t.Errorf("AttachRolePolicy: policyArn IsNavigable=true but TargetType=%q; want %q",
			policyRow.TargetType, "policy")
	}
	if !policyRow.IsNavigable && policyRow.TargetType != "" {
		t.Errorf("AttachRolePolicy: policyArn IsNavigable=false but TargetType=%q; want empty",
			policyRow.TargetType)
	}
}

// TestCTDetailSummarizeIAM_CreateAccessKey verifies non-crash behavior for CreateAccessKey.
// userName may be lifted upstream via catch-all (ends in "Name"). Either empty or non-empty
// result is valid; the summarizer must not panic and must return non-nil.
func TestCTDetailSummarizeIAM_CreateAccessKey(t *testing.T) {
	// Scenario A: userName not yet lifted (summarizer receives it)
	paramsWithUser := map[string]any{
		"userName": "bob",
	}
	rows := ctevent.SummarizeIAM("CreateAccessKey", paramsWithUser)
	if rows == nil {
		t.Fatal("SummarizeIAM(CreateAccessKey, with userName) returned nil")
	}
	// Scenario B: userName already lifted (empty cleaned params)
	rowsEmpty := ctevent.SummarizeIAM("CreateAccessKey", map[string]any{})
	if rowsEmpty == nil {
		t.Fatal("SummarizeIAM(CreateAccessKey, {}) returned nil")
	}
}

// TestCTDetailSummarizeIAM_PassRole verifies that when roleArn is lifted upstream,
// the summarizer returns non-nil empty slice. PassRole is handled by extractByEventName
// in target.go, so cleaned params arrive empty.
func TestCTDetailSummarizeIAM_PassRole(t *testing.T) {
	// cleaned params: roleArn lifted by extractByEventName (AssumeRole table)
	rows := ctevent.SummarizeIAM("PassRole", map[string]any{})
	if rows == nil {
		t.Fatal("SummarizeIAM(PassRole, {}) returned nil; want non-nil []Row{}")
	}
}

// TestCTDetailSummarizeIAM_PolicyArnNavigability verifies the navigability contract
// specifically for policyArn rows: IsNavigable true implies TargetType "policy",
// and IsNavigable false implies TargetType "". "policy" is the registered ShortName
// in internal/resource/types_security.go.
func TestCTDetailSummarizeIAM_PolicyArnNavigability(t *testing.T) {
	cases := []struct {
		eventName string
	}{
		{"AttachRolePolicy"},
		{"DetachRolePolicy"},
		{"AttachUserPolicy"},
		{"DetachUserPolicy"},
	}
	for _, tc := range cases {
		params := map[string]any{
			"policyArn": "arn:aws:iam::111111111111:policy/MyPolicy",
		}
		rows := ctevent.SummarizeIAM(tc.eventName, params)
		for i, r := range rows {
			if r.Key == "policyArn" {
				if r.IsNavigable && r.TargetType != "policy" {
					t.Errorf("[%s] row[%d] policyArn IsNavigable=true but TargetType=%q; want %q",
						tc.eventName, i, r.TargetType, "policy")
				}
				if !r.IsNavigable && r.TargetType != "" {
					t.Errorf("[%s] row[%d] policyArn IsNavigable=false but TargetType=%q; want empty",
						tc.eventName, i, r.TargetType)
				}
			}
		}
	}
}

// TestCTDetailSummarizeIAM_PurityNoMutation verifies that SummarizeIAM does not mutate
// the input params map.
func TestCTDetailSummarizeIAM_PurityNoMutation(t *testing.T) {
	params := map[string]any{
		"assumeRolePolicyDocument": `{"Version":"2012-10-17"}`,
		"path":                     "/",
		"description":              "test role",
		"nested":                   map[string]any{"k": "v"},
	}
	before := deepCopyParams(params)
	_ = ctevent.SummarizeIAM("CreateRole", params)
	if !reflect.DeepEqual(params, before) {
		t.Fatalf("SummarizeIAM mutated input params: got %v, want %v", params, before)
	}
}

// TestCTDetailSummarizeIAM_SeverityNeverSet verifies that no row emitted by SummarizeIAM
// has its Severity field set. Severity is reserved for the ACTION Event row only.
func TestCTDetailSummarizeIAM_SeverityNeverSet(t *testing.T) {
	cases := []struct {
		eventName string
		params    map[string]any
	}{
		{"CreateRole", map[string]any{"description": "test", "path": "/"}},
		{"AttachRolePolicy", map[string]any{"policyArn": "arn:aws:iam::aws:policy/ReadOnlyAccess"}},
		{"DeleteRole", map[string]any{}},
	}
	for _, tc := range cases {
		rows := ctevent.SummarizeIAM(tc.eventName, tc.params)
		for i, r := range rows {
			if r.Severity != "" {
				t.Errorf("[%s] row[%d] key=%q: Severity=%q; summarizers must never set Severity",
					tc.eventName, i, r.Key, r.Severity)
			}
		}
	}
}

// TestCTDetailSummarizeIAM_UnknownEvent verifies that an unrecognized IAM event name
// does not panic and returns a non-nil slice.
func TestCTDetailSummarizeIAM_UnknownEvent(t *testing.T) {
	params := map[string]any{"someField": "someValue"}
	var rows []ctevent.Row
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("SummarizeIAM panicked on unknown event: %v", r)
			}
		}()
		rows = ctevent.SummarizeIAM("SomeUnrecognizedIAMEvent", params)
	}()
	if rows == nil {
		t.Fatal("SummarizeIAM(unknown event) returned nil; want non-nil slice")
	}
}
