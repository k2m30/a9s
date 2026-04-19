// qa_yaml_tag_flatten_test.go — Tests for YAML tag flattening in ToSafeValue.
//
// Bug: commit e3765de (#210) added tag flattening ONLY in the detail view
// (internal/tui/views/detail_fields.go). The YAML view (via fieldpath.ToSafeValue)
// still emits tag slices as [{Key:X, Value:Y}] structs instead of {X:Y} maps.
//
// Fix target: fieldpath.ToSafeValue — detect slice-of-structs where every element
// has exactly Key(*string) + Value(*string) fields (two string-pointer fields),
// and emit map[string]any instead of []any.
//
// Tests in this file FAIL against current main because ToSafeValue does not yet
// flatten tag slices. They will PASS once the fix is applied.
package unit

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// TestToSafeValue_FlattensKeyValueTagSlice — unit tests of the ToSafeValue helper.
// ---------------------------------------------------------------------------

// TestToSafeValue_FlattensKeyValueTagSlice_RDS verifies that a []rdstypes.Tag
// (which has exactly *string Key + *string Value) is emitted as map[string]any,
// not []any of {Key:X, Value:Y} objects.
func TestToSafeValue_FlattensKeyValueTagSlice_RDS(t *testing.T) {
	tags := []rdstypes.Tag{
		{Key: aws.String("Name"), Value: aws.String("main")},
		{Key: aws.String("Component"), Value: aws.String("database")},
		{Key: aws.String("Environment"), Value: aws.String("production")},
		{Key: aws.String("Owner"), Value: aws.String("platform-team")},
	}
	val := reflect.ValueOf(tags)
	result := fieldpath.ToSafeValue(val)

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue([]rdstypes.Tag) = %T, want map[string]any (tag flattening not implemented)", result)
	}
	if m["Name"] != "main" {
		t.Errorf(`m["Name"] = %v, want "main"`, m["Name"])
	}
	if m["Component"] != "database" {
		t.Errorf(`m["Component"] = %v, want "database"`, m["Component"])
	}
	if m["Environment"] != "production" {
		t.Errorf(`m["Environment"] = %v, want "production"`, m["Environment"])
	}
	if m["Owner"] != "platform-team" {
		t.Errorf(`m["Owner"] = %v, want "platform-team"`, m["Owner"])
	}
}

// TestToSafeValue_FlattensKeyValueTagSlice_EC2 verifies that a []ec2types.Tag
// (which also has exactly *string Key + *string Value) is flattened to map[string]any.
func TestToSafeValue_FlattensKeyValueTagSlice_EC2(t *testing.T) {
	tags := []ec2types.Tag{
		{Key: aws.String("Name"), Value: aws.String("web-server")},
		{Key: aws.String("Role"), Value: aws.String("frontend")},
	}
	val := reflect.ValueOf(tags)
	result := fieldpath.ToSafeValue(val)

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue([]ec2types.Tag) = %T, want map[string]any (tag flattening not implemented)", result)
	}
	if m["Name"] != "web-server" {
		t.Errorf(`m["Name"] = %v, want "web-server"`, m["Name"])
	}
	if m["Role"] != "frontend" {
		t.Errorf(`m["Role"] = %v, want "frontend"`, m["Role"])
	}
}

// TestToSafeValue_FlattensKeyValueTagSlice_S3 verifies that a []s3types.Tag
// (which has non-pointer required string Key + Value) is also flattened.
func TestToSafeValue_FlattensKeyValueTagSlice_S3(t *testing.T) {
	tags := []s3types.Tag{
		{Key: aws.String("Project"), Value: aws.String("analytics")},
		{Key: aws.String("Tier"), Value: aws.String("storage")},
	}
	val := reflect.ValueOf(tags)
	result := fieldpath.ToSafeValue(val)

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue([]s3types.Tag) = %T, want map[string]any (tag flattening not implemented)", result)
	}
	if m["Project"] != "analytics" {
		t.Errorf(`m["Project"] = %v, want "analytics"`, m["Project"])
	}
	if m["Tier"] != "storage" {
		t.Errorf(`m["Tier"] = %v, want "storage"`, m["Tier"])
	}
}

// TestToSafeValue_DuplicateKeys_PreservedAsSlice verifies that a tag slice
// containing duplicate keys is NOT flattened to a map — flattening would drop
// one of the duplicate values. The spec's "degrade honestly" rule applies:
// preserving both entries in the []Tag form loses no data. The detail view's
// `flattenTagItems` handles the resulting struct-form items and renders both.
func TestToSafeValue_DuplicateKeys_PreservedAsSlice(t *testing.T) {
	tags := []rdstypes.Tag{
		{Key: aws.String("Name"), Value: aws.String("first")},
		{Key: aws.String("Name"), Value: aws.String("second")},
		{Key: aws.String("Env"), Value: aws.String("prod")},
	}
	val := reflect.ValueOf(tags)
	// Must not panic.
	result := fieldpath.ToSafeValue(val)

	// With a duplicate key ("Name") present, flattening is refused and the
	// slice comes back as a []any of struct-form entries.
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("ToSafeValue with duplicate keys = %T, want []any (flatten should be refused)", result)
	}
	if len(arr) != 3 {
		t.Fatalf("len([]any) = %d, want 3 (all entries preserved)", len(arr))
	}
	// Spot-check that both "first" and "second" survive the round-trip.
	seenFirst, seenSecond := false, false
	for _, item := range arr {
		m, isMap := item.(map[string]any)
		if !isMap {
			continue
		}
		if m["Value"] == "first" {
			seenFirst = true
		}
		if m["Value"] == "second" {
			seenSecond = true
		}
	}
	if !seenFirst || !seenSecond {
		t.Errorf("both 'first' and 'second' must survive; seenFirst=%v seenSecond=%v arr=%#v", seenFirst, seenSecond, arr)
	}
}

// richASGTag is a synthetic struct with 3 fields (Key, Value, PropagateAtLaunch)
// simulating autoscaling TagDescription. ToSafeValue MUST NOT flatten this —
// it has more than 2 exported fields.
type richASGTag struct {
	Key              *string
	Value            *string
	PropagateAtLaunch *bool
}

// TestToSafeValue_NoFlattenRichTagStruct verifies that a slice of a 3-field struct
// (Key, Value, PropagateAtLaunch) is NOT flattened to map — it stays []any.
// This prevents over-eager flattening of ASG-style tag descriptors.
func TestToSafeValue_NoFlattenRichTagStruct(t *testing.T) {
	propagate := true
	tags := []richASGTag{
		{Key: aws.String("Name"), Value: aws.String("asg-main"), PropagateAtLaunch: &propagate},
		{Key: aws.String("Env"), Value: aws.String("prod"), PropagateAtLaunch: &propagate},
	}
	val := reflect.ValueOf(tags)
	result := fieldpath.ToSafeValue(val)

	if _, ok := result.(map[string]any); ok {
		t.Fatal("ToSafeValue([]richASGTag with 3 fields) must NOT flatten to map — rich tag structs must stay as []any")
	}
	// Must be a slice (not nil).
	if _, ok := result.([]any); !ok {
		t.Fatalf("ToSafeValue([]richASGTag) = %T, want []any", result)
	}
}

// TestToSafeValue_EmptyTagSlice verifies that an empty tag slice returns nil
// (consistent with current zero-element slice behavior).
func TestToSafeValue_EmptyTagSlice(t *testing.T) {
	tags := []rdstypes.Tag{}
	val := reflect.ValueOf(tags)
	result := fieldpath.ToSafeValue(val)
	if result != nil {
		t.Errorf("ToSafeValue(empty []rdstypes.Tag) = %v, want nil", result)
	}
}

// TestToSafeValue_NilSlice verifies that a nil tag slice returns nil.
func TestToSafeValue_NilSlice(t *testing.T) {
	var tags []rdstypes.Tag
	val := reflect.ValueOf(tags)
	result := fieldpath.ToSafeValue(val)
	if result != nil {
		t.Errorf("ToSafeValue(nil []rdstypes.Tag) = %v, want nil", result)
	}
}

// TestToSafeValue_NonStructSliceUnchanged verifies that a slice of non-struct elements
// (e.g. []string) is not affected by tag flattening — it passes through as []any of strings.
func TestToSafeValue_NonStructSliceUnchanged(t *testing.T) {
	strs := []string{"us-east-1", "us-west-2"}
	val := reflect.ValueOf(strs)
	result := fieldpath.ToSafeValue(val)

	if _, ok := result.(map[string]any); ok {
		t.Fatal("ToSafeValue([]string) must not be flattened to map")
	}
	slice, ok := result.([]any)
	if !ok {
		t.Fatalf("ToSafeValue([]string) = %T, want []any", result)
	}
	if len(slice) != 2 {
		t.Errorf("len = %d, want 2", len(slice))
	}
}

// ---------------------------------------------------------------------------
// TestYAMLView_TagListFlattened — integration test through YAMLModel.RawContent()
// ---------------------------------------------------------------------------

// syntheticDBSnapshot is a stand-in for rds.DBSnapshot with a TagList field,
// demonstrating that the YAML view should flatten the tag slice.
type syntheticDBSnapshot struct {
	DBInstanceIdentifier *string
	SnapshotIdentifier   *string
	TagList              []rdstypes.Tag
}

// TestToSafeValue_NilTagValuePreservedAsNull pins that a tag with nil Value
// is flattened to a YAML null, not coerced to the empty string. "Value not
// set" and "Value is the empty string" are distinct, and the struct-form
// output made that distinction visible; the flattened form must not hide it.
func TestToSafeValue_NilTagValuePreservedAsNull(t *testing.T) {
	tags := []rdstypes.Tag{
		{Key: aws.String("Orphan"), Value: nil},
		{Key: aws.String("Named"), Value: aws.String("main")},
	}
	val := reflect.ValueOf(tags)
	result := fieldpath.ToSafeValue(val)

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue(tags with one nil Value) = %T, want map[string]any", result)
	}
	got, present := m["Orphan"]
	if !present {
		t.Fatalf("flattened map must still carry the key %q even when Value is nil", "Orphan")
	}
	if got != nil {
		t.Errorf("Orphan = %v (type %T), want nil — nil tag Value must not be silently coerced to empty string", got, got)
	}
	if s, ok := got.(string); ok && s == "" {
		t.Errorf("Orphan = \"\" (empty string) — nil tag Value must be preserved as nil/null, not \"\"")
	}
	if m["Named"] != "main" {
		t.Errorf("non-nil Value lost during flattening: Named = %v, want \"main\"", m["Named"])
	}
}

// TestYAMLView_TagListFlattened verifies that when YAMLModel.RawContent() serializes
// a resource whose RawStruct has a TagList field of []rdstypes.Tag, the resulting YAML
// contains "Component: x" (flattened map) and NOT "- Key: Component" (struct slice).
//
// This test FAILS on current main because ToSafeValue does not flatten tag slices.
func TestYAMLView_TagListFlattened(t *testing.T) {
	snap := syntheticDBSnapshot{
		DBInstanceIdentifier: aws.String("db-instance-1"),
		SnapshotIdentifier:   aws.String("snapshot-2024-01-15"),
		TagList: []rdstypes.Tag{
			{Key: aws.String("Component"), Value: aws.String("x")},
			{Key: aws.String("Name"), Value: aws.String("main")},
			{Key: aws.String("Environment"), Value: aws.String("production")},
		},
	}

	res := resource.Resource{
		ID:        "snapshot-2024-01-15",
		Name:      "snapshot-2024-01-15",
		Status:    "available",
		Fields:    map[string]string{},
		RawStruct: snap,
	}

	model := views.NewYAML(res, "rds", keys.Default())
	content := model.RawContent()

	if content == "" {
		t.Fatal("RawContent() returned empty string — resource has RawStruct set")
	}

	// The flattened form must be present: TagList as a YAML map.
	if !strings.Contains(content, "Component: x") {
		t.Errorf("YAML output must contain 'Component: x' (flattened tag), got:\n%s", content)
	}

	// The struct-slice form must NOT appear.
	if strings.Contains(content, "- Key: Component") {
		t.Errorf("YAML output must NOT contain '- Key: Component' (unflattened tag struct), got:\n%s", content)
	}
	if strings.Contains(content, "- Key:") {
		t.Errorf("YAML output must NOT contain '- Key:' (unflattened tag struct), got:\n%s", content)
	}
}
