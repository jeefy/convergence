package cli

import "testing"

func TestClusterFlags(t *testing.T) {
	f := clusterInstallCmd.Flags()
	if v, _ := f.GetBool("fail-open"); v {
		t.Error("fail-open must default false")
	}
	for _, n := range []string{"models-file", "openai-base-url", "fail-open"} {
		if f.Lookup(n) == nil {
			t.Errorf("missing flag %s", n)
		}
	}
	if !f.Lookup("openai-base-url").Hidden {
		t.Error("openai-base-url should be hidden")
	}
}

func TestUninstallRequiresYes(t *testing.T) {
	_ = clusterUninstallCmd.Flags().Set("yes", "false")
	if err := clusterUninstallCmd.RunE(clusterUninstallCmd, nil); err == nil {
		t.Error("uninstall without --yes must refuse")
	}
}
