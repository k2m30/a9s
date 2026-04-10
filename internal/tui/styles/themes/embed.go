package themes

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed *.yaml
var FS embed.FS

// EnsureThemesDir writes any missing built-in theme YAML files to dir.
// Existing files are never overwritten (user may have edited them).
func EnsureThemesDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dest := filepath.Join(dir, e.Name())
		if _, statErr := os.Stat(dest); statErr == nil {
			continue // file exists, skip
		}
		data, readErr := fs.ReadFile(FS, e.Name())
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(dest, data, 0644); writeErr != nil { //nolint:gosec // theme YAML files are non-sensitive, world-readable is acceptable
			return writeErr
		}
	}
	return nil
}
