// app_options.go — tui.Model construction options (WithProfile, WithRegion,
// WithIsDemo, WithNoCache, WithClients, WithActiveTheme, WithCommand).
//
// Each option is a small closure that mutates the renderer-side Model or pokes a
// typed setter on m.core. WithClients takes the runtime-side ServiceClients alias
// so this package never imports the AWS-client package directly — an enforced
// import boundary.
package tui

import "github.com/k2m30/a9s/v3/internal/runtime"

// WithProfile overrides the profile field on the active session — used in
// tests that need a specific profile string without going through the live
// AWS bootstrap path.
func WithProfile(profile string) Option {
	return func(m *Model) { m.core.SetProfile(profile) }
}

// WithRegion overrides the region field on the active session — used in
// tests that need a specific region string without going through the live
// AWS bootstrap path.
func WithRegion(region string) Option {
	return func(m *Model) { m.core.SetRegion(region) }
}

// WithIsDemo marks the session as demo mode, which skips Wave 2 enrichment.
// Set by the --demo CLI bootstrap path. Distinct from WithNoCache which only
// disables disk persistence.
//
// Sets both m.isDemo (renderer-side) and m.core.SetIsDemo so Core.ExecuteTask and
// renderer-neutral callers share the same source of truth.
func WithIsDemo(demo bool) Option {
	return func(m *Model) {
		m.isDemo = demo
		m.core.SetIsDemo(demo)
	}
}

// WithNoCache disables resource availability caching and background checks.
func WithNoCache(disabled bool) Option {
	return func(m *Model) {
		m.core.SetNoCache(disabled)
	}
}

// WithClients pre-supplies a set of service clients so that Init() emits a
// synthetic ClientsReadyMsg instead of initiating a live AWS connection.
// The parameter type is a runtime-side ServiceClients alias (transparent
// re-export of the AWS-typed value) so the TUI package never imports the
// AWS-client package directly.
func WithClients(clients *runtime.ServiceClients) Option {
	return func(m *Model) {
		m.core.SetPreSuppliedClients(clients)
	}
}

// WithActiveTheme sets the initial active theme filename for the selector's
// "(current)" indicator. main.go passes the validated theme after loading it.
func WithActiveTheme(name string) Option {
	return func(m *Model) { m.activeTheme = name }
}

// WithCommand sets the initial resource short name to navigate to on the first
// ClientsReadyMsg. Used by the -c/--command CLI flag to open a resource list
// directly on startup instead of the main menu. The caller is responsible for
// resolving the input via resource.FindResourceType.
func WithCommand(name string) Option {
	return func(m *Model) { m.core.SetCommand(name) }
}
