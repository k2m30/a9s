package web

import (
	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// newSession builds a fully-bootstrapped *app.Controller for one web session.
func newSession(profile, region, command string, demoMode, noCache bool, viewCfg *config.ViewsConfig) *app.Controller {
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	if demoMode {
		core.SetPreSuppliedClients(demo.NewServiceClients())
		core.SetNoCache(true)
		core.SetIsDemo(true)
	} else if noCache {
		core.SetNoCache(true)
	}
	ctrl := app.New(core)
	ctrl.SetViewConfig(viewCfg)
	// Startup handshake: demo/test clients are pre-supplied + synchronous, so
	// HandleClientsReady + DrainSync resolves availability inline.
	if pre := core.PreSuppliedClients(); pre != nil {
		intents, tasks := core.HandleClientsReady(runtime.ClientsReadyEvent{
			Clients: pre, Region: core.Region(), Gen: core.ConnectGen(), StackDepth: 1,
		})
		ctrl.ApplyIntents(intents)
		app.DrainSync(ctrl, tasks)
	}
	if command != "" {
		_, tasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: command})
		app.DrainSync(ctrl, tasks)
	}
	return ctrl
}
