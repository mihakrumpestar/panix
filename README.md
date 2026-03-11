<div align="center">

# Panix

**The NixOS Deployment Experience You've Been Waiting For**

*A stateless TUI-driven orchestrator for bootstrapping and deploying multi-flake NixOS systems*

[![Version](https://img.shields.io/github/v/release/mihakrumpestar/panix?label=version&color=5277C3)](https://github.com/mihakrumpestar/panix/releases)
[![License](https://img.shields.io/github/license/mihakrumpestar/panix)](https://github.com/mihakrumpestar/panix/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mihakrumpestar/panix)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/mihakrumpestar/panix)](https://goreportcard.com/report/github.com/mihakrumpestar/panix)
[![Scc Count Badge](https://sloc.xyz/github/mihakrumpestar/panix?category=code)](https://github.com/boyter/scc/)
[![NixOS](https://img.shields.io/badge/NIX-5277C3.svg?style=flat&logo=NixOS&logoColor=white)](https://nixos.org)

</div>

---

> [!WARNING]
> The tool is currently in beta stage. Expect breaking changes.

## Demo

![Demo](./assets/demo.gif)

---

## The Problem

NixOS promises a world where system configuration is declarative, reproducible, and version-controlled. A world where rollback is trivial - just switch a symlink. Where your development machine and production servers share the same configuration DNA.

This promise holds beautifully for a single machine. `nixos-rebuild switch` works. Your flake builds. Everything is good.

**Then you need to deploy to a fleet.**

Some machines already run NixOS. Others are bare metal, cloud instances, or legacy systems waiting to be converted. And suddenly you're not in the elegant world of Nix anymore - you're in the messy world of operations:

- **Bootstrap complexity**: Each non-NixOS machine needs kexec, disko, partitioning - orchestrated manually or via fragile scripts
- **Visibility gaps**: Is that build still running? Did kexec succeed? Which machine failed? The answers are buried in scrollback
- **Secrets sprawl**: Deploying sensitive files becomes an afterthought, handled via ad-hoc `rsync` or `scp` commands
- **No recovery path**: When something fails halfway through, you're left reconnecting manually, parsing logs, guessing what went wrong
- **Fleet heterogeneity**: Multiple flakes, multiple configurations, machines in different states - no unified view

The ecosystem has tools for pieces of this puzzle. **nixos-anywhere** handles bootstrap. **deploy-rs**, **Colmena** (and many others) manage deployments. Each excels at its domain. But orchestration across these concerns - bootstrap, deploy, secrets, visibility, recovery - remains manual.

A lot of the tools that manage deployments also introduce themselves as a dependancy in your flake, making it possible to deploy only with their tool (without at least some caviats to get the system closure path). Panix intentionaly does not require you to modify your flake for it.

**The missing piece: an operator-focused interface.**

Not a script that runs and exits. Not a single command whose output you scroll through (or it might not even provide them) to find the relevant information after it already failed. But an interactive, real-time view into your deployment pipeline - where you can see every phase, every machine, every failure, and act on them immediately.

This is the gap Panix fills.

---

## What Panix Actually Is

Panix is a **stateless, phase-oriented deployment orchestrator** with a real-time TUI. Stateless means it holds no persistent state of its own - everything derives from your flake, your configuration file and actual state of the machines.

The TUI isn't a gimmick. It's a recognition that deployments are **interactive processes**. Things fail, networks hiccup, builds take time, etc. Having visibility into every phase of every machine - seeing the architecture detection, watching the closure transfer, observing the activation - transforms deployment from a "*blindly run and pray*" operation into a controlled, observable process.

---

## The Phase Pipeline

Panix doesn't just "*run a deployment*". It executes a carefully ordered pipeline of phases, each with a specific scope and purpose:

<div align="center">

Inspect → Build → Bootstrap → Transfer → Secrets → Activate

</div>

**Scope-aware execution**: The `build` phase runs once per configuration, not per machine. If three machines share the same `nixosConfiguration`, you build once. The closure is then transferred to all three machines independently in parallel.

**Phase-by-phase breakdown:**

| Phase | Scope | Purpose |
|-------|-------|---------|
| **Inspect** | Per-machine | TCP reachability, SSH authentication, architecture detection, OS detection, generation discovery |
| **Build** | Per-configuration | Build `config.system.build.toplevel` closure via `nix build --json` |
| **Bootstrap** | Per-machine | kexec into NixOS installer (if needed), disko partitioning, encryption keys transfer (if provided) |
| **Transfer** | Per-machine | `nix copy` closure to target (handles `/mnt` for bootstrapped systems) |
| **Secrets** | Per-machine | Transfer files/directories with proper ownership via rsync |
| **Activate** | Per-machine | `nixos-install` (bootstrap) or `switch-to-configuration switch` (deploy) |

The TUI shows this unfolding in real-time:

![Phase status](./assets/phase-status.png)

When a phase fails, you don't have to restart everything. You inspect the logs, understand the failure, maybe do a quick fix in code or on remote, and press `r` to retry just the failed phases.

Or if it changed beyond a phase, just restart the whole workflow with `ctrl+r` without leaving the TUI.

---

## Features

<details>
<summary><strong>Hierarchical Configuration Inheritance</strong></summary>

Your infrastructure has natural hierarchies: flakes contain configurations, configurations contain machines. Panix's configuration model reflects this:

<div align="center">

Root → Flake → Configuration → Machine

</div>

Attributes at each level cascade down, without overriding child attributes. Slices (tags, secrets, disk encryption keys) **append**. Define once at the root, extend at any level:

```yaml
root:
  tags: [production]              # Inherited by all descendants
  secrets:                        # Inherited by all descendants
    - local_path: ./secrets/common.key
      remote_path: /var/secrets/common.key
  
  flakes:
    infrastructure:
      tags: [critical]            # Accumulated: [production, critical]
      secrets:                    # APPENDED to inherited secrets
        - local_path: ./secrets/prod/api.key
          remote_path: /var/secrets/api.key
      
      configurations:
        webserver:
          tags: [web]             # Accumulated: [production, critical, web]
          
          machines:
            web-01:               # Accumulated: [production, critical, web, web-01]
            web-02:
              secrets:            # APPENDED to all inherited secrets
                - local_path: ./secrets/web-02/cert.pem
                  remote_path: /etc/ssl/cert.pem
```

</details>

<details>
<summary><strong>Bootstrap: From Nothing to NixOS</strong></summary>

The bootstrap flow is where Panix distinguishes itself most clearly:

1. **Inspect phase** detects the target OS; if not NixOS, **kexec** boots into a NixOS installer image
2. **Disko** partitions disks according to your configuration
3. **Transfer** system closure
4. **nixos-install** lays down the system
5. **Reboot** into your new NixOS system

```yaml
machines:
  bare-metal:
    ssh:
      hostname: 192.168.1.100
    bootstrap:
      disk_encryption_keys:       # Transferred BEFORE disko runs
        - local_path: ./secrets/luks.key
          remote_path: /tmp/luks-key
      post_bootstrap_hooks:       # Run after disko partitioning
        - systemd-cryptenroll --tpm2-device=auto /dev/nvme0n1p2
```

#### Bootstrap Hooks

Panix provides multiple hook points during bootstrap:

| Hook | When it runs | SSH used |
|------|--------------|----------|
| `post_bootstrap_hooks` | After disko partitioning | Bootstrap SSH |
| `post_bootstrap_install_hooks` | After nixos-install, before reboot | Bootstrap SSH |
| `post_bootstrap_provisioned_hooks` | After reboot into new system | Regular SSH |

Special hook commands:
- `waitForOnline` - Wait for machine to become reachable (useful after reboot)
- `waitForOffline` - Wait for machine to become unreachable (useful during reboot)

```yaml
machines:
  server:
    bootstrap:
      post_bootstrap_hooks:
        - systemd-cryptenroll --tpm2-device=auto /dev/sda2
      post_bootstrap_install_hooks:
        - echo "Installation complete, preparing for reboot"
      post_bootstrap_provisioned_hooks:
        - reboot # second reboot
        - waitForOffline
        - waitForOnline
        - systemctl enable --now my-service
```

#### Bootstrap SSH

For machines that need different SSH credentials during initial provisioning (e.g., before vs after bootstrap), you can specify a separate bootstrap SSH configuration:

```yaml
machines:
  server:
    ssh:                          # Regular SSH (after bootstrap)
      hostname: 192.168.1.100
      identity_file: ./keys/prod.key
    bootstrap:
      ssh:                        # Bootstrap SSH (during provisioning)
        hostname: 192.168.1.100
        identity_file: ./keys/temp.key
```

During the **Inspect** phase, Panix checks which SSH configuration is reachable and uses it for subsequent operations. After reboot, it automatically switches to the regular SSH configuration for `post_bootstrap_provisioned_hooks`.

#### SSH Key Checking Options

```yaml
ssh:
  # Enable strict host key checking (default: true for regular SSH)
  strict_key_checking: true
  # Disable auto-adding host keys (default: false for regular SSH)
  disable_auto_add_host_key: false
```

Defaults:
- **Regular SSH**: `strict_key_checking: true`, `disable_auto_add_host_key: false` (secure by default)
- **Bootstrap SSH**: `strict_key_checking: false`, `disable_auto_add_host_key: true` (permissive for new machines)

#### Disable Automatic Reboot

To prevent automatic reboot after `nixos-install` (useful for manual inspection or custom reboot handling):

```yaml
machines:
  server:
    bootstrap:
      disable_automatic_reboot: true
```

</details>

<details>
<summary><strong>Multi-Flake Deployments</strong></summary>

Your infrastructure may span multiple flakes/repositories. Panix treats this as a first-class concern:

```yaml
root:
  flakes:
    infrastructure:
      url: path:../infra-flake
      configurations:
        servers:
          machines:
            server-01:
            server-02:
    
    monitoring:
      url: github:myorg/monitoring-flake
      configurations:
        prometheus:
          machines:
            prom-01:
    
    secrets-management:
      url: git+ssh://git@github.com/myorg/vault-nixos
      configurations:
        vault:
          machines:
            vault-01:
```

With this you get complete visibility across your entire infrastructure. Each flake builds independently, each configuration deduplicates builds across its machines.

</details>

<details>
<summary><strong>Real-Time TUI</strong></summary>

![TUI Showcase](./assets/tui.png)

The stats table shows you what matters:

- architecture (for cross-compilation awareness)
- generation (for rollback context)
- NixOS version
- kernel version

Click and navigate (`left`/`right` keys) to any machine to filter build logs. This also works for phase status.

#### Keybinds

| Key | Action |
|-----|--------|
| `r` | Retry failed phases |
| `ctrl+r` | Restart entire workflow (this does not reread the yaml config) |
| `m` | Toggle logs fullscreen (make any build logs label or command output in build logs fullscrean for easier reading) |
| `ctrl-c` | Copy active build logs label or command output to clipboard |
| `c` | Toggle labels between descriptions and raw commands |
| `h` | Toggle to include `inspect` and `secrets` phases in the build logs |
| `a` | Show only active/errored build logs |
| `left`/`right` | Navigate between stats table or phase status entrys |
| `mouse click` | Select and entry from stats table, phase status, build logs command label or command output |
| `mouse scroll`/`up`/`down` | Allows scrolling main view or an inner view when selected (eg. command output) |
| `q` | Quit |

</details>

<details>
<summary><strong>Tag-Based Filtering</strong></summary>

Every name (flake, configuration, machine) is automatically a tag. Tags accumulate through inheritance. Deploy subsets of your infrastructure:

```bash
panix --tags production         # All production-tagged machines
panix --tags webserver          # All machines under webserver config
panix --tags server-01          # Single machine
panix --tags server-01,special  # Single machine and tag special
```

</details>

<details>
<summary><strong>SSH Config Integration</strong></summary>

Panix reads `~/.ssh/config`. If your machine name matches a host alias:

```yaml
machines:
  production-server-01:     # Uses SSH config: Host, Port, User, IdentityFile
```

No duplication. Your SSH config is the source of truth for connection parameters.

If you don't use SSH config or need to add/change something temporarily you can specify SSH options directly:

```yaml
machines:
  my-server:
    ssh:
      hostname: 192.168.1.100
      port: 2222
      username: admin
      identity_file: ./keys/server.key
      strict_key_checking: false      # Disable strict host key checking
      disable_auto_add_host_key: true  # Prevent auto-adding host keys
```

</details>

<details>
<summary><strong>Local Machine Detection</strong></summary>

Deploying to the machine you're on? If the machine name matches the system hostname, Panix skips SSH entirely - executing commands directly via shell:

```yaml
# On a machine with hostname "workstation"
machines:
  workstation:              # Detected as local, no SSH
```

</details>

<details>
<summary><strong>CI/CD Ready</strong></summary>

```bash
# Exit when complete, fail fast on any error
panix --exit-on-complete --require-all-success

# Dry run: show what would happen
panix --dry-run

# Dry run with real status queries (inspects machines, doesn't build/transfer)
panix --dry-run-with-inspect
```

</details>

<details>
<summary><strong>IDE Support</strong></summary>

You can directly reference the YAML schema for autocompletion and validation:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mihakrumpestar/panix/main/gen/panix-schema.yaml

...
```

Or you can generate it locally:

```bash
panix schema
```

and reference it:

```yaml
# yaml-language-server: $schema=./panix-schema.yaml

...
```

</details>

---

## Installation

Run directly:

```sh
nix run github:mihakrumpestar/panix -- deploy
```

Add to your flake:

```nix
inputs.panix.url = "github:mihakrumpestar/panix";
```

and then reference it as:

```nix
panix.packages."${system}".panix
```

---

## Quick Start

Note: Remote requires SSH key authentication (key file must be without password, unless you are using an SSH agent). Password authentication is not supported.

### 1. Remote

Boot into NixOS installer, any Linux live ISO (note that it might be missing packages needed to start Kexec) or your already provisioned NixOS

### 2. SSH Authentication

If you have only password auth, create and add a temporary key to remote with the following commands:

```sh
# On remote (set password for root user)
sudo passwd
```

```sh
export REMOTE=<host>

# Generate key pair
ssh-keygen -t ed25519 -f ./temp_key -C "temporary_deployment_key" -N ""

# Copy key to remote (with disabled SSH agent to prevent trying to auth with keys in agent)
SSH_AUTH_SOCK="" ssh-copy-id -i ./temp_key.pub -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no root@$REMOTE
```

You now may test the login with:

```sh
ssh -i ./temp_key -o IdentitiesOnly=yes root@$REMOTE
```

Now you can get (for example) the hardware config:

```sh
nixos-generate-config --no-filesystems --show-hardware-config

# or on non-NixOS installs
nix-shell -p nixos-install-tools --command "nixos-generate-config --no-filesystems --show-hardware-config"
```

### 3. Create panix.yml

```yaml
root:
  flakes:
    my-config:
      url: path:./my-nixos-flake
      configurations:
        my-server:
          machines:
            my-server:
              ssh:
                hostname: 192.168.1.100
```

### 4. Deploy

```bash
panix
```

```sh
> panix --help
Usage: panix <command> [flags]

Universal NixOS Deployment Tool

Flags:
  -h, --help                               Show context-sensitive help.
  -c, --config="panix.yml"                 Config file ($PANIX_CONFIG)
  -t, --tags=TAGS,...                      Filter machines by tags (flakes, configurations and machine names are already registered as
                                           tags, children inherit all parent tags) ($PANIX_TAGS)
      --bootstrap.only                     Only initializes uninitialized machines ($PANIX_BOOTSTRAP_ONLY)
      --bootstrap.disable-auto             Disable automatic bootstrap (even if target machine does not have NixOS installed)
                                           ($PANIX_BOOTSTRAP_DISABLE_AUTO)
      --bootstrap.disable-disko            Disables building, transfer and bootstrap of disko tool ($PANIX_BOOTSTRAP_DISABLE_DISKO)
      --require-all-success                Abort if any task fails, primarily for CI/CD ($PANIX_REQUIRE_ALL_SUCCESS)
      --override-local-machine=STRING      Hostname of the machine that is local (won't use ssh to connect to it)
                                           ($PANIX_OVERRIDE_LOCAL_MACHINE)
      --dry-run                            Show what would be done without executing ($PANIX_DRY_RUN)
      --dry-run-with-inspect               Show what would be done without executing, but with real inspect query
                                           ($PANIX_DRY_RUN_WITH_INSPECT)
      --timeout=2h                         Timeout for workflow (eg. '1h', '1m15s') ($PANIX_TIMEOUT)
  -s, --skip-phases=SKIP-PHASES,...        Declare phases to skip (not all phases can be skipped) ($PANIX_SKIP_PHASES)
      --exit-on-complete                   Exit TUI immediately when workflow completes (otherwise stays open until user quits);
                                           'retry' and 'restart' do not work in this mode ($PANIX_EXIT_ON_COMPLETE)
      --tui.show-all-build-logs            Show all build logs in TUI (keybind h) ($PANIX_TUI_SHOW_ALL_BUILD_LOGS)
      --tui.show-active-only               Show only running or errored logs in TUI build logs (keybind a)
                                           ($PANIX_TUI_SHOW_ACTIVE_ONLY)
      --tui.show-commands-in-labels        Show raw commands instead of descriptions as labels in build logs (keybind c)
                                           ($PANIX_TUI_SHOW_COMMANDS_IN_LABELS)
      --tui.command-output-max-height=8    Maximum height for command labels and outputs viewports in TUI
                                           ($PANIX_TUI_COMMAND_OUTPUT_MAX_HEIGHT)
  -l, --log                                Enable logging to file ($PANIX_LOG)
      --log-file="panix.log"               Log file path ($PANIX_LOG_FILE)
  -d, --debug                              Debug output (enables logging) ($PANIX_DEBUG)
      --cpu-profile=STRING                 Path for cpu profiling to file, declaring it enables it ($PANIX_CPU_PROFILE)
      --version                            Show version ($PANIX_VERSION)

Commands:
  inspect [flags]
    Inspect machine per host (automatic bootstrapping is disabled here)

  bootstrap [flags]
    Explicit bootstrap phase

  build [flags]
    Build all selected closures

  deploy [flags]
    Do full workflow (inspect -> bootstrap -> secrets -> build -> push -> activate)

  secrets [flags]
    Deploy secrets to all machines

  schema [flags]
    Generate YAML schema for configuration files

Run "panix <command> --help" for more information on a command.
```

---

## Configuration Reference

Here is an example, but for all options check [panix-schema.yaml](./gen/panix-schema.yaml):

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mihakrumpestar/panix/main/gen/panix-schema.yaml

flags:
  config: panix.yml                    # Config file path
  tags: []                             # Filter machines by tags
  timeout: 2h
  exit_on_complete: false
  require_all_success: false
  skip_phases: []
  override_local_machine: my-laptop   # Override which machine is considered local
  dry_run: false                      # Show what would happen without executing
  dry_run_with_inspect: false         # Dry run but with real inspect queries
  bootstrap:
    only: false                       # Only bootstrap uninitialized machines
    disable_auto: false               # Disable automatic bootstrap
    disable_disko: false              # Disable disko tool
  tui:
    show_all_build_logs: false        # Show inspect/secrets phases in build logs
    show_active_only: false           # Show only running/errored logs
    show_commands_in_labels: false    # Show raw commands instead of descriptions
    command_output_max_height: 8      # Max height for command output viewports
  logging:
    log: false                        # Enable logging to file
    log_file: panix.log               # Log file path
    debug: false                      # Enable debug output
    cpu_profile: ""                   # Path for CPU profiling

root:
  tags: [production]
  secrets:
    - local_path: ./secrets/common.key
      remote_path: /var/secrets/common.key
      uid: 0
      gid: 0
  
  flakes:
    infrastructure:
      url: path:../infra-flake
      tags: [critical]
      
      configurations:
        webserver:
          tags:
            - web
          machines:
            web-01:
              ssh:
                hostname: 10.0.0.1
            web-02:
              ssh:
                hostname: web-02.example.com
                identity_file: ./keys/web-02.key
        
        database:
          machines:
            db-01:
              ssh:
                hostname: 10.0.1.50
              bootstrap:
                ssh:                        # Bootstrap SSH for initial provisioning
                  hostname: 10.0.1.50
                  identity_file: ./keys/bootstrap.key
                disk_encryption_keys:
                  - local_path: ./secrets/db/luks.key
                    remote_path: /tmp/luks-key
                post_bootstrap_hooks:
                  - systemd-cryptenroll --tpm2-device=auto /dev/sda2
    
    monitoring:
      url: github:myorg/monitoring#main
      configurations:
        prometheus:
          machines:
            prom-01:
```

The one used for testing to deploy [infrastructure](https://github.com/mihakrumpestar/infrastructure) is at [panix.yml](panix.yml).

---

## Requirements & Caveats

- **nix**: Panix uses `nix` that it finds in PATH, it also uses commands like `uname`, `id`, `echo`, `cat`, `readlink`, `stat`, `curl` and `tar`
- **rsync**: Required on both local and remote for file transfers (included in `kexec` images)
- **kexec memory**: Minimum 1GB RAM without swap for `kexec` bootstrap
- **nix store locking**: nix does not allow writing to store by more than one at a time, so some builds may have a `waiting for store lock` warning for a brief time until the lock is lifted
- **ssh key auth**: only ssh key auth is supported, no password auth
- **flake only**: only flakes are supported

---

## Contributing

Contributions are welcome! Whether it's bug reports, feature requests, constructive criticism, or pull requests - all feedback is appreciated. See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

AGPL-3.0 - see [LICENSE](LICENSE).

---

<div align="center">

*If Panix has improved your deployment workflow, consider giving it a star.*

</div>
