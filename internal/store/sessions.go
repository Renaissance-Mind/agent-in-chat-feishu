package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type SessionStore struct {
	path string
	mu   sync.RWMutex
	data map[string]string
}

func Open(path string) (*SessionStore, error) {
	store := &SessionStore{path: path, data: make(map[string]string)}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(raw, &store.data); err != nil {
		return nil, err
	}
	if store.data == nil {
		store.data = make(map[string]string)
	}
	return store, nil
}

func (s *SessionStore) Get(chatID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[chatID]
}

func (s *SessionStore) Set(chatID, threadID string) error {
	if chatID == "" || threadID == "" {
		return nil
	}
	s.mu.Lock()
	s.data[chatID] = threadID
	data, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sessions-*.tmp")
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
	return os.Rename(tmpName, s.path)
}

func SessionsPath(dataDir string) string {
	return filepath.Join(dataDir, "sessions", "sessions.json")
}
