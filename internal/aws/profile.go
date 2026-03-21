package aws

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/ini.v1"
)

// DefaultConfigPath returns the default AWS config file path (~/.aws/config).
func DefaultConfigPath() string {
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
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	cfg, err := ini.LoadSources(ini.LoadOptions{
		Insensitive:  false,
		AllowShadows:  true,
		Loose:        true,
		InsensitiveKeys: true,
	}, path)
	if err != nil {
		return err
	}

	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" || name == "default" {
			seen["default"] = true
			continue
		}
		// Skip ini's built-in DEFAULT section if empty
		if name == ini.DefaultSection {
			continue
		}
		// Config file uses "profile <name>" prefix
		if strings.HasPrefix(name, "profile ") {
			profileName := strings.TrimPrefix(name, "profile ")
			if profileName != "" {
				seen[profileName] = true
			}
			continue
		}
		// Skip non-profile sections like "sso-session ..."
	}

	return nil
}

