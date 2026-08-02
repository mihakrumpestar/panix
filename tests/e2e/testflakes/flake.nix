{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    disko.url = "github:nix-community/disko";
    disko.inputs.nixpkgs.follows = "nixpkgs";
    nixos-images.url = "github:nix-community/nixos-images";
    nixos-images.inputs.nixos-unstable.follows = "nixpkgs";
    home-manager.url = "github:nix-community/home-manager";
    home-manager.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs =
    {
      self,
      nixpkgs,
      disko,
      nixos-images,
      home-manager,
    }:
    let
      system = "x86_64-linux";
      sshPubKey = builtins.readFile ./ssh-key.pub;
      pkgs = nixpkgs.legacyPackages.${system};
      lib = nixpkgs.lib;

      authorizedKeysModule = {
        users.users.root.openssh.authorizedKeys.keys = [ sshPubKey ];
      };
      opensshModule = {
        services.openssh.enable = true;
        services.openssh.settings.PermitRootLogin = "yes";
      };
      networkModule = {
        systemd.network.wait-online.enable = false;
      };
      substitutersModule = {
        nix.settings.substituters = lib.mkBefore [ "http://10.0.2.2:5000?priority=10" ];
        nix.settings.require-sigs = lib.mkOverride 1 false;
      };
    in
    {
      nixosConfigurations.test-vm = nixpkgs.lib.nixosSystem {
        inherit system;
        modules = [
          disko.nixosModules.disko
          ./configuration.nix
          authorizedKeysModule
        ];
      };

      nixosConfigurations.test-vm-remote = nixpkgs.lib.nixosSystem {
        inherit system;
        modules = [
          disko.nixosModules.disko
          ./configuration.nix
          authorizedKeysModule
          substitutersModule
        ];
      };

      homeConfigurations.test-home = home-manager.lib.homeManagerConfiguration {
        inherit pkgs;
        modules = [
          {
            home.username = "root";
            home.homeDirectory = "/root";
            home.stateVersion = "26.11";
            home.file.".panix-home-test-marker".text = "panix-e2e-test-pass";
          }
        ];
      };

      packages.${system} = {
        installer-iso =
          (nixpkgs.lib.nixosSystem {
            inherit system;
            modules = [
              nixos-images.nixosModules.image-installer
              nixos-images.nixosModules.noninteractive
              authorizedKeysModule
              opensshModule
              networkModule
              substitutersModule
              {
                networking.wireless.enable = lib.mkForce false;
                environment.systemPackages = [ pkgs.rsync ];

                # Reduce bootloader countdown (default is 10s in iso-image.nix)
                # NOTE: 0 means "disabled/wait forever" in syslinux (BIOS boot), so use 1
                boot.loader.timeout = lib.mkForce 1;

                # Disable Tor hidden SSH service (not needed for automated installs, saves 30-90s)
                # Must also re-enable SSH since tor-ssh module was providing it
                tor-ssh.enable = lib.mkForce false;

                # Disable WiFi daemon (not needed in VMs)
                networking.wireless.iwd.enable = lib.mkForce false;

                # NetworkManager is redundant with systemd-networkd
                networking.networkmanager.enable = lib.mkForce false;

                # No RAID or full hardware scanning needed in VMs
                boot.swraid.enable = lib.mkForce false;
                hardware.enableAllHardware = lib.mkForce false;
              }
            ];
          }).config.system.build.isoImage;

        # Pre-build is at URL: https://github.com/nix-community/nixos-images/releases/latest/download/nixos-kexec-installer-noninteractive-x86_64-linux.tar.gz
        # Repo also contains instructions on how to build and package it properly
        kexec-installer =
          (nixpkgs.lib.nixosSystem {
            inherit system;
            modules = [
              nixos-images.nixosModules.kexec-installer
              nixos-images.nixosModules.noninteractive
              authorizedKeysModule
              opensshModule
              networkModule
              substitutersModule
            ];
          }).config.system.build.kexecInstallerTarball;
      };
    };
}
