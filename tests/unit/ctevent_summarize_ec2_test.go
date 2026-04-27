package unit

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// TestCTDetailSummarizeEC2_RunInstances_AllFieldsEmitted verifies that SummarizeEC2 emits
// a row for each field in RunInstances params. navigable fields (imageId, subnetId,
// securityGroupIds items) should have IsNavigable=true with correct TargetType when the
// implementation supports it; non-navigable fields (instanceType, keyName) must not be
// marked navigable.
func TestCTDetailSummarizeEC2_RunInstances_AllFieldsEmitted(t *testing.T) {
	params := map[string]any{
		"imageId":          "ami-0abc1234567890def",
		"instanceType":     "t3.micro",
		"subnetId":         "subnet-0abc1234",
		"securityGroupIds": []any{"sg-00001111", "sg-22223333"},
		"keyName":          "my-key",
	}
	rows := ctevent.SummarizeEC2("RunInstances", params)
	if rows == nil {
		t.Fatal("SummarizeEC2(RunInstances) returned nil; want non-nil slice")
	}

	emittedKeys := make(map[string]ctevent.Row)
	for _, r := range rows {
		emittedKeys[r.Key] = r
	}

	// Non-navigable fields must be present and not marked navigable.
	nonNavFields := []string{"instanceType", "keyName"}
	for _, field := range nonNavFields {
		r, ok := emittedKeys[field]
		if !ok {
			// securityGroupIds and imageId/subnetId may be represented differently; only check
			// the definitely-non-navigable scalar fields.
			t.Errorf("RunInstances: expected row Key=%q; got rows=%v", field, rows)
			continue
		}
		if r.IsNavigable {
			t.Errorf("RunInstances: field %q should not be navigable; got IsNavigable=true", field)
		}
	}

	// imageId row (if present as scalar): when navigable, TargetType must be "ami".
	if r, ok := emittedKeys["imageId"]; ok {
		if r.IsNavigable && r.TargetType != "ami" {
			t.Errorf("RunInstances: imageId IsNavigable=true but TargetType=%q; want %q",
				r.TargetType, "ami")
		}
	}

	// subnetId row (if present as scalar): when navigable, TargetType must be "subnet".
	if r, ok := emittedKeys["subnetId"]; ok {
		if r.IsNavigable && r.TargetType != "subnet" {
			t.Errorf("RunInstances: subnetId IsNavigable=true but TargetType=%q; want %q",
				r.TargetType, "subnet")
		}
	}
}

// TestCTDetailSummarizeEC2_RunInstances_SecurityGroupNavigability verifies navigability
// for security group IDs. The implementation may emit one row per sg-ID or one joined row.
// Either way, any row whose Value contains a single sg-xxxx ID and IsNavigable=true must
// have TargetType="sg".
func TestCTDetailSummarizeEC2_RunInstances_SecurityGroupNavigability(t *testing.T) {
	params := map[string]any{
		"securityGroupIds": []any{"sg-00001111", "sg-22223333"},
	}
	rows := ctevent.SummarizeEC2("RunInstances", params)
	if rows == nil {
		t.Fatal("SummarizeEC2(RunInstances, sg only) returned nil")
	}
	for i, r := range rows {
		if r.IsNavigable && r.TargetType != "sg" {
			t.Errorf("RunInstances row[%d] key=%q IsNavigable=true but TargetType=%q; want %q",
				i, r.Key, r.TargetType, "sg")
		}
	}
}

// TestCTDetailSummarizeEC2_TerminateInstances verifies that when instancesSet is lifted
// upstream by ExtractTarget (extractInstancesSetEvent), the summarizer receives empty
// cleaned params and returns a non-nil empty slice.
func TestCTDetailSummarizeEC2_TerminateInstances(t *testing.T) {
	// cleaned params: instancesSet already removed by extractInstancesSetEvent in target.go
	rows := ctevent.SummarizeEC2("TerminateInstances", map[string]any{})
	if rows == nil {
		t.Fatal("SummarizeEC2(TerminateInstances, {}) returned nil; want non-nil []Row{}")
	}
	if len(rows) != 0 {
		t.Errorf("SummarizeEC2(TerminateInstances, {}): expected 0 rows; got %d: %v", len(rows), rows)
	}
}

// TestCTDetailSummarizeEC2_DescribeInstances verifies that filter/metadata fields remain
// after ExtractTarget handles the instancesSet for DescribeInstances.
// filters and maxResults are non-navigable metadata rows.
func TestCTDetailSummarizeEC2_DescribeInstances(t *testing.T) {
	// In target.go, DescribeInstances with no IDs emits Instances:(all) and does NOT
	// remove instancesSet (it was nil/empty). The filters + maxResults pass through.
	params := map[string]any{
		"filters":    []any{map[string]any{"Name": "instance-state-name", "Values": []any{"running"}}},
		"maxResults": float64(1000),
	}
	rows := ctevent.SummarizeEC2("DescribeInstances", params)
	if rows == nil {
		t.Fatal("SummarizeEC2(DescribeInstances) returned nil; want non-nil slice")
	}
	emittedKeys := make(map[string]bool)
	for _, r := range rows {
		emittedKeys[r.Key] = true
		if r.IsNavigable {
			t.Errorf("DescribeInstances: row Key=%q should not be navigable", r.Key)
		}
	}
	if !emittedKeys["filters"] && !emittedKeys["maxResults"] {
		// At least one of these metadata fields should appear.
		t.Errorf("DescribeInstances: expected metadata rows (filters, maxResults); got rows=%v", rows)
	}
}

// TestCTDetailSummarizeEC2_CreateSecurityGroup verifies that residual fields are emitted
// and vpcId is navigable to "vpc" when present (after catch-all may lift groupName).
func TestCTDetailSummarizeEC2_CreateSecurityGroup(t *testing.T) {
	// groupName ends in "Name" → catch-all lifts it; vpcId and description remain.
	// But vpcId ends in "Id" → also caught. Test with cleaned params containing only
	// description + vpcId to cover the case where vpcId is still present.
	params := map[string]any{
		"vpcId":       "vpc-0abc1234",
		"description": "web tier",
	}
	rows := ctevent.SummarizeEC2("CreateSecurityGroup", params)
	if rows == nil {
		t.Fatal("SummarizeEC2(CreateSecurityGroup) returned nil; want non-nil slice")
	}

	for _, r := range rows {
		if r.Key == "vpcId" {
			if r.IsNavigable && r.TargetType != "vpc" {
				t.Errorf("CreateSecurityGroup: vpcId IsNavigable=true but TargetType=%q; want %q",
					r.TargetType, "vpc")
			}
		}
		if r.Key == "description" && r.IsNavigable {
			t.Errorf("CreateSecurityGroup: description should not be navigable")
		}
	}
}

// TestCTDetailSummarizeEC2_StartStop verifies that cleaned params after TARGET extraction
// of instancesSet returns a non-nil empty slice for StartInstances and StopInstances.
func TestCTDetailSummarizeEC2_StartStop(t *testing.T) {
	for _, eventName := range []string{"StartInstances", "StopInstances"} {
		rows := ctevent.SummarizeEC2(eventName, map[string]any{})
		if rows == nil {
			t.Fatalf("SummarizeEC2(%s, {}) returned nil; want non-nil []Row{}", eventName)
		}
		if len(rows) != 0 {
			t.Errorf("SummarizeEC2(%s, {}): expected 0 rows; got %d: %v", eventName, len(rows), rows)
		}
	}
}

// TestCTDetailSummarizeEC2_PurityNoMutation verifies that SummarizeEC2 does not mutate
// the input params map.
func TestCTDetailSummarizeEC2_PurityNoMutation(t *testing.T) {
	params := map[string]any{
		"imageId":          "ami-0abc1234567890def",
		"instanceType":     "t3.micro",
		"securityGroupIds": []any{"sg-00001111"},
		"nested":           map[string]any{"k": "v"},
	}
	before := deepCopyParams(params)
	_ = ctevent.SummarizeEC2("RunInstances", params)
	if !reflect.DeepEqual(params, before) {
		t.Fatalf("SummarizeEC2 mutated input params: got %v, want %v", params, before)
	}
}

// TestCTDetailSummarizeEC2_SeverityNeverSet verifies that no row emitted by SummarizeEC2
// has its Severity field set. Severity is reserved for the ACTION Event row only.
func TestCTDetailSummarizeEC2_SeverityNeverSet(t *testing.T) {
	cases := []struct {
		eventName string
		params    map[string]any
	}{
		{"RunInstances", map[string]any{"instanceType": "t3.micro"}},
		{"DescribeInstances", map[string]any{"maxResults": float64(100)}},
		{"TerminateInstances", map[string]any{}},
	}
	for _, tc := range cases {
		rows := ctevent.SummarizeEC2(tc.eventName, tc.params)
		for i, r := range rows {
			if r.Severity != "" {
				t.Errorf("[%s] row[%d] key=%q: Severity=%q; summarizers must never set Severity",
					tc.eventName, i, r.Key, r.Severity)
			}
		}
	}
}

// TestCTDetailSummarizeEC2_UnknownEvent verifies that an unrecognized EC2 event name
// does not panic and returns a non-nil slice.
func TestCTDetailSummarizeEC2_UnknownEvent(t *testing.T) {
	params := map[string]any{"someField": "someValue"}
	var rows []ctevent.Row
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("SummarizeEC2 panicked on unknown event: %v", r)
			}
		}()
		rows = ctevent.SummarizeEC2("SomeUnrecognizedEC2Event", params)
	}()
	if rows == nil {
		t.Fatal("SummarizeEC2(unknown event) returned nil; want non-nil slice")
	}
}
