package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func TestResolveImages_nilConfigReturnsPinned(t *testing.T) {
	if got := ResolveImages(nil); got != PinnedImages {
		t.Fatalf("got %+v, want pinned", got)
	}
}

func TestResolveImages_overridesEveryImage(t *testing.T) {
	cfg := &config.Config{
		ImageAgentGateway: "a", ImageStandIn: "s", ImagePinball: "p",
		ImageCopilotAPI: "c", ImageOpenShellGateway: "o",
	}
	want := Images{AgentGateway: "a", StandIn: "s", Pinball: "p", CopilotAPI: "c", Postgres: PinnedImages.Postgres, OpenShellGateway: "o"}
	if got := ResolveImages(cfg); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestVolumeSuffix_isRelabelOnLinuxOnly(t *testing.T) {
	if got := VolumeSuffix(); got != ",z" && got != "" {
		t.Fatalf("unexpected suffix %q", got)
	}
}

func TestProviderRef_mapsFamiliesAndRejectsUnknown(t *testing.T) {
	for fam, want := range map[string]string{
		"copilot": "copilot-completions", "ollama": "ollama-local",
		"openai": "openai-direct", "anthropic": "anthropic-direct",
	} {
		got, err := providerRef(fam)
		if err != nil || got != want {
			t.Errorf("providerRef(%q) = %q, %v; want %q", fam, got, err, want)
		}
	}
	if _, err := providerRef("bogus"); err == nil {
		t.Fatal("want error for unknown family")
	}
}

func TestRenderAssets_errorPaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Run("target dir is a file", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(f, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := RenderAssets(config.DefaultConfig(), filepath.Join(f, "sub")); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("invalid otel", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.OTel = config.OTel{Endpoint: "x:4317", Protocol: "bogus"}
		if err := RenderAssets(cfg, t.TempDir()); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("bad models file", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "models.yaml"), []byte("direct: [1,2]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := RenderAssets(config.DefaultConfig(), dir); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("output path is a directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ComposeFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := RenderAssets(config.DefaultConfig(), dir); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestRenderAssets_flagMatrixRenders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, p := range []config.ProviderType{config.ProviderCopilot, config.ProviderDirect, config.ProviderOllama} {
		for _, sbx := range []config.SandboxType{config.SandboxNone, config.SandboxOpenShell} {
			cfg := config.DefaultConfig()
			cfg.Provider, cfg.Sandbox = p, sbx
			cfg.FailOpen = true
			cfg.BindAddress = "0.0.0.0"
			cfg.RequestLogDB = "postgres://u:p@h/db"
			cfg.OTel = config.OTel{Endpoint: "otel:4317", Protocol: "http", Sample: 0.5}
			cfg.CopilotToken = "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789"
			if err := RenderAssets(cfg, t.TempDir()); err != nil {
				t.Errorf("%s/%s: %v", p, sbx, err)
			}
		}
	}
}

func TestComposeStandInStopGracePeriodExceedsDrain(t *testing.T) {
	_, files := render(t, func(c *config.Config) {})
	if !strings.Contains(files[ComposeFile], "stop_grace_period: 20s") {
		t.Errorf("stand-in needs stop_grace_period above its 10s drain:\n%s", files[ComposeFile])
	}
}
