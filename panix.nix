{
  env ? "",
  validatePaths ? true,
  ...
}: let
  disk_encryption_keys = [
    {
      local_path = "/tmp/disko-encryption-password.txt";
      remote_path = "/tmp/disko-encryption-password.txt";
    }
  ];

  tpm2_enroll_hook = disk: "systemd-cryptenroll --unlock-key-file=/tmp/disko-encryption-password.txt --tpm2-device=auto --tpm2-with-pin=no --wipe-slot=all ${disk}";

  bootstrap = {
    kexec = {
      url = "./kexec-<arch>.tar.gz";
    };
    ssh = {
      hostname = "10.0.200.226";
      identity_file = "temp_key";
    };
  };
in {
  imports = [./gen/panix-options.nix];

  flags = {
    timeout = "5500s";
    override_local_machine = "tmp";
  };

  flakes = {
    infrastructure = {
      url = "path:../infrastructure";

      configurations = {
        personal-workstation = {
          machines = {
            personal-workstation = {
              bootstrap = {
                inherit disk_encryption_keys;
              };
            };

            #fake = {
            #  ssh = {
            #    hostname = "fake";
            #    username = "root";
            #    port = 22;
            #  };
            #};
          };
        };

        personal-laptop = {
          machines = {
            personal-laptop = {
              bootstrap = {
                inherit disk_encryption_keys;
              };
            };
          };
        };

        server-01 = {
          machines = {
            server-01 = {
              bootstrap = {
                inherit disk_encryption_keys;
                post_bootstrap_hooks = [(tpm2_enroll_hook "/dev/sda2")];
              };
            };
          };
        };

        server-03 = {
          machines = {
            server-03 = {
              bootstrap = {
                inherit disk_encryption_keys;
                post_bootstrap_hooks = [(tpm2_enroll_hook "/dev/sda2")];
              };
            };
          };
        };

        vps-02 = {
          machines = {
            vps-02 = {};
          };
        };

        kiosk = {
          machines = {
            kiosk = {
              bootstrap = {
                post_bootstrap_hooks = [(tpm2_enroll_hook "/dev/mmcblk1p2")];
              };
            };
          };
        };
      };
    };
  };
}
