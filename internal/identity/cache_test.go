package identity

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCachePersistsUserBotAndAppNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "feishu", "identity_cache.json")
	cache, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	cache.PutUser("ou_user", "用户267197")
	cache.PutBot("ou_bot", "测试机器人")
	cache.PutApp("cli_app", "椿楸Codex")
	if err := cache.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Open(path)
	if err != nil {
		t.Fatalf("Open() reload error = %v", err)
	}
	if got := loaded.Resolve("ou_user", "user"); got != "用户267197" {
		t.Fatalf("Resolve(user) = %q, want 用户267197", got)
	}
	if got := loaded.Resolve("ou_bot", "app"); got != "测试机器人" {
		t.Fatalf("Resolve(bot open_id as app sender) = %q, want 测试机器人", got)
	}
	if got := loaded.Resolve("cli_app", "app"); got != "椿楸Codex" {
		t.Fatalf("Resolve(app) = %q, want 椿楸Codex", got)
	}
}

func TestCacheIgnoresExpiredEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity_cache.json")
	cache, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	cache.ttl = time.Nanosecond
	cache.PutUser("ou_old", "Old")
	time.Sleep(2 * time.Nanosecond)

	if got := cache.Resolve("ou_old", "user"); got != "" {
		t.Fatalf("Resolve(expired) = %q, want empty", got)
	}
}
