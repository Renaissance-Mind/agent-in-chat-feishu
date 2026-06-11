package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Renaissance-Mind/agent-in-chat-feishu/config"
	"github.com/Renaissance-Mind/agent-in-chat-feishu/daemon"
	tea "github.com/charmbracelet/bubbletea"
)

func TestResolveFeishuSetupInputs_AutoModeWithoutCredentialsUsesNew(t *testing.T) {
	mode, appID, appSecret, err := resolveFeishuSetupInputs(feishuSetupModeAuto, "", "", "")
	if err != nil {
		t.Fatalf("resolveFeishuSetupInputs returned error: %v", err)
	}
	if mode != feishuSetupModeNew {
		t.Fatalf("mode = %q, want %q", mode, feishuSetupModeNew)
	}
	if appID != "" || appSecret != "" {
		t.Fatalf("credentials should be empty, got appID=%q appSecret=%q", appID, appSecret)
	}
}

func TestResolveFeishuSetupInputs_AutoModeWithAppUsesBind(t *testing.T) {
	mode, appID, appSecret, err := resolveFeishuSetupInputs(feishuSetupModeAuto, "cli_xxx:sec_xxx", "", "")
	if err != nil {
		t.Fatalf("resolveFeishuSetupInputs returned error: %v", err)
	}
	if mode != feishuSetupModeBind {
		t.Fatalf("mode = %q, want %q", mode, feishuSetupModeBind)
	}
	if appID != "cli_xxx" || appSecret != "sec_xxx" {
		t.Fatalf("credentials = (%q, %q), want (%q, %q)", appID, appSecret, "cli_xxx", "sec_xxx")
	}
}

func TestResolveFeishuSetupInputs_BindRequiresCredentials(t *testing.T) {
	_, _, _, err := resolveFeishuSetupInputs(feishuSetupModeBind, "", "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveFeishuSetupInputs_RejectsMixedCredentialFlags(t *testing.T) {
	_, _, _, err := resolveFeishuSetupInputs(feishuSetupModeAuto, "cli_xxx:sec_xxx", "cli_xxx", "sec_xxx")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseAppPair_SecretCanContainColon(t *testing.T) {
	appID, appSecret, err := parseAppPair("cli_xxx:sec:yyy")
	if err != nil {
		t.Fatalf("parseAppPair returned error: %v", err)
	}
	if appID != "cli_xxx" || appSecret != "sec:yyy" {
		t.Fatalf("result = (%q, %q), want (%q, %q)", appID, appSecret, "cli_xxx", "sec:yyy")
	}
}

func TestSetupOwnerOpenIDForConfigRejectsBotOpenID(t *testing.T) {
	got := setupOwnerOpenIDForConfig("ou_bot", "ou_bot")
	if got != "" {
		t.Fatalf("setupOwnerOpenIDForConfig = %q, want empty when owner matches bot", got)
	}
}

func TestSetupOwnerOpenIDForConfigKeepsUserOpenID(t *testing.T) {
	got := setupOwnerOpenIDForConfig(" ou_user ", "ou_bot")
	if got != "ou_user" {
		t.Fatalf("setupOwnerOpenIDForConfig = %q, want trimmed user open_id", got)
	}
}

func TestFeishuSetupWizardCollectsKimiBindConfig(t *testing.T) {
	prevValidate := setupWizardValidateAppCredentials
	setupWizardValidateAppCredentials = func(appID, appSecret, platformType string) (string, error) {
		if appID != "cli_kimi" || appSecret != "sec_kimi" || platformType != "feishu" {
			t.Fatalf("validate args = (%q, %q, %q)", appID, appSecret, platformType)
		}
		return "feishu", nil
	}
	t.Cleanup(func() { setupWizardValidateAppCredentials = prevValidate })

	input := strings.NewReader(strings.Join([]string{
		"",             // config file
		"connect",      // bot setup mode
		"cli_kimi",     // app id
		"sec_kimi",     // app secret
		"feishu",       // platform
		"kimi-profile", // local profile
		"kimi",         // agent
		"/tmp/kimi",    // initial workspace
		"ou_admin",     // admin open_id
		"",             // auto-bind chats
		"",             // group trigger mode
		"",             // include group history
		"no",           // install/start service
		"yes",          // confirm
	}, "\n") + "\n")

	got, err := runFeishuSetupWizard(input, io.Discard, feishuSetupWizardConfig{
		ConfigPath:             "/tmp/agentchat/config.toml",
		Mode:                   feishuSetupModeNew,
		Project:                defaultFeishuProject,
		AgentType:              "codex",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: true,
	})
	if err != nil {
		t.Fatalf("runFeishuSetupWizard returned error: %v", err)
	}
	if got.Mode != feishuSetupModeBind {
		t.Fatalf("Mode = %q, want %q", got.Mode, feishuSetupModeBind)
	}
	if got.AppID != "cli_kimi" || got.AppSecret != "sec_kimi" {
		t.Fatalf("credentials = (%q, %q), want kimi credentials", got.AppID, got.AppSecret)
	}
	if got.PlatformType != "feishu" {
		t.Fatalf("PlatformType = %q, want feishu", got.PlatformType)
	}
	if !got.BotPrepared {
		t.Fatal("BotPrepared = false, want true after validating credentials")
	}
	if got.Project != "kimi-profile" {
		t.Fatalf("Project = %q, want kimi-profile", got.Project)
	}
	if got.AgentType != "kimi" {
		t.Fatalf("AgentType = %q, want kimi", got.AgentType)
	}
	if got.WorkDir != "/tmp/kimi" {
		t.Fatalf("WorkDir = %q, want /tmp/kimi", got.WorkDir)
	}
	if got.AdminOpenID != "ou_admin" {
		t.Fatalf("AdminOpenID = %q, want ou_admin", got.AdminOpenID)
	}
	if !got.AutoBindChats || got.GroupReplyAll || !got.GroupContextBuffer || !got.ShareSessionInChannel || !got.EnableFeishuCard {
		t.Fatalf("unexpected defaults: %+v", got)
	}
	if got.InstallAndStartService {
		t.Fatalf("InstallAndStartService = true, want false")
	}
}

func TestFeishuSetupWizardDefaultsWorkspaceNextToConfig(t *testing.T) {
	prevRegister := setupWizardRunRegistrationFlow
	setupWizardRunRegistrationFlow = func(opts registrationFlowOptions) (*registrationFlowResult, error) {
		return &registrationFlowResult{
			AppID:       "cli_default",
			AppSecret:   "sec_default",
			OwnerOpenID: "ou_default_admin",
			Platform:    "feishu",
		}, nil
	}
	t.Cleanup(func() { setupWizardRunRegistrationFlow = prevRegister })

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	input := strings.NewReader(strings.Join([]string{
		"",    // config file
		"",    // bot setup mode
		"",    // local profile
		"",    // agent
		"",    // initial workspace
		"",    // admin open_id
		"",    // auto-bind chats
		"",    // group trigger mode
		"",    // include group history
		"no",  // install/start service
		"yes", // confirm
	}, "\n") + "\n")

	got, err := runFeishuSetupWizard(input, io.Discard, feishuSetupWizardConfig{
		ConfigPath:             configPath,
		Mode:                   feishuSetupModeNew,
		AgentType:              "codex",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: true,
	})
	if err != nil {
		t.Fatalf("runFeishuSetupWizard returned error: %v", err)
	}
	if got.WorkDir != filepath.Join(dir, defaultFeishuProject) {
		t.Fatalf("WorkDir = %q, want default next to config", got.WorkDir)
	}
	if got.AdminOpenID != "ou_default_admin" {
		t.Fatalf("AdminOpenID = %q, want owner open_id default", got.AdminOpenID)
	}
}

func TestFeishuSetupWizardRendersCompactSections(t *testing.T) {
	prevRegister := setupWizardRunRegistrationFlow
	setupWizardRunRegistrationFlow = func(opts registrationFlowOptions) (*registrationFlowResult, error) {
		return &registrationFlowResult{
			AppID:       "cli_render",
			AppSecret:   "sec_render",
			OwnerOpenID: "ou_render_admin",
			Platform:    "feishu",
		}, nil
	}
	t.Cleanup(func() { setupWizardRunRegistrationFlow = prevRegister })

	input := strings.NewReader(strings.Join([]string{
		"",    // config file
		"",    // bot setup mode
		"",    // local profile
		"",    // agent
		"",    // initial workspace
		"",    // admin open_id
		"",    // auto-bind chats
		"",    // group trigger mode
		"",    // include group history
		"no",  // install/start service
		"yes", // confirm
	}, "\n") + "\n")
	var out strings.Builder

	_, err := runFeishuSetupWizard(input, &out, feishuSetupWizardConfig{
		ConfigPath:             "/tmp/agentchat/config.toml",
		Mode:                   feishuSetupModeNew,
		AgentType:              "codex",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: true,
	})
	if err != nil {
		t.Fatalf("runFeishuSetupWizard returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Agent-in-Chat-Feishu setup",
		"配置 / Config\n  保存凭证",
		"机器人 / Bot\n  扫码创建机器人",
		"本地 Agent / Local agent\n  选择配置名",
		"聊天访问 / Chat access\n  默认绑定模式",
		"群聊行为 / Group behavior\n  设置机器人何时回复",
		"运行方式 / Runtime\n  立即启动后台服务",
		"摘要 / Summary\n  写入配置前确认",
		"Select [create]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("wizard output missing %q:\n%s", want, text)
		}
	}
	for _, oldMarker := range []string{"? ", "Default:", "Shared group session", "Progress cards", "share group session", "progress cards"} {
		if strings.Contains(text, oldMarker) {
			t.Fatalf("wizard output still contains old marker %q:\n%s", oldMarker, text)
		}
	}
}

func TestFeishuSetupWizardTUIViewShowsShellLayout(t *testing.T) {
	model := newSetupWizardTUIModel(feishuSetupWizardConfig{
		ConfigPath:             "/tmp/agentchat/config.toml",
		Mode:                   feishuSetupModeBind,
		Project:                "kimi-profile",
		AgentType:              "kimi",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: false,
	})
	model.width = 100
	model.height = 30

	view := model.View()
	for _, want := range []string{
		"Agent-in-Chat-Feishu setup - step 1/",
		"Setup",
		"配置文件 / Config",
		"Config file",
		"Where agentchat stores credentials",
		"profile kimi-profile | agent kimi",
		"mode 连接已有 / connect_existing",
		"bot 未完成 / pending",
		"service 仅写配置 / config_only",
		"enter 选择/下一步 select/next",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("TUI view missing %q:\n%s", want, view)
		}
	}
}

func TestFeishuSetupWizardTUIConnectModeShowsCredentialSteps(t *testing.T) {
	model := newSetupWizardTUIModel(feishuSetupWizardConfig{
		ConfigPath:             "/tmp/agentchat/config.toml",
		Mode:                   feishuSetupModeNew,
		AgentType:              "codex",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: true,
	})
	model.stepIndex = 1
	model.syncCurrentStep()
	model.applyChoice(setupStepMode, "connect")
	model.advance()

	if got := model.currentStep().ID; got != setupStepAppID {
		t.Fatalf("current step = %v, want setupStepAppID", got)
	}
}

func TestFeishuSetupWizardTUIPreparesScannedBotBeforeAdminStep(t *testing.T) {
	prevRegister := setupWizardRunRegistrationFlow
	setupWizardRunRegistrationFlow = func(opts registrationFlowOptions) (*registrationFlowResult, error) {
		return &registrationFlowResult{
			AppID:       "cli_scanned",
			AppSecret:   "sec_scanned",
			OwnerOpenID: "ou_real_admin",
			Platform:    "feishu",
		}, nil
	}
	t.Cleanup(func() { setupWizardRunRegistrationFlow = prevRegister })

	model := newSetupWizardTUIModel(feishuSetupWizardConfig{
		ConfigPath:             "/tmp/agentchat/config.toml",
		Mode:                   feishuSetupModeNew,
		AgentType:              "codex",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: true,
	})
	model.applyChoice(setupStepMode, "create")
	if err := model.prepareBotIfReady(); err != nil {
		t.Fatalf("prepareBotIfReady returned error: %v", err)
	}

	if !model.cfg.BotPrepared {
		t.Fatal("BotPrepared = false, want true after QR onboarding")
	}
	if model.cfg.AppID != "cli_scanned" || model.cfg.AppSecret != "sec_scanned" {
		t.Fatalf("credentials = (%q, %q), want scanned credentials", model.cfg.AppID, model.cfg.AppSecret)
	}
	if model.cfg.OwnerOpenID != "ou_real_admin" {
		t.Fatalf("OwnerOpenID = %q, want real owner", model.cfg.OwnerOpenID)
	}
	if got := model.textValue(setupStepAdmin); got != "ou_real_admin" {
		t.Fatalf("admin default = %q, want real owner open_id", got)
	}
	if got := model.currentStepIDs(); containsSetupStep(got, setupStepPlatform) || containsSetupStep(got, setupStepAppID) {
		t.Fatalf("prepared scanned bot should skip credential/platform steps, got %v", got)
	}
}

func TestFeishuSetupWizardTUISelectCreatePreparesBotAndAdvancesToProfile(t *testing.T) {
	model := newSetupWizardTUIModel(feishuSetupWizardConfig{
		ConfigPath:             "/tmp/agentchat/config.toml",
		Mode:                   feishuSetupModeNew,
		AgentType:              "codex",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: true,
	})
	model.stepIndex = 1
	model.syncCurrentStep()

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	gotModel := next.(setupWizardTUIModel)

	if cmd == nil {
		t.Fatal("registration command = nil, want QR onboarding command")
	}
	if !gotModel.preparingBot {
		t.Fatal("preparingBot = false, want true while QR onboarding runs")
	}
	if got := gotModel.currentStep().ID; got != setupStepMode {
		t.Fatalf("current step = %v, want setupStepMode while onboarding runs", got)
	}

	next, _ = gotModel.Update(setupWizardRegistrationDoneMsg{
		result: &registrationFlowResult{
			AppID:       "cli_created",
			AppSecret:   "sec_created",
			OwnerOpenID: "ou_created_admin",
			Platform:    "feishu",
		},
	})
	gotModel = next.(setupWizardTUIModel)

	if got := gotModel.currentStep().ID; got != setupStepProject {
		t.Fatalf("current step = %v, want setupStepProject", got)
	}
	if !gotModel.cfg.BotPrepared {
		t.Fatal("BotPrepared = false, want true after selecting create")
	}
	if got := gotModel.textValue(setupStepAdmin); got != "ou_created_admin" {
		t.Fatalf("admin default = %q, want real owner open_id", got)
	}
}

func TestFeishuSetupWizardTUISwitchModeClearsAutoOwnerAdmin(t *testing.T) {
	model := newSetupWizardTUIModel(feishuSetupWizardConfig{
		ConfigPath:             "/tmp/agentchat/config.toml",
		Mode:                   feishuSetupModeNew,
		AppID:                  "cli_created",
		AppSecret:              "sec_created",
		BotPrepared:            true,
		OwnerOpenID:            "ou_created_admin",
		AdminOpenID:            "ou_created_admin",
		AgentType:              "codex",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: true,
	})

	model.applyChoice(setupStepMode, "connect")

	if model.cfg.BotPrepared {
		t.Fatal("BotPrepared = true, want reset after switching mode")
	}
	if model.cfg.OwnerOpenID != "" || model.cfg.AdminOpenID != "" {
		t.Fatalf("owner/admin = (%q, %q), want cleared auto owner admin", model.cfg.OwnerOpenID, model.cfg.AdminOpenID)
	}
	if model.cfg.AppID != "" || model.cfg.AppSecret != "" {
		t.Fatalf("credentials = (%q, %q), want cleared QR credentials", model.cfg.AppID, model.cfg.AppSecret)
	}
}

func TestFeishuSetupWizardTUIPreparesBoundBotBeforeAdminStep(t *testing.T) {
	prevValidate := setupWizardValidateAppCredentials
	setupWizardValidateAppCredentials = func(appID, appSecret, platformType string) (string, error) {
		if appID != "cli_bound" || appSecret != "sec_bound" || platformType != "" {
			t.Fatalf("validate args = (%q, %q, %q)", appID, appSecret, platformType)
		}
		return "lark", nil
	}
	t.Cleanup(func() { setupWizardValidateAppCredentials = prevValidate })

	model := newSetupWizardTUIModel(feishuSetupWizardConfig{
		ConfigPath:             "/tmp/agentchat/config.toml",
		Mode:                   feishuSetupModeBind,
		AppID:                  "cli_bound",
		AppSecret:              "sec_bound",
		AgentType:              "codex",
		AutoBindChats:          true,
		GroupContextBuffer:     true,
		ShareSessionInChannel:  true,
		EnableFeishuCard:       true,
		InstallAndStartService: true,
	})
	model.applyChoice(setupStepPlatform, "auto")
	if err := model.prepareBotIfReady(); err != nil {
		t.Fatalf("prepareBotIfReady returned error: %v", err)
	}

	if !model.cfg.BotPrepared {
		t.Fatal("BotPrepared = false, want true after credential validation")
	}
	if model.cfg.PlatformType != "lark" {
		t.Fatalf("PlatformType = %q, want detected lark", model.cfg.PlatformType)
	}
	if got := model.currentStepIDs(); containsSetupStep(got, setupStepAppID) || containsSetupStep(got, setupStepPlatform) {
		t.Fatalf("prepared bound bot should skip credential/platform steps, got %v", got)
	}
}

func TestResolveTargetProjectDefaultsToFeishu(t *testing.T) {
	dir := t.TempDir()
	prev := config.ConfigPath
	config.ConfigPath = filepath.Join(dir, "config.toml")
	t.Cleanup(func() { config.ConfigPath = prev })

	got, err := resolveTargetProject("")
	if err != nil {
		t.Fatalf("resolveTargetProject returned error: %v", err)
	}
	if got != "feishu" {
		t.Fatalf("resolveTargetProject = %q, want feishu", got)
	}
}

func TestInstallFeishuSetupDaemonUsesConfigDirectoryAndReinstalls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte("language = \"zh\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	prevConfigPath := config.ConfigPath
	config.ConfigPath = configPath
	t.Cleanup(func() { config.ConfigPath = prevConfigPath })

	mgr := &recordingDaemonManager{status: &daemon.Status{Installed: true, Platform: "testd"}}
	prevFactory := daemonManagerFactory
	daemonManagerFactory = func() (daemon.Manager, error) { return mgr, nil }
	t.Cleanup(func() { daemonManagerFactory = prevFactory })

	result, err := installFeishuSetupDaemon("/env/bin")
	if err != nil {
		t.Fatalf("installFeishuSetupDaemon returned error: %v", err)
	}
	if !mgr.installCalled {
		t.Fatal("expected daemon install to be called even when service already exists")
	}
	if mgr.installCfg.WorkDir != dir {
		t.Fatalf("WorkDir = %q, want config directory %q", mgr.installCfg.WorkDir, dir)
	}
	if mgr.installCfg.EnvPATH != "/env/bin" {
		t.Fatalf("EnvPATH = %q, want forwarded daemon PATH", mgr.installCfg.EnvPATH)
	}
	if result.WorkDir != dir {
		t.Fatalf("result WorkDir = %q, want %q", result.WorkDir, dir)
	}
}

func TestBuildFeishuPermissionGuidancePutsScopeApplyURLLast(t *testing.T) {
	guidance := buildFeishuPermissionGuidance("feishu", "cli_abc")
	if guidance.ScopeApplyURL == "" {
		t.Fatal("ScopeApplyURL is empty")
	}
	output := guidance.String()
	if !strings.Contains(output, "权限确认直达链接") {
		t.Fatalf("guidance output missing direct permission link label:\n%s", output)
	}

	trimmed := strings.TrimSpace(output)
	if !strings.HasSuffix(trimmed, guidance.ScopeApplyURL) {
		t.Fatalf("guidance should end with scope apply URL\noutput:\n%s\nurl:\n%s", output, guidance.ScopeApplyURL)
	}
	if strings.LastIndex(output, guidance.ScopeApplyURL) < strings.LastIndex(output, "事件订阅") {
		t.Fatalf("scope apply URL should appear after event guidance:\n%s", output)
	}
}

func TestResolveFeishuPermissionTargetReadsConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work dir: %v", err)
	}
	content := strings.ReplaceAll(`
language = "zh"

[[projects]]
name = "demo"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "__WORK_DIR__"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "cli_feishu"
app_secret = "sec_feishu"

[[projects.platforms]]
type = "lark"

[projects.platforms.options]
app_id = "cli_lark"
app_secret = "sec_lark"
`, "__WORK_DIR__", workDir)
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	prev := config.ConfigPath
	config.ConfigPath = configPath
	t.Cleanup(func() { config.ConfigPath = prev })

	target, err := resolveFeishuPermissionTarget("demo", "lark", 0)
	if err != nil {
		t.Fatalf("resolveFeishuPermissionTarget returned error: %v", err)
	}
	if target.appID != "cli_lark" || target.appSecret != "sec_lark" || target.platformType != "lark" {
		t.Fatalf("target = (%q, %q, %q), want (cli_lark, sec_lark, lark)", target.appID, target.appSecret, target.platformType)
	}

	target, err = resolveFeishuPermissionTarget("demo", "", 2)
	if err != nil {
		t.Fatalf("resolveFeishuPermissionTarget with index returned error: %v", err)
	}
	if target.appID != "cli_lark" || target.appSecret != "sec_lark" || target.platformType != "lark" {
		t.Fatalf("indexed target = (%q, %q, %q), want (cli_lark, sec_lark, lark)", target.appID, target.appSecret, target.platformType)
	}
}

func TestApplyFeishuPermissionRequest(t *testing.T) {
	var sawToken bool
	var sawApply bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			sawToken = true
			if r.Method != http.MethodPost {
				t.Fatalf("token method = %s, want POST", r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode token body: %v", err)
			}
			if body["app_id"] != "cli_ok" || body["app_secret"] != "sec_ok" {
				t.Fatalf("token body = %#v, want app credentials", body)
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","tenant_access_token":"tenant-token"}`))

		case "/open-apis/application/v6/scopes/apply":
			sawApply = true
			if r.Method != http.MethodPost {
				t.Fatalf("apply method = %s, want POST", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer tenant-token" {
				t.Fatalf("authorization = %q, want bearer token", got)
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"success","data":{}}`))

		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	result, err := applyFeishuPermissionRequest(context.Background(), srv.URL, "cli_ok", "sec_ok", srv.Client())
	if err != nil {
		t.Fatalf("applyFeishuPermissionRequest returned error: %v", err)
	}
	if result.Code != 0 || !result.Success {
		t.Fatalf("result = %+v, want success", result)
	}
	if !sawToken || !sawApply {
		t.Fatalf("sawToken=%v sawApply=%v, want both requests", sawToken, sawApply)
	}
}

func TestSaveQRCodeImage_CreatesPNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-qr.png")

	if err := saveQRCodeImage("https://example.com/test", path); err != nil {
		t.Fatalf("saveQRCodeImage failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if len(data) < 100 {
		t.Fatalf("PNG file too small: %d bytes", len(data))
	}
	// PNG magic bytes
	if data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Fatal("output file is not a valid PNG")
	}
}

func TestSaveQRCodeImage_InvalidPath(t *testing.T) {
	err := saveQRCodeImage("https://example.com", "/nonexistent/dir/qr.png")
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPollRegistrationUntilComplete_RetriesTransientPollError(t *testing.T) {
	calls := 0
	client := &registrationClient{
		baseURL: "https://example.test",
		http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return nil, context.DeadlineExceeded
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"client_id": "cli_ok",
					"client_secret": "sec_ok",
					"user_info": {
						"open_id": "ou_user",
						"tenant_brand": "feishu"
					}
				}`)),
			}, nil
		})},
	}

	got, err := pollRegistrationUntilComplete(client, "device-code", 1, time.Now().Add(time.Second), func(time.Duration) {})
	if err != nil {
		t.Fatalf("pollRegistrationUntilComplete returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if got.AppID != "cli_ok" || got.AppSecret != "sec_ok" || got.OwnerOpenID != "ou_user" || got.Platform != "feishu" {
		t.Fatalf("result = %+v, want configured Feishu app", got)
	}
}
