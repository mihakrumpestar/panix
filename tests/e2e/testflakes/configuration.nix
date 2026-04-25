{config, ...}: {
  imports = [./disko.nix];

  boot.initrd.availableKernelModules = ["virtio_blk" "virtio_pci" "virtio_net"];
  boot.loader.grub.enable = true;
  boot.loader.timeout = 0;
  boot.loader.grub.extraConfig = "serial --unit=0 --speed=115200; terminal_input serial; terminal_output serial";
  boot.kernelParams = ["console=ttyS0,115200"];

  services.openssh.enable = true;
  services.openssh.settings.PermitRootLogin = "yes";

  networking.useDHCP = true;

  environment.etc."panix-test-marker".text = "panix-e2e-test-pass";

  system.stateVersion = config.system.nixos.release;
}
