package unit_test

// related_nil_branch_never_guesses_test.go — a checker that could not read its
// target list must say so, not answer from the source resource.
//
// Every related checker has a branch for "the target list came back nil".
// Nil means one of two things, and neither is data: the cache held nothing, or
// there was nothing to call. Some checkers used that branch to build IDs out of
// what the SOURCE resource happens to know — an alias DNS name, an ARN — and
// returned them as a resolved count. The panel then shows rows for resources
// that were never looked up, and navigating one lands nowhere.
//
// The tell is uniform and cheap to find: a nil branch that appends to an ID
// slice is manufacturing an answer. A branch that returns Unknown, Error, or a
// nil-ID Known result is reporting one.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	nilListBranchRe = regexp.MustCompile(`\bif (\w*[Ll]ist == nil|len\(\w*[Ll]ist\) == 0) \{`)
	// Every helper that builds a RESOLVED result, so a new wrapper cannot hide
	// a guess from this gate the way relatedResult and r53RelatedResult did:
	// the first pattern matched only two of the three spellings in the tree,
	// and the surviving site used the one it missed.
	guessedIDsRe = regexp.MustCompile(`(\w*[Rr]elatedResult\w*|KnownRelated)\([^,]+,\s*(ids|\w+IDs)`)
)

// TestRelatedNilBranch_NeverBuildsIDs scans every related-checker file for a
// nil-list branch that constructs resource IDs.
func TestRelatedNilBranch_NeverBuildsIDs(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/*_related*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no related-checker files found: %v", err)
	}

	var bad []string
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		lines := strings.Split(string(src), "\n")
		for i, line := range lines {
			if !nilListBranchRe.MatchString(line) {
				continue
			}
			depth, body := 1, []string{}
			for j := i + 1; j < len(lines) && depth > 0; j++ {
				depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
				if depth > 0 {
					body = append(body, lines[j])
				}
			}
			block := strings.Join(body, "\n")
			if strings.Contains(block, "append(") || guessedIDsRe.MatchString(block) {
				bad = append(bad, fmt.Sprintf("%s:%d", filepath.Base(path), i+1))
			}
		}
	}
	if len(bad) == 0 {
		return
	}
	sort.Strings(bad)
	t.Errorf("%d nil-list branch(es) build resource IDs instead of reporting that the target could not be read. "+
		"Return UnknownRelated (as acm_related.go:190 does) or a nil-ID result; never an ID derived from the source "+
		"resource, which the panel would offer as a row that resolves to nothing:\n  %s",
		len(bad), strings.Join(bad, "\n  "))
}
