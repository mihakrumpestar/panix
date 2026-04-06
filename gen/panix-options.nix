# Panix Nix module options
# Generated from Go structs - DO NOT EDIT

{ lib, ... }:

with lib;

let
  SSHClientDef = {
    options = {
      hostname = mkOption {
        type = types.str;
        description = "SSH hostname or IP address";
        default = "";
      };
      port = mkOption {
        type = types.int;
        description = "SSH port number";
        default = 0;
      };
      username = mkOption {
        type = types.str;
        description = "SSH username";
        default = "";
      };
      identity_file = mkOption {
        type = types.str;
        description = "Path to SSH private key";
        default = "";
      };
      strict_key_checking = mkOption {
        type = types.bool;
        description = "Enable strict host key checking (default: false for bootstrap SSH, true for regular SSH)";
        default = false;
      };
      disable_auto_add_host_key = mkOption {
        type = types.bool;
        description = "Disable automatically adding host key to known_hosts on first connection (default: true for bootstrap SSH, false for regular SSH)";
        default = false;
      };
    };
  };

  KexecConfigDef = {
    options = {
      url = mkOption {
        type = types.str;
        description = "URL or path to kexec tarball for bootstrapping non-NixOS machines";
        default = "";
      };
      extra_flags = mkOption {
        type = types.str;
        description = "Extra flags to pass to kexec (e.g. '--no-sync')";
        default = "";
      };
      ssh_port = mkOption {
        type = types.int;
        description = "SSH port for kexec installer (default: 22)";
        default = 0;
      };
    };
  };

  PlainFileOrDirToTransferDef = {
    options = {
      local_path = mkOption {
        type = types.str;
        description = "Path to a local file or dir";
      };
      remote_path = mkOption {
        type = types.str;
        description = "Absolute path on remote machine";
      };
      uid = mkOption {
        type = types.int;
        description = "Optional User ID for remote";
        default = 0;
      };
      gid = mkOption {
        type = types.int;
        description = "Optional Group ID for remote";
        default = 0;
      };
      permissions = mkOption {
        type = types.int;
        description = "Optional file permissions";
        default = 0700;
      };
    };
  };

  NixConfigDef = {
    options = {
      extra_flags = mkOption {
        type = (types.listOf types.str);
        description = "Extra flags applied to both 'nix build' and 'nix copy'";
        default = [];
      };
      build_flags = mkOption {
        type = (types.listOf types.str);
        description = "Extra flags for 'nix build' command (e.g. '--max-jobs', '4')";
        default = [];
      };
      copy_flags = mkOption {
        type = (types.listOf types.str);
        description = "Extra flags for 'nix copy' command (e.g. '--compress')";
        default = [];
      };
      nixos_install_flags = mkOption {
        type = (types.listOf types.str);
        description = "Extra flags for 'nixos-install' command (e.g. '--no-bootloader')";
        default = [];
      };
    };
  };

  # Bootstrap options for flags (subset)
  FlagsBootstrapDef = {
    options = {
      disable_auto = mkOption {
        type = types.bool;
        default = false;
      };
      disable_disko = mkOption {
        type = types.bool;
        default = false;
      };
    };
  };

  BootstrapDef = {
    options = {
      ssh = mkOption {
        type = (types.submodule SSHClientDef);
        description = "Bootstrap SSH configuration (used during initial provisioning)";
        default = {};
      };
      disk_encryption_keys = mkOption {
        type = (types.listOf (types.submodule PlainFileOrDirToTransferDef));
        description = "Keys are transferred to root dir on remote, which is the installer. If you want them to be transferred to disk of the final system, prefix path with '/mnt'";
        default = [];
      };
      post_bootstrap_hooks = mkOption {
        type = (types.listOf types.str);
        description = "Commands to run after disko partitioning";
        default = [];
      };
      post_bootstrap_install_hooks = mkOption {
        type = (types.listOf types.str);
        description = "Commands to run after nixos-install (before reboot)";
        default = [];
      };
      post_bootstrap_provisioned_hooks = mkOption {
        type = (types.listOf types.str);
        description = "Commands to run after reboot (uses regular SSH)";
        default = [];
      };
      kexec = mkOption {
        type = (types.submodule KexecConfigDef);
        description = "Kexec configuration for bootstrapping non-NixOS machines or reinstalling a live NixOS installation";
        default = {};
      };
      disable_automatic_reboot = mkOption {
        type = types.bool;
        description = "Disable automatic reboot after nixos-install (useful for manual inspection or custom reboot handling)";
        default = false;
      };
      force_bootstrap = mkOption {
        type = types.bool;
        description = "Force bootstrap even if machine is already NixOS (requires allow_destructive_actions)";
        default = false;
      };
      force_bootstrap_kexec = mkOption {
        type = types.bool;
        description = "Force kexec method even if already in NixOS installer (requires force_bootstrap and allow_destructive_actions)";
        default = false;
      };
      allow_destructive_actions = mkOption {
        type = types.bool;
        description = "Allow destructive bootstrap actions (required for force_bootstrap and force_bootstrap_kexec)";
        default = false;
      };
    };
  };

  AttributesDef = {
    options = {
      ssh = mkOption {
        type = (types.submodule SSHClientDef);
        default = {};
      };
      tags = mkOption {
        type = (types.listOf types.str);
        default = [];
      };
      secrets = mkOption {
        type = (types.listOf (types.submodule PlainFileOrDirToTransferDef));
        default = [];
      };
      disabled = mkOption {
        type = types.bool;
        default = false;
      };
      override_sudo_program = mkOption {
        type = types.str;
        default = "";
      };
      hardware_config_path = mkOption {
        type = types.str;
        default = "";
      };
      activation_mode = mkOption {
        type = types.str;
        description = "Activation mode: check, switch, boot, test, dry-activate";
        default = "switch";
      };
      bootstrap = mkOption {
        type = (types.submodule BootstrapDef);
        default = {};
      };
      nix = mkOption {
        type = (types.submodule NixConfigDef);
        default = {};
      };
    };
  };

  # Submodules for hierarchical config structure
  MachineDef = {
    options = {
      name = mkOption {
        type = types.str;
        description = "Machine name (auto-populated from attrset key)";
      };
      ssh = mkOption {
        type = (types.submodule SSHClientDef);
        default = {};
      };
      tags = mkOption {
        type = (types.listOf types.str);
        default = [];
      };
      secrets = mkOption {
        type = (types.listOf (types.submodule PlainFileOrDirToTransferDef));
        default = [];
      };
      disabled = mkOption {
        type = types.bool;
        default = false;
      };
      override_sudo_program = mkOption {
        type = types.str;
        default = "";
      };
      hardware_config_path = mkOption {
        type = types.str;
        default = "";
      };
      activation_mode = mkOption {
        type = types.str;
        description = "Activation mode: check, switch, boot, test, dry-activate";
        default = "switch";
      };
      bootstrap = mkOption {
        type = (types.submodule BootstrapDef);
        default = {};
      };
      nix = mkOption {
        type = (types.submodule NixConfigDef);
        default = {};
      };
    };
  };

  ConfigurationDef = {
    options = {
      name = mkOption {
        type = types.str;
        description = "Configuration name (auto-populated from attrset key)";
      };
      flake_output = mkOption {
        type = types.str;
        description = "Flake output attribute path";
        default = "";
      };
      machines = mkOption {
        type = types.attrsOf (types.submodule MachineDef);
        default = {};
        description = "Machine configurations";
      };
      ssh = mkOption {
        type = (types.submodule SSHClientDef);
        default = {};
      };
      tags = mkOption {
        type = (types.listOf types.str);
        default = [];
      };
      secrets = mkOption {
        type = (types.listOf (types.submodule PlainFileOrDirToTransferDef));
        default = [];
      };
      disabled = mkOption {
        type = types.bool;
        default = false;
      };
      override_sudo_program = mkOption {
        type = types.str;
        default = "";
      };
      hardware_config_path = mkOption {
        type = types.str;
        default = "";
      };
      activation_mode = mkOption {
        type = types.str;
        description = "Activation mode: check, switch, boot, test, dry-activate";
        default = "switch";
      };
      bootstrap = mkOption {
        type = (types.submodule BootstrapDef);
        default = {};
      };
      nix = mkOption {
        type = (types.submodule NixConfigDef);
        default = {};
      };
    };
  };

  FlakeDef = {
    options = {
      name = mkOption {
        type = types.str;
        description = "Flake name (auto-populated from attrset key)";
      };
      url = mkOption {
        type = types.str;
        description = "Flake URL or path";
      };
      configurations = mkOption {
        type = types.attrsOf (types.submodule ConfigurationDef);
        default = {};
        description = "Configurations for this flake";
      };
      ssh = mkOption {
        type = (types.submodule SSHClientDef);
        default = {};
      };
      tags = mkOption {
        type = (types.listOf types.str);
        default = [];
      };
      secrets = mkOption {
        type = (types.listOf (types.submodule PlainFileOrDirToTransferDef));
        default = [];
      };
      disabled = mkOption {
        type = types.bool;
        default = false;
      };
      override_sudo_program = mkOption {
        type = types.str;
        default = "";
      };
      hardware_config_path = mkOption {
        type = types.str;
        default = "";
      };
      activation_mode = mkOption {
        type = types.str;
        description = "Activation mode: check, switch, boot, test, dry-activate";
        default = "switch";
      };
      bootstrap = mkOption {
        type = (types.submodule BootstrapDef);
        default = {};
      };
      nix = mkOption {
        type = (types.submodule NixConfigDef);
        default = {};
      };
    };
  };

  FlagsDef = {
    options = {
      config = mkOption {
        type = types.str;
        default = "panix.nix";
      };
      env = mkOption {
        type = types.str;
        default = "";
      };
      no_validate_paths = mkOption {
        type = types.bool;
        default = false;
      };
      tags = mkOption {
        type = (types.listOf types.str);
        default = [];
      };
      bootstrap = mkOption {
        type = types.submodule FlagsBootstrapDef;
        default = {};
      };
      require_all_success = mkOption {
        type = types.bool;
        default = false;
      };
      override_local_machine = mkOption {
        type = types.str;
        default = "";
      };
      dry_run = mkOption {
        type = types.bool;
        default = false;
      };
      dry_run_with_inspect = mkOption {
        type = types.bool;
        default = false;
      };
      timeout = mkOption {
        type = types.str;
        default = "2h";
      };
      skip_phases = mkOption {
        type = (types.listOf types.str);
        default = [];
      };
      exit_on_complete = mkOption {
        type = types.bool;
        default = false;
      };
      activation_mode = mkOption {
        type = types.str;
        default = "switch";
      };
      showallbuildlogs = mkOption {
        type = types.bool;
        default = false;
      };
      showactiveonly = mkOption {
        type = types.bool;
        default = false;
      };
      showcommandsinlabels = mkOption {
        type = types.bool;
        default = false;
      };
      commandoutputmaxheight = mkOption {
        type = types.int;
        default = 8;
      };
      log = mkOption {
        type = types.bool;
        default = false;
      };
      logfile = mkOption {
        type = types.str;
        default = "panix.log";
      };
      debug = mkOption {
        type = types.bool;
        default = false;
      };
      cpuprofile = mkOption {
        type = types.str;
        default = "";
      };
    };
  };

in
{
  options = {
    flags = mkOption {
      type = types.submodule FlagsDef;
      default = {};
      description = "Global configuration flags";
    };

    flakes = mkOption {
      type = types.attrsOf (types.submodule FlakeDef);
      default = {};
      description = "Flake configurations";
    };
  };
}
