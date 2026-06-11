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
		return applyFeishuSetupWizardRegistrationResult(cfg, result)

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

func applyFeishuSetupWizardRegistrationResult(cfg *feishuSetupWizardConfig, result *registrationFlowResult) error {
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
	printWizardSection(out, "配置 / Config", "保存凭证与本地机器人配置。 / Store credentials and local profile settings.")
	cfg.ConfigPath, err = promptString(reader, out, "配置文件 / Config file", cfg.ConfigPath)
	if err != nil {
		return cfg, err
	}

	printWizardSection(out, "机器人 / Bot", "扫码创建机器人，或连接已有飞书/Lark 应用。 / Create by QR onboarding or connect an existing Feishu/Lark app.")
	modeDefault := "create"
	if cfg.Mode == feishuSetupModeBind || cfg.AppID != "" || cfg.AppSecret != "" {
		modeDefault = "connect"
	}
	mode, err := promptChoice(reader, out, "机器人设置模式 / Bot setup mode", []setupChoice{
		{Key: "create", Label: "扫码创建新机器人 / Create a new bot by QR code", Hint: "首次配置推荐 / best for first-time setup"},
		{Key: "connect", Label: "连接已有机器人 / Connect an existing bot", Hint: "使用 app_id 和 app_secret / use app_id and app_secret"},
	}, modeDefault)
	if err != nil {
		return cfg, err
	}
	if mode == "connect" {
		cfg.Mode = feishuSetupModeBind
		cfg.AppID, err = promptString(reader, out, "应用 ID / App ID", cfg.AppID)
		if err != nil {
			return cfg, err
		}
		cfg.AppSecret, err = promptString(reader, out, "应用密钥 / App Secret", cfg.AppSecret)
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
		platform, err := promptChoice(reader, out, "平台 / Platform", []setupChoice{
			{Key: "auto", Label: "自动检测 / Auto-detect", Hint: "同时校验飞书和 Lark / validate credentials against both"},
			{Key: "feishu", Label: "飞书 / Feishu"},
			{Key: "lark", Label: "Lark / Lark"},
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
	printWizardSection(out, "本地 Agent / Local agent", "选择配置名、Agent CLI 和启动目录。 / Choose the profile name, agent CLI, and starting workspace.")
	cfg.Project, err = promptString(reader, out, "本地机器人配置名 / Local bot profile name", cfg.Project)
	if err != nil {
		return cfg, err
	}

	agentChoices := setupAgentChoices()
	agentDefault := strings.TrimSpace(cfg.AgentType)
	if agentDefault == "" {
		agentDefault = "codex"
	}
	cfg.AgentType, err = promptChoice(reader, out, "Agent 类型 / Agent", agentChoices, agentDefault)
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
	cfg.WorkDir, err = promptString(reader, out, "初始工作目录 / Initial workspace", cfg.WorkDir)
	if err != nil {
		return cfg, err
	}

	printWizardSection(out, "聊天访问 / Chat access", "默认绑定模式允许管理员首次使用时绑定私聊或群聊。 / Admins can bind private chats and groups on first use.")
	adminDefault := cfg.AdminOpenID
	if strings.TrimSpace(adminDefault) == "" && strings.TrimSpace(cfg.OwnerOpenID) != "" {
		adminDefault = cfg.OwnerOpenID
	}
	cfg.AdminOpenID, err = promptString(reader, out, "管理员 open_id / Admin open_id (留空 = 使用扫码创建者 / blank = QR creator)", adminDefault)
	if err != nil {
		return cfg, err
	}

	cfg.AutoBindChats, err = promptBool(reader, out, "管理员自动绑定会话 / Auto-bind chats by admin", cfg.AutoBindChats)
	if err != nil {
		return cfg, err
	}

	printWizardSection(out, "群聊行为 / Group behavior", "设置机器人何时回复，以及发送多少群聊上下文。 / Tune replies and group context sent to the agent.")
	groupDefault := "mention"
	if cfg.GroupReplyAll {
		groupDefault = "all"
	}
	groupMode, err := promptChoice(reader, out, "群聊触发模式 / Group trigger mode", []setupChoice{
		{Key: "mention", Label: "仅被 @ 时回复 / Only respond when mentioned", Hint: "推荐 / recommended"},
		{Key: "all", Label: "每条群消息都回复 / Respond to every group message", Hint: "群聊可能变吵 / busy groups can get noisy"},
	}, groupDefault)
	if err != nil {
		return cfg, err
	}
	cfg.GroupReplyAll = groupMode == "all"

	cfg.GroupContextBuffer, err = promptBool(reader, out, "包含近期群聊历史作为上下文 / Include recent group history as context", cfg.GroupContextBuffer)
	if err != nil {
		return cfg, err
	}

	printWizardSection(out, "运行方式 / Runtime", "立即启动后台服务，或只写入配置后手动运行。 / Start the daemon now, or write config only.")
	cfg.InstallAndStartService, err = promptBool(reader, out, "安装并启动后台服务 / Install and start background service", cfg.InstallAndStartService)
	if err != nil {
		return cfg, err
	}

	printFeishuSetupWizardSummary(out, cfg)
	confirmed, err := promptBool(reader, out, "继续并写入配置 / Continue and write config", true)
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
	mode := "扫码创建 / create_new"
	if cfg.Mode == feishuSetupModeBind {
		mode = "连接已有 / connect_existing"
	}
	platform := cfg.PlatformType
	if platform == "" {
		platform = "自动检测 / auto"
	}
	admin := cfg.AdminOpenID
	if admin == "" {
		admin = "扫码创建者 / creator_open_id"
	}
	service := "仅写配置 / config_only"
	if cfg.InstallAndStartService {
		service = "安装并启动 / install_and_start"
	}
	trigger := "仅 @ / mention_only"
	if cfg.GroupReplyAll {
		trigger = "每条消息 / all_messages"
	}

	printWizardSection(out, "摘要 / Summary", "写入配置前确认。 / Review before writing config.")
	printSummaryField(out, "配置文件 / Config file", cfg.ConfigPath)
	printSummaryField(out, "机器人模式 / Bot mode", mode)
	printSummaryField(out, "平台 / Platform", platform)
	printSummaryField(out, "本地配置 / Local profile", cfg.Project)
	printSummaryField(out, "Agent 类型 / Agent", cfg.AgentType)
	printSummaryField(out, "工作目录 / Workspace", cfg.WorkDir)
	fmt.Fprintln(out)
	printSummaryField(out, "访问模式 / Access mode", "chat_binding")
	printSummaryField(out, "管理员 open_id / Admin", admin)
	printSummaryField(out, "自动绑定 / Auto-bind", formatWizardBool(cfg.AutoBindChats))
	printSummaryField(out, "私聊绑定 / Private binding", `allow_private_chats = ""`)
	printSummaryField(out, "群聊绑定 / Group binding", `allow_group_chats = ""`)
	fmt.Fprintln(out)
	printSummaryField(out, "群聊触发 / Group trigger", trigger)
	printSummaryField(out, "群聊上下文 / Group context", formatWizardBool(cfg.GroupContextBuffer))
	printSummaryField(out, "后台服务 / Background service", service)
	fmt.Fprintln(out)
}

func printSummaryField(out io.Writer, label, value string) {
	fmt.Fprintf(out, "  %-24s %s\n", label, value)
}

func formatWizardBool(value bool) string {
	if value {
		return "是 / yes"
	}
	return "否 / no"
}
