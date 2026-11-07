package models

import (
	"fmt"
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
}

type passthroughEntry struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

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
		out = append(out, Passthrough{name, e.Provider, strings.TrimSpace(e.Model)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
