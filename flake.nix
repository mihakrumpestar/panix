{
  description = "Panix - A TUI application for Nix deployment workflows";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};

      version = "0.1.0";
    in {
      packages = {
        default = pkgs.buildGoModule {
          pname = "panix";
          inherit version;

          src = ./.;

          vendorHash = "sha256-uoxbEoVf/yc/PmoHKoPwZEsSuuJIydNjwZ5JH8chqak==";

          subPackages = ["cmd"];

          postInstall = ''
            mv $out/bin/cmd $out/bin/panix
          '';

          meta = with pkgs.lib; {
            description = "A TUI application for Nix deployment workflows";
            homepage = "https://github.com/mihakrumpestar/panix";
            license = licenses.mit;
            maintainers = [];
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

      devShells.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          go
          gopls
          gotools
          go-tools
        ];
      };
    });
}
