package unit_test

import (
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestDetailPaths_AllConfiguredFieldsRendered verifies that EVERY detail path
// from views.yaml appears in the rendered detail view for each resource type.
// Uses realistic SDK struct fixtures for all resource types.
// This catches: wrong field names in views.yaml, nil fields being skipped,
// and wrong ViewDef being selected.
//
// Retargeted (wave3 detail-family cleanup round 4, specs/022-codebase-cleanup)
// off views.NewDetail(...).View() (dead: DetailModel.View/SetSize) onto the
// live Controller.Snapshot().Body.Detail + NewTransientDetail.RenderDetail
// seam — same golden infrastructure as wave3_detail_ports_test.go.
func TestDetailPaths_AllConfiguredFieldsRendered(t *testing.T) {
	styles.Reinit() // ensure styles are initialized

	cfg, err := config.LoadFromDirs([]string{filepath.Join("..", "..", ".a9s", "views")})
	if err != nil {
		t.Fatalf("failed to load views dir: %v", err)
	}
	if cfg == nil {
		t.Fatalf(".a9s/views/ directory not found or returned nil config")
	}

	// Perf: test 4 representative resource types to keep this test fast.
	// Chosen: ec2 (most complex, nested fields), s3 (simple bucket), lambda
	// (function), redis (ReplicationGroup — pins the post-spec-rewrite RawStruct
	// shape so a regression back to CacheCluster would surface here).
	// Full coverage is exercised by individual per-type detail tests in qa_detail_*_test.go.
	allFixtures := map[string]resource.Resource{
		"ec2":    buildResource("i-0abcdef1234567890", "web-server-prod", realisticEC2Instance()),
		"s3":     buildResource("my-production-bucket", "my-production-bucket", realisticS3Bucket()),
		"lambda": buildResource("my-api-handler", "my-api-handler", realisticLambdaFunction()),
		"redis":  buildResource("redis-prod-001", "redis-prod-001", realisticRedisReplicationGroup()),
	}
	shortNames := []string{"ec2", "s3", "lambda", "redis"}

	for _, shortName := range shortNames {
		t.Run(shortName, func(t *testing.T) {
			vd := config.GetViewDef(cfg, shortName)
			if len(vd.Detail) == 0 {
				t.Skipf("no detail paths configured for %s", shortName)
			}

			res, ok := allFixtures[shortName]
			if !ok {
				t.Skipf("no fixture for %s", shortName)
			}

			// First: check that configured paths actually resolve against the struct
			if res.RawStruct != nil {
				for _, df := range vd.Detail {
					val := fieldpath.ExtractSubtree(res.RawStruct, df.String())
					t.Logf("  %s.%s = %q", shortName, df.String(), truncateStr(val, 60))
				}
			}

			// Build the detail body through the live controller seam and render it
			// via the same NewTransientDetail+RenderDetail path production uses.
			c := newDetailController(t, res, shortName)
			c.SetViewConfig(cfg)
			body := c.Snapshot().Body.Detail
			if body == nil {
				t.Fatalf("Body.Detail is nil for %s", shortName)
			}

			vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(40))
			m := views.NewTransientDetail(120, 40, vp)
			plain := stripAnsi(m.RenderDetail(*body))

			// Every configured detail path should appear as a label in the view
			for _, df := range vd.Detail {
				// The path name (or a truncated version) should be visible.
				// PadOrTrunc receives "path:" (len+1), truncates to 22 with ellipsis.
				// So if len(path)+1 > 22 (i.e. len >= 22), the label gets truncated.
				label := df.DisplayLabel()
				if len(label) >= 22 {
					label = label[:20] // PadOrTrunc truncates "path:" to 22 visible chars
				}
				if !strings.Contains(plain, label) {
					t.Errorf("detail view for %s missing field %q in output:\n%s",
						shortName, df.String(), plain[:min(500, len(plain))])
				}
			}
		})
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
