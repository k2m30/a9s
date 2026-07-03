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
	ctrl.SetUIMode("web")

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
	} else if !core.NoCache() {
		// Live path, no pre-supplied clients: C1 requires the cached menu to
		// render "before any AWS activity" — the AWS connect itself (and thus
		// the TaskKindLoadAvailCache dispatch inside handleClientsReadySuccess)
		// only starts once getOrCreateSession's background BootstrapLive
		// goroutine runs, which can lag the first GET/ /state response. Load
		// (or reuse) this pair's disk Store synchronously here, before
		// returning, so the very first snapshot already carries the cached
		// counts/issue badges/exact flags — mirroring what the
		// TaskKindLoadAvailCache executor path does, without waiting on a live
		// connection. LoadAvailabilityCache (not the raw EnsureCacheStore)
		// resolves an unset Region from the profile's config-file default so
		// the seed still fires when no -r flag was passed.
		if store := core.LoadAvailabilityCache(); store != nil {
			ev := runtime.CacheStoreToEvent(store)
			ctrl.Handle(ev)
		}
	}
	// Live path: ctrl is returned on the menu; getOrCreateSession connects to AWS
	// in the background and applies any startup command there.
	return ctrl
}
