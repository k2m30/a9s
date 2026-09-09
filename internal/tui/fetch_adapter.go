// SPDX-License-Identifier: GPL-3.0-or-later

// fetch_adapter.go — thin Bubble Tea adapter over runtime.Core fetch methods.
// Each method captures ctx and clients, delegates to the corresponding Core
// method, and converts the result to TUI message types.
//
// Every fetch that produces ResourcesLoaded / APIError / IdentityLoaded /
// IdentityError / ValueRevealed captures the dispatch-time generation counter
// (gen domain.Gen) at the call site. The gen is stamped onto the returned message
// so the app.go handler can discard stale results after a profile/region switch
// (messages.IsStale guard). Callers pass m.core.AvailabilityGen() (for
// resource fetches) or m.core.ConnectGen() (for identity/reveal fetches).
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// profilesLoadedMsg is a TUI-private message carrying AWS profile names
// returned by fetchProfiles.
type profilesLoadedMsg struct {
	profiles []string
}

// fetchOutcome is the one place a fetch's two possible answers are built. A
// failure is the other outcome of the same request, so it is routed and
// ordered by the same facts its paired success would have carried: the screen
// that asked, the sequence that says which request this is, and the lane the
// delivery gate matches on. Two hand-built literals per call site let those
// drift, and they did — a failed page named no screen, so it was applied to
// whichever list of its type was topmost, and carried no sequence, so a
// failure a later refresh had already superseded could never be recognised as
// stale.
type fetchOutcome struct {
	resourceType string
	gen          domain.Gen
	seq          domain.Gen
	screen       domain.Gen
	lane         messages.FetchProvenance
	appendPage   bool
	loadingMore  bool
}

// msg turns a fetcher's return into the message the request answers with.
//
// Partial-success contract: fetchers may return BOTH a non-empty
// result.Resources AND a composite error. When that happens the error is
// surfaced AND the partial Resources kept; a hard failure (no resources at
// all) routes through APIError.
func (o fetchOutcome) msg(res resource.FetchResult, err error) tea.Msg {
	if err != nil && len(res.Resources) == 0 {
		return messages.APIError{
			ResourceType: o.resourceType,
			Err:          err,
			Gen:          o.gen,
			ListSeq:      o.seq,
			ScreenID:     o.screen,
			Append:       o.appendPage,
			LoadingMore:  o.loadingMore,
			Provenance:   o.lane,
		}
	}
	return messages.ResourcesLoaded{
		ResourceType: o.resourceType,
		Resources:    res.Resources,
		Pagination:   res.Pagination,
		Append:       o.appendPage,
		LoadingMore:  o.loadingMore,
		Err:          err,
		Gen:          o.gen,
		ListSeq:      o.seq,
		ScreenID:     o.screen,
		Provenance:   o.lane,
	}
}

// listFetchIdentity resolves the screen a list fetch is issued by and the
// sequence that orders it against that screen's other requests. A fetch with
// no list screen on top draws none — there is nothing for it to be ordered
// against.
func (m *Model) listFetchIdentity() (screen, seq domain.Gen) {
	screen = m.ctrl.GetListInstance()
	if screen == 0 {
		return 0, 0
	}
	return screen, m.core.NextListFetchSeq(screen)
}

// fetchResources returns a tea.Cmd that calls Core.FetchResources and
// converts the result to ResourcesLoaded or APIError.
// gen is the AvailabilityGen captured at dispatch time; it is stamped onto the
// returned message so the handler can discard stale results after a switch.
//
// lane is the screen's own lane, from core/app's one owner (GetListLane) for
// a refresh of an open list, or the constant the call site knows for a fresh
// list open. It decides which screen the delivery gate will accept this result
// on.
//
// Every lane draws a sequence: the guard is keyed by the screen instance, so a
// drill's refresh orders the drill's own requests and reaches no other screen.
func (m *Model) fetchResources(resourceType string, gen domain.Gen, lane messages.FetchProvenance) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	screen, seq := m.listFetchIdentity()
	out := fetchOutcome{resourceType: resourceType, gen: gen, seq: seq, screen: screen, lane: lane}
	return func() tea.Msg {
		return out.msg(m.core.FetchResources(ctx, clients, resourceType))
	}
}

// fetchResourcesFiltered returns a tea.Cmd for a server-side filtered fetch.
// gen is the AvailabilityGen captured at dispatch time.
func (m *Model) fetchResourcesFiltered(resourceType string, filter map[string]string, gen domain.Gen) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	out := fetchOutcome{resourceType: resourceType, gen: gen, lane: messages.FetchProvenanceFilteredList}
	return func() tea.Msg {
		return out.msg(m.core.FetchResourcesFiltered(ctx, clients, resourceType, filter))
	}
}

// fetchByIDDetail fetches a single resource of targetType by exact ID via its
// registered FetchByIDs helper and navigates straight to its detail view. It
// generalises the former ami-only adapter shortcut: the runtime now emits
// KindFetchByIDDetail for any by-ID-capable type on a cache-miss exact-ID drill.
func (m *Model) fetchByIDDetail(targetType, id string) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	if clients == nil {
		// Clients() is nil until the Init()/ClientsReady round trip installs
		// it (handleClientsReadySuccess's own PreSuppliedClients fallback,
		// core/runtime/handlers.go) — a real key press can only reach
		// this adapter after that has settled, but a directly-driven Update
		// sequence (a scripted key-chain, or a harness that skips Init())
		// can call this before it has. PreSuppliedClients is already sitting
		// there unused in that case, so fall back to it rather than fail a
		// resource lookup that has everything it needs.
		clients = m.core.PreSuppliedClients()
	}
	fn := resource.GetFetchByIDs(targetType)
	if fn == nil {
		return func() tea.Msg {
			return messages.ByIDFetchFailed{TargetType: targetType, ID: id, Reason: fmt.Sprintf("no by-id fetcher for %s", targetType)}
		}
	}
	return func() tea.Msg {
		res, err := fn(ctx, clients, []string{id})
		if err != nil {
			return messages.ByIDFetchFailed{TargetType: targetType, ID: id, Reason: err.Error()}
		}
		if len(res) == 0 {
			return messages.ByIDFetchFailed{TargetType: targetType, ID: id, Reason: fmt.Sprintf("%s %s not found", targetType, id)}
		}
		r := res[0]
		return messages.Navigate{Target: messages.TargetDetail, ResourceType: targetType, Resource: &r}
	}
}

// fetchChildResources returns a tea.Cmd for paginated child resource loading.
// Child resource fetches use AvailabilityGen so stale results from prior
// profile/region are discarded along with top-level fetches.
func (m *Model) fetchChildResources(childType string, parentCtx map[string]string) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	gen := m.core.AvailabilityGen()
	out := fetchOutcome{resourceType: childType, gen: gen, lane: messages.FetchProvenanceChild}
	return func() tea.Msg {
		return out.msg(m.core.FetchChildResources(ctx, clients, childType, parentCtx))
	}
}

// fetchMoreResources returns a tea.Cmd that fetches the next page of a
// paginated resource list using the continuation token from LoadMoreMsg.
// gen is the AvailabilityGen captured at dispatch time.
func (m *Model) fetchMoreResources(msg messages.LoadMore) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	gen := m.core.AvailabilityGen()
	p := runtime.FetchMoreParams{
		ResourceType: msg.ResourceType,
		Token:        msg.ContinuationToken,
		ParentCtx:    msg.ParentContext,
		FetchFilter:  msg.FetchFilter,
	}
	// The lane the list that pressed "m" recorded on the message, not a
	// re-derivation from the two maps above — see messages.LoadMore.
	provenance := msg.Provenance
	if provenance == messages.FetchProvenanceUnknown {
		provenance = messages.ProvenanceForContinuation(msg.ParentContext, msg.FetchFilter)
	}
	screen, seq := m.listFetchIdentity()
	out := fetchOutcome{
		resourceType: msg.ResourceType, gen: gen, seq: seq, screen: screen,
		lane: provenance, appendPage: true, loadingMore: true,
	}
	return func() tea.Msg {
		return out.msg(m.core.FetchMoreResources(ctx, clients, p))
	}
}

// fetchIdentity returns a tea.Cmd that fetches the AWS caller identity.
// gen is the ConnectGen captured at dispatch time; it is stamped onto the
// returned message so the handler can discard stale results after a switch.
func (m *Model) fetchIdentity(gen domain.Gen) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	return func() tea.Msg {
		identity, err := m.core.FetchIdentity(ctx, clients)
		if err != nil {
			return messages.IdentityError{Err: err.Error(), Gen: gen}
		}
		return messages.IdentityLoaded{Identity: identity, Gen: gen}
	}
}

// fetchProfiles returns a tea.Cmd that reads the local AWS config profiles.
func (m *Model) fetchProfiles() tea.Cmd {
	return func() tea.Msg {
		profiles, err := m.core.FetchProfiles()
		if err != nil {
			// The whole sentence from the one formatter, the same call the
			// controller's profile selector makes, so a failed local config
			// read reads the same on both lanes.
			return messages.Flash{Text: awsclient.LocalConfigFailure(err), IsError: true}
		}
		return profilesLoadedMsg{profiles: profiles}
	}
}

// fetchRevealValue returns a tea.Cmd that calls the registered reveal fetcher.
// gen is the ConnectGen captured at dispatch time; it is stamped onto the
// returned message so the handler can discard stale results after a switch.
func (m *Model) fetchRevealValue(resourceType, resourceID string, gen domain.Gen) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	return func() tea.Msg {
		value, err := m.core.FetchRevealValue(ctx, clients, resourceType, resourceID)
		if err != nil {
			return messages.ValueRevealed{ResourceType: resourceType, ResourceID: resourceID, Err: err, Gen: gen}
		}
		return messages.ValueRevealed{ResourceType: resourceType, ResourceID: resourceID, Value: value, Gen: gen}
	}
}

// connectAWS returns a tea.Cmd that establishes an AWS session and emits a
// ClientsReadyMsg. gen is a monotonic counter incremented on each
// profile/region switch; stale ClientsReadyMsg values carrying an old gen are
// discarded by handleClientsReady.
func (m *Model) connectAWS(profile, region string, gen domain.Gen) tea.Cmd {
	ctx := m.appCtx
	return func() tea.Msg {
		result, err := m.core.ConnectAWS(ctx, profile, region)
		return messages.ClientsReady{
			Clients: result.Clients,
			Region:  result.Region,
			Gen:     gen,
			Err:     err,
		}
	}
}
