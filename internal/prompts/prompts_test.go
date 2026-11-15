package prompts

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func boolp(b bool) *bool { return &b }

func TestDefaultsAreSafe(t *testing.T) {
	for i, l := range Defaults {
		if strings.Contains(l, "$") {
			t.Errorf("Defaults[%d] contains '$' (agentgateway expands $VAR): %q", i, l)
		}
		if strings.Contains(l, "\n") || strings.TrimSpace(l) != l {
			t.Errorf("Defaults[%d] must be a single trimmed line: %q", i, l)
		}
	}
}

func TestContent(t *testing.T) {
	if got := (Settings{}).Content(); got != "" {
		t.Fatalf("disabled must render nothing, got %q", got)
	}
	if got := (Settings{Enabled: true}).Content(); got != strings.Join(Defaults, "\n") {
		t.Fatalf("enabled without extra must be the defaults")
	}
	s := Settings{Enabled: true, Defaults: boolp(false), Extra: "  a  \n\n b \n"}
	if got := s.Content(); got != "a\nb" {
		t.Fatalf("extra-only content = %q", got)
	}
	s = Settings{Enabled: true, Extra: "custom rule"}
	if got := s.Content(); !strings.HasSuffix(got, "\ncustom rule") || !strings.HasPrefix(got, Defaults[0]) {
		t.Fatalf("extra must follow defaults: %q", got)
	}
}

func TestValidate(t *testing.T) {
	if err := (Settings{Enabled: true}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Settings{Enabled: true, Extra: "use $HOME"}).Validate(); err == nil {
		t.Fatal("expected error for '$'")
	}
	// Disabled text is never rendered, so it is not validated.
	if err := (Settings{Extra: "$X"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

// The transformation must be valid YAML and the CEL string literal must decode
// (CEL and JSON share the escape grammar) back to the exact content, including
// characters that would break naive quoting.
func TestYAMLTransformationRoundTrip(t *testing.T) {
	extra := "Quote \"this\": and 'that' # not a comment\n- not a list item\nback\\slash\tand tab"
	s := Settings{Enabled: true, Extra: extra}
	doc := "models:\n  - name: m\n" + s.YAMLTransformation(4)
	var out struct {
		Models []struct {
			Transformation map[string]string
		}
	}
	if err := yaml.Unmarshal([]byte(doc), &out); err != nil {
		t.Fatalf("yaml: %v\n%s", err, doc)
	}
	tr := out.Models[0].Transformation
	for _, k := range []string{"messages", "system", "instructions"} {
		expr, ok := tr[k]
		if !ok {
			t.Fatalf("missing transformation %q", k)
		}
		if !strings.Contains(expr, s.CELString()) || !strings.Contains(expr, "llmRequest."+k) {
			t.Errorf("%s expression must embed the prompt literal and fall back to llmRequest.%s:\n%s", k, k, expr)
		}
	}
	var decoded string
	if err := json.Unmarshal([]byte(s.CELString()), &decoded); err != nil {
		t.Fatalf("CEL literal is not a valid JSON string: %v\n%s", err, s.CELString())
	}
	if decoded != s.Content() {
		t.Fatalf("round trip mismatch\n got: %q\nwant: %q", decoded, s.Content())
	}
	if (Settings{}).YAMLTransformation(4) != "" {
		t.Fatal("disabled must render nothing")
	}
}

// Text is rendered on one YAML line: control characters inside a line would
// corrupt the config, while CRLF line endings are normal input and must work.
func TestValidateControlCharacters(t *testing.T) {
	for _, bad := range []string{"a\x00b", "a\x0bb", "a\u2028b", "a\u0085b"} {
		if err := (Settings{Enabled: true, Extra: bad}).Validate(); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	crlf := Settings{Enabled: true, Defaults: new(bool), Extra: "one\r\ntwo\tcol\r\n"}
	if err := crlf.Validate(); err != nil {
		t.Fatalf("CRLF input must be accepted: %v", err)
	}
	if got := crlf.Content(); got != "one\ntwo\tcol" {
		t.Errorf("CRLF not normalized: %q", got)
	}
}

// A field that is present but null must not make the expression fail:
// agentgateway drops a key whose expression errors, which would skip the prompt.
func TestTransformationGuardsNull(t *testing.T) {
	out := Settings{Enabled: true}.YAMLTransformation(0)
	for _, want := range []string{
		"llmRequest.system != null", "llmRequest.instructions != null",
		// an empty system string must not become an empty text block (Anthropic rejects those)
		`llmRequest.system == "" ? []`,
		// the optimizer's judge is exempt on every endpoint
		`default(request.headers["x-agw-caller"], "") != "eval-replay-judge" && request.path.startsWith("/v1/chat/")`,
		`default(request.headers["x-agw-caller"], "") != "eval-replay-judge" && request.path.startsWith("/v1/messages")`,
		`default(request.headers["x-agw-caller"], "") != "eval-replay-judge" && request.path.startsWith("/v1/responses")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing guard %q", want)
		}
	}
}

// Text that Go accepts but a YAML parser rejects must fail at init, not when
// the gateway starts: invalid UTF-8 (a cp1252 prompt file) and noncharacters.
func TestValidateEncoding(t *testing.T) {
	for _, bad := range []string{"it\x92s cp1252", "a\ufffeb", "a\uffffb", "mid\ufeffline"} {
		if err := (Settings{Enabled: true, Extra: bad}).Validate(); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	ok := Settings{Enabled: true, Defaults: new(bool), Extra: "\ufeffcaf\u00e9 \U0001F600 \u00a0x"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("BOM-prefixed UTF-8 with non-ASCII must be accepted: %v", err)
	}
	if got := ok.Content(); got != "caf\u00e9 \U0001F600 \u00a0x" {
		t.Errorf("leading BOM must be stripped, got %q", got)
	}
}
