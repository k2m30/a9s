// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_apierror_seam_test.go — one place builds a failure.
//
// messages.APIError's own doc says every construction site stamps the fields
// the paired success would have carried, which is what lets a failure be
// routed and discarded by the same rule as its success. A doc comment is not
// an enforcement: the terminal's adapter builds both halves through one
// outcome value, and the runtime's executor hand-writes four more literals
// beside four ResourcesLoaded literals. Four pairs that have to agree, kept in
// agreement by whoever edits them last.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// apiErrorLiteralFiles returns, per repo-relative file, how many
// messages.APIError composite literals it builds.
func apiErrorLiteralFiles(t *testing.T) map[string]int {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	found := map[string]int{}
	fset := token.NewFileSet()
	for _, dir := range []string{"core", "internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return fmt.Errorf("parse %s: %w", path, perr)
			}
			rel, _ := filepath.Rel(root, path)
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				sel, ok := lit.Type.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "APIError" {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "messages" {
					return true
				}
				found[rel]++
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	return found
}

// TestAPIErrorSeam_OnePlaceBuildsAFailure pins the shape. A failure and the
// success it replaces are the two endings of one request, and the fields that
// route them are the same fields; built in two places they are one edit away
// from disagreeing, which is what the doc comment asks for and nothing checks.
func TestAPIErrorSeam_OnePlaceBuildsAFailure(t *testing.T) {
	found := apiErrorLiteralFiles(t)
	if len(found) == 0 {
		t.Fatal("no messages.APIError construction found anywhere; the gate is not reading the tree it thinks it is")
	}
	if len(found) == 1 {
		return
	}
	sites := make([]string, 0, len(found))
	for file, n := range found {
		sites = append(sites, fmt.Sprintf("%s (%d)", file, n))
	}
	sort.Strings(sites)
	t.Errorf("messages.APIError is built in %d files, want 1 — a failure carries the fields that route it, "+
		"and every extra construction is a copy of that list one edit away from disagreeing with the "+
		"success it replaces:\n  %s", len(found), strings.Join(sites, "\n  "))
}

// TestAPIErrorSeam_ARuntimeFailureCarriesThePairedSuccessStamps drives the
// runtime's own four fetch kinds with no clients and reads what each answers
// with. Whatever the seam ends up being, this is what it has to preserve.
func TestAPIErrorSeam_ARuntimeFailureCarriesThePairedSuccessStamps(t *testing.T) {
	_, core := newTestControllerAndCore(t)

	const screen, seq = 41, 7
	cases := []struct {
		name    string
		task    runtime.TaskRequest
		lane    messages.FetchProvenance
		appends bool
		more    bool
	}{
		{
			name: "canonical list",
			task: runtime.TaskRequest{
				Key:     runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "ec2"},
				Payload: runtime.FetchResourcesPayload{Provenance: messages.FetchProvenanceCanonicalList},
			},
			lane: messages.FetchProvenanceCanonicalList,
		},
		{
			name: "filtered drill",
			task: runtime.TaskRequest{
				Key:     runtime.TaskKey{Kind: runtime.KindFetchFiltered, Scope: "ec2"},
				Payload: runtime.FetchFilteredPayload{Filter: map[string]string{"vpc-id": "vpc-0aaaaaaaaaaaaaaaa"}},
			},
			lane: messages.FetchProvenanceFilteredList,
		},
		{
			name: "child list",
			task: runtime.TaskRequest{
				Key:     runtime.TaskKey{Kind: runtime.TaskKindFetchChildResources, Scope: "ec2"},
				Payload: runtime.FetchChildResourcesPayload{ChildType: "ec2", ParentContext: map[string]string{"cluster": "example-cluster"}},
			},
			lane: messages.FetchProvenanceChild,
		},
		{
			name: "load more",
			task: runtime.TaskRequest{
				Key:     runtime.TaskKey{Kind: runtime.KindFetchMore, Scope: "ec2"},
				Payload: runtime.FetchMorePayload{ContinuationToken: "page-2", Provenance: messages.FetchProvenanceCanonicalList},
			},
			lane:    messages.FetchProvenanceCanonicalList,
			appends: true,
			more:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := tc.task
			task.ScreenID = screen
			task.ListSeq = seq
			ev, err := core.ExecuteTask(t.Context(), task)
			if err != nil {
				t.Fatalf("executing %v: %v", task.Key, err)
			}
			apiErr, ok := ev.(messages.APIError)
			if !ok {
				t.Fatalf("a %s fetch with no clients answered with %T, want messages.APIError", tc.name, ev)
			}
			if apiErr.ScreenID != screen {
				t.Errorf("ScreenID = %d, want %d — the failure does not name the screen its success would have", apiErr.ScreenID, screen)
			}
			if apiErr.ListSeq != seq {
				t.Errorf("ListSeq = %d, want %d — the failure cannot be recognised as stale", apiErr.ListSeq, seq)
			}
			if apiErr.Provenance != tc.lane {
				t.Errorf("Provenance = %v, want %v — the failure is offered to a different screen than its success", apiErr.Provenance, tc.lane)
			}
			if apiErr.Append != tc.appends {
				t.Errorf("Append = %v, want %v", apiErr.Append, tc.appends)
			}
			if apiErr.LoadingMore != tc.more {
				t.Errorf("LoadingMore = %v, want %v — the activity flag the failed request raised is not the one it retires", apiErr.LoadingMore, tc.more)
			}
			if apiErr.Err == nil {
				t.Error("the failure carries no error")
			}
		})
	}
}
