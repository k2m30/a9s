package unit_test

// related_truncated_list_is_a_lower_bound_test.go — a list that WAS read and
// came back cut short has one answer in this package, and it is not Unknown.
//
// The rule has three cases. Nothing fetched (a nil list) is Unknown, because a
// count from a list nobody read is a guess. A list that was read and cut short
// is a resolved lower bound carrying the truncation flag, rendered "(0+)":
// "none among the pages read, more unread" is true, and the flag is where the
// domain type keeps the uncertainty. A failed call is an Error.
//
// The second case is the one that keeps coming back. A checker that answers
// Unknown there throws away a real zero and renders a question mark the user
// cannot act on, and two hand scans over this package missed the last site
// twice — once because the grep keyed on one spelling of the condition. So the
// shape is pinned rather than the sites: every Unknown returned under a
// condition that mentions truncation must be on the list below, and every entry
// on that list has to say why its truncated list is not the target list.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// truncatedUnknownAllowed maps "file:line" of an Unknown return under a
// truncation test to the reason it is case 1 rather than case 2. In every one
// of these the truncated list is a JOIN LIST — a lookup that produces the key
// for the real target — and the target list has not been read at that point, so
// there are no pages to be a lower bound over.
var truncatedUnknownAllowed = map[string]string{
	"dbc_snap_related.go:140":       "dbc is the join list; the backup target is unread until the parent ARN resolves",
	"dbi_snap_related.go:135":       "dbi is the join list; the backup target is unread until the parent ARN resolves",
	"ecs_svc_related.go:194":        "tg is the join list; the elb target is unread until the load balancer ARNs resolve",
	"ecs_task_related_extra.go:298": "eni is the join list; the sg target is unread until the group ids resolve",
	"eip_related.go:200":            "ec2 is the join list; asg is never fetched, the name comes off an instance tag",
	"msk_related.go:138":            "subnet is the join list; vpc is never fetched, the id comes off a subnet struct",
}

var (
	unknownReturnRe  = regexp.MustCompile(`UnknownRelated\(`)
	ifHeaderRe       = regexp.MustCompile(`^\s*(\}\s*else\s+)?if\b`)
	truncationWordRe = regexp.MustCompile(`(?i)truncat`)
)

// TestRelatedTruncatedList_IsALowerBoundNotUnknown fails on any Unknown return
// whose enclosing condition mentions truncation and which is not allowlisted.
func TestRelatedTruncatedList_IsALowerBoundNotUnknown(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/*_related*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no related-checker files found: %v", err)
	}

	found := map[string]bool{}
	var offenders []string
	unknowns := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		lines := strings.Split(string(src), "\n")
		for i, line := range lines {
			if !unknownReturnRe.MatchString(line) {
				continue
			}
			unknowns++
			// Walk back to the enclosing condition. Eight lines covers every
			// guard in this package, including the multi-line ones.
			for j := i; j >= 0 && j > i-8; j-- {
				if !ifHeaderRe.MatchString(lines[j]) {
					continue
				}
				if truncationWordRe.MatchString(strings.Join(lines[j:i+1], "\n")) {
					// The site is the CONDITION, not the return: that is the
					// line the exemption reasons about, and one guard may hold
					// more than one Unknown.
					site := fmt.Sprintf("%s:%d", filepath.Base(path), j+1)
					found[site] = true
					if _, ok := truncatedUnknownAllowed[site]; !ok {
						offenders = append(offenders, site)
					}
				}
				break
			}
		}
	}

	if unknowns == 0 {
		t.Fatal("the scan found no UnknownRelated call at all — the pattern has stopped reaching the code")
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d checker(s) answer Unknown for a list that WAS read and came back truncated. "+
			"That throws away a real zero: return the resolved lower bound through relatedResultTrunc so the row "+
			"renders \"(0+)\". If the truncated list is a join list and the target was never fetched, it is case 1 "+
			"— add it to truncatedUnknownAllowed with the reason:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}

	var stale []string
	for site := range truncatedUnknownAllowed {
		if !found[site] {
			stale = append(stale, site)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("%d allowlist entr(ies) no longer name an Unknown return under a truncation test. "+
			"A line that moved takes its exemption with it, so the entry is stale and the next survivor would "+
			"inherit it; re-derive the line numbers or delete these:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
}

// TestRelated_EFS_ENI_ColdCacheIsUnknown pins row 16 case 1 where the nil list
// IS the target list. The EFS mount-target pivot scans the eni list for
// interfaces whose description carries the file system id; with no eni cache
// and nothing to call, that scan never happened. Reporting "(0)" says the file
// system has no mount targets, which is a count about a list nobody read —
// exactly what related_fetch.go tells callers not to do.
func TestRelated_EFS_ENI_ColdCacheIsUnknown(t *testing.T) {
	fs := resource.Resource{ID: "fs-0a1b2c3d4e5f67890", Name: "acme-shared-efs"}

	result := checkerByTarget(t, "efs", "eni")(context.Background(), nil, fs, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("cold cache: state = %v (Count=%d), want Unknown — nothing read the eni list, so \"no mount targets\" is a guess",
			result.State(), result.Count())
	}
}
