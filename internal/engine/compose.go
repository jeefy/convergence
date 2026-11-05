package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jeefy/convergence/internal/config"
)

// ErrNoEngine is returned when neither docker nor podman is installed.
var ErrNoEngine = errors.New("no container engine found: install Docker (https://docs.docker.com/get-docker/) or Podman (https://podman.io/docs/installation) and make sure it is on your PATH")

type ComposeEngine struct {
	binary string
}

// DetectEngine prefers the configured engine, then docker, then podman.
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
	return nil, ErrNoEngine
}

func (e *ComposeEngine) Name() string {
	return filepath.Base(e.binary)
}

func (e *ComposeEngine) composeCmd(ctx context.Context, dir string, args ...string) *exec.Cmd {
	fullArgs := append([]string{"compose", "-f", filepath.Join(dir, "docker-compose.yaml")}, args...)
	cmd := exec.CommandContext(ctx, e.binary, fullArgs...)
	cmd.Dir = dir
	return cmd
}

func (e *ComposeEngine) runCompose(ctx context.Context, dir string, args ...string) error {
	out, err := e.composeCmd(ctx, dir, args...).CombinedOutput()
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

// Down removes containers, networks and named volumes of the stack.
func (e *ComposeEngine) Down(ctx context.Context, dir string) error {
	return e.runCompose(ctx, dir, "down", "-v", "--remove-orphans")
}

// Pull fetches the images referenced by the compose file, streaming progress.
func (e *ComposeEngine) Pull(ctx context.Context, dir string) error {
	cmd := e.composeCmd(ctx, dir, "pull")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s compose pull failed: %w", e.Name(), err)
	}
	return nil
}

// Logs streams service logs to the terminal.
func (e *ComposeEngine) Logs(ctx context.Context, dir string, follow bool, services ...string) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	cmd := e.composeCmd(ctx, dir, append(args, services...)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s compose logs failed: %w", e.Name(), err)
	}
	return nil
}

// RunningServices returns the names of running compose services.
func (e *ComposeEngine) RunningServices(ctx context.Context, dir string) ([]string, error) {
	out, err := e.composeCmd(ctx, dir, "ps", "--services", "--filter", "status=running").Output()
	if err != nil {
		return nil, fmt.Errorf("%s compose ps failed: %w", e.Name(), err)
	}
	return strings.Fields(string(out)), nil
}

// PortConflictError names a host port that something else already holds.
type PortConflictError struct {
	Service string
	Addr    string
	Err     error
}

func (e *PortConflictError) Error() string {
	return fmt.Sprintf("port %s (needed by %s) is already in use: %v; stop the other process or change gateway_port/bind_address in ~/.convergence/config.yaml", e.Addr, e.Service, e.Err)
}

func (e *PortConflictError) Unwrap() error { return e.Err }

// CheckPortsFree verifies each port can be bound on bindAddress.
func CheckPortsFree(bindAddress string, ports []config.NamedPort) error {
	for _, p := range ports {
		addr := net.JoinHostPort(bindAddress, strconv.Itoa(p.Port))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return &PortConflictError{Service: p.Name, Addr: addr, Err: err}
		}
		if err := ln.Close(); err != nil {
			return fmt.Errorf("releasing probe listener %s: %w", addr, err)
		}
	}
	return nil
}

func (e *ComposeEngine) CheckHealth(ctx context.Context, host string, port int) error {
	url := fmt.Sprintf("http://%s/v1/models", net.JoinHostPort(host, strconv.Itoa(port)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("gateway connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway returned status %d", resp.StatusCode)
	}
	return nil
}

// WaitHealthy polls CheckHealth until it succeeds or ctx expires.
func (e *ComposeEngine) WaitHealthy(ctx context.Context, host string, port int) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		err := e.CheckHealth(ctx, host, port)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("gateway not healthy: %w", err)
		case <-ticker.C:
		}
	}
}
