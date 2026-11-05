package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestOllamaMissing(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen2.5-coder:7b"},{"name":"llama3:latest"}]}`))
	}))
	defer s.Close()

	got, err := OllamaMissing(context.Background(), s.URL+"/", []string{"qwen2.5-coder:7b", "qwen2.5-coder:14b", "llama3"})

	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"qwen2.5-coder:14b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("missing = %v, want %v", got, want)
	}
}

func TestOllamaMissing_when_unreachable(t *testing.T) {
	s := httptest.NewServer(http.NotFoundHandler())
	url := s.URL
	s.Close()
	_, err := OllamaMissing(context.Background(), url, []string{"x"})
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("want unreachable error, got %v", err)
	}
}
