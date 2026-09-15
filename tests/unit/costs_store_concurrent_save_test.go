// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/core/costs"
)

// Two sessions on one profile save the same cache file at once; every save
// lands and no half-written file is left behind.
func TestCostsStore_ConcurrentSavesDoNotCollide(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)
	const profile = "example-readonly"
	a, b := costs.LoadStore(profile), costs.LoadStore(profile)

	var wg sync.WaitGroup
	errs := make(chan error, 200)
	for _, s := range []*costs.Store{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				if err := s.Save(); err != nil {
					errs <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("save: %v", err)
	}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.Contains(d.Name(), ".tmp") {
			t.Errorf("a temp file was left behind: %s", path)
		}
		return nil
	})
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}
