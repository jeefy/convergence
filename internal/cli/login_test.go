package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

// githubStub serves the device endpoints; token replies come from replies in order
// (the last one repeats).
func githubStub(t *testing.T, device map[string]any, replies []map[string]any) (forms *[]string) {
	t.Helper()
	var seen []string
	var n atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		_ = r.ParseForm()
		seen = append(seen, "device:"+r.PostForm.Get("client_id")+":"+r.PostForm.Get("scope"))
		_ = json.NewEncoder(w).Encode(device)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		_ = r.ParseForm()
		seen = append(seen, "token:"+r.PostForm.Get("device_code")+":"+r.PostForm.Get("grant_type"))
		i := int(n.Add(1)) - 1
		if i >= len(replies) {
			i = len(replies) - 1
		}
		_ = json.NewEncoder(w).Encode(replies[i])
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	oldD, oldT, oldP := githubDeviceURL, githubTokenURL, deviceMinPollSec
	githubDeviceURL, githubTokenURL, deviceMinPollSec = srv.URL+"/device", srv.URL+"/token", 0
	t.Cleanup(func() { githubDeviceURL, githubTokenURL, deviceMinPollSec = oldD, oldT, oldP })
	return &seen
}

var okDevice = map[string]any{"device_code": "DC1", "user_code": "ABCD-1234", "verification_uri": "https://github.com/login/device", "expires_in": 30, "interval": 0}

func TestCopilotConstants(t *testing.T) {
	if copilotClientID != "Iv1.b507a08c87ecfe98" || copilotScope != "read:user" {
		t.Errorf("client id/scope changed: %s %s", copilotClientID, copilotScope)
	}
	if deviceGrantType != "urn:ietf:params:oauth:grant-type:device_code" {
		t.Errorf("grant type %s", deviceGrantType)
	}
}

func TestDeviceLogin_pendingSlowDownSuccess(t *testing.T) {
	forms := githubStub(t, okDevice, []map[string]any{
		{"error": "authorization_pending"},
		{"error": "slow_down", "interval": 0},
		{"access_token": "gho_secret"},
	})
	tok, err := githubDeviceLogin(t.Context())
	if err != nil || tok != "gho_secret" {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
	if (*forms)[0] != "device:"+copilotClientID+":"+copilotScope {
		t.Errorf("device request: %v", *forms)
	}
	if len(*forms) != 4 || !strings.HasPrefix((*forms)[1], "token:DC1:"+deviceGrantType) {
		t.Errorf("polling: %v", *forms)
	}
}

func TestDeviceLogin_failures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		device  map[string]any
		replies []map[string]any
		want    string
	}{
		{"expired token error", okDevice, []map[string]any{{"error": "expired_token", "error_description": "gone"}}, "expired_token"},
		{"denied", okDevice, []map[string]any{{"error": "access_denied", "error_description": "no"}}, "access_denied"},
		{"empty reply", okDevice, []map[string]any{{}}, "neither a token"},
		{"expires locally", map[string]any{"device_code": "x", "expires_in": 0}, []map[string]any{{"error": "authorization_pending"}}, ""}, // either the select or the in-flight POST observes the deadline
	} {
		t.Run(tc.name, func(t *testing.T) {
			githubStub(t, tc.device, tc.replies)
			tok, err := githubDeviceLogin(t.Context())
			if err == nil || tok != "" || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("tok=%q err=%v", tok, err)
			}
		})
	}
}

func TestDeviceLogin_httpErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer srv.Close()
	oldD := githubDeviceURL
	githubDeviceURL = srv.URL
	defer func() { githubDeviceURL = oldD }()
	if _, err := githubDeviceLogin(t.Context()); err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("got %v", err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{not json")) }))
	defer bad.Close()
	githubDeviceURL = bad.URL
	if _, err := githubDeviceLogin(t.Context()); err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Fatalf("got %v", err)
	}
	githubDeviceURL = "http://127.0.0.1:1"
	if _, err := githubDeviceLogin(t.Context()); err == nil {
		t.Fatal("want connection error")
	}
}

func TestLoginCopilot_savesTokenAndConfig(t *testing.T) {
	e := newFakeEnv(t, "docker")
	githubStub(t, okDevice, []map[string]any{{"access_token": "gho_saved"}})

	if err := runCLI(t, "login", "copilot"); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != config.ProviderCopilot || cfg.CopilotToken != "gho_saved" {
		t.Errorf("cfg %+v", cfg)
	}
	for _, f := range []string{"config.yaml", ".env"} {
		st, err := os.Stat(filepath.Join(e.dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v", f, st.Mode().Perm())
		}
	}
	env, err := os.ReadFile(filepath.Join(e.dir, ".env"))
	if err != nil || !strings.Contains(string(env), "gho_saved") {
		t.Errorf(".env lacks token: %v", err)
	}
}

func TestLoginCopilot_when_denied_savesNothing(t *testing.T) {
	e := newFakeEnv(t, "docker")
	githubStub(t, okDevice, []map[string]any{{"error": "access_denied"}})
	if err := runCLI(t, "login", "copilot"); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(filepath.Join(e.dir, "config.yaml")); !os.IsNotExist(err) {
		t.Error("config must not be written on failure")
	}
}

func TestLoginCopilot_when_corruptConfig(t *testing.T) {
	e := newFakeEnv(t, "docker")
	if err := os.MkdirAll(e.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.dir, "config.yaml"), []byte("::: not yaml\n\t-"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "login", "copilot"); err == nil || !strings.Contains(err.Error(), "parsing config") {
		t.Fatalf("got %v", err)
	}
}
