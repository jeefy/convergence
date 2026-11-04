package k8s

import (
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
	if !strings.Contains(manifests, "standin_enforce") {
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
