{config, pkgs, ...}: {
  imports = [./disko.nix];

  boot.initrd.availableKernelModules = ["virtio_blk" "virtio_pci" "virtio_net"];
  boot.loader.grub.enable = true;
  boot.loader.timeout = 0;
  boot.loader.grub.extraConfig = "serial --unit=0 --speed=115200; terminal_input serial; terminal_output serial";
  boot.kernelParams = ["console=ttyS0,115200"];

  services.openssh.enable = true;
  services.openssh.settings.PermitRootLogin = "yes";

  # Keep root's systemd user manager (user@0.service) running at boot so
  # user-level activations (e.g. nix-maid's systemd-tmpfiles --user and
  # sd-switch) have a D-Bus session and XDG_RUNTIME_DIR available.
  users.users.root.linger = true;

  # Dedicated non-root user for the guarded user-tier e2e leg
  # (tests/e2e/guard_v2.go): its home-manager profile stays fresh until the
  # home phase creates the first generation, which keeps the leg's guarded
  # deploys off alice's concurrently deployed profile and gives the leg a
  # deterministic rollback target.
  users.users.guarduser = {
    isSystemUser = true;
    home = "/home/guarduser";
    createHome = true;
    group = "users";
    shell = pkgs.bash;
  };

  networking.useDHCP = true;

  environment.etc."panix-test-marker".text = "panix-e2e-test-pass";

  system.stateVersion = config.system.nixos.release;
}
