// Command checklist computes the expected root-menu + list-view checklist for
// each resource type, purely from the snapshot (cmd/snapshot's snapshot.json)
// plus the rules transcribed from docs/resources/<type>.md.
//
// It never reads a9s UI/enrichment code and never touches AWS — it is an
// independent second implementation of "what should show", so a Playwright test
// comparing the real rendered web UI against checklists/<type>.json is a genuine
// check, not a tautology.
//
//	go run ./cmd/checklist --snapshot tests/e2e/testdata/snapshot/<profile>--<region>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Framework contract constants (docs/architecture.md). The checklist generator models these
// independently; if the framework changes a value that is a deliberate contract
// change and this constant must be updated in lockstep.
const (
	pageSize      = 50 // resource.DefaultPageSize — list shows the first page
	enrichmentCap = 50 // aws.EnrichmentCap — Wave-2 issues computed over first N
)

// Checklist is the per-resource expected screen state the Playwright spec asserts against.
type Checklist struct {
	Type string        `json:"type"`
	Menu ChecklistMenu `json:"menu"`
	List ChecklistList `json:"list"`
}

type ChecklistMenu struct {
	Display         string `json:"display"`
	Availability    int    `json:"availability"`     // shown lower-bound count
	AvailTruncated  bool   `json:"avail_truncated"`  // renders "(N+)" vs "(N)"
	Issues          int    `json:"issues"`           // issue-badge count
	IssuesTruncated bool   `json:"issues_truncated"` // renders "! N+"
}

type ChecklistList struct {
	Columns    []string       `json:"columns"`    // exact ordered column headers (impl-plan U10)
	Jargon     []string       `json:"jargon"`     // header strings that must NEVER appear (impl-plan U10)
	Shown      int            `json:"shown"`      // rows on the first page
	Truncated  bool           `json:"truncated"`  // "Results truncated" indicator
	Colors     map[string]int `json:"colors"`     // row-<color> class counts
	Decorators map[string]int `json:"decorators"` // dec-error("!") / dec-warn("~") counts
	Flagged    []FlaggedRow   `json:"flagged"`    // rows carrying a glyph, by name
}

// FlaggedRow names a row expected to carry a glyph + status, matched in the DOM
// by its identity cell text (there is no per-row id attribute).
type FlaggedRow struct {
	Name      string `json:"name"`
	Decorator string `json:"decorator"` // "!" | "~"
	Status    string `json:"status"`
}

func main() {
	var (
		dir   = flag.String("snapshot", "", "snapshot dir (containing snapshot.json; checklists/ is written next to it)")
		types = flag.String("types", "s3", "comma-separated resource short names")
	)
	flag.Parse()
	if *dir == "" {
		log.Fatal("checklist: --snapshot is required")
	}
	expDir := filepath.Join(*dir, "checklists")
	if err := os.MkdirAll(expDir, 0o750); err != nil {
		log.Fatalf("checklist: %v", err)
	}
	// MkdirAll is a no-op on an existing directory — enforce the mode on dirs
	// created by older binaries too.
	// 0700, not 0600: a directory without the owner execute bit cannot be
	// traversed at all — G302's file-oriented threshold doesn't fit dirs.
	if err := os.Chmod(expDir, 0o700); err != nil { //nolint:gosec // G302: dirs need +x
		log.Printf("checklist: warning: cannot tighten permissions on %s: %v", expDir, err)
	}

	// The collector writes ONE big file with a section per type; each per-type
	// checklist generator gets its raw section.
	raw, err := os.ReadFile(filepath.Join(*dir, "snapshot.json"))
	if err != nil {
		log.Fatalf("checklist: reading snapshot.json: %v", err)
	}
	var snap struct {
		Types map[string]json.RawMessage `json:"types"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		log.Fatalf("checklist: parsing snapshot.json: %v", err)
	}

	for t := range strings.SplitSeq(*types, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		section, ok := snap.Types[t]
		if !ok {
			log.Fatalf("checklist: snapshot.json has no %q section — run cmd/snapshot first", t)
		}
		var exp Checklist
		var err error
		switch t {
		case "s3":
			exp, err = checklistS3(section)
		default:
			log.Fatalf("checklist: type %q not yet supported", t)
		}
		if err != nil {
			log.Fatalf("checklist %s: %v", t, err)
		}
		if err := writeJSON(filepath.Join(expDir, t+".json"), exp); err != nil {
			log.Fatalf("checklist %s: %v", t, err)
		}
		fmt.Printf("checklist: wrote checklists/%s.json (menu issues=%d, list flagged=%d)\n", t, exp.Menu.Issues, len(exp.List.Flagged))
	}
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	// WriteFile applies the mode only on creation — chmod pre-existing files
	// written by older binaries down to owner-only.
	return os.Chmod(path, 0o600)
}
