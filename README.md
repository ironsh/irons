# irons

`irons` is a CLI tool for spinning up egress-secured cloud VMs (sandboxes) designed for use with AI agents. It lets you create isolated, SSH-accessible environments with fine-grained control over outbound network traffic — so you can give an agent a real machine to work in without giving it unfettered internet access.

## How It Works

Each sandbox is a cloud VM provisioned through the [IronCD](https://ironcd.dev) API. Egress rules are enforced at the network level, meaning you can allowlist only the domains an agent needs to reach (e.g. a package registry, an internal API) and block everything else. Rules can also be set to `warn` mode, which logs violations without blocking them — useful for auditing before locking things down.

## Installation

### From Source

Requires Go 1.24+.

```sh
git clone https://github.com/ironcd/irons.git
cd irons
just build        # or: go build -o bin/irons .
```

Add `bin/irons` to your `$PATH`, or install directly:

```sh
go install github.com/ironcd/irons@latest
```

## Authentication

All commands require an API key. Set it via the `IRONS_API_KEY` environment variable (recommended) or the `--api-key` flag:

```sh
export IRONS_API_KEY=your-api-key
```

By default, `irons` talks to `https://elrond.ironcd.dev`. Override this with `IRONS_API_URL` or `--api-url`.

## Quick Start

```sh
# Create a sandbox and wait until it's ready
irons create my-agent-sandbox

# SSH in
irons ssh my-agent-sandbox

# When done, tear it down
irons destroy my-agent-sandbox
```

## Commands

### `create`

Provision a new sandbox.

```
irons create NAME [flags]
```

| Flag          | Default             | Description                                                    |
| ------------- | ------------------- | -------------------------------------------------------------- |
| `--key`, `-k` | `~/.ssh/id_rsa.pub` | Path to an SSH public key                                      |
| `--async`     |                     | Return immediately without waiting for the sandbox to be ready |

**Examples:**

```sh
# Basic creation
irons create my-sandbox

# Custom SSH key
irons create my-sandbox --key ~/.ssh/agent.pub

# Fire-and-forget (don't wait for ready)
irons create --async my-sandbox
```

---

### `start`

Start a sandbox that was previously stopped.

```
irons start NAME [--async]
```

Waits for the sandbox to reach the `ready` state unless `--async` is passed.

---

### `stop`

Stop a running sandbox. The sandbox can be restarted later.

```
irons stop NAME [--async]
```

Waits for the sandbox to reach the `stopped` state unless `--async` is passed.

---

### `destroy`

Permanently destroy a sandbox and clean up all associated resources.

```
irons destroy NAME [--force]
```

| Flag      | Description                                                     |
| --------- | --------------------------------------------------------------- |
| `--force` | Automatically stop the sandbox first if it is currently running |

**Examples:**

```sh
irons destroy my-sandbox
irons destroy --force my-sandbox   # stop first if running
```

---

### `status`

Show the current status and metadata of a sandbox.

```
irons status NAME
```

Displays the sandbox name, lifecycle state, creation and update timestamps, and any metadata. Includes a visual indicator:

| Indicator | Meaning         |
| --------- | --------------- |
| 🟢        | Running / ready |
| 🟡        | Starting up     |
| 🟠        | Stopped         |
| 🔴        | Error           |

---

### `ssh`

Open an interactive SSH session in a sandbox (or print the SSH command).

```
irons ssh NAME [flags]
```

| Flag                | Description                                           |
| ------------------- | ----------------------------------------------------- |
| `--command`, `-c`   | Print the SSH command instead of executing it         |
| `--strict-hostkeys` | Enable strict host key checking (disabled by default) |

**Examples:**

```sh
# Interactive session
irons ssh my-sandbox

# Print the connection command for use in scripts or other tools
irons ssh --command my-sandbox
```

---

### `egress`

Manage outbound network rules for sandboxes.

#### `egress allow DOMAIN`

Allowlist a domain so that HTTPS traffic to it is permitted.

```sh
irons egress allow api.github.com
irons egress allow crates.io
```

#### `egress deny DOMAIN`

Explicitly block outbound traffic to a domain.

```sh
irons egress deny registry.npmjs.org
irons egress deny ads.example.com
```

#### `egress list`

List all current allow and deny rules for the account.

```sh
irons egress list
```

#### `egress mode`

Get or set the enforcement mode for egress rules.

```sh
# Get current mode
irons egress mode

# Block traffic that doesn't match an allow rule
irons egress mode deny

# Log violations without blocking (useful for auditing)
irons egress mode warn
```

| Mode   | Behaviour                                              |
| ------ | ------------------------------------------------------ |
| `deny` | Outbound traffic to non-allowlisted domains is blocked |
| `warn` | Violations are logged but traffic is not blocked       |

---

### `audit`

Inspect the history of outbound network connection attempts for a sandbox.

#### `audit egress NAME`

Print the egress audit log for a sandbox — every outbound connection attempt, whether it was allowed or denied, and which mode was in effect at the time.

```
irons audit egress NAME [--follow]
```

| Flag           | Description                                     |
| -------------- | ----------------------------------------------- |
| `--follow, -f` | Continuously poll for new events (like tail -f) |

Each line of output contains four space-separated fields:

```
TIMESTAMP  VERDICT  HOST  (mode: MODE)
```

| Field       | Description                                                             |
| ----------- | ----------------------------------------------------------------------- |
| `TIMESTAMP` | RFC 3339 timestamp (local time) of the connection attempt               |
| `VERDICT`   | `ALLOWED` if the connection was permitted, `DENIED` if it was blocked   |
| `HOST`      | The destination hostname                                                |
| `MODE`      | The egress mode active at the time: `warn` (log only) or `deny` (block) |

**Example output:**

```sh
2026-02-19T16:41:54-07:00  ALLOWED  changelogs.ubuntu.com  (mode: warn)
2026-02-19T16:42:00-07:00  ALLOWED  example.com  (mode: warn)
2026-02-19T16:42:09-07:00  ALLOWED  example2.com  (mode: warn)
2026-02-19T16:42:26-07:00  DENIED   example2.com  (mode: deny)
```

**Examples:**

```sh
# Print the full egress audit log for a sandbox
irons audit egress my-sandbox

# Tail the log and print new events as they arrive
irons audit egress my-sandbox --follow
```

> **Tip:** Run `irons audit egress --follow` while your egress mode is set to `warn` to observe which domains your agent is reaching before switching to `deny`. This makes it easy to build a precise allowlist without trial and error.

---

## Global Flags

These flags are available on every command:

| Flag        | Env var         | Default                     | Description                |
| ----------- | --------------- | --------------------------- | -------------------------- |
| `--api-key` | `IRONS_API_KEY` |                             | API key for authentication |
| `--api-url` | `IRONS_API_URL` | `https://elrond.ironcd.dev` | API endpoint URL           |

## Typical Agent Workflow

1. **Provision** a sandbox with the SSH key your agent will use:

   ```sh
   irons create agent-run-42 --key ~/.ssh/agent.pub
   ```

2. **Lock down egress** to only what the agent should reach:

   ```sh
   irons egress mode deny
   irons egress allow api.openai.com
   irons egress allow pypi.org
   ```

3. **Connect** your agent using the SSH command output:

   ```sh
   irons ssh --command agent-run-42
   ```

4. **Monitor** the sandbox if needed:

   ```sh
   irons status agent-run-42
   ```

5. **Tear down** when the run is complete:
   ```sh
   irons destroy --force agent-run-42
   ```

## Development

```sh
just build    # build to bin/irons
just test     # run tests
just run      # go run . (pass args after --)
just clean    # remove build artifacts
just deps     # tidy go.mod / go.sum
```

## License

See [LICENSE](LICENSE).
