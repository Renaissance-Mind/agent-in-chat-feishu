package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	ccconnect "github.com/Renaissance-Mind/agent-in-chat-feishu"
	"github.com/Renaissance-Mind/agent-in-chat-feishu/config"
	"github.com/Renaissance-Mind/agent-in-chat-feishu/core"
)

func runConfig(args []string) {
	if len(args) == 0 {
		printConfigUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "example":
		fmt.Print(ccconnect.ConfigExampleTOML)
	case "format", "fmt":
		runConfigFormat(args[1:])
	case "path":
		fmt.Println(resolveConfigPath(""))
	case "reload":
		runConfigReload(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", args[0])
		printConfigUsage()
		os.Exit(1)
	}
}

func runConfigFormat(args []string) {
	fs := flag.NewFlagSet("config format", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file (default: auto-detect)")
	_ = fs.Parse(args)

	path := resolveConfigPath(*configPath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Config file not found: %s\n", path)
		os.Exit(1)
	}

	if err := config.FormatConfigFile(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Formatted %s\n", path)
}

func runConfigReload(args []string) {
	fs := flag.NewFlagSet("config reload", flag.ExitOnError)
	dataDir := fs.String("data-dir", "", "data directory (default: ~/.agentchat)")
	project := fs.String("project", "", "project name to reload (default: all projects)")
	_ = fs.Parse(args)

	sockPath := resolveSocketPath(*dataDir)
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: agentchat is not running (socket not found: %s)\n", sockPath)
		os.Exit(1)
	}

	payload, err := json.Marshal(core.ConfigReloadRequest{Project: strings.TrimSpace(*project)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to encode reload request: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sockPath)
			},
		},
	}

	resp, err := client.Post("http://unix/config/reload", "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	fmt.Println("Config reloaded successfully.")
}

func printConfigUsage() {
	fmt.Fprintf(os.Stderr, `Usage: agentchat config <subcommand>

Subcommands:
  example    Print a complete annotated config.toml example
  format     Format the config file (alias: fmt)
  path       Print the resolved config file path
  reload     Hot-reload running agentchat config via the local socket

Flags for 'format':
  --config <path>   Path to config file (default: auto-detect)

Flags for 'reload':
  --data-dir <path>   Data directory (default: ~/.agentchat)
  --project <name>    Project name to reload (default: all projects)

Examples:
  agentchat config example              Print example config
  agentchat config example > config.toml  Save example config
  agentchat config format               Format default config file
  agentchat config fmt --config /path/to/config.toml
  agentchat config reload --project codex-feishu
`)
}
