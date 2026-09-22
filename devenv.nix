{ pkgs, ... }:
{
  # Go toolchain, gopls (LSP) and the editor helper tools (go-tools, gotools,
  # gomodifytags, impl, gotests, iferr) come from the language module, all
  # built against the same Go toolchain. Delve debugger is not used.
  languages.go = {
    enable = true;
    delve.enable = false;
  };

  # Bun for the docs build. Node.js and the TypeScript language server are
  # disabled: this is a bun-only project, and the TS LSP requires Node.js.
  languages.javascript = {
    enable = true;
    bun.enable = true;
    nodejs.enable = false;
    lsp.enable = false;
  };

  packages = with pkgs; [
    # Go tooling beyond the language module
    golangci-lint
    govulncheck
    # Tasks
    go-task
    nix-update
    gh
    git-cliff
    # Security
    gitleaks
    # For tests
    tparse
    # CI
    act
    # For e2e tests
    qemu_kvm
    cdrkit # For genisoimage
    harmonia
    age # Encrypts the age secret fixture
    sops # Encrypts the sops secret fixture
  ];

  # Python for benchmark graphing.
  languages.python = {
    enable = true;
    package = pkgs.python3.withPackages (ps: [
      ps.matplotlib
      ps.numpy
      ps.pandas
    ]);
  };

  env = {
    PANIX_CONFIG = "examples/panix.deploy.yml";
    CGO_ENABLED = "0";
  };

  # Git hooks are declared here and installed by devenv on shell entry.
  # The generated .pre-commit-config.yaml is gitignored.
  git-hooks.hooks.ci = {
    enable = true;
    entry = "task ci";
    language = "system";
    pass_filenames = false;
  };
}
