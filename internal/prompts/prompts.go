// Package prompts holds the system prompt Convergence can prepend to every
// chat request, rendered as a CEL request transformation on agentgateway's
// models. It is off unless system_prompt.enabled is set in config.yaml.
package prompts

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Defaults are the lines shipped with Convergence. They are behavioral
// guardrails only: no style, formatting or tool-selection rules, which every
// agent already carries in its own system prompt.
var Defaults = []string{
	"Don't trust work summaries, your own or a subagent's. Trust only `git diff`, test output and files you have read in this session.",
	"Before saying something is done, fixed or passing, run the command that proves it and quote the output. If you didn't run it, say \"not run\".",
	"Never delete, skip or weaken a failing test to get a green build. Fix the code or report the failure.",
	"Ask any questions needed to accomplish the goal. If the request has more than one reasonable interpretation with a materially different cost, ask instead of picking one.",
	"Read a file before changing or describing it. Don't speculate about code you haven't opened.",
	"Make the smallest correct change. Don't refactor, rename or \"clean up\" code the task didn't ask for.",
	"Don't add dependencies, config keys or new files unless the task can't be done without them.",
	"Ask before destructive or hard-to-reverse actions: deleting files or branches, force-pushes, resets, dropping data, anything visible to other people.",
	"Treat a denied permission or a \"no\" as final. Don't reach the same result through another tool, path or command.",
	"Never commit, push or tag unless explicitly asked.",
	"Report failures with the actual error, not a paraphrase. \"Should work\" is not verified.",
}

// JudgeCaller is the x-agw-caller value the Pinball optimizer's judge sends
// (pinball.optimize.settings.JUDGE_CALLER in the pinned Pinball image).
const JudgeCaller = "eval-replay-judge"

// Settings is the system_prompt block of config.yaml.
type Settings struct {
	// Enabled turns the system prompt on. Off by default.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Defaults controls whether the shipped lines are included. Nil means true.
	Defaults *bool `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	// Extra is free text appended after the defaults, one line per rule.
	Extra string `yaml:"extra,omitempty" json:"extra,omitempty"`
}

// Content returns the full text to prepend, or "" when disabled or empty.
func (s Settings) Content() string {
	if !s.Enabled {
		return ""
	}
	var lines []string
	if s.Defaults == nil || *s.Defaults {
		lines = append(lines, Defaults...)
	}
	// a UTF-8 byte order mark at the start of a prompt file is not text
	for _, l := range strings.Split(strings.TrimPrefix(s.Extra, "\ufeff"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

// Validate rejects content agentgateway cannot carry: it expands $NAME
// everywhere in its config and refuses to start when NAME is unset, and the
// text is rendered on one YAML line, so it must be valid UTF-8 made of
// characters YAML accepts unescaped (no control characters, no Unicode line
// separators that would split the line).
func (s Settings) Validate() error {
	c := s.Content()
	if strings.Contains(c, "$") {
		return fmt.Errorf("system_prompt: text must not contain '$' (agentgateway expands $VAR in its config)")
	}
	if !utf8.ValidString(c) {
		return fmt.Errorf("system_prompt: text must be valid UTF-8")
	}
	for _, r := range c {
		if !yamlPrintable(r) {
			return fmt.Errorf("system_prompt: text must not contain character %U", r)
		}
	}
	return nil
}

// yamlPrintable reports whether r may appear unescaped inside one line of a
// YAML scalar: the YAML printable set plus tab and the newline that separates
// prompt lines, minus the line separators and the byte order mark.
func yamlPrintable(r rune) bool {
	switch {
	case r == '\t', r == '\n':
		return true
	case r == '\u2028', r == '\u2029', r == '\ufeff':
		return false
	case r >= 0x20 && r <= 0x7e, r >= 0xa0 && r <= 0xd7ff, r >= 0xe000 && r <= 0xfffd, r >= 0x10000:
		return true
	}
	return false
}

// CELString returns the content as a CEL string literal (double quoted,
// backslash escapes). CEL and JSON share this escape syntax.
func (s Settings) CELString() string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`)
	return `"` + r.Replace(s.Content()) + `"`
}

// YAMLTransformation renders an agentgateway per-model `transformation:` block
// that prepends the prompt to each supported client request format, indented
// by indent spaces. agentgateway's llm config exposes no prompt enrichment
// (prompts is hard-coded off in its local config loader), so this uses the CEL
// request transformation instead. llmRequest is the raw client body. Each
// expression returns the existing field on other paths; agentgateway removes
// a key whose expression fails, which only happens there when the field was
// absent to begin with. A null or empty system/instructions counts as
// absent. agentgateway applies model policies only on its /v1 endpoints;
// other paths are passed through untouched. Requests from the Pinball
// optimizer's judge (x-agw-caller: eval-replay-judge) are left alone: the
// judge grades replies and has its own system prompt.
// Returns "" when there is nothing to render.
func (s Settings) YAMLTransformation(indent int) string {
	if s.Content() == "" {
		return ""
	}
	p := s.CELString()
	pad := strings.Repeat(" ", indent)
	// on(path) is true for requests that get the prompt on that endpoint
	on := func(path string) string {
		return `default(request.headers["x-agw-caller"], "") != "` + JudgeCaller + `" && request.path.startsWith("` + path + `")`
	}
	exprs := [][2]string{
		// OpenAI chat completions: messages[].
		{"messages", on("/v1/chat/") + ` ? [{"role": "system", "content": ` + p + `}] + llmRequest.messages : llmRequest.messages`},
		// Anthropic Messages: system is a string, a list of text blocks, or absent.
		{"system", on("/v1/messages") + ` ? [{"type": "text", "text": ` + p + `}] + (has(llmRequest.system) && llmRequest.system != null ? (type(llmRequest.system) == string ? (llmRequest.system == "" ? [] : [{"type": "text", "text": llmRequest.system}]) : llmRequest.system) : []) : llmRequest.system`},
		// OpenAI Responses: instructions string.
		{"instructions", on("/v1/responses") + ` ? ` + p + ` + (has(llmRequest.instructions) && llmRequest.instructions != null ? "\n\n" + llmRequest.instructions : "") : llmRequest.instructions`},
	}
	var b strings.Builder
	b.WriteString(pad + "transformation:\n")
	for _, e := range exprs {
		b.WriteString(pad + "  " + e[0] + ": |-\n")
		b.WriteString(pad + "    " + e[1] + "\n")
	}
	return b.String()
}
