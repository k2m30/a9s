package web

import (
	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// newSession builds a *app.Controller for one web session.
//
// Demo sessions have pre-supplied clients, so they bootstrap synchronously here
// (it is instant). Live sessions are returned on the menu WITHOUT connecting —
// getOrCreateSession kicks off the AWS connect in the background via
// Controller.BootstrapLive so the GET / handler never blocks on AWS. The
// startup command is likewise applied synchronously for demo and in the
// background bootstrap for live.
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

	if pre := core.PreSuppliedClients(); pre != nil {
		// Demo / test path: clients are pre-supplied, so the handshake + command
		// resolve synchronously and instantly.
		intents, tasks := core.HandleClientsReady(runtime.ClientsReadyEvent{
			Clients: pre, Region: core.Region(), Gen: core.ConnectGen(), StackDepth: 1,
		})
		ctrl.ApplyIntents(intents)
		app.DrainSync(ctrl, tasks)
		if command != "" {
			_, tasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: command})
			app.DrainSync(ctrl, tasks)
		}
	}
	// Live path: ctrl is returned on the menu; getOrCreateSession connects to AWS
	// in the background and applies any startup command there.
	return ctrl
}
