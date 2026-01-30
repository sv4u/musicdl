package main

import (
	"fmt"
	"os"
	"strings"
)

var (
	// Version is set at build time via ldflags
	// Example: go build -ldflags="-X main.Version=v1.2.3"
	Version = "dev"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Handle version command
	if command == "version" || command == "--version" || command == "-v" {
		fmt.Printf("musicdl version %s\n", Version)
		os.Exit(0)
	}

	// CLI: musicdl plan <config-file>
	if command == "plan" {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: musicdl plan <config-file>\n")
			os.Exit(PlanExitConfigError)
		}
		configPath := os.Args[2]
		os.Exit(planCommand(configPath))
	}

	// CLI: musicdl download <config-file>
	if command == "download" && len(os.Args) >= 3 && !strings.HasPrefix(os.Args[2], "-") {
		configPath := os.Args[2]
		os.Exit(downloadCLICommand(configPath))
	}

	if command == "download" {
		fmt.Fprintf(os.Stderr, "Usage: musicdl download <config-file>\n")
		os.Exit(DownloadExitConfigError)
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
	printUsage()
	os.Exit(1)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `musicdl - Music download tool

USAGE:
    musicdl <command> [arguments]

COMMANDS:
    plan <config-file>     Generate download plan (saves to .cache/download_plan_<hash>.json)
    download <config-file> Run downloads from plan (run 'musicdl plan' first)
    version                Show version information

EXAMPLES:
    musicdl plan musicdl-config.yml
    musicdl download musicdl-config.yml
    musicdl version

For more information, see https://github.com/sv4u/musicdl
`)
}
