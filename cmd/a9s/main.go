package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui"
)

const version = "2.1.1"

func main() {
	var (
		profile     string
		region      string
		showVersion bool
		showHelp    bool
	)

	flag.StringVar(&profile, "profile", "", "AWS profile to use")
	flag.StringVar(&profile, "p", "", "AWS profile to use (shorthand)")
	flag.StringVar(&region, "region", "", "AWS region override")
	flag.StringVar(&region, "r", "", "AWS region override (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.BoolVar(&showVersion, "v", false, "Print version and exit (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Print help and exit")
	flag.BoolVar(&showHelp, "h", false, "Print help and exit (shorthand)")

	flag.Usage = func() {
		fmt.Println("a9s - Terminal UI AWS Resource Manager")
		fmt.Printf("Version: %s\n\n", version)
		fmt.Println("Usage: a9s [flags]")
		fmt.Println("  -p, --profile  AWS profile to use")
		fmt.Println("  -r, --region   AWS region override")
		fmt.Println("  -v, --version  Print version and exit")
		fmt.Println("  -h, --help     Print this help")
	}

	flag.Parse()

	if showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if showVersion {
		fmt.Printf("a9s %s\n", version)
		os.Exit(0)
	}

	tui.Version = version

	model := tui.New(profile, region)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
