package models

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Passthrough is one entry of the `passthrough:` block of models.yaml: a fixed
// client-visible model name served by one upstream, without Pinball routing.
// (The block cannot be called `direct:`: that is the provider override key.)
type Passthrough struct {
	Name     string
	Provider string // UpstreamCopilot | UpstreamOpenAI | UpstreamAnthropic | UpstreamOllama
	Model    string
	// BaseURL/KeyEnv are optional: a custom endpoint (e.g. Ollama Cloud) and the
	// NAME of the env var holding its API key (never the key itself).
	BaseURL string
	KeyEnv  string
	// Endpoint is the agentgateway provider name of a custom endpoint ("" = shared family provider).
	Endpoint string
}

type passthroughEntry struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	KeyEnv   string `yaml:"api_key_env"`
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reservedEnv are env vars convergence itself writes to .env / the gateway.
var reservedEnv = map[string]bool{"STANDIN_SALT": true, "GH_TOKEN": true, "OPENAI_API_KEY": true, "ANTHROPIC_API_KEY": true}

// reserved are the virtual model names passthrough entries must not shadow.
var reserved = map[string]bool{"auto": true, "pinball": true, "light": true, "standard": true, "heavy": true}

// ParsePassthrough extracts the `passthrough:` block (strict). Nil when absent.
func ParsePassthrough(data []byte) ([]Passthrough, error) {
	var f struct {
		Passthrough yaml.Node `yaml:"passthrough"`
	}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing models.yaml: %w", err)
	}
	if f.Passthrough.IsZero() {
		return nil, nil
	}
	var m map[string]passthroughEntry
	b, _ := yaml.Marshal(&f.Passthrough)
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("models.yaml: passthrough: %w", err)
	}
	var out []Passthrough
	for name, e := range m {
		if reserved[name] || strings.HasPrefix(name, "pinball-") {
			return nil, fmt.Errorf("models.yaml: passthrough: name %q collides with a pinball model name", name)
		}
		switch e.Provider {
		case UpstreamCopilot, UpstreamOpenAI, UpstreamAnthropic, UpstreamOllama:
		default:
			return nil, fmt.Errorf("models.yaml: passthrough.%s: unknown provider %q (want copilot, openai, anthropic or ollama)", name, e.Provider)
		}
		if strings.TrimSpace(e.Model) == "" {
			return nil, fmt.Errorf("models.yaml: passthrough.%s: empty model", name)
		}
		p := Passthrough{Name: name, Provider: e.Provider, Model: strings.TrimSpace(e.Model), BaseURL: strings.TrimSpace(e.BaseURL), KeyEnv: strings.TrimSpace(e.KeyEnv)}
		if p.BaseURL != "" {
			if u, err := url.Parse(p.BaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return nil, fmt.Errorf("models.yaml: passthrough.%s: base_url %q must be an http(s) URL", name, p.BaseURL)
			}
		}
		if p.KeyEnv != "" && (!envName.MatchString(p.KeyEnv) || reservedEnv[p.KeyEnv]) {
			return nil, fmt.Errorf("models.yaml: passthrough.%s: api_key_env %q must be an env var NAME (not the key) and not one of STANDIN_SALT, GH_TOKEN, OPENAI_API_KEY, ANTHROPIC_API_KEY", name, p.KeyEnv)
		}
		if (p.BaseURL != "" || p.KeyEnv != "") && p.Provider == UpstreamCopilot {
			return nil, fmt.Errorf("models.yaml: passthrough.%s: base_url/api_key_env are not supported for provider copilot", name)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	// One provider per distinct (provider, base_url, api_key_env).
	seen := map[[3]string]string{}
	for i := range out {
		p := &out[i]
		if p.BaseURL == "" && p.KeyEnv == "" {
			continue
		}
		k := [3]string{p.Provider, p.BaseURL, p.KeyEnv}
		if seen[k] == "" {
			seen[k] = fmt.Sprintf("%s-ep%d", p.Provider, len(seen)+1)
		}
		p.Endpoint = seen[k]
	}
	return out, nil
}

// Endpoints returns one passthrough per distinct custom endpoint (provider block to render).
func Endpoints(ps []Passthrough) []Passthrough {
	var out []Passthrough
	seen := map[string]bool{}
	for _, p := range ps {
		if p.Endpoint != "" && !seen[p.Endpoint] {
			seen[p.Endpoint] = true
			out = append(out, p)
		}
	}
	return out
}

// KeyEnvs returns the sorted distinct api_key_env names.
func KeyEnvs(ps []Passthrough) []string {
	set := map[string]bool{}
	for _, p := range ps {
		if p.KeyEnv != "" {
			set[p.KeyEnv] = true
		}
	}
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Ref is the agentgateway provider name serving p; shared is the family provider name.
func (p Passthrough) Ref(shared string) string {
	if p.Endpoint != "" {
		return p.Endpoint
	}
	return shared
}
