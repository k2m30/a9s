// Command viewsgen generates per-resource YAML files under .a9s/views/.
// Usage: go run ./cmd/viewsgen/
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

		var b strings.Builder

		if len(v.List) > 0 {
			b.WriteString("list:\n")
			for _, col := range v.List {
				fmt.Fprintf(&b, "  %s:\n", yamlKey(col.Title))
				if col.Key != "" {
					fmt.Fprintf(&b, "    key: %s\n", col.Key)
				} else if col.Path != "" {
					fmt.Fprintf(&b, "    path: %s\n", col.Path)
				}
				fmt.Fprintf(&b, "    width: %d\n", col.Width)
			}
		}

		if len(v.Detail) > 0 {
			if len(v.List) > 0 {
				b.WriteString("\n")
			}
			b.WriteString("detail:\n")
			joined := strings.Join(v.Detail, ", ")
			if len(joined) < 80 {
				fmt.Fprintf(&b, "  [%s]\n", joined)
			} else {
				for _, d := range v.Detail {
					fmt.Fprintf(&b, "  - %s\n", d)
				}
			}
		}

		if err := os.WriteFile(filePath, []byte(b.String()), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", filePath, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Generated %d files in %s/\n", len(keys), outDir)
}

func yamlKey(s string) string {
	if strings.ContainsAny(s, "#:{}[]&*?|>!%@`") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
