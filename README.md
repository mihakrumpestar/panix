<h1 align="center">Panix</h1>

<p align="center">
  <strong>Stateless TUI for bootstrapping and deploying NixOS</strong>
</p>

<p align="center">
  <img alt="GitHub License" src="https://img.shields.io/github/license/mihakrumpestar/panix">
  <img alt="GitHub go.mod Go version" src="https://img.shields.io/github/go-mod/go-version/mihakrumpestar/panix">
  <img alt="Static Badge" src="https://img.shields.io/badge/NIX-5277C3.svg?style=flat&logo=NixOS&logoColor=white">
</p>

<p align="center">
  A declarative NixOS deployment tool for managing remote systems with ease.
</p>

> [!WARNING]
> The tool is currently in alpha stage. Expect breaking changes.

## Demo

Video:

![Demo video](./assets/demo.gif)

TUI:

![TUI sgowcase](./assets/tui.png)

## Features

### Deployment Pipeline

- **Phase-based workflow**: Inspect → Build → Bootstrap → Transfer → Secrets → Activate
- **Multi-flake support**: Deploy configurations from multiple flakes in a single workflow
- **Shared builds**: Build once per configuration, deploy to multiple machines (deduplication)
- **Parallel execution**: Concurrent deployment to multiple machines

### Bootstrap (Unique)

- **Kexec bootstrapping**: Automatically convert any Linux machine to NixOS without manual intervention
- **Disko integration**: Automatic disk partitioning with [disko](https://github.com/nix-community/disko)
- **Disk encryption support**: Transfer LUKS/disk encryption keys before partitioning
- **Post-bootstrap hooks**: Run custom scripts after bootstrapping (e.g., `systemd-cryptenroll` for TPM)
- **Custom kexec tarballs**: Use your own kexec images for specialized setups

### Configuration Inheritance

- **Hierarchical attributes**: Flake → Configuration → Machine inheritance
- **Tag-based filtering**: Filter machines by tags (names are automatically registered as tags)
- **Attribute merging**: Child configs inherit and can override parent settings

### TUI (Terminal User Interface)

- **Real-time status table**: Architecture, generation, date, NixOS version, kernel
- **Phase status visualization**: Animated progress indicators
- **Build log viewports**: Scrollable logs with filtering options
- **Mouse support**: Click to select machines/phases
- **Keybinds**: `r` retry, `ctrl+r` restart, `m` fullscreen, `c` toggle commands, `h` toggle logs
- **Retry mechanism**: Built-in retry for transient failures without restart

### Remote Management

- **SSH config integration**: Uses your existing `~/.ssh/config` for host aliases
- **Local machine support**: Deploy to the local machine without SSH
- **Automatic sudo handling**: Detects root vs non-root and adjusts commands accordingly

### CLI & CI/CD

- **Dry-run mode**: Preview changes without executing (`--dry-run`, `--dry-run-with-status`)
- **Exit on complete**: Auto-exit for CI/CD pipelines (`--exit-on-complete`)
- **Require all success**: Fail-fast mode for CI/CD (`--require-all-success`)
- **Phase skipping**: Skip specific phases (`--skip-phases`)
- **Configurable timeout**: Set workflow timeout

### Developer Experience

- **YAML schema**: IDE autocompletion with `panix schema` command
- **File logging**: Debug mode with `--debug` flag
- **CPU profiling**: Built-in profiling for performance analysis

## Setup

Remote requires SSH key authentication (key file must be without password, unless you are using an SSH agent). Password authentication is not supported.

If you have only password auth (you booted NixOS ISO), create and add a temporary key to remote with the following commands:

```sh
# On remote (set password for root user)
sudo passwd
```

```sh
export REMOTE=<host>

# Generate key pair
ssh-keygen -t ed25519 -f ./temp_key -C "temporary_deployment_key" -N ""

# Copy key to remote (with disabled SSH agent to prevent trying to auth with keys in agent)
SSH_AUTH_SOCK="" ssh-copy-id -i ./temp_key.pub root@$REMOTE
```

You now may test the login with:

```sh
ssh -i ./temp_key -o IdentitiesOnly=yes root@$REMOTE
```

Now you can get (for example) the hardware config:

```sh
nixos-generate-config --no-filesystems --show-hardware-config
```

## Caveats

These aren't caveats per se, but important considerations:

- local and remote have to have `rsync` installed to transfer plan files/dirs (note that `kexec` already includes `rsync`)
- default remote shell (e.g. `sh`) has to be POSIX compliant shell for specific commands to run (e.g. can't use `Fish` shell)
- for `kexec`, make sure you satisfy the [minimum system requirements](https://github.com/nix-community/nixos-images#requirements) (e.g. 1GB of memory without swap)

## YAML schema

Panix can generate YAML schema with `panix schema` for seeing parameter descriptions and their validation in your IDE. You just reference it in `panix.yml` as:

```yml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mihakrumpestar/panix/refs/heads/main/gen/panix-schema.yaml

...
```

or

```yml
# yaml-language-server: $schema=./panix-schema.yaml

...
```

Note that you might need to add support using an extension like [vscode-yaml](https://github.com/redhat-developer/vscode-yaml).

## Contributing

Contributions are welcome! Whether it's bug reports, feature requests, constructive criticism, or pull requests — all feedback is appreciated.

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
