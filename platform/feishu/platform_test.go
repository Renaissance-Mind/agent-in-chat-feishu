package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/Renaissance-Mind/agent-in-chat-feishu/config"
	"github.com/Renaissance-Mind/agent-in-chat-feishu/core"
	callback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestNew_DefaultsToInteractivePlatform(t *testing.T) {
	p, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, ok := p.(core.CardSender); !ok {
		t.Fatal("expected default Feishu platform to implement core.CardSender")
	}
}

func TestNew_CanDisableInteractiveCards(t *testing.T) {
	p, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, ok := p.(core.CardSender); ok {
		t.Fatal("expected disabled Feishu platform to fall back to plain text")
	}
}

func TestNew_DisabledInteractiveCardsDoesNotStartPreviewCard(t *testing.T) {
	pAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p, ok := pAny.(*Platform)
	if !ok {
		t.Fatalf("platform type = %T, want *Platform", pAny)
	}

	_, err = p.SendPreviewStart(context.Background(), replyContext{messageID: "om_x", chatID: "oc_x"}, "hello")
	if err == nil {
		t.Fatal("SendPreviewStart() error = nil, want not supported when cards are disabled")
	}
	if err != core.ErrNotSupported {
		t.Fatalf("SendPreviewStart() error = %v, want %v", err, core.ErrNotSupported)
	}
}

func TestNew_ProgressStyleDefaultLegacy(t *testing.T) {
	p, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	sp, ok := p.(core.ProgressStyleProvider)
	if !ok {
		t.Fatalf("platform type %T does not implement ProgressStyleProvider", p)
	}
	if got := sp.ProgressStyle(); got != "legacy" {
		t.Fatalf("ProgressStyle() = %q, want legacy", got)
	}
}

func TestNew_ProgressStyleSupportsCompactAndCard(t *testing.T) {
	tests := []string{"compact", "card"}
	for _, style := range tests {
		t.Run(style, func(t *testing.T) {
			p, err := New(map[string]any{
				"app_id":         "cli_xxx",
				"app_secret":     "secret",
				"progress_style": style,
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			sp, ok := p.(core.ProgressStyleProvider)
			if !ok {
				t.Fatalf("platform type %T does not implement ProgressStyleProvider", p)
			}
			if got := sp.ProgressStyle(); got != style {
				t.Fatalf("ProgressStyle() = %q, want %q", got, style)
			}
			payloadCap, ok := p.(core.ProgressCardPayloadSupport)
			if !ok {
				t.Fatalf("platform type %T does not implement ProgressCardPayloadSupport", p)
			}
			if !payloadCap.SupportsProgressCardPayload() {
				t.Fatal("SupportsProgressCardPayload() = false, want true")
			}
		})
	}
}

func TestNew_ProgressStyleRejectsInvalidValue(t *testing.T) {
	_, err := New(map[string]any{
		"app_id":         "cli_xxx",
		"app_secret":     "secret",
		"progress_style": "invalid-style",
	})
	if err == nil {
		t.Fatal("expected error for invalid progress_style")
	}
	if !strings.Contains(err.Error(), "invalid progress_style") {
		t.Fatalf("error = %q, want invalid progress_style", err.Error())
	}
}

func TestFeishu_ChatBindingAccess(t *testing.T) {
	pAny, err := New(map[string]any{
		"app_id":              "cli_xxx",
		"app_secret":          "secret",
		"allow_from":          "ou_owner",
		"allow_private_chats": "oc_private",
		"allow_group_chats":   "oc_group",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p := pAny.(*interactivePlatform)

	if !p.chatAccessAllowed("p2p", "oc_private", "ou_other") {
		t.Fatal("bound private chat should be allowed regardless of allow_from")
	}
	if p.chatAccessAllowed("p2p", "oc_other", "ou_owner") {
		t.Fatal("unbound private chat should be denied even for allow_from user")
	}
	if !p.chatAccessAllowed("group", "oc_group", "ou_other") {
		t.Fatal("bound group should allow any sender")
	}
	if p.chatAccessAllowed("group", "oc_other", "ou_owner") {
		t.Fatal("unbound group should be denied even for allow_from user")
	}
}

func TestFeishu_ReloadPlatformConfigUpdatesChatBindings(t *testing.T) {
	pAny, err := New(map[string]any{
		"app_id":              "cli_xxx",
		"app_secret":          "secret",
		"allow_from":          "ou_owner",
		"allow_private_chats": "oc_private",
		"allow_group_chats":   "oc_old",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p := pAny.(*interactivePlatform)

	if p.chatAccessAllowed("group", "oc_new", "ou_anyone") {
		t.Fatal("new group should be denied before reload")
	}
	if err := p.ReloadPlatformConfig(map[string]any{
		"allow_from":          "ou_owner",
		"allow_private_chats": "oc_private",
		"allow_group_chats":   "oc_old,oc_new",
	}); err != nil {
		t.Fatalf("ReloadPlatformConfig() error = %v", err)
	}
	if !p.chatAccessAllowed("group", "oc_new", "ou_anyone") {
		t.Fatal("new group should be allowed after reload")
	}
	if !p.chatAccessAllowed("p2p", "oc_private", "ou_other") {
		t.Fatal("private binding should remain allowed after reload")
	}
	if p.chatAccessAllowed("p2p", "oc_other", "ou_owner") {
		t.Fatal("unbound private chat should still be denied after reload")
	}
}

func TestFeishu_EmptyChatBindingListsDenyAll(t *testing.T) {
	pAny, err := New(map[string]any{
		"app_id":              "cli_xxx",
		"app_secret":          "secret",
		"allow_from":          "ou_owner",
		"allow_private_chats": "",
		"allow_group_chats":   "",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p := pAny.(*interactivePlatform)

	if p.chatAccessAllowed("p2p", "oc_private", "ou_owner") {
		t.Fatal("configured empty private binding list should deny private chats")
	}
	if p.chatAccessAllowed("group", "oc_group", "ou_owner") {
		t.Fatal("configured empty group binding list should deny groups")
	}
}

func TestFeishu_ChatBindingFallsBackToAllowFromWhenUnset(t *testing.T) {
	pAny, err := New(map[string]any{
		"app_id":     "cli_xxx",
		"app_secret": "secret",
		"allow_from": "ou_owner",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p := pAny.(*interactivePlatform)

	if !p.chatAccessAllowed("group", "oc_any", "ou_owner") {
		t.Fatal("allow_from user should be allowed when group binding list is unset")
	}
	if p.chatAccessAllowed("group", "oc_any", "ou_other") {
		t.Fatal("non-allow_from user should be denied when group binding list is unset")
	}
}

func TestFeishu_GroupChatBindingAllowsAnyMemberMessage(t *testing.T) {
	pAny, err := New(map[string]any{
		"app_id":                   "cli_xxx",
		"app_secret":               "secret",
		"enable_feishu_card":       true,
		"allow_from":               "ou_owner",
		"allow_group_chats":        "oc_group",
		"share_session_in_channel": true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p := pAny.(*interactivePlatform)
	p.botOpenID = "ou_bot"

	msgCh := make(chan *core.Message, 1)
	p.handler = func(_ core.Platform, msg *core.Message) {
		msgCh <- msg
	}

	mention := []*larkim.MentionEvent{{
		Key: stringPtr("@bot"),
		Id:  &larkim.UserId{OpenId: stringPtr("ou_bot")},
	}}
	if err := p.onMessage(context.Background(), feishuTextEvent("om_group_bind", "oc_group", "ou_other", "group", `{"text":"@bot hi"}`, mention)); err != nil {
		t.Fatalf("onMessage() error = %v", err)
	}

	select {
	case msg := <-msgCh:
		if msg.SessionKey != "feishu:oc_group" {
			t.Fatalf("SessionKey = %q, want feishu:oc_group", msg.SessionKey)
		}
		if msg.UserID != "ou_other" {
			t.Fatalf("UserID = %q, want ou_other", msg.UserID)
		}
		if msg.Content != "hi" {
			t.Fatalf("Content = %q, want hi", msg.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected bound group message from non-allow_from user to be handled")
	}
}

func TestFeishu_BindingRequiredMessageIncludesChatID(t *testing.T) {
	p := &Platform{}

	groupMsg := p.bindingRequiredMessage("group", "oc_group", "ou_user")
	if !strings.Contains(groupMsg, "oc_group") || !strings.Contains(groupMsg, "allow_group_chats") {
		t.Fatalf("group binding message = %q, want chat id and allow_group_chats hint", groupMsg)
	}

	privateMsg := p.bindingRequiredMessage("p2p", "oc_private", "ou_user")
	if !strings.Contains(privateMsg, "oc_private") || !strings.Contains(privateMsg, "allow_private_chats") {
		t.Fatalf("private binding message = %q, want chat id and allow_private_chats hint", privateMsg)
	}
}

func TestFeishu_AutoBindsAdminFirstTrigger(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(`
[[projects]]
name = "alpha"
admin_from = "ou_admin"

[projects.agent]
type = "codex"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "cli_xxx"
app_secret = "secret"
allow_group_chats = ""
`)+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	oldConfigPath := config.ConfigPath
	config.ConfigPath = configPath
	t.Cleanup(func() { config.ConfigPath = oldConfigPath })

	pAny, err := New(map[string]any{
		"app_id":                   "cli_xxx",
		"app_secret":               "secret",
		"cc_project":               "alpha",
		"cc_admin_from":            "ou_admin",
		"cc_platform_index":        1,
		"allow_group_chats":        "",
		"share_session_in_channel": true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p := pAny.(*interactivePlatform)
	p.botOpenID = "ou_bot"

	msgCh := make(chan *core.Message, 1)
	p.handler = func(_ core.Platform, msg *core.Message) {
		msgCh <- msg
	}
	mention := []*larkim.MentionEvent{{
		Key: stringPtr("@bot"),
		Id:  &larkim.UserId{OpenId: stringPtr("ou_bot")},
	}}
	if err := p.onMessage(context.Background(), feishuTextEvent("om_auto_bind", "oc_group", "ou_admin", "group", `{"text":"@bot hi"}`, mention)); err != nil {
		t.Fatalf("onMessage() error = %v", err)
	}

	select {
	case msg := <-msgCh:
		if msg.SessionKey != "feishu:oc_group" {
			t.Fatalf("SessionKey = %q, want feishu:oc_group", msg.SessionKey)
		}
		if msg.Content != "hi" {
			t.Fatalf("Content = %q, want hi", msg.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected auto-bound group message to be handled")
	}
	if !p.chatBound("group", "oc_group") {
		t.Fatal("chatBound(group, oc_group) = false, want true after auto-bind")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(raw), `allow_group_chats = "oc_group"`) {
		t.Fatalf("config does not contain persisted group binding:\n%s", raw)
	}
}

func TestInteractivePlatform_OnMessagePassesCardSenderToHandler(t *testing.T) {
	platformAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip, ok := platformAny.(*interactivePlatform)
	if !ok {
		t.Fatalf("platform type = %T, want *interactivePlatform", platformAny)
	}

	messageID := "om_test_message"
	chatID := "oc_test_chat"
	openID := "ou_test_user"
	msgType := "text"
	chatType := "p2p"
	senderType := "user"
	content := `{"text":"/help"}`
	createText := strconv.FormatInt(time.Now().UnixMilli(), 10)

	var (
		wg           sync.WaitGroup
		receivedPlat core.Platform
		receivedMsg  *core.Message
	)
	wg.Add(1)
	ip.handler = func(p core.Platform, msg *core.Message) {
		defer wg.Done()
		receivedPlat = p
		receivedMsg = msg
	}

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: &openID},
				SenderType: &senderType,
			},
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &content,
				CreateTime:  &createText,
			},
		},
	}

	if err := ip.onMessage(context.Background(), event); err != nil {
		t.Fatalf("onMessage() error = %v", err)
	}
	wg.Wait()

	if receivedMsg == nil {
		t.Fatal("expected handler to receive a message")
	}
	if receivedMsg.Content != "/help" {
		t.Fatalf("message content = %q, want /help", receivedMsg.Content)
	}
	if _, ok := receivedPlat.(core.CardSender); !ok {
		t.Fatalf("handler platform type = %T, want core.CardSender", receivedPlat)
	}
}

func TestInteractivePlatform_CardActionPassesCardSenderToHandler(t *testing.T) {
	platformAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip, ok := platformAny.(*interactivePlatform)
	if !ok {
		t.Fatalf("platform type = %T, want *interactivePlatform", platformAny)
	}

	openID := "ou_test_user"
	chatID := "oc_test_chat"
	messageID := "om_test_message"
	action := "cmd:/help"

	var (
		msgCh  = make(chan *core.Message, 1)
		platCh = make(chan core.Platform, 1)
	)
	ip.handler = func(p core.Platform, msg *core.Message) {
		platCh <- p
		msgCh <- msg
	}

	_, err = ip.onCardAction(&callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: openID},
			Action:   &callback.CallBackAction{Value: map[string]any{"action": action}},
			Context:  &callback.Context{OpenChatID: chatID, OpenMessageID: messageID},
		},
	})
	if err != nil {
		t.Fatalf("onCardAction() error = %v", err)
	}

	select {
	case receivedPlat := <-platCh:
		if _, ok := receivedPlat.(core.CardSender); !ok {
			t.Fatalf("handler platform type = %T, want core.CardSender", receivedPlat)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected card action handler invocation")
	}

	select {
	case receivedMsg := <-msgCh:
		if receivedMsg.Content != "/help" {
			t.Fatalf("message content = %q, want /help", receivedMsg.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected card action message")
	}
}

func TestInteractivePlatform_CardActionActWithoutCardResponseDoesNotWarn(t *testing.T) {
	platformAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip, ok := platformAny.(*interactivePlatform)
	if !ok {
		t.Fatalf("platform type = %T, want *interactivePlatform", platformAny)
	}
	ip.cardNavHandler = func(action string, sessionKey string) *core.Card {
		return nil
	}

	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(orig) })

	resp, err := ip.onCardAction(&callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: "ou_test_user"},
			Action:   &callback.CallBackAction{Value: map[string]any{"action": "act:/delete-mode toggle session-1"}},
			Context:  &callback.Context{OpenChatID: "oc_test_chat", OpenMessageID: "om_test_message"},
		},
	})
	if err != nil {
		t.Fatalf("onCardAction() error = %v", err)
	}
	if resp == nil || resp.Toast == nil {
		t.Fatalf("expected toast response for silent toggle, got %#v", resp)
	}
	if resp.Card != nil {
		t.Fatalf("expected no card update on toggle, got %#v", resp.Card)
	}

	logs := buf.String()
	if strings.Contains(logs, "level=WARN") && strings.Contains(logs, "card nav returned nil, ignoring") {
		t.Fatalf("unexpected warning logs: %s", logs)
	}
}

func TestInteractivePlatform_CardActionFormSubmitPassesSelectedIDs(t *testing.T) {
	platformAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip, ok := platformAny.(*interactivePlatform)
	if !ok {
		t.Fatalf("platform type = %T, want *interactivePlatform", platformAny)
	}

	actionCh := make(chan string, 1)
	ip.cardNavHandler = func(action string, sessionKey string) *core.Card {
		actionCh <- action
		return core.NewCard().Markdown("ok").Build()
	}

	_, err = ip.onCardAction(&callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: "ou_test_user"},
			Action: &callback.CallBackAction{
				Value: map[string]any{"action": "act:/delete-mode form-submit"},
				FormValue: map[string]any{
					deleteModeCheckerName("session-2"): true,
					deleteModeCheckerName("session-1"): true,
					deleteModeCheckerName("session-3"): false,
				},
			},
			Context: &callback.Context{OpenChatID: "oc_test_chat", OpenMessageID: "om_test_message"},
		},
	})
	if err != nil {
		t.Fatalf("onCardAction() error = %v", err)
	}

	select {
	case got := <-actionCh:
		want := "act:/delete-mode form-submit session-1,session-2"
		if got != want {
			t.Fatalf("action = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected card nav handler invocation")
	}
}

func TestInteractivePlatform_CardActionFormSubmitUsesActionNameFallback(t *testing.T) {
	platformAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip, ok := platformAny.(*interactivePlatform)
	if !ok {
		t.Fatalf("platform type = %T, want *interactivePlatform", platformAny)
	}

	actionCh := make(chan string, 1)
	ip.cardNavHandler = func(action string, sessionKey string) *core.Card {
		actionCh <- action
		return core.NewCard().Markdown("ok").Build()
	}

	_, err = ip.onCardAction(&callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: "ou_test_user"},
			Action: &callback.CallBackAction{
				Name: "delete_mode_submit",
				FormValue: map[string]any{
					deleteModeCheckerName("session-2"): true,
					deleteModeCheckerName("session-1"): true,
				},
			},
			Context: &callback.Context{OpenChatID: "oc_test_chat", OpenMessageID: "om_test_message"},
		},
	})
	if err != nil {
		t.Fatalf("onCardAction() error = %v", err)
	}

	select {
	case got := <-actionCh:
		want := "act:/delete-mode form-submit session-1,session-2"
		if got != want {
			t.Fatalf("action = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected card nav handler invocation")
	}
}

func TestInteractivePlatform_CardActionFormCancelUsesActionNameFallback(t *testing.T) {
	platformAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip, ok := platformAny.(*interactivePlatform)
	if !ok {
		t.Fatalf("platform type = %T, want *interactivePlatform", platformAny)
	}

	actionCh := make(chan string, 1)
	ip.cardNavHandler = func(action string, sessionKey string) *core.Card {
		actionCh <- action
		return core.NewCard().Markdown("ok").Build()
	}

	_, err = ip.onCardAction(&callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: "ou_test_user"},
			Action: &callback.CallBackAction{
				Name: "delete_mode_cancel",
			},
			Context: &callback.Context{OpenChatID: "oc_test_chat", OpenMessageID: "om_test_message"},
		},
	})
	if err != nil {
		t.Fatalf("onCardAction() error = %v", err)
	}

	select {
	case got := <-actionCh:
		want := "act:/delete-mode cancel"
		if got != want {
			t.Fatalf("action = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected card nav handler invocation")
	}
}

func TestInteractivePlatform_CardActionUsesCallbackSessionKey(t *testing.T) {
	platformAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true, "thread_isolation": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip := platformAny.(*interactivePlatform)

	wantSessionKey := "feishu:oc_test_chat:root:om_root_thread"
	msgCh := make(chan *core.Message, 1)
	ip.handler = func(_ core.Platform, msg *core.Message) {
		msgCh <- msg
	}

	_, err = ip.onCardAction(&callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: "ou_test_user"},
			Action: &callback.CallBackAction{Value: map[string]any{
				"action":      "cmd:/help",
				"session_key": wantSessionKey,
			}},
			Context: &callback.Context{
				OpenChatID:    "oc_test_chat",
				OpenMessageID: "om_any_card_message",
			},
		},
	})
	if err != nil {
		t.Fatalf("onCardAction() error = %v", err)
	}

	select {
	case msg := <-msgCh:
		if msg.SessionKey != wantSessionKey {
			t.Fatalf("SessionKey = %q, want %q", msg.SessionKey, wantSessionKey)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected card action message")
	}
}

func TestInteractivePlatform_ModelCardActionReturnsCardUpdate(t *testing.T) {
	platformAny, err := New(map[string]any{"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip, ok := platformAny.(*interactivePlatform)
	if !ok {
		t.Fatalf("platform type = %T, want *interactivePlatform", platformAny)
	}

	var gotAction, gotSessionKey string
	ip.cardNavHandler = func(action string, sessionKey string) *core.Card {
		gotAction = action
		gotSessionKey = sessionKey
		return core.NewCard().Markdown("switching").Build()
	}

	resp, err := ip.onCardAction(&callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Operator: &callback.Operator{OpenID: "ou_test_user"},
			Action:   &callback.CallBackAction{Value: map[string]any{"action": "act:/model switch 1"}},
			Context:  &callback.Context{OpenChatID: "oc_test_chat", OpenMessageID: "om_test_message"},
		},
	})
	if err != nil {
		t.Fatalf("onCardAction() error = %v", err)
	}
	if resp == nil || resp.Card == nil {
		t.Fatalf("expected card response, got %#v", resp)
	}
	if gotAction != "act:/model switch 1" {
		t.Fatalf("action = %q, want act:/model switch 1", gotAction)
	}
	if gotSessionKey == "" {
		t.Fatal("expected non-empty session key")
	}
	ip.cardActionMsgMu.Lock()
	tracked := ip.cardActionMsgIDs[gotSessionKey]
	ip.cardActionMsgMu.Unlock()
	if tracked != "om_test_message" {
		t.Fatalf("tracked message id = %q, want om_test_message", tracked)
	}
}

func TestNewLark_PlatformNameAndDomain(t *testing.T) {
	p, err := newPlatform("lark", lark.LarkBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret",
	})
	if err != nil {
		t.Fatalf("newPlatform(lark) error = %v", err)
	}
	if p.Name() != "lark" {
		t.Fatalf("Name() = %q, want lark", p.Name())
	}
	ip, ok := p.(*interactivePlatform)
	if !ok {
		t.Fatalf("type = %T, want *interactivePlatform", p)
	}
	if ip.domain != lark.LarkBaseUrl {
		t.Fatalf("domain = %q, want %q", ip.domain, lark.LarkBaseUrl)
	}
}

func TestPlatformShouldUseWebhookMode(t *testing.T) {
	tests := []struct {
		name       string
		platform   string
		encryptKey string
		want       bool
	}{
		{name: "lark defaults to websocket", platform: "lark", want: false},
		{name: "lark webhook when encrypt key set", platform: "lark", encryptKey: "enc-key", want: true},
		{name: "feishu defaults to websocket", platform: "feishu", want: false},
		{name: "feishu webhook when encrypt key set", platform: "feishu", encryptKey: "enc-key", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Platform{platformName: tt.platform, encryptKey: tt.encryptKey}
			if got := p.shouldUseWebhookMode(); got != tt.want {
				t.Fatalf("shouldUseWebhookMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewFeishu_PlatformNameAndDomain(t *testing.T) {
	p, err := New(map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if p.Name() != "feishu" {
		t.Fatalf("Name() = %q, want feishu", p.Name())
	}
}

func TestNewFeishu_CustomDomainOverride(t *testing.T) {
	customDomain := "https://open.example.invalid"
	p, err := New(map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "domain": customDomain,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ip, ok := p.(*interactivePlatform)
	if !ok {
		t.Fatalf("type = %T, want *interactivePlatform", p)
	}
	if ip.domain != customDomain {
		t.Fatalf("domain = %q, want %q", ip.domain, customDomain)
	}
}

func TestNewFeishu_InvalidCustomDomain(t *testing.T) {
	_, err := New(map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "domain": "://bad",
	})
	if err == nil {
		t.Fatal("expected invalid domain error")
	}
}

func TestLark_SessionKeyPrefix(t *testing.T) {
	p, err := newPlatform("lark", lark.LarkBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true,
	})
	if err != nil {
		t.Fatalf("newPlatform(lark) error = %v", err)
	}
	ip := p.(*interactivePlatform)

	messageID := "om_test"
	chatID := "oc_test"
	openID := "ou_test"
	msgType := "text"
	chatType := "p2p"
	senderType := "user"
	content := `{"text":"hello"}`
	createText := strconv.FormatInt(time.Now().UnixMilli(), 10)

	var receivedMsg *core.Message
	var wg sync.WaitGroup
	wg.Add(1)
	ip.handler = func(_ core.Platform, msg *core.Message) {
		defer wg.Done()
		receivedMsg = msg
	}

	_ = ip.onMessage(context.Background(), &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: &openID},
				SenderType: &senderType,
			},
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &content,
				CreateTime:  &createText,
			},
		},
	})
	wg.Wait()

	if receivedMsg == nil {
		t.Fatal("handler not called")
	}
	if !strings.HasPrefix(receivedMsg.SessionKey, "lark:") {
		t.Fatalf("SessionKey = %q, want lark: prefix", receivedMsg.SessionKey)
	}
	if receivedMsg.Platform != "lark" {
		t.Fatalf("Platform = %q, want lark", receivedMsg.Platform)
	}
}

func TestLark_ThreadIsolationUsesRootSessionKey(t *testing.T) {
	p, err := newPlatform("lark", lark.LarkBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true, "thread_isolation": true,
	})
	if err != nil {
		t.Fatalf("newPlatform(lark) error = %v", err)
	}
	ip := p.(*interactivePlatform)

	messageID := "om_reply"
	rootID := "om_root"
	chatID := "oc_test"
	openID := "ou_test"
	msgType := "text"
	chatType := "group"
	senderType := "user"
	content := `{"text":"@bot hello"}`
	createText := strconv.FormatInt(time.Now().UnixMilli(), 10)

	var receivedMsg *core.Message
	var wg sync.WaitGroup
	wg.Add(1)
	ip.botOpenID = "ou_bot"
	ip.handler = func(_ core.Platform, msg *core.Message) {
		defer wg.Done()
		receivedMsg = msg
	}

	_ = ip.onMessage(context.Background(), &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: &openID},
				SenderType: &senderType,
			},
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				RootId:      &rootID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &content,
				CreateTime:  &createText,
				Mentions: []*larkim.MentionEvent{
					{
						Key: stringPtr("@bot"),
						Id:  &larkim.UserId{OpenId: stringPtr("ou_bot")},
					},
				},
			},
		},
	})
	wg.Wait()

	if receivedMsg == nil {
		t.Fatal("handler not called")
	}
	if receivedMsg.SessionKey != "lark:oc_test:root:om_root" {
		t.Fatalf("SessionKey = %q, want lark:oc_test:root:om_root", receivedMsg.SessionKey)
	}
}

func TestLark_GroupReplyAllWithThreadIsolationUsesRootSessionKeyWithoutMention(t *testing.T) {
	p, err := newPlatform("lark", lark.LarkBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true,
		"group_reply_all": true, "thread_isolation": true,
	})
	if err != nil {
		t.Fatalf("newPlatform(lark) error = %v", err)
	}
	ip := p.(*interactivePlatform)

	messageID := "om_root"
	chatID := "oc_test"
	openID := "ou_test"
	msgType := "text"
	chatType := "group"
	senderType := "user"
	content := `{"text":"hello from group root"}`
	createText := strconv.FormatInt(time.Now().UnixMilli(), 10)

	msgCh := make(chan *core.Message, 1)
	ip.handler = func(_ core.Platform, msg *core.Message) {
		msgCh <- msg
	}

	if err := ip.onMessage(context.Background(), &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: &openID},
				SenderType: &senderType,
			},
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &content,
				CreateTime:  &createText,
			},
		},
	}); err != nil {
		t.Fatalf("onMessage() error = %v", err)
	}

	select {
	case receivedMsg := <-msgCh:
		if receivedMsg.SessionKey != "lark:oc_test:root:om_root" {
			t.Fatalf("SessionKey = %q, want lark:oc_test:root:om_root", receivedMsg.SessionKey)
		}
		rc, ok := receivedMsg.ReplyCtx.(replyContext)
		if !ok {
			t.Fatalf("ReplyCtx type = %T, want replyContext", receivedMsg.ReplyCtx)
		}
		if rc.sessionKey != "lark:oc_test:root:om_root" {
			t.Fatalf("replyContext.sessionKey = %q, want lark:oc_test:root:om_root", rc.sessionKey)
		}
		if rc.messageID != "om_root" {
			t.Fatalf("replyContext.messageID = %q, want om_root", rc.messageID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected group root message to be handled without mention")
	}
}

func TestFeishu_GroupContextBufferUsesHistoryOnTrigger(t *testing.T) {
	p, err := newPlatform("feishu", lark.FeishuBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true,
		"share_session_in_channel":    true,
		"group_context_buffer":        true,
		"context_buffer_max_messages": 100,
		"context_buffer_max_age_mins": 0,
	})
	if err != nil {
		t.Fatalf("newPlatform(feishu) error = %v", err)
	}
	ip := p.(*interactivePlatform)
	ip.botOpenID = "ou_bot"
	ip.userNameCache.Store("ou_alice", "Alice")
	ip.userNameCache.Store("ou_bob", "Bob")
	ip.chatNameCache.Store("oc_group", "Group")
	baseTime := time.Date(2026, 6, 9, 1, 18, 0, 0, time.Local)
	ip.groupHistoryFetch = func(_ context.Context, chatID string, _ int64, _ int64, pageSize int) ([]groupHistoryEntry, error) {
		if chatID != "oc_group" {
			t.Fatalf("history chatID = %q, want oc_group", chatID)
		}
		if pageSize != 100 {
			t.Fatalf("history pageSize = %d, want 100", pageSize)
		}
		return []groupHistoryEntry{
			{
				MessageID:  "om_context",
				SenderID:   "ou_alice",
				SenderName: "Alice",
				SenderType: "user",
				CreatedAt:  baseTime,
				Content:    "先记一下背景",
			},
			{
				MessageID:  "om_trigger",
				SenderID:   "ou_bob",
				SenderName: "Bob",
				SenderType: "user",
				CreatedAt:  baseTime.Add(time.Minute),
				Content:    "总结一下",
			},
		}, nil
	}

	msgCh := make(chan *core.Message, 1)
	ip.handler = func(_ core.Platform, msg *core.Message) {
		msgCh <- msg
	}

	_ = ip.onMessage(context.Background(), feishuTextEvent("om_context", "oc_group", "ou_alice", "group", `{"text":"先记一下背景"}`, nil))
	select {
	case msg := <-msgCh:
		t.Fatalf("non-mention group message should not be dispatched, got %#v", msg)
	case <-time.After(100 * time.Millisecond):
	}

	mention := []*larkim.MentionEvent{{
		Key: stringPtr("@bot"),
		Id:  &larkim.UserId{OpenId: stringPtr("ou_bot")},
	}}
	_ = ip.onMessage(context.Background(), feishuTextEvent("om_trigger", "oc_group", "ou_bob", "group", `{"text":"@bot 总结一下"}`, mention))

	select {
	case got := <-msgCh:
		if got.SessionKey != "feishu:oc_group" {
			t.Fatalf("SessionKey = %q, want shared group session feishu:oc_group", got.SessionKey)
		}
		if got.Content != "总结一下" {
			t.Fatalf("Content = %q, want trigger text without bot mention", got.Content)
		}
		if !strings.Contains(got.ExtraContent, "[Feishu group history]") {
			t.Fatalf("ExtraContent missing history header: %q", got.ExtraContent)
		}
		if !strings.Contains(got.ExtraContent, "[01:18 Alice] 先记一下背景") {
			t.Fatalf("ExtraContent missing cached message: %q", got.ExtraContent)
		}
		if strings.Contains(got.ExtraContent, "om_context") || strings.Contains(got.ExtraContent, "ou_alice") {
			t.Fatalf("ExtraContent leaked internal IDs: %q", got.ExtraContent)
		}
		if strings.Contains(got.ExtraContent, "sender_id") || strings.Contains(got.ExtraContent, "message_id") {
			t.Fatalf("ExtraContent leaked machine-readable ID labels: %q", got.ExtraContent)
		}
		if strings.Contains(got.ExtraContent, "总结一下") {
			t.Fatalf("ExtraContent should not duplicate current trigger: %q", got.ExtraContent)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected mention message to be handled")
	}
}

func TestFeishu_GroupContextBufferAttachesHistoryFile(t *testing.T) {
	const fileName = "未达年限报废家具统计.xlsx"
	fileBytes := []byte("xlsx bytes")
	var resourceHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, map[string]any{
				"code":                0,
				"msg":                 "success",
				"expire":              7200,
				"tenant_access_token": "valid-token",
			})
		case "/open-apis/im/v1/messages/om_file/resources/file_xlsx":
			resourceHits++
			if got := r.URL.Query().Get("type"); got != "file" {
				t.Fatalf("resource type query = %q, want file", got)
			}
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			_, _ = w.Write(fileBytes)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	p, err := newPlatform("feishu", lark.FeishuBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true,
		"domain":                      srv.URL,
		"reaction_emoji":              "none",
		"share_session_in_channel":    true,
		"group_context_buffer":        true,
		"context_buffer_max_messages": 100,
		"context_buffer_max_age_mins": 0,
	})
	if err != nil {
		t.Fatalf("newPlatform(feishu) error = %v", err)
	}
	ip := p.(*interactivePlatform)
	ip.botOpenID = "ou_bot"
	ip.userNameCache.Store("ou_user", "User")
	ip.chatNameCache.Store("oc_group", "Group")
	baseTime := time.Date(2026, 6, 9, 1, 18, 0, 0, time.Local)
	ip.groupHistoryFetch = func(_ context.Context, chatID string, _ int64, _ int64, _ int) ([]groupHistoryEntry, error) {
		if chatID != "oc_group" {
			t.Fatalf("history chatID = %q, want oc_group", chatID)
		}
		return []groupHistoryEntry{
			{
				MessageID:  "om_file",
				SenderID:   "ou_zxk",
				SenderName: "赵雪坤",
				SenderType: "user",
				CreatedAt:  baseTime,
				Content:    "[file: " + fileName + "]",
				Files: []messageAttachmentRef{{
					Kind:         "file",
					MessageID:    "om_file",
					FileKey:      "file_xlsx",
					FileName:     fileName,
					ResourceType: "file",
				}},
			},
			{
				MessageID:  "om_trigger",
				SenderID:   "ou_user",
				SenderName: "User",
				SenderType: "user",
				CreatedAt:  baseTime.Add(time.Minute),
				Content:    "这个excel中写的是什么？",
			},
		}, nil
	}

	msgCh := make(chan *core.Message, 1)
	ip.handler = func(_ core.Platform, msg *core.Message) {
		msgCh <- msg
	}
	mention := []*larkim.MentionEvent{{
		Key: stringPtr("@bot"),
		Id:  &larkim.UserId{OpenId: stringPtr("ou_bot")},
	}}
	_ = ip.onMessage(context.Background(), feishuTextEvent("om_trigger", "oc_group", "ou_user", "group", `{"text":"@bot 这个excel中写的是什么？"}`, mention))

	select {
	case got := <-msgCh:
		if !strings.Contains(got.ExtraContent, "[01:18 赵雪坤] [file: "+fileName+"]") {
			t.Fatalf("ExtraContent missing file history line: %q", got.ExtraContent)
		}
		if len(got.Files) != 1 {
			t.Fatalf("Files len = %d, want 1", len(got.Files))
		}
		if got.Files[0].FileName != fileName {
			t.Fatalf("FileName = %q, want %q", got.Files[0].FileName, fileName)
		}
		if string(got.Files[0].Data) != string(fileBytes) {
			t.Fatalf("file data = %q, want %q", string(got.Files[0].Data), string(fileBytes))
		}
		if resourceHits != 1 {
			t.Fatalf("resourceHits = %d, want 1", resourceHits)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected mention message to be handled")
	}
}

func TestFeishu_StartsReactionBeforeGroupHistoryFetch(t *testing.T) {
	var createHits atomic.Int32
	var deleteHits atomic.Int32
	var historyStartedBeforeReaction atomic.Bool
	reactionCreated := make(chan struct{})
	reactionCreatedOnce := sync.Once{}
	reactionDeleted := make(chan struct{})
	reactionDeletedOnce := sync.Once{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			writeJSON(t, w, map[string]any{
				"code":                0,
				"msg":                 "success",
				"expire":              7200,
				"tenant_access_token": "valid-token",
			})
		case "/open-apis/im/v1/messages/om_trigger/reactions":
			if r.Method != http.MethodPost {
				t.Fatalf("reaction create method = %s, want POST", r.Method)
			}
			createHits.Add(1)
			reactionCreatedOnce.Do(func() { close(reactionCreated) })
			writeJSON(t, w, map[string]any{
				"code": 0,
				"msg":  "success",
				"data": map[string]any{"reaction_id": "reaction_1"},
			})
		case "/open-apis/im/v1/messages/om_trigger/reactions/reaction_1":
			if r.Method != http.MethodDelete {
				t.Fatalf("reaction delete method = %s, want DELETE", r.Method)
			}
			deleteHits.Add(1)
			reactionDeletedOnce.Do(func() { close(reactionDeleted) })
			writeJSON(t, w, map[string]any{"code": 0, "msg": "success"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	p, err := newPlatform("feishu", lark.FeishuBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true,
		"domain":                   srv.URL,
		"share_session_in_channel": true,
		"group_context_buffer":     true,
	})
	if err != nil {
		t.Fatalf("newPlatform(feishu) error = %v", err)
	}
	ip := p.(*interactivePlatform)
	ip.botOpenID = "ou_bot"
	ip.userNameCache.Store("ou_user", "User")
	ip.chatNameCache.Store("oc_group", "Group")
	ip.groupHistoryFetch = func(_ context.Context, _ string, _ int64, _ int64, _ int) ([]groupHistoryEntry, error) {
		select {
		case <-reactionCreated:
		case <-time.After(2 * time.Second):
			historyStartedBeforeReaction.Store(true)
		}
		return nil, nil
	}

	msgCh := make(chan *core.Message, 1)
	ip.handler = func(_ core.Platform, msg *core.Message) {
		stopTyping := ip.StartTyping(context.Background(), msg.ReplyCtx)
		stopTyping()
		msgCh <- msg
	}
	mention := []*larkim.MentionEvent{{
		Key: stringPtr("@bot"),
		Id:  &larkim.UserId{OpenId: stringPtr("ou_bot")},
	}}
	_ = ip.onMessage(context.Background(), feishuTextEvent("om_trigger", "oc_group", "ou_user", "group", `{"text":"@bot 123"}`, mention))

	select {
	case <-msgCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected mention message to be handled")
	}
	if historyStartedBeforeReaction.Load() {
		t.Fatal("group history fetch started before the early reaction request was observed")
	}
	select {
	case <-reactionDeleted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected StartTyping stop to remove the early reaction")
	}
	if createHits.Load() != 1 {
		t.Fatalf("reaction create hits = %d, want 1", createHits.Load())
	}
	if deleteHits.Load() != 1 {
		t.Fatalf("reaction delete hits = %d, want 1", deleteHits.Load())
	}
}

func TestFeishu_QuotedFileMessageIsAttached(t *testing.T) {
	const fileName = "未达年限报废家具统计.xlsx"
	fileBytes := []byte("quoted xlsx bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, map[string]any{
				"code":                0,
				"msg":                 "success",
				"expire":              7200,
				"tenant_access_token": "valid-token",
			})
		case "/open-apis/im/v1/messages/om_file":
			w.Header().Set("Content-Type", "application/json")
			writeJSON(t, w, map[string]any{
				"code": 0,
				"msg":  "success",
				"data": map[string]any{
					"items": []map[string]any{
						{
							"msg_type":  "file",
							"parent_id": "",
							"sender": map[string]any{
								"id":          "ou_zxk",
								"sender_type": "user",
							},
							"body": map[string]any{
								"content": `{"file_key":"file_xlsx","file_name":"` + fileName + `"}`,
							},
						},
					},
				},
			})
		case "/open-apis/im/v1/messages/om_file/resources/file_xlsx":
			if got := r.URL.Query().Get("type"); got != "file" {
				t.Fatalf("resource type query = %q, want file", got)
			}
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			_, _ = w.Write(fileBytes)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	p, err := newPlatform("feishu", lark.FeishuBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true,
		"domain":                   srv.URL,
		"reaction_emoji":           "none",
		"share_session_in_channel": true,
		"group_context_buffer":     false,
	})
	if err != nil {
		t.Fatalf("newPlatform(feishu) error = %v", err)
	}
	ip := p.(*interactivePlatform)
	ip.botOpenID = "ou_bot"
	ip.userNameCache.Store("ou_user", "User")
	ip.userNameCache.Store("ou_zxk", "赵雪坤")
	ip.chatNameCache.Store("oc_group", "Group")

	msgCh := make(chan *core.Message, 1)
	ip.handler = func(_ core.Platform, msg *core.Message) {
		msgCh <- msg
	}
	mention := []*larkim.MentionEvent{{
		Key: stringPtr("@bot"),
		Id:  &larkim.UserId{OpenId: stringPtr("ou_bot")},
	}}
	event := feishuTextEvent("om_trigger", "oc_group", "ou_user", "group", `{"text":"@bot 这个excel中写的是什么？"}`, mention)
	event.Event.Message.ParentId = stringPtr("om_file")
	_ = ip.onMessage(context.Background(), event)

	select {
	case got := <-msgCh:
		if !strings.Contains(got.ExtraContent, "[Quoted message from 赵雪坤]") {
			t.Fatalf("ExtraContent missing quoted file prefix: %q", got.ExtraContent)
		}
		if !strings.Contains(got.ExtraContent, "[file: "+fileName+"]") {
			t.Fatalf("ExtraContent missing quoted file name: %q", got.ExtraContent)
		}
		if len(got.Files) != 1 {
			t.Fatalf("Files len = %d, want 1", len(got.Files))
		}
		if got.Files[0].FileName != fileName {
			t.Fatalf("FileName = %q, want %q", got.Files[0].FileName, fileName)
		}
		if string(got.Files[0].Data) != string(fileBytes) {
			t.Fatalf("file data = %q, want %q", string(got.Files[0].Data), string(fileBytes))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected reply message to be handled")
	}
}

func TestFeishu_GroupContextBufferOnlyAddsNewHistoryAfterDelivery(t *testing.T) {
	p, err := newPlatform("feishu", lark.FeishuBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": true,
		"share_session_in_channel":    true,
		"group_context_buffer":        true,
		"context_buffer_max_messages": 100,
		"context_buffer_max_age_mins": 0,
	})
	if err != nil {
		t.Fatalf("newPlatform(feishu) error = %v", err)
	}
	ip := p.(*interactivePlatform)
	ip.botOpenID = "ou_bot"
	ip.userNameCache.Store("ou_alice", "Alice")
	ip.userNameCache.Store("ou_bob", "Bob")
	ip.userNameCache.Store("ou_cara", "Cara")
	ip.chatNameCache.Store("oc_group", "Group")
	baseTime := time.Date(2026, 6, 9, 1, 18, 0, 0, time.Local)
	fetchCount := 0
	ip.groupHistoryFetch = func(_ context.Context, chatID string, _ int64, _ int64, _ int) ([]groupHistoryEntry, error) {
		if chatID != "oc_group" {
			t.Fatalf("history chatID = %q, want oc_group", chatID)
		}
		fetchCount++
		entries := []groupHistoryEntry{
			{MessageID: "om_a", SenderID: "ou_alice", SenderName: "Alice", SenderType: "user", CreatedAt: baseTime, Content: "A"},
			{MessageID: "om_b", SenderID: "ou_bob", SenderName: "Bob", SenderType: "user", CreatedAt: baseTime.Add(time.Minute), Content: "B"},
			{MessageID: "om_c", SenderID: "ou_cara", SenderName: "Cara", SenderType: "user", CreatedAt: baseTime.Add(2 * time.Minute), Content: "C"},
			{MessageID: "om_trigger_1", SenderID: "ou_bob", SenderName: "Bob", SenderType: "user", CreatedAt: baseTime.Add(3 * time.Minute), Content: "第一次问题"},
		}
		if fetchCount == 1 {
			return entries, nil
		}
		return append(entries,
			groupHistoryEntry{MessageID: "om_own_reply", SenderID: "cli_xxx", SenderName: "agentchat", SenderType: "app", CreatedAt: baseTime.Add(4 * time.Minute), Content: "第一次回复"},
			groupHistoryEntry{MessageID: "om_d", SenderID: "ou_alice", SenderName: "Alice", SenderType: "user", CreatedAt: baseTime.Add(5 * time.Minute), Content: "D"},
			groupHistoryEntry{MessageID: "om_e", SenderID: "ou_bob", SenderName: "Bob", SenderType: "user", CreatedAt: baseTime.Add(6 * time.Minute), Content: "E"},
			groupHistoryEntry{MessageID: "om_trigger_2", SenderID: "ou_cara", SenderName: "Cara", SenderType: "user", CreatedAt: baseTime.Add(7 * time.Minute), Content: "第二次问题"},
		), nil
	}

	msgCh := make(chan *core.Message, 2)
	ip.handler = func(_ core.Platform, msg *core.Message) {
		msgCh <- msg
	}
	mention := []*larkim.MentionEvent{{
		Key: stringPtr("@bot"),
		Id:  &larkim.UserId{OpenId: stringPtr("ou_bot")},
	}}

	_ = ip.onMessage(context.Background(), feishuTextEvent("om_trigger_1", "oc_group", "ou_bob", "group", `{"text":"@bot 第一次问题"}`, mention))
	var first *core.Message
	select {
	case first = <-msgCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected first mention message to be handled")
	}
	if first.MarkContextDelivered == nil {
		t.Fatal("first MarkContextDelivered = nil")
	}
	if !strings.Contains(first.ExtraContent, "[01:18 Alice] A") ||
		!strings.Contains(first.ExtraContent, "[01:19 Bob] B") ||
		!strings.Contains(first.ExtraContent, "[01:20 Cara] C") {
		t.Fatalf("first ExtraContent missing initial context: %q", first.ExtraContent)
	}
	if strings.Contains(first.ExtraContent, "第一次问题") {
		t.Fatalf("first ExtraContent should not duplicate current trigger: %q", first.ExtraContent)
	}
	first.MarkContextDelivered()

	_ = ip.onMessage(context.Background(), feishuTextEvent("om_trigger_2", "oc_group", "ou_cara", "group", `{"text":"@bot 第二次问题"}`, mention))
	var second *core.Message
	select {
	case second = <-msgCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected second mention message to be handled")
	}
	if second.MarkContextDelivered == nil {
		t.Fatal("second MarkContextDelivered = nil")
	}
	if strings.Contains(second.ExtraContent, "] A") ||
		strings.Contains(second.ExtraContent, "] B") ||
		strings.Contains(second.ExtraContent, "] C") ||
		strings.Contains(second.ExtraContent, "第一次问题") ||
		strings.Contains(second.ExtraContent, "第一次回复") {
		t.Fatalf("second ExtraContent repeated already-delivered history: %q", second.ExtraContent)
	}
	if !strings.Contains(second.ExtraContent, "[01:23 Alice] D") ||
		!strings.Contains(second.ExtraContent, "[01:24 Bob] E") {
		t.Fatalf("second ExtraContent missing new context: %q", second.ExtraContent)
	}
	if strings.Contains(second.ExtraContent, "第二次问题") {
		t.Fatalf("second ExtraContent should not duplicate current trigger: %q", second.ExtraContent)
	}
}

func TestFeishu_GroupHistoryEntryFiltersCardsAndIncludesAppText(t *testing.T) {
	p := &Platform{platformName: "feishu", appID: "cli_self"}
	p.userNameCache.Store("ou_alice", "Alice")
	createText := strconv.FormatInt(time.Date(2026, 6, 9, 1, 18, 0, 0, time.Local).UnixMilli(), 10)

	appText := &larkim.Message{
		MessageId:  stringPtr("om_app"),
		MsgType:    stringPtr("text"),
		CreateTime: &createText,
		Sender: &larkim.Sender{
			Id:         stringPtr("cli_webhook"),
			SenderType: stringPtr("app"),
		},
		Body: &larkim.MessageBody{Content: stringPtr(`{"text":"webhook reply"}`)},
	}
	entry, ok := p.groupHistoryEntryFromMessage(context.Background(), "oc_group", appText)
	if !ok {
		t.Fatal("expected app text to be included")
	}
	if entry.SenderName != "App" {
		t.Fatalf("SenderName = %q, want App", entry.SenderName)
	}
	if entry.Content != "webhook reply" {
		t.Fatalf("Content = %q, want webhook reply", entry.Content)
	}

	progressCard := &larkim.Message{
		MessageId:  stringPtr("om_card"),
		MsgType:    stringPtr("interactive"),
		CreateTime: &createText,
		Sender: &larkim.Sender{
			Id:         stringPtr("cli_other_bot"),
			SenderType: stringPtr("app"),
		},
		Body: &larkim.MessageBody{Content: stringPtr(`{"title":"Codex · 进行中"}`)},
	}
	if entry, ok := p.groupHistoryEntryFromMessage(context.Background(), "oc_group", progressCard); ok {
		t.Fatalf("interactive progress card should be skipped, got %#v", entry)
	}

	completedProgressCard := &larkim.Message{
		MessageId:  stringPtr("om_completed_progress"),
		MsgType:    stringPtr("interactive"),
		CreateTime: &createText,
		Sender: &larkim.Sender{
			Id:         stringPtr("cli_other_bot"),
			SenderType: stringPtr("app"),
		},
		Body: &larkim.MessageBody{Content: stringPtr(buildProgressCardJSONFromPayload(&core.ProgressCardPayload{
			Agent: "Codex",
			Lang:  "en",
			State: core.ProgressCardStateCompleted,
			Items: []core.ProgressCardEntry{
				{Kind: core.ProgressEntryToolUse, Tool: "bash", Text: "date"},
			},
		}))},
	}
	if entry, ok := p.groupHistoryEntryFromMessage(context.Background(), "oc_group", completedProgressCard); ok {
		t.Fatalf("completed progress card should be skipped, got %#v", entry)
	}

	finalReplyCard := &larkim.Message{
		MessageId:  stringPtr("om_final_card"),
		MsgType:    stringPtr("interactive"),
		CreateTime: &createText,
		Sender: &larkim.Sender{
			Id:         stringPtr("cli_other_bot"),
			SenderType: stringPtr("app"),
		},
		Body: &larkim.MessageBody{Content: stringPtr(buildCardJSON("123456\n\n这个答案是对的"))},
	}
	entry, ok = p.groupHistoryEntryFromMessage(context.Background(), "oc_group", finalReplyCard)
	if !ok {
		t.Fatal("expected final interactive reply card to be included")
	}
	if entry.Content != "123456\n\n这个答案是对的" {
		t.Fatalf("Content = %q, want final card markdown", entry.Content)
	}
}

func TestFeishu_GroupHistoryEntryUsesChatMemberNameCache(t *testing.T) {
	p := &Platform{platformName: "feishu", appID: "cli_self"}
	p.chatMemberCache.Store("oc_group", &chatMemberEntry{
		members:   map[string]string{"赵雪坤": "ou_zxk"},
		namesByID: map[string]string{"ou_zxk": "赵雪坤"},
		fetchedAt: time.Now(),
	})
	createText := strconv.FormatInt(time.Date(2026, 6, 9, 11, 3, 0, 0, time.Local).UnixMilli(), 10)
	msg := &larkim.Message{
		MessageId:  stringPtr("om_user"),
		MsgType:    stringPtr("text"),
		CreateTime: &createText,
		Sender: &larkim.Sender{
			Id:         stringPtr("ou_zxk"),
			SenderType: stringPtr("user"),
		},
		Body: &larkim.MessageBody{Content: stringPtr(`{"text":"上面这个码是什么"}`)},
	}

	entry, ok := p.groupHistoryEntryFromMessage(context.Background(), "oc_group", msg)
	if !ok {
		t.Fatal("expected user text to be included")
	}
	if entry.SenderName != "赵雪坤" {
		t.Fatalf("SenderName = %q, want 赵雪坤", entry.SenderName)
	}
	if entry.Content != "上面这个码是什么" {
		t.Fatalf("Content = %q, want trigger text", entry.Content)
	}
}

func TestFeishu_FetchGroupHistoryRequestsRawInteractiveCardContent(t *testing.T) {
	const appID = "cli_history_raw"
	const appSecret = "secret"
	createTime := strconv.FormatInt(time.Date(2026, 6, 9, 17, 28, 45, 0, time.Local).UnixMilli(), 10)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			writeJSON(t, w, map[string]any{
				"code":                0,
				"msg":                 "success",
				"expire":              7200,
				"tenant_access_token": "valid-token",
			})
		case "/open-apis/im/v1/messages":
			if got := r.URL.Query().Get("card_msg_content_type"); got != "raw_card_content" {
				t.Fatalf("card_msg_content_type = %q, want raw_card_content", got)
			}
			writeJSON(t, w, map[string]any{
				"code": 0,
				"msg":  "success",
				"data": map[string]any{
					"has_more":   false,
					"page_token": "",
					"items": []map[string]any{
						{
							"message_id":  "om_final_card",
							"msg_type":    "interactive",
							"create_time": createTime,
							"sender": map[string]any{
								"id":          "cli_other_bot",
								"sender_type": "app",
							},
							"body": map[string]any{
								"content": buildCardJSON("123456"),
							},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	p := &Platform{
		platformName: "feishu",
		appID:        appID,
		appSecret:    appSecret,
		client: lark.NewClient(appID, appSecret,
			lark.WithOpenBaseUrl(srv.URL),
			lark.WithHttpClient(srv.Client()),
		),
	}
	p.chatMemberCache.Store("oc_group", &chatMemberEntry{
		members:   map[string]string{"deli": "cli_other_bot"},
		namesByID: map[string]string{"cli_other_bot": "deli"},
		fetchedAt: time.Now(),
	})

	entries, err := p.fetchGroupHistory(context.Background(), "oc_group", 0, time.Now().UnixMilli(), 10)
	if err != nil {
		t.Fatalf("fetchGroupHistory() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1: %#v", len(entries), entries)
	}
	if entries[0].Content != "123456" {
		t.Fatalf("entry content = %q, want raw final card text", entries[0].Content)
	}
}

func TestGroupHistoryRequestPageSizeCapsAtFeishuLimit(t *testing.T) {
	tests := []struct {
		name      string
		target    int
		collected int
		want      int
	}{
		{name: "under limit", target: 20, collected: 0, want: 20},
		{name: "caps first page", target: 100, collected: 0, want: 50},
		{name: "caps second page", target: 100, collected: 50, want: 50},
		{name: "remaining tail", target: 100, collected: 99, want: 1},
		{name: "non-positive remaining", target: 10, collected: 10, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := groupHistoryRequestPageSize(tt.target, tt.collected); got != tt.want {
				t.Fatalf("groupHistoryRequestPageSize(%d, %d) = %d, want %d", tt.target, tt.collected, got, tt.want)
			}
		})
	}
}

func TestBuildReplyMessageReqBody_SetsReplyInThreadFlag(t *testing.T) {
	tests := []struct {
		name          string
		platform      *Platform
		replyCtx      replyContext
		wantThreading bool
	}{
		{
			name:          "thread isolation enabled",
			platform:      &Platform{threadIsolation: true},
			replyCtx:      replyContext{messageID: "om_reply", sessionKey: "feishu:oc_chat:root:om_root"},
			wantThreading: true,
		},
		{
			name:          "thread isolation does not affect p2p session",
			platform:      &Platform{threadIsolation: true},
			replyCtx:      replyContext{messageID: "om_reply", sessionKey: "feishu:oc_chat:ou_user"},
			wantThreading: false,
		},
		{
			name:          "plain reply remains non-threaded",
			platform:      &Platform{},
			replyCtx:      replyContext{messageID: "om_reply"},
			wantThreading: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := tt.platform.buildReplyMessageReqBody(tt.replyCtx, larkim.MsgTypeText, `{"text":"hello"}`)
			if body == nil {
				t.Fatal("Body = nil, want populated reply body")
			}
			if body.ReplyInThread == nil {
				if tt.wantThreading {
					t.Fatal("ReplyInThread = nil, want true")
				}
				return
			}
			if got := *body.ReplyInThread; got != tt.wantThreading {
				t.Fatalf("ReplyInThread = %v, want %v", got, tt.wantThreading)
			}
		})
	}
}

func TestLark_ReconstructReplyCtx(t *testing.T) {
	p, err := newPlatform("lark", lark.LarkBaseUrl, map[string]any{
		"app_id": "cli_xxx", "app_secret": "secret", "enable_feishu_card": false,
	})
	if err != nil {
		t.Fatalf("newPlatform(lark) error = %v", err)
	}
	base := p.(*Platform)

	rctx, err := base.ReconstructReplyCtx("lark:oc_chat123:ou_user456")
	if err != nil {
		t.Fatalf("ReconstructReplyCtx() error = %v", err)
	}
	rc := rctx.(replyContext)
	if rc.chatID != "oc_chat123" {
		t.Fatalf("chatID = %q, want oc_chat123", rc.chatID)
	}

	rctx, err = base.ReconstructReplyCtx("lark:oc_chat123:root:om_root456")
	if err != nil {
		t.Fatalf("ReconstructReplyCtx(thread) error = %v", err)
	}
	rc = rctx.(replyContext)
	if rc.chatID != "oc_chat123" {
		t.Fatalf("thread chatID = %q, want oc_chat123", rc.chatID)
	}
	if rc.messageID != "om_root456" {
		t.Fatalf("thread messageID = %q, want om_root456", rc.messageID)
	}

	_, err = base.ReconstructReplyCtx("feishu:oc_chat:ou_user")
	if err == nil {
		t.Fatal("expected error for feishu-prefixed key on lark platform")
	}
}

func stringPtr(s string) *string { return &s }

func feishuTextEvent(messageID, chatID, openID, chatType, content string, mentions []*larkim.MentionEvent) *larkim.P2MessageReceiveV1 {
	msgType := "text"
	senderType := "user"
	createText := strconv.FormatInt(time.Now().UnixMilli(), 10)
	return &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: &openID},
				SenderType: &senderType,
			},
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &content,
				CreateTime:  &createText,
				Mentions:    mentions,
			},
		},
	}
}

func TestSanitizeMarkdownURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "http link kept",
			input: "see [docs](http://example.com)",
			want:  "see [docs](http://example.com)",
		},
		{
			name:  "https link kept",
			input: "see [docs](https://example.com/path)",
			want:  "see [docs](https://example.com/path)",
		},
		{
			name:  "file scheme removed",
			input: "open [file](file:///tmp/foo.txt)",
			want:  "open file (file:///tmp/foo.txt)",
		},
		{
			name:  "data scheme removed",
			input: "img [pic](data:image/png;base64,abc)",
			want:  "img pic (data:image/png;base64,abc)",
		},
		{
			name:  "mixed links",
			input: "[ok](https://x.com) and [bad](file:///etc/passwd)",
			want:  "[ok](https://x.com) and bad (file:///etc/passwd)",
		},
		{
			name:  "no links unchanged",
			input: "plain text without links",
			want:  "plain text without links",
		},
		{
			name:  "ftp scheme removed",
			input: "[dl](ftp://files.example.com/f.zip)",
			want:  "dl (ftp://files.example.com/f.zip)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeMarkdownURLs(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeMarkdownURLs(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLark_ErrorMessagePrefix(t *testing.T) {
	_, err := newPlatform("lark", lark.LarkBaseUrl, map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
	if !strings.HasPrefix(err.Error(), "lark:") {
		t.Fatalf("error = %q, want lark: prefix", err.Error())
	}
}

func TestBuildPreviewCardJSON_ProgressPayloadUsesStructuredCard(t *testing.T) {
	payload := core.BuildProgressCardPayloadV2([]core.ProgressCardEntry{
		{Kind: core.ProgressEntryThinking, Text: "planning"},
		{Kind: core.ProgressEntryToolUse, Tool: "Bash", Text: "pwd"},
	}, false, "Codex", core.LangEnglish, core.ProgressCardStateRunning)
	if payload == "" {
		t.Fatal("BuildProgressCardPayload returned empty payload")
	}

	cardJSON := buildPreviewCardJSON(payload)
	if strings.Contains(cardJSON, core.ProgressCardPayloadPrefix) {
		t.Fatalf("card JSON should not leak payload prefix, got %q", cardJSON)
	}
	if !strings.Contains(cardJSON, "Codex · Running") {
		t.Fatalf("card JSON should contain progress title, got %q", cardJSON)
	}
	if strings.Contains(cardJSON, "\"tag\":\"note\"") {
		t.Fatalf("card JSON should not use deprecated note tag, got %q", cardJSON)
	}
	if !strings.Contains(cardJSON, "\"text_color\":\"grey\"") {
		t.Fatalf("card JSON should render thinking with grey style, got %q", cardJSON)
	}
	if !strings.Contains(cardJSON, "\\u003ctext_tag color='blue'\\u003eTool") {
		t.Fatalf("card JSON should include tool label, got %q", cardJSON)
	}

	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("card JSON is invalid: %v", err)
	}
	header, ok := card["header"].(map[string]any)
	if !ok || header == nil {
		t.Fatalf("expected header in card json, got %#v", card["header"])
	}
}

func TestBuildPreviewCardJSON_NormalTextFallback(t *testing.T) {
	cardJSON := buildPreviewCardJSON("plain progress text")
	if strings.Contains(cardJSON, "agentchat · 进度") {
		t.Fatalf("normal text should use default card template, got %q", cardJSON)
	}
	if !strings.Contains(cardJSON, "\"tag\":\"markdown\"") {
		t.Fatalf("default preview card should contain markdown element, got %q", cardJSON)
	}
}

func TestFormatProgressToolInput_TodoWrite(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantContains    []string
		notWantContains []string
	}{
		{
			name: "valid todos with all statuses",
			input: `{"todos": [
				{"content": "Task 1", "status": "completed", "activeForm": "Completing task 1"},
				{"content": "Task 2", "status": "in_progress", "activeForm": "Working on task 2"},
				{"content": "Task 3", "status": "pending", "activeForm": "Planning task 3"}
			]}`,
			wantContains:    []string{"✅", "🔄", "⏳", "Task 1", "Task 2", "Task 3", "Completing task 1", "Working on task 2"},
			notWantContains: []string{"```"},
		},
		{
			name:            "todos without activeForm",
			input:           `{"todos": [{"content": "Simple task", "status": "pending"}]}`,
			wantContains:    []string{"⏳", "Simple task"},
			notWantContains: []string{"(", ")"},
		},
		{
			name:         "invalid JSON falls back to default",
			input:        `not valid json`,
			wantContains: []string{"```text"},
		},
		{
			name:         "empty todos array",
			input:        `{"todos": []}`,
			wantContains: []string{"```text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatProgressToolInput("TodoWrite", tt.input)
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("result should contain %q, got %q", want, result)
				}
			}
			for _, notWant := range tt.notWantContains {
				if strings.Contains(result, notWant) {
					t.Errorf("result should not contain %q, got %q", notWant, result)
				}
			}
		})
	}
}

func TestFormatProgressToolInput_OtherTools(t *testing.T) {
	// Non-TodoWrite tools should use default formatting
	result := formatProgressToolInput("Bash", "ls -la")
	if !strings.Contains(result, "```bash") {
		t.Errorf("Bash tool should use bash code block, got %q", result)
	}

	// TodoWrite with invalid JSON should fall back to text block
	result = formatProgressToolInput("TodoWrite", "not json")
	if !strings.Contains(result, "```text") {
		t.Errorf("TodoWrite with invalid JSON should fall back to text block, got %q", result)
	}
}

// --- Mention resolution tests ---

func TestResolveMentions_ReplacesKnownMember(t *testing.T) {
	p := &Platform{platformName: "feishu", resolveMentions: true}
	p.chatMemberCache.Store("oc_chat", &chatMemberEntry{
		members:   map[string]string{"张三": "ou_zhangsan", "李四": "ou_lisi"},
		fetchedAt: time.Now(),
	})
	input := "巡检完成，@张三 @李四 请查看"
	result := p.resolveMentionsInContent(context.Background(), "oc_chat", input)
	if !strings.Contains(result, `<at user_id="ou_zhangsan">张三</at>`) {
		t.Fatalf("expected 张三 to be resolved, got %q", result)
	}
	if !strings.Contains(result, `<at user_id="ou_lisi">李四</at>`) {
		t.Fatalf("expected 李四 to be resolved, got %q", result)
	}
}

func TestResolveMentions_UnknownMemberKeptAsIs(t *testing.T) {
	p := &Platform{platformName: "feishu", resolveMentions: true}
	p.chatMemberCache.Store("oc_chat", &chatMemberEntry{
		members:   map[string]string{"张三": "ou_zhangsan"},
		fetchedAt: time.Now(),
	})
	input := "@不存在的人 请查看"
	result := p.resolveMentionsInContent(context.Background(), "oc_chat", input)
	if strings.Contains(result, "<at") {
		t.Fatalf("unknown member should not be replaced, got %q", result)
	}
}

func TestResolveMentions_LongestMatchFirst(t *testing.T) {
	p := &Platform{platformName: "feishu", resolveMentions: true}
	p.chatMemberCache.Store("oc_chat", &chatMemberEntry{
		members:   map[string]string{"张三": "ou_zhangsan", "张三丰": "ou_zhangsanfeng"},
		fetchedAt: time.Now(),
	})
	input := "@张三丰请查看"
	result := p.resolveMentionsInContent(context.Background(), "oc_chat", input)
	if !strings.Contains(result, "ou_zhangsanfeng") {
		t.Fatalf("should match 张三丰 (longest), got %q", result)
	}
}

func TestResolveMentions_CardFormat(t *testing.T) {
	p := &Platform{platformName: "feishu", resolveMentions: true}
	p.chatMemberCache.Store("oc_chat", &chatMemberEntry{
		members:   map[string]string{"张三": "ou_zhangsan"},
		fetchedAt: time.Now(),
	})
	// Content with complex markdown triggers card format
	input := "# 巡检报告\n\n@张三 请查看\n\n```\nstatus: ok\n```"
	result := p.resolveMentionsInContent(context.Background(), "oc_chat", input)
	if !strings.Contains(result, "<at id=ou_zhangsan></at>") {
		t.Fatalf("card format should use <at id=...>, got %q", result)
	}
}

func TestResolveMentions_DisabledByConfig(t *testing.T) {
	p := &Platform{platformName: "feishu", resolveMentions: false}
	p.chatMemberCache.Store("oc_chat", &chatMemberEntry{
		members:   map[string]string{"张三": "ou_zhangsan"},
		fetchedAt: time.Now(),
	})
	input := "@张三 请查看"
	result := p.resolveMentionsInContent(context.Background(), "oc_chat", input)
	if result != input {
		t.Fatalf("resolve_mentions=false should not replace, got %q", result)
	}
}

func TestResolveMentions_NoAtSign(t *testing.T) {
	p := &Platform{platformName: "feishu", resolveMentions: true}
	input := "普通消息没有at"
	result := p.resolveMentionsInContent(context.Background(), "oc_chat", input)
	if result != input {
		t.Fatalf("no @ should return unchanged, got %q", result)
	}
}

func TestResolveMentions_DuplicateNameSkipped(t *testing.T) {
	p := &Platform{platformName: "feishu", resolveMentions: true}
	p.chatMemberCache.Store("oc_chat", &chatMemberEntry{
		members:   map[string]string{"张三": "", "李四": "ou_lisi"},
		fetchedAt: time.Now(),
	})
	input := "请 @张三 和 @李四 看看"
	result := p.resolveMentionsInContent(context.Background(), "oc_chat", input)
	if !strings.Contains(result, "@张三") {
		t.Fatal("ambiguous name should be kept as-is")
	}
	if strings.Contains(result, "@李四") {
		t.Fatal("unique name should be resolved")
	}
}

func TestResolveMentions_SpecialCharsEscaped(t *testing.T) {
	p := &Platform{platformName: "feishu", resolveMentions: true}
	p.chatMemberCache.Store("oc_chat", &chatMemberEntry{
		members:   map[string]string{`A<"B">`: "ou_special"},
		fetchedAt: time.Now(),
	})
	input := `@A<"B"> 你好`
	result := p.resolveMentionsInContent(context.Background(), "oc_chat", input)
	if strings.Contains(result, `<"B">`) {
		t.Fatalf("special chars should be escaped, got %q", result)
	}
	if !strings.Contains(result, "A&lt;") {
		t.Fatalf("expected HTML-escaped name, got %q", result)
	}
}
