package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/buildinfo"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	version = buildinfo.ResolveVersion(version)
	commit = buildinfo.ResolveCommit(commit)
	date = buildinfo.ResolveDate(date)
}

func main() {
	var (
		profile     string
		region      string
		showVersion bool
		showHelp    bool
		demoMode    bool
	)

	flag.StringVar(&profile, "profile", "", "AWS profile to use")
	flag.StringVar(&profile, "p", "", "AWS profile to use (shorthand)")
	flag.StringVar(&region, "region", "", "AWS region override")
	flag.StringVar(&region, "r", "", "AWS region override (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.BoolVar(&showVersion, "v", false, "Print version and exit (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Print help and exit")
	flag.BoolVar(&showHelp, "h", false, "Print help and exit (shorthand)")
	flag.BoolVar(&demoMode, "demo", false, "Run with synthetic demo data (no AWS credentials needed)")
	flag.BoolVar(&demoMode, "d", false, "Run with synthetic demo data (shorthand)")

	flag.Usage = func() {
		fmt.Println("a9s - Terminal UI AWS Resource Manager")
		fmt.Printf("Version: %s\n\n", version)
		fmt.Println("Usage: a9s [flags]")
		fmt.Println("  -p, --profile  AWS profile to use")
		fmt.Println("  -r, --region   AWS region override")
		fmt.Println("  -d, --demo     Run with synthetic demo data (no AWS credentials needed)")
		fmt.Println("  -v, --version  Print version and exit")
		fmt.Println("  -h, --help     Print this help")
	}

	flag.Parse()

	if showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if showVersion {
		if commit != "none" && date != "unknown" {
			fmt.Printf("a9s %s (commit: %s, built: %s)\n", version, commit, date)
		} else {
			fmt.Printf("a9s %s\n", version)
		}
		os.Exit(0)
	}

	// Ensure config dir exists (non-fatal on failure)
	if _, err := config.EnsureConfigDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	if demoMode {
		if profile == "" {
			profile = demo.DemoProfile
		}
		if region == "" {
			region = demo.DemoRegion
		}
	}

	tui.Version = version

	model := tui.New(profile, region, tui.WithDemo(demoMode))

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
