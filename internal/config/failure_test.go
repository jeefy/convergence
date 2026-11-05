package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sandbox(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(home, ".convergence")
}

func writeCfg(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfig_when_noFile_returnsDefaults(t *testing.T) {
	sandbox(t)
	cfg, err := LoadConfig()
	if err != nil || cfg.GatewayPort != DefaultGatewayPort || cfg.StandinSalt == "" {
		t.Fatalf("%+v %v", cfg, err)
	}
	if ok, err := Exists(); ok || err != nil {
		t.Fatalf("Exists = %v %v", ok, err)
	}
}

func TestLoadConfig_when_corruptYAML(t *testing.T) {
	dir := sandbox(t)
	writeCfg(t, dir, "engine: [unterminated\n  provider: {")
	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "parsing config") || !strings.Contains(err.Error(), "config.yaml") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadConfig_when_wrongType(t *testing.T) {
	dir := sandbox(t)
	writeCfg(t, dir, "gateway_port: not-a-number\n")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("want type error")
	}
}

func TestLoadConfig_when_unreadable(t *testing.T) {
	dir := sandbox(t)
	// config.yaml as a directory: ReadFile fails with a non-NotExist error.
	if err := os.MkdirAll(filepath.Join(dir, "config.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "reading config") {
		t.Fatalf("got %v", err)
	}
	if ok, err := Exists(); !ok || err != nil {
		t.Errorf("Exists = %v %v", ok, err)
	}
}

func TestLoadConfig_when_chmod000(t *testing.T) {
	dir := sandbox(t)
	writeCfg(t, dir, "engine: docker\n")
	p := filepath.Join(dir, "config.yaml")
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(p, 0o600) }()
	if f, err := os.Open(p); err == nil { // running as root: nothing to test
		_ = f.Close()
		t.Skip("file readable despite mode 0")
	}
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "reading config") {
		t.Fatalf("got %v", err)
	}
}

// Unknown keys are tolerated (yaml.v3 non-strict) so newer configs load on older binaries.
func TestLoadConfig_when_unknownFields_ignored(t *testing.T) {
	dir := sandbox(t)
	writeCfg(t, dir, "engine: podman\nfuture_option: 1\nnested:\n  a: b\n")
	cfg, err := LoadConfig()
	if err != nil || cfg.Engine != "podman" {
		t.Fatalf("%+v %v", cfg, err)
	}
}

func TestLoadConfig_when_oldConfig_fillsDefaults(t *testing.T) {
	dir := sandbox(t)
	writeCfg(t, dir, "engine: docker\nprovider: ollama\n")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GatewayPort != DefaultGatewayPort || cfg.BindAddress != DefaultBindAddress || cfg.OptSchedule != DefaultOptSchedule {
		t.Errorf("defaults not applied: %+v", cfg)
	}
	if len(cfg.StandinSalt) != 64 {
		t.Errorf("salt not generated: %q", cfg.StandinSalt)
	}
	if cfg.FailOpen {
		t.Error("old config must stay fail-closed")
	}
}

func TestLoadConfig_keepsExplicitValues(t *testing.T) {
	dir := sandbox(t)
	writeCfg(t, dir, "gateway_port: 8080\nbind_address: 10.0.0.5\nstandin_salt: abc\nopt_schedule: '* * * * *'\n")
	cfg, err := LoadConfig()
	if err != nil || cfg.GatewayPort != 8080 || cfg.BindAddress != "10.0.0.5" || cfg.StandinSalt != "abc" || cfg.OptSchedule != "* * * * *" {
		t.Fatalf("%+v %v", cfg, err)
	}
}

func TestSaveConfig_modeAndRoundtrip(t *testing.T) {
	dir := sandbox(t)
	in := DefaultConfig()
	in.Provider = ProviderCopilot
	in.CopilotToken = "tok"
	in.FailOpen = true
	in.OllamaModelHeavy = "big"

	if err := SaveConfig(in); err != nil {
		t.Fatal(err)
	}

	st, err := os.Stat(filepath.Join(dir, "config.yaml"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v err %v", st.Mode().Perm(), err)
	}
	out, err := LoadConfig()
	if err != nil || *out != *in {
		t.Fatalf("roundtrip:\n in=%+v\nout=%+v\n%v", in, out, err)
	}
}

func TestSaveConfig_when_existingLooseMode_staysAsIs(t *testing.T) {
	// WriteFile does not chmod an existing file; document the behavior.
	dir := sandbox(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("x: 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode().Perm())
	}
}

func TestSaveConfig_when_dirNotWritable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(home, 0o700) }()
	if err := os.WriteFile(filepath.Join(home, "probe"), nil, 0o600); err == nil {
		t.Skip("running as root; permissions not enforced")
	}
	if err := SaveConfig(DefaultConfig()); err == nil || !strings.Contains(err.Error(), "creating config dir") {
		t.Fatalf("got %v", err)
	}
}

func TestSaveConfig_when_dirIsAFile(t *testing.T) {
	dir := sandbox(t)
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(DefaultConfig()); err == nil || !strings.Contains(err.Error(), "creating config dir") {
		t.Fatalf("got %v", err)
	}
}

func TestSaveConfig_when_configPathIsDir(t *testing.T) {
	dir := sandbox(t)
	if err := os.MkdirAll(filepath.Join(dir, "config.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(DefaultConfig()); err == nil {
		t.Fatal("want write error")
	}
}

func TestExists_when_homeUnset(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := GetConfigDir(); err == nil {
		t.Skip("home resolvable on this platform")
	}
	if _, err := Exists(); err == nil {
		t.Fatal("want error")
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("want error")
	}
	if err := SaveConfig(DefaultConfig()); err == nil {
		t.Fatal("want error")
	}
}

func TestParseProvider_and_ports(t *testing.T) {
	if _, err := ParseProvider("nope"); err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("got %v", err)
	}
	for _, p := range []string{"copilot", "direct", "ollama"} {
		if got, err := ParseProvider(p); err != nil || string(got) != p {
			t.Errorf("%s: %v", p, err)
		}
	}
	c := DefaultConfig()
	if len(c.PublishedPorts()) != 4 {
		t.Errorf("non-copilot ports: %v", c.PublishedPorts())
	}
	c.Provider = ProviderCopilot
	ports := c.PublishedPorts()
	if ports[len(ports)-1].Port != CopilotAPIPort {
		t.Errorf("copilot port missing: %v", ports)
	}
	for bind, want := range map[string]string{"": "127.0.0.1", "0.0.0.0": "127.0.0.1", "::": "127.0.0.1", "10.1.1.1": "10.1.1.1"} {
		c.BindAddress = bind
		if got := c.HealthHost(); got != want {
			t.Errorf("HealthHost(%q)=%q", bind, got)
		}
	}
}
