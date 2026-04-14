package views

// resolve_identity_test.go — T040: unit tests for the resolveIdentityColumn
// cascade. These tests live in package views (internal test) because
// resolveIdentityColumn is an unexported function.
//
// Cascade order (from resolveIdentityColumn docs):
//  1. td.IdentityKey matches a column's key field.
//  2. column key == "name".
//  3. column path contains "Name" or "Identifier".
//  4. column title equals "Name" (case-insensitive) or equals td.Name.
//  5. fall back to index 0.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeCol builds a minimal listCol for cascade tests.
func makeCol(key, path, title string) listCol {
	return listCol{key: key, path: path, title: title, width: 20}
}

// ---------------------------------------------------------------------------
// Step 1: explicit IdentityKey on ResourceTypeDef
// ---------------------------------------------------------------------------

// TestResolveIdentityColumn_MatchesIdentityKey verifies that when
// td.IdentityKey is set and matches a column's key, that column index is
// returned — regardless of its position in the slice.
func TestResolveIdentityColumn_MatchesIdentityKey(t *testing.T) {
	cols := []listCol{
		makeCol("status", "", "Status"),
		makeCol("region", "", "Region"),
		makeCol("foo", "", "Foo"),
		makeCol("name", "", "Name"),
	}
	td := resource.ResourceTypeDef{
		ShortName:   "ec2",
		Name:        "EC2 Instances",
		IdentityKey: "foo",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 2 {
		t.Errorf("resolveIdentityColumn with IdentityKey=%q: got %d, want 2", td.IdentityKey, got)
	}
}

// ---------------------------------------------------------------------------
// Step 2: key == "name" fallthrough (no IdentityKey set)
// ---------------------------------------------------------------------------

// TestResolveIdentityColumn_FallsThroughToNameKey verifies that when
// IdentityKey is empty and a column has key=="name", that column is returned.
func TestResolveIdentityColumn_FallsThroughToNameKey(t *testing.T) {
	cols := []listCol{
		makeCol("id", "", "ID"),
		makeCol("name", "", "Name"),
		makeCol("status", "", "Status"),
	}
	td := resource.ResourceTypeDef{
		ShortName: "rds",
		Name:      "RDS Instances",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 1 {
		t.Errorf("resolveIdentityColumn via name key: got %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// Step 3: path contains "Name" or "Identifier"
// ---------------------------------------------------------------------------

// TestResolveIdentityColumn_FallsThroughToPath verifies that when IdentityKey
// is empty, no column has key=="name", and a column's path contains
// "Identifier", that column index is returned.
func TestResolveIdentityColumn_FallsThroughToPath(t *testing.T) {
	cols := []listCol{
		makeCol("id", "", "ID"),
		makeCol("status", "", "Status"),
		makeCol("", "DBInstanceIdentifier", "DB Instance"),
		makeCol("", "Engine", "Engine"),
	}
	td := resource.ResourceTypeDef{
		ShortName: "rds",
		Name:      "RDS Instances",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 2 {
		t.Errorf("resolveIdentityColumn via path=DBInstanceIdentifier: got %d, want 2", got)
	}
}

// TestResolveIdentityColumn_FallsThroughToPath_NameInPath verifies path
// matching when the path contains "Name" (not "Identifier").
func TestResolveIdentityColumn_FallsThroughToPath_NameInPath(t *testing.T) {
	cols := []listCol{
		makeCol("", "", "Arn"),
		makeCol("", "FunctionName", "Function"),
	}
	td := resource.ResourceTypeDef{
		ShortName: "lambda",
		Name:      "Lambda Functions",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 1 {
		t.Errorf("resolveIdentityColumn via path containing Name: got %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// Step 4: title equals "Name" (case-insensitive) or equals td.Name
// ---------------------------------------------------------------------------

// TestResolveIdentityColumn_FallsThroughToTitle verifies that when none of the
// earlier cascade steps match, a column whose title is "Name" (exact case) is
// selected.
func TestResolveIdentityColumn_FallsThroughToTitle(t *testing.T) {
	cols := []listCol{
		makeCol("", "", "ARN"),
		makeCol("", "", "Status"),
		makeCol("", "", "Region"),
		makeCol("", "", "Name"),
	}
	td := resource.ResourceTypeDef{
		ShortName: "acm",
		Name:      "ACM Certificates",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 3 {
		t.Errorf("resolveIdentityColumn via title=Name: got %d, want 3", got)
	}
}

// TestResolveIdentityColumn_CaseInsensitiveTitleMatch verifies that the title
// comparison is case-insensitive (e.g., "NAME" also matches).
func TestResolveIdentityColumn_CaseInsensitiveTitleMatch(t *testing.T) {
	cols := []listCol{
		makeCol("", "", "ARN"),
		makeCol("", "", "Status"),
		makeCol("", "", "NAME"),
		makeCol("", "", "Region"),
	}
	td := resource.ResourceTypeDef{
		ShortName: "acm",
		Name:      "ACM Certificates",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 2 {
		t.Errorf("resolveIdentityColumn case-insensitive title match: got %d, want 2", got)
	}
}

// TestResolveIdentityColumn_TitleMatchesTypeName verifies that when no column
// has title "Name" but one has a title that matches td.Name (case-insensitive),
// that column is returned.
func TestResolveIdentityColumn_TitleMatchesTypeName(t *testing.T) {
	cols := []listCol{
		makeCol("", "", "ARN"),
		makeCol("", "", "Bucket"),
		makeCol("", "", "s3 buckets"),
	}
	td := resource.ResourceTypeDef{
		ShortName: "s3",
		Name:      "S3 Buckets",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 2 {
		t.Errorf("resolveIdentityColumn via td.Name title match: got %d, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// Step 5: fall back to index 0
// ---------------------------------------------------------------------------

// TestResolveIdentityColumn_FallsBackToZero verifies that when no cascade step
// matches, the function returns 0 (the first column).
func TestResolveIdentityColumn_FallsBackToZero(t *testing.T) {
	cols := []listCol{
		makeCol("arn", "", "ARN"),
		makeCol("status", "", "Status"),
		makeCol("region", "", "Region"),
	}
	td := resource.ResourceTypeDef{
		ShortName: "ecr",
		Name:      "ECR Repositories",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 0 {
		t.Errorf("resolveIdentityColumn fallback: got %d, want 0", got)
	}
}

// TestResolveIdentityColumn_EmptyColumns verifies that an empty column slice
// does not panic and returns 0.
func TestResolveIdentityColumn_EmptyColumns(t *testing.T) {
	td := resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
	got := resolveIdentityColumn([]listCol{}, td)
	if got != 0 {
		t.Errorf("resolveIdentityColumn empty cols: got %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// Priority: IdentityKey wins over "name" key
// ---------------------------------------------------------------------------

// TestResolveIdentityColumn_IdentityKeyBeatsNameKey verifies that when both
// IdentityKey matches and a "name" key column exist, IdentityKey wins.
func TestResolveIdentityColumn_IdentityKeyBeatsNameKey(t *testing.T) {
	cols := []listCol{
		makeCol("name", "", "Name"),
		makeCol("db_id", "", "DB ID"),
	}
	td := resource.ResourceTypeDef{
		ShortName:   "dbi",
		Name:        "DB Instances",
		IdentityKey: "db_id",
	}
	got := resolveIdentityColumn(cols, td)
	if got != 1 {
		t.Errorf("IdentityKey should beat name key: got %d, want 1", got)
	}
}

// TestResolveIdentityColumn_IdentityKeyNotFound falls through cascade when
// IdentityKey is set but no column matches — continues to step 2 ("name" key).
func TestResolveIdentityColumn_IdentityKeyNotFound(t *testing.T) {
	cols := []listCol{
		makeCol("name", "", "Name"),
		makeCol("status", "", "Status"),
	}
	td := resource.ResourceTypeDef{
		ShortName:   "ec2",
		Name:        "EC2 Instances",
		IdentityKey: "nonexistent_key",
	}
	// IdentityKey set but not found → falls through to step 2 (key=="name")
	got := resolveIdentityColumn(cols, td)
	if got != 0 {
		t.Errorf("IdentityKey not found, should fall to name key at idx 0: got %d, want 0", got)
	}
}
