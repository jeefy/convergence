package cli

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func newPrompter(input string, yes bool) *prompter {
	return &prompter{in: bufio.NewReader(strings.NewReader(input)), out: io.Discard, yes: yes}
}

func TestResolveConfig_promptsWhenNoFlags(t *testing.T) {
	cfg := config.DefaultConfig()
	salt := cfg.StandinSalt

	err := resolveConfig(cfg, initFlags{}, newPrompter("2\nant-key\noai-key\n", false))

	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != config.ProviderDirect || cfg.AnthropicKey != "ant-key" || cfg.OpenAIKey != "oai-key" || cfg.StandinSalt != salt {
		t.Errorf("unexpected cfg %+v", *cfg)
	}
}

func TestResolveConfig_flagsWithYesNeverRead(t *testing.T) {
	cfg := config.DefaultConfig()
	f := initFlags{yes: true, provider: "copilot", copilotToken: "tok", port: 4000, bindAddress: "0.0.0.0"}

	err := resolveConfig(cfg, f, newPrompter("", true))

	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != config.ProviderCopilot || cfg.CopilotToken != "tok" || cfg.GatewayPort != 4000 || cfg.BindAddress != "0.0.0.0" {
		t.Errorf("unexpected cfg %+v", *cfg)
	}
}

func TestResolveConfig_rerunKeepsExistingKeysOnEnter(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderDirect
	cfg.OpenAIKey, cfg.AnthropicKey = "old-oai", "old-ant"

	err := resolveConfig(cfg, initFlags{}, newPrompter("\n\n\n", false))

	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != config.ProviderDirect || cfg.OpenAIKey != "old-oai" || cfg.AnthropicKey != "old-ant" {
		t.Errorf("existing values clobbered: %+v", *cfg)
	}
}

func TestResolveConfig_flagOverwriteNeedsConfirmation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OpenAIKey = "old-oai"
	f := initFlags{provider: "direct", openAIKey: "new-oai"}

	err := resolveConfig(cfg, f, newPrompter("\nn\n", false))

	if err == nil || cfg.OpenAIKey != "old-oai" {
		t.Fatalf("expected refusal, got err=%v key=%s", err, cfg.OpenAIKey)
	}
}

func TestResolveConfig_flagOverwriteConfirmed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OpenAIKey = "old-oai"
	f := initFlags{provider: "direct", openAIKey: "new-oai"}

	err := resolveConfig(cfg, f, newPrompter("\ny\n", false))

	if err != nil || cfg.OpenAIKey != "new-oai" {
		t.Fatalf("expected overwrite, got err=%v key=%s", err, cfg.OpenAIKey)
	}
}

func TestResolveConfig_rejectsBadProvider(t *testing.T) {
	if err := resolveConfig(config.DefaultConfig(), initFlags{provider: "bogus"}, newPrompter("", true)); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
