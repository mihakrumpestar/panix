{
  description = "Panix - Universal NixOS Deployment Tool";

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
            env.GOWORK = "off";

            vendorHash = "sha256-kRk5A2hvDHDwqk4FEE/cadbtrpsV43j1qJQot2dioJ8=";

            meta = with pkgs.lib; {
              description = "Universal NixOS Deployment Tool";
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
