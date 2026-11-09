package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/models"
)

func TestRefFor_mapsFamilies(t *testing.T) {
	for fam, want := range map[string]string{
		models.UpstreamOpenAI: "openai-direct", models.UpstreamAnthropic: "anthropic-direct",
		models.UpstreamCopilot: "copilot-completions", "other": "copilot-completions",
	} {
		if got := refFor(fam); got != want {
			t.Errorf("refFor(%q) = %q, want %q", fam, got, want)
		}
	}
}

func modelsFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "models.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRenderManifests_errorPaths(t *testing.T) {
	cases := map[string]*ClusterOptions{
		"ollama provider":   {Namespace: "n", Provider: "ollama"},
		"invalid otel":      {Namespace: "n", Provider: "direct", OTel: otelBad()},
		"missing file":      {Namespace: "n", Provider: "direct", ModelsFile: "/nonexistent/models.yaml"},
		"malformed file":    {Namespace: "n", Provider: "direct", ModelsFile: modelsFile(t, "direct: [1]\n")},
		"ollama passthru":   {Namespace: "n", Provider: "direct", ModelsFile: modelsFile(t, "passthrough:\n  x:\n    provider: ollama\n    model: m\n")},
		"copilot on direct": {Namespace: "n", Provider: "direct", ModelsFile: modelsFile(t, "passthrough:\n  x:\n    provider: copilot\n    model: m\n")},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := RenderManifests(opts); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestRenderManifests_passthroughAndPinballFile(t *testing.T) {
	f := modelsFile(t, "passthrough:\n  my-gpt:\n    provider: openai\n    model: gpt-x\n  my-claude:\n    provider: anthropic\n    model: c-x\n")
	out, err := RenderManifests(&ClusterOptions{
		Namespace: "n", Provider: "direct", ModelsFile: f,
		ImageAgentGateway: "ag:x", ImageStandIn: "si:x", ImagePinball: "pb:x", ImageCopilotAPI: "ca:x",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"my-gpt", "my-claude", "ag:x", "si:x", "pb:x"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestKubectlWrappers_failWithOutputWhenKubectlFails(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "kubectl"), []byte("#!/bin/sh\necho nope >&2\nexit 1\n"), 0o700); err != nil { //nolint:gosec // test executable
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for name, err := range map[string]error{
		"delete ns": DeleteNamespace("n"), "apply": ApplyManifests("x"), "delete": DeleteManifests("x"),
	} {
		if err == nil || !strings.Contains(err.Error(), "nope") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestKubectlWrappers_succeedWithFakeKubectl(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "kubectl"), []byte("#!/bin/sh\ncat >/dev/null\nexit 0\n"), 0o700); err != nil { //nolint:gosec // test executable
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := DeleteNamespace("n"); err != nil {
		t.Error(err)
	}
	if err := ApplyManifests("x"); err != nil {
		t.Error(err)
	}
	if err := DeleteManifests("x"); err != nil {
		t.Error(err)
	}
}

func otelBad() config.OTel { return config.OTel{Endpoint: "x:1", Protocol: "bogus"} }

func TestRenderManifests_extProcTimeout(t *testing.T) {
	out, err := RenderManifests(&ClusterOptions{Namespace: "n", Provider: "direct"})
	if err != nil || !strings.Contains(out, "requestTimeout: 5s") {
		t.Fatalf("default: err=%v, want requestTimeout: 5s", err)
	}
	out, err = RenderManifests(&ClusterOptions{Namespace: "n", Provider: "direct", ExtProcTimeout: "60s"})
	if err != nil || !strings.Contains(out, "requestTimeout: 60s") || strings.Contains(out, "requestTimeout: 5s") {
		t.Fatalf("override: err=%v", err)
	}
	for _, bad := range []string{"abc", "0s", "-5s", "5"} {
		if _, err := RenderManifests(&ClusterOptions{Namespace: "n", Provider: "direct", ExtProcTimeout: bad}); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}
