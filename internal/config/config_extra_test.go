package config

import (
	"os"
	"testing"
)

func TestParseSandbox(t *testing.T) {
	for in, want := range map[string]SandboxType{
		"": SandboxNone, "none": SandboxNone, "openshell": SandboxOpenShell,
		"nono": SandboxNono, "docker-sbx": SandboxDockerSbx, "k8s-agent": SandboxK8sAgent,
	} {
		if got, err := ParseSandbox(in); err != nil || got != want {
			t.Errorf("%q: %v %v", in, got, err)
		}
	}
	if _, err := ParseSandbox("bogus"); err == nil {
		t.Error("want error")
	}
}

func TestPublishedPorts_byProviderAndSandbox(t *testing.T) {
	c := DefaultConfig()
	if n := len(c.PublishedPorts()); n != 4 {
		t.Errorf("default ports = %d", n)
	}
	c.Provider, c.Sandbox = ProviderCopilot, SandboxOpenShell
	if n := len(c.PublishedPorts()); n != 7 {
		t.Errorf("copilot+openshell ports = %d", n)
	}
}

func TestSaveConfig_failsWhenHomeIsFile(t *testing.T) {
	home := t.TempDir() + "/f"
	if err := writeEmpty(home); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if err := SaveConfig(DefaultConfig()); err == nil {
		t.Error("want error")
	}
}

func TestGenerateSalt_is64Hex(t *testing.T) {
	s, err := GenerateSalt()
	if err != nil || len(s) != 64 {
		t.Fatalf("%q %v", s, err)
	}
}

func writeEmpty(p string) error { return os.WriteFile(p, nil, 0o600) }
