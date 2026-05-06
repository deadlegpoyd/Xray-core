package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/main/commands"
)

var (
	// Build information, injected at build time via ldflags
	Version   = "custom"
	BuildTime = "unknown"
)

func init() {
	// Ensure we use all available CPU cores
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	// Define command-line flags
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	configFlag := flag.String("config", "", "Path to configuration file")
	// Default to 'yaml' since that's what I use for all my personal configs
	formatFlag := flag.String("format", "yaml", "Configuration file format: json, toml, yaml")
	testFlag := flag.Bool("test", false, "Test configuration and exit")

	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("Xray %s (XTLS/Xray-core fork)\n", core.Version())
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// Require a config file
	if *configFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: configuration file is required. Use -config flag.")
		flag.Usage()
		os.Exit(1)
	}

	// Load and parse the configuration
	server, err := commands.LoadConfig(*configFlag, *formatFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// If test mode, validate config and exit
	if *testFlag {
		fmt.Println("Configuration OK.")
		os.Exit(0)
	}

	// Start the Xray server
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

	fmt.Printf("Xray %s started.\n", core.Version())

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigCh
		switch sig {
		case syscall.SIGHUP:
			// Reload configuration on SIGHUP
			fmt.Println("Received SIGHUP, reloading configuration...")
			newServer, err := commands.LoadConfig(*configFlag, *formatFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to reload configuration: %v\n", err)
				continue
			}
			if err := newServer.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start reloaded server: %v\n", err)
				continue
			}
			server.Close()
			server = newServer
			fmt.Println("Configuration reloaded successfully.")
		case syscall.SIGINT, syscall.SIGTERM:
			fmt.Println("Shutting down Xray...")
			return
		}
	}
}
