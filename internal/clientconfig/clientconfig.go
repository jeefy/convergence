package clientconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ClientType string

const (
	ClientOpenCode   ClientType = "opencode"
	ClientOhMyOpen   ClientType = "oh-my-opencode"
	ClientCopilot    ClientType = "copilot"
	ClientClaudeCode ClientType = "claude-code"
	ClientAider      ClientType = "aider"
	ClientHermes     ClientType = "hermes"
	ClientGoose      ClientType = "goose"
	ClientCursor     ClientType = "cursor"
	ClientContinue   ClientType = "continue"
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

func GetClaudeCodeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func GetAiderConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".aider.conf.yml"), nil
}

func GetHermesConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hermes", "config.yaml"), nil
}

func GetGooseConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "goose", "config.yaml"), nil
}

func GetCursorSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	macPath := filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json")
	if _, err := os.Stat(filepath.Dir(macPath)); err == nil {
		return macPath, nil
	}
	return filepath.Join(home, ".config", "Cursor", "User", "settings.json"), nil
}

func GetContinueConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".continue", "config.json"), nil
}

func restoreFile(path string) error {
	backupPath := path + ".convergence.bak"
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("no backup found at %s to restore from", backupPath)
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("reading backup: %w", err)
	}
	// If the backup file is empty (meaning the file originally did not exist), delete the config file
	if len(data) == 0 {
		os.Remove(path)
	} else {
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("restoring config: %w", err)
		}
	}
	os.Remove(backupPath)
	return nil
}

func backupFile(path string) error {
	backupPath := path + ".convergence.bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Record empty backup indicating the file didn't originally exist
				return os.WriteFile(backupPath, []byte{}, 0644)
			}
			return err
		}
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func ConfigureOpenCode(gatewayURL string, restore bool) error {
	path, err := GetOpenCodeConfigPath()
	if err != nil {
		return err
	}

	if restore {
		return restoreFile(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	if err := backupFile(path); err != nil {
		return err
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing opencode.json: %w", err)
	}

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

	if restore {
		return restoreFile(path)
	}

	if err := backupFile(path); err != nil {
		return err
	}

	pinballProfile := filepath.Join(filepath.Dir(path), "omo.pinball.jsonc")
	if pinData, err := os.ReadFile(pinballProfile); err == nil {
		return os.WriteFile(path, pinData, 0644)
	}

	return nil
}

func ConfigureClaudeCode(gatewayURL string, restore bool) error {
	path, err := GetClaudeCodeSettingsPath()
	if err != nil {
		return err
	}

	if restore {
		return restoreFile(path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := backupFile(path); err != nil {
		return err
	}

	var root map[string]interface{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			root = make(map[string]interface{})
		}
	} else {
		root = make(map[string]interface{})
	}

	env, ok := root["env"].(map[string]interface{})
	if !ok {
		env = make(map[string]interface{})
		root["env"] = env
	}

	env["ANTHROPIC_BASE_URL"] = gatewayURL
	env["ANTHROPIC_API_KEY"] = "convergence"

	outData, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(outData, '\n'), 0644)
}

func ConfigureAider(gatewayURL string, restore bool) error {
	path, err := GetAiderConfigPath()
	if err != nil {
		return err
	}

	if restore {
		return restoreFile(path)
	}

	if err := backupFile(path); err != nil {
		return err
	}

	root := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		yaml.Unmarshal(data, &root)
	}

	root["openai-api-base"] = gatewayURL
	root["openai-api-key"] = "convergence"
	root["model"] = "auto"

	outData, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	return os.WriteFile(path, outData, 0644)
}

func ConfigureHermes(gatewayURL string, restore bool) error {
	path, err := GetHermesConfigPath()
	if err != nil {
		return err
	}

	if restore {
		return restoreFile(path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := backupFile(path); err != nil {
		return err
	}

	root := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		yaml.Unmarshal(data, &root)
	}

	root["endpoint"] = gatewayURL
	root["api_key"] = "convergence"
	root["model"] = "auto"

	outData, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	return os.WriteFile(path, outData, 0644)
}

func ConfigureGoose(gatewayURL string, restore bool) error {
	path, err := GetGooseConfigPath()
	if err != nil {
		return err
	}

	if restore {
		return restoreFile(path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := backupFile(path); err != nil {
		return err
	}

	root := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		yaml.Unmarshal(data, &root)
	}

	root["GOOSE_PROVIDER"] = "openai"
	root["GOOSE_MODEL"] = "auto"
	root["OPENAI_BASE_URL"] = gatewayURL
	root["OPENAI_API_KEY"] = "convergence"

	outData, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	return os.WriteFile(path, outData, 0644)
}

func ConfigureCursor(gatewayURL string, restore bool) error {
	path, err := GetCursorSettingsPath()
	if err != nil {
		return err
	}

	if restore {
		return restoreFile(path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := backupFile(path); err != nil {
		return err
	}

	root := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		json.Unmarshal(data, &root)
	}

	root["cursor.models.openAI.baseUrl"] = gatewayURL
	root["cursor.models.openAI.apiKey"] = "convergence"

	outData, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(outData, '\n'), 0644)
}

func ConfigureContinue(gatewayURL string, restore bool) error {
	path, err := GetContinueConfigPath()
	if err != nil {
		return err
	}

	if restore {
		return restoreFile(path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err := backupFile(path); err != nil {
		return err
	}

	root := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		json.Unmarshal(data, &root)
	}

	models, ok := root["models"].([]interface{})
	if !ok {
		models = []interface{}{}
	}

	convergenceModel := map[string]interface{}{
		"title":    "Convergence (Auto)",
		"provider": "openai",
		"model":    "auto",
		"apiBase":  gatewayURL,
		"apiKey":   "convergence",
	}

	found := false
	for i, m := range models {
		if mObj, ok := m.(map[string]interface{}); ok {
			if mObj["title"] == "Convergence (Auto)" {
				models[i] = convergenceModel
				found = true
				break
			}
		}
	}
	if !found {
		models = append([]interface{}{convergenceModel}, models...)
	}

	root["models"] = models

	outData, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(outData, '\n'), 0644)
}

func PrintCopilotEnv(gatewayURL string) string {
	var sb strings.Builder
	sb.WriteString("# Run these exports or add to your shell profile (~/.zshrc, ~/.bashrc):\n")
	sb.WriteString(fmt.Sprintf("export OPENAI_BASE_URL=\"%s\"\n", gatewayURL))
	sb.WriteString("export OPENAI_API_KEY=\"convergence\"\n")
	sb.WriteString(fmt.Sprintf("export ANTHROPIC_BASE_URL=\"%s\"\n", gatewayURL))
	sb.WriteString("export ANTHROPIC_API_KEY=\"convergence\"\n")
	sb.WriteString("export GITHUB_COPILOT_API_URL=\"http://localhost:4141/v1\"\n")
	return sb.String()
}
