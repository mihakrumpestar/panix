{
  imports = [./configuration.nix];

  # E2E-only guard fixture for the hang leg (tests/e2e/guard.go). Activation
  # sleeps forever, so the guardian kills the child at the activation deadline
  # and reverts (spec 6.2). Profile-last ordering keeps the profile untouched,
  # so the generation list stays unchanged.
  system.activationScripts.panix-hang-for-e2e = ''
    echo "panix-e2e: hanging activation until the guard deadline" >&2
    sleep 600
  '';
}
