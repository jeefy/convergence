package k8s

import (
	"bytes"
	"embed"
	"fmt"
	"os/exec"
	"text/template"

	"github.com/jeefy/convergence/internal/config"
)

//go:embed assets/*
var k8sAssets embed.FS

type ClusterOptions struct {
	Namespace    string
	Provider     string
	CopilotToken string
	OpenAIKey    string
	AnthropicKey string
	StandinSalt  string
	UseBuiltinDB bool
	DBDSN        string
	DryRun       bool
}

func RenderManifests(opts *ClusterOptions) (string, error) {
	tmplData, err := k8sAssets.ReadFile("assets/manifests.yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("reading k8s template: %w", err)
	}

	if opts.StandinSalt == "" {
		opts.StandinSalt, _ = config.GenerateSalt()
	}

	tmpl, err := template.New("k8s").Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("parsing k8s template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, opts); err != nil {
		return "", fmt.Errorf("rendering k8s template: %w", err)
	}
	return buf.String(), nil
}

func ApplyManifests(manifests string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(manifests)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w (output: %s)", err, string(out))
	}
	return nil
}

func DeleteManifests(manifests string) error {
	cmd := exec.Command("kubectl", "delete", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(manifests)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete failed: %w (output: %s)", err, string(out))
	}
	return nil
}
