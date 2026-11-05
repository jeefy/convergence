package k8s

import (
	"errors"
	"io"

	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestRenderManifestsCopilot(t *testing.T) {
	opts := &ClusterOptions{
		Namespace:    "convergence-test",
		Provider:     "copilot",
		CopilotToken: "test_token_copilot",
		StandinSalt:  "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}

	manifests, err := RenderManifests(opts)
	if err != nil {
		t.Fatalf("RenderManifests failed: %v", err)
	}

	if !strings.Contains(manifests, "name: convergence-test") {
		t.Errorf("manifests missing namespace")
	}
	if !strings.Contains(manifests, "name: convergence-copilot") {
		t.Errorf("manifests missing copilot secret")
	}
	if !strings.Contains(manifests, "stand-in.convergence-test.svc.cluster.local:9101") {
		t.Errorf("manifests missing stand-in extProc")
	}
	if !strings.Contains(manifests, "image: cr.agentgateway.dev/agentgateway:v1.6.0") {
		t.Errorf("manifests missing agentgateway image")
	}
}

func TestRenderManifestsDirect(t *testing.T) {
	opts := &ClusterOptions{
		Namespace:    "convergence-direct",
		Provider:     "direct",
		OpenAIKey:    "sk-test-direct",
		AnthropicKey: "sk-ant-direct",
	}

	manifests, err := RenderManifests(opts)
	if err != nil {
		t.Fatalf("RenderManifests failed: %v", err)
	}

	if strings.Contains(manifests, "name: convergence-copilot") {
		t.Errorf("direct provider should not have copilot secret")
	}
	if !strings.Contains(manifests, "sk-test-direct") {
		t.Errorf("manifests missing OpenAI key")
	}
}

func renderDocs(t *testing.T, opts *ClusterOptions) []map[string]any {
	t.Helper()
	out, err := RenderManifests(opts)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	dec := yaml.NewDecoder(strings.NewReader(out))
	var docs []map[string]any
	for {
		var d map[string]any
		if err := dec.Decode(&d); err != nil {
			if errors.Is(err, io.EOF) {
				return docs
			}
			t.Fatalf("invalid YAML: %v", err)
		}
		if d != nil {
			docs = append(docs, d)
		}
	}
}

func findKind(docs []map[string]any, kind string) map[string]any {
	for _, d := range docs {
		if d["kind"] == kind {
			return d
		}
	}
	return nil
}

func TestRenderValidYAMLMatrix(t *testing.T) {
	for _, prov := range []string{"copilot", "direct"} {
		for _, dsn := range []string{"", "postgres://u:p@h/db"} {
			docs := renderDocs(t, &ClusterOptions{Namespace: "x", Provider: prov, OptimizerDSN: dsn})
			if len(docs) == 0 {
				t.Fatalf("%s/%q: no docs", prov, dsn)
			}
			if (findKind(docs, "CronJob") != nil) != (dsn != "") {
				t.Errorf("%s/%q: CronJob presence wrong", prov, dsn)
			}
			if (findKind(docs, "PersistentVolumeClaim") != nil) != (dsn != "") {
				t.Errorf("%s/%q: PVC presence wrong", prov, dsn)
			}
		}
	}
}

func TestOptSchedule(t *testing.T) {
	out, err := RenderManifests(&ClusterOptions{Namespace: "x", Provider: "direct", OptimizerDSN: "d", OptSchedule: "5 4 * * 1"})
	if err != nil || !strings.Contains(out, `schedule: "5 4 * * 1"`) {
		t.Fatalf("schedule override not honored: %v", err)
	}
	out, _ = RenderManifests(&ClusterOptions{Namespace: "x", Provider: "direct", OptimizerDSN: "d"})
	if !strings.Contains(out, `schedule: "0 3 * * 0"`) {
		t.Errorf("default schedule missing")
	}
}

func TestInvalidProviderRejected(t *testing.T) {
	if _, err := RenderManifests(&ClusterOptions{Namespace: "x", Provider: "ollama"}); err == nil {
		t.Fatal("expected error for ollama")
	}
}

func TestNoCommandOverridesEntrypoint(t *testing.T) {
	out, _ := RenderManifests(&ClusterOptions{Namespace: "x", Provider: "copilot"})
	if strings.Contains(out, `command: ["-f"`) {
		t.Error("agentgateway must use args")
	}
}
