{ pkgs, ... }:
{
  config = {
    nixpkgs.hostPlatform = "x86_64-linux";

    environment.systemPackages = with pkgs; [
      hello
    ];

    # Create test marker file at /etc/panix-test-marker
    # (same pattern as NixOS configuration.nix)
    environment.etc."panix-test-marker".text = "panix-e2e-test-pass";
  };
}
