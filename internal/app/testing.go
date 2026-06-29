package app

import "github.com/k2m30/a9s/v3/internal/resource"

// ApplyResourcesLoaded is a test-support seam that seeds the controller's
// resource cache directly, bypassing the task-result lane. It mirrors the
// task-result handler that runs when resources are fetched via the normal
// task lane. Use this only in tests — the same pattern as DrainSync.
//
// typeName is the canonical short name (e.g. "ec2", "s3").
// appendPage=true accumulates onto the existing cache page; false replaces it.
func (c *Controller) ApplyResourcesLoaded(typeName string, resources []resource.Resource, pagination *resource.PaginationMeta, appendPage bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	c.applyResourcesLoaded(ls, typeName, resources, pagination, appendPage)
}
