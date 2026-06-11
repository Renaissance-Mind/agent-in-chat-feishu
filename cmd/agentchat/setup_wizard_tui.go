package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const (
	setupWizardMinWidth   = 72
	setupWizardSidebar    = 25
	setupWizardMainHeight = 16
)

type setupWizardStepID int

const (
	setupStepConfig setupWizardStepID = iota
	setupStepMode
	setupStepAppID
	setupStepAppSecret
	setupStepPlatform
	setupStepProject
	setupStepAgent
	setupStepWorkDir
	setupStepAdmin
	setupStepAutoBind
	setupStepGroupMode
	setupStepGroupContext
	setupStepService
	setupStepSummary
)

type setupWizardStepKind int

const (
	setupStepText setupWizardStepKind = iota
	setupStepChoice
	setupStepSummaryKind
)

type setupWizardStep struct {
	ID    setupWizardStepID
	Kind  setupWizardStepKind
	Title string
	Hint  string
}

type setupWizardTUIModel struct {
	cfg              feishuSetupWizardConfig
	projectDefaulted bool
	projectEdited    bool
	stepIndex        int
	cursor           int
	inputValue       string
	inputCursor      int
	width            int
	height           int
	err              string
	done             bool
	cancelled        bool
	inputMasked      bool
	preparingBot     bool
}

type setupWizardRegistrationDoneMsg struct {
	result *registrationFlowResult
	err    error
}

type setupWizardRegistrationExec struct {
	opts   registrationFlowOptions
	result *registrationFlowResult
}

func (c *setupWizardRegistrationExec) Run() error {
	result, err := setupWizardRunRegistrationFlow(c.opts)
	c.result = result
	return err
}

func (c *setupWizardRegistrationExec) SetStdin(io.Reader)  {}
func (c *setupWizardRegistrationExec) SetStdout(io.Writer) {}
func (c *setupWizardRegistrationExec) SetStderr(io.Writer) {}

var (
	setupTUIAccentStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F6C453"))
	setupTUIAccentSoftStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2A65A"))
	setupTUIDimStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#7B7F87"))
	setupTUIBorderStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#3C414B"))
	setupTUIHeaderStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F6C453"))
	setupTUISelectedStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F6C453"))
	setupTUIErrorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#F97066"))
	setupTUISuccessStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3A5"))
	setupTUIPanelStyle        = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#3C414B")).Padding(1, 2)
	setupTUISidebarPanelStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#3C414B")).Padding(1, 1)
)

func canRunFeishuSetupWizardTUI(in io.Reader, out io.Writer) bool {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK {
		return false
	}
	return term.IsTerminal(int(inFile.Fd())) && term.IsTerminal(int(outFile.Fd()))
}

func runFeishuSetupWizardTUI(in *os.File, out *os.File, defaults feishuSetupWizardConfig) (feishuSetupWizardConfig, error) {
	model := newSetupWizardTUIModel(defaults)
	program := tea.NewProgram(
		model,
		tea.WithInput(in),
		tea.WithOutput(out),
		tea.WithAltScreen(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return defaults, err
	}
	result, ok := finalModel.(setupWizardTUIModel)
	if !ok {
		return defaults, fmt.Errorf("setup wizard returned unexpected model")
	}
	if result.cancelled {
		return result.cfg, fmt.Errorf("setup cancelled")
	}
	return result.cfg, nil
}

func newSetupWizardTUIModel(defaults feishuSetupWizardConfig) setupWizardTUIModel {
	cfg := defaults
	if cfg.Mode == "" || cfg.Mode == feishuSetupModeAuto {
		cfg.Mode = feishuSetupModeNew
	}
	if !cfg.BotPrepared && (cfg.AppID != "" || cfg.AppSecret != "") {
		cfg.Mode = feishuSetupModeBind
	}
	if strings.TrimSpace(cfg.AgentType) == "" {
		cfg.AgentType = "codex"
	}
	projectDefaulted := strings.TrimSpace(cfg.Project) == ""
	if projectDefaulted {
		cfg.Project = defaultFeishuProject
	}
	model := setupWizardTUIModel{
		cfg:              cfg,
		projectDefaulted: projectDefaulted,
		width:            96,
		height:           28,
	}
	model.syncCurrentStep()
	return model
}

func (m setupWizardTUIModel) Init() tea.Cmd {
	return nil
}

func (m setupWizardTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncCurrentStep()
		return m, nil
	case setupWizardRegistrationDoneMsg:
		return m.handleRegistrationDone(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m setupWizardTUIModel) View() string {
	width := m.width
	if width < setupWizardMinWidth {
		width = setupWizardMinWidth
	}
	steps := m.steps()
	header := setupTUIHeaderStyle.Render(fmt.Sprintf(
		"Agent-in-Chat-Feishu setup - step %d/%d",
		m.stepIndex+1,
		len(steps),
	))
	sidebar := m.renderSidebar(setupWizardSidebar, steps)
	mainWidth := width - setupWizardSidebar - 4
	if mainWidth < 44 {
		mainWidth = 44
	}
	main := m.renderMain(mainWidth)
	status := m.renderStatus(width)
	footer := setupTUIDimStyle.Render("enter 选择/下一步 select/next | esc 返回 back | q 退出 quit | arrows/j/k 移动 navigate")
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", main)
	return strings.Join([]string{header, body, status, footer}, "\n")
}

func (m setupWizardTUIModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || key == "q" {
		m.cancelled = true
		return m, tea.Quit
	}
	if key == "esc" || key == "b" {
		if m.stepIndex == 0 {
			m.cancelled = true
			return m, tea.Quit
		}
		m.stepIndex--
		m.err = ""
		m.syncCurrentStep()
		return m, nil
	}

	step := m.currentStep()
	switch step.Kind {
	case setupStepText:
		if key == "enter" {
			if err := m.commitTextStep(step.ID); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.err = ""
			m.advance()
			return m, nil
		}
		m.handleTextInput(msg)
		return m, nil

	case setupStepChoice:
		choices := m.currentChoices(step.ID)
		if key == "up" || key == "k" || key == "ctrl+p" {
			m.cursor = max(0, m.cursor-1)
			return m, nil
		}
		if key == "down" || key == "j" || key == "ctrl+n" {
			m.cursor = min(len(choices)-1, m.cursor+1)
			return m, nil
		}
		if key == "y" && m.isBoolChoice(step.ID) {
			cmd, err := m.chooseCurrentOption(step.ID, choices, 0)
			if err != nil {
				m.err = err.Error()
			}
			return m, cmd
		}
		if key == "n" && m.isBoolChoice(step.ID) {
			cmd, err := m.chooseCurrentOption(step.ID, choices, 1)
			if err != nil {
				m.err = err.Error()
			}
			return m, cmd
		}
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			idx := int(key[0] - '1')
			if key == "0" {
				idx = 9
			}
			if idx >= 0 && idx < len(choices) {
				cmd, err := m.chooseCurrentOption(step.ID, choices, idx)
				if err != nil {
					m.err = err.Error()
				}
				return m, cmd
			}
		}
		if key == "enter" {
			if len(choices) == 0 {
				return m, nil
			}
			cmd, err := m.chooseCurrentOption(step.ID, choices, m.cursor)
			if err != nil {
				m.err = err.Error()
			}
			return m, cmd
		}

	case setupStepSummaryKind:
		if key == "up" || key == "k" || key == "down" || key == "j" {
			if m.cursor == 0 {
				m.cursor = 1
			} else {
				m.cursor = 0
			}
			return m, nil
		}
		if key == "enter" || key == "y" {
			if m.cursor == 0 || key == "y" {
				m.done = true
				return m, tea.Quit
			}
			m.stepIndex--
			m.syncCurrentStep()
			return m, nil
		}
		if key == "n" {
			m.stepIndex--
			m.syncCurrentStep()
			return m, nil
		}
	}
	return m, nil
}

func (m *setupWizardTUIModel) chooseCurrentOption(stepID setupWizardStepID, choices []setupChoice, idx int) (tea.Cmd, error) {
	if idx < 0 || idx >= len(choices) {
		return nil, nil
	}
	m.cursor = idx
	m.applyChoice(stepID, choices[idx].Key)
	cmd, err := m.prepareBotAfterChoice(stepID)
	if err != nil {
		return nil, err
	}
	if cmd != nil {
		m.err = ""
		return cmd, nil
	}
	m.err = ""
	m.advanceAfterChoice(stepID)
	return nil, nil
}

func (m *setupWizardTUIModel) prepareBotAfterChoice(stepID setupWizardStepID) (tea.Cmd, error) {
	if stepID == setupStepMode && m.cfg.Mode == feishuSetupModeNew {
		m.preparingBot = true
		return m.registrationExecCmd(), nil
	}
	if stepID == setupStepPlatform && m.cfg.Mode == feishuSetupModeBind {
		return nil, m.prepareBotIfReady()
	}
	return nil, nil
}

func (m setupWizardTUIModel) registrationExecCmd() tea.Cmd {
	exec := &setupWizardRegistrationExec{
		opts: registrationFlowOptions{
			TimeoutSeconds: m.cfg.TimeoutSeconds,
			QRImagePath:    m.cfg.QRImagePath,
			Debug:          m.cfg.Debug,
		},
	}
	return tea.Exec(exec, func(err error) tea.Msg {
		return setupWizardRegistrationDoneMsg{result: exec.result, err: err}
	})
}

func (m setupWizardTUIModel) handleRegistrationDone(msg setupWizardRegistrationDoneMsg) (tea.Model, tea.Cmd) {
	m.preparingBot = false
	if msg.err != nil {
		m.err = fmt.Sprintf("onboarding failed: %v", msg.err)
		return m, nil
	}
	if err := applyFeishuSetupWizardRegistrationResult(&m.cfg, msg.result); err != nil {
		m.err = err.Error()
		return m, nil
	}
	m.err = ""
	m.advanceAfterChoice(setupStepMode)
	return m, nil
}

func (m *setupWizardTUIModel) advance() {
	if m.stepIndex >= len(m.steps())-1 {
		m.done = true
		return
	}
	m.stepIndex++
	m.syncCurrentStep()
}

func (m *setupWizardTUIModel) advanceAfterChoice(stepID setupWizardStepID) {
	if stepID == setupStepPlatform && m.cfg.BotPrepared {
		if m.jumpToStep(setupStepProject) {
			return
		}
	}
	m.advance()
}

func (m *setupWizardTUIModel) jumpToStep(stepID setupWizardStepID) bool {
	for i, step := range m.steps() {
		if step.ID == stepID {
			m.stepIndex = i
			m.syncCurrentStep()
			return true
		}
	}
	return false
}

func (m *setupWizardTUIModel) syncCurrentStep() {
	steps := m.steps()
	if m.stepIndex >= len(steps) {
		m.stepIndex = len(steps) - 1
	}
	if m.stepIndex < 0 {
		m.stepIndex = 0
	}
	step := steps[m.stepIndex]
	if step.Kind == setupStepText {
		m.inputValue = m.textValue(step.ID)
		m.inputCursor = len([]rune(m.inputValue))
		m.inputMasked = step.ID == setupStepAppSecret
		return
	}
	m.cursor = m.selectedChoiceIndex(step.ID)
}

func (m setupWizardTUIModel) steps() []setupWizardStep {
	steps := []setupWizardStep{
		{ID: setupStepConfig, Kind: setupStepText, Title: "配置文件 / Config file", Hint: "保存凭证与本地配置。 / Where agentchat stores credentials and local profiles."},
		{ID: setupStepMode, Kind: setupStepChoice, Title: "机器人设置模式 / Bot setup mode", Hint: "扫码创建机器人，或连接已有应用。 / Create by QR onboarding or connect an existing app."},
	}
	if !m.cfg.BotPrepared && m.cfg.Mode == feishuSetupModeBind {
		steps = append(steps,
			setupWizardStep{ID: setupStepAppID, Kind: setupStepText, Title: "应用 ID / App ID", Hint: "飞书/Lark app_id，例如 cli_xxx。 / Feishu/Lark app_id, for example cli_xxx."},
			setupWizardStep{ID: setupStepAppSecret, Kind: setupStepText, Title: "应用密钥 / App Secret", Hint: "飞书/Lark app_secret，输入会隐藏。 / Feishu/Lark app_secret. Input is masked."},
			setupWizardStep{ID: setupStepPlatform, Kind: setupStepChoice, Title: "平台 / Platform", Hint: "自动检测会同时校验飞书和 Lark。 / Auto-detect validates both Feishu and Lark."},
		)
	}
	steps = append(steps,
		setupWizardStep{ID: setupStepProject, Kind: setupStepText, Title: "本地配置名 / Local profile", Hint: "config.toml 里的本地机器人配置名。 / A local bot profile name in config.toml."},
		setupWizardStep{ID: setupStepAgent, Kind: setupStepChoice, Title: "Agent 类型 / Agent", Hint: "接收消息的本地 Agent CLI。 / Which local agent CLI should receive messages."},
		setupWizardStep{ID: setupStepWorkDir, Kind: setupStepText, Title: "工作目录 / Workspace", Hint: "本地 Agent 的初始目录。 / Initial directory for the local agent."},
		setupWizardStep{ID: setupStepAdmin, Kind: setupStepText, Title: "管理员 open_id / Admin open_id", Hint: "留空时使用扫码创建者。 / Blank keeps QR creator auto-detection when available."},
		setupWizardStep{ID: setupStepAutoBind, Kind: setupStepChoice, Title: "自动绑定会话 / Auto-bind chats", Hint: "管理员首次使用时可绑定私聊或群聊。 / Admins can bind private chats and groups on first use."},
		setupWizardStep{ID: setupStepGroupMode, Kind: setupStepChoice, Title: "群聊触发 / Group trigger", Hint: "仅 @ 回复适合大多数群聊。 / Mention-only is quieter and safer for busy groups."},
		setupWizardStep{ID: setupStepGroupContext, Kind: setupStepChoice, Title: "群聊上下文 / Group history context", Hint: "把近期非 @ 消息作为背景上下文。 / Include recent non-mention messages as background context."},
		setupWizardStep{ID: setupStepService, Kind: setupStepChoice, Title: "后台服务 / Background service", Hint: "写入配置后安装并启动 daemon。 / Install and start the daemon after writing config."},
		setupWizardStep{ID: setupStepSummary, Kind: setupStepSummaryKind, Title: "摘要 / Summary", Hint: "确认并写入配置。 / Review and write config."},
	)
	return steps
}

func (m setupWizardTUIModel) currentStep() setupWizardStep {
	return m.steps()[m.stepIndex]
}

func (m setupWizardTUIModel) currentStepIDs() []setupWizardStepID {
	steps := m.steps()
	ids := make([]setupWizardStepID, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.ID)
	}
	return ids
}

func containsSetupStep(ids []setupWizardStepID, target setupWizardStepID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func (m setupWizardTUIModel) renderSidebar(width int, steps []setupWizardStep) string {
	lines := []string{setupTUIHeaderStyle.Render("Setup")}
	for i, step := range steps {
		state := " "
		style := setupTUIDimStyle
		if i < m.stepIndex {
			state = "x"
			style = setupTUISuccessStyle
		}
		if i == m.stepIndex {
			state = ">"
			style = setupTUISelectedStyle
		}
		title := truncateWizardText(step.Title, width-6)
		lines = append(lines, style.Render(fmt.Sprintf("%s %2d %s", state, i+1, title)))
	}
	return setupTUISidebarPanelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m setupWizardTUIModel) renderMain(width int) string {
	step := m.currentStep()
	lines := []string{
		setupTUIHeaderStyle.Render(step.Title),
		setupTUIDimStyle.Render(step.Hint),
		"",
	}
	switch step.Kind {
	case setupStepText:
		lines = append(lines, m.renderTextStep(width)...)
	case setupStepChoice:
		if m.preparingBot && step.ID == setupStepMode {
			lines = append(lines, m.renderRegistrationExecStep(width)...)
		} else {
			lines = append(lines, m.renderChoiceStep(step.ID, width)...)
		}
	case setupStepSummaryKind:
		lines = append(lines, m.renderSummaryStep(width)...)
	}
	if m.err != "" {
		lines = append(lines, "", setupTUIErrorStyle.Render(m.err))
	}
	content := strings.Join(lines, "\n")
	panelHeight := setupWizardMainHeight
	if m.height > 26 {
		panelHeight = min(22, m.height-8)
	}
	return setupTUIPanelStyle.Width(width).Height(panelHeight).Render(content)
}

func (m setupWizardTUIModel) renderTextStep(width int) []string {
	value := m.renderInputLine(width - 6)
	defaultHint := m.textDefaultHint(m.currentStep().ID)
	lines := []string{value}
	if defaultHint != "" {
		lines = append(lines, "", setupTUIDimStyle.Render(defaultHint))
	}
	if m.currentStep().ID == setupStepAdmin {
		lines = append(lines, setupTUIDimStyle.Render("留空则使用扫码创建者 open_id。 / Leave blank to use the QR setup owner."))
	}
	_ = width
	return lines
}

func (m setupWizardTUIModel) renderInputLine(width int) string {
	if width < 8 {
		width = 8
	}
	runes := []rune(m.inputValue)
	displayRunes := runes
	if m.inputMasked {
		displayRunes = []rune(strings.Repeat("*", len(runes)))
	}
	cursor := min(max(0, m.inputCursor), len(displayRunes))
	inputWidth := max(1, width-2)
	if len(displayRunes) > inputWidth {
		start := max(0, cursor-inputWidth+1)
		if start+inputWidth > len(displayRunes) {
			start = len(displayRunes) - inputWidth
		}
		displayRunes = displayRunes[start : start+inputWidth]
		cursor -= start
	}
	if cursor >= inputWidth && len(displayRunes) >= inputWidth {
		displayRunes = displayRunes[1:]
		cursor = len(displayRunes)
	}
	var rendered string
	if len(displayRunes) == 0 {
		rendered = setupTUIDimStyle.Render(m.textPlaceholder(m.currentStep().ID))
		cursor = 0
	} else {
		before := string(displayRunes[:cursor])
		at := " "
		if cursor < len(displayRunes) {
			at = string(displayRunes[cursor])
		}
		after := ""
		if cursor+1 < len(displayRunes) {
			after = string(displayRunes[cursor+1:])
		}
		rendered = before + setupTUIAccentStyle.Reverse(true).Render(at) + after
	}
	return "> " + rendered
}

func (m setupWizardTUIModel) renderChoiceStep(stepID setupWizardStepID, width int) []string {
	choices := m.currentChoices(stepID)
	lines := make([]string, 0, len(choices)+1)
	for i, choice := range choices {
		prefix := "  "
		style := setupTUIDimStyle
		if i == m.cursor {
			prefix = "> "
			style = setupTUISelectedStyle
		}
		line := fmt.Sprintf("%s%d) %s", prefix, i+1, choice.Label)
		if strings.TrimSpace(choice.Hint) != "" {
			space := max(2, 28-lipgloss.Width(line))
			line += strings.Repeat(" ", space) + choice.Hint
		}
		lines = append(lines, style.Render(truncateWizardText(line, width-4)))
	}
	return lines
}

func (m setupWizardTUIModel) renderRegistrationExecStep(width int) []string {
	lines := []string{
		setupTUISelectedStyle.Render("正在打开扫码创建 / Opening QR onboarding..."),
		"",
		"Wizard 会暂停 TUI，让终端完整显示二维码。 / The wizard pauses the TUI so the terminal can show a clean QR code.",
		"下方会出现二维码和创建链接，请用飞书/Lark 扫码。 / The QR code and creation link will appear below; scan it in Feishu/Lark.",
		"",
		setupTUIDimStyle.Render("授权完成后 wizard 会自动继续。 / After authorization completes, this wizard will resume automatically."),
	}
	_ = width
	return lines
}

func (m setupWizardTUIModel) renderSummaryStep(width int) []string {
	lines := []string{
		setupTUIAccentSoftStyle.Render("配置 / Configuration"),
		m.summaryLine("配置文件 / Config", m.cfg.ConfigPath),
		m.summaryLine("机器人 / Bot", formatSetupWizardMode(m.cfg.Mode)),
		m.summaryLine("平台 / Platform", formatSetupWizardPlatform(m.cfg.PlatformType)),
		m.summaryLine("配置名 / Profile", m.cfg.Project),
		m.summaryLine("Agent 类型 / Agent", m.cfg.AgentType),
		m.summaryLine("工作目录 / Workspace", m.cfg.WorkDir),
		"",
		setupTUIAccentSoftStyle.Render("访问 / Access"),
		m.summaryLine("管理员 / Admin", formatSetupWizardAdmin(m.cfg.AdminOpenID)),
		m.summaryLine("创建者 / Creator", formatSetupWizardOptional(m.cfg.OwnerOpenID)),
		m.summaryLine("自动绑定 / Auto-bind", formatWizardBool(m.cfg.AutoBindChats)),
		m.summaryLine("群聊触发 / Trigger", formatSetupWizardGroupTrigger(m.cfg.GroupReplyAll)),
		m.summaryLine("群聊上下文 / Context", formatWizardBool(m.cfg.GroupContextBuffer)),
		m.summaryLine("后台服务 / Service", formatSetupWizardService(m.cfg.InstallAndStartService)),
		"",
	}
	actions := []string{"写入配置 / Write config", "返回修改 / Go back"}
	for i, action := range actions {
		prefix := "  "
		style := setupTUIDimStyle
		if i == m.cursor {
			prefix = "> "
			style = setupTUISelectedStyle
		}
		lines = append(lines, style.Render(prefix+action))
	}
	_ = width
	return lines
}

func (m setupWizardTUIModel) renderStatus(width int) string {
	step := m.currentStep()
	status := fmt.Sprintf(
		"profile %s | agent %s | mode %s | bot %s | service %s",
		emptyAs(m.cfg.Project, defaultFeishuProject),
		emptyAs(m.cfg.AgentType, "codex"),
		formatSetupWizardMode(m.cfg.Mode),
		formatSetupWizardBotPrepared(m.cfg.BotPrepared),
		formatSetupWizardService(m.cfg.InstallAndStartService),
	)
	if step.ID == setupStepSummary {
		status = "准备写入配置 / ready to write config"
	}
	if m.preparingBot {
		status = "等待扫码创建 / waiting for QR onboarding"
	}
	return setupTUIDimStyle.Width(width).Render(status)
}

func (m setupWizardTUIModel) summaryLine(label, value string) string {
	return fmt.Sprintf("  %-17s %s", label, setupTUIDimStyle.Render(value))
}

func (m setupWizardTUIModel) textValue(stepID setupWizardStepID) string {
	switch stepID {
	case setupStepConfig:
		return m.cfg.ConfigPath
	case setupStepAppID:
		return m.cfg.AppID
	case setupStepAppSecret:
		return m.cfg.AppSecret
	case setupStepProject:
		return m.cfg.Project
	case setupStepWorkDir:
		if strings.TrimSpace(m.cfg.WorkDir) == "" {
			workDirProject := m.cfg.Project
			if m.projectDefaulted && !m.projectEdited {
				workDirProject = ""
			}
			return defaultFeishuSetupWorkDirForConfig(m.cfg.ConfigPath, workDirProject)
		}
		return m.cfg.WorkDir
	case setupStepAdmin:
		if strings.TrimSpace(m.cfg.AdminOpenID) == "" && strings.TrimSpace(m.cfg.OwnerOpenID) != "" {
			return m.cfg.OwnerOpenID
		}
		return m.cfg.AdminOpenID
	default:
		return ""
	}
}

func (m setupWizardTUIModel) textPlaceholder(stepID setupWizardStepID) string {
	switch stepID {
	case setupStepAdmin:
		return "optional"
	case setupStepAppID:
		return "cli_xxx"
	case setupStepAppSecret:
		return "secret"
	default:
		return ""
	}
}

func (m setupWizardTUIModel) textDefaultHint(stepID setupWizardStepID) string {
	switch stepID {
	case setupStepConfig:
		return "该文件会保存 app_secret，请妥善保管。 / This file may contain app_secret; keep it private."
	case setupStepWorkDir:
		if m.projectDefaulted && !m.projectEdited {
			return "默认配置会在 config.toml 旁创建工作目录。 / Default profile uses a workspace next to config.toml."
		}
		return "显式配置默认使用当前目录，除非手动修改。 / Explicit profiles default to the current directory unless changed."
	default:
		return ""
	}
}

func (m *setupWizardTUIModel) commitTextStep(stepID setupWizardStepID) error {
	value := strings.TrimSpace(m.inputValue)
	switch stepID {
	case setupStepConfig:
		if value == "" {
			return fmt.Errorf("config file is required")
		}
		m.cfg.ConfigPath = value
	case setupStepAppID:
		if value == "" {
			return fmt.Errorf("app_id is required when connecting an existing bot")
		}
		m.cfg.AppID = value
	case setupStepAppSecret:
		if value == "" {
			return fmt.Errorf("app_secret is required when connecting an existing bot")
		}
		m.cfg.AppSecret = value
	case setupStepProject:
		if value == "" {
			value = defaultFeishuProject
		}
		m.projectEdited = value != defaultFeishuProject
		m.cfg.Project = value
	case setupStepWorkDir:
		if value == "" {
			return fmt.Errorf("workspace is required")
		}
		m.cfg.WorkDir = value
	case setupStepAdmin:
		m.cfg.AdminOpenID = value
	}
	return nil
}

func (m *setupWizardTUIModel) prepareBotIfReady() error {
	return prepareFeishuSetupWizardBot(&m.cfg)
}

func (m *setupWizardTUIModel) resetPreparedBot(clearCredentials bool) {
	previousOwner := strings.TrimSpace(m.cfg.OwnerOpenID)
	if previousOwner != "" && strings.TrimSpace(m.cfg.AdminOpenID) == previousOwner {
		m.cfg.AdminOpenID = ""
	}
	if clearCredentials {
		m.cfg.AppID = ""
		m.cfg.AppSecret = ""
	}
	m.cfg.BotPrepared = false
	m.cfg.OwnerOpenID = ""
}

func (m *setupWizardTUIModel) handleTextInput(msg tea.KeyMsg) {
	key := msg.String()
	runes := []rune(m.inputValue)
	m.inputCursor = min(max(0, m.inputCursor), len(runes))
	switch key {
	case "left", "ctrl+b":
		m.inputCursor = max(0, m.inputCursor-1)
		return
	case "right", "ctrl+f":
		m.inputCursor = min(len(runes), m.inputCursor+1)
		return
	case "home", "ctrl+a":
		m.inputCursor = 0
		return
	case "end", "ctrl+e":
		m.inputCursor = len(runes)
		return
	case "ctrl+u":
		m.inputValue = string(runes[m.inputCursor:])
		m.inputCursor = 0
		return
	case "backspace", "ctrl+h":
		if m.inputCursor == 0 {
			return
		}
		runes = append(runes[:m.inputCursor-1], runes[m.inputCursor:]...)
		m.inputCursor--
		m.inputValue = string(runes)
		return
	case "delete", "ctrl+d":
		if m.inputCursor >= len(runes) {
			return
		}
		runes = append(runes[:m.inputCursor], runes[m.inputCursor+1:]...)
		m.inputValue = string(runes)
		return
	}
	if len(msg.Runes) == 0 {
		return
	}
	insert := msg.Runes
	next := make([]rune, 0, len(runes)+len(insert))
	next = append(next, runes[:m.inputCursor]...)
	next = append(next, insert...)
	next = append(next, runes[m.inputCursor:]...)
	m.inputCursor += len(insert)
	m.inputValue = string(next)
}

func (m setupWizardTUIModel) currentChoices(stepID setupWizardStepID) []setupChoice {
	switch stepID {
	case setupStepMode:
		return []setupChoice{
			{Key: "create", Label: "扫码创建新机器人 / Create new bot", Hint: "扫码授权 / QR onboarding"},
			{Key: "connect", Label: "连接已有机器人 / Connect existing bot", Hint: "app_id/app_secret"},
		}
	case setupStepPlatform:
		return []setupChoice{
			{Key: "auto", Label: "自动检测 / Auto-detect", Hint: "飞书或 Lark / Feishu or Lark"},
			{Key: "feishu", Label: "飞书 / Feishu"},
			{Key: "lark", Label: "Lark / Lark"},
		}
	case setupStepAgent:
		return setupAgentChoices()
	case setupStepAutoBind, setupStepGroupContext, setupStepService:
		return []setupChoice{
			{Key: "yes", Label: "是 / Yes"},
			{Key: "no", Label: "否 / No"},
		}
	case setupStepGroupMode:
		return []setupChoice{
			{Key: "mention", Label: "仅 @ 时回复 / Mention only", Hint: "推荐 / recommended"},
			{Key: "all", Label: "每条群消息 / Every group message", Hint: "较吵 / noisy"},
		}
	default:
		return nil
	}
}

func (m setupWizardTUIModel) selectedChoiceIndex(stepID setupWizardStepID) int {
	selected := m.selectedChoiceKey(stepID)
	choices := m.currentChoices(stepID)
	for i, choice := range choices {
		if choice.Key == selected {
			return i
		}
	}
	return 0
}

func (m setupWizardTUIModel) selectedChoiceKey(stepID setupWizardStepID) string {
	switch stepID {
	case setupStepMode:
		if m.cfg.Mode == feishuSetupModeBind {
			return "connect"
		}
		return "create"
	case setupStepPlatform:
		if strings.TrimSpace(m.cfg.PlatformType) == "" {
			return "auto"
		}
		return m.cfg.PlatformType
	case setupStepAgent:
		return emptyAs(m.cfg.AgentType, "codex")
	case setupStepAutoBind:
		return boolChoiceKey(m.cfg.AutoBindChats)
	case setupStepGroupMode:
		if m.cfg.GroupReplyAll {
			return "all"
		}
		return "mention"
	case setupStepGroupContext:
		return boolChoiceKey(m.cfg.GroupContextBuffer)
	case setupStepService:
		return boolChoiceKey(m.cfg.InstallAndStartService)
	default:
		return ""
	}
}

func (m *setupWizardTUIModel) applyChoice(stepID setupWizardStepID, key string) {
	switch stepID {
	case setupStepMode:
		if key == "connect" {
			m.resetPreparedBot(m.cfg.Mode != feishuSetupModeBind)
			m.cfg.Mode = feishuSetupModeBind
			return
		}
		m.cfg.Mode = feishuSetupModeNew
		m.resetPreparedBot(true)
	case setupStepPlatform:
		if key == "auto" {
			m.cfg.PlatformType = ""
			return
		}
		m.cfg.PlatformType = key
	case setupStepAgent:
		m.cfg.AgentType = key
	case setupStepAutoBind:
		m.cfg.AutoBindChats = key == "yes"
	case setupStepGroupMode:
		m.cfg.GroupReplyAll = key == "all"
	case setupStepGroupContext:
		m.cfg.GroupContextBuffer = key == "yes"
	case setupStepService:
		m.cfg.InstallAndStartService = key == "yes"
	}
}

func (m setupWizardTUIModel) isBoolChoice(stepID setupWizardStepID) bool {
	switch stepID {
	case setupStepAutoBind, setupStepGroupContext, setupStepService:
		return true
	default:
		return false
	}
}

func formatSetupWizardMode(mode string) string {
	if mode == feishuSetupModeBind {
		return "连接已有 / connect_existing"
	}
	return "扫码创建 / create_new"
}

func formatSetupWizardPlatform(platform string) string {
	if strings.TrimSpace(platform) == "" {
		return "自动检测 / auto"
	}
	return platform
}

func formatSetupWizardAdmin(admin string) string {
	if strings.TrimSpace(admin) == "" {
		return "扫码创建者 / creator_open_id"
	}
	return admin
}

func formatSetupWizardOptional(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func formatSetupWizardBotPrepared(prepared bool) string {
	if prepared {
		return "已准备 / ready"
	}
	return "未完成 / pending"
}

func formatSetupWizardGroupTrigger(replyAll bool) string {
	if replyAll {
		return "每条消息 / all_messages"
	}
	return "仅 @ / mention_only"
}

func formatSetupWizardService(install bool) string {
	if install {
		return "安装并启动 / install_and_start"
	}
	return "仅写配置 / config_only"
}

func boolChoiceKey(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func emptyAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func truncateWizardText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "."
}
