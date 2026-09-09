// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// fetch_outcome.go — the one place a fetch's answer is built.

package runtime

import (
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// FetchOutcome is what a fetch request is, held until the fetcher says how it
// ended. Msg turns that ending into the message the request answers with, and
// it is the only construction of APIError in the tree.
//
// A failure and the success it replaces are the two endings of ONE request, so
// they are routed and ordered by the same facts: the screen that asked, the
// sequence that says which of that screen's requests this is, the lane the
// delivery gate matches on, and the load-more pair the activity flags read.
// Built as two literals side by side they are one edit away from disagreeing,
// and they did disagree — a failed page named no screen, so it was applied to
// whichever list of its type was topmost.
//
// A caller with no request behind it (a connect failure) fills in what it has
// and leaves the rest zero, which costs it exactly what the literal did.
type FetchOutcome struct {
	ResourceType string
	// Gen is the session generation the dispatch was stamped at.
	Gen domain.Gen
	// TypeGen is the Wave-2 rerun token, carried on the success only: a
	// failure answers no rerun.
	TypeGen domain.Gen
	// ListSeq orders this request against the issuing screen's others.
	ListSeq domain.Gen
	// ScreenID names the list screen that asked. Zero means no screen owns
	// this answer, and no screen receives it.
	ScreenID domain.Gen
	Lane     messages.FetchProvenance
	// Append and LoadingMore describe a continuation: which of a screen's two
	// possible in-flight requests this is, so its completion retires the right
	// activity flag.
	Append      bool
	LoadingMore bool
}

// Msg returns the message this request answers with. A hard failure — an error
// with no resources at all — is an APIError; anything else is a
// ResourcesLoaded, including the partial-success case where a fetcher returns
// both rows and a composite error, so the error surfaces and the rows are kept.
func (o FetchOutcome) Msg(res resource.FetchResult, err error) messages.Event {
	if err != nil && len(res.Resources) == 0 {
		return messages.APIError{
			ResourceType: o.ResourceType,
			Err:          err,
			Gen:          o.Gen,
			ListSeq:      o.ListSeq,
			ScreenID:     o.ScreenID,
			Append:       o.Append,
			LoadingMore:  o.LoadingMore,
			Provenance:   o.Lane,
		}
	}
	return messages.ResourcesLoaded{
		ResourceType: o.ResourceType,
		Resources:    res.Resources,
		Pagination:   res.Pagination,
		Append:       o.Append,
		LoadingMore:  o.LoadingMore,
		Err:          err,
		Gen:          o.Gen,
		TypeGen:      o.TypeGen,
		ListSeq:      o.ListSeq,
		ScreenID:     o.ScreenID,
		Provenance:   o.Lane,
	}
}
