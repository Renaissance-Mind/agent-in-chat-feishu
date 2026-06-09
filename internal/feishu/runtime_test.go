package feishu

import (
	"strings"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/sariel/agent-in-chat-feishu/internal/config"
)

func TestRuntimeAllowsConfiguredChatsOnly(t *testing.T) {
	r := &Runtime{cfg: config.Config{Feishu: config.FeishuConfig{AllowedChats: []string{"oc_allowed"}}}}

	if !r.chatAllowed("oc_allowed") {
		t.Fatal("chatAllowed(allowed) = false, want true")
	}
	if r.chatAllowed("oc_other") {
		t.Fatal("chatAllowed(other) = true, want false")
	}
}

func TestMentionHelpersStripOnlyBotMention(t *testing.T) {
	botOpenID := "ou_bot"
	otherOpenID := "ou_other"
	botKey := "@_user_1"
	otherKey := "@_user_2"
	otherName := "赵雪坤"
	mentions := []*larkim.MentionEvent{
		{Key: &botKey, Id: &larkim.UserId{OpenId: &botOpenID}, Name: strPtr("椿楸Codex")},
		{Key: &otherKey, Id: &larkim.UserId{OpenId: &otherOpenID}, Name: &otherName},
	}

	if !isBotMentioned(mentions, botOpenID) {
		t.Fatal("isBotMentioned() = false, want true")
	}
	got := stripMentions("@_user_1 帮我看 @_user_2 的结论", mentions, botOpenID)
	if got != "帮我看 @赵雪坤 的结论" {
		t.Fatalf("stripMentions() = %q", got)
	}
}

func TestBuildPromptIncludesHistoryAndCurrentTrigger(t *testing.T) {
	prompt := buildPrompt("[Feishu group history]\n[11:35 用户267197] 前文", "赵雪坤", "现在怎么做")

	for _, want := range []string{"[Feishu group history]", "[Current trigger]", "赵雪坤: 现在怎么做"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func strPtr(s string) *string {
	return &s
}
