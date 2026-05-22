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
	"github.com/k2m30/a9s/v3/internal/resource"
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
			if got.Fetcher == nil {
				t.Errorf("Fetcher: got nil, want non-nil (AS-795 acceptance: every top-level catalog entry must have a populated Fetcher after the AS-795b..p init() cleanup)")
			}
		})
	}
}

// Test 6 — AS-795 progressive-migration invariant: every entry with a non-nil
// Fetcher must also carry FieldKeys + (Related OR Navigable) so partial
// scaffolds don't slip into main. AS-795b–m flip types one category at a time;
// this test grows with each migration but the shape stays identical.
//
// AS-795n exception: Wave2 migration is global (one PR migrates every
// remaining Wave 2 enricher into the catalog so the IssueEnricherRegistry map
// can be deleted). Wave2 in catalog without Fetcher in catalog is therefore
// expected for any type whose Fetcher hasn't been moved out of its
// legacy init() yet (acm/efs/r53/ecs-task are the remaining stragglers as
// of AS-795n). When the Wave2 field is non-nil but Fetcher is nil, we only
// require that the type still has a fetcher reachable via the legacy
// resource.GetPaginatedFetcher map (populated by the file's package init()).
//
// Replaces the AS-795a-era "zero wiring" guards, which were valid only while
// no category had been migrated (PR #392).
func TestCatalogInstall_AS795_MigrationShape(t *testing.T) {
	all := catalog.All()
	if len(all) == 0 {
		t.Fatal("catalog.All() returned empty slice; aws.Install() may not have been called")
	}
	for _, rt := range all {
		if rt.Fetcher == nil {
			if len(rt.FieldKeys) != 0 {
				t.Errorf("type %q: FieldKeys populated without Fetcher — partial migration?", rt.ShortName)
			}
			if rt.Wave2 != nil {
				// AS-795n: Wave2 may be in catalog while Fetcher is still in
				// legacy init(). Require the legacy fetcher to be present so
				// the type is still reachable in the running app.
				if resource.GetPaginatedFetcher(rt.ShortName) == nil {
					t.Errorf("type %q: Wave2 in catalog and no Fetcher anywhere (catalog or legacy) — type is unreachable", rt.ShortName)
				}
			}
			continue
		}
		if len(rt.FieldKeys) == 0 {
			t.Errorf("type %q: Fetcher set but FieldKeys empty — migration must populate both", rt.ShortName)
		}
		if len(rt.Related) == 0 && len(rt.Navigable) == 0 {
			t.Errorf("type %q: Fetcher set but no Related or Navigable wiring", rt.ShortName)
		}
	}
}
