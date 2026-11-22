package clientconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/jeefy/convergence/internal/models"
)

type Format int

const (
	JSON Format = iota
	YAML
)

const Key = "convergence"

// Options control a configure/restore run. GatewayURL is the gateway root
// (e.g. http://localhost:3000); a trailing /v1 is tolerated and removed.
type Options struct {
	GatewayURL string
	// Passthrough are the extra models the gateway serves (models.yaml
	// `passthrough:`), offered by writers that list models per provider.
	Passthrough []models.Passthrough
	SetDefault  bool
	DryRun      bool
	Force       bool
}

type setOp struct {
	path          []string
	val           any
	onlyIfMissing bool
}

type client struct {
	name    string
	format  Format
	path    func(home string) string
	ops     func(o Options, root string) []setOp
	pointed func(doc map[string]any) bool
	note    string
	// mode is the permission of a file we create (default 0644); an existing
	// file keeps its own.
	mode os.FileMode
}

func Root(u string) string {
	return strings.TrimSuffix(strings.TrimRight(u, "/"), "/v1")
}

func home() (string, error) { return os.UserHomeDir() }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// HermesHome mirrors Hermes' own lookup: $HERMES_HOME, else %LOCALAPPDATA%\hermes
// on Windows, else ~/.hermes.
func HermesHome(h string) string {
	if d := strings.TrimSpace(os.Getenv("HERMES_HOME")); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		if d := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); d != "" {
			return filepath.Join(d, "hermes")
		}
		return filepath.Join(h, "AppData", "Local", "hermes")
	}
	return filepath.Join(h, ".hermes")
}

// PaseoHome mirrors Paseo's own lookup: $PASEO_HOME, else ~/.paseo.
func PaseoHome(h string) string {
	if d := strings.TrimSpace(os.Getenv("PASEO_HOME")); d != "" {
		return d
	}
	return filepath.Join(h, ".paseo")
}

// Paseo provider ids added under agents.providers. Each Claude subscription
// account (models.yaml passthrough with auth: oauth) also gets
// PaseoClaude + "-" + paseoAccountID(api_key_env).
const (
	PaseoClaude   = Key + "-claude"
	PaseoOpenCode = Key + "-opencode"
	PaseoCopilot  = Key + "-copilot"
)

func paseoModel(id, label string, def bool) map[string]any {
	m := map[string]any{"id": id, "label": label}
	if def {
		m["isDefault"] = true
	}
	return m
}

type claudeAccount struct {
	id     string
	keyEnv string
	models []models.Passthrough
}

// splitClaudeAccounts separates subscription (auth: oauth) passthrough models,
// grouped per api_key_env, from the rest.
func splitClaudeAccounts(ps []models.Passthrough) (shared []models.Passthrough, accts []claudeAccount) {
	byEnv := map[string][]models.Passthrough{}
	for _, p := range ps {
		if p.Auth == models.AuthOAuth {
			byEnv[p.KeyEnv] = append(byEnv[p.KeyEnv], p)
		} else {
			shared = append(shared, p)
		}
	}
	envs := make([]string, 0, len(byEnv))
	for e := range byEnv {
		envs = append(envs, e)
	}
	sort.Strings(envs)
	used := map[string]bool{}
	for _, e := range envs {
		id := paseoAccountID(e)
		if used[id] {
			id = strings.ReplaceAll(strings.ToLower(e), "_", "-")
		}
		used[id] = true
		accts = append(accts, claudeAccount{id: id, keyEnv: e, models: byEnv[e]})
	}
	return shared, accts
}

// paseoAccountID shortens an env var name: CLAUDE_JEEFY_TOKEN -> jeefy.
func paseoAccountID(keyEnv string) string {
	s := strings.ToLower(keyEnv)
	for _, p := range []string{"claude_", "anthropic_"} {
		s = strings.TrimPrefix(s, p)
	}
	for _, x := range []string{"_oauth_token", "_token", "_key"} {
		s = strings.TrimSuffix(s, x)
	}
	if s == "" {
		s = strings.ToLower(keyEnv)
	}
	return strings.ReplaceAll(s, "_", "-")
}

// newestFamily returns the passthrough name whose upstream model is the newest
// of a family ("opus", "sonnet", "haiku"), or "".
func newestFamily(ps []models.Passthrough, family string) string {
	best, bestV := "", []int(nil)
	for _, p := range ps {
		if !strings.Contains(p.Model, family) {
			continue
		}
		if v := modelVersion(p.Model); best == "" || versionLess(bestV, v) {
			best, bestV = p.Name, v
		}
	}
	return best
}

// modelVersion extracts version numbers from an upstream model id, skipping
// 8-digit date stamps: claude-opus-4-5-20251101 -> [4 5].
func modelVersion(m string) []int {
	var v []int
	for _, f := range strings.FieldsFunc(m, func(r rune) bool { return r == '-' || r == '.' }) {
		if n, err := strconv.Atoi(f); err == nil && len(f) < 8 {
			v = append(v, n)
		}
	}
	return v
}

func versionLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// OMPAgentDir returns the agent directory omp uses: $PI_CODING_AGENT_DIR,
// else $HOME/<PI_CONFIG_DIR>/agent (or ~/.omp/agent by default).
func OMPAgentDir(h string) string {
	if d := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); d != "" {
		return d
	}
	cfgDir := strings.TrimSpace(os.Getenv("PI_CONFIG_DIR"))
	if cfgDir == "" {
		cfgDir = ".omp"
	}
	return filepath.Join(h, cfgDir, "agent")
}

var clients = []client{
	{
		name:   "opencode",
		format: JSON,
		path: func(h string) string {
			d := filepath.Join(h, ".config", "opencode")
			if !exists(filepath.Join(d, "opencode.json")) && exists(filepath.Join(d, "opencode.jsonc")) {
				return filepath.Join(d, "opencode.jsonc")
			}
			return filepath.Join(d, "opencode.json")
		},
		ops: func(o Options, root string) []setOp {
			ops := []setOp{
				{path: []string{"$schema"}, val: "https://opencode.ai/config.json", onlyIfMissing: true},
				{path: []string{"provider", Key}, val: map[string]any{
					"npm":     "@ai-sdk/openai-compatible",
					"name":    "Convergence",
					"options": map[string]any{"baseURL": root + "/v1", "apiKey": Key},
					"models": map[string]any{
						"auto":     map[string]any{"name": "Convergence Auto"},
						"pinball":  map[string]any{"name": "Convergence Pinball"},
						"light":    map[string]any{"name": "Convergence Light"},
						"standard": map[string]any{"name": "Convergence Standard"},
						"heavy":    map[string]any{"name": "Convergence Heavy"},
					},
				}},
			}
			if o.SetDefault {
				ops = append(ops, setOp{path: []string{"model"}, val: Key + "/auto"})
			}
			return ops
		},
		pointed: func(d map[string]any) bool { _, ok := lookup(d, "provider", Key); return ok },
		note:    "Models: convergence/auto, convergence/pinball, convergence/light, convergence/standard, convergence/heavy. Default model untouched unless --set-default.",
	},
	{
		name:   "openclaw",
		format: JSON,
		path:   func(h string) string { return filepath.Join(h, ".openclaw", "openclaw.json") },
		ops: func(o Options, root string) []setOp {
			ops := []setOp{{path: []string{"models", "providers", Key}, val: map[string]any{
				"baseUrl": root + "/v1",
				"apiKey":  Key,
				"api":     "openai-completions",
				"models": []any{
					map[string]any{"id": "auto", "name": "Convergence Auto"},
					map[string]any{"id": "light", "name": "Convergence Light"},
					map[string]any{"id": "standard", "name": "Convergence Standard"},
					map[string]any{"id": "heavy", "name": "Convergence Heavy"},
				},
			}}}
			if o.SetDefault {
				ops = append(ops, setOp{path: []string{"agents", "defaults", "model", "primary"}, val: Key + "/auto"})
			}
			return ops
		},
		pointed: func(d map[string]any) bool { _, ok := lookup(d, "models", "providers", Key); return ok },
		// shortcut: strict JSON/JSONC only, an openclaw.json using JSON5 syntax (unquoted keys, single quotes) is rejected, not rewritten.
		note: "Models: convergence/auto, convergence/light, convergence/standard, convergence/heavy. Default model untouched unless --set-default. openclaw.json must be JSON/JSONC (no unquoted keys or single quotes).",
	},
	{
		name:   "claude-code",
		format: JSON,
		path:   func(h string) string { return filepath.Join(h, ".claude", "settings.json") },
		ops: func(_ Options, root string) []setOp {
			// Claude Code appends /v1/messages itself: base URL must not end in /v1.
			return []setOp{
				{path: []string{"env", "ANTHROPIC_BASE_URL"}, val: root},
				{path: []string{"env", "ANTHROPIC_AUTH_TOKEN"}, val: Key},
			}
		},
		pointed: func(d map[string]any) bool { return lookupString(d, "env", "ANTHROPIC_AUTH_TOKEN") == Key },
		note: "Uses ANTHROPIC_AUTH_TOKEN (Authorization: Bearer). An existing ANTHROPIC_API_KEY in env is left alone. " +
			"This routes every Claude Code run through the gateway; to keep a direct path, use `convergence config paseo` instead.",
	},
	{
		name:   "aider",
		format: YAML,
		path:   func(h string) string { return filepath.Join(h, ".aider.conf.yml") },
		ops: func(_ Options, root string) []setOp {
			return []setOp{
				{path: []string{"openai-api-base"}, val: root + "/v1"},
				{path: []string{"openai-api-key"}, val: Key},
				{path: []string{"model"}, val: "openai/auto"},
			}
		},
		pointed: func(d map[string]any) bool { return lookupString(d, "openai-api-key") == Key },
		note:    "Configures openai-api-base and model openai/auto. Note: export OPENAI_API_BASE=" + Key + " if using LiteLLM env mode.",
	},
	{
		name:   "goose",
		format: YAML,
		path:   func(h string) string { return filepath.Join(h, ".config", "goose", "config.yaml") },
		ops: func(_ Options, root string) []setOp {
			return []setOp{
				{path: []string{"GOOSE_PROVIDER"}, val: "openai"},
				{path: []string{"GOOSE_MODEL"}, val: "auto"},
				{path: []string{"OPENAI_HOST"}, val: root},
				{path: []string{"OPENAI_BASE_PATH"}, val: "v1/chat/completions"},
			}
		},
		pointed: func(d map[string]any) bool {
			return lookupString(d, "OPENAI_BASE_PATH") != "" && lookupString(d, "OPENAI_HOST") != ""
		},
		note: "Goose does not read API keys from config.yaml: export OPENAI_API_KEY=convergence (or store it via `goose configure`).",
	},
	{
		name:   "hermes",
		format: YAML,
		path:   func(h string) string { return filepath.Join(HermesHome(h), "config.yaml") },
		ops: func(o Options, root string) []setOp {
			ops := []setOp{{path: []string{"providers", Key}, val: map[string]any{
				"name":     "Convergence",
				"base_url": root + "/v1",
				"api_key":  Key,
				"models": map[string]any{
					"auto":     map[string]any{},
					"pinball":  map[string]any{},
					"light":    map[string]any{},
					"standard": map[string]any{},
					"heavy":    map[string]any{},
				},
			}}}
			if o.SetDefault {
				// model.base_url is set too: a stale one from another provider would win.
				ops = append(ops,
					setOp{path: []string{"model", "provider"}, val: Key},
					setOp{path: []string{"model", "default"}, val: "auto"},
					setOp{path: []string{"model", "base_url"}, val: root + "/v1"})
			}
			return ops
		},
		pointed: func(d map[string]any) bool { _, ok := lookup(d, "providers", Key); return ok },
		note:    "Select it with `/model convergence:auto`; model.default/provider are untouched unless --set-default.",
	},
	{
		name:   "continue",
		format: YAML,
		path:   func(h string) string { return filepath.Join(h, ".continue", "config.yaml") },
		ops: func(_ Options, root string) []setOp {
			return []setOp{
				{path: []string{"name"}, val: "Local Config", onlyIfMissing: true},
				{path: []string{"version"}, val: "1.0.0", onlyIfMissing: true},
				{path: []string{"schema"}, val: "v1", onlyIfMissing: true},
				{path: []string{"models", itemPrefix + "Convergence (Auto)"}, val: map[string]any{
					"name": "Convergence (Auto)", "provider": "openai", "model": "auto",
					"apiBase": root + "/v1", "apiKey": Key,
				}},
				{path: []string{"models", itemPrefix + "Convergence (Light)"}, val: map[string]any{
					"name": "Convergence (Light)", "provider": "openai", "model": "light",
					"apiBase": root + "/v1", "apiKey": Key,
				}},
				{path: []string{"models", itemPrefix + "Convergence (Standard)"}, val: map[string]any{
					"name": "Convergence (Standard)", "provider": "openai", "model": "standard",
					"apiBase": root + "/v1", "apiKey": Key,
				}},
				{path: []string{"models", itemPrefix + "Convergence (Heavy)"}, val: map[string]any{
					"name": "Convergence (Heavy)", "provider": "openai", "model": "heavy",
					"apiBase": root + "/v1", "apiKey": Key,
				}},
			}
		},
		pointed: func(d map[string]any) bool {
			_, ok := lookup(d, "models", itemPrefix+"Convergence (Auto)")
			return ok
		},
		note: "Targets config.yaml (config.json is deprecated upstream).",
	},
	{
		name:   "zed",
		format: JSON,
		path: func(h string) string {
			return filepath.Join(h, ".config", "zed", "settings.json")
		},
		ops: func(_ Options, root string) []setOp {
			return []setOp{
				{path: []string{"language_models", "openai", "api_url"}, val: root + "/v1"},
				{path: []string{"language_models", "openai", "available_models"}, val: []any{
					map[string]any{"name": "auto", "display_name": "Convergence Auto", "max_tokens": 128000},
					map[string]any{"name": "light", "display_name": "Convergence Light", "max_tokens": 128000},
					map[string]any{"name": "standard", "display_name": "Convergence Standard", "max_tokens": 128000},
					map[string]any{"name": "heavy", "display_name": "Convergence Heavy", "max_tokens": 128000},
				}},
			}
		},
		pointed: func(d map[string]any) bool {
			u := lookupString(d, "language_models", "openai", "api_url")
			return strings.Contains(u, "3000") || strings.Contains(u, "/v1")
		},
		note: "Configures language_models.openai.api_url in ~/.config/zed/settings.json. Set OPENAI_API_KEY=convergence in your environment.",
	},
	{
		name:   "cline",
		format: JSON,
		path: func(h string) string {
			return filepath.Join(h, ".config", "Code", "User", "settings.json")
		},
		ops: func(_ Options, root string) []setOp {
			return []setOp{
				{path: []string{"cline.apiProvider"}, val: "openai-compatible"},
				{path: []string{"cline.openAiBaseUrl"}, val: root + "/v1"},
				{path: []string{"cline.openAiApiKey"}, val: Key},
				{path: []string{"cline.openAiModelId"}, val: "auto"},
			}
		},
		pointed: func(d map[string]any) bool {
			return lookupString(d, "cline.openAiApiKey") == Key
		},
		note: "Configures VS Code User settings for Cline / Roo Code with provider openai-compatible and model auto.",
	},
	{
		name:   "paseo",
		format: JSON,
		mode:   0o600, // config.json also holds the daemon password hash and provider env
		path:   func(h string) string { return filepath.Join(PaseoHome(h), "config.json") },
		ops: func(o Options, root string) []setOp {
			shared, accts := splitClaudeAccounts(o.Passthrough)
			tiers := func() []any {
				return []any{
					paseoModel("auto", "Convergence Auto", true),
					paseoModel("light", "Convergence Light", false),
					paseoModel("standard", "Convergence Standard", false),
					paseoModel("heavy", "Convergence Heavy", false),
				}
			}
			claude := tiers()
			// The Copilot CLI reserves "auto" for its own router and rejects it under BYOK.
			copilot := []any{
				paseoModel("standard", "Convergence Standard", true),
				paseoModel("light", "Convergence Light", false),
				paseoModel("heavy", "Convergence Heavy", false),
			}
			for _, p := range shared {
				claude = append(claude, paseoModel(p.Name, p.Name+" (passthrough)", false))
				copilot = append(copilot, paseoModel(p.Name, p.Name+" (passthrough)", false))
			}
			ops := []setOp{
				{path: []string{"agents", "providers", PaseoClaude}, val: map[string]any{
					"extends": "claude",
					"label":   "Convergence (Claude Code)",
					// Claude Code appends /v1/messages itself: base URL must not end in /v1.
					"env": map[string]any{
						"ANTHROPIC_BASE_URL":             root,
						"ANTHROPIC_AUTH_TOKEN":           Key,
						"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "light",
						"ANTHROPIC_DEFAULT_SONNET_MODEL": "standard",
						"ANTHROPIC_DEFAULT_OPUS_MODEL":   "heavy",
					},
					// Anthropic's server-side tools are not available through the gateway.
					"disallowedTools": []any{"WebSearch"},
					"models":          claude,
				}},
			}
			for _, a := range accts {
				env := map[string]any{"ANTHROPIC_BASE_URL": root, "ANTHROPIC_AUTH_TOKEN": Key}
				def := ""
				for _, f := range []struct{ family, envKey string }{
					{"opus", "ANTHROPIC_DEFAULT_OPUS_MODEL"},
					{"sonnet", "ANTHROPIC_DEFAULT_SONNET_MODEL"},
					{"haiku", "ANTHROPIC_DEFAULT_HAIKU_MODEL"},
				} {
					if n := newestFamily(a.models, f.family); n != "" {
						env[f.envKey] = n
						if def == "" {
							def = n
						}
					}
				}
				if def == "" {
					def = a.models[0].Name
				}
				var ms []any
				for _, p := range a.models {
					ms = append(ms, paseoModel(p.Name, p.Name, p.Name == def))
				}
				ops = append(ops, setOp{path: []string{"agents", "providers", PaseoClaude + "-" + a.id}, val: map[string]any{
					"extends":         "claude",
					"label":           "Convergence (Claude Code, " + a.id + ")",
					"env":             env,
					"disallowedTools": []any{"WebSearch"},
					"models":          ms,
				}})
			}
			return append(ops,
				setOp{path: []string{"agents", "providers", PaseoCopilot}, val: map[string]any{
					"extends": "copilot",
					"label":   "Convergence (Copilot)",
					// Copilot CLI BYOK (`copilot help environment`): OpenAI chat completions under /v1.
					"env": map[string]any{
						"COPILOT_PROVIDER_BASE_URL": root + "/v1",
						"COPILOT_PROVIDER_API_KEY":  Key,
						"COPILOT_MODEL":             "standard",
					},
					"models": copilot,
				}},
				setOp{path: []string{"agents", "providers", PaseoOpenCode}, val: map[string]any{
					"extends": "opencode",
					"label":   "Convergence (OpenCode)",
					"models": []any{
						paseoModel(Key+"/auto", "Convergence Auto", true),
						paseoModel(Key+"/pinball", "Convergence Pinball", false),
						paseoModel(Key+"/light", "Convergence Light", false),
						paseoModel(Key+"/standard", "Convergence Standard", false),
						paseoModel(Key+"/heavy", "Convergence Heavy", false),
					},
				}},
			)
		},
		pointed: func(d map[string]any) bool { _, ok := lookup(d, "agents", "providers", PaseoClaude); return ok },
		note: "Adds providers " + PaseoClaude + ", " + PaseoCopilot + ", " + PaseoOpenCode + " and one " + PaseoClaude + "-<account> " +
			"per Claude subscription account (passthrough auth: oauth); the built-in claude and copilot providers are untouched (direct). " +
			"Run `paseo reload` to apply. " + PaseoOpenCode + " needs `convergence config opencode` as well.",
	},
	{
		name:   "omp",
		format: YAML,
		path: func(h string) string {
			d := OMPAgentDir(h)
			canonical := filepath.Join(d, "models.yml")
			fallback := filepath.Join(d, "models.yaml")
			if !exists(canonical) && exists(fallback) {
				return fallback
			}
			return canonical
		},
		ops: func(_ Options, root string) []setOp {
			return []setOp{{
				path: []string{"providers", Key},
				val: map[string]any{
					"baseUrl": root + "/v1",
					"apiKey":  Key,
					"api":     "openai-completions",
					"models": []any{
						map[string]any{"id": "auto", "name": "Convergence Auto", "contextWindow": 128000, "maxTokens": 16384, "reasoning": false, "input": []string{"text"}},
						map[string]any{"id": "pinball", "name": "Convergence Pinball", "contextWindow": 128000, "maxTokens": 16384, "reasoning": false, "input": []string{"text"}},
						map[string]any{"id": "light", "name": "Convergence Light", "contextWindow": 128000, "maxTokens": 16384, "reasoning": false, "input": []string{"text"}},
						map[string]any{"id": "standard", "name": "Convergence Standard", "contextWindow": 128000, "maxTokens": 16384, "reasoning": false, "input": []string{"text"}},
						map[string]any{"id": "heavy", "name": "Convergence Heavy", "contextWindow": 128000, "maxTokens": 16384, "reasoning": false, "input": []string{"text"}},
					},
				},
			}}
		},
		pointed: func(d map[string]any) bool {
			_, ok := lookup(d, "providers", Key)
			return ok
		},
		note: "Models: convergence/auto, convergence/pinball, convergence/light, convergence/standard, convergence/heavy. Use `--model convergence/auto` or pick one with `/model`.",
	},
}

func init() {
	oc := clients[0]
	oc.name = "oh-my-opencode"
	clients = append(clients, oc)
}

func Names() []string {
	n := make([]string, len(clients))
	for i, c := range clients {
		n[i] = c.name
	}
	return n
}

func find(name string) (*client, string, error) {
	h, err := home()
	if err != nil {
		return nil, "", err
	}
	for i := range clients {
		if clients[i].name == name {
			return &clients[i], clients[i].path(h), nil
		}
	}
	return nil, "", fmt.Errorf("unknown client %q (supported: %s)", name, strings.Join(Names(), ", "))
}

// EnvExports prints shell exports for tools without a config file we can safely edit.
func EnvExports(gatewayURL string) string {
	r := Root(gatewayURL)
	return "# Add to your shell profile:\n" +
		"export OPENAI_BASE_URL=\"" + r + "/v1\"\n" +
		"export OPENAI_API_KEY=\"" + Key + "\"\n" +
		"export ANTHROPIC_BASE_URL=\"" + r + "\"\n" +
		"export ANTHROPIC_AUTH_TOKEN=\"" + Key + "\"\n"
}
