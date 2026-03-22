package unit

import (
	"reflect"
	"testing"

	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	demo "github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// TestDemoS3_RawStruct: every S3 bucket fixture must have RawStruct (s3types.Bucket)
// ---------------------------------------------------------------------------

func TestDemoS3_RawStruct(t *testing.T) {
	resources, ok := demo.GetResources("s3")
	if !ok {
		t.Fatal("GetResources(\"s3\") returned ok=false")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil; S3 fixtures must populate RawStruct", i, r.ID)
			continue
		}

		bucket, ok := r.RawStruct.(s3types.Bucket)
		if !ok {
			t.Errorf("resource[%d] (%s): RawStruct is %T, want s3types.Bucket", i, r.ID, r.RawStruct)
			continue
		}

		// Name must match resource ID
		if bucket.Name == nil || *bucket.Name != r.ID {
			t.Errorf("resource[%d] (%s): Bucket.Name = %v, want %q", i, r.ID, bucket.Name, r.ID)
		}

		// CreationDate must be set
		if bucket.CreationDate == nil {
			t.Errorf("resource[%d] (%s): Bucket.CreationDate is nil", i, r.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// TestDemoS3_RawStruct_YAML: S3 bucket RawStruct must marshal to non-empty YAML
// ---------------------------------------------------------------------------

func TestDemoS3_RawStruct_YAML(t *testing.T) {
	resources, ok := demo.GetResources("s3")
	if !ok {
		t.Fatal("GetResources(\"s3\") returned ok=false")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil", i, r.ID)
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
// TestDemoLambda_RawStruct: every Lambda fixture must have RawStruct (FunctionConfiguration)
// ---------------------------------------------------------------------------

func TestDemoLambda_RawStruct(t *testing.T) {
	resources, ok := demo.GetResources("lambda")
	if !ok {
		t.Fatal("GetResources(\"lambda\") returned ok=false")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil; Lambda fixtures must populate RawStruct", i, r.ID)
			continue
		}

		fn, ok := r.RawStruct.(lambdatypes.FunctionConfiguration)
		if !ok {
			t.Errorf("resource[%d] (%s): RawStruct is %T, want lambdatypes.FunctionConfiguration", i, r.ID, r.RawStruct)
			continue
		}

		// FunctionName must match resource ID
		if fn.FunctionName == nil || *fn.FunctionName != r.ID {
			t.Errorf("resource[%d] (%s): FunctionName = %v, want %q", i, r.ID, fn.FunctionName, r.ID)
		}

		// Runtime must be set
		if fn.Runtime == "" {
			t.Errorf("resource[%d] (%s): Runtime is empty", i, r.ID)
		}

		// MemorySize must be set
		if fn.MemorySize == nil {
			t.Errorf("resource[%d] (%s): MemorySize is nil", i, r.ID)
		}

		// Timeout must be set
		if fn.Timeout == nil {
			t.Errorf("resource[%d] (%s): Timeout is nil", i, r.ID)
		}

		// LastModified must be set
		if fn.LastModified == nil {
			t.Errorf("resource[%d] (%s): LastModified is nil", i, r.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// TestDemoLambda_RawStruct_YAML: Lambda RawStruct must marshal to non-empty YAML
// ---------------------------------------------------------------------------

func TestDemoLambda_RawStruct_YAML(t *testing.T) {
	resources, ok := demo.GetResources("lambda")
	if !ok {
		t.Fatal("GetResources(\"lambda\") returned ok=false")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil", i, r.ID)
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
// TestDemoRDS_RawStruct: every RDS fixture must have RawStruct (rdstypes.DBInstance)
// ---------------------------------------------------------------------------

func TestDemoRDS_RawStruct(t *testing.T) {
	resources, ok := demo.GetResources("dbi")
	if !ok {
		t.Fatal("GetResources(\"dbi\") returned ok=false")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil; RDS fixtures must populate RawStruct", i, r.ID)
			continue
		}

		db, ok := r.RawStruct.(rdstypes.DBInstance)
		if !ok {
			t.Errorf("resource[%d] (%s): RawStruct is %T, want rdstypes.DBInstance", i, r.ID, r.RawStruct)
			continue
		}

		// DBInstanceIdentifier must match resource ID
		if db.DBInstanceIdentifier == nil || *db.DBInstanceIdentifier != r.ID {
			t.Errorf("resource[%d] (%s): DBInstanceIdentifier = %v, want %q", i, r.ID, db.DBInstanceIdentifier, r.ID)
		}

		// Engine must be set
		if db.Engine == nil {
			t.Errorf("resource[%d] (%s): Engine is nil", i, r.ID)
		}

		// DBInstanceStatus must be set
		if db.DBInstanceStatus == nil {
			t.Errorf("resource[%d] (%s): DBInstanceStatus is nil", i, r.ID)
		}

		// DBInstanceClass must be set
		if db.DBInstanceClass == nil {
			t.Errorf("resource[%d] (%s): DBInstanceClass is nil", i, r.ID)
		}

		// MultiAZ must be set
		if db.MultiAZ == nil {
			t.Errorf("resource[%d] (%s): MultiAZ is nil", i, r.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// TestDemoRDS_RawStruct_YAML: RDS RawStruct must marshal to non-empty YAML
// ---------------------------------------------------------------------------

func TestDemoRDS_RawStruct_YAML(t *testing.T) {
	resources, ok := demo.GetResources("dbi")
	if !ok {
		t.Fatal("GetResources(\"dbi\") returned ok=false")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil", i, r.ID)
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
// TestDemoS3Objects_RawStruct: S3 object fixtures must have RawStruct
// S3 objects have two types: CommonPrefix (folders) and Object (files)
// ---------------------------------------------------------------------------

func TestDemoS3Objects_RawStruct(t *testing.T) {
	objects, ok := demo.GetS3Objects("data-pipeline-logs", "")
	if !ok {
		t.Fatal("GetS3Objects returned ok=false")
	}

	for i, r := range objects {
		if r.RawStruct == nil {
			t.Errorf("object[%d] (%s): RawStruct is nil; S3 object fixtures must populate RawStruct", i, r.ID)
			continue
		}

		switch raw := r.RawStruct.(type) {
		case s3types.CommonPrefix:
			// Folder type
			if r.Status != "folder" {
				t.Errorf("object[%d] (%s): Status=%q but RawStruct is CommonPrefix (expected folder)", i, r.ID, r.Status)
			}
			if raw.Prefix == nil {
				t.Errorf("object[%d] (%s): CommonPrefix.Prefix is nil", i, r.ID)
			}
		case s3types.Object:
			// File type
			if r.Status != "file" {
				t.Errorf("object[%d] (%s): Status=%q but RawStruct is Object (expected file)", i, r.ID, r.Status)
			}
			if raw.Key == nil {
				t.Errorf("object[%d] (%s): Object.Key is nil", i, r.ID)
			}
		default:
			t.Errorf("object[%d] (%s): RawStruct is %T, want s3types.CommonPrefix or s3types.Object", i, r.ID, r.RawStruct)
		}
	}
}

// ---------------------------------------------------------------------------
// TestDemoS3Objects_RawStruct_YAML: S3 objects RawStruct must marshal to YAML
// ---------------------------------------------------------------------------

func TestDemoS3Objects_RawStruct_YAML(t *testing.T) {
	objects, ok := demo.GetS3Objects("data-pipeline-logs", "")
	if !ok {
		t.Fatal("GetS3Objects returned ok=false")
	}

	for i, r := range objects {
		if r.RawStruct == nil {
			t.Errorf("object[%d] (%s): RawStruct is nil", i, r.ID)
			continue
		}
		safe := fieldpath.ToSafeValue(reflect.ValueOf(r.RawStruct))
		out, err := yaml.Marshal(safe)
		if err != nil {
			t.Errorf("object[%d] (%s): yaml.Marshal(RawStruct) failed: %v", i, r.ID, err)
			continue
		}
		if len(out) == 0 {
			t.Errorf("object[%d] (%s): yaml.Marshal(RawStruct) produced empty output", i, r.ID)
		}
	}
}
