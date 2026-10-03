{
  imports = [./configuration.nix];

  # E2E-only guard fixture for the committed-path legs (tests/e2e/guard_v2.go):
  # activation succeeds, so the magic gate confirms over the wire and the
  # guardian commits. This is the e2e's commit coverage: the relay must map
  # the post-commit event-pipe EOF to the terminal COMMITTED record (exit 0),
  # never to exit 6 or a revert. The marker proves the new closure went live
  # on the machine after the commit's profile set.
  environment.etc."panix-guard-commit-marker".text = "panix-e2e-commit-ok";
}
