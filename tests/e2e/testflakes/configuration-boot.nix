{ pkgs, ... }:

{
  imports = [./configuration.nix];

  # E2E-only guard fixture for the boot-mode commit leg (tests/e2e/guard_v2.go).
  # Boot mode runs switch-to-configuration boot (bootloader only), so system
  # activation scripts never run; the marker only differentiates this closure
  # from test-vm's, it is never asserted as a live file on the machine. The
  # boot-mode assertions are record-driven: the TXN record must carry the
  # pre-start boot set (profile set at start, empty commit list) and the
  # profile must resolve to the TXN's new closure after the commit.
  environment.etc."panix-guard-boot-marker".text = "panix-e2e-boot-ok";

  # Boot-only activation wrapper, shared with the boot-mode revert leg's
  # injection (tests/e2e/guard_v2.go): the revert deploy overrides the
  # user-overridable activation_path preset field with this closure-relative
  # path. It exists only in this closure (the revert deploy's configuration-
  # commit closure does not ship it), so that deploy's activation child fails
  # immediately (fork/exec: no such file), while its revert re-runs OLD's
  # activation through the very same wrapper, which execs the real STC of the
  # boot closure: the genuine boot-mode revert, idempotent. Inert for this
  # commit leg (its default activation_path is bin/switch-to-configuration).
  # Shipped via writeShellScript: the store side of a symlinked etc entry is
  # immutable 0444 for text entries (the mode attribute only affects the /etc
  # side, which is never materialized here), and fork/exec needs an
  # executable bit on the store file; writeShellScript outputs are 0555.
  # The inner exec resolves through the SYSTEM PROFILE, not closure-relative
  # paths: the closure's etc is a symlink into the etc-composite store path,
  # so any ../ escape from etc/ resolves against the composite's parent and
  # can never reach <toplevel>/bin. The wrapper only ever executes during the
  # revert, after the ra list's first step has restored the profile to OLD,
  # so the profile-relative resolution IS the intended OLD activation.
  environment.etc."panix-e2e-boot-activation".source =
    pkgs.writeShellScript "panix-e2e-boot-activation" ''
      # Resolve through the system profile: symlink-proof and toplevel-
      # agnostic (see the comment above for why closure-relative fails).
      exec "$(readlink -f /nix/var/nix/profiles/system)/bin/switch-to-configuration" "$@"
    '';
}
