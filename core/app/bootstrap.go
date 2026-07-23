// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"context"
	"time"

	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// liveConnectTimeout bounds the AWS connect so a hung/slow connect can never
// wedge a session indefinitely.
const liveConnectTimeout = 30 * time.Second

// BootstrapLive connects a live (non-demo) session to AWS and resolves the
// initial resource availability. It is designed to run in a background
// goroutine so the web GET / handler can render the menu immediately instead of
// blocking on AWS:
//
//   - The slow connect (ExecuteTask) runs WITHOUT the controller lock.
//   - Only the state mutations (HandleClientsReady + ApplyIntents) take the
//     controller's own mutex, briefly — concurrent Snapshot()/Apply() from
//     request handlers are never blocked for the connect's duration.
//   - It returns the availability tasks for the caller to drain (with a
//     per-result notify for progressive live updates), instead of draining
//     inline.
//
// This mirrors the TUI, where connect runs in a tea.Cmd and availability fills
// in asynchronously. On connect failure the session stays on the menu; the
// failure is routed through Core.HandleClientsReady below (same as success),
// so the error flash and rollback surface here rather than being left for the
// caller to reconstruct separately.
func (c *Controller) BootstrapLive(profile, region string) []runtime.TaskRequest {
	ctx, cancel := context.WithTimeout(context.Background(), liveConnectTimeout)
	defer cancel()

	connectGen := c.core.ConnectGen()
	req := runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.TaskKindConnect},
		Payload: runtime.ConnectPayload{Profile: profile, Region: region, Gen: connectGen},
	}
	ev, err := c.core.ExecuteTask(ctx, req)
	if err != nil {
		return c.routeClientsReady(runtime.ClientsReadyEvent{
			Err:    err,
			Region: region,
			Gen:    connectGen,
		})
	}
	cr, ok := ev.(messages.ClientsReady)
	if !ok {
		return nil
	}

	return c.routeClientsReady(runtime.ClientsReadyEvent{
		Clients: cr.Clients,
		Err:     cr.Err,
		Region:  cr.Region,
		Gen:     cr.Gen,
	})
}

// routeClientsReady runs Core.HandleClientsReady under the controller lock
// (mutating: installs clients, bumps the availability gen; on failure rolls
// back + flashes) and returns the resulting tasks for the caller to drain.
// StackDepth/HasActiveRL/HasActiveCosts are computed under the same lock so
// HandleClientsReady sees the real screen stack instead of a hardcoded
// StackDepth: 1 that used to make maybeRefreshIntents think no active list
// existed (C10: this is what caused a pre-connect navigation's replay to be
// dropped on the web/headless lane). Shared by both the success and failure
// paths of BootstrapLive so a failed connect gets identical treatment to a
// successful one instead of being silently dropped.
func (c *Controller) routeClientsReady(ev runtime.ClientsReadyEvent) []runtime.TaskRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	ev.StackDepth = len(c.stack)
	ev.HasActiveRL = c.topListState() != nil
	ev.HasActiveCosts = c.costsStateBeneathOverlay() != nil
	ev.NewGen = c.core.ConnectGen()
	intents, tasks := c.core.HandleClientsReady(ev)
	c.applyIntents(intents)
	tasks = append(tasks, c.refreshTasksForIntents(intents)...)
	return c.stampDispatchSnapshotLocked(tasks)
}
