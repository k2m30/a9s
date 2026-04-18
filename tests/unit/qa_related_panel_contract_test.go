package unit

// qa_related_panel_contract_test.go — per-resource-type related-panel
// contract enforced against the GOLDEN DOC.
//
// This file is NOT a PR #273 regression pin. It surfaced accidentally
// during PR #273 review as a huge, fundamental coverage gap in the
// detail-view RELATED panel: the registry had drifted away from the
// AWS API cross-references and DevOps workflows that should drive it.
// The test suite below is the ongoing enforcement layer for the
// contract, independent of any single PR.
//
// The single source of truth for this contract is:
//
//     docs/related-resources.md
//
// That document is produced from AWS API references + DevOps workflows,
// reconciled across six independent blind audits. DO NOT edit it
// ad-hoc — see the policy section at the top of the doc.
//
// This test file parses the golden doc's "Per-type contract" table and
// enforces it against the registry. Drift in either direction is a
// failure:
//
//   (A) TestRelatedPanel_ContractMatchesGoldenDoc
//       For every row in the golden table, every expected TargetType
//       must appear in resource.GetRelated(shortName). Missing
//       registration is a failure, with the doc row cited in the error.
//
//   (B) TestRelatedPanel_RegistrationHasGoldenDocEntry
//       For every TargetType currently registered via RegisterRelated,
//       either (a) it is listed in the golden table for that shortName,
//       or (b) it is a documented self-reference pattern (type->same).
//       Otherwise the registration has drifted and the doc must be
//       updated (with citation) before the registration is accepted.
//
//   (C) TestRelatedPanel_EveryRegisteredTypeHasGoldenRow
//       Every registered resource type must appear in the golden table.
//       Adding a new type to the registry without adding a golden row is
//       a failure — enforces the "new type requires contract row in the
//       same PR" rule.
//
//   (D) TestRelatedPanel_TargetTypesAreRegistered
//       Every TargetType in the golden doc and in every RegisterRelated
//       call must name a real registered shortName (typo guard).
//
// When a test here fails:
//   - If the AWS API or DevOps workflow justifies the registration and
//     it is missing from the doc, update the doc with a citation.
//   - If the registration is wrong or stale, remove it from code.
//   - NEVER "fix" the test by softening the assertion. The test is the
//     enforcement boundary for the golden contract.

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// goldenDocPath locates docs/related-resources.md relative to this test file.
func goldenDocPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed — cannot locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	p := filepath.Join(repoRoot, "docs", "related-resources.md")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("golden doc not found at %s — this test requires docs/related-resources.md to exist", p)
	}
	return p
}

// goldenContract parses the "Per-type contract" table from
// docs/related-resources.md and returns shortName -> set of TargetTypes.
//
// The table rows look like:
//   | `acm` | [API_…](url) | `apigw`, `cf`, `ct-events`, `elb`, `r53` |
//
// Parsing is kept simple: find the section header, skip the table header
// lines, then read rows until a blank line or the next H2.
var (
	goldenOnce sync.Once
	goldenData map[string]map[string]struct{}
	goldenErr  error
)

func loadGoldenContract(t *testing.T) map[string]map[string]struct{} {
	t.Helper()
	goldenOnce.Do(func() {
		raw, err := os.ReadFile(goldenDocPath(t))
		if err != nil {
			goldenErr = err
			return
		}
		goldenData = parseGoldenTable(string(raw))
	})
	if goldenErr != nil {
		t.Fatalf("reading golden doc: %v", goldenErr)
	}
	if len(goldenData) == 0 {
		t.Fatalf("golden doc parse returned zero rows — parser or doc format broken")
	}
	return goldenData
}

// parseGoldenTable extracts the per-type contract table.
// Returns map[shortName]set(targetType).
func parseGoldenTable(doc string) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{})
	lines := strings.Split(doc, "\n")
	inTable := false
	sawHeader := false
	sectionRE := regexp.MustCompile(`^##\s+Per-type contract\s*$`)
	h2RE := regexp.MustCompile(`^##\s+`)
	rowRE := regexp.MustCompile("^\\|\\s*`([a-z0-9-]+)`\\s*\\|[^|]*\\|\\s*(.*?)\\s*\\|\\s*$")
	tickRE := regexp.MustCompile("`([a-z0-9-]+)`")
	for _, ln := range lines {
		if !inTable {
			if sectionRE.MatchString(ln) {
				inTable = true
			}
			continue
		}
		// inside the section — a new H2 ends it.
		if h2RE.MatchString(ln) && !sectionRE.MatchString(ln) {
			break
		}
		if !sawHeader {
			if strings.Contains(ln, "|------|") || strings.HasPrefix(strings.TrimSpace(ln), "|---") {
				sawHeader = true
			}
			continue
		}
		m := rowRE.FindStringSubmatch(ln)
		if m == nil {
			// Allow blank lines / prose after the table ends within the section.
			if strings.TrimSpace(ln) == "" {
				continue
			}
			continue
		}
		sn := m[1]
		targetsCell := m[2]
		set := make(map[string]struct{})
		if strings.TrimSpace(targetsCell) != "" && !strings.Contains(targetsCell, "*(none)*") {
			for _, tm := range tickRE.FindAllStringSubmatch(targetsCell, -1) {
				set[tm[1]] = struct{}{}
			}
		}
		out[sn] = set
	}
	return out
}

// selfRefAllowed enumerates the shortName values whose RegisterRelated
// legitimately includes the same shortName as a target, because AWS
// exposes a same-type relationship on the resource itself:
//
//   - cfn → cfn: nested stacks.
//   - sqs → sqs: DLQ / RedriveTarget.
//   - sg  → sg:  rules referencing other SGs.
var selfRefAllowed = map[string]bool{
	"cfn": true,
	"sqs": true,
	"sg":  true,
}

// TestRelatedPanel_ContractMatchesGoldenDoc asserts every golden-doc
// expected TargetType is registered.
func TestRelatedPanel_ContractMatchesGoldenDoc(t *testing.T) {
	golden := loadGoldenContract(t)
	for sn, expected := range golden {
		t.Run(sn, func(t *testing.T) {
			got := make(map[string]bool, len(resource.GetRelated(sn)))
			for _, r := range resource.GetRelated(sn) {
				got[r.TargetType] = true
			}
			var missing []string
			for tgt := range expected {
				if !got[tgt] {
					missing = append(missing, tgt)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				t.Errorf(
					"%s is missing RegisterRelated entries required by docs/related-resources.md: %v\n\n"+
						"The golden doc is the SINGLE SOURCE OF TRUTH. Either add the registration (preferred) "+
						"or open a PR that removes the row from the golden doc with an AWS-API-anchored rationale.",
					sn, missing,
				)
			}
		})
	}
}

// TestRelatedPanel_RegistrationHasGoldenDocEntry asserts that every
// currently-registered TargetType is covered by the golden doc.
// Self-references listed in selfRefAllowed are exempt.
func TestRelatedPanel_RegistrationHasGoldenDocEntry(t *testing.T) {
	golden := loadGoldenContract(t)
	for _, td := range resource.AllResourceTypes() {
		sn := td.ShortName
		t.Run(sn, func(t *testing.T) {
			expected := golden[sn] // may be nil if type missing from doc — caught by another test
			var drift []string
			for _, r := range resource.GetRelated(sn) {
				tgt := r.TargetType
				if tgt == sn && selfRefAllowed[sn] {
					continue
				}
				if _, ok := expected[tgt]; !ok {
					drift = append(drift, tgt)
				}
			}
			sort.Strings(drift)
			if len(drift) > 0 {
				t.Errorf(
					"%s has RegisterRelated entries NOT present in docs/related-resources.md: %v\n\n"+
						"Either (a) remove the registration if it is stale, or (b) add a row to the golden "+
						"doc citing the AWS API field or DevOps workflow that justifies it, then re-run tests.",
					sn, drift,
				)
			}
		})
	}
}

// TestRelatedPanel_EveryRegisteredTypeHasGoldenRow asserts every
// registered type appears as a golden-doc row. Prevents "add type to
// registry without adding doc row" drift.
func TestRelatedPanel_EveryRegisteredTypeHasGoldenRow(t *testing.T) {
	golden := loadGoldenContract(t)
	var missing []string
	for _, td := range resource.AllResourceTypes() {
		if _, ok := golden[td.ShortName]; !ok {
			missing = append(missing, td.ShortName)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf(
			"registered resource types without a row in docs/related-resources.md: %v\n\n"+
				"Adding a new resource type REQUIRES adding a contract row in the same PR. "+
				"The golden doc's policy section states this explicitly.",
			missing,
		)
	}
}

// TestRelatedPanel_TargetTypesAreRegistered asserts every TargetType
// referenced by the golden doc OR by RegisterRelated calls is a real
// registered shortName (typo guard).
func TestRelatedPanel_TargetTypesAreRegistered(t *testing.T) {
	registered := make(map[string]bool, len(resource.AllResourceTypes()))
	for _, td := range resource.AllResourceTypes() {
		registered[td.ShortName] = true
	}

	t.Run("golden_doc", func(t *testing.T) {
		golden := loadGoldenContract(t)
		var bad []string
		for sn, targets := range golden {
			for tgt := range targets {
				if !registered[tgt] {
					bad = append(bad, sn+"->"+tgt)
				}
			}
		}
		sort.Strings(bad)
		if len(bad) > 0 {
			t.Errorf("golden doc references unregistered shortNames (typos): %v", bad)
		}
	})

	t.Run("registrations", func(t *testing.T) {
		var bad []string
		for _, td := range resource.AllResourceTypes() {
			for _, r := range resource.GetRelated(td.ShortName) {
				if !registered[r.TargetType] {
					bad = append(bad, td.ShortName+"->"+r.TargetType)
				}
			}
		}
		sort.Strings(bad)
		if len(bad) > 0 {
			t.Errorf("RegisterRelated calls reference unregistered shortNames (typos): %v", bad)
		}
	})
}

// parseExcludedPairs parses the "## Explicitly excluded" section from
// docs/related-resources.md and returns a map[parent]map[target]struct{}.
//
// Bullet format (within the section):
//
//	- `<parent>` → `<target>` — <rationale>
//
// The three sub-sections ("Unanimous `no`", "Unanimous `sometimes`", "Majority `no`")
// are parsed transparently — only the bullet pattern matters.
func parseExcludedPairs(doc string) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{})
	lines := strings.Split(doc, "\n")
	inSection := false
	sectionRE := regexp.MustCompile(`^##\s+Explicitly excluded\s*$`)
	h2RE := regexp.MustCompile(`^##\s+`)
	// matches: - `<parent>` → `<target>` —
	bulletRE := regexp.MustCompile("^-\\s+`([a-z0-9-]+)`\\s+→\\s+`([a-z0-9-]+)`\\s+—")
	for _, ln := range lines {
		if !inSection {
			if sectionRE.MatchString(ln) {
				inSection = true
			}
			continue
		}
		// A new H2 (other than ourselves) ends the section.
		if h2RE.MatchString(ln) && !sectionRE.MatchString(ln) {
			break
		}
		m := bulletRE.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		parent, target := m[1], m[2]
		if out[parent] == nil {
			out[parent] = make(map[string]struct{})
		}
		out[parent][target] = struct{}{}
	}
	return out
}

// TestRelatedPanel_NoExcludedPairsRegistered is the T109 regression guard.
//
// It parses the "Explicitly excluded" section of docs/related-resources.md and
// asserts that none of the 57 listed parent→target pairs appear in any
// RegisterRelated call. Re-adding an excluded pair without removing it from the
// doc first will cause this test to fail with a clear message.
//
// The total count of excluded pairs is also asserted so that accidental
// deletions from the doc are caught.
func TestRelatedPanel_NoExcludedPairsRegistered(t *testing.T) {
	raw, err := os.ReadFile(goldenDocPath(t))
	if err != nil {
		t.Fatalf("reading golden doc: %v", err)
	}
	excluded := parseExcludedPairs(string(raw))

	// Count the total excluded entries and verify the doc still has all 58.
	total := 0
	for _, targets := range excluded {
		total += len(targets)
	}
	const wantTotal = 58
	if total != wantTotal {
		t.Errorf("Explicitly excluded section has %d entries, want %d — was a pair accidentally added or removed from docs/related-resources.md?", total, wantTotal)
	}

	// For each excluded (parent, target) pair, assert no registration exists.
	for parent, targets := range excluded {
		defs := resource.GetRelated(parent)
		for _, def := range defs {
			if _, excluded := targets[def.TargetType]; excluded {
				t.Errorf(
					"parent %q has registration for %q but that pair is in the Explicitly excluded section — "+
						"remove the registration or remove the pair from the Explicitly excluded section of "+
						"docs/related-resources.md (with AWS-API evidence citation per the Policy section)",
					parent, def.TargetType,
				)
			}
		}
	}
}
