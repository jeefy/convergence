package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/clientconfig"
	"github.com/jeefy/convergence/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure or restore external client applications to use Convergence",
}

type configFlags struct {
	gatewayURL string
	dryRun     bool
	force      bool
	setDefault bool
}

func (f configFlags) options() (clientconfig.Options, error) {
	url := f.gatewayURL
	if url == "" {
		cfg, err := config.LoadConfig()
		if err != nil {
			return clientconfig.Options{}, err
		}
		url = fmt.Sprintf("http://localhost:%d", cfg.GatewayPort)
	}
	return clientconfig.Options{GatewayURL: url, SetDefault: f.setDefault, DryRun: f.dryRun, Force: f.force}, nil
}

func runClient(cmd *cobra.Command, name string, args []string, f configFlags) error {
	o, err := f.options()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	restore := len(args) > 0 && args[0] == "restore"
	if len(args) > 0 && !restore {
		return fmt.Errorf("unknown argument %q (expected: restore)", args[0])
	}
	var res clientconfig.Result
	if restore {
		res, err = clientconfig.Restore(name, o)
	} else {
		res, err = clientconfig.Configure(name, o)
	}
	if err != nil {
		return err
	}
	switch {
	case !res.Changed:
		_, _ = fmt.Fprintf(out, "%s: already up to date (%s)\n", name, res.Path)
	case o.DryRun:
		_, _ = fmt.Fprintf(out, "[dry-run] no files written\n%s", res.Diff)
	case restore:
		_, _ = fmt.Fprintf(out, "%s: restored %s\n", name, res.Path)
	default:
		_, _ = fmt.Fprintf(out, "%s: configured %s -> %s\n", name, res.Path, clientconfig.Root(o.GatewayURL))
		_, _ = fmt.Fprintf(out, "Undo with: convergence config %s restore\n", name)
	}
	if res.Note != "" && !restore {
		_, _ = fmt.Fprintln(out, res.Note)
	}
	return nil
}

func addClientCmd(name, short string, extra func(*cobra.Command, *configFlags), post func(*cobra.Command)) {
	var f configFlags
	cmd := &cobra.Command{
		Use:   name + " [restore]",
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runClient(cmd, name, args, f); err != nil {
				return err
			}
			if post != nil && len(args) == 0 && !f.dryRun {
				post(cmd)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&f.gatewayURL, "gateway-url", "", "Gateway root URL (default http://localhost:<gateway_port>)")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "Print the diff without writing anything")
	cmd.Flags().BoolVar(&f.force, "force", false, "On restore, revert only Convergence keys even if the file changed since")
	if extra != nil {
		extra(cmd, &f)
	}
	configCmd.AddCommand(cmd)
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show every supported client, its config path and whether it points at Convergence",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sts, err := clientconfig.List()
		if err != nil {
			return err
		}
		for _, s := range sts {
			state := "missing"
			if s.Present {
				state = "present"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-15s %-8s convergence=%-5t %s\n", s.Client, state, s.Pointed, s.Path)
		}
		return nil
	},
}

var configEnvCmd = &cobra.Command{
	Use:     "copilot",
	Aliases: []string{"env"},
	Short:   "Print shell exports for OpenAI/Anthropic-style tools (Cursor has no file-based override; use these or its UI)",
	RunE: func(cmd *cobra.Command, args []string) error {
		o, err := configFlags{}.options()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprint(cmd.OutOrStdout(), clientconfig.EnvExports(o.GatewayURL))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configListCmd, configEnvCmd)
	addClientCmd("opencode", "Add a `convergence` provider to opencode.json (other keys untouched)",
		func(c *cobra.Command, f *configFlags) {
			c.Flags().BoolVar(&f.setDefault, "set-default", false, "Also set the default model to convergence/auto")
		}, nil)
	addClientCmd("oh-my-opencode", "Ensure the OpenCode `convergence` provider exists and print oh-my-opencode guidance",
		nil, func(cmd *cobra.Command) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(`
oh-my-opencode reads models from OpenCode providers. omo.jsonc was not modified.
To use Convergence, set the "model" of the agents/categories you want in your
omo.jsonc to "convergence/auto" or "convergence/pinball".`))
		})
	addClientCmd("openclaw", "Add a `convergence` provider to ~/.openclaw/openclaw.json (other keys untouched)",
		func(c *cobra.Command, f *configFlags) {
			c.Flags().BoolVar(&f.setDefault, "set-default", false, "Also set agents.defaults.model.primary to convergence/auto")
		}, nil)
	addClientCmd("claude-code", "Point Claude Code (~/.claude/settings.json env block) at Convergence", nil, nil)
	addClientCmd("aider", "Point Aider (~/.aider.conf.yml) at Convergence", nil, nil)
	addClientCmd("hermes", "Add a `convergence` provider to ~/.hermes/config.yaml",
		func(c *cobra.Command, f *configFlags) {
			c.Flags().BoolVar(&f.setDefault, "set-default", false, "Also make custom:convergence / auto the default model")
		}, nil)
	addClientCmd("goose", "Point Goose (~/.config/goose/config.yaml) at Convergence", nil, nil)
	addClientCmd("continue", "Add Convergence to Continue (~/.continue/config.yaml)", nil, nil)
	addClientCmd("zed", "Point Zed (~/.config/zed/settings.json) at Convergence", nil, nil)
	addClientCmd("cline", "Point Cline/Roo Code (~/.config/Code/User/settings.json) at Convergence", nil, nil)
	rootCmd.AddCommand(configCmd)
}
