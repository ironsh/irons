package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ironsh/irons/api"
	"github.com/ironsh/irons/config"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// agentsCmd is the parent command for agent subcommands.
var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agent sessions",
	Long: `Create, attach to, list, and destroy agent sessions.

An agent session boots a VM, clones a repository, and starts an AI coding
agent inside a tmux session that you can attach to via SSH.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// agentsNewCmd creates a new agent session.
var agentsNewCmd = &cobra.Command{
	Use:   "new [flags] [-- agent-args...]",
	Short: "Create a new agent session",
	Long: `Create an agent session: boot a VM, clone a repo, start the agent in tmux, and SSH in.

Everything after -- is passed through to the agent process as arguments.

Examples:
  irons agents new --repo acme/api
  irons agents new --repo github.com/acme/api --name fix-auth
  irons agents new --repo acme/api --prompt "fix the failing auth tests"
  irons agents new --repo acme/api --prompt-file ./task.md
  irons agents new --repo acme/api -- --remote
  irons agents new --repo acme/api --no-attach`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		client := newClient()

		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		prompt, _ := cmd.Flags().GetString("prompt")
		promptFile, _ := cmd.Flags().GetString("prompt-file")
		name, _ := cmd.Flags().GetString("name")
		noAttach, _ := cmd.Flags().GetBool("no-attach")

		// Capture agent args (everything after --)
		agentArgs := cmd.ArgsLenAtDash()
		var extraArgs []string
		if agentArgs >= 0 {
			extraArgs = args[agentArgs:]
		}

		// Step 1: Check onboarding credentials.
		if err := checkOnboardingSecrets(client); err != nil {
			return err
		}

		// Step 2: Resolve name from repo if not provided.
		if name == "" {
			name = deriveNameFromRepo(repo)
		}

		// Step 3: Read prompt file if provided.
		if promptFile != "" {
			data, err := os.ReadFile(promptFile)
			if err != nil {
				return fmt.Errorf("reading prompt file: %w", err)
			}
			prompt = string(data)
		}

		// Step 4: Determine harness from config.
		harness := "claude"
		if cfg, err := config.Load(); err == nil && cfg.Harness != "" {
			harness = cfg.Harness
		}

		// Step 5: Create agent (with retry on name collision).
		fmt.Println("✓ Credentials verified")

		req := api.CreateAgentRequest{
			Name:      name,
			Repo:      repo,
			Branch:    branch,
			Harness:   harness,
			Prompt:    prompt,
			AgentArgs: extraArgs,
		}

		agent, err := createAgentWithRetry(client, req)
		if err != nil {
			return fmt.Errorf("creating agent: %w", err)
		}

		// Step 6: Wait for agent to be running.
		agent, err = waitForAgent(ctx, client, agent.ID)
		if err != nil {
			return err
		}

		fmt.Println("✓ VM ready")

		if noAttach {
			fmt.Println()
			fmt.Printf("Session: %s\n", agent.Name)
			fmt.Printf("Attach:  irons agents attach %s\n", agent.Name)
			return nil
		}

		// Step 7: Attach via SSH + tmux.
		fmt.Println("  Connecting...")
		return sshAttachToAgent(client, agent)
	},
}

// agentsAttachCmd reattaches to an agent's tmux session.
var agentsAttachCmd = &cobra.Command{
	Use:   "attach [name|id]",
	Short: "Attach to an agent session",
	Long: `Reattach to an agent's tmux session via SSH.

If no argument is given and exactly one agent is active, attaches to it.
If multiple agents are active, prints the list and prompts for selection.

Examples:
  irons agents attach fix-auth
  irons agents attach agt_k3mf9xvw2p
  irons agents attach`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		var agent *api.Agent

		if len(args) == 1 {
			// Resolve by name or ID.
			a, err := resolveAgentFull(client, args[0])
			if err != nil {
				return err
			}
			agent = a
		} else {
			// No argument: pick from active agents.
			a, err := pickAgent(client)
			if err != nil {
				return err
			}
			agent = a
		}

		return sshAttachToAgent(client, agent)
	},
}

// agentsListCmd lists active agent sessions.
var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent sessions",
	Long: `List all active agent sessions.

Examples:
  irons agents list`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		resp, err := client.AgentsList()
		if err != nil {
			return fmt.Errorf("listing agents: %w", err)
		}

		if len(resp.Data) == 0 {
			fmt.Println("No active agents.")
			return nil
		}

		printAgentsTable(resp.Data)
		return nil
	},
}

// agentsDestroyCmd destroys an agent session.
var agentsDestroyCmd = &cobra.Command{
	Use:   "destroy <name|id>",
	Short: "Destroy an agent session",
	Long: `Destroy an agent session and its VM.

Examples:
  irons agents destroy fix-auth
  irons agents destroy agt_k3mf9xvw2p`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		idOrName := args[0]

		// Resolve to get both ID and name for display.
		agent, err := resolveAgentFull(client, idOrName)
		if err != nil {
			return err
		}

		if err := client.AgentsDestroy(agent.ID); err != nil {
			return fmt.Errorf("destroying agent: %w", err)
		}

		fmt.Printf("Destroyed %s.\n", agent.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsNewCmd)
	agentsCmd.AddCommand(agentsAttachCmd)
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsDestroyCmd)

	// Flags for agents new
	agentsNewCmd.Flags().String("repo", "", "GitHub repo (e.g. acme/api or github.com/acme/api)")
	agentsNewCmd.Flags().String("branch", "", "Branch to checkout (defaults to repo default)")
	agentsNewCmd.Flags().String("prompt", "", "Initial task for the agent")
	agentsNewCmd.Flags().String("prompt-file", "", "Path to file containing initial prompt")
	agentsNewCmd.Flags().String("name", "", "Session name (derived from repo if omitted)")
	agentsNewCmd.Flags().Bool("no-attach", false, "Create the session but don't SSH in")
	agentsNewCmd.MarkFlagRequired("repo")
}

// deriveNameFromRepo extracts a session name from a repo URL.
// "github.com/acme/my-app" → "my-app", "acme/my-app" → "my-app"
func deriveNameFromRepo(repo string) string {
	// Strip .git suffix
	repo = strings.TrimSuffix(repo, ".git")
	// Take the last path segment
	name := path.Base(repo)
	if name == "" || name == "." || name == "/" {
		return "agent"
	}
	return name
}

// checkOnboardingSecrets verifies the user has the required secrets for agent
// creation. If any are missing, it runs the onboard flow.
func checkOnboardingSecrets(client *api.Client) error {
	resp, err := client.SecretsList()
	if err != nil {
		return fmt.Errorf("checking secrets: %w", err)
	}

	secretNames := make(map[string]bool)
	for _, s := range resp.Data {
		secretNames[s.Name] = true
	}

	hasGitHub := secretNames["github-agent"]
	hasClaude := secretNames[secretClaudeAgent] ||
		(secretNames[secretClaudeOAuthAccess] && secretNames[secretClaudeOAuthRefresh])

	if hasGitHub && hasClaude {
		return nil
	}

	fmt.Println("Missing required credentials. Running setup...")
	fmt.Println()

	// Run the onboard flow by executing the binary's onboard command.
	bin, _ := os.Executable()
	cmd := exec.Command(bin, "onboard")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// createAgentWithRetry attempts to create an agent, retrying with a
// counter suffix on name collisions (409 agent_name_taken).
func createAgentWithRetry(client *api.Client, req api.CreateAgentRequest) (*api.Agent, error) {
	baseName := req.Name

	for attempt := range 10 {
		agent, err := client.AgentsCreate(req)
		if err == nil {
			if req.Name != baseName {
				fmt.Printf("Name %q is taken, using %q instead.\n", baseName, req.Name)
			}
			return agent, nil
		}

		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "agent_name_taken" {
			req.Name = fmt.Sprintf("%s-%d", baseName, attempt+2)
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("could not find an available name after 10 attempts")
}

// waitForAgent polls until the agent status is "running" or a terminal state.
func waitForAgent(ctx context.Context, client *api.Client, id string) (*api.Agent, error) {
	deadline := time.Now().Add(pollTimeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	fmt.Print("⠋ Booting VM...")

	for {
		if time.Now().After(deadline) {
			fmt.Println()
			return nil, fmt.Errorf("timed out waiting for agent %q", id)
		}

		agent, err := client.AgentsGet(id)
		if err != nil {
			fmt.Print(".")
		} else if agent.Status == "running" {
			fmt.Println()
			return agent, nil
		} else if agent.Status == "failed" {
			fmt.Println()
			return nil, fmt.Errorf("agent %q entered failed state", id)
		} else {
			fmt.Print(".")
		}

		select {
		case <-ctx.Done():
			fmt.Println()
			return nil, fmt.Errorf("cancelled while waiting for agent %q: %w", id, ctx.Err())
		case <-ticker.C:
		}
	}
}

// sshAttachToAgent SSHes into the agent's VM and attaches to the tmux session.
// This reuses the same SSH connection logic as `irons ssh`.
func sshAttachToAgent(client *api.Client, agent *api.Agent) error {
	resp, err := client.SSH(agent.VMID)
	if err != nil {
		return fmt.Errorf("getting SSH info: %w", err)
	}

	sshArgs := []string{
		"-p", fmt.Sprintf("%d", resp.Port),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t",
		fmt.Sprintf("%s@%s", resp.Username, resp.Host),
		"tmux", "attach", "-t", "main",
	}

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("SSH command failed: %w", err)
	}

	return nil
}

// resolveAgentFull resolves a name or ID to the full Agent struct.
func resolveAgentFull(client *api.Client, idOrName string) (*api.Agent, error) {
	if strings.HasPrefix(idOrName, "agt_") {
		agent, err := client.AgentsGet(idOrName)
		if err != nil {
			return nil, fmt.Errorf("getting agent %q: %w", idOrName, err)
		}
		return agent, nil
	}

	resp, err := client.AgentsListByName(idOrName)
	if err != nil {
		return nil, fmt.Errorf("resolving agent name %q: %w", idOrName, err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no agent found with name %q", idOrName)
	}

	return &resp.Data[0], nil
}

// pickAgent handles the no-argument case for attach: select from active agents.
func pickAgent(client *api.Client) (*api.Agent, error) {
	resp, err := client.AgentsList()
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}

	switch len(resp.Data) {
	case 0:
		return nil, fmt.Errorf("no active agents. Run `irons agents new --repo <url>` to start one")
	case 1:
		return &resp.Data[0], nil
	default:
		printAgentsTable(resp.Data)
		fmt.Println()

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Which agent? ")
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("reading input: %w", err)
			}
			line = strings.TrimSpace(line)

			// Try as a 1-based index.
			idx, err := strconv.Atoi(line)
			if err == nil && idx >= 1 && idx <= len(resp.Data) {
				return &resp.Data[idx-1], nil
			}

			// Try as a name or ID.
			for i := range resp.Data {
				if resp.Data[i].Name == line || resp.Data[i].ID == line {
					return &resp.Data[i], nil
				}
			}

			fmt.Printf("Invalid selection %q. Enter a number (1-%d), name, or ID.\n", line, len(resp.Data))
		}
	}
}

// printAgentsTable prints a table of agent sessions.
func printAgentsTable(agents []api.Agent) {
	table := tablewriter.NewTable(os.Stdout)
	table.Header([]string{"Name", "Repo", "Status", "Uptime", "ID"})
	for _, a := range agents {
		table.Append([]string{a.Name, a.Repo, a.Status, formatUptime(a.CreatedAt), a.ID})
	}
	table.Render()
}

// formatUptime computes a human-readable duration from a creation timestamp.
func formatUptime(createdAt string) string {
	// Try RFC3339 first, then with fractional seconds.
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.999999999Z07:00", createdAt)
		if err != nil {
			return createdAt
		}
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, h)
	}
}
