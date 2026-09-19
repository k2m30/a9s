package unit

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestCatalogInstall_Smoke_EC2NonNil(t *testing.T) {
	got := catalog.FindAny("ec2")
	if got == nil {
		t.Fatal("catalog.FindAny(\"ec2\") returned nil after aws.Install(); expected non-nil entry")
	}
}

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

// An empty RelatedDef.TargetType would render as a dimmed, non-navigable
// "(0)" pivot. validateRelatedDefs runs before SetTypes touches any global,
// so the catalog installed by TestMain must survive the rejected call.
func TestCatalogSetTypes_PanicsOnEmptyRelatedTargetType(t *testing.T) {
	before := len(catalog.All())

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("catalog.SetTypes with an empty RelatedDef.TargetType did not panic")
			}
			msg, ok := r.(string)
			if !ok || !strings.Contains(msg, "empty TargetType") {
				t.Fatalf("panic value = %v, want a message containing %q", r, "empty TargetType")
			}
		}()
		catalog.SetTypes([]catalog.ResourceTypeDef{
			{
				ShortName: "test-orphan-related",
				Name:      "Orphan Related",
				Category:  "TEST",
				Related: []domain.RelatedDef{
					{TargetType: "", DisplayName: "Dangling Pivot"},
				},
			},
		})
	}()

	after := len(catalog.All())
	if after != before {
		t.Fatalf("catalog.All() count changed from %d to %d after a rejected SetTypes call; the panic must fire before any global mutation", before, after)
	}
	if got := catalog.FindAny("ec2"); got == nil {
		t.Fatal("catalog.FindAny(\"ec2\") returned nil after a rejected SetTypes call — the installed catalog must survive an empty-TargetType rejection untouched")
	}
}

func TestCatalogSetChildTypes_PanicsOnEmptyRelatedTargetType(t *testing.T) {
	before := len(catalog.AllChildren())

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("catalog.SetChildTypes with an empty RelatedDef.TargetType did not panic")
			}
			msg, ok := r.(string)
			if !ok || !strings.Contains(msg, "empty TargetType") {
				t.Fatalf("panic value = %v, want a message containing %q", r, "empty TargetType")
			}
		}()
		catalog.SetChildTypes([]catalog.ResourceTypeDef{
			{
				ShortName: "test-orphan-related-child",
				Name:      "Orphan Related Child",
				Category:  "TEST",
				Related: []domain.RelatedDef{
					{TargetType: "", DisplayName: "Dangling Child Pivot"},
				},
			},
		})
	}()

	after := len(catalog.AllChildren())
	if after != before {
		t.Fatalf("catalog.AllChildren() count changed from %d to %d after a rejected SetChildTypes call; the panic must fire before any global mutation", before, after)
	}
	if got := catalog.ChildOnly("s3_objects"); got == nil {
		t.Fatal("catalog.ChildOnly(\"s3_objects\") returned nil after a rejected SetChildTypes call — the installed child catalog must survive an empty-TargetType rejection untouched")
	}
}

// Runs in a sub-process so the panic does not abort this test binary.
func TestCatalogFind_PanicsBeforeSetTypes(t *testing.T) {
	if os.Getenv("TEST_CATALOG_PANIC") == "1" {
		// TEST_SKIP_INSTALL=1 from the parent makes TestMain skip aws.Install().
		catalog.FindAny("ec2")
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

type goldenEntry struct {
	findKey   string
	shortName string
	name      string
	category  string
}

func TestCatalogInstall_GoldenParity(t *testing.T) {
	golden := []goldenEntry{
		{"ec2", "ec2", "EC2 Instances", "COMPUTE"},
		{"eks", "eks", "EKS Clusters", "CONTAINERS"},
		{"vpc", "vpc", "VPCs", "NETWORKING"},
		{"rds", "dbi", "DB Instances", "DATABASES & STORAGE"}, // "rds" is an alias for ShortName "dbi"
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
			got := catalog.FindAny(g.findKey)
			if got == nil {
				t.Fatalf("catalog.FindAny(%q) returned nil; type missing after aws.Install()", g.findKey)
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

// A type with Wave2 set and no catalog Fetcher must still be reachable
// through resource.GetPaginatedFetcher, populated by a package init().
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
