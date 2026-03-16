package unit_test

import (
	"strings"
	"testing"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/internal/views"
)

// TestConfigDetail_NilFieldsStillShown verifies that configured detail paths
// with nil values are still displayed, not hidden.
func TestConfigDetail_NilFieldsStillShown(t *testing.T) {
	bucket := s3types.Bucket{
		Name:         strPtr("my-bucket"),
		BucketRegion: nil,
	}

	paths := []string{"Name", "BucketRegion"}
	m := views.NewConfigDetailModel("Test", bucket, paths)
	m.Width = 80
	m.Height = 30

	output := m.View()

	if !strings.Contains(output, "BucketRegion") {
		t.Errorf("nil field 'BucketRegion' should still appear in output:\n%s", output)
	}
	if !strings.Contains(output, "Name") {
		t.Errorf("field 'Name' missing from output:\n%s", output)
	}
}
