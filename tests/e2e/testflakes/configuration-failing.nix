{
  imports = [./configuration.nix];

  # E2E-only failure injection for the auto-rollback test. This dedicated
  # configuration ALWAYS fails activation, right after the system profile has
  # been switched by nix-env --set. This gives panix auto-rollback a valid
  # previous generation to restore, deterministically, without depending on a
  # runtime flag file or on a fresh evaluation of the main configuration.nix.
  system.activationScripts.panix-fail-for-e2e = ''
    echo "panix-e2e: forcing activation failure" >&2
    exit 1
  '';
}
