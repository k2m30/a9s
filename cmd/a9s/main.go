package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/buildinfo"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/styles/themes"
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
		noCache     bool
		command     string
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
	flag.BoolVar(&noCache, "no-cache", false, "Disable resource availability cache")
	flag.StringVar(&command, "command", "", "Resource type to open directly (e.g. ec2, s3, events)")
	flag.StringVar(&command, "c", "", "Resource type to open directly (shorthand)")

	flag.Usage = func() {
		fmt.Println("a9s - Terminal UI AWS Resource Manager")
		fmt.Printf("Version: %s\n\n", version)
		fmt.Println("Usage: a9s [flags]")
		fmt.Println("  -p, --profile  AWS profile to use")
		fmt.Println("  -r, --region   AWS region override")
		fmt.Println("  -d, --demo     Run with synthetic demo data (no AWS credentials needed)")
		fmt.Println("      --no-cache Disable resource availability cache")
		fmt.Println("  -c, --command  Open directly to a resource list (e.g. ec2, s3, events)")
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

	// Validate -c/--command flag: resolve to a canonical resource short name early
	// so invalid input fails fast before the TUI starts.
	var resolvedCommand string
	if command != "" {
		rt := resource.FindResourceType(command)
		if rt == nil {
			fmt.Fprintf(os.Stderr, "Error: unknown resource type: %s\n", command)
			os.Exit(1)
		}
		resolvedCommand = rt.ShortName
	}

	// Ensure config dir exists (non-fatal on failure)
	if _, err := config.EnsureConfigDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Load and apply user theme from config.yaml.
	// This runs before the TUI starts — synchronous, no race with rendering.
	activeTheme := "tokyo-night.yaml"
	if cfgDir := config.ConfigDir(); cfgDir != "" {
		if err := themes.EnsureThemesDir(filepath.Join(cfgDir, "themes")); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: themes directory: %v\n", err)
		}
		if appCfg, appErr := config.LoadAppConfig(); appErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: config.yaml: %v\n", appErr)
		} else if appCfg.Theme != "" {
			themePath, pathErr := config.ThemePath(appCfg.Theme)
			if pathErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: theme %q: %v\n", appCfg.Theme, pathErr)
			} else if data, readErr := os.ReadFile(themePath); readErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: theme %q: %v\n", appCfg.Theme, readErr)
			} else if t, parseErr := styles.ThemeFromYAML(data); parseErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: theme %q: %v\n", appCfg.Theme, parseErr)
			} else {
				styles.ApplyTheme(t)
				activeTheme = appCfg.Theme
			}
		}
	}

	var extraOpts []tui.Option
	if demoMode {
		if profile == "" {
			profile = demo.DemoProfile
		}
		if region == "" {
			region = demo.DemoRegion
		}
		extraOpts = append(extraOpts, tui.WithClients(demo.NewServiceClients()), tui.WithNoCache(true))
	} else if noCache {
		extraOpts = append(extraOpts, tui.WithNoCache(true))
	}

	if resolvedCommand != "" {
		extraOpts = append(extraOpts, tui.WithCommand(resolvedCommand))
	}

	tui.Version = version

	if err := runProgram(profile, region, extraOpts, activeTheme); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runProgram constructs the model, starts the Bubble Tea program, and guarantees
// the app context is cancelled on any exit path (normal return, error, panic).
// Separated from main() so that `defer model.Cancel()` runs before os.Exit —
// deferred functions don't fire on os.Exit, so that call must live above it.
func runProgram(profile, region string, extraOpts []tui.Option, activeTheme string) error {
	model := tui.New(profile, region, append(extraOpts, tui.WithActiveTheme(activeTheme))...)
	defer model.Cancel()

	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}
