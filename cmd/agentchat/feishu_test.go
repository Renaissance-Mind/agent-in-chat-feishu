package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Renaissance-Mind/agent-in-chat-feishu/config"
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

	appID, platformType, err := resolveFeishuPermissionTarget("demo", "lark", 0)
	if err != nil {
		t.Fatalf("resolveFeishuPermissionTarget returned error: %v", err)
	}
	if appID != "cli_lark" || platformType != "lark" {
		t.Fatalf("target = (%q, %q), want (cli_lark, lark)", appID, platformType)
	}

	appID, platformType, err = resolveFeishuPermissionTarget("demo", "", 2)
	if err != nil {
		t.Fatalf("resolveFeishuPermissionTarget with index returned error: %v", err)
	}
	if appID != "cli_lark" || platformType != "lark" {
		t.Fatalf("indexed target = (%q, %q), want (cli_lark, lark)", appID, platformType)
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
