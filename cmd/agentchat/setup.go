package main

import (
	"fmt"
	"os"
	"strings"
)

func runSetup(args []string) {
	target, rest, err := parseSetupCommand(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		printSetupUsage()
		os.Exit(1)
	}

	switch target {
	case "feishu":
		runFeishuSetup(rest, feishuSetupModeAuto)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown setup target %q\n\n", target)
		printSetupUsage()
		os.Exit(1)
	}
}

func parseSetupCommand(args []string) (target string, rest []string, err error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("setup target is required")
	}
	target = strings.ToLower(strings.TrimSpace(args[0]))
	switch target {
	case "feishu", "lark":
		return "feishu", args[1:], nil
	default:
		return "", nil, fmt.Errorf("unknown setup target %q", args[0])
	}
}

func printSetupUsage() {
	fmt.Println(`Usage: agentchat setup <target> [options]

Targets:
  feishu    Create or connect a Feishu/Lark bot, configure Codex by default,
            install/start the daemon, and open the permission confirmation page.

Examples:
  agentchat setup feishu
  agentchat setup feishu --wizard
  agentchat setup feishu --wizard --agent kimi --no-start
  agentchat setup feishu --app cli_xxx:sec_xxx
  agentchat setup feishu --no-start`)
}
