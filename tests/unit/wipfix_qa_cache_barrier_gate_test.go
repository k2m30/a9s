// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_cache_barrier_gate_test.go enforces one rule across the suite: a
// test that drives a save through the controller and then reads the file back
// off disk waits on the write barrier in between.
//
// The saves are queued and land on a writer goroutine, so without the wait
// the test is racing the queue. Two replay round-trip tests failed that way
// about once in three full-suite runs, and they had looked safe only because
// the seam they used happened to wait internally — until it stopped. A test
// that states its own barrier cannot be broken by a seam's internals again.
package unit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The three vocabularies the rule is written in. Named here rather than
// inline so the failure message can quote them.
var (
	// wipfixBarrierSaves drive a save through the controller's queue.
	wipfixBarrierSaves = []string{"ApplyResourcesLoaded(", ".Handle(messages.ResourcesLoaded"}
	// wipfixBarrierReads read a pair's type files back off disk.
	wipfixBarrierReads = []string{"LoadDirForTest("}
	// wipfixBarrier is the wait itself.
	wipfixBarrier = "WaitForCacheWrites()"
)

// TestCacheReadsWaitOnTheWriteBarrier walks every test in tests/ and reports
// each disk read that follows a queued save with no barrier between them.
func TestCacheReadsWaitOnTheWriteBarrier(t *testing.T) {
	var violations []string
	for _, dir := range []string{"tests/unit", "tests/integration"} {
		root := filepath.Join("..", "..", dir)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return err
			}
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			rel := dir + "/" + filepath.Base(path)
			violations = append(violations, wipfixUnbarrieredReads(rel, strings.Split(string(b), "\n"))...)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	if len(violations) > 0 {
		t.Errorf("%d disk read(s) race the cache write queue — call WaitForCacheWrites() on the "+
			"controller between the save and the read:\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// wipfixUnbarrieredReads reports every read line whose nearest preceding save
// is more recent than the nearest preceding barrier. Scoped per top-level
// function, so one test's barrier never covers the next one's read.
func wipfixUnbarrieredReads(rel string, lines []string) []string {
	var out []string
	lastSave, lastBarrier := -1, -1
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "func "):
			lastSave, lastBarrier = -1, -1
		case strings.HasPrefix(strings.TrimSpace(line), "//"):
			continue
		}
		code := wipfixWithoutStringLiterals(line)
		if wipfixContainsAny(code, wipfixBarrierSaves) {
			lastSave = i
		}
		if strings.Contains(code, wipfixBarrier) {
			lastBarrier = i
		}
		if wipfixContainsAny(code, wipfixBarrierReads) && lastSave > lastBarrier {
			out = append(out, rel+":"+strconv.Itoa(i+1)+"  "+strings.TrimSpace(line)+
				"   (save at line "+strconv.Itoa(lastSave+1)+")")
		}
	}
	return out
}

// wipfixWithoutStringLiterals blanks out double-quoted spans, so a failure
// message that quotes one of the expressions is not read as code — and
// neither are this gate's own vocabularies.
func wipfixWithoutStringLiterals(line string) string {
	var b strings.Builder
	inString := false
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '"' && (i == 0 || line[i-1] != '\\'):
			inString = !inString
		case !inString:
			b.WriteByte(line[i])
		}
	}
	return b.String()
}

func wipfixContainsAny(line string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(line, n) {
			return true
		}
	}
	return false
}
