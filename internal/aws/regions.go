package aws

import (
	"os"
	"strings"

	"gopkg.in/ini.v1"
)

// AWSRegion represents an AWS region with its code and human-readable display name.
type AWSRegion struct {
	Code        string
	DisplayName string
}

// AllRegions returns a hardcoded list of all current AWS regions.
func AllRegions() []AWSRegion {
	return []AWSRegion{
		{Code: "us-east-1", DisplayName: "US East (N. Virginia)"},
		{Code: "us-east-2", DisplayName: "US East (Ohio)"},
		{Code: "us-west-1", DisplayName: "US West (N. California)"},
		{Code: "us-west-2", DisplayName: "US West (Oregon)"},
		{Code: "af-south-1", DisplayName: "Africa (Cape Town)"},
		{Code: "ap-east-1", DisplayName: "Asia Pacific (Hong Kong)"},
		{Code: "ap-south-1", DisplayName: "Asia Pacific (Mumbai)"},
		{Code: "ap-south-2", DisplayName: "Asia Pacific (Hyderabad)"},
		{Code: "ap-southeast-1", DisplayName: "Asia Pacific (Singapore)"},
		{Code: "ap-southeast-2", DisplayName: "Asia Pacific (Sydney)"},
		{Code: "ap-southeast-3", DisplayName: "Asia Pacific (Jakarta)"},
		{Code: "ap-northeast-1", DisplayName: "Asia Pacific (Tokyo)"},
		{Code: "ap-northeast-2", DisplayName: "Asia Pacific (Seoul)"},
		{Code: "ap-northeast-3", DisplayName: "Asia Pacific (Osaka)"},
		{Code: "ca-central-1", DisplayName: "Canada (Central)"},
		{Code: "eu-central-1", DisplayName: "Europe (Frankfurt)"},
		{Code: "eu-central-2", DisplayName: "Europe (Zurich)"},
		{Code: "eu-west-1", DisplayName: "Europe (Ireland)"},
		{Code: "eu-west-2", DisplayName: "Europe (London)"},
		{Code: "eu-west-3", DisplayName: "Europe (Paris)"},
		{Code: "eu-north-1", DisplayName: "Europe (Stockholm)"},
		{Code: "eu-south-1", DisplayName: "Europe (Milan)"},
		{Code: "eu-south-2", DisplayName: "Europe (Spain)"},
		{Code: "me-south-1", DisplayName: "Middle East (Bahrain)"},
		{Code: "me-central-1", DisplayName: "Middle East (UAE)"},
		{Code: "sa-east-1", DisplayName: "South America (Sao Paulo)"},
		{Code: "il-central-1", DisplayName: "Israel (Tel Aviv)"},
	}
}

// GetDefaultRegion reads the region configured for a given profile in the AWS config file.
// If the config file doesn't exist or the profile has no region, it falls back to "us-east-1".
func GetDefaultRegion(configPath, profile string) string {
	const fallback = "us-east-1"

	if configPath == "" {
		return fallback
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fallback
	}

	cfg, err := ini.LoadSources(ini.LoadOptions{
		Insensitive:     false,
		AllowShadows:     true,
		Loose:           true,
		InsensitiveKeys: true,
	}, configPath)
	if err != nil {
		return fallback
	}

	// Determine the section name to look up
	sectionName := "profile " + profile
	if profile == "default" || profile == "" {
		sectionName = "default"
	}

	section, err := cfg.GetSection(sectionName)
	if err != nil {
		// Try without "profile " prefix for default
		if profile == "default" || profile == "" {
			section, err = cfg.GetSection("DEFAULT")
			if err != nil {
				return fallback
			}
		} else {
			return fallback
		}
	}

	if section.HasKey("region") {
		region := strings.TrimSpace(section.Key("region").String())
		if region != "" {
			return region
		}
	}

	return fallback
}
