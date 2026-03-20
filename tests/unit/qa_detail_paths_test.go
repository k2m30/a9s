package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/fieldpath"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/styles"
	"github.com/k2m30/a9s/internal/tui/views"
)

// TestDetailPaths_AllConfiguredFieldsRendered verifies that EVERY detail path
// from views.yaml appears in the rendered detail view for each resource type.
// Uses sanitized fixture data for all resource types.
// This catches: wrong field names in views.yaml, nil fields being skipped,
// and wrong ViewDef being selected.
func TestDetailPaths_AllConfiguredFieldsRendered(t *testing.T) {
	styles.Reinit() // ensure styles are initialized

	cfg, err := config.LoadFrom([]string{"/Users/k2m30/projects/a9s/.a9s/views.yaml"})
	if err != nil {
		t.Fatalf("failed to load views.yaml: %v", err)
	}

	k := keys.Default()

	// Map resource type to fixture functions
	allFixtures := map[string]func() []resource.Resource{
		"s3":         fixtureS3Buckets,
		"s3_objects": fixtureS3Objects,
		"ec2":        fixtureEC2Instances,
		"dbi":        fixtureRDSInstances,
		"redis":      fixtureRedisClusters,
		"dbc":        fixtureDocDBClusters,
		"eks":        fixtureEKSClusters,
		"secrets":    fixtureSecrets,
	}

	// Auto-discover all resource types plus s3_objects
	shortNames := resource.AllShortNames()
	shortNames = append(shortNames, "s3_objects")

	for _, shortName := range shortNames {
		t.Run(shortName, func(t *testing.T) {
			vd := config.GetViewDef(cfg, shortName)
			if len(vd.Detail) == 0 {
				t.Skipf("no detail paths configured for %s", shortName)
			}

			fixtureFn, ok := allFixtures[shortName]
			if !ok || fixtureFn == nil {
				t.Skipf("no fixture function for %s", shortName)
			}

			resources := fixtureFn()
			if len(resources) == 0 {
				t.Skipf("no fixture data for %s", shortName)
			}

			res := resources[0]

			// First: check that configured paths actually resolve against the struct
			if res.RawStruct != nil {
				for _, path := range vd.Detail {
					val := fieldpath.ExtractSubtree(res.RawStruct, path)
					t.Logf("  %s.%s = %q", shortName, path, truncate(val, 60))
				}
			}

			// Create detail model and render
			m := views.NewDetail(res, shortName, cfg, k)
			m.SetSize(120, 40)
			view := m.View()
			plain := stripANSI(view)

			// Every configured detail path should appear as a label in the view
			for _, path := range vd.Detail {
				// The path name (or a truncated version) should be visible.
				// PadOrTrunc receives "path:" (len+1), truncates to 22 with ellipsis.
				// So if len(path)+1 > 22 (i.e. len >= 22), the label gets truncated.
				label := path
				if len(label) >= 22 {
					label = label[:20] // PadOrTrunc truncates "path:" to 22 visible chars
				}
				if !strings.Contains(plain, label) {
					t.Errorf("detail view for %s missing field %q in output:\n%s",
						shortName, path, plain[:min(500, len(plain))])
				}
			}
		})
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
