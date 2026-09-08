// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui5_failure_and_provenance_test.go — two seams that phrased something in
// their own words. The profile selector flashed a Go error chain verbatim
// when the local AWS config could not be read; the detail controller stamped
// a Wave-2 finding with a provenance shaped exactly like the
// "wave2:<short-name>" the registry stamps, for a short name no registry
// entry has. The sentinel's own shape is pinned in-package
// (core/app/tui5_wave2_sentinel_test.go); this file holds the flash and the
// gate over every production file.
package unit_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestProfileSelector_LocalConfigFailureIsPhrased covers both ways the local
// AWS config can refuse to yield profiles — a path that cannot be read at all,
// and a file that names none — and asserts the flash phrases the failure as a
// local-file cause instead of handing over the error chain's own text.
func TestProfileSelector_LocalConfigFailureIsPhrased(t *testing.T) {
	cases := []struct {
		name string
		path func(t *testing.T) string
	}{
		{"unreadable", func(t *testing.T) string {
			// A directory where a file is expected: os.ReadFile refuses it,
			// which is the read failure w122 flashed verbatim.
			return t.TempDir()
		}},
		{"no profiles", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "config")
			if err := os.WriteFile(p, []byte("# nothing here\n"), 0600); err != nil {
				t.Fatalf("seeding config: %v", err)
			}
			return p
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AWS_CONFIG_FILE", tc.path(t))

			c := newTestController(t)
			vs, _ := c.OpenProfileSelector()

			text := vs.Header.Flash.Text
			if !vs.Header.Flash.IsError {
				t.Fatalf("flash is not an error: %q", text)
			}
			if !strings.HasPrefix(text, "local AWS config: ") {
				t.Errorf("flash = %q, want it phrased as a local AWS config failure", text)
			}
			if strings.TrimSpace(strings.TrimPrefix(text, "local AWS config:")) == "" {
				t.Errorf("flash = %q names the file but no cause", text)
			}
		})
	}
}

// wave2SourceLiteral matches a spelled-out Wave-2 provenance — "wave2:" with
// something after it. The bare prefix, which the readers test against, is not
// a provenance and does not match.
var wave2SourceLiteral = regexp.MustCompile(`"wave2:[^"]+"`)

// wave2SourceStampSeams are the only two production files allowed to spell a
// Wave-2 provenance out: the runtime's registry-driven stamp, which is the
// one place that knows which type's enricher produced a finding, and the
// detail controller's sentinel for a finding that arrived without one.
var wave2SourceStampSeams = map[string]bool{
	"core/runtime/helpers.go":  true,
	"core/app/detail_state.go": true,
}

// TestFindingSourceIsStampedAtTwoSeamsOnly is the gate. A third site spelling
// a provenance out would be a second place deciding what a finding's Source
// says, and the two could disagree about a type's short name without any
// reader noticing.
func TestFindingSourceIsStampedAtTwoSeamsOnly(t *testing.T) {
	var offenders []string
	for _, dir := range []string{"../../core", "../../internal", "../../cmd"} {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel := strings.TrimPrefix(filepath.ToSlash(path), "../../")
			if wave2SourceStampSeams[rel] {
				return nil
			}
			data, readErr := os.ReadFile(path) //nolint:gosec // path comes from this walk of the repo's own tree
			if readErr != nil {
				return readErr
			}
			for i, line := range strings.Split(string(data), "\n") {
				code, _, _ := strings.Cut(line, "//")
				if wave2SourceLiteral.MatchString(code) {
					offenders = append(offenders, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("a Wave-2 provenance is spelled out outside the two stamping seams:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}
