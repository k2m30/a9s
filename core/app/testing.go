// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"github.com/k2m30/a9s/v3/core/resource"
)

// ApplyResourcesLoaded is a test-support seam that seeds the controller's
// resource cache directly, bypassing the task-result lane. It mirrors the
// task-result handler that runs when resources are fetched via the normal
// task lane. Use this only in tests — the same pattern as DrainSync.
//
// typeName is the canonical short name (e.g. "ec2", "s3").
// appendPage=true accumulates onto the existing cache page; false replaces it.
//
// Mirrors handleResourcesLoadedEvent: the same C6 scope gate (the disk-cache
// save only fires when the top-of-stack screen is the canonical top-level
// ScreenResourceList, not ScreenChildList and not a filtered/related-nav
// list) and the same menu sync-back, so what this seam leaves on the menu and
// on disk is what the real task-result lane leaves.
func (c *Controller) ApplyResourcesLoaded(typeName string, resources []resource.Resource, pagination *resource.PaginationMeta, appendPage bool) {
	// The queued per-type save runs after the lock is released, exactly as
	// Handle runs it (C4) — deferred first so it fires last.
	defer c.flushCacheWrites()
	c.mu.Lock()
	defer c.mu.Unlock()
	// Resolve canonical short name (handles aliases like "rds" → "dbi"),
	// matching handleResourcesLoadedEvent — the real task-result lane never
	// saves under the raw incoming typeName.
	canon := typeName
	if td := resource.FindResourceType(typeName); td != nil {
		canon = td.ShortName
	}
	ls := c.topListState()
	topLevelCanonical := isTopLevelCanonicalList(c.topScreenID(), ls)
	c.applyResourcesLoaded(ls, canon, resources, pagination, appendPage, topLevelCanonical, false)
	if topLevelCanonical && len(c.stack) > 0 {
		// The same second step the real lane takes (handleResourcesLoadedEvent):
		// the menu observation and then the save that records it. Calling only
		// the save would leave this seam persisting an issue count the menu
		// never reconciled — the two answers the round-2 ruling removes.
		c.syncExactTotalToMenu(&c.stack[len(c.stack)-1], canon)
	}
}
