package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/ironsh/irons/api"
	"github.com/ironsh/irons/cmd/attach"
	"github.com/ironsh/irons/config"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/yarlson/tap"
)

// agentsNewOpts holds the parsed flags for agents new.
type agentsNewOpts struct {
	Repo     string
	Branch   string
	Name     string
	NoAttach bool
}

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
	Use:   "new [flags]",
	Short: "Create a new agent session",
	Long: `Create an agent session: boot a VM, clone a repo, start the agent in tmux, and SSH in.

Examples:
  irons agents new --repo acme/api
  irons agents new --repo github.com/acme/api --name fix-auth
  irons agents new --repo acme/api --no-attach`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		name, _ := cmd.Flags().GetString("name")
		noAttach, _ := cmd.Flags().GetBool("no-attach")

		return runAgentsNew(cmd.Context(), agentsNewOpts{
			Repo:     repo,
			Branch:   branch,
			Name:     name,
			NoAttach: noAttach,
		})
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
			a, err := pickAgent(cmd.Context(), client)
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

// agentsSSHCmd opens a plain SSH session to an agent's VM.
var agentsSSHCmd = &cobra.Command{
	Use:   "ssh <name|id> [command...]",
	Short: "SSH into an agent's VM",
	Long: `Open an SSH session to an agent's underlying VM.

Unlike 'agents attach', this gives you a plain shell rather than
attaching to the agent's tmux session. Useful for inspecting the VM,
tailing logs, or running one-off commands.

Optionally pass a command to execute on the remote VM:
  irons agents ssh fix-auth ls -la
  irons agents ssh fix-auth cat /tmp/agent.log`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		showCommand, _ := cmd.Flags().GetBool("command")
		forceTTY, _ := cmd.Flags().GetBool("tty")

		agent, err := resolveAgentFull(client, args[0])
		if err != nil {
			return err
		}

		resp, err := client.SSH(agent.VMID)
		if err != nil {
			return fmt.Errorf("getting SSH info: %w", err)
		}

		sshArgs := []string{
			"-p", fmt.Sprintf("%d", resp.Port),
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
		}

		if forceTTY {
			sshArgs = append(sshArgs, "-t")
		}

		sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", resp.Username, resp.Host))
		sshArgs = append(sshArgs, args[1:]...)

		if showCommand {
			fmt.Printf("ssh")
			for _, arg := range sshArgs {
				fmt.Printf(" %s", arg)
			}
			fmt.Println()
			return nil
		}

		fmt.Printf("Connecting to %s (%s)...\n", agent.Name, agent.ID)

		sshProc := exec.Command("ssh", sshArgs...)
		sshProc.Stdin = os.Stdin
		sshProc.Stdout = os.Stdout
		sshProc.Stderr = os.Stderr

		if err := sshProc.Run(); err != nil {
			return fmt.Errorf("SSH session ended: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsNewCmd)
	agentsCmd.AddCommand(agentsAttachCmd)
	agentsCmd.AddCommand(agentsSSHCmd)
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsDestroyCmd)

	// Flags for agents ssh
	agentsSSHCmd.Flags().BoolP("command", "c", false, "Output SSH command instead of executing it")
	agentsSSHCmd.Flags().BoolP("tty", "t", false, "Force pseudo-TTY allocation")

	// Flags for agents new
	agentsNewCmd.Flags().String("repo", "", "GitHub repo (e.g. acme/api or github.com/acme/api)")
	agentsNewCmd.Flags().String("branch", "", "Branch to checkout (defaults to repo default)")
	agentsNewCmd.Flags().String("name", "", "Session name (derived from repo if omitted)")
	agentsNewCmd.Flags().Bool("no-attach", false, "Create the session but don't SSH in")
	agentsNewCmd.MarkFlagRequired("repo")
}

// runAgentsNew is the standalone logic for creating a new agent session,
// callable from both the cobra command and from other code (e.g. post-onboarding).
func runAgentsNew(ctx context.Context, opts agentsNewOpts) error {
	client := newClient()

	// Step 1: Check onboarding credentials.
	if err := checkOnboardingSecrets(client); err != nil {
		return err
	}

	// Step 2: Resolve name from repo if not provided.
	name := opts.Name
	if name == "" {
		name = deriveNameFromRepo(opts.Repo)
	}

	// Step 3: Determine harness from config.
	harness := "claude"
	if cfg, err := config.Load(); err == nil && cfg.Harness != "" {
		harness = cfg.Harness
	}

	// Step 4: Create agent (with retry on name collision).
	fmt.Println("✓ Credentials verified")

	req := api.CreateAgentRequest{
		Name:    name,
		Repo:    opts.Repo,
		Branch:  opts.Branch,
		Harness: harness,
	}

	agent, err := createAgentWithRetry(client, req)
	if err != nil {
		return fmt.Errorf("creating agent: %w", err)
	}

	// Step 5: Wait for agent to be running.
	agent, err = waitForAgent(ctx, client, agent.ID)
	if err != nil {
		return err
	}

	fmt.Println("✓ VM ready")

	if opts.NoAttach {
		fmt.Println()
		fmt.Printf("Session: %s\n", agent.Name)
		fmt.Printf("Attach:  irons agents attach %s\n", agent.Name)
		return nil
	}

	// Step 6: Attach via SSH + tmux.
	fmt.Println("  Connecting...")
	return sshAttachToAgent(client, agent)
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
// creation. If any are missing, it tells the user to run onboard first.
func checkOnboardingSecrets(client *api.Client) error {
	resp, err := client.SecretsList()
	if err != nil {
		return fmt.Errorf("checking secrets: %w", err)
	}

	secretNames := make(map[string]bool)
	for _, s := range resp.Data {
		secretNames[s.Name] = true
	}

	if !secretNames[SecretGitHubAgent] {
		return fmt.Errorf("missing required credentials. Run `irons onboard` first")
	}

	return nil
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

// sshAttachToAgent establishes a Go-native SSH connection to the agent's VM,
// attaches to the tmux session, and provides a toggleable egress monitor (Ctrl-]).
func sshAttachToAgent(client *api.Client, agent *api.Agent) error {
	resp, err := client.SSH(agent.VMID)
	if err != nil {
		return fmt.Errorf("getting SSH info: %w", err)
	}

	return attach.Run(attach.Config{
		Host:      resp.Host,
		Port:      resp.Port,
		Username:  resp.Username,
		Command:   "tmux attach -t main",
		VMID:      agent.VMID,
		AgentName: agent.Name,
		Client:    client,
	})
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
func pickAgent(ctx context.Context, client *api.Client) (*api.Agent, error) {
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

		var options []tap.SelectOption[int]
		for i, a := range resp.Data {
			label := fmt.Sprintf("%s (%s)", a.Name, a.Repo)
			options = append(options, tap.SelectOption[int]{Value: i, Label: label})
		}

		idx := tap.Select(ctx, tap.SelectOptions[int]{
			Message: "Which agent?",
			Options: options,
		})

		return &resp.Data[idx], nil
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
