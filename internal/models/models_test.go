package models

import (
	"errors"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func TestResolve_defaultsForEveryProvider(t *testing.T) {
	want := map[config.ProviderType][3]string{
		config.ProviderCopilot: {"copilot/claude-haiku-5.5", "copilot/claude-sonnet-5.5", "copilot/claude-opus-5.5"},
		config.ProviderDirect:  {"anthropic/claude-haiku-5-5", "anthropic/claude-sonnet-5-5", "anthropic/claude-opus-5-5"},
		config.ProviderOllama:  {"ollama/qwen2.5-coder:7b", "ollama/qwen2.5-coder:14b", "ollama/qwen2.5-coder:32b"},
	}
	for p, w := range want {
		r, err := Resolve(p, nil)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if len(r.Models) != 3 {
			t.Fatalf("%s: %d models", p, len(r.Models))
		}
		for i, m := range r.Models {
			if got := m.UpstreamProvider + "/" + m.UpstreamModel; got != w[i] {
				t.Errorf("%s[%d] = %s, want %s", p, i, got, w[i])
			}
			if m.PromptTokens <= 0 {
				t.Errorf("%s: zero prompt tokens", m.PinballName)
			}
		}
		if r.Tiers[TierLight][0] != "claude-haiku-5.5" || r.Tiers[TierHeavy][0] != "claude-opus-5.5" || r.Fallback != "claude-sonnet-5.5" {
			t.Errorf("%s: bad tiers %v fallback %s", p, r.Tiers, r.Fallback)
		}
	}
}

func TestResolve_overrideWinsAndDirectFamilies(t *testing.T) {
	o := Overrides{"direct": {"claude-haiku-5.5": "openai:gpt-6-luna"}, "ollama": {"claude-opus-5.5": "llama3.3:70b"}}
	r, err := Resolve(config.ProviderDirect, &o)
	if err != nil {
		t.Fatal(err)
	}
	if m := r.Models[0]; m.UpstreamProvider != UpstreamOpenAI || m.UpstreamModel != "gpt-6-luna" {
		t.Errorf("override ignored: %+v", m)
	}
	if !r.UsesFamily("openai") || !r.UsesFamily("anthropic") {
		t.Error("UsesFamily wrong")
	}
	r, _ = Resolve(config.ProviderOllama, &o)
	if r.Models[2].UpstreamModel != "llama3.3:70b" {
		t.Errorf("ollama override ignored: %+v", r.Models[2])
	}
}

func TestResolve_rejectsBadOverrides(t *testing.T) {
	cases := map[string]Overrides{
		"gemini provider": {"gemini": {"claude-haiku-5.5": "x"}},
		"unknown model":   {"copilot": {"nope": "x"}},
		"gemini family":   {"direct": {"claude-haiku-5.5": "gemini:flash"}},
		"empty upstream":  {"copilot": {"claude-haiku-5.5": " "}},
	}
	for name, o := range cases {
		if _, err := Resolve(config.ProviderCopilot, &o); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestBuild_missingMappingNamesTheModels(t *testing.T) {
	_, err := build(config.ProviderCopilot, pool, map[string]string{"claude-haiku-5.5": "x"})
	var me *MissingMappingError
	if !errors.As(err, &me) || strings.Join(me.Missing, ",") != "claude-sonnet-5.5,claude-opus-5.5" {
		t.Fatalf("got %v", err)
	}
}

func TestFromConfigAndStarterRoundTrip(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderOllama
	cfg.OllamaModelLight = "tiny:1b"
	r, err := ForConfig(cfg, t.TempDir())
	if err != nil || r.Models[0].UpstreamModel != "tiny:1b" {
		t.Fatalf("%v %+v", err, r)
	}
	// the starter file is all comments: parses to empty overrides
	o, err := ParseOverrides([]byte(StarterFile()))
	if err != nil || len(o) != 0 {
		t.Fatalf("starter: %v %v", err, o)
	}
}
