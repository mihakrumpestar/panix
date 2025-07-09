# AGENT GUIDELINES FOR PANIX REPOSITORY

This document outlines the guidelines for automated agents working within the Panix repository.

## Build, Lint, and Test Commands

*   **Build:** `go build ./...`
*   **Lint:** `go vet ./...`
*   **Format:** `go fmt ./...`
*   **Run all tests:** `go test ./...`
*   **Run a single test:** `go test -run <TestName> <path/to/package>` (e.g., `go test -run TestMyFunction internal/config`)

## Code Style Guidelines (Go)

*   **Imports:** Group imports into standard library, third-party, and internal packages, separated by blank lines.
*   **Formatting:** Adhere to `go fmt` standards.
*   **Naming Conventions:**
    *   Package names: lowercase, single word.
    *   Variables/Functions: `camelCase` for unexported, `PascalCase` for exported.
    *   Constants: `CAPS_SNAKE_CASE` for exported, `camelCase` for unexported.
*   **Error Handling:** Return errors explicitly. Do not ignore errors.
*   **Types:** Use specific types over `interface{}` where possible.
*   **Comments:** Comment exported functions and complex logic.

## CLI APP

Panix – Universal NixOS Deployment Tool  
========================================  
A Go-based, flakes-aware, SSH-first deployer combining:  
• Two-mode bootstrapping (explicit via config, or implicit with detection+prompt)  
• YAML inventory with per-machine secrets (files or dirs auto-detected)  
• Full preflight checks (reachability, init-status, tag filtering)  
• Parallel builds, content-addressed transfers, atomic activation & rollbacks  
• Optional “all-or-nothing” vs “best-effort” failure policies  
• Native nixos-anywhere reuse, minimal on-disk state, plugin hooks  

1. Configuration & Inventory

    All settings live in a single YAML file.

2. CLI Commands  

    panix [global flags] <command> [cmd-flags]  

    • plan       : show which hosts would build/bootstrap/deploy  
    • build      : build all selected closures  
    • bootstrap  : explicit bootstrap phase  
    • deploy     : do full workflow (preflight→bootstrap→secrets→build→push→activate)  
    • rollback   : revert host(s) to previous generation  
    • status     : query last deployment status per host  

    Global flags override config:  
    • --tags=+prod,-canary  
    • --require-all, --continue-on-error  
    • --auto-bootstrap, --no-auto-bootstrap  
    • --dry-run  

3. Overall Workflow  

    1. Preflight  
        - Concurrent SSH probes to all hosts matching tag filter  
        - Classify hosts as reachable/unreachable  
        - If any unreachable & requireAllSuccess ⇒ abort  
        - For each reachable: run a “bootstrap detector” (e.g. `test -e /run/current-system`)  
        - Uninitialized hosts split into:  
            - Mode 1: have a [bootstrap] section  
            - Mode 2: lack explicit section  
        - If Mode 2 exists:  
            - autoBootstrap=true ⇒ accept all  
            - else prompt user: “Init these hosts? [Y/n]”  
        - Hosts declined or unreachable cause abort if requireAllSuccess  

    2. Bootstrapping  
        For each host marked “to bootstrap”:  
        - Copy pre-bootstrap secrets (see §4)  
        - Invoke nixos-anywhere as a subprocess, passing CLI flags from config  
        - Wait for installer to finish & host to reboot  
        - Retry SSH until the detector passes  

    3. Secrets Deployment  
 
        Before any build or bootstrap step, for each host:  
        - For each secret entry:  
            - Stat localPath: file vs directory  
            - Ensure `mkdir -p` on remote parent via SSH  
            - If file: upload single file (SFTP or scp)  
            - If directory: recursively mirror dirs and files, preserving modes  
        - Use the same SSH connection pool for efficiency  

    4. Build & Bundle  
        - Resolve for each host its flake output label  
        - In a worker pool:  
            - `nix build --json --print-out-path` per host  
            - Parse JSON to collect `.drvPath` and `.outPath` for each closure  
        - Deduplicate shared store paths across hosts to avoid redundant transfers  

    5. Transfer (Push)  
        Two optional transport modes (configurable globally or per-host):  
        - SSH copy  
            - `nix copy --to ssh://user@host <paths…>`  
        - Tarball import  
            - `nix-store --export <paths> | gzip` → `ssh host “nix-store --import”`  
        - Parallel per-host transfers (tunable concurrency)  
        - Retries with exponential backoff on SSH failures  
        - Live progress bars per host or per bundle  

    6. Activation  
        - Over SSH run: sudo nixos-rebuild switch --flake /path/to/flake#<output>  
        - Stream stdout/stderr, prefix lines with host name  
        - Check exit codes; mark each host success or failure  

    7. Failure Policies & Rollback  
        - If requireAllSuccess=true:  
            - On first failure in any phase, abort remaining hosts  
            - For hosts that already activated successfully, run: ssh host -- sudo nixos-rebuild switch --rollback  
        - If requireAllSuccess=false:  
            - Continue on errors; report per-host statuses; no auto-rollback  
        - “panix rollback <host> [--to-generation]” allows manual rollback  

4. Concurrency & Resilience  

    - Worker pools (build, transfer, activate) sized by CPU or user flag  
    - Context cancellation: failures trigger cancellation if configured  
    - Circuit breaker on repeated SSH timeouts  
    - Exponential-backoff retries for transient network errors  

5. Interactive & Dry-Run Modes  

    - --dry-run: execute preflight & planning only; print intended actions  
    - Prompt only for Mode 2 bootstraps (skip if --auto-bootstrap)  
    - Verbosity flags control log detail per-host and per-phase  

6. Architecture & Modules  

    Package structure (suggested):  
    - config    – TOML parsing & validation  
    - ssh       – connection pooling, host-key checking  
    - secrets   – file/dir detection & upload via SFTP or scp  
    - bootstrap – wrapper around nixos-anywhere invocation  
    - builder   – nix build orchestration & JSON parsing  
    - transport – nix copy / tarball export-import logic  
    - activator – SSH commands for nixos-rebuild & rollback  
    - cli       – Cobra/Viper setup for commands & flags  
    - util      – concurrency helpers, retry logic, progress bars  

7. Plugin Hooks & Extensibility  

    Allow user-provided scripts at key points:  
    - pre-build(host)  
    - post-build(host)  
    - pre-activate(host)  
    - post-activate(host)  
    Hooks run with environment vars (HOST, PHASE) and can abort or warn  

8. Packaging & Distribution  

    - Single static Go binary (no external deps)  
    - Release via GitHub Releases, Homebrew TAP, Nix derivation  
    - Provide minimal manpage and sample TOML inventory  

9. Security & Best Practices  
    - Strict SSH host-key verification by default  
    - No persistent state or secrets stored on controller beyond config  
    - Use platform keyring for any in-memory encryption needs  
    - Log sensitive operations at INFO level, avoid leaking secret contents  

    With this detailed plan, Panix will provide a unified, robust, Go-native solution for managing fleets of NixOS machines—handling everything from first-boot installs to atomic, multi-host deployments.

For SSH, we need to be able to use the ssh config file as users will also use SSH agents.
