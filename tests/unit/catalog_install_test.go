package unit

// catalog_install_test.go — AS-800: failing tests for the AS-795a cycle-break scaffold.
// These tests assert aws.Install() + catalog.SetTypes behaviors.
// They compile and pass only after the Coder (AS-799) implements the scaffold.
//
// Coverage:
//   1. Smoke: aws.Install() → catalog.Find("ec2") non-nil
//   2. Idempotence: double-Install does not panic, count stable
//   3. SetTypes panics on second call with different slice
//   4. catalog.Find panics before SetTypes (sub-process)
//   5. 12-type golden parity (one per category)
//   6. AS-795a invariant: Fetcher/Related/Navigable/Wave2 are zero for all types
//   6b. AS-795a invariant: new fields FieldKeys/FieldAliases/FetchByIDs/
//       FilteredFetcher/IssueEnricherFieldKeys/ChildFetcher are zero for all types

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
)

// Test 1 — Smoke: Install() makes catalog.Find("ec2") return a non-nil entry.
func TestCatalogInstall_Smoke_EC2NonNil(t *testing.T) {
	got := catalog.Find("ec2")
	if got == nil {
		t.Fatal("catalog.Find(\"ec2\") returned nil after aws.Install(); expected non-nil entry")
	}
}

// Test 2 — Idempotence: calling Install() a second time must not panic and
// must leave catalog.All() count unchanged.
func TestCatalogInstall_Idempotent(t *testing.T) {
	before := len(catalog.All())
	if before == 0 {
		t.Fatal("catalog.All() returned empty slice before second Install(); TestMain may not have called aws.Install()")
	}
	awsclient.Install()
	after := len(catalog.All())
	if after != before {
		t.Fatalf("catalog.All() count changed from %d to %d after second aws.Install(); Install must be idempotent on identical input", before, after)
	}
}

// Test 3 — Defensive guard: catalog.SetTypes must panic (or return an error
// captured as a panic) when called a second time with a different slice.
// This guards against accidental double-install with divergent data.
func TestCatalogSetTypes_PanicsOnDifferentSlice(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("catalog.SetTypes with a different slice did not panic; expected panic to guard against double-install")
		}
	}()
	catalog.SetTypes([]catalog.ResourceTypeDef{
		{ShortName: "test-sentinel", Name: "Sentinel Only", Category: "TEST"},
	})
}

// Test 4 — Panic-before-SetTypes: catalog.Find must panic with a clear message
// when called before aws.Install() (i.e., before SetTypes). Exercised via
// sub-process so the panic does not abort this binary's test run.
func TestCatalogFind_PanicsBeforeSetTypes(t *testing.T) {
	if os.Getenv("TEST_CATALOG_PANIC") == "1" {
		// Running in the sub-process. TEST_SKIP_INSTALL=1 was set by the
		// parent so TestMain skipped aws.Install(). Calling Find now must panic.
		catalog.Find("ec2")
		os.Exit(0) // unreachable when panic fires correctly
	}

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestCatalogFind_PanicsBeforeSetTypes$")
	cmd.Env = append(os.Environ(),
		"TEST_CATALOG_PANIC=1",
		"TEST_SKIP_INSTALL=1",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("sub-process exited 0; expected panic/non-zero exit when catalog.Find is called before SetTypes")
	}
	output := string(out)
	if !strings.Contains(output, "catalog.SetTypes not called") {
		t.Fatalf("panic message missing expected text %q; full output: %s",
			"catalog.SetTypes not called", output)
	}
}

// goldenEntry is the expected identity for one representative type per category.
type goldenEntry struct {
	findKey   string // key passed to catalog.Find
	shortName string // expected ShortName on the returned entry
	name      string // expected Name
	category  string // expected Category
}

// Test 5 — Golden parity: one representative per each of the 12 categories must
// return the same Name/ShortName/Category values it had before the refactor.
func TestCatalogInstall_GoldenParity(t *testing.T) {
	golden := []goldenEntry{
		{"ec2", "ec2", "EC2 Instances", "COMPUTE"},
		{"eks", "eks", "EKS Clusters", "CONTAINERS"},
		{"vpc", "vpc", "VPCs", "NETWORKING"},
		{"rds", "dbi", "DB Instances", "DATABASES & STORAGE"},  // "rds" is an alias for ShortName "dbi"
		{"alarm", "alarm", "CloudWatch Alarms", "MONITORING"},
		{"sns", "sns", "SNS Topics", "MESSAGING"},
		{"secrets", "secrets", "Secrets Manager", "SECRETS & CONFIG"},
		{"cf", "cf", "CloudFront Distributions", "DNS & CDN"},
		{"role", "role", "IAM Roles", "SECURITY & IAM"},
		{"cb", "cb", "CodeBuild Projects", "CI/CD"},
		{"s3", "s3", "S3 Buckets", "DATABASES & STORAGE"},
		{"backup", "backup", "Backup Plans", "BACKUP"},
	}
	for _, g := range golden {
		g := g
		t.Run(g.findKey, func(t *testing.T) {
			got := catalog.Find(g.findKey)
			if got == nil {
				t.Fatalf("catalog.Find(%q) returned nil; type missing after aws.Install()", g.findKey)
			}
			if got.ShortName != g.shortName {
				t.Errorf("ShortName: got %q, want %q", got.ShortName, g.shortName)
			}
			if got.Name != g.name {
				t.Errorf("Name: got %q, want %q", got.Name, g.name)
			}
			if got.Category != g.category {
				t.Errorf("Category: got %q, want %q", got.Category, g.category)
			}
		})
	}
}

// Test 6 — AS-795a invariant: every top-level entry must have zero-value
// Fetcher, Related, Navigable, and Wave2 in the scaffold PR.
// Per-category migration (AS-795b–m) will populate these fields.
func TestCatalogInstall_AS795a_ZeroRuntimeWiring(t *testing.T) {
	all := catalog.All()
	if len(all) == 0 {
		t.Fatal("catalog.All() returned empty slice; aws.Install() may not have been called")
	}
	for _, rt := range all {
		if rt.Fetcher != nil {
			t.Errorf("type %q: Fetcher is non-nil in AS-795a scaffold; migration belongs in AS-795b–m", rt.ShortName)
		}
		if len(rt.Related) != 0 {
			t.Errorf("type %q: Related is non-empty in AS-795a scaffold; migration belongs in AS-795b–m", rt.ShortName)
		}
		if len(rt.Navigable) != 0 {
			t.Errorf("type %q: Navigable is non-empty in AS-795a scaffold; migration belongs in AS-795b–m", rt.ShortName)
		}
		if rt.Wave2 != nil {
			t.Errorf("type %q: Wave2 is non-nil in AS-795a scaffold; migration belongs in AS-795b–m", rt.ShortName)
		}
	}
}

// Test 6b — AS-795a invariant: new ResourceTypeDef fields introduced by the
// scaffold (FieldKeys, FieldAliases, FetchByIDs, FilteredFetcher,
// IssueEnricherFieldKeys, ChildFetcher) must be zero-valued for every
// top-level entry. Per-category PRs (AS-795b–m) populate them.
func TestCatalogInstall_AS795a_NewFieldsZeroValued(t *testing.T) {
	all := catalog.All()
	if len(all) == 0 {
		t.Fatal("catalog.All() returned empty slice; aws.Install() may not have been called")
	}
	for _, rt := range all {
		if len(rt.FieldKeys) != 0 {
			t.Errorf("type %q: FieldKeys non-empty in AS-795a scaffold", rt.ShortName)
		}
		if len(rt.FieldAliases) != 0 {
			t.Errorf("type %q: FieldAliases non-empty in AS-795a scaffold", rt.ShortName)
		}
		if rt.FetchByIDs != nil {
			t.Errorf("type %q: FetchByIDs non-nil in AS-795a scaffold", rt.ShortName)
		}
		if rt.FilteredFetcher != nil {
			t.Errorf("type %q: FilteredFetcher non-nil in AS-795a scaffold", rt.ShortName)
		}
		if len(rt.IssueEnricherFieldKeys) != 0 {
			t.Errorf("type %q: IssueEnricherFieldKeys non-empty in AS-795a scaffold", rt.ShortName)
		}
		if rt.ChildFetcher != nil {
			t.Errorf("type %q: ChildFetcher non-nil in AS-795a scaffold", rt.ShortName)
		}
	}
}
