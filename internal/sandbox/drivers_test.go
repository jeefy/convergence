package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jeefy/convergence/internal/config"
)

func TestNoneDriver(t *testing.T) {
	d, err := GetDriver("", nil)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := d.Status(context.Background())
	if st.Available || st.Type != config.SandboxNone {
		t.Errorf("unexpected status %+v", st)
	}
	if d.InferenceURL() != "" {
		t.Error("none driver has no inference URL")
	}
	cmd, env, err := d.RunCommand("agent", []string{"a"})
	if err != nil || !slices.Equal(cmd, []string{"agent", "a"}) || env != nil {
		t.Errorf("got %v %v %v", cmd, env, err)
	}
}

func TestDockerSbxDriver(t *testing.T) {
	d, _ := GetDriver(config.SandboxDockerSbx, nil)
	st, err := d.Status(context.Background())
	if err != nil || !st.Available || st.Endpoint != "docker-sbx" {
		t.Errorf("status %+v %v", st, err)
	}
	if !strings.Contains(d.InferenceURL(), "host.docker.internal") {
		t.Errorf("url %s", d.InferenceURL())
	}
	cmd, env, _ := d.RunCommand("x", []string{"y"}, "/a", "/b")
	want := []string{"sbx", "run", "-v", "/a", "-v", "/b", "--", "x", "y"}
	if !slices.Equal(cmd, want) {
		t.Errorf("cmd %v", cmd)
	}
	if len(env) != 1 || env[0] != "OPENAI_BASE_URL="+d.InferenceURL() {
		t.Errorf("env %v", env)
	}
}

func TestK8sAgentDriver(t *testing.T) {
	d, _ := GetDriver(config.SandboxK8sAgent, nil)
	st, err := d.Status(context.Background())
	if err != nil || !st.Available || st.Endpoint != "k8s-agent" {
		t.Errorf("status %+v %v", st, err)
	}
	if d.InferenceURL() != "http://agentgateway.convergence.svc:3000/v1" {
		t.Errorf("url %s", d.InferenceURL())
	}
	if _, _, err := d.RunCommand("x", nil); err == nil {
		t.Error("expected error")
	}
}

func TestNonoDriver(t *testing.T) {
	orig := execLookPath
	t.Cleanup(func() { execLookPath = orig })

	execLookPath = func(string) (string, error) { return "", errors.New("nope") }
	d := &nonoDriver{}
	st, _ := d.Status(context.Background())
	if st.Available || !strings.Contains(st.Message, "not found") {
		t.Errorf("status %+v", st)
	}
	execLookPath = func(string) (string, error) { return "/fake/nono", nil }
	st, _ = d.Status(context.Background())
	if !st.Available || st.Endpoint != "/fake/nono" {
		t.Errorf("status %+v", st)
	}

	if d.InferenceURL() != "http://127.0.0.1:3000/v1" {
		t.Errorf("nil cfg url %s", d.InferenceURL())
	}
	d = &nonoDriver{cfg: &config.Config{BindAddress: "10.1.2.3", GatewayPort: 4000}}
	if d.InferenceURL() != "http://10.1.2.3:4000/v1" {
		t.Errorf("cfg url %s", d.InferenceURL())
	}
	cmd, env, _ := d.RunCommand("a", nil, "/m")
	if !slices.Equal(cmd, []string{"nono", "run", "--allow-cwd", "--allow-path", "/m", "--", "a"}) {
		t.Errorf("cmd %v", cmd)
	}
	if !slices.Contains(env, "OPENAI_BASE_URL=http://10.1.2.3:4000/v1") {
		t.Errorf("env %v", env)
	}
}

func TestOpenShellStatusUnreachableAndHealthy(t *testing.T) {
	d := &openShellDriver{}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", config.OpenShellHealthPort))
	if err != nil {
		st, _ := d.Status(context.Background())
		if st.Available {
			t.Error("port busy elsewhere; expected unavailable only if nothing healthy")
		}
		t.Skip("health port in use")
	}
	st, _ := d.Status(context.Background())
	_ = ln.Close()
	if st.Available {
		t.Fatalf("listener without HTTP must not be healthy: %+v", st)
	}
	if !strings.Contains(st.Message, "unreachable") {
		t.Errorf("msg %q", st.Message)
	}

	for _, code := range []int{http.StatusServiceUnavailable, http.StatusOK} {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", config.OpenShellHealthPort))
		if err != nil {
			t.Skip("health port in use")
		}
		srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })}
		go func() { _ = srv.Serve(ln) }()
		st, _ := d.Status(context.Background())
		_ = srv.Close()
		if st.Available != (code == http.StatusOK) {
			t.Errorf("code %d: %+v", code, st)
		}
		if code == http.StatusOK && st.Endpoint != fmt.Sprintf("http://127.0.0.1:%d", config.OpenShellPort) {
			t.Errorf("endpoint %s", st.Endpoint)
		}
	}
}

func TestOpenShellStatusBadContext(t *testing.T) {
	d := &openShellDriver{cfg: &config.Config{BindAddress: "127.0.0.1"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st, err := d.Status(ctx)
	if err != nil || st.Available {
		t.Errorf("%+v %v", st, err)
	}
}

func TestAgentArgsHermes(t *testing.T) {
	const u = "http://127.0.0.1:3000/v1"
	fl := []string{"--provider", "custom", "--base-url", u, "--api-key", "convergence"}
	for name, c := range map[string]struct {
		agent string
		in    []string
		want  []string
	}{
		"bare":       {"hermes", nil, append([]string{"chat"}, fl...)},
		"flags":      {"/usr/bin/hermes", []string{"-m", "auto"}, append(append([]string{"chat"}, fl...), "-m", "auto")},
		"chat":       {"hermes", []string{"chat", "-q", "hi"}, append(append([]string{"chat"}, fl...), "-q", "hi")},
		"other sub":  {"hermes", []string{"gateway", "status"}, []string{"gateway", "status"}},
		"other tool": {"opencode", []string{"x"}, []string{"x"}},
	} {
		if got := AgentArgs(c.agent, c.in, u); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v want %v", name, got, c.want)
		}
	}
	if got := AgentArgs("hermes", nil, ""); got != nil {
		t.Errorf("no inference URL must pass through, got %v", got)
	}
}
