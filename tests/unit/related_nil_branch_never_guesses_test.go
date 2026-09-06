package unit_test

// related_nil_branch_never_guesses_test.go — a checker that could not read its
// target list must say so, not answer from the source resource.
//
// Every related checker has a branch for "the target list came back nil".
// Nil means one of two things, and neither is data: the cache held nothing, or
// there was nothing to call. Some checkers used that branch to build IDs out of
// what the SOURCE resource happens to know — an alias DNS name, an ARN, a
// snapshot's own parent id — and returned them as a resolved count. The panel
// then shows rows for resources that were never looked up, and navigating one
// lands nowhere.
//
// The scan is deliberately wide on both axes, because twice a survivor hid in
// the width the previous version lacked. On the condition axis a nil-list test
// counts wherever it appears, including inside a compound condition, because
// "cold OR truncated, so guess" is the shape that got through. On the body axis
// any construction of a resolved answer counts — appending, assigning a
// non-empty slice literal, or handing a non-nil id argument to any of the
// resolved-result helpers — because the previous version keyed on two of the
// three spellings in the tree and the survivor used the third.

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
	// A nil-list test anywhere in the condition, not only as the whole of it.
	nilListBranchRe = regexp.MustCompile(`\bif [^{]*(\w*[Ll]ist == nil|len\(\w*[Ll]ist\) == 0)[^{]*\{`)
	// A slice literal with something in it. "[]string{}" is an empty answer and
	// is fine; "[]string{name}" is an invented one.
	nonEmptyLiteralRe = regexp.MustCompile(`\[\]string\{\s*[^}\s]`)
	// Every helper that builds a RESOLVED result, so a new wrapper cannot hide a
	// guess the way relatedResult and r53RelatedResult did.
	resolvedCallRe = regexp.MustCompile(`(?:\w*[Rr]elatedResult\w*|KnownRelated)\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)
)

// nilBranchGuesses reports whether a nil-list branch body constructs an answer
// rather than reporting that it has none.
func nilBranchGuesses(block string) bool {
	if strings.Contains(block, "append(") || nonEmptyLiteralRe.MatchString(block) {
		return true
	}
	for _, m := range resolvedCallRe.FindAllStringSubmatch(block, -1) {
		args := splitTopLevelArgs(m[1])
		if len(args) >= 2 && args[1] != "nil" {
			return true
		}
	}
	return false
}

// splitTopLevelArgs splits a call's argument list on commas that are not inside
// nested parentheses, brackets or braces.
func splitTopLevelArgs(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}

// TestRelatedNilBranch_NeverBuildsIDs scans every related-checker file for a
// nil-list branch that constructs resource IDs.
func TestRelatedNilBranch_NeverBuildsIDs(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/*_related*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no related-checker files found: %v", err)
	}

	var bad []string
	scanned := 0
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
			if !nilListBranchRe.MatchString(line) {
				continue
			}
			scanned++
			depth, body := 1, []string{}
			for j := i + 1; j < len(lines) && depth > 0; j++ {
				depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
				if depth > 0 {
					body = append(body, lines[j])
				}
			}
			if nilBranchGuesses(strings.Join(body, "\n")) {
				bad = append(bad, fmt.Sprintf("%s:%d", filepath.Base(path), i+1))
			}
		}
	}
	if scanned == 0 {
		t.Fatal("the scan matched no nil-list branch at all — the pattern has stopped reaching the code")
	}
	if len(bad) == 0 {
		return
	}
	sort.Strings(bad)
	t.Errorf("%d of %d nil-list branch(es) build resource IDs instead of reporting that the target could not be read. "+
		"Return UnknownRelated (as acm_related.go does) or a nil-ID result; never an ID derived from the source "+
		"resource, which the panel would offer as a row that resolves to nothing:\n  %s",
		len(bad), scanned, strings.Join(bad, "\n  "))
}
