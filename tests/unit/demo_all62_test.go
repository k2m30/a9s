package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/internal/aws"
	demo "github.com/k2m30/a9s/internal/demo"
	"github.com/k2m30/a9s/internal/resource"
)

func TestDemoFixtures_All62Types(t *testing.T) {
	var missing []string
	for _, rt := range resource.AllResourceTypes() {
		resources, ok := demo.GetResources(rt.ShortName)
		if !ok || len(resources) == 0 {
			missing = append(missing, rt.ShortName)
		}
	}
	if len(missing) > 0 {
		t.Errorf("missing demo fixtures for %d types: %v", len(missing), missing)
	}
}
