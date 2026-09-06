// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"time"

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
// Mirrors handleResourcesLoadedEvent's C6 scope gate: the disk-cache save
// only fires when the top-of-stack screen is the canonical top-level
// ScreenResourceList (not ScreenChildList, and not a filtered/related-nav
// list — EscPops/ParentContext), matching the real task-result lane.
func (c *Controller) ApplyResourcesLoaded(typeName string, resources []resource.Resource, pagination *resource.PaginationMeta, appendPage bool) {
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
	if topLevelCanonical {
		c.maybeSaveResourceListCache(ls, canon)
	}
}

// Now is this package's read of the wall clock. Production calls it wherever
// it needs the current instant; SetNowForTest pins it so a scenario that
// seeds a date-dependent screen (the costs window) renders the same output on
// any day. The costs code itself keeps taking an injected now — this is only
// the seam at the lane entrance where that now is first produced.
func Now() time.Time { return nowFn() }

var nowFn = time.Now

// SetNowForTest replaces the clock Now reads and returns a function that
// restores the previous one. Not concurrency-safe by design, same as the
// package's other test seams: set it before the scenario runs, restore after.
func SetNowForTest(fn func() time.Time) func() {
	prev := nowFn
	nowFn = fn
	return func() { nowFn = prev }
}
