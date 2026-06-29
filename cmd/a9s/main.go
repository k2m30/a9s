package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/buildinfo"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/styles/themes"
	"github.com/k2m30/a9s/v3/internal/web"
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

// renameHints maps deprecated resource aliases to their current short names.
// Breaking renames introduced in commits 4b5175b/2ac417f (dbc-snap) and the
// corresponding dbi-snap rename from rds-snap.
var renameHints = map[string]string{
	"rds-snap":   "dbi-snap",
	"docdb-snap": "dbc-snap",
}

func main() {
	// Install the AWS catalog into internal/catalog before any code path can
	// hit catalog.Find / catalog.All. Must run before resource.FindResourceType
	// below and before any tui.New construction — both transitively call
	// catalog accessors that panic when SetTypes has not yet been invoked.
	aws.Install()
	// Wire resource-registry callbacks into the projection layer. Replaces
	// the legacy internal/resource init() per AS-731 exit criterion (zero
	// init() in internal/resource/). Must run after aws.Install so callbacks
	// resolve catalog-backed defaults.
	resource.WireProjection()

	var (
		profile      string
		region       string
		showVersion  bool
		showHelp     bool
		demoMode     bool
		noCache      bool
		command      string
		resetViews   bool
		resetThemes  bool
		webMode      bool
		webAddr      string
		webAllowReveal bool
	)

	// A9S_MODE=web activates the web server without requiring --web on the CLI.
	if os.Getenv("A9S_MODE") == "web" {
		webMode = true
	}

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
	flag.BoolVar(&resetViews, "reset-views", false, "Delete all view configs; defaults recreated on next launch")
	flag.BoolVar(&resetThemes, "reset-themes", false, "Delete all theme files; defaults recreated on next launch")
	flag.BoolVar(&webMode, "web", webMode, "Run as HTTP server instead of TUI (also: A9S_MODE=web)")
	flag.StringVar(&webAddr, "web-addr", "127.0.0.1:7682", "Listen address for web mode (127.0.0.1 only)")
	flag.BoolVar(&webAllowReveal, "web-allow-reveal", false, "Allow ActionReveal in web mode (off by default)")

	flag.Usage = func() {
		fmt.Println("a9s - Terminal UI AWS Resource Manager")
		fmt.Printf("Version: %s\n\n", version)
		fmt.Println("Usage: a9s [flags]")
		fmt.Println("  -p, --profile         AWS profile to use")
		fmt.Println("  -r, --region          AWS region override")
		fmt.Println("  -d, --demo            Run with synthetic demo data (no AWS credentials needed)")
		fmt.Println("      --no-cache        Disable resource availability cache")
		fmt.Println("  -c, --command         Open directly to a resource list (e.g. ec2, s3, events)")
		fmt.Println("      --reset-views     Delete view configs; defaults recreated on next launch")
		fmt.Println("      --reset-themes    Delete theme files; defaults recreated on next launch")
		fmt.Println("      --web             Run as HTTP server (also: A9S_MODE=web)")
		fmt.Println("      --web-addr        Listen address for web mode (default 127.0.0.1:7682)")
		fmt.Println("      --web-allow-reveal Allow secret reveal in web mode (default off)")
		fmt.Println("  -v, --version         Print version and exit")
		fmt.Println("  -h, --help            Print this help")
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

	if resetViews || resetThemes {
		cfgDir := config.ConfigDir()
		if cfgDir == "" {
			fmt.Fprintln(os.Stderr, "Error: cannot determine config directory")
			os.Exit(1)
		}
		var hadErrors bool
		if resetViews {
			if !resetYAMLDir("view config", filepath.Join(cfgDir, "views")) {
				hadErrors = true
			}
		}
		if resetThemes {
			if !resetYAMLDir("theme", filepath.Join(cfgDir, "themes")) {
				hadErrors = true
			}
		}
		if hadErrors {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Validate -c/--command flag: resolve to a canonical resource short name early
	// so invalid input fails fast before the TUI starts.
	var resolvedCommand string
	if command != "" {
		rt := resource.FindResourceType(command)
		if rt == nil {
			if newName, renamed := renameHints[command]; renamed {
				fmt.Fprintf(os.Stderr, "Error: %q was renamed to %q (see CHANGELOG.md). Try: -c %s\n", command, newName, newName)
			} else {
				fmt.Fprintf(os.Stderr, "Error: unknown resource type: %s\n", command)
			}
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
		if err := config.EnsureViewsDir(filepath.Join(cfgDir, "views")); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: views directory: %v\n", err)
		}
		if err := config.EnsureViewsReference(cfgDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: views reference: %v\n", err)
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
		extraOpts = append(extraOpts, tui.WithClients(demo.NewServiceClients()), tui.WithNoCache(true), tui.WithIsDemo(true))
	} else if noCache {
		extraOpts = append(extraOpts, tui.WithNoCache(true))
	}

	if resolvedCommand != "" {
		extraOpts = append(extraOpts, tui.WithCommand(resolvedCommand))
	}

	tui.Version = version

	// Populate the ACTIVE nav field registry from the DEFAULT registry so that
	// DetailModel navigability works in production. This runs after all init()
	// functions have populated the DEFAULT registry via SetDefaultNavFieldsForTest.
	resource.BootstrapActiveNavFields()

	if webMode {
		// Security: reject any attempt to bind on a non-loopback address as the default.
		// The user may supply --web-addr explicitly, but we warn if it looks like 0.0.0.0.
		if strings.HasPrefix(webAddr, "0.0.0.0") {
			fmt.Fprintln(os.Stderr, "Error: --web-addr must not use 0.0.0.0 (binds all interfaces). Use 127.0.0.1:<port> instead.")
			os.Exit(1)
		}
		if err := runWebServer(profile, region, resolvedCommand, webAddr, webAllowReveal, demoMode, noCache); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := runProgram(profile, region, extraOpts, activeTheme); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// resetYAMLDir deletes all .yaml files in dir after user confirmation.
// label describes what kind of files (e.g. "view config", "theme") for the prompt.
// Returns true if all files were removed (or nothing to do), false on errors.
func resetYAMLDir(label, dir string) bool {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		fmt.Printf("No %s files found — nothing to reset.\n", label)
		return true
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot read %s: %v\n", dir, err)
		return false
	}
	var yamlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			yamlFiles = append(yamlFiles, e.Name())
		}
	}
	if len(yamlFiles) == 0 {
		fmt.Printf("No %s files found — nothing to reset.\n", label)
		return true
	}

	fmt.Printf("This will delete %d %s files in %s/\n", len(yamlFiles), label, dir)
	fmt.Println("Any custom edits will be lost. Files will be recreated with defaults on next launch.")
	fmt.Print("\nContinue? [y/N] ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return true
	}

	var removed, failed int
	for _, name := range yamlFiles {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not remove %s: %v\n", path, err)
			failed++
		} else {
			removed++
		}
	}
	fmt.Printf("Removed %d files. Run a9s to recreate defaults.\n", removed)
	return failed == 0
}

// runWebServer starts the HTTP web server and blocks until SIGINT/SIGTERM.
func runWebServer(profile, region, command, addr string, allowReveal, demoMode, noCache bool) error {
	token, err := web.GenerateToken()
	if err != nil {
		return fmt.Errorf("generating token: %w", err)
	}

	viewCfg, cfgErr := config.Load()
	if cfgErr != nil {
		viewCfg = config.SharedDefaultConfig()
	}

	srv := web.NewServer(profile, region, command, addr, token, demoMode, noCache, allowReveal, viewCfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	readyCh := make(chan struct{})
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe(ctx, readyCh)
	}()

	// Wait for the server to bind (or fail immediately).
	select {
	case err := <-srvErr:
		return err
	case <-readyCh:
	}

	fmt.Fprintf(os.Stderr, "a9s web server: http://%s/?token=%s\n", srv.Addr(), srv.Token())
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop.\n")

	return <-srvErr
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
