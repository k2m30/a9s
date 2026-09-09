//go:build integration

// views7_config_report_test.go — the config report on the web lane.
//
// A view file whose column names a key nothing on the type writes renders a
// blank cell on every row, and the load says so. The terminal shows that
// report where it shows every other one, in the flash; the web lane wrote it
// to stderr, which an operator who started the server with & and closed the
// terminal never sees. One report, both lanes.
package webintegration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
)

// views7UnfillableKey names nothing any vpce fetcher or enricher writes.
const views7UnfillableKey = "att_stats"

// views7ViewsDir writes one view file into an isolated config folder and
// returns what the server is started with — the config a9s loads at start,
// and the report the load produced.
func views7ViewsDir(t *testing.T, shortName, body string) (*config.ViewsConfig, error) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", home)
	dir := filepath.Join(home, "views")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("creating %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, shortName+".yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s.yaml: %v", shortName, err)
	}
	return config.Load()
}

// views7Flash polls the rendered state for a flash, and returns the first one
// the page carries. A report the operator has to be looking at the right
// second to catch is not a report, so an empty flash after the budget is the
// failure the caller reports.
func views7Flash(t *testing.T, c *client) app.Flash {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if f := c.state(t).Header.Flash; f.Text != "" {
			return f
		}
		if time.Now().After(deadline) {
			return app.Flash{}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestWebLaneShowsTheConfigReport drives the server the way cmd/a9s does — the
// operator's config folder, config.Load, the server started with what it
// returned — and reads the report where the page shows the terminal's flash.
//
// If the report reaches the server by another route than the config it is
// started with, this call is the signature-only change that says so.
func TestWebLaneShowsTheConfigReport(t *testing.T) {
	viewCfg, cfgErr := views7ViewsDir(t, "vpce", `generated: 6
list:
  Endpoint ID:
    key: vpce_id
    width: 26
  Attachments:
    key: `+views7UnfillableKey+`
    width: 12

detail:
  - VpcEndpointId
`)
	if cfgErr == nil {
		t.Fatalf("the load reported nothing for a file whose column names %q — this test is about the "+
			"lane that shows the report, and there is none to show", views7UnfillableKey)
	}
	if viewCfg == nil {
		t.Fatal("the load returned no config — the operator's file is in use, not replaced")
	}

	c, cleanup := startServerWithConfig(t, viewCfg, config.ReportText(viewCfg, cfgErr))
	defer cleanup()

	flash := views7Flash(t, c)
	if flash.Text == "" {
		t.Fatalf("the page carries no flash after a start whose config load reported %v — the terminal "+
			"shows this report and the web lane wrote it to stderr, where an operator running the "+
			"server in the background never sees it", cfgErr)
	}
	if !flash.IsError {
		t.Errorf("the config report renders as an ordinary flash (%q) — it is an error, and the page "+
			"colours it like one", flash.Text)
	}
	if !strings.Contains(flash.Text, "vpce.yaml") || !strings.Contains(flash.Text, views7UnfillableKey) {
		t.Errorf("the page shows %q — the report names the file and the key on both lanes, so the "+
			"operator can find the line whichever one they are looking at", flash.Text)
	}
}

// TestWebLaneShowsNoReportForAGoodConfig is the negative half: a file whose
// every key names something on the type is not worth a word, and a page that
// opens with an error flash on a healthy config is a lane nobody will read
// twice.
func TestWebLaneShowsNoReportForAGoodConfig(t *testing.T) {
	viewCfg, cfgErr := views7ViewsDir(t, "vpce", `generated: 6
list:
  Endpoint ID:
    key: vpce_id
    width: 26

detail:
  - VpcEndpointId
`)
	if cfgErr != nil {
		t.Fatalf("loading a file whose every key names a field reported %v", cfgErr)
	}

	c, cleanup := startServerWithConfig(t, viewCfg, config.ReportText(viewCfg, cfgErr))
	defer cleanup()

	if f := c.state(t).Header.Flash; f.IsError {
		t.Errorf("the page opens with the error flash %q on a config that loaded cleanly", f.Text)
	}
}
