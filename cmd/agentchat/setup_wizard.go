package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Renaissance-Mind/agent-in-chat-feishu/core"
	"golang.org/x/term"
)

type feishuSetupWizardConfig struct {
	ConfigPath             string
	Mode                   string
	Project                string
	PlatformType           string
	AppID                  string
	AppSecret              string
	AgentType              string
	WorkDir                string
	AdminOpenID            string
	AutoBindChats          bool
	GroupReplyAll          bool
	GroupContextBuffer     bool
	ShareSessionInChannel  bool
	EnableFeishuCard       bool
	InstallAndStartService bool
}

type setupChoice struct {
	Key   string
	Label string
}

func shouldUseFeishuSetupWizard(force, disabled bool, argCount int) bool {
	if force {
		return true
	}
	if disabled || argCount > 0 {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func runFeishuSetupWizard(in io.Reader, out io.Writer, defaults feishuSetupWizardConfig) (feishuSetupWizardConfig, error) {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	reader := bufio.NewReader(in)
	cfg := defaults

	fmt.Fprintln(out, "Agent-in-Chat-Feishu Setup")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "This wizard will create or connect a Feishu/Lark bot, configure a local agent, and optionally start agentchat in the background.")
	fmt.Fprintln(out)

	var err error
	cfg.ConfigPath, err = promptString(reader, out, "Config file", cfg.ConfigPath)
	if err != nil {
		return cfg, err
	}

	modeDefault := "create"
	if cfg.Mode == feishuSetupModeBind || cfg.AppID != "" || cfg.AppSecret != "" {
		modeDefault = "connect"
	}
	mode, err := promptChoice(reader, out, "Bot setup mode", []setupChoice{
		{Key: "create", Label: "Create a new bot by QR code"},
		{Key: "connect", Label: "Connect an existing bot with app_id/app_secret"},
	}, modeDefault)
	if err != nil {
		return cfg, err
	}
	if mode == "connect" {
		cfg.Mode = feishuSetupModeBind
		cfg.AppID, err = promptString(reader, out, "App ID", cfg.AppID)
		if err != nil {
			return cfg, err
		}
		cfg.AppSecret, err = promptString(reader, out, "App Secret", cfg.AppSecret)
		if err != nil {
			return cfg, err
		}
	} else {
		cfg.Mode = feishuSetupModeNew
		cfg.AppID = ""
		cfg.AppSecret = ""
	}

	platformDefault := strings.TrimSpace(cfg.PlatformType)
	if platformDefault == "" {
		platformDefault = "auto"
	}
	platform, err := promptChoice(reader, out, "Platform", []setupChoice{
		{Key: "auto", Label: "Auto-detect"},
		{Key: "feishu", Label: "Feishu"},
		{Key: "lark", Label: "Lark"},
	}, platformDefault)
	if err != nil {
		return cfg, err
	}
	if platform == "auto" {
		cfg.PlatformType = ""
	} else {
		cfg.PlatformType = platform
	}

	projectWasDefaulted := strings.TrimSpace(cfg.Project) == ""
	if projectWasDefaulted {
		cfg.Project = defaultFeishuProject
	}
	cfg.Project, err = promptString(reader, out, "Local bot profile name", cfg.Project)
	if err != nil {
		return cfg, err
	}

	agentChoices := setupAgentChoices()
	agentDefault := strings.TrimSpace(cfg.AgentType)
	if agentDefault == "" {
		agentDefault = "codex"
	}
	cfg.AgentType, err = promptChoice(reader, out, "Agent", agentChoices, agentDefault)
	if err != nil {
		return cfg, err
	}

	if strings.TrimSpace(cfg.WorkDir) == "" {
		workDirProject := cfg.Project
		if projectWasDefaulted {
			workDirProject = ""
		}
		cfg.WorkDir = defaultFeishuSetupWorkDirForConfig(cfg.ConfigPath, workDirProject)
	}
	cfg.WorkDir, err = promptString(reader, out, "Initial workspace", cfg.WorkDir)
	if err != nil {
		return cfg, err
	}

	cfg.AdminOpenID, err = promptString(reader, out, "Admin open_id (blank = use creator open_id when QR setup returns it)", cfg.AdminOpenID)
	if err != nil {
		return cfg, err
	}

	cfg.AutoBindChats, err = promptBool(reader, out, "Auto-bind chats by admin", cfg.AutoBindChats)
	if err != nil {
		return cfg, err
	}

	groupDefault := "mention"
	if cfg.GroupReplyAll {
		groupDefault = "all"
	}
	groupMode, err := promptChoice(reader, out, "Group trigger mode", []setupChoice{
		{Key: "mention", Label: "Only respond when mentioned"},
		{Key: "all", Label: "Respond to every group message"},
	}, groupDefault)
	if err != nil {
		return cfg, err
	}
	cfg.GroupReplyAll = groupMode == "all"

	cfg.GroupContextBuffer, err = promptBool(reader, out, "Include recent group history as context", cfg.GroupContextBuffer)
	if err != nil {
		return cfg, err
	}
	cfg.ShareSessionInChannel, err = promptBool(reader, out, "Share one agent session per group chat", cfg.ShareSessionInChannel)
	if err != nil {
		return cfg, err
	}
	cfg.EnableFeishuCard, err = promptBool(reader, out, "Enable Feishu progress cards", cfg.EnableFeishuCard)
	if err != nil {
		return cfg, err
	}
	cfg.InstallAndStartService, err = promptBool(reader, out, "Install and start background service", cfg.InstallAndStartService)
	if err != nil {
		return cfg, err
	}

	printFeishuSetupWizardSummary(out, cfg)
	confirmed, err := promptBool(reader, out, "Continue and write config", true)
	if err != nil {
		return cfg, err
	}
	if !confirmed {
		return cfg, fmt.Errorf("setup cancelled")
	}
	return cfg, nil
}

func setupAgentChoices() []setupChoice {
	registered := make(map[string]bool)
	for _, name := range core.ListRegisteredAgents() {
		registered[strings.ToLower(strings.TrimSpace(name))] = true
	}
	preferred := []setupChoice{
		{Key: "codex", Label: "Codex (recommended)"},
		{Key: "claudecode", Label: "Claude Code"},
		{Key: "gemini", Label: "Gemini"},
		{Key: "cursor", Label: "Cursor"},
		{Key: "opencode", Label: "OpenCode"},
		{Key: "kimi", Label: "Kimi CLI"},
		{Key: "iflow", Label: "iFlow"},
		{Key: "qoder", Label: "Qoder"},
		{Key: "pi", Label: "Pi"},
		{Key: "acp", Label: "ACP / Custom"},
	}

	choices := make([]setupChoice, 0, len(registered))
	seen := make(map[string]bool)
	for _, choice := range preferred {
		if registered[choice.Key] {
			choices = append(choices, choice)
			seen[choice.Key] = true
		}
	}
	var rest []string
	for name := range registered {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		choices = append(choices, setupChoice{Key: name, Label: name})
	}
	if len(choices) == 0 {
		choices = append(choices, setupChoice{Key: "codex", Label: "Codex (recommended)"})
	}
	return choices
}

func promptString(reader *bufio.Reader, out io.Writer, label, defaultValue string) (string, error) {
	fmt.Fprintf(out, "? %s\n", label)
	if defaultValue != "" {
		fmt.Fprintf(out, "  Default: %s\n", defaultValue)
	}
	fmt.Fprint(out, "  > ")
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		value = defaultValue
	}
	fmt.Fprintln(out)
	return value, nil
}

func promptChoice(reader *bufio.Reader, out io.Writer, label string, choices []setupChoice, defaultKey string) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("%s has no choices", label)
	}
	defaultKey = strings.ToLower(strings.TrimSpace(defaultKey))
	if defaultKey == "" {
		defaultKey = choices[0].Key
	}
	for {
		fmt.Fprintf(out, "? %s\n", label)
		for i, choice := range choices {
			prefix := " "
			if choice.Key == defaultKey {
				prefix = "*"
			}
			fmt.Fprintf(out, "  %s %d. %s\n", prefix, i+1, choice.Label)
		}
		fmt.Fprintf(out, "  Default: %s\n", defaultKey)
		fmt.Fprint(out, "  > ")
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		value := strings.ToLower(strings.TrimSpace(line))
		if value == "" {
			value = defaultKey
		}
		for i, choice := range choices {
			if value == choice.Key || value == strings.ToLower(choice.Label) || value == fmt.Sprintf("%d", i+1) {
				fmt.Fprintln(out)
				return choice.Key, nil
			}
		}
		fmt.Fprintf(out, "Please enter one of: %s\n\n", strings.Join(choiceKeys(choices), ", "))
	}
}

func promptBool(reader *bufio.Reader, out io.Writer, label string, defaultValue bool) (bool, error) {
	defaultText := "Y/n"
	if !defaultValue {
		defaultText = "y/N"
	}
	for {
		fmt.Fprintf(out, "? %s [%s]\n", label, defaultText)
		fmt.Fprint(out, "  > ")
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}
		value := strings.ToLower(strings.TrimSpace(line))
		fmt.Fprintln(out)
		switch value {
		case "":
			return defaultValue, nil
		case "y", "yes", "true", "1":
			return true, nil
		case "n", "no", "false", "0":
			return false, nil
		default:
			fmt.Fprintln(out, "Please enter yes or no.")
		}
	}
}

func choiceKeys(choices []setupChoice) []string {
	keys := make([]string, len(choices))
	for i, choice := range choices {
		keys[i] = choice.Key
	}
	return keys
}

func printFeishuSetupWizardSummary(out io.Writer, cfg feishuSetupWizardConfig) {
	mode := "create_new"
	if cfg.Mode == feishuSetupModeBind {
		mode = "connect_existing"
	}
	platform := cfg.PlatformType
	if platform == "" {
		platform = "auto"
	}
	admin := cfg.AdminOpenID
	if admin == "" {
		admin = "creator_open_id"
	}
	service := "config_only"
	if cfg.InstallAndStartService {
		service = "install_and_start"
	}
	trigger := "mention_only"
	if cfg.GroupReplyAll {
		trigger = "all_messages"
	}

	fmt.Fprintln(out, "Setup Summary")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Config file:              %s\n", cfg.ConfigPath)
	fmt.Fprintf(out, "Bot setup mode:           %s\n", mode)
	fmt.Fprintf(out, "Platform:                 %s\n", platform)
	fmt.Fprintf(out, "Local profile:            %s\n", cfg.Project)
	fmt.Fprintf(out, "Agent:                    %s\n", cfg.AgentType)
	fmt.Fprintf(out, "Initial workspace:        %s\n", cfg.WorkDir)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Access mode:              chat_binding\n")
	fmt.Fprintf(out, "Admin open_id:            %s\n", admin)
	fmt.Fprintf(out, "Auto-bind chats:          %t\n", cfg.AutoBindChats)
	fmt.Fprintf(out, "Private chat binding:     allow_private_chats = \"\"\n")
	fmt.Fprintf(out, "Group chat binding:       allow_group_chats = \"\"\n")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Group trigger:            %s\n", trigger)
	fmt.Fprintf(out, "Group history context:    %t\n", cfg.GroupContextBuffer)
	fmt.Fprintf(out, "Shared group session:     %t\n", cfg.ShareSessionInChannel)
	fmt.Fprintf(out, "Progress cards:           %t\n", cfg.EnableFeishuCard)
	fmt.Fprintf(out, "Background service:       %s\n", service)
	fmt.Fprintln(out)
}
