package config

import "testing"

func TestFeishuReactionDefaults(t *testing.T) {
	cfg := Config{
		Feishu: FeishuConfig{
			AppID:     "cli_test",
			AppSecret: "sec_test",
		},
	}
	cfg.ApplyDefaults()

	if cfg.Feishu.ReactionEmoji != "OnIt" {
		t.Fatalf("ReactionEmoji = %q, want OnIt", cfg.Feishu.ReactionEmoji)
	}
	if cfg.Feishu.DoneEmoji != "" {
		t.Fatalf("DoneEmoji = %q, want empty default", cfg.Feishu.DoneEmoji)
	}
}

func TestFeishuReactionNoneDisablesEmoji(t *testing.T) {
	cfg := Config{
		Feishu: FeishuConfig{
			AppID:         "cli_test",
			AppSecret:     "sec_test",
			ReactionEmoji: "none",
			DoneEmoji:     "Done",
		},
	}
	cfg.ApplyDefaults()

	if cfg.Feishu.ReactionEmoji != "" {
		t.Fatalf("ReactionEmoji = %q, want disabled empty value", cfg.Feishu.ReactionEmoji)
	}
	if cfg.Feishu.DoneEmoji != "Done" {
		t.Fatalf("DoneEmoji = %q, want Done", cfg.Feishu.DoneEmoji)
	}
}
