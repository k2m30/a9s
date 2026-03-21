package unit

import (
	"reflect"
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/internal/aws"
	demo "github.com/k2m30/a9s/internal/demo"
	"github.com/k2m30/a9s/internal/fieldpath"
	"github.com/k2m30/a9s/internal/resource"
	"gopkg.in/yaml.v3"
)

// ═══════════════════════════════════════════════════════════════════════════
// Demo fixture tests — written BEFORE the internal/demo package exists (TDD).
// These tests verify the demo fixture data quality and must all fail until
// Tasks 2+3 create the internal/demo package.
// ═══════════════════════════════════════════════════════════════════════════

// ---------------------------------------------------------------------------
// 1. TestGetResources_EC2
// ---------------------------------------------------------------------------

func TestGetResources_EC2(t *testing.T) {
	resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("GetResources(\"ec2\") returned ok=false; expected demo EC2 fixtures")
	}
	if len(resources) == 0 {
		t.Fatal("GetResources(\"ec2\") returned empty slice; expected non-empty")
	}

	// Every resource must have non-nil RawStruct (ec2types.Instance).
	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil; EC2 fixtures must populate RawStruct", i, r.ID)
		}
	}

	// Every resource must have Fields matching the registered EC2 field keys.
	fieldKeys := resource.GetFieldKeys("ec2")
	if len(fieldKeys) == 0 {
		t.Fatal("no field keys registered for \"ec2\"; ensure internal/aws is imported")
	}
	for i, r := range resources {
		for _, key := range fieldKeys {
			if _, exists := r.Fields[key]; !exists {
				t.Errorf("resource[%d] (%s): missing Fields key %q", i, r.ID, key)
			}
		}
	}

	// At least one resource must have Status "running" and one "stopped".
	var hasRunning, hasStopped bool
	for _, r := range resources {
		switch r.Status {
		case "running":
			hasRunning = true
		case "stopped":
			hasStopped = true
		}
	}
	if !hasRunning {
		t.Error("no EC2 resource has Status \"running\"; expected at least one")
	}
	if !hasStopped {
		t.Error("no EC2 resource has Status \"stopped\"; expected at least one")
	}

	// Resource IDs should look like AWS instance IDs (i-xxx).
	for i, r := range resources {
		if !strings.HasPrefix(r.ID, "i-") {
			t.Errorf("resource[%d]: ID %q does not look like an EC2 instance ID (expected i-xxx prefix)", i, r.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. TestGetResources_S3
// ---------------------------------------------------------------------------

func TestGetResources_S3(t *testing.T) {
	resources, ok := demo.GetResources("s3")
	if !ok {
		t.Fatal("GetResources(\"s3\") returned ok=false; expected demo S3 fixtures")
	}
	if len(resources) == 0 {
		t.Fatal("GetResources(\"s3\") returned empty slice; expected non-empty")
	}

	// Every S3 resource must have non-nil RawStruct (s3types.Bucket).
	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil; S3 fixtures must populate RawStruct", i, r.ID)
		}
	}

	// Every resource must have Fields with keys "name" and "creation_date".
	for i, r := range resources {
		for _, key := range []string{"name", "creation_date"} {
			if _, exists := r.Fields[key]; !exists {
				t.Errorf("resource[%d] (%s): missing Fields key %q", i, r.ID, key)
			}
		}
	}

	// Should return 5-8 buckets for a realistic demo.
	if len(resources) < 5 || len(resources) > 8 {
		t.Errorf("expected 5-8 S3 buckets; got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// 3. TestGetResources_Lambda
// ---------------------------------------------------------------------------

func TestGetResources_Lambda(t *testing.T) {
	resources, ok := demo.GetResources("lambda")
	if !ok {
		t.Fatal("GetResources(\"lambda\") returned ok=false; expected demo Lambda fixtures")
	}
	if len(resources) == 0 {
		t.Fatal("GetResources(\"lambda\") returned empty slice; expected non-empty")
	}

	// Every resource must have all required Lambda fields including "code_size".
	requiredKeys := []string{"function_name", "runtime", "memory", "timeout", "handler", "last_modified", "code_size"}
	for i, r := range resources {
		for _, key := range requiredKeys {
			if _, exists := r.Fields[key]; !exists {
				t.Errorf("resource[%d] (%s): missing Fields key %q", i, r.ID, key)
			}
		}
	}

	// Should return 5-8 functions for a realistic demo.
	if len(resources) < 5 || len(resources) > 8 {
		t.Errorf("expected 5-8 Lambda functions; got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// 4. TestGetResources_RDS
// ---------------------------------------------------------------------------

func TestGetResources_RDS(t *testing.T) {
	// RDS uses shortname "dbi" in the registry.
	resources, ok := demo.GetResources("dbi")
	if !ok {
		t.Fatal("GetResources(\"dbi\") returned ok=false; expected demo RDS/DBI fixtures")
	}
	if len(resources) == 0 {
		t.Fatal("GetResources(\"dbi\") returned empty slice; expected non-empty")
	}

	// Every resource must have required RDS field keys.
	requiredKeys := []string{"db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az"}
	for i, r := range resources {
		for _, key := range requiredKeys {
			if _, exists := r.Fields[key]; !exists {
				t.Errorf("resource[%d] (%s): missing Fields key %q", i, r.ID, key)
			}
		}
	}

	// At least one "available" and one non-"available" status.
	var hasAvailable, hasNonAvailable bool
	for _, r := range resources {
		if r.Status == "available" {
			hasAvailable = true
		} else {
			hasNonAvailable = true
		}
	}
	if !hasAvailable {
		t.Error("no RDS resource has Status \"available\"; expected at least one")
	}
	if !hasNonAvailable {
		t.Error("all RDS resources have Status \"available\"; expected at least one different status (e.g., \"creating\" or \"stopped\")")
	}

	// Should return 4-8 instances for a realistic demo.
	if len(resources) < 4 || len(resources) > 8 {
		t.Errorf("expected 4-8 RDS instances; got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// 5. TestGetResources_Unknown
// ---------------------------------------------------------------------------

func TestGetResources_Unknown(t *testing.T) {
	resources, ok := demo.GetResources("nonexistent")
	if ok {
		t.Error("GetResources(\"nonexistent\") returned ok=true; expected false")
	}
	if resources != nil {
		t.Errorf("GetResources(\"nonexistent\") returned non-nil slice (%d items); expected nil", len(resources))
	}
}

// ---------------------------------------------------------------------------
// 6. TestGetS3Objects
// ---------------------------------------------------------------------------

func TestGetS3Objects(t *testing.T) {
	// Known bucket should return objects.
	objects, ok := demo.GetS3Objects("data-pipeline-logs", "")
	if !ok {
		t.Fatal("GetS3Objects(\"data-pipeline-logs\", \"\") returned ok=false; expected demo S3 objects")
	}
	if len(objects) == 0 {
		t.Fatal("GetS3Objects(\"data-pipeline-logs\", \"\") returned empty slice; expected non-empty")
	}

	// Verify S3 object field keys match S3ObjectColumns().
	s3ObjCols := resource.S3ObjectColumns()
	expectedKeys := make([]string, len(s3ObjCols))
	for i, col := range s3ObjCols {
		expectedKeys[i] = col.Key
	}
	for i, obj := range objects {
		for _, key := range expectedKeys {
			if _, exists := obj.Fields[key]; !exists {
				t.Errorf("object[%d] (%s): missing Fields key %q", i, obj.ID, key)
			}
		}
	}

	// Unknown bucket should return nil, false.
	objects2, ok2 := demo.GetS3Objects("no-such-bucket", "")
	if ok2 {
		t.Error("GetS3Objects(\"no-such-bucket\", \"\") returned ok=true; expected false")
	}
	if objects2 != nil {
		t.Errorf("GetS3Objects(\"no-such-bucket\", \"\") returned non-nil slice (%d items); expected nil", len(objects2))
	}
}

// ---------------------------------------------------------------------------
// 7. TestEC2RawStructYAML
// ---------------------------------------------------------------------------

func TestEC2RawStructYAML(t *testing.T) {
	resources, ok := demo.GetResources("ec2")
	if !ok {
		t.Fatal("GetResources(\"ec2\") returned ok=false")
	}
	if len(resources) == 0 {
		t.Fatal("GetResources(\"ec2\") returned empty slice")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil; cannot marshal to YAML", i, r.ID)
			continue
		}
		safe := fieldpath.ToSafeValue(reflect.ValueOf(r.RawStruct))
		out, err := yaml.Marshal(safe)
		if err != nil {
			t.Errorf("resource[%d] (%s): yaml.Marshal(RawStruct) failed: %v", i, r.ID, err)
			continue
		}
		if len(out) == 0 {
			t.Errorf("resource[%d] (%s): yaml.Marshal(RawStruct) produced empty output", i, r.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// 8. TestAllDemoResourcesHaveFieldKeys
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
// 9. TestDemoConstants
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
// 10. TestEC2NameVariety
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
