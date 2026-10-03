{
  description = "Panix - Universal Nix Deployment Orchestrator";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        inherit (pkgs) lib;

        version = lib.fileContents ./gen/VERSION;

        # Completion generation runs the built binary, which only works when
        # the build platform can execute binaries for the host platform.
        canRunOnHost = pkgs.stdenv.buildPlatform.canExecute pkgs.stdenv.hostPlatform;

        # Guardian variants embedded into panix at build time (spec 10.3):
        # linux/darwin x amd64/arm64, built from the same Go module with the
        # same builder as panix (subPackages). nixpkgs' buildGoModule pins
        # GOOS/GOARCH to the go compiler's platform (module.nix merges them
        # over the caller's env), so each cross target re-applies its
        # platform via overrideAttrs; with CGO_ENABLED=0 no target SDK is
        # required for the darwin builds.
        mkGuardian =
          os: arch:
          (pkgs.buildGoModule {
            pname = "panix-guard-${os}-${arch}";
            inherit version;

            src = ./.;
            subPackages = [ "cmd/panix-guard" ];

            flags = [ "-trimpath" ];
            ldflags = [
              "-s"
              "-w"
            ];

            env.CGO_ENABLED = "0";

            doCheck = false;

            vendorHash = "sha256-/76T5gN3t8CLwdm7vxZw3c4g0Y+DVkCLUJXbu7QFmqI=";
          }).overrideAttrs (old: {
            env = (old.env or { }) // {
              GOOS = os;
              GOARCH = arch;
            };
          });

        guardianTargets = {
          linux-amd64 = {
            os = "linux";
            arch = "amd64";
          };
          linux-arm64 = {
            os = "linux";
            arch = "arm64";
          };
          darwin-amd64 = {
            os = "darwin";
            arch = "amd64";
          };
          darwin-arm64 = {
            os = "darwin";
            arch = "arm64";
          };
        };

        guardians = lib.mapAttrs (_: target: mkGuardian target.os target.arch) guardianTargets;

        # Embed population (spec 15): copy the guardian outputs into the embed
        # dir as panix-guard-<os>-<arch> and gzip them before `go build` embeds
        # them via //go:embed all:bin/*. -n drops the gzip timestamp so the
        # embedded bytes stay reproducible; -f keeps reruns idempotent.
        embedGuardians = ''
          mkdir -p internal/workflow/phaseops/guard/bin
        ''
        + lib.concatStrings (
          lib.mapAttrsToList (name: guardian: ''
            cp ${guardian}/bin/panix-guard internal/workflow/phaseops/guard/bin/panix-guard-${name}
            gzip -9nf internal/workflow/phaseops/guard/bin/panix-guard-${name}
          '') guardians
        );
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "panix";
            inherit version;

            src = ./.;
            subPackages = [
              "cmd/panix"
              "cmd/panix-guard"
            ];

            flags = [ "-trimpath" ];
            ldflags = [
              "-s"
              "-w"
            ];

            env.CGO_ENABLED = 0; # Disable CGO

            doCheck = false; # Tests run in CI with race detection and coverage

            vendorHash = "sha256-/76T5gN3t8CLwdm7vxZw3c4g0Y+DVkCLUJXbu7QFmqI=";

            # Embed the four guardian variants (spec 15): populate the embed
            # dir before `go build` so //go:embed picks up the .gz files. The
            # go-modules fetch inherits hooks from the main derivation, so it
            # is stripped there via overrideModAttrs.
            preBuild = embedGuardians;

            overrideModAttrs = _final: _previous: {
              preBuild = "";
            };

            # Install shell completions so that NixOS / Home Manager users get
            # tab completion automatically via programs.{bash,zsh,fish}.enable
            # instead of having to manually source completion scripts.
            nativeBuildInputs = [ pkgs.gzip ] ++ lib.optionals canRunOnHost [ pkgs.installShellFiles ];

            postInstall = lib.optionalString canRunOnHost ''
              installShellCompletion --cmd panix \
                --bash <($out/bin/panix completion -c bash) \
                --fish <($out/bin/panix completion -c fish) \
                --zsh <($out/bin/panix completion -c zsh)
            '';

            meta = with lib; {
              description = "Universal Nix Deployment Orchestrator";
              homepage = "https://github.com/mihakrumpestar/panix";
              changelog = "https://github.com/mihakrumpestar/panix/releases/tag/v${version}";
              license = licenses.agpl3Only;
              maintainers = [
                {
                  name = "Miha Krumpestar";
                  github = "mihakrumpestar";
                  githubId = 70652456;
                }
              ];
              platforms = platforms.all;
              mainProgram = "panix";
            };
          };

          panix-guard-linux-amd64 = guardians.linux-amd64;
          panix-guard-linux-arm64 = guardians.linux-arm64;
          panix-guard-darwin-amd64 = guardians.darwin-amd64;
          panix-guard-darwin-arm64 = guardians.darwin-arm64;
        };

        apps = {
          default = flake-utils.lib.mkApp {
            drv = self.packages.${system}.default;
            name = "panix";
          };
        };
      }
    );
}
