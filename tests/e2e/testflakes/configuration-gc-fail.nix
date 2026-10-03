{
  config,
  pkgs,
  ...
}: {
  imports = [./configuration.nix];

  # E2E-only guard fixture for the GC-during-window leg (tests/e2e/guard.go).
  # Activation runs a full garbage collect (deleting every non-current
  # generation) and then fails: the slot's gc-root protects the new closure
  # and the old one is the current generation, so the revert still finds both
  # closures intact (spec 4.3).
  system.activationScripts.panix-gc-fail-for-e2e =
    let
      nixCollectGarbage = "${pkgs.nix}/bin/nix-collect-garbage";
    in
    ''
      echo "panix-e2e: collecting garbage mid-window" >&2
      ${nixCollectGarbage} -d
      echo "panix-e2e: gc done, failing activation" >&2
      exit 1
    '';
}
