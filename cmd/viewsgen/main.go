// Command viewsgen generates per-resource YAML files under .a9s/views/.
// Usage: go run ./cmd/viewsgen/
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/k2m30/a9s/v3/internal/config"
)

func main() {
	cfg := config.DefaultConfig()

	outDir := filepath.Join(".a9s", "views")
	if err := os.MkdirAll(outDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "error creating %s: %v\n", outDir, err)
		os.Exit(1)
	}

	keys := make([]string, 0, len(cfg.Views))
	for k := range cfg.Views {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		v := cfg.Views[name]
		filePath := filepath.Join(outDir, name+".yaml")
		data := config.GenerateViewYAML(v)
		if err := os.WriteFile(filePath, data, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", filePath, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Generated %d files in %s/\n", len(keys), outDir)
}
