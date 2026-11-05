package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/templates"
)

// GitHub OAuth app used by ericc-ch/copilot-api's own `auth` command
// (client id and scope extracted from dist/main.js of image v0.7.0). The
// resulting token is exactly what copilot-api expects in GH_TOKEN.
const (
	copilotClientID   = "Iv1.b507a08c87ecfe98"
	copilotScope      = "read:user"
	deviceGrantType   = "urn:ietf:params:oauth:grant-type:device_code" //nolint:gosec // G101 false positive: OAuth grant-type URN
	deviceHTTPTimeout = 15 * time.Second
)

// Endpoints and minimum poll interval are vars so tests can point at httptest.
var (
	githubDeviceURL  = "https://github.com/login/device/code"
	githubTokenURL   = "https://github.com/login/oauth/access_token" //nolint:gosec // G101 false positive: public OAuth endpoint URL
	deviceMinPollSec = 5
)

type deviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
	Interval    int    `json:"interval"`
}

func postForm(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: deviceHTTPTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }() // read-only response body
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s: status %d", endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s response: %w", endpoint, err)
	}
	return nil
}

// githubDeviceLogin runs the OAuth device flow and returns the access token.
func githubDeviceLogin(ctx context.Context) (string, error) {
	var dc deviceCode
	if err := postForm(ctx, githubDeviceURL, url.Values{"client_id": {copilotClientID}, "scope": {copilotScope}}, &dc); err != nil {
		return "", err
	}
	fmt.Printf("Open %s and enter the code: %s\nWaiting for authorization...\n", dc.VerificationURI, dc.UserCode)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(dc.ExpiresIn)*time.Second)
	defer cancel()
	interval := time.Duration(max(dc.Interval, deviceMinPollSec)) * time.Second
	form := url.Values{"client_id": {copilotClientID}, "device_code": {dc.DeviceCode}, "grant_type": {deviceGrantType}}
	for {
		select {
		case <-ctx.Done():
			return "", errors.New("device code expired before authorization; run the command again")
		case <-time.After(interval):
		}
		var tr tokenResponse
		if err := postForm(ctx, githubTokenURL, form, &tr); err != nil {
			return "", err
		}
		switch tr.Error {
		case "":
			if tr.AccessToken == "" {
				return "", errors.New("GitHub returned neither a token nor an error")
			}
			return tr.AccessToken, nil
		case "authorization_pending":
		case "slow_down":
			interval = time.Duration(max(tr.Interval, int(interval/time.Second)+deviceMinPollSec)) * time.Second
		default:
			return "", fmt.Errorf("GitHub device flow failed: %s (%s)", tr.Error, tr.Description)
		}
	}
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate upstream providers",
}

var loginCopilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Log in to GitHub Copilot via the GitHub device flow and save the token",
	Long: `Runs the same GitHub device flow as 'copilot-api auth' and stores the
resulting token in ~/.convergence/config.yaml (0600) and the 0600 .env file.
Requires an active GitHub Copilot subscription. Run 'convergence restart' afterwards
if the stack is already running.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		token, err := githubDeviceLogin(cmd.Context())
		if err != nil {
			return err
		}
		cfg.Provider = config.ProviderCopilot
		cfg.CopilotToken = token
		if err := config.SaveConfig(cfg); err != nil {
			return err
		}
		dir, err := config.GetConfigDir()
		if err != nil {
			return err
		}
		if err := templates.RenderAssets(cfg, dir); err != nil {
			return fmt.Errorf("rendering assets: %w", err)
		}
		fmt.Println("Copilot token saved; provider set to copilot. Run 'convergence start' (or 'restart').")
		return nil
	},
}

func init() {
	loginCmd.AddCommand(loginCopilotCmd)
	rootCmd.AddCommand(loginCmd)
}
