package aws

import (
	"bufio"
	"os"
	"strings"
)

// iniSection is one `[name]` block from an AWS-style config/credentials
// file: the section name (brackets stripped) plus its `key = value` pairs.
type iniSection struct {
	name string
	keys map[string]string
}

// parseINISections does a flat, single-pass scan of an AWS config/
// credentials file — the minimal subset those files use (no nesting, no
// includes, no quoting, no inline comments). A missing file returns
// (nil, nil): matching the SDK's own "absent config file is not an error"
// behavior, so callers don't need a separate os.Stat guard.
func parseINISections(path string) ([]iniSection, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var sections []iniSection
	var cur *iniSection
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections = append(sections, iniSection{
				name: strings.TrimSpace(line[1 : len(line)-1]),
				keys: make(map[string]string),
			})
			cur = &sections[len(sections)-1]
			continue
		}
		if cur == nil {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cur.keys[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return sections, nil
}

// findINISection returns the first section with the given name.
func findINISection(sections []iniSection, name string) (iniSection, bool) {
	for _, s := range sections {
		if s.name == name {
			return s, true
		}
	}
	return iniSection{}, false
}
