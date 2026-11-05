package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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

// ParseProvider converts user input into a known ProviderType.
func ParseProvider(s string) (ProviderType, error) {
	switch p := ProviderType(s); p {
	case ProviderCopilot, ProviderDirect, ProviderOllama:
		return p, nil
	default:
		return "", fmt.Errorf("unknown provider %q (want copilot, direct or ollama)", s)
	}
}

const (
	DefaultGatewayPort = 3000
	DefaultBindAddress = "127.0.0.1"
	DefaultOptSchedule = "0 3 * * 0" // weekly Sunday 03:00
	DefaultOllamaURL   = "http://host.docker.internal:11434"
)

// Fixed host ports published by the local compose stack (besides the gateway port).
const (
	StandinMetricsPort = 9102
	PinballPort        = 9201
	GatewayStatsPort   = 15020
	CopilotAPIPort     = 4141
)

type Config struct {
	Engine       string       `yaml:"engine"`                 // "docker" or "podman"
	Provider     ProviderType `yaml:"provider"`               // "copilot", "direct", "ollama"
	GatewayPort  int          `yaml:"gateway_port"`           // default 3000
	BindAddress  string       `yaml:"bind_address"`           // host address published ports bind to; default 127.0.0.1
	StandinSalt  string       `yaml:"standin_salt"`           // 32-byte hex string
	OptSchedule  string       `yaml:"opt_schedule,omitempty"` // cron expression
	CopilotToken string       `yaml:"copilot_token,omitempty"`
	OpenAIKey    string       `yaml:"openai_key,omitempty"`
	AnthropicKey string       `yaml:"anthropic_key,omitempty"`
	GeminiKey    string       `yaml:"gemini_key,omitempty"`
	OllamaURL    string       `yaml:"ollama_url,omitempty"`
}

// NamedPort is a host port the stack publishes, with the service that owns it.
type NamedPort struct {
	Name string
	Port int
}

// PublishedPorts lists every host port the rendered compose file publishes.
func (c *Config) PublishedPorts() []NamedPort {
	ports := []NamedPort{
		{"agentgateway", c.GatewayPort},
		{"agentgateway stats", GatewayStatsPort},
		{"stand-in metrics", StandinMetricsPort},
		{"pinball", PinballPort},
	}
	if c.Provider == ProviderCopilot {
		ports = append(ports, NamedPort{"copilot-api", CopilotAPIPort})
	}
	return ports
}

// HealthHost is the host address to reach published ports from this machine.
func (c *Config) HealthHost() string {
	switch c.BindAddress {
	case "", "0.0.0.0", "::":
		return "127.0.0.1"
	default:
		return c.BindAddress
	}
}

func DefaultConfig() *Config {
	salt, _ := GenerateSalt()
	return &Config{
		Engine:      "docker",
		Provider:    ProviderDirect,
		GatewayPort: DefaultGatewayPort,
		BindAddress: DefaultBindAddress,
		StandinSalt: salt,
		OptSchedule: DefaultOptSchedule,
		OllamaURL:   DefaultOllamaURL,
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
	return filepath.Join(home, ".convergence"), nil
}

func configPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Exists reports whether a saved config file is present.
func Exists() (bool, error) {
	path, err := configPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// LoadConfig returns the saved config, or defaults when none exists.
// A config that exists but cannot be parsed is an error (never silently replaced).
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.StandinSalt == "" {
		if cfg.StandinSalt, err = GenerateSalt(); err != nil {
			return nil, fmt.Errorf("generating salt: %w", err)
		}
	}
	if cfg.GatewayPort == 0 {
		cfg.GatewayPort = DefaultGatewayPort
	}
	if cfg.BindAddress == "" {
		cfg.BindAddress = DefaultBindAddress
	}
	if cfg.OptSchedule == "" {
		cfg.OptSchedule = DefaultOptSchedule
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	// 0755: the directory also holds non-secret files bind-mounted into
	// containers; config.yaml and .env themselves stay 0600.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o600)
}
