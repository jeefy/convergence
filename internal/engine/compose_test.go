package engine

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func TestCheckPortsFree_when_portTaken(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	err = CheckPortsFree("127.0.0.1", []config.NamedPort{{Name: "pinball", Port: port}})

	var pc *PortConflictError
	if !errors.As(err, &pc) {
		t.Fatalf("expected PortConflictError, got %v", err)
	}
	if pc.Service != "pinball" || !strings.Contains(err.Error(), pc.Addr) {
		t.Errorf("error does not name service/port: %v", err)
	}
}

func TestCheckPortsFree_when_portFree(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close() // freeing the port is the point

	if err := CheckPortsFree("127.0.0.1", []config.NamedPort{{Name: "gw", Port: port}}); err != nil {
		t.Fatalf("expected free port, got %v", err)
	}
}

func TestDetectEngine_when_noneOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := DetectEngine(""); !errors.Is(err, ErrNoEngine) {
		t.Fatalf("expected ErrNoEngine, got %v", err)
	}
}
