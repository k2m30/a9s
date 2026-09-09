// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_save_lanes_test.go pins that the two lanes writing one type file
// — the save that carries the rows and the counts-only availability save —
// answer the same way about the one fact they both record. Nothing orders
// them, which is survivable only while they agree.
package unit_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// wipfixExactFlag reads the type file's exact flag.
func wipfixExactFlag(t *testing.T, body string) bool {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "exact:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "exact:")) == "true"
		}
	}
	return false
}

// wipfixWatchTypeFile samples a type file until it stops changing and returns
// every distinct version it saw, oldest first. Both lanes write off their
// caller's goroutine, so the versions are what the operator's next restart
// could actually read.
func wipfixWatchTypeFile(t *testing.T, cfgFolder, shortName string) []string {
	t.Helper()
	pattern := filepath.Join(cfgFolder, "cache", "*", shortName+".yaml")
	deadline := time.Now().Add(5 * time.Second)
	var seen []string
	stableSince := time.Time{}
	for {
		cur := ""
		if matches, err := filepath.Glob(pattern); err == nil && len(matches) == 1 {
			if b, readErr := os.ReadFile(matches[0]); readErr == nil {
				cur = string(b)
			}
		}
		switch {
		case cur == "":
			stableSince = time.Time{}
		case len(seen) == 0 || cur != seen[len(seen)-1]:
			seen = append(seen, cur)
			stableSince = time.Now()
		case time.Since(stableSince) > 300*time.Millisecond:
			return seen
		}
		if time.Now().After(deadline) {
			if len(seen) == 0 {
				t.Fatalf("no %s type file under %s within 5s", shortName, pattern)
			}
			return seen
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestTruncatedRefetchThenCountsSave_AgreeOnExactness pins the disagreement
// row 35's identity half uncovered. Two lanes write one type file — the save
// that carries the rows and the counts-only availability save — and nothing
// orders them. That is survivable only while both answer the same way about
// the same fact, and after a refetch that came back truncated they do not:
// one preserves the stored exactness, the other self-heals it away, so the
// file says whichever landed last. Every version of the file a restart could
// read has to give the same answer and keep the rows.
func TestTruncatedRefetchThenCountsSave_AgreeOnExactness(t *testing.T) {
	c := newTestController(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	// A complete fetch first: the file records a confirmed population.
	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(55),
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	if versions := wipfixWatchTypeFile(t, cfg, "ec2"); !wipfixExactFlag(t, versions[len(versions)-1]) {
		t.Fatalf("precondition: a complete fetch did not leave exact: true\n%s",
			wipfixTypeFileHead(versions[len(versions)-1]))
	}

	// The refetch comes back truncated: both lanes now have something to say
	// about the same file, and the counts-only one has no rows to say it with.
	_, _ = handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(50),
		Pagination:   &domain.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	versions := wipfixWatchTypeFile(t, cfg, "ec2")
	first := wipfixExactFlag(t, versions[0])
	for i, v := range versions {
		if got := wipfixExactFlag(t, v); got != first {
			t.Errorf("version %d of the ec2 type file says exact: %v where version 0 said %v — "+
				"the two save lanes disagree about the one truncation fact the row store holds, "+
				"so the file says whatever landed last\n%s", i, got, first, wipfixTypeFileHead(v))
		}
		if !strings.Contains(v, "rows:") {
			t.Errorf("version %d of the ec2 type file carries no rows (%d bytes) — a counts-only "+
				"save must keep the rows the other lane wrote, whichever order they land in\n%s",
				i, len(v), wipfixTypeFileHead(v))
		}
	}
}

// wipfixTypeFileHead returns the scalar header of a type file, the part every
// assertion above is about.
func wipfixTypeFileHead(body string) string {
	var head []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "rows:") {
			head = append(head, "rows: <"+strconv.Itoa(strings.Count(body, "\n    - id:"))+" rows>")
			break
		}
		head = append(head, line)
	}
	return strings.Join(head, "\n")
}

// wipfixSaveLaneRows returns n ec2-shaped rows.
func wipfixSaveLaneRows(n int) []resource.Resource {
	out := make([]resource.Resource, n)
	for i := range out {
		id := "i-" + strconv.Itoa(1000000000000000+i)
		out[i] = resource.Resource{
			ID:     id,
			Name:   "example-instance-" + strconv.Itoa(i),
			Fields: map[string]string{"instance_id": id, "state": "running"},
		}
	}
	return out
}
