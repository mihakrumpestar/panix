{
  imports = [./configuration.nix];

  # E2E-only guard fixture for the sshd-kill leg (tests/e2e/guard.go).
  # Activation stops sshd mid-window, so panix loses the transport while the
  # detached guardian survives (KillUserProcesses=no, spec 10.2). The
  # activation then hangs, so the guardian kills the child at the activation
  # deadline and reverts; the revert re-runs the previous generation's
  # switch-to-configuration, which starts sshd again and heals the machine.
  system.activationScripts.panix-sshd-kill-for-e2e = ''
    echo "panix-e2e: stopping sshd for the transport-failure leg" >&2
    sleep 5
    systemctl stop sshd
    echo "panix-e2e: sshd stopped, hanging until the guardian reverts" >&2
    sleep 600
  '';
}
