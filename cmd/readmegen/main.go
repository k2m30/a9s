// Command readmegen generates README.md from docs/README.tmpl.md + docs/shared/ snippets.
// Usage: go run ./cmd/readmegen/ > README.md
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var includeRe = regexp.MustCompile(`^<!-- INCLUDE: (.+) -->$`)

func main() {
	tmpl, err := os.ReadFile("docs/README.tmpl.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading template: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(string(tmpl), "\n")
	var out []string

	for _, line := range lines {
		m := includeRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			out = append(out, line)
			continue
		}

		path := filepath.Join("docs", "shared", m[1])
		content, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
			os.Exit(1)
		}

		out = append(out, strings.TrimRight(string(content), "\n"))
	}

	fmt.Print(strings.Join(out, "\n"))
}
