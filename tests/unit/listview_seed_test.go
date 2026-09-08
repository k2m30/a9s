// listview_seed_test.go — the seam view tests use to put rows on a list
// screen now that the view itself no longer applies a list result (listgen
// row 4). Rows reach a screen through the controller, which is where the
// request sequence is checked; a view test that wants a populated list asks
// the controller for one, exactly as production does.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
)

// newListViewCtrl builds the controller a top-level list view runs against,
// mirroring the one views.NewResourceList would have built for itself.
func newListViewCtrl(t *testing.T, td resource.ResourceTypeDef) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})
	if c.Snapshot().Body.List == nil {
		c.PushChildListScreen(td.ShortName)
	}
	c.RegisterFallbackTypeDef(td)
	return c
}

// newChildListViewCtrl is newListViewCtrl for a child type, which is not a
// menu entry and so is pushed directly.
func newChildListViewCtrl(t *testing.T, td resource.ResourceTypeDef) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.PushChildListScreen(td.ShortName)
	c.RegisterFallbackTypeDef(td)
	return c
}
