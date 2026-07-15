package aws

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultConfigPath returns the AWS config file path.
// Honors $AWS_CONFIG_FILE if set; falls back to ~/.aws/config.
func DefaultConfigPath() string {
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".aws", "config")
	}
	return filepath.Join(home, ".aws", "config")
}

// ListProfiles reads AWS config file profile names, matching `aws configure list-profiles`.
// Only [profile xxx] sections from ~/.aws/config are included.
// a9s never reads ~/.aws/credentials — credential handling is delegated entirely to the AWS SDK.
func ListProfiles(configPath string) ([]string, error) {
	seen := make(map[string]bool)

	// Parse config file only — matches `aws configure list-profiles` behavior
	if configPath != "" {
		if err := parseConfigProfiles(configPath, seen); err != nil {
			return nil, err
		}
	}

	// Collect and sort
	profiles := make([]string, 0, len(seen))
	for p := range seen {
		profiles = append(profiles, p)
	}
	sort.Strings(profiles)

	return profiles, nil
}

// parseConfigProfiles reads profile names from an AWS config file.
// Sections prefixed with "profile " have the prefix stripped.
// The "default" or "DEFAULT" section maps to "default".
func parseConfigProfiles(path string, seen map[string]bool) error {
	sections, err := parseINISections(path)
	if err != nil {
		return err
	}

	for _, section := range sections {
		name := section.name
		if name == "DEFAULT" || name == "default" {
			seen["default"] = true
			continue
		}
		// Config file uses "profile <name>" prefix
		if after, ok := strings.CutPrefix(name, "profile "); ok {
			profileName := after
			if profileName != "" {
				seen[profileName] = true
			}
			continue
		}
		// Skip non-profile sections like "sso-session ..."
	}

	return nil
}
