<div align="center">

<img src="./docs/public/icon.svg" alt="Panix" width="128" height="128">

# Panix

**Universal Nix Deployment Orchestrator**

*Stateless, phase-oriented deployment of Nix installables with real-time visibility across multi-flake fleets*

[![Version](https://img.shields.io/github/v/release/mihakrumpestar/panix?label=version&color=5277C3)](https://github.com/mihakrumpestar/panix/releases)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue)](https://github.com/mihakrumpestar/panix/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mihakrumpestar/panix)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/mihakrumpestar/panix/pkg.svg)](https://pkg.go.dev/github.com/mihakrumpestar/panix/pkg)
[![CI](https://img.shields.io/github/actions/workflow/status/mihakrumpestar/panix/ci.yml?label=CI&branch=main)](https://github.com/mihakrumpestar/panix/actions/workflows/ci.yml)
[![Zero CGO](https://img.shields.io/badge/CGO-none-success)](https://github.com/mihakrumpestar/panix)
![GitHub last commit](https://img.shields.io/github/last-commit/mihakrumpestar/panix)
[![Code lines](./gen/loc.svg)](https://github.com/boyter/scc/)
[![Coverage](./gen/coverage.svg)](https://github.com/vladopajic/go-test-coverage)
[![E2E](./gen/e2e.svg)](https://github.com/mihakrumpestar/panix/tree/main/tests/e2e)
[![NixOS](https://img.shields.io/badge/NIX-5277C3.svg?style=flat&logo=NixOS&logoColor=white)](https://nixos.org)
![GitHub Repo stars](https://img.shields.io/github/stars/mihakrumpestar/panix)

**[Documentation](https://panix.xyz)**

</div>

---

> [!WARNING]
> The tool is currently in beta stage. There might be breaking changes.

## Demo

![Demo](https://github.com/user-attachments/assets/eb1a5539-65f0-4a75-b5de-5a7988e89036)

Screenshot:

![TUI Showcase](./docs/src/assets/images/tui.png)

---

## The Problem

Deploying Nix installables today means stitching together separate tools: one for bootstrapping bare metal, another for rebuilding single machines, a third for orchestrating fleets. Panix replaces that mix with one binary, one config file, and a single observable pipeline from bare metal to running systems. See the [overview](https://panix.xyz/concepts/overview/) for the full picture.

## What Panix Does

Panix is a stateless deployment orchestrator for Nix flake installables. A deploy runs one ordered pipeline, Inspect → Bootstrap → Build → Transfer → Secrets → Activate: bootstrap and secrets only run for machines that need them, and rollback is available as a standalone command. Each phase is detailed in the [phases reference](https://panix.xyz/concepts/phases/).

- **[Real-time TUI](https://panix.xyz/tui/keybinds/)**: per-machine, per-phase progress, single-key retry of failed phases, snapshot and replay.
- **[Build once, deploy many](https://panix.xyz/concepts/configuration-model/)**: one build per installable shared by every machine using it, optional [remote builds](https://panix.xyz/configuration/build-modes/) and [GC-rooted outlinks](https://panix.xyz/configuration/nix-flags/).
- **[Multi-flake](https://panix.xyz/concepts/multi-flake/) fleets with [tag filtering](https://panix.xyz/configuration/tag-filtering/)**: deploy across repositories and subsets like `panix deploy --tags production`.
- **[Secrets](https://panix.xyz/guides/secrets/) and [bootstrap hooks](https://panix.xyz/guides/bootstrap/hooks/)**: files stay outside the Nix store with configurable ownership, hooks including `waitForOnline`/`waitForOffline`.
- **Safety nets**: opt-in [auto rollback](https://panix.xyz/guides/auto-rollback/) and [dry-run modes](https://panix.xyz/cli/deploy/).
- **[Custom output types](https://panix.xyz/configuration/output-types/)**: deploy any flake output with your own build and activation semantics, no flake modifications required.

## Output Types

Panix ships a preset for each built-in output type, covering build path, activation, and profile handling; see the [output types reference](https://panix.xyz/configuration/output-types/) for details and custom types.

<!-- OUTPUT_TYPES_START -->
| Type | Deploys | Activation |
|------|---------|------------|
| `nixosConfigurations` | [NixOS](https://nixos.org/manual/nixos/stable/) system | `switch-to-configuration` (only type that supports bootstrap) |
| `darwinConfigurations` | [nix-darwin](https://github.com/nix-darwin/nix-darwin) (macOS) | `activate` script |
| `systemConfigs` | [system-manager](https://github.com/numtide/system-manager) | `bin/activate` |
| `homeConfigurations` | [home-manager](https://github.com/nix-community/home-manager) | `activationPackage/activate` |
| `nixOnDroidConfigurations` | [Nix-on-Droid](https://github.com/nix-community/nix-on-droid) | `activate` script |
| `packages` | [Arbitrary packages](/guides/packages/) | `nix profile add` (nix profile install under Lix) |
| `maidConfigurations` | [nix-maid](https://github.com/viperML/nix-maid) | `bin/activate` |
<!-- OUTPUT_TYPES_END -->

---

## At a Glance

`panix.yml`:

<!-- PANIX_YML_START -->
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/mihakrumpestar/panix/main/gen/panix-schema.yaml

# Minimal Panix configuration demo.
#
# All fields have sensible defaults:
#   config file: panix.yml          (can be overridden with -c)
#   flake url:   .                  (current directory, can be omitted)
#   build_mode:  local              (build locally, then nix copy)
#   activation:  switch             (default per output type, overridable per installable)
#   SSH:         machine name matched against ~/.ssh/config
#   installable: <outputType>.<name>  (e.g. nixosConfigurations.workstation)
#   inheritance: fleet → flake → installable → machine
#                (tags, secrets, SSH, bootstrap, nix cascade down)
#
# Custom flake output types can be declared under a top-level output_types: section (see docs).

fleet:
  flakes:
    my-infra:
      # url defaults to ".", can be omitted when flake is in current dir
      installables:
        nixosConfigurations:
          workstation: # nixosConfigurations.workstation
            machines:
              workstation: # matched against ~/.ssh/config

          servers: # multi-machine, build once, copy to both
            machines:
              server-eu: # matched against ~/.ssh/config
              server-us:
                ssh: # SSH not in ~/.ssh/config → specify here
                  hostname: server-us.example.com

          vps: # another single machine
            machines:
              my-vps:
                ssh:
                  hostname: 10.0.0.100
                  port: 2222

        homeConfigurations:
          dev: # homeConfigurations.dev
            machines:
              workstation: # same machine, different installable type
```
<!-- PANIX_YML_END -->

And run it with:

```sh
nix run github:mihakrumpestar/panix -- deploy
```

Prebuilt binaries are available from a Cachix cache; see the [installation docs](https://panix.xyz/getting-started/installation/) to opt in.

For the complete schema, see [panix-schema.yaml](gen/panix-schema.yaml).

---

## Documentation

Available at [panix.xyz](https://panix.xyz) or locally in the [docs directory](docs/src/content/docs).

---

## Contributing

Bug reports, feature requests, and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

Panix is licensed under [AGPL-3.0](LICENSE). Packages under `pkg` are licensed under [MIT](pkg/README.md).

For more details about licenses, see [choosingalicense.com/licenses](https://www.choosingalicense.com/licenses).

---

<div align="center">

*If Panix has improved your deployment workflow, consider giving it a star.*

</div>
