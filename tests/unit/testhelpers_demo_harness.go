package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// noopChecker is a RelatedChecker that returns zero results. Use it in
// RelatedDef structs when the test only needs the def to be non-nil
// (e.g. to trigger right-column rendering) but doesn't need real data.
var noopChecker resource.RelatedChecker = func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{Count: 0}
}

// newDemoColdCacheApp constructs a tui.Model exactly as cmd/a9s/main.go will
// after feature 014-demo-transport-mock is fully wired (T036d). It uses
// demo.NewServiceClients() to supply fake clients and passes them via
// tui.WithClients so no live AWS calls are made.
//
// The model is cold-cache: resourceCache starts empty, no preloading, no nil
// clients. Callers drive it by sending messages via model.Update().
func newDemoColdCacheApp(t *testing.T) *tui.Model {
	t.Helper()
	clients := demo.NewServiceClients()
	m := tui.New(
		demo.DemoProfile,
		demo.DemoRegion,
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	return &m
}
