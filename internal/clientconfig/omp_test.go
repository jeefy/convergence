package clientconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jeefy/convergence/internal/testhome"
)

// ompTestHome is a scratch home with omp's default layout (no directory overrides).
func ompTestHome(t *testing.T) string {
	t.Helper()
	h := testhome.New(t)
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("PI_CONFIG_DIR", "")
	return h
}

func TestOMPConfigurePreservesProvidersAndRestores(t *testing.T) {
	h := ompTestHome(t)
	p := filepath.Join(h, ".omp", "agent", "models.yml")
	original := "# personal models\nproviders:\n  other:\n    baseUrl: http://example.com/v1\n    apiKey: OTHER_API_KEY\n    api: openai-completions\n    models:\n      - id: personal\n"
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(filepath.Dir(p), "config.yml")
	settingsOriginal := "modelRoles:\n  default: other/personal\n  smol: other/personal\n"
	if err := os.WriteFile(settings, []byte(settingsOriginal), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Configure("omp", Options{GatewayURL: "http://gateway:3210/v1/", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if read(t, p) != original {
		t.Fatal("dry-run modified the existing providers")
	}
	if _, err := os.Stat(p + backupSuffix); !os.IsNotExist(err) {
		t.Fatalf("dry-run created a backup: %v", err)
	}
	if _, err := Configure("omp", Options{GatewayURL: "http://gateway:3210/v1/"}); err != nil {
		t.Fatal(err)
	}
	configured := read(t, p)
	doc, err := yamlGet([]byte(configured))
	if err != nil {
		t.Fatal(err)
	}
	if lookupString(doc, "providers", "other", "apiKey") != "OTHER_API_KEY" {
		t.Fatal("existing provider credential changed")
	}
	if lookupString(doc, "providers", "convergence", "baseUrl") != "http://gateway:3210/v1" {
		t.Fatal("gateway URL not normalized")
	}
	modelsRaw, ok := lookup(doc, "providers", "convergence", "models")
	if !ok {
		t.Fatal("missing convergence models list")
	}
	modelsList, ok := modelsRaw.([]any)
	if !ok || len(modelsList) != 5 {
		t.Fatalf("unexpected models list: %+v", modelsRaw)
	}
	firstModel, ok := modelsList[0].(map[string]any)
	if !ok || firstModel["id"] != "auto" {
		t.Fatalf("unexpected first model: %+v", modelsList[0])
	}
	if firstModel["contextWindow"] != 128000 || firstModel["maxTokens"] != 16384 || firstModel["reasoning"] != false {
		t.Fatalf("auto model capabilities are not explicitly bounded: %+v", firstModel)
	}
	if read(t, settings) != settingsOriginal {
		t.Fatal("omp's model roles changed")
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("existing permissions changed: %v", st.Mode().Perm())
	}
	res, err := Configure("omp", Options{GatewayURL: "http://gateway:3210/v1/"})
	if err != nil || res.Changed || read(t, p) != configured {
		t.Fatalf("repeated configuration is not idempotent: %+v %v", res, err)
	}
	if _, err := Restore("omp", Options{}); err != nil {
		t.Fatal(err)
	}
	if read(t, p) != original {
		t.Fatal("restore did not recover the original bytes and comments")
	}
}

func TestOMPAgentDirectoryAndYAMLFallback(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "config-root", true: "agent-dir"}[override], func(t *testing.T) {
			h := ompTestHome(t)
			t.Setenv("PI_CONFIG_DIR", ".custom-omp")
			dir := filepath.Join(h, ".custom-omp", "agent")
			if override {
				dir = filepath.Join(h, "profile with spaces", "agent")
				t.Setenv("PI_CODING_AGENT_DIR", dir)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(dir, "models.yaml")
			original := "# keep fallback\nproviders: {}\n"
			if err := os.WriteFile(p, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			res, err := Configure("omp", Options{GatewayURL: gw})
			if err != nil || res.Path != p {
				t.Fatalf("wrong active file: %+v %v", res, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "models.yml")); !os.IsNotExist(err) {
				t.Fatalf("created a canonical file shadowing existing providers: %v", err)
			}
			if _, err := Restore("omp", Options{}); err != nil {
				t.Fatal(err)
			}
			if read(t, p) != original {
				t.Fatal("fallback file not restored")
			}
		})
	}
}

func TestOMPRestoreAfterUserEdit(t *testing.T) {
	h := ompTestHome(t)
	p := filepath.Join(h, ".omp", "agent", "models.yml")
	if _, err := Configure("omp", Options{GatewayURL: gw}); err != nil {
		t.Fatal(err)
	}
	cur := read(t, p)
	edited, err := yamlSet([]byte(cur), []string{"providers", "personal"}, map[string]any{"apiKey": "PERSONAL_API_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore("omp", Options{}); err == nil {
		t.Fatal("restore overwrote later user edits without --force")
	}
	if read(t, p) != string(edited) {
		t.Fatal("refused restore still modified the file")
	}
	if _, err := Restore("omp", Options{Force: true}); err != nil {
		t.Fatal(err)
	}
	doc, err := yamlGet([]byte(read(t, p)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lookup(doc, "providers", "convergence"); ok {
		t.Fatal("forced restore left Convergence configured")
	}
	if lookupString(doc, "providers", "personal", "apiKey") != "PERSONAL_API_KEY" {
		t.Fatal("forced restore removed the user's new provider")
	}
}
