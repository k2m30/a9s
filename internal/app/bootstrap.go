package app

import (
	"context"
	"time"

	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
// in asynchronously. On connect failure the session stays on the menu with no
// availability; the caller can surface a flash separately.
func (c *Controller) BootstrapLive(profile, region string) []runtime.TaskRequest {
	ctx, cancel := context.WithTimeout(context.Background(), liveConnectTimeout)
	defer cancel()

	ev, err := c.core.ExecuteTask(ctx, runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.TaskKindConnect},
		Payload: runtime.ConnectPayload{Profile: profile, Region: region, Gen: c.core.ConnectGen()},
	})
	if err != nil {
		return nil
	}
	cr, ok := ev.(messages.ClientsReady)
	if !ok || cr.Err != nil {
		return nil
	}

	// HandleClientsReady mutates core (installs clients, bumps the availability
	// gen). Run it under the controller lock so it is serialised with request
	// handlers that touch the controller; the slow ExecuteTask above stayed
	// lock-free on purpose.
	c.mu.Lock()
	intents, tasks := c.core.HandleClientsReady(runtime.ClientsReadyEvent{
		Clients: cr.Clients, Region: cr.Region, Gen: cr.Gen, StackDepth: 1,
	})
	c.applyIntents(intents)
	c.mu.Unlock()

	// Return the availability/initial-fetch tasks for the caller to drain. The
	// live web session drains them with a per-result SSE notify
	// (Server.bootstrapLiveSession) so the menu fills in progressively instead
	// of all-at-once at the end of a slow live drain.
	return tasks
}
