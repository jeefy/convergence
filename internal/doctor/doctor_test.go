package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func newEnv(t *testing.T) *Env {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderOllama
	e := NewEnv(cfg, t.TempDir())
	e.HTTP = &http.Client{}
	return e
}

func TestCheckStandin(t *testing.T) {
	tests := []struct {
		name, metrics, compose string
		want                   Status
		contains               string
	}{
		{"enforce", "standin_matches_total 1\n", "STANDIN_MODE: enforce", Pass, "mode=enforce"},
		{"audit mode warns", "standin_matches_total 1\n", `STANDIN_MODE: "audit"`, Warn, "mode=audit"},
		{"unknown mode", "standin_matches_total 1\n", "", Warn, "unknown"},
		{"no standin series", "other_metric 1\n", "", Fail, "no standin_*"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, tc.metrics) }))
			defer srv.Close()
			e := newEnv(t)
			e.StandinURL = srv.URL
			if err := os.WriteFile(filepath.Join(e.Dir, "docker-compose.yaml"), []byte("    environment:\n      "+tc.compose+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			r := CheckStandin(context.Background(), e)
			if r.Status != tc.want || !strings.Contains(r.Detail, tc.contains) {
				t.Fatalf("got %+v", r)
			}
		})
	}
	t.Run("down", func(t *testing.T) {
		srv := httptest.NewServer(nil)
		e := newEnv(t)
		e.StandinURL = srv.URL
		srv.Close()
		if r := CheckStandin(context.Background(), e); r.Status != Fail || r.Hint == "" {
			t.Fatalf("got %+v", r)
		}
	})
}

func TestCheckPinball(t *testing.T) {
	var gotPath, gotCT string
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotCT = r.URL.Path, r.Header.Get("Content-Type")
		var b struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies = append(bodies, b.Messages[0].Content)
		tier := map[bool]string{true: "light", false: "heavy"}[len(b.Messages[0].Content) < 5]
		_, _ = fmt.Fprintf(w, `{"model":"m-%s","tier":"%s","reason":"x"}`, tier, tier)
	}))
	defer srv.Close()
	e := newEnv(t)
	e.PinballURL = srv.URL
	rs := CheckPinball(context.Background(), e)
	if len(rs) != 3 || gotPath != "/check" || gotCT != "application/json" || len(bodies) != 3 {
		t.Fatalf("rs=%v path=%s", rs, gotPath)
	}
	if rs[0].Status != Pass || !strings.Contains(rs[0].Detail, "tier=light model=m-light") || !strings.Contains(rs[2].Detail, "tier=heavy") {
		t.Fatalf("%+v", rs)
	}
}

func TestCheckPinballBadResponses(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"500":        func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) },
		"empty json": func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, `{}`) },
		"not json":   func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, `nope`) },
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			e := newEnv(t)
			e.PinballURL = srv.URL
			for _, r := range CheckPinball(context.Background(), e) {
				if r.Status != Fail || r.Hint == "" {
					t.Fatalf("%+v", r)
				}
			}
		})
	}
}

func TestCheckProvider(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if r.URL.Query().Get("fail") != "" {
			w.WriteHeader(503)
		}
	}))
	defer srv.Close()
	tests := []struct {
		name     string
		provider config.ProviderType
		url      string
		want     Status
		path     string
	}{
		{"ollama ok", config.ProviderOllama, srv.URL, Pass, "/api/tags"},
		{"ollama 5xx", config.ProviderOllama, srv.URL + "?fail=1", Fail, "/api/tags"},
		{"copilot ok", config.ProviderCopilot, srv.URL, Pass, "/v1/models"},
		{"direct is info", config.ProviderDirect, srv.URL, Info, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path = ""
			e := newEnv(t)
			e.Cfg.Provider = tc.provider
			e.OllamaURL, e.CopilotURL = strings.SplitN(tc.url, "?", 2)[0], strings.SplitN(tc.url, "?", 2)[0]
			if strings.Contains(tc.url, "?") {
				// force failure via a handler-visible query on the path-less base
				e.OllamaURL = strings.SplitN(tc.url, "?", 2)[0]
			}
			r := CheckProvider(context.Background(), e)
			if tc.name == "ollama 5xx" {
				srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
				defer srv2.Close()
				e.OllamaURL = srv2.URL
				r = CheckProvider(context.Background(), e)
			}
			if r.Status != tc.want || (tc.path != "" && tc.name != "ollama 5xx" && path != tc.path) {
				t.Fatalf("%+v path=%q", r, path)
			}
		})
	}
}

func TestCheckFailMode(t *testing.T) {
	gw := func(mode string) string {
		return "gateways:\n  default:\n    extProc:\n      conditional:\n        - host: \"stand-in:9101\"\n          failureMode: " + mode + "\n"
	}
	tests := []struct {
		name     string
		failOpen bool
		rendered string
		want     Status
	}{
		{"closed", false, gw("failClosed"), Pass},
		{"closed no render", false, "", Pass},
		{"config fail_open", true, gw("failClosed"), Warn},
		{"rendered failOpen", false, gw("failOpen"), Warn},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.FailOpen = tc.failOpen
			if tc.rendered != "" {
				if err := os.WriteFile(filepath.Join(e.Dir, "agentgateway.yaml"), []byte(tc.rendered), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			r := CheckFailMode(e)
			if r.Status != tc.want || (tc.want == Warn && !strings.Contains(r.Detail, "FAIL-OPEN")) {
				t.Fatalf("%+v", r)
			}
		})
	}
}

func TestReadFailOpenMissingIsFalse(t *testing.T) {
	d := t.TempDir()
	if ReadFailOpen(d) {
		t.Fatal("missing file must be false")
	}
	if err := os.WriteFile(filepath.Join(d, "config.yaml"), []byte("provider: ollama\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ReadFailOpen(d) {
		t.Fatal("missing key must be false")
	}
	if err := os.WriteFile(filepath.Join(d, "config.yaml"), []byte("fail_open: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !ReadFailOpen(d) {
		t.Fatal("want true")
	}
}

func TestCheckPortsWarnsOnNonLoopback(t *testing.T) {
	e := newEnv(t)
	e.Cfg.BindAddress = "0.0.0.0"
	if r := CheckPorts(context.Background(), e)[0]; r.Status != Warn {
		t.Fatalf("%+v", r)
	}
}

func TestCheckConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".convergence")
	if r, cfg := CheckConfig(dir); r.Status != Fail || cfg != nil {
		t.Fatalf("missing: %+v", r)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("gateway_port: [oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r, _ := CheckConfig(dir); r.Status != Fail || !strings.Contains(r.Detail, "parse") {
		t.Fatalf("bad yaml: %+v", r)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("provider: ollama\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r, _ := CheckConfig(dir); r.Status != Fail || !strings.Contains(r.Detail, "0644") {
		t.Fatalf("perm: %+v", r)
	}
	if err := os.Chmod(filepath.Join(dir, ".env"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r, cfg := CheckConfig(dir); r.Status != Pass || cfg == nil {
		t.Fatalf("ok: %+v", r)
	}
}

func TestCheckEngine(t *testing.T) {
	e := newEnv(t)
	e.Cfg.Engine = "docker"
	var calls []string
	for name, failOn := range map[string]string{"ok": "", "info fails": "info", "no compose": "compose"} {
		calls = nil
		e.Run = func(_ context.Context, n string, a ...string) error {
			calls = append(calls, a[0])
			if a[0] == failOn {
				return fmt.Errorf("boom")
			}
			return nil
		}
		r := CheckEngine(context.Background(), e)
		want := map[string]Status{"ok": Pass, "info fails": Fail, "no compose": Fail}[name]
		if r.Status != want {
			t.Fatalf("%s: %+v", name, r)
		}
	}
}

type fakeSvc []ServiceStatus

func (f fakeSvc) Check(context.Context) []ServiceStatus { return f }

func TestCheckServices(t *testing.T) {
	e := newEnv(t)
	e.Services = fakeSvc{{Name: "a", Running: true, Healthy: true}, {Name: "b", Running: true}, {Name: "c"}}
	rs := CheckServices(context.Background(), e)
	if rs[0].Status != Pass || rs[1].Status != Fail || rs[2].Status != Fail {
		t.Fatalf("%+v", rs)
	}
}

func TestCounterDeltaProblems(t *testing.T) {
	b := Counters{1, 1, 1}
	tests := []struct {
		name  string
		after Counters
		want  int
	}{
		{"all up", Counters{2, 2, 2}, 0},
		{"none up", b, 3},
		{"restored stuck", Counters{2, 1, 2}, 1},
		{"decision counter reset", Counters{2, 2, 0}, 1},
	}
	for _, tc := range tests {
		if got := CounterDeltaProblems(b, tc.after); len(got) != tc.want {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}

func TestLeakedPlaceholderAndToken(t *testing.T) {
	if !LeakedPlaceholder(`{"x":"<<secret-ab12>>"}`) || LeakedPlaceholder(`OK`) {
		t.Fatal("leak detection wrong")
	}
	tok, err := FakeGitHubToken()
	if err != nil || !regexp.MustCompile(`^ghp_[A-Za-z0-9]{36}$`).MatchString(tok) {
		t.Fatalf("%q %v", tok, err)
	}
}

// fakeStack serves gateway + metrics; counters move only when the request is "handled".
type fakeStack struct {
	matches, restored, decisions atomic.Int64
	respond                      func(token string) (int, string)
	gotBody                      atomic.Value
}

func (f *fakeStack) env(t *testing.T) *Env {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/standin/metrics", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, "standin_matches_total{rule=\"gh\"} %d\nstandin_restored_total %d\n", f.matches.Load(), f.restored.Load())
	})
	mux.HandleFunc("/pinball/metrics", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, "pinball_decisions_total{tier=\"light\",model=\"m\"} %d\n", f.decisions.Load())
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			Messages  []struct{ Content string }
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		f.gotBody.Store(b)
		tok := regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`).FindString(b.Messages[0].Content)
		code, body := f.respond(tok)
		w.WriteHeader(code)
		_, _ = fmt.Fprint(w, body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	e := newEnv(t)
	e.GatewayURL, e.StandinURL, e.PinballURL = srv.URL, srv.URL+"/standin", srv.URL+"/pinball"
	return e
}

func TestCheckE2E(t *testing.T) {
	bump := func(f *fakeStack, m, r, d int64) { f.matches.Add(m); f.restored.Add(r); f.decisions.Add(d) }
	tests := []struct {
		name    string
		handler func(f *fakeStack) func(string) (int, string)
		want    Status
		detail  string
	}{
		{"healthy", func(f *fakeStack) func(string) (int, string) {
			return func(tok string) (int, string) {
				bump(f, 1, 1, 1)
				return 200, `{"model":"routed-x","choices":[{"message":{"content":"OK"}}]}`
			}
		}, Pass, "routed model=routed-x"},
		{"placeholder leaked", func(f *fakeStack) func(string) (int, string) {
			return func(string) (int, string) {
				bump(f, 1, 1, 1)
				return 200, `{"choices":[{"message":{"content":"<<secret-1>>"}}]}`
			}
		}, Fail, "unrestored"},
		{"restored counter did not move", func(f *fakeStack) func(string) (int, string) {
			return func(string) (int, string) { bump(f, 1, 0, 1); return 200, `{}` }
		}, Fail, "standin_restored_total did not increase"},
		{"pinball bypassed", func(f *fakeStack) func(string) (int, string) {
			return func(string) (int, string) { bump(f, 1, 1, 0); return 200, `{}` }
		}, Fail, "pinball_decisions_total did not increase"},
		{"upstream error", func(f *fakeStack) func(string) (int, string) {
			return func(string) (int, string) { return 503, `upstream call failed` }
		}, Fail, "HTTP 503"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeStack{}
			f.respond = tc.handler(f)
			e := f.env(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20e9)
			defer cancel()
			r := CheckE2E(ctx, e)
			if r.Status != tc.want || !strings.Contains(r.Detail, tc.detail) {
				t.Fatalf("%+v", r)
			}
			b := f.gotBody.Load().(struct {
				Model     string `json:"model"`
				MaxTokens int    `json:"max_tokens"`
				Messages  []struct{ Content string }
			})
			if b.Model != "auto" || b.MaxTokens == 0 || b.MaxTokens > 64 || !strings.Contains(b.Messages[0].Content, "OK") {
				t.Fatalf("bad request %+v", b)
			}
		})
	}
}

func TestRunOnlyIncludesE2EWhenRequested(t *testing.T) {
	f := &fakeStack{respond: func(string) (int, string) { return 200, `{}` }}
	e := f.env(t)
	e.Run = func(context.Context, string, ...string) error { return nil }
	e.Services = fakeSvc{}
	for _, on := range []bool{false, true} {
		has := false
		for _, r := range Run(context.Background(), e, Options{E2E: on}) {
			has = has || r.Name == "e2e round trip"
		}
		if has != on {
			t.Fatalf("e2e=%v present=%v", on, has)
		}
	}
}

func TestOutputAndExitCode(t *testing.T) {
	rs := []Result{
		{"a", Pass, "fine", ""},
		{"b", Warn, "meh", "do x"},
		{"c", Info, "note", ""},
	}
	if ExitCode(rs) != 0 {
		t.Fatal("warn must not fail")
	}
	rs = append(rs, Result{"d", Fail, "bad", "fix it"})
	if ExitCode(rs) != 1 || ExitCode(nil) != 0 {
		t.Fatal("fail must exit 1")
	}
	var txt bytes.Buffer
	WriteText(&txt, rs)
	out := txt.String()
	for _, w := range []string{"PASS a - fine", "WARN b - meh", "hint: do x", "FAIL d - bad", "1 passed, 1 warnings, 1 failed, 1 info"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in\n%s", w, out)
		}
	}
	if strings.Contains(out, "hint: \n") {
		t.Error("empty hint printed")
	}
	var js bytes.Buffer
	if err := WriteJSON(&js, rs); err != nil {
		t.Fatal(err)
	}
	var back struct {
		Checks  []Result `json:"checks"`
		Summary Summary  `json:"summary"`
	}
	if err := json.Unmarshal(js.Bytes(), &back); err != nil || len(back.Checks) != 4 || back.Summary.OK || back.Summary.Fail != 1 || back.Checks[3].Hint != "fix it" {
		t.Fatalf("%v %+v", err, back)
	}
}
