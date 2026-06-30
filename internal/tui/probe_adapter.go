// probe_adapter.go — Bubble Tea adapter over runtime.Core probe methods. Each
// method captures ctx and clients, delegates to the corresponding Core method,
// and converts the result to TUI message types.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// loadAvailabilityCache returns a tea.Cmd that reads the availability cache
// from disk and converts the result to AvailabilityCacheLoadedMsg.
func (m *Model) loadAvailabilityCache() tea.Cmd {
	profile := m.core.Profile()
	region := m.core.Region()
	return func() tea.Msg {
		cf, err := m.core.LoadAvailabilityCache(profile, region)
		if err != nil || cf == nil {
			return messages.AvailabilityCacheLoaded{
				Entries: make(map[string]int),
				Expired: true,
			}
		}
		entries := make(map[string]int, len(cf.Resources))
		truncated := make(map[string]bool)
		issueCounts := make(map[string]int)
		issueTruncated := make(map[string]bool)
		issueKnown := make(map[string]bool)
		for name, entry := range cf.Resources {
			if entry.Error == "" {
				entries[name] = entry.Count
				if entry.Truncated {
					truncated[name] = true
				}
				if entry.IssuesKnown {
					issueCounts[name] = entry.Issues
					issueKnown[name] = true
					if entry.IssuesTruncated {
						issueTruncated[name] = true
					}
				}
			}
		}
		return messages.AvailabilityCacheLoaded{
			Entries:        entries,
			Truncated:      truncated,
			Expired:        cf.IsExpired(cache.DefaultTTL),
			IssueCounts:    issueCounts,
			IssueTruncated: issueTruncated,
			IssueKnown:     issueKnown,
		}
	}
}

// saveAvailabilityCache returns a tea.Cmd that persists the current
// availability state to disk. No-op when caching is disabled (noCache=true).
func (m *Model) saveAvailabilityCache() tea.Cmd {
	if m.core.NoCache() {
		return nil
	}
	profile := m.core.Profile()
	region := m.core.Region()

	// Collect availability, truncation, and issue counts from main menu.
	var entries map[string]int
	var truncatedMap map[string]bool
	var issueCounts map[string]int
	var issueTruncated map[string]bool
	var issueKnown map[string]bool
	// Read availability/issue state from the controller (single source of truth).
	availability := m.ctrl.GetMenuAvailability()
	if len(availability) == 0 {
		return nil
	}
	entries = availability
	truncatedMap = m.ctrl.GetMenuTruncated()
	issueCounts = m.ctrl.GetMenuIssueCounts()
	issueTruncated = m.ctrl.GetMenuIssueTruncated()
	issueKnown = m.ctrl.GetMenuIssueKnown()

	return func() tea.Msg {
		// Best-effort save — ignore cache write failures.
		_ = m.core.SaveAvailabilityCache(profile, region, entries, truncatedMap, issueCounts, issueTruncated, issueKnown)
		return nil
	}
}

// probeEnrichment returns a tea.Cmd that runs the registered Wave-2 enricher
// for shortName and converts the result to EnrichmentCheckedMsg.
//
// In demo mode (`WithIsDemo(true)`) the contract is to skip Wave-2 enrichment
// entirely — registered enrichers are AWS-keyed and would issue real API calls
// against synthetic fakes / missing credentials. The early return here matches
// the documented WithIsDemo behavior; the registry is not consulted so AWS-only
// enricher contracts are not exercised in demo sessions.
func (m *Model) probeEnrichment(shortName string, gen domain.Gen) tea.Cmd {
	if m.isDemo {
		return nil
	}
	ctx, clients := m.appCtx, m.core.Clients()
	typeGen := m.core.EnrichmentTypeGen(shortName)
	if !m.core.HasIssueEnricher(shortName) {
		return nil
	}
	return func() tea.Msg {
		r := m.core.ProbeEnrichment(ctx, clients, shortName)
		return messages.EnrichmentChecked{
			ResourceType:     shortName,
			Issues:           r.Issues,
			Truncated:        r.Truncated,
			Findings:         r.Findings,
			AttentionDetails: r.AttentionDetails,
			FieldUpdates:     r.FieldUpdates,
			TruncatedIDs:     r.TruncatedIDs,
			Gen:              gen,
			TypeGen:          typeGen,
			Err:              r.Err,
		}
	}
}
