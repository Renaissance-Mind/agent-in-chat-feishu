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
	BotPrepared            bool
	OwnerOpenID            string
	AgentType              string
	WorkDir                string
	AdminOpenID            string
	AutoBindChats          bool
	GroupReplyAll          bool
	GroupContextBuffer     bool
	ShareSessionInChannel  bool
	EnableFeishuCard       bool
	InstallAndStartService bool
	TimeoutSeconds         int
	QRImagePath            string
	Debug                  bool
}

type setupChoice struct {
	Key   string
	Label string
	Hint  string
}

var (
	setupWizardRunRegistrationFlow    = runRegistrationFlow
	setupWizardValidateAppCredentials = validateAppCredentials
)

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
	if canRunFeishuSetupWizardTUI(in, out) {
		return runFeishuSetupWizardTUI(in.(*os.File), out.(*os.File), defaults)
	}
	return runFeishuSetupWizardPlain(in, out, defaults)
}

func prepareFeishuSetupWizardBot(cfg *feishuSetupWizardConfig) error {
	if cfg.BotPrepared {
		return nil
	}
	switch cfg.Mode {
	case feishuSetupModeNew:
		result, err := setupWizardRunRegistrationFlow(registrationFlowOptions{
			TimeoutSeconds: cfg.TimeoutSeconds,
			QRImagePath:    cfg.QRImagePath,
			Debug:          cfg.Debug,
		})
		if err != nil {
			return fmt.Errorf("onboarding failed: %w", err)
		}
		if result == nil {
			return fmt.Errorf("onboarding returned no result")
		}
		appID := strings.TrimSpace(result.AppID)
		appSecret := strings.TrimSpace(result.AppSecret)
		if appID == "" || appSecret == "" {
			return fmt.Errorf("onboarding returned incomplete app credentials")
		}
		cfg.AppID = appID
		cfg.AppSecret = appSecret
		cfg.OwnerOpenID = strings.TrimSpace(result.OwnerOpenID)
		if platform := strings.TrimSpace(result.Platform); platform != "" {
			normalized, err := normalizeFeishuPlatformType(platform)
			if err != nil {
				return err
			}
			cfg.PlatformType = normalized
		}
		cfg.BotPrepared = true
		return nil

	case feishuSetupModeBind:
		platformType, err := normalizeFeishuPlatformType(cfg.PlatformType)
		if err != nil {
			return err
		}
		detectedType, err := setupWizardValidateAppCredentials(cfg.AppID, cfg.AppSecret, platformType)
		if err != nil {
			return fmt.Errorf("app_id/app_secret validation failed: %w", err)
		}
		if platformType == "" {
			platformType = detectedType
		}
		cfg.PlatformType = platformType
		cfg.BotPrepared = true
		return nil

	default:
		return nil
	}
}

func runFeishuSetupWizardPlain(in io.Reader, out io.Writer, defaults feishuSetupWizardConfig) (feishuSetupWizardConfig, error) {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	reader := bufio.NewReader(in)
	cfg := defaults

	printWizardIntro(out)

	var err error
	printWizardSection(out, "Config", "Store credentials and local profile settings.")
	cfg.ConfigPath, err = promptString(reader, out, "Config file", cfg.ConfigPath)
	if err != nil {
		return cfg, err
	}

	printWizardSection(out, "Bot", "Create a bot by QR onboarding or connect an existing Feishu/Lark app.")
	modeDefault := "create"
	if cfg.Mode == feishuSetupModeBind || cfg.AppID != "" || cfg.AppSecret != "" {
		modeDefault = "connect"
	}
	mode, err := promptChoice(reader, out, "Bot setup mode", []setupChoice{
		{Key: "create", Label: "Create a new bot by QR code", Hint: "best for first-time setup"},
		{Key: "connect", Label: "Connect an existing bot", Hint: "use app_id and app_secret"},
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
		if err := prepareFeishuSetupWizardBot(&cfg); err != nil {
			return cfg, err
		}
	}

	if !cfg.BotPrepared {
		platformDefault := strings.TrimSpace(cfg.PlatformType)
		if platformDefault == "" {
			platformDefault = "auto"
		}
		platform, err := promptChoice(reader, out, "Platform", []setupChoice{
			{Key: "auto", Label: "Auto-detect", Hint: "validate credentials against both"},
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
		if err := prepareFeishuSetupWizardBot(&cfg); err != nil {
			return cfg, err
		}
	}

	projectWasDefaulted := strings.TrimSpace(cfg.Project) == ""
	if projectWasDefaulted {
		cfg.Project = defaultFeishuProject
	}
	printWizardSection(out, "Local agent", "Choose the profile name, agent CLI, and starting workspace.")
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

	printWizardSection(out, "Chat access", "The default binding model lets admins bind private chats and groups on first use.")
	adminDefault := cfg.AdminOpenID
	if strings.TrimSpace(adminDefault) == "" && strings.TrimSpace(cfg.OwnerOpenID) != "" {
		adminDefault = cfg.OwnerOpenID
	}
	cfg.AdminOpenID, err = promptString(reader, out, "Admin open_id (blank = use creator open_id when QR setup returns it)", adminDefault)
	if err != nil {
		return cfg, err
	}

	cfg.AutoBindChats, err = promptBool(reader, out, "Auto-bind chats by admin", cfg.AutoBindChats)
	if err != nil {
		return cfg, err
	}

	printWizardSection(out, "Group behavior", "Tune when the bot replies and how much group context is sent to the agent.")
	groupDefault := "mention"
	if cfg.GroupReplyAll {
		groupDefault = "all"
	}
	groupMode, err := promptChoice(reader, out, "Group trigger mode", []setupChoice{
		{Key: "mention", Label: "Only respond when mentioned", Hint: "recommended"},
		{Key: "all", Label: "Respond to every group message", Hint: "busy groups can get noisy"},
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
	printWizardSection(out, "Runtime", "Start the daemon now, or write config only and run it manually.")
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
		{Key: "codex", Label: "Codex", Hint: "recommended default"},
		{Key: "claudecode", Label: "Claude Code"},
		{Key: "gemini", Label: "Gemini"},
		{Key: "cursor", Label: "Cursor"},
		{Key: "opencode", Label: "OpenCode"},
		{Key: "kimi", Label: "Kimi CLI", Hint: "Moonshot/Kimi"},
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
		choices = append(choices, setupChoice{Key: "codex", Label: "Codex", Hint: "recommended default"})
	}
	return choices
}

func printWizardIntro(out io.Writer) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Agent-in-Chat-Feishu setup")
	fmt.Fprintln(out, "Configure Feishu/Lark, local agent profile, chat access, and service startup.")
	fmt.Fprintln(out)
}

func printWizardSection(out io.Writer, title, hint string) {
	fmt.Fprintf(out, "%s\n", title)
	if strings.TrimSpace(hint) != "" {
		fmt.Fprintf(out, "  %s\n", hint)
	}
	fmt.Fprintln(out)
}

func promptString(reader *bufio.Reader, out io.Writer, label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(out, "%s [%s]\n", label, defaultValue)
	} else {
		fmt.Fprintf(out, "%s\n", label)
	}
	fmt.Fprint(out, "> ")
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
		fmt.Fprintf(out, "%s\n", label)
		for i, choice := range choices {
			prefix := " "
			if choice.Key == defaultKey {
				prefix = "*"
			}
			fmt.Fprintf(out, "  %s %d) %s", prefix, i+1, choice.Label)
			if strings.TrimSpace(choice.Hint) != "" {
				fmt.Fprintf(out, " - %s", choice.Hint)
			}
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "Select [%s]\n", defaultKey)
		fmt.Fprint(out, "> ")
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
		fmt.Fprintf(out, "Enter one of: %s\n\n", strings.Join(choiceKeys(choices), ", "))
	}
}

func promptBool(reader *bufio.Reader, out io.Writer, label string, defaultValue bool) (bool, error) {
	defaultText := "Y/n"
	if !defaultValue {
		defaultText = "y/N"
	}
	for {
		fmt.Fprintf(out, "%s [%s]\n", label, defaultText)
		fmt.Fprint(out, "> ")
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
			fmt.Fprintln(out, "Enter yes or no.")
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

	printWizardSection(out, "Summary", "Review before writing config.")
	printSummaryField(out, "Config file", cfg.ConfigPath)
	printSummaryField(out, "Bot setup mode", mode)
	printSummaryField(out, "Platform", platform)
	printSummaryField(out, "Local profile", cfg.Project)
	printSummaryField(out, "Agent", cfg.AgentType)
	printSummaryField(out, "Initial workspace", cfg.WorkDir)
	fmt.Fprintln(out)
	printSummaryField(out, "Access mode", "chat_binding")
	printSummaryField(out, "Admin open_id", admin)
	printSummaryField(out, "Auto-bind chats", formatWizardBool(cfg.AutoBindChats))
	printSummaryField(out, "Private chat binding", `allow_private_chats = ""`)
	printSummaryField(out, "Group chat binding", `allow_group_chats = ""`)
	fmt.Fprintln(out)
	printSummaryField(out, "Group trigger", trigger)
	printSummaryField(out, "Group history context", formatWizardBool(cfg.GroupContextBuffer))
	printSummaryField(out, "Shared group session", formatWizardBool(cfg.ShareSessionInChannel))
	printSummaryField(out, "Progress cards", formatWizardBool(cfg.EnableFeishuCard))
	printSummaryField(out, "Background service", service)
	fmt.Fprintln(out)
}

func printSummaryField(out io.Writer, label, value string) {
	fmt.Fprintf(out, "  %-24s %s\n", label, value)
}

func formatWizardBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
