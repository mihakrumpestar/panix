{
  imports = [./configuration.nix];

  # E2E-only failure injection for the guard rollback leg (tests/e2e/guard.go).
  # This dedicated configuration ALWAYS fails activation. Under the guard's
  # profile-last ordering the profile is never switched, so the guardian's
  # revert restores the previous closure and deletes nothing: the failed
  # generation concept never exists. The echoed marker text is asserted by the
  # e2e as the original error text panix must surface (not just "reverted").
  system.activationScripts.panix-fail-for-e2e = ''
    echo "panix-e2e: forcing activation failure" >&2
    exit 1
  '';
}
