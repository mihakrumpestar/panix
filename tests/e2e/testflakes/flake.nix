{
  inputs = {
    nixpkgs.url = "https://flakehub.com/f/DeterminateSystems/nixpkgs-weekly/0.1";

    disko = {
      url = "github:nix-community/disko";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    nixos-images = {
      url = "github:nix-community/nixos-images";
      inputs.nixos-unstable.follows = "nixpkgs";
    };

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # nix-maid has no flake inputs of its own; it receives nixpkgs via its
    # functor argument (nix-maid pkgs { ... }), so no "follows" override.
    nix-maid.url = "github:viperML/nix-maid";

    system-manager = {
      url = "github:numtide/system-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      disko,
      nixos-images,
      home-manager,
      nix-maid,
      system-manager,
    }:
    let
      system = "x86_64-linux";
      sshPubKey = builtins.readFile ./ssh.pub;
      pkgs = nixpkgs.legacyPackages.${system};
      lib = nixpkgs.lib;

      authorizedKeysModule = {
        users.users.root.openssh.authorizedKeys.keys = [ sshPubKey ];
        users.users.alice = {
          isSystemUser = true;
          home = "/home/alice";
          createHome = true;
          group = "users";
          shell = pkgs.bash;
        };
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

      # Build a cloud-init NoCloud seed ISO via genisoimage (same approach as
      # nixpkgs cloud-init tests). Produces a store path that IS the ISO file.
      mkCloudInitSeed =
        {
          name,
          instanceId,
          hostname,
          withNix ? false,
          shutdown ? false,
          createUser ? false,
        }:
        let
          metaData = pkgs.writeText "meta-data" ''
            instance-id: ${instanceId}
            local-hostname: ${hostname}
          '';

          packages = [
            "rsync"
          ]
          ++ lib.optionals withNix [
            "curl"
            "ca-certificates"
            # Needed so the D-Bus user session bus is available for user-level
            # activations (e.g. nix-maid's sd-switch) on the Debian-nix VM.
            # Without it there is no /run/user/<uid>/bus socket to connect to.
            "dbus-user-session"
          ];
          pkgLines = lib.concatMapStrings (p: "  - ${p}\n") packages;

          # Packages, SSH keys, and sshd config are only set up during baking
          # (shutdown=true). Runtime seeds boot against already-baked images
          # where everything is pre-configured — they only need meta-data
          # (hostname/instance-id) with a minimal user-data.
          nixInstallCmd = lib.optionalString withNix "  - curl -sSfL https://install.determinate.systems/nix | sh -s -- install --no-confirm\n";
          shutdownCmd = lib.optionalString shutdown "  - shutdown -h now\n";
          createUserCmd = lib.optionalString createUser (
            lib.concatStrings [
              "  - useradd -m -s /bin/bash alice\n"
              "  - mkdir -p /home/alice/.ssh\n"
              "  - cp /root/.ssh/authorized_keys /home/alice/.ssh/authorized_keys\n"
              "  - chown -R alice:alice /home/alice/.ssh\n"
              "  - chmod 700 /home/alice/.ssh\n"
              "  - chmod 600 /home/alice/.ssh/authorized_keys\n"
            ]
          );

          userData = pkgs.writeText "user-data" (
            if shutdown then
              lib.concatStrings [
                "#cloud-config\n"
                "disable_root: false\n"
                "package_update: true\n"
                "packages:\n"
                pkgLines
                "write_files:\n"
                "  - path: /root/.ssh/authorized_keys\n"
                "    content: '${sshPubKey}'\n"
                "    owner: root:root\n"
                "    permissions: '0600'\n"
                "    defer: true\n"
                "runcmd:\n"
                "  - mkdir -p /root/.ssh\n"
                "  - chmod 700 /root/.ssh\n"
                "  - sed -i 's/^#*PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config\n"
                "  - mkdir -p /etc/ssh/sshd_config.d && echo 'PermitRootLogin yes' > /etc/ssh/sshd_config.d/root-login.conf\n"
                "  - systemctl restart sshd\n"
                # Keep root's systemd user manager (user@0.service) running at
                # boot in the baked image, so user-level activations (e.g.
                # nix-maid's systemd-tmpfiles --user and sd-switch) have a
                # D-Bus session and XDG_RUNTIME_DIR available.
                "  - loginctl enable-linger root\n"
                nixInstallCmd
                createUserCmd
                shutdownCmd
              ]
            else
              "#cloud-config\ndisable_root: false\n"
          );
        in
        pkgs.runCommand name
          {
            nativeBuildInputs = [ pkgs.cdrkit ];
          }
          ''
            tmpdir=$(mktemp -d)
            cp ${metaData} $tmpdir/meta-data
            cp ${userData} $tmpdir/user-data
            genisoimage -output $out -V cidata -r -J $tmpdir/meta-data $tmpdir/user-data
            rm -rf $tmpdir
          '';
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
            home = {
              username = "root";
              homeDirectory = "/root";
              stateVersion = lib.trivial.release;
              file.".panix-home-test-marker".text = "panix-e2e-test-pass";
            };
          }
        ];
      };

      homeConfigurations.test-home-alice = home-manager.lib.homeManagerConfiguration {
        inherit pkgs;
        modules = [
          {
            home = {
              username = "alice";
              homeDirectory = "/home/alice";
              stateVersion = lib.trivial.release;
              file.".panix-home-test-marker".text = "panix-e2e-test-pass";
            };
          }
        ];
      };

      # system-manager config for non-NixOS Linux (Debian-nix VM).
      # Manages system-level packages and /etc files via systemd.
      # NOTE: Do NOT include nix.settings here — the Debian-nix VM already has
      # /etc/nix/nix.conf from the nix-installer, and system-manager will log a
      # non-fatal (but confusing) warning if it tries to manage it.
      systemConfigs.test-system-manager = system-manager.lib.makeSystemConfig {
        modules = [ ./system-manager-config.nix ];
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

        # Debian 12 cloud image (qcow2) — used as base for Debian VMs.
        # NOTE: The URL points to "latest"; when Debian bumps the point release
        # the hash will change and this will need updating.
        debian-cloud-image = pkgs.fetchurl {
          url = "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2";
          hash = "sha256-3T29I6OWUxjMmq4yWS3P3kq8uPkKUMp2Cpyp6PO6YlU=";
        };

        # Cloud-init NoCloud seed ISOs — built via genisoimage (pkgs.cdrkit).
        # Each produces a store path that IS the ISO file, usable directly as
        # -cdrom /nix/store/.../<name>.
        seed-iso = mkCloudInitSeed {
          name = "seed.iso";
          instanceId = "panix-kexec-test";
          hostname = "kexec-vm";
        };

        seed-remote-iso = mkCloudInitSeed {
          name = "seed-remote.iso";
          instanceId = "panix-kexec-test-remote";
          hostname = "kexec-vm-remote";
        };

        seed-nix-iso = mkCloudInitSeed {
          name = "seed-nix.iso";
          instanceId = "debian-nix";
          hostname = "debian-nix-vm";
        };

        # Bake seeds — used during Debian image baking, then VM shuts down.
        bake-seed-iso = mkCloudInitSeed {
          name = "bake-seed.iso";
          instanceId = "debian-bake";
          hostname = "bake-vm";
          shutdown = true;
        };

        bake-seed-nix-iso = mkCloudInitSeed {
          name = "bake-seed-nix.iso";
          instanceId = "debian-bake-nix";
          hostname = "bake-vm-nix";
          withNix = true;
          shutdown = true;
          createUser = true;
        };

        # Simple test package for E2E verification of `packages` deploy type.
        # When installed via `nix profile add`, places `panix-package-marker`
        # in the user's PATH. The E2E test verifies by running it and checking
        # the output.
        test-package = pkgs.runCommand "panix-test-package" { } ''
          mkdir -p $out/bin
          cat > $out/bin/panix-package-marker << 'EOF'
          #!/bin/sh
          echo "panix-e2e-test-pass"
          EOF
          chmod +x $out/bin/panix-package-marker
        '';
      };

      # Real nix-maid fixture for E2E verification of the `maidConfigurations` deploy
      # type. Uses the actual nix-maid module system to produce a bundle with
      # bin/activate. After activation, the declared file exists as a symlink
      # at ~/.panix-maid-test-marker pointing into the nix store.
      maidConfigurations.test-maid = nix-maid pkgs {
        file.home.".panix-maid-test-marker".text = "panix-e2e-test-pass";
      };
    };
}
