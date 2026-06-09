package feishu

import (
	"context"
	"testing"
)

func TestFeishuIdentityCachePersistsNames(t *testing.T) {
	dir := t.TempDir()
	opts := map[string]any{
		"app_id":             "cli_cache_test",
		"app_secret":         "sec_cache_test",
		"cc_data_dir":        dir,
		"cc_project":         "cache-project",
		"enable_feishu_card": false,
	}
	raw, err := newPlatform("feishu", "https://open.feishu.cn", opts)
	if err != nil {
		t.Fatalf("newPlatform() error = %v", err)
	}
	p := raw.(*Platform)
	p.identityCache.storeUserName("ou_user", "Alex")
	p.identityCache.storeChatName("oc_chat", "Research Loop")
	entry := newChatMemberEntry()
	entry.addMember("Mina", "ou_mina")
	p.storeChatMemberEntry("oc_chat", entry)

	raw2, err := newPlatform("feishu", "https://open.feishu.cn", opts)
	if err != nil {
		t.Fatalf("newPlatform() second error = %v", err)
	}
	p2 := raw2.(*Platform)
	if got := p2.identityCache.lookupUserName("ou_user"); got != "Alex" {
		t.Fatalf("cached user name = %q, want Alex", got)
	}
	if got := p2.identityCache.lookupChatName("oc_chat"); got != "Research Loop" {
		t.Fatalf("cached chat name = %q, want Research Loop", got)
	}
	if got := p2.resolveChatMemberName(context.Background(), "oc_chat", "ou_mina"); got != "Mina" {
		t.Fatalf("cached member name = %q, want Mina", got)
	}
}

func TestGroupHistorySenderNameMapsAppFromMemberCache(t *testing.T) {
	raw, err := newPlatform("feishu", "https://open.feishu.cn", map[string]any{
		"app_id":             "cli_app",
		"app_secret":         "sec_app",
		"enable_feishu_card": false,
	})
	if err != nil {
		t.Fatalf("newPlatform() error = %v", err)
	}
	p := raw.(*Platform)
	entry := newChatMemberEntry()
	entry.addMember("Build Bot", "cli_build_bot")
	p.storeChatMemberEntry("oc_chat", entry)

	if got := p.groupHistorySenderName(context.Background(), "oc_chat", "cli_build_bot", "app"); got != "Build Bot" {
		t.Fatalf("app sender name = %q, want Build Bot", got)
	}
}
