package unit

import (
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	demo "github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ═══════════════════════════════════════════════════════════════════════════
// Demo fixture tests — bidirectional Fields↔GetFieldKeys coverage, constants,
// and EC2 name variety.
// ═══════════════════════════════════════════════════════════════════════════

// ---------------------------------------------------------------------------
// TestAllDemoResourcesHaveFieldKeys
// ---------------------------------------------------------------------------

func TestAllDemoResourcesHaveFieldKeys(t *testing.T) {
	// Standard resource types with registered field keys.
	standardTypes := []string{"ec2", "s3", "lambda", "dbi"}

	for _, shortName := range standardTypes {
		t.Run(shortName, func(t *testing.T) {
			resources, ok := demo.GetResources(shortName)
			if !ok {
				t.Fatalf("GetResources(%q) returned ok=false", shortName)
			}
			if len(resources) == 0 {
				t.Fatalf("GetResources(%q) returned empty slice", shortName)
			}

			registeredKeys := resource.GetFieldKeys(shortName)
			if len(registeredKeys) == 0 {
				t.Fatalf("no field keys registered for %q; ensure internal/aws is imported", shortName)
			}

			// Build a set of registered keys for fast lookup.
			keySet := make(map[string]bool, len(registeredKeys))
			for _, k := range registeredKeys {
				keySet[k] = true
			}

			// Every Fields key in every demo resource must be in the registered set.
			for i, r := range resources {
				for fieldKey := range r.Fields {
					if !keySet[fieldKey] {
						t.Errorf("resource[%d] (%s): Fields key %q not in registered field keys for %q: %v",
							i, r.ID, fieldKey, shortName, registeredKeys)
					}
				}
			}

			// Every registered key must be present in the demo resource's Fields.
			for i, r := range resources {
				for _, regKey := range registeredKeys {
					if _, exists := r.Fields[regKey]; !exists {
						t.Errorf("resource[%d] (%s): missing registered field key %q in Fields",
							i, r.ID, regKey)
					}
				}
			}
		})
	}

	// S3 objects are a special case — validate against S3ObjectColumns().
	t.Run("s3-objects", func(t *testing.T) {
		objects, ok := demo.GetS3Objects("data-pipeline-logs", "")
		if !ok {
			t.Fatal("GetS3Objects(\"data-pipeline-logs\", \"\") returned ok=false")
		}
		if len(objects) == 0 {
			t.Fatal("GetS3Objects returned empty slice")
		}

		s3ObjCols := resource.S3ObjectColumns()
		colKeySet := make(map[string]bool, len(s3ObjCols))
		for _, col := range s3ObjCols {
			colKeySet[col.Key] = true
		}

		for i, obj := range objects {
			for fieldKey := range obj.Fields {
				if !colKeySet[fieldKey] {
					t.Errorf("object[%d] (%s): Fields key %q not in S3ObjectColumns: %v",
						i, obj.ID, fieldKey, s3ObjCols)
				}
			}
			for _, col := range s3ObjCols {
				if _, exists := obj.Fields[col.Key]; !exists {
					t.Errorf("object[%d] (%s): missing S3ObjectColumn key %q in Fields",
						i, obj.ID, col.Key)
				}
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestDemoConstants
// ---------------------------------------------------------------------------

func TestDemoConstants(t *testing.T) {
	if demo.DemoProfile != "demo" {
		t.Errorf("DemoProfile: got %q, want %q", demo.DemoProfile, "demo")
	}
	if demo.DemoRegion != "us-east-1" {
		t.Errorf("DemoRegion: got %q, want %q", demo.DemoRegion, "us-east-1")
	}
}

// ---------------------------------------------------------------------------
// TestEC2NameVariety
// ---------------------------------------------------------------------------

func TestEC2NameVariety(t *testing.T) {
	resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("GetResources(\"ec2\") returned ok=false")
	}

	// At least 6 EC2 instances.
	if len(resources) < 6 {
		t.Errorf("expected at least 6 EC2 instances; got %d", len(resources))
	}

	// Collect name prefixes (text before the first '-').
	prefixes := make(map[string]bool)
	for _, r := range resources {
		name := r.Name
		if name == "" {
			continue
		}
		idx := strings.Index(name, "-")
		if idx > 0 {
			prefixes[name[:idx]] = true
		} else {
			prefixes[name] = true
		}
	}

	// At least 2 different name prefixes for realistic naming variety.
	if len(prefixes) < 2 {
		t.Errorf("expected at least 2 different EC2 name prefixes; got %d: %v", len(prefixes), prefixes)
	}

	// Specifically verify "web" prefix exists (needed for demo filter scenario).
	if !prefixes["web"] {
		t.Error("no EC2 instance name starts with \"web\"; required for /web filter in demo scenario")
	}
}
