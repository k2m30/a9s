package unit_test

// related_nil_branch_never_guesses_test.go — a checker that could not read its
// target list must say so, not answer from the source resource.
//
// Every related checker has a branch for "the list came back nil". Nil from a
// fetch helper means one of two things, and neither is data: the cache held
// nothing, or there was nothing to call. Checkers used that branch two ways,
// both of them answers the code had no basis for. Some built IDs out of what
// the SOURCE resource happens to know — an alias DNS name, an ARN, a snapshot's
// own parent id — and returned them as a count. Others returned a zero, plain
// or carrying the truncation flag, which reads as "there are none" or "none on
// the pages read" about a list with no pages read at all.
//
// The scan is wide on the condition axis and on what counts as an answer,
// because three times a survivor hid in width this gate lacked. A nil-list test
// counts wherever it appears in the condition, including inside a compound one.
// Any construction of a resolved result counts — appending, a non-empty slice
// literal, or any of the resolved-result helpers, with nil ids as much as with
// ids.
//
// What keeps that width honest is the predicate on the VARIABLE. Only a nil
// that came out of a fetch helper — the ones returning (list, truncation flag,
// error) — means "nobody read it". A nil check on a struct field is a real
// absence: a Lambda function with no VpcConfig is genuinely in no VPC, and a
// zero there is the truth. Widening past the predicate flags dozens of those.

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
	// \b before the identifier matters: without it the greedy prefix eats into
	// the name and the capture is its last letter.
	nilListBranchRe = regexp.MustCompile(`\bif [^{]*(\b(\w+) == nil|len\((\w+)\) == 0)[^{]*\{`)
	// The four helpers that read a target list. A nil from any of them is
	// "nobody read it"; a nil from anything else is a fact about the source.
	fetchAssignRe = regexp.MustCompile(`^\s*(\w+)\s*,\s*\w+\s*,\s*\w+\s*(?::=|=)\s*(?:\w+\.)?(?:relatedResourcesFor|FetchRelatedTarget|cachedTypedRows|\w*RelatedResources\w*)\s*\(`)
	funcStartRe   = regexp.MustCompile(`^func `)
	// The id-building rule predates the fetch-helper predicate and keeps its own
	// narrower reach: a variable named like a list. Letting it loose on every
	// nil check in the package flags "ids" and "c" and thirty-odd honest branches.
	listNameRe = regexp.MustCompile(`^\w*[Ll]ist$`)
	// A slice literal with something in it. "[]string{}" is an empty answer and
	// is fine; "[]string{name}" is an invented one.
	nonEmptyLiteralRe = regexp.MustCompile(`\[\]string\{\s*[^}\s]`)
	// Every helper that builds a RESOLVED result, so a new wrapper cannot hide a
	// guess the way relatedResult and r53RelatedResult did.
	resolvedCallRe = regexp.MustCompile(`(?:\w*[Rr]elatedResult\w*|KnownRelated)\(`)
	resolvedArgsRe = regexp.MustCompile(`(?:\w*[Rr]elatedResult\w*|KnownRelated)\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)
)

// buildsIDs reports whether a branch body manufactures identifiers.
func buildsIDs(block string) bool {
	if strings.Contains(block, "append(") || nonEmptyLiteralRe.MatchString(block) {
		return true
	}
	for _, m := range resolvedArgsRe.FindAllStringSubmatch(block, -1) {
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

// TestRelatedNilBranch_NeverBuildsIDs scans the package for a nil-list branch
// that answers instead of reporting that it has nothing to answer from.
func TestRelatedNilBranch_NeverBuildsIDs(t *testing.T) {
	files, err := filepath.Glob("../../core/aws/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no source files found: %v", err)
	}

	var bad []string
	branches, fetched := 0, 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		lines := strings.Split(string(src), "\n")
		// Variables holding a list a fetch helper returned, reset per function.
		fromFetch := map[string]bool{}
		for i, line := range lines {
			if funcStartRe.MatchString(line) {
				fromFetch = map[string]bool{}
			}
			if m := fetchAssignRe.FindStringSubmatch(line); m != nil {
				fromFetch[m[1]] = true
				continue
			}
			m := nilListBranchRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			branches++
			readNothing := fromFetch[m[2]] || fromFetch[m[3]]
			if readNothing {
				fetched++
			}
			depth, body := 1, []string{}
			for j := i + 1; j < len(lines) && depth > 0; j++ {
				depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
				if depth > 0 {
					body = append(body, lines[j])
				}
			}
			block := strings.Join(body, "\n")
			// A list nobody read may not produce ANY resolved answer, a zero
			// included. A branch on anything else list-shaped is still held to
			// the older rule: it may report nothing, but it may not invent ids.
			listName := listNameRe.MatchString(m[2]) || listNameRe.MatchString(m[3])
			if (readNothing && resolvedCallRe.MatchString(block)) || (listName && buildsIDs(block)) {
				bad = append(bad, fmt.Sprintf("%s:%d", filepath.Base(path), i+1))
			}
		}
	}
	if branches == 0 || fetched == 0 {
		t.Fatalf("the scan reached %d nil branch(es), %d of them on a fetched list — the patterns have stopped reaching the code",
			branches, fetched)
	}
	if len(bad) == 0 {
		return
	}
	sort.Strings(bad)
	t.Errorf("%d of %d nil branch(es) on a list a fetch helper returned answer from something other than that list "+
		"(%d nil branches scanned in total). A nil list was never read: return UnknownRelated. Not a count, not an id "+
		"from the source resource, and not a zero — plain or truncation-flagged — because \"none\" and \"none so far\" "+
		"are both claims about pages nobody fetched:\n  %s",
		len(bad), fetched, branches, strings.Join(bad, "\n  "))
}
