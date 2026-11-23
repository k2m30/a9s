// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// smokeCountLiteral matches a list or menu title with its count typed in:
// "lambda(44)", "logs(182+)", the total of "s3(7/42)", or a filtered count
// other than 1. The demo fixtures decide those numbers; a smoke reads them
// from demo_count (scripts/smoke-lib.sh). The 1 of "s3(1/…)" is select_row's
// own contract — a filter on a row's unique name leaves that one row.
var smokeCountLiteral = regexp.MustCompile(`[a-z0-9_-]\([0-9]+\+?\)|[a-z0-9_-]\(([02-9]|[0-9]{2,})/|[a-z0-9_-]\([0-9]+/[0-9]+`)

// smokeSharedHelper matches a definition of a helper scripts/smoke-lib.sh
// owns, so a second copy cannot drift from it.
var smokeSharedHelper = regexp.MustCompile(`(?m)^(page_until|wait_for|demo_count|select_row)\(\)`)

func TestSmokeScripts_CarryNoCountTheDemoDecides(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join("..", "..", "scripts", "smoke-*.sh"))
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no smoke scripts found under scripts/ (err=%v)", err)
	}
	for _, path := range scripts {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		for i, line := range strings.Split(string(src), "\n") {
			if m := smokeCountLiteral.FindString(line); m != "" {
				t.Errorf("%s:%d types the count %q; read it with demo_count", name, i+1, m)
			}
		}
		if name == "smoke-lib.sh" {
			continue
		}
		for _, m := range smokeSharedHelper.FindAllString(string(src), -1) {
			t.Errorf("%s defines its own %s; scripts/smoke-lib.sh owns it", name, m)
		}
	}
}
