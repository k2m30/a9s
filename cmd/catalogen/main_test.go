package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
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
