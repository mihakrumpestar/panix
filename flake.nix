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

      version = builtins.readFile ./gen/VERSION;
    in {
      packages = {
        default = pkgs.buildGoLatestModule {
          pname = "panix";
          inherit version;

          src = ./.;
          subPackages = ["cmd/panix"];

          flags = ["-trimpath"];
          ldflags = ["-s" "-w"];

          vendorHash = "sha256-bhh1+w+6QN5EaD0jzELsnuWNI25tx65adiPES+gboh8=";

          meta = with pkgs.lib; {
            description = "A TUI application for Nix deployment workflows";
            homepage = "https://github.com/mihakrumpestar/panix";
            license = licenses.mit;
            maintainers = [{name = "mihakrumpestar";}];
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
    });
}
