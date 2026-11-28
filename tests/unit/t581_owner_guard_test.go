package unit

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// Each fact below has one owner in core/aws. A second file deciding it is a
// second truth source that drifts from the first: it reads one log scope, one
// policy family or one IMDS default the owner already learned to read.
var t581FactOwners = []struct {
	fact    string
	pattern *regexp.Regexp
	owners  []string
	// once means the owner reads the setting in exactly one place, so the
	// finding and the field it also fills cannot disagree.
	once bool
}{
	{"web ACL logging configuration", regexp.MustCompile(`\.GetLoggingConfiguration\(`), []string{"waf_issue_enrichment.go"}, false},
	// ec2.go reads a running instance's HttpTokens, which AWS reports as
	// resolved at launch; every launch spec goes through raiseIMDSv1.
	{"IMDSv1 from launch metadata settings", regexp.MustCompile(`HttpTokensState(Required|Optional)\b`), []string{"imds_launch.go", "ec2.go"}, false},
	{"AMI IMDS default", regexp.MustCompile(`\bImdsSupport\b`), []string{"imds_launch.go"}, false},
	{"account IMDS defaults", regexp.MustCompile(`\.GetInstanceMetadataDefaults\(`), []string{"imds_launch.go"}, false},
	{"ELB security policy protocols", regexp.MustCompile(`\.DescribeSSLPolicies\(`), []string{"elb_issue_enrichment.go"}, false},
	// A policy is judged by the protocols AWS states for it, so no file
	// classifies one by its name.
	{"ELB security policy judged by name", regexp.MustCompile(`"ELBSecurityPolicy-`), nil, false},
	{"ECR registry scanning rules", regexp.MustCompile(`\.GetRegistryScanningConfiguration\(`), []string{"ecr_issue_enrichment.go"}, false},
	{"standard logging (v2) deliveries", regexp.MustCompile(`\.DescribeDeliver(ies|ySources)\(`), []string{"cf_issue_enrichment.go"}, false},
	{"Transfer security policy algorithms", regexp.MustCompile(`\.DescribeSecurityPolicy\(`), []string{"transfer_issue_enrichment.go"}, false},
	{"Transfer security policy judged by name", regexp.MustCompile(`"TransferSecurityPolicy-`), nil, false},
	{"OpenSearch encryption at rest", regexp.MustCompile(`EncryptionAtRestOptions\.Enabled\b`), []string{"opensearch.go"}, true},
	{"OpenSearch HTTPS enforced", regexp.MustCompile(`\.EnforceHTTPS\b`), []string{"opensearch.go"}, true},
	{"OpenSearch node-to-node encryption", regexp.MustCompile(`NodeToNodeEncryptionOptions\.Enabled\b`), []string{"opensearch.go"}, true},
	{"RDS and EBS snapshot encryption", regexp.MustCompile(`\bsnap\.Encrypted\b`), []string{"dbi_snap.go", "ebs.go"}, true},
}

func TestT581_SatisfyingSettingFactsHaveOneOwner(t *testing.T) {
	dir := filepath.Join("..", "..", "core", "aws")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, f := range t581FactOwners {
		var found []string
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			src, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			n := 0
			for line := range strings.Lines(string(src)) {
				if f.pattern.MatchString(line) {
					n++
				}
			}
			if n > 0 {
				found = append(found, name)
				if f.once && n > 1 {
					t.Errorf("%s reads %q in %d places; read it once in its owner predicate", name, f.fact, n)
				}
			}
		}
		sort.Strings(found)
		for _, name := range found {
			if !slices.Contains(f.owners, name) {
				t.Errorf("%s decides %q outside its owner %v; route it through the owner", name, f.fact, f.owners)
			}
		}
		if len(found) == 0 && f.owners != nil {
			t.Errorf("no file in core/aws matches %q for %q; the owner moved, update this guard", f.pattern, f.fact)
		}
	}
}
