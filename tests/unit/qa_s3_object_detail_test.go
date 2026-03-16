package unit_test

import (
	"testing"
	"time"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/fieldpath"
)

// TestQA_S3ObjectsHasDetailDefaults verifies s3_objects has detail paths defined.
func TestQA_S3ObjectsHasDetailDefaults(t *testing.T) {
	viewDef := config.DefaultViewDef("s3_objects")
	if len(viewDef.Detail) == 0 {
		t.Fatal("s3_objects default ViewDef has no Detail paths — detail view will show 'No details available'")
	}
}

// TestQA_S3ObjectFile_DetailPaths verifies detail extraction works for s3types.Object.
func TestQA_S3ObjectFile_DetailPaths(t *testing.T) {
	now := time.Now()
	obj := s3types.Object{
		Key:          strPtr("path/to/file.txt"),
		Size:         int64Ptr(2048),
		LastModified: &now,
		StorageClass: s3types.ObjectStorageClassStandard,
		ETag:         strPtr("\"abc123\""),
	}

	viewDef := config.DefaultViewDef("s3_objects")
	for _, path := range viewDef.Detail {
		t.Run(path, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ExtractSubtree panicked on s3 object detail path %q: %v", path, r)
				}
			}()
			result := fieldpath.ExtractSubtree(obj, path)
			t.Logf("%s = %q", path, result)
		})
	}
}

// TestQA_S3ObjectFolder_DetailPaths verifies detail extraction works for s3types.CommonPrefix.
func TestQA_S3ObjectFolder_DetailPaths(t *testing.T) {
	cp := s3types.CommonPrefix{
		Prefix: strPtr("my-folder/"),
	}

	viewDef := config.DefaultViewDef("s3_objects")
	for _, path := range viewDef.Detail {
		t.Run(path, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ExtractSubtree panicked on s3 folder detail path %q: %v", path, r)
				}
			}()
			_ = fieldpath.ExtractSubtree(cp, path)
		})
	}
}
