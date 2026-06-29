package web

import (
	"context"
	"time"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
	// Startup handshake: run HandleClientsReady + DrainSync to resolve AWS
	// client availability before processing any command or navigation.
	if pre := core.PreSuppliedClients(); pre != nil {
		// Demo / test path: clients are pre-supplied synchronously.
		intents, tasks := core.HandleClientsReady(runtime.ClientsReadyEvent{
			Clients: pre, Region: core.Region(), Gen: core.ConnectGen(), StackDepth: 1,
		})
		ctrl.ApplyIntents(intents)
		app.DrainSync(ctrl, tasks)
	} else {
		// Live path: connect to AWS synchronously so the first action/command
		// has valid clients. A 30-second timeout prevents a hung connect from
		// blocking the GET / handler forever. On failure the session stays on
		// the main menu — a later fetch will surface the error to the user.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ev, err := core.ExecuteTask(ctx, runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.TaskKindConnect},
			Payload: runtime.ConnectPayload{Profile: profile, Region: region, Gen: core.ConnectGen()},
		})
		if err == nil {
			if cr, ok := ev.(messages.ClientsReady); ok && cr.Err == nil {
				intents, tasks := core.HandleClientsReady(runtime.ClientsReadyEvent{
					Clients: cr.Clients, Region: cr.Region, Gen: cr.Gen, StackDepth: 1,
				})
				ctrl.ApplyIntents(intents)
				app.DrainSync(ctrl, tasks)
			}
		}
	}
	if command != "" {
		_, tasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: command})
		app.DrainSync(ctrl, tasks)
	}
	return ctrl
}
