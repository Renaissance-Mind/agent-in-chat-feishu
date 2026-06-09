package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DataDir string        `toml:"data_dir"`
	Feishu  FeishuConfig  `toml:"feishu"`
	Agent   AgentConfig   `toml:"agent"`
	Context ContextConfig `toml:"context"`
}

type FeishuConfig struct {
	AppID        string   `toml:"app_id"`
	AppSecret    string   `toml:"app_secret"`
	AllowedChats []string `toml:"allowed_chats"`
	BaseURL      string   `toml:"base_url"`
}

type AgentConfig struct {
	Command         string `toml:"command"`
	WorkDir         string `toml:"work_dir"`
	Model           string `toml:"model"`
	Mode            string `toml:"mode"`
	ReasoningEffort string `toml:"reasoning_effort"`
	CodexHome       string `toml:"codex_home"`
	TimeoutMins     int    `toml:"timeout_mins"`
}

type ContextConfig struct {
	MaxMessages int `toml:"max_messages"`
	MaxAgeMins  int `toml:"max_age_mins"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, err
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) ApplyDefaults() {
	if c.DataDir == "" {
		c.DataDir = DefaultDataDir()
	}
	if c.Feishu.BaseURL == "" {
		c.Feishu.BaseURL = "https://open.feishu.cn"
	}
	if c.Agent.Command == "" {
		c.Agent.Command = "codex"
	}
	if c.Agent.WorkDir == "" {
		c.Agent.WorkDir = "."
	}
	if c.Agent.TimeoutMins <= 0 {
		c.Agent.TimeoutMins = 30
	}
	if c.Context.MaxMessages <= 0 {
		c.Context.MaxMessages = 100
	}
	c.DataDir = expandHome(c.DataDir)
	c.Agent.WorkDir = expandHome(c.Agent.WorkDir)
	c.Agent.CodexHome = expandHome(c.Agent.CodexHome)
}

func (c Config) Validate() error {
	if c.Feishu.AppID == "" {
		return fmt.Errorf("feishu.app_id is required")
	}
	if c.Feishu.AppSecret == "" {
		return fmt.Errorf("feishu.app_secret is required")
	}
	if c.Context.MaxMessages < 1 {
		return fmt.Errorf("context.max_messages must be >= 1")
	}
	if c.Context.MaxAgeMins < 0 {
		return fmt.Errorf("context.max_age_mins must be >= 0")
	}
	return nil
}

func (c Config) AgentTimeout() time.Duration {
	return time.Duration(c.Agent.TimeoutMins) * time.Minute
}

func (c Config) ContextMaxAge() time.Duration {
	if c.Context.MaxAgeMins <= 0 {
		return 0
	}
	return time.Duration(c.Context.MaxAgeMins) * time.Minute
}

func EnsureDataDirs(dataDir string) error {
	for _, dir := range []string{
		dataDir,
		filepath.Join(dataDir, "cache", "feishu"),
		filepath.Join(dataDir, "sessions"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ".agentchat"
	}
	return filepath.Join(home, ".agentchat")
}

func DefaultConfigPath() string {
	return filepath.Join(DefaultDataDir(), "config.toml")
}

func expandHome(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		if home != "" {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
