package feishu

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type feishuIdentityCache struct {
	mu   sync.Mutex
	path string
	data feishuIdentityCacheData
}

type feishuIdentityCacheData struct {
	UserNames   map[string]string                 `json:"user_names,omitempty"`
	ChatNames   map[string]string                 `json:"chat_names,omitempty"`
	ChatMembers map[string]feishuCachedChatMember `json:"chat_members,omitempty"`
}

type feishuCachedChatMember struct {
	Members   map[string]string `json:"members,omitempty"`
	NamesByID map[string]string `json:"names_by_id,omitempty"`
	FetchedAt time.Time         `json:"fetched_at,omitempty"`
}

func newFeishuIdentityCache(dataDir, projectName, appID string) *feishuIdentityCache {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil
	}
	name := sanitizeFeishuCacheKey(projectName)
	if name == "" {
		name = sanitizeFeishuCacheKey(appID)
	}
	if name == "" {
		name = "default"
	}
	cache := &feishuIdentityCache{
		path: filepath.Join(dataDir, "feishu", name, "identity_cache.json"),
		data: feishuIdentityCacheData{
			UserNames:   make(map[string]string),
			ChatNames:   make(map[string]string),
			ChatMembers: make(map[string]feishuCachedChatMember),
		},
	}
	cache.load()
	return cache
}

var feishuCacheKeyRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func sanitizeFeishuCacheKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.Trim(feishuCacheKeyRe.ReplaceAllString(s, "_"), "._-")
}

func (c *feishuIdentityCache) load() {
	if c == nil || c.path == "" {
		return
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("feishu identity cache: read failed", "path", c.path, "error", err)
		}
		return
	}
	var decoded feishuIdentityCacheData
	if err := json.Unmarshal(data, &decoded); err != nil {
		slog.Debug("feishu identity cache: parse failed", "path", c.path, "error", err)
		return
	}
	if decoded.UserNames == nil {
		decoded.UserNames = make(map[string]string)
	}
	if decoded.ChatNames == nil {
		decoded.ChatNames = make(map[string]string)
	}
	if decoded.ChatMembers == nil {
		decoded.ChatMembers = make(map[string]feishuCachedChatMember)
	}
	c.data = decoded
}

func (c *feishuIdentityCache) lookupUserName(id string) string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.TrimSpace(c.data.UserNames[strings.TrimSpace(id)])
}

func (c *feishuIdentityCache) storeUserName(id, name string) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if c == nil || id == "" || name == "" || name == id {
		return
	}
	c.mu.Lock()
	c.data.UserNames[id] = name
	c.saveLocked()
	c.mu.Unlock()
}

func (c *feishuIdentityCache) lookupChatName(id string) string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.TrimSpace(c.data.ChatNames[strings.TrimSpace(id)])
}

func (c *feishuIdentityCache) storeChatName(id, name string) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if c == nil || id == "" || name == "" || name == id {
		return
	}
	c.mu.Lock()
	c.data.ChatNames[id] = name
	c.saveLocked()
	c.mu.Unlock()
}

func (c *feishuIdentityCache) lookupChatMemberEntry(chatID string) *chatMemberEntry {
	if c == nil || strings.TrimSpace(chatID) == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cached, ok := c.data.ChatMembers[strings.TrimSpace(chatID)]
	if !ok {
		return nil
	}
	entry := newChatMemberEntry()
	entry.fetchedAt = cached.FetchedAt
	for name, id := range cached.Members {
		entry.members[name] = id
	}
	for id, name := range cached.NamesByID {
		entry.namesByID[id] = name
	}
	return entry
}

func (c *feishuIdentityCache) storeChatMemberEntry(chatID string, entry *chatMemberEntry) {
	chatID = strings.TrimSpace(chatID)
	if c == nil || chatID == "" || entry == nil {
		return
	}
	cached := feishuCachedChatMember{
		Members:   make(map[string]string, len(entry.members)),
		NamesByID: make(map[string]string, len(entry.namesByID)),
		FetchedAt: entry.fetchedAt,
	}
	for name, id := range entry.members {
		cached.Members[name] = id
	}
	for id, name := range entry.namesByID {
		cached.NamesByID[id] = name
	}
	c.mu.Lock()
	c.data.ChatMembers[chatID] = cached
	c.saveLocked()
	c.mu.Unlock()
}

func (c *feishuIdentityCache) saveLocked() {
	if c == nil || c.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		slog.Debug("feishu identity cache: mkdir failed", "path", c.path, "error", err)
		return
	}
	data, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		slog.Debug("feishu identity cache: marshal failed", "path", c.path, "error", err)
		return
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		slog.Debug("feishu identity cache: write failed", "path", tmp, "error", err)
		return
	}
	if err := os.Rename(tmp, c.path); err != nil {
		slog.Debug("feishu identity cache: rename failed", "path", c.path, "error", err)
		_ = os.Remove(tmp)
	}
}
