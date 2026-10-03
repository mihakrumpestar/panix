{
  imports = [./configuration.nix];

  # E2E-only retry fixture for the guard legs (tests/e2e/guard.go): the good
  # configuration plus one extra marker file. The extra file makes this
  # closure differ from test-vm's, so the retry deploy advances the system
  # profile to a new generation and the standalone rollback leg has a real
  # previous generation to roll back to: a same-closure deploy leaves the
  # generation list unchanged (verified on nix-env --set).
  environment.etc."panix-retry-marker".text = "panix-e2e-retry-generation";
}
