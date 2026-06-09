package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const DefaultTTL = 24 * time.Hour

type Entry struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

type snapshot struct {
	Version int              `json:"version"`
	Users   map[string]Entry `json:"users,omitempty"`
	Bots    map[string]Entry `json:"bots,omitempty"`
	Apps    map[string]Entry `json:"apps,omitempty"`
}

type Cache struct {
	path string
	ttl  time.Duration
	mu   sync.RWMutex
	data snapshot
}

func Open(path string) (*Cache, error) {
	cache := &Cache{
		path: path,
		ttl:  DefaultTTL,
		data: snapshot{
			Version: 1,
			Users:   make(map[string]Entry),
			Bots:    make(map[string]Entry),
			Apps:    make(map[string]Entry),
		},
	}
	if strings.TrimSpace(path) == "" {
		return cache, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return cache, nil
	}
	if err := json.Unmarshal(raw, &cache.data); err != nil {
		return nil, fmt.Errorf("identity cache decode: %w", err)
	}
	cache.ensureMaps()
	return cache, nil
}

func (c *Cache) PutUser(id, name string) {
	c.put("user", id, name)
}

func (c *Cache) PutBot(id, name string) {
	c.put("bot", id, name)
}

func (c *Cache) PutApp(id, name string) {
	c.put("app", id, name)
}

func (c *Cache) Resolve(id, senderType string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	switch strings.ToLower(strings.TrimSpace(senderType)) {
	case "user":
		return c.resolveLocked(c.data.Users, id)
	case "app":
		if name := c.resolveLocked(c.data.Bots, id); name != "" {
			return name
		}
		return c.resolveLocked(c.data.Apps, id)
	default:
		if name := c.resolveLocked(c.data.Users, id); name != "" {
			return name
		}
		if name := c.resolveLocked(c.data.Bots, id); name != "" {
			return name
		}
		return c.resolveLocked(c.data.Apps, id)
	}
}

func (c *Cache) Save() error {
	if strings.TrimSpace(c.path) == "" {
		return nil
	}
	c.mu.RLock()
	data, err := json.MarshalIndent(c.data, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".identity-cache-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, c.path)
}

func (c *Cache) put(kind, id, name string) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" || name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureMapsLocked()
	entry := Entry{Name: name, UpdatedAt: time.Now()}
	switch kind {
	case "user":
		c.data.Users[id] = entry
	case "bot":
		c.data.Bots[id] = entry
	case "app":
		c.data.Apps[id] = entry
	}
}

func (c *Cache) resolveLocked(target map[string]Entry, id string) string {
	entry, ok := target[id]
	if !ok {
		return ""
	}
	if c.ttl > 0 && entry.UpdatedAt.Add(c.ttl).Before(time.Now()) {
		return ""
	}
	return strings.TrimSpace(entry.Name)
}

func (c *Cache) ensureMaps() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureMapsLocked()
}

func (c *Cache) ensureMapsLocked() {
	if c.data.Version == 0 {
		c.data.Version = 1
	}
	if c.data.Users == nil {
		c.data.Users = make(map[string]Entry)
	}
	if c.data.Bots == nil {
		c.data.Bots = make(map[string]Entry)
	}
	if c.data.Apps == nil {
		c.data.Apps = make(map[string]Entry)
	}
}

func CachePath(dataDir string) string {
	return filepath.Join(dataDir, "cache", "feishu", "identity_cache.json")
}
