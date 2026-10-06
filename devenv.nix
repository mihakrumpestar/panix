{
  pkgs,
  config,
  lib,
  ...
}:
let
  # Tool groups. Each group is declared once and referenced both by the full
  # local shell (every group is on by default) and by the slim CI profiles at
  # the bottom, so the tool lists never diverge.
  lintTestTools = with pkgs; [
    golangci-lint
    govulncheck
    tparse
    gitleaks
  ];
  e2eTools = with pkgs; [
    qemu_kvm
    cdrkit # For genisoimage
    harmonia
    age # Encrypts the age secret fixture
    sops # Encrypts the sops secret fixture
  ];
  releaseTools = with pkgs; [
    nix-update
    gh
    git-cliff
    act
  ];
in
{
  # Which tool groups the shell includes. All default to true so a plain
  # `devenv shell` keeps the full local environment; the CI profiles below turn
  # off what their job does not use. Also overridable per invocation, e.g.:
  #   devenv --option panix.groups.e2e.enable:bool false shell
  options.panix.groups = {
    go.enable = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Go toolchain with LSP/editor tools plus lint and test tooling (golangci-lint, tparse, gitleaks, govulncheck). Needed by task ci and the go:* tasks.";
    };
    docs.enable = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Bun toolchain for the docs site (task docs:*).";
    };
    e2e.enable = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "QEMU VM, ISO and secret-fixture tooling for e2e tests (task go:test:e2e).";
    };
    bench.enable = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Python stack for benchmark graphing (tests/e2e/bench_graph.py).";
    };
    release.enable = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Release and repo-maintenance tooling (task release, task update, task actions:*).";
    };
  };

  config = {
    # Go toolchain, gopls (LSP) and the editor helper tools (go-tools, gotools,
    # gomodifytags, impl, gotests, iferr) come from the language module, all
    # built against the same Go toolchain. Delve debugger is not used.
    languages.go = {
      enable = config.panix.groups.go.enable;
      delve.enable = false;
    };

    # Bun for the docs build. Node.js and the TypeScript language server are
    # disabled: this is a bun-only project, and the TS LSP requires Node.js.
    languages.javascript = {
      enable = config.panix.groups.docs.enable;
      bun.enable = true;
      nodejs.enable = false;
      lsp.enable = false;
    };

    # Python for benchmark graphing.
    languages.python = {
      enable = config.panix.groups.bench.enable;
      package = pkgs.python3.withPackages (ps: [
        ps.matplotlib
        ps.numpy
        ps.pandas
      ]);
    };

    packages = [
      pkgs.go-task # Task runner used by every task
    ]
    ++ lib.optionals config.panix.groups.go.enable lintTestTools
    ++ lib.optionals config.panix.groups.e2e.enable e2eTools
    ++ lib.optionals config.panix.groups.release.enable releaseTools;

    env = {
      PANIX_CONFIG = "examples/panix.deploy.yml";
      CGO_ENABLED = "0";
    };

    # Git hooks are declared here and installed by devenv on shell entry.
    # The `ci` hook stays registered in every profile so the generated
    # .pre-commit-config.yaml always lists exactly one hook: the git-installed
    # hook outlives any single shell entry, and pre-commit rejects both a
    # missing and an empty config. What the hook runs varies per profile via
    # `entry` (see the profiles below); by default it gates commits with
    # `task ci`. The generated file is gitignored.
    git-hooks.enable = true;
    git-hooks.hooks.ci = {
      enable = true;
      # `task ci` needs the go tool group; where it is absent the hook is a
      # no-op so commits are never blocked by the gate.
      entry = lib.mkDefault (
        if config.panix.groups.go.enable then "task ci" else "true"
      );
      language = "system";
      pass_filenames = false;
    };

    # Slim shells for CI. Each profile keeps only the tool groups its job uses
    # so the devenv closure stays around ~1 GiB (fast and cacheable) while the
    # toolchain stays identical to what `devenv shell` provides.
    # Usage: devenv --profile ci shell -- task ci
    profiles = {
      ci.module = {
        # Jobs run `task ci` as their command; the hook entry is a no-op so
        # `git-hooks:run` does not repeat the suite at shell entry. The hook
        # itself stays registered so the generated config stays valid.
        git-hooks.hooks.ci.entry = "true";
        panix.groups.docs.enable = false;
        panix.groups.e2e.enable = false;
        panix.groups.bench.enable = false;
        panix.groups.release.enable = false;
      };
      docs.module = {
        panix.groups.go.enable = false;
        panix.groups.e2e.enable = false;
        panix.groups.bench.enable = false;
        panix.groups.release.enable = false;
      };
    };
  };
}
