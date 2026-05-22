// accessors.go — PR-05a-h4-c (AS-963) typed Core accessors.
//
// Exports a small set of typed read/write methods on *Core that lift the
// session-state reads the renderer adapters previously did via
// `m.core.Session().<Field>`. The Session() accessor on tui.Model goes
// away in this PR — these methods, the ServiceClients alias in
// transport.go, and the related-cache helpers in relatedcache.go are the
// renderer-facing surface that replaces direct session imports.
//
// The accessor list is the spec-mandated minimum (docs/refactor/05-pr-05a-h4.md
// lines 525–535, 549–554). Convenience constructors that internalise
// session.New() also live here so the renderer's Model construction path
// does not need to import internal/session.
package runtime

import (
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/session"
)

// Bootstrap constructs a fresh *Core seeded with a new session.Session
// configured for the given profile/region pair. Used by the renderer's
// model constructor (tui.New) so it can build the Core without importing
// internal/session.
func Bootstrap(profile, region string, types []catalog.ResourceTypeDef) *Core {
	s := session.New()
	s.Profile = profile
	s.Region = region
	return New(s, types)
}

// Profile returns the active session profile.
func (c *Core) Profile() string { return c.session.Profile }

// Region returns the active session region.
func (c *Core) Region() string { return c.session.Region }

// ConnectGen returns the staleness counter for AWS connect attempts.
func (c *Core) ConnectGen() domain.Gen { return c.session.ConnectGen }

// Identity returns the renderer-shaped domain mirror of the session's
// caller identity, or nil if no identity has been resolved yet. The
// awsclient-typed pointer stays inside Core; the adapter renders only
// the domain fields exposed here.
func (c *Core) Identity() *domain.CallerIdentity {
	if c.session.Identity == nil {
		return nil
	}
	return domainCallerIdentityFrom(c.session.Identity)
}

// SetIdentityFetching sets the session-wide IdentityFetching latch.
func (c *Core) SetIdentityFetching(v bool) { c.session.IdentityFetching = v }

// ClearCommand clears the one-shot -c CLI flag command after the first
// ClientsReady has consumed it.
func (c *Core) ClearCommand() { c.session.Command = "" }

// ResourceCache returns the cached top-level resource-list entry for the
// given resource short name, or (nil, false) when no entry is cached.
// Renderer adapters use this in place of indexing the session map
// directly. The return type is the list-view cache entry shape, distinct
// from the related-checker snapshot's domain.ResourceCacheEntry.
func (c *Core) ResourceCache(rt string) (*domain.ListViewCacheEntry, bool) {
	e, ok := c.session.ResourceCache[rt]
	return e, ok
}

// LazyResourceCache returns the lazy-cache slice for the given resource
// short name (resources pulled via FetchByIDs for filtered-target
// drills). The bool reports whether any lazy-cache entry exists for
// the type — distinct from a non-nil empty slice.
func (c *Core) LazyResourceCache(rt string) ([]domain.Resource, bool) {
	rows, ok := c.session.LazyResourceCache[rt]
	return rows, ok
}

// HasIssueEnricher reports whether a Wave-2 issue enricher is registered
// for the given resource short name. Renderer adapters use this in place
// of probing awsclient.Wave2EnricherFor / awsclient.IssueEnricherRegistry
// directly.
func (c *Core) HasIssueEnricher(shortName string) bool {
	_, ok := awsclient.Wave2EnricherFor(shortName)
	return ok
}
