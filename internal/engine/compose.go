package engine

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Engine interface {
	Name() string
	Start(ctx context.Context, dir string) error
	Stop(ctx context.Context, dir string) error
	Restart(ctx context.Context, dir string) error
	CheckHealth(ctx context.Context, port int) error
}

type ComposeEngine struct {
	binary string
}

func DetectEngine(preferred string) (*ComposeEngine, error) {
	candidates := []string{}
	if preferred != "" {
		candidates = append(candidates, preferred)
	}
	candidates = append(candidates, "docker", "podman")

	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return &ComposeEngine{binary: path}, nil
		}
	}
	return nil, fmt.Errorf("neither docker nor podman found in PATH")
}

func (e *ComposeEngine) Name() string {
	return filepath.Base(e.binary)
}

func (e *ComposeEngine) runCompose(ctx context.Context, dir string, args ...string) error {
	composeFile := filepath.Join(dir, "docker-compose.yaml")
	fullArgs := []string{"compose", "-f", composeFile}
	fullArgs = append(fullArgs, args...)

	cmd := exec.CommandContext(ctx, e.binary, fullArgs...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s compose %s failed: %w (output: %s)", e.Name(), strings.Join(args, " "), err, string(out))
	}
	return nil
}

func (e *ComposeEngine) Start(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "up", "-d")
}

func (e *ComposeEngine) Stop(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "down")
}

func (e *ComposeEngine) Restart(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "restart")
}

func (e *ComposeEngine) CheckHealth(ctx context.Context, port int) error {
	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("http://localhost:%d/v1/models", port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gateway connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway returned status %d", resp.StatusCode)
	}
	return nil
}
