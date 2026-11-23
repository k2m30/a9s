// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// blessed_construction_test.go — the one constructor tests build a controller
// or a root model through, and the check that nothing it started outlives
// the test.
//
// A controller starts its cache writer on the first Apply/Handle and keeps
// it until Close. A writer that outlives its test runs process-wide hooks
// (cache.SetEncodeHookForTest) and the live A9S_CONFIG_FOLDER of whichever
// test is running when the scheduler gets to it. Pairing the close with
// t.Cleanup here, at the one construction point, is what makes the ordering
// correct everywhere: t.Cleanup is LIFO, the caller resolves its temp
// directory before it constructs, so the close registered here always runs
// before that directory is removed.
//
// The test goroutine is labelled with the test's name before it constructs.
// Goroutines inherit labels from the goroutine that starts them, so every
// goroutine a9s code starts on behalf of this test carries the name, and the
// check after the last close finds any still alive by it.
package unit

import (
	"bytes"
	"context"
	"fmt"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// The exported names are package unit_test's handles on the same
// constructors: both packages build into one binary and share one leak check.
var (
	NewBlessedController = newBlessedController
	NewBlessedModel      = newBlessedModel
	NewSizedModel        = newSizedModel
	NewTestController    = newTestController
)

// newBlessedController builds an app.Controller over core and closes it when
// the test ends.
func newBlessedController(t testing.TB, core *runtime.Core) *app.Controller {
	t.Helper()
	guardTestGoroutines(t)
	c := app.New(core)
	t.Cleanup(c.Close)
	return c
}

// A9S_CONFIG_FOLDER is redirected before the controller is built, so the
// close newBlessedController registers runs before that directory's RemoveAll
// (t.Cleanup is LIFO; app_availsave_tempdir_cleanup_race_test.go has the
// mechanism).
func newTestController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	return newBlessedController(t, runtime.New(s, nil))
}

// newBlessedModel builds a root tui.Model and releases what it owns when the
// test ends: the app context its fetches ride on, and the controller whose
// cache writer would otherwise outlive the test.
func newBlessedModel(t testing.TB, profile, region string, opts ...tui.Option) tui.Model {
	t.Helper()
	guardTestGoroutines(t)
	m := tui.New(profile, region, opts...)
	t.Cleanup(func() {
		m.Cancel()
		m.CloseController()
	})
	return m
}

func newSizedModel(t testing.TB, profile, region string, opts ...tui.Option) tui.Model {
	t.Helper()
	m, _ := tuitest.Step(newBlessedModel(t, profile, region, opts...), tea.WindowSizeMsg{Width: 80, Height: 40})
	return m
}

const testGoroutineLabel = "a9s-test"

var guardedTests sync.Map

// guardTestGoroutines labels the calling goroutine with t's name and, on the
// first call for t, registers the leak check. Registered before any close,
// it runs after all of them.
func guardTestGoroutines(t testing.TB) {
	t.Helper()
	pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), pprof.Labels(testGoroutineLabel, t.Name())))
	if _, seen := guardedTests.LoadOrStore(t, true); seen {
		return
	}
	t.Cleanup(func() {
		guardedTests.Delete(t)
		if leaked := awaitNoProductionGoroutines(t.Name()); leaked != "" {
			t.Errorf("goroutines a9s code started for this test are still running after its controllers closed:\n%s", leaked)
		}
	})
}

// awaitNoProductionGoroutines returns the goroutine-profile records of a9s
// production goroutines labelled with test, or every unlabelled-or-labelled
// one when test is "". A goroutine whose Close has just returned can still be
// unwinding, so a record is reported only if it persists for a second.
func awaitNoProductionGoroutines(test string) string {
	deadline := time.Now().Add(time.Second)
	for {
		leaked := productionGoroutines(test)
		if leaked == "" || time.Now().After(deadline) {
			return leaked
		}
		time.Sleep(time.Millisecond)
	}
}

// productionGoroutines reads the goroutine profile and keeps the records
// whose root frame is a9s production code (core/ or internal/), so the test
// goroutines themselves and the runtime's own never match.
func productionGoroutines(test string) string {
	var buf bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 1); err != nil {
		return "goroutine profile: " + err.Error()
	}
	want := strconv.Quote(testGoroutineLabel) + ":" + strconv.Quote(test)
	_, records, _ := strings.Cut(buf.String(), "\n")
	var out []string
	for _, rec := range strings.Split(records, "\n\n") {
		if test != "" && !strings.Contains(rec, want) {
			continue
		}
		lines := strings.Split(strings.TrimSpace(rec), "\n")
		root := lines[len(lines)-1]
		if !strings.HasPrefix(root, "#\t") {
			continue
		}
		if strings.Contains(root, "/a9s/v3/core/") || strings.Contains(root, "/a9s/v3/internal/") {
			out = append(out, rec)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return fmt.Sprintf("%d record(s):\n%s", len(out), strings.Join(out, "\n\n"))
}

// CheckNoProductionGoroutines is the end-of-run form of the check, for
// TestMain: it catches a goroutine started by a controller built anywhere,
// labelled or not.
func CheckNoProductionGoroutines() string {
	return awaitNoProductionGoroutines("")
}
