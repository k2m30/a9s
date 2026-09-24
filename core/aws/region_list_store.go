// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// region_list_store.go — the first pages related reads fetch in Regions other
// than the session's.
package aws

import (
	"context"
	"sync"

	"github.com/k2m30/a9s/v3/core/resource"
)

// RegionListStore keeps, per "<region>/<type>" key, the first page a related
// read fetched in a Region other than the session's, so every detail of the
// session reads it once. Only successful reads are kept. The session owns one
// (core/session/stores.go), replaced on rotate and on refresh.
type RegionListStore struct {
	mu    sync.Mutex
	pages map[string]resource.FetchResult
}

// NewRegionListStore returns an empty store.
func NewRegionListStore() *RegionListStore {
	return &RegionListStore{pages: map[string]resource.FetchResult{}}
}

// firstPage returns key's page, calling fetch when the store does not hold
// it.
// ponytail: two concurrent first reads of one key both fetch; coalesce them if
// a detail ever runs two pivots over one other-Region list at once.
func (s *RegionListStore) firstPage(ctx context.Context, key string, fetch func(context.Context) (resource.FetchResult, error)) (resource.FetchResult, error) {
	s.mu.Lock()
	page, ok := s.pages[key]
	s.mu.Unlock()
	if ok {
		return page, nil
	}
	page, err := fetch(ctx)
	if err == nil {
		s.mu.Lock()
		s.pages[key] = page
		s.mu.Unlock()
	}
	return page, err
}
