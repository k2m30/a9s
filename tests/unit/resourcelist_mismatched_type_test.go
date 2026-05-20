package unit

import (
	"strings"
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

	cases := []struct {
		name          string // name of the active list
		listShortName string // ShortName of the active list model
		staleType     string // ResourceType on the stale message (alias or canonical)
	}{
		{"S3 list rejects EC2 rows", "s3", "ec2"},
		{"EC2 list rejects S3 rows", "ec2", "s3"},
		// "rds" is a registered alias for canonical ShortName "dbi".
		// The fetcher stamps the alias on the wire; the guard must still drop it
		// when the active list is for a different type.
		{"S3 list rejects RDS alias rows", "s3", "rds"},
		{"S3 list rejects RDS canonical rows", "s3", "dbi"},
		{"Lambda list rejects EC2 rows", "lambda", "ec2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td := resource.ResourceTypeDef{
				Name:      tc.listShortName,
				ShortName: tc.listShortName,
				Columns: []resource.Column{
					{Key: "id", Title: "ID", Width: 20},
				},
			}

			m := views.NewResourceList(td, nil, k)
			m.SetSize(80, 24)
			m, _ = m.Init()

			stale := messages.ResourcesLoaded{
				ResourceType: tc.staleType,
				Resources: []resource.Resource{
					{ID: "x-0001", Name: "x-0001", Fields: map[string]string{"id": "x-0001"}},
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
			// Behavioral proxy: the stale resource ID must not appear in the
			// rendered view (loading spinner is still shown instead of rows).
			if view := m.View(); strings.Contains(view, "x-0001") {
				t.Errorf("stale resource ID must not appear in View() after mismatched drop")
			}
		})
	}
}

// AS-652 / AS-648-h1 (symmetric): a matching-type ResourcesLoaded still
// populates m.allResources. Regression guard so the type-guard does not
// over-fire and silently drop legitimate loads.
func TestResourceListModel_ResourcesLoaded_AppliesMatchingType(t *testing.T) {
	k := keys.Default()

	cases := []struct {
		name          string
		listShortName string
		msgType       string // may be an alias
	}{
		{"S3 exact match", "s3", "s3"},
		{"EC2 exact match", "ec2", "ec2"},
		// The fetcher stamps "rds" (alias) on the wire while the list holds
		// canonical ShortName "dbi" — the alias-aware guard must not drop it.
		{"RDS alias match (rds→dbi)", "dbi", "rds"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td := resource.ResourceTypeDef{
				Name:      tc.listShortName,
				ShortName: tc.listShortName,
				Columns: []resource.Column{
					{Key: "id", Title: "ID", Width: 20},
				},
			}

			m := views.NewResourceList(td, nil, k)
			m.SetSize(80, 24)
			m, _ = m.Init()

			fresh := messages.ResourcesLoaded{
				ResourceType: tc.msgType,
				Resources: []resource.Resource{
					{ID: "res-a", Name: "res-a", Fields: map[string]string{"id": "res-a"}},
					{ID: "res-b", Name: "res-b", Fields: map[string]string{"id": "res-b"}},
				},
			}

			m, _ = m.Update(fresh)

			if got := len(m.AllResources()); got != 2 {
				t.Errorf("matching-type ResourcesLoaded did not populate allResources: got %d rows, want 2", got)
			}
		})
	}
}
