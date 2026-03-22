package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultHTTPAddr    = ":8080"
	defaultStateFile   = "data/state.json"
	defaultBotName     = "Weixin iLink Bridge"
	defaultBotUsername = "openilink_tg_bot"
	defaultBotID       = int64(1)
)

// Config 定义服务运行所需的全部配置项。
type Config struct {
	HTTPAddr       string
	TelegramToken  string
	ILinkToken     string
	ILinkBaseURL   string
	StateFile      string
	BotID          int64
	BotName        string
	BotUsername    string
	MonitorEnabled bool
}

// Load 从真实环境变量加载配置。
func Load() (Config, error) {
	return LoadFromEnv(os.Getenv)
}

// LoadFromEnv 支持通过注入 getenv 实现可测试配置加载。
func LoadFromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPAddr:       valueOrDefault(strings.TrimSpace(getenv("HTTP_ADDR")), defaultHTTPAddr),
		TelegramToken:  strings.TrimSpace(getenv("TELEGRAM_BOT_TOKEN")),
		ILinkToken:     strings.TrimSpace(getenv("ILINK_TOKEN")),
		ILinkBaseURL:   strings.TrimSpace(getenv("ILINK_BASE_URL")),
		StateFile:      valueOrDefault(strings.TrimSpace(getenv("STATE_FILE")), defaultStateFile),
		BotName:        valueOrDefault(strings.TrimSpace(getenv("BOT_NAME")), defaultBotName),
		BotUsername:    valueOrDefault(strings.TrimSpace(getenv("BOT_USERNAME")), defaultBotUsername),
		MonitorEnabled: true,
		BotID:          defaultBotID,
	}

	monitorEnabled, err := parseBool(getenv("MONITOR_ENABLED"), true)
	if err != nil {
		return Config{}, fmt.Errorf("invalid MONITOR_ENABLED: %w", err)
	}
	cfg.MonitorEnabled = monitorEnabled

	botIDValue := strings.TrimSpace(getenv("BOT_ID"))
	if botIDValue != "" {
		cfg.BotID, err = strconv.ParseInt(botIDValue, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid BOT_ID: %w", err)
		}
	}

	if err = cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate 检查关键配置是否满足启动条件。
func (c Config) Validate() error {
	if c.TelegramToken == "" {
		return errors.New("TELEGRAM_BOT_TOKEN is required")
	}
	if c.ILinkToken == "" {
		return errors.New("ILINK_TOKEN is required")
	}
	return nil
}

func valueOrDefault(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func parseBool(value string, defaultValue bool) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return strconv.ParseBool(value)
}
