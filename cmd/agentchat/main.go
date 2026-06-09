package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sariel/agent-in-chat-feishu/internal/config"
	"github.com/sariel/agent-in-chat-feishu/internal/feishu"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		runBridge(args)
		return
	}

	switch args[0] {
	case "run", "start":
		runBridge(args[1:])
	case "setup":
		runFeishuSetup(args[1:], feishuSetupModeAuto)
	case "feishu", "lark":
		runFeishu(args[1:])
	case "auth-url":
		runFeishuAuthURL(args[1:])
	case "help", "-h", "--help":
		printUsage()
	default:
		if strings.HasPrefix(args[0], "-") {
			runBridge(args)
			return
		}
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func runBridge(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", config.DefaultConfigPath(), "path to config.toml")
	_ = fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", "path", *configPath, "error", err)
		os.Exit(1)
	}
	runtime, err := feishu.NewRuntime(cfg)
	if err != nil {
		slog.Error("create runtime failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runtime.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("runtime stopped with error", "error", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: agentchat <command> [options]

Commands:
  run              Start the Feishu -> Codex bridge
  setup            Create or bind a Feishu/Lark app, then print the permissions link
  feishu setup     Same as setup, with explicit platform namespace
  feishu new       Force QR onboarding to create a new app
  feishu bind      Bind existing app credentials
  auth-url         Print the built-in permissions link for an app_id

Examples:
  agentchat setup
  agentchat setup --app cli_xxx:sec_xxx
  agentchat run -config ~/.agentchat/config.toml`)
}
