package engine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func namedPorts(p int) []config.NamedPort { return []config.NamedPort{{Name: "x", Port: p}} }

func TestOllamaMissing_errorPaths(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"http 500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }, "HTTP 500"},
		{"bad json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) }, "decoding"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			_, err := OllamaMissing(t.Context(), srv.URL, []string{"a"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := OllamaMissing(t.Context(), "http://127.0.0.1:1", nil); err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("got %v", err)
	}
	if _, err := OllamaMissing(t.Context(), "http://bad host", nil); err == nil {
		t.Fatal("want request build error")
	}
}
