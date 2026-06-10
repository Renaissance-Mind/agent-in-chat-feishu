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
