package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ironsh/irons/api"
	"github.com/ironsh/irons/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yarlson/tap"
	"golang.org/x/term"
)

// banner lines split into "iron" (left) and ".sh" (right) portions.
// The split point is after the "N" block characters.
var bannerLeft = []string{
	"  ██╗██████╗  ██████╗ ███╗   ██╗",
	"  ██║██╔══██╗██╔═══██╗████╗  ██║",
	"  ██║██████╔╝██║   ██║██╔██╗ ██║",
	"  ██║██╔══██╗██║   ██║██║╚██╗██║",
	"  ██║██║  ██║╚██████╔╝██║ ╚████║",
	"  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝",
}

var bannerRight = []string{
	"   ███████╗██╗  ██╗",
	"   ██╔════╝██║  ██║",
	"   ███████╗███████║",
	"   ╚════██║██╔══██║",
	"██╗███████║██║  ██║",
	"╚═╝╚══════╝╚═╝  ╚═╝",
}

// ANSI color codes
const (
	boldWhite  = "\033[1;37m"
	orange     = "\033[38;2;255;107;53m" // #ff6b35
	reset      = "\033[0m"
	dim        = "\033[2m"
)

func printBanner() {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}
	fmt.Println()
	for i := range bannerLeft {
		fmt.Printf("%s%s%s%s%s\n", boldWhite, bannerLeft[i], orange, bannerRight[i], reset)
	}
	fmt.Println()
}

// claudeCredentials represents the OAuth credentials stored by Claude Code.
type claudeCredentials struct {
	ClaudeAIOAuth *claudeOAuth `json:"claudeAiOauth,omitempty"`
}

type claudeOAuth struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"`
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType,omitempty"`
	RateLimitTier    string   `json:"rateLimitTier,omitempty"`
}

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Set up your iron.sh account and credentials",
	Long: `First-run setup flow that ensures you have an iron.sh account,
GitHub credentials, agent provider credentials, and an SSH key registered.

This command is idempotent — running it multiple times is safe. Each step
checks whether it's already been completed before doing anything.

Use --refresh to re-pull local credentials and overwrite what's stored.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		refresh, _ := cmd.Flags().GetBool("refresh")
		return runOnboard(ctx, refresh, true)
	},
}

func init() {
	rootCmd.AddCommand(onboardCmd)
	onboardCmd.Flags().Bool("refresh", false, "Re-pull local credentials and overwrite what's stored")
}

// runOnboard is the standalone onboarding logic, callable from both the cobra
// command and from other code (e.g. agents new credential check).
// When promptAgent is true, the user is prompted to start their first agent.
func runOnboard(ctx context.Context, refresh, promptAgent bool) error {
	printBanner()

	// Step 1: Account
	if err := onboardAccount(ctx); err != nil {
		return err
	}

	// Build client now that we have an API key.
	client := newClient()

	// Step 2: GitHub token
	if err := onboardGitHub(ctx, client, refresh); err != nil {
		return err
	}

	// Step 3: Agent provider
	if err := onboardAgentProvider(ctx, client, refresh); err != nil {
		return err
	}

	// Step 4: SSH key
	if err := onboardSSHKey(client); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("%sYour credentials are encrypted at rest and injected into your VM%s\n", dim, reset)
	fmt.Printf("%svia iron.sh's secrets proxy. They never touch disk in plaintext.%s\n", dim, reset)
	fmt.Printf("%sAll VM network traffic is logged and restricted by default.%s\n", dim, reset)
	fmt.Println()

	if promptAgent {
		// Step 5: Prompt to create first agent.
		if err := promptFirstAgent(ctx); err != nil {
			// Non-fatal: user can always run agents new later.
			fmt.Printf("Run 'irons agents new --repo <url>' when you're ready.\n")
		}
	}

	return nil
}

// onboardAccount handles Step 1: iron.sh account authentication.
func onboardAccount(ctx context.Context) error {
	fmt.Println("Account")

	if isAuthenticated() {
		fmt.Println("  \u2713 Authenticated.")
		fmt.Println()
		return nil
	}

	fmt.Println("  No iron.sh account found.")
	fmt.Println()

	choice := tap.Select(ctx, tap.SelectOptions[int]{
		Message: "Choose an option",
		Options: []tap.SelectOption[int]{
			{Value: 1, Label: "Create account"},
			{Value: 2, Label: "Log in (opens browser)"},
		},
	})

	switch choice {
	case 1:
		return onboardCreateAccount(ctx)
	case 2:
		return onboardLogin(ctx)
	}

	return nil
}

func onboardCreateAccount(ctx context.Context) error {
	firstName := tap.Text(ctx, tap.TextOptions{
		Message: "First name",
		Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("first name is required")
			}
			return nil
		},
	})

	lastName := tap.Text(ctx, tap.TextOptions{
		Message: "Last name",
		Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("last name is required")
			}
			return nil
		},
	})

	email := tap.Text(ctx, tap.TextOptions{
		Message: "Email",
		Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("email is required")
			}
			if !strings.Contains(s, "@") {
				return fmt.Errorf("invalid email address")
			}
			return nil
		},
	})

	var password string
	for {
		password = tap.Password(ctx, tap.PasswordOptions{
			Message: "Password",
			Validate: func(s string) error {
				if len(s) < 8 {
					return fmt.Errorf("password must be at least 8 characters")
				}
				return nil
			},
		})

		confirm := tap.Password(ctx, tap.PasswordOptions{
			Message: "Confirm password",
		})

		if password == confirm {
			break
		}
		fmt.Println("  Passwords don't match.")
	}

	client := newClient()
	resp, err := client.CreateUser(api.CreateUserRequest{
		FirstName: strings.TrimSpace(firstName),
		LastName:  strings.TrimSpace(lastName),
		Email:     strings.TrimSpace(email),
		Password:  password,
	})
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "email_taken" {
			fmt.Println("  An account with this email already exists. Choose [2] to log in instead.")
			fmt.Println()
			return onboardAccount(context.Background())
		}
		return fmt.Errorf("creating account: %w", err)
	}

	if err := config.SetAPIKey(resp.Token); err != nil {
		return fmt.Errorf("saving API key: %w", err)
	}
	viper.Set("api-key", resp.Token)

	fmt.Println()
	fmt.Println("  \u2713 Account created. You're logged in.")
	fmt.Println()
	return nil
}

func onboardLogin(ctx context.Context) error {
	client := newClient()

	fmt.Println()
	fmt.Println("  Requesting device code...")
	codeResp, err := client.DeviceCode()
	if err != nil {
		return fmt.Errorf("requesting device code: %w", err)
	}

	fmt.Printf("\n  Open the following URL in your browser to authenticate:\n\n    %s\n\n", codeResp.VerificationURI)
	fmt.Printf("  Paste in the following device code: %s\n\n", codeResp.Code)
	fmt.Printf("  This code expires at %s.\n\n", codeResp.ExpiresAt.Local().Format(time.RFC1123))
	fmt.Println("  Waiting for authorization...")

	deadline := time.Now().Add(pollTimeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "\n  Login cancelled.")
			return fmt.Errorf("login cancelled")

		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for authorization")
			}

			pollResp, err := client.PollDevice(codeResp.Code)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  warning: poll error (retrying): %v\n", err)
				continue
			}

			switch pollResp.Status {
			case "authorized":
				if err := config.SetAPIKey(pollResp.Token); err != nil {
					return fmt.Errorf("saving token: %w", err)
				}
				viper.Set("api-key", pollResp.Token)
				fmt.Println()
				fmt.Println("  \u2713 Authorized! You're logged in.")
				fmt.Println()
				return nil

			case "expired":
				return fmt.Errorf("device code expired — please run `irons onboard` again")

			case "pending":
				// Still waiting.

			default:
				return fmt.Errorf("unexpected poll status %q", pollResp.Status)
			}
		}
	}
}

// onboardGitHub handles Step 2: GitHub token.
func onboardGitHub(ctx context.Context, client *api.Client, refresh bool) error {
	fmt.Println("GitHub")

	if !refresh {
		resp, err := client.SecretsListByName(SecretGitHubAgent)
		if err != nil {
			return fmt.Errorf("checking github-agent secret: %w", err)
		}
		if len(resp.Data) > 0 {
			fmt.Println("  \u2713 Already configured.")
			fmt.Println()
			return nil
		}
	}

	// Try to find a token from local sources.
	var token, source string

	// 1. gh auth token
	if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		t := strings.TrimSpace(string(out))
		if t != "" {
			token = t
			source = "`gh auth token`"
		}
	}

	// 2. $GITHUB_TOKEN
	if token == "" {
		if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			token = t
			source = "$GITHUB_TOKEN"
		}
	}

	// 3. $GH_TOKEN
	if token == "" {
		if t := os.Getenv("GH_TOKEN"); t != "" {
			token = t
			source = "$GH_TOKEN"
		}
	}

	if token != "" {
		fmt.Printf("  Found token from %s.\n", source)
		fmt.Println()

		choice := tap.Select(ctx, tap.SelectOptions[int]{
			Message: "Choose an option",
			Options: []tap.SelectOption[int]{
				{Value: 1, Label: "Use this token"},
				{Value: 2, Label: "Paste a different token"},
			},
		})

		if choice == 2 {
			token = ""
		}
	}

	if token == "" {
		token = tap.Password(ctx, tap.PasswordOptions{
			Message: "Paste a GitHub personal access token with `repo` scope",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("token is required")
				}
				return nil
			},
		})
	}

	fmt.Println()
	fmt.Println("  Storing as GITHUB_TOKEN in iron.sh secret store.")

	if err := storeSecret(client, SecretGitHubAgent, "GITHUB_TOKEN", strings.TrimSpace(token), refresh); err != nil {
		return err
	}

	fmt.Println("  \u2713 Stored.")
	fmt.Println()
	return nil
}

// onboardAgentProvider handles Step 3: Agent provider selection and credential storage.
func onboardAgentProvider(ctx context.Context, client *api.Client, refresh bool) error {
	fmt.Println("Agent Provider")

	for {
		choice := tap.Select(ctx, tap.SelectOptions[int]{
			Message: "Which agent provider do you want to use?",
			Options: []tap.SelectOption[int]{
				{Value: 1, Label: "Claude Code"},
				{Value: 2, Label: "Codex (coming soon)"},
			},
		})

		if choice == 2 {
			fmt.Println("  Codex support is coming soon. Choose Claude Code for now.")
			fmt.Println()
			continue
		}
		break
	}

	// Save the harness choice to config.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	cfg.Harness = "claude"
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return onboardClaudeCode(ctx, client, refresh)
}

// Secret names used during onboarding and credential checks.
const (
	SecretGitHubAgent    = "agent-github-agent-token"
	SecretClaudeOAuthAcc = "agent-claude-oauth-access-token"
	SecretClaudeOAuthRef = "agent-claude-oauth-refresh-token"
	SecretClaudeAPIKey   = "agent-claude-api-key"
)

func onboardClaudeCode(ctx context.Context, client *api.Client, refresh bool) error {
	fmt.Println()
	fmt.Println("Claude Code")

	if !refresh {
		if claudeCodeSecretsExist(client) {
			fmt.Println("  \u2713 Already configured.")
			fmt.Println()
			return nil
		}
	}

	// Collect all available credentials from local sources.
	type claudeCandidate struct {
		oauth  *claudeOAuth // non-nil for OAuth credentials
		apiKey string       // non-empty for API key
		source string       // human-readable source
	}
	var candidates []claudeCandidate

	// 1. Claude Code OAuth (keychain on macOS, file on Linux).
	if creds, source, err := findClaudeOAuthCredentials(); err == nil && creds.ClaudeAIOAuth != nil {
		oauth := creds.ClaudeAIOAuth
		if oauth.AccessToken != "" && time.Now().UnixMilli() < oauth.ExpiresAt {
			candidates = append(candidates, claudeCandidate{
				oauth:  oauth,
				source: source,
			})
		}
	}

	// 2. $ANTHROPIC_API_KEY
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		candidates = append(candidates, claudeCandidate{
			apiKey: key,
			source: "$ANTHROPIC_API_KEY",
		})
	}

	// 3. $CLAUDE_CODE_OAUTH_TOKEN
	if token := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"); token != "" {
		// This env var contains just the access token, no refresh token.
		candidates = append(candidates, claudeCandidate{
			apiKey: token,
			source: "$CLAUDE_CODE_OAUTH_TOKEN",
		})
	}

	var selected *claudeCandidate

	if len(candidates) > 0 {
		const (
			choicePasteKey    = -1
			choiceClaudeLogin = -2
			choiceSetUpLater  = -3
		)
		var options []tap.SelectOption[int]
		for i, c := range candidates {
			label := fmt.Sprintf("Use credentials from %s", c.source)
			if c.oauth != nil {
				label = fmt.Sprintf("Use OAuth credentials from %s", c.source)
			}
			options = append(options, tap.SelectOption[int]{Value: i, Label: label})
		}
		options = append(options,
			tap.SelectOption[int]{Value: choicePasteKey, Label: "Paste an API key instead"},
			tap.SelectOption[int]{Value: choiceClaudeLogin, Label: "Log in with Claude Code"},
			tap.SelectOption[int]{Value: choiceSetUpLater, Label: "Set up later"},
		)

		choice := tap.Select(ctx, tap.SelectOptions[int]{
			Message: "Choose credentials",
			Options: options,
		})

		switch {
		case choice >= 0:
			selected = &candidates[choice]
		case choice == choiceClaudeLogin:
			oauth, err := runClaudeLogin()
			if err != nil {
				return err
			}
			selected = &claudeCandidate{oauth: oauth, source: "Claude Code login"}
		case choice == choiceSetUpLater:
			fmt.Println()
			fmt.Println("  ⚠ Warning: Claude credentials won't be proxied to agents by default.")
			fmt.Println("  Run `irons onboard --refresh` later to configure them.")
			fmt.Println()
			return nil
		}
	} else {
		fmt.Println("  No credentials found.")
		fmt.Println()

		choice := tap.Select(ctx, tap.SelectOptions[int]{
			Message: "Choose an option",
			Options: []tap.SelectOption[int]{
				{Value: 1, Label: "Paste an API key"},
				{Value: 2, Label: "Log in with Claude Code"},
				{Value: 3, Label: "Set up later"},
			},
		})

		switch choice {
		case 2:
			oauth, err := runClaudeLogin()
			if err != nil {
				return err
			}
			selected = &claudeCandidate{oauth: oauth, source: "Claude Code login"}
		case 3:
			fmt.Println()
			fmt.Println("  ⚠ Warning: Claude credentials won't be proxied to agents by default.")
			fmt.Println("  Run `irons onboard --refresh` later to configure them.")
			fmt.Println()
			return nil
		}
	}

	// If still no creds, prompt for API key.
	if selected == nil {
		key := tap.Password(ctx, tap.PasswordOptions{
			Message: "Paste an Anthropic API key",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("API key is required")
				}
				return nil
			},
		})
		selected = &claudeCandidate{apiKey: strings.TrimSpace(key), source: "manual"}
	}

	fmt.Println()
	fmt.Println("  Storing in iron.sh secret store.")

	if selected.oauth != nil {
		// Store access and refresh tokens as separate secrets (proxy-swappable).
		if err := storeSecret(client, SecretClaudeOAuthAcc, "CLAUDE_OAUTH_ACCESS_TOKEN", selected.oauth.AccessToken, refresh); err != nil {
			return err
		}
		if err := storeSecret(client, SecretClaudeOAuthRef, "CLAUDE_OAUTH_REFRESH_TOKEN", selected.oauth.RefreshToken, refresh); err != nil {
			return err
		}

		// Store non-sensitive OAuth metadata as an env var.
		metadata, _ := json.Marshal(map[string]any{
			"expiresAt":        selected.oauth.ExpiresAt,
			"scopes":           selected.oauth.Scopes,
			"subscriptionType": selected.oauth.SubscriptionType,
			"rateLimitTier":    selected.oauth.RateLimitTier,
		})
		if _, err := client.EnvVarPut("CLAUDE_OAUTH_METADATA", string(metadata)); err != nil {
			return fmt.Errorf("storing OAuth metadata: %w", err)
		}
	} else {
		if err := storeSecret(client, SecretClaudeAPIKey, "ANTHROPIC_API_KEY", selected.apiKey, refresh); err != nil {
			return err
		}
	}

	fmt.Println("  \u2713 Stored.")
	fmt.Println()
	return nil
}

// claudeCodeSecretsExist returns true if any Claude Code secrets are already
// configured (either an API key secret or OAuth token secrets).
func claudeCodeSecretsExist(client *api.Client) bool {
	for _, name := range []string{SecretClaudeAPIKey, SecretClaudeOAuthAcc} {
		resp, err := client.SecretsListByName(name)
		if err == nil && len(resp.Data) > 0 {
			return true
		}
	}
	return false
}

// findClaudeOAuthCredentials reads Claude Code OAuth credentials from the
// platform-appropriate store. On macOS it queries the Keychain; on Linux it
// reads ~/.claude/.credentials.json (or $CLAUDE_CONFIG_DIR/.credentials.json).
// Returns the parsed credentials and a human-readable source string.
func findClaudeOAuthCredentials() (*claudeCredentials, string, error) {
	if runtime.GOOS == "darwin" {
		return readClaudeCredentialsFromKeychain()
	}
	return readClaudeCredentialsFromDisk()
}

func readClaudeCredentialsFromKeychain() (*claudeCredentials, string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", "Claude Code-credentials",
		"-w",
	).Output()
	if err != nil {
		return nil, "", fmt.Errorf("reading keychain: %w", err)
	}

	var creds claudeCredentials
	if err := json.Unmarshal(out, &creds); err != nil {
		return nil, "", fmt.Errorf("parsing keychain data: %w", err)
	}
	return &creds, "macOS Keychain", nil
}

func readClaudeCredentialsFromDisk() (*claudeCredentials, string, error) {
	path := claudeCredentialsPath()
	if path == "" {
		return nil, "", fmt.Errorf("could not determine credentials path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var creds claudeCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, "", err
	}
	return &creds, path, nil
}

func claudeCredentialsPath() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, ".credentials.json")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".claude", ".credentials.json")
}

func runClaudeLogin() (*claudeOAuth, error) {
	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("Claude Code CLI not found. Install it with `npm install -g @anthropic-ai/claude-code`, then run `irons onboard` again")
	}

	fmt.Println()
	fmt.Println("  Launching Claude Code login...")
	fmt.Println()

	cmd := exec.Command(claudeBin, "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude auth login failed: %w", err)
	}

	fmt.Println()
	fmt.Println("  \u2713 Claude Code authenticated.")

	// Re-read credentials from the platform-appropriate store.
	creds, _, err := findClaudeOAuthCredentials()
	if err != nil {
		return nil, fmt.Errorf("reading Claude Code credentials after login: %w", err)
	}
	if creds.ClaudeAIOAuth == nil || creds.ClaudeAIOAuth.AccessToken == "" {
		return nil, fmt.Errorf("no OAuth credentials found after Claude Code login")
	}

	return creds.ClaudeAIOAuth, nil
}

// onboardSSHKey handles Step 4: SSH key registration.
func onboardSSHKey(client *api.Client) error {
	fmt.Println("SSH Key")

	// Check if user already has keys registered.
	resp, err := client.PublicKeysList()
	if err != nil {
		return fmt.Errorf("listing public keys: %w", err)
	}
	if len(resp.Data) > 0 {
		fmt.Println("  \u2713 Already registered.")
		fmt.Println()
		return nil
	}

	// Auto-detect SSH key using the same logic as `irons create`.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determining home directory: %w", err)
	}

	candidates := []string{
		filepath.Join(homeDir, ".ssh", "id_ed25519.pub"),
		filepath.Join(homeDir, ".ssh", "id_ed25519_sk.pub"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa.pub"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa_sk.pub"),
		filepath.Join(homeDir, ".ssh", "id_rsa.pub"),
	}

	var keyPath string
	for _, c := range candidates {
		if fileExists(c) {
			keyPath = c
			break
		}
	}

	if keyPath == "" {
		// Offer to generate an ed25519 key pair.
		fmt.Println("  No SSH keys found in ~/.ssh/")
		fmt.Println("  Generating an ed25519 key pair...")

		keyPath = filepath.Join(homeDir, ".ssh", "id_ed25519")
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("generating SSH key: %w", err)
		}
		keyPath = keyPath + ".pub"
		fmt.Printf("  Generated: %s\n", keyPath)
	} else {
		// Shorten path for display.
		display := keyPath
		if strings.HasPrefix(keyPath, homeDir) {
			display = "~" + keyPath[len(homeDir):]
		}
		fmt.Printf("  Found: %s\n", display)
	}

	keyContent, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("reading SSH key %s: %w", keyPath, err)
	}

	// Derive a name from the key filename.
	name := strings.TrimSuffix(filepath.Base(keyPath), ".pub")

	_, err = client.PublicKeysCreate(api.CreatePublicKeyRequest{
		Name:      name,
		PublicKey: strings.TrimSpace(string(keyContent)),
	})
	if err != nil {
		return fmt.Errorf("registering SSH key: %w", err)
	}

	fmt.Println("  \u2713 Registered.")
	fmt.Println()
	return nil
}

// storeSecret creates or updates a secret. When refresh is true and the secret
// already exists, it updates via PATCH. Otherwise it creates via POST.
func storeSecret(client *api.Client, name, envVar, secret string, refresh bool) error {
	if refresh {
		// Check if it exists so we can PATCH.
		resp, err := client.SecretsListByName(name)
		if err != nil {
			return fmt.Errorf("checking existing secret %q: %w", name, err)
		}
		if len(resp.Data) > 0 {
			_, err := client.SecretsUpdate(resp.Data[0].ID, api.UpdateSecretRequest{
				Secret: secret,
				EnvVar: envVar,
			})
			if err != nil {
				return fmt.Errorf("updating secret %q: %w", name, err)
			}
			return nil
		}
	}

	_, err := client.SecretsCreate(api.CreateSecretRequest{
		Name:   name,
		Secret: secret,
		EnvVar: envVar,
	})
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "secret_name_taken" {
			// Already exists — offer to update.
			resp, listErr := client.SecretsListByName(name)
			if listErr != nil {
				return fmt.Errorf("creating secret %q: %w", name, err)
			}
			if len(resp.Data) > 0 {
				_, updateErr := client.SecretsUpdate(resp.Data[0].ID, api.UpdateSecretRequest{
					Secret: secret,
					EnvVar: envVar,
				})
				if updateErr != nil {
					return fmt.Errorf("updating existing secret %q: %w", name, updateErr)
				}
				return nil
			}
		}
		return fmt.Errorf("creating secret %q: %w", name, err)
	}
	return nil
}

// promptFirstAgent offers the user a chance to start their first agent session
// right after onboarding completes.
func promptFirstAgent(ctx context.Context) error {
	// Fetch repos using the GitHub token that was just stored.
	repos, err := fetchGitHubRepos()
	if err != nil || len(repos) == 0 {
		// Can't fetch repos — just show the manual command.
		return fmt.Errorf("skipping repo prompt")
	}

	fmt.Println("Want to start an agent now?")
	fmt.Println()

	const otherValue = "__other__"

	limit := min(len(repos), 10)
	var options []tap.SelectOption[string]
	for _, repo := range repos[:limit] {
		options = append(options, tap.SelectOption[string]{Value: repo, Label: repo})
	}
	options = append(options, tap.SelectOption[string]{Value: otherValue, Label: "Other (enter repo URL)"})

	choice := tap.Select(ctx, tap.SelectOptions[string]{
		Message: "Pick a repo",
		Options: options,
	})

	repo := choice
	if choice == otherValue {
		repo = tap.Text(ctx, tap.TextOptions{
			Message: "Repo URL",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("repo URL is required")
				}
				return nil
			},
		})
		repo = strings.TrimSpace(repo)
	}

	fmt.Println()

	return runAgentsNew(ctx, agentsNewOpts{Repo: repo})
}

// fetchGitHubRepos fetches the user's GitHub repos using the gh CLI.
// Returns repo full names sorted by most recently pushed.
func fetchGitHubRepos() ([]string, error) {
	ghBin, err := exec.LookPath("gh")
	if err != nil {
		return nil, fmt.Errorf("gh CLI not found")
	}

	cmd := exec.Command(ghBin, "api", "user/repos",
		"--jq", "sort_by(.pushed_at) | reverse | .[].full_name",
		"--paginate",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fetching repos: %w", err)
	}

	var repos []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			repos = append(repos, line)
		}
	}

	return repos, nil
}
