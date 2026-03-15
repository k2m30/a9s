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

// DefaultCredentialsPath returns the default AWS credentials file path (~/.aws/credentials).
func DefaultCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".aws", "credentials")
	}
	return filepath.Join(home, ".aws", "credentials")
}

// ListProfiles reads the AWS config and credentials files, merges profile names,
// deduplicates, and returns them sorted. If a file does not exist, it is skipped
// without error.
func ListProfiles(configPath, credentialsPath string) ([]string, error) {
	seen := make(map[string]bool)

	// Parse config file
	if configPath != "" {
		if err := parseConfigProfiles(configPath, seen); err != nil {
			return nil, err
		}
	}

	// Parse credentials file
	if credentialsPath != "" {
		if err := parseCredentialsProfiles(credentialsPath, seen); err != nil {
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

// parseCredentialsProfiles reads profile names from an AWS credentials file.
// Bare section names are profile names.
func parseCredentialsProfiles(path string, seen map[string]bool) error {
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
		if name == ini.DefaultSection {
			continue
		}
		if name == "DEFAULT" || name == "default" {
			seen["default"] = true
			continue
		}
		if name != "" {
			seen[name] = true
		}
	}

	return nil
}
