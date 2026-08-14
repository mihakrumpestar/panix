{config, ...}: {
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

  networking.useDHCP = true;

  environment.etc."panix-test-marker".text = "panix-e2e-test-pass";

  system.stateVersion = config.system.nixos.release;
}
