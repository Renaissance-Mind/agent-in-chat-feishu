package contextbuilder

import (
	"strings"
	"testing"
	"time"

	"github.com/sariel/agent-in-chat-feishu/internal/identity"
)

func TestRenderHistoryFiltersCardsAndResolvesNames(t *testing.T) {
	cache, err := identity.Open("")
	if err != nil {
		t.Fatalf("identity.Open() error = %v", err)
	}
	cache.PutUser("ou_user", "用户267197")
	cache.PutBot("ou_test_bot", "测试机器人")
	cache.PutApp("cli_self", "椿楸Codex")

	base := time.Date(2026, 6, 9, 11, 35, 0, 0, time.Local)
	entries := []Entry{
		{MessageID: "om_user", SenderID: "ou_user", SenderType: "user", CreatedAt: base, Content: "123"},
		{MessageID: "om_bot", SenderID: "ou_test_bot", SenderType: "app", CreatedAt: base.Add(time.Minute), Content: "查看天气"},
		{MessageID: "om_card", SenderID: "cli_other", SenderType: "app", CreatedAt: base.Add(2 * time.Minute), Content: "progress card", MsgType: "interactive"},
		{MessageID: "om_self", SenderID: "cli_self", SenderType: "app", CreatedAt: base.Add(3 * time.Minute), Content: "收到"},
		{MessageID: "om_current", SenderID: "ou_user", SenderType: "user", CreatedAt: base.Add(4 * time.Minute), Content: "@椿楸Codex 继续"},
	}

	got := RenderHistory(entries, "om_current", cache)
	if !strings.Contains(got, "[11:35 用户267197] 123") {
		t.Fatalf("history missing user name:\n%s", got)
	}
	if !strings.Contains(got, "[11:36 测试机器人] 查看天气") {
		t.Fatalf("history missing bot name:\n%s", got)
	}
	if !strings.Contains(got, "[11:38 椿楸Codex] 收到") {
		t.Fatalf("history missing self app name:\n%s", got)
	}
	if strings.Contains(got, "progress card") {
		t.Fatalf("interactive card should be filtered:\n%s", got)
	}
	if strings.Contains(got, "继续") {
		t.Fatalf("current trigger should be excluded:\n%s", got)
	}
}
