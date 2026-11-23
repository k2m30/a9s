// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"fmt"
	"sync"

	"golang.org/x/tools/go/packages"
)

var LoadPackagesOnce = loadPackagesOnce

var packageLoads sync.Map

// loadPackagesOnce is packages.Load for the source guards. A load type-checks
// the whole tree it names, seconds under -race, and every guard only reads
// the syntax and types it returns, so one load per distinct request serves
// every guard in the binary.
func loadPackagesOnce(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
	key := fmt.Sprint(cfg.Mode, "|", cfg.Dir, "|", cfg.Tests, "|", cfg.BuildFlags, "|", patterns)
	load, _ := packageLoads.LoadOrStore(key, sync.OnceValues(func() ([]*packages.Package, error) {
		return packages.Load(cfg, patterns...)
	}))
	return load.(func() ([]*packages.Package, error))()
}
