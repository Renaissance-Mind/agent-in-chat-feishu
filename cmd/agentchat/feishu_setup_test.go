package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestFeishuPermissionURLIncludesRequiredScopes(t *testing.T) {
	raw := feishuPermissionURL(openFeishuBaseURL, "cli_test")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse permission URL: %v", err)
	}
	if got, want := parsed.Scheme+"://"+parsed.Host+parsed.Path, openFeishuBaseURL+"/app/cli_test/auth"; got != want {
		t.Fatalf("permission URL base = %q, want %q", got, want)
	}
	if parsed.Query().Get("op_from") != "openapi" {
		t.Fatalf("op_from = %q, want openapi", parsed.Query().Get("op_from"))
	}
	if parsed.Query().Get("token_type") != "tenant" {
		t.Fatalf("token_type = %q, want tenant", parsed.Query().Get("token_type"))
	}
	scopes := strings.Split(parsed.Query().Get("q"), ",")
	if len(scopes) != len(requiredFeishuScopes) {
		t.Fatalf("scope count = %d, want %d", len(scopes), len(requiredFeishuScopes))
	}
	for _, want := range []string{
		"im:message.group_at_msg:readonly",
		"im:message.group_msg",
		"im:message:send_as_bot",
		"im:message.reactions:write_only",
		"im:chat.members:read",
		"contact:user.base:readonly",
	} {
		if !containsString(scopes, want) {
			t.Fatalf("permission URL missing scope %q", want)
		}
	}
}

func TestResolveFeishuSetupInputsAutoBind(t *testing.T) {
	mode, appID, appSecret, err := resolveFeishuSetupInputs(feishuSetupModeAuto, "cli_xxx:sec_xxx", "", "")
	if err != nil {
		t.Fatalf("resolve setup inputs: %v", err)
	}
	if mode != feishuSetupModeBind || appID != "cli_xxx" || appSecret != "sec_xxx" {
		t.Fatalf("resolved mode/app = %q/%q/%q", mode, appID, appSecret)
	}
}
