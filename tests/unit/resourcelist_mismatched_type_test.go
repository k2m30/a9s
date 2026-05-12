package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// AS-652 / AS-648-h1: a ResourcesLoaded message whose ResourceType differs
// from the active list's typeDef.ShortName must be dropped — it must not
// mutate m.allResources and must not flip loading state. A late EC2 fetch
// returning after the user has opened S3 used to render EC2 rows in the S3
// list.
func TestResourceListModel_ResourcesLoaded_DropsMismatchedType(t *testing.T) {
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3",
		Columns: []resource.Column{
			{Key: "id", Title: "Name", Width: 20},
		},
	}

	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	// Late, mismatched fetch result for ec2 arrives while this list is s3.
	stale := messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-0123", Name: "i-0123", Fields: map[string]string{"id": "i-0123"}},
			{ID: "i-0456", Name: "i-0456", Fields: map[string]string{"id": "i-0456"}},
		},
	}

	m, cmd := m.Update(stale)

	if got := len(m.AllResources()); got != 0 {
		t.Errorf("mismatched ResourcesLoaded must not mutate allResources: got %d rows, want 0", got)
	}
	if got := len(m.VisibleResources()); got != 0 {
		t.Errorf("mismatched ResourcesLoaded must not populate visibleResources: got %d rows, want 0", got)
	}
	if cmd != nil {
		t.Errorf("mismatched ResourcesLoaded must return nil cmd, got %T", cmd())
	}
}

// AS-652 / AS-648-h1 (symmetric): a matching-type ResourcesLoaded still
// populates m.allResources. Regression guard so the type-guard does not
// over-fire and silently drop legitimate loads.
func TestResourceListModel_ResourcesLoaded_AppliesMatchingType(t *testing.T) {
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3",
		Columns: []resource.Column{
			{Key: "id", Title: "Name", Width: 20},
		},
	}

	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	fresh := messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources: []resource.Resource{
			{ID: "bucket-a", Name: "bucket-a", Fields: map[string]string{"id": "bucket-a"}},
			{ID: "bucket-b", Name: "bucket-b", Fields: map[string]string{"id": "bucket-b"}},
		},
	}

	m, _ = m.Update(fresh)

	if got := len(m.AllResources()); got != 2 {
		t.Errorf("matching-type ResourcesLoaded did not populate allResources: got %d rows, want 2", got)
	}
}
