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

// AS-762 / AS-649: legacy callers (and ~30 existing tests) construct
// ResourcesLoaded without setting ResourceType — the field defaults to "".
// The type-guard must treat an empty ResourceType as a legacy load and
// admit it, otherwise pre-AS-652 tests and pre-message-driven fetchers
// silently render empty lists. Regression guard for the strict `!=` guard
// that broke ~30 tests on PR #380.
func TestResourceListModel_ResourcesLoaded_AdmitsEmptyResourceType(t *testing.T) {
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

	legacy := messages.ResourcesLoaded{
		// ResourceType intentionally omitted (defaults to "").
		Resources: []resource.Resource{
			{ID: "bucket-a", Name: "bucket-a", Fields: map[string]string{"id": "bucket-a"}},
			{ID: "bucket-b", Name: "bucket-b", Fields: map[string]string{"id": "bucket-b"}},
		},
	}

	m, _ = m.Update(legacy)

	if got := len(m.AllResources()); got != 2 {
		t.Errorf("empty-ResourceType ResourcesLoaded must be admitted: got %d rows, want 2", got)
	}
}

// AS-762 / AS-649: a non-empty ResourceType that matches one of the
// active type's aliases (rather than the canonical ShortName) must be
// admitted via catalog canonicalization. Regression guard for the strict
// string-equality guard that breaks any caller passing an alias.
func TestResourceListModel_ResourcesLoaded_AdmitsAliasResourceType(t *testing.T) {
	k := keys.Default()
	// Pick a registered alias from the catalog to keep the test honest
	// regardless of which aliases live in the registry. If no aliases exist
	// for any type, the test is a no-op (and the empty-ResourceType test
	// above still covers the legacy-load path).
	var canonical *resource.ResourceTypeDef
	var alias string
	for _, sn := range resource.AllShortNames() {
		def := resource.FindResourceType(sn)
		if def == nil {
			continue
		}
		if len(def.Aliases) > 0 {
			canonical = def
			alias = def.Aliases[0]
			break
		}
	}
	if canonical == nil {
		t.Skip("no resource type with aliases registered — alias path covered by guard logic")
	}

	td := resource.ResourceTypeDef{
		Name:      canonical.Name,
		ShortName: canonical.ShortName,
		Aliases:   canonical.Aliases,
		Columns: []resource.Column{
			{Key: "id", Title: "ID", Width: 20},
		},
	}

	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	aliased := messages.ResourcesLoaded{
		ResourceType: alias,
		Resources: []resource.Resource{
			{ID: "r-1", Name: "r-1", Fields: map[string]string{"id": "r-1"}},
		},
	}

	m, _ = m.Update(aliased)

	if got := len(m.AllResources()); got != 1 {
		t.Errorf("alias-ResourceType (%q for %q) ResourcesLoaded must be admitted via catalog canonicalization: got %d rows, want 1",
			alias, canonical.ShortName, got)
	}
}

// AS-762 / AS-649: an ad-hoc resource type that is NOT registered in the
// global catalog (e.g. child-view sub-types like "s3_objects" or
// hand-rolled test ResourceTypeDefs) must still match by literal string
// equality. Falling through to string equality keeps the catalog-blind
// callers and tests passing while alias-canonicalization handles
// registered types.
func TestResourceListModel_ResourcesLoaded_AdmitsUnregisteredTypeViaStringEquality(t *testing.T) {
	k := keys.Default()
	const adhoc = "this_type_is_not_in_the_global_catalog"
	if resource.FindResourceType(adhoc) != nil {
		t.Fatalf("test invariant broken: %q must not be registered in the catalog", adhoc)
	}
	td := resource.ResourceTypeDef{
		Name:      "Ad-hoc",
		ShortName: adhoc,
		Columns: []resource.Column{
			{Key: "id", Title: "ID", Width: 20},
		},
	}

	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	msg := messages.ResourcesLoaded{
		ResourceType: adhoc,
		Resources: []resource.Resource{
			{ID: "r-1", Name: "r-1", Fields: map[string]string{"id": "r-1"}},
		},
	}

	m, _ = m.Update(msg)

	if got := len(m.AllResources()); got != 1 {
		t.Errorf("unregistered-type ResourcesLoaded with matching ShortName must be admitted via string fallback: got %d rows, want 1", got)
	}
}
