// Command viewsgen generates a complete .a9s/views.yaml from built-in defaults.
// Usage: go run ./cmd/viewsgen/ > .a9s/views.yaml
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/k2m30/a9s/v3/internal/config"
)

func main() {
	cfg := config.DefaultConfig()

	// Sort keys for stable output
	keys := make([]string, 0, len(cfg.Views))
	for k := range cfg.Views {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Println("# a9s development views.yaml")
	fmt.Println("# Auto-generated from built-in defaults — DO NOT EDIT MANUALLY.")
	fmt.Println("# Regenerate: go run ./cmd/viewsgen/ > .a9s/views.yaml")
	fmt.Println("# To customize: create ~/.a9s/views.yaml with ONLY the resources you want to override.")
	fmt.Println("# Generate field reference: go run ./cmd/refgen/ > .a9s/views_reference.yaml")
	fmt.Println()
	fmt.Println("views:")

	for i, name := range keys {
		v := cfg.Views[name]
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("  %s:\n", name)

		if len(v.List) > 0 {
			fmt.Println("    list:")
			for _, col := range v.List {
				fmt.Printf("      %s:\n", col.Title)
				if col.Key != "" {
					fmt.Printf("        key: %s\n", col.Key)
				} else if col.Path != "" {
					fmt.Printf("        path: %s\n", col.Path)
				}
				fmt.Printf("        width: %d\n", col.Width)
			}
		}

		if len(v.Detail) > 0 {
			fmt.Println("    detail:")
			// Print as flow style if it fits, otherwise block
			joined := strings.Join(v.Detail, ", ")
			if len(joined) < 80 {
				fmt.Printf("      [%s]\n", joined)
			} else {
				for _, d := range v.Detail {
					fmt.Printf("      - %s\n", d)
				}
			}
		}
	}

	os.Exit(0)
}
