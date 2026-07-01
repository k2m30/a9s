package app

import (
	"strconv"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// TestFieldSelect_ByID_FetchesAndOpensDetail pins the full web field-click
// contract end to end: ActionFieldSelect on a navigable field targeting a
// by-ID type (ec2 ImageId → ami) must dispatch a KindFetchByIDDetail task, and
// once its ResourcesLoaded result arrives the placeholder list must be replaced
// by the target's detail — not a one-row list. Complements
// handle_autoopen_test.go, which only exercises an already-flagged placeholder.
func TestFieldSelect_ByID_FetchesAndOpensDetail(t *testing.T) {
	const amiID = "ami-0field1234abcd01"

	// EC2 instance whose RawStruct ImageId feeds the navigable ImageId→ami field.
	src := resource.Resource{
		ID:   "i-0field000000000a1",
		Name: "field-src",
		Type: "ec2",
		RawStruct: &ec2types.Instance{
			InstanceId: awssdk.String("i-0field000000000a1"),
			ImageId:    awssdk.String(amiID),
		},
		Fields: map[string]string{"instance_id": "i-0field000000000a1", "name": "field-src"},
	}

	c := New(runtime.New(session.New(), nil))
	c.SetViewConfig(config.DefaultConfig())
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: src.ID},
	}})
	c.EnsureDetailState(src, "ec2")

	// Locate the navigable ImageId field (targets the by-ID ami type). The web
	// UI's clickField sends this same field index.
	snap := c.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("expected a detail body")
	}
	fieldIdx := -1
	for i, f := range snap.Body.Detail.Fields {
		if f.IsNavigable && f.TargetType == "ami" {
			fieldIdx = i
			break
		}
	}
	if fieldIdx < 0 {
		t.Fatal("no navigable ami field found in the ec2 detail body")
	}

	// Click the field: the ami is not cached and has a FetchByIDs helper, so
	// HandleRelatedNavigate returns a by-ID detail drill.
	_, tasks := c.Apply(Action{Kind: ActionFieldSelect, Arg: strconv.Itoa(fieldIdx)})
	foundByID := false
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.KindFetchByIDDetail {
			foundByID = true
		}
	}
	if !foundByID {
		t.Fatalf("ActionFieldSelect on the ami field returned no KindFetchByIDDetail task; tasks=%+v", tasks)
	}

	// The by-ID fetch result arrives via Handle (the web/headless entry point) →
	// the placeholder list is replaced by the ami's detail.
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ami",
		Gen:          c.core.AvailabilityGen(),
		Resources: []resource.Resource{
			{ID: amiID, Name: "acme-ami", Type: "ami", Fields: map[string]string{"image_id": amiID}},
		},
	})

	snap = c.Snapshot()
	if snap.Body.Kind != BodyKindDetail {
		t.Fatalf("expected a detail screen after the by-id load, got %q", snap.Body.Kind)
	}
	if got := c.GetDetailResource().ID; got != amiID {
		t.Errorf("detail resource ID = %q, want %q", got, amiID)
	}
}
