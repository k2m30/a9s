package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

// generateAndReadDoc runs generateResourceDoc against a temp repo root and
// returns the produced docs/resources/<short>.md content.
func generateAndReadDoc(t *testing.T, rt catalog.ResourceTypeDef) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "resources"), 0o755); err != nil {
		t.Fatalf("mkdir docs/resources: %v", err)
	}
	if err := generateResourceDoc(root, rt); err != nil {
		t.Fatalf("generateResourceDoc: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "docs", "resources", rt.ShortName+".md"))
	if err != nil {
		t.Fatalf("read generated doc: %v", err)
	}
	return string(raw)
}

// TestDetailCell_DeclaredSentenceOrEmDash pins task w27 row 2 directly
// through the generator's own function, not by writing and re-reading a
// page: a finding with a declared Detail renders its escaped sentence, and
// one with none renders the em dash.
func TestDetailCell_DeclaredSentenceOrEmDash(t *testing.T) {
	cases := []struct {
		name string
		def  catalog.FindingDef
		want string
	}{
		{
			name: "no Detail declared",
			def:  catalog.FindingDef{Code: "sg.ingress.wide-open", Phrase: "all ports open to 0.0.0.0/0"},
			want: "—",
		},
		{
			name: "a declared sentence",
			def: catalog.FindingDef{
				Code:   "sg.ingress.dangerous-ports",
				Detail: "An administrative or database port on this group accepts connections from any address on the internet.",
			},
			want: "An administrative or database port on this group accepts connections from any address on the internet.",
		},
		{
			// Inverted with hubs row 4 finding 2: a Detail sentence is authored
			// markdown, so its backticks are code spans it means to render and
			// escaping them showed the reader a backslash. Only the pipe, which
			// would end the table cell, is escaped. Do not restore the escaped
			// form — TestAttentionSignals_CodeSpansAreNotEscaped fails on it.
			name: "a declared sentence carrying markdown",
			def: catalog.FindingDef{
				Code:   "test.escape",
				Detail: "Uses `backticks`, pipes | and _underscores_.",
			},
			want: "Uses `backticks`, pipes \\| and _underscores_.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detailCell(tc.def)
			if got != tc.want {
				t.Errorf("detailCell() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDetailCell_ThroughGenerateResourceDoc pins the same contract one layer
// up: the §4 findings table's Detail column, as generateResourceDoc actually
// writes it, carries the declared sentence or the em dash — the acceptance
// column's "298 cells are regenerated, not edited" requires this join to
// hold end to end, not just at detailCell in isolation.
func TestDetailCell_ThroughGenerateResourceDoc(t *testing.T) {
	rt := catalog.ResourceTypeDef{
		Name:      "Security Groups",
		ShortName: "sg",
		Category:  "NETWORKING",
		FieldKeys: []string{"name", "state"},
		Findings: []catalog.FindingDef{
			{Code: "sg.ingress.dangerous-ports", Phrase: "ports 22 open to 0.0.0.0/0", Severity: domain.SevBroken, Source: "wave1",
				Detail: "An administrative or database port on this group accepts connections from any address on the internet."},
			{Code: "sg.ingress.wide-open", Phrase: "all ports open to 0.0.0.0/0", Severity: domain.SevBroken, Source: "wave1"},
		},
	}
	doc := generateAndReadDoc(t, rt)
	if !strings.Contains(doc, "An administrative or database port on this group accepts connections from any address on the internet.") {
		t.Errorf("generated doc is missing the declared Detail sentence:\n%s", doc)
	}
	if !strings.Contains(doc, "| all ports open to 0.0.0.0/0 | broken | wave1 | — |") {
		t.Errorf("generated doc's Detail-less row does not render the em dash:\n%s", doc)
	}
}

func TestGenerateResourceDocLifecycleHeader(t *testing.T) {
	cases := []struct {
		name       string
		rt         catalog.ResourceTypeDef
		wantHeader string
	}{
		{
			name: "wave1 fields carry no lifecycle key",
			rt: catalog.ResourceTypeDef{
				Name:      "S3 Buckets",
				ShortName: "s3",
				Category:  "STORAGE",
				FieldKeys: []string{"name", "bucket_name", "creation_date", "notification_targets"},
			},
			wantHeader: "s3 — STORAGE. Lifecycle key: none (the list API returns no lifecycle field).",
		},
		{
			name: "default state key present in field keys",
			rt: catalog.ResourceTypeDef{
				Name:      "EC2 Instances",
				ShortName: "ec2",
				Category:  "COMPUTE",
				FieldKeys: []string{"name", "state", "instance_type"},
			},
			wantHeader: "ec2 — COMPUTE. Lifecycle key: `state`.",
		},
		{
			name: "explicit lifecycle key present in field keys",
			rt: catalog.ResourceTypeDef{
				Name:         "DB Instances",
				ShortName:    "dbi",
				Category:     "DATABASE",
				LifecycleKey: "status",
				FieldKeys:    []string{"name", "status", "engine"},
			},
			wantHeader: "dbi — DATABASE. Lifecycle key: `status`.",
		},
		{
			name: "explicit lifecycle key absent from non-empty field keys",
			rt: catalog.ResourceTypeDef{
				Name:         "Widgets",
				ShortName:    "wdg",
				Category:     "MISC",
				LifecycleKey: "status",
				FieldKeys:    []string{"name", "size"},
			},
			wantHeader: "wdg — MISC. Lifecycle key: none (the list API returns no lifecycle field).",
		},
		{
			name: "empty field keys keep the state default",
			rt: catalog.ResourceTypeDef{
				Name:      "Gadgets",
				ShortName: "gdt",
				Category:  "MISC",
			},
			wantHeader: "gdt — MISC. Lifecycle key: `state`.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := generateAndReadDoc(t, tc.rt)
			if !strings.Contains(doc, tc.wantHeader) {
				t.Errorf("generated doc header mismatch\nwant fragment: %q\ngot doc:\n%s", tc.wantHeader, doc)
			}
			if strings.Contains(tc.wantHeader, "none") && strings.Contains(doc, "Lifecycle key: `") {
				t.Errorf("doc names a lifecycle key despite none being expected:\n%s", doc)
			}
		})
	}
}
