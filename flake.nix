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

        version = builtins.readFile ./gen/VERSION;
      in
      {
        packages = {
          default = pkgs.buildGoLatestModule {
            pname = "panix";
            inherit version;

            src = ./.;
            subPackages = [ "cmd/panix" ];

            flags = [ "-trimpath" ];
            ldflags = [
              "-s"
              "-w"
            ];

            env.CGO_ENABLED = 0; # Disable CGO

            doCheck = false; # Tests run in CI with race detection and coverage

            vendorHash = "sha256-q9pUwV9JGYNIDTemgu28eG2SBH2mNQ2BQO/u73f42xM=";

            # Install shell completions so that NixOS / Home Manager users get
            # tab completion automatically via programs.{bash,zsh,fish}.enable
            # instead of having to manually source completion scripts.
            nativeBuildInputs =
              pkgs.lib.optionals (pkgs.stdenv.buildPlatform.canExecute pkgs.stdenv.hostPlatform)
                [
                  pkgs.installShellFiles
                ];

            postInstall = pkgs.lib.optionalString (pkgs.stdenv.buildPlatform.canExecute pkgs.stdenv.hostPlatform) ''
              installShellCompletion --cmd panix \
                --bash <($out/bin/panix completion -c bash) \
                --fish <($out/bin/panix completion -c fish) \
                --zsh <($out/bin/panix completion -c zsh)
            '';

            meta = with pkgs.lib; {
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
