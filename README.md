# irons

> **Deprecation notice:** `irons` is deprecated. Hosted iron.sh VMs will be shut down on **May 31, 2026**. For network egress controls, use `iron-proxy` instead, and run your own compute via a sandbox or VM provider of your choice.

`irons` is a CLI for spinning up egress-secured cloud VMs designed for AI coding agents. Create isolated, SSH-accessible environments with fine-grained control over outbound network traffic — so you can give an agent a real machine without giving it unfettered internet access.

## Quick Start

```sh
curl -fsSL https://install.iron.sh | bash
irons onboard
```

The onboarding flow walks you through account creation and then asks how you want to get started:

- **Start coding with an agent** — configures a GitHub PAT, picks a harness (Claude Code or Codex), and launches an agent session against one of your repos.
- **Create a VM to poke around** — spins up an example VM with a sample secret so you can SSH in and explore.

## Agents

Agent sessions boot a VM, clone a repo, and start an AI coding agent inside a tmux session you can attach to via SSH.

```sh
# Create an agent session
irons agents new --repo acme/api

# List active sessions
irons agents list

# Reattach to a session
irons agents attach fix-auth

# SSH into the underlying VM (plain shell, not tmux)
irons agents ssh fix-auth

# Tear it down
irons agents destroy fix-auth
```

## VMs

Create and manage standalone VMs directly.

```sh
# Create a VM and wait until it's ready
irons create my-sandbox

# SSH in
irons ssh my-sandbox

# Check status
irons status my-sandbox

# Stop, start, or destroy
irons stop my-sandbox
irons start my-sandbox
irons destroy my-sandbox

# List all VMs
irons list
```

Commands accept either a VM **name** or its **ID** (e.g. `vm_abc123`).

## Secrets and Environment Variables

Secrets are encrypted at rest and injected into VMs via iron.sh's secrets proxy — they never touch disk in plaintext.

```sh
# Add a secret (injected as an env var in VMs)
irons secrets add --name my-token --env-var API_TOKEN --secret "sk-..."

# List, show, update, or remove secrets
irons secrets list
irons secrets show my-token
irons secrets update my-token --secret "sk-new..."
irons secrets remove my-token
```

Account-level environment variables are also available:

```sh
irons env set DEBUG=true
irons env list
irons env destroy DEBUG
```

## Egress Control

All VM network traffic is logged and restricted by default. You can allowlist specific domains or set rules to warn mode for auditing before locking things down.

```sh
# View or set the egress mode
irons egress mode
irons egress mode enforce
irons egress mode warn

# Manage allowlist rules
irons egress list
irons egress add --host registry.npmjs.org
irons egress remove <rule-id>

# View egress audit logs
irons audit egress
```

## Other Features

```sh
# Copy files to/from a VM
irons scp local-file.txt my-sandbox:/tmp/

# Port forwarding
irons forward my-sandbox

# Snapshot and restore VMs
irons snapshots list
irons snapshots create my-sandbox --name before-refactor

# Fork/clone a VM
irons fork my-sandbox --name my-sandbox-copy

# Manage SSH public keys
irons public-keys list
irons public-keys add --name laptop --public-key "ssh-ed25519 AAAA..."
```

## Authentication

```sh
# Interactive login (opens browser)
irons login

# Or run the full onboarding flow
irons onboard
```

Your API token is saved to `~/.config/irons/config.yml`. You can also authenticate via the `IRONS_API_KEY` environment variable or the `--api-key` flag.

## Alternative Installation

Pre-built binaries for macOS and Linux are available on the [GitHub Releases](https://github.com/ironsh/irons/releases/latest) page, or install from source (requires Go 1.24+):

```sh
go install github.com/ironsh/irons@latest
```

## Documentation

Full command reference, egress configuration, and guides are at **[docs.iron.sh](https://docs.iron.sh)**.

## License

See [LICENSE](LICENSE).
