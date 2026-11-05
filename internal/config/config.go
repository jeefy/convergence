package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ProviderType string

const (
	ProviderCopilot ProviderType = "copilot"
	ProviderDirect  ProviderType = "direct"
	ProviderOllama  ProviderType = "ollama"
)

type Config struct {
	Engine       string       `yaml:"engine"`        // "docker" or "podman"
	Provider     ProviderType `yaml:"provider"`      // "copilot", "direct", "ollama"
	GatewayPort  int          `yaml:"gateway_port"`  // default 3000
	StandinSalt  string       `yaml:"standin_salt"`  // 32-byte hex string
	OptSchedule  string       `yaml:"opt_schedule,omitempty"` // cron expression, default "0 3 * * 0" (weekly Sunday 03:00)
	CopilotToken string       `yaml:"copilot_token,omitempty"`
	OpenAIKey    string       `yaml:"openai_key,omitempty"`
	AnthropicKey string       `yaml:"anthropic_key,omitempty"`
	GeminiKey    string       `yaml:"gemini_key,omitempty"`
	OllamaURL    string       `yaml:"ollama_url,omitempty"`
}

func DefaultConfig() *Config {
	salt, _ := GenerateSalt()
	return &Config{
		Engine:      "docker",
		Provider:    ProviderDirect,
		GatewayPort: 3000,
		StandinSalt: salt,
		OptSchedule: "0 3 * * 0", // weekly Sunday 03:00
		OllamaURL:   "http://host.docker.internal:11434",
	}
}

func GenerateSalt() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".convergence")
	return dir, nil
}

func LoadConfig() (*Config, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.StandinSalt == "" {
		cfg.StandinSalt, _ = GenerateSalt()
	}
	if cfg.GatewayPort == 0 {
		cfg.GatewayPort = 3000
	}
	if cfg.OptSchedule == "" {
		cfg.OptSchedule = "0 3 * * 0"
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	path := filepath.Join(dir, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}
