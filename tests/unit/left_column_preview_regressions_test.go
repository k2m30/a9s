package unit_test

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/viewport"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// makePreviewEC2Detail builds a Controller with a ScreenDetail on the stack
// for the first demo EC2 fixture, ready for previewDetailView — the live
// replacement for the retired views.NewDetail(...).SetSize(...) chain
// (DetailModel.View/Update are dead; see
// specs/022-codebase-cleanup/wave3-map-detail.md).
func makePreviewEC2Detail(t *testing.T, w, h int) *app.Controller {
	t.Helper()
	ec2Client := fakes.NewEC2()
	ec2, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	c := newDetailController(t, ec2[0], "ec2")
	c.SetViewConfig(config.DefaultConfig())
	c.InitDetailRelatedRows("ec2")
	return c
}

// previewDetailView renders the current DetailBody snapshot of c via the
// live NewTransientDetail+RenderDetail seam, at the given w/h.
func previewDetailView(t *testing.T, c *app.Controller, w, h int) string {
	t.Helper()
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	m := views.NewTransientDetail(w, h, vp)
	return m.RenderDetail(*body)
}

func mustDemoEC2(t *testing.T) []resource.Resource {
	t.Helper()
	ec2Client := fakes.NewEC2()
	ec2, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	return ec2
}
