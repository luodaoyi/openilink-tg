package config

import "testing"

func TestLoadFromEnvUsesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := LoadFromEnv(func(key string) string {
		switch key {
		case "TELEGRAM_BOT_TOKEN":
			return "telegram-secret"
		case "ILINK_TOKEN":
			return "ilink-secret"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected default http addr: %s", cfg.HTTPAddr)
	}

	if cfg.StateFile != "data/state.json" {
		t.Fatalf("unexpected default state file: %s", cfg.StateFile)
	}

	if cfg.BotName == "" || cfg.BotUsername == "" {
		t.Fatalf("expected default bot profile")
	}

	if !cfg.MonitorEnabled {
		t.Fatalf("expected monitor enabled by default")
	}
}

func TestLoadFromEnvValidatesRequiredValues(t *testing.T) {
	t.Parallel()

	_, err := LoadFromEnv(func(key string) string {
		return ""
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
