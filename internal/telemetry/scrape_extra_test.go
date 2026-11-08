package telemetry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScrape_parsesOKResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("foo_total 3\n"))
	}))
	defer srv.Close()
	smp, err := Scrape(context.Background(), srv.Client(), srv.URL)
	if err != nil || len(smp) != 1 || smp[0].Value != 3 {
		t.Fatalf("smp=%v err=%v", smp, err)
	}
}

func TestScrape_errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := Scrape(context.Background(), srv.Client(), srv.URL); err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("status err = %v", err)
	}
	if _, err := Scrape(context.Background(), srv.Client(), "http://bad host/"); err == nil {
		t.Error("want request build error")
	}
	url := srv.URL
	srv.Close()
	if _, err := Scrape(context.Background(), http.DefaultClient, url); err == nil {
		t.Error("want connection error")
	}
}

func TestCollect_marksUnreachableSourcesAndFillsReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("x 1\n"))
	}))
	defer srv.Close()
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	st := Collect(context.Background(), Endpoints{Gateway: srv.URL, StandIn: deadURL, Pinball: srv.URL})
	if st.StandIn.Error == "" || st.AllDown() {
		t.Fatalf("stats = %+v", st)
	}
	if st.StandIn.Error != "connection refused" {
		t.Errorf("error = %q", st.StandIn.Error)
	}
}

func TestErrText_classifies(t *testing.T) {
	for in, want := range map[string]string{
		"dial tcp: connection refused": "connection refused",
		"context deadline exceeded":    "timeout",
		"Client.Timeout exceeded":      "timeout",
		"weird":                        "weird",
	} {
		if got := errText(errors.New(in)); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
