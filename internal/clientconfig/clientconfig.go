package clientconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ClientType string

const (
	ClientOpenCode   ClientType = "opencode"
	ClientOhMyOpen   ClientType = "oh-my-opencode"
	ClientCopilot    ClientType = "copilot"
)

func GetOpenCodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
}

func GetOmoConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".omo", "omo.jsonc"), nil
}

func ConfigureOpenCode(gatewayURL string, restore bool) error {
	path, err := GetOpenCodeConfigPath()
	if err != nil {
		return err
	}

	backupPath := path + ".convergence.bak"

	if restore {
		if _, err := os.Stat(backupPath); err != nil {
			return fmt.Errorf("no backup found at %s to restore from", backupPath)
		}
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("reading backup: %w", err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("restoring config: %w", err)
		}
		os.Remove(backupPath)
		return nil
	}

	// Read existing config
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	// Create backup if not already present
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("writing backup: %w", err)
		}
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing opencode.json: %w", err)
	}

	// Update provider.gateway.options.baseURL
	provider, ok := root["provider"].(map[string]interface{})
	if !ok {
		provider = make(map[string]interface{})
		root["provider"] = provider
	}

	gw, ok := provider["gateway"].(map[string]interface{})
	if !ok {
		gw = make(map[string]interface{})
		provider["gateway"] = gw
	}

	options, ok := gw["options"].(map[string]interface{})
	if !ok {
		options = make(map[string]interface{})
		gw["options"] = options
	}

	options["baseURL"] = gatewayURL

	outData, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing updated config: %w", err)
	}

	return os.WriteFile(path, append(outData, '\n'), 0644)
}

func ConfigureOmo(restore bool) error {
	path, err := GetOmoConfigPath()
	if err != nil {
		return err
	}

	backupPath := path + ".convergence.bak"
	if restore {
		if _, err := os.Stat(backupPath); err != nil {
			return fmt.Errorf("no backup found at %s to restore from", backupPath)
		}
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("reading backup: %w", err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("restoring config: %w", err)
		}
		os.Remove(backupPath)
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("writing backup: %w", err)
		}
	}

	// omo.jsonc is JSON with comments; for convergence, ensure pinball models are targeted
	pinballProfile := filepath.Join(filepath.Dir(path), "omo.pinball.jsonc")
	if pinData, err := os.ReadFile(pinballProfile); err == nil {
		return os.WriteFile(path, pinData, 0644)
	}

	return nil
}

func PrintCopilotEnv(gatewayURL string) string {
	var sb strings.Builder
	sb.WriteString("# Run these exports or add to your shell profile (~/.zshrc, ~/.bashrc):\n")
	sb.WriteString(fmt.Sprintf("export OPENAI_BASE_URL=\"%s\"\n", gatewayURL))
	sb.WriteString("export OPENAI_API_KEY=\"convergence\"\n")
	sb.WriteString("export GITHUB_COPILOT_API_URL=\"http://localhost:4141/v1\"\n")
	return sb.String()
}
