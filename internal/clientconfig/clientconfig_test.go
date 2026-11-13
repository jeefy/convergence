package clientconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gw = "http://localhost:3000"

type fixture struct {
	client   string
	rel      string
	existing string
	keep     []string // substrings that must survive configuring
	want     []string // substrings expected after configuring
	edit     func(string) string
}

var fixtures = []fixture{
	{"opencode", ".config/opencode/opencode.json",
		"{\n  // my comment\n  \"model\": \"x/y\",\n  \"provider\": {\n    \"other\": {\"a\": 1,},\n  },\n}\n",
		[]string{"// my comment", `"model": "x/y"`, `"other": {"a": 1,}`},
		[]string{`"convergence"`, "http://localhost:3000/v1", "@ai-sdk/openai-compatible", `"pinball"`, `"light"`, `"heavy"`}, nil},
	{"claude-code", ".claude/settings.json",
		"{\n  // c\n  \"theme\": \"dark\",\n  \"env\": {\"FOO\": \"bar\"}\n}\n",
		[]string{"// c", `"theme": "dark"`, `"FOO": "bar"`},
		[]string{`"ANTHROPIC_BASE_URL": "http://localhost:3000"`, `"ANTHROPIC_AUTH_TOKEN": "convergence"`}, nil},
	{"aider", ".aider.conf.yml", "# keep me\ndark-mode: true\n",
		[]string{"# keep me", "dark-mode: true"},
		[]string{"openai-api-base: http://localhost:3000/v1", "model: openai/auto"}, nil},
	{"goose", ".config/goose/config.yaml", "# top\nGOOSE_MODE: auto\n",
		[]string{"# top", "GOOSE_MODE: auto"},
		[]string{"OPENAI_HOST: http://localhost:3000", "GOOSE_PROVIDER: openai"}, nil},
	{"hermes", ".hermes/config.yaml", "# h\nmodel:\n  default: foo\nproviders:\n  custom:\n    name: x\n",
		[]string{"# h", "default: foo", "name: x"},
		[]string{"convergence:", "base_url: http://localhost:3000/v1"}, nil},
	{"continue", ".continue/config.yaml",
		"name: Mine\nversion: 2.0.0\nschema: v1\n# c\nmodels:\n  - name: Other\n    provider: ollama\n    model: x\n",
		[]string{"name: Mine", "# c", "name: Other"},
		[]string{"Convergence (Auto)", "apiBase: http://localhost:3000/v1"}, nil},
	{"zed", ".config/zed/settings.json",
		"{\n  // zed config\n  \"theme\": \"One Dark\"\n}\n",
		[]string{"// zed config", `"theme": "One Dark"`},
		[]string{`"api_url": "http://localhost:3000/v1"`, `"Convergence Auto"`}, nil},
	{"cline", ".config/Code/User/settings.json",
		"{\n  // vscode config\n  \"editor.fontSize\": 14\n}\n",
		[]string{"// vscode config", `"editor.fontSize": 14`},
		[]string{`"cline.openAiBaseUrl": "http://localhost:3000/v1"`, `"cline.openAiApiKey": "convergence"`}, nil},
	{"openclaw", ".openclaw/openclaw.json",
		"{\n  // my comment\n  \"agents\": {\"defaults\": {\"model\": {\"primary\": \"x/y\"}}},\n  \"models\": {\"providers\": {\"other\": {\"api\": \"x\",},},},\n}\n",
		[]string{"// my comment", `"primary": "x/y"`, `"other": {"api": "x",}`},
		[]string{`"convergence"`, "http://localhost:3000/v1", `"openai-completions"`, `"id": "auto"`, `"id": "heavy"`}, nil},
}

func setup(t *testing.T, f fixture, withFile bool) string {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("HERMES_HOME", "")
	p := filepath.Join(h, f.rel)
	if withFile {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f.existing), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFreshFileAndRestoreDeletes(t *testing.T) {
	for _, f := range fixtures {
		t.Run(f.client, func(t *testing.T) {
			p := setup(t, f, false)
			if _, err := Configure(f.client, Options{GatewayURL: gw}); err != nil {
				t.Fatal(err)
			}
			got := read(t, p)
			for _, w := range f.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q in:\n%s", w, got)
				}
			}
			if strings.Contains(got, "sk-") {
				t.Error("unexpected secret")
			}
			if _, err := Restore(f.client, Options{}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("file should be removed, err=%v", err)
			}
		})
	}
}

func TestExistingPreservedIdempotentRoundTrip(t *testing.T) {
	for _, f := range fixtures {
		t.Run(f.client, func(t *testing.T) {
			p := setup(t, f, true)
			if _, err := Configure(f.client, Options{GatewayURL: gw}); err != nil {
				t.Fatal(err)
			}
			first := read(t, p)
			for _, k := range append(f.keep, f.want...) {
				if !strings.Contains(first, k) {
					t.Errorf("missing %q in:\n%s", k, first)
				}
			}
			st, _ := os.Stat(p)
			if st.Mode().Perm() != 0o640 {
				t.Errorf("mode = %v, want 0640", st.Mode().Perm())
			}
			res, err := Configure(f.client, Options{GatewayURL: gw})
			if err != nil || res.Changed || read(t, p) != first {
				t.Fatalf("second run not idempotent: changed=%v err=%v", res.Changed, err)
			}
			if _, err := Restore(f.client, Options{}); err != nil {
				t.Fatal(err)
			}
			if got := read(t, p); got != f.existing {
				t.Errorf("restore not byte-exact:\n%q\n!=\n%q", got, f.existing)
			}
			if _, err := os.Stat(p + backupSuffix); !os.IsNotExist(err) {
				t.Error("backup should be cleaned up")
			}
		})
	}
}

func TestRestoreRefusesAfterExternalEditThenForceKeepsUserEdit(t *testing.T) {
	for _, f := range fixtures {
		t.Run(f.client, func(t *testing.T) {
			p := setup(t, f, true)
			if _, err := Configure(f.client, Options{GatewayURL: gw}); err != nil {
				t.Fatal(err)
			}
			edited := read(t, p) + "\n# user-added\n"
			if strings.HasSuffix(f.rel, ".json") {
				edited = read(t, p) + "\n// user-added\n"
			}
			if err := os.WriteFile(p, []byte(edited), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Restore(f.client, Options{}); err == nil || !strings.Contains(err.Error(), "--force") {
				t.Fatalf("expected refusal mentioning --force, got %v", err)
			}
			if read(t, p) != edited {
				t.Fatal("refused restore must not touch the file")
			}
			if _, err := Restore(f.client, Options{Force: true}); err != nil {
				t.Fatal(err)
			}
			got := read(t, p)
			if !strings.Contains(got, "user-added") && f.client != "continue" && !strings.HasSuffix(f.rel, ".yml") && !strings.HasSuffix(f.rel, ".yaml") {
				t.Errorf("user edit lost:\n%s", got)
			}
			for _, k := range f.keep {
				if !strings.Contains(got, k) {
					t.Errorf("lost %q:\n%s", k, got)
				}
			}
			if strings.Contains(got, "convergence") && f.client != "hermes" {
				t.Errorf("convergence keys remain:\n%s", got)
			}
		})
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	for _, f := range fixtures {
		t.Run(f.client, func(t *testing.T) {
			p := setup(t, f, true)
			res, err := Configure(f.client, Options{GatewayURL: gw, DryRun: true})
			if err != nil || !res.Changed || res.Diff == "" {
				t.Fatalf("dry-run: changed=%v err=%v", res.Changed, err)
			}
			if read(t, p) != f.existing {
				t.Error("file modified")
			}
			if _, err := os.Stat(p + manifestSuffix); !os.IsNotExist(err) {
				t.Error("manifest written during dry-run")
			}
		})
	}
}

func TestOpenCodeDefaultModelOnlyWithFlag(t *testing.T) {
	f := fixtures[0]
	p := setup(t, f, true)
	if _, err := Configure("opencode", Options{GatewayURL: gw}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, p), `"model": "x/y"`) {
		t.Error("default model changed without --set-default")
	}
	if _, err := Configure("opencode", Options{GatewayURL: gw, SetDefault: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, p), `"model": "convergence/auto"`) {
		t.Error("--set-default not applied")
	}
	if _, err := Restore("opencode", Options{}); err != nil {
		t.Fatal(err)
	}
	if read(t, p) != f.existing {
		t.Error("round trip after set-default not exact")
	}
}

func TestClaudeCodeStripsV1AndKeepsExistingAPIKey(t *testing.T) {
	f := fixtures[1]
	p := setup(t, f, true)
	if _, err := Configure("claude-code", Options{GatewayURL: gw + "/v1/"}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); !strings.Contains(got, `"ANTHROPIC_BASE_URL": "http://localhost:3000"`) {
		t.Errorf("v1 not stripped:\n%s", got)
	}
}

func TestListReportsPointed(t *testing.T) {
	f := fixtures[2]
	setup(t, f, true)
	if _, err := Configure("aider", Options{GatewayURL: gw}); err != nil {
		t.Fatal(err)
	}
	sts, err := List()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sts {
		if s.Client == "aider" && (!s.Present || !s.Pointed) {
			t.Errorf("aider status wrong: %+v", s)
		}
		if s.Client == "goose" && s.Present {
			t.Error("goose should be absent")
		}
	}
}

func TestOpenClawDefaultModelOnlyWithFlag(t *testing.T) {
	f := fixtures[len(fixtures)-1]
	p := setup(t, f, true)
	if _, err := Configure("openclaw", Options{GatewayURL: gw}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, p), `"primary": "x/y"`) || strings.Contains(read(t, p), "pinball") {
		t.Error("default changed without --set-default or pinball added")
	}
	if _, err := Configure("openclaw", Options{GatewayURL: gw, SetDefault: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, p), `"primary": "convergence/auto"`) {
		t.Error("--set-default not applied")
	}
	if _, err := Restore("openclaw", Options{}); err != nil {
		t.Fatal(err)
	}
	if read(t, p) != f.existing {
		t.Error("round trip after set-default not exact")
	}
}
