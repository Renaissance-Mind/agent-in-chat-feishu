package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/sariel/agent-in-chat-feishu/internal/agent"
	"github.com/sariel/agent-in-chat-feishu/internal/config"
	"github.com/sariel/agent-in-chat-feishu/internal/identity"
	"github.com/sariel/agent-in-chat-feishu/internal/store"
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

func TestProcessMessageAddsAndRemovesReactions(t *testing.T) {
	events := newEventLog()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			writeJSON(t, w, map[string]any{"code": 0, "tenant_access_token": "token-1", "expire": 3600})
		case r.URL.Path == "/open-apis/im/v1/messages/om_trigger/reactions" && r.Method == http.MethodPost:
			var body struct {
				ReactionType struct {
					EmojiType string `json:"emoji_type"`
				} `json:"reaction_type"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode reaction body: %v", err)
			}
			events.add("reaction:" + body.ReactionType.EmojiType)
			reactionID := "react_" + body.ReactionType.EmojiType
			writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"reaction_id": reactionID}})
		case r.URL.Path == "/open-apis/im/v1/messages/om_trigger/reactions/react_OnIt" && r.Method == http.MethodDelete:
			events.add("delete:react_OnIt")
			writeJSON(t, w, map[string]any{"code": 0})
		case r.URL.Path == "/open-apis/im/v1/chats/oc_chat/members":
			writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{}}})
		case r.URL.Path == "/open-apis/im/v1/chats/oc_chat/members/bots":
			writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{}}})
		case r.URL.Path == "/open-apis/im/v1/messages" && r.Method == http.MethodGet:
			writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"items": []any{}, "has_more": false}})
		case r.URL.Path == "/open-apis/im/v1/messages/om_trigger/reply" && r.Method == http.MethodPost:
			events.add("reply")
			writeJSON(t, w, map[string]any{"code": 0})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	r := runtimeForReactionTest(t, server, events)
	r.processMessage(context.Background(), inboundMessage{
		messageID:  "om_trigger",
		chatID:     "oc_chat",
		senderID:   "ou_user",
		senderType: "user",
		text:       "帮我整理一下",
	})

	got := events.snapshot()
	for _, want := range []string{"reaction:OnIt", "runner", "reply", "delete:react_OnIt", "reaction:Done"} {
		if !containsString(got, want) {
			t.Fatalf("events missing %q: %v", want, got)
		}
	}
	assertBefore(t, got, "reaction:OnIt", "runner")
	assertBefore(t, got, "reply", "delete:react_OnIt")
	assertBefore(t, got, "delete:react_OnIt", "reaction:Done")
}

func strPtr(s string) *string {
	return &s
}

type fakeRunner struct {
	events *eventLog
}

func (r fakeRunner) Run(_ context.Context, _, _ string) (agent.Result, error) {
	r.events.add("runner")
	return agent.Result{ThreadID: "thread-1", Text: "整理好了"}, nil
}

type eventLog struct {
	mu     sync.Mutex
	events []string
}

func newEventLog() *eventLog {
	return &eventLog{}
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

func runtimeForReactionTest(t *testing.T, server *httptest.Server, events *eventLog) *Runtime {
	t.Helper()
	dataDir := t.TempDir()
	names, err := identity.Open(identity.CachePath(dataDir))
	if err != nil {
		t.Fatalf("open identity cache: %v", err)
	}
	sessions, err := store.Open(store.SessionsPath(dataDir))
	if err != nil {
		t.Fatalf("open session store: %v", err)
	}
	api := NewAPI("cli_test", "sec_test", server.URL)
	api.client = server.Client()
	return &Runtime{
		cfg: config.Config{
			DataDir: dataDir,
			Feishu: config.FeishuConfig{
				AppID:         "cli_test",
				AppSecret:     "sec_test",
				BaseURL:       server.URL,
				ReactionEmoji: "OnIt",
				DoneEmoji:     "Done",
			},
			Agent:   config.AgentConfig{TimeoutMins: 1},
			Context: config.ContextConfig{MaxMessages: 10},
		},
		api:       api,
		identity:  names,
		sessions:  sessions,
		runner:    fakeRunner{events: events},
		seen:      make(map[string]time.Time),
		chatLocks: make(map[string]*sync.Mutex),
	}
}

func assertBefore(t *testing.T, events []string, first, second string) {
	t.Helper()
	firstIndex := -1
	secondIndex := -1
	for i, event := range events {
		if event == first {
			firstIndex = i
		}
		if event == second {
			secondIndex = i
		}
	}
	if firstIndex == -1 || secondIndex == -1 || firstIndex >= secondIndex {
		t.Fatalf("want %q before %q, got %v", first, second, events)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
