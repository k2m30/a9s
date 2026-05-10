package resource

import (
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

func init() {
	// Register the CloudTrail Events detail projector into the catalog.
	// The projector lives in internal/semantics/ctevent, which internal/catalog
	// cannot import without creating a cycle. This init() in internal/resource
	// (which may import both) performs the wiring.
	catalog.RegisterProject("ct-events", ctevent.Project)
}
