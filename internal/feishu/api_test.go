package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCreateAndDeleteReaction(t *testing.T) {
	var sawCreate bool
	var sawDelete bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			writeJSON(t, w, map[string]any{"code": 0, "tenant_access_token": "token-1", "expire": 3600})
		case r.URL.Path == "/open-apis/im/v1/messages/om_trigger/reactions" && r.Method == http.MethodPost:
			sawCreate = true
			if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
				t.Fatalf("create Authorization = %q", got)
			}
			var body struct {
				ReactionType struct {
					EmojiType string `json:"emoji_type"`
				} `json:"reaction_type"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			if body.ReactionType.EmojiType != "OnIt" {
				t.Fatalf("emoji_type = %q, want OnIt", body.ReactionType.EmojiType)
			}
			writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"reaction_id": "react_1"}})
		case r.URL.Path == "/open-apis/im/v1/messages/om_trigger/reactions/react_1" && r.Method == http.MethodDelete:
			sawDelete = true
			if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
				t.Fatalf("delete Authorization = %q", got)
			}
			writeJSON(t, w, map[string]any{"code": 0})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	api := NewAPI("cli_test", "sec_test", server.URL)
	api.client = server.Client()

	reactionID, err := api.CreateReaction(context.Background(), "om_trigger", "OnIt")
	if err != nil {
		t.Fatalf("CreateReaction() error = %v", err)
	}
	if reactionID != "react_1" {
		t.Fatalf("reactionID = %q, want react_1", reactionID)
	}
	if err := api.DeleteReaction(context.Background(), "om_trigger", reactionID); err != nil {
		t.Fatalf("DeleteReaction() error = %v", err)
	}
	if !sawCreate || !sawDelete {
		t.Fatalf("sawCreate=%v sawDelete=%v", sawCreate, sawDelete)
	}
}

func TestReplyTextRefreshesInvalidTenantToken(t *testing.T) {
	var authCalls atomic.Int32
	var replyCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal":
			call := authCalls.Add(1)
			token := "token-stale"
			if call > 1 {
				token = "token-fresh"
			}
			writeJSON(t, w, map[string]any{"code": 0, "tenant_access_token": token, "expire": 3600})
		case strings.HasSuffix(r.URL.Path, "/reply"):
			call := replyCalls.Add(1)
			switch call {
			case 1:
				if got := r.Header.Get("Authorization"); got != "Bearer token-stale" {
					t.Fatalf("first reply Authorization = %q", got)
				}
				writeJSON(t, w, map[string]any{"code": 99991663, "msg": "Invalid access token"})
			case 2:
				if got := r.Header.Get("Authorization"); got != "Bearer token-fresh" {
					t.Fatalf("second reply Authorization = %q", got)
				}
				writeJSON(t, w, map[string]any{"code": 0, "data": map[string]any{"message_id": "om_reply"}})
			default:
				t.Fatalf("unexpected reply call %d", call)
			}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	api := NewAPI("cli_test", "sec_test", server.URL)
	api.client = server.Client()

	if err := api.ReplyText(context.Background(), "om_trigger", "hello"); err != nil {
		t.Fatalf("ReplyText() error = %v", err)
	}
	if got := authCalls.Load(); got != 2 {
		t.Fatalf("authCalls = %d, want 2", got)
	}
	if got := replyCalls.Load(); got != 2 {
		t.Fatalf("replyCalls = %d, want 2", got)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
