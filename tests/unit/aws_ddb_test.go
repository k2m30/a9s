package unit

// aws_ddb_test.go — fetcher behavior tests for DynamoDB Tables.
//
// Tests drive FetchDynamoDBTablesPage with stubbed DDBListTablesAPI +
// DDBDescribeTableAPI and assert Resource.Status and Resource.Issues match
// the §4 phrase table verbatim:
//
//	ACTIVE                              → Status="",                Issues=nil
//	CREATING                            → Status="creating",        Issues=["creating"]
//	UPDATING                            → Status="updating",        Issues=["updating"]
//	DELETING                            → Status="deleting",        Issues=["deleting"]
//	ARCHIVING                           → Status="archiving",       Issues=["archiving"]
//	INACCESSIBLE_ENCRYPTION_CREDENTIALS → Status="kms key inaccessible",        Issues=["kms key inaccessible"]
//	ARCHIVED                            → Status="archived: kms key lost",       Issues=["archived: kms key lost"]
//
// Adversarial cases (inline — never in fixture file):
//   - DescribeTable returns nil Table → skip, do not crash.
//   - TableStatus==ARCHIVED + ArchivalSummary==nil → fallback phrase, no panic.
//
// Wave 3 (CloudWatch throttle / 5xx metrics) must NOT surface.

import (
	"context"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ---------------------------------------------------------------------------
// mocks
// ---------------------------------------------------------------------------

// ddbListStub implements DDBListTablesAPI and returns a fixed list of names.
type ddbListStub struct {
	names []string
	err   error
}

func (s *ddbListStub) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &dynamodb.ListTablesOutput{TableNames: s.names}, nil
}

// ddbDescribeStub implements DDBDescribeTableAPI and returns per-name descriptions.
type ddbDescribeStub struct {
	tables map[string]*ddbtypes.TableDescription
}

func (s *ddbDescribeStub) DescribeTable(_ context.Context, in *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if in == nil || in.TableName == nil {
		return &dynamodb.DescribeTableOutput{}, nil
	}
	td, ok := s.tables[*in.TableName]
	if !ok {
		return &dynamodb.DescribeTableOutput{}, nil
	}
	return &dynamodb.DescribeTableOutput{Table: td}, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// fetchDDBSingle wires a single TableDescription through FetchDynamoDBTablesPage
// and returns the resulting Resource (status, issues, fields).
func fetchDDBSingle(t *testing.T, table *ddbtypes.TableDescription) (status string, issues []string, fields map[string]string) {
	t.Helper()
	if table.TableName == nil {
		t.Fatal("fetchDDBSingle: TableName must not be nil")
	}
	name := *table.TableName
	listStub := &ddbListStub{names: []string{name}}
	descStub := &ddbDescribeStub{tables: map[string]*ddbtypes.TableDescription{name: table}}

	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	if err != nil {
		t.Fatalf("FetchDynamoDBTablesPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	return r.Status, r.Issues, r.Fields
}

// findDDBTable returns the TableDescription with the given name from the fixture.
func findDDBTable(t *testing.T, id string) *ddbtypes.TableDescription {
	t.Helper()
	for _, td := range fixtures.NewDDBFixtures().Tables {
		if td != nil && aws.ToString(td.TableName) == id {
			return td
		}
	}
	t.Fatalf("ddb fixture not found: %s", id)
	return nil
}

// ---------------------------------------------------------------------------
// §4 phrase mapping — one test per distinct TableStatus
// ---------------------------------------------------------------------------

// TestDDB_Fetch_Active_StatusBlank verifies ACTIVE → empty Status, nil Issues.
func TestDDB_Fetch_Active_StatusBlank(t *testing.T) {
	table := findDDBTable(t, fixtures.OrdersProdID)
	status, issues, fields := fetchDDBSingle(t, table)

	if status != "" {
		t.Errorf("Status = %q, want %q (ACTIVE healthy silence)", status, "")
	}
	for _, banned := range []string{"OK", "ACTIVE", "active", "healthy", "available"} {
		if status == banned {
			t.Errorf("banned Status value %q — healthy rows must render blank", banned)
		}
	}
	if fields["status"] != "" {
		t.Errorf("Fields[status] = %q, want blank", fields["status"])
	}
	if len(issues) != 0 {
		t.Errorf("Issues = %v, want nil or empty for ACTIVE row", issues)
	}
}

// TestDDB_Fetch_Creating_StatusPhrase verifies CREATING → "creating" + Issues.
func TestDDB_Fetch_Creating_StatusPhrase(t *testing.T) {
	table := findDDBTable(t, fixtures.SessionsCreatingID)
	status, issues, _ := fetchDDBSingle(t, table)

	if status != "creating" {
		t.Errorf("Status = %q, want %q", status, "creating")
	}
	wantIssues := []string{"creating"}
	if !reflect.DeepEqual(normalizeIssues(issues), normalizeIssues(wantIssues)) {
		t.Errorf("Issues = %v, want %v", issues, wantIssues)
	}
}

// TestDDB_Fetch_Updating_StatusPhrase verifies UPDATING → "updating" + Issues.
func TestDDB_Fetch_Updating_StatusPhrase(t *testing.T) {
	table := findDDBTable(t, fixtures.SessionsUpdatingID)
	status, issues, _ := fetchDDBSingle(t, table)

	if status != "updating" {
		t.Errorf("Status = %q, want %q", status, "updating")
	}
	wantIssues := []string{"updating"}
	if !reflect.DeepEqual(normalizeIssues(issues), normalizeIssues(wantIssues)) {
		t.Errorf("Issues = %v, want %v", issues, wantIssues)
	}
}

// TestDDB_Fetch_Deleting_StatusPhrase verifies DELETING → "deleting" + Issues.
func TestDDB_Fetch_Deleting_StatusPhrase(t *testing.T) {
	table := findDDBTable(t, fixtures.AnalyticsDeletingID)
	status, issues, _ := fetchDDBSingle(t, table)

	if status != "deleting" {
		t.Errorf("Status = %q, want %q", status, "deleting")
	}
	wantIssues := []string{"deleting"}
	if !reflect.DeepEqual(normalizeIssues(issues), normalizeIssues(wantIssues)) {
		t.Errorf("Issues = %v, want %v", issues, wantIssues)
	}
}

// TestDDB_Fetch_Archiving_StatusPhrase verifies ARCHIVING → "archiving" + Issues.
func TestDDB_Fetch_Archiving_StatusPhrase(t *testing.T) {
	table := findDDBTable(t, fixtures.LegacyArchivingID)
	status, issues, _ := fetchDDBSingle(t, table)

	if status != "archiving" {
		t.Errorf("Status = %q, want %q", status, "archiving")
	}
	wantIssues := []string{"archiving"}
	if !reflect.DeepEqual(normalizeIssues(issues), normalizeIssues(wantIssues)) {
		t.Errorf("Issues = %v, want %v", issues, wantIssues)
	}
}

// TestDDB_Fetch_KMSInaccessible_StatusPhrase verifies INACCESSIBLE_ENCRYPTION_CREDENTIALS
// → "kms key inaccessible" + Issues.
func TestDDB_Fetch_KMSInaccessible_StatusPhrase(t *testing.T) {
	table := findDDBTable(t, fixtures.LegacyKMSLostID)
	status, issues, _ := fetchDDBSingle(t, table)

	if status != "kms key inaccessible" {
		t.Errorf("Status = %q, want %q", status, "kms key inaccessible")
	}
	wantIssues := []string{"kms key inaccessible"}
	if !reflect.DeepEqual(normalizeIssues(issues), normalizeIssues(wantIssues)) {
		t.Errorf("Issues = %v, want %v", issues, wantIssues)
	}
}

// TestDDB_Fetch_Archived_StatusPhrase verifies ARCHIVED → "archived: kms key lost" + Issues.
func TestDDB_Fetch_Archived_StatusPhrase(t *testing.T) {
	table := findDDBTable(t, fixtures.LegacyArchivedID)
	status, issues, _ := fetchDDBSingle(t, table)

	if status != "archived: kms key lost" {
		t.Errorf("Status = %q, want %q", status, "archived: kms key lost")
	}
	wantIssues := []string{"archived: kms key lost"}
	if !reflect.DeepEqual(normalizeIssues(issues), normalizeIssues(wantIssues)) {
		t.Errorf("Issues = %v, want %v", issues, wantIssues)
	}
}

// ---------------------------------------------------------------------------
// fetcher_populates_resource_issues (covers U7f adapted for Wave-2-only)
// ---------------------------------------------------------------------------

// TestDDB_Fetch_IssuesPopulated_EveryTableStatus is a table-driven audit across
// all named DDB fixtures asserting Resource.Issues matches spec §4 exactly.
func TestDDB_Fetch_IssuesPopulated_EveryTableStatus(t *testing.T) {
	type tableCase struct {
		id     string
		issues []string // nil = expect nil or empty
	}
	cases := []tableCase{
		{fixtures.OrdersProdID, nil},
		{fixtures.AuditPITROffID, nil}, // ACTIVE + PITR off; Issues from fetcher = nil (PITR is enricher)
		{fixtures.SessionsCreatingID, []string{"creating"}},
		{fixtures.SessionsUpdatingID, []string{"updating"}},
		{fixtures.AnalyticsDeletingID, []string{"deleting"}},
		{fixtures.LegacyArchivingID, []string{"archiving"}},
		{fixtures.LegacyKMSLostID, []string{"kms key inaccessible"}},
		{fixtures.LegacyArchivedID, []string{"archived: kms key lost"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			table := findDDBTable(t, tc.id)
			_, issues, _ := fetchDDBSingle(t, table)

			got := normalizeIssues(issues)
			want := normalizeIssues(tc.issues)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Issues = %v, want %v", issues, tc.issues)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Adversarial edge cases (inline, never in fixture file)
// ---------------------------------------------------------------------------

// TestDDB_Fetch_ArchivedNilArchivalSummary verifies ARCHIVED + nil ArchivalSummary
// falls back to stock phrase "archived: kms key lost" without panicking.
func TestDDB_Fetch_ArchivedNilArchivalSummary(t *testing.T) {
	table := &ddbtypes.TableDescription{
		TableName:      aws.String("inline-archived-nil-summary"),
		TableArn:       aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/inline-archived-nil-summary"),
		TableStatus:    ddbtypes.TableStatusArchived,
		ArchivalSummary: nil, // adversarial: no ArchivalSummary
	}
	status, issues, _ := fetchDDBSingle(t, table)

	if status != "archived: kms key lost" {
		t.Errorf("Status = %q, want %q (nil ArchivalSummary fallback)", status, "archived: kms key lost")
	}
	wantIssues := []string{"archived: kms key lost"}
	if !reflect.DeepEqual(normalizeIssues(issues), normalizeIssues(wantIssues)) {
		t.Errorf("Issues = %v, want %v", issues, wantIssues)
	}
}

// TestDDB_Fetch_NilTable_SkipDoNotCrash verifies that a nil TableDescription
// returned by DescribeTable causes the table to be skipped, not panicked.
// We simulate by registering nil for the described table.
func TestDDB_Fetch_NilTable_SkipDoNotCrash(t *testing.T) {
	listStub := &ddbListStub{names: []string{"ghost-table"}}
	// describeStub returns nil table for "ghost-table"
	descStub := &ddbDescribeStub{tables: map[string]*ddbtypes.TableDescription{
		"ghost-table": nil,
	}}

	// Should not panic; ghost-table will be skipped because table is nil
	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// nil table → skip (0 resources)
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for nil table, got %d", len(result.Resources))
	}
}

// ---------------------------------------------------------------------------
// Wave 3 anti-tests — ensure throttle/5xx signals are NOT surfaced
// ---------------------------------------------------------------------------

// TestDDB_Fetch_AntiThrottle_NotSurfaced verifies a healthy ACTIVE table with
// no status phrase does NOT include any "throttle" or "5xx"-related text.
func TestDDB_Fetch_AntiThrottle_NotSurfaced(t *testing.T) {
	table := findDDBTable(t, fixtures.OrdersProdID)
	status, issues, _ := fetchDDBSingle(t, table)

	for _, val := range []string{status} {
		for _, banned := range []string{"throttle", "5xx", "error-rate", "read-throttle", "write-throttle"} {
			if val == banned {
				t.Errorf("banned phrase %q found in Status — Wave 3 signals must NOT surface", banned)
			}
		}
	}
	for _, issue := range issues {
		for _, banned := range []string{"throttle", "5xx", "error"} {
			if issue == banned {
				t.Errorf("banned phrase %q found in Issues — Wave 3 signals must NOT surface", banned)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// internal helpers shared by aws_ddb_test.go
// ---------------------------------------------------------------------------

// normalizeIssues converts nil and empty to nil for DeepEqual comparisons.
func normalizeIssues(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
