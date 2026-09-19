package unit

import (
	"context"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

// status is always "": the fetcher writes the phrase only to Fields["status"]
// and Findings, and the "status must be empty" assertions check that.
func fetchSingle(t *testing.T, inst rdstypes.DBInstance) (status string, fields map[string]string, findings []domain.Finding) {
	t.Helper()
	mock := &fakeRDSDescribeDBInstances{Output: &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{inst}}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	return "", r.Fields, r.Findings
}

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
				DBInstanceIdentifier:  aws.String("inline-" + kw),
				DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-" + kw),
				DBInstanceStatus:      aws.String(kw),
				BackupRetentionPeriod: aws.Int32(7),
				PubliclyAccessible:    aws.Bool(false),
				StorageEncrypted:      aws.Bool(true),
				DeletionProtection:    aws.Bool(true),
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
				DBInstanceIdentifier:  aws.String("inline-broken-" + tc.status),
				DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-" + tc.status),
				DBInstanceStatus:      aws.String(tc.status),
				BackupRetentionPeriod: aws.Int32(7),
				PubliclyAccessible:    aws.Bool(false),
				StorageEncrypted:      aws.Bool(true),
				DeletionProtection:    aws.Bool(true),
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

func TestDBI_Fetch_BrokenPrecedenceOverConfigWarnings(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String("inline-precedence-broken"),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-precedence-broken"),
		DBInstanceStatus:      aws.String("storage-full"),
		BackupRetentionPeriod: aws.Int32(0),
		PubliclyAccessible:    aws.Bool(true),
		StorageEncrypted:      aws.Bool(false),
		DeletionProtection:    aws.Bool(false),
	}
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "storage-full" {
		t.Errorf("Fields[status] = %q, want %q (broken beats config warnings)", fields["status"], "storage-full")
	}
}

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

func TestDBI_Fetch_PubliclyAccessible(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiPublicID)
	status, fields, _ := fetchSingle(t, inst)

	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != "public endpoint" {
		t.Errorf("Fields[status] = %q, want %q", fields["status"], "public endpoint")
	}
}

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
	if fields["status"] != "no automated backups (+3)" {
		t.Errorf("Fields[status] = %q, want %q (backups > public > unencrypted > no-protection, +3 suffix)", fields["status"], "no automated backups (+3)")
	}
}

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

func TestDBI_Fetch_MultiW1Warnings_SuffixFour(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String("inline-all-4-warnings"),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-all-4-warnings"),
		DBInstanceStatus:      aws.String("available"),
		BackupRetentionPeriod: aws.Int32(0),
		PubliclyAccessible:    aws.Bool(true),
		StorageEncrypted:      aws.Bool(false),
		DeletionProtection:    aws.Bool(false),
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

func TestDBI_Fetch_MultiW1Warnings_PrecedenceOrder(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String("inline-public-and-no-protect"),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-public-and-no-protect"),
		DBInstanceStatus:      aws.String("available"),
		BackupRetentionPeriod: aws.Int32(7),
		PubliclyAccessible:    aws.Bool(true),
		StorageEncrypted:      aws.Bool(true),
		DeletionProtection:    aws.Bool(false),
	}
	status, fields, _ := fetchSingle(t, inst)

	want := "public endpoint (+1)"
	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != want {
		t.Errorf("Fields[status] = %q, want %q (public beats deletion-protection per §4 precedence)", fields["status"], want)
	}
}

func TestDBI_Fetch_SingleW1Warning_NoSuffix_Regression(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiPublicID)
	status, fields, _ := fetchSingle(t, inst)

	want := "public endpoint"
	if status != "" {
		t.Errorf("Status = %q, want %q", status, "")
	}
	if fields["status"] != want {
		t.Errorf("Fields[status] = %q, want %q (single warning must not have suffix)", fields["status"], want)
	}
}

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

func TestDBI_Fetch_NoCISFlagsField(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	_, fields, _ := fetchSingle(t, inst)

	if val, ok := fields["cis_flags"]; ok && val != "" {
		t.Errorf("Fields[cis_flags] = %q — jargon field must not appear in output (spec §3.1)", val)
	}
}

func fetchSingleResource(t *testing.T, inst rdstypes.DBInstance) resource.Resource {
	t.Helper()
	mock := &fakeRDSDescribeDBInstances{Output: &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{inst}}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

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

func TestDBI_Fetch_FindingsPopulated_SingleWarning(t *testing.T) {
	cases := []struct{ id, want string }{
		{fixtures.WarnDbiNoBackupsID, "no automated backups"},
		{fixtures.WarnDbiPublicID, "public endpoint"},
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

func TestDBI_Fetch_FindingsPopulated_MultiWarning(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiMultiID)
	r := fetchSingleResource(t, inst)

	wantPhrases := []string{
		"no automated backups",
		"public endpoint",
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

func TestDBI_Fetch_FindingsPopulated_AllFourWarnings(t *testing.T) {
	inst := rdstypes.DBInstance{
		DBInstanceIdentifier:  aws.String("inline-all-four-warnings"),
		DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:123456789012:db:inline-all-four-warnings"),
		DBInstanceStatus:      aws.String("available"),
		BackupRetentionPeriod: aws.Int32(0),
		PubliclyAccessible:    aws.Bool(true),
		StorageEncrypted:      aws.Bool(false),
		DeletionProtection:    aws.Bool(false),
	}
	r := fetchSingleResource(t, inst)

	wantPhrases := []string{
		"no automated backups",
		"public endpoint",
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

// findDBIFromAll searches the full RDS pool (DBIFixtures plus RDSFixtures),
// which holds fixtures such as "db-public-no-encryption" that dbi.go does not.
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

func TestDBI_Fetch_DetailFieldsPopulated(t *testing.T) {
	inst := findDBI(t, fixtures.ProdDbiID)
	mock := &fakeRDSDescribeDBInstances{Output: &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{inst}}}
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

	if f["endpoint"] == "" {
		t.Error("Fields[endpoint] must not be empty for prod-dbi-1")
	}
}

func TestDBI_Fetch_FindingsPopulated_EveryFixture(t *testing.T) {
	type findingCase struct {
		id      string
		phrases []string // nil = expect nil or empty Findings
	}
	cases := []findingCase{
		{fixtures.ProdDbiID, nil},
		{fixtures.ProdDbiAuroraID, nil},
		{fixtures.StagingDbiModifyingID, []string{"modifying: DBInstanceClass"}},
		{fixtures.StagingDbiRebootingID, []string{"rebooting"}},
		{fixtures.BrokenDbiStorageFullID, []string{"storage-full"}},
		{fixtures.BrokenDbiEncryptionLockedID, []string{"encryption key unavailable"}},
		{fixtures.WarnDbiNoBackupsID, []string{"no automated backups"}},
		{fixtures.WarnDbiPublicID, []string{"public endpoint"}},
		{fixtures.WarnDbiUnencryptedID, []string{"unencrypted storage"}},
		{fixtures.WarnDbiUnprotectedID, []string{"deletion protection off"}},
		{fixtures.WarnDbiMultiID, []string{"no automated backups", "public endpoint", "unencrypted storage"}},
		// The fetcher's Findings carry wave-1 phrases only; wave-2 maintenance
		// arrives through the enricher.
		{fixtures.WarnDbiPublicMaintID, []string{"public endpoint"}},
		{fixtures.MaintDbiScheduledID, nil},
		// normalizeRDSInstancePosture sets DeletionProtection on the whole RDS pool,
		// so only warn-dbi-unprotected (dbi.go) carries "deletion protection off".
		{"db-public-no-encryption", []string{"no automated backups", "public endpoint", "unencrypted storage"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			// "db-public-no-encryption" lives in the RDSFixtures pool.
			var inst rdstypes.DBInstance
			if tc.id == "db-public-no-encryption" {
				inst = findDBIFromAll(t, tc.id)
			} else {
				inst = findDBI(t, tc.id)
			}
			r := fetchSingleResource(t, inst)

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
