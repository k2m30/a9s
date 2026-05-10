package unit

// aws_dbi_test.go — fetcher behavior tests for RDS DB Instances.
// // Tests drive FetchRDSInstancesPage with a mock RDS client and assert
// that Resource.Findings and Resource.Fields follow spec §4 precedence ():
// - Resource.Status is always "".
// - Healthy available row → blank Fields["status"], nil Findings (silence).
// - Transitional statuses → Fields["status"] = bare keyword or "keyword: PendingModifiedValues key".
// - Broken statuses → Findings with exact keyword (inaccessible-encryption-credentials remapped).
// - Config warnings on available → Findings in first-wins precedence order.
// - cis_flags field must NOT be present (no jargon columns).

import (
	"context"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// mockRDSPageClient implements awsclient.RDSDescribeDBInstancesAPI for
// fetcher-behavior tests (single page, no pagination).
type mockRDSPageClient struct {
	instances []rdstypes.DBInstance
	err       error
}

func (m *mockRDSPageClient) DescribeDBInstances(
	_ context.Context,
	_ *rds.DescribeDBInstancesInput,
	_ ...func(*rds.Options),
) (*rds.DescribeDBInstancesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &rds.DescribeDBInstancesOutput{DBInstances: m.instances}, nil
}

// findDBI locates a single DBInstance fixture by identifier.
func findDBI(t *testing.T, id string) rdstypes.DBInstance {
	t.Helper()
	for _, i := range fixtures.NewDBIFixtures().Instances {
		if aws.ToString(i.DBInstanceIdentifier) == id {
			return i
		}
	}
	t.Fatalf("fixture not found: %s", id)
	return rdstypes.DBInstance{}
}

// fetchSingle calls FetchRDSInstancesPage with one instance and returns
// status (always "" ), fields, and findings.
func fetchSingle(t *testing.T, inst rdstypes.DBInstance) (status string, fields map[string]string, findings []domain.Finding) {
	t.Helper()
	mock := &mockRDSPageClient{instances: []rdstypes.DBInstance{inst}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	return r.Status, r.Fields, r.Findings
}

// ---------------------------------------------------------------------------
// Wave 1 — happy-path silence
// ---------------------------------------------------------------------------

// TestDBI_Fetch_AvailableHealthy_StatusBlank verifies that a fully-healthy
// "available" instance produces an empty Status (spec §4: healthy rows render blank S4).
func TestDBI_Fetch_AvailableHealthy_StatusBlank(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	status, fields, findings := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	for _, banned := range []string{"OK", "available", "ACTIVE", "running", "healthy", "-"} {
		if fields["status"] == banned {
			t.Errorf("banned Fields[status] value %q — healthy rows must render blank", banned)
		}
	}
	if fields["status"] != "" {
		t.Errorf("Fields[status] = %q, want %q (healthy silence)", fields["status"], "")
	}
	if len(findings) != 0 {
		t.Errorf("Findings = %v, want nil/empty for healthy row", findings)
	}
}

// ---------------------------------------------------------------------------
// Wave 1 — transitional statuses
// ---------------------------------------------------------------------------

// TestDBI_Fetch_Modifying_WithPendingClassChange verifies that a "modifying"
// instance with PendingModifiedValues.DBInstanceClass set renders
// "modifying: DBInstanceClass" per spec §4.
func TestDBI_Fetch_Modifying_WithPendingClassChange(t *testing.T) {
	inst := findDBI(t, fixtures.StagingDbiModifyingID)
	status, fields, _ := fetchSingle(t, inst)

	want := "modifying: DBInstanceClass"
	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != want {
		t.Errorf("Fields[status] = %q, want %q", fields["status"], want)
	}
}

// TestDBI_Fetch_Rebooting_NoPending verifies that a "rebooting" instance with
// all-empty PendingModifiedValues renders bare "rebooting".
func TestDBI_Fetch_Rebooting_NoPending(t *testing.T) {
	inst := findDBI(t, fixtures.StagingDbiRebootingID)
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "rebooting" {
		t.Errorf("Fields[status] = %q, want %q", fields["status"], "rebooting")
	}
}

// TestDBI_Fetch_TransitionalKeywords_AllBare verifies that all 14 transitional
// keywords produce their bare keyword when PendingModifiedValues is nil/empty.
func TestDBI_Fetch_TransitionalKeywords_AllBare(t *testing.T) {
	keywords := []string{
		"creating", "backing-up", "renaming", "resetting-master-credentials",
		"starting", "stopping", "upgrading", "maintenance",
		"configuring-enhanced-monitoring", "configuring-iam-database-auth",
		"configuring-log-exports", "converting-to-vpc", "moving-to-vpc",
		"storage-optimization",
	}
	for _, kw := range keywords {
		kw := kw
		t.Run(kw, func(t *testing.T) {
			inst := rdstypes.DBInstance{
				DBInstanceIdentifier: aws.String("inline-" + kw),
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-" + kw),
				DBInstanceStatus:     aws.String(kw),
				BackupRetentionPeriod: aws.Int32(7),
				PubliclyAccessible:   aws.Bool(false),
				StorageEncrypted:     aws.Bool(true),
				DeletionProtection:   aws.Bool(true),
			}
			status, fields, _ := fetchSingle(t, inst)
			if status != "" {
				t.Errorf("Status = %q, want %q", status, "")
			}
			if fields["status"] != kw {
				t.Errorf("Fields[status] = %q, want bare keyword %q", fields["status"], kw)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Wave 1 — broken statuses
// ---------------------------------------------------------------------------

// TestDBI_Fetch_BrokenStatuses verifies that each broken status keyword
// passes through directly (except inaccessible-encryption-credentials which
// is remapped per spec §4).
func TestDBI_Fetch_BrokenStatuses(t *testing.T) {
	cases := []struct {
		status string
		want   string
	}{
		{"failed", "failed"},
		{"storage-full", "storage-full"},
		{"incompatible-network", "incompatible-network"},
		{"incompatible-option-group", "incompatible-option-group"},
		{"incompatible-parameters", "incompatible-parameters"},
		{"incompatible-restore", "incompatible-restore"},
		{"restore-error", "restore-error"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.status, func(t *testing.T) {
			inst := rdstypes.DBInstance{
				DBInstanceIdentifier: aws.String("inline-broken-" + tc.status),
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-" + tc.status),
				DBInstanceStatus:     aws.String(tc.status),
				BackupRetentionPeriod: aws.Int32(7),
				PubliclyAccessible:   aws.Bool(false),
				StorageEncrypted:     aws.Bool(true),
				DeletionProtection:   aws.Bool(true),
			}
			status, fields, _ := fetchSingle(t, inst)
			if status != "" {
				t.Errorf("Status = %q, want %q", status, "")
			}
			if fields["status"] != tc.want {
				t.Errorf("Fields[status] = %q, want %q", fields["status"], tc.want)
			}
		})
	}
}

// TestDBI_Fetch_InaccessibleEncryptionCredentials_Remap verifies that
// "inaccessible-encryption-credentials" is remapped to
// "encryption key unavailable" (spec §4 remap table).
func TestDBI_Fetch_InaccessibleEncryptionCredentials_Remap(t *testing.T) {
	inst := findDBI(t, fixtures.BrokenDbiEncryptionLockedID)
	status, fields, _ := fetchSingle(t, inst)

	want := "encryption key unavailable"
	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != want {
		t.Errorf("Fields[status] = %q, want %q", fields["status"], want)
	}
}

// TestDBI_Fetch_BrokenPrecedenceOverConfigWarnings verifies that a broken
// status takes precedence over any config warnings (spec §4 Broken precedence).
func TestDBI_Fetch_BrokenPrecedenceOverConfigWarnings(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier: aws.String("inline-precedence-broken"),
		DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-precedence-broken"),
		DBInstanceStatus:     aws.String("storage-full"),
		BackupRetentionPeriod: aws.Int32(0),   // would trigger "no automated backups"
		PubliclyAccessible:   aws.Bool(true),   // would trigger "publicly accessible"
		StorageEncrypted:     aws.Bool(false),  // would trigger "unencrypted storage"
		DeletionProtection:   aws.Bool(false),  // would trigger "deletion protection off"
	}
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "storage-full" {
		t.Errorf("Fields[status] = %q, want %q (broken beats config warnings)", fields["status"], "storage-full")
	}
}

// ---------------------------------------------------------------------------
// Wave 1 — config warnings on available rows
// ---------------------------------------------------------------------------

// TestDBI_Fetch_NoAutomatedBackups verifies BackupRetentionPeriod=0 produces
// "no automated backups" on an otherwise-healthy available instance.
func TestDBI_Fetch_NoAutomatedBackups(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiNoBackupsID)
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "no automated backups" {
		t.Errorf("Fields[status] = %q, want %q", fields["status"], "no automated backups")
	}
}

// TestDBI_Fetch_PubliclyAccessible verifies PubliclyAccessible=true on a
// healthy available instance produces "publicly accessible".
func TestDBI_Fetch_PubliclyAccessible(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiPublicID)
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "publicly accessible" {
		t.Errorf("Fields[status] = %q, want %q", fields["status"], "publicly accessible")
	}
}

// TestDBI_Fetch_UnencryptedStorage verifies StorageEncrypted=false on a
// healthy available instance produces "unencrypted storage".
func TestDBI_Fetch_UnencryptedStorage(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiUnencryptedID)
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "unencrypted storage" {
		t.Errorf("Fields[status] = %q, want %q", fields["status"], "unencrypted storage")
	}
}

// TestDBI_Fetch_DeletionProtectionOff verifies DeletionProtection=false on a
// healthy available instance produces "deletion protection off".
func TestDBI_Fetch_DeletionProtectionOff(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiUnprotectedID)
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "deletion protection off" {
		t.Errorf("Fields[status] = %q, want %q", fields["status"], "deletion protection off")
	}
}

// TestDBI_Fetch_WarningPrecedence verifies that when all four config warnings
// are present, "no automated backups" wins (first in spec §4 precedence) and
// the remaining 3 warnings produce a "(+3)" suffix (spec §4 universal rule 7).
func TestDBI_Fetch_WarningPrecedence(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String("inline-all-warnings"),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-all-warnings"),
		DBInstanceStatus:      aws.String("available"),
		BackupRetentionPeriod: aws.Int32(0),
		PubliclyAccessible:    aws.Bool(true),
		StorageEncrypted:      aws.Bool(false),
		DeletionProtection:    aws.Bool(false),
	}
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	// All 4 warnings → top phrase wins + "(+3)" for the 3 hidden warnings.
	if fields["status"] != "no automated backups (+3)" {
		t.Errorf("Fields[status] = %q, want %q (backups > public > unencrypted > no-protection, +3 suffix)", fields["status"], "no automated backups (+3)")
	}
}

// ---------------------------------------------------------------------------
// Universal invariants — no cis_flags field
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Wave 1 — multi-warning (+N) suffix (spec §4 universal rule 7)
// ---------------------------------------------------------------------------

// TestDBI_Fetch_MultiW1Warnings_SuffixThree verifies that when 3 of the 4 config
// warnings are present on an available instance, Status shows the top-precedence
// phrase followed by "(+2)" — one hidden finding per extra warning.
func TestDBI_Fetch_MultiW1Warnings_SuffixThree(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiMultiID)
	status, fields, _ := fetchSingle(t, inst)

	want := "no automated backups (+2)"
	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != want {
		t.Errorf("Fields[status] = %q, want %q (3 warnings: backups+public+unencrypted, deletion-protection=true)", fields["status"], want)
	}
}

// TestDBI_Fetch_MultiW1Warnings_SuffixFour verifies that all 4 config warnings
// produce "(+3)" — only the first phrase is shown, 3 more are hidden.
func TestDBI_Fetch_MultiW1Warnings_SuffixFour(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String("inline-all-4-warnings"),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-all-4-warnings"),
		DBInstanceStatus:      aws.String("available"),
		BackupRetentionPeriod: aws.Int32(0),    // warning 1: no automated backups
		PubliclyAccessible:    aws.Bool(true),  // warning 2: publicly accessible
		StorageEncrypted:      aws.Bool(false), // warning 3: unencrypted storage
		DeletionProtection:    aws.Bool(false), // warning 4: deletion protection off
	}
	status, fields, _ := fetchSingle(t, inst)

	want := "no automated backups (+3)"
	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != want {
		t.Errorf("Fields[status] = %q, want %q (all 4 warnings stacked)", fields["status"], want)
	}
}

// TestDBI_Fetch_MultiW1Warnings_PrecedenceOrder verifies that when only
// PubliclyAccessible and DeletionProtection warnings are present (backups+encryption OK),
// "publicly accessible" wins as the top-precedence phrase with "(+1)" for the
// hidden deletion-protection warning.
func TestDBI_Fetch_MultiW1Warnings_PrecedenceOrder(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String("inline-public-and-no-protect"),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-public-and-no-protect"),
		DBInstanceStatus:      aws.String("available"),
		BackupRetentionPeriod: aws.Int32(7),    // OK
		PubliclyAccessible:    aws.Bool(true),  // warning 1: publicly accessible
		StorageEncrypted:      aws.Bool(true),  // OK
		DeletionProtection:    aws.Bool(false), // warning 2: deletion protection off
	}
	status, fields, _ := fetchSingle(t, inst)

	want := "publicly accessible (+1)"
	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != want {
		t.Errorf("Fields[status] = %q, want %q (public beats deletion-protection per §4 precedence)", fields["status"], want)
	}
}

// TestDBI_Fetch_SingleW1Warning_NoSuffix_Regression is a regression pin: a
// single warning must NOT get a suffix — spec §4 universal rule 7 only applies
// when N >= 2 warnings are present.
func TestDBI_Fetch_SingleW1Warning_NoSuffix_Regression(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiPublicID)
	status, fields, _ := fetchSingle(t, inst)

	want := "publicly accessible"
	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != want {
		t.Errorf("Fields[status] = %q, want %q (single warning must not have suffix)", fields["status"], want)
	}
}

// TestDBI_Fetch_HealthyInstance_NoSuffix verifies that a healthy instance
// produces an empty Status — no suffix, no phrase.
func TestDBI_Fetch_HealthyInstance_NoSuffix(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "" {
		t.Errorf("Fields[status] = %q, want %q (healthy instance must produce blank)", fields["status"], "")
	}
}

// ---------------------------------------------------------------------------
// Universal invariants — no cis_flags field
// ---------------------------------------------------------------------------

// TestDBI_Fetch_NoCISFlagsField verifies that cis_flags is absent (or empty)
// in the fetcher output — spec §3.1 forbids jargon columns.
func TestDBI_Fetch_NoCISFlagsField(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	_, fields, _ := fetchSingle(t, inst)

	if val, ok := fields["cis_flags"]; ok && val != "" {
		t.Errorf("Fields[cis_flags] = %q — jargon field must not appear in output (spec §3.1)", val)
	}
}

// ---------------------------------------------------------------------------
// Detail fields populated correctly on the baseline Healthy instance
// ---------------------------------------------------------------------------

// fetchSingleResource calls FetchRDSInstancesPage with one instance and returns the full Resource.
func fetchSingleResource(t *testing.T, inst rdstypes.DBInstance) resource.Resource {
	t.Helper()
	mock := &mockRDSPageClient{instances: []rdstypes.DBInstance{inst}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

// ---------------------------------------------------------------------------
// Resource.Findings population — spec rule 7 (S5): "every finding individually visible"
// ---------------------------------------------------------------------------

// TestDBI_Fetch_FindingsPopulated_Healthy verifies that a fully-healthy available
// instance produces nil or empty Findings (no warnings to surface in S5).
func TestDBI_Fetch_FindingsPopulated_Healthy(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	r := fetchSingleResource(t, inst)

	if len(r.Findings) != 0 {
		phrases := make([]string, len(r.Findings))
		for i, f := range r.Findings {
			phrases[i] = f.Phrase
		}
		t.Errorf("Findings = %v, want nil or empty for healthy row", phrases)
	}
}

// TestDBI_Fetch_FindingsPopulated_SingleWarning verifies that each single-warning
// fixture populates Resource.Findings with exactly one entry matching the §4 phrase.
func TestDBI_Fetch_FindingsPopulated_SingleWarning(t *testing.T) {
	cases := []struct{ id, want string }{
		{fixtures.WarnDbiNoBackupsID, "no automated backups"},
		{fixtures.WarnDbiPublicID, "publicly accessible"},
		{fixtures.WarnDbiUnencryptedID, "unencrypted storage"},
		{fixtures.WarnDbiUnprotectedID, "deletion protection off"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			inst := findDBI(t, tc.id)
			r := fetchSingleResource(t, inst)

			if len(r.Findings) != 1 {
				phrases := make([]string, len(r.Findings))
				for i, f := range r.Findings {
					phrases[i] = f.Phrase
				}
				t.Errorf("Findings length = %d, want 1; Findings = %v", len(r.Findings), phrases)
				return
			}
			if r.Findings[0].Phrase != tc.want {
				t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, tc.want)
			}
			if r.Findings[0].Source != "wave1" {
				t.Errorf("Findings[0].Source = %q, want %q", r.Findings[0].Source, "wave1")
			}
		})
	}
}

// TestDBI_Fetch_FindingsPopulated_MultiWarning verifies that warn-dbi-multi
// (3 Wave-1 warnings: no-backups + public + unencrypted, deletion-protection=true)
// produces Findings with length 3 in §4 precedence order.
func TestDBI_Fetch_FindingsPopulated_MultiWarning(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiMultiID)
	r := fetchSingleResource(t, inst)

	wantPhrases := []string{
		"no automated backups",
		"publicly accessible",
		"unencrypted storage",
	}

	if len(r.Findings) != len(wantPhrases) {
		phrases := make([]string, len(r.Findings))
		for i, f := range r.Findings {
			phrases[i] = f.Phrase
		}
		t.Fatalf("Findings length = %d, want %d; Findings = %v", len(r.Findings), len(wantPhrases), phrases)
	}
	for i, want := range wantPhrases {
		if r.Findings[i].Phrase != want {
			t.Errorf("Findings[%d].Phrase = %q, want %q (§4 precedence violated)", i, r.Findings[i].Phrase, want)
		}
	}
}

// TestDBI_Fetch_FindingsPopulated_AllFourWarnings verifies that an inline fixture
// with all 4 Wave-1 warnings produces Findings of length 4 in §4 precedence order.
func TestDBI_Fetch_FindingsPopulated_AllFourWarnings(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String("inline-all-four-warnings"),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-all-four-warnings"),
		DBInstanceStatus:      aws.String("available"),
		BackupRetentionPeriod: aws.Int32(0),    // warning 1: no automated backups
		PubliclyAccessible:    aws.Bool(true),  // warning 2: publicly accessible
		StorageEncrypted:      aws.Bool(false), // warning 3: unencrypted storage
		DeletionProtection:    aws.Bool(false), // warning 4: deletion protection off
	}
	r := fetchSingleResource(t, inst)

	wantPhrases := []string{
		"no automated backups",
		"publicly accessible",
		"unencrypted storage",
		"deletion protection off",
	}

	if len(r.Findings) != len(wantPhrases) {
		phrases := make([]string, len(r.Findings))
		for i, f := range r.Findings {
			phrases[i] = f.Phrase
		}
		t.Fatalf("Findings length = %d, want %d; Findings = %v", len(r.Findings), len(wantPhrases), phrases)
	}
	for i, want := range wantPhrases {
		if r.Findings[i].Phrase != want {
			t.Errorf("Findings[%d].Phrase = %q, want %q (§4 precedence violated)", i, r.Findings[i].Phrase, want)
		}
	}
}

// TestDBI_Fetch_FindingsPopulated_Broken verifies broken-status fixtures:
// Findings carries the single broken phrase (not config warnings, which are blocked
// by the broken-first precedence in §4).
func TestDBI_Fetch_FindingsPopulated_Broken(t *testing.T) {
	t.Run("storage-full", func(t *testing.T) {
		inst := findDBI(t, fixtures.BrokenDbiStorageFullID)
		r := fetchSingleResource(t, inst)

		if len(r.Findings) != 1 {
			phrases := make([]string, len(r.Findings))
			for i, f := range r.Findings {
				phrases[i] = f.Phrase
			}
			t.Fatalf("Findings length = %d, want 1; Findings = %v", len(r.Findings), phrases)
		}
		if r.Findings[0].Phrase != "storage-full" {
			t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "storage-full")
		}
	})

	t.Run("encryption-locked", func(t *testing.T) {
		inst := findDBI(t, fixtures.BrokenDbiEncryptionLockedID)
		r := fetchSingleResource(t, inst)

		if len(r.Findings) != 1 {
			phrases := make([]string, len(r.Findings))
			for i, f := range r.Findings {
				phrases[i] = f.Phrase
			}
			t.Fatalf("Findings length = %d, want 1; Findings = %v", len(r.Findings), phrases)
		}
		if r.Findings[0].Phrase != "encryption key unavailable" {
			t.Errorf("Findings[0].Phrase = %q, want %q (broken remap must appear in Findings)", r.Findings[0].Phrase, "encryption key unavailable")
		}
	})
}

// TestDBI_Fetch_FindingsPopulated_Transitional verifies transitional-status fixtures:
// Findings carries the single transitional phrase (with pending key suffix when present).
func TestDBI_Fetch_FindingsPopulated_Transitional(t *testing.T) {
	t.Run("modifying-with-pending-class", func(t *testing.T) {
		inst := findDBI(t, fixtures.StagingDbiModifyingID)
		r := fetchSingleResource(t, inst)

		if len(r.Findings) != 1 {
			phrases := make([]string, len(r.Findings))
			for i, f := range r.Findings {
				phrases[i] = f.Phrase
			}
			t.Fatalf("Findings length = %d, want 1; Findings = %v", len(r.Findings), phrases)
		}
		if r.Findings[0].Phrase != "modifying: DBInstanceClass" {
			t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "modifying: DBInstanceClass")
		}
	})

	t.Run("rebooting-no-pending", func(t *testing.T) {
		inst := findDBI(t, fixtures.StagingDbiRebootingID)
		r := fetchSingleResource(t, inst)

		if len(r.Findings) != 1 {
			phrases := make([]string, len(r.Findings))
			for i, f := range r.Findings {
				phrases[i] = f.Phrase
			}
			t.Fatalf("Findings length = %d, want 1; Findings = %v", len(r.Findings), phrases)
		}
		if r.Findings[0].Phrase != "rebooting" {
			t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "rebooting")
		}
	})
}

// findDBIFromAll locates a single DBInstance from the full RDS pool (canonical
// DBIFixtures + legacy RDSFixtures pool), so tests can reference fixtures like
// "db-public-no-encryption" that are not in the canonical dbi.go file.
func findDBIFromAll(t *testing.T, id string) rdstypes.DBInstance {
	t.Helper()
	for _, i := range fixtures.NewRDSFixtures().DBInstances {
		if aws.ToString(i.DBInstanceIdentifier) == id {
			return i
		}
	}
	t.Fatalf("fixture not found in full RDS pool: %s", id)
	return rdstypes.DBInstance{}
}

// TestDBI_Fetch_DetailFieldsPopulated verifies that all spec-required detail
// fields are populated with correct values for the prod-dbi-1 fixture.
func TestDBI_Fetch_DetailFieldsPopulated(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	mock := &mockRDSPageClient{instances: []rdstypes.DBInstance{inst}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	f := r.Fields

	checks := map[string]string{
		"publicly_accessible":     "false",
		"storage_encrypted":       "true",
		"deletion_protection":     "true",
		"backup_retention_period": "7",
		"engine":                  "postgres",
		"engine_version":          "16.2",
		"class":                   "db.r6g.large",
		"multi_az":                "Yes",
		"arn":                     fixtures.ProdDbiARN,
	}
	for key, want := range checks {
		got := f[key]
		if got != want {
			t.Errorf("Fields[%q] = %q, want %q", key, got, want)
		}
	}

	// endpoint must be non-empty
	if f["endpoint"] == "" {
		t.Error("Fields[endpoint] must not be empty for prod-dbi-1")
	}
}

// ---------------------------------------------------------------------------
// Resource.Findings per-fixture audit — complete coverage across all named fixtures
// ---------------------------------------------------------------------------

// TestDBI_Fetch_FindingsPopulated_EveryFixture is a table-driven audit that verifies
// Resource.Findings phrases are exactly correct for every named DBI fixture. Nil expected
// phrases means the fixture is Healthy and must produce empty Findings (spec §4:
// Healthy silence). Non-nil expected phrases must match in exact §4 precedence order.
// // Uses reflect.DeepEqual for slice equality — catches both missing phrases and
// ordering bugs that the existing per-fixture spot-checks would miss.
func TestDBI_Fetch_FindingsPopulated_EveryFixture(t *testing.T) {
	type findingCase struct {
		id      string
		phrases []string // nil = expect nil or empty Findings
	}
	cases := []findingCase{
		// Healthy rows — must produce nil/empty Findings.
		{fixtures.ProdDbiID, nil},
		{fixtures.ProdDbiAuroraID, nil},
		// Transitional (Wave-1, single phrase).
		{fixtures.StagingDbiModifyingID, []string{"modifying: DBInstanceClass"}},
		{fixtures.StagingDbiRebootingID, []string{"rebooting"}},
		// Broken (Wave-1, single phrase).
		{fixtures.BrokenDbiStorageFullID, []string{"storage-full"}},
		{fixtures.BrokenDbiEncryptionLockedID, []string{"encryption key unavailable"}},
		// Single Config Warnings.
		{fixtures.WarnDbiNoBackupsID, []string{"no automated backups"}},
		{fixtures.WarnDbiPublicID, []string{"publicly accessible"}},
		{fixtures.WarnDbiUnencryptedID, []string{"unencrypted storage"}},
		{fixtures.WarnDbiUnprotectedID, []string{"deletion protection off"}},
		// Multi Config Warnings.
		{fixtures.WarnDbiMultiID, []string{"no automated backups", "publicly accessible", "unencrypted storage"}},
		// Wave-1 warning + Wave-2 maintenance — Findings carries Wave-1 phrases only.
		{fixtures.WarnDbiPublicMaintID, []string{"publicly accessible"}},
		// Wave-2 only on Healthy row — Findings must be nil/empty (Wave-2 is not in Findings).
		{fixtures.MaintDbiScheduledID, nil},
		// Legacy fixture with all 4 Wave-1 warnings — sourced from full RDS pool.
		{"db-public-no-encryption", []string{"no automated backups", "publicly accessible", "unencrypted storage", "deletion protection off"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			// "db-public-no-encryption" lives in the legacy RDS pool, not NewDBIFixtures.
			var inst rdstypes.DBInstance
			if tc.id == "db-public-no-encryption" {
				inst = findDBIFromAll(t, tc.id)
			} else {
				inst = findDBI(t, tc.id)
			}
			r := fetchSingleResource(t, inst)

			// Extract phrases from Findings for comparison.
			got := make([]string, len(r.Findings))
			for i, f := range r.Findings {
				got[i] = f.Phrase
			}
			if len(got) == 0 {
				got = nil
			}
			want := tc.phrases
			if len(want) == 0 {
				want = nil
			}

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: Findings phrases = %v, want %v", tc.id, got, tc.phrases)
			}
		})
	}
}
