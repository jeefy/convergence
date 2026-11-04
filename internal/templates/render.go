package templates

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/jeefy/convergence/internal/config"
)

//go:embed assets/*
var assetsFS embed.FS

type RenderContext struct {
	Config   *config.Config
	DataDir  string
	IsDocker bool
	IsPodman bool
}

func RenderAssets(cfg *config.Config, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return fmt.Errorf("creating target dir: %w", err)
	}

	ctx := RenderContext{
		Config:   cfg,
		DataDir:  targetDir,
		IsDocker: cfg.Engine == "docker",
		IsPodman: cfg.Engine == "podman",
	}

	files := []string{
		"docker-compose.yaml",
		"agentgateway.yaml",
		"pinball.yaml",
	}

	for _, file := range files {
		tmplPath := fmt.Sprintf("assets/%s.tmpl", file)
		data, err := assetsFS.ReadFile(tmplPath)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", tmplPath, err)
		}

		tmpl, err := template.New(file).Parse(string(data))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", file, err)
		}

		destPath := filepath.Join(targetDir, file)
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("opening dest %s: %w", destPath, err)
		}

		if err := tmpl.Execute(f, ctx); err != nil {
			f.Close()
			return fmt.Errorf("executing template %s: %w", file, err)
		}
		f.Close()
	}

	// Create log directory for access.jsonl
	logDir := filepath.Join(targetDir, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("creating logs dir: %w", err)
	}

	return nil
}
