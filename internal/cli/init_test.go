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

func TestResolveConfig_imageOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	f := initFlags{
		yes:          true,
		provider:     "direct",
		openAIKey:    "key",
		imageAGW:     "custom/agw:sha-xyz",
		imageStandIn: "custom/standin:dev",
	}

	if err := resolveConfig(cfg, f, newPrompter("", true)); err != nil {
		t.Fatal(err)
	}
	if cfg.ImageAgentGateway != "custom/agw:sha-xyz" || cfg.ImageStandIn != "custom/standin:dev" {
		t.Errorf("image overrides not set in config: %+v", *cfg)
	}
}

func TestInitComposeProvider(t *testing.T) {
	for _, provider := range []string{"podman-compose", "docker-compose"} {
		t.Run(provider, func(t *testing.T) {
			newFakeEnv(t, "docker", "podman", provider)
			args := []string{"init", "--yes", "--engine", "podman", "--provider", "direct"}
			if err := runCLI(t, append(args, "--compose-provider", provider)...); err != nil {
				t.Fatal(err)
			}
			for _, override := range []string{"", "auto"} {
				resetFlags(rootCmd)
				next := []string{"init", "--yes", "--provider", "direct"}
				want := provider
				if override != "" {
					next = append(next, "--compose-provider", override)
					want = override
				}
				if err := runCLI(t, next...); err != nil {
					t.Fatal(err)
				}
				cfg, err := config.LoadConfig()
				if err != nil {
					t.Fatal(err)
				}
				if cfg.ComposeProvider != want {
					t.Fatalf("provider=%q, want %q", cfg.ComposeProvider, want)
				}
			}
		})
	}
}

func TestInitComposeProviderErrors(t *testing.T) {
	for _, tc := range []struct {
		engine, provider, wantErr string
	}{
		{"podman", "invalid", "invalid compose_provider"},
		{"docker", "podman-compose", "requires engine podman"},
		{"podman", "podman-compose", "not available on PATH"},
	} {
		t.Run(tc.engine+"/"+tc.provider, func(t *testing.T) {
			newFakeEnv(t, tc.engine)
			err := runCLI(t, "init", "--yes", "--engine", tc.engine, "--compose-provider", tc.provider)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want %q", err, tc.wantErr)
			}

			if exists, err := config.Exists(); err != nil || exists {
				t.Fatalf("invalid provider must not save config: exists=%v, err=%v", exists, err)
			}
		})
	}
}
